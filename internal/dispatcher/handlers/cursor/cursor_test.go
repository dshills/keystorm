package cursor_test

import (
	"testing"

	cursorhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// MockEngine implements execctx.EngineInterface for testing.
type MockEngine struct {
	text     string
	lines    []string
	lineEnds []int64 // cumulative offsets for line ends
}

func NewMockEngine(text string) *MockEngine {
	e := &MockEngine{text: text}
	e.computeLines()
	return e
}

func (e *MockEngine) computeLines() {
	e.lines = nil
	e.lineEnds = nil
	start := 0
	offset := int64(0)

	for i, r := range e.text {
		if r == '\n' {
			e.lines = append(e.lines, e.text[start:i])
			e.lineEnds = append(e.lineEnds, int64(i+1))
			start = i + 1
		}
		offset = int64(i + 1)
	}

	// Last line (no trailing newline)
	if start <= len(e.text) {
		e.lines = append(e.lines, e.text[start:])
		e.lineEnds = append(e.lineEnds, offset)
	}
}

func (e *MockEngine) Text() string {
	return e.text
}

func (e *MockEngine) TextRange(start, end int64) string {
	if start < 0 {
		start = 0
	}
	if end > int64(len(e.text)) {
		end = int64(len(e.text))
	}
	return e.text[start:end]
}

func (e *MockEngine) LineText(line uint32) string {
	if int(line) >= len(e.lines) {
		return ""
	}
	return e.lines[line]
}

func (e *MockEngine) Len() int64 {
	return int64(len(e.text))
}

func (e *MockEngine) LineCount() uint32 {
	return uint32(len(e.lines))
}

func (e *MockEngine) LineStartOffset(line uint32) int64 {
	if line == 0 {
		return 0
	}
	if int(line) > len(e.lineEnds) {
		return e.Len()
	}
	return e.lineEnds[line-1]
}

func (e *MockEngine) LineEndOffset(line uint32) int64 {
	if int(line) >= len(e.lineEnds) {
		return e.Len()
	}
	// Return offset before newline
	endOffset := e.lineEnds[line]
	if endOffset > 0 && e.text[endOffset-1] == '\n' {
		return endOffset - 1
	}
	return endOffset
}

func (e *MockEngine) LineLen(line uint32) uint32 {
	if int(line) >= len(e.lines) {
		return 0
	}
	return uint32(len(e.lines[line]))
}

func (e *MockEngine) OffsetToPoint(offset int64) struct{ Line, Column uint32 } {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(e.text)) {
		offset = int64(len(e.text))
	}

	lineStart := int64(0)
	for line, end := range e.lineEnds {
		if offset < end {
			return struct{ Line, Column uint32 }{
				Line:   uint32(line),
				Column: uint32(offset - lineStart),
			}
		}
		lineStart = end
	}

	// At or past end
	lastLine := len(e.lines) - 1
	if lastLine < 0 {
		return struct{ Line, Column uint32 }{0, 0}
	}
	return struct{ Line, Column uint32 }{
		Line:   uint32(lastLine),
		Column: uint32(offset - e.LineStartOffset(uint32(lastLine))),
	}
}

func (e *MockEngine) PointToOffset(point struct{ Line, Column uint32 }) int64 {
	lineStart := e.LineStartOffset(point.Line)
	lineLen := e.LineLen(point.Line)

	col := point.Column
	if col > lineLen {
		col = lineLen
	}

	return lineStart + int64(col)
}

// Not used in tests but required by interface
func (e *MockEngine) Insert(offset int64, text string) (struct {
	OldRange, NewRange struct{ Start, End int64 }
	RevisionID         uint64
}, error) {
	return struct {
		OldRange, NewRange struct{ Start, End int64 }
		RevisionID         uint64
	}{}, nil
}

func (e *MockEngine) Delete(start, end int64) (struct {
	OldRange, NewRange struct{ Start, End int64 }
	RevisionID         uint64
}, error) {
	return struct {
		OldRange, NewRange struct{ Start, End int64 }
		RevisionID         uint64
	}{}, nil
}

func (e *MockEngine) Replace(start, end int64, text string) (struct {
	OldRange, NewRange struct{ Start, End int64 }
	RevisionID         uint64
}, error) {
	return struct {
		OldRange, NewRange struct{ Start, End int64 }
		RevisionID         uint64
	}{}, nil
}

func (e *MockEngine) Snapshot() interface{} { return nil }
func (e *MockEngine) RevisionID() uint64    { return 0 }

// MockCursors implements execctx.CursorManagerInterface for testing.
type MockCursors struct {
	selections []struct{ Anchor, Head int64 }
}

func NewMockCursors(offset int64) *MockCursors {
	return &MockCursors{
		selections: []struct{ Anchor, Head int64 }{{Anchor: offset, Head: offset}},
	}
}

func (c *MockCursors) Primary() struct{ Anchor, Head int64 } {
	if len(c.selections) == 0 {
		return struct{ Anchor, Head int64 }{}
	}
	return c.selections[0]
}

func (c *MockCursors) SetPrimary(sel struct{ Anchor, Head int64 }) {
	if len(c.selections) == 0 {
		c.selections = []struct{ Anchor, Head int64 }{sel}
	} else {
		c.selections[0] = sel
	}
}

func (c *MockCursors) All() []struct{ Anchor, Head int64 } {
	result := make([]struct{ Anchor, Head int64 }, len(c.selections))
	copy(result, c.selections)
	return result
}

func (c *MockCursors) Add(sel struct{ Anchor, Head int64 }) {
	c.selections = append(c.selections, sel)
}

