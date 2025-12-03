package tracking

import (
	"fmt"
	"strings"

	"github.com/dshills/keystorm/internal/engine/buffer"
)

// ChangeType categorizes the type of a change.
type ChangeType uint8

const (
	// ChangeInsert indicates text was inserted (OldText is empty).
	ChangeInsert ChangeType = iota

	// ChangeDelete indicates text was deleted (NewText is empty).
	ChangeDelete

	// ChangeReplace indicates text was replaced (both OldText and NewText present).
	ChangeReplace
)

// String returns a human-readable representation of the change type.
func (ct ChangeType) String() string {
	switch ct {
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
// It captures both what changed and where, enabling reconstruction
// of the transformation from old to new state.
type Change struct {
	// Type indicates whether this is an insert, delete, or replace.
	Type ChangeType

	// Range is the affected range in the OLD text (before the change).
	// For inserts, Start == End (point insertion).
	Range buffer.Range

	// NewRange is the affected range in the NEW text (after the change).
	// For deletes, Start == End.
	NewRange buffer.Range

	// OldText is the text that was removed (empty for inserts).
	OldText string

	// NewText is the text that was added (empty for deletes).
	NewText string

	// RevisionID is the revision after this change was applied.
	RevisionID RevisionID
}

// NewInsertChange creates a change representing an insertion.
func NewInsertChange(offset buffer.ByteOffset, text string, revID RevisionID) Change {
	return Change{
		Type:       ChangeInsert,
		Range:      buffer.Range{Start: offset, End: offset},
		NewRange:   buffer.Range{Start: offset, End: offset + buffer.ByteOffset(len(text))},
		NewText:    text,
		RevisionID: revID,
	}
}

// NewDeleteChange creates a change representing a deletion.
func NewDeleteChange(start, end buffer.ByteOffset, oldText string, revID RevisionID) Change {
	return Change{
		Type:       ChangeDelete,
		Range:      buffer.Range{Start: start, End: end},
		NewRange:   buffer.Range{Start: start, End: start},
		OldText:    oldText,
		RevisionID: revID,
	}
}

// NewReplaceChange creates a change representing a replacement.
func NewReplaceChange(start, end buffer.ByteOffset, oldText, newText string, revID RevisionID) Change {
	return Change{
		Type:       ChangeReplace,
		Range:      buffer.Range{Start: start, End: end},
		NewRange:   buffer.Range{Start: start, End: start + buffer.ByteOffset(len(newText))},
		OldText:    oldText,
		NewText:    newText,
		RevisionID: revID,
	}
}

// String returns a human-readable representation of the change.
func (c Change) String() string {
	switch c.Type {
	case ChangeInsert:
		text := c.NewText
		if len(text) > 20 {
			text = text[:17] + "..."
		}
		return fmt.Sprintf("Insert %q at %d", text, c.Range.Start)
	case ChangeDelete:
		text := c.OldText
		if len(text) > 20 {
			text = text[:17] + "..."
		}
		return fmt.Sprintf("Delete %q at %v", text, c.Range)
	case ChangeReplace:
		oldText := c.OldText
		if len(oldText) > 10 {
			oldText = oldText[:7] + "..."
		}
		newText := c.NewText
		if len(newText) > 10 {
			newText = newText[:7] + "..."
		}
		return fmt.Sprintf("Replace %q with %q at %v", oldText, newText, c.Range)
	default:
		return "Unknown change"
	}
}

// Delta returns the byte delta of this change.
// Positive means the buffer grew, negative means it shrank.
func (c Change) Delta() int64 {
	return int64(len(c.NewText)) - int64(len(c.OldText))
}

// IsInsert returns true if this is a pure insertion.
func (c Change) IsInsert() bool {
	return c.Type == ChangeInsert
}

// IsDelete returns true if this is a pure deletion.
func (c Change) IsDelete() bool {
	return c.Type == ChangeDelete
}

// IsReplace returns true if this is a replacement.
func (c Change) IsReplace() bool {
	return c.Type == ChangeReplace
}

// Invert returns a change that undoes this change.
func (c Change) Invert() Change {
	return Change{
		Type:       c.invertedType(),
		Range:      c.NewRange,
		NewRange:   c.Range,
		OldText:    c.NewText,
		NewText:    c.OldText,
		RevisionID: c.RevisionID, // Note: This doesn't create a new revision
	}
}

func (c Change) invertedType() ChangeType {
	switch c.Type {
	case ChangeInsert:
		return ChangeDelete
	case ChangeDelete:
		return ChangeInsert
	default:
		return ChangeReplace
	}
}

// ChangeSet represents a collection of related changes.
// Changes are stored in the order they should be applied.
type ChangeSet struct {
	// Changes in application order.
	Changes []Change

	// StartRevision is the revision before any changes.
	StartRevision RevisionID

	// EndRevision is the revision after all changes.
	EndRevision RevisionID
}

// NewChangeSet creates an empty change set starting at the given revision.
func NewChangeSet(startRevision RevisionID) *ChangeSet {
	return &ChangeSet{
		StartRevision: startRevision,
		EndRevision:   startRevision,
	}
}

// Add adds a change to the set.
func (cs *ChangeSet) Add(c Change) {
	cs.Changes = append(cs.Changes, c)
	cs.EndRevision = c.RevisionID
}

// Len returns the number of changes.
func (cs *ChangeSet) Len() int {
	return len(cs.Changes)
}

// IsEmpty returns true if there are no changes.
func (cs *ChangeSet) IsEmpty() bool {
	return len(cs.Changes) == 0
}

// TotalDelta returns the total byte delta of all changes.
func (cs *ChangeSet) TotalDelta() int64 {
	var delta int64
	for _, c := range cs.Changes {
		delta += c.Delta()
	}
	return delta
}

// Summary returns a human-readable summary of the changes.
func (cs *ChangeSet) Summary() string {
	if cs.IsEmpty() {
		return "no changes"
	}

	var inserts, deletes, replaces int
	var insertedBytes, deletedBytes int64

	for _, c := range cs.Changes {
		switch c.Type {
		case ChangeInsert:
			inserts++
			insertedBytes += int64(len(c.NewText))
		case ChangeDelete:
			deletes++
			deletedBytes += int64(len(c.OldText))
		case ChangeReplace:
			replaces++
			insertedBytes += int64(len(c.NewText))
			deletedBytes += int64(len(c.OldText))
		}
	}

	var parts []string
	if inserts > 0 {
		parts = append(parts, fmt.Sprintf("%d inserts (+%d bytes)", inserts, insertedBytes))
	}
	if deletes > 0 {
		parts = append(parts, fmt.Sprintf("%d deletes (-%d bytes)", deletes, deletedBytes))
	}
	if replaces > 0 {
		parts = append(parts, fmt.Sprintf("%d replaces", replaces))
	}

	return strings.Join(parts, ", ")
}

// trackedChange pairs a change with its revision for internal storage.
type trackedChange struct {
	revision RevisionID
	change   Change
}
