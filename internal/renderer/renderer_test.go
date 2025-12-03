package renderer

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/renderer/backend"
)

// mockBufferReader implements BufferReader for testing.
type mockBufferReader struct {
	lines    []string
	tabWidth int
}

func newMockBuffer(lines ...string) *mockBufferReader {
	return &mockBufferReader{
		lines:    lines,
		tabWidth: 4,
	}
}

func (m *mockBufferReader) LineText(line uint32) string {
	if int(line) >= len(m.lines) {
		return ""
	}
	return m.lines[line]
}

func (m *mockBufferReader) LineCount() uint32 {
	return uint32(len(m.lines))
}

func (m *mockBufferReader) TabWidth() int {
	return m.tabWidth
}

// mockCursorProvider implements CursorProvider for testing.
type mockCursorProvider struct {
	line, col  uint32
	selections []Selection
}

func (m *mockCursorProvider) PrimaryCursor() (line uint32, col uint32) {
	return m.line, m.col
}

func (m *mockCursorProvider) Selections() []Selection {
	return m.selections
}

// mockHighlightProvider implements HighlightProvider for testing.
type mockHighlightProvider struct {
	highlights map[uint32][]StyleSpan
}

func (m *mockHighlightProvider) HighlightsForLine(line uint32) []StyleSpan {
	if m.highlights == nil {
		return nil
	}
	return m.highlights[line]
}

func (m *mockHighlightProvider) InvalidateLines(startLine, endLine uint32) {
	// No-op for tests
}

// newTestBackend creates and initializes a NullBackend for testing.
func newTestBackend(width, height int) *backend.NullBackend {
	b := backend.NewNullBackend(width, height)
	_ = b.Init()
	return b
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if !opts.ShowLineNumbers {
		t.Error("ShowLineNumbers should be true by default")
	}
	if !opts.ShowGutter {
		t.Error("ShowGutter should be true by default")
	}
	if opts.WordWrap {
		t.Error("WordWrap should be false by default")
	}
	if opts.MaxFPS != 60 {
		t.Errorf("expected MaxFPS 60, got %d", opts.MaxFPS)
	}
	if opts.CursorStyle != backend.CursorBlock {
		t.Error("CursorStyle should be CursorBlock by default")
	}
}

func TestNewRenderer(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	opts := DefaultOptions()
	r := New(nullBackend, opts)

	if r == nil {
		t.Fatal("New returned nil")
	}

	width, height := r.Size()
	if width != 80 || height != 24 {
		t.Errorf("expected size (80, 24), got (%d, %d)", width, height)
	}

	if !r.NeedsRedraw() {
		t.Error("new renderer should need redraw")
	}

	if r.FrameCount() != 0 {
		t.Errorf("expected frame count 0, got %d", r.FrameCount())
	}
}

func TestRendererSetBuffer(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3")
	r.SetBuffer(buf)

	if !r.NeedsRedraw() {
		t.Error("setting buffer should mark redraw needed")
	}
}

func TestRendererSetCursorProvider(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	cursor := &mockCursorProvider{line: 5, col: 10}
	r.SetCursorProvider(cursor)

	if !r.NeedsRedraw() {
		t.Error("setting cursor provider should mark redraw needed")
	}
}

func TestRendererSetHighlightProvider(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	hl := &mockHighlightProvider{}
	r.SetHighlightProvider(hl)

	if !r.NeedsRedraw() {
		t.Error("setting highlight provider should mark redraw needed")
	}
}

func TestRendererResize(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	r.Resize(100, 40)

	width, height := r.Size()
	if width != 100 || height != 40 {
		t.Errorf("expected size (100, 40), got (%d, %d)", width, height)
	}

	if !r.NeedsRedraw() {
		t.Error("resize should mark redraw needed")
	}
}

func TestRendererMarkDirty(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	// Render to clear dirty flag
	r.RenderNow()

	if r.NeedsRedraw() {
		t.Error("render should clear dirty flag")
	}

	r.MarkDirty()

	if !r.NeedsRedraw() {
		t.Error("MarkDirty should set dirty flag")
	}
}

func TestRendererMarkFullRedraw(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	r.RenderNow()
	r.MarkFullRedraw()

	if !r.NeedsRedraw() {
		t.Error("MarkFullRedraw should set dirty flag")
	}
}

