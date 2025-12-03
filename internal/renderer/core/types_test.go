package core

import (
	"testing"
)

func TestColorDefault(t *testing.T) {
	c := ColorDefault
	if !c.IsDefault() {
		t.Error("ColorDefault should be default")
	}
}

func TestColorFromRGB(t *testing.T) {
	c := ColorFromRGB(255, 128, 64)

	if c.R != 255 {
		t.Errorf("expected R 255, got %d", c.R)
	}
	if c.G != 128 {
		t.Errorf("expected G 128, got %d", c.G)
	}
	if c.B != 64 {
		t.Errorf("expected B 64, got %d", c.B)
	}
	if c.Indexed {
		t.Error("RGB color should not be indexed")
	}
	if c.IsDefault() {
		t.Error("RGB color should not be default")
	}
}

func TestColorFromIndex(t *testing.T) {
	c := ColorFromIndex(42)

	if c.R != 42 {
		t.Errorf("expected index 42, got %d", c.R)
	}
	if !c.Indexed {
		t.Error("indexed color should have Indexed true")
	}
	if c.IsDefault() {
		t.Error("indexed color should not be default")
	}
}

func TestColorFromHex(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b uint8
		wantErr bool
	}{
		{"#FF8040", 255, 128, 64, false},
		{"#ff8040", 255, 128, 64, false},
		{"FF8040", 255, 128, 64, false},
		{"#FFF", 255, 255, 255, false}, // Short form
		{"#000", 0, 0, 0, false},
		{"invalid", 0, 0, 0, true},
		{"#GGG", 0, 0, 0, true},
	}

	for _, tt := range tests {
		c, err := ColorFromHex(tt.hex)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ColorFromHex(%q) expected error, got nil", tt.hex)
			}
			continue
		}
		if err != nil {
			t.Errorf("ColorFromHex(%q) unexpected error: %v", tt.hex, err)
			continue
		}
		if c.R != tt.r || c.G != tt.g || c.B != tt.b {
			t.Errorf("ColorFromHex(%q) = (%d,%d,%d), want (%d,%d,%d)",
				tt.hex, c.R, c.G, c.B, tt.r, tt.g, tt.b)
		}
	}
}

func TestColorEquals(t *testing.T) {
	c1 := ColorFromRGB(255, 128, 64)
	c2 := ColorFromRGB(255, 128, 64)
	c3 := ColorFromRGB(255, 128, 65)
	c4 := ColorFromIndex(10)
	c5 := ColorFromIndex(10)

	if !c1.Equals(c2) {
		t.Error("identical RGB colors should be equal")
	}
	if c1.Equals(c3) {
		t.Error("different RGB colors should not be equal")
	}
	if !c4.Equals(c5) {
		t.Error("identical indexed colors should be equal")
	}
	if c1.Equals(c4) {
		t.Error("RGB and indexed colors should not be equal")
	}
}

func TestPredefinedColors(t *testing.T) {
	colors := []Color{
		ColorBlack, ColorWhite, ColorRed, ColorGreen,
		ColorBlue, ColorYellow, ColorCyan, ColorMagenta, ColorGray,
	}

	for _, c := range colors {
		if c.IsDefault() {
			t.Errorf("predefined color should not be default: %+v", c)
		}
		if c.Indexed {
			t.Errorf("predefined color should be RGB: %+v", c)
		}
	}
}

func TestAttributeHas(t *testing.T) {
	a := AttrBold | AttrItalic

	if !a.Has(AttrBold) {
		t.Error("should have Bold")
	}
	if !a.Has(AttrItalic) {
		t.Error("should have Italic")
	}
	if a.Has(AttrUnderline) {
		t.Error("should not have Underline")
	}
}

func TestAttributeWith(t *testing.T) {
	a := AttrNone.With(AttrBold)
	if !a.Has(AttrBold) {
		t.Error("With should add attribute")
	}

	a = a.With(AttrItalic)
	if !a.Has(AttrBold) || !a.Has(AttrItalic) {
		t.Error("With should preserve existing attributes")
	}
}

