package api

import (
	"errors"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockModeProvider implements ModeProvider for testing.
type mockModeProvider struct {
	current     string
	validModes  map[string]bool
	switchError error
}

func newMockModeProvider() *mockModeProvider {
	return &mockModeProvider{
		current: "normal",
		validModes: map[string]bool{
			"normal":      true,
			"insert":      true,
			"visual":      true,
			"visual_line": true,
			"command":     true,
		},
	}
}

func (m *mockModeProvider) Current() string { return m.current }
func (m *mockModeProvider) Switch(mode string) error {
	if m.switchError != nil {
		return m.switchError
	}
	if !m.validModes[mode] {
		return errors.New("invalid mode")
	}
	m.current = mode
	return nil
}
func (m *mockModeProvider) Is(mode string) bool {
	return m.current == mode
}

func setupModeTest(t *testing.T, mode *mockModeProvider) (*lua.LState, *ModeModule) {
	t.Helper()

	ctx := &Context{Mode: mode}
	mod := NewModeModule(ctx)

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestModeModuleName(t *testing.T) {
	mod := NewModeModule(&Context{})
	if mod.Name() != "mode" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "mode")
	}
}

func TestModeModuleCapability(t *testing.T) {
	mod := NewModeModule(&Context{})
	// Mode module requires no special capability
	if mod.RequiredCapability() != security.Capability("") {
		t.Errorf("RequiredCapability() = %q, want empty", mod.RequiredCapability())
	}
}

func TestModeCurrent(t *testing.T) {
	mode := newMockModeProvider()
	mode.current = "visual"
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		result = _ks_mode.current()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "visual" {
		t.Errorf("current() = %q, want %q", result.String(), "visual")
	}
}

func TestModeSwitch(t *testing.T) {
	mode := newMockModeProvider()
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		_ks_mode.switch("insert")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if mode.current != "insert" {
		t.Errorf("mode after switch = %q, want %q", mode.current, "insert")
	}
}

func TestModeSwitchInvalid(t *testing.T) {
	mode := newMockModeProvider()
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		_ks_mode.switch("invalid_mode")
	`)
	if err == nil {
		t.Error("switch to invalid mode should error")
	}
}

func TestModeSwitchEmpty(t *testing.T) {
	mode := newMockModeProvider()
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		_ks_mode.switch("")
	`)
	if err == nil {
		t.Error("switch to empty mode should error")
	}
}

func TestModeIs(t *testing.T) {
	mode := newMockModeProvider()
	mode.current = "normal"
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		is_normal = _ks_mode.is("normal")
		is_insert = _ks_mode.is("insert")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	isNormal := L.GetGlobal("is_normal")
	if isNormal != lua.LTrue {
		t.Error("is('normal') should be true")
	}

	isInsert := L.GetGlobal("is_insert")
	if isInsert != lua.LFalse {
		t.Error("is('insert') should be false")
	}
}

func TestModeConstants(t *testing.T) {
	mode := newMockModeProvider()
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		assert(_ks_mode.NORMAL == "normal", "NORMAL constant")
		assert(_ks_mode.INSERT == "insert", "INSERT constant")
		assert(_ks_mode.VISUAL == "visual", "VISUAL constant")
		assert(_ks_mode.VISUAL_LINE == "visual_line", "VISUAL_LINE constant")
		assert(_ks_mode.VISUAL_BLOCK == "visual_block", "VISUAL_BLOCK constant")
		assert(_ks_mode.COMMAND == "command", "COMMAND constant")
		assert(_ks_mode.REPLACE == "replace", "REPLACE constant")
		assert(_ks_mode.OPERATOR_PENDING == "operator_pending", "OPERATOR_PENDING constant")
	`)
	if err != nil {
		t.Errorf("mode constants error = %v", err)
	}
}

func TestModeNilContext(t *testing.T) {
	ctx := &Context{Mode: nil}
	mod := NewModeModule(ctx)

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Should not panic, should return default values
	err := L.DoString(`
		assert(_ks_mode.current() == "normal", "default mode should be normal")
		assert(_ks_mode.is("normal") == true, "is('normal') should be true by default")
		assert(_ks_mode.is("insert") == false, "is('insert') should be false by default")
	`)
	if err != nil {
		t.Errorf("DoString with nil mode error = %v", err)
	}
}

func TestModeSwitchWithConstant(t *testing.T) {
	mode := newMockModeProvider()
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		_ks_mode.switch(_ks_mode.INSERT)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if mode.current != "insert" {
		t.Errorf("mode after switch = %q, want %q", mode.current, "insert")
	}
}

func TestModeWorkflow(t *testing.T) {
	// Test a typical mode switching workflow
	mode := newMockModeProvider()
	L, _ := setupModeTest(t, mode)

	err := L.DoString(`
		-- Start in normal mode
		assert(_ks_mode.is("normal"), "should start in normal mode")

		-- Switch to insert mode
		_ks_mode.switch("insert")
		assert(_ks_mode.current() == "insert", "should be in insert mode")
		assert(_ks_mode.is("insert"), "is should return true for insert")
		assert(not _ks_mode.is("normal"), "is should return false for normal")

		-- Switch to visual mode
		_ks_mode.switch("visual")
		assert(_ks_mode.current() == "visual", "should be in visual mode")

		-- Back to normal
		_ks_mode.switch("normal")
		assert(_ks_mode.is("normal"), "should be back in normal mode")
	`)
	if err != nil {
		t.Errorf("mode workflow error = %v", err)
	}
}
