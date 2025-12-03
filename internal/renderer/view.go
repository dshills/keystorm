package renderer

import (
	"sync"

	"github.com/dshills/keystorm/internal/renderer/backend"
	"github.com/dshills/keystorm/internal/renderer/layout"
	"github.com/dshills/keystorm/internal/renderer/viewport"
)

// View represents a single editor view within the renderer.
// Multiple views can be used for split panes.
type View struct {
	mu sync.RWMutex

	// Identity
	id string

	// Position within the terminal
	x, y          int
	width, height int

	// Content providers
	bufReader  BufferReader
	cursorProv CursorProvider
	hlProvider HighlightProvider

	// Components
	viewport  *viewport.Viewport
	lineCache *layout.LineCache
	layout    *layout.LayoutEngine

	// Options
	opts ViewOptions

	// State
	gutterWidth int
	focused     bool
	needsRedraw bool
}

// ViewOptions configures a single view.
type ViewOptions struct {
	ShowLineNumbers bool
	LineNumberWidth int
	ShowGutter      bool
	WordWrap        bool
	WrapAtColumn    int
	ScrollMargins   viewport.MarginConfig
	SmoothScroll    bool
}

// DefaultViewOptions returns default view options.
func DefaultViewOptions() ViewOptions {
	return ViewOptions{
		ShowLineNumbers: true,
		LineNumberWidth: 0,
		ShowGutter:      true,
		WordWrap:        false,
		WrapAtColumn:    0,
		ScrollMargins:   viewport.DefaultMargins(),
		SmoothScroll:    true,
	}
}

// NewView creates a new view with the given bounds.
func NewView(id string, x, y, width, height int, opts ViewOptions) *View {
	layoutEngine := layout.NewLayoutEngine(4)
	lineCache := layout.NewLineCache(layoutEngine, 500)

	v := &View{
		id:          id,
		x:           x,
		y:           y,
		width:       width,
		height:      height,
		viewport:    viewport.NewViewport(width, height),
		lineCache:   lineCache,
		layout:      layoutEngine,
		opts:        opts,
		needsRedraw: true,
	}

	v.viewport.SetMarginsFromConfig(opts.ScrollMargins)
	v.viewport.SetSmoothScroll(opts.SmoothScroll)

	return v
}

// ID returns the view's identifier.
func (v *View) ID() string {
	return v.id
}

// Bounds returns the view's position and size.
func (v *View) Bounds() (x, y, width, height int) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.x, v.y, v.width, v.height
}

// SetBounds updates the view's position and size.
func (v *View) SetBounds(x, y, width, height int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.x = x
	v.y = y
	v.width = width
	v.height = height
	v.viewport.Resize(width, height)
	v.needsRedraw = true
}

// SetBuffer sets the buffer reader for this view.
func (v *View) SetBuffer(buf BufferReader) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.bufReader = buf
	if buf != nil {
		v.layout.SetTabWidth(buf.TabWidth())
		v.viewport.SetMaxLine(buf.LineCount())
	}
	v.lineCache.InvalidateAll()
	v.needsRedraw = true
}

// SetCursorProvider sets the cursor provider.
func (v *View) SetCursorProvider(cp CursorProvider) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cursorProv = cp
	v.needsRedraw = true
}

// SetHighlightProvider sets the syntax highlighting provider.
func (v *View) SetHighlightProvider(hp HighlightProvider) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hlProvider = hp
	v.lineCache.InvalidateAll()
	v.needsRedraw = true
}

// SetFocused sets whether this view has input focus.
func (v *View) SetFocused(focused bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.focused = focused
	v.needsRedraw = true
}

// IsFocused returns whether this view has input focus.
func (v *View) IsFocused() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.focused
}

// Viewport returns the view's viewport.
func (v *View) Viewport() *viewport.Viewport {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.viewport
}

// Options returns the view's options.
func (v *View) Options() ViewOptions {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.opts
}

// SetOptions updates the view's options.
func (v *View) SetOptions(opts ViewOptions) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.opts = opts
	v.viewport.SetMarginsFromConfig(opts.ScrollMargins)
	v.viewport.SetSmoothScroll(opts.SmoothScroll)
	v.needsRedraw = true
}

