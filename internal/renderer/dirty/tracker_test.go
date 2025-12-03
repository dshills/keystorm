package dirty

import (
	"sync"
	"testing"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker(80, 24)

	if tracker.screenWidth != 80 {
		t.Errorf("screenWidth = %d, want 80", tracker.screenWidth)
	}
	if tracker.screenHeight != 24 {
		t.Errorf("screenHeight = %d, want 24", tracker.screenHeight)
	}
	if tracker.fullRedraw {
		t.Error("New tracker should not need full redraw")
	}
	if len(tracker.regions) != 0 {
		t.Error("New tracker should have no regions")
	}
}

func TestTrackerSetScreenSize(t *testing.T) {
	tracker := NewTracker(80, 24)

	tracker.SetScreenSize(120, 40)

	if tracker.screenWidth != 120 {
		t.Errorf("screenWidth = %d, want 120", tracker.screenWidth)
	}
	if tracker.screenHeight != 40 {
		t.Errorf("screenHeight = %d, want 40", tracker.screenHeight)
	}
	if !tracker.fullRedraw {
		t.Error("Screen resize should trigger full redraw")
	}
}

func TestTrackerMarkFullRedraw(t *testing.T) {
	tracker := NewTracker(80, 24)
	tracker.MarkLine(5)

	tracker.MarkFullRedraw()

	if !tracker.NeedsFullRedraw() {
		t.Error("Should need full redraw")
	}
	if len(tracker.regions) != 0 {
		t.Error("Regions should be cleared on full redraw")
	}
}

func TestTrackerMarkLine(t *testing.T) {
	tracker := NewTracker(80, 24)

	tracker.MarkLine(5)

	if !tracker.IsDirty() {
		t.Error("Should be dirty after marking line")
	}
	if !tracker.IsLineDirty(5) {
		t.Error("Line 5 should be dirty")
	}
	if tracker.IsLineDirty(4) {
		t.Error("Line 4 should not be dirty")
	}
}

func TestTrackerMarkLines(t *testing.T) {
	tracker := NewTracker(80, 24)

	tracker.MarkLines(5, 10)

	for line := uint32(5); line <= 10; line++ {
		if !tracker.IsLineDirty(line) {
			t.Errorf("Line %d should be dirty", line)
		}
	}
	if tracker.IsLineDirty(4) {
		t.Error("Line 4 should not be dirty")
	}
	if tracker.IsLineDirty(11) {
		t.Error("Line 11 should not be dirty")
	}
}

func TestTrackerMarkRegion(t *testing.T) {
	tracker := NewTracker(80, 24)

	region := NewColumnRegion(5, 10, 20)
	tracker.MarkRegion(region)

	if !tracker.IsLineDirty(5) {
		t.Error("Line 5 should be dirty")
	}
	if !tracker.IsRegionDirty(NewColumnRegion(5, 15, 18)) {
		t.Error("Overlapping column region should be dirty")
	}
}

func TestTrackerMarkChange(t *testing.T) {
	t.Run("resize triggers full redraw", func(t *testing.T) {
		tracker := NewTracker(80, 24)
		tracker.MarkChange(Change{Type: ChangeResize})

		if !tracker.NeedsFullRedraw() {
			t.Error("Resize should trigger full redraw")
		}
	})

	t.Run("scroll triggers full redraw", func(t *testing.T) {
		tracker := NewTracker(80, 24)
		tracker.MarkChange(Change{Type: ChangeScroll})

		if !tracker.NeedsFullRedraw() {
			t.Error("Scroll should trigger full redraw")
		}
	})

	t.Run("insert extends to end of screen", func(t *testing.T) {
		tracker := NewTracker(80, 24)
		tracker.MarkChange(Change{
			Type:   ChangeInsert,
			Region: NewSingleLine(5),
		})

		// Should extend to bottom of screen
		if !tracker.IsLineDirty(23) {
			t.Error("Insert should mark lines to end of screen")
		}
	})

	t.Run("style change marks only affected region", func(t *testing.T) {
		tracker := NewTracker(80, 24)
		tracker.MarkChange(Change{
			Type:   ChangeStyle,
			Region: NewSingleLine(5),
		})

		if !tracker.IsLineDirty(5) {
			t.Error("Line 5 should be dirty")
		}
		if tracker.IsLineDirty(6) {
			t.Error("Line 6 should not be dirty for style change")
		}
	})
}

