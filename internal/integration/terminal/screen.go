package terminal

import (
	"sync"
)

// Color represents a terminal color.
type Color struct {
	R, G, B uint8
	Index   int  // -1 for RGB, 0-255 for indexed
	Default bool // Use default fg/bg
}

// DefaultForeground is the default foreground color.
var DefaultForeground = Color{Default: true}

// DefaultBackground is the default background color.
var DefaultBackground = Color{Default: true}

// Standard ANSI colors (indices 0-15).
var (
	ColorBlack         = Color{Index: 0, R: 0, G: 0, B: 0}
	ColorRed           = Color{Index: 1, R: 205, G: 0, B: 0}
	ColorGreen         = Color{Index: 2, R: 0, G: 205, B: 0}
	ColorYellow        = Color{Index: 3, R: 205, G: 205, B: 0}
	ColorBlue          = Color{Index: 4, R: 0, G: 0, B: 238}
	ColorMagenta       = Color{Index: 5, R: 205, G: 0, B: 205}
	ColorCyan          = Color{Index: 6, R: 0, G: 205, B: 205}
	ColorWhite         = Color{Index: 7, R: 229, G: 229, B: 229}
	ColorBrightBlack   = Color{Index: 8, R: 127, G: 127, B: 127}
	ColorBrightRed     = Color{Index: 9, R: 255, G: 0, B: 0}
	ColorBrightGreen   = Color{Index: 10, R: 0, G: 255, B: 0}
	ColorBrightYellow  = Color{Index: 11, R: 255, G: 255, B: 0}
	ColorBrightBlue    = Color{Index: 12, R: 92, G: 92, B: 255}
	ColorBrightMagenta = Color{Index: 13, R: 255, G: 0, B: 255}
	ColorBrightCyan    = Color{Index: 14, R: 0, G: 255, B: 255}
	ColorBrightWhite   = Color{Index: 15, R: 255, G: 255, B: 255}
)

// ANSIColors is the standard 16-color palette.
var ANSIColors = []Color{
	ColorBlack, ColorRed, ColorGreen, ColorYellow,
	ColorBlue, ColorMagenta, ColorCyan, ColorWhite,
	ColorBrightBlack, ColorBrightRed, ColorBrightGreen, ColorBrightYellow,
	ColorBrightBlue, ColorBrightMagenta, ColorBrightCyan, ColorBrightWhite,
}

// ColorFromIndex returns a color from a 256-color index.
func ColorFromIndex(index int) Color {
	if index < 0 || index > 255 {
		return DefaultForeground
	}

	if index < 16 {
		return ANSIColors[index]
	}

	// 216-color cube (indices 16-231)
	if index < 232 {
		index -= 16
		r := uint8((index / 36) * 51)
		g := uint8(((index / 6) % 6) * 51)
		b := uint8((index % 6) * 51)
		return Color{R: r, G: g, B: b, Index: index + 16}
	}

	// Grayscale (indices 232-255)
	gray := uint8((index-232)*10 + 8)
	return Color{R: gray, G: gray, B: gray, Index: index}
}

// ColorFromRGB creates an RGB color.
func ColorFromRGB(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, Index: -1}
}

// CellAttributes represents text attributes for a cell.
type CellAttributes uint16

const (
	AttrNone      CellAttributes = 0
	AttrBold      CellAttributes = 1 << 0
	AttrDim       CellAttributes = 1 << 1
	AttrItalic    CellAttributes = 1 << 2
	AttrUnderline CellAttributes = 1 << 3
	AttrBlink     CellAttributes = 1 << 4
	AttrReverse   CellAttributes = 1 << 5
	AttrHidden    CellAttributes = 1 << 6
	AttrStrike    CellAttributes = 1 << 7
)

// Has returns true if the attribute is set.
func (a CellAttributes) Has(attr CellAttributes) bool {
	return a&attr != 0
}

// Cell represents a single character cell in the terminal.
type Cell struct {
	Rune       rune
	Width      int // Display width (1 for normal, 2 for wide chars)
	Foreground Color
	Background Color
	Attributes CellAttributes
}

// EmptyCell returns a cell with default values.
func EmptyCell() Cell {
	return Cell{
		Rune:       ' ',
		Width:      1,
		Foreground: DefaultForeground,
		Background: DefaultBackground,
		Attributes: AttrNone,
	}
}

// Line represents a single line in the terminal.
type Line struct {
	Cells   []Cell
	Wrapped bool // True if this line wraps to the next
}

