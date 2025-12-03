package renderer

import "github.com/dshills/keystorm/internal/renderer/core"

// Cell represents a single terminal cell.
// Re-exported from core package.
type Cell = core.Cell

// EmptyCell returns an empty cell with default style.
func EmptyCell() Cell {
	return core.EmptyCell()
}

// NewCell creates a cell with the given rune and default style.
func NewCell(r rune) Cell {
	return core.NewCell(r)
}

// NewStyledCell creates a cell with the given rune and style.
func NewStyledCell(r rune, style Style) Cell {
	return core.NewStyledCell(r, style)
}

// ContinuationCell returns a continuation cell for wide characters.
func ContinuationCell() Cell {
	return core.ContinuationCell()
}

// RuneWidth returns the display width of a rune.
func RuneWidth(r rune) int {
	return core.RuneWidth(r)
}

// CellsFromString creates cells from a string.
func CellsFromString(s string, style Style) []Cell {
	return core.CellsFromString(s, style)
}

// StringFromCells converts cells back to a string.
func StringFromCells(cells []Cell) string {
	return core.StringFromCells(cells)
}
