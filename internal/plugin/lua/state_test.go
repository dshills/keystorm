package lua

import (
	"testing"
	"time"

	glua "github.com/yuin/gopher-lua"
)

func TestNewState(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	if state.IsClosed() {
		t.Error("NewState() returned closed state")
	}

	if state.LuaState() == nil {
		t.Error("NewState() LuaState() is nil")
	}
}

func TestStateWithOptions(t *testing.T) {
	state, err := NewState(
		WithMemoryLimit(5*1024*1024),
		WithExecutionTimeout(2*time.Second),
		WithInstructionLimit(500000),
	)
	if err != nil {
		t.Fatalf("NewState() with options error = %v", err)
	}
	defer state.Close()

	if state.IsClosed() {
		t.Error("NewState() with options returned closed state")
	}
}

func TestStateDoString(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Test simple Lua code
	err = state.DoString(`x = 1 + 1`)
	if err != nil {
		t.Errorf("DoString() error = %v", err)
	}

	// Verify the result
	v := state.GetGlobal("x")
	if v == glua.LNil {
		t.Error("GetGlobal(x) returned LNil")
	}

	// Check the value
	if num, ok := v.(glua.LNumber); ok {
		if float64(num) != 2 {
			t.Errorf("x = %v, want 2", num)
		}
	} else {
		t.Errorf("x is not a number, got %T", v)
	}
}

func TestStateDoStringSyntaxError(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Test invalid Lua code
	err = state.DoString(`invalid lua code !!!`)
	if err == nil {
		t.Error("DoString() with invalid code should return error")
	}
}

func TestStateCall(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Define a function
	err = state.DoString(`
		function add(a, b)
			return a + b
		end
	`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}

	// Call the function
	results, err := state.Call("add", glua.LNumber(2), glua.LNumber(3))
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Call() returned %d results, want 1", len(results))
	}

	if num, ok := results[0].(glua.LNumber); ok {
		if float64(num) != 5 {
			t.Errorf("add(2, 3) = %v, want 5", num)
		}
	} else {
		t.Errorf("result is not a number, got %T", results[0])
	}
}

func TestStateCallMultipleReturns(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Define a function that returns multiple values
	err = state.DoString(`
		function multi()
			return 1, "hello", true
		end
	`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}

	// Call the function
	results, err := state.Call("multi")
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Call() returned %d results, want 3", len(results))
	}
}

func TestStateCallUndefinedFunction(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Call undefined function
	_, err = state.Call("undefined_function")
	if err == nil {
		t.Error("Call() on undefined function should return error")
	}
}

func TestStateRegisterFunc(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Register a Go function
	state.RegisterFunc("double", func(L *glua.LState) int {
		n := L.CheckNumber(1)
		L.Push(glua.LNumber(float64(n) * 2))
		return 1
	})

	// Call the registered function from Lua
	err = state.DoString(`result = double(21)`)
	if err != nil {
		t.Errorf("DoString() error = %v", err)
	}

	v := state.GetGlobal("result")
	if num, ok := v.(glua.LNumber); ok {
		if float64(num) != 42 {
			t.Errorf("double(21) = %v, want 42", num)
		}
	}
}

func TestStateRegisterModule(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Register a module
	state.RegisterModule("testmod", map[string]glua.LGFunction{
		"hello": func(L *glua.LState) int {
			L.Push(glua.LString("world"))
			return 1
		},
	})

	// Use the module from Lua
	err = state.DoString(`result = testmod.hello()`)
	if err != nil {
		t.Errorf("DoString() error = %v", err)
	}

	v := state.GetGlobal("result")
	if str, ok := v.(glua.LString); ok {
		if string(str) != "world" {
			t.Errorf("testmod.hello() = %v, want 'world'", str)
		}
	}
}

func TestStateClose(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	state.Close()

	if !state.IsClosed() {
		t.Error("Close() did not close state")
	}

	// Double close should not panic
	state.Close()
}

func TestStateClosedOperations(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	state.Close()

	// Operations on closed state should return errors
	err = state.DoString(`x = 1`)
	if err != ErrStateClosed {
		t.Errorf("DoString() on closed state error = %v, want ErrStateClosed", err)
	}

	_, err = state.Call("test")
	if err != ErrStateClosed {
		t.Errorf("Call() on closed state error = %v, want ErrStateClosed", err)
	}
}

func TestStateSandbox(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	sandbox := state.Sandbox()
	if sandbox == nil {
		t.Error("Sandbox() returned nil")
	}
}

func TestStateSetGetGlobal(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Set a global
	state.SetGlobal("testvar", glua.LString("hello"))

	// Get it back
	v := state.GetGlobal("testvar")
	if v == glua.LNil {
		t.Error("GetGlobal() returned LNil")
	}

	if str, ok := v.(glua.LString); ok {
		if string(str) != "hello" {
			t.Errorf("testvar = %v, want 'hello'", str)
		}
	}
}

func TestStateReset(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// Set some globals
	err = state.DoString(`foo = 42; bar = "hello"`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}

	// Verify they exist
	if state.GetGlobal("foo") == glua.LNil {
		t.Error("foo should exist before reset")
	}

	// Reset
	err = state.Reset()
	if err != nil {
		t.Errorf("Reset() error = %v", err)
	}

	// Verify custom globals are gone
	if state.GetGlobal("foo") != glua.LNil {
		t.Error("foo should be nil after reset")
	}
	if state.GetGlobal("bar") != glua.LNil {
		t.Error("bar should be nil after reset")
	}

	// Verify built-ins still work
	if state.GetGlobal("print") == glua.LNil {
		t.Error("print should still exist after reset")
	}
}

func TestStateDangerousFunctionsRemoved(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	defer state.Close()

	// These functions should be removed by sandbox
	dangerousFuncs := []string{"dofile", "loadfile", "load", "loadstring"}

	for _, fn := range dangerousFuncs {
		v := state.GetGlobal(fn)
		if v != glua.LNil {
			t.Errorf("%s should be removed by sandbox, got %T", fn, v)
		}
	}
}
