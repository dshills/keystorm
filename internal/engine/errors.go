package engine

import "errors"

// Errors returned by engine operations.
var (
	// ErrOffsetOutOfRange indicates an offset is outside the valid buffer range.
	ErrOffsetOutOfRange = errors.New("offset out of range")

	// ErrRangeInvalid indicates an invalid range (e.g., end < start).
	ErrRangeInvalid = errors.New("invalid range")

	// ErrEditsOverlap indicates edits overlap or are not in reverse order.
	ErrEditsOverlap = errors.New("edits overlap or are not in reverse order")

	// ErrNothingToUndo indicates the undo stack is empty.
	ErrNothingToUndo = errors.New("nothing to undo")

	// ErrNothingToRedo indicates the redo stack is empty.
	ErrNothingToRedo = errors.New("nothing to redo")

	// ErrSnapshotNotFound indicates a snapshot was not found.
	ErrSnapshotNotFound = errors.New("snapshot not found")

	// ErrRevisionNotFound indicates a revision was not found.
	ErrRevisionNotFound = errors.New("revision not found")

	// ErrReadOnly indicates an operation was attempted on a read-only engine.
	ErrReadOnly = errors.New("engine is read-only")
)
