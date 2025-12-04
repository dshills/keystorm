package mode

import (
	"testing"

	"github.com/dshills/keystorm/internal/input/key"
)

// mockEditor implements EditorState for testing.
type mockEditor struct {
	line, col uint32
	lineCount uint32
	filePath  string
	fileType  string
	modified  bool
	selection bool
	curLine   string
}

func (m *mockEditor) CursorPosition() (uint32, uint32) { return m.line, m.col }
func (m *mockEditor) HasSelection() bool               { return m.selection }
func (m *mockEditor) CurrentLine() string              { return m.curLine }
func (m *mockEditor) LineCount() uint32                { return m.lineCount }
func (m *mockEditor) FilePath() string                 { return m.filePath }
func (m *mockEditor) FileType() string                 { return m.fileType }
func (m *mockEditor) IsModified() bool                 { return m.modified }

func TestCursorStyleString(t *testing.T) {
	tests := []struct {
		style CursorStyle
		want  string
	}{
		{CursorBlock, "block"},
		{CursorBar, "bar"},
		{CursorUnderline, "underline"},
		{CursorHidden, "hidden"},
		{CursorStyle(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.style.String(); got != tt.want {
			t.Errorf("CursorStyle(%d).String() = %q, want %q", tt.style, got, tt.want)
		}
	}
}

func TestSelectionModeString(t *testing.T) {
	tests := []struct {
		mode SelectionMode
		want string
	}{
		{SelectChar, "char"},
		{SelectLine, "line"},
		{SelectBlock, "block"},
		{SelectionMode(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("SelectionMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestContextWithEditor(t *testing.T) {
	ctx := NewContext()
	editor := &mockEditor{line: 10, col: 5}

	ctx2 := ctx.WithEditor(editor)

	if ctx2.Editor == nil {
		t.Error("WithEditor should set editor")
	}
	if ctx.Editor != nil {
		t.Error("WithEditor should not modify original")
	}
}

func TestContextWithCount(t *testing.T) {
	ctx := NewContext()
	ctx2 := ctx.WithCount(5)

	if ctx2.Count != 5 {
		t.Errorf("WithCount(5) = %d, want 5", ctx2.Count)
	}
	if ctx.Count != 0 {
		t.Error("WithCount should not modify original")
	}
}

func TestNormalMode(t *testing.T) {
	m := NewNormalMode()

	if m.Name() != ModeNormal {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeNormal)
	}
	if m.DisplayName() != "NORMAL" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "NORMAL")
	}
	if m.CursorStyle() != CursorBlock {
		t.Errorf("CursorStyle() = %v, want CursorBlock", m.CursorStyle())
	}

	// Test enter/exit
	ctx := NewContext()
	if err := m.Enter(ctx); err != nil {
		t.Errorf("Enter() error = %v", err)
	}
	if err := m.Exit(ctx); err != nil {
		t.Errorf("Exit() error = %v", err)
	}
}

func TestNormalModeCount(t *testing.T) {
	m := NewNormalMode()
	ctx := NewContext()
	_ = m.Enter(ctx)

	// Initial count is 1
	if m.Count() != 1 {
		t.Errorf("initial Count() = %d, want 1", m.Count())
	}

	// Build up count with digit keys
	event := key.NewRuneEvent('5', key.ModNone)
	result := m.HandleUnmapped(event, ctx)
	if !result.Consumed {
		t.Error("digit '5' should be consumed")
	}

	event = key.NewRuneEvent('3', key.ModNone)
	_ = m.HandleUnmapped(event, ctx)

	if m.Count() != 53 {
		t.Errorf("Count() after '53' = %d, want 53", m.Count())
	}

	// '0' after digits
	event = key.NewRuneEvent('0', key.ModNone)
	_ = m.HandleUnmapped(event, ctx)

	if m.Count() != 530 {
		t.Errorf("Count() after '530' = %d, want 530", m.Count())
	}

	// Clear count
	m.ClearCount()
	if m.Count() != 1 {
		t.Errorf("Count() after clear = %d, want 1", m.Count())
	}
}

func TestNormalModeOperator(t *testing.T) {
	m := NewNormalMode()

	m.SetPendingOperator("d")
	if m.PendingOperator() != "d" {
		t.Errorf("PendingOperator() = %q, want %q", m.PendingOperator(), "d")
	}

	m.ClearPendingOperator()
	if m.PendingOperator() != "" {
		t.Errorf("PendingOperator() after clear = %q, want empty", m.PendingOperator())
	}
}

func TestInsertMode(t *testing.T) {
	m := NewInsertMode()

	if m.Name() != ModeInsert {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeInsert)
	}
	if m.DisplayName() != "INSERT" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "INSERT")
	}
	if m.CursorStyle() != CursorBar {
		t.Errorf("CursorStyle() = %v, want CursorBar", m.CursorStyle())
	}
}