// NewLine creates a new line with the given width.
func NewLine(width int) *Line {
	cells := make([]Cell, width)
	for i := range cells {
		cells[i] = EmptyCell()
	}
	return &Line{Cells: cells}
}

// Clear clears the line with empty cells.
func (l *Line) Clear() {
	for i := range l.Cells {
		l.Cells[i] = EmptyCell()
	}
	l.Wrapped = false
}

// ClearRange clears cells in the range [start, end).
func (l *Line) ClearRange(start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(l.Cells) {
		end = len(l.Cells)
	}
	for i := start; i < end; i++ {
		l.Cells[i] = EmptyCell()
	}
}

// Screen represents the terminal screen buffer.
type Screen struct {
	mu sync.RWMutex

	width  int
	height int
	lines  []*Line

	// Cursor position (0-indexed)
	cursorX int
	cursorY int

	// Cursor state
	cursorVisible bool
	cursorStyle   CursorStyle

	// Scroll region
	scrollTop    int
	scrollBottom int

	// Current cell attributes for new characters
	currentFg    Color
	currentBg    Color
	currentAttrs CellAttributes

	// Saved cursor state
	savedX, savedY   int
	savedFg, savedBg Color
	savedAttrs       CellAttributes

	// Mode flags
	originMode bool // DECOM - origin mode
	autoWrap   bool // DECAWM - auto wrap mode
}

// CursorStyle represents the cursor appearance.
type CursorStyle int

const (
	CursorBlock CursorStyle = iota
	CursorUnderline
	CursorBar
)

// NewScreen creates a new screen buffer with the given dimensions.
func NewScreen(width, height int) *Screen {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}

	s := &Screen{
		width:         width,
		height:        height,
		lines:         make([]*Line, height),
		cursorVisible: true,
		cursorStyle:   CursorBlock,
		scrollTop:     0,
		scrollBottom:  height - 1,
		currentFg:     DefaultForeground,
		currentBg:     DefaultBackground,
		autoWrap:      true,
	}

	for i := range s.lines {
		s.lines[i] = NewLine(width)
	}

	return s
}

// Width returns the screen width.
func (s *Screen) Width() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.width
}

// Height returns the screen height.
func (s *Screen) Height() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.height
}

// CursorPos returns the cursor position.
func (s *Screen) CursorPos() (x, y int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cursorX, s.cursorY
}

// CursorVisible returns whether the cursor is visible.
func (s *Screen) CursorVisible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cursorVisible
}

// Cell returns the cell at the given position.
// Returns an empty cell if out of bounds.
func (s *Screen) Cell(x, y int) Cell {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if x < 0 || x >= s.width || y < 0 || y >= s.height {
		return EmptyCell()
	}
	return s.lines[y].Cells[x]
}

// Line returns a copy of the line at the given y position.
func (s *Screen) Line(y int) []Cell {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if y < 0 || y >= s.height {
		return nil
	}

	cells := make([]Cell, len(s.lines[y].Cells))
	copy(cells, s.lines[y].Cells)
	return cells
}

// SetCell sets the cell at the given position.
func (s *Screen) SetCell(x, y int, cell Cell) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if x < 0 || x >= s.width || y < 0 || y >= s.height {
		return
	}
	s.lines[y].Cells[x] = cell
}

// WriteRune writes a rune at the cursor position and advances the cursor.
func (s *Screen) WriteRune(r rune) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writeRuneLocked(r)
}

func (s *Screen) writeRuneLocked(r rune) {
	// Bounds check
	if len(s.lines) == 0 || s.width == 0 {
		return
	}

	// Handle auto-wrap at end of line
	if s.cursorX >= s.width {
		if s.autoWrap {
			if s.cursorY >= 0 && s.cursorY < len(s.lines) {
				s.lines[s.cursorY].Wrapped = true
			}
			s.cursorX = 0
			s.lineFeedLocked()
		} else {
			s.cursorX = s.width - 1
		}
	}

	// Ensure cursor is in bounds after potential line feed
	if s.cursorY < 0 || s.cursorY >= len(s.lines) {
		return
	}
	if s.cursorX < 0 || s.cursorX >= len(s.lines[s.cursorY].Cells) {
		return
	}

	cell := Cell{
		Rune:       r,
		Width:      1,
		Foreground: s.currentFg,
		Background: s.currentBg,
		Attributes: s.currentAttrs,
	}

	s.lines[s.cursorY].Cells[s.cursorX] = cell
	s.cursorX++
}

