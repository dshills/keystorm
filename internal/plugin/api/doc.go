// Package api provides the Lua API modules exposed to Keystorm plugins.
//
// The API package implements the bridge between plugin Lua scripts and the
// Keystorm editor core. Plugins access editor functionality through the "ks"
// namespace, which aggregates several submodules:
//
//   - ks.buf: Buffer operations (read, write, insert, delete)
//   - ks.cursor: Cursor manipulation (position, selection, multi-cursor)
//   - ks.mode: Mode queries and switching (normal, insert, visual, etc.)
//   - ks.util: Utility functions (string manipulation, table helpers)
//
// Additional modules planned for future phases:
//   - ks.keymap: Keybinding registration
//   - ks.command: Command palette registration
//   - ks.event: Event subscription
//   - ks.config: Configuration access
//   - ks.ui: UI notifications and overlays
//   - ks.lsp: Language server protocol access
//
// # Architecture
//
// Each API module implements the Module interface:
//
//	type Module interface {
//	    Name() string
//	    RequiredCapability() security.Capability
//	    Register(L *lua.LState) error
//	}
//
// Modules are registered into a Registry which handles:
//   - Module lifecycle management
//   - Capability-based access control
//   - Injection into Lua states
//
// # Capability-Based Security
//
// API modules declare their required capabilities. When injecting modules
// into a plugin's Lua state, the Registry checks the plugin's granted
// capabilities and only injects modules the plugin has permission to use.
//
// For example, the buffer module requires CapabilityBuffer. A plugin without
// this capability will not have access to ks.buf functions.
//
// # Context
//
// The Context struct provides API modules with access to editor state:
//
//	ctx := &api.Context{
//	    Buffer: myBufferProvider,  // BufferProvider interface
//	    Cursor: myCursorProvider,  // CursorProvider interface
//	    Mode:   myModeProvider,    // ModeProvider interface
//	}
//
// Provider interfaces abstract the editor internals, allowing modules to
// operate independently of specific editor implementations.
//
// # Usage
//
// To set up API modules for a plugin:
//
//	// Create context with editor providers
//	ctx := &api.Context{
//	    Buffer: bufferAdapter,
//	    Cursor: cursorAdapter,
//	    Mode:   modeAdapter,
//	}
//
//	// Create registry with default modules
//	registry := api.DefaultRegistry(ctx)
//
//	// Inject into plugin's Lua state
//	err := registry.InjectAll(luaState, permissionChecker)
//
// From Lua, plugins access the API:
//
//	local ks = require("ks")
//
//	-- Read buffer content
//	local text = ks.buf.text()
//	local line = ks.buf.line(1)
//
//	-- Manipulate cursor
//	local pos = ks.cursor.get()
//	ks.cursor.set(pos + 10)
//
//	-- Check mode
//	if ks.mode.is("normal") then
//	    ks.mode.switch("insert")
//	end
//
//	-- Use utilities
//	local parts = ks.util.split("a,b,c", ",")
//	local trimmed = ks.util.trim("  hello  ")
package api
