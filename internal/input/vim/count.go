package vim

import "math"

// CountState tracks count prefix accumulation during parsing.
type CountState struct {
	// Value is the accumulated count value.
	Value int

	// Active indicates if a count is being accumulated.
	Active bool
}

// NewCountState creates a new count state.
func NewCountState() *CountState {
	return &CountState{}
}

// Reset clears the count state.
func (c *CountState) Reset() {
	c.Value = 0
	c.Active = false
}

// AccumulateDigit adds a digit to the count.
// Returns true if the digit was accepted.
// Only accepts ASCII digits 0-9.
func (c *CountState) AccumulateDigit(r rune) bool {
	// Only accept ASCII digits for consistency with keyboard input
	if r < '0' || r > '9' {
		return false
	}

	digit := int(r - '0')

	// Special case: '0' at the start is not a count, it's a motion
	if !c.Active && digit == 0 {
		return false
	}

	c.Active = true

	// Guard against integer overflow
	if c.Value > (math.MaxInt-digit)/10 {
		// Cap at a reasonable maximum rather than overflow
		c.Value = math.MaxInt / 10
		return true
	}

	c.Value = c.Value*10 + digit
	return true
}

// Get returns the effective count (1 if no count was specified).
func (c *CountState) Get() int {
	if c.Value <= 0 {
		return 1
	}
	return c.Value
}

// Multiply multiplies two counts, handling the case where one or both
// might be zero (meaning 1).
func (c *CountState) Multiply(other int) int {
	count := c.Get()
	if other <= 0 {
		other = 1
	}
	return count * other
}

// IsCountStart returns true if the character could start a count.
// Note: '0' cannot start a count (it's a motion to line start).
func IsCountStart(r rune) bool {
	return r >= '1' && r <= '9'
}

// IsCountDigit returns true if the character is a digit valid in a count.
func IsCountDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// ParseCountFromRunes parses a count from a sequence of runes.
// Returns the count value and the number of runes consumed.
func ParseCountFromRunes(runes []rune) (count int, consumed int) {
	if len(runes) == 0 {
		return 0, 0
	}

	// First character must be 1-9 (0 is a motion)
	if !IsCountStart(runes[0]) {
		return 0, 0
	}

	count = int(runes[0] - '0')
	consumed = 1

	for i := 1; i < len(runes); i++ {
		if !IsCountDigit(runes[i]) {
			break
		}
		count = count*10 + int(runes[i]-'0')
		consumed++
	}

	return count, consumed
}

// CombineCounts multiplies two counts together with overflow protection.
// This is used when both a pre-operator count and post-operator count exist.
// e.g., "2d3w" = delete (2*3=6) words
func CombineCounts(count1, count2 int) int {
	if count1 <= 0 {
		count1 = 1
	}
	if count2 <= 0 {
		count2 = 1
	}

	// Guard against overflow
	if count1 > math.MaxInt/count2 {
		return math.MaxInt / 10 // Cap at reasonable maximum
	}

	return count1 * count2
}
