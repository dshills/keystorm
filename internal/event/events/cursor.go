package events

import "github.com/dshills/keystorm/internal/event/topic"

// Cursor event topics.
const (
	// TopicCursorMoved is published when the primary cursor moves.
	TopicCursorMoved topic.Topic = "cursor.moved"

	// TopicCursorAdded is published when a secondary cursor is added.
	TopicCursorAdded topic.Topic = "cursor.added"

	// TopicCursorRemoved is published when a secondary cursor is removed.
	TopicCursorRemoved topic.Topic = "cursor.removed"

	// TopicCursorSelectionChanged is published when selection changes.
	TopicCursorSelectionChanged topic.Topic = "cursor.selection.changed"

	// TopicCursorAllCleared is published when all secondary cursors are cleared.
	TopicCursorAllCleared topic.Topic = "cursor.all.cleared"
)

// Selection represents a text selection.
type Selection struct {
	// Anchor is the fixed end of the selection (where selection started).
	Anchor Position

	// Head is the movable end of the selection (where cursor is).
	Head Position

	// IsReversed indicates if the selection is backward (head before anchor).
	IsReversed bool
}

// IsEmpty returns true if the selection has zero length.
func (s Selection) IsEmpty() bool {
	return s.Anchor == s.Head
}

// CursorMoved is published when the primary cursor moves.
type CursorMoved struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// OldPosition is where the cursor was before.
	OldPosition Position

	// NewPosition is where the cursor is now.
	NewPosition Position

	// Selection is the current selection, nil if none.
	Selection *Selection

	// Reason describes what caused the move (e.g., "user", "edit", "scroll").
	Reason string
}

// CursorAdded is published when a secondary cursor is added.
type CursorAdded struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// CursorID is the unique identifier of the new cursor.
	CursorID string

	// Position is where the cursor was added.
	Position Position

	// Selection is the cursor's selection, nil if none.
	Selection *Selection

	// Index is the position in the cursor list.
	Index int
}

// CursorRemoved is published when a secondary cursor is removed.
type CursorRemoved struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// CursorID is the unique identifier of the removed cursor.
	CursorID string

	// Position was the cursor's position before removal.
	Position Position
}

// CursorSelectionChanged is published when selection changes.
type CursorSelectionChanged struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// CursorID is the cursor's identifier (empty for primary cursor).
	CursorID string

	// OldSelection was the previous selection, nil if none.
	OldSelection *Selection

	// NewSelection is the new selection, nil if cleared.
	NewSelection *Selection
}

// CursorAllCleared is published when all secondary cursors are cleared.
type CursorAllCleared struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// RemovedCount is the number of cursors that were removed.
	RemovedCount int
}