// MoveCursor moves the cursor to the specified position.
func (s *Screen) MoveCursor(x, y int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.moveCursorLocked(x, y)
}

func (s *Screen) moveCursorLocked(x, y int) {
	// Clamp to valid range
	if x < 0 {
		x = 0
	}
	if x >= s.width {
		x = s.width - 1
	}

	// Handle origin mode
	top := 0
	bottom := s.height - 1
	if s.originMode {
		top = s.scrollTop
		bottom = s.scrollBottom
		y += top
	}

	if y < top {
		y = top
	}
	if y > bottom {
		y = bottom
	}

	s.cursorX = x
	s.cursorY = y
}

// MoveCursorRelative moves the cursor by the given delta.
func (s *Screen) MoveCursorRelative(dx, dy int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.moveCursorLocked(s.cursorX+dx, s.cursorY+dy)
}

// CarriageReturn moves cursor to beginning of current line.
func (s *Screen) CarriageReturn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorX = 0
}

// LineFeed moves cursor down one line, scrolling if needed.
func (s *Screen) LineFeed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lineFeedLocked()
}

func (s *Screen) lineFeedLocked() {
	if s.cursorY >= s.scrollBottom {
		// Scroll up
		s.scrollUpLocked(1)
	} else {
		s.cursorY++
	}
}

// ReverseLineFeed moves cursor up one line, scrolling if needed.
func (s *Screen) ReverseLineFeed() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursorY <= s.scrollTop {
		s.scrollDownLocked(1)
	} else {
		s.cursorY--
	}
}

// ScrollUp scrolls the scroll region up by n lines.
func (s *Screen) ScrollUp(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scrollUpLocked(n)
}

func (s *Screen) scrollUpLocked(n int) {
	if n <= 0 || len(s.lines) == 0 {
		return
	}

	top := s.scrollTop
	bottom := s.scrollBottom

	// Validate scroll region bounds
	if top < 0 {
		top = 0
	}
	if bottom >= len(s.lines) {
		bottom = len(s.lines) - 1
	}
	if top > bottom {
		return
	}

	// Clamp n to scroll region size
	regionSize := bottom - top + 1
	if n > regionSize {
		n = regionSize
	}

	// Move lines up
	for y := top; y <= bottom-n; y++ {
		s.lines[y] = s.lines[y+n]
	}

	// Create new blank lines at bottom
	for y := bottom - n + 1; y <= bottom; y++ {
		s.lines[y] = NewLine(s.width)
	}
}

// ScrollDown scrolls the scroll region down by n lines.
func (s *Screen) ScrollDown(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scrollDownLocked(n)
}

func (s *Screen) scrollDownLocked(n int) {
	if n <= 0 || len(s.lines) == 0 {
		return
	}

	top := s.scrollTop
	bottom := s.scrollBottom

	// Validate scroll region bounds
	if top < 0 {
		top = 0
	}
	if bottom >= len(s.lines) {
		bottom = len(s.lines) - 1
	}
	if top > bottom {
		return
	}

	// Clamp n to scroll region size
	regionSize := bottom - top + 1
	if n > regionSize {
		n = regionSize
	}

	// Move lines down
	for y := bottom; y >= top+n; y-- {
		s.lines[y] = s.lines[y-n]
	}

	// Create new blank lines at top
	for y := top; y < top+n; y++ {
		s.lines[y] = NewLine(s.width)
	}
}

// SetScrollRegion sets the scroll region.
func (s *Screen) SetScrollRegion(top, bottom int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if top < 0 {
		top = 0
	}
	if bottom >= s.height {
		bottom = s.height - 1
	}
	if top >= bottom {
		return
	}

	s.scrollTop = top
	s.scrollBottom = bottom

	// Reset cursor to top-left of region (or screen if not origin mode)
	if s.originMode {
		s.cursorX = 0
		s.cursorY = top
	} else {
		s.cursorX = 0
		s.cursorY = 0
	}
}

// ResetScrollRegion resets the scroll region to full screen.
func (s *Screen) ResetScrollRegion() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scrollTop = 0
	s.scrollBottom = s.height - 1
}

// ClearScreen clears the entire screen.
func (s *Screen) ClearScreen() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for y := 0; y < s.height; y++ {
		s.lines[y].Clear()
	}
}

// ClearScreenAbove clears from cursor to top of screen.
func (s *Screen) ClearScreenAbove() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear lines above cursor
	for y := 0; y < s.cursorY; y++ {
		s.lines[y].Clear()
	}

	// Clear current line up to and including cursor
	s.lines[s.cursorY].ClearRange(0, s.cursorX+1)
}