func TestAttributeWithout(t *testing.T) {
	a := AttrBold | AttrItalic
	a = a.Without(AttrBold)

	if a.Has(AttrBold) {
		t.Error("Without should remove attribute")
	}
	if !a.Has(AttrItalic) {
		t.Error("Without should preserve other attributes")
	}
}

func TestDefaultStyle(t *testing.T) {
	s := DefaultStyle()

	if !s.Foreground.IsDefault() {
		t.Error("default style foreground should be default")
	}
	if !s.Background.IsDefault() {
		t.Error("default style background should be default")
	}
	if s.Attributes != AttrNone {
		t.Error("default style should have no attributes")
	}
}

func TestNewStyle(t *testing.T) {
	s := NewStyle(ColorRed)

	if !s.Foreground.Equals(ColorRed) {
		t.Error("NewStyle should set foreground")
	}
	if !s.Background.IsDefault() {
		t.Error("NewStyle should leave background default")
	}
}

func TestStyleWithForeground(t *testing.T) {
	s := DefaultStyle().WithForeground(ColorBlue)

	if !s.Foreground.Equals(ColorBlue) {
		t.Error("WithForeground should set foreground")
	}
}

func TestStyleWithBackground(t *testing.T) {
	s := DefaultStyle().WithBackground(ColorGreen)

	if !s.Background.Equals(ColorGreen) {
		t.Error("WithBackground should set background")
	}
}

func TestStyleAttributes(t *testing.T) {
	s := DefaultStyle().Bold().Italic().Underline()

	if !s.Attributes.Has(AttrBold) {
		t.Error("should have Bold")
	}
	if !s.Attributes.Has(AttrItalic) {
		t.Error("should have Italic")
	}
	if !s.Attributes.Has(AttrUnderline) {
		t.Error("should have Underline")
	}
}

func TestStyleDim(t *testing.T) {
	s := DefaultStyle().Dim()
	if !s.Attributes.Has(AttrDim) {
		t.Error("should have Dim")
	}
}

func TestStyleBlink(t *testing.T) {
	s := DefaultStyle().WithAttributes(AttrBlink)
	if !s.Attributes.Has(AttrBlink) {
		t.Error("should have Blink")
	}
}

func TestStyleReverse(t *testing.T) {
	s := DefaultStyle().Reverse()
	if !s.Attributes.Has(AttrReverse) {
		t.Error("should have Reverse")
	}
}

func TestStyleStrikethrough(t *testing.T) {
	s := DefaultStyle().Strikethrough()
	if !s.Attributes.Has(AttrStrikethrough) {
		t.Error("should have Strikethrough")
	}
}

func TestStyleHidden(t *testing.T) {
	s := DefaultStyle().WithAttributes(AttrHidden)
	if !s.Attributes.Has(AttrHidden) {
		t.Error("should have Hidden")
	}
}

func TestStyleMerge(t *testing.T) {
	s1 := DefaultStyle().WithForeground(ColorRed).Bold()
	s2 := DefaultStyle().WithBackground(ColorBlue).Italic()

	merged := s1.Merge(s2)

	// s2's background should override s1's default
	if !merged.Background.Equals(ColorBlue) {
		t.Error("merge should apply s2's background")
	}
	// s2's foreground is default, so s1's should remain
	if !merged.Foreground.Equals(ColorRed) {
		t.Error("merge should preserve s1's non-default foreground")
	}
	// Attributes should be combined
	if !merged.Attributes.Has(AttrBold) || !merged.Attributes.Has(AttrItalic) {
		t.Error("merge should combine attributes")
	}
}

func TestEmptyCell(t *testing.T) {
	c := EmptyCell()

	if c.Rune != ' ' {
		t.Errorf("empty cell rune should be space, got %q", c.Rune)
	}
	if c.Width != 1 {
		t.Errorf("empty cell width should be 1, got %d", c.Width)
	}
	if !c.Style.Foreground.IsDefault() {
		t.Error("empty cell should have default style")
	}
}

func TestNewCell(t *testing.T) {
	c := NewCell('X')

	if c.Rune != 'X' {
		t.Errorf("expected rune 'X', got %q", c.Rune)
	}
	if c.Width != 1 {
		t.Errorf("expected width 1, got %d", c.Width)
	}
}

