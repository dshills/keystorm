package backend

import (
	"github.com/dshills/keystorm/internal/renderer/core"
)

// ScreenBuffer provides double-buffered rendering with change tracking.
// It maintains two buffers: front (displayed) and back (drawing).
// On sync, it computes the diff and only updates changed cells.
type ScreenBuffer struct {
	width, height int
	front         [][]core.Cell
	back          [][]core.Cell
	dirty         [][]bool
	fullRedraw    bool
}

// NewScreenBuffer creates a screen buffer with the given dimensions.
func NewScreenBuffer(width, height int) *ScreenBuffer {
	sb := &ScreenBuffer{
		width:      width,
		height:     height,
		fullRedraw: true,
	}
	sb.allocate()
	return sb
}

// allocate creates the internal buffers.
func (sb *ScreenBuffer) allocate() {
	sb.front = make([][]core.Cell, sb.height)
	sb.back = make([][]core.Cell, sb.height)
	sb.dirty = make([][]bool, sb.height)

	for y := 0; y < sb.height; y++ {
		sb.front[y] = make([]core.Cell, sb.width)
		sb.back[y] = make([]core.Cell, sb.width)
		sb.dirty[y] = make([]bool, sb.width)

		for x := 0; x < sb.width; x++ {
			sb.front[y][x] = core.EmptyCell()
			sb.back[y][x] = core.EmptyCell()
		}
	}
}

// Resize resizes the buffer, preserving content where possible.
func (sb *ScreenBuffer) Resize(width, height int) {
	if width == sb.width && height == sb.height {
		return
	}

	oldBack := sb.back
	oldWidth := sb.width
	oldHeight := sb.height

	sb.width = width
	sb.height = height
	sb.allocate()

	// Copy preserved content
	copyHeight := min(oldHeight, height)
	copyWidth := min(oldWidth, width)
	for y := 0; y < copyHeight; y++ {
		for x := 0; x < copyWidth; x++ {
			sb.back[y][x] = oldBack[y][x]
		}
	}

	sb.fullRedraw = true
}

// Size returns the buffer dimensions.
func (sb *ScreenBuffer) Size() (width, height int) {
	return sb.width, sb.height
}

// SetCell sets a cell in the back buffer.
func (sb *ScreenBuffer) SetCell(x, y int, cell core.Cell) {
	if x < 0 || x >= sb.width || y < 0 || y >= sb.height {
		return
	}
	sb.back[y][x] = cell
	sb.dirty[y][x] = true
}

// GetCell returns a cell from the back buffer.
func (sb *ScreenBuffer) GetCell(x, y int) core.Cell {
	if x < 0 || x >= sb.width || y < 0 || y >= sb.height {
		return core.EmptyCell()
	}
	return sb.back[y][x]
}

// GetFrontCell returns a cell from the front buffer (currently displayed).
func (sb *ScreenBuffer) GetFrontCell(x, y int) core.Cell {
	if x < 0 || x >= sb.width || y < 0 || y >= sb.height {
		return core.EmptyCell()
	}
	return sb.front[y][x]
}

// Fill fills a rectangle with the given cell.
func (sb *ScreenBuffer) Fill(rect core.ScreenRect, cell core.Cell) {
	for y := rect.Top; y < rect.Bottom && y < sb.height; y++ {
		for x := rect.Left; x < rect.Right && x < sb.width; x++ {
			if x >= 0 && y >= 0 {
				sb.back[y][x] = cell
				sb.dirty[y][x] = true
			}
		}
	}
}

// Clear clears the back buffer with empty cells.
func (sb *ScreenBuffer) Clear() {
	empty := core.EmptyCell()
	for y := 0; y < sb.height; y++ {
		for x := 0; x < sb.width; x++ {
			sb.back[y][x] = empty
			sb.dirty[y][x] = true
		}
	}
}

// ClearRegion clears a rectangular region.
func (sb *ScreenBuffer) ClearRegion(rect core.ScreenRect) {
	sb.Fill(rect, core.EmptyCell())
}

