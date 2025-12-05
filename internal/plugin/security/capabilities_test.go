package security

import (
	"testing"
)

func TestCapabilityConstants(t *testing.T) {
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
		{CapabilityEditor, "editor"},
		{CapabilityBuffer, "editor.buffer"},
		{CapabilityCursor, "editor.cursor"},
		{CapabilityKeymap, "editor.keymap"},
		{CapabilityCommand, "editor.command"},
		{CapabilityUI, "editor.ui"},
		{CapabilityConfig, "editor.config"},
		{CapabilityEvent, "editor.event"},
		{CapabilityLSP, "editor.lsp"},
	}

	for _, tt := range tests {
		if string(tt.cap) != tt.expected {
			t.Errorf("Capability %q != %q", tt.cap, tt.expected)
		}
	}
}

func TestGetCapabilityInfo(t *testing.T) {
	info, ok := GetCapabilityInfo(CapabilityFileRead)
	if !ok {
		t.Fatal("GetCapabilityInfo(CapabilityFileRead) ok = false")
	}
	if info.Name != CapabilityFileRead {
		t.Errorf("info.Name = %q, want %q", info.Name, CapabilityFileRead)
	}
	if info.DisplayName == "" {
		t.Error("info.DisplayName is empty")
	}
	if info.Description == "" {
		t.Error("info.Description is empty")
	}

	_, ok = GetCapabilityInfo("nonexistent")
	if ok {
		t.Error("GetCapabilityInfo(nonexistent) should return ok = false")
	}
}

func TestIsValidCapability(t *testing.T) {
	if !IsValidCapability(CapabilityFileRead) {
		t.Error("IsValidCapability(CapabilityFileRead) = false")
	}
	if !IsValidCapability(CapabilityNetwork) {
		t.Error("IsValidCapability(CapabilityNetwork) = false")
	}
	if IsValidCapability("nonexistent") {
		t.Error("IsValidCapability(nonexistent) = true")
	}
}

func TestAllCapabilities(t *testing.T) {
	caps := AllCapabilities()
	if len(caps) == 0 {
		t.Error("AllCapabilities() returned empty")
	}

	// Check that known capabilities are included
	found := map[Capability]bool{}
	for _, cap := range caps {
		found[cap] = true
	}

	mustHave := []Capability{
		CapabilityFileRead,
		CapabilityFileWrite,
		CapabilityNetwork,
		CapabilityShell,
	}
	for _, cap := range mustHave {
		if !found[cap] {
			t.Errorf("AllCapabilities() missing %q", cap)
		}
	}
}

func TestHighRiskCapabilities(t *testing.T) {
	caps := HighRiskCapabilities()
	if len(caps) == 0 {
		t.Error("HighRiskCapabilities() returned empty")
	}

	// These should require user approval
	mustHave := []Capability{
		CapabilityFileWrite,
		CapabilityNetwork,
		CapabilityShell,
		CapabilityProcess,
		CapabilityUnsafe,
	}

	found := map[Capability]bool{}
	for _, cap := range caps {
		found[cap] = true
	}

	for _, cap := range mustHave {
		if !found[cap] {
			t.Errorf("HighRiskCapabilities() missing %q", cap)
		}
	}
}

func TestIsChildOf(t *testing.T) {
	tests := []struct {
		child    Capability
		parent   Capability
		expected bool
	}{
		{CapabilityBuffer, CapabilityEditor, true},
		{CapabilityCursor, CapabilityEditor, true},
		{CapabilityKeymap, CapabilityEditor, true},
		{CapabilityEditor, CapabilityBuffer, false},
		{CapabilityFileRead, CapabilityFileWrite, false},
		{CapabilityFileRead, CapabilityNetwork, false},
		{CapabilityEditor, CapabilityEditor, false}, // Same is not child
	}

	for _, tt := range tests {
		got := IsChildOf(tt.child, tt.parent)
		if got != tt.expected {
			t.Errorf("IsChildOf(%q, %q) = %v, want %v", tt.child, tt.parent, got, tt.expected)
		}
	}
}

func TestImpliesCapability(t *testing.T) {
	tests := []struct {
		granted  Capability
		required Capability
		expected bool
	}{
		// Same capability
		{CapabilityFileRead, CapabilityFileRead, true},
		{CapabilityNetwork, CapabilityNetwork, true},
		// Parent implies child
		{CapabilityEditor, CapabilityBuffer, true},
		{CapabilityEditor, CapabilityCursor, true},
		{CapabilityEditor, CapabilityKeymap, true},
		// Child doesn't imply parent
		{CapabilityBuffer, CapabilityEditor, false},
		// Unrelated
		{CapabilityFileRead, CapabilityNetwork, false},
		{CapabilityShell, CapabilityClipboard, false},
	}

	for _, tt := range tests {
		got := ImpliesCapability(tt.granted, tt.required)
		if got != tt.expected {
			t.Errorf("ImpliesCapability(%q, %q) = %v, want %v", tt.granted, tt.required, got, tt.expected)
		}
	}
}

func TestRiskLevelString(t *testing.T) {
	tests := []struct {
		level    RiskLevel
		expected string
	}{
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskCritical, "critical"},
		{RiskLevel(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("RiskLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestCapabilityError(t *testing.T) {
	err := NewCapabilityError(CapabilityFileRead, "read file", "not granted")
	if err == nil {
		t.Fatal("NewCapabilityError returned nil")
	}

	if err.Capability != CapabilityFileRead {
		t.Errorf("err.Capability = %q, want %q", err.Capability, CapabilityFileRead)
	}
	if err.Operation != "read file" {
		t.Errorf("err.Operation = %q, want %q", err.Operation, "read file")
	}
	if err.Message != "not granted" {
		t.Errorf("err.Message = %q, want %q", err.Message, "not granted")
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("err.Error() is empty")
	}

	// Error without operation
	err2 := NewCapabilityError(CapabilityNetwork, "", "blocked")
	errStr2 := err2.Error()
	if errStr2 == "" {
		t.Error("err2.Error() is empty")
	}
}

func TestCapabilityInfoRiskLevels(t *testing.T) {
	// Check that dangerous capabilities have high risk
	dangerousCaps := []Capability{
		CapabilityShell,
		CapabilityProcess,
		CapabilityUnsafe,
	}

	for _, cap := range dangerousCaps {
		info, ok := GetCapabilityInfo(cap)
		if !ok {
			t.Errorf("GetCapabilityInfo(%q) not found", cap)
			continue
		}
		if info.RiskLevel < RiskHigh {
			t.Errorf("Capability %q has risk level %v, expected >= RiskHigh", cap, info.RiskLevel)
		}
		if !info.RequiresUserApproval {
			t.Errorf("Capability %q should require user approval", cap)
		}
	}

	// Check that editor capabilities have low risk
	editorCaps := []Capability{
		CapabilityBuffer,
		CapabilityCursor,
		CapabilityKeymap,
		CapabilityCommand,
		CapabilityUI,
	}

	for _, cap := range editorCaps {
		info, ok := GetCapabilityInfo(cap)
		if !ok {
			t.Errorf("GetCapabilityInfo(%q) not found", cap)
			continue
		}
		if info.RiskLevel > RiskLow {
			t.Errorf("Capability %q has risk level %v, expected RiskLow", cap, info.RiskLevel)
		}
	}
}
