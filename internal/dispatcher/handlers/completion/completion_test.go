package completion

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

func (e *mockEngine) LineText(line uint32) string { return e.text }

func (e *mockEngine) Len() buffer.ByteOffset {
	return buffer.ByteOffset(len(e.text))
}

func (e *mockEngine) LineCount() uint32 { return 1 }

func (e *mockEngine) LineStartOffset(line uint32) buffer.ByteOffset {
	return 0
}

func (e *mockEngine) LineEndOffset(line uint32) buffer.ByteOffset {
	return e.Len()
}

func (e *mockEngine) LineLen(line uint32) uint32 { return uint32(len(e.text)) }

func (e *mockEngine) OffsetToPoint(offset buffer.ByteOffset) buffer.Point {
	return buffer.Point{Line: 0, Column: uint32(offset)}
}

func (e *mockEngine) PointToOffset(point buffer.Point) buffer.ByteOffset {
	return buffer.ByteOffset(point.Column)
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

// mockCompletionProvider implements CompletionProvider for testing.
type mockCompletionProvider struct {
	items []CompletionItem
	sig   string
}

func newMockProvider(items []CompletionItem) *mockCompletionProvider {
	return &mockCompletionProvider{items: items}
}

func (p *mockCompletionProvider) GetCompletions(ctx *execctx.ExecutionContext, offset buffer.ByteOffset) ([]CompletionItem, error) {
	return p.items, nil
}

func (p *mockCompletionProvider) GetWordCompletions(ctx *execctx.ExecutionContext, prefix string) ([]CompletionItem, error) {
	var filtered []CompletionItem
	for _, item := range p.items {
		if hasPrefix(item.Label, prefix) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (p *mockCompletionProvider) GetLineCompletions(ctx *execctx.ExecutionContext, prefix string) ([]CompletionItem, error) {
	return p.items, nil
}

func (p *mockCompletionProvider) GetPathCompletions(ctx *execctx.ExecutionContext, prefix string) ([]CompletionItem, error) {
	return p.items, nil
}

func (p *mockCompletionProvider) GetSignatureHelp(ctx *execctx.ExecutionContext, offset buffer.ByteOffset) (string, error) {
	return p.sig, nil
}

func TestHandler_Namespace(t *testing.T) {
	h := NewHandler()
	if h.Namespace() != "completion" {
		t.Errorf("expected namespace 'completion', got '%s'", h.Namespace())
	}
}

func TestHandler_CanHandle(t *testing.T) {
	h := NewHandler()

	validActions := []string{
		ActionTrigger,
		ActionAccept,
		ActionAcceptWord,
		ActionCancel,
		ActionNext,
		ActionPrev,
		ActionPageDown,
		ActionPageUp,
		ActionWordComplete,
		ActionLineComplete,
		ActionPathComplete,
		ActionOmniComplete,
		ActionSignatureHelp,
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

func TestHandler_TriggerNoProvider(t *testing.T) {
	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{Name: ActionTrigger}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for no provider, got %v", result.Status)
	}
}

func TestHandler_Trigger(t *testing.T) {
	items := []CompletionItem{
		{Label: "foo", Kind: KindFunction},
		{Label: "bar", Kind: KindVariable},
	}
	provider := newMockProvider(items)
	h := NewHandlerWithProvider(provider)

	engine := newMockEngine("f")
	cursors := newMockCursorManager(1) // After "f"

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{Name: ActionTrigger}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	completions, ok := result.GetData("completions")
	if !ok {
		t.Error("expected completions in result data")
	}

	if len(completions.([]CompletionItem)) != 2 {
		t.Errorf("expected 2 completions, got %d", len(completions.([]CompletionItem)))
	}
}

func TestHandler_TriggerNoCompletions(t *testing.T) {
	provider := newMockProvider(nil)
	h := NewHandlerWithProvider(provider)

	engine := newMockEngine("x")
	cursors := newMockCursorManager(1)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{Name: ActionTrigger}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for empty completions, got %v", result.Status)
	}
}

func TestHandler_Accept(t *testing.T) {
	items := []CompletionItem{
		{Label: "foobar", Kind: KindFunction},
	}
	provider := newMockProvider(items)
	h := NewHandlerWithProvider(provider)

	engine := newMockEngine("foo")
	cursors := newMockCursorManager(3) // After "foo"

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	// First trigger completion
	action := input.Action{Name: ActionTrigger}
	h.HandleAction(action, ctx)

	// Then accept
	acceptAction := input.Action{Name: ActionAccept}
	result := h.HandleAction(acceptAction, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Buffer should have the completion
	if engine.Text() != "foobar" {
		t.Errorf("expected 'foobar', got '%s'", engine.Text())
	}

	// Cursor should be at end
	if cursors.Primary().Head != 6 {
		t.Errorf("expected cursor at 6, got %d", cursors.Primary().Head)
	}
}

func TestHandler_AcceptNoState(t *testing.T) {
	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{Name: ActionAccept}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for no completion state, got %v", result.Status)
	}
}

func TestHandler_Cancel(t *testing.T) {
	items := []CompletionItem{
		{Label: "test", Kind: KindText},
	}
	provider := newMockProvider(items)
	h := NewHandlerWithProvider(provider)

	engine := newMockEngine("t")
	cursors := newMockCursorManager(1)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	// Trigger completion
	h.HandleAction(input.Action{Name: ActionTrigger}, ctx)

	// Cancel
	result := h.HandleAction(input.Action{Name: ActionCancel}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	cancelled, ok := result.GetData("completionCancelled")
	if !ok || !cancelled.(bool) {
		t.Error("expected completionCancelled in result data")
	}
}

func TestHandler_Navigate(t *testing.T) {
	items := []CompletionItem{
		{Label: "aaa", Kind: KindText},
		{Label: "bbb", Kind: KindText},
		{Label: "ccc", Kind: KindText},
	}
	provider := newMockProvider(items)
	h := NewHandlerWithProvider(provider)

	engine := newMockEngine("a")
	cursors := newMockCursorManager(1)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	// Trigger completion
	h.HandleAction(input.Action{Name: ActionTrigger}, ctx)

	// Navigate next
	result := h.HandleAction(input.Action{Name: ActionNext}, ctx)
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	selected, ok := result.GetData("selected")
	if !ok {
		t.Error("expected selected in result data")
	}
	if selected.(int) != 1 {
		t.Errorf("expected selected 1, got %d", selected.(int))
	}

	// Navigate prev
	result = h.HandleAction(input.Action{Name: ActionPrev}, ctx)
	selected, _ = result.GetData("selected")
	if selected.(int) != 0 {
		t.Errorf("expected selected 0, got %d", selected.(int))
	}
}

func TestHandler_NavigateWrap(t *testing.T) {
	items := []CompletionItem{
		{Label: "aaa", Kind: KindText},
		{Label: "bbb", Kind: KindText},
	}
	provider := newMockProvider(items)
	h := NewHandlerWithProvider(provider)

	engine := newMockEngine("a")
	cursors := newMockCursorManager(1)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	// Trigger completion
	h.HandleAction(input.Action{Name: ActionTrigger}, ctx)

	// Navigate prev from 0 should wrap to last
	result := h.HandleAction(input.Action{Name: ActionPrev}, ctx)
	selected, _ := result.GetData("selected")
	if selected.(int) != 1 {
		t.Errorf("expected selected 1 (wrapped), got %d", selected.(int))
	}
}

func TestHandler_WordComplete(t *testing.T) {
	h := NewHandler()

	engine := newMockEngine("hello world hello")
	cursors := newMockCursorManager(3) // After "hel"

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{Name: ActionWordComplete}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	completions, ok := result.GetData("completions")
	if !ok {
		t.Error("expected completions in result data")
	}

	// Should find "hello" twice but only return unique
	items := completions.([]CompletionItem)
	if len(items) != 1 {
		t.Errorf("expected 1 completion, got %d", len(items))
	}

	if items[0].Label != "hello" {
		t.Errorf("expected 'hello', got '%s'", items[0].Label)
	}
}

func TestHandler_SignatureHelp(t *testing.T) {
	provider := newMockProvider(nil)
	provider.sig = "func(a int, b string) error"
	h := NewHandlerWithProvider(provider)

	engine := newMockEngine("test(")
	cursors := newMockCursorManager(5)

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.Cursors = cursors

	action := input.Action{Name: ActionSignatureHelp}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	sig, ok := result.GetData("signatureHelp")
	if !ok {
		t.Error("expected signatureHelp in result data")
	}

	if sig.(string) != "func(a int, b string) error" {
		t.Errorf("expected signature, got '%s'", sig.(string))
	}
}

func TestFindWordsWithPrefix(t *testing.T) {
	text := "foo bar foobar baz foo"

	words := findWordsWithPrefix(text, "foo")
	if len(words) != 1 {
		t.Errorf("expected 1 word (foobar), got %d: %v", len(words), words)
	}

	if len(words) > 0 && words[0] != "foobar" {
		t.Errorf("expected 'foobar', got '%s'", words[0])
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		s, prefix string
		want      bool
	}{
		{"foobar", "foo", true},
		{"foobar", "FOO", true}, // case insensitive
		{"FooBar", "foo", true},
		{"bar", "foo", false},
		{"fo", "foo", false},
	}

	for _, tt := range tests {
		got := hasPrefix(tt.s, tt.prefix)
		if got != tt.want {
			t.Errorf("hasPrefix(%q, %q) = %v, want %v", tt.s, tt.prefix, got, tt.want)
		}
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
	}

	for _, tt := range tests {
		got := isWordChar(tt.r)
		if got != tt.want {
			t.Errorf("isWordChar(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{-1, "-1"},
	}

	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %s, want %s", tt.n, got, tt.want)
		}
	}
}