// SetLine sets a row of cells starting at the given position.
func (sb *ScreenBuffer) SetLine(x, y int, cells []core.Cell) {
	if y < 0 || y >= sb.height {
		return
	}
	for i, cell := range cells {
		col := x + i
		if col >= 0 && col < sb.width {
			sb.back[y][col] = cell
			sb.dirty[y][col] = true
		}
	}
}

// SetString writes a string with the given style starting at the position.
func (sb *ScreenBuffer) SetString(x, y int, s string, style core.Style) {
	if y < 0 || y >= sb.height {
		return
	}
	col := x
	for _, r := range s {
		if col < 0 {
			col++
			continue
		}
		if col >= sb.width {
			break
		}

		width := core.RuneWidth(r)
		sb.back[y][col] = core.Cell{
			Rune:  r,
			Width: width,
			Style: style,
		}
		sb.dirty[y][col] = true
		col++

		// Handle wide characters
		if width == 2 && col < sb.width {
			sb.back[y][col] = core.ContinuationCell()
			sb.dirty[y][col] = true
			col++
		}
	}
}

// DiffChange represents a cell change for synchronization.
type DiffChange struct {
	X, Y int
	Cell core.Cell
}

// ComputeDiff returns the changes needed to update the display.
// Returns nil if no changes are needed.
func (sb *ScreenBuffer) ComputeDiff() []DiffChange {
	var changes []DiffChange

	for y := 0; y < sb.height; y++ {
		for x := 0; x < sb.width; x++ {
			if sb.fullRedraw || sb.dirty[y][x] {
				if sb.fullRedraw || !sb.back[y][x].Equals(sb.front[y][x]) {
					changes = append(changes, DiffChange{
						X:    x,
						Y:    y,
						Cell: sb.back[y][x],
					})
				}
			}
		}
	}

	return changes
}

// Sync copies the back buffer to the front buffer and clears dirty flags.
// Call this after applying changes to the backend.
func (sb *ScreenBuffer) Sync() {
	for y := 0; y < sb.height; y++ {
		for x := 0; x < sb.width; x++ {
			sb.front[y][x] = sb.back[y][x]
			sb.dirty[y][x] = false
		}
	}
	sb.fullRedraw = false
}

// MarkDirty marks a cell as dirty (needs redraw).
func (sb *ScreenBuffer) MarkDirty(x, y int) {
	if x >= 0 && x < sb.width && y >= 0 && y < sb.height {
		sb.dirty[y][x] = true
	}
}

// MarkRegionDirty marks a rectangular region as dirty.
func (sb *ScreenBuffer) MarkRegionDirty(rect core.ScreenRect) {
	for y := rect.Top; y < rect.Bottom && y < sb.height; y++ {
		for x := rect.Left; x < rect.Right && x < sb.width; x++ {
			if x >= 0 && y >= 0 {
				sb.dirty[y][x] = true
			}
		}
	}
}

// MarkFullRedraw forces a complete redraw on next sync.
func (sb *ScreenBuffer) MarkFullRedraw() {
	sb.fullRedraw = true
}

// IsDirty returns true if there are pending changes.
func (sb *ScreenBuffer) IsDirty() bool {
	if sb.fullRedraw {
		return true
	}
	for y := 0; y < sb.height; y++ {
		for x := 0; x < sb.width; x++ {
			if sb.dirty[y][x] {
				return true
			}
		}
	}
	return false
}

// DirtyCount returns the number of dirty cells.
func (sb *ScreenBuffer) DirtyCount() int {
	if sb.fullRedraw {
		return sb.width * sb.height
	}
	count := 0
	for y := 0; y < sb.height; y++ {
		for x := 0; x < sb.width; x++ {
			if sb.dirty[y][x] {
				count++
			}
		}
	}
	return count
}

// BufferedBackend wraps a Backend with double-buffered rendering.
type BufferedBackend struct {
	backend Backend
	buffer  *ScreenBuffer
}

