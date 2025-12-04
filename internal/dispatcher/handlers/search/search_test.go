package search

import (
	"regexp"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// mockEngine implements execctx.EngineInterface for testing.
type mockEngine struct {
	text string
}

func newMockEngine(text string) *mockEngine {
	return &mockEngine{text: text}
}

func (e *mockEngine) Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error) {
	before := e.text[:offset]
	after := e.text[offset:]
	e.text = before + text + after
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Delete(start, end buffer.ByteOffset) (buffer.EditResult, error) {
	e.text = e.text[:start] + e.text[end:]
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error) {
	e.text = e.text[:start] + text + e.text[end:]
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Text() string { return e.text }

func (e *mockEngine) TextRange(start, end buffer.ByteOffset) string {
	if int(end) > len(e.text) {
		end = buffer.ByteOffset(len(e.text))
	}
	return e.text[start:end]
}

func (e *mockEngine) LineText(line uint32) string {
	lines := splitLines(e.text)
	if int(line) >= len(lines) {
		return ""
	}
	return lines[line]
}

func (e *mockEngine) Len() buffer.ByteOffset {
	return buffer.ByteOffset(len(e.text))
}

func (e *mockEngine) LineCount() uint32 {
	if e.text == "" {
		return 0
	}
	count := uint32(1)
	for _, c := range e.text {
		if c == '\n' {
			count++
		}
	}
	return count
}

func (e *mockEngine) LineStartOffset(line uint32) buffer.ByteOffset {
	offset := buffer.ByteOffset(0)
	currentLine := uint32(0)
	for i, c := range e.text {
		if currentLine == line {
			return buffer.ByteOffset(i)
		}
		if c == '\n' {
			currentLine++
			offset = buffer.ByteOffset(i + 1)
		}
	}
	if currentLine == line {
		return offset
	}
	return e.Len()
}

func (e *mockEngine) LineEndOffset(line uint32) buffer.ByteOffset {
	start := e.LineStartOffset(line)
	for i := int(start); i < len(e.text); i++ {
		if e.text[i] == '\n' {
			return buffer.ByteOffset(i)
		}
	}
	return e.Len()
}

func (e *mockEngine) LineLen(line uint32) uint32 {
	return uint32(e.LineEndOffset(line) - e.LineStartOffset(line))
}

func (e *mockEngine) OffsetToPoint(offset buffer.ByteOffset) buffer.Point {
	line := uint32(0)
	col := uint32(0)
	for i := 0; i < int(offset) && i < len(e.text); i++ {
		if e.text[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return buffer.Point{Line: line, Column: col}
}

func (e *mockEngine) PointToOffset(point buffer.Point) buffer.ByteOffset {
	return e.LineStartOffset(point.Line) + buffer.ByteOffset(point.Column)
}

func (e *mockEngine) Snapshot() execctx.EngineReader { return e }
func (e *mockEngine) RevisionID() buffer.RevisionID  { return 0 }

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	var lines []string
	start := 0
	for i, c := range text {
		if c == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start <= len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

// mockCursorManager implements execctx.CursorManagerInterface for testing.
type mockCursorManager struct {
	cursors []cursor.Selection
}

func newMockCursorManager(offset buffer.ByteOffset) *mockCursorManager {
	return &mockCursorManager{
		cursors: []cursor.Selection{cursor.NewCursorSelection(offset)},
	}
}

func (m *mockCursorManager) Primary() cursor.Selection {
	if len(m.cursors) == 0 {
		return cursor.NewCursorSelection(0)
	}
	return m.cursors[0]
}

func (m *mockCursorManager) SetPrimary(sel cursor.Selection) {
	if len(m.cursors) == 0 {
		m.cursors = []cursor.Selection{sel}
	} else {
		m.cursors[0] = sel
	}
}

func (m *mockCursorManager) All() []cursor.Selection  { return m.cursors }
func (m *mockCursorManager) Add(sel cursor.Selection) { m.cursors = append(m.cursors, sel) }
func (m *mockCursorManager) Clear()                   { m.cursors = m.cursors[:1] }
func (m *mockCursorManager) Count() int               { return len(m.cursors) }
func (m *mockCursorManager) IsMulti() bool            { return len(m.cursors) > 1 }
func (m *mockCursorManager) HasSelection() bool       { return m.cursors[0].Head != m.cursors[0].Anchor }
func (m *mockCursorManager) SetAll(sels []cursor.Selection) {
	m.cursors = make([]cursor.Selection, len(sels))
	copy(m.cursors, sels)
}
func (m *mockCursorManager) MapInPlace(f func(sel cursor.Selection) cursor.Selection) {
	for i, sel := range m.cursors {
		m.cursors[i] = f(sel)
	}
}
func (m *mockCursorManager) Clone() *cursor.CursorSet { return nil }
func (m *mockCursorManager) Clamp(maxOffset cursor.ByteOffset) {
	for i, sel := range m.cursors {
		if sel.Head > maxOffset {
			m.cursors[i] = sel.MoveTo(maxOffset)
		}
	}
}

func TestHandler_Namespace(t *testing.T) {
	h := NewHandler()
	if h.Namespace() != "search" {
		t.Errorf("expected namespace 'search', got '%s'", h.Namespace())
	}
}

func TestHandler_CanHandle(t *testing.T) {
	h := NewHandler()

	validActions := []string{
		ActionSearchForward,
		ActionSearchBackward,
		ActionSearchNext,
		ActionSearchPrev,
		ActionSearchWordForward,
		ActionSearchWordBackward,
		ActionReplace,
		ActionReplaceAll,
		ActionClearSearch,
	}

	for _, action := range validActions {
		if !h.CanHandle(action) {
			t.Errorf("expected CanHandle(%s) to return true", action)
		}
	}

	if h.CanHandle("invalid.action") {
		t.Error("expected CanHandle('invalid.action') to return false")
	}
}

func TestHandler_SearchForward(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello world hello")
	cursors := newMockCursorManager(0)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{
		Name: ActionSearchForward,
		Args: input.ActionArgs{SearchPattern: "world"},
	}

	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Cursor should move to "world" at position 6
	if cursors.Primary().Head != 6 {
		t.Errorf("expected cursor at 6, got %d", cursors.Primary().Head)
	}
}

func TestHandler_SearchBackward(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello world hello")
	cursors := newMockCursorManager(17) // End of string

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{
		Name: ActionSearchBackward,
		Args: input.ActionArgs{SearchPattern: "hello"},
	}

	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Cursor should move to second "hello" at position 12
	if cursors.Primary().Head != 12 {
		t.Errorf("expected cursor at 12, got %d", cursors.Primary().Head)
	}
}

func TestHandler_SearchNext(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("foo bar foo baz foo")
	cursors := newMockCursorManager(0)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	// First search
	action := input.Action{
		Name: ActionSearchForward,
		Args: input.ActionArgs{SearchPattern: "foo"},
	}
	h.HandleAction(action, ctx)

	// Search next - after first search found position 8, this finds position 16
	nextAction := input.Action{Name: ActionSearchNext}
	result := h.HandleAction(nextAction, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	// Should find third "foo" at position 16
	// (first search from pos 0 found pos 8, next search from pos 8 finds pos 16)
	if cursors.Primary().Head != 16 {
		t.Errorf("expected cursor at 16, got %d", cursors.Primary().Head)
	}
}

func TestHandler_SearchPrev(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("foo bar foo baz foo")
	cursors := newMockCursorManager(19) // End

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	// First search backward
	action := input.Action{
		Name: ActionSearchBackward,
		Args: input.ActionArgs{SearchPattern: "foo"},
	}
	h.HandleAction(action, ctx)

	// Search prev (which goes forward since original direction was backward)
	prevAction := input.Action{Name: ActionSearchPrev}
	result := h.HandleAction(prevAction, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
}

func TestHandler_SearchWordForward(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello world hello")
	cursors := newMockCursorManager(0) // At start of "hello"

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{Name: ActionSearchWordForward}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Should find second "hello" at position 12
	if cursors.Primary().Head != 12 {
		t.Errorf("expected cursor at 12, got %d", cursors.Primary().Head)
	}
}

func TestHandler_SearchWordBackward(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello world hello")
	cursors := newMockCursorManager(12) // At second "hello"

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{Name: ActionSearchWordBackward}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Should find first "hello" at position 0
	if cursors.Primary().Head != 0 {
		t.Errorf("expected cursor at 0, got %d", cursors.Primary().Head)
	}
}

func TestHandler_ClearSearch(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello world")
	cursors := newMockCursorManager(0)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	// Set up a search
	action := input.Action{
		Name: ActionSearchForward,
		Args: input.ActionArgs{SearchPattern: "world"},
	}
	h.HandleAction(action, ctx)

	// Clear search
	clearAction := input.Action{Name: ActionClearSearch}
	result := h.HandleAction(clearAction, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	// Search next should fail
	nextAction := input.Action{Name: ActionSearchNext}
	result = h.HandleAction(nextAction, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp after clear, got %v", result.Status)
	}
}

func TestHandler_SearchNotFound(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello world")
	cursors := newMockCursorManager(0)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{
		Name: ActionSearchForward,
		Args: input.ActionArgs{SearchPattern: "notfound"},
	}

	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp, got %v", result.Status)
	}
}

func TestHandler_SearchWrapAround(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("foo bar baz")
	cursors := newMockCursorManager(8) // After "bar"

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{
		Name: ActionSearchForward,
		Args: input.ActionArgs{SearchPattern: "foo"},
	}

	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	// Should wrap and find "foo" at position 0
	if cursors.Primary().Head != 0 {
		t.Errorf("expected cursor at 0 (wrapped), got %d", cursors.Primary().Head)
	}
}

func TestHandler_InvalidPattern(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello world")
	cursors := newMockCursorManager(0)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{
		Name: ActionSearchForward,
		Args: input.ActionArgs{SearchPattern: "[invalid"},
	}

	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for invalid regex, got %v", result.Status)
	}
}

func TestHandler_MissingEngine(t *testing.T) {
	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{
		Name: ActionSearchForward,
		Args: input.ActionArgs{SearchPattern: "test"},
	}

	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for missing engine, got %v", result.Status)
	}
}

func TestCompilePattern(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		forward       bool
		caseSensitive bool
		wantErr       bool
	}{
		{
			name:          "simple pattern",
			pattern:       "hello",
			forward:       true,
			caseSensitive: true,
			wantErr:       false,
		},
		{
			name:          "regex pattern",
			pattern:       "hel+o",
			forward:       true,
			caseSensitive: true,
			wantErr:       false,
		},
		{
			name:          "case insensitive",
			pattern:       "hello",
			forward:       true,
			caseSensitive: false,
			wantErr:       false,
		},
		{
			name:          "invalid regex",
			pattern:       "[invalid",
			forward:       true,
			caseSensitive: true,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := compilePattern(tt.pattern, tt.forward, tt.caseSensitive)
			if (err != nil) != tt.wantErr {
				t.Errorf("compilePattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if state.Pattern != tt.pattern {
					t.Errorf("expected pattern %s, got %s", tt.pattern, state.Pattern)
				}
				if state.Forward != tt.forward {
					t.Errorf("expected forward %v, got %v", tt.forward, state.Forward)
				}
				if state.CaseSensitive != tt.caseSensitive {
					t.Errorf("expected caseSensitive %v, got %v", tt.caseSensitive, state.CaseSensitive)
				}
			}
		})
	}
}

func TestIsWordChar(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'_', true},
		{' ', false},
		{'.', false},
		{'-', false},
		{'!', false},
	}

	for _, tt := range tests {
		if got := isWordChar(tt.r); got != tt.want {
			t.Errorf("isWordChar(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestSearchState_InContext(t *testing.T) {
	ctx := execctx.New()

	// Initially no search state
	state := getSearchState(ctx)
	if state != nil {
		t.Error("expected nil search state initially")
	}

	// Set search state
	re, _ := regexp.Compile("test")
	newState := &SearchState{
		Pattern:       "test",
		Regex:         re,
		Forward:       true,
		CaseSensitive: true,
	}
	ctx.SetData(searchStateKey, newState)

	// Retrieve search state
	state = getSearchState(ctx)
	if state == nil {
		t.Fatal("expected non-nil search state")
	}
	if state.Pattern != "test" {
		t.Errorf("expected pattern 'test', got '%s'", state.Pattern)
	}
}
