// Package gutter provides gutter rendering for the editor.
// The gutter is the area to the left of the text content that displays
// line numbers, signs (breakpoints, errors), and fold markers.
package gutter

import (
	"sync"
)

// Config holds gutter configuration.
type Config struct {
	// ShowLineNumbers enables line number display.
	ShowLineNumbers bool

	// LineNumberWidth is the fixed width for line numbers (0 = auto).
	LineNumberWidth int

	// MinLineNumberWidth is the minimum width for auto-calculated widths.
	MinLineNumberWidth int

	// ShowSigns enables the sign column (breakpoints, errors, etc.).
	ShowSigns bool

	// SignColumnWidth is the width of the sign column.
	SignColumnWidth int

	// ShowFoldMarkers enables fold markers.
	ShowFoldMarkers bool

	// RelativeLineNumbers shows line numbers relative to cursor.
	RelativeLineNumbers bool
}

// DefaultConfig returns the default gutter configuration.
func DefaultConfig() Config {
	return Config{
		ShowLineNumbers:     true,
		LineNumberWidth:     0, // Auto
		MinLineNumberWidth:  3,
		ShowSigns:           false, // Not implemented yet
		SignColumnWidth:     2,
		ShowFoldMarkers:     false, // Not implemented yet
		RelativeLineNumbers: false,
	}
}

// SignType represents the type of sign to display.
type SignType uint8

const (
	SignNone SignType = iota
	SignError
	SignWarning
	SignInfo
	SignBreakpoint
	SignBreakpointConditional
	SignBookmark
	SignGitAdded
	SignGitModified
	SignGitDeleted
)

// Sign represents a sign to display in the gutter.
type Sign struct {
	Line uint32
	Type SignType
}

// SignProvider provides signs for the gutter.
type SignProvider interface {
	// SignsForLine returns signs for a given line.
	SignsForLine(line uint32) []Sign

	// AllSigns returns all signs (for efficient batch queries).
	AllSigns() []Sign
}

// CellStyle describes how to style a gutter cell.
type CellStyle uint8

const (
	StyleNormal CellStyle = iota
	StyleCurrentLine
	StyleDim
	StyleError
	StyleWarning
	StyleInfo
	StyleGitAdd
	StyleGitModify
	StyleGitDelete
)

// Cell represents a single gutter cell.
type Cell struct {
	Rune  rune
	Style CellStyle
}

// Gutter manages the gutter area rendering.
type Gutter struct {
	mu sync.RWMutex

	config Config

	// Current state
	width       int    // Total calculated width
	lineCount   uint32 // Total lines in buffer
	currentLine uint32 // Current cursor line

	// Sign provider (optional)
	signProvider SignProvider
}

// New creates a new gutter with the given configuration.
func New(config Config) *Gutter {
	return &Gutter{
		config: config,
		width:  calculateWidth(config, 1),
	}
}

// Width returns the current gutter width.
func (g *Gutter) Width() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.width
}

// Config returns the current configuration.
func (g *Gutter) Config() Config {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config
}

// SetConfig updates the gutter configuration.
func (g *Gutter) SetConfig(config Config) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.config = config
	g.width = calculateWidth(config, g.lineCount)
}

// SetLineCount updates the total line count (affects width calculation).
func (g *Gutter) SetLineCount(count uint32) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lineCount = count
	g.width = calculateWidth(g.config, count)
}

// SetCurrentLine updates the current cursor line.
func (g *Gutter) SetCurrentLine(line uint32) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.currentLine = line
}

// SetSignProvider sets the sign provider.
func (g *Gutter) SetSignProvider(sp SignProvider) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.signProvider = sp
}

// LineNumberWidth returns just the line number width (without signs/separator).
func (g *Gutter) LineNumberWidth() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lineNumberWidth()
}

// RenderLine renders the gutter for a single line.
// isVisible indicates if the line exists in the buffer.
// Returns the cells to display.
func (g *Gutter) RenderLine(line uint32, isVisible bool) []Cell {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.width == 0 {
		return nil
	}

	cells := make([]Cell, g.width)

	// Initialize with spaces
	for i := range cells {
		cells[i] = Cell{Rune: ' ', Style: StyleNormal}
	}

	col := 0

	// Signs column (if enabled)
	if g.config.ShowSigns && g.config.SignColumnWidth > 0 {
		signCells := g.renderSigns(line)
		for i := 0; i < len(signCells) && col < g.width-1; i++ {
			cells[col] = signCells[i]
			col++
		}
	}

	// Line numbers (if enabled)
	if g.config.ShowLineNumbers && isVisible {
		numCells := g.renderLineNumber(line)
		numWidth := g.lineNumberWidth()

		// Right-align line number
		padding := numWidth - len(numCells)
		for i := 0; i < padding && col < g.width-1; i++ {
			cells[col] = Cell{Rune: ' ', Style: g.styleForLine(line)}
			col++
		}
		for i := 0; i < len(numCells) && col < g.width-1; i++ {
			cells[col] = numCells[i]
			col++
		}
	} else if g.config.ShowLineNumbers && !isVisible {
		// Show ~ for non-existent lines
		numWidth := g.lineNumberWidth()
		for i := 0; i < numWidth-1 && col < g.width-1; i++ {
			cells[col] = Cell{Rune: ' ', Style: StyleDim}
			col++
		}
		if col < g.width-1 {
			cells[col] = Cell{Rune: '~', Style: StyleDim}
			col++
		}
	}

	// Fold markers (if enabled) - placeholder for future
	if g.config.ShowFoldMarkers {
		// Will be implemented in a future phase
		_ = col // col will be used in future implementation
	}

	// Separator (last column)
	if g.width > 0 {
		cells[g.width-1] = Cell{Rune: ' ', Style: StyleNormal}
	}

	return cells
}

