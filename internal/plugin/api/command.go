package api

import (
	"fmt"
	"sync"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/input/palette"
	"github.com/dshills/keystorm/internal/plugin/security"
)

// CommandProvider defines the interface for command/palette operations.
type CommandProvider interface {
	// Register adds a command to the palette.
	Register(cmd *palette.Command) error

	// Unregister removes a command from the palette.
	Unregister(id string) bool

	// UnregisterBySource removes all commands from a specific source.
	UnregisterBySource(source string) int

	// Get retrieves a command by ID.
	Get(id string) *palette.Command

	// Has checks if a command exists.
	Has(id string) bool

	// Execute runs a command by ID with arguments.
	Execute(id string, args map[string]any) error

	// All returns all registered commands.
	All() []*palette.Command
}

// CommandModule implements the ks.command API module.
type CommandModule struct {
	ctx        *Context
	pluginName string
	L          *lua.LState // Store the Lua state for handler callbacks

	// Track registered command IDs for cleanup
	mu         sync.Mutex
	commandIDs map[string]bool
	handlerKey string // Key in registry for storing handler functions
	handlerTbl *lua.LTable
}

// NewCommandModule creates a new command module.
func NewCommandModule(ctx *Context, pluginName string) *CommandModule {
	return &CommandModule{
		ctx:        ctx,
		pluginName: pluginName,
		commandIDs: make(map[string]bool),
		handlerKey: "_ks_cmd_handlers_" + pluginName,
	}
}

// Name returns the module name.
func (m *CommandModule) Name() string {
	return "command"
}

// RequiredCapability returns the capability required for this module.
func (m *CommandModule) RequiredCapability() security.Capability {
	return security.CapabilityCommand
}

// Register registers the module into the Lua state.
func (m *CommandModule) Register(L *lua.LState) error {
	// Store the Lua state for use in handlers
	m.L = L

	// Create a table to store handler functions (prevents GC)
	m.handlerTbl = L.NewTable()
	L.SetGlobal(m.handlerKey, m.handlerTbl)

	mod := L.NewTable()

	// Register command functions
	L.SetField(mod, "register", L.NewFunction(m.register))
	L.SetField(mod, "unregister", L.NewFunction(m.unregister))
	L.SetField(mod, "execute", L.NewFunction(m.execute))
	L.SetField(mod, "list", L.NewFunction(m.list))

	L.SetGlobal("_ks_command", mod)
	return nil
}

// Cleanup releases all handler references.
// This should be called when the plugin is unloaded.
func (m *CommandModule) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.L == nil {
		return
	}

	// Clear the handler table
	m.L.SetGlobal(m.handlerKey, lua.LNil)
	m.handlerTbl = nil
	m.commandIDs = make(map[string]bool)

	// Unregister all commands from this plugin
	if m.ctx.Command != nil {
		m.ctx.Command.UnregisterBySource("plugin:" + m.pluginName)
	}
}

// register(opts) -> nil
// Registers a command with the palette.
// opts must include: id, title, handler
// opts can include: description, category, when
func (m *CommandModule) register(L *lua.LState) int {
	opts := L.CheckTable(1)

	// Get required fields
	id := getTableString(L, opts, "id")
	title := getTableString(L, opts, "title")
	handler := L.GetField(opts, "handler")

	if id == "" {
		L.ArgError(1, "id is required")
		return 0
	}
	if title == "" {
		L.ArgError(1, "title is required")
		return 0
	}
	if handler.Type() != lua.LTFunction {
		L.ArgError(1, "handler must be a function")
		return 0
	}

	if m.ctx.Command == nil {
		L.RaiseError("register: no command provider available")
		return 0
	}

	// Get optional fields
	description := getTableString(L, opts, "description")
	category := getTableString(L, opts, "category")
	when := getTableString(L, opts, "when")

	// Store handler in our table to prevent GC
	m.mu.Lock()
	if m.handlerTbl != nil {
		m.handlerTbl.RawSetString(id, handler)
	}
	m.commandIDs[id] = true
	m.mu.Unlock()

	// Create Go handler that calls the Lua function
	goHandler := m.createHandler(id)

	// Create command
	cmd := &palette.Command{
		ID:          id,
		Title:       title,
		Description: description,
		Category:    category,
		When:        when,
		Handler:     goHandler,
		Source:      "plugin:" + m.pluginName,
	}

	// Register with palette
	if err := m.ctx.Command.Register(cmd); err != nil {
		L.RaiseError("register: %v", err)
		return 0
	}

	return 0
}

// createHandler creates a Go handler that calls a Lua function.
func (m *CommandModule) createHandler(cmdID string) palette.CommandHandler {
	return func(args map[string]any) error {
		m.mu.Lock()
		L := m.L
		handlerTbl := m.handlerTbl
		m.mu.Unlock()

		if L == nil || handlerTbl == nil {
			return fmt.Errorf("plugin unloaded")
		}

		// Get the handler function from our table
		handler := L.GetField(handlerTbl, cmdID)
		if handler.Type() != lua.LTFunction {
			return fmt.Errorf("handler not found for command %s", cmdID)
		}

		// Convert args to Lua table
		argsTable := m.mapToTable(L, args)

		// Push handler and args onto stack
		L.Push(handler)
		L.Push(argsTable)

		// Call the handler
		if err := L.PCall(1, 0, nil); err != nil {
			return fmt.Errorf("command %s handler error: %w", cmdID, err)
		}

		return nil
	}
}

