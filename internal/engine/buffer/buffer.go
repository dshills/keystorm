package buffer

import (
	"errors"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/engine/rope"
)

// Errors returned by buffer operations.
var (
	ErrOffsetOutOfRange = errors.New("offset out of range")
	ErrRangeInvalid     = errors.New("invalid range")
	ErrEditsOverlap     = errors.New("edits overlap or are not in reverse order")
)

// LineEnding specifies the line ending style.
type LineEnding uint8

const (
	LineEndingLF   LineEnding = iota // Unix: \n
	LineEndingCRLF                   // Windows: \r\n
	LineEndingCR                     // Old Mac: \r
)

// String returns the string representation of the line ending.
func (le LineEnding) String() string {
	switch le {
	case LineEndingLF:
		return "\\n"
	case LineEndingCRLF:
		return "\\r\\n"
	case LineEndingCR:
		return "\\r"
	default:
		return "\\n"
	}
}

// Sequence returns the actual line ending characters.
func (le LineEnding) Sequence() string {
	switch le {
	case LineEndingLF:
		return "\n"
	case LineEndingCRLF:
		return "\r\n"
	case LineEndingCR:
		return "\r"
	default:
		return "\n"
	}
}

// Buffer wraps a Rope with additional editor functionality.
// It provides the primary interface for text manipulation.
// All methods are thread-safe.
type Buffer struct {
	mu         sync.RWMutex
	rope       rope.Rope
	revisionID RevisionID
	lineEnding LineEnding
	tabWidth   int
}

// NewBuffer creates a new empty buffer.
func NewBuffer(opts ...Option) *Buffer {
	b := &Buffer{
		rope:       rope.New(),
		revisionID: NewRevisionID(),
		lineEnding: LineEndingLF,
		tabWidth:   4,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// NewBufferFromString creates a buffer with initial content.
func NewBufferFromString(s string, opts ...Option) *Buffer {
	b := NewBuffer(opts...)
	s = b.normalizeLineEndings(s)
	b.rope = rope.FromString(s)
	return b
}

// NewBufferFromReader creates a buffer from an io.Reader.
func NewBufferFromReader(r io.Reader, opts ...Option) (*Buffer, error) {
	b := NewBuffer(opts...)

	// Read all content first to handle line ending normalization correctly
	// (CRLF sequences may be split across read boundaries)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	text := b.normalizeLineEndings(string(data))
	b.rope = rope.FromString(text)
	return b, nil
}

// normalizeLineEndings converts all line endings to the buffer's preferred style.
func (b *Buffer) normalizeLineEndings(s string) string {
	if b.lineEnding == LineEndingLF {
		// Normalize CRLF and CR to LF
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
	} else if b.lineEnding == LineEndingCRLF {
		// First normalize to LF, then convert to CRLF
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
		s = strings.ReplaceAll(s, "\n", "\r\n")
	} else if b.lineEnding == LineEndingCR {
		// Normalize CRLF and LF to CR
		s = strings.ReplaceAll(s, "\r\n", "\r")
		s = strings.ReplaceAll(s, "\n", "\r")
	}
	return s
}

// Read Operations

// Text returns the full buffer content as a string.
// For large buffers, prefer using TextRange or iterators.
func (b *Buffer) Text() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rope.String()
}

// TextRange returns text in the given byte range.
func (b *Buffer) TextRange(start, end ByteOffset) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rope.Slice(rope.ByteOffset(start), rope.ByteOffset(end))
}

// Len returns the total byte length of the buffer.
func (b *Buffer) Len() ByteOffset {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return ByteOffset(b.rope.Len())
}

// LineCount returns the number of lines.
func (b *Buffer) LineCount() uint32 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rope.LineCount()
}

// LineText returns the text of a specific line (without newline).
func (b *Buffer) LineText(line uint32) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rope.LineText(line)
}

// LineLen returns the length of a specific line in bytes (without newline).
func (b *Buffer) LineLen(line uint32) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	start := b.rope.LineStartOffset(line)
	end := b.rope.LineEndOffset(line)
	return int(end - start)
}

// ByteAt returns the byte at the given offset.
func (b *Buffer) ByteAt(offset ByteOffset) (byte, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rope.ByteAt(rope.ByteOffset(offset))
}