// styleForLine returns the appropriate style for a line number.
func (g *Gutter) styleForLine(line uint32) CellStyle {
	if line == g.currentLine {
		return StyleCurrentLine
	}
	return StyleDim
}

// renderLineNumber returns cells for a line number.
func (g *Gutter) renderLineNumber(line uint32) []Cell {
	style := g.styleForLine(line)

	var num uint32
	if g.config.RelativeLineNumbers && line != g.currentLine {
		// Show relative line number
		if line > g.currentLine {
			num = line - g.currentLine
		} else {
			num = g.currentLine - line
		}
	} else {
		// Show absolute line number (1-indexed)
		num = line + 1
	}

	numStr := FormatNumber(num)
	cells := make([]Cell, len(numStr))
	for i, r := range numStr {
		cells[i] = Cell{Rune: r, Style: style}
	}
	return cells
}

// renderSigns returns cells for the sign column.
func (g *Gutter) renderSigns(line uint32) []Cell {
	cells := make([]Cell, g.config.SignColumnWidth)
	for i := range cells {
		cells[i] = Cell{Rune: ' ', Style: StyleNormal}
	}

	if g.signProvider == nil {
		return cells
	}

	signs := g.signProvider.SignsForLine(line)
	if len(signs) == 0 {
		return cells
	}

	// Display highest priority sign
	sign := highestPriority(signs)
	r, style := signGlyph(sign.Type)
	if g.config.SignColumnWidth > 0 {
		cells[0] = Cell{Rune: r, Style: style}
	}

	return cells
}

// lineNumberWidth returns the width for line numbers.
func (g *Gutter) lineNumberWidth() int {
	if g.config.LineNumberWidth > 0 {
		return g.config.LineNumberWidth
	}

	// Auto-calculate based on line count
	digits := countDigits(g.lineCount)
	if digits < g.config.MinLineNumberWidth {
		digits = g.config.MinLineNumberWidth
	}
	return digits
}

// calculateWidth calculates the total gutter width.
func calculateWidth(config Config, lineCount uint32) int {
	width := 0

	if config.ShowSigns {
		width += config.SignColumnWidth
	}

	if config.ShowLineNumbers {
		if config.LineNumberWidth > 0 {
			width += config.LineNumberWidth
		} else {
			digits := countDigits(lineCount)
			if digits < config.MinLineNumberWidth {
				digits = config.MinLineNumberWidth
			}
			width += digits
		}
	}

	if config.ShowFoldMarkers {
		width++
	}

	// Add separator
	if width > 0 {
		width++
	}

	return width
}

// countDigits returns the number of digits needed to display a number.
func countDigits(n uint32) int {
	if n == 0 {
		return 1
	}
	digits := 0
	for n > 0 {
		digits++
		n /= 10
	}
	return digits
}

// FormatNumber converts a number to a string.
func FormatNumber(n uint32) string {
	if n == 0 {
		return "0"
	}

	var buf [10]byte // Max 10 digits for uint32
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return string(buf[i:])
}

// highestPriority returns the sign with highest priority.
func highestPriority(signs []Sign) Sign {
	if len(signs) == 0 {
		return Sign{Type: SignNone}
	}

	best := signs[0]
	for _, s := range signs[1:] {
		if signPriority(s.Type) > signPriority(best.Type) {
			best = s
		}
	}
	return best
}

// signPriority returns the priority of a sign type (higher = more important).
func signPriority(st SignType) int {
	switch st {
	case SignError:
		return 100
	case SignBreakpoint:
		return 90
	case SignBreakpointConditional:
		return 85
	case SignWarning:
		return 80
	case SignInfo:
		return 70
	case SignBookmark:
		return 60
	case SignGitDeleted:
		return 50
	case SignGitModified:
		return 40
	case SignGitAdded:
		return 30
	default:
		return 0
	}
}

// signGlyph returns the glyph and style for a sign type.
func signGlyph(st SignType) (rune, CellStyle) {
	switch st {
	case SignError:
		return 'E', StyleError
	case SignWarning:
		return 'W', StyleWarning
	case SignInfo:
		return 'I', StyleInfo
	case SignBreakpoint:
		return '*', StyleError
	case SignBreakpointConditional:
		return '?', StyleError
	case SignBookmark:
		return '#', StyleInfo
	case SignGitAdded:
		return '+', StyleGitAdd
	case SignGitModified:
		return '~', StyleGitModify
	case SignGitDeleted:
		return '-', StyleGitDelete
	default:
		return ' ', StyleNormal
	}
}
