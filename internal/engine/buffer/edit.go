package buffer

import "fmt"

// Edit represents a text edit operation.
// It specifies a range to replace and the new text.
type Edit struct {
	Range   Range  // The range to replace
	NewText string // The replacement text
}

// NewEdit creates a new Edit.
func NewEdit(r Range, newText string) Edit {
	return Edit{Range: r, NewText: newText}
}

// NewInsert creates an Edit that inserts text at a position.
func NewInsert(offset ByteOffset, text string) Edit {
	return Edit{
		Range:   Range{Start: offset, End: offset},
		NewText: text,
	}
}

// NewDelete creates an Edit that deletes a range of text.
func NewDelete(start, end ByteOffset) Edit {
	return Edit{
		Range:   Range{Start: start, End: end},
		NewText: "",
	}
}

// String returns a human-readable representation of the edit.
func (e Edit) String() string {
	if e.Range.IsEmpty() {
		return fmt.Sprintf("Insert(%d, %q)", e.Range.Start, e.NewText)
	}
	if e.NewText == "" {
		return fmt.Sprintf("Delete%s", e.Range.String())
	}
	return fmt.Sprintf("Replace%s with %q", e.Range.String(), e.NewText)
}

// IsInsert returns true if this is a pure insertion (empty range).
func (e Edit) IsInsert() bool {
	return e.Range.IsEmpty() && e.NewText != ""
}

// IsDelete returns true if this is a pure deletion (empty replacement).
func (e Edit) IsDelete() bool {
	return !e.Range.IsEmpty() && e.NewText == ""
}

// IsReplace returns true if this replaces existing text with new text.
func (e Edit) IsReplace() bool {
	return !e.Range.IsEmpty() && e.NewText != ""
}

// IsNoOp returns true if this edit does nothing.
func (e Edit) IsNoOp() bool {
	return e.Range.IsEmpty() && e.NewText == ""
}

// Delta returns the change in buffer length caused by this edit.
func (e Edit) Delta() ByteOffset {
	return ByteOffset(len(e.NewText)) - e.Range.Len()
}

// EditResult contains information about an applied edit.
type EditResult struct {
	OldRange Range  // The original range that was modified
	NewRange Range  // The resulting range after the edit
	OldText  string // The text that was replaced (if any)
	Delta    int64  // Change in buffer length
}

// ChangeType categorizes the type of change made to the buffer.
type ChangeType uint8

const (
	ChangeInsert  ChangeType = iota // Text was inserted
	ChangeDelete                    // Text was deleted
	ChangeReplace                   // Text was replaced
)

// String returns a string representation of the change type.
func (c ChangeType) String() string {
	switch c {
	case ChangeInsert:
		return "insert"
	case ChangeDelete:
		return "delete"
	case ChangeReplace:
		return "replace"
	default:
		return "unknown"
	}
}

// Change represents a single change to the buffer.
// This is used for change tracking and undo/redo.
type Change struct {
	Type     ChangeType // Type of change
	Range    Range      // Original range that was affected
	NewRange Range      // Resulting range after the change
	OldText  string     // Text that was removed (for delete/replace)
	NewText  string     // Text that was added (for insert/replace)
}

// Invert returns the inverse change that would undo this change.
func (c Change) Invert() Change {
	switch c.Type {
	case ChangeInsert:
		return Change{
			Type:    ChangeDelete,
			Range:   c.NewRange,
			OldText: c.NewText,
		}
	case ChangeDelete:
		return Change{
			Type:     ChangeInsert,
			Range:    Range{Start: c.Range.Start, End: c.Range.Start},
			NewRange: c.Range,
			NewText:  c.OldText,
		}
	case ChangeReplace:
		return Change{
			Type:     ChangeReplace,
			Range:    c.NewRange,
			NewRange: c.Range,
			OldText:  c.NewText,
			NewText:  c.OldText,
		}
	default:
		return c
	}
}

// ToEdit converts a Change to an Edit for reapplication.
func (c Change) ToEdit() Edit {
	return Edit{
		Range:   c.Range,
		NewText: c.NewText,
	}
}
