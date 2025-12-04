package input

import (
	"github.com/dshills/keystorm/internal/input/key"
)

// Context provides context for input processing.
// It tracks the current state needed for key binding lookup and action building.
type Context struct {
	// Mode is the current editor mode (normal, insert, visual, etc.).
	Mode string

	// FileType is the current file type (go, python, etc.).
	FileType string

	// FilePath is the path of the current file.
	FilePath string

	// HasSelection indicates whether there is an active selection.
	HasSelection bool

	// IsModified indicates whether the buffer has unsaved changes.
	IsModified bool

	// IsReadOnly indicates whether the buffer is read-only.
	IsReadOnly bool

	// LineNumber is the current line number (1-based).
	LineNumber uint32

	// ColumnNumber is the current column number (0-based).
	ColumnNumber uint32

	// Conditions holds condition flags for binding evaluation.
	// Keys: "editorTextFocus", "editorReadonly", "inQuickOpen", etc.
	Conditions map[string]bool

	// Variables holds context variables.
	// Keys: "resourceLangId", "activeEditor", etc.
	Variables map[string]string

	// PendingOperator is set when an operator is waiting for a motion.
	PendingOperator string

	// PendingCount is the accumulated count prefix.
	PendingCount int

	// PendingRegister is the selected register for the next operation.
	PendingRegister rune

	// PendingSequence holds the accumulated key sequence.
	PendingSequence *key.Sequence
}

// NewContext creates a new input context with default values.
func NewContext() *Context {
	return &Context{
		Mode:       "normal",
		Conditions: make(map[string]bool),
		Variables:  make(map[string]string),
	}
}

// Clone returns a deep copy of the context.
// Nil maps are preserved as nil in the clone (not converted to empty maps).
func (c *Context) Clone() *Context {
	clone := &Context{
		Mode:            c.Mode,
		FileType:        c.FileType,
		FilePath:        c.FilePath,
		HasSelection:    c.HasSelection,
		IsModified:      c.IsModified,
		IsReadOnly:      c.IsReadOnly,
		LineNumber:      c.LineNumber,
		ColumnNumber:    c.ColumnNumber,
		PendingOperator: c.PendingOperator,
		PendingCount:    c.PendingCount,
		PendingRegister: c.PendingRegister,
	}

	// Preserve nil vs empty map semantics
	if c.Conditions != nil {
		clone.Conditions = make(map[string]bool, len(c.Conditions))
		for k, v := range c.Conditions {
			clone.Conditions[k] = v
		}
	}

	if c.Variables != nil {
		clone.Variables = make(map[string]string, len(c.Variables))
		for k, v := range c.Variables {
			clone.Variables[k] = v
		}
	}

	if c.PendingSequence != nil {
		clone.PendingSequence = c.PendingSequence.Clone()
	}

	return clone
}

// SetCondition sets a condition flag.
func (c *Context) SetCondition(name string, value bool) {
	if c.Conditions == nil {
		c.Conditions = make(map[string]bool)
	}
	c.Conditions[name] = value
}

// GetCondition returns a condition flag value.
func (c *Context) GetCondition(name string) bool {
	if c.Conditions == nil {
		return false
	}
	return c.Conditions[name]
}

// SetVariable sets a context variable.
func (c *Context) SetVariable(name, value string) {
	if c.Variables == nil {
		c.Variables = make(map[string]string)
	}
	c.Variables[name] = value
}

// GetVariable returns a context variable value.
func (c *Context) GetVariable(name string) string {
	if c.Variables == nil {
		return ""
	}
	return c.Variables[name]
}

// ClearPending clears all pending state (operator, count, register, sequence).
func (c *Context) ClearPending() {
	c.PendingOperator = ""
	c.PendingCount = 0
	c.PendingRegister = 0
	c.PendingSequence = nil
}

// HasPendingOperator returns true if an operator is pending.
func (c *Context) HasPendingOperator() bool {
	return c.PendingOperator != ""
}

// HasPendingCount returns true if a count prefix has been entered.
func (c *Context) HasPendingCount() bool {
	return c.PendingCount > 0
}

// GetCount returns the pending count, or 1 if no count is set.
func (c *Context) GetCount() int {
	if c.PendingCount <= 0 {
		return 1
	}
	return c.PendingCount
}

// AccumulateCount adds a digit to the pending count.
func (c *Context) AccumulateCount(digit int) {
	if digit < 0 || digit > 9 {
		return
	}
	c.PendingCount = c.PendingCount*10 + digit
}

// AppendToSequence adds a key event to the pending sequence.
func (c *Context) AppendToSequence(event key.Event) {
	if c.PendingSequence == nil {
		c.PendingSequence = key.NewSequence()
	}
	c.PendingSequence.Add(event)
}

// ClearSequence clears only the pending sequence.
func (c *Context) ClearSequence() {
	c.PendingSequence = nil
}

// EditorStateProvider provides editor state for context updates.
type EditorStateProvider interface {
	// Mode returns the current editor mode.
	Mode() string

	// FileType returns the current file type.
	FileType() string

	// FilePath returns the current file path.
	FilePath() string

	// HasSelection returns true if there is an active selection.
	HasSelection() bool

	// IsModified returns true if the buffer has unsaved changes.
	IsModified() bool

	// IsReadOnly returns true if the buffer is read-only.
	IsReadOnly() bool

	// CursorPosition returns the line and column of the cursor.
	CursorPosition() (line, col uint32)
}

// UpdateFromEditor updates the context from an editor state provider.
func (c *Context) UpdateFromEditor(editor EditorStateProvider) {
	if editor == nil {
		return
	}

	c.Mode = editor.Mode()
	c.FileType = editor.FileType()
	c.FilePath = editor.FilePath()
	c.HasSelection = editor.HasSelection()
	c.IsModified = editor.IsModified()
	c.IsReadOnly = editor.IsReadOnly()
	c.LineNumber, c.ColumnNumber = editor.CursorPosition()

	// Set standard conditions
	c.SetCondition("editorTextFocus", true)
	c.SetCondition("editorReadonly", c.IsReadOnly)
	c.SetCondition("editorHasSelection", c.HasSelection)

	// Set standard variables
	c.SetVariable("resourceLangId", c.FileType)
}
