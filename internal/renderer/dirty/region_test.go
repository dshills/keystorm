package dirty

import (
	"testing"
)

func TestNewLineRegion(t *testing.T) {
	t.Run("normal order", func(t *testing.T) {
		r := NewLineRegion(5, 10)
		if r.StartLine != 5 || r.EndLine != 10 {
			t.Errorf("NewLineRegion(5, 10) = {%d, %d}, want {5, 10}", r.StartLine, r.EndLine)
		}
		if !r.FullWidth {
			t.Error("Line region should be full width")
		}
	})

	t.Run("reversed order", func(t *testing.T) {
		r := NewLineRegion(10, 5)
		if r.StartLine != 5 || r.EndLine != 10 {
			t.Errorf("NewLineRegion(10, 5) should swap to {5, 10}, got {%d, %d}", r.StartLine, r.EndLine)
		}
	})
}

func TestNewSingleLine(t *testing.T) {
	r := NewSingleLine(7)
	if r.StartLine != 7 || r.EndLine != 7 {
		t.Errorf("NewSingleLine(7) = {%d, %d}, want {7, 7}", r.StartLine, r.EndLine)
	}
	if !r.FullWidth {
		t.Error("Single line region should be full width")
	}
}

func TestNewColumnRegion(t *testing.T) {
	r := NewColumnRegion(5, 10, 20)
	if r.StartLine != 5 || r.EndLine != 5 {
		t.Errorf("Line = {%d, %d}, want {5, 5}", r.StartLine, r.EndLine)
	}
	if r.StartCol != 10 || r.EndCol != 20 {
		t.Errorf("Cols = {%d, %d}, want {10, 20}", r.StartCol, r.EndCol)
	}
	if r.FullWidth {
		t.Error("Column region should not be full width")
	}
}

func TestNewRectRegion(t *testing.T) {
	r := NewRectRegion(5, 10, 20, 30)
	if r.StartLine != 5 || r.EndLine != 10 {
		t.Errorf("Lines = {%d, %d}, want {5, 10}", r.StartLine, r.EndLine)
	}
	if r.StartCol != 20 || r.EndCol != 30 {
		t.Errorf("Cols = {%d, %d}, want {20, 30}", r.StartCol, r.EndCol)
	}
}

func TestRegionIsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		region Region
		empty  bool
	}{
		{"valid line region", NewLineRegion(5, 10), false},
		{"single line", NewSingleLine(5), false},
		{"valid column region", NewColumnRegion(5, 10, 20), false},
		{"empty column region", NewColumnRegion(5, 20, 20), true},
		{"inverted lines", Region{StartLine: 10, EndLine: 5}, true},
		{"inverted cols", Region{StartLine: 5, EndLine: 5, StartCol: 20, EndCol: 10}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.region.IsEmpty(); got != tt.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.empty)
			}
		})
	}
}

func TestRegionLineCount(t *testing.T) {
	tests := []struct {
		region Region
		count  uint32
	}{
		{NewLineRegion(5, 10), 6},
		{NewSingleLine(5), 1},
		{NewLineRegion(0, 0), 1},
		{Region{StartLine: 10, EndLine: 5}, 0}, // Invalid
	}

	for _, tt := range tests {
		if got := tt.region.LineCount(); got != tt.count {
			t.Errorf("LineCount() = %d, want %d", got, tt.count)
		}
	}
}

func TestRegionContainsLine(t *testing.T) {
	r := NewLineRegion(5, 10)

	tests := []struct {
		line     uint32
		contains bool
	}{
		{4, false},
		{5, true},
		{7, true},
		{10, true},
		{11, false},
	}

	for _, tt := range tests {
		if got := r.ContainsLine(tt.line); got != tt.contains {
			t.Errorf("ContainsLine(%d) = %v, want %v", tt.line, got, tt.contains)
		}
	}
}

func TestRegionContains(t *testing.T) {
	t.Run("full width", func(t *testing.T) {
		r := NewLineRegion(5, 10)
		if !r.Contains(7, 100) {
			t.Error("Full width region should contain any column on valid line")
		}
		if r.Contains(3, 0) {
			t.Error("Should not contain line outside range")
		}
	})

	t.Run("column region", func(t *testing.T) {
		r := NewColumnRegion(5, 10, 20)
		if !r.Contains(5, 15) {
			t.Error("Should contain column in range")
		}
		if r.Contains(5, 5) {
			t.Error("Should not contain column before range")
		}
		if r.Contains(5, 20) {
			t.Error("EndCol is exclusive")
		}
	})
}

