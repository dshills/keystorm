package renderer

import (
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/renderer/backend"
	"github.com/dshills/keystorm/internal/renderer/layout"
	"github.com/dshills/keystorm/internal/renderer/viewport"
)

// BufferReader provides read access to buffer content.
// This interface abstracts the engine for rendering.
type BufferReader interface {
	// LineText returns the text content of a line (0-indexed).
	LineText(line uint32) string

	// LineCount returns the total number of lines in the buffer.
	LineCount() uint32

	// TabWidth returns the configured tab width.
	TabWidth() int
}

// CursorProvider provides cursor and selection information.
type CursorProvider interface {
	// PrimaryCursor returns the primary cursor position (line, column).
	PrimaryCursor() (line uint32, col uint32)

	// Selections returns all active selections for rendering.
	Selections() []Selection
}

// Selection represents a selection range for rendering.
type Selection struct {
	StartLine uint32
	StartCol  uint32
	EndLine   uint32
	EndCol    uint32
	IsPrimary bool
}

// HighlightProvider provides syntax highlighting information.
type HighlightProvider interface {
	// HighlightsForLine returns style spans for the given line.
	// Returns spans sorted by start position.
	HighlightsForLine(line uint32) []StyleSpan

	// InvalidateLines invalidates cached highlighting for a range.
	InvalidateLines(startLine, endLine uint32)
}

// Options configures the renderer.
type Options struct {
	// Display
	ShowLineNumbers bool // Show line numbers in gutter
	LineNumberWidth int  // Width of line number column (0 = auto)
	ShowGutter      bool // Show gutter (line numbers, signs, etc.)
	WordWrap        bool // Enable word wrap
	WrapAtColumn    int  // Column to wrap at (0 = window width)

	// Scrolling
	ScrollMarginTop    int  // Lines to keep above cursor
	ScrollMarginBottom int  // Lines to keep below cursor
	ScrollMarginLeft   int  // Columns to keep left of cursor
	ScrollMarginRight  int  // Columns to keep right of cursor
	SmoothScroll       bool // Enable smooth scroll animation

	// Cursor
	CursorStyle     backend.CursorStyle // Cursor appearance
	CursorBlink     bool                // Enable cursor blink
	CursorBlinkRate time.Duration       // Blink rate

	// Performance
	MaxFPS           int  // Maximum frames per second
	LazyHighlighting bool // Defer highlighting for off-screen lines
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		ShowLineNumbers:    true,
		LineNumberWidth:    0, // Auto-calculate
		ShowGutter:         true,
		WordWrap:           false,
		WrapAtColumn:       0, // Window width
		ScrollMarginTop:    5,
		ScrollMarginBottom: 5,
		ScrollMarginLeft:   10,
		ScrollMarginRight:  10,
		SmoothScroll:       true,
		CursorStyle:        backend.CursorBlock,
		CursorBlink:        true,
		CursorBlinkRate:    500 * time.Millisecond,
		MaxFPS:             60,
		LazyHighlighting:   true,
	}
}

// Renderer is the main rendering facade.
// It coordinates all rendering components to display buffer content.
type Renderer struct {
	mu sync.RWMutex

	// Configuration
	opts Options

	// Backend and screen
	backend backend.Backend
	width   int
	height  int

	// Content providers
	bufReader  BufferReader
	cursorProv CursorProvider
	hlProvider HighlightProvider

	// Components
	viewport  *viewport.Viewport
	lineCache *layout.LineCache
	layout    *layout.LayoutEngine

	// Frame timing
	lastFrame    time.Time
	minFrameTime time.Duration
	frameCount   uint64
	needsRedraw  bool
	fullRedraw   bool

	// Gutter state
	gutterWidth int
}

// New creates a new renderer with the given backend and options.
func New(backend backend.Backend, opts Options) *Renderer {
	width, height := backend.Size()

	layoutEngine := layout.NewLayoutEngine(4)            // Default tab width
	lineCache := layout.NewLineCache(layoutEngine, 1000) // Cache up to 1000 lines

	r := &Renderer{
		opts:         opts,
		backend:      backend,
		width:        width,
		height:       height,
		viewport:     viewport.NewViewport(width, height),
		lineCache:    lineCache,
		layout:       layoutEngine,
		lastFrame:    time.Now(),
		minFrameTime: time.Second / time.Duration(opts.MaxFPS),
		needsRedraw:  true,
		fullRedraw:   true,
	}

	// Configure viewport margins
	r.viewport.SetMargins(
		opts.ScrollMarginTop,
		opts.ScrollMarginBottom,
		opts.ScrollMarginLeft,
		opts.ScrollMarginRight,
	)
	r.viewport.SetSmoothScroll(opts.SmoothScroll)

	// Register resize handler
	backend.OnResize(func(w, h int) {
		r.Resize(w, h)
	})

	return r
}

// SetBuffer sets the buffer reader for content.
func (r *Renderer) SetBuffer(buf BufferReader) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.bufReader = buf
	if buf != nil {
		r.layout.SetTabWidth(buf.TabWidth())
		r.viewport.SetMaxLine(buf.LineCount())
	}
	r.lineCache.InvalidateAll()
	r.needsRedraw = true
	r.fullRedraw = true
}

