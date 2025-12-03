package history

import (
	"fmt"
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
)

// Command represents a composable edit action that can be executed and undone.
type Command interface {
	// Execute performs the command and returns an error if it fails.
	Execute(buf *buffer.Buffer, cursors *cursor.CursorSet) error

	// Undo reverses the command and returns an error if it fails.
	Undo(buf *buffer.Buffer, cursors *cursor.CursorSet) error

	// Description returns a human-readable description of the command.
	Description() string
}

// InsertCommand inserts text at all cursor positions.
type InsertCommand struct {
	Text       string
	operations OperationList
}

// NewInsertCommand creates a new insert command.
func NewInsertCommand(text string) *InsertCommand {
	return &InsertCommand{Text: text}
}

// Execute inserts text at all cursor/selection positions.
func (c *InsertCommand) Execute(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	if len(c.Text) == 0 {
		return nil
	}

	c.operations = nil

	// Get selections sorted by offset (we'll process in reverse order)
	sels := cursors.All()
	if len(sels) == 0 {
		return nil
	}

	// Process in reverse order (highest offset first) to preserve offsets
	for i := len(sels) - 1; i >= 0; i-- {
		sel := sels[i]
		r := sel.Range()

		// Get the text being replaced (if any)
		oldText := ""
		if !r.IsEmpty() {
			oldText = buf.TextRange(r.Start, r.End)
		}

		// Create operation record
		op := NewReplaceOperation(r, oldText, c.Text)
		op.CursorsBefore = []Selection{sel}

		// Apply the edit
		newEnd, err := buf.Replace(r.Start, r.End, c.Text)
		if err != nil {
			return fmt.Errorf("insert at offset %d: %w", r.Start, err)
		}

		op.CursorsAfter = []Selection{cursor.NewCursorSelection(newEnd)}
		c.operations = append(c.operations, op)
	}

	// Update cursor positions - move all to end of inserted text
	newSels := make([]Selection, len(sels))
	delta := ByteOffset(0)
	for i := 0; i < len(sels); i++ {
		sel := sels[i]
		r := sel.Range()
		oldLen := r.End - r.Start
		newLen := ByteOffset(len(c.Text))

		// New position is at end of inserted text, adjusted for previous edits
		newPos := r.Start + delta + newLen
		newSels[i] = cursor.NewCursorSelection(newPos)

		delta += newLen - oldLen
	}
	cursors.SetAll(newSels)

	return nil
}

// Undo removes the inserted text and restores selections.
func (c *InsertCommand) Undo(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	if len(c.operations) == 0 {
		return nil
	}

	// Apply inverse operations in reverse order
	for i := len(c.operations) - 1; i >= 0; i-- {
		inv := c.operations[i].Invert()
		_, err := buf.Replace(inv.Range.Start, inv.Range.End, inv.NewText)
		if err != nil {
			return fmt.Errorf("undo insert: %w", err)
		}
	}

	// Restore cursor positions
	if len(c.operations) > 0 {
		var restoredSels []Selection
		for _, op := range c.operations {
			restoredSels = append(restoredSels, op.CursorsBefore...)
		}
		// Operations were stored in reverse order, so reverse to get original order
		for i, j := 0, len(restoredSels)-1; i < j; i, j = i+1, j-1 {
			restoredSels[i], restoredSels[j] = restoredSels[j], restoredSels[i]
		}
		cursors.SetAll(restoredSels)
	}

	return nil
}

// Description returns a human-readable description.
func (c *InsertCommand) Description() string {
	if len(c.Text) == 1 {
		if c.Text == "\n" {
			return "Insert newline"
		}
		if c.Text == "\t" {
			return "Insert tab"
		}
		return fmt.Sprintf("Type '%s'", c.Text)
	}
	if utf8.RuneCountInString(c.Text) <= 20 {
		return fmt.Sprintf("Insert \"%s\"", c.Text)
	}
	return fmt.Sprintf("Insert %d characters", utf8.RuneCountInString(c.Text))
}

// DeleteDirection specifies the direction of deletion.
type DeleteDirection int

const (
	// DeleteBackward deletes backward (like Backspace key).
	DeleteBackward DeleteDirection = iota
	// DeleteForward deletes forward (like Delete key).
	DeleteForward
)

// DeleteCommand deletes text at cursor positions.
type DeleteCommand struct {
	Direction  DeleteDirection
	Count      int // Number of characters/units to delete (default 1)
	operations OperationList
}

