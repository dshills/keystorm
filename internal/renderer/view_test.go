package renderer

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/backend"
	"github.com/dshills/keystorm/internal/renderer/viewport"
)

func TestDefaultViewOptions(t *testing.T) {
	opts := DefaultViewOptions()

	if !opts.ShowLineNumbers {
		t.Error("ShowLineNumbers should be true by default")
	}
	if !opts.ShowGutter {
		t.Error("ShowGutter should be true by default")
	}
	if opts.WordWrap {
		t.Error("WordWrap should be false by default")
	}
	if !opts.SmoothScroll {
		t.Error("SmoothScroll should be true by default")
	}
}

func TestNewView(t *testing.T) {
	v := NewView("test", 10, 5, 80, 24, DefaultViewOptions())

	if v == nil {
		t.Fatal("NewView returned nil")
	}

	if v.ID() != "test" {
		t.Errorf("expected ID 'test', got %q", v.ID())
	}

	x, y, width, height := v.Bounds()
	if x != 10 || y != 5 || width != 80 || height != 24 {
		t.Errorf("expected bounds (10, 5, 80, 24), got (%d, %d, %d, %d)", x, y, width, height)
	}

	if !v.NeedsRedraw() {
		t.Error("new view should need redraw")
	}
}

func TestViewSetBounds(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())
	v.SetBounds(20, 10, 100, 40)

	x, y, width, height := v.Bounds()
	if x != 20 || y != 10 || width != 100 || height != 40 {
		t.Errorf("expected bounds (20, 10, 100, 40), got (%d, %d, %d, %d)", x, y, width, height)
	}

	if !v.NeedsRedraw() {
		t.Error("SetBounds should mark redraw needed")
	}
}

func TestViewSetBuffer(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3")
	v.SetBuffer(buf)

	if !v.NeedsRedraw() {
		t.Error("SetBuffer should mark redraw needed")
	}
}

func TestViewSetCursorProvider(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	cursor := &mockCursorProvider{line: 5, col: 10}
	v.SetCursorProvider(cursor)

	if !v.NeedsRedraw() {
		t.Error("SetCursorProvider should mark redraw needed")
	}
}

func TestViewSetHighlightProvider(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	hl := &mockHighlightProvider{}
	v.SetHighlightProvider(hl)

	if !v.NeedsRedraw() {
		t.Error("SetHighlightProvider should mark redraw needed")
	}
}

func TestViewFocus(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	if v.IsFocused() {
		t.Error("new view should not be focused")
	}

	v.SetFocused(true)
	if !v.IsFocused() {
		t.Error("view should be focused after SetFocused(true)")
	}

	if !v.NeedsRedraw() {
		t.Error("SetFocused should mark redraw needed")
	}

	v.SetFocused(false)
	if v.IsFocused() {
		t.Error("view should not be focused after SetFocused(false)")
	}
}

func TestViewViewport(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	vp := v.Viewport()
	if vp == nil {
		t.Error("Viewport should not be nil")
	}
}

func TestViewOptions(t *testing.T) {
	opts := DefaultViewOptions()
	opts.ShowLineNumbers = false
	v := NewView("test", 0, 0, 80, 24, opts)

	gotOpts := v.Options()
	if gotOpts.ShowLineNumbers {
		t.Error("options should reflect constructor settings")
	}
}

func TestViewSetOptions(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	newOpts := DefaultViewOptions()
	newOpts.ShowLineNumbers = false
	newOpts.SmoothScroll = false
	v.SetOptions(newOpts)

	if !v.NeedsRedraw() {
		t.Error("SetOptions should mark redraw needed")
	}

	gotOpts := v.Options()
	if gotOpts.ShowLineNumbers {
		t.Error("options should be updated")
	}
	if gotOpts.SmoothScroll {
		t.Error("smooth scroll should be disabled")
	}
}

func TestViewMarkDirty(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	if v.NeedsRedraw() {
		t.Error("render should clear dirty flag")
	}

	v.MarkDirty()

	if !v.NeedsRedraw() {
		t.Error("MarkDirty should set dirty flag")
	}
}

func TestViewInvalidateLine(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3")
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	v.InvalidateLine(1)

	if !v.NeedsRedraw() {
		t.Error("InvalidateLine should set dirty flag")
	}
}

func TestViewInvalidateLines(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3", "Line 4", "Line 5")
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	v.InvalidateLines(1, 3)

	if !v.NeedsRedraw() {
		t.Error("InvalidateLines should set dirty flag")
	}
}

func TestViewUpdate(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	needsRedraw := v.Update(0.016)
	if needsRedraw {
		t.Error("Update without changes should not need redraw")
	}
}

func TestViewRender(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("Hello, World!")
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	if v.NeedsRedraw() {
		t.Error("render should clear dirty flag")
	}
}

func TestViewRenderEmpty(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()

	// Render without buffer
	v.Render(nullBackend)

	if v.NeedsRedraw() {
		t.Error("render should clear dirty flag")
	}
}

func TestViewRenderWithOffset(t *testing.T) {
	v := NewView("test", 10, 5, 60, 20, DefaultViewOptions())

	buf := newMockBuffer("Hello, World!")
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(100, 40)
	nullBackend.Init()
	v.Render(nullBackend)

	// View is at offset (10, 5), so content should be placed there
	// Gutter + "H" should be at x = 10 + gutterWidth
}

func TestViewScrollToLine(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	v.ScrollToLine(50, false)

	if !v.NeedsRedraw() {
		t.Error("ScrollToLine should mark redraw needed")
	}

	vp := v.Viewport()
	if !vp.IsLineVisible(50) {
		t.Error("line 50 should be visible after ScrollToLine")
	}
}

