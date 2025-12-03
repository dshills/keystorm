package rope

import (
	"io"
	"strings"
)

// Rope is an immutable rope data structure for efficient text storage.
// Operations return new Rope values; the original is never modified.
// This enables cheap snapshots and thread-safe concurrent read access.
type Rope struct {
	root *Node
}

// New creates an empty rope.
func New() Rope {
	return Rope{root: newLeafNode()}
}

// FromString creates a rope from a string.
func FromString(s string) Rope {
	if len(s) == 0 {
		return New()
	}

	chunks := splitIntoChunks(s)
	return buildFromChunks(chunks)
}

// FromReader creates a rope from an io.Reader.
func FromReader(r io.Reader) (Rope, error) {
	var builder Builder
	buf := make([]byte, 64*1024) // 64KB read buffer

	for {
		n, err := r.Read(buf)
		if n > 0 {
			builder.WriteString(string(buf[:n]))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return Rope{}, err
		}
	}

	return builder.Build(), nil
}

// buildFromChunks builds a rope from a slice of chunks.
func buildFromChunks(chunks []Chunk) Rope {
	if len(chunks) == 0 {
		return New()
	}

	// Build leaf nodes
	var leaves []*Node
	for i := 0; i < len(chunks); i += MaxChunksPerLeaf {
		end := i + MaxChunksPerLeaf
		if end > len(chunks) {
			end = len(chunks)
		}
		leafChunks := make([]Chunk, end-i)
		copy(leafChunks, chunks[i:end])
		leaves = append(leaves, newLeafNodeWithChunks(leafChunks))
	}

	// Build tree bottom-up
	nodes := leaves
	for len(nodes) > 1 {
		var parents []*Node
		for i := 0; i < len(nodes); i += MaxChildren {
			end := i + MaxChildren
			if end > len(nodes) {
				end = len(nodes)
			}
			children := make([]*Node, end-i)
			copy(children, nodes[i:end])
			parents = append(parents, newInternalNode(children))
		}
		nodes = parents
	}

	if len(nodes) == 0 {
		return New()
	}
	return Rope{root: nodes[0]}
}

// Len returns the total byte length.
func (r Rope) Len() ByteOffset {
	if r.root == nil {
		return 0
	}
	return r.root.Len()
}

// LineCount returns the number of lines (newlines + 1).
func (r Rope) LineCount() uint32 {
	if r.root == nil {
		return 1
	}
	return r.root.LineCount()
}

// IsEmpty returns true if the rope contains no text.
func (r Rope) IsEmpty() bool {
	return r.Len() == 0
}

// String returns the full text as a string.
// Use sparingly for large ropes.
func (r Rope) String() string {
	if r.root == nil {
		return ""
	}

	var sb strings.Builder
	sb.Grow(int(r.Len()))
	r.root.appendTo(&sb)
	return sb.String()
}

// Slice returns the text in the byte range [start, end).
func (r Rope) Slice(start, end ByteOffset) string {
	if r.root == nil || start >= end {
		return ""
	}
	return r.root.textInRange(start, end)
}

// ByteAt returns the byte at the given offset.
// Returns 0 and false if offset is out of range.
func (r Rope) ByteAt(offset ByteOffset) (byte, bool) {
	if r.root == nil || offset >= r.Len() {
		return 0, false
	}

	// Navigate to the byte
	node := r.root
	for !node.IsLeaf() {
		idx, childOffset := node.findChildByOffset(offset)
		node = node.children[idx]
		offset = childOffset
	}

	// Find byte within leaf chunks
	for _, chunk := range node.chunks {
		chunkLen := ByteOffset(chunk.Len())
		if offset < chunkLen {
			return chunk.String()[offset], true
		}
		offset -= chunkLen
	}

	return 0, false
}

// Insert inserts text at the given byte offset.
// Returns a new rope; original is unchanged.
func (r Rope) Insert(offset ByteOffset, text string) Rope {
	if len(text) == 0 {
		return r
	}

	if r.root == nil || r.Len() == 0 {
		return FromString(text)
	}

	if offset == 0 {
		return FromString(text).Concat(r)
	}

	if offset >= r.Len() {
		return r.Concat(FromString(text))
	}

	// Split at offset, insert in middle
	left, right := r.Split(offset)
	return left.Concat(FromString(text)).Concat(right)
}

// Delete removes text in the byte range [start, end).
// Returns a new rope; original is unchanged.
func (r Rope) Delete(start, end ByteOffset) Rope {
	if r.root == nil || start >= end {
		return r
	}

	// Clamp to valid range
	ropeLen := r.Len()
	if start >= ropeLen {
		return r
	}
	if end > ropeLen {
		end = ropeLen
	}

	// Handle edge cases
	if start == 0 && end >= ropeLen {
		return New()
	}
	if start == 0 {
		_, right := r.Split(end)
		return right
	}
	if end >= ropeLen {
		left, _ := r.Split(start)
		return left
	}

	// Split around the deleted region
	left, temp := r.Split(start)
	_, right := temp.Split(end - start)

	return left.Concat(right)
}

