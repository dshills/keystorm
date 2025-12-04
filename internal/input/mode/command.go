package mode

import (
	"unicode"

	"github.com/dshills/keystorm/internal/input/key"
)

// CommandMode implements Vim's command-line mode (ex commands).
// Activated with ":" in normal mode.
type CommandMode struct {
	// buffer holds the command being typed.
	buffer []rune

	// cursorPos is the cursor position within the command buffer.
	cursorPos int

	// history holds previous commands.
	history []string

	// historyIndex is the current position in history (-1 = current input).
	historyIndex int

	// savedBuffer holds the buffer when navigating history.
	savedBuffer []rune

	// prompt is the command prompt character (usually ':').
	prompt rune
}

// NewCommandMode creates a new command mode instance.
func NewCommandMode() *CommandMode {
	return &CommandMode{
		buffer:       make([]rune, 0, 64),
		history:      make([]string, 0, 100),
		historyIndex: -1,
		prompt:       ':',
	}
}

// Name returns the mode identifier.
func (m *CommandMode) Name() string {
	return ModeCommand
}

// DisplayName returns the human-readable mode name.
func (m *CommandMode) DisplayName() string {
	return "COMMAND"
}

// CursorStyle returns the cursor style for command mode.
func (m *CommandMode) CursorStyle() CursorStyle {
	return CursorBar
}

// Enter is called when entering command mode.
func (m *CommandMode) Enter(ctx *Context) error {
	m.buffer = m.buffer[:0]
	m.cursorPos = 0
	m.historyIndex = -1
	m.savedBuffer = nil
	return nil
}

// Exit is called when leaving command mode.
func (m *CommandMode) Exit(ctx *Context) error {
	return nil
}

// HandleUnmapped handles key events that have no explicit binding.
func (m *CommandMode) HandleUnmapped(event key.Event, ctx *Context) *UnmappedResult {
	// Handle character input
	if event.IsRune() && !event.IsModified() {
		r := event.Rune
		if unicode.IsPrint(r) {
			m.insertRune(r)
			return &UnmappedResult{Consumed: true}
		}
	}

	// Space is printable
	if event.Key == key.KeySpace && !event.IsModified() {
		m.insertRune(' ')
		return &UnmappedResult{Consumed: true}
	}

	return &UnmappedResult{Consumed: false}
}

// insertRune inserts a character at the cursor position.
func (m *CommandMode) insertRune(r rune) {
	if m.cursorPos >= len(m.buffer) {
		m.buffer = append(m.buffer, r)
	} else {
		m.buffer = append(m.buffer[:m.cursorPos+1], m.buffer[m.cursorPos:]...)
		m.buffer[m.cursorPos] = r
	}
	m.cursorPos++
}

// Buffer returns the current command buffer content.
func (m *CommandMode) Buffer() string {
	return string(m.buffer)
}

// SetBuffer sets the command buffer content.
func (m *CommandMode) SetBuffer(s string) {
	m.buffer = []rune(s)
	m.cursorPos = len(m.buffer)
}

// CursorPos returns the cursor position in the command buffer.
func (m *CommandMode) CursorPos() int {
	return m.cursorPos
}

// SetCursorPos sets the cursor position in the command buffer.
func (m *CommandMode) SetCursorPos(pos int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(m.buffer) {
		pos = len(m.buffer)
	}
	m.cursorPos = pos
}

// Prompt returns the command prompt character.
func (m *CommandMode) Prompt() rune {
	return m.prompt
}

// SetPrompt sets the command prompt character.
func (m *CommandMode) SetPrompt(r rune) {
	m.prompt = r
}

// Clear clears the command buffer.
func (m *CommandMode) Clear() {
	m.buffer = m.buffer[:0]
	m.cursorPos = 0
}

// Backspace deletes the character before the cursor.
func (m *CommandMode) Backspace() bool {
	if m.cursorPos == 0 {
		return false
	}
	m.buffer = append(m.buffer[:m.cursorPos-1], m.buffer[m.cursorPos:]...)
	m.cursorPos--
	return true
}

// Delete deletes the character at the cursor.
func (m *CommandMode) Delete() bool {
	if m.cursorPos >= len(m.buffer) {
		return false
	}
	m.buffer = append(m.buffer[:m.cursorPos], m.buffer[m.cursorPos+1:]...)
	return true
}

// MoveLeft moves the cursor left.
func (m *CommandMode) MoveLeft() bool {
	if m.cursorPos == 0 {
		return false
	}
	m.cursorPos--
	return true
}

// MoveRight moves the cursor right.
func (m *CommandMode) MoveRight() bool {
	if m.cursorPos >= len(m.buffer) {
		return false
	}
	m.cursorPos++
	return true
}

// MoveToStart moves the cursor to the start.
func (m *CommandMode) MoveToStart() {
	m.cursorPos = 0
}

// MoveToEnd moves the cursor to the end.
func (m *CommandMode) MoveToEnd() {
	m.cursorPos = len(m.buffer)
}

// AddToHistory adds a command to the history.
func (m *CommandMode) AddToHistory(cmd string) {
	if cmd == "" {
		return
	}
	// Don't add duplicates of the last command
	if len(m.history) > 0 && m.history[len(m.history)-1] == cmd {
		return
	}
	m.history = append(m.history, cmd)
}