// MarkDirty marks the view as needing redraw.
func (v *View) MarkDirty() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.needsRedraw = true
}

// NeedsRedraw returns whether the view needs redrawing.
func (v *View) NeedsRedraw() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.needsRedraw
}

// InvalidateLine invalidates a specific line in the cache.
func (v *View) InvalidateLine(line uint32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.lineCache.Invalidate(line)
	v.needsRedraw = true
}

// InvalidateLines invalidates a range of lines.
func (v *View) InvalidateLines(startLine, endLine uint32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.lineCache.InvalidateRange(startLine, endLine)
	v.needsRedraw = true
}

// Update advances animations and returns true if view needs redrawing.
func (v *View) Update(dt float64) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	moved := v.viewport.Update(dt)
	if moved {
		v.needsRedraw = true
	}
	return v.needsRedraw
}

// Render renders the view to the given backend.
func (v *View) Render(backend backend.Backend) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.bufReader == nil {
		v.renderEmpty(backend)
		v.needsRedraw = false
		return
	}

	// Update max line
	v.viewport.SetMaxLine(v.bufReader.LineCount())

	// Calculate gutter width
	v.gutterWidth = v.calculateGutterWidth()

	// Get visible line range
	startLine, endLine := v.viewport.VisibleLineRange()

	// Render each visible line
	for line := startLine; line <= endLine; line++ {
		screenRow := v.viewport.LineToScreenRow(line)
		if screenRow >= 0 && screenRow < v.height {
			v.renderLine(backend, line, screenRow)
		}
	}

	// Render cursor if focused
	if v.focused {
		v.renderCursor(backend)
	}

	v.needsRedraw = false
}

// renderEmpty renders when there's no buffer.
func (v *View) renderEmpty(backend backend.Backend) {
	empty := EmptyCell()
	for row := 0; row < v.height; row++ {
		for col := 0; col < v.width; col++ {
			backend.SetCell(v.x+col, v.y+row, empty)
		}
	}
}

// renderLine renders a single line.
func (v *View) renderLine(backend backend.Backend, line uint32, screenRow int) {
	absoluteRow := v.y + screenRow

	// Render gutter
	if v.opts.ShowGutter {
		v.renderGutter(backend, line, absoluteRow)
	}

	// Get line text
	lineCount := v.bufReader.LineCount()
	if line >= lineCount {
		v.clearLineContent(backend, absoluteRow)
		return
	}

	text := v.bufReader.LineText(line)
	lineLayout := v.lineCache.Get(line, text)

	// Apply highlighting
	if v.hlProvider != nil {
		spans := v.hlProvider.HighlightsForLine(line)
		if len(spans) > 0 {
			v.layout.ApplyStyles(lineLayout, spans)
		}
	}

	// Render cells
	leftCol := v.viewport.LeftColumn()
	contentWidth := v.width - v.gutterWidth

	for x := 0; x < contentWidth; x++ {
		visCol := leftCol + x
		screenX := v.x + v.gutterWidth + x

		var cell Cell
		if visCol >= 0 && visCol < len(lineLayout.Cells) {
			cell = lineLayout.Cells[visCol]
		} else {
			cell = EmptyCell()
		}

		backend.SetCell(screenX, absoluteRow, cell)
	}
}

// renderGutter renders the gutter for a line.
func (v *View) renderGutter(backend backend.Backend, line uint32, absoluteRow int) {
	if !v.opts.ShowLineNumbers {
		return
	}

	lineCount := v.bufReader.LineCount()
	var numStr string
	if line < lineCount {
		numStr = formatLineNumber(line+1, v.gutterWidth-1)
	} else {
		numStr = formatLineNumber(0, v.gutterWidth-1)
	}

	gutterStyle := DefaultStyle().Dim()

	for x, ch := range numStr {
		if x < v.gutterWidth-1 {
			backend.SetCell(v.x+x, absoluteRow, Cell{
				Rune:  ch,
				Width: 1,
				Style: gutterStyle,
			})
		}
	}

	// Separator
	backend.SetCell(v.x+v.gutterWidth-1, absoluteRow, Cell{
		Rune:  ' ',
		Width: 1,
		Style: DefaultStyle(),
	})
}

