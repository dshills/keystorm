// Package viewport provides viewport management for the renderer.
package viewport

import (
	"math"
	"sync"
)

// Viewport represents the visible portion of the buffer.
type Viewport struct {
	mu sync.RWMutex

	// Position in buffer (first visible line)
	topLine    uint32
	leftColumn int

	// Size in screen cells
	width  int
	height int

	// Scroll margins (keep cursor this far from edges)
	marginTop    int
	marginBottom int
	marginLeft   int
	marginRight  int

	// Scroll animation state
	targetTopLine    uint32
	targetLeftColumn int
	scrollVelY       float64
	scrollVelX       float64
	animating        bool
	smoothScroll     bool

	// Buffer size limits
	maxLine uint32
}

// NewViewport creates a viewport with the given size.
// Width and height are clamped to a minimum of 1 to prevent underflow.
func NewViewport(width, height int) *Viewport {
	// Ensure minimum dimensions to prevent underflow in calculations
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	return &Viewport{
		topLine:      0,
		leftColumn:   0,
		width:        width,
		height:       height,
		marginTop:    5,
		marginBottom: 5,
		marginLeft:   10,
		marginRight:  10,
		smoothScroll: true,
	}
}

// Width returns the viewport width.
func (v *Viewport) Width() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.width
}

// Height returns the viewport height.
func (v *Viewport) Height() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.height
}

// TopLine returns the first visible line.
func (v *Viewport) TopLine() uint32 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.topLine
}

// BottomLine returns the last visible line.
func (v *Viewport) BottomLine() uint32 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.bottomLine()
}

// bottomLine returns the last visible line (internal, no lock).
func (v *Viewport) bottomLine() uint32 {
	// Guard against zero height to prevent underflow
	if v.height < 1 {
		return v.topLine
	}
	bottom := v.topLine + uint32(v.height) - 1
	if v.maxLine > 0 && bottom > v.maxLine-1 {
		bottom = v.maxLine - 1
	}
	return bottom
}

// LeftColumn returns the first visible column.
func (v *Viewport) LeftColumn() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.leftColumn
}

// RightColumn returns the last visible column (exclusive).
func (v *Viewport) RightColumn() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.leftColumn + v.width
}

// Resize updates the viewport size.
// Width and height are clamped to a minimum of 1 to prevent underflow.
func (v *Viewport) Resize(width, height int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Ensure minimum dimensions to prevent underflow in calculations
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	v.width = width
	v.height = height
}

// SetMaxLine sets the maximum line number in the buffer.
func (v *Viewport) SetMaxLine(maxLine uint32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.maxLine = maxLine

	// Clamp topLine if needed
	if v.maxLine > 0 && v.topLine >= v.maxLine {
		if v.maxLine > 0 {
			v.topLine = v.maxLine - 1
		} else {
			v.topLine = 0
		}
	}
}

// SetMargins sets the scroll margins.
func (v *Viewport) SetMargins(top, bottom, left, right int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.marginTop = top
	v.marginBottom = bottom
	v.marginLeft = left
	v.marginRight = right
}

// Margins returns the current scroll margins.
func (v *Viewport) Margins() (top, bottom, left, right int) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.marginTop, v.marginBottom, v.marginLeft, v.marginRight
}

// SetSmoothScroll enables or disables smooth scrolling.
func (v *Viewport) SetSmoothScroll(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.smoothScroll = enabled
}

// SmoothScroll returns whether smooth scrolling is enabled.
func (v *Viewport) SmoothScroll() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.smoothScroll
}

// VisibleLineRange returns the range of visible buffer lines.
func (v *Viewport) VisibleLineRange() (start, end uint32) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.topLine, v.bottomLine()
}

// IsLineVisible returns true if the line is within the viewport.
func (v *Viewport) IsLineVisible(line uint32) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return line >= v.topLine && line <= v.bottomLine()
}

// IsColumnVisible returns true if the column is within the viewport.
func (v *Viewport) IsColumnVisible(col int) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return col >= v.leftColumn && col < v.leftColumn+v.width
}

// IsPositionVisible returns true if the position is within the viewport.
func (v *Viewport) IsPositionVisible(line uint32, col int) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return line >= v.topLine && line <= v.bottomLine() &&
		col >= v.leftColumn && col < v.leftColumn+v.width
}

// LineToScreenRow converts a buffer line to a screen row.
// Returns -1 if the line is not visible.
func (v *Viewport) LineToScreenRow(line uint32) int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if line < v.topLine || line > v.bottomLine() {
		return -1
	}
	return int(line - v.topLine)
}

// ScreenRowToLine converts a screen row to a buffer line.
func (v *Viewport) ScreenRowToLine(row int) uint32 {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if row < 0 {
		return v.topLine
	}
	line := v.topLine + uint32(row)
	if v.maxLine > 0 && line >= v.maxLine {
		line = v.maxLine - 1
	}
	return line
}

// ColumnToScreenCol converts a buffer column to a screen column.
func (v *Viewport) ColumnToScreenCol(col int) int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return col - v.leftColumn
}

// ScreenColToColumn converts a screen column to a buffer column.
func (v *Viewport) ScreenColToColumn(col int) int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return col + v.leftColumn
}