// HistoryPrev moves to the previous history entry.
func (m *CommandMode) HistoryPrev() bool {
	if len(m.history) == 0 {
		return false
	}

	if m.historyIndex == -1 {
		// Save current buffer
		m.savedBuffer = make([]rune, len(m.buffer))
		copy(m.savedBuffer, m.buffer)
		m.historyIndex = len(m.history) - 1
	} else if m.historyIndex > 0 {
		m.historyIndex--
	} else {
		return false
	}

	m.SetBuffer(m.history[m.historyIndex])
	return true
}

// HistoryNext moves to the next history entry.
func (m *CommandMode) HistoryNext() bool {
	if m.historyIndex == -1 {
		return false
	}

	m.historyIndex++
	if m.historyIndex >= len(m.history) {
		// Restore saved buffer
		m.historyIndex = -1
		if m.savedBuffer != nil {
			m.buffer = m.savedBuffer
			m.cursorPos = len(m.buffer)
			m.savedBuffer = nil
		} else {
			m.Clear()
		}
	} else {
		m.SetBuffer(m.history[m.historyIndex])
	}
	return true
}

// History returns the command history.
func (m *CommandMode) History() []string {
	return m.history
}

// OperatorPendingMode represents the state when waiting for a motion or text object.
// For example, after pressing 'd' in normal mode, we're in operator-pending mode
// waiting for a motion like 'w' or a text object like 'iw'.
type OperatorPendingMode struct {
	// operator is the pending operator (e.g., "d", "c", "y").
	operator string

	// count holds the numeric prefix for the motion.
	count int
}

// NewOperatorPendingMode creates a new operator-pending mode instance.
func NewOperatorPendingMode() *OperatorPendingMode {
	return &OperatorPendingMode{}
}

// Name returns the mode identifier.
func (m *OperatorPendingMode) Name() string {
	return ModeOperatorPending
}

// DisplayName returns the human-readable mode name.
func (m *OperatorPendingMode) DisplayName() string {
	return "OPERATOR"
}

// CursorStyle returns the cursor style for operator-pending mode.
func (m *OperatorPendingMode) CursorStyle() CursorStyle {
	return CursorUnderline
}

// Enter is called when entering operator-pending mode.
func (m *OperatorPendingMode) Enter(ctx *Context) error {
	// Operator should be set via context
	if op, ok := ctx.Extra["operator"].(string); ok {
		m.operator = op
	}
	if count, ok := ctx.Extra["count"].(int); ok {
		m.count = count
	}
	return nil
}

// Exit is called when leaving operator-pending mode.
func (m *OperatorPendingMode) Exit(ctx *Context) error {
	m.operator = ""
	m.count = 0
	return nil
}

// HandleUnmapped handles key events that have no explicit binding.
func (m *OperatorPendingMode) HandleUnmapped(event key.Event, ctx *Context) *UnmappedResult {
	// Handle count prefix
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

	// Unmapped keys cancel operator-pending mode
	return &UnmappedResult{Consumed: false}
}

// Operator returns the pending operator.
func (m *OperatorPendingMode) Operator() string {
	return m.operator
}

// SetOperator sets the pending operator.
func (m *OperatorPendingMode) SetOperator(op string) {
	m.operator = op
}

// Count returns the current count prefix.
func (m *OperatorPendingMode) Count() int {
	if m.count == 0 {
		return 1
	}
	return m.count
}

// ReplaceMode implements single-character replace mode (r command).
type ReplaceMode struct{}

// NewReplaceMode creates a new replace mode instance.
func NewReplaceMode() *ReplaceMode {
	return &ReplaceMode{}
}

// Name returns the mode identifier.
func (m *ReplaceMode) Name() string {
	return ModeReplace
}

// DisplayName returns the human-readable mode name.
func (m *ReplaceMode) DisplayName() string {
	return "REPLACE"
}

// CursorStyle returns the cursor style for replace mode.
func (m *ReplaceMode) CursorStyle() CursorStyle {
	return CursorUnderline
}

// Enter is called when entering replace mode.
func (m *ReplaceMode) Enter(ctx *Context) error {
	return nil
}

// Exit is called when leaving replace mode.
func (m *ReplaceMode) Exit(ctx *Context) error {
	return nil
}

// HandleUnmapped handles key events that have no explicit binding.
func (m *ReplaceMode) HandleUnmapped(event key.Event, ctx *Context) *UnmappedResult {
	// In replace mode, any character replaces the current character
	if event.IsRune() && !event.IsModified() {
		return &UnmappedResult{
			Consumed: true,
			Action: &Action{
				Name: "editor.replaceChar",
				Args: map[string]any{"char": string(event.Rune)},
			},
		}
	}

	// Space replaces with space
	if event.Key == key.KeySpace && !event.IsModified() {
		return &UnmappedResult{
			Consumed: true,
			Action: &Action{
				Name: "editor.replaceChar",
				Args: map[string]any{"char": " "},
			},
		}
	}

	return &UnmappedResult{Consumed: false}
}
