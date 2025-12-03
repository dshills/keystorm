package renderer

// Cell represents a single terminal cell.
type Cell struct {
	// Rune is the character to display.
	// A value of 0 indicates a continuation cell (for wide characters).
	Rune rune

	// Width is the display width of this cell.
	// 0 for continuation cells, 1 for normal chars, 2 for wide CJK chars.
	Width int

	// Style is the visual style for this cell.
	Style Style
}

// EmptyCell returns an empty cell with default style.
func EmptyCell() Cell {
	return Cell{
		Rune:  ' ',
		Width: 1,
		Style: DefaultStyle(),
	}
}

// NewCell creates a cell with the given rune and default style.
func NewCell(r rune) Cell {
	return Cell{
		Rune:  r,
		Width: RuneWidth(r),
		Style: DefaultStyle(),
	}
}

// NewStyledCell creates a cell with the given rune and style.
func NewStyledCell(r rune, style Style) Cell {
	return Cell{
		Rune:  r,
		Width: RuneWidth(r),
		Style: style,
	}
}

// WithStyle returns a new cell with the given style.
func (c Cell) WithStyle(style Style) Cell {
	c.Style = style
	return c
}

// WithRune returns a new cell with the given rune.
func (c Cell) WithRune(r rune) Cell {
	c.Rune = r
	c.Width = RuneWidth(r)
	return c
}

// IsEmpty returns true if this is an empty (space) cell.
func (c Cell) IsEmpty() bool {
	return c.Rune == ' ' || c.Rune == 0
}

// IsContinuation returns true if this is a continuation cell
// (second cell of a wide character).
func (c Cell) IsContinuation() bool {
	return c.Width == 0 && c.Rune == 0
}

// Equals returns true if two cells are identical.
func (c Cell) Equals(other Cell) bool {
	return c.Rune == other.Rune &&
		c.Width == other.Width &&
		c.Style.Equals(other.Style)
}

// ContinuationCell returns a continuation cell for wide characters.
func ContinuationCell() Cell {
	return Cell{
		Rune:  0,
		Width: 0,
		Style: DefaultStyle(),
	}
}

// RuneWidth returns the display width of a rune.
// Returns 0 for control characters, 1 for normal characters,
// and 2 for wide (CJK) characters.
func RuneWidth(r rune) int {
	// Control characters have zero width
	if r < 32 || r == 0x7F {
		return 0
	}

	// Check for wide characters (simplified East Asian Width)
	// This is a simplified version - for production, use a proper
	// Unicode width library like github.com/mattn/go-runewidth
	if isWideRune(r) {
		return 2
	}

	return 1
}

// isWideRune checks if a rune is a wide (double-width) character.
// This is a simplified implementation covering common CJK ranges.
func isWideRune(r rune) bool {
	// Hangul Jamo
	if r >= 0x1100 && r <= 0x115F {
		return true
	}
	// Hangul Compatibility Jamo
	if r >= 0x3130 && r <= 0x318F {
		return true
	}
	// CJK Unified Ideographs and related
	if r >= 0x2E80 && r <= 0x9FFF {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7A3 {
		return true
	}
	// CJK Compatibility Ideographs
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	// Vertical forms
	if r >= 0xFE10 && r <= 0xFE1F {
		return true
	}
	// CJK Compatibility Forms
	if r >= 0xFE30 && r <= 0xFE6F {
		return true
	}
	// Fullwidth Forms
	if r >= 0xFF00 && r <= 0xFF60 {
		return true
	}
	// Fullwidth symbol variants
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return true
	}
	// CJK Unified Ideographs Extension B and beyond
	if r >= 0x20000 && r <= 0x2FFFF {
		return true
	}
	// CJK Compatibility Ideographs Supplement
	if r >= 0x2F800 && r <= 0x2FA1F {
		return true
	}

	return false
}

// CellsFromString creates cells from a string.
// Does not handle tabs - use the layout engine for that.
func CellsFromString(s string, style Style) []Cell {
	cells := make([]Cell, 0, len(s))

	for _, r := range s {
		width := RuneWidth(r)
		cells = append(cells, Cell{
			Rune:  r,
			Width: width,
			Style: style,
		})

		// Add continuation cell for wide characters
		if width == 2 {
			cells = append(cells, ContinuationCell())
		}
	}

	return cells
}

// StringFromCells converts cells back to a string.
// Skips continuation cells.
func StringFromCells(cells []Cell) string {
	runes := make([]rune, 0, len(cells))
	for _, c := range cells {
		if !c.IsContinuation() && c.Rune != 0 {
			runes = append(runes, c.Rune)
		}
	}
	return string(runes)
}