// BufferToScreen converts buffer coordinates to screen coordinates.
// Returns (-1, -1) if the position is not visible.
func (v *Viewport) BufferToScreen(line uint32, col int) (screenRow, screenCol int) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if line < v.topLine || line > v.bottomLine() {
		return -1, -1
	}
	if col < v.leftColumn || col >= v.leftColumn+v.width {
		return -1, -1
	}

	return int(line - v.topLine), col - v.leftColumn
}

// ScreenToBuffer converts screen coordinates to buffer coordinates.
func (v *Viewport) ScreenToBuffer(screenRow, screenCol int) (line uint32, col int) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	line = v.topLine + uint32(screenRow)
	col = v.leftColumn + screenCol
	return
}

// IsAnimating returns true if a scroll animation is in progress.
func (v *Viewport) IsAnimating() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.animating
}

// ScrollTo scrolls to show the given line at the top.
func (v *Viewport) ScrollTo(line uint32, smooth bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Clamp to valid range
	if v.maxLine > 0 && line >= v.maxLine {
		if v.maxLine > 0 {
			line = v.maxLine - 1
		} else {
			line = 0
		}
	}

	if smooth && v.smoothScroll {
		v.targetTopLine = line
		v.animating = true
		v.scrollVelY = 0
	} else {
		v.topLine = line
		v.targetTopLine = line
		v.animating = false
	}
}

// ScrollBy scrolls by a delta number of lines.
func (v *Viewport) ScrollBy(deltaLines int, smooth bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	newTop := int64(v.topLine) + int64(deltaLines)
	if newTop < 0 {
		newTop = 0
	}
	if v.maxLine > 0 && uint32(newTop) >= v.maxLine {
		newTop = int64(v.maxLine) - 1
		if newTop < 0 {
			newTop = 0
		}
	}

	if smooth && v.smoothScroll {
		v.targetTopLine = uint32(newTop)
		v.animating = true
	} else {
		v.topLine = uint32(newTop)
		v.targetTopLine = uint32(newTop)
		v.animating = false
	}
}

// ScrollHorizontalBy scrolls horizontally by a delta.
func (v *Viewport) ScrollHorizontalBy(deltaCols int, smooth bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	newLeft := v.leftColumn + deltaCols
	if newLeft < 0 {
		newLeft = 0
	}

	if smooth && v.smoothScroll {
		v.targetLeftColumn = newLeft
		v.animating = true
	} else {
		v.leftColumn = newLeft
		v.targetLeftColumn = newLeft
	}
}

// ScrollToReveal scrolls minimally to reveal a position.
// Uses margins to keep context around the target.
// Returns true if scrolling occurred.
func (v *Viewport) ScrollToReveal(line uint32, col int, smooth bool) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	needScroll := false
	targetTop := v.topLine
	targetLeft := v.leftColumn

	// Vertical scroll
	if line < v.topLine+uint32(v.marginTop) {
		// Scroll up
		margin := uint32(v.marginTop)
		if line >= margin {
			targetTop = line - margin
		} else {
			targetTop = 0
		}
		needScroll = true
	} else if line > v.bottomLine()-uint32(v.marginBottom) {
		// Scroll down
		if v.height > v.marginBottom {
			targetTop = line - uint32(v.height) + uint32(v.marginBottom) + 1
		} else {
			targetTop = line
		}
		needScroll = true
	}

	// Horizontal scroll
	screenCol := col - v.leftColumn
	if screenCol < v.marginLeft {
		targetLeft = col - v.marginLeft
		if targetLeft < 0 {
			targetLeft = 0
		}
		needScroll = true
	} else if screenCol > v.width-v.marginRight {
		targetLeft = col - v.width + v.marginRight
		needScroll = true
	}

	// Clamp vertical to valid range
	if v.maxLine > 0 && targetTop >= v.maxLine {
		targetTop = v.maxLine - 1
	}

	if needScroll {
		if smooth && v.smoothScroll {
			v.targetTopLine = targetTop
			v.targetLeftColumn = targetLeft
			v.animating = true
		} else {
			v.topLine = targetTop
			v.leftColumn = targetLeft
			v.targetTopLine = targetTop
			v.targetLeftColumn = targetLeft
			v.animating = false
		}
	}

	return needScroll
}

// CenterOn centers the viewport on the given line.
func (v *Viewport) CenterOn(line uint32, smooth bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	halfHeight := uint32(v.height / 2)
	var targetTop uint32
	if line >= halfHeight {
		targetTop = line - halfHeight
	} else {
		targetTop = 0
	}

	// Clamp to valid range
	if v.maxLine > 0 && targetTop >= v.maxLine {
		if v.maxLine > uint32(v.height) {
			targetTop = v.maxLine - uint32(v.height)
		} else {
			targetTop = 0
		}
	}

	if smooth && v.smoothScroll {
		v.targetTopLine = targetTop
		v.animating = true
	} else {
		v.topLine = targetTop
		v.targetTopLine = targetTop
		v.animating = false
	}
}

