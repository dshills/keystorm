package history

import (
	"time"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
)

// ByteOffset is an alias for buffer.ByteOffset for convenience.
type ByteOffset = buffer.ByteOffset

// Range is an alias for buffer.Range for convenience.
type Range = buffer.Range

// Selection is an alias for cursor.Selection for convenience.
type Selection = cursor.Selection

// Operation represents a single undoable edit.
// It captures all information needed to undo or redo the edit.
type Operation struct {
	// Edit data
	Range   Range  // Range that was modified (in original document)
	OldText string // Text that was replaced (for undo)
	NewText string // Text that was inserted (for redo)

	// Cursor state for restore
	CursorsBefore []Selection // Cursor positions before the edit
	CursorsAfter  []Selection // Cursor positions after the edit

	// Metadata
	Timestamp time.Time // When the operation occurred
}

// NewOperation creates a new operation.
func NewOperation(r Range, oldText, newText string) *Operation {
	return &Operation{
		Range:     r,
		OldText:   oldText,
		NewText:   newText,
		Timestamp: time.Now(),
	}
}

// NewInsertOperation creates an operation for an insertion.
func NewInsertOperation(offset ByteOffset, text string) *Operation {
	return &Operation{
		Range:     Range{Start: offset, End: offset},
		OldText:   "",
		NewText:   text,
		Timestamp: time.Now(),
	}
}

// NewDeleteOperation creates an operation for a deletion.
func NewDeleteOperation(r Range, deletedText string) *Operation {
	return &Operation{
		Range:     r,
		OldText:   deletedText,
		NewText:   "",
		Timestamp: time.Now(),
	}
}

// NewReplaceOperation creates an operation for a replacement.
func NewReplaceOperation(r Range, oldText, newText string) *Operation {
	return &Operation{
		Range:     r,
		OldText:   oldText,
		NewText:   newText,
		Timestamp: time.Now(),
	}
}

// IsInsert returns true if this operation is a pure insertion.
func (op *Operation) IsInsert() bool {
	return op.Range.IsEmpty() && len(op.NewText) > 0
}

// IsDelete returns true if this operation is a pure deletion.
func (op *Operation) IsDelete() bool {
	return !op.Range.IsEmpty() && len(op.NewText) == 0
}

// IsReplace returns true if this operation replaces text.
func (op *Operation) IsReplace() bool {
	return !op.Range.IsEmpty() && len(op.NewText) > 0
}

// IsNoop returns true if this operation makes no changes.
func (op *Operation) IsNoop() bool {
	return op.Range.IsEmpty() && len(op.NewText) == 0
}

// BytesDelta returns the change in document length.
func (op *Operation) BytesDelta() int {
	return len(op.NewText) - int(op.Range.End-op.Range.Start)
}

// NewRange returns the range of the text after the operation.
func (op *Operation) NewRange() Range {
	return Range{
		Start: op.Range.Start,
		End:   op.Range.Start + ByteOffset(len(op.NewText)),
	}
}

// Invert returns an operation that undoes this one.
func (op *Operation) Invert() *Operation {
	return &Operation{
		Range:         op.NewRange(),
		OldText:       op.NewText,
		NewText:       op.OldText,
		CursorsBefore: op.CursorsAfter,
		CursorsAfter:  op.CursorsBefore,
		Timestamp:     time.Now(),
	}
}

// WithCursors sets the cursor state and returns the operation for chaining.
func (op *Operation) WithCursors(before, after []Selection) *Operation {
	op.CursorsBefore = before
	op.CursorsAfter = after
	return op
}

// Clone creates a deep copy of the operation.
func (op *Operation) Clone() *Operation {
	clone := &Operation{
		Range:     op.Range,
		OldText:   op.OldText,
		NewText:   op.NewText,
		Timestamp: op.Timestamp,
	}

	if op.CursorsBefore != nil {
		clone.CursorsBefore = make([]Selection, len(op.CursorsBefore))
		copy(clone.CursorsBefore, op.CursorsBefore)
	}

	if op.CursorsAfter != nil {
		clone.CursorsAfter = make([]Selection, len(op.CursorsAfter))
		copy(clone.CursorsAfter, op.CursorsAfter)
	}

	return clone
}

// OperationInfo provides read-only info about an operation.
// Used for displaying undo/redo history to users.
type OperationInfo struct {
	Description string    // Human-readable description
	Timestamp   time.Time // When the operation occurred
	BytesDelta  int       // Positive for insertions, negative for deletions
}

// OperationList is a collection of operations that can be applied together.
type OperationList []*Operation

// Invert returns a list of inverse operations in reverse order.
func (ops OperationList) Invert() OperationList {
	result := make(OperationList, len(ops))
	for i, op := range ops {
		result[len(ops)-1-i] = op.Invert()
	}
	return result
}

// TotalBytesDelta returns the total change in document length.
func (ops OperationList) TotalBytesDelta() int {
	total := 0
	for _, op := range ops {
		total += op.BytesDelta()
	}
	return total
}
