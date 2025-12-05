package lua

import (
	"os"
	"sync/atomic"

	lua "github.com/yuin/gopher-lua"
)

// Sandbox restricts Lua execution to safe operations.
type Sandbox struct {
	L *lua.LState

	// Instruction limiting
	instructionLimit int64
	instructionCount int64

	// Capabilities
	capabilities map[Capability]bool
}

// Capability represents a permission that can be granted to plugins.
type Capability string

// Available capabilities.
const (
	CapabilityFileRead  Capability = "filesystem.read"
	CapabilityFileWrite Capability = "filesystem.write"
	CapabilityNetwork   Capability = "network"
	CapabilityShell     Capability = "shell"
	CapabilityClipboard Capability = "clipboard"
	CapabilityProcess   Capability = "process.spawn"
	CapabilityUnsafe    Capability = "unsafe" // Full Lua stdlib access
)

// NewSandbox creates a new sandbox for the Lua state.
func NewSandbox(L *lua.LState, instructionLimit int64) *Sandbox {
	return &Sandbox{
		L:                L,
		instructionLimit: instructionLimit,
		capabilities:     make(map[Capability]bool),
	}
}

// Install sets up the sandbox restrictions.
func (s *Sandbox) Install() {
	// Remove dangerous functions that could be used to bypass sandbox
	dangerousFuncs := []string{
		"dofile",     // Load and execute file
		"loadfile",   // Load file as function
		"load",       // Load string as function
		"loadstring", // Load string as function (deprecated but may exist)
	}

	for _, name := range dangerousFuncs {
		s.L.SetGlobal(name, lua.LNil)
	}

	// Remove rawset/rawget if we want stricter sandboxing
	// For now, keep them as they're useful and not a major security risk

	// Replace print with a safe version that can be captured
	s.installSafePrint()

	// Install a safe require that only allows whitelisted modules
	s.installSafeRequire()
}

// installSafePrint replaces print with a version we can intercept.
func (s *Sandbox) installSafePrint() {
	// Keep default print for now, but this is where we'd redirect output
	// to a plugin log or buffer instead of stdout
}

