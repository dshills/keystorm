package rope

import "unicode/utf8"

// chunkIterFrame represents a position in the tree traversal for chunk iteration.
type chunkIterFrame struct {
	node     *Node
	childIdx int        // Next child index to visit (for internal nodes)
	chunkIdx int        // Next chunk index to visit (for leaf nodes)
	offset   ByteOffset // Absolute byte offset at start of this node
}

// ChunkIterator iterates over chunks in a rope.
type ChunkIterator struct {
	rope       Rope
	stack      []chunkIterFrame
	started    bool
	chunk      Chunk
	chunkStart ByteOffset
}

// Chunks returns an iterator over all chunks in the rope.
func (r Rope) Chunks() *ChunkIterator {
	return &ChunkIterator{
		rope:  r,
		stack: make([]chunkIterFrame, 0, 16),
	}
}

// Next advances to the next chunk.
// Returns true if there is a chunk, false if iteration is complete.
func (it *ChunkIterator) Next() bool {
	if !it.started {
		it.started = true
		if it.rope.root == nil {
			return false
		}
		// Initialize stack with root
		it.stack = append(it.stack, chunkIterFrame{
			node:     it.rope.root,
			childIdx: 0,
			chunkIdx: 0,
			offset:   0,
		})
		return it.findNextChunk()
	}

	// Advance to next chunk by incrementing chunkIdx in current leaf
	if len(it.stack) > 0 {
		frame := &it.stack[len(it.stack)-1]
		if frame.node.IsLeaf() {
			frame.chunkIdx++
		}
	}
	return it.findNextChunk()
}

// findNextChunk finds the next available chunk.
func (it *ChunkIterator) findNextChunk() bool {
	for len(it.stack) > 0 {
		frame := &it.stack[len(it.stack)-1]
		node := frame.node

		if node.IsLeaf() {
			if frame.chunkIdx < len(node.chunks) {
				// Calculate offset of this chunk within the leaf
				chunkOffset := frame.offset
				for i := 0; i < frame.chunkIdx; i++ {
					chunkOffset += ByteOffset(node.chunks[i].Len())
				}
				it.chunk = node.chunks[frame.chunkIdx]
				it.chunkStart = chunkOffset
				return true
			}
			// Done with this leaf, pop
			it.stack = it.stack[:len(it.stack)-1]
			// After popping, increment parent's childIdx
			if len(it.stack) > 0 {
				it.stack[len(it.stack)-1].childIdx++
			}
			continue
		}

		// Internal node - descend to next unvisited child
		if frame.childIdx < len(node.children) {
			// Calculate offset at start of this child
			childOffset := frame.offset
			for i := 0; i < frame.childIdx; i++ {
				childOffset += node.childSummaries[i].Bytes
			}

			child := node.children[frame.childIdx]
			it.stack = append(it.stack, chunkIterFrame{
				node:     child,
				childIdx: 0,
				chunkIdx: 0,
				offset:   childOffset,
			})
			continue
		}

		// Done with this internal node, pop
		it.stack = it.stack[:len(it.stack)-1]
		// After popping, increment parent's childIdx
		if len(it.stack) > 0 {
			it.stack[len(it.stack)-1].childIdx++
		}
	}

	return false
}

// Chunk returns the current chunk.
func (it *ChunkIterator) Chunk() Chunk {
	return it.chunk
}

// Offset returns the byte offset of the start of the current chunk.
func (it *ChunkIterator) Offset() ByteOffset {
	return it.chunkStart
}

// LineIterator iterates over lines in a rope.
type LineIterator struct {
	cursor    *Cursor
	lineNum   uint32
	lineStart ByteOffset
	lineEnd   ByteOffset
	text      string
	done      bool
	started   bool
}

// Lines returns an iterator over all lines in the rope.
func (r Rope) Lines() *LineIterator {
	return &LineIterator{
		cursor: NewCursor(r),
	}
}

// Next advances to the next line.
// Returns true if there is a line, false if iteration is complete.
func (it *LineIterator) Next() bool {
	if it.done {
		return false
	}

	if !it.started {
		it.started = true
		if it.cursor.rope.IsEmpty() {
			it.text = ""
			it.lineStart = 0
			it.lineEnd = 0
			it.done = true
			return true // Return empty string for empty rope
		}
	} else {
		// Move to next line
		it.lineNum++
		if it.lineNum >= it.cursor.rope.LineCount() {
			it.done = true
			return false
		}
	}

	// Get line bounds
	it.lineStart = it.cursor.rope.LineStartOffset(it.lineNum)
	it.lineEnd = it.cursor.rope.LineEndOffset(it.lineNum)
	it.text = it.cursor.rope.Slice(it.lineStart, it.lineEnd)

	return true
}

