package renderer

import (
	"testing"
)

func TestEmptyCell(t *testing.T) {
	c := EmptyCell()
	if c.Rune != ' ' {
		t.Errorf("empty cell rune should be space, got %q", c.Rune)
	}
	if c.Width != 1 {
		t.Errorf("empty cell width should be 1, got %d", c.Width)
	}
	if !c.Style.IsDefault() {
		t.Error("empty cell should have default style")
	}
}

func TestNewCell(t *testing.T) {
	c := NewCell('A')
	if c.Rune != 'A' {
		t.Errorf("expected rune 'A', got %q", c.Rune)
	}
	if c.Width != 1 {
		t.Errorf("expected width 1, got %d", c.Width)
	}
}

func TestNewStyledCell(t *testing.T) {
	style := DefaultStyle().WithForeground(ColorRed)
	c := NewStyledCell('X', style)

	if c.Rune != 'X' {
		t.Errorf("expected rune 'X', got %q", c.Rune)
	}
	if !c.Style.Foreground.Equals(ColorRed) {
		t.Error("styled cell should have red foreground")
	}
}

func TestCellWithStyle(t *testing.T) {
	c := NewCell('A')
	style := DefaultStyle().WithForeground(ColorBlue)
	c2 := c.WithStyle(style)

	if c2.Rune != 'A' {
		t.Error("WithStyle should preserve rune")
	}
	if !c2.Style.Foreground.Equals(ColorBlue) {
		t.Error("WithStyle should set style")
	}
}

func TestCellWithRune(t *testing.T) {
	c := NewCell('A')
	c2 := c.WithRune('B')

	if c2.Rune != 'B' {
		t.Errorf("expected rune 'B', got %q", c2.Rune)
	}
}

func TestCellIsEmpty(t *testing.T) {
	empty := EmptyCell()
	if !empty.IsEmpty() {
		t.Error("space cell should be empty")
	}

	cell := NewCell('A')
	if cell.IsEmpty() {
		t.Error("'A' cell should not be empty")
	}

	nullCell := Cell{Rune: 0}
	if !nullCell.IsEmpty() {
		t.Error("null rune cell should be empty")
	}
}

func TestCellIsContinuation(t *testing.T) {
	cont := ContinuationCell()
	if !cont.IsContinuation() {
		t.Error("continuation cell should return true")
	}

	normal := NewCell('A')
	if normal.IsContinuation() {
		t.Error("normal cell should not be continuation")
	}
}

func TestCellEquals(t *testing.T) {
	c1 := NewStyledCell('A', DefaultStyle().WithForeground(ColorRed))
	c2 := NewStyledCell('A', DefaultStyle().WithForeground(ColorRed))
	c3 := NewStyledCell('A', DefaultStyle().WithForeground(ColorBlue))
	c4 := NewStyledCell('B', DefaultStyle().WithForeground(ColorRed))

	if !c1.Equals(c2) {
		t.Error("identical cells should be equal")
	}
	if c1.Equals(c3) {
		t.Error("cells with different styles should not be equal")
	}
	if c1.Equals(c4) {
		t.Error("cells with different runes should not be equal")
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		r     rune
		width int
	}{
		{'A', 1},
		{'a', 1},
		{'1', 1},
		{' ', 1},
		{'@', 1},
		{'\t', 0}, // control character
		{'\n', 0}, // control character
		{0x7F, 0}, // DEL control character
		{'中', 2},  // CJK
		{'日', 2},  // CJK
		{'한', 2},  // Hangul
		{'あ', 2},  // Hiragana (in CJK range)
		{'Ａ', 2},  // Fullwidth A
		{'ａ', 2},  // Fullwidth a
		{'é', 1},  // Latin with accent
		{'α', 1},  // Greek
		{'→', 1},  // Arrow
	}

	for _, tt := range tests {
		got := RuneWidth(tt.r)
		if got != tt.width {
			t.Errorf("RuneWidth(%q): expected %d, got %d", tt.r, tt.width, got)
		}
	}
}

func TestCellsFromString(t *testing.T) {
	style := DefaultStyle().WithForeground(ColorGreen)

	// ASCII string
	cells := CellsFromString("Hello", style)
	if len(cells) != 5 {
		t.Errorf("expected 5 cells, got %d", len(cells))
	}
	for i, c := range cells {
		expected := []rune("Hello")[i]
		if c.Rune != expected {
			t.Errorf("cell %d: expected %q, got %q", i, expected, c.Rune)
		}
		if !c.Style.Foreground.Equals(ColorGreen) {
			t.Errorf("cell %d: expected green foreground", i)
		}
	}

	// String with wide characters
	cells = CellsFromString("Hi中文", style)
	// "Hi" = 2 cells, "中" = 2 cells (1 + continuation), "文" = 2 cells (1 + continuation)
	// Total = 2 + 2 + 2 = 6 cells
	if len(cells) != 6 {
		t.Errorf("expected 6 cells for 'Hi中文', got %d", len(cells))
	}

	// Check continuation cells
	if !cells[3].IsContinuation() {
		t.Error("cell after wide char should be continuation")
	}
	if !cells[5].IsContinuation() {
		t.Error("cell after second wide char should be continuation")
	}
}

func TestStringFromCells(t *testing.T) {
	style := DefaultStyle()

	// Round-trip ASCII
	original := "Hello World"
	cells := CellsFromString(original, style)
	result := StringFromCells(cells)
	if result != original {
		t.Errorf("expected %q, got %q", original, result)
	}

	// Round-trip with wide characters
	original = "Hi中文"
	cells = CellsFromString(original, style)
	result = StringFromCells(cells)
	if result != original {
		t.Errorf("expected %q, got %q", original, result)
	}
}

func TestContinuationCell(t *testing.T) {
	cont := ContinuationCell()

	if cont.Rune != 0 {
		t.Errorf("continuation cell rune should be 0, got %q", cont.Rune)
	}
	if cont.Width != 0 {
		t.Errorf("continuation cell width should be 0, got %d", cont.Width)
	}
}

func TestWideRuneWidth(t *testing.T) {
	wideRunes := []rune{'中', '日', '韓', '한', 'あ', 'ア', 'Ａ', 'ａ', '１'}
	for _, r := range wideRunes {
		if RuneWidth(r) != 2 {
			t.Errorf("expected %q to be wide (width 2), got %d", r, RuneWidth(r))
		}
	}

	narrowRunes := []rune{'A', 'z', '1', ' ', '@', 'α', 'é'}
	for _, r := range narrowRunes {
		if RuneWidth(r) != 1 {
			t.Errorf("expected %q to be narrow (width 1), got %d", r, RuneWidth(r))
		}
	}
}