// Update advances scroll animation by dt seconds.
// Returns true if the viewport moved.
func (v *Viewport) Update(dt float64) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.animating {
		return false
	}

	moved := false

	// Vertical animation using exponential decay
	diffY := float64(int64(v.targetTopLine) - int64(v.topLine))
	if math.Abs(diffY) < 0.5 {
		if v.topLine != v.targetTopLine {
			v.topLine = v.targetTopLine
			moved = true
		}
	} else {
		// Exponential interpolation - move 20% of remaining distance per frame
		// This ensures convergence regardless of distance
		factor := 1.0 - math.Pow(0.1, dt*10) // ~20% per frame at 60fps
		moveY := diffY * factor

		// Ensure we move at least 1 line to prevent stalling
		if math.Abs(moveY) < 1.0 && math.Abs(diffY) >= 1.0 {
			if diffY > 0 {
				moveY = 1.0
			} else {
				moveY = -1.0
			}
		}

		if math.Abs(moveY) >= math.Abs(diffY) {
			v.topLine = v.targetTopLine
		} else {
			v.topLine = uint32(int64(v.topLine) + int64(moveY))
		}
		moved = true
	}

	// Horizontal animation using exponential decay
	diffX := float64(v.targetLeftColumn - v.leftColumn)
	if math.Abs(diffX) < 0.5 {
		if v.leftColumn != v.targetLeftColumn {
			v.leftColumn = v.targetLeftColumn
			moved = true
		}
	} else {
		// Exponential interpolation - same as vertical
		factor := 1.0 - math.Pow(0.1, dt*10)
		moveX := diffX * factor

		// Ensure we move at least 1 column to prevent stalling
		if math.Abs(moveX) < 1.0 && math.Abs(diffX) >= 1.0 {
			if diffX > 0 {
				moveX = 1.0
			} else {
				moveX = -1.0
			}
		}

		if math.Abs(moveX) >= math.Abs(diffX) {
			v.leftColumn = v.targetLeftColumn
		} else {
			v.leftColumn += int(moveX)
		}
		moved = true
	}

	// Check if animation is complete
	if v.topLine == v.targetTopLine && v.leftColumn == v.targetLeftColumn {
		v.animating = false
	}

	return moved
}

// StopAnimation stops any ongoing scroll animation.
func (v *Viewport) StopAnimation() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.animating = false
	v.targetTopLine = v.topLine
	v.targetLeftColumn = v.leftColumn
}

// PageUp scrolls up by one page (viewport height minus overlap).
func (v *Viewport) PageUp(smooth bool) {
	v.mu.RLock()
	pageSize := v.height - 2 // Keep 2 lines of overlap
	if pageSize < 1 {
		pageSize = 1
	}
	v.mu.RUnlock()

	v.ScrollBy(-pageSize, smooth)
}

// PageDown scrolls down by one page (viewport height minus overlap).
func (v *Viewport) PageDown(smooth bool) {
	v.mu.RLock()
	pageSize := v.height - 2 // Keep 2 lines of overlap
	if pageSize < 1 {
		pageSize = 1
	}
	v.mu.RUnlock()

	v.ScrollBy(pageSize, smooth)
}

// HalfPageUp scrolls up by half a page.
func (v *Viewport) HalfPageUp(smooth bool) {
	v.mu.RLock()
	halfPage := v.height / 2
	if halfPage < 1 {
		halfPage = 1
	}
	v.mu.RUnlock()

	v.ScrollBy(-halfPage, smooth)
}

// HalfPageDown scrolls down by half a page.
func (v *Viewport) HalfPageDown(smooth bool) {
	v.mu.RLock()
	halfPage := v.height / 2
	if halfPage < 1 {
		halfPage = 1
	}
	v.mu.RUnlock()

	v.ScrollBy(halfPage, smooth)
}

// ScrollToTop scrolls to the top of the buffer.
func (v *Viewport) ScrollToTop(smooth bool) {
	v.ScrollTo(0, smooth)
}

// ScrollToBottom scrolls to the bottom of the buffer.
func (v *Viewport) ScrollToBottom(smooth bool) {
	v.mu.RLock()
	maxLine := v.maxLine
	height := v.height
	v.mu.RUnlock()

	if maxLine == 0 {
		v.ScrollTo(0, smooth)
		return
	}

	var targetLine uint32
	if maxLine > uint32(height) {
		targetLine = maxLine - uint32(height)
	} else {
		targetLine = 0
	}
	v.ScrollTo(targetLine, smooth)
}

// Clone creates a copy of the viewport state.
func (v *Viewport) Clone() *Viewport {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return &Viewport{
		topLine:          v.topLine,
		leftColumn:       v.leftColumn,
		width:            v.width,
		height:           v.height,
		marginTop:        v.marginTop,
		marginBottom:     v.marginBottom,
		marginLeft:       v.marginLeft,
		marginRight:      v.marginRight,
		targetTopLine:    v.targetTopLine,
		targetLeftColumn: v.targetLeftColumn,
		scrollVelY:       v.scrollVelY,
		scrollVelX:       v.scrollVelX,
		animating:        v.animating,
		smoothScroll:     v.smoothScroll,
		maxLine:          v.maxLine,
	}
}