func TestNewStyledCell(t *testing.T) {
	style := DefaultStyle().WithForeground(ColorRed)
	c := NewStyledCell('A', style)

	if c.Rune != 'A' {
		t.Errorf("expected rune 'A', got %q", c.Rune)
	}
	if !c.Style.Foreground.Equals(ColorRed) {
		t.Error("styled cell should have red foreground")
	}
}

func TestContinuationCell(t *testing.T) {
	c := ContinuationCell()

	if c.Rune != 0 {
		t.Errorf("continuation cell rune should be 0, got %q", c.Rune)
	}
	if c.Width != 0 {
		t.Errorf("continuation cell width should be 0, got %d", c.Width)
	}
	if !c.IsContinuation() {
		t.Error("IsContinuation should return true")
	}
}

func TestCellEquals(t *testing.T) {
	c1 := NewCell('A')
	c2 := NewCell('A')
	c3 := NewCell('B')

	if !c1.Equals(c2) {
		t.Error("identical cells should be equal")
	}
	if c1.Equals(c3) {
		t.Error("different cells should not be equal")
	}
}

func TestCellWithRune(t *testing.T) {
	c := NewCell('A')
	c2 := c.WithRune('B')

	if c2.Rune != 'B' {
		t.Errorf("expected rune 'B', got %q", c2.Rune)
	}
	// Original should be unchanged
	if c.Rune != 'A' {
		t.Error("original cell should be unchanged")
	}
}

func TestCellWithStyle(t *testing.T) {
	c := NewCell('A')
	style := DefaultStyle().WithForeground(ColorGreen)
	c2 := c.WithStyle(style)

	if !c2.Style.Foreground.Equals(ColorGreen) {
		t.Error("cell should have green foreground")
	}
	// Original should be unchanged
	if !c.Style.Foreground.IsDefault() {
		t.Error("original cell should be unchanged")
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		r     rune
		width int
	}{
		{'A', 1},
		{'a', 1},
		{'0', 1},
		{' ', 1},
		{'中', 2},
		{'日', 2},
		{'あ', 2},
		{'\t', 0}, // Tab is a control character, display width handled by layout
		{'\n', 0},
		{'\x00', 0},
	}

	for _, tt := range tests {
		got := RuneWidth(tt.r)
		if got != tt.width {
			t.Errorf("RuneWidth(%q) = %d, want %d", tt.r, got, tt.width)
		}
	}
}

func TestCellsFromString(t *testing.T) {
	style := DefaultStyle()
	cells := CellsFromString("Hello", style)

	if len(cells) != 5 {
		t.Errorf("expected 5 cells, got %d", len(cells))
	}
	if cells[0].Rune != 'H' {
		t.Errorf("expected first cell 'H', got %q", cells[0].Rune)
	}
}

func TestCellsFromStringWithWide(t *testing.T) {
	style := DefaultStyle()
	cells := CellsFromString("A中B", style)

	// A(1) + 中(2) + B(1) = 4 cells
	if len(cells) != 4 {
		t.Errorf("expected 4 cells, got %d", len(cells))
	}
	if cells[0].Rune != 'A' {
		t.Error("first cell should be 'A'")
	}
	if cells[1].Rune != '中' {
		t.Error("second cell should be '中'")
	}
	if !cells[2].IsContinuation() {
		t.Error("third cell should be continuation")
	}
	if cells[3].Rune != 'B' {
		t.Error("fourth cell should be 'B'")
	}
}

func TestStringFromCells(t *testing.T) {
	cells := []Cell{
		NewCell('H'),
		NewCell('i'),
		NewCell('!'),
	}

	s := StringFromCells(cells)
	if s != "Hi!" {
		t.Errorf("expected 'Hi!', got %q", s)
	}
}

func TestStringFromCellsWithContinuation(t *testing.T) {
	cells := []Cell{
		NewCell('A'),
		{Rune: '中', Width: 2, Style: DefaultStyle()},
		ContinuationCell(),
		NewCell('B'),
	}

	s := StringFromCells(cells)
	if s != "A中B" {
		t.Errorf("expected 'A中B', got %q", s)
	}
}

