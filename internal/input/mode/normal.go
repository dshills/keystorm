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
	// Handle Escape - clear any pending state
	if event.Key == key.KeyEscape {
		m.ResetState()
		return &UnmappedResult{Consumed: true}
	}

	// Handle Ctrl+C - also clear state (and potentially quit)
	if event.Key == key.KeyRune && event.Rune == 'c' && event.Modifiers.HasCtrl() {
		m.ResetState()
		return &UnmappedResult{Consumed: true}
	}

	// Handle unmodified character keys
	if event.IsRune() && !event.IsModified() {
		r := event.Rune

		// Digits build up the count prefix (except 0 at start which is a command)
		if r >= '1' && r <= '9' {
			m.count = m.count*10 + int(r-'0')
			return &UnmappedResult{Consumed: true}
		}
		if r == '0' && m.count > 0 {
			m.count = m.count * 10
			return &UnmappedResult{Consumed: true}
		}

		// Get count and reset for command execution
		count := m.Count()

		// Mode switching
		switch r {
		case 'i': // Enter insert mode
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.insert"},
			}
		case 'I': // Enter insert mode at beginning of line
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.insert", Args: map[string]any{"position": "line_start"}},
			}
		case 'a': // Enter insert mode after cursor
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.insert", Args: map[string]any{"position": "after"}},
			}
		case 'A': // Enter insert mode at end of line
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.insert", Args: map[string]any{"position": "line_end"}},
			}
		case 'o': // Open line below
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.insert", Args: map[string]any{"position": "new_line_below"}},
			}
		case 'O': // Open line above
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.insert", Args: map[string]any{"position": "new_line_above"}},
			}
		case 'v': // Enter visual mode
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.visual"},
			}
		case 'V': // Enter visual line mode
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.visual", Args: map[string]any{"type": "line"}},
			}
		case ':': // Enter command mode
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.command"},
			}

		// Basic motions
		case 'h': // Move left
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.left", Args: map[string]any{"count": count}},
			}
		case 'j': // Move down
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.down", Args: map[string]any{"count": count}},
			}
		case 'k': // Move up
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.up", Args: map[string]any{"count": count}},
			}
		case 'l': // Move right
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.right", Args: map[string]any{"count": count}},
			}
		case 'w': // Move word forward
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.word_forward", Args: map[string]any{"count": count}},
			}
		case 'b': // Move word backward
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.word_backward", Args: map[string]any{"count": count}},
			}
		case 'e': // Move to end of word
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.word_end", Args: map[string]any{"count": count}},
			}
		case '0': // Move to beginning of line (when no count)
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.line_start"},
			}
		case '$': // Move to end of line
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.line_end"},
			}
		case '^': // Move to first non-blank
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.first_non_blank"},
			}
		case 'G': // Go to line or end of file
			m.ResetState()
			if count > 1 {
				return &UnmappedResult{
					Consumed: true,
					Action:   &Action{Name: "cursor.go_to_line", Args: map[string]any{"line": count}},
				}
			}
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "cursor.file_end"},
			}

		// Basic editing
		case 'x': // Delete character under cursor
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "editor.delete_char", Args: map[string]any{"count": count}},
			}
		case 'X': // Delete character before cursor
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "editor.delete_char_before", Args: map[string]any{"count": count}},
			}
		case 'u': // Undo
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "editor.undo", Args: map[string]any{"count": count}},
			}
		case 'r': // Replace character (need to wait for next char, but simplified for now)
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "mode.replace"},
			}

		// Operators (simplified - would normally wait for motion)
		case 'd': // Delete operator
			if m.pendingOperator == "d" {
				// dd - delete line
				m.ResetState()
				return &UnmappedResult{
					Consumed: true,
					Action:   &Action{Name: "editor.delete_line", Args: map[string]any{"count": count}},
				}
			}
			m.pendingOperator = "d"
			return &UnmappedResult{Consumed: true}
		case 'y': // Yank operator
			if m.pendingOperator == "y" {
				// yy - yank line
				m.ResetState()
				return &UnmappedResult{
					Consumed: true,
					Action:   &Action{Name: "editor.yank_line", Args: map[string]any{"count": count}},
				}
			}
			m.pendingOperator = "y"
			return &UnmappedResult{Consumed: true}
		case 'c': // Change operator
			if m.pendingOperator == "c" {
				// cc - change line
				m.ResetState()
				return &UnmappedResult{
					Consumed: true,
					Action:   &Action{Name: "editor.change_line", Args: map[string]any{"count": count}},
				}
			}
			m.pendingOperator = "c"
			return &UnmappedResult{Consumed: true}
		case 'p': // Paste after
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "editor.paste", Args: map[string]any{"after": true}},
			}
		case 'P': // Paste before
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "editor.paste", Args: map[string]any{"after": false}},
			}
		}
	}

	// Handle arrow keys
	switch event.Key {
	case key.KeyLeft:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "cursor.left", Args: map[string]any{"count": m.Count()}},
		}
	case key.KeyRight:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "cursor.right", Args: map[string]any{"count": m.Count()}},
		}
	case key.KeyUp:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "cursor.up", Args: map[string]any{"count": m.Count()}},
		}
	case key.KeyDown:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "cursor.down", Args: map[string]any{"count": m.Count()}},
		}
	case key.KeyHome:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "cursor.line_start"},
		}
	case key.KeyEnd:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "cursor.line_end"},
		}
	case key.KeyPageUp:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "view.page_up"},
		}
	case key.KeyPageDown:
		m.ResetState()
		return &UnmappedResult{
			Consumed: true,
			Action:   &Action{Name: "view.page_down"},
		}
	}

	// Handle Ctrl combinations
	if event.Modifiers.HasCtrl() && event.IsRune() {
		switch event.Rune {
		case 'r', 'R': // Redo
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "editor.redo"},
			}
		case 'f', 'F': // Page down
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "view.page_down"},
			}
		case 'b', 'B': // Page up
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "view.page_up"},
			}
		case 'd', 'D': // Half page down
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "view.half_page_down"},
			}
		case 'u', 'U': // Half page up
			m.ResetState()
			return &UnmappedResult{
				Consumed: true,
				Action:   &Action{Name: "view.half_page_up"},
			}
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
