package viewport

import (
	"testing"
)

func TestNewViewport(t *testing.T) {
	v := NewViewport(80, 24)

	if v.Width() != 80 {
		t.Errorf("expected width 80, got %d", v.Width())
	}
	if v.Height() != 24 {
		t.Errorf("expected height 24, got %d", v.Height())
	}
	if v.TopLine() != 0 {
		t.Errorf("expected top line 0, got %d", v.TopLine())
	}
	if v.LeftColumn() != 0 {
		t.Errorf("expected left column 0, got %d", v.LeftColumn())
	}
}

func TestViewportResize(t *testing.T) {
	v := NewViewport(80, 24)
	v.Resize(120, 40)

	if v.Width() != 120 {
		t.Errorf("expected width 120, got %d", v.Width())
	}
	if v.Height() != 40 {
		t.Errorf("expected height 40, got %d", v.Height())
	}
}

func TestViewportVisibleLineRange(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	start, end := v.VisibleLineRange()
	if start != 0 {
		t.Errorf("expected start 0, got %d", start)
	}
	if end != 23 {
		t.Errorf("expected end 23, got %d", end)
	}

	// Scroll down
	v.ScrollTo(10, false)
	start, end = v.VisibleLineRange()
	if start != 10 {
		t.Errorf("expected start 10, got %d", start)
	}
	if end != 33 {
		t.Errorf("expected end 33, got %d", end)
	}
}

func TestViewportIsLineVisible(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	if !v.IsLineVisible(0) {
		t.Error("line 0 should be visible")
	}
	if !v.IsLineVisible(23) {
		t.Error("line 23 should be visible")
	}
	if v.IsLineVisible(24) {
		t.Error("line 24 should not be visible")
	}

	v.ScrollTo(10, false)
	if v.IsLineVisible(9) {
		t.Error("line 9 should not be visible after scroll")
	}
	if !v.IsLineVisible(10) {
		t.Error("line 10 should be visible after scroll")
	}
}

func TestViewportLineToScreenRow(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	// At top
	if v.LineToScreenRow(0) != 0 {
		t.Errorf("expected row 0 for line 0, got %d", v.LineToScreenRow(0))
	}
	if v.LineToScreenRow(10) != 10 {
		t.Errorf("expected row 10 for line 10, got %d", v.LineToScreenRow(10))
	}

	// After scroll
	v.ScrollTo(5, false)
	if v.LineToScreenRow(5) != 0 {
		t.Errorf("expected row 0 for line 5 after scroll, got %d", v.LineToScreenRow(5))
	}
	if v.LineToScreenRow(4) != -1 {
		t.Errorf("expected row -1 for line 4 after scroll, got %d", v.LineToScreenRow(4))
	}
}

func TestViewportScreenRowToLine(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	if v.ScreenRowToLine(0) != 0 {
		t.Errorf("expected line 0 for row 0, got %d", v.ScreenRowToLine(0))
	}

	v.ScrollTo(10, false)
	if v.ScreenRowToLine(0) != 10 {
		t.Errorf("expected line 10 for row 0 after scroll, got %d", v.ScreenRowToLine(0))
	}
	if v.ScreenRowToLine(5) != 15 {
		t.Errorf("expected line 15 for row 5 after scroll, got %d", v.ScreenRowToLine(5))
	}
}

func TestViewportColumnConversions(t *testing.T) {
	v := NewViewport(80, 24)

	if v.ColumnToScreenCol(0) != 0 {
		t.Errorf("expected screen col 0, got %d", v.ColumnToScreenCol(0))
	}
	if v.ColumnToScreenCol(40) != 40 {
		t.Errorf("expected screen col 40, got %d", v.ColumnToScreenCol(40))
	}

	// After horizontal scroll
	v.ScrollHorizontalBy(10, false)
	if v.ColumnToScreenCol(10) != 0 {
		t.Errorf("expected screen col 0 for buf col 10, got %d", v.ColumnToScreenCol(10))
	}
	if v.ScreenColToColumn(0) != 10 {
		t.Errorf("expected buf col 10 for screen col 0, got %d", v.ScreenColToColumn(0))
	}
}

func TestViewportBufferToScreen(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	row, col := v.BufferToScreen(5, 10)
	if row != 5 || col != 10 {
		t.Errorf("expected (5, 10), got (%d, %d)", row, col)
	}

	// Not visible
	row, col = v.BufferToScreen(50, 10)
	if row != -1 || col != -1 {
		t.Errorf("expected (-1, -1) for invisible position, got (%d, %d)", row, col)
	}

	// After scroll
	v.ScrollTo(10, false)
	row, col = v.BufferToScreen(15, 20)
	if row != 5 || col != 20 {
		t.Errorf("expected (5, 20) after scroll, got (%d, %d)", row, col)
	}
}

func TestViewportScreenToBuffer(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	line, col := v.ScreenToBuffer(5, 10)
	if line != 5 || col != 10 {
		t.Errorf("expected (5, 10), got (%d, %d)", line, col)
	}

	v.ScrollTo(10, false)
	v.ScrollHorizontalBy(5, false)
	line, col = v.ScreenToBuffer(5, 10)
	if line != 15 || col != 15 {
		t.Errorf("expected (15, 15) after scroll, got (%d, %d)", line, col)
	}
}

