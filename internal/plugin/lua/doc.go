// Package lua provides the Lua runtime integration for the plugin system.
//
// This package wraps the gopher-lua library to provide:
//   - Sandboxed Lua state management
//   - Go-Lua type conversion bridge
//   - Capability-based security
//   - Execution timeouts and instruction limits
//
// # State
//
// The State type manages a Lua runtime with sandboxing:
//
//	state, err := lua.NewState(
//	    lua.WithMemoryLimit(10 * 1024 * 1024),
//	    lua.WithExecutionTimeout(5 * time.Second),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer state.Close()
//
//	if err := state.DoFile("plugin.lua"); err != nil {
//	    log.Fatal(err)
//	}
//
// # Sandbox
//
// The Sandbox restricts Lua code execution by:
//   - Removing dangerous functions (dofile, loadfile, load)
//   - Restricting os module to safe functions only
//   - Counting instructions to prevent infinite loops
//   - Enforcing capability requirements
//
// # Bridge
//
// The Bridge provides bidirectional type conversion:
//
//	bridge := lua.NewBridge(state.LuaState())
//
//	// Go to Lua
//	luaVal := bridge.ToLuaValue(map[string]interface{}{
//	    "name": "test",
//	    "count": 42,
//	})
//
//	// Lua to Go
//	goVal := bridge.ToGoValue(luaVal)
//
// # Capabilities
//
// Plugins must be granted capabilities to access restricted functionality:
//
//	state.Sandbox().Grant(lua.CapabilityFileRead)
//	state.Sandbox().Grant(lua.CapabilityNetwork)
//
// Available capabilities:
//   - CapabilityFileRead: Read files from filesystem
//   - CapabilityFileWrite: Write files to filesystem
//   - CapabilityNetwork: Make network requests
//   - CapabilityShell: Execute shell commands
//   - CapabilityClipboard: Access system clipboard
//   - CapabilityProcess: Spawn child processes
//   - CapabilityUnsafe: Disable all sandbox restrictions
package lua
