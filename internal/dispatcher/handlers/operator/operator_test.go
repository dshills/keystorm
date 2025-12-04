package operator_test

import (
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/handlers/operator"
	"github.com/dshills/keystorm/internal/input"
)

// TestOperatorHandlerNamespace verifies the OperatorHandler returns correct namespace.
func TestOperatorHandlerNamespace(t *testing.T) {
	h := operator.NewOperatorHandler()
	if h.Namespace() != "operator" {
		t.Errorf("expected namespace 'operator', got %q", h.Namespace())
	}
}

// TestOperatorHandlerCanHandle verifies OperatorHandler can handle operator actions.
func TestOperatorHandlerCanHandle(t *testing.T) {
	h := operator.NewOperatorHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{operator.ActionDelete, true},
		{operator.ActionChange, true},
		{operator.ActionYank, true},
		{operator.ActionIndent, true},
		{operator.ActionOutdent, true},
		{operator.ActionLowercase, true},
		{operator.ActionUppercase, true},
		{operator.ActionToggleCase, true},
		{operator.ActionFormat, true},
		{"operator.unknown", false},
		{"cursor.moveLeft", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

// TestOperatorActionConstants verifies action names follow the operator.* pattern.
func TestOperatorActionConstants(t *testing.T) {
	actions := []string{
		operator.ActionDelete,
		operator.ActionChange,
		operator.ActionYank,
		operator.ActionIndent,
		operator.ActionOutdent,
		operator.ActionLowercase,
		operator.ActionUppercase,
		operator.ActionToggleCase,
		operator.ActionFormat,
	}

	for _, action := range actions {
		if len(action) < 10 || action[:9] != "operator." {
			t.Errorf("action %q does not follow operator.* pattern", action)
		}
	}
}

// TestActionForOperatorInput verifies we can create actions with the constants.
func TestActionForOperatorInput(t *testing.T) {
	action := input.Action{
		Name:  operator.ActionDelete,
		Count: 1,
		Args: input.ActionArgs{
			Motion: &input.Motion{
				Name:      "word",
				Direction: input.DirForward,
				Count:     1,
			},
		},
	}

	if action.Name != "operator.delete" {
		t.Errorf("expected action name 'operator.delete', got %q", action.Name)
	}

	if action.Args.Motion == nil {
		t.Error("expected motion to be set")
	}

	if action.Args.Motion.Name != "word" {
		t.Errorf("expected motion name 'word', got %q", action.Args.Motion.Name)
	}
}

// TestActionWithTextObject verifies actions can use text objects.
func TestActionWithTextObject(t *testing.T) {
	action := input.Action{
		Name: operator.ActionChange,
		Args: input.ActionArgs{
			TextObject: &input.TextObject{
				Name:      "word",
				Inner:     true,
				Delimiter: 0,
			},
		},
	}

	if action.Args.TextObject == nil {
		t.Error("expected text object to be set")
	}

	if action.Args.TextObject.Name != "word" {
		t.Errorf("expected text object name 'word', got %q", action.Args.TextObject.Name)
	}

	if !action.Args.TextObject.Inner {
		t.Error("expected text object to be inner")
	}
}

// TestActionWithRegister verifies actions can use registers.
func TestActionWithRegister(t *testing.T) {
	action := input.Action{
		Name: operator.ActionYank,
		Args: input.ActionArgs{
			Register: '"',
		},
	}

	if action.Args.Register != '"' {
		t.Errorf("expected register '\"', got %q", action.Args.Register)
	}
}
