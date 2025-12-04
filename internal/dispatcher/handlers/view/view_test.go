package view

import (
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
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Delete(start, end buffer.ByteOffset) (buffer.EditResult, error) {
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error) {
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
	start := e.LineStartOffset(line)
	end := e.LineEndOffset(line)
	return e.text[start:end]
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
func (m *mockCursorManager) Clone() *cursor.CursorSet          { return nil }
func (m *mockCursorManager) Clamp(maxOffset cursor.ByteOffset) {}

// mockRenderer implements execctx.RendererInterface for testing.
type mockRenderer struct {
	startLine uint32
	endLine   uint32
}

func newMockRenderer(start, end uint32) *mockRenderer {
	return &mockRenderer{startLine: start, endLine: end}
}

func (r *mockRenderer) ScrollTo(line, col uint32) {
	height := r.endLine - r.startLine
	r.startLine = line
	r.endLine = line + height
}

func (r *mockRenderer) CenterOnLine(line uint32) {
	height := r.endLine - r.startLine
	halfHeight := height / 2
	if line >= halfHeight {
		r.startLine = line - halfHeight
	} else {
		r.startLine = 0
	}
	r.endLine = r.startLine + height
}

func (r *mockRenderer) Redraw() {}

func (r *mockRenderer) RedrawLines(lines []uint32) {}

func (r *mockRenderer) VisibleLineRange() (start, end uint32) {
	return r.startLine, r.endLine
}

func TestHandler_Namespace(t *testing.T) {
	h := NewHandler()
	if h.Namespace() != "view" {
		t.Errorf("expected namespace 'view', got '%s'", h.Namespace())
	}
}

func TestHandler_CanHandle(t *testing.T) {
	h := NewHandler()

	validActions := []string{
		ActionScrollDown,
		ActionScrollUp,
		ActionPageDown,
		ActionPageUp,
		ActionHalfPageDown,
		ActionHalfPageUp,
		ActionScrollToTop,
		ActionScrollToBottom,
		ActionMoveToTop,
		ActionMoveToMiddle,
		ActionMoveToBottom,
		ActionCenterCursor,
		ActionTopCursor,
		ActionBottomCursor,
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

// Create a multi-line buffer for testing
func createMultiLineBuffer() string {
	text := ""
	for i := 0; i < 100; i++ {
		text += "line " + string(rune('0'+i%10)) + "\n"
	}
	return text
}

func TestHandler_ScrollDown(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer
	ctx.Count = 5

	action := input.Action{Name: ActionScrollDown}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	start, _ := renderer.VisibleLineRange()
	if start != 5 {
		t.Errorf("expected view start at 5, got %d", start)
	}
}

func TestHandler_ScrollUp(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(10, 30)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer
	ctx.Count = 5

	action := input.Action{Name: ActionScrollUp}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	start, _ := renderer.VisibleLineRange()
	if start != 5 {
		t.Errorf("expected view start at 5, got %d", start)
	}
}

func TestHandler_PageDown(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionPageDown}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	start, _ := renderer.VisibleLineRange()
	if start != 20 {
		t.Errorf("expected view start at 20, got %d", start)
	}
}

func TestHandler_PageUp(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(40, 60)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionPageUp}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	start, _ := renderer.VisibleLineRange()
	if start != 20 {
		t.Errorf("expected view start at 20, got %d", start)
	}
}

func TestHandler_HalfPageDown(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionHalfPageDown}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	start, _ := renderer.VisibleLineRange()
	if start != 10 {
		t.Errorf("expected view start at 10, got %d", start)
	}
}

func TestHandler_ScrollToTop(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(100)
	renderer := newMockRenderer(50, 70)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionScrollToTop}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	start, _ := renderer.VisibleLineRange()
	if start != 0 {
		t.Errorf("expected view start at 0, got %d", start)
	}

	// Cursor should move to first line
	cursorLine := engine.OffsetToPoint(cursors.Primary().Head).Line
	if cursorLine != 0 {
		t.Errorf("expected cursor at line 0, got %d", cursorLine)
	}
}

