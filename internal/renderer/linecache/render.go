package linecache

import (
	"math"

	"github.com/dshills/keystorm/internal/renderer/core"
	"github.com/dshills/keystorm/internal/renderer/dirty"
	"github.com/dshills/keystorm/internal/renderer/style"
)

// RenderLine represents a line ready for rendering to the backend.
type RenderLine struct {
	// Line is the buffer line number.
	Line uint32

	// ScreenRow is the screen row to render at.
	ScreenRow int

	// Cells contains the styled cells to render.
	Cells []core.Cell

	// GutterCells contains the gutter cells (line numbers, signs).
	GutterCells []core.Cell

	// IsDirty indicates if this line needs to be redrawn.
	IsDirty bool

	// HasOverlay indicates if the line has overlay content.
	HasOverlay bool

	// AfterContent contains cells to append after the line content.
	AfterContent []core.Cell
}

// LineRenderer renders lines using the cache and style resolver.
type LineRenderer struct {
	cache         *Cache
	styleResolver *style.Resolver
	dirtyTracker  *dirty.Tracker

	// Default styles
	baseStyle      core.Style
	gutterStyle    core.Style
	selectionStyle core.Style

	// Screen dimensions
	screenWidth  int
	screenHeight int
	gutterWidth  int

	// Viewport state
	topLine    uint32
	leftColumn int
}

// NewLineRenderer creates a new line renderer.
func NewLineRenderer(cache *Cache) *LineRenderer {
	return &LineRenderer{
		cache:          cache,
		styleResolver:  cache.StyleResolver(),
		baseStyle:      core.DefaultStyle(),
		gutterStyle:    core.DefaultStyle().Dim(),
		selectionStyle: core.NewStyle(core.ColorDefault).WithBackground(core.ColorFromRGB(60, 90, 130)),
		screenWidth:    80,
		screenHeight:   24,
		gutterWidth:    4,
	}
}

// SetDirtyTracker sets the dirty tracker.
func (lr *LineRenderer) SetDirtyTracker(tracker *dirty.Tracker) {
	lr.dirtyTracker = tracker
}

// SetScreenSize sets the screen dimensions.
func (lr *LineRenderer) SetScreenSize(width, height int) {
	lr.screenWidth = width
	lr.screenHeight = height
}

// SetGutterWidth sets the gutter width.
func (lr *LineRenderer) SetGutterWidth(width int) {
	lr.gutterWidth = width
}

// SetViewport sets the viewport position.
func (lr *LineRenderer) SetViewport(topLine uint32, leftColumn int) {
	lr.topLine = topLine
	lr.leftColumn = leftColumn
}

// SetBaseStyle sets the base style for rendering.
func (lr *LineRenderer) SetBaseStyle(s core.Style) {
	lr.baseStyle = s
	lr.styleResolver.SetBaseStyle(s)
}

// SetGutterStyle sets the gutter style.
func (lr *LineRenderer) SetGutterStyle(s core.Style) {
	lr.gutterStyle = s
}

// SetSelectionStyle sets the selection highlight style.
func (lr *LineRenderer) SetSelectionStyle(s core.Style) {
	lr.selectionStyle = s
}

// RenderVisibleLines renders all visible lines.
// getText provides the text content for each line.
func (lr *LineRenderer) RenderVisibleLines(getText func(line uint32) string) []RenderLine {
	result := make([]RenderLine, 0, lr.screenHeight)

	for screenRow := 0; screenRow < lr.screenHeight; screenRow++ {
		// Guard against uint32 overflow
		var line uint32
		if lr.topLine > math.MaxUint32-uint32(screenRow) {
			line = math.MaxUint32
		} else {
			line = lr.topLine + uint32(screenRow)
		}
		text := getText(line)

		renderLine := lr.renderLine(line, screenRow, text)
		result = append(result, renderLine)

		// If we've reached max line, stop
		if line == math.MaxUint32 {
			break
		}
	}

	return result
}

// RenderDirtyLines renders only the dirty lines.
func (lr *LineRenderer) RenderDirtyLines(getText func(line uint32) string) []RenderLine {
	if lr.dirtyTracker == nil || lr.dirtyTracker.NeedsFullRedraw() {
		return lr.RenderVisibleLines(getText)
	}

	dirtyLines := lr.dirtyTracker.DirtyLines()
	result := make([]RenderLine, 0, len(dirtyLines))

	for _, line := range dirtyLines {
		// Check if line is visible
		if line < lr.topLine {
			continue
		}
		screenRow := int(line - lr.topLine)
		if screenRow >= lr.screenHeight {
			continue
		}

		text := getText(line)
		renderLine := lr.renderLine(line, screenRow, text)
		result = append(result, renderLine)
	}

	return result
}

// renderLine renders a single line.
func (lr *LineRenderer) renderLine(line uint32, screenRow int, text string) RenderLine {
	// Check if dirty
	isDirty := true
	if lr.dirtyTracker != nil {
		isDirty = lr.dirtyTracker.IsLineDirty(line)
	}

	// Get styled cells from cache
	cells := lr.cache.GetStyledCells(line, text)

	// Build content cells for the visible portion
	contentWidth := lr.screenWidth - lr.gutterWidth
	contentCells := make([]core.Cell, contentWidth)

	for x := 0; x < contentWidth; x++ {
		visCol := lr.leftColumn + x
		if visCol >= 0 && visCol < len(cells) {
			contentCells[x] = cells[visCol]
		} else {
			contentCells[x] = core.Cell{
				Rune:  ' ',
				Width: 1,
				Style: lr.baseStyle,
			}
		}
	}

	// Build gutter cells
	gutterCells := lr.buildGutterCells(line)

	return RenderLine{
		Line:        line,
		ScreenRow:   screenRow,
		Cells:       contentCells,
		GutterCells: gutterCells,
		IsDirty:     isDirty,
	}
}

