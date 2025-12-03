package layout

import (
	"testing"
)

func TestNewTabExpander(t *testing.T) {
	te := NewTabExpander(4)
	if te.TabWidth() != 4 {
		t.Errorf("expected tab width 4, got %d", te.TabWidth())
	}

	// Invalid width defaults to 4
	te = NewTabExpander(0)
	if te.TabWidth() != 4 {
		t.Errorf("expected default tab width 4, got %d", te.TabWidth())
	}

	te = NewTabExpander(-1)
	if te.TabWidth() != 4 {
		t.Errorf("expected default tab width 4 for negative, got %d", te.TabWidth())
	}
}

func TestTabExpanderSetTabWidth(t *testing.T) {
	te := NewTabExpander(4)
	te.SetTabWidth(8)
	if te.TabWidth() != 8 {
		t.Errorf("expected tab width 8, got %d", te.TabWidth())
	}

	te.SetTabWidth(0)
	if te.TabWidth() != 1 {
		t.Errorf("expected minimum tab width 1, got %d", te.TabWidth())
	}
}

func TestNextTabStop(t *testing.T) {
	te := NewTabExpander(4)

	tests := []struct {
		col      int
		expected int
	}{
		{0, 4},
		{1, 4},
		{2, 4},
		{3, 4},
		{4, 8},
		{5, 8},
		{7, 8},
		{8, 12},
	}

	for _, tt := range tests {
		got := te.NextTabStop(tt.col)
		if got != tt.expected {
			t.Errorf("NextTabStop(%d): expected %d, got %d", tt.col, tt.expected, got)
		}
	}
}

func TestTabStopOffset(t *testing.T) {
	te := NewTabExpander(4)

	tests := []struct {
		col      int
		expected int
	}{
		{0, 4},
		{1, 3},
		{2, 2},
		{3, 1},
		{4, 4},
		{5, 3},
	}

	for _, tt := range tests {
		got := te.TabStopOffset(tt.col)
		if got != tt.expected {
			t.Errorf("TabStopOffset(%d): expected %d, got %d", tt.col, tt.expected, got)
		}
	}
}

func TestIsTabStop(t *testing.T) {
	te := NewTabExpander(4)

	tabStops := []int{0, 4, 8, 12, 16}
	for _, col := range tabStops {
		if !te.IsTabStop(col) {
			t.Errorf("IsTabStop(%d) should be true", col)
		}
	}

	nonTabStops := []int{1, 2, 3, 5, 6, 7, 9, 10, 11}
	for _, col := range nonTabStops {
		if te.IsTabStop(col) {
			t.Errorf("IsTabStop(%d) should be false", col)
		}
	}
}

func TestPrevTabStop(t *testing.T) {
	te := NewTabExpander(4)

	tests := []struct {
		col      int
		expected int
	}{
		{0, 0},
		{1, 0},
		{3, 0},
		{4, 0},
		{5, 4},
		{7, 4},
		{8, 4},
		{9, 8},
	}

	for _, tt := range tests {
		got := te.PrevTabStop(tt.col)
		if got != tt.expected {
			t.Errorf("PrevTabStop(%d): expected %d, got %d", tt.col, tt.expected, got)
		}
	}
}

func TestExpandedWidth(t *testing.T) {
	te := NewTabExpander(4)

	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 5},
		{"\t", 4},
		{"\t\t", 8},
		{"a\t", 4},
		{"ab\t", 4},
		{"abc\t", 4},
		{"abcd\t", 8},
		{"a\tb", 5},
		{"\thello", 9},
	}

	for _, tt := range tests {
		got := te.ExpandedWidth(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandedWidth(%q): expected %d, got %d", tt.input, tt.expected, got)
		}
	}
}

func TestExpandTabs(t *testing.T) {
	te := NewTabExpander(4)

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"hello", "hello"},
		{"\t", "    "},
		{"\t\t", "        "},
		{"a\t", "a   "},
		{"ab\t", "ab  "},
		{"abc\t", "abc "},
		{"abcd\t", "abcd    "},
		{"a\tb", "a   b"},
	}

	for _, tt := range tests {
		got := te.ExpandTabs(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandTabs(%q): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestColumnToOffset(t *testing.T) {
	te := NewTabExpander(4)

	tests := []struct {
		input  string
		visCol int
		offset int
	}{
		{"hello", 0, 0},
		{"hello", 2, 2},
		{"hello", 5, 5},
		{"hello", 10, -1}, // Beyond string
		{"\thello", 0, 0}, // Within tab
		{"\thello", 3, 0}, // Still within tab
		{"\thello", 4, 1}, // After tab
		{"\thello", 5, 2}, // 'e'
		{"a\tb", 0, 0},
		{"a\tb", 1, 1}, // Within tab
		{"a\tb", 3, 1}, // Still within tab
		{"a\tb", 4, 2}, // 'b'
	}

	for _, tt := range tests {
		got := te.ColumnToOffset(tt.input, tt.visCol)
		if got != tt.offset {
			t.Errorf("ColumnToOffset(%q, %d): expected %d, got %d",
				tt.input, tt.visCol, tt.offset, got)
		}
	}
}

func TestOffsetToColumn(t *testing.T) {
	te := NewTabExpander(4)

	tests := []struct {
		input    string
		offset   int
		expected int
	}{
		{"hello", 0, 0},
		{"hello", 2, 2},
		{"hello", 5, 5},
		{"\thello", 0, 0},
		{"\thello", 1, 4}, // After tab
		{"\thello", 2, 5},
		{"a\tb", 0, 0},
		{"a\tb", 1, 1},
		{"a\tb", 2, 4}, // After tab, at 'b'
	}

	for _, tt := range tests {
		got := te.OffsetToColumn(tt.input, tt.offset)
		if got != tt.expected {
			t.Errorf("OffsetToColumn(%q, %d): expected %d, got %d",
				tt.input, tt.offset, tt.expected, got)
		}
	}
}

func TestDefaultTabExpander(t *testing.T) {
	te := DefaultTabExpander()
	if te.TabWidth() != 4 {
		t.Errorf("default tab width should be 4, got %d", te.TabWidth())
	}
}

func TestTabExpanderWithDifferentWidths(t *testing.T) {
	widths := []int{2, 4, 8}

	for _, width := range widths {
		te := NewTabExpander(width)
		// Tab at column 0 should expand to width
		if te.NextTabStop(0) != width {
			t.Errorf("width %d: NextTabStop(0) should be %d, got %d",
				width, width, te.NextTabStop(0))
		}
		// Expanded width of single tab
		if te.ExpandedWidth("\t") != width {
			t.Errorf("width %d: single tab should expand to %d, got %d",
				width, width, te.ExpandedWidth("\t"))
		}
	}
}
