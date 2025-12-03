package viewport

// ScrollState represents the current scroll state.
type ScrollState struct {
	// Current position
	TopLine    uint32
	LeftColumn int

	// Target position (for animation)
	TargetTopLine    uint32
	TargetLeftColumn int

	// Animation
	Animating bool
	VelocityY float64
	VelocityX float64
}

// GetScrollState returns the current scroll state.
func (v *Viewport) GetScrollState() ScrollState {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return ScrollState{
		TopLine:          v.topLine,
		LeftColumn:       v.leftColumn,
		TargetTopLine:    v.targetTopLine,
		TargetLeftColumn: v.targetLeftColumn,
		Animating:        v.animating,
		VelocityY:        v.scrollVelY,
		VelocityX:        v.scrollVelX,
	}
}

// SetScrollState sets the scroll state directly.
func (v *Viewport) SetScrollState(state ScrollState) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.topLine = state.TopLine
	v.leftColumn = state.LeftColumn
	v.targetTopLine = state.TargetTopLine
	v.targetLeftColumn = state.TargetLeftColumn
	v.animating = state.Animating
	v.scrollVelY = state.VelocityY
	v.scrollVelX = state.VelocityX
}

// ScrollProgress returns the animation progress (0.0 to 1.0).
// Returns 1.0 if not animating.
func (v *Viewport) ScrollProgress() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.animating {
		return 1.0
	}

	// Calculate progress based on distance remaining
	totalDistY := float64(int64(v.targetTopLine) - int64(v.topLine))
	totalDistX := float64(v.targetLeftColumn - v.leftColumn)

	if totalDistY == 0 && totalDistX == 0 {
		return 1.0
	}

	// This is a rough approximation since we don't track starting position
	// In a real implementation, you might want to track the start position
	return 0.5
}

// ScrollDirection represents the scroll direction.
type ScrollDirection uint8

const (
	ScrollNone ScrollDirection = iota
	ScrollUp
	ScrollDown
	ScrollLeft
	ScrollRight
)

// ScrollingDirection returns the current scroll direction.
func (v *Viewport) ScrollingDirection() (vertical, horizontal ScrollDirection) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.animating {
		return ScrollNone, ScrollNone
	}

	// Vertical
	if v.targetTopLine > v.topLine {
		vertical = ScrollDown
	} else if v.targetTopLine < v.topLine {
		vertical = ScrollUp
	} else {
		vertical = ScrollNone
	}

	// Horizontal
	if v.targetLeftColumn > v.leftColumn {
		horizontal = ScrollRight
	} else if v.targetLeftColumn < v.leftColumn {
		horizontal = ScrollLeft
	} else {
		horizontal = ScrollNone
	}

	return
}

// LineScrollOffset returns the fractional line offset during smooth scroll.
// Returns 0.0 when not animating or at exact line boundary.
// Used for sub-pixel smooth scrolling (if supported by backend).
func (v *Viewport) LineScrollOffset() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// For integer-based scrolling, always return 0
	// This could be extended for sub-pixel scrolling
	return 0.0
}

// EnsureLineVisible ensures the given line is visible in the viewport.
// Returns true if scrolling was needed.
func (v *Viewport) EnsureLineVisible(line uint32, smooth bool) bool {
	v.mu.RLock()
	visible := line >= v.topLine && line <= v.bottomLine()
	v.mu.RUnlock()

	if visible {
		return false
	}

	return v.ScrollToReveal(line, 0, smooth)
}

// EnsureRangeVisible ensures a range of lines is visible.
// Prioritizes keeping the start line visible if the range is larger than viewport.
func (v *Viewport) EnsureRangeVisible(startLine, endLine uint32, smooth bool) bool {
	v.mu.RLock()
	height := uint32(v.height)
	topLine := v.topLine
	bottomLine := v.bottomLine()
	v.mu.RUnlock()

	// If range fits in viewport, center it
	rangeSize := endLine - startLine + 1
	if rangeSize <= height {
		// Check if already visible
		if startLine >= topLine && endLine <= bottomLine {
			return false
		}

		// Center the range
		centerLine := startLine + rangeSize/2
		v.CenterOn(centerLine, smooth)
		return true
	}

	// Range is larger than viewport, show start
	if startLine >= topLine && startLine <= bottomLine {
		return false
	}

	return v.ScrollToReveal(startLine, 0, smooth)
}

// ScrollPercent returns how far through the document we've scrolled (0.0 to 1.0).
func (v *Viewport) ScrollPercent() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.maxLine == 0 {
		return 0.0
	}

	maxScroll := v.maxLine
	if v.maxLine > uint32(v.height) {
		maxScroll = v.maxLine - uint32(v.height)
	} else {
		return 0.0
	}

	return float64(v.topLine) / float64(maxScroll)
}

// ScrollToPercent scrolls to a percentage of the document.
func (v *Viewport) ScrollToPercent(percent float64, smooth bool) {
	v.mu.RLock()
	maxLine := v.maxLine
	height := v.height
	v.mu.RUnlock()

	if maxLine == 0 {
		return
	}

	// Clamp percent
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	var maxScroll uint32
	if maxLine > uint32(height) {
		maxScroll = maxLine - uint32(height)
	} else {
		maxScroll = 0
	}

	targetLine := uint32(float64(maxScroll) * percent)
	v.ScrollTo(targetLine, smooth)
}
