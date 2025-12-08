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
// Uses chunk-based iteration for amortized O(1) per rune.
type ReverseRuneIterator struct {
	rope    Rope
	offset  ByteOffset // Current rune's byte offset
	current rune
	size    int
	started bool

	// Chunk caching for amortized O(1) access
	chunkData   string     // Current chunk's text data
	chunkStart  ByteOffset // Start offset of current chunk
	chunkIdx    int        // Current position within chunk (pointing at start of current rune)
	chunksStack []reverseChunkFrame
}

// reverseChunkFrame tracks position in tree for reverse chunk traversal.
type reverseChunkFrame struct {
	node     *Node
	childIdx int // Current child index (for internal nodes)
	chunkIdx int // Current chunk index (for leaf nodes)
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
		// Initialize by finding the last chunk
		if it.rope.IsEmpty() {
			return false
		}
		if !it.loadLastChunk() {
			return false
		}
		// Position at end of last chunk
		it.chunkIdx = len(it.chunkData)
	}

	// Move backwards in current chunk to find previous rune
	if it.chunkIdx > 0 {
		return it.prevRuneInChunk()
	}

	// Need to load previous chunk
	if !it.loadPrevChunk() {
		return false
	}

	// Position at end of new chunk
	it.chunkIdx = len(it.chunkData)
	return it.prevRuneInChunk()
}

// prevRuneInChunk moves to the previous rune within the current chunk.
func (it *ReverseRuneIterator) prevRuneInChunk() bool {
	if it.chunkIdx <= 0 {
		return false
	}

	// Move backwards to find start of previous rune
	it.chunkIdx--
	for it.chunkIdx > 0 && !isUTF8Start(it.chunkData[it.chunkIdx]) {
		it.chunkIdx--
	}

	// Decode the rune
	it.current, it.size = utf8.DecodeRuneInString(it.chunkData[it.chunkIdx:])
	it.offset = it.chunkStart + ByteOffset(it.chunkIdx)

	return it.size > 0
}

// loadLastChunk initializes the iterator to point at the last chunk.
func (it *ReverseRuneIterator) loadLastChunk() bool {
	if it.rope.root == nil {
		return false
	}

	// Build stack by descending to rightmost leaf
	it.chunksStack = make([]reverseChunkFrame, 0, 16)
	node := it.rope.root
	offset := ByteOffset(0)

	for !node.IsLeaf() {
		lastChild := len(node.children) - 1
		// Calculate offset to this child
		for i := 0; i < lastChild; i++ {
			offset += node.childSummaries[i].Bytes
		}
		it.chunksStack = append(it.chunksStack, reverseChunkFrame{
			node:     node,
			childIdx: lastChild,
		})
		node = node.children[lastChild]
	}

	// Now at a leaf - get last chunk
	if len(node.chunks) == 0 {
		return false
	}

	lastChunk := len(node.chunks) - 1
	// Calculate offset to this chunk
	for i := 0; i < lastChunk; i++ {
		offset += ByteOffset(node.chunks[i].Len())
	}

	it.chunksStack = append(it.chunksStack, reverseChunkFrame{
		node:     node,
		chunkIdx: lastChunk,
	})

	it.chunkData = node.chunks[lastChunk].String()
	it.chunkStart = offset

	return true
}

// loadPrevChunk loads the previous chunk in reverse order.
func (it *ReverseRuneIterator) loadPrevChunk() bool {
	if len(it.chunksStack) == 0 {
		return false
	}

	// Pop current leaf frame
	frame := &it.chunksStack[len(it.chunksStack)-1]

	if frame.node.IsLeaf() {
		// Try previous chunk in same leaf
		if frame.chunkIdx > 0 {
			frame.chunkIdx--
			// Recalculate chunk start offset
			offset := it.calculateNodeOffset(len(it.chunksStack) - 1)
			for i := 0; i < frame.chunkIdx; i++ {
				offset += ByteOffset(frame.node.chunks[i].Len())
			}
			it.chunkData = frame.node.chunks[frame.chunkIdx].String()
			it.chunkStart = offset
			return true
		}

		// Need to go up and find previous subtree
		it.chunksStack = it.chunksStack[:len(it.chunksStack)-1]
	}

	// Walk up and find a node with a previous child
	for len(it.chunksStack) > 0 {
		frame := &it.chunksStack[len(it.chunksStack)-1]

		if frame.childIdx > 0 {
			frame.childIdx--
			// Descend to rightmost leaf of this subtree
			return it.descendToRightmostLeaf(len(it.chunksStack) - 1)
		}

		it.chunksStack = it.chunksStack[:len(it.chunksStack)-1]
	}

	return false
}

// descendToRightmostLeaf descends from the current position to the rightmost leaf.
func (it *ReverseRuneIterator) descendToRightmostLeaf(stackIdx int) bool {
	frame := it.chunksStack[stackIdx]
	node := frame.node.children[frame.childIdx]
	offset := it.calculateNodeOffset(stackIdx)

	// Calculate offset to this child
	for i := 0; i < frame.childIdx; i++ {
		offset += frame.node.childSummaries[i].Bytes
	}

	for !node.IsLeaf() {
		lastChild := len(node.children) - 1
		for i := 0; i < lastChild; i++ {
			offset += node.childSummaries[i].Bytes
		}
		it.chunksStack = append(it.chunksStack, reverseChunkFrame{
			node:     node,
			childIdx: lastChild,
		})
		node = node.children[lastChild]
	}

	if len(node.chunks) == 0 {
		return false
	}

	lastChunk := len(node.chunks) - 1
	for i := 0; i < lastChunk; i++ {
		offset += ByteOffset(node.chunks[i].Len())
	}

	it.chunksStack = append(it.chunksStack, reverseChunkFrame{
		node:     node,
		chunkIdx: lastChunk,
	})

	it.chunkData = node.chunks[lastChunk].String()
	it.chunkStart = offset

	return true
}

// calculateNodeOffset calculates the byte offset at the start of a node in the stack.
func (it *ReverseRuneIterator) calculateNodeOffset(stackIdx int) ByteOffset {
	var offset ByteOffset
	for i := 0; i < stackIdx; i++ {
		frame := it.chunksStack[i]
		if !frame.node.IsLeaf() {
			for j := 0; j < frame.childIdx; j++ {
				offset += frame.node.childSummaries[j].Bytes
			}
		}
	}
	return offset
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