// Replace replaces text in the byte range [start, end) with new text.
// Returns a new rope; original is unchanged.
func (r Rope) Replace(start, end ByteOffset, text string) Rope {
	if start >= end && len(text) == 0 {
		return r
	}

	// Optimize for simple cases
	if start >= end {
		return r.Insert(start, text)
	}
	if len(text) == 0 {
		return r.Delete(start, end)
	}

	return r.Delete(start, end).Insert(start, text)
}

// Split splits the rope at offset, returning two ropes.
// Left rope contains [0, offset), right contains [offset, end).
func (r Rope) Split(offset ByteOffset) (Rope, Rope) {
	if r.root == nil || offset == 0 {
		return New(), r
	}
	if offset >= r.Len() {
		return r, New()
	}

	leftRoot, rightRoot := r.root.split(offset)
	return Rope{root: leftRoot}, Rope{root: rightRoot}
}

// Concat concatenates two ropes.
// Returns a new rope; originals are unchanged.
func (r Rope) Concat(other Rope) Rope {
	if r.root == nil || r.Len() == 0 {
		return other
	}
	if other.root == nil || other.Len() == 0 {
		return r
	}

	newRoot := concat(r.root, other.root)
	return Rope{root: newRoot}
}

// Summary returns the aggregated metrics for the entire rope.
func (r Rope) Summary() TextSummary {
	if r.root == nil {
		return TextSummary{Flags: FlagASCII}
	}
	return r.root.summary
}

// LineStartOffset returns the byte offset of the start of the given line.
// Lines are 0-indexed.
func (r Rope) LineStartOffset(line uint32) ByteOffset {
	if r.root == nil || line == 0 {
		return 0
	}

	if line >= r.LineCount() {
		return r.Len()
	}

	// Find the line by counting newlines
	cursor := NewCursor(r)
	if cursor.SeekLine(line) {
		return cursor.Offset()
	}
	return r.Len()
}

// LineEndOffset returns the byte offset of the end of the given line
// (not including the newline character).
func (r Rope) LineEndOffset(line uint32) ByteOffset {
	if r.root == nil {
		return 0
	}

	lineCount := r.LineCount()
	if line >= lineCount {
		return r.Len()
	}

	// Start of next line minus 1 (the newline), or end of rope
	if line == lineCount-1 {
		return r.Len()
	}

	nextLineStart := r.LineStartOffset(line + 1)
	if nextLineStart > 0 {
		return nextLineStart - 1
	}
	return 0
}

// LineText returns the text of the given line (not including newline).
func (r Rope) LineText(line uint32) string {
	start := r.LineStartOffset(line)
	end := r.LineEndOffset(line)
	return r.Slice(start, end)
}

// OffsetToPoint converts a byte offset to a line/column position.
func (r Rope) OffsetToPoint(offset ByteOffset) Point {
	if r.root == nil || offset == 0 {
		return Point{Line: 0, Column: 0}
	}

	if offset >= r.Len() {
		// Return position at end
		lastLine := r.LineCount() - 1
		return Point{
			Line:   lastLine,
			Column: uint32(r.Len() - r.LineStartOffset(lastLine)),
		}
	}

	cursor := NewCursor(r)
	cursor.SeekOffset(offset)
	return cursor.Point()
}

// PointToOffset converts a line/column position to a byte offset.
func (r Rope) PointToOffset(point Point) ByteOffset {
	if r.root == nil {
		return 0
	}

	lineStart := r.LineStartOffset(point.Line)
	lineEnd := r.LineEndOffset(point.Line)
	lineLen := lineEnd - lineStart

	if ByteOffset(point.Column) >= lineLen {
		return lineEnd
	}
	return lineStart + ByteOffset(point.Column)
}

// Height returns the height of the rope tree.
// Useful for debugging and testing balance.
func (r Rope) Height() int {
	if r.root == nil {
		return 0
	}
	return int(r.root.height) + 1
}

// ChunkCount returns the total number of chunks in the rope.
// Useful for debugging.
func (r Rope) ChunkCount() int {
	if r.root == nil {
		return 0
	}
	return countChunks(r.root)
}

func countChunks(n *Node) int {
	if n.IsLeaf() {
		return len(n.chunks)
	}
	count := 0
	for _, child := range n.children {
		count += countChunks(child)
	}
	return count
}

// Equals returns true if two ropes contain the same text.
// Note: This compares content, not structure.
func (r Rope) Equals(other Rope) bool {
	if r.Len() != other.Len() {
		return false
	}
	// For efficiency, compare chunk by chunk using iterators
	iter1 := r.Chunks()
	iter2 := other.Chunks()

	for iter1.Next() {
		if !iter2.Next() {
			return false
		}
		if iter1.Chunk().String() != iter2.Chunk().String() {
			return false
		}
	}
	return !iter2.Next()
}
