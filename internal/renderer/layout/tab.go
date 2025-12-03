package layout

import "unicode/utf8"

// TabExpander provides tab expansion utilities.
type TabExpander struct {
	tabWidth int
}

// NewTabExpander creates a tab expander with the given tab width.
func NewTabExpander(tabWidth int) *TabExpander {
	if tabWidth < 1 {
		tabWidth = 4
	}
	return &TabExpander{tabWidth: tabWidth}
}

// TabWidth returns the current tab width.
func (t *TabExpander) TabWidth() int {
	return t.tabWidth
}

// SetTabWidth sets the tab width.
func (t *TabExpander) SetTabWidth(width int) {
	if width < 1 {
		width = 1
	}
	t.tabWidth = width
}

// NextTabStop returns the next tab stop column after the given column.
func (t *TabExpander) NextTabStop(col int) int {
	return col + t.TabWidth() - (col % t.TabWidth())
}

// TabStopOffset returns how many spaces a tab at the given column expands to.
func (t *TabExpander) TabStopOffset(col int) int {
	return t.TabWidth() - (col % t.TabWidth())
}

// IsTabStop returns true if the given column is a tab stop.
func (t *TabExpander) IsTabStop(col int) bool {
	return col%t.TabWidth() == 0
}

// PrevTabStop returns the previous tab stop column before the given column.
// Returns 0 if already at or before the first tab stop.
func (t *TabExpander) PrevTabStop(col int) int {
	if col <= 0 {
		return 0
	}
	// If exactly on a tab stop, go to previous one
	if col%t.TabWidth() == 0 {
		return col - t.TabWidth()
	}
	// Otherwise, go to the most recent tab stop
	return (col / t.TabWidth()) * t.TabWidth()
}

// ExpandedWidth calculates the visual width of a string with tab expansion.
func (t *TabExpander) ExpandedWidth(s string) int {
	col := 0
	for _, r := range s {
		if r == '\t' {
			col = t.NextTabStop(col)
		} else {
			col++
		}
	}
	return col
}

// ExpandTabs returns a string with tabs replaced by spaces.
// Useful for copying text to clipboard or display without tab support.
func (t *TabExpander) ExpandTabs(s string) string {
	result := make([]rune, 0, len(s)*2)
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := t.TabStopOffset(col)
			for i := 0; i < spaces; i++ {
				result = append(result, ' ')
			}
			col += spaces
		} else {
			result = append(result, r)
			col++
		}
	}
	return string(result)
}

// ColumnToOffset converts a visual column to a byte offset in the string.
// Accounts for tab expansion. Returns -1 if the column is beyond the string.
func (t *TabExpander) ColumnToOffset(s string, visualCol int) int {
	col := 0
	offset := 0
	for _, r := range s {
		if col >= visualCol {
			return offset
		}
		if r == '\t' {
			nextCol := t.NextTabStop(col)
			if visualCol < nextCol {
				// Within the tab expansion
				return offset
			}
			col = nextCol
		} else {
			col++
		}
		offset += utf8.RuneLen(r)
	}
	if col >= visualCol {
		return offset
	}
	return -1
}

// OffsetToColumn converts a byte offset to a visual column.
// Accounts for tab expansion.
func (t *TabExpander) OffsetToColumn(s string, byteOffset int) int {
	col := 0
	offset := 0
	for _, r := range s {
		if offset >= byteOffset {
			return col
		}
		if r == '\t' {
			col = t.NextTabStop(col)
		} else {
			col++
		}
		offset += utf8.RuneLen(r)
	}
	return col
}

// DefaultTabExpander returns a tab expander with the default tab width of 4.
func DefaultTabExpander() *TabExpander {
	return NewTabExpander(4)
}
