package backend

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer"
)

func TestNewScreenBuffer(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	w, h := sb.Size()
	if w != 80 || h != 24 {
		t.Errorf("expected size (80, 24), got (%d, %d)", w, h)
	}
}

func TestScreenBufferSetGetCell(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	cell := renderer.NewStyledCell('A', renderer.DefaultStyle().WithForeground(renderer.ColorBlue))
	sb.SetCell(10, 5, cell)

	got := sb.GetCell(10, 5)
	if !got.Equals(cell) {
		t.Errorf("cell mismatch: expected %+v, got %+v", cell, got)
	}

	// Out of bounds
	sb.SetCell(-1, 0, cell) // Should not panic
	sb.SetCell(100, 0, cell)

	empty := sb.GetCell(-1, 0)
	if !empty.Equals(renderer.EmptyCell()) {
		t.Error("out of bounds should return empty cell")
	}
}

func TestScreenBufferFill(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	cell := renderer.NewCell('#')
	rect := renderer.NewScreenRect(5, 10, 15, 30)
	sb.Fill(rect, cell)

	// Inside rect
	if !sb.GetCell(20, 10).Equals(cell) {
		t.Error("cell inside rect should be filled")
	}

	// Outside rect
	if sb.GetCell(0, 0).Equals(cell) {
		t.Error("cell outside rect should not be filled")
	}
}

func TestScreenBufferClear(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	sb.SetCell(10, 10, renderer.NewCell('X'))
	sb.Clear()

	got := sb.GetCell(10, 10)
	if !got.Equals(renderer.EmptyCell()) {
		t.Error("clear should reset all cells")
	}
}

func TestScreenBufferClearRegion(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	// Fill everything
	sb.Fill(renderer.NewScreenRect(0, 0, 24, 80), renderer.NewCell('X'))

	// Clear a region
	sb.ClearRegion(renderer.NewScreenRect(5, 10, 15, 30))

	// Inside cleared region
	got := sb.GetCell(20, 10)
	if !got.Equals(renderer.EmptyCell()) {
		t.Error("cleared region should have empty cells")
	}

	// Outside cleared region
	got = sb.GetCell(0, 0)
	if got.Equals(renderer.EmptyCell()) {
		t.Error("outside cleared region should still have filled cells")
	}
}

func TestScreenBufferSetLine(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	cells := []renderer.Cell{
		renderer.NewCell('H'),
		renderer.NewCell('i'),
		renderer.NewCell('!'),
	}
	sb.SetLine(10, 5, cells)

	if sb.GetCell(10, 5).Rune != 'H' {
		t.Error("first cell should be 'H'")
	}
	if sb.GetCell(11, 5).Rune != 'i' {
		t.Error("second cell should be 'i'")
	}
	if sb.GetCell(12, 5).Rune != '!' {
		t.Error("third cell should be '!'")
	}
}

func TestScreenBufferSetString(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	style := renderer.DefaultStyle().WithForeground(renderer.ColorGreen)
	sb.SetString(5, 10, "Hello", style)

	got := sb.GetCell(5, 10)
	if got.Rune != 'H' {
		t.Errorf("expected 'H', got %q", got.Rune)
	}
	if !got.Style.Foreground.Equals(renderer.ColorGreen) {
		t.Error("style should be green")
	}
}

func TestScreenBufferSetStringWithWideChars(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	style := renderer.DefaultStyle()
	sb.SetString(0, 0, "A中B", style)

	// A at 0
	if sb.GetCell(0, 0).Rune != 'A' {
		t.Error("cell 0 should be 'A'")
	}
	// 中 at 1, continuation at 2
	if sb.GetCell(1, 0).Rune != '中' {
		t.Error("cell 1 should be '中'")
	}
	if !sb.GetCell(2, 0).IsContinuation() {
		t.Error("cell 2 should be continuation")
	}
	// B at 3
	if sb.GetCell(3, 0).Rune != 'B' {
		t.Error("cell 3 should be 'B'")
	}
}

func TestScreenBufferResize(t *testing.T) {
	sb := NewScreenBuffer(80, 24)
	sb.SetCell(10, 10, renderer.NewCell('X'))

	sb.Resize(100, 40)

	w, h := sb.Size()
	if w != 100 || h != 40 {
		t.Errorf("expected size (100, 40), got (%d, %d)", w, h)
	}

	// Preserved cell
	got := sb.GetCell(10, 10)
	if got.Rune != 'X' {
		t.Error("resize should preserve existing content")
	}
}

func TestScreenBufferResizeSmallerPreserves(t *testing.T) {
	sb := NewScreenBuffer(80, 24)
	sb.SetCell(10, 10, renderer.NewCell('X'))
	sb.SetCell(70, 20, renderer.NewCell('Y'))

	sb.Resize(50, 15)

	// Cell within new bounds preserved
	got := sb.GetCell(10, 10)
	if got.Rune != 'X' {
		t.Error("resize should preserve content within new bounds")
	}

	// Cell outside new bounds is gone (can't access it)
	got = sb.GetCell(70, 20)
	if got.Rune == 'Y' {
		t.Error("cell outside new bounds should be empty")
	}
}

func TestScreenBufferDirtyTracking(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	// Initial state has fullRedraw
	if !sb.IsDirty() {
		t.Error("new buffer should be dirty")
	}

	sb.Sync()
	if sb.IsDirty() {
		t.Error("buffer should be clean after sync")
	}

	sb.SetCell(10, 5, renderer.NewCell('A'))
	if !sb.IsDirty() {
		t.Error("buffer should be dirty after SetCell")
	}
}

func TestScreenBufferMarkDirty(t *testing.T) {
	sb := NewScreenBuffer(80, 24)
	sb.Sync()

	sb.MarkDirty(10, 5)
	if !sb.IsDirty() {
		t.Error("buffer should be dirty after MarkDirty")
	}
}