func TestViewportScrollTo(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.ScrollTo(20, false)
	if v.TopLine() != 20 {
		t.Errorf("expected top line 20, got %d", v.TopLine())
	}

	// Clamp to max
	v.ScrollTo(200, false)
	if v.TopLine() != 99 {
		t.Errorf("expected top line 99 (clamped), got %d", v.TopLine())
	}
}

func TestViewportScrollBy(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.ScrollBy(10, false)
	if v.TopLine() != 10 {
		t.Errorf("expected top line 10, got %d", v.TopLine())
	}

	v.ScrollBy(-5, false)
	if v.TopLine() != 5 {
		t.Errorf("expected top line 5, got %d", v.TopLine())
	}

	// Clamp to 0
	v.ScrollBy(-100, false)
	if v.TopLine() != 0 {
		t.Errorf("expected top line 0, got %d", v.TopLine())
	}
}

func TestViewportScrollToReveal(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetMargins(5, 5, 10, 10)
	v.SetSmoothScroll(false)

	// Already visible - no scroll
	scrolled := v.ScrollToReveal(10, 40, false)
	if scrolled {
		t.Error("should not scroll for already visible position")
	}

	// Below viewport - scroll down
	scrolled = v.ScrollToReveal(50, 40, false)
	if !scrolled {
		t.Error("should scroll to reveal line 50")
	}
	if !v.IsLineVisible(50) {
		t.Error("line 50 should be visible after scroll")
	}

	// Reset and test above viewport
	v.ScrollTo(50, false)
	scrolled = v.ScrollToReveal(10, 40, false)
	if !scrolled {
		t.Error("should scroll to reveal line 10")
	}
	if !v.IsLineVisible(10) {
		t.Error("line 10 should be visible after scroll")
	}
}

func TestViewportCenterOn(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.CenterOn(50, false)

	// Line 50 should be near center (around row 12)
	row := v.LineToScreenRow(50)
	if row < 10 || row > 14 {
		t.Errorf("line 50 should be near center, got row %d", row)
	}
}

func TestViewportSmoothScroll(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(true)

	v.ScrollTo(50, true)

	if !v.IsAnimating() {
		t.Error("should be animating after smooth scroll")
	}

	// Simulate animation
	for i := 0; i < 100; i++ {
		if !v.IsAnimating() {
			break
		}
		v.Update(0.016) // ~60fps
	}

	if v.TopLine() != 50 {
		t.Errorf("expected top line 50 after animation, got %d", v.TopLine())
	}
	if v.IsAnimating() {
		t.Error("should not be animating after completion")
	}
}

func TestViewportStopAnimation(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(true)

	v.ScrollTo(50, true)
	v.StopAnimation()

	if v.IsAnimating() {
		t.Error("should not be animating after StopAnimation")
	}
}

func TestViewportPageUpDown(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.PageDown(false)
	// Should scroll by height - 2 (overlap)
	if v.TopLine() != 22 {
		t.Errorf("expected top line 22 after PageDown, got %d", v.TopLine())
	}

	v.PageUp(false)
	if v.TopLine() != 0 {
		t.Errorf("expected top line 0 after PageUp, got %d", v.TopLine())
	}
}

func TestViewportHalfPageUpDown(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.HalfPageDown(false)
	if v.TopLine() != 12 {
		t.Errorf("expected top line 12 after HalfPageDown, got %d", v.TopLine())
	}

	v.HalfPageUp(false)
	if v.TopLine() != 0 {
		t.Errorf("expected top line 0 after HalfPageUp, got %d", v.TopLine())
	}
}

func TestViewportScrollToTopBottom(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.ScrollToBottom(false)
	// Should show last lines
	if v.BottomLine() != 99 {
		t.Errorf("expected bottom line 99, got %d", v.BottomLine())
	}

	v.ScrollToTop(false)
	if v.TopLine() != 0 {
		t.Errorf("expected top line 0, got %d", v.TopLine())
	}
}

func TestViewportMargins(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMargins(3, 4, 5, 6)

	top, bottom, left, right := v.Margins()
	if top != 3 || bottom != 4 || left != 5 || right != 6 {
		t.Errorf("expected margins (3,4,5,6), got (%d,%d,%d,%d)",
			top, bottom, left, right)
	}
}

func TestViewportClone(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.ScrollTo(10, false)
	v.ScrollHorizontalBy(5, false)

	clone := v.Clone()

	if clone.TopLine() != v.TopLine() {
		t.Error("clone should have same top line")
	}
	if clone.LeftColumn() != v.LeftColumn() {
		t.Error("clone should have same left column")
	}

	// Modify original, clone should not change
	v.ScrollTo(50, false)
	if clone.TopLine() == v.TopLine() {
		t.Error("clone should be independent")
	}
}

func TestViewportIsPositionVisible(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	if !v.IsPositionVisible(10, 40) {
		t.Error("position (10, 40) should be visible")
	}

	if v.IsPositionVisible(10, 100) {
		t.Error("position (10, 100) should not be visible (col out of range)")
	}

	if v.IsPositionVisible(50, 40) {
		t.Error("position (50, 40) should not be visible (line out of range)")
	}
}

func TestViewportMaxLineClamp(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(50)
	v.SetSmoothScroll(false)

	// Try to scroll past max
	v.ScrollTo(100, false)

	// Should be clamped
	if v.TopLine() >= 50 {
		t.Errorf("top line should be clamped, got %d", v.TopLine())
	}
}