// unregister(id) -> bool
// Unregisters a command from the palette.
// Returns true if the command existed.
func (m *CommandModule) unregister(L *lua.LState) int {
	id := L.CheckString(1)

	if id == "" {
		L.ArgError(1, "id cannot be empty")
		return 0
	}

	if m.ctx.Command == nil {
		L.Push(lua.LFalse)
		return 1
	}

	// Check if command belongs to this plugin
	cmd := m.ctx.Command.Get(id)
	if cmd == nil || cmd.Source != "plugin:"+m.pluginName {
		L.Push(lua.LFalse)
		return 1
	}

	// Remove from our handler table
	m.mu.Lock()
	if m.handlerTbl != nil {
		m.handlerTbl.RawSetString(id, lua.LNil)
	}
	delete(m.commandIDs, id)
	m.mu.Unlock()

	// Unregister the command
	existed := m.ctx.Command.Unregister(id)
	L.Push(lua.LBool(existed))
	return 1
}

// execute(id, args?) -> result
// Executes a command by ID.
func (m *CommandModule) execute(L *lua.LState) int {
	id := L.CheckString(1)

	if id == "" {
		L.ArgError(1, "id cannot be empty")
		return 0
	}

	if m.ctx.Command == nil {
		L.RaiseError("execute: no command provider available")
		return 0
	}

	// Parse optional args table
	var args map[string]any
	if L.GetTop() >= 2 {
		argsTable := L.CheckTable(2)
		args = m.tableToMap(L, argsTable)
	}

	// Execute the command
	if err := m.ctx.Command.Execute(id, args); err != nil {
		L.RaiseError("execute: %v", err)
		return 0
	}

	return 0
}

// list() -> {commands...}
// Lists all commands registered by this plugin.
func (m *CommandModule) list(L *lua.LState) int {
	if m.ctx.Command == nil {
		L.Push(L.NewTable())
		return 1
	}

	// Get all commands
	allCommands := m.ctx.Command.All()

	// Filter to only this plugin's commands
	source := "plugin:" + m.pluginName
	result := L.NewTable()
	idx := 1

	for _, cmd := range allCommands {
		if cmd.Source == source {
			tbl := L.NewTable()
			L.SetField(tbl, "id", lua.LString(cmd.ID))
			L.SetField(tbl, "title", lua.LString(cmd.Title))
			L.SetField(tbl, "description", lua.LString(cmd.Description))
			L.SetField(tbl, "category", lua.LString(cmd.Category))
			L.SetField(tbl, "when", lua.LString(cmd.When))

			result.RawSetInt(idx, tbl)
			idx++
		}
	}

	L.Push(result)
	return 1
}

// mapToTable converts a Go map to a Lua table.
func (m *CommandModule) mapToTable(L *lua.LState, args map[string]any) *lua.LTable {
	if args == nil {
		return L.NewTable()
	}

	tbl := L.NewTable()
	for k, v := range args {
		tbl.RawSetString(k, m.anyToLValue(L, v))
	}
	return tbl
}

// tableToMap converts a Lua table to a Go map.
func (m *CommandModule) tableToMap(L *lua.LState, tbl *lua.LTable) map[string]any {
	result := make(map[string]any)
	tbl.ForEach(func(key, value lua.LValue) {
		if keyStr, ok := key.(lua.LString); ok {
			result[string(keyStr)] = m.lvalueToAny(value)
		}
	})
	return result
}

// anyToLValue converts a Go value to a Lua value.
func (m *CommandModule) anyToLValue(L *lua.LState, v any) lua.LValue {
	switch val := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case string:
		return lua.LString(val)
	case []any:
		tbl := L.NewTable()
		for i, item := range val {
			tbl.RawSetInt(i+1, m.anyToLValue(L, item))
		}
		return tbl
	case map[string]any:
		return m.mapToTable(L, val)
	default:
		return lua.LString(fmt.Sprintf("%v", val))
	}
}

// lvalueToAny converts a Lua value to a Go value.
func (m *CommandModule) lvalueToAny(v lua.LValue) any {
	if v == nil || v == lua.LNil {
		return nil
	}

	switch val := v.(type) {
	case lua.LBool:
		return bool(val)
	case lua.LNumber:
		return float64(val)
	case lua.LString:
		return string(val)
	case *lua.LTable:
		// Check if it's an array-like table
		isArray := true
		maxIdx := 0
		val.ForEach(func(k, _ lua.LValue) {
			if num, ok := k.(lua.LNumber); ok {
				idx := int(num)
				if idx > maxIdx {
					maxIdx = idx
				}
			} else {
				isArray = false
			}
		})

		if isArray && maxIdx > 0 {
			arr := make([]any, maxIdx)
			val.ForEach(func(k, v lua.LValue) {
				if num, ok := k.(lua.LNumber); ok {
					idx := int(num) - 1
					if idx >= 0 && idx < maxIdx {
						arr[idx] = m.lvalueToAny(v)
					}
				}
			})
			return arr
		}

		// Treat as map
		result := make(map[string]any)
		val.ForEach(func(k, v lua.LValue) {
			var keyStr string
			switch key := k.(type) {
			case lua.LString:
				keyStr = string(key)
			case lua.LNumber:
				keyStr = fmt.Sprintf("%v", float64(key))
			default:
				keyStr = k.String()
			}
			result[keyStr] = m.lvalueToAny(v)
		})
		return result
	default:
		return v.String()
	}
}
