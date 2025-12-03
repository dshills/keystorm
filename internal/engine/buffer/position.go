package buffer

import (
	"fmt"
	"sync/atomic"
)

// ByteOffset represents a byte position in the buffer.
// This is the fundamental position type, directly indexing into the text.
type ByteOffset = int64

// Point represents a line and column position.
// Both Line and Column are 0-indexed.
// Column is measured in bytes from the start of the line.
type Point struct {
	Line   uint32 // 0-indexed line number
	Column uint32 // 0-indexed column (byte offset within line)
}

// String returns a human-readable representation of the point.
func (p Point) String() string {
	return fmt.Sprintf("(%d:%d)", p.Line, p.Column)
}

// Compare returns -1 if p < other, 0 if p == other, 1 if p > other.
func (p Point) Compare(other Point) int {
	if p.Line < other.Line {
		return -1
	}
	if p.Line > other.Line {
		return 1
	}
	if p.Column < other.Column {
		return -1
	}
	if p.Column > other.Column {
		return 1
	}
	return 0
}

// Before returns true if p comes before other.
func (p Point) Before(other Point) bool {
	return p.Compare(other) < 0
}

// After returns true if p comes after other.
func (p Point) After(other Point) bool {
	return p.Compare(other) > 0
}

// IsZero returns true if this is the zero point (0:0).
func (p Point) IsZero() bool {
	return p.Line == 0 && p.Column == 0
}

// PointUTF16 represents a line and column position where the column
// is measured in UTF-16 code units. This is used for LSP compatibility
// since many editors and the LSP protocol use UTF-16 encoding.
type PointUTF16 struct {
	Line   uint32 // 0-indexed line number
	Column uint32 // 0-indexed column in UTF-16 code units
}

// String returns a human-readable representation of the point.
func (p PointUTF16) String() string {
	return fmt.Sprintf("(%d:%d utf16)", p.Line, p.Column)
}

// Compare returns -1 if p < other, 0 if p == other, 1 if p > other.
func (p PointUTF16) Compare(other PointUTF16) int {
	if p.Line < other.Line {
		return -1
	}
	if p.Line > other.Line {
		return 1
	}
	if p.Column < other.Column {
		return -1
	}
	if p.Column > other.Column {
		return 1
	}
	return 0
}

// Before returns true if p comes before other.
func (p PointUTF16) Before(other PointUTF16) bool {
	return p.Compare(other) < 0
}

// After returns true if p comes after other.
func (p PointUTF16) After(other PointUTF16) bool {
	return p.Compare(other) > 0
}

// IsZero returns true if this is the zero point (0:0).
func (p PointUTF16) IsZero() bool {
	return p.Line == 0 && p.Column == 0
}

// RevisionID uniquely identifies a buffer revision.
// Each modification to the buffer creates a new revision.
type RevisionID uint64

// revisionCounter is used to generate unique revision IDs.
var revisionCounter uint64

// NewRevisionID generates a new unique revision ID.
// This is thread-safe using atomic operations.
func NewRevisionID() RevisionID {
	return RevisionID(atomic.AddUint64(&revisionCounter, 1))
}
