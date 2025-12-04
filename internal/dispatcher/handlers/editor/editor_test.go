package editor_test

import (
	"testing"

	editorhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/editor"
	"github.com/dshills/keystorm/internal/input"
)

// TestInsertHandlerNamespace verifies the InsertHandler returns correct namespace.
func TestInsertHandlerNamespace(t *testing.T) {
	h := editorhandler.NewInsertHandler()
	if h.Namespace() != "editor" {
		t.Errorf("expected namespace 'editor', got %q", h.Namespace())
	}
}

// TestInsertHandlerCanHandle verifies InsertHandler can handle insert actions.
func TestInsertHandlerCanHandle(t *testing.T) {
	h := editorhandler.NewInsertHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{editorhandler.ActionInsertChar, true},
		{editorhandler.ActionInsertText, true},
		{editorhandler.ActionInsertNewline, true},
		{editorhandler.ActionInsertLineAbove, true},
		{editorhandler.ActionInsertLineBelow, true},
		{editorhandler.ActionInsertTab, true},
		{"editor.unknown", false},
		{"cursor.moveLeft", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

// TestDeleteHandlerNamespace verifies the DeleteHandler returns correct namespace.
func TestDeleteHandlerNamespace(t *testing.T) {
	h := editorhandler.NewDeleteHandler()
	if h.Namespace() != "editor" {
		t.Errorf("expected namespace 'editor', got %q", h.Namespace())
	}
}

// TestDeleteHandlerCanHandle verifies DeleteHandler can handle delete actions.
func TestDeleteHandlerCanHandle(t *testing.T) {
	h := editorhandler.NewDeleteHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{editorhandler.ActionDeleteChar, true},
		{editorhandler.ActionDeleteCharBack, true},
		{editorhandler.ActionDeleteLine, true},
		{editorhandler.ActionDeleteToEnd, true},
		{editorhandler.ActionDeleteSelection, true},
		{editorhandler.ActionDeleteWord, true},
		{editorhandler.ActionDeleteWordBack, true},
		{"editor.unknown", false},
		{"cursor.moveLeft", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

// TestYankHandlerNamespace verifies the YankHandler returns correct namespace.
func TestYankHandlerNamespace(t *testing.T) {
	h := editorhandler.NewYankHandler()
	if h.Namespace() != "editor" {
		t.Errorf("expected namespace 'editor', got %q", h.Namespace())
	}
}

// TestYankHandlerCanHandle verifies YankHandler can handle yank/paste actions.
func TestYankHandlerCanHandle(t *testing.T) {
	h := editorhandler.NewYankHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{editorhandler.ActionYankSelection, true},
		{editorhandler.ActionYankLine, true},
		{editorhandler.ActionYankToEnd, true},
		{editorhandler.ActionYankWord, true},
		{editorhandler.ActionPasteAfter, true},
		{editorhandler.ActionPasteBefore, true},
		{"editor.unknown", false},
		{"cursor.moveLeft", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

// TestIndentHandlerNamespace verifies the IndentHandler returns correct namespace.
func TestIndentHandlerNamespace(t *testing.T) {
	h := editorhandler.NewIndentHandler()
	if h.Namespace() != "editor" {
		t.Errorf("expected namespace 'editor', got %q", h.Namespace())
	}
}

// TestIndentHandlerCanHandle verifies IndentHandler can handle indent actions.
func TestIndentHandlerCanHandle(t *testing.T) {
	h := editorhandler.NewIndentHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{editorhandler.ActionIndent, true},
		{editorhandler.ActionOutdent, true},
		{editorhandler.ActionAutoIndent, true},
		{editorhandler.ActionIndentBlock, true},
		{editorhandler.ActionOutdentBlock, true},
		{"editor.unknown", false},
		{"cursor.moveLeft", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

// TestIndentHandlerWithConfig verifies custom indent configuration.
func TestIndentHandlerWithConfig(t *testing.T) {
	h := editorhandler.NewIndentHandlerWithConfig(8, 2, true)
	if h.Namespace() != "editor" {
		t.Errorf("expected namespace 'editor', got %q", h.Namespace())
	}
	// Verify it can handle indent actions
	if !h.CanHandle(editorhandler.ActionIndent) {
		t.Error("expected custom configured handler to handle indent action")
	}
}

// TestInsertActionConstants verifies action names follow the editor.* pattern.
func TestInsertActionConstants(t *testing.T) {
	actions := []string{
		editorhandler.ActionInsertChar,
		editorhandler.ActionInsertText,
		editorhandler.ActionInsertNewline,
		editorhandler.ActionInsertLineAbove,
		editorhandler.ActionInsertLineBelow,
		editorhandler.ActionInsertTab,
	}

	for _, action := range actions {
		if len(action) < 8 || action[:7] != "editor." {
			t.Errorf("action %q does not follow editor.* pattern", action)
		}
	}
}

// TestDeleteActionConstants verifies action names follow the editor.* pattern.
func TestDeleteActionConstants(t *testing.T) {
	actions := []string{
		editorhandler.ActionDeleteChar,
		editorhandler.ActionDeleteCharBack,
		editorhandler.ActionDeleteLine,
		editorhandler.ActionDeleteToEnd,
		editorhandler.ActionDeleteSelection,
		editorhandler.ActionDeleteWord,
		editorhandler.ActionDeleteWordBack,
	}

	for _, action := range actions {
		if len(action) < 8 || action[:7] != "editor." {
			t.Errorf("action %q does not follow editor.* pattern", action)
		}
	}
}

// TestYankActionConstants verifies action names follow the editor.* pattern.
func TestYankActionConstants(t *testing.T) {
	actions := []string{
		editorhandler.ActionYankSelection,
		editorhandler.ActionYankLine,
		editorhandler.ActionYankToEnd,
		editorhandler.ActionYankWord,
		editorhandler.ActionPasteAfter,
		editorhandler.ActionPasteBefore,
	}

	for _, action := range actions {
		if len(action) < 8 || action[:7] != "editor." {
			t.Errorf("action %q does not follow editor.* pattern", action)
		}
	}
}

// TestIndentActionConstants verifies action names follow the editor.* pattern.
func TestIndentActionConstants(t *testing.T) {
	actions := []string{
		editorhandler.ActionIndent,
		editorhandler.ActionOutdent,
		editorhandler.ActionAutoIndent,
		editorhandler.ActionIndentBlock,
		editorhandler.ActionOutdentBlock,
	}

	for _, action := range actions {
		if len(action) < 8 || action[:7] != "editor." {
			t.Errorf("action %q does not follow editor.* pattern", action)
		}
	}
}

// TestActionForInput verifies we can create actions with the constants.
func TestActionForInput(t *testing.T) {
	action := input.Action{
		Name:  editorhandler.ActionInsertText,
		Count: 1,
	}

	if action.Name != "editor.insertText" {
		t.Errorf("expected action name 'editor.insertText', got %q", action.Name)
	}
	if action.Count != 1 {
		t.Errorf("expected count 1, got %d", action.Count)
	}
}

// TestDefaultIndentSettings verifies default indent handler settings.
func TestDefaultIndentSettings(t *testing.T) {
	if editorhandler.DefaultTabWidth != 4 {
		t.Errorf("expected DefaultTabWidth 4, got %d", editorhandler.DefaultTabWidth)
	}
	if editorhandler.DefaultUseTabs != false {
		t.Errorf("expected DefaultUseTabs false, got %v", editorhandler.DefaultUseTabs)
	}
	if editorhandler.DefaultIndentSize != 4 {
		t.Errorf("expected DefaultIndentSize 4, got %d", editorhandler.DefaultIndentSize)
	}
}