func TestScreenBufferMarkRegionDirty(t *testing.T) {
	sb := NewScreenBuffer(80, 24)
	sb.Sync()

	sb.MarkRegionDirty(renderer.NewScreenRect(5, 10, 15, 30))
	if !sb.IsDirty() {
		t.Error("buffer should be dirty after MarkRegionDirty")
	}
}

func TestScreenBufferMarkFullRedraw(t *testing.T) {
	sb := NewScreenBuffer(80, 24)
	sb.Sync()

	sb.MarkFullRedraw()
	if !sb.IsDirty() {
		t.Error("buffer should be dirty after MarkFullRedraw")
	}

	// Full redraw means all cells
	count := sb.DirtyCount()
	if count != 80*24 {
		t.Errorf("expected %d dirty cells, got %d", 80*24, count)
	}
}

func TestScreenBufferComputeDiff(t *testing.T) {
	sb := NewScreenBuffer(80, 24)
	sb.Sync()

	sb.SetCell(10, 5, renderer.NewCell('A'))
	sb.SetCell(20, 10, renderer.NewCell('B'))

	diff := sb.ComputeDiff()
	if len(diff) != 2 {
		t.Errorf("expected 2 changes, got %d", len(diff))
	}
}

func TestScreenBufferComputeDiffSkipsUnchanged(t *testing.T) {
	sb := NewScreenBuffer(80, 24)
	sb.Sync()

	// Set and sync
	sb.SetCell(10, 5, renderer.NewCell('A'))
	sb.Sync()

	// Set same value
	sb.SetCell(10, 5, renderer.NewCell('A'))

	diff := sb.ComputeDiff()
	if len(diff) != 0 {
		t.Errorf("expected 0 changes for unchanged cell, got %d", len(diff))
	}
}

func TestScreenBufferSync(t *testing.T) {
	sb := NewScreenBuffer(80, 24)

	sb.SetCell(10, 5, renderer.NewCell('X'))
	sb.Sync()

	// Front buffer should now have the cell
	got := sb.GetFrontCell(10, 5)
	if got.Rune != 'X' {
		t.Error("sync should copy back to front buffer")
	}

	// Dirty flags should be cleared
	if sb.IsDirty() {
		t.Error("sync should clear dirty flags")
	}
}

func TestBufferedBackend(t *testing.T) {
	nullBackend := NewNullBackend(80, 24)
	buffered := NewBufferedBackend(nullBackend)

	if err := buffered.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	w, h := buffered.Size()
	if w != 80 || h != 24 {
		t.Errorf("expected size (80, 24), got (%d, %d)", w, h)
	}
}

func TestBufferedBackendSetCell(t *testing.T) {
	nullBackend := NewNullBackend(80, 24)
	buffered := NewBufferedBackend(nullBackend)
	buffered.Init()

	cell := renderer.NewCell('X')
	buffered.SetCell(10, 5, cell)

	// Before Show, underlying backend should not have the cell
	underlying := nullBackend.GetCell(10, 5)
	if underlying.Equals(cell) {
		t.Error("underlying backend should not have cell before Show")
	}

	buffered.Show()

	// After Show, underlying backend should have the cell
	underlying = nullBackend.GetCell(10, 5)
	if !underlying.Equals(cell) {
		t.Error("underlying backend should have cell after Show")
	}
}

func TestBufferedBackendMinimalUpdates(t *testing.T) {
	nullBackend := NewNullBackend(80, 24)
	buffered := NewBufferedBackend(nullBackend)
	buffered.Init()

	// Fill everything and show
	buffered.Fill(renderer.NewScreenRect(0, 0, 24, 80), renderer.NewCell('.'))
	buffered.Show()

	// Change one cell
	buffered.SetCell(10, 5, renderer.NewCell('X'))

	// Compute diff should only have 1 change
	diff := buffered.Buffer().ComputeDiff()
	if len(diff) != 1 {
		t.Errorf("expected 1 change, got %d", len(diff))
	}
}

func TestBufferedBackendCursor(t *testing.T) {
	nullBackend := NewNullBackend(80, 24)
	buffered := NewBufferedBackend(nullBackend)
	buffered.Init()

	buffered.ShowCursor(15, 10)

	x, y, visible := nullBackend.CursorPosition()
	if x != 15 || y != 10 || !visible {
		t.Errorf("cursor should be at (15, 10), got (%d, %d)", x, y)
	}

	buffered.HideCursor()
	_, _, visible = nullBackend.CursorPosition()
	if visible {
		t.Error("cursor should be hidden")
	}
}

func TestBufferedBackendSetString(t *testing.T) {
	nullBackend := NewNullBackend(80, 24)
	buffered := NewBufferedBackend(nullBackend)
	buffered.Init()

	style := renderer.DefaultStyle().WithForeground(renderer.ColorRed)
	buffered.SetString(5, 10, "Test", style)
	buffered.Show()

	// Verify the string was written
	got := nullBackend.GetCell(5, 10)
	if got.Rune != 'T' {
		t.Errorf("expected 'T', got %q", got.Rune)
	}
}

func TestBufferedBackendResize(t *testing.T) {
	nullBackend := NewNullBackend(80, 24)
	buffered := NewBufferedBackend(nullBackend)
	buffered.Init()

	resizeCalled := false
	buffered.OnResize(func(w, h int) {
		resizeCalled = true
	})

	nullBackend.Resize(100, 40)

	if !resizeCalled {
		t.Error("resize callback should be called")
	}

	w, h := buffered.Size()
	if w != 100 || h != 40 {
		t.Errorf("expected size (100, 40), got (%d, %d)", w, h)
	}
}