func TestRendererInvalidateLine(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3")
	r.SetBuffer(buf)
	r.RenderNow()

	r.InvalidateLine(1)

	if !r.NeedsRedraw() {
		t.Error("InvalidateLine should set dirty flag")
	}
}

func TestRendererInvalidateLines(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3", "Line 4", "Line 5")
	r.SetBuffer(buf)
	r.RenderNow()

	r.InvalidateLines(1, 3)

	if !r.NeedsRedraw() {
		t.Error("InvalidateLines should set dirty flag")
	}
}

func TestRendererViewport(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	vp := r.Viewport()
	if vp == nil {
		t.Error("Viewport should not be nil")
	}
}

func TestRendererOptions(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	opts := DefaultOptions()
	opts.ShowLineNumbers = false
	r := New(nullBackend, opts)

	gotOpts := r.Options()
	if gotOpts.ShowLineNumbers {
		t.Error("options should reflect constructor settings")
	}
}

func TestRendererSetOptions(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())
	r.RenderNow()

	newOpts := DefaultOptions()
	newOpts.ShowLineNumbers = false
	newOpts.MaxFPS = 30
	r.SetOptions(newOpts)

	if !r.NeedsRedraw() {
		t.Error("SetOptions should mark redraw needed")
	}

	gotOpts := r.Options()
	if gotOpts.ShowLineNumbers {
		t.Error("options should be updated")
	}
	if gotOpts.MaxFPS != 30 {
		t.Errorf("expected MaxFPS 30, got %d", gotOpts.MaxFPS)
	}
}

func TestRendererUpdate(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())
	r.RenderNow()

	// Update without animation
	needsRedraw := r.Update(0.016) // ~60fps
	if needsRedraw {
		t.Error("Update without changes should not need redraw")
	}
}

func TestRendererRenderNow(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	nullBackend.Init()
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("Hello, World!")
	r.SetBuffer(buf)

	r.RenderNow()

	if r.NeedsRedraw() {
		t.Error("RenderNow should clear dirty flag")
	}

	if r.FrameCount() != 1 {
		t.Errorf("expected frame count 1, got %d", r.FrameCount())
	}
}

func TestRendererRenderFrameRateLimiting(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	nullBackend.Init()

	opts := DefaultOptions()
	opts.MaxFPS = 60
	r := New(nullBackend, opts)

	buf := newMockBuffer("Hello, World!")
	r.SetBuffer(buf)

	// First render should succeed
	r.Render()
	firstCount := r.FrameCount()

	// Immediate second render should be rate-limited
	r.MarkDirty()
	r.Render()
	secondCount := r.FrameCount()

	if secondCount > firstCount {
		t.Error("Immediate second render should be rate-limited")
	}
}

func TestRendererRenderEmpty(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	nullBackend.Init()
	r := New(nullBackend, DefaultOptions())

	// Render without buffer
	r.RenderNow()

	if r.FrameCount() != 1 {
		t.Errorf("expected frame count 1, got %d", r.FrameCount())
	}
}

func TestRendererScrollToLine(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	// Create buffer with many lines
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	r.SetBuffer(buf)
	r.RenderNow()

	r.ScrollToLine(50, false)

	if !r.NeedsRedraw() {
		t.Error("ScrollToLine should mark redraw needed")
	}

	vp := r.Viewport()
	if !vp.IsLineVisible(50) {
		t.Error("line 50 should be visible after ScrollToLine")
	}
}

func TestRendererScrollToReveal(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	r.SetBuffer(buf)
	r.RenderNow()

	r.ScrollToReveal(50, 0, false)

	if !r.NeedsRedraw() {
		t.Error("ScrollToReveal should mark redraw needed")
	}
}

func TestRendererCenterOnLine(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	r.SetBuffer(buf)
	r.RenderNow()

	r.CenterOnLine(50, false)

	if !r.NeedsRedraw() {
		t.Error("CenterOnLine should mark redraw needed")
	}
}

func TestRendererGutterWidth(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	// Create buffer with many lines to test auto-width
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	r.SetBuffer(buf)
	r.RenderNow()

	gutterWidth := r.GutterWidth()
	// 1000 lines = 4 digits + 1 separator = 5
	if gutterWidth < 5 {
		t.Errorf("expected gutter width >= 5 for 1000 lines, got %d", gutterWidth)
	}
}