// installSafeRequire replaces require with a version that only allows safe modules.
// This is a critical security function that prevents arbitrary module loading.
//
// SECURITY: This function clears package.path/cpath to prevent loading modules from
// disk, and replaces require() with a whitelist-based version. Only preloaded modules
// (via L.PreloadModule) and whitelisted built-in modules can be loaded.
func (s *Sandbox) installSafeRequire() {
	// Clear package.path and package.cpath to prevent loading modules from disk.
	// Also clear package.loaded except for already-loaded safe modules.
	pkg := s.L.GetGlobal("package")
	if pkg != lua.LNil {
		if pkgTable, ok := pkg.(*lua.LTable); ok {
			s.L.SetField(pkgTable, "path", lua.LString(""))
			s.L.SetField(pkgTable, "cpath", lua.LString(""))

			// Clear package.loaded to prevent pre-injected modules from being used
			// Keep only the safe built-in modules that are already loaded
			safeLoaded := map[string]bool{
				"_G": true, "string": true, "table": true, "math": true,
				"bit32": true, "utf8": true, "package": true,
			}
			loaded := s.L.GetField(pkgTable, "loaded")
			if loadedTbl, ok := loaded.(*lua.LTable); ok {
				var keysToRemove []string
				loadedTbl.ForEach(func(k, _ lua.LValue) {
					if ks, ok := k.(lua.LString); ok {
						if !safeLoaded[string(ks)] {
							keysToRemove = append(keysToRemove, string(ks))
						}
					}
				})
				for _, key := range keysToRemove {
					loadedTbl.RawSetString(key, lua.LNil)
				}
			}
		}
	}

	// Whitelist of safe modules (built-in to gopher-lua)
	safeModules := map[string]bool{
		"string": true,
		"table":  true,
		"math":   true,
		"bit32":  true,
		"utf8":   true,
	}

	// Store original require for loading preloaded and safe modules
	originalRequire := s.L.GetGlobal("require")

	s.L.SetGlobal("require", s.L.NewFunction(func(L *lua.LState) int {
		modName := L.CheckString(1)

		// Allow safe built-in modules
		if safeModules[modName] {
			L.Push(originalRequire)
			L.Push(lua.LString(modName))
			L.Call(1, 1)
			return 1
		}

		// Allow ks.* modules (will be provided by the editor via PreloadModule)
		if len(modName) > 3 && modName[:3] == "ks." {
			L.Push(originalRequire)
			L.Push(lua.LString(modName))
			L.Call(1, 1)
			return 1
		}

		// Allow the main ks module (provided via PreloadModule)
		if modName == "ks" {
			L.Push(originalRequire)
			L.Push(lua.LString(modName))
			L.Call(1, 1)
			return 1
		}

		// Check for capability-gated modules
		switch modName {
		case "io":
			if !s.capabilities[CapabilityFileRead] && !s.capabilities[CapabilityFileWrite] {
				L.RaiseError("module 'io' requires filesystem capability")
			}
			// If capability is granted, fall through to use original require
			L.Push(originalRequire)
			L.Push(lua.LString(modName))
			L.Call(1, 1)
			return 1
		case "os":
			if !s.capabilities[CapabilityShell] && !s.capabilities[CapabilityProcess] {
				L.RaiseError("module 'os' requires shell or process capability")
			}
			L.Push(originalRequire)
			L.Push(lua.LString(modName))
			L.Call(1, 1)
			return 1
		case "debug":
			if !s.capabilities[CapabilityUnsafe] {
				L.RaiseError("module 'debug' requires unsafe capability")
			}
			L.Push(originalRequire)
			L.Push(lua.LString(modName))
			L.Call(1, 1)
			return 1
		}

		// Reject unknown modules - do not allow arbitrary file loading.
		// Only preloaded modules (via L.PreloadModule) will work.
		// Note: L.RaiseError does a longjmp, so code after it is unreachable.
		L.RaiseError("module %q is not available", modName)
		return 0 // unreachable, but required for Go compiler
	}))
}

// ResetInstructionCount resets the instruction counter.
func (s *Sandbox) ResetInstructionCount() {
	atomic.StoreInt64(&s.instructionCount, 0)
}

// InstructionCount returns the current instruction count.
func (s *Sandbox) InstructionCount() int64 {
	return atomic.LoadInt64(&s.instructionCount)
}

// IncrementInstructions adds to the instruction count and returns true if limit exceeded.
func (s *Sandbox) IncrementInstructions(n int64) bool {
	if s.instructionLimit <= 0 {
		return false
	}
	count := atomic.AddInt64(&s.instructionCount, n)
	return count > s.instructionLimit
}

// Grant enables a capability.
func (s *Sandbox) Grant(cap Capability) {
	s.capabilities[cap] = true

	// Inject corresponding modules based on capability
	switch cap {
	case CapabilityFileRead:
		s.injectFileReadAPI()
	case CapabilityFileWrite:
		s.injectFileWriteAPI()
	case CapabilityNetwork:
		s.injectNetworkAPI()
	case CapabilityShell:
		s.injectShellAPI()
	case CapabilityUnsafe:
		s.injectUnsafeLibraries()
	}
}

// Revoke disables a capability.
func (s *Sandbox) Revoke(cap Capability) {
	delete(s.capabilities, cap)
	// Note: Already injected APIs are not removed
	// A full reset would be needed for that
}

// HasCapability returns true if the capability is granted.
func (s *Sandbox) HasCapability(cap Capability) bool {
	return s.capabilities[cap]
}

// Capabilities returns all granted capabilities.
func (s *Sandbox) Capabilities() []Capability {
	caps := make([]Capability, 0, len(s.capabilities))
	for cap, granted := range s.capabilities {
		if granted {
			caps = append(caps, cap)
		}
	}
	return caps
}

