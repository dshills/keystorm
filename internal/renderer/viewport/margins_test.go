package viewport

import (
	"testing"
)

func TestDefaultMargins(t *testing.T) {
	m := DefaultMargins()

	if m.Top != 5 || m.Bottom != 5 {
		t.Errorf("expected vertical margins (5, 5), got (%d, %d)", m.Top, m.Bottom)
	}
	if m.Left != 10 || m.Right != 10 {
		t.Errorf("expected horizontal margins (10, 10), got (%d, %d)", m.Left, m.Right)
	}
}

func TestCompactMargins(t *testing.T) {
	m := CompactMargins()

	if m.Top != 2 || m.Bottom != 2 {
		t.Errorf("expected vertical margins (2, 2), got (%d, %d)", m.Top, m.Bottom)
	}
	if m.Left != 5 || m.Right != 5 {
		t.Errorf("expected horizontal margins (5, 5), got (%d, %d)", m.Left, m.Right)
	}
}

func TestNoMargins(t *testing.T) {
	m := NoMargins()

	if m.Top != 0 || m.Bottom != 0 || m.Left != 0 || m.Right != 0 {
		t.Error("NoMargins should return all zeros")
	}
}

func TestSetMarginsFromConfig(t *testing.T) {
	v := NewViewport(80, 24)
	config := MarginConfig{Top: 3, Bottom: 4, Left: 5, Right: 6}

	v.SetMarginsFromConfig(config)

	top, bottom, left, right := v.Margins()
	if top != 3 || bottom != 4 || left != 5 || right != 6 {
		t.Errorf("expected (3,4,5,6), got (%d,%d,%d,%d)", top, bottom, left, right)
	}
}

func TestGetMarginConfig(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMargins(7, 8, 9, 10)

	config := v.GetMarginConfig()

	if config.Top != 7 || config.Bottom != 8 || config.Left != 9 || config.Right != 10 {
		t.Errorf("expected (7,8,9,10), got (%d,%d,%d,%d)",
			config.Top, config.Bottom, config.Left, config.Right)
	}
}

func TestEffectiveMargins(t *testing.T) {
	v := NewViewport(80, 24)

	// Normal margins - should not be clamped
	v.SetMargins(5, 5, 10, 10)
	eff := v.EffectiveMargins()
	if eff.Top != 5 || eff.Bottom != 5 {
		t.Errorf("expected vertical margins (5, 5), got (%d, %d)", eff.Top, eff.Bottom)
	}

	// Excessive margins - should be clamped to 1/3 of dimension
	v.SetMargins(20, 20, 40, 40)
	eff = v.EffectiveMargins()

	maxVertical := 24 / 3 // 8
	if eff.Top > maxVertical || eff.Bottom > maxVertical {
		t.Errorf("vertical margins should be clamped to %d, got (%d, %d)",
			maxVertical, eff.Top, eff.Bottom)
	}

	maxHorizontal := 80 / 3 // 26
	if eff.Left > maxHorizontal || eff.Right > maxHorizontal {
		t.Errorf("horizontal margins should be clamped to %d, got (%d, %d)",
			maxHorizontal, eff.Left, eff.Right)
	}
}

func TestCursorZones(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetMargins(5, 5, 10, 10)

	tests := []struct {
		name      string
		line      uint32
		col       int
		expectedV CursorZone
		expectedH CursorZone
	}{
		{"center", 12, 40, ZoneCenter, ZoneCenter},
		{"top margin", 2, 40, ZoneTopMargin, ZoneCenter},
		{"bottom margin", 21, 40, ZoneBottomMargin, ZoneCenter},
		{"left margin", 12, 5, ZoneCenter, ZoneLeftMargin},
		{"right margin", 12, 75, ZoneCenter, ZoneRightMargin},
		{"first line in top margin", 0, 40, ZoneTopMargin, ZoneCenter}, // Line 0 with topLine=0 is in top margin (row 0 < marginTop 5)
		{"below viewport", 50, 40, ZoneBelow, ZoneCenter},
		{"left of viewport", 12, -5, ZoneCenter, ZoneLeft},
		{"right of viewport", 12, 100, ZoneCenter, ZoneRight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vZone, hZone := v.CursorZones(tt.line, tt.col)
			if vZone != tt.expectedV {
				t.Errorf("vertical zone: expected %d, got %d", tt.expectedV, vZone)
			}
			if hZone != tt.expectedH {
				t.Errorf("horizontal zone: expected %d, got %d", tt.expectedH, hZone)
			}
		})
	}
}

func TestNeedsScrollForCursor(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetMargins(5, 5, 10, 10)

	// Center - no scroll needed
	if v.NeedsScrollForCursor(12, 40) {
		t.Error("center position should not need scroll")
	}

	// Top margin - scroll needed
	if !v.NeedsScrollForCursor(2, 40) {
		t.Error("top margin position should need scroll")
	}

	// Below viewport - scroll needed
	if !v.NeedsScrollForCursor(50, 40) {
		t.Error("below viewport should need scroll")
	}
}

func TestVisibleContentArea(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetMargins(5, 5, 10, 10)

	area := v.VisibleContentArea()

	if area.StartLine != 5 {
		t.Errorf("expected start line 5, got %d", area.StartLine)
	}
	// End line should be bottomLine - marginBottom
	expectedEndLine := uint32(23 - 5) // 18
	if area.EndLine != expectedEndLine {
		t.Errorf("expected end line %d, got %d", expectedEndLine, area.EndLine)
	}

	if area.StartColumn != 10 {
		t.Errorf("expected start column 10, got %d", area.StartColumn)
	}
	expectedEndColumn := 80 - 10 // 70
	if area.EndColumn != expectedEndColumn {
		t.Errorf("expected end column %d, got %d", expectedEndColumn, area.EndColumn)
	}
}

func TestVisibleContentAreaAfterScroll(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetMargins(5, 5, 10, 10)
	v.SetSmoothScroll(false)

	v.ScrollTo(20, false)
	v.ScrollHorizontalBy(15, false)

	area := v.VisibleContentArea()

	// Start line should be topLine + marginTop
	if area.StartLine != 25 { // 20 + 5
		t.Errorf("expected start line 25, got %d", area.StartLine)
	}

	// Start column should be leftColumn + marginLeft
	if area.StartColumn != 25 { // 15 + 10
		t.Errorf("expected start column 25, got %d", area.StartColumn)
	}
}

func TestCursorZonesWithScroll(t *testing.T) {
	v := NewViewport(80, 24)
	v.SetMaxLine(100)
	v.SetMargins(5, 5, 10, 10)
	v.SetSmoothScroll(false)

	v.ScrollTo(20, false)

	// Line 22 is now in top margin (row 2 in viewport)
	vZone, _ := v.CursorZones(22, 40)
	if vZone != ZoneTopMargin {
		t.Errorf("line 22 should be in top margin after scroll, got zone %d", vZone)
	}

	// Line 30 should be in center
	vZone, _ = v.CursorZones(30, 40)
	if vZone != ZoneCenter {
		t.Errorf("line 30 should be in center, got zone %d", vZone)
	}

	// Line 15 is above viewport
	vZone, _ = v.CursorZones(15, 40)
	if vZone != ZoneAbove {
		t.Errorf("line 15 should be above viewport, got zone %d", vZone)
	}
}
