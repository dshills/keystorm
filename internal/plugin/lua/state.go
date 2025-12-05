// Package lua provides Lua runtime integration for the plugin system.
package lua

import (
	"fmt"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Default limits for Lua state.
const (
	DefaultMemoryLimit      = 10 * 1024 * 1024 // 10 MB (advisory, not enforced by gopher-lua)
	DefaultExecutionTimeout = 5 * time.Second  // Timeout for Lua execution (best-effort)
	DefaultInstructionLimit = 10_000_000       // Maximum instructions per execution
)

// State wraps gopher-lua with additional features for plugin execution.
//
// IMPORTANT: gopher-lua's LState is not goroutine-safe. All operations on a State
// must be called from a single goroutine, or external synchronization must be used.
// The mutex in this struct protects against concurrent access from Go code, but
// Lua code execution is inherently single-threaded.
//
// Memory limits are advisory only - gopher-lua does not provide a mechanism to
// enforce hard memory limits. The memoryLimit field is provided for documentation
// and potential future use.
type State struct {
	L *lua.LState

	mu sync.Mutex

	// Configuration
	memoryLimit      int64         // Advisory only, not enforced
	executionTimeout time.Duration // Best-effort timeout
	instructionLimit int64

	// Sandbox
	sandbox *Sandbox

	// Tracking
	closed bool
}

// StateOption configures a State.
type StateOption func(*State)

// WithMemoryLimit sets the memory limit for the Lua state.
// NOTE: This is advisory only - gopher-lua does not enforce memory limits.
func WithMemoryLimit(bytes int64) StateOption {
	return func(s *State) {
		s.memoryLimit = bytes
	}
}

// WithExecutionTimeout sets the execution timeout for Lua calls.
// NOTE: This is a best-effort timeout. Long-running Lua code that doesn't
// yield or call Go functions cannot be interrupted mid-execution.
func WithExecutionTimeout(d time.Duration) StateOption {
	return func(s *State) {
		s.executionTimeout = d
	}
}

// WithInstructionLimit sets the maximum instructions per execution.
func WithInstructionLimit(limit int64) StateOption {
	return func(s *State) {
		s.instructionLimit = limit
	}
}

// NewState creates a new sandboxed Lua state.
func NewState(opts ...StateOption) (*State, error) {
	state := &State{
		memoryLimit:      DefaultMemoryLimit,
		executionTimeout: DefaultExecutionTimeout,
		instructionLimit: DefaultInstructionLimit,
	}

	// Apply options
	for _, opt := range opts {
		opt(state)
	}

	// Create Lua state with limited libraries
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true, // We'll open selectively
	})

	state.L = L

	// Open safe base libraries
	openSafeLibraries(L)

	// Create and install sandbox
	state.sandbox = NewSandbox(L, state.instructionLimit)
	state.sandbox.Install()

	return state, nil
}

// openSafeLibraries opens only safe Lua standard libraries.
func openSafeLibraries(L *lua.LState) {
	// Open base library (print, type, pairs, ipairs, etc.)
	lua.OpenBase(L)

	// Open safe libraries
	lua.OpenTable(L)
	lua.OpenString(L)
	lua.OpenMath(L)

	// Note: These are intentionally NOT opened:
	// - io (file system access)
	// - os (system calls, execute)
	// - debug (can bypass sandbox)
	// - package (can load arbitrary modules)
}

// DoFile executes a Lua file.
// Execution is synchronous - the call blocks until completion or error.
func (s *State) DoFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStateClosed
	}

	s.sandbox.ResetInstructionCount()

	// Execute synchronously with panic recovery
	return s.doWithRecovery(func() error {
		return s.L.DoFile(path)
	})
}

// DoString executes a Lua string.
// Execution is synchronous - the call blocks until completion or error.
func (s *State) DoString(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStateClosed
	}

	s.sandbox.ResetInstructionCount()

	// Execute synchronously with panic recovery
	return s.doWithRecovery(func() error {
		return s.L.DoString(code)
	})
}

// doWithRecovery executes a function with panic recovery.
func (s *State) doWithRecovery(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lua panic: %v", r)
		}
	}()
	return fn()
}

