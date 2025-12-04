package mouse

import "time"

// clickTracker tracks click patterns for double/triple click detection.
type clickTracker struct {
	// Configuration
	maxTime     time.Duration
	maxDistance int

	// Last click state
	lastPos   Position
	lastTime  time.Time
	lastCount int
}

// newClickTracker creates a new click tracker.
func newClickTracker(maxTime time.Duration, maxDistance int) *clickTracker {
	return &clickTracker{
		maxTime:     maxTime,
		maxDistance: maxDistance,
	}
}

// recordClick records a click and returns the click count (1, 2, or 3).
// Click count wraps back to 1 after 3 (quad-click = single click).
// Requires a valid (non-zero) timestamp for proper double/triple click detection.
// If timestamp is zero, uses time.Now() as fallback.
func (t *clickTracker) recordClick(pos Position, timestamp time.Time) int {
	// Ensure valid timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// Check if this click is part of a sequence
	if t.isPartOfSequence(pos, timestamp) {
		// Increment count, wrapping after 3
		t.lastCount++
		if t.lastCount > 3 {
			t.lastCount = 1
		}
	} else {
		// New click sequence
		t.lastCount = 1
	}

	// Update state
	t.lastPos = pos
	t.lastTime = timestamp

	return t.lastCount
}

// isPartOfSequence checks if a click is part of the current click sequence.
func (t *clickTracker) isPartOfSequence(pos Position, timestamp time.Time) bool {
	// No previous click or invalid previous timestamp
	if t.lastCount == 0 || t.lastTime.IsZero() {
		return false
	}

	// Check time threshold
	// Handle clock skew: if elapsed time is negative, treat as new sequence
	elapsed := timestamp.Sub(t.lastTime)
	if elapsed < 0 || elapsed > t.maxTime {
		return false
	}

	// Check distance threshold (Manhattan distance)
	if pos.Distance(t.lastPos) > t.maxDistance {
		return false
	}

	return true
}

// reset clears the click tracking state.
func (t *clickTracker) reset() {
	t.lastCount = 0
	t.lastTime = time.Time{}
	t.lastPos = Position{}
}

// lastClickCount returns the last recorded click count.
func (t *clickTracker) lastClickCount() int {
	return t.lastCount
}

// ClickType represents the type of click detected.
type ClickType uint8

const (
	// ClickSingle is a single click.
	ClickSingle ClickType = 1
	// ClickDouble is a double click.
	ClickDouble ClickType = 2
	// ClickTriple is a triple click.
	ClickTriple ClickType = 3
)

// String returns a string representation of the click type.
func (c ClickType) String() string {
	switch c {
	case ClickSingle:
		return "single"
	case ClickDouble:
		return "double"
	case ClickTriple:
		return "triple"
	default:
		return "unknown"
	}
}
