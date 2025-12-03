package gutter

// LineNumberMode defines how line numbers are displayed.
type LineNumberMode uint8

const (
	// LineNumberAbsolute shows absolute line numbers (1, 2, 3, ...).
	LineNumberAbsolute LineNumberMode = iota

	// LineNumberRelative shows relative line numbers from cursor.
	LineNumberRelative

	// LineNumberHybrid shows absolute for current line, relative for others.
	LineNumberHybrid
)

// LineNumberFormatter formats line numbers according to configuration.
type LineNumberFormatter struct {
	mode        LineNumberMode
	width       int
	currentLine uint32
}

// NewLineNumberFormatter creates a new line number formatter.
func NewLineNumberFormatter(mode LineNumberMode, width int) *LineNumberFormatter {
	return &LineNumberFormatter{
		mode:  mode,
		width: width,
	}
}

// SetMode changes the line number mode.
func (f *LineNumberFormatter) SetMode(mode LineNumberMode) {
	f.mode = mode
}

// SetWidth sets the display width for line numbers.
func (f *LineNumberFormatter) SetWidth(width int) {
	f.width = width
}

// SetCurrentLine sets the current cursor line for relative calculations.
func (f *LineNumberFormatter) SetCurrentLine(line uint32) {
	f.currentLine = line
}

// Format returns the formatted line number string.
func (f *LineNumberFormatter) Format(line uint32) string {
	num := f.calculateNumber(line)
	s := FormatNumber(num)
	return PadLeft(s, f.width)
}

// FormatWithHighlight returns the formatted number and whether it should be highlighted.
func (f *LineNumberFormatter) FormatWithHighlight(line uint32) (string, bool) {
	isCurrentLine := line == f.currentLine
	num := f.calculateNumber(line)
	s := FormatNumber(num)
	return PadLeft(s, f.width), isCurrentLine
}

// calculateNumber returns the number to display for a line.
func (f *LineNumberFormatter) calculateNumber(line uint32) uint32 {
	switch f.mode {
	case LineNumberRelative:
		if line == f.currentLine {
			// Show 0 for current line in full relative mode
			return 0
		}
		return absDiff(line, f.currentLine)

	case LineNumberHybrid:
		if line == f.currentLine {
			// Show absolute number for current line
			return line + 1
		}
		return absDiff(line, f.currentLine)

	default: // LineNumberAbsolute
		return line + 1 // 1-indexed display
	}
}

// absDiff returns the absolute difference between two uint32 values.
func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// PadLeft pads a string with spaces on the left to the specified width.
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	padding := make([]byte, width-len(s))
	for i := range padding {
		padding[i] = ' '
	}
	return string(padding) + s
}

// CalculateWidth calculates the minimum width needed to display line numbers
// for the given line count.
func CalculateWidth(lineCount uint32, minWidth int) int {
	digits := countDigits(lineCount)
	if digits < minWidth {
		return minWidth
	}
	return digits
}

// WidthForLines returns the width needed for the given maximum line number.
func WidthForLines(maxLine uint32) int {
	return countDigits(maxLine)
}

// FormatLineRange formats a line range as "startLine-endLine".
func FormatLineRange(start, end uint32) string {
	if start == end {
		return FormatNumber(start + 1) // 1-indexed
	}
	return FormatNumber(start+1) + "-" + FormatNumber(end+1)
}

// FormatPosition formats a position as "line:col".
func FormatPosition(line, col uint32) string {
	return FormatNumber(line+1) + ":" + FormatNumber(col+1)
}
