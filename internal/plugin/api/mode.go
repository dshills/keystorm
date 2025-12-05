package api

import (
	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// ModeModule implements the ks.mode API module.
type ModeModule struct {
	ctx *Context
}

// NewModeModule creates a new mode module.
func NewModeModule(ctx *Context) *ModeModule {
	return &ModeModule{ctx: ctx}
}

// Name returns the module name.
func (m *ModeModule) Name() string {
	return "mode"
}

// RequiredCapability returns the capability required for this module.
// Mode operations require no special capability by default.
func (m *ModeModule) RequiredCapability() security.Capability {
	return "" // No special capability required for basic mode operations
}

// Register registers the module into the Lua state.
func (m *ModeModule) Register(L *lua.LState) error {
	mod := L.NewTable()

	// Register all mode functions
	L.SetField(mod, "current", L.NewFunction(m.current))
	L.SetField(mod, "switch", L.NewFunction(m.switchMode))
	L.SetField(mod, "is", L.NewFunction(m.is))

	// Common mode constants
	L.SetField(mod, "NORMAL", lua.LString("normal"))
	L.SetField(mod, "INSERT", lua.LString("insert"))
	L.SetField(mod, "VISUAL", lua.LString("visual"))
	L.SetField(mod, "VISUAL_LINE", lua.LString("visual_line"))
	L.SetField(mod, "VISUAL_BLOCK", lua.LString("visual_block"))
	L.SetField(mod, "COMMAND", lua.LString("command"))
	L.SetField(mod, "REPLACE", lua.LString("replace"))
	L.SetField(mod, "OPERATOR_PENDING", lua.LString("operator_pending"))

	L.SetGlobal("_ks_mode", mod)
	return nil
}

// current() -> string
// Returns the current mode name.
func (m *ModeModule) current(L *lua.LState) int {
	if m.ctx.Mode == nil {
		L.Push(lua.LString("normal"))
		return 1
	}

	L.Push(lua.LString(m.ctx.Mode.Current()))
	return 1
}

// switch(mode) -> nil
// Switches to a different mode.
func (m *ModeModule) switchMode(L *lua.LState) int {
	mode := L.CheckString(1)

	if mode == "" {
		L.ArgError(1, "mode cannot be empty")
		return 0
	}

	if m.ctx.Mode == nil {
		L.RaiseError("switch: no mode manager available")
		return 0
	}

	if err := m.ctx.Mode.Switch(mode); err != nil {
		L.RaiseError("switch: %v", err)
		return 0
	}

	return 0
}

// is(mode) -> bool
// Checks if currently in the given mode.
func (m *ModeModule) is(L *lua.LState) int {
	mode := L.CheckString(1)

	if m.ctx.Mode == nil {
		// Default to normal mode if no mode manager
		L.Push(lua.LBool(mode == "normal"))
		return 1
	}

	L.Push(lua.LBool(m.ctx.Mode.Is(mode)))
	return 1
}