func TestTrackerMarkIgnoredOnFullRedraw(t *testing.T) {
	tracker := NewTracker(80, 24)
	tracker.MarkFullRedraw()

	initialRegions := len(tracker.regions)
	tracker.MarkLine(5)

	if len(tracker.regions) != initialRegions {
		t.Error("Should not add regions when full redraw is set")
	}
}

func TestTrackerCoalescing(t *testing.T) {
	t.Run("adjacent lines merge", func(t *testing.T) {
		tracker := NewTracker(80, 24)

		tracker.MarkLine(5)
		tracker.MarkLine(6)

		if tracker.RegionCount() != 1 {
			t.Errorf("RegionCount = %d, want 1 (should merge)", tracker.RegionCount())
		}
	})

	t.Run("overlapping regions merge", func(t *testing.T) {
		tracker := NewTracker(80, 24)

		tracker.MarkLines(5, 10)
		tracker.MarkLines(8, 15)

		if tracker.RegionCount() != 1 {
			t.Errorf("RegionCount = %d, want 1 (should merge)", tracker.RegionCount())
		}

		regions := tracker.DirtyRegions()
		if len(regions) != 1 {
			t.Fatalf("Expected 1 region, got %d", len(regions))
		}
		if regions[0].StartLine != 5 || regions[0].EndLine != 15 {
			t.Errorf("Region = {%d, %d}, want {5, 15}", regions[0].StartLine, regions[0].EndLine)
		}
	})

	t.Run("non-adjacent regions stay separate", func(t *testing.T) {
		tracker := NewTracker(80, 24)

		tracker.MarkLines(0, 2)
		tracker.MarkLines(10, 12)

		if tracker.RegionCount() != 2 {
			t.Errorf("RegionCount = %d, want 2 (should not merge)", tracker.RegionCount())
		}
	})
}

func TestTrackerMaxRegions(t *testing.T) {
	tracker := NewTracker(80, 100)
	tracker.SetMaxRegions(5)
	tracker.SetCoalesceThreshold(0.8) // High threshold to prevent auto-full-redraw

	// Add many adjacent regions that should merge
	for i := uint32(0); i < 20; i++ {
		tracker.MarkLine(i)
	}

	// Adjacent lines should merge into one region
	if tracker.RegionCount() > 1 {
		t.Errorf("Adjacent lines should merge, got %d regions", tracker.RegionCount())
	}
}

func TestTrackerCoalesceThreshold(t *testing.T) {
	tracker := NewTracker(80, 20)
	tracker.SetCoalesceThreshold(0.3) // 30% threshold

	// Mark 40% of screen (8 lines out of 20)
	tracker.MarkLines(0, 7)

	if !tracker.NeedsFullRedraw() {
		t.Error("Should trigger full redraw when dirty ratio exceeds threshold")
	}
}

func TestTrackerDirtyRegions(t *testing.T) {
	t.Run("returns copy of regions", func(t *testing.T) {
		tracker := NewTracker(80, 24)
		tracker.MarkLines(5, 10)

		regions1 := tracker.DirtyRegions()
		regions2 := tracker.DirtyRegions()

		// Modify first slice
		if len(regions1) > 0 {
			regions1[0].StartLine = 100
		}

		// Second should be unaffected
		if len(regions2) > 0 && regions2[0].StartLine == 100 {
			t.Error("DirtyRegions should return a copy")
		}
	})

	t.Run("full redraw returns full screen region", func(t *testing.T) {
		tracker := NewTracker(80, 24)
		tracker.MarkFullRedraw()

		regions := tracker.DirtyRegions()
		if len(regions) != 1 {
			t.Fatalf("Expected 1 region for full redraw, got %d", len(regions))
		}
		if regions[0].StartLine != 0 || regions[0].EndLine != 23 {
			t.Errorf("Full redraw region = {%d, %d}, want {0, 23}", regions[0].StartLine, regions[0].EndLine)
		}
	})
}

