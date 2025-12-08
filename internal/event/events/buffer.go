package events

import "github.com/dshills/keystorm/internal/event/topic"

// Buffer event topics.
const (
	// TopicBufferContentInserted is published when text is inserted into a buffer.
	TopicBufferContentInserted topic.Topic = "buffer.content.inserted"

	// TopicBufferContentDeleted is published when text is deleted from a buffer.
	TopicBufferContentDeleted topic.Topic = "buffer.content.deleted"

	// TopicBufferContentReplaced is published when text is replaced in a buffer.
	TopicBufferContentReplaced topic.Topic = "buffer.content.replaced"

	// TopicBufferRevisionChanged is published when a new revision is created.
	TopicBufferRevisionChanged topic.Topic = "buffer.revision.changed"

	// TopicBufferSnapshotCreated is published when a named snapshot is created.
	TopicBufferSnapshotCreated topic.Topic = "buffer.snapshot.created"

	// TopicBufferCleared is published when a buffer is cleared.
	TopicBufferCleared topic.Topic = "buffer.cleared"

	// TopicBufferReadOnlyChanged is published when read-only state changes.
	TopicBufferReadOnlyChanged topic.Topic = "buffer.readonly.changed"

	// TopicBufferCreated is published when a new buffer is created.
	TopicBufferCreated topic.Topic = "buffer.created"

	// TopicBufferClosed is published when a buffer is closed.
	TopicBufferClosed topic.Topic = "buffer.closed"

	// TopicBufferSaved is published when a buffer is saved to disk.
	TopicBufferSaved topic.Topic = "buffer.saved"

	// TopicBufferDirtyChanged is published when dirty state changes.
	TopicBufferDirtyChanged topic.Topic = "buffer.dirty.changed"
)

// Position represents a position in a buffer.
type Position struct {
	// Line is the zero-based line number.
	Line int

	// Column is the zero-based column number (in bytes).
	Column int

	// Offset is the zero-based byte offset from the start of the buffer.
	Offset int
}

// Range represents a range in a buffer.
type Range struct {
	// Start is the beginning of the range (inclusive).
	Start Position

	// End is the end of the range (exclusive).
	End Position
}

// BufferContentInserted is published when text is inserted into a buffer.
type BufferContentInserted struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// Position is where the text was inserted.
	Position Position

	// Text is the inserted text content.
	Text string

	// NewRange is the range covered by the new text.
	NewRange Range

	// RevisionID is the revision after the insertion.
	RevisionID string
}

// BufferContentDeleted is published when text is deleted from a buffer.
type BufferContentDeleted struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// Range is the range that was deleted.
	Range Range

	// DeletedText is the text that was removed.
	DeletedText string

	// RevisionID is the revision after the deletion.
	RevisionID string
}

// BufferContentReplaced is published when text is replaced in a buffer.
type BufferContentReplaced struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// OldRange is the range that was replaced.
	OldRange Range

	// NewRange is the range of the new text.
	NewRange Range

	// OldText is the text that was replaced.
	OldText string

	// NewText is the replacement text.
	NewText string

	// RevisionID is the revision after the replacement.
	RevisionID string
}

// BufferRevisionChanged is published when a new revision is created.
type BufferRevisionChanged struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// RevisionID is the new revision identifier.
	RevisionID string

	// PreviousID is the previous revision identifier.
	PreviousID string

	// ChangeCount is the number of changes in this revision.
	ChangeCount int
}

// BufferSnapshotCreated is published when a named snapshot is created.
type BufferSnapshotCreated struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// SnapshotID is the unique identifier of the snapshot.
	SnapshotID string

	// Name is the user-provided name for the snapshot.
	Name string

	// RevisionID is the revision at the time of the snapshot.
	RevisionID string
}

// BufferCleared is published when a buffer is cleared.
type BufferCleared struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// RevisionID is the revision after clearing.
	RevisionID string
}

// BufferReadOnlyChanged is published when read-only state changes.
type BufferReadOnlyChanged struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// IsReadOnly indicates whether the buffer is now read-only.
	IsReadOnly bool
}

// BufferCreated is published when a new buffer is created.
type BufferCreated struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// FilePath is the associated file path, if any.
	FilePath string

	// LanguageID identifies the language for syntax highlighting.
	LanguageID string
}

// BufferClosed is published when a buffer is closed.
type BufferClosed struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// FilePath is the associated file path, if any.
	FilePath string
}

// BufferSaved is published when a buffer is saved to disk.
type BufferSaved struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// FilePath is the path where the buffer was saved.
	FilePath string

	// RevisionID is the revision that was saved.
	RevisionID string
}

// BufferDirtyChanged is published when dirty state changes.
type BufferDirtyChanged struct {
	// BufferID is the unique identifier of the buffer.
	BufferID string

	// IsDirty indicates whether the buffer has unsaved changes.
	IsDirty bool
}