// buildGutterCells builds the gutter cells for a line.
func (lr *LineRenderer) buildGutterCells(line uint32) []core.Cell {
	if lr.gutterWidth <= 0 {
		return nil
	}

	cells := make([]core.Cell, lr.gutterWidth)

	// Initialize all cells with spaces and gutter style to avoid NUL characters
	for i := range cells {
		cells[i] = core.Cell{
			Rune:  ' ',
			Width: 1,
			Style: lr.gutterStyle,
		}
	}

	// Format line number (1-indexed for display), right-aligned
	numStr := formatLineNumber(line+1, lr.gutterWidth-1)

	// Fill gutter cells with line number (right-aligned)
	// If number is longer than available width, show rightmost digits
	availWidth := lr.gutterWidth - 1 // Reserve last cell for separator
	startIdx := 0
	if len(numStr) > availWidth {
		// Truncate from left (show least-significant digits)
		startIdx = len(numStr) - availWidth
	}
	visiblePart := numStr[startIdx:]
	// Right-align: place digits at the end of the available space
	offset := availWidth - len(visiblePart)
	for i, r := range visiblePart {
		cells[offset+i].Rune = r
	}

	// Separator uses base style
	if lr.gutterWidth > 0 {
		cells[lr.gutterWidth-1].Style = lr.baseStyle
	}

	return cells
}

// formatLineNumber formats a line number with left-padding spaces.
// Returns empty string if num is 0 (since line numbers are 1-indexed for display)
// or if width is non-positive.
func formatLineNumber(num uint32, width int) string {
	if num == 0 || width <= 0 {
		return ""
	}

	// Convert number to string
	var buf [10]byte // Max 10 digits for uint32
	i := len(buf)
	n := num
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	numStr := string(buf[i:])

	// Pad with spaces
	if len(numStr) >= width {
		return numStr
	}

	padding := make([]byte, width-len(numStr))
	for j := range padding {
		padding[j] = ' '
	}
	return string(padding) + numStr
}

// VisibleLineRange returns the range of visible lines.
func (lr *LineRenderer) VisibleLineRange() (startLine, endLine uint32) {
	startLine = lr.topLine
	// Guard against uint32 overflow
	if lr.screenHeight <= 0 {
		endLine = lr.topLine
		return
	}
	offset := uint32(lr.screenHeight - 1)
	if lr.topLine > math.MaxUint32-offset {
		// Would overflow, clamp to max
		endLine = math.MaxUint32
	} else {
		endLine = lr.topLine + offset
	}
	return
}

// LineToScreenRow converts a buffer line to a screen row.
// Returns -1 if the line is not visible.
func (lr *LineRenderer) LineToScreenRow(line uint32) int {
	if line < lr.topLine {
		return -1
	}
	row := int(line - lr.topLine)
	if row >= lr.screenHeight {
		return -1
	}
	return row
}

// ScreenRowToLine converts a screen row to a buffer line.
func (lr *LineRenderer) ScreenRowToLine(row int) uint32 {
	if row < 0 {
		return lr.topLine
	}
	return lr.topLine + uint32(row)
}

// Compositor applies overlay effects to rendered lines.
type Compositor struct {
	ghostTextStyle core.Style
	diffAddStyle   core.Style
	diffDelStyle   core.Style
	diffModStyle   core.Style
}

// NewCompositor creates a new compositor with default styles.
func NewCompositor() *Compositor {
	defaults := style.NewDefaultStyles()
	return &Compositor{
		ghostTextStyle: defaults.GhostText,
		diffAddStyle:   defaults.DiffAdd,
		diffDelStyle:   defaults.DiffDelete,
		diffModStyle:   defaults.DiffModify,
	}
}

// SetGhostTextStyle sets the ghost text style.
func (c *Compositor) SetGhostTextStyle(s core.Style) {
	c.ghostTextStyle = s
}

// SetDiffStyles sets the diff preview styles.
func (c *Compositor) SetDiffStyles(add, del, mod core.Style) {
	c.diffAddStyle = add
	c.diffDelStyle = del
	c.diffModStyle = mod
}

// ApplyGhostText appends ghost text cells to a render line.
func (c *Compositor) ApplyGhostText(line *RenderLine, text string) {
	if text == "" {
		return
	}

	cells := make([]core.Cell, 0, len(text))
	for _, r := range text {
		cells = append(cells, core.Cell{
			Rune:  r,
			Width: core.RuneWidth(r),
			Style: c.ghostTextStyle,
		})
	}

	line.AfterContent = cells
	line.HasOverlay = true
}

// ApplyDiffAdd applies diff addition styling to cells.
func (c *Compositor) ApplyDiffAdd(cells []core.Cell) []core.Cell {
	result := make([]core.Cell, len(cells))
	for i, cell := range cells {
		result[i] = cell
		result[i].Style = c.diffAddStyle
	}
	return result
}

// ApplyDiffDelete applies diff deletion styling to cells.
func (c *Compositor) ApplyDiffDelete(cells []core.Cell) []core.Cell {
	result := make([]core.Cell, len(cells))
	for i, cell := range cells {
		result[i] = cell
		result[i].Style = c.diffDelStyle
	}
	return result
}

// ApplyDiffModify applies diff modification styling to cells.
func (c *Compositor) ApplyDiffModify(cells []core.Cell) []core.Cell {
	result := make([]core.Cell, len(cells))
	for i, cell := range cells {
		result[i] = cell
		result[i].Style = c.diffModStyle
	}
	return result
}
