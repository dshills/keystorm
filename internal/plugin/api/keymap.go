package api

import (
	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/input/keymap"
	"github.com/dshills/keystorm/internal/plugin/security"
)

// KeymapProvider defines the interface for keymap operations.
type KeymapProvider interface {
	// Register registers a keymap with the system.
	Register(km *keymap.Keymap) error

	// Unregister removes a keymap by name.
	Unregister(name string)

	// Get returns a keymap by name.
	Get(name string) *keymap.ParsedKeymap

	// AllBindings returns all bindings for a mode.
	AllBindings(mode string) []keymap.BindingMatch
}

// KeymapModule implements the ks.keymap API module.
type KeymapModule struct {
	ctx        *Context
	pluginName string
}

// NewKeymapModule creates a new keymap module.
func NewKeymapModule(ctx *Context, pluginName string) *KeymapModule {
	return &KeymapModule{ctx: ctx, pluginName: pluginName}
}

// Name returns the module name.
func (m *KeymapModule) Name() string {
	return "keymap"
}

// RequiredCapability returns the capability required for this module.
func (m *KeymapModule) RequiredCapability() security.Capability {
	return security.CapabilityKeymap
}

// Register registers the module into the Lua state.
func (m *KeymapModule) Register(L *lua.LState) error {
	mod := L.NewTable()

	// Register keymap functions
	L.SetField(mod, "set", L.NewFunction(m.set))
	L.SetField(mod, "del", L.NewFunction(m.del))
	L.SetField(mod, "get", L.NewFunction(m.get))
	L.SetField(mod, "list", L.NewFunction(m.list))

	L.SetGlobal("_ks_keymap", mod)
	return nil
}

// set(mode, keys, action, opts?) -> nil
// Sets a keybinding for a mode.
// opts can include: desc, when, priority, category
func (m *KeymapModule) set(L *lua.LState) int {
	mode := L.CheckString(1)
	keys := L.CheckString(2)
	action := L.CheckString(3)

	if keys == "" {
		L.ArgError(2, "keys cannot be empty")
		return 0
	}
	if action == "" {
		L.ArgError(3, "action cannot be empty")
		return 0
	}

	if m.ctx.Keymap == nil {
		L.RaiseError("set: no keymap provider available")
		return 0
	}

	// Parse optional options table
	var desc, when, category string
	var priority int
	if L.GetTop() >= 4 {
		opts := L.CheckTable(4)
		desc = getTableString(L, opts, "desc")
		when = getTableString(L, opts, "when")
		category = getTableString(L, opts, "category")
		priority = getTableInt(L, opts, "priority")
	}

	// Create binding
	binding := keymap.Binding{
		Keys:        keys,
		Action:      action,
		Description: desc,
		When:        when,
		Priority:    priority,
		Category:    category,
	}

	// Create a unique keymap name for this binding
	// Format: pluginName_mode_keys (sanitized)
	kmName := m.keymapName(mode, keys)

	// Create keymap for this plugin
	km := keymap.NewKeymap(kmName).
		ForMode(mode).
		WithSource("plugin:" + m.pluginName).
		AddBinding(binding)

	// Register with system
	if err := m.ctx.Keymap.Register(km); err != nil {
		L.RaiseError("set: %v", err)
		return 0
	}

	return 0
}

// del(mode, keys) -> nil
// Removes a keybinding.
func (m *KeymapModule) del(L *lua.LState) int {
	mode := L.CheckString(1)
	keys := L.CheckString(2)

	if keys == "" {
		L.ArgError(2, "keys cannot be empty")
		return 0
	}

	if m.ctx.Keymap == nil {
		L.RaiseError("del: no keymap provider available")
		return 0
	}

	// Generate the same keymap name that was used in set()
	kmName := m.keymapName(mode, keys)

	// Unregister the keymap
	m.ctx.Keymap.Unregister(kmName)

	return 0
}

// get(mode, keys) -> table or nil
// Gets a keybinding by mode and keys.
// Returns nil if not found.
func (m *KeymapModule) get(L *lua.LState) int {
	mode := L.CheckString(1)
	keys := L.CheckString(2)

	if keys == "" {
		L.ArgError(2, "keys cannot be empty")
		return 0
	}

	if m.ctx.Keymap == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Get the keymap by name
	kmName := m.keymapName(mode, keys)
	km := m.ctx.Keymap.Get(kmName)

	if km == nil || len(km.ParsedBindings) == 0 {
		L.Push(lua.LNil)
		return 1
	}

	// Return the first binding as a table
	binding := km.ParsedBindings[0]
	tbl := L.NewTable()
	L.SetField(tbl, "keys", lua.LString(binding.Keys))
	L.SetField(tbl, "action", lua.LString(binding.Action))
	L.SetField(tbl, "desc", lua.LString(binding.Description))
	L.SetField(tbl, "when", lua.LString(binding.When))
	L.SetField(tbl, "priority", lua.LNumber(binding.Priority))
	L.SetField(tbl, "category", lua.LString(binding.Category))

	L.Push(tbl)
	return 1
}

// list(mode?) -> {bindings...}
// Lists all keybindings, optionally filtered by mode.
func (m *KeymapModule) list(L *lua.LState) int {
	mode := L.OptString(1, "")

	if m.ctx.Keymap == nil {
		L.Push(L.NewTable())
		return 1
	}

	// Get all bindings for the mode
	bindings := m.ctx.Keymap.AllBindings(mode)

	// Filter to only show bindings from this plugin
	source := "plugin:" + m.pluginName
	result := L.NewTable()
	idx := 1

	for _, match := range bindings {
		// Only include bindings from this plugin
		if match.Keymap != nil && match.Keymap.Source == source {
			tbl := L.NewTable()
			L.SetField(tbl, "keys", lua.LString(match.Keys))
			L.SetField(tbl, "action", lua.LString(match.Action))
			L.SetField(tbl, "mode", lua.LString(match.Keymap.Mode))
			L.SetField(tbl, "desc", lua.LString(match.Description))
			L.SetField(tbl, "when", lua.LString(match.When))
			L.SetField(tbl, "priority", lua.LNumber(match.Priority))
			L.SetField(tbl, "category", lua.LString(match.Category))

			result.RawSetInt(idx, tbl)
			idx++
		}
	}

	L.Push(result)
	return 1
}

// keymapName generates a unique keymap name for a binding.
func (m *KeymapModule) keymapName(mode, keys string) string {
	// Sanitize keys for use in name (replace special chars)
	sanitized := sanitizeForName(keys)
	return m.pluginName + "_" + mode + "_" + sanitized
}

// sanitizeForName converts a key sequence to a safe name component.
func sanitizeForName(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			result = append(result, c)
		case c == ' ':
			result = append(result, '_')
		case c == '-', c == '_':
			result = append(result, c)
		default:
			// Convert special characters to hex representation
			result = append(result, 'x')
			result = append(result, hexDigit(c>>4))
			result = append(result, hexDigit(c&0xf))
		}
	}
	return string(result)
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + (n - 10)
}

// getTableString gets a string field from a Lua table.
func getTableString(L *lua.LState, tbl *lua.LTable, field string) string {
	val := L.GetField(tbl, field)
	if str, ok := val.(lua.LString); ok {
		return string(str)
	}
	return ""
}

// getTableInt gets an int field from a Lua table.
func getTableInt(L *lua.LState, tbl *lua.LTable, field string) int {
	val := L.GetField(tbl, field)
	if num, ok := val.(lua.LNumber); ok {
		return int(num)
	}
	return 0
}