// RuneAt returns the rune at the given byte offset.
// Returns utf8.RuneError and size 0 if offset is out of range.
func (b *Buffer) RuneAt(offset ByteOffset) (rune, int) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ropeLen := ByteOffset(b.rope.Len())
	if offset < 0 || offset >= ropeLen {
		return utf8.RuneError, 0
	}

	// Get up to 4 bytes (max UTF-8 rune length)
	end := offset + 4
	if end > ropeLen {
		end = ropeLen
	}

	s := b.rope.Slice(rope.ByteOffset(offset), rope.ByteOffset(end))
	return utf8.DecodeRuneInString(s)
}

// Coordinate Conversion

// OffsetToPoint converts a byte offset to line/column.
func (b *Buffer) OffsetToPoint(offset ByteOffset) Point {
	b.mu.RLock()
	defer b.mu.RUnlock()
	p := b.rope.OffsetToPoint(rope.ByteOffset(offset))
	return Point{Line: p.Line, Column: p.Column}
}

// PointToOffset converts line/column to byte offset.
func (b *Buffer) PointToOffset(point Point) ByteOffset {
	b.mu.RLock()
	defer b.mu.RUnlock()
	p := rope.Point{Line: point.Line, Column: point.Column}
	return ByteOffset(b.rope.PointToOffset(p))
}

// OffsetToPointUTF16 converts a byte offset to UTF-16 line/column.
func (b *Buffer) OffsetToPointUTF16(offset ByteOffset) PointUTF16 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	point := b.rope.OffsetToPoint(rope.ByteOffset(offset))
	lineStart := b.rope.LineStartOffset(point.Line)
	lineText := b.rope.Slice(lineStart, rope.ByteOffset(offset))

	// Count UTF-16 code units
	utf16Col := utf16ColumnFromString(lineText)

	return PointUTF16{Line: point.Line, Column: utf16Col}
}

// PointUTF16ToOffset converts UTF-16 line/column to byte offset.
func (b *Buffer) PointUTF16ToOffset(point PointUTF16) ByteOffset {
	b.mu.RLock()
	defer b.mu.RUnlock()

	lineStart := b.rope.LineStartOffset(point.Line)
	lineEnd := b.rope.LineEndOffset(point.Line)
	lineText := b.rope.Slice(lineStart, lineEnd)

	// Convert UTF-16 column to byte offset within the line
	byteCol := byteOffsetFromUTF16Column(lineText, point.Column)

	return ByteOffset(lineStart) + ByteOffset(byteCol)
}

// LineStartOffset returns the byte offset of the start of a line.
func (b *Buffer) LineStartOffset(line uint32) ByteOffset {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return ByteOffset(b.rope.LineStartOffset(line))
}

// LineEndOffset returns the byte offset of the end of a line (before newline).
func (b *Buffer) LineEndOffset(line uint32) ByteOffset {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return ByteOffset(b.rope.LineEndOffset(line))
}

// Write Operations

// Insert inserts text at the given offset.
// Returns the end position of the inserted text.
func (b *Buffer) Insert(offset ByteOffset, text string) (ByteOffset, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if offset < 0 || offset > ByteOffset(b.rope.Len()) {
		return 0, ErrOffsetOutOfRange
	}

	text = b.normalizeLineEndings(text)
	b.rope = b.rope.Insert(rope.ByteOffset(offset), text)
	b.revisionID = NewRevisionID()

	return offset + ByteOffset(len(text)), nil
}

// Delete removes text in the given range.
func (b *Buffer) Delete(start, end ByteOffset) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if start < 0 || start > end || end > ByteOffset(b.rope.Len()) {
		return ErrRangeInvalid
	}

	b.rope = b.rope.Delete(rope.ByteOffset(start), rope.ByteOffset(end))
	b.revisionID = NewRevisionID()

	return nil
}

// Replace replaces text in the given range with new text.
// Returns the end position of the replacement text.
func (b *Buffer) Replace(start, end ByteOffset, text string) (ByteOffset, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if start < 0 || start > end || end > ByteOffset(b.rope.Len()) {
		return 0, ErrRangeInvalid
	}

	text = b.normalizeLineEndings(text)
	b.rope = b.rope.Replace(rope.ByteOffset(start), rope.ByteOffset(end), text)
	b.revisionID = NewRevisionID()

	return start + ByteOffset(len(text)), nil
}

