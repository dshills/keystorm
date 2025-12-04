package mode

import (
	"github.com/dshills/keystorm/internal/input/key"
)

// VisualMode implements Vim's visual mode (character-wise selection).
type VisualMode struct {
	// selectMode determines selection granularity.
	selectMode SelectionMode

	// anchor is the starting position of the selection.
	anchor Position

	// count holds the numeric prefix for the next command.
	count int
}

// NewVisualMode creates a new visual mode instance for character selection.
func NewVisualMode() *VisualMode {
	return &VisualMode{
		selectMode: SelectChar,
	}
}

// Name returns the mode identifier.
func (m *VisualMode) Name() string {
	return ModeVisual
}

// DisplayName returns the human-readable mode name.
func (m *VisualMode) DisplayName() string {
	return "VISUAL"
}

// CursorStyle returns the cursor style for visual mode.
func (m *VisualMode) CursorStyle() CursorStyle {
	return CursorBlock
}

// Enter is called when entering visual mode.
func (m *VisualMode) Enter(ctx *Context) error {
	m.count = 0

	// Set anchor to current cursor position
	if ctx.Editor != nil {
		line, col := ctx.Editor.CursorPosition()
		m.anchor = Position{Line: line, Column: col}
	}

	return nil
}

// Exit is called when leaving visual mode.
func (m *VisualMode) Exit(ctx *Context) error {
	m.count = 0
	return nil
}

// HandleUnmapped handles key events that have no explicit binding.
func (m *VisualMode) HandleUnmapped(event key.Event, ctx *Context) *UnmappedResult {
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

	// Unmapped keys in visual mode are ignored
	return &UnmappedResult{Consumed: false}
}

// Anchor returns the selection anchor position.
func (m *VisualMode) Anchor() Position {
	return m.anchor
}

// SetAnchor sets the selection anchor position.
func (m *VisualMode) SetAnchor(pos Position) {
	m.anchor = pos
}

// SelectionMode returns the selection mode.
func (m *VisualMode) SelectionMode() SelectionMode {
	return m.selectMode
}

// Count returns the current count prefix.
func (m *VisualMode) Count() int {
	if m.count == 0 {
		return 1
	}
	return m.count
}

// ClearCount clears the count prefix.
func (m *VisualMode) ClearCount() {
	m.count = 0
}

// VisualLineMode implements Vim's visual line mode (line-wise selection).
type VisualLineMode struct {
	VisualMode
}

// NewVisualLineMode creates a new visual line mode instance.
func NewVisualLineMode() *VisualLineMode {
	return &VisualLineMode{
		VisualMode: VisualMode{
			selectMode: SelectLine,
		},
	}
}

// Name returns the mode identifier.
func (m *VisualLineMode) Name() string {
	return ModeVisualLine
}

// DisplayName returns the human-readable mode name.
func (m *VisualLineMode) DisplayName() string {
	return "VISUAL LINE"
}

// VisualBlockMode implements Vim's visual block mode (column selection).
type VisualBlockMode struct {
	VisualMode
}

// NewVisualBlockMode creates a new visual block mode instance.
func NewVisualBlockMode() *VisualBlockMode {
	return &VisualBlockMode{
		VisualMode: VisualMode{
			selectMode: SelectBlock,
		},
	}
}

// Name returns the mode identifier.
func (m *VisualBlockMode) Name() string {
	return ModeVisualBlock
}

// DisplayName returns the human-readable mode name.
func (m *VisualBlockMode) DisplayName() string {
	return "VISUAL BLOCK"
}