// ClearScreenBelow clears from cursor to bottom of screen.
func (s *Screen) ClearScreenBelow() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear current line from cursor to end
	s.lines[s.cursorY].ClearRange(s.cursorX, s.width)

	// Clear lines below cursor
	for y := s.cursorY + 1; y < s.height; y++ {
		s.lines[y].Clear()
	}
}

// ClearLine clears the entire current line.
func (s *Screen) ClearLine() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines[s.cursorY].Clear()
}

// ClearLineLeft clears from start of line to cursor.
func (s *Screen) ClearLineLeft() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines[s.cursorY].ClearRange(0, s.cursorX+1)
}

// ClearLineRight clears from cursor to end of line.
func (s *Screen) ClearLineRight() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines[s.cursorY].ClearRange(s.cursorX, s.width)
}

// InsertLines inserts n blank lines at cursor, scrolling down.
func (s *Screen) InsertLines(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursorY < s.scrollTop || s.cursorY > s.scrollBottom {
		return
	}

	// Insert blank lines at cursor position, push content down
	// Only affects lines from cursor to scroll bottom
	oldTop := s.scrollTop
	s.scrollTop = s.cursorY
	s.scrollDownLocked(n)
	s.scrollTop = oldTop
}

// DeleteLines deletes n lines at cursor, scrolling up.
func (s *Screen) DeleteLines(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursorY < s.scrollTop || s.cursorY > s.scrollBottom {
		return
	}

	// Temporarily adjust scroll top to cursor position
	oldTop := s.scrollTop
	s.scrollTop = s.cursorY
	s.scrollUpLocked(n)
	s.scrollTop = oldTop
}

// InsertChars inserts n blank characters at cursor.
func (s *Screen) InsertChars(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursorY < 0 || s.cursorY >= len(s.lines) {
		return
	}
	line := s.lines[s.cursorY]
	if n <= 0 || s.cursorX >= s.width {
		return
	}

	// Clamp n to available space
	maxInsert := s.width - s.cursorX
	if n > maxInsert {
		n = maxInsert
	}

	// Shift characters right
	for x := s.width - 1; x >= s.cursorX+n; x-- {
		line.Cells[x] = line.Cells[x-n]
	}

	// Insert blank cells
	for x := s.cursorX; x < s.cursorX+n && x < s.width; x++ {
		line.Cells[x] = EmptyCell()
	}
}

// DeleteChars deletes n characters at cursor, shifting left.
func (s *Screen) DeleteChars(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursorY < 0 || s.cursorY >= len(s.lines) {
		return
	}
	line := s.lines[s.cursorY]
	if n <= 0 || s.cursorX >= s.width {
		return
	}

	// Clamp n to prevent negative indices
	maxDelete := s.width - s.cursorX
	if n > maxDelete {
		n = maxDelete
	}

	// Shift characters left
	for x := s.cursorX; x < s.width-n; x++ {
		line.Cells[x] = line.Cells[x+n]
	}

	// Clear cells at end
	clearStart := s.width - n
	if clearStart < s.cursorX {
		clearStart = s.cursorX
	}
	for x := clearStart; x < s.width; x++ {
		line.Cells[x] = EmptyCell()
	}
}

// EraseChars erases n characters at cursor (replace with blanks).
func (s *Screen) EraseChars(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursorY < 0 || s.cursorY >= len(s.lines) {
		return
	}
	line := s.lines[s.cursorY]
	for x := s.cursorX; x < s.cursorX+n && x < s.width; x++ {
		line.Cells[x] = EmptyCell()
	}
}

// SetAttributes sets the current text attributes.
func (s *Screen) SetAttributes(fg, bg Color, attrs CellAttributes) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentFg = fg
	s.currentBg = bg
	s.currentAttrs = attrs
}

// SetForeground sets the current foreground color.
func (s *Screen) SetForeground(fg Color) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentFg = fg
}

// SetBackground sets the current background color.
func (s *Screen) SetBackground(bg Color) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentBg = bg
}

// AddAttribute adds an attribute to the current attributes.
func (s *Screen) AddAttribute(attr CellAttributes) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentAttrs |= attr
}

// RemoveAttribute removes an attribute from the current attributes.
func (s *Screen) RemoveAttribute(attr CellAttributes) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentAttrs &^= attr
}