func TestHandler_ScrollToBottom(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionScrollToBottom}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Cursor should be on last line
	lineCount := engine.LineCount()
	cursorLine := engine.OffsetToPoint(cursors.Primary().Head).Line
	if cursorLine != lineCount-1 {
		t.Errorf("expected cursor at line %d, got %d", lineCount-1, cursorLine)
	}
}

func TestHandler_MoveToTop(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(200) // Somewhere in the middle
	renderer := newMockRenderer(10, 30)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionMoveToTop}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Cursor should be at line 10 (top of visible area)
	cursorLine := engine.OffsetToPoint(cursors.Primary().Head).Line
	if cursorLine != 10 {
		t.Errorf("expected cursor at line 10, got %d", cursorLine)
	}
}

func TestHandler_MoveToMiddle(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(10, 30)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionMoveToMiddle}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Cursor should be at line 20 (middle of 10-30)
	cursorLine := engine.OffsetToPoint(cursors.Primary().Head).Line
	if cursorLine != 20 {
		t.Errorf("expected cursor at line 20, got %d", cursorLine)
	}
}

func TestHandler_MoveToBottom(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	cursors := newMockCursorManager(0)
	renderer := newMockRenderer(10, 30)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionMoveToBottom}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Cursor should be at line 29 (bottom of 10-30, end is exclusive)
	cursorLine := engine.OffsetToPoint(cursors.Primary().Head).Line
	if cursorLine != 29 {
		t.Errorf("expected cursor at line 29, got %d", cursorLine)
	}
}

func TestHandler_CenterCursor(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	// Set cursor at line 50
	offset := engine.LineStartOffset(50)
	cursors := newMockCursorManager(offset)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionCenterCursor}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// View should be centered on line 50
	start, end := renderer.VisibleLineRange()
	middle := start + (end-start)/2
	if middle != 50 {
		t.Errorf("expected view centered on line 50, got center at %d", middle)
	}
}

func TestHandler_TopCursor(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	offset := engine.LineStartOffset(50)
	cursors := newMockCursorManager(offset)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionTopCursor}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// View should start at cursor line 50
	start, _ := renderer.VisibleLineRange()
	if start != 50 {
		t.Errorf("expected view start at 50, got %d", start)
	}
}

func TestHandler_BottomCursor(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine(createMultiLineBuffer())
	offset := engine.LineStartOffset(50)
	cursors := newMockCursorManager(offset)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	action := input.Action{Name: ActionBottomCursor}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// View should end at cursor line 50
	_, end := renderer.VisibleLineRange()
	if end != 51 { // end is exclusive
		t.Errorf("expected view end at 51, got %d", end)
	}
}

func TestHandler_MissingEngine(t *testing.T) {
	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{Name: ActionScrollDown}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for missing engine, got %v", result.Status)
	}
}

func TestHandler_MissingRenderer(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("hello")
	ctx := execctx.New()
	ctx.Engine = engine

	action := input.Action{Name: ActionScrollDown}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for missing renderer, got %v", result.Status)
	}
}

func TestGetVisibleLineCount(t *testing.T) {
	ctx := execctx.New()

	// Without renderer, should return default
	count := GetVisibleLineCount(ctx)
	if count != 20 {
		t.Errorf("expected default 20, got %d", count)
	}

	// With renderer
	renderer := newMockRenderer(10, 35)
	ctx.Renderer = renderer

	count = GetVisibleLineCount(ctx)
	if count != 25 {
		t.Errorf("expected 25, got %d", count)
	}
}

func TestEnsureCursorVisible(t *testing.T) {
	engine := newMockEngine(createMultiLineBuffer())
	offset := engine.LineStartOffset(50)
	cursors := newMockCursorManager(offset)
	renderer := newMockRenderer(0, 20)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors
	ctx.Renderer = renderer

	result := EnsureCursorVisible(ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	// Cursor at line 50 should now be visible
	start, end := renderer.VisibleLineRange()
	cursorLine := engine.OffsetToPoint(cursors.Primary().Head).Line
	if cursorLine < start || cursorLine >= end {
		t.Errorf("cursor at line %d not visible in range [%d, %d)", cursorLine, start, end)
	}
}