// injectFileReadAPI adds file reading functions.
func (s *Sandbox) injectFileReadAPI() {
	// Create a limited io module for reading
	ioMod := s.L.NewTable()

	// io.open for reading only
	s.L.SetField(ioMod, "open", s.L.NewFunction(func(L *lua.LState) int {
		filename := L.CheckString(1)
		mode := L.OptString(2, "r")

		// Only allow read modes
		if mode != "r" && mode != "rb" {
			L.ArgError(2, "only read modes (r, rb) are allowed")
			return 0
		}

		// Open file using Go's os package
		file, err := os.Open(filename)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		// Create a userdata wrapper for the file
		ud := L.NewUserData()
		ud.Value = file
		L.SetMetatable(ud, s.getFileMetatable())
		L.Push(ud)
		return 1
	}))

	// io.lines for reading lines
	s.L.SetField(ioMod, "lines", s.L.NewFunction(func(L *lua.LState) int {
		filename := L.CheckString(1)
		content, err := os.ReadFile(filename)
		if err != nil {
			L.RaiseError("cannot open file: %s", err.Error())
			return 0
		}

		lines := splitLines(string(content))
		idx := 0

		L.Push(L.NewFunction(func(L *lua.LState) int {
			if idx >= len(lines) {
				return 0
			}
			L.Push(lua.LString(lines[idx]))
			idx++
			return 1
		}))
		return 1
	}))

	// io.read - read from stdin (limited functionality)
	s.L.SetField(ioMod, "read", s.L.NewFunction(func(L *lua.LState) int {
		// Not implemented for plugins - they should use ks.ui for input
		L.Push(lua.LNil)
		return 1
	}))

	s.L.SetGlobal("io", ioMod)
}