func TestRegionOverlaps(t *testing.T) {
	tests := []struct {
		name     string
		r1, r2   Region
		overlaps bool
	}{
		{
			"same region",
			NewLineRegion(5, 10),
			NewLineRegion(5, 10),
			true,
		},
		{
			"partial line overlap",
			NewLineRegion(5, 10),
			NewLineRegion(8, 15),
			true,
		},
		{
			"no line overlap",
			NewLineRegion(5, 10),
			NewLineRegion(11, 15),
			false,
		},
		{
			"column overlap",
			NewColumnRegion(5, 10, 20),
			NewColumnRegion(5, 15, 25),
			true,
		},
		{
			"no column overlap",
			NewColumnRegion(5, 10, 20),
			NewColumnRegion(5, 25, 30),
			false,
		},
		{
			"full width overlaps column",
			NewLineRegion(5, 5),
			NewColumnRegion(5, 10, 20),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r1.Overlaps(tt.r2); got != tt.overlaps {
				t.Errorf("Overlaps() = %v, want %v", got, tt.overlaps)
			}
		})
	}
}

func TestRegionAdjacent(t *testing.T) {
	tests := []struct {
		name     string
		r1, r2   Region
		adjacent bool
	}{
		{
			"vertically adjacent full width",
			NewLineRegion(5, 10),
			NewLineRegion(11, 15),
			true,
		},
		{
			"gap between",
			NewLineRegion(5, 10),
			NewLineRegion(12, 15),
			false,
		},
		{
			"horizontally adjacent same line",
			NewColumnRegion(5, 10, 20),
			NewColumnRegion(5, 20, 30),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r1.Adjacent(tt.r2); got != tt.adjacent {
				t.Errorf("Adjacent() = %v, want %v", got, tt.adjacent)
			}
		})
	}
}

func TestRegionMerge(t *testing.T) {
	t.Run("overlapping regions", func(t *testing.T) {
		r1 := NewLineRegion(5, 10)
		r2 := NewLineRegion(8, 15)

		merged, ok := r1.Merge(r2)
		if !ok {
			t.Fatal("Should merge overlapping regions")
		}
		if merged.StartLine != 5 || merged.EndLine != 15 {
			t.Errorf("Merged = {%d, %d}, want {5, 15}", merged.StartLine, merged.EndLine)
		}
	})

	t.Run("adjacent regions", func(t *testing.T) {
		r1 := NewLineRegion(5, 10)
		r2 := NewLineRegion(11, 15)

		merged, ok := r1.Merge(r2)
		if !ok {
			t.Fatal("Should merge adjacent regions")
		}
		if merged.StartLine != 5 || merged.EndLine != 15 {
			t.Errorf("Merged = {%d, %d}, want {5, 15}", merged.StartLine, merged.EndLine)
		}
	})

	t.Run("non-adjacent regions", func(t *testing.T) {
		r1 := NewLineRegion(5, 10)
		r2 := NewLineRegion(12, 15)

		_, ok := r1.Merge(r2)
		if ok {
			t.Error("Should not merge non-adjacent regions")
		}
	})

	t.Run("column merge", func(t *testing.T) {
		r1 := NewColumnRegion(5, 10, 20)
		r2 := NewColumnRegion(5, 15, 30)

		merged, ok := r1.Merge(r2)
		if !ok {
			t.Fatal("Should merge overlapping column regions")
		}
		if merged.StartCol != 10 || merged.EndCol != 30 {
			t.Errorf("Merged cols = {%d, %d}, want {10, 30}", merged.StartCol, merged.EndCol)
		}
	})
}

func TestRegionExpand(t *testing.T) {
	r := NewLineRegion(5, 10)

	expanded := r.Expand(3, 0)
	if expanded.StartLine != 3 {
		t.Errorf("StartLine = %d, want 3", expanded.StartLine)
	}

	expanded = r.Expand(15, 0)
	if expanded.EndLine != 15 {
		t.Errorf("EndLine = %d, want 15", expanded.EndLine)
	}
}

func TestRegionExpandLines(t *testing.T) {
	r := NewLineRegion(5, 10)

	expanded := r.ExpandLines(3, 15)
	if expanded.StartLine != 3 || expanded.EndLine != 15 {
		t.Errorf("Expanded = {%d, %d}, want {3, 15}", expanded.StartLine, expanded.EndLine)
	}
}

func TestRegionIntersect(t *testing.T) {
	t.Run("overlapping", func(t *testing.T) {
		r1 := NewLineRegion(5, 10)
		r2 := NewLineRegion(8, 15)

		inter := r1.Intersect(r2)
		if inter.StartLine != 8 || inter.EndLine != 10 {
			t.Errorf("Intersect = {%d, %d}, want {8, 10}", inter.StartLine, inter.EndLine)
		}
	})

	t.Run("no overlap", func(t *testing.T) {
		r1 := NewLineRegion(5, 10)
		r2 := NewLineRegion(15, 20)

		inter := r1.Intersect(r2)
		if !inter.IsEmpty() {
			t.Error("Non-overlapping regions should have empty intersection")
		}
	})
}

func TestRegionEquals(t *testing.T) {
	r1 := NewLineRegion(5, 10)
	r2 := NewLineRegion(5, 10)
	r3 := NewLineRegion(5, 11)

	if !r1.Equals(r2) {
		t.Error("Identical regions should be equal")
	}
	if r1.Equals(r3) {
		t.Error("Different regions should not be equal")
	}
}