// NewBufferedBackend creates a buffered wrapper around a backend.
func NewBufferedBackend(backend Backend) *BufferedBackend {
	width, height := backend.Size()
	return &BufferedBackend{
		backend: backend,
		buffer:  NewScreenBuffer(width, height),
	}
}

func (b *BufferedBackend) Init() error {
	if err := b.backend.Init(); err != nil {
		return err
	}
	width, height := b.backend.Size()
	b.buffer.Resize(width, height)
	b.backend.OnResize(func(w, h int) {
		b.buffer.Resize(w, h)
	})
	return nil
}

func (b *BufferedBackend) Shutdown() {
	b.backend.Shutdown()
}

func (b *BufferedBackend) Size() (int, int) {
	return b.buffer.Size()
}

func (b *BufferedBackend) OnResize(callback func(width, height int)) {
	b.backend.OnResize(func(w, h int) {
		b.buffer.Resize(w, h)
		callback(w, h)
	})
}

func (b *BufferedBackend) SetCell(x, y int, cell core.Cell) {
	b.buffer.SetCell(x, y, cell)
}

func (b *BufferedBackend) GetCell(x, y int) core.Cell {
	return b.buffer.GetCell(x, y)
}

func (b *BufferedBackend) Fill(rect core.ScreenRect, cell core.Cell) {
	b.buffer.Fill(rect, cell)
}

func (b *BufferedBackend) Clear() {
	b.buffer.Clear()
}

// Show computes the diff and applies only changed cells to the backend.
func (b *BufferedBackend) Show() {
	changes := b.buffer.ComputeDiff()
	for _, ch := range changes {
		b.backend.SetCell(ch.X, ch.Y, ch.Cell)
	}
	b.buffer.Sync()
	b.backend.Show()
}

func (b *BufferedBackend) ShowCursor(x, y int) {
	b.backend.ShowCursor(x, y)
}

func (b *BufferedBackend) HideCursor() {
	b.backend.HideCursor()
}

func (b *BufferedBackend) SetCursorStyle(style CursorStyle) {
	b.backend.SetCursorStyle(style)
}

func (b *BufferedBackend) PollEvent() Event {
	return b.backend.PollEvent()
}

func (b *BufferedBackend) PostEvent(event Event) {
	b.backend.PostEvent(event)
}

func (b *BufferedBackend) HasTrueColor() bool {
	return b.backend.HasTrueColor()
}

func (b *BufferedBackend) Beep() {
	b.backend.Beep()
}

func (b *BufferedBackend) EnableMouse() {
	b.backend.EnableMouse()
}

func (b *BufferedBackend) DisableMouse() {
	b.backend.DisableMouse()
}

func (b *BufferedBackend) EnablePaste() {
	b.backend.EnablePaste()
}

func (b *BufferedBackend) DisablePaste() {
	b.backend.DisablePaste()
}

func (b *BufferedBackend) Suspend() error {
	return b.backend.Suspend()
}

func (b *BufferedBackend) Resume() error {
	return b.backend.Resume()
}

// Buffer returns the underlying screen buffer for direct access.
func (b *BufferedBackend) Buffer() *ScreenBuffer {
	return b.buffer
}

// SetString is a convenience method to write a string.
func (b *BufferedBackend) SetString(x, y int, s string, style core.Style) {
	b.buffer.SetString(x, y, s, style)
}

// SetLine is a convenience method to write a line of cells.
func (b *BufferedBackend) SetLine(x, y int, cells []core.Cell) {
	b.buffer.SetLine(x, y, cells)
}

// MarkDirty marks a cell as needing redraw.
func (b *BufferedBackend) MarkDirty(x, y int) {
	b.buffer.MarkDirty(x, y)
}

// MarkRegionDirty marks a region as needing redraw.
func (b *BufferedBackend) MarkRegionDirty(rect core.ScreenRect) {
	b.buffer.MarkRegionDirty(rect)
}

// MarkFullRedraw forces a complete redraw.
func (b *BufferedBackend) MarkFullRedraw() {
	b.buffer.MarkFullRedraw()
}
