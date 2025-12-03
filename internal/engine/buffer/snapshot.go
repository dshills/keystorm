package buffer

import (
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/engine/rope"
)

// Snapshot provides a read-only view of a buffer at a specific point in time.
// It is safe for concurrent access and will not change even if the original
// buffer is modified.
type Snapshot struct {
	rope       rope.Rope
	revisionID RevisionID
	lineEnding LineEnding
	tabWidth   int
}

// Text returns the full snapshot content as a string.
func (s *Snapshot) Text() string {
	return s.rope.String()
}

// TextRange returns text in the given byte range.
func (s *Snapshot) TextRange(start, end ByteOffset) string {
	return s.rope.Slice(rope.ByteOffset(start), rope.ByteOffset(end))
}

// Len returns the total byte length of the snapshot.
func (s *Snapshot) Len() ByteOffset {
	return ByteOffset(s.rope.Len())
}

// LineCount returns the number of lines.
func (s *Snapshot) LineCount() uint32 {
	return s.rope.LineCount()
}

// LineText returns the text of a specific line (without newline).
func (s *Snapshot) LineText(line uint32) string {
	return s.rope.LineText(line)
}

// LineLen returns the length of a specific line in bytes (without newline).
func (s *Snapshot) LineLen(line uint32) int {
	start := s.rope.LineStartOffset(line)
	end := s.rope.LineEndOffset(line)
	return int(end - start)
}

// ByteAt returns the byte at the given offset.
func (s *Snapshot) ByteAt(offset ByteOffset) (byte, bool) {
	return s.rope.ByteAt(rope.ByteOffset(offset))
}

// RuneAt returns the rune at the given byte offset.
// Returns utf8.RuneError and size 0 if offset is out of range.
func (s *Snapshot) RuneAt(offset ByteOffset) (rune, int) {
	ropeLen := ByteOffset(s.rope.Len())
	if offset < 0 || offset >= ropeLen {
		return utf8.RuneError, 0
	}

	// Get up to 4 bytes (max UTF-8 rune length)
	end := offset + 4
	if end > ropeLen {
		end = ropeLen
	}

	str := s.rope.Slice(rope.ByteOffset(offset), rope.ByteOffset(end))
	return utf8.DecodeRuneInString(str)
}

// OffsetToPoint converts a byte offset to line/column.
func (s *Snapshot) OffsetToPoint(offset ByteOffset) Point {
	p := s.rope.OffsetToPoint(rope.ByteOffset(offset))
	return Point{Line: p.Line, Column: p.Column}
}

// PointToOffset converts line/column to byte offset.
func (s *Snapshot) PointToOffset(point Point) ByteOffset {
	p := rope.Point{Line: point.Line, Column: point.Column}
	return ByteOffset(s.rope.PointToOffset(p))
}

// OffsetToPointUTF16 converts a byte offset to UTF-16 line/column.
func (s *Snapshot) OffsetToPointUTF16(offset ByteOffset) PointUTF16 {
	point := s.rope.OffsetToPoint(rope.ByteOffset(offset))
	lineStart := s.rope.LineStartOffset(point.Line)
	lineText := s.rope.Slice(lineStart, rope.ByteOffset(offset))

	utf16Col := utf16ColumnFromString(lineText)

	return PointUTF16{Line: point.Line, Column: utf16Col}
}

// PointUTF16ToOffset converts UTF-16 line/column to byte offset.
func (s *Snapshot) PointUTF16ToOffset(point PointUTF16) ByteOffset {
	lineStart := s.rope.LineStartOffset(point.Line)
	lineEnd := s.rope.LineEndOffset(point.Line)
	lineText := s.rope.Slice(lineStart, lineEnd)

	byteCol := byteOffsetFromUTF16Column(lineText, point.Column)

	return ByteOffset(lineStart) + ByteOffset(byteCol)
}

// LineStartOffset returns the byte offset of the start of a line.
func (s *Snapshot) LineStartOffset(line uint32) ByteOffset {
	return ByteOffset(s.rope.LineStartOffset(line))
}

// LineEndOffset returns the byte offset of the end of a line (before newline).
func (s *Snapshot) LineEndOffset(line uint32) ByteOffset {
	return ByteOffset(s.rope.LineEndOffset(line))
}

// RevisionID returns the revision ID of this snapshot.
func (s *Snapshot) RevisionID() RevisionID {
	return s.revisionID
}

// IsEmpty returns true if the snapshot is empty.
func (s *Snapshot) IsEmpty() bool {
	return s.rope.IsEmpty()
}

// LineEnding returns the snapshot's line ending style.
func (s *Snapshot) LineEnding() LineEnding {
	return s.lineEnding
}

// TabWidth returns the snapshot's tab width.
func (s *Snapshot) TabWidth() int {
	return s.tabWidth
}

// Chunks returns an iterator over all chunks in the snapshot's rope.
func (s *Snapshot) Chunks() *rope.ChunkIterator {
	return s.rope.Chunks()
}

// Lines returns an iterator over all lines in the snapshot.
func (s *Snapshot) Lines() *rope.LineIterator {
	return s.rope.Lines()
}

// Runes returns an iterator over all runes in the snapshot.
func (s *Snapshot) Runes() *rope.RuneIterator {
	return s.rope.Runes()
}

// Bytes returns an iterator over all bytes in the snapshot.
func (s *Snapshot) Bytes() *rope.ByteIterator {
	return s.rope.Bytes()
}