// ApplyEdit applies a single edit to the buffer.
func (b *Buffer) ApplyEdit(edit Edit) (EditResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if edit.Range.Start < 0 || edit.Range.Start > edit.Range.End ||
		edit.Range.End > ByteOffset(b.rope.Len()) {
		return EditResult{}, ErrRangeInvalid
	}

	oldText := b.rope.Slice(rope.ByteOffset(edit.Range.Start), rope.ByteOffset(edit.Range.End))
	text := b.normalizeLineEndings(edit.NewText)
	b.rope = b.rope.Replace(rope.ByteOffset(edit.Range.Start), rope.ByteOffset(edit.Range.End), text)
	b.revisionID = NewRevisionID()

	newEnd := edit.Range.Start + ByteOffset(len(text))

	return EditResult{
		OldRange: edit.Range,
		NewRange: Range{Start: edit.Range.Start, End: newEnd},
		OldText:  oldText,
		Delta:    int64(len(text)) - int64(edit.Range.Len()),
	}, nil
}

// ApplyEdits applies multiple edits atomically.
// Edits must be in reverse order (highest offset first) to maintain validity.
func (b *Buffer) ApplyEdits(edits []Edit) error {
	if len(edits) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Validate edits are in reverse order and non-overlapping
	for i := 1; i < len(edits); i++ {
		if edits[i].Range.End > edits[i-1].Range.Start {
			return ErrEditsOverlap
		}
	}

	// Validate all ranges
	ropeLen := ByteOffset(b.rope.Len())
	for _, edit := range edits {
		if edit.Range.Start < 0 || edit.Range.Start > edit.Range.End ||
			edit.Range.End > ropeLen {
			return ErrRangeInvalid
		}
	}

	// Apply edits in reverse order
	for _, edit := range edits {
		text := b.normalizeLineEndings(edit.NewText)
		b.rope = b.rope.Replace(rope.ByteOffset(edit.Range.Start), rope.ByteOffset(edit.Range.End), text)
	}

	b.revisionID = NewRevisionID()
	return nil
}

// Buffer State

// RevisionID returns the current revision ID.
func (b *Buffer) RevisionID() RevisionID {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.revisionID
}

// IsEmpty returns true if the buffer is empty.
func (b *Buffer) IsEmpty() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rope.IsEmpty()
}

// LineEnding returns the buffer's line ending style.
func (b *Buffer) LineEnding() LineEnding {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lineEnding
}

// TabWidth returns the buffer's tab width.
func (b *Buffer) TabWidth() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.tabWidth
}

// SetLineEnding sets the buffer's line ending style.
// This does not convert existing line endings.
func (b *Buffer) SetLineEnding(le LineEnding) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lineEnding = le
}

// SetTabWidth sets the buffer's tab width.
func (b *Buffer) SetTabWidth(width int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tabWidth = width
}

// Snapshot returns a read-only snapshot of the current buffer state.
// Safe for concurrent access from other goroutines.
func (b *Buffer) Snapshot() *Snapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return &Snapshot{
		rope:       b.rope, // Ropes are immutable, safe to share
		revisionID: b.revisionID,
		lineEnding: b.lineEnding,
		tabWidth:   b.tabWidth,
	}
}

// Helper functions for UTF-16 conversion

// utf16ColumnFromString counts UTF-16 code units in a string.
func utf16ColumnFromString(s string) uint32 {
	var col uint32
	for _, r := range s {
		if r >= 0x10000 {
			col += 2 // Surrogate pair (characters outside BMP)
		} else {
			col++
		}
	}
	return col
}

// byteOffsetFromUTF16Column converts a UTF-16 column to byte offset within a line.
func byteOffsetFromUTF16Column(line string, utf16Col uint32) int {
	var col uint32
	var byteOffset int

	for _, r := range line {
		if col >= utf16Col {
			break
		}

		// Count UTF-16 code units without allocating
		if r >= 0x10000 {
			col += 2 // Surrogate pair
		} else {
			col++
		}
		byteOffset += utf8.RuneLen(r)
	}

	return byteOffset
}
