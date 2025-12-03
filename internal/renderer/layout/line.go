// Package layout provides line layout computation for the renderer.
package layout

import (
	"github.com/dshills/keystorm/internal/renderer/core"
)

// LineLayout represents the visual layout of a single buffer line.
type LineLayout struct {
	// Source information
	BufferLine uint32 // The buffer line number (0-indexed)

	// Visual representation
	Cells []core.Cell // Visual cells (after tab expansion, etc.)

	// Column mappings for cursor positioning
	VisualCols []uint32 // Map visual column -> buffer column
	BufferCols []uint32 // Map buffer column -> visual column

	// Wrapping (if enabled)
	WrapPoints []int // Visual columns where line wraps
	RowCount   int   // Number of visual rows (1 if no wrap)

	// Metadata
	Width   int  // Total visual width in columns
	HasTabs bool // Contains tab characters
	HasWide bool // Contains wide (CJK) characters
}

// VisualColumn converts a buffer column to visual column.
// If bufCol is beyond the line, extrapolates from the end.
func (l *LineLayout) VisualColumn(bufCol uint32) int {
	if len(l.BufferCols) == 0 {
		return int(bufCol)
	}
	if int(bufCol) >= len(l.BufferCols) {
		// Beyond end of line - extrapolate
		lastVisCol := l.BufferCols[len(l.BufferCols)-1]
		return int(lastVisCol) + int(bufCol) - len(l.BufferCols) + 1
	}
	return int(l.BufferCols[bufCol])
}

// BufferColumn converts a visual column to buffer column.
// If visCol is beyond the line, extrapolates from the end.
func (l *LineLayout) BufferColumn(visCol int) uint32 {
	if len(l.VisualCols) == 0 {
		return uint32(visCol)
	}
	if visCol < 0 {
		return 0
	}
	if visCol >= len(l.VisualCols) {
		// Beyond end of line - extrapolate
		lastBufCol := l.VisualCols[len(l.VisualCols)-1]
		return lastBufCol + uint32(visCol-len(l.VisualCols)+1)
	}
	return l.VisualCols[visCol]
}

// VisualRow returns which visual row a visual column falls on.
// Returns 0 if no wrapping or column is on first row.
func (l *LineLayout) VisualRow(visCol int) int {
	if len(l.WrapPoints) == 0 {
		return 0
	}
	row := 0
	for _, wp := range l.WrapPoints {
		if visCol >= wp {
			row++
		} else {
			break
		}
	}
	return row
}

// ColumnInRow returns the column offset within a wrapped row.
func (l *LineLayout) ColumnInRow(visCol int) int {
	row := l.VisualRow(visCol)
	if row == 0 {
		return visCol
	}
	// Subtract the wrap point of the previous row
	return visCol - l.WrapPoints[row-1]
}

// RowStartColumn returns the visual column where a wrapped row starts.
func (l *LineLayout) RowStartColumn(row int) int {
	if row == 0 || len(l.WrapPoints) == 0 {
		return 0
	}
	if row > len(l.WrapPoints) {
		row = len(l.WrapPoints)
	}
	return l.WrapPoints[row-1]
}

// RowEndColumn returns the visual column where a wrapped row ends (exclusive).
func (l *LineLayout) RowEndColumn(row int) int {
	if len(l.WrapPoints) == 0 || row >= len(l.WrapPoints) {
		return l.Width
	}
	return l.WrapPoints[row]
}

// CellsForRow returns the cells for a specific wrapped row.
func (l *LineLayout) CellsForRow(row int) []core.Cell {
	start := l.RowStartColumn(row)
	end := l.RowEndColumn(row)
	if start >= len(l.Cells) {
		return nil
	}
	if end > len(l.Cells) {
		end = len(l.Cells)
	}
	return l.Cells[start:end]
}

// IsEmpty returns true if the layout represents an empty line.
func (l *LineLayout) IsEmpty() bool {
	return len(l.Cells) == 0
}

// LayoutEngine computes line layouts.
type LayoutEngine struct {
	tabWidth   int
	wrapWidth  int  // 0 = no wrap
	wrapAtWord bool // Try to wrap at word boundaries
}

// NewLayoutEngine creates a layout engine with the given tab width.
func NewLayoutEngine(tabWidth int) *LayoutEngine {
	if tabWidth < 1 {
		tabWidth = 4
	}
	return &LayoutEngine{
		tabWidth:   tabWidth,
		wrapWidth:  0,
		wrapAtWord: true,
	}
}

// TabWidth returns the current tab width.
func (e *LayoutEngine) TabWidth() int {
	return e.tabWidth
}

// SetTabWidth sets the tab width.
func (e *LayoutEngine) SetTabWidth(width int) {
	if width < 1 {
		width = 1
	}
	e.tabWidth = width
}