func TestInsertModeHandleUnmapped(t *testing.T) {
	m := NewInsertMode()
	ctx := NewContext()
	_ = m.Enter(ctx)

	// Printable characters are typed
	event := key.NewRuneEvent('a', key.ModNone)
	result := m.HandleUnmapped(event, ctx)

	if !result.Consumed {
		t.Error("printable char should be consumed")
	}
	if result.InsertText != "a" {
		t.Errorf("InsertText = %q, want %q", result.InsertText, "a")
	}
	if result.Action == nil || result.Action.Name != "editor.insertText" {
		t.Error("Action should be editor.insertText")
	}

	// Ctrl+key is not consumed
	event = key.NewRuneEvent('a', key.ModCtrl)
	result = m.HandleUnmapped(event, ctx)
	if result.Consumed {
		t.Error("Ctrl+a should not be consumed as text")
	}
}

func TestVisualMode(t *testing.T) {
	m := NewVisualMode()

	if m.Name() != ModeVisual {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeVisual)
	}
	if m.DisplayName() != "VISUAL" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "VISUAL")
	}
	if m.SelectionMode() != SelectChar {
		t.Errorf("SelectionMode() = %v, want SelectChar", m.SelectionMode())
	}

	// Test anchor
	ctx := NewContext()
	ctx.Editor = &mockEditor{line: 5, col: 10}
	_ = m.Enter(ctx)

	anchor := m.Anchor()
	if anchor.Line != 5 || anchor.Column != 10 {
		t.Errorf("Anchor() = %v, want {5, 10}", anchor)
	}
}

func TestVisualLineMode(t *testing.T) {
	m := NewVisualLineMode()

	if m.Name() != ModeVisualLine {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeVisualLine)
	}
	if m.DisplayName() != "VISUAL LINE" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "VISUAL LINE")
	}
	if m.SelectionMode() != SelectLine {
		t.Errorf("SelectionMode() = %v, want SelectLine", m.SelectionMode())
	}
}

func TestVisualBlockMode(t *testing.T) {
	m := NewVisualBlockMode()

	if m.Name() != ModeVisualBlock {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeVisualBlock)
	}
	if m.DisplayName() != "VISUAL BLOCK" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "VISUAL BLOCK")
	}
	if m.SelectionMode() != SelectBlock {
		t.Errorf("SelectionMode() = %v, want SelectBlock", m.SelectionMode())
	}
}

func TestCommandMode(t *testing.T) {
	m := NewCommandMode()

	if m.Name() != ModeCommand {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeCommand)
	}
	if m.DisplayName() != "COMMAND" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "COMMAND")
	}
	if m.CursorStyle() != CursorBar {
		t.Errorf("CursorStyle() = %v, want CursorBar", m.CursorStyle())
	}
	if m.Prompt() != ':' {
		t.Errorf("Prompt() = %q, want ':'", m.Prompt())
	}
}

func TestCommandModeBuffer(t *testing.T) {
	m := NewCommandMode()
	ctx := NewContext()
	_ = m.Enter(ctx)

	// Type characters
	m.HandleUnmapped(key.NewRuneEvent('w', key.ModNone), ctx)
	m.HandleUnmapped(key.NewRuneEvent('q', key.ModNone), ctx)

	if m.Buffer() != "wq" {
		t.Errorf("Buffer() = %q, want %q", m.Buffer(), "wq")
	}
	if m.CursorPos() != 2 {
		t.Errorf("CursorPos() = %d, want 2", m.CursorPos())
	}

	// Backspace
	m.Backspace()
	if m.Buffer() != "w" {
		t.Errorf("Buffer() after backspace = %q, want %q", m.Buffer(), "w")
	}

	// Clear
	m.Clear()
	if m.Buffer() != "" {
		t.Errorf("Buffer() after clear = %q, want empty", m.Buffer())
	}
}