// Text returns the text of the current line (without newline).
func (it *LineIterator) Text() string {
	return it.text
}

// Line returns the current line number (0-indexed).
func (it *LineIterator) Line() uint32 {
	return it.lineNum
}

// StartOffset returns the byte offset of the start of the current line.
func (it *LineIterator) StartOffset() ByteOffset {
	return it.lineStart
}

// EndOffset returns the byte offset of the end of the current line.
func (it *LineIterator) EndOffset() ByteOffset {
	return it.lineEnd
}

// RuneIterator iterates over runes in a rope.
type RuneIterator struct {
	cursor  *Cursor
	current rune
	size    int
	offset  ByteOffset
	started bool
}

// Runes returns an iterator over all runes in the rope.
func (r Rope) Runes() *RuneIterator {
	return &RuneIterator{
		cursor: NewCursor(r),
	}
}

// Next advances to the next rune.
// Returns true if there is a rune, false if iteration is complete.
func (it *RuneIterator) Next() bool {
	if !it.started {
		it.started = true
		if it.cursor.AtEnd() {
			return false
		}
		it.offset = it.cursor.Offset()
		it.current, it.size = it.cursor.Rune()
		return it.size > 0
	}

	// Advance cursor
	if !it.cursor.Next() {
		return false
	}

	if it.cursor.AtEnd() {
		return false
	}

	it.offset = it.cursor.Offset()
	it.current, it.size = it.cursor.Rune()
	return it.size > 0
}

// Rune returns the current rune.
func (it *RuneIterator) Rune() rune {
	return it.current
}

// Size returns the byte size of the current rune.
func (it *RuneIterator) Size() int {
	return it.size
}

// Offset returns the byte offset of the current rune.
func (it *RuneIterator) Offset() ByteOffset {
	return it.offset
}

// ByteIterator iterates over bytes in a rope.
type ByteIterator struct {
	chunkIter *ChunkIterator
	chunkData string
	idx       int
	offset    ByteOffset
	started   bool
}

// Bytes returns an iterator over all bytes in the rope.
func (r Rope) Bytes() *ByteIterator {
	return &ByteIterator{
		chunkIter: r.Chunks(),
	}
}

// Next advances to the next byte.
// Returns true if there is a byte, false if iteration is complete.
func (it *ByteIterator) Next() bool {
	if !it.started {
		it.started = true
		if !it.chunkIter.Next() {
			return false
		}
		it.chunkData = it.chunkIter.Chunk().String()
		it.idx = 0
		it.offset = it.chunkIter.Offset()
		return len(it.chunkData) > 0
	}

	it.idx++
	it.offset++

	if it.idx >= len(it.chunkData) {
		// Move to next chunk
		if !it.chunkIter.Next() {
			return false
		}
		it.chunkData = it.chunkIter.Chunk().String()
		it.idx = 0
		it.offset = it.chunkIter.Offset()
		return len(it.chunkData) > 0
	}

	return true
}

// Byte returns the current byte.
func (it *ByteIterator) Byte() byte {
	if it.idx < len(it.chunkData) {
		return it.chunkData[it.idx]
	}
	return 0
}

// Offset returns the byte offset of the current byte.
func (it *ByteIterator) Offset() ByteOffset {
	return it.offset
}

// ReverseRuneIterator iterates over runes in reverse order.
type ReverseRuneIterator struct {
	rope    Rope
	offset  ByteOffset
	current rune
	size    int
	started bool
}

// ReverseRunes returns an iterator over runes in reverse order.
func (r Rope) ReverseRunes() *ReverseRuneIterator {
	return &ReverseRuneIterator{
		rope:   r,
		offset: r.Len(),
	}
}

// Next moves to the previous rune (advances the reverse iteration).
// Returns true if there is a rune, false if iteration is complete.
func (it *ReverseRuneIterator) Next() bool {
	if !it.started {
		it.started = true
	}

	if it.offset == 0 {
		return false
	}

	// Find start of previous rune
	it.offset--
	for it.offset > 0 {
		b, ok := it.rope.ByteAt(it.offset)
		if !ok {
			return false
		}
		if isUTF8Start(b) {
			break
		}
		it.offset--
	}

	// Decode the rune
	text := it.rope.Slice(it.offset, it.offset+4) // Max UTF-8 is 4 bytes
	it.current, it.size = utf8.DecodeRuneInString(text)

	return it.size > 0
}

// Rune returns the current rune.
func (it *ReverseRuneIterator) Rune() rune {
	return it.current
}

// Size returns the byte size of the current rune.
func (it *ReverseRuneIterator) Size() int {
	return it.size
}

// Offset returns the byte offset of the current rune.
func (it *ReverseRuneIterator) Offset() ByteOffset {
	return it.offset
}