func TestTrackerDirtyLines(t *testing.T) {
	t.Run("returns sorted line numbers", func(t *testing.T) {
		tracker := NewTracker(80, 24)
		tracker.MarkLines(10, 12)
		tracker.MarkLines(5, 7)

		lines := tracker.DirtyLines()

		// Should be sorted
		for i := 1; i < len(lines); i++ {
			if lines[i] < lines[i-1] {
				t.Error("Lines should be sorted")
			}
		}

		// Should include all dirty lines
		expected := map[uint32]bool{5: true, 6: true, 7: true, 10: true, 11: true, 12: true}
		for _, line := range lines {
			if !expected[line] {
				t.Errorf("Unexpected line %d", line)
			}
			delete(expected, line)
		}
		if len(expected) > 0 {
			t.Errorf("Missing lines: %v", expected)
		}
	})

	t.Run("full redraw returns all lines", func(t *testing.T) {
		tracker := NewTracker(80, 10)
		tracker.MarkFullRedraw()

		lines := tracker.DirtyLines()
		if len(lines) != 10 {
			t.Errorf("Expected 10 lines for full redraw, got %d", len(lines))
		}
	})
}

func TestTrackerClear(t *testing.T) {
	tracker := NewTracker(80, 24)
	tracker.MarkLines(5, 10)
	tracker.MarkFullRedraw()

	tracker.Clear()

	if tracker.IsDirty() {
		t.Error("Should not be dirty after clear")
	}
	if tracker.NeedsFullRedraw() {
		t.Error("Should not need full redraw after clear")
	}
	if tracker.RegionCount() != 0 {
		t.Error("Should have no regions after clear")
	}
}

func TestTrackerStats(t *testing.T) {
	tracker := NewTracker(80, 24)
	tracker.MarkLines(5, 10)

	stats := tracker.Stats()

	if stats.ScreenWidth != 80 {
		t.Errorf("ScreenWidth = %d, want 80", stats.ScreenWidth)
	}
	if stats.ScreenHeight != 24 {
		t.Errorf("ScreenHeight = %d, want 24", stats.ScreenHeight)
	}
	if stats.RegionCount != 1 {
		t.Errorf("RegionCount = %d, want 1", stats.RegionCount)
	}
	if stats.FullRedraw {
		t.Error("FullRedraw should be false")
	}
	if stats.DirtyRatio <= 0 {
		t.Error("DirtyRatio should be positive")
	}
}

func TestTrackerConcurrency(t *testing.T) {
	tracker := NewTracker(80, 24)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(line uint32) {
			defer wg.Done()
			tracker.MarkLine(line)
			tracker.IsLineDirty(line)
			tracker.DirtyRegions()
		}(uint32(i))
	}
	wg.Wait()

	// Should not panic and should have some dirty lines
	if !tracker.IsDirty() {
		t.Error("Should be dirty after concurrent marks")
	}
}

func TestChangeTypeString(t *testing.T) {
	tests := []struct {
		ct       ChangeType
		expected string
	}{
		{ChangeInsert, "insert"},
		{ChangeDelete, "delete"},
		{ChangeReplace, "replace"},
		{ChangeStyle, "style"},
		{ChangeCursor, "cursor"},
		{ChangeSelection, "selection"},
		{ChangeScroll, "scroll"},
		{ChangeResize, "resize"},
		{ChangeOverlay, "overlay"},
		{ChangeType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.expected {
			t.Errorf("%d.String() = %q, want %q", tt.ct, got, tt.expected)
		}
	}
}

func TestTrackerClampToScreen(t *testing.T) {
	tracker := NewTracker(80, 24)

	// Mark region beyond screen height
	tracker.MarkLines(20, 50)

	regions := tracker.DirtyRegions()
	if len(regions) != 1 {
		t.Fatalf("Expected 1 region, got %d", len(regions))
	}

	// Should be clamped to screen height - 1
	if regions[0].EndLine > 23 {
		t.Errorf("EndLine = %d, should be clamped to 23", regions[0].EndLine)
	}
}

func TestTrackerEmptyRegion(t *testing.T) {
	tracker := NewTracker(80, 24)

	// Try to mark an empty region
	tracker.MarkRegion(Region{StartLine: 10, EndLine: 5}) // Invalid

	if tracker.IsDirty() {
		t.Error("Empty/invalid regions should be ignored")
	}
}
