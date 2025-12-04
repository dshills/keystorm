package mode

import (
	"github.com/dshills/keystorm/internal/input/key"
)

// Mode defines the interface for editor modes.
// Each mode determines how key events are interpreted and what cursor
// style is displayed.
type Mode interface {
	// Name returns the unique mode identifier (e.g., "normal", "insert").
	Name() string

	// DisplayName returns a human-readable name for the status line.
	DisplayName() string

	// CursorStyle returns the cursor style for this mode.
	CursorStyle() CursorStyle

	// Enter is called when entering this mode.
	// The context provides information about the transition.
	Enter(ctx *Context) error

	// Exit is called when leaving this mode.
	// The context provides information about the transition.
	Exit(ctx *Context) error

	// HandleUnmapped handles key events that have no binding in this mode.
	// Returns an action to execute, or nil if the key should be ignored.
	HandleUnmapped(event key.Event, ctx *Context) *UnmappedResult
}

// UnmappedResult describes what to do with an unmapped key.
type UnmappedResult struct {
	// Action is the action to execute, if any.
	Action *Action

	// Consumed indicates whether the key was handled.
	Consumed bool

	// InsertText is text to insert (for insert mode).
	InsertText string
}

// Action represents a command to be executed.
// This is a simplified version; the full Action type is in the parent package.
type Action struct {
	Name string
	Args map[string]any
}

// Context provides information during mode transitions and key handling.
type Context struct {
	// PreviousMode is the mode being transitioned from (for Enter).
	PreviousMode string

	// NextMode is the mode being transitioned to (for Exit).
	NextMode string

	// Editor provides read-only access to editor state.
	Editor EditorState

	// Selection information, if any.
	Selection *Selection

	// Register is the currently selected register (e.g., '"', 'a'-'z').
	Register rune

	// Count is the numeric prefix, if any (e.g., 5 in "5j").
	Count int

	// Extra holds mode-specific context data.
	Extra map[string]any
}

// NewContext creates a new mode context.
func NewContext() *Context {
	return &Context{
		Extra: make(map[string]any),
	}
}

// WithEditor returns a copy of the context with the given editor state.
func (c *Context) WithEditor(editor EditorState) *Context {
	copy := *c
	copy.Editor = editor
	return &copy
}

// WithCount returns a copy of the context with the given count.
func (c *Context) WithCount(count int) *Context {
	copy := *c
	copy.Count = count
	return &copy
}

// CursorStyle defines the visual appearance of the cursor.
type CursorStyle uint8

const (
	// CursorBlock is a full-cell block cursor (normal mode).
	CursorBlock CursorStyle = iota

	// CursorBar is a thin vertical bar cursor (insert mode).
	CursorBar

	// CursorUnderline is an underline cursor.
	CursorUnderline

	// CursorHidden hides the cursor.
	CursorHidden
)

// String returns a human-readable cursor style name.
func (c CursorStyle) String() string {
	switch c {
	case CursorBlock:
		return "block"
	case CursorBar:
		return "bar"
	case CursorUnderline:
		return "underline"
	case CursorHidden:
		return "hidden"
	default:
		return "unknown"
	}
}

// EditorState provides read-only access to editor state.
// This interface is implemented by the editor to provide context to modes.
type EditorState interface {
	// CursorPosition returns the current cursor position (line, column).
	// Lines and columns are 0-indexed.
	CursorPosition() (line, col uint32)

	// HasSelection returns true if there is an active selection.
	HasSelection() bool

	// CurrentLine returns the text of the current line.
	CurrentLine() string

	// LineCount returns the total number of lines in the buffer.
	LineCount() uint32

	// FilePath returns the path of the current file, or empty string.
	FilePath() string

	// FileType returns the detected file type (e.g., "go", "python").
	FileType() string

	// IsModified returns true if the buffer has unsaved changes.
	IsModified() bool
}

// Selection represents a text selection.
type Selection struct {
	// Start is the selection start position.
	Start Position

	// End is the selection end position.
	End Position

	// Mode is the selection mode (character, line, block).
	Mode SelectionMode
}

// Position represents a position in the buffer.
type Position struct {
	Line   uint32
	Column uint32
}

// SelectionMode defines the type of selection.
type SelectionMode uint8

const (
	// SelectChar is character-wise selection (visual mode).
	SelectChar SelectionMode = iota

	// SelectLine is line-wise selection (visual line mode).
	SelectLine

	// SelectBlock is block/column selection (visual block mode).
	SelectBlock
)

// String returns a human-readable selection mode name.
func (s SelectionMode) String() string {
	switch s {
	case SelectChar:
		return "char"
	case SelectLine:
		return "line"
	case SelectBlock:
		return "block"
	default:
		return "unknown"
	}
}

// Standard mode names.
const (
	ModeNormal          = "normal"
	ModeInsert          = "insert"
	ModeVisual          = "visual"
	ModeVisualLine      = "visual-line"
	ModeVisualBlock     = "visual-block"
	ModeCommand         = "command"
	ModeOperatorPending = "operator-pending"
	ModeReplace         = "replace"
)
