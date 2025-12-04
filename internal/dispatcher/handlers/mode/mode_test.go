package mode_test

import (
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/handlers/mode"
	"github.com/dshills/keystorm/internal/input"
)

// TestModeHandlerNamespace verifies the ModeHandler returns correct namespace.
func TestModeHandlerNamespace(t *testing.T) {
	h := mode.NewModeHandler()
	if h.Namespace() != "mode" {
		t.Errorf("expected namespace 'mode', got %q", h.Namespace())
	}
}

// TestModeHandlerCanHandle verifies ModeHandler can handle mode actions.
func TestModeHandlerCanHandle(t *testing.T) {
	h := mode.NewModeHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{mode.ActionNormal, true},
		{mode.ActionInsert, true},
		{mode.ActionInsertLineStart, true},
		{mode.ActionAppend, true},
		{mode.ActionAppendLineEnd, true},
		{mode.ActionOpenBelow, true},
		{mode.ActionOpenAbove, true},
		{mode.ActionVisual, true},
		{mode.ActionVisualLine, true},
		{mode.ActionVisualBlock, true},
		{mode.ActionCommand, true},
		{mode.ActionReplace, true},
		{mode.ActionReplaceChar, true},
		{"mode.unknown", false},
		{"cursor.moveLeft", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

// TestModeActionConstants verifies action names follow the mode.* pattern.
func TestModeActionConstants(t *testing.T) {
	actions := []string{
		mode.ActionNormal,
		mode.ActionInsert,
		mode.ActionInsertLineStart,
		mode.ActionAppend,
		mode.ActionAppendLineEnd,
		mode.ActionOpenBelow,
		mode.ActionOpenAbove,
		mode.ActionVisual,
		mode.ActionVisualLine,
		mode.ActionVisualBlock,
		mode.ActionCommand,
		mode.ActionReplace,
		mode.ActionReplaceChar,
	}

	for _, action := range actions {
		if len(action) < 6 || action[:5] != "mode." {
			t.Errorf("action %q does not follow mode.* pattern", action)
		}
	}
}

// TestActionForModeInput verifies we can create actions with the constants.
func TestActionForModeInput(t *testing.T) {
	action := input.Action{
		Name:  mode.ActionInsert,
		Count: 1,
	}

	if action.Name != "mode.insert" {
		t.Errorf("expected action name 'mode.insert', got %q", action.Name)
	}
}
