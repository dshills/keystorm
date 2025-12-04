package mode

import (
	"unicode"

	"github.com/dshills/keystorm/internal/input/key"
)

// InsertMode implements Vim's insert mode.
// In insert mode, most keys are interpreted as text to be inserted.
type InsertMode struct {
	// completionActive indicates if auto-completion is showing.
	completionActive bool

	// insertStart records where insert mode began (for undo grouping).
	insertStart Position
}

// NewInsertMode creates a new insert mode instance.
func NewInsertMode() *InsertMode {
	return &InsertMode{}
}

// Name returns the mode identifier.
func (m *InsertMode) Name() string {
	return ModeInsert
}

// DisplayName returns the human-readable mode name.
func (m *InsertMode) DisplayName() string {
	return "INSERT"
}

// CursorStyle returns the cursor style for insert mode.
func (m *InsertMode) CursorStyle() CursorStyle {
	return CursorBar
}

// Enter is called when entering insert mode.
func (m *InsertMode) Enter(ctx *Context) error {
	m.completionActive = false

	// Record where insert mode started
	if ctx.Editor != nil {
		line, col := ctx.Editor.CursorPosition()
		m.insertStart = Position{Line: line, Column: col}
	}

	return nil
}

// Exit is called when leaving insert mode.
func (m *InsertMode) Exit(ctx *Context) error {
	// Hide any active completion
	m.completionActive = false
	return nil
}

// HandleUnmapped handles key events that have no explicit binding.
func (m *InsertMode) HandleUnmapped(event key.Event, ctx *Context) *UnmappedResult {
	// In insert mode, unmodified character keys are typed as text
	if event.IsRune() && !event.IsModified() {
		r := event.Rune
		if unicode.IsPrint(r) || r == '\t' {
			return &UnmappedResult{
				Consumed:   true,
				InsertText: string(r),
				Action: &Action{
					Name: "editor.insertText",
					Args: map[string]any{"text": string(r)},
				},
			}
		}
	}

	// Space is a printable character
	if event.Key == key.KeySpace && !event.IsModified() {
		return &UnmappedResult{
			Consumed:   true,
			InsertText: " ",
			Action: &Action{
				Name: "editor.insertText",
				Args: map[string]any{"text": " "},
			},
		}
	}

	// Other unmapped keys are ignored in insert mode
	return &UnmappedResult{Consumed: false}
}

// IsCompletionActive returns true if auto-completion is showing.
func (m *InsertMode) IsCompletionActive() bool {
	return m.completionActive
}

// SetCompletionActive sets the completion state.
func (m *InsertMode) SetCompletionActive(active bool) {
	m.completionActive = active
}

// InsertStart returns the position where insert mode began.
func (m *InsertMode) InsertStart() Position {
	return m.insertStart
}