// SetCursorProvider sets the cursor provider.
func (r *Renderer) SetCursorProvider(cp CursorProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cursorProv = cp
	r.needsRedraw = true
}

// SetHighlightProvider sets the syntax highlighting provider.
func (r *Renderer) SetHighlightProvider(hp HighlightProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hlProvider = hp
	r.lineCache.InvalidateAll()
	r.needsRedraw = true
	r.fullRedraw = true
}

// Resize handles terminal resize events.
func (r *Renderer) Resize(width, height int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.width = width
	r.height = height
	r.viewport.Resize(width, height)
	r.needsRedraw = true
	r.fullRedraw = true
}

// MarkDirty marks the renderer as needing a redraw.
func (r *Renderer) MarkDirty() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.needsRedraw = true
}

// MarkFullRedraw marks the renderer as needing a complete redraw.
func (r *Renderer) MarkFullRedraw() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.needsRedraw = true
	r.fullRedraw = true
}

// InvalidateLine marks a specific line as needing redraw.
func (r *Renderer) InvalidateLine(line uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lineCache.Invalidate(line)
	r.needsRedraw = true
}

// InvalidateLines marks a range of lines as needing redraw.
func (r *Renderer) InvalidateLines(startLine, endLine uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lineCache.InvalidateRange(startLine, endLine)
	r.needsRedraw = true
}

// Viewport returns the viewport for external manipulation.
func (r *Renderer) Viewport() *viewport.Viewport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewport
}

// Options returns the current options.
func (r *Renderer) Options() Options {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.opts
}

// SetOptions updates the renderer options.
func (r *Renderer) SetOptions(opts Options) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.opts = opts
	r.minFrameTime = time.Second / time.Duration(opts.MaxFPS)
	r.viewport.SetMargins(
		opts.ScrollMarginTop,
		opts.ScrollMarginBottom,
		opts.ScrollMarginLeft,
		opts.ScrollMarginRight,
	)
	r.viewport.SetSmoothScroll(opts.SmoothScroll)
	r.backend.SetCursorStyle(opts.CursorStyle)
	r.fullRedraw = true
	r.needsRedraw = true
}

// NeedsRedraw returns true if the renderer needs to redraw.
func (r *Renderer) NeedsRedraw() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.needsRedraw
}

// Update advances animations and prepares for rendering.
// Returns true if the display needs updating.
func (r *Renderer) Update(dt float64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	moved := r.viewport.Update(dt)
	if moved {
		r.needsRedraw = true
	}

	return r.needsRedraw
}

// Render performs a full render cycle.
// Respects frame rate limiting.
func (r *Renderer) Render() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Frame rate limiting
	now := time.Now()
	elapsed := now.Sub(r.lastFrame)
	if elapsed < r.minFrameTime {
		return
	}
	r.lastFrame = now

	if !r.needsRedraw {
		return
	}

	r.render()
	r.needsRedraw = false
	r.fullRedraw = false
	r.frameCount++
}

// RenderNow performs an immediate render, ignoring frame rate limiting.
func (r *Renderer) RenderNow() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.render()
	r.needsRedraw = false
	r.fullRedraw = false
	r.frameCount++
	r.lastFrame = time.Now()
}

// render performs the actual rendering (must hold lock).
func (r *Renderer) render() {
	if r.bufReader == nil {
		r.renderEmpty()
		return
	}

	// Update max line in viewport
	r.viewport.SetMaxLine(r.bufReader.LineCount())

	// Calculate gutter width
	r.gutterWidth = r.calculateGutterWidth()

	// Update viewport content width
	contentWidth := r.width - r.gutterWidth
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Clear screen if full redraw
	if r.fullRedraw {
		r.backend.Clear()
	}

	// Get visible line range
	startLine, endLine := r.viewport.VisibleLineRange()

	// Render each visible line
	for line := startLine; line <= endLine; line++ {
		screenRow := r.viewport.LineToScreenRow(line)
		if screenRow >= 0 && screenRow < r.height {
			r.renderLine(line, screenRow)
		}
	}

	// Render cursor
	r.renderCursor()

	// Flush to screen
	r.backend.Show()
}

// renderEmpty renders when there's no buffer.
func (r *Renderer) renderEmpty() {
	r.backend.Clear()
	r.backend.HideCursor()
	r.backend.Show()
}

// renderLine renders a single buffer line at the given screen row.
func (r *Renderer) renderLine(line uint32, screenRow int) {
	// Render gutter
	if r.opts.ShowGutter {
		r.renderGutter(line, screenRow)
	}

	// Get line text
	lineCount := r.bufReader.LineCount()
	if line >= lineCount {
		// Clear rest of screen for lines beyond buffer
		r.clearLineContent(screenRow)
		return
	}

	text := r.bufReader.LineText(line)

	// Get layout from cache
	lineLayout := r.lineCache.Get(line, text)

	// Apply syntax highlighting if available
	if r.hlProvider != nil {
		spans := r.hlProvider.HighlightsForLine(line)
		if len(spans) > 0 {
			r.layout.ApplyStyles(lineLayout, spans)
		}
	}

	// Render cells
	leftCol := r.viewport.LeftColumn()
	contentWidth := r.width - r.gutterWidth

	for x := 0; x < contentWidth; x++ {
		visCol := leftCol + x
		screenX := r.gutterWidth + x

		var cell Cell
		if visCol >= 0 && visCol < len(lineLayout.Cells) {
			cell = lineLayout.Cells[visCol]
		} else {
			cell = EmptyCell()
		}

		r.backend.SetCell(screenX, screenRow, cell)
	}
}