func (c *MockCursors) Clear() {
	if len(c.selections) > 1 {
		c.selections = c.selections[:1]
	}
}

func (c *MockCursors) Count() int {
	return len(c.selections)
}

func (c *MockCursors) IsMulti() bool {
	return len(c.selections) > 1
}

func (c *MockCursors) HasSelection() bool {
	for _, sel := range c.selections {
		if sel.Anchor != sel.Head {
			return true
		}
	}
	return false
}

func (c *MockCursors) SetAll(sels []struct{ Anchor, Head int64 }) {
	c.selections = make([]struct{ Anchor, Head int64 }, len(sels))
	copy(c.selections, sels)
}

func (c *MockCursors) MapInPlace(f func(sel struct{ Anchor, Head int64 }) struct{ Anchor, Head int64 }) {
	for i, sel := range c.selections {
		c.selections[i] = f(sel)
	}
}

func (c *MockCursors) Clone() interface{} {
	clone := &MockCursors{
		selections: make([]struct{ Anchor, Head int64 }, len(c.selections)),
	}
	copy(clone.selections, c.selections)
	return clone
}

func (c *MockCursors) Clamp(maxOffset int64) {
	for i, sel := range c.selections {
		if sel.Anchor > maxOffset {
			c.selections[i].Anchor = maxOffset
		}
		if sel.Head > maxOffset {
			c.selections[i].Head = maxOffset
		}
	}
}

func TestHandlerNamespace(t *testing.T) {
	h := cursorhandler.NewHandler()
	if h.Namespace() != "cursor" {
		t.Errorf("expected namespace 'cursor', got %q", h.Namespace())
	}
}

func TestHandlerCanHandle(t *testing.T) {
	h := cursorhandler.NewHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{cursorhandler.ActionMoveLeft, true},
		{cursorhandler.ActionMoveRight, true},
		{cursorhandler.ActionMoveUp, true},
		{cursorhandler.ActionMoveDown, true},
		{cursorhandler.ActionMoveLineStart, true},
		{cursorhandler.ActionMoveLineEnd, true},
		{cursorhandler.ActionMoveFirstLine, true},
		{cursorhandler.ActionMoveLastLine, true},
		{"cursor.unknown", false},
		{"editor.save", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

func TestMotionHandlerNamespace(t *testing.T) {
	h := cursorhandler.NewMotionHandler()
	if h.Namespace() != "cursor" {
		t.Errorf("expected namespace 'cursor', got %q", h.Namespace())
	}
}

func TestMotionHandlerCanHandle(t *testing.T) {
	h := cursorhandler.NewMotionHandler()

	tests := []struct {
		action   string
		expected bool
	}{
		{cursorhandler.ActionWordForward, true},
		{cursorhandler.ActionWordBackward, true},
		{cursorhandler.ActionWordEndForward, true},
		{cursorhandler.ActionBigWordForward, true},
		{cursorhandler.ActionFirstNonBlank, true},
		{cursorhandler.ActionParagraphForward, true},
		{cursorhandler.ActionSentenceForward, true},
		{cursorhandler.ActionMatchingBracket, true},
		{"cursor.unknown", false},
	}

	for _, tc := range tests {
		if h.CanHandle(tc.action) != tc.expected {
			t.Errorf("CanHandle(%q) = %v, want %v", tc.action, h.CanHandle(tc.action), tc.expected)
		}
	}
}

// Integration tests with actual cursor movements would require
// proper mock setup that implements the interfaces exactly.
// For now we test the handler construction and capability checking.

func TestActionConstants(t *testing.T) {
	// Verify action names follow the cursor.* pattern
	actions := []string{
		cursorhandler.ActionMoveLeft,
		cursorhandler.ActionMoveRight,
		cursorhandler.ActionMoveUp,
		cursorhandler.ActionMoveDown,
		cursorhandler.ActionMoveLineStart,
		cursorhandler.ActionMoveLineEnd,
		cursorhandler.ActionMoveFirstLine,
		cursorhandler.ActionMoveLastLine,
		cursorhandler.ActionWordForward,
		cursorhandler.ActionWordBackward,
		cursorhandler.ActionWordEndForward,
		cursorhandler.ActionBigWordForward,
		cursorhandler.ActionBigWordBackward,
		cursorhandler.ActionBigWordEndForward,
		cursorhandler.ActionFirstNonBlank,
		cursorhandler.ActionGotoLine,
		cursorhandler.ActionGotoColumn,
		cursorhandler.ActionMatchingBracket,
		cursorhandler.ActionGotoPercent,
		cursorhandler.ActionParagraphForward,
		cursorhandler.ActionParagraphBackward,
		cursorhandler.ActionSentenceForward,
		cursorhandler.ActionSentenceBackward,
		cursorhandler.ActionScreenTop,
		cursorhandler.ActionScreenMiddle,
		cursorhandler.ActionScreenBottom,
	}

	for _, action := range actions {
		if len(action) < 8 || action[:7] != "cursor." {
			t.Errorf("action %q does not follow cursor.* pattern", action)
		}
	}
}

func TestActionForInput(t *testing.T) {
	// Test that we can create actions
	action := input.Action{
		Name:  cursorhandler.ActionMoveDown,
		Count: 5,
	}

	if action.Name != "cursor.moveDown" {
		t.Errorf("expected action name 'cursor.moveDown', got %q", action.Name)
	}
	if action.Count != 5 {
		t.Errorf("expected count 5, got %d", action.Count)
	}
}