// ResetAttributes resets all attributes to default.
func (s *Screen) ResetAttributes() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentFg = DefaultForeground
	s.currentBg = DefaultBackground
	s.currentAttrs = AttrNone
}

// SaveCursor saves the current cursor position and attributes.
func (s *Screen) SaveCursor() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.savedX = s.cursorX
	s.savedY = s.cursorY
	s.savedFg = s.currentFg
	s.savedBg = s.currentBg
	s.savedAttrs = s.currentAttrs
}

// RestoreCursor restores the saved cursor position and attributes.
func (s *Screen) RestoreCursor() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cursorX = s.savedX
	s.cursorY = s.savedY
	s.currentFg = s.savedFg
	s.currentBg = s.savedBg
	s.currentAttrs = s.savedAttrs
}

// SetCursorVisible sets cursor visibility.
func (s *Screen) SetCursorVisible(visible bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorVisible = visible
}

// SetCursorStyle sets the cursor style.
func (s *Screen) SetCursorStyle(style CursorStyle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorStyle = style
}

// SetOriginMode sets origin mode (cursor relative to scroll region).
func (s *Screen) SetOriginMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.originMode = enabled
}

// SetAutoWrap sets auto-wrap mode.
func (s *Screen) SetAutoWrap(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoWrap = enabled
}

// Resize resizes the screen.
func (s *Screen) Resize(width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	// Create new lines
	newLines := make([]*Line, height)
	for y := 0; y < height; y++ {
		if y < len(s.lines) {
			// Copy existing line (with nil check)
			oldLine := s.lines[y]
			newLines[y] = NewLine(width)
			if oldLine != nil {
				copyLen := width
				if len(oldLine.Cells) < copyLen {
					copyLen = len(oldLine.Cells)
				}
				for x := 0; x < copyLen; x++ {
					newLines[y].Cells[x] = oldLine.Cells[x]
				}
			}
		} else {
			newLines[y] = NewLine(width)
		}
	}

	s.lines = newLines
	s.width = width
	s.height = height

	// Adjust scroll region - ensure valid bounds and invariant
	if s.scrollTop < 0 {
		s.scrollTop = 0
	}
	if s.scrollTop >= height {
		s.scrollTop = 0
	}
	if s.scrollBottom >= height {
		s.scrollBottom = height - 1
	}
	if s.scrollBottom < 0 {
		s.scrollBottom = height - 1
	}
	// Ensure scrollTop <= scrollBottom
	if s.scrollTop > s.scrollBottom {
		s.scrollTop = 0
		s.scrollBottom = height - 1
	}

	// Adjust cursor
	if s.cursorX < 0 {
		s.cursorX = 0
	}
	if s.cursorX >= width {
		s.cursorX = width - 1
	}
	if s.cursorY < 0 {
		s.cursorY = 0
	}
	if s.cursorY >= height {
		s.cursorY = height - 1
	}

	// Adjust saved cursor state
	if s.savedX < 0 {
		s.savedX = 0
	}
	if s.savedX >= width {
		s.savedX = width - 1
	}
	if s.savedY < 0 {
		s.savedY = 0
	}
	if s.savedY >= height {
		s.savedY = height - 1
	}
}

// Reset resets the screen to initial state.
func (s *Screen) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for y := 0; y < s.height; y++ {
		s.lines[y].Clear()
	}

	s.cursorX = 0
	s.cursorY = 0
	s.cursorVisible = true
	s.cursorStyle = CursorBlock
	s.scrollTop = 0
	s.scrollBottom = s.height - 1
	s.currentFg = DefaultForeground
	s.currentBg = DefaultBackground
	s.currentAttrs = AttrNone
	s.originMode = false
	s.autoWrap = true
}

// GetText returns the text content of the screen as a string.
func (s *Screen) GetText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []rune
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			result = append(result, s.lines[y].Cells[x].Rune)
		}
		if y < s.height-1 {
			result = append(result, '\n')
		}
	}
	return string(result)
}

// GetTextRange returns the text in the given range.
func (s *Screen) GetTextRange(startX, startY, endX, endY int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []rune

	for y := startY; y <= endY && y < s.height; y++ {
		sX := 0
		eX := s.width - 1

		if y == startY {
			sX = startX
		}
		if y == endY {
			eX = endX
		}

		for x := sX; x <= eX && x < s.width; x++ {
			result = append(result, s.lines[y].Cells[x].Rune)
		}

		if y < endY {
			result = append(result, '\n')
		}
	}

	return string(result)
}