// getFileMetatable returns the metatable for file handles.
func (s *Sandbox) getFileMetatable() *lua.LTable {
	mt := s.L.NewTable()
	index := s.L.NewTable()

	// file:read()
	s.L.SetField(index, "read", s.L.NewFunction(func(L *lua.LState) int {
		ud := L.CheckUserData(1)
		file, ok := ud.Value.(*os.File)
		if !ok {
			L.ArgError(1, "expected file")
			return 0
		}

		format := L.OptString(2, "*l")
		switch format {
		case "*a", "*all":
			// Read entire file
			content, err := os.ReadFile(file.Name())
			if err != nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LString(content))
			return 1
		case "*l", "*line":
			// Read a line (simplified - reads all and returns first unread line)
			// This is a simplified implementation
			L.Push(lua.LNil)
			return 1
		case "*n", "*number":
			L.Push(lua.LNil)
			return 1
		default:
			L.Push(lua.LNil)
			return 1
		}
	}))

	// file:close()
	s.L.SetField(index, "close", s.L.NewFunction(func(L *lua.LState) int {
		ud := L.CheckUserData(1)
		file, ok := ud.Value.(*os.File)
		if !ok {
			L.ArgError(1, "expected file")
			return 0
		}
		err := file.Close()
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	// file:lines()
	s.L.SetField(index, "lines", s.L.NewFunction(func(L *lua.LState) int {
		ud := L.CheckUserData(1)
		file, ok := ud.Value.(*os.File)
		if !ok {
			L.ArgError(1, "expected file")
			return 0
		}

		content, err := os.ReadFile(file.Name())
		if err != nil {
			L.RaiseError("cannot read file: %s", err.Error())
			return 0
		}

		lines := splitLines(string(content))
		idx := 0

		L.Push(L.NewFunction(func(L *lua.LState) int {
			if idx >= len(lines) {
				return 0
			}
			L.Push(lua.LString(lines[idx]))
			idx++
			return 1
		}))
		return 1
	}))

	s.L.SetField(mt, "__index", index)
	return mt
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// injectFileWriteAPI adds file writing functions.
func (s *Sandbox) injectFileWriteAPI() {
	// Get or create io module
	ioVal := s.L.GetGlobal("io")
	var ioMod *lua.LTable
	if ioVal == lua.LNil {
		ioMod = s.L.NewTable()
	} else {
		var ok bool
		ioMod, ok = ioVal.(*lua.LTable)
		if !ok {
			ioMod = s.L.NewTable()
		}
	}

	// Extend io.open to allow write modes
	s.L.SetField(ioMod, "open", s.L.NewFunction(func(L *lua.LState) int {
		filename := L.CheckString(1)
		mode := L.OptString(2, "r")

		// Allow all standard modes when write capability is granted
		var flag int
		switch mode {
		case "r", "rb":
			flag = os.O_RDONLY
		case "w", "wb":
			flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		case "a", "ab":
			flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		case "r+", "r+b":
			flag = os.O_RDWR
		case "w+", "w+b":
			flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
		case "a+", "a+b":
			flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
		default:
			L.ArgError(2, "invalid mode")
			return 0
		}

		file, err := os.OpenFile(filename, flag, 0644)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		ud := L.NewUserData()
		ud.Value = file
		L.SetMetatable(ud, s.getWriteFileMetatable())
		L.Push(ud)
		return 1
	}))

	s.L.SetGlobal("io", ioMod)
}

// getWriteFileMetatable returns the metatable for writable file handles.
func (s *Sandbox) getWriteFileMetatable() *lua.LTable {
	mt := s.getFileMetatable()
	index := s.L.GetField(mt, "__index").(*lua.LTable)

	// file:write()
	s.L.SetField(index, "write", s.L.NewFunction(func(L *lua.LState) int {
		ud := L.CheckUserData(1)
		file, ok := ud.Value.(*os.File)
		if !ok {
			L.ArgError(1, "expected file")
			return 0
		}

		// Write all arguments
		for i := 2; i <= L.GetTop(); i++ {
			str := L.CheckString(i)
			_, err := file.WriteString(str)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
		}

		L.Push(ud) // Return file for chaining
		return 1
	}))

	// file:flush()
	s.L.SetField(index, "flush", s.L.NewFunction(func(L *lua.LState) int {
		ud := L.CheckUserData(1)
		file, ok := ud.Value.(*os.File)
		if !ok {
			L.ArgError(1, "expected file")
			return 0
		}

		err := file.Sync()
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LTrue)
		return 1
	}))

	return mt
}

// injectNetworkAPI adds network functions.
func (s *Sandbox) injectNetworkAPI() {
	// Network API would go here
	// For now, we don't implement network access
	// This would require careful consideration of security implications
}

// injectShellAPI adds shell execution functions.
func (s *Sandbox) injectShellAPI() {
	// Create limited os module
	osMod := s.L.NewTable()

	// os.execute - run shell commands
	s.L.SetField(osMod, "execute", s.L.NewFunction(func(L *lua.LState) int {
		// This is intentionally not implemented even with shell capability
		// Plugins should use the editor's job system instead
		L.RaiseError("os.execute is not available; use ks.job.run() instead")
		return 0
	}))

	// os.getenv - read environment variables (relatively safe)
	s.L.SetField(osMod, "getenv", s.L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		value := os.Getenv(name)
		if value == "" {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(value))
		}
		return 1
	}))

	// os.time - get current time
	s.L.SetField(osMod, "time", s.L.NewFunction(func(L *lua.LState) int {
		// Simplified: return current Unix timestamp
		L.Push(lua.LNumber(float64(os.Getpid()))) // Placeholder
		return 1
	}))

	// os.clock - CPU time
	s.L.SetField(osMod, "clock", s.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))

	s.L.SetGlobal("os", osMod)
}

// injectUnsafeLibraries opens all standard Lua libraries.
// This should only be used for trusted plugins.
func (s *Sandbox) injectUnsafeLibraries() {
	lua.OpenIo(s.L)
	lua.OpenOs(s.L)
	lua.OpenDebug(s.L)

	// Restore functions we removed
	// Note: dofile/loadfile/load are part of base which is already loaded
}

// CheckCapability returns an error if the capability is not granted.
func (s *Sandbox) CheckCapability(cap Capability) error {
	if !s.capabilities[cap] {
		return &CapabilityError{Capability: cap}
	}
	return nil
}

// CapabilityError is returned when a capability is not granted.
type CapabilityError struct {
	Capability Capability
}

func (e *CapabilityError) Error() string {
	return "capability not granted: " + string(e.Capability)
}
