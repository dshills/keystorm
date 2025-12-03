package cursor

import (
	"fmt"

	"github.com/dshills/keystorm/internal/engine/buffer"
)

// ByteOffset is an alias for buffer.ByteOffset for convenience.
type ByteOffset = buffer.ByteOffset

// Point is an alias for buffer.Point for convenience.
type Point = buffer.Point

// Cursor represents an insertion point in the buffer.
// Cursor is an immutable value type.
type Cursor struct {
	offset ByteOffset
}

// NewCursor creates a cursor at the given offset.
func NewCursor(offset ByteOffset) Cursor {
	if offset < 0 {
		offset = 0
	}
	return Cursor{offset: offset}
}

// Offset returns the cursor's byte offset.
func (c Cursor) Offset() ByteOffset {
	return c.offset
}

// MoveTo returns a new cursor at the given offset.
func (c Cursor) MoveTo(offset ByteOffset) Cursor {
	if offset < 0 {
		offset = 0
	}
	return Cursor{offset: offset}
}

// MoveBy returns a new cursor shifted by delta bytes.
func (c Cursor) MoveBy(delta ByteOffset) Cursor {
	newOffset := c.offset + delta
	if newOffset < 0 {
		newOffset = 0
	}
	return Cursor{offset: newOffset}
}

// Clamp returns a cursor clamped to the valid range [0, maxOffset].
func (c Cursor) Clamp(maxOffset ByteOffset) Cursor {
	if c.offset < 0 {
		return Cursor{offset: 0}
	}
	if c.offset > maxOffset {
		return Cursor{offset: maxOffset}
	}
	return c
}

// String returns a string representation of the cursor.
func (c Cursor) String() string {
	return fmt.Sprintf("Cursor(%d)", c.offset)
}

// Equals returns true if two cursors are at the same position.
func (c Cursor) Equals(other Cursor) bool {
	return c.offset == other.offset
}

// Compare returns -1 if c < other, 0 if c == other, 1 if c > other.
func (c Cursor) Compare(other Cursor) int {
	if c.offset < other.offset {
		return -1
	}
	if c.offset > other.offset {
		return 1
	}
	return 0
}

// Before returns true if c is before other.
func (c Cursor) Before(other Cursor) bool {
	return c.offset < other.offset
}

// After returns true if c is after other.
func (c Cursor) After(other Cursor) bool {
	return c.offset > other.offset
}

// ToSelection converts this cursor to a selection with no extent.
func (c Cursor) ToSelection() Selection {
	return Selection{Anchor: c.offset, Head: c.offset}
}