// renderGutter renders the gutter (line numbers) for a line.
func (r *Renderer) renderGutter(line uint32, screenRow int) {
	if !r.opts.ShowLineNumbers {
		return
	}

	lineCount := r.bufReader.LineCount()

	// Format line number
	var numStr string
	if line < lineCount {
		numStr = formatLineNumber(line+1, r.gutterWidth-1) // +1 for 1-indexed display
	} else {
		numStr = formatLineNumber(0, r.gutterWidth-1) // Show ~ or empty for non-existent lines
	}

	// Gutter style (dim)
	gutterStyle := DefaultStyle().Dim()

	// Render line number
	for x, ch := range numStr {
		if x < r.gutterWidth-1 {
			r.backend.SetCell(x, screenRow, Cell{
				Rune:  ch,
				Width: 1,
				Style: gutterStyle,
			})
		}
	}

	// Separator
	r.backend.SetCell(r.gutterWidth-1, screenRow, Cell{
		Rune:  ' ',
		Width: 1,
		Style: DefaultStyle(),
	})
}

// clearLineContent clears the content area of a line.
func (r *Renderer) clearLineContent(screenRow int) {
	empty := EmptyCell()
	for x := r.gutterWidth; x < r.width; x++ {
		r.backend.SetCell(x, screenRow, empty)
	}
}

// renderCursor renders the cursor at the current position.
func (r *Renderer) renderCursor() {
	if r.cursorProv == nil {
		r.backend.HideCursor()
		return
	}

	line, col := r.cursorProv.PrimaryCursor()

	// Check if cursor is visible
	if !r.viewport.IsLineVisible(line) {
		r.backend.HideCursor()
		return
	}

	// Get layout for cursor line
	text := r.bufReader.LineText(line)
	lineLayout := r.lineCache.Get(line, text)

	// Convert buffer column to visual column
	visCol := lineLayout.VisualColumn(col)

	// Convert to screen coordinates
	screenRow := r.viewport.LineToScreenRow(line)
	screenCol := visCol - r.viewport.LeftColumn() + r.gutterWidth

	// Check if cursor is in visible area
	if screenCol < r.gutterWidth || screenCol >= r.width {
		r.backend.HideCursor()
		return
	}

	r.backend.ShowCursor(screenCol, screenRow)
}

// calculateGutterWidth calculates the required gutter width.
func (r *Renderer) calculateGutterWidth() int {
	if !r.opts.ShowGutter || !r.opts.ShowLineNumbers {
		return 0
	}

	if r.opts.LineNumberWidth > 0 {
		return r.opts.LineNumberWidth + 1 // +1 for separator
	}

	// Auto-calculate based on line count
	if r.bufReader == nil {
		return 4 // Default minimum
	}

	lineCount := r.bufReader.LineCount()
	digits := 1
	for n := lineCount; n >= 10; n /= 10 {
		digits++
	}

	// Minimum 3 digits, plus separator
	if digits < 3 {
		digits = 3
	}
	return digits + 1
}

// formatLineNumber formats a line number with padding.
func formatLineNumber(num uint32, width int) string {
	if num == 0 {
		// Non-existent line
		return padLeft("~", width)
	}

	s := uintToString(num)
	return padLeft(s, width)
}

// padLeft pads a string with spaces on the left.
func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	padding := make([]byte, width-len(s))
	for i := range padding {
		padding[i] = ' '
	}
	return string(padding) + s
}

// uintToString converts a uint32 to string without fmt package.
func uintToString(n uint32) string {
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

// FrameCount returns the number of frames rendered.
func (r *Renderer) FrameCount() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.frameCount
}

// Size returns the current screen dimensions.
func (r *Renderer) Size() (width, height int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.width, r.height
}

// GutterWidth returns the current gutter width.
func (r *Renderer) GutterWidth() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gutterWidth
}

// ScrollToLine scrolls to make the given line visible.
func (r *Renderer) ScrollToLine(line uint32, smooth bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.viewport.EnsureLineVisible(line, smooth)
	r.needsRedraw = true
}

// ScrollToReveal scrolls minimally to reveal a position.
func (r *Renderer) ScrollToReveal(line uint32, col int, smooth bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.viewport.ScrollToReveal(line, col, smooth)
	r.needsRedraw = true
}

// CenterOnLine centers the viewport on the given line.
func (r *Renderer) CenterOnLine(line uint32, smooth bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.viewport.CenterOn(line, smooth)
	r.needsRedraw = true
}