func TestCommandModeHistory(t *testing.T) {
	m := NewCommandMode()

	m.AddToHistory("w")
	m.AddToHistory("q")
	m.AddToHistory("wq")

	if len(m.History()) != 3 {
		t.Errorf("History() length = %d, want 3", len(m.History()))
	}

	// Navigate history
	m.SetBuffer("new")
	m.HistoryPrev()
	if m.Buffer() != "wq" {
		t.Errorf("Buffer() after HistoryPrev = %q, want %q", m.Buffer(), "wq")
	}

	m.HistoryPrev()
	if m.Buffer() != "q" {
		t.Errorf("Buffer() after 2nd HistoryPrev = %q, want %q", m.Buffer(), "q")
	}

	m.HistoryNext()
	if m.Buffer() != "wq" {
		t.Errorf("Buffer() after HistoryNext = %q, want %q", m.Buffer(), "wq")
	}

	m.HistoryNext()
	if m.Buffer() != "new" {
		t.Errorf("Buffer() after returning = %q, want %q", m.Buffer(), "new")
	}
}

func TestCommandModeCursorMovement(t *testing.T) {
	m := NewCommandMode()
	m.SetBuffer("hello")

	if m.CursorPos() != 5 {
		t.Errorf("initial CursorPos() = %d, want 5", m.CursorPos())
	}

	m.MoveLeft()
	if m.CursorPos() != 4 {
		t.Errorf("CursorPos() after MoveLeft = %d, want 4", m.CursorPos())
	}

	m.MoveToStart()
	if m.CursorPos() != 0 {
		t.Errorf("CursorPos() after MoveToStart = %d, want 0", m.CursorPos())
	}

	m.MoveRight()
	if m.CursorPos() != 1 {
		t.Errorf("CursorPos() after MoveRight = %d, want 1", m.CursorPos())
	}

	m.MoveToEnd()
	if m.CursorPos() != 5 {
		t.Errorf("CursorPos() after MoveToEnd = %d, want 5", m.CursorPos())
	}
}

func TestOperatorPendingMode(t *testing.T) {
	m := NewOperatorPendingMode()

	if m.Name() != ModeOperatorPending {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeOperatorPending)
	}
	if m.DisplayName() != "OPERATOR" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "OPERATOR")
	}
	if m.CursorStyle() != CursorUnderline {
		t.Errorf("CursorStyle() = %v, want CursorUnderline", m.CursorStyle())
	}

	// Enter with context
	ctx := NewContext()
	ctx.Extra["operator"] = "d"
	ctx.Extra["count"] = 3

	_ = m.Enter(ctx)

	if m.Operator() != "d" {
		t.Errorf("Operator() = %q, want %q", m.Operator(), "d")
	}
	if m.Count() != 3 {
		t.Errorf("Count() = %d, want 3", m.Count())
	}
}

func TestReplaceMode(t *testing.T) {
	m := NewReplaceMode()

	if m.Name() != ModeReplace {
		t.Errorf("Name() = %q, want %q", m.Name(), ModeReplace)
	}
	if m.DisplayName() != "REPLACE" {
		t.Errorf("DisplayName() = %q, want %q", m.DisplayName(), "REPLACE")
	}
	if m.CursorStyle() != CursorUnderline {
		t.Errorf("CursorStyle() = %v, want CursorUnderline", m.CursorStyle())
	}

	ctx := NewContext()
	event := key.NewRuneEvent('x', key.ModNone)
	result := m.HandleUnmapped(event, ctx)

	if !result.Consumed {
		t.Error("replace char should be consumed")
	}
	if result.Action == nil || result.Action.Name != "editor.replaceChar" {
		t.Error("Action should be editor.replaceChar")
	}
}