func TestRendererGutterWidthSmallBuffer(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("Line 1", "Line 2")
	r.SetBuffer(buf)
	r.RenderNow()

	gutterWidth := r.GutterWidth()
	// Minimum 3 digits + 1 separator = 4
	if gutterWidth < 4 {
		t.Errorf("expected gutter width >= 4, got %d", gutterWidth)
	}
}

func TestRendererNoGutter(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	opts := DefaultOptions()
	opts.ShowGutter = false
	r := New(nullBackend, opts)

	buf := newMockBuffer("Line 1", "Line 2")
	r.SetBuffer(buf)
	r.RenderNow()

	gutterWidth := r.GutterWidth()
	if gutterWidth != 0 {
		t.Errorf("expected gutter width 0 when disabled, got %d", gutterWidth)
	}
}

func TestFormatLineNumber(t *testing.T) {
	tests := []struct {
		num   uint32
		width int
		want  string
	}{
		{1, 3, "  1"},
		{42, 3, " 42"},
		{123, 3, "123"},
		{1234, 3, "1234"},
		{0, 3, "  ~"},
	}

	for _, tt := range tests {
		got := formatLineNumber(tt.num, tt.width)
		if got != tt.want {
			t.Errorf("formatLineNumber(%d, %d) = %q, want %q", tt.num, tt.width, got, tt.want)
		}
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		s     string
		width int
		want  string
	}{
		{"1", 3, "  1"},
		{"42", 3, " 42"},
		{"123", 3, "123"},
		{"1234", 3, "1234"},
		{"", 3, "   "},
	}

	for _, tt := range tests {
		got := padLeft(tt.s, tt.width)
		if got != tt.want {
			t.Errorf("padLeft(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
		}
	}
}

func TestUintToString(t *testing.T) {
	tests := []struct {
		n    uint32
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123, "123"},
		{4294967295, "4294967295"},
	}

	for _, tt := range tests {
		got := uintToString(tt.n)
		if got != tt.want {
			t.Errorf("uintToString(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestRendererWithHighlighting(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	nullBackend.Init()
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("func main() {}")
	r.SetBuffer(buf)

	hl := &mockHighlightProvider{
		highlights: map[uint32][]StyleSpan{
			0: {
				{StartCol: 0, EndCol: 4, Style: DefaultStyle().WithForeground(ColorBlue)},
				{StartCol: 5, EndCol: 9, Style: DefaultStyle().WithForeground(ColorGreen)},
			},
		},
	}
	r.SetHighlightProvider(hl)

	r.RenderNow()

	if r.FrameCount() != 1 {
		t.Errorf("expected frame count 1, got %d", r.FrameCount())
	}
}

func TestRendererCursorRendering(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	nullBackend.Init()
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("Hello, World!")
	r.SetBuffer(buf)

	cursor := &mockCursorProvider{line: 0, col: 5}
	r.SetCursorProvider(cursor)

	r.RenderNow()

	// Check cursor position
	cx, cy, visible := nullBackend.CursorPosition()
	if !visible {
		t.Error("cursor should be visible")
	}
	if cy != 0 {
		t.Errorf("expected cursor row 0, got %d", cy)
	}
	// Gutter width + col 5
	expectedX := r.GutterWidth() + 5
	if cx != expectedX {
		t.Errorf("expected cursor col %d, got %d", expectedX, cx)
	}
}

func TestRendererConcurrency(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	r := New(nullBackend, DefaultOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3")
	r.SetBuffer(buf)

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = r.NeedsRedraw()
			_ = r.FrameCount()
			_, _ = r.Size()
			_ = r.Options()
			_ = r.GutterWidth()
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			r.MarkDirty()
			r.InvalidateLine(0)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}

func TestRendererMinFrameTime(t *testing.T) {
	nullBackend := newTestBackend(80, 24)
	opts := DefaultOptions()
	opts.MaxFPS = 30 // 33.33ms per frame
	r := New(nullBackend, opts)

	buf := newMockBuffer("Hello")
	r.SetBuffer(buf)

	// First render
	r.RenderNow()
	r.MarkDirty()

	// Should not render immediately due to rate limiting
	r.Render()
	if r.FrameCount() > 1 {
		t.Error("Immediate render should be rate-limited")
	}

	// Wait for frame time
	time.Sleep(40 * time.Millisecond)
	r.Render()

	if r.FrameCount() != 2 {
		t.Errorf("expected frame count 2 after waiting, got %d", r.FrameCount())
	}
}
