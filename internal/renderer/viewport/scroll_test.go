package viewport

import (
	"testing"
)

func TestGetScrollState(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.ScrollTo(10, false)

	state := v.GetScrollState()

	if state.TopLine != 10 {
		t.Errorf("expected top line 10, got %d", state.TopLine)
	}
	if state.Animating {
		t.Error("should not be animating")
	}
}

func TestSetScrollState(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	state := ScrollState{
		TopLine:          20,
		LeftColumn:       5,
		TargetTopLine:    20,
		TargetLeftColumn: 5,
		Animating:        false,
	}

	v.SetScrollState(state)

	if v.TopLine() != 20 {
		t.Errorf("expected top line 20, got %d", v.TopLine())
	}
	if v.LeftColumn() != 5 {
		t.Errorf("expected left column 5, got %d", v.LeftColumn())
	}
}

func TestScrollProgress(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)

	// Not animating - should be 1.0
	progress := v.ScrollProgress()
	if progress != 1.0 {
		t.Errorf("expected progress 1.0 when not animating, got %f", progress)
	}

	// Start animation
	v.SetSmoothScroll(true)
	v.ScrollTo(50, true)

	// Should be less than 1.0 during animation
	if v.ScrollProgress() == 1.0 && v.IsAnimating() {
		t.Error("progress should not be 1.0 during animation")
	}
}

func TestScrollingDirection(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(true)

	// Not animating
	vDir, hDir := v.ScrollingDirection()
	if vDir != ScrollNone || hDir != ScrollNone {
		t.Error("should be ScrollNone when not animating")
	}

	// Scroll down
	v.ScrollTo(50, true)
	vDir, hDir = v.ScrollingDirection()
	if vDir != ScrollDown {
		t.Errorf("expected ScrollDown, got %d", vDir)
	}

	// Complete animation and scroll up
	for v.IsAnimating() {
		v.Update(0.1)
	}

	v.ScrollTo(10, true)
	vDir, _ = v.ScrollingDirection()
	if vDir != ScrollUp {
		t.Errorf("expected ScrollUp, got %d", vDir)
	}
}

func TestEnsureLineVisible(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	// Already visible
	scrolled := v.EnsureLineVisible(10, false)
	if scrolled {
		t.Error("should not scroll for already visible line")
	}

	// Not visible
	scrolled = v.EnsureLineVisible(50, false)
	if !scrolled {
		t.Error("should scroll to make line 50 visible")
	}
	if !v.IsLineVisible(50) {
		t.Error("line 50 should be visible after scroll")
	}
}

func TestEnsureRangeVisible(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	// Range already visible
	scrolled := v.EnsureRangeVisible(5, 15, false)
	if scrolled {
		t.Error("should not scroll for already visible range")
	}

	// Range not visible but fits
	scrolled = v.EnsureRangeVisible(50, 60, false)
	if !scrolled {
		t.Error("should scroll for non-visible range")
	}
	if !v.IsLineVisible(50) || !v.IsLineVisible(60) {
		t.Error("range should be visible after scroll")
	}
}

func TestScrollPercent(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	// At top
	percent := v.ScrollPercent()
	if percent != 0.0 {
		t.Errorf("expected 0%% at top, got %f", percent)
	}

	// At bottom
	v.ScrollToBottom(false)
	percent = v.ScrollPercent()
	if percent < 0.99 {
		t.Errorf("expected ~100%% at bottom, got %f", percent)
	}

	// Middle
	v.ScrollTo(38, false) // 38 is roughly 50% of (100 - 24)
	percent = v.ScrollPercent()
	if percent < 0.45 || percent > 0.55 {
		t.Errorf("expected ~50%% in middle, got %f", percent)
	}
}

func TestScrollToPercent(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	v.ScrollToPercent(0.5, false)
	percent := v.ScrollPercent()
	if percent < 0.45 || percent > 0.55 {
		t.Errorf("expected ~50%%, got %f", percent)
	}

	v.ScrollToPercent(0.0, false)
	if v.TopLine() != 0 {
		t.Errorf("expected top line 0 at 0%%, got %d", v.TopLine())
	}

	v.ScrollToPercent(1.0, false)
	// Should be at bottom
	if v.BottomLine() != 99 {
		t.Errorf("expected bottom line 99 at 100%%, got %d", v.BottomLine())
	}
}

func TestScrollPercentClamp(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetSmoothScroll(false)

	// Negative percent should clamp to 0
	v.ScrollToPercent(-0.5, false)
	if v.TopLine() != 0 {
		t.Error("negative percent should scroll to top")
	}

	// > 1.0 should clamp to bottom
	v.ScrollToPercent(2.0, false)
	if v.BottomLine() != 99 {
		t.Error("> 1.0 percent should scroll to bottom")
	}
}

func TestLineScrollOffset(t *testing.T) {
	v := NewViewport(80, 24)

	// For now, always returns 0 (integer scrolling)
	offset := v.LineScrollOffset()
	if offset != 0.0 {
		t.Errorf("expected offset 0.0, got %f", offset)
	}
}