// NewDeleteCommand creates a new delete command.
func NewDeleteCommand(direction DeleteDirection) *DeleteCommand {
	return &DeleteCommand{
		Direction: direction,
		Count:     1,
	}
}

// NewDeleteCommandN creates a delete command that deletes N characters.
func NewDeleteCommandN(direction DeleteDirection, count int) *DeleteCommand {
	if count < 1 {
		count = 1
	}
	return &DeleteCommand{
		Direction: direction,
		Count:     count,
	}
}

// Execute deletes text at all cursor/selection positions.
func (c *DeleteCommand) Execute(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	c.operations = nil

	sels := cursors.All()
	if len(sels) == 0 {
		return nil
	}

	// Process in reverse order to preserve offsets
	for i := len(sels) - 1; i >= 0; i-- {
		sel := sels[i]
		var deleteRange Range

		if !sel.IsEmpty() {
			// Delete selected text
			deleteRange = sel.Range()
		} else {
			// Delete based on direction
			pos := sel.Head
			if c.Direction == DeleteBackward {
				// Backspace - delete before cursor
				start := pos
				for j := 0; j < c.Count && start > 0; j++ {
					start--
				}
				deleteRange = Range{Start: start, End: pos}
			} else {
				// Delete key - delete after cursor
				end := pos
				bufLen := buf.Len()
				for j := 0; j < c.Count && end < bufLen; j++ {
					end++
				}
				deleteRange = Range{Start: pos, End: end}
			}
		}

		if deleteRange.IsEmpty() {
			continue
		}

		// Get the text being deleted
		oldText := buf.TextRange(deleteRange.Start, deleteRange.End)

		// Create operation record
		op := NewDeleteOperation(deleteRange, oldText)
		op.CursorsBefore = []Selection{sel}

		// Apply the deletion
		err := buf.Delete(deleteRange.Start, deleteRange.End)
		if err != nil {
			return fmt.Errorf("delete at range [%d,%d): %w", deleteRange.Start, deleteRange.End, err)
		}

		op.CursorsAfter = []Selection{cursor.NewCursorSelection(deleteRange.Start)}
		c.operations = append(c.operations, op)
	}

	// Update cursor positions
	newSels := make([]Selection, 0, len(sels))
	delta := ByteOffset(0)
	for i := 0; i < len(sels); i++ {
		sel := sels[i]
		var newPos ByteOffset

		if !sel.IsEmpty() {
			newPos = sel.Start() + delta
		} else if c.Direction == DeleteBackward {
			// After backspace, cursor moves back
			start := sel.Head
			for j := 0; j < c.Count && start > 0; j++ {
				start--
			}
			newPos = start + delta
		} else {
			// After delete, cursor stays in place
			newPos = sel.Head + delta
		}

		newSels = append(newSels, cursor.NewCursorSelection(newPos))

		// Calculate delta for next cursor
		var deleteLen ByteOffset
		if !sel.IsEmpty() {
			deleteLen = sel.End() - sel.Start()
		} else if c.Direction == DeleteBackward {
			deleteLen = ByteOffset(c.Count)
			if sel.Head < deleteLen {
				deleteLen = sel.Head
			}
		} else {
			// Forward delete: clamp to actual bytes that could be deleted
			deleteLen = ByteOffset(c.Count)
			remaining := buf.Len() - sel.Head
			if remaining < deleteLen {
				deleteLen = remaining
			}
		}
		delta -= deleteLen
	}
	cursors.SetAll(newSels)

	return nil
}

// Undo restores the deleted text and cursor positions.
func (c *DeleteCommand) Undo(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	if len(c.operations) == 0 {
		return nil
	}

	// Apply inverse operations in reverse order
	for i := len(c.operations) - 1; i >= 0; i-- {
		inv := c.operations[i].Invert()
		_, err := buf.Replace(inv.Range.Start, inv.Range.End, inv.NewText)
		if err != nil {
			return fmt.Errorf("undo delete: %w", err)
		}
	}

	// Restore cursor positions
	var restoredSels []Selection
	for _, op := range c.operations {
		restoredSels = append(restoredSels, op.CursorsBefore...)
	}
	// Operations were stored in reverse order
	for i, j := 0, len(restoredSels)-1; i < j; i, j = i+1, j-1 {
		restoredSels[i], restoredSels[j] = restoredSels[j], restoredSels[i]
	}
	cursors.SetAll(restoredSels)

	return nil
}

