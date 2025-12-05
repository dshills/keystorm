package lua

import (
	"testing"

	glua "github.com/yuin/gopher-lua"
)

func TestNewSandbox(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 1000000)
	if sandbox == nil {
		t.Error("NewSandbox() returned nil")
	}
	if sandbox.L != L {
		t.Error("NewSandbox() has wrong LState")
	}
}

func TestSandboxInstall(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	glua.OpenBase(L)

	sandbox := NewSandbox(L, 1000000)
	sandbox.Install()

	// Verify dangerous functions are removed
	dangerousFuncs := []string{"dofile", "loadfile", "load", "loadstring"}
	for _, fn := range dangerousFuncs {
		v := L.GetGlobal(fn)
		if v != glua.LNil {
			t.Errorf("%s should be removed, got %T", fn, v)
		}
	}
}

func TestSandboxGrantCapability(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 1000000)

	// Initially no capabilities
	if sandbox.HasCapability(CapabilityFileRead) {
		t.Error("Should not have CapabilityFileRead initially")
	}

	// Grant capability
	sandbox.Grant(CapabilityFileRead)

	if !sandbox.HasCapability(CapabilityFileRead) {
		t.Error("Should have CapabilityFileRead after Grant")
	}
}

func TestSandboxRevokeCapability(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 1000000)
	sandbox.Grant(CapabilityFileRead)

	// Revoke
	sandbox.Revoke(CapabilityFileRead)

	if sandbox.HasCapability(CapabilityFileRead) {
		t.Error("Should not have CapabilityFileRead after Revoke")
	}
}

func TestSandboxCapabilities(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 1000000)
	sandbox.Grant(CapabilityFileRead)
	sandbox.Grant(CapabilityNetwork)

	caps := sandbox.Capabilities()
	if len(caps) != 2 {
		t.Errorf("Capabilities() returned %d items, want 2", len(caps))
	}

	// Check both capabilities are present
	hasFileRead := false
	hasNetwork := false
	for _, c := range caps {
		if c == CapabilityFileRead {
			hasFileRead = true
		}
		if c == CapabilityNetwork {
			hasNetwork = true
		}
	}

	if !hasFileRead {
		t.Error("Capabilities() missing CapabilityFileRead")
	}
	if !hasNetwork {
		t.Error("Capabilities() missing CapabilityNetwork")
	}
}

func TestSandboxCheckCapability(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 1000000)

	// Should fail without capability
	err := sandbox.CheckCapability(CapabilityFileRead)
	if err == nil {
		t.Error("CheckCapability should fail without capability")
	}

	capErr, ok := err.(*CapabilityError)
	if !ok {
		t.Errorf("CheckCapability returned %T, want *CapabilityError", err)
	}
	if capErr.Capability != CapabilityFileRead {
		t.Errorf("CapabilityError.Capability = %v, want %v", capErr.Capability, CapabilityFileRead)
	}

	// Should succeed with capability
	sandbox.Grant(CapabilityFileRead)
	err = sandbox.CheckCapability(CapabilityFileRead)
	if err != nil {
		t.Errorf("CheckCapability with capability error = %v", err)
	}
}

func TestCapabilityError(t *testing.T) {
	err := &CapabilityError{Capability: CapabilityShell}
	expected := "capability not granted: shell"
	if err.Error() != expected {
		t.Errorf("CapabilityError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestSandboxInstructionCount(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 1000000)

	// Initial count should be 0
	if sandbox.InstructionCount() != 0 {
		t.Errorf("Initial InstructionCount = %d, want 0", sandbox.InstructionCount())
	}

	// Increment
	sandbox.IncrementInstructions(100)
	if sandbox.InstructionCount() != 100 {
		t.Errorf("InstructionCount after increment = %d, want 100", sandbox.InstructionCount())
	}

	// Reset
	sandbox.ResetInstructionCount()
	if sandbox.InstructionCount() != 0 {
		t.Errorf("InstructionCount after reset = %d, want 0", sandbox.InstructionCount())
	}
}

func TestSandboxInstructionLimit(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 100) // Low limit

	// Should not exceed initially
	if sandbox.IncrementInstructions(50) {
		t.Error("IncrementInstructions(50) should not exceed limit 100")
	}

	// Should exceed
	if !sandbox.IncrementInstructions(60) {
		t.Error("IncrementInstructions(60) should exceed limit 100")
	}
}

func TestSandboxInstructionLimitDisabled(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	sandbox := NewSandbox(L, 0) // Disabled

	// Should never exceed when disabled
	if sandbox.IncrementInstructions(999999999) {
		t.Error("IncrementInstructions should not exceed when limit is 0")
	}
}

func TestSandboxGrantFileRead(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	glua.OpenBase(L)

	sandbox := NewSandbox(L, 1000000)
	sandbox.Install()
	sandbox.Grant(CapabilityFileRead)

	// io module should now be available
	io := L.GetGlobal("io")
	if io == glua.LNil {
		t.Error("io module should be available after granting CapabilityFileRead")
	}

	// io.open should exist
	ioTbl, ok := io.(*glua.LTable)
	if !ok {
		t.Fatalf("io is not a table, got %T", io)
	}

	openFn := ioTbl.RawGetString("open")
	if openFn == glua.LNil {
		t.Error("io.open should exist")
	}
}

func TestSandboxGrantShell(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	glua.OpenBase(L)

	sandbox := NewSandbox(L, 1000000)
	sandbox.Install()
	sandbox.Grant(CapabilityShell)

	// os module should now be available
	os := L.GetGlobal("os")
	if os == glua.LNil {
		t.Error("os module should be available after granting CapabilityShell")
	}

	// os.getenv should exist
	osTbl, ok := os.(*glua.LTable)
	if !ok {
		t.Fatalf("os is not a table, got %T", os)
	}

	getenvFn := osTbl.RawGetString("getenv")
	if getenvFn == glua.LNil {
		t.Error("os.getenv should exist")
	}
}

func TestSandboxSafeRequire(t *testing.T) {
	L := glua.NewState(glua.Options{SkipOpenLibs: true})
	defer L.Close()
	glua.OpenBase(L)
	glua.OpenPackage(L) // Need package for require
	glua.OpenString(L)
	glua.OpenTable(L)
	glua.OpenMath(L)

	sandbox := NewSandbox(L, 1000000)
	sandbox.Install()

	// Safe modules should work
	err := L.DoString(`local s = require("string")`)
	if err != nil {
		t.Errorf("require('string') failed: %v", err)
	}

	err = L.DoString(`local m = require("math")`)
	if err != nil {
		t.Errorf("require('math') failed: %v", err)
	}

	err = L.DoString(`local t = require("table")`)
	if err != nil {
		t.Errorf("require('table') failed: %v", err)
	}
}

func TestCapabilityConstants(t *testing.T) {
	// Verify capability string values
	tests := []struct {
		cap      Capability
		expected string
	}{
		{CapabilityFileRead, "filesystem.read"},
		{CapabilityFileWrite, "filesystem.write"},
		{CapabilityNetwork, "network"},
		{CapabilityShell, "shell"},
		{CapabilityClipboard, "clipboard"},
		{CapabilityProcess, "process.spawn"},
		{CapabilityUnsafe, "unsafe"},
	}

	for _, tt := range tests {
		if string(tt.cap) != tt.expected {
			t.Errorf("%v = %q, want %q", tt.cap, string(tt.cap), tt.expected)
		}
	}
}