func TestViewScrollToReveal(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	v.ScrollToReveal(50, 0, false)

	if !v.NeedsRedraw() {
		t.Error("ScrollToReveal should mark redraw needed")
	}
}

func TestViewCenterOnLine(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	v.CenterOnLine(50, false)

	if !v.NeedsRedraw() {
		t.Error("CenterOnLine should mark redraw needed")
	}
}

func TestViewGutterWidth(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	gutterWidth := v.GutterWidth()
	// 1000 lines = 4 digits + 1 separator = 5
	if gutterWidth < 5 {
		t.Errorf("expected gutter width >= 5 for 1000 lines, got %d", gutterWidth)
	}
}

func TestViewNoGutter(t *testing.T) {
	opts := DefaultViewOptions()
	opts.ShowGutter = false
	v := NewView("test", 0, 0, 80, 24, opts)

	buf := newMockBuffer("Line 1", "Line 2")
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	gutterWidth := v.GutterWidth()
	if gutterWidth != 0 {
		t.Errorf("expected gutter width 0 when disabled, got %d", gutterWidth)
	}
}

func TestViewScreenToBuffer(t *testing.T) {
	v := NewView("test", 10, 5, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("Hello, World!")
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(100, 40)
	nullBackend.Init()
	v.Render(nullBackend)

	// Position outside view
	_, _, ok := v.ScreenToBuffer(0, 0)
	if ok {
		t.Error("position outside view should return false")
	}

	// Position in gutter
	_, _, ok = v.ScreenToBuffer(10, 5)
	if ok {
		t.Error("position in gutter should return false")
	}

	// Position in content area
	gutterWidth := v.GutterWidth()
	line, col, ok := v.ScreenToBuffer(10+gutterWidth, 5)
	if !ok {
		t.Error("valid position should return true")
	}
	if line != 0 {
		t.Errorf("expected line 0, got %d", line)
	}
	if col != 0 {
		t.Errorf("expected col 0, got %d", col)
	}
}

func TestViewBufferToScreen(t *testing.T) {
	v := NewView("test", 10, 5, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("Hello, World!")
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(100, 40)
	nullBackend.Init()
	v.Render(nullBackend)

	// Position on screen
	screenX, screenY, ok := v.BufferToScreen(0, 0)
	if !ok {
		t.Error("visible position should return true")
	}

	gutterWidth := v.GutterWidth()
	expectedX := 10 + gutterWidth
	expectedY := 5
	if screenX != expectedX {
		t.Errorf("expected screen X %d, got %d", expectedX, screenX)
	}
	if screenY != expectedY {
		t.Errorf("expected screen Y %d, got %d", expectedY, screenY)
	}

	// Position off screen
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line"
	}
	buf2 := newMockBuffer(lines...)
	v.SetBuffer(buf2)
	v.Render(nullBackend)

	_, _, ok = v.BufferToScreen(50, 0)
	if ok {
		t.Error("off-screen position should return false")
	}
}

func TestViewWithHighlighting(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("func main() {}")
	v.SetBuffer(buf)

	hl := &mockHighlightProvider{
		highlights: map[uint32][]StyleSpan{
			0: {
				{StartCol: 0, EndCol: 4, Style: DefaultStyle().WithForeground(ColorBlue)},
			},
		},
	}
	v.SetHighlightProvider(hl)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	if v.NeedsRedraw() {
		t.Error("render should clear dirty flag")
	}
}

func TestViewCursorRendering(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())
	v.SetFocused(true)

	buf := newMockBuffer("Hello, World!")
	v.SetBuffer(buf)

	cursor := &mockCursorProvider{line: 0, col: 5}
	v.SetCursorProvider(cursor)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	// Check cursor position
	cx, cy, visible := nullBackend.CursorPosition()
	if !visible {
		t.Error("cursor should be visible when view is focused")
	}
	if cy != 0 {
		t.Errorf("expected cursor row 0, got %d", cy)
	}

	expectedX := v.GutterWidth() + 5
	if cx != expectedX {
		t.Errorf("expected cursor col %d, got %d", expectedX, cx)
	}
}

func TestViewCursorNotShownWhenUnfocused(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())
	// View is not focused by default

	buf := newMockBuffer("Hello, World!")
	v.SetBuffer(buf)

	cursor := &mockCursorProvider{line: 0, col: 5}
	v.SetCursorProvider(cursor)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	// Cursor should not be shown when unfocused
	_, _, visible := nullBackend.CursorPosition()
	if visible {
		t.Error("cursor should not be visible when view is unfocused")
	}
}

func TestViewConcurrency(t *testing.T) {
	v := NewView("test", 0, 0, 80, 24, DefaultViewOptions())

	buf := newMockBuffer("Line 1", "Line 2", "Line 3")
	v.SetBuffer(buf)

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = v.NeedsRedraw()
			_ = v.IsFocused()
			_, _, _, _ = v.Bounds()
			_ = v.Options()
			_ = v.GutterWidth()
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			v.MarkDirty()
			v.InvalidateLine(0)
		}
		done <- true
	}()

	<-done
	<-done
}

func TestViewWithScrollMargins(t *testing.T) {
	opts := DefaultViewOptions()
	opts.ScrollMargins = viewport.MarginConfig{
		Top:    10,
		Bottom: 10,
		Left:   5,
		Right:  5,
	}
	v := NewView("test", 0, 0, 80, 24, opts)

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "Line content"
	}
	buf := newMockBuffer(lines...)
	v.SetBuffer(buf)

	nullBackend := backend.NewNullBackend(80, 24)
	nullBackend.Init()
	v.Render(nullBackend)

	// Scroll to line 50
	v.ScrollToLine(50, false)

	vp := v.Viewport()
	if !vp.IsLineVisible(50) {
		t.Error("line 50 should be visible")
	}
}
