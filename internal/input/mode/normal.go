package mode

import (
	"github.com/dshills/keystorm/internal/input/key"
)

// NormalMode implements Vim's normal mode.
// In normal mode, keys are interpreted as commands rather than text input.
type NormalMode struct {
	// pendingOperator holds the operator waiting for a motion/text object.
	pendingOperator string

	// count holds the numeric prefix for the next command.
	count int
}

// NewNormalMode creates a new normal mode instance.
func NewNormalMode() *NormalMode {
	return &NormalMode{}
}

// Name returns the mode identifier.
func (m *NormalMode) Name() string {
	return ModeNormal
}

// DisplayName returns the human-readable mode name.
func (m *NormalMode) DisplayName() string {
	return "NORMAL"
}

// CursorStyle returns the cursor style for normal mode.
func (m *NormalMode) CursorStyle() CursorStyle {
	return CursorBlock
}

// Enter is called when entering normal mode.
func (m *NormalMode) Enter(ctx *Context) error {
	// Reset state
	m.pendingOperator = ""
	m.count = 0
	return nil
}

// Exit is called when leaving normal mode.
func (m *NormalMode) Exit(ctx *Context) error {
	// Clear any pending state
	m.pendingOperator = ""
	m.count = 0
	return nil
}

// HandleUnmapped handles key events that have no explicit binding.
func (m *NormalMode) HandleUnmapped(event key.Event, ctx *Context) *UnmappedResult {
	// In normal mode, unmapped keys are generally ignored
	// However, we handle some special cases:

	// Digits build up the count prefix (except 0 which is a command)
	if event.IsRune() && !event.IsModified() {
		r := event.Rune
		if r >= '1' && r <= '9' {
			m.count = m.count*10 + int(r-'0')
			return &UnmappedResult{Consumed: true}
		}
		if r == '0' && m.count > 0 {
			m.count = m.count * 10
			return &UnmappedResult{Consumed: true}
		}
	}

	// Unmapped keys in normal mode are ignored
	return &UnmappedResult{Consumed: false}
}

// PendingOperator returns the currently pending operator, if any.
func (m *NormalMode) PendingOperator() string {
	return m.pendingOperator
}

// SetPendingOperator sets the pending operator.
func (m *NormalMode) SetPendingOperator(op string) {
	m.pendingOperator = op
}

// ClearPendingOperator clears the pending operator.
func (m *NormalMode) ClearPendingOperator() {
	m.pendingOperator = ""
}

// Count returns the current count prefix.
func (m *NormalMode) Count() int {
	if m.count == 0 {
		return 1 // Default count is 1
	}
	return m.count
}

// SetCount sets the count prefix.
func (m *NormalMode) SetCount(count int) {
	m.count = count
}

// ClearCount clears the count prefix.
func (m *NormalMode) ClearCount() {
	m.count = 0
}

// ResetState clears all pending state (operator and count).
func (m *NormalMode) ResetState() {
	m.pendingOperator = ""
	m.count = 0
}