// clearLineContent clears the content area of a line.
func (v *View) clearLineContent(backend backend.Backend, absoluteRow int) {
	empty := EmptyCell()
	for x := v.gutterWidth; x < v.width; x++ {
		backend.SetCell(v.x+x, absoluteRow, empty)
	}
}

// renderCursor renders the cursor.
func (v *View) renderCursor(backend backend.Backend) {
	if v.cursorProv == nil {
		return
	}

	line, col := v.cursorProv.PrimaryCursor()

	if !v.viewport.IsLineVisible(line) {
		return
	}

	text := v.bufReader.LineText(line)
	lineLayout := v.lineCache.Get(line, text)

	visCol := lineLayout.VisualColumn(col)
	screenRow := v.viewport.LineToScreenRow(line)
	screenCol := visCol - v.viewport.LeftColumn() + v.gutterWidth

	if screenCol < v.gutterWidth || screenCol >= v.width {
		return
	}

	backend.ShowCursor(v.x+screenCol, v.y+screenRow)
}

// calculateGutterWidth calculates the required gutter width.
func (v *View) calculateGutterWidth() int {
	if !v.opts.ShowGutter || !v.opts.ShowLineNumbers {
		return 0
	}

	if v.opts.LineNumberWidth > 0 {
		return v.opts.LineNumberWidth + 1
	}

	if v.bufReader == nil {
		return 4
	}

	lineCount := v.bufReader.LineCount()
	digits := 1
	for n := lineCount; n >= 10; n /= 10 {
		digits++
	}

	if digits < 3 {
		digits = 3
	}
	return digits + 1
}

// GutterWidth returns the current gutter width.
func (v *View) GutterWidth() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.gutterWidth
}

// ScrollToLine scrolls to make the given line visible.
func (v *View) ScrollToLine(line uint32, smooth bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.viewport.EnsureLineVisible(line, smooth)
	v.needsRedraw = true
}

// ScrollToReveal scrolls minimally to reveal a position.
func (v *View) ScrollToReveal(line uint32, col int, smooth bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.viewport.ScrollToReveal(line, col, smooth)
	v.needsRedraw = true
}

// CenterOnLine centers the view on the given line.
func (v *View) CenterOnLine(line uint32, smooth bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.viewport.CenterOn(line, smooth)
	v.needsRedraw = true
}

// ScreenToBuffer converts screen coordinates to buffer position.
func (v *View) ScreenToBuffer(screenX, screenY int) (line uint32, col uint32, ok bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check if within view bounds
	if screenX < v.x || screenX >= v.x+v.width ||
		screenY < v.y || screenY >= v.y+v.height {
		return 0, 0, false
	}

	// Adjust for view position
	localX := screenX - v.x
	localY := screenY - v.y

	// Check if in gutter
	if localX < v.gutterWidth {
		return 0, 0, false
	}

	// Convert to buffer coordinates
	contentX := localX - v.gutterWidth
	line = v.viewport.ScreenRowToLine(localY)

	// Get visual column
	visCol := v.viewport.LeftColumn() + contentX

	// Convert visual column to buffer column
	if v.bufReader == nil {
		return line, uint32(visCol), true
	}

	if line >= v.bufReader.LineCount() {
		return line, 0, true
	}

	text := v.bufReader.LineText(line)
	lineLayout := v.lineCache.Get(line, text)
	col = lineLayout.BufferColumn(visCol)

	return line, col, true
}

// BufferToScreen converts buffer position to screen coordinates.
func (v *View) BufferToScreen(line uint32, col uint32) (screenX, screenY int, ok bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.viewport.IsLineVisible(line) {
		return 0, 0, false
	}

	screenRow := v.viewport.LineToScreenRow(line)
	if screenRow < 0 {
		return 0, 0, false
	}

	// Get visual column
	var visCol int
	if v.bufReader != nil && line < v.bufReader.LineCount() {
		text := v.bufReader.LineText(line)
		lineLayout := v.lineCache.Get(line, text)
		visCol = lineLayout.VisualColumn(col)
	} else {
		visCol = int(col)
	}

	screenCol := visCol - v.viewport.LeftColumn() + v.gutterWidth

	if screenCol < v.gutterWidth || screenCol >= v.width {
		return 0, 0, false
	}

	return v.x + screenCol, v.y + screenRow, true
}