// WrapWidth returns the current wrap width (0 = no wrap).
func (e *LayoutEngine) WrapWidth() int {
	return e.wrapWidth
}

// SetWrap configures word wrapping.
// width of 0 disables wrapping.
func (e *LayoutEngine) SetWrap(width int, atWord bool) {
	if width < 0 {
		width = 0
	}
	e.wrapWidth = width
	e.wrapAtWord = atWord
}

// Layout computes the visual layout for a line.
func (e *LayoutEngine) Layout(line string, bufferLine uint32) *LineLayout {
	layout := &LineLayout{
		BufferLine: bufferLine,
		Cells:      make([]core.Cell, 0, len(line)),
		VisualCols: make([]uint32, 0, len(line)*2), // May grow for tabs/wide
		BufferCols: make([]uint32, 0, len(line)),
		RowCount:   1,
	}

	visCol := 0
	bufCol := uint32(0)
	defaultStyle := core.DefaultStyle()

	for _, r := range line {
		// Record buffer -> visual mapping at start of each character
		for uint32(len(layout.BufferCols)) <= bufCol {
			layout.BufferCols = append(layout.BufferCols, uint32(visCol))
		}

		if r == '\t' {
			// Tab expansion
			layout.HasTabs = true
			tabStop := e.tabWidth - (visCol % e.tabWidth)
			for i := 0; i < tabStop; i++ {
				layout.Cells = append(layout.Cells, core.Cell{
					Rune:  ' ',
					Width: 1,
					Style: defaultStyle,
				})
				layout.VisualCols = append(layout.VisualCols, bufCol)
				visCol++
			}
		} else {
			// Regular character
			width := core.RuneWidth(r)
			if width == 2 {
				layout.HasWide = true
			}

			if width == 0 {
				// Control character - skip visual representation but track mapping
				bufCol++
				continue
			}

			layout.Cells = append(layout.Cells, core.Cell{
				Rune:  r,
				Width: width,
				Style: defaultStyle,
			})
			layout.VisualCols = append(layout.VisualCols, bufCol)
			visCol++

			// For wide characters, add continuation cell
			if width == 2 {
				layout.Cells = append(layout.Cells, core.ContinuationCell())
				layout.VisualCols = append(layout.VisualCols, bufCol)
				visCol++
			}
		}

		bufCol++

		// Check for word wrap
		if e.wrapWidth > 0 && visCol >= e.wrapWidth {
			wrapPoint := e.findWrapPoint(layout, visCol)
			layout.WrapPoints = append(layout.WrapPoints, wrapPoint)
			layout.RowCount++
		}
	}

	layout.Width = visCol
	return layout
}

// LayoutWithStyle computes the visual layout for a line with a base style.
func (e *LayoutEngine) LayoutWithStyle(line string, bufferLine uint32, style core.Style) *LineLayout {
	layout := e.Layout(line, bufferLine)
	// Apply style to all cells
	for i := range layout.Cells {
		layout.Cells[i].Style = style
	}
	return layout
}

// findWrapPoint finds the best point to wrap (at word boundary if possible).
func (e *LayoutEngine) findWrapPoint(layout *LineLayout, currentCol int) int {
	if !e.wrapAtWord || currentCol <= 1 {
		return currentCol
	}

	// Look backward for a space (up to 20 chars)
	searchStart := currentCol - 1
	searchEnd := currentCol - 20
	if searchEnd < 0 {
		searchEnd = 0
	}

	for i := searchStart; i >= searchEnd; i-- {
		if i < len(layout.Cells) && layout.Cells[i].Rune == ' ' {
			return i + 1
		}
	}

	// No good wrap point found, wrap at column
	return currentCol
}

// ApplyStyles applies a slice of style spans to a layout.
// Spans are applied in order, so later spans override earlier ones.
func (e *LayoutEngine) ApplyStyles(layout *LineLayout, spans []core.StyleSpan) {
	for _, span := range spans {
		// Validate span
		if span.StartCol > span.EndCol {
			continue
		}

		start := int(span.StartCol)
		end := int(span.EndCol)

		// Convert buffer columns to visual columns
		if start < len(layout.BufferCols) {
			start = int(layout.BufferCols[start])
		} else {
			// Start is beyond line, nothing to apply
			start = layout.Width
		}
		if end < len(layout.BufferCols) {
			end = int(layout.BufferCols[end])
		} else {
			// End is beyond line, clamp to layout width
			end = layout.Width
		}

		// Clamp to valid cell range
		if start > len(layout.Cells) {
			start = len(layout.Cells)
		}
		if end > len(layout.Cells) {
			end = len(layout.Cells)
		}

		// Apply style to cells in range
		for i := start; i < end; i++ {
			layout.Cells[i].Style = layout.Cells[i].Style.Merge(span.Style)
		}
	}
}