// Call calls a global Lua function with the given arguments.
// Returns an empty slice (not nil) if the function returns no values.
func (s *State) Call(fn string, args ...lua.LValue) ([]lua.LValue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrStateClosed
	}

	s.sandbox.ResetInstructionCount()

	// Get the function
	fnVal := s.L.GetGlobal(fn)
	if fnVal == lua.LNil {
		return nil, fmt.Errorf("function %q not found", fn)
	}

	if fnVal.Type() != lua.LTFunction {
		return nil, fmt.Errorf("%q is not a function (got %s)", fn, fnVal.Type())
	}

	// Record stack top before pushing anything
	stackTop := s.L.GetTop()

	// Push function and arguments
	s.L.Push(fnVal)
	for _, arg := range args {
		s.L.Push(arg)
	}

	// Call with panic recovery
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("lua panic: %v", r)
			}
		}()
		callErr = s.L.PCall(len(args), lua.MultRet, nil)
	}()

	if callErr != nil {
		return nil, callErr
	}

	// Collect return values (only the new values added after the call)
	nRet := s.L.GetTop() - stackTop
	if nRet <= 0 {
		return []lua.LValue{}, nil
	}
	results := make([]lua.LValue, nRet)
	for i := 0; i < nRet; i++ {
		results[i] = s.L.Get(stackTop + i + 1)
	}
	s.L.Pop(nRet)

	return results, nil
}

// GetGlobal returns a global variable value.
func (s *State) GetGlobal(name string) lua.LValue {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return lua.LNil
	}

	return s.L.GetGlobal(name)
}

// SetGlobal sets a global variable.
func (s *State) SetGlobal(name string, value lua.LValue) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.L.SetGlobal(name, value)
}

// RegisterFunc registers a Go function as a global Lua function.
func (s *State) RegisterFunc(name string, fn lua.LGFunction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.L.SetGlobal(name, s.L.NewFunction(fn))
}

// RegisterModule registers a module with the given functions.
func (s *State) RegisterModule(name string, funcs map[string]lua.LGFunction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	mod := s.L.SetFuncs(s.L.NewTable(), funcs)
	s.L.SetGlobal(name, mod)
}

// LuaState returns the underlying gopher-lua state.
//
// WARNING: Direct access to LState bypasses all safety measures including
// the mutex lock and sandbox. Use with extreme caution. The caller is
// responsible for ensuring thread-safety and proper cleanup.
func (s *State) LuaState() *lua.LState {
	return s.L
}

// Sandbox returns the sandbox for capability management.
//
// NOTE: The sandbox is shared with the State. Modifications to sandbox
// capabilities affect future Lua executions.
func (s *State) Sandbox() *Sandbox {
	return s.sandbox
}

// IsClosed returns true if the state has been closed.
func (s *State) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Close releases all resources associated with the Lua state.
// After Close is called, all other methods will return ErrStateClosed.
func (s *State) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.L.Close()
	s.closed = true
	return nil
}

// Reset clears the Lua state for reuse.
// This removes all user-defined globals while preserving built-in libraries.
// This is more efficient than creating a new state but may not fully clean
// up all state (e.g., metatables, registry entries).
func (s *State) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStateClosed
	}

	// Clear all globals except built-in libraries
	globals := s.L.Get(lua.GlobalsIndex).(*lua.LTable)
	safeGlobals := map[string]bool{
		"_G": true, "_VERSION": true,
		"assert": true, "error": true, "getmetatable": true,
		"ipairs": true, "next": true, "pairs": true, "pcall": true,
		"print": true, "rawequal": true, "rawget": true, "rawlen": true,
		"rawset": true, "select": true, "setmetatable": true,
		"tonumber": true, "tostring": true, "type": true, "xpcall": true,
		"coroutine": true, "math": true, "string": true, "table": true,
	}

	var keysToRemove []lua.LValue
	globals.ForEach(func(k, _ lua.LValue) {
		if ks, ok := k.(lua.LString); ok {
			if !safeGlobals[string(ks)] {
				keysToRemove = append(keysToRemove, k)
			}
		}
	})

	for _, k := range keysToRemove {
		s.L.SetGlobal(k.String(), lua.LNil)
	}

	s.sandbox.ResetInstructionCount()
	return nil
}