func TestNewScreenPos(t *testing.T) {
	p := NewScreenPos(10, 20)

	if p.Row != 10 {
		t.Errorf("expected row 10, got %d", p.Row)
	}
	if p.Col != 20 {
		t.Errorf("expected col 20, got %d", p.Col)
	}
}

func TestNewScreenRect(t *testing.T) {
	r := NewScreenRect(5, 10, 15, 30)

	if r.Top != 5 {
		t.Errorf("expected top 5, got %d", r.Top)
	}
	if r.Left != 10 {
		t.Errorf("expected left 10, got %d", r.Left)
	}
	if r.Bottom != 15 {
		t.Errorf("expected bottom 15, got %d", r.Bottom)
	}
	if r.Right != 30 {
		t.Errorf("expected right 30, got %d", r.Right)
	}
}

func TestRectFromSize(t *testing.T) {
	r := RectFromSize(5, 10, 20, 40)

	if r.Top != 5 {
		t.Errorf("expected top 5, got %d", r.Top)
	}
	if r.Left != 10 {
		t.Errorf("expected left 10, got %d", r.Left)
	}
	if r.Bottom != 25 { // top + height
		t.Errorf("expected bottom 25, got %d", r.Bottom)
	}
	if r.Right != 50 { // left + width
		t.Errorf("expected right 50, got %d", r.Right)
	}
}

func TestScreenRectWidth(t *testing.T) {
	r := NewScreenRect(0, 10, 20, 50)

	if r.Width() != 40 {
		t.Errorf("expected width 40, got %d", r.Width())
	}
}

func TestScreenRectHeight(t *testing.T) {
	r := NewScreenRect(5, 0, 25, 80)

	if r.Height() != 20 {
		t.Errorf("expected height 20, got %d", r.Height())
	}
}

func TestScreenRectContains(t *testing.T) {
	r := NewScreenRect(10, 20, 30, 50)

	tests := []struct {
		row, col int
		want     bool
	}{
		{15, 25, true},  // Inside
		{10, 20, true},  // Top-left corner
		{29, 49, true},  // Just inside bottom-right
		{5, 25, false},  // Above
		{35, 25, false}, // Below
		{15, 15, false}, // Left
		{15, 55, false}, // Right
		{30, 50, false}, // Exactly at bottom-right (exclusive)
	}

	for _, tt := range tests {
		got := r.Contains(NewScreenPos(tt.row, tt.col))
		if got != tt.want {
			t.Errorf("Contains(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.want)
		}
	}
}

func TestScreenRectIntersects(t *testing.T) {
	r1 := NewScreenRect(0, 0, 20, 40)
	r2 := NewScreenRect(10, 30, 30, 50)
	r3 := NewScreenRect(25, 45, 35, 55)

	if !r1.Intersects(r2) {
		t.Error("r1 and r2 should intersect")
	}
	if r1.Intersects(r3) {
		t.Error("r1 and r3 should not intersect")
	}
}

func TestScreenRectIntersection(t *testing.T) {
	r1 := NewScreenRect(0, 0, 20, 40)
	r2 := NewScreenRect(10, 30, 30, 50)

	r3 := r1.Intersection(r2)

	if r3.Top != 10 {
		t.Errorf("expected intersection top 10, got %d", r3.Top)
	}
	if r3.Left != 30 {
		t.Errorf("expected intersection left 30, got %d", r3.Left)
	}
	if r3.Bottom != 20 {
		t.Errorf("expected intersection bottom 20, got %d", r3.Bottom)
	}
	if r3.Right != 40 {
		t.Errorf("expected intersection right 40, got %d", r3.Right)
	}
}

func TestStyleSpan(t *testing.T) {
	span := StyleSpan{
		StartCol: 0,
		EndCol:   10,
		Style:    DefaultStyle().WithForeground(ColorRed),
	}

	if span.StartCol != 0 {
		t.Errorf("expected StartCol 0, got %d", span.StartCol)
	}
	if span.EndCol != 10 {
		t.Errorf("expected EndCol 10, got %d", span.EndCol)
	}
	if !span.Style.Foreground.Equals(ColorRed) {
		t.Error("span style should have red foreground")
	}
}