// Description returns a human-readable description.
func (c *DeleteCommand) Description() string {
	if c.Count == 1 {
		if c.Direction == DeleteBackward {
			return "Backspace"
		}
		return "Delete"
	}
	if c.Direction == DeleteBackward {
		return fmt.Sprintf("Backspace %d characters", c.Count)
	}
	return fmt.Sprintf("Delete %d characters", c.Count)
}

// ReplaceCommand replaces text in a specific range.
type ReplaceCommand struct {
	Range      Range
	NewText    string
	operations OperationList
}

// NewReplaceCommand creates a new replace command.
func NewReplaceCommand(r Range, newText string) *ReplaceCommand {
	return &ReplaceCommand{
		Range:   r,
		NewText: newText,
	}
}

// Execute replaces text in the specified range.
func (c *ReplaceCommand) Execute(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	c.operations = nil

	// Store cursor state before
	cursorsBefore := cursors.All()

	// Get the text being replaced
	oldText := buf.TextRange(c.Range.Start, c.Range.End)

	// Create operation record
	op := NewReplaceOperation(c.Range, oldText, c.NewText)
	op.CursorsBefore = cursorsBefore

	// Apply the replacement
	_, err := buf.Replace(c.Range.Start, c.Range.End, c.NewText)
	if err != nil {
		return fmt.Errorf("replace at range [%d,%d): %w", c.Range.Start, c.Range.End, err)
	}

	// Update cursors based on the edit
	edit := buffer.Edit{Range: c.Range, NewText: c.NewText}
	cursor.TransformCursorSet(cursors, edit)

	op.CursorsAfter = cursors.All()
	c.operations = append(c.operations, op)

	return nil
}

// Undo restores the original text and cursor positions.
func (c *ReplaceCommand) Undo(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	if len(c.operations) == 0 {
		return nil
	}

	op := c.operations[0]
	inv := op.Invert()
	_, err := buf.Replace(inv.Range.Start, inv.Range.End, inv.NewText)
	if err != nil {
		return fmt.Errorf("undo replace: %w", err)
	}

	cursors.SetAll(op.CursorsBefore)
	return nil
}

// Description returns a human-readable description.
func (c *ReplaceCommand) Description() string {
	oldLen := c.Range.End - c.Range.Start
	newLen := utf8.RuneCountInString(c.NewText)
	if oldLen == 0 {
		return fmt.Sprintf("Insert %d characters", newLen)
	}
	if newLen == 0 {
		return fmt.Sprintf("Delete %d characters", oldLen)
	}
	return fmt.Sprintf("Replace %d with %d characters", oldLen, newLen)
}

// CompoundCommand groups multiple commands as one undo unit.
type CompoundCommand struct {
	Name     string
	Commands []Command
}

// NewCompoundCommand creates a new compound command.
func NewCompoundCommand(name string, commands ...Command) *CompoundCommand {
	return &CompoundCommand{
		Name:     name,
		Commands: commands,
	}
}

// Execute runs all commands in order.
func (c *CompoundCommand) Execute(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	for i, cmd := range c.Commands {
		if err := cmd.Execute(buf, cursors); err != nil {
			// On error, try to undo what we've done
			for j := i - 1; j >= 0; j-- {
				_ = c.Commands[j].Undo(buf, cursors)
			}
			return fmt.Errorf("compound command '%s' step %d: %w", c.Name, i, err)
		}
	}
	return nil
}

// Undo reverses all commands in reverse order.
func (c *CompoundCommand) Undo(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	for i := len(c.Commands) - 1; i >= 0; i-- {
		if err := c.Commands[i].Undo(buf, cursors); err != nil {
			return fmt.Errorf("undo compound command '%s' step %d: %w", c.Name, i, err)
		}
	}
	return nil
}

// Description returns the compound command's name.
func (c *CompoundCommand) Description() string {
	if c.Name != "" {
		return c.Name
	}
	if len(c.Commands) == 1 {
		return c.Commands[0].Description()
	}
	return fmt.Sprintf("%d operations", len(c.Commands))
}

// Add adds a command to the compound command.
func (c *CompoundCommand) Add(cmd Command) {
	c.Commands = append(c.Commands, cmd)
}

// IsEmpty returns true if the compound command has no commands.
func (c *CompoundCommand) IsEmpty() bool {
	return len(c.Commands) == 0
}
