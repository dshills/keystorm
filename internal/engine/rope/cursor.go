package rope

import "unicode/utf8"

// Cursor enables efficient traversal of a rope.
// It maintains a path from root to the current position, allowing
// for O(log n) seeking and O(1) local movement.
type Cursor struct {
	rope     Rope
	path     []cursorFrame
	offset   ByteOffset // Current byte offset in the rope
	point    Point      // Current line/column (lazily computed)
	pointSet bool       // Whether point has been computed

	// Current position within a leaf
	leafNode *Node // Current leaf node
	chunkIdx int   // Index of current chunk within leaf
	chunkOff int   // Byte offset within current chunk
}

// cursorFrame represents a position in the tree traversal path.
type cursorFrame struct {
	node     *Node
	childIdx int        // Index of the child we descended into
	offset   ByteOffset // Byte offset at the start of this node
	line     uint32     // Line number at the start of this node
}

// NewCursor creates a cursor at the start of the rope.
func NewCursor(r Rope) *Cursor {
	c := &Cursor{
		rope:   r,
		path:   make([]cursorFrame, 0, 16),
		offset: 0,
		point:  Point{},
	}
	c.seekToStart()
	return c
}

// seekToStart positions the cursor at the beginning of the rope.
func (c *Cursor) seekToStart() {
	c.path = c.path[:0]
	c.offset = 0
	c.point = Point{}
	c.pointSet = true

	if c.rope.root == nil {
		c.leafNode = nil
		return
	}

	// Descend to leftmost leaf
	node := c.rope.root
	for !node.IsLeaf() {
		c.path = append(c.path, cursorFrame{
			node:     node,
			childIdx: 0,
			offset:   0,
			line:     0,
		})
		node = node.children[0]
	}

	c.leafNode = node
	c.chunkIdx = 0
	c.chunkOff = 0
}

// Offset returns the current byte offset.
func (c *Cursor) Offset() ByteOffset {
	return c.offset
}

// Point returns the current line/column position.
func (c *Cursor) Point() Point {
	if !c.pointSet {
		c.computePoint()
	}
	return c.point
}

// computePoint calculates the current line/column from the path.
func (c *Cursor) computePoint() {
	c.point = Point{Line: 0, Column: 0}

	// Sum up lines from path
	for _, frame := range c.path {
		for i := 0; i < frame.childIdx; i++ {
			c.point.Line += frame.node.childSummaries[i].Lines
		}
	}

	// Count lines in current leaf up to current position
	if c.leafNode != nil {
		for i := 0; i < c.chunkIdx; i++ {
			c.point.Line += c.leafNode.chunks[i].Summary().Lines
		}

		// Count lines within current chunk up to current offset
		if c.chunkIdx < len(c.leafNode.chunks) {
			chunk := c.leafNode.chunks[c.chunkIdx]
			text := chunk.String()[:c.chunkOff]
			for _, ch := range text {
				if ch == '\n' {
					c.point.Line++
				}
			}
		}
	}

	// Calculate column (bytes from last newline)
	c.point.Column = c.computeColumn()
	c.pointSet = true
}

// computeColumn calculates the column (bytes from the last newline).
func (c *Cursor) computeColumn() uint32 {
	// Walk backward from current position to find last newline
	lineStart := c.LineStartOffset()
	return uint32(c.offset - lineStart)
}

// LineStartOffset returns the byte offset of the start of the current line.
func (c *Cursor) LineStartOffset() ByteOffset {
	if c.offset == 0 {
		return 0
	}

	// Use cursor position to search within current chunk first using newline index
	if c.leafNode != nil && c.chunkIdx < len(c.leafNode.chunks) {
		chunk := c.leafNode.chunks[c.chunkIdx]
		newlines := chunk.Newlines()

		// Use newline index for O(1) lookup within chunk
		pos := newlines.NewlineBefore(c.chunkOff)
		if pos >= 0 {
			// Found newline in current chunk
			chunkStart := c.offset - ByteOffset(c.chunkOff)
			return chunkStart + ByteOffset(pos) + 1
		}

		// Not found in current chunk - need to search earlier chunks/leaves
		// Calculate offset at start of current chunk
		chunkStart := c.offset - ByteOffset(c.chunkOff)

		// Search earlier chunks in current leaf using newline index
		for i := c.chunkIdx - 1; i >= 0; i-- {
			prevChunk := c.leafNode.chunks[i]
			chunkStart -= ByteOffset(prevChunk.Len())

			// Use newline index to find last newline in this chunk
			prevNewlines := prevChunk.Newlines()
			lastPos := prevNewlines.LastNewlinePosition()
			if lastPos >= 0 {
				return chunkStart + ByteOffset(lastPos) + 1
			}
		}

		// Not found in current leaf - fall back to rope-level search
		// This is O(log n) per byte but only for cross-leaf searches
		searchOffset := chunkStart
		for searchOffset > 0 {
			b, ok := c.rope.ByteAt(searchOffset - 1)
			if !ok {
				break
			}
			if b == '\n' {
				return searchOffset
			}
			searchOffset--
		}
	}

	return 0
}

// SeekOffset moves the cursor to the given byte offset.
// Returns true if successful, false if offset is out of range.
// The offset must be at a valid UTF-8 rune boundary.
func (c *Cursor) SeekOffset(offset ByteOffset) bool {
	if c.rope.root == nil {
		return offset == 0
	}

	ropeLen := c.rope.Len()
	if offset > ropeLen {
		return false
	}

	// Reset path
	c.path = c.path[:0]
	c.offset = offset
	c.pointSet = false

	// Handle edge case at end
	if offset == ropeLen {
		return c.seekToEnd()
	}

	// Descend from root, tracking absolute offsets
	node := c.rope.root
	nodeStartOffset := ByteOffset(0) // Absolute offset at start of current node
	nodeStartLine := uint32(0)       // Line number at start of current node

	for !node.IsLeaf() {
		// Find child containing the target offset
		childStartOffset := nodeStartOffset
		childStartLine := nodeStartLine
		found := false

		for i, summary := range node.childSummaries {
			childEndOffset := childStartOffset + summary.Bytes
			if childEndOffset > offset {
				// Target is in this child
				c.path = append(c.path, cursorFrame{
					node:     node,
					childIdx: i,
					offset:   childStartOffset,
					line:     childStartLine,
				})
				node = node.children[i]
				nodeStartOffset = childStartOffset
				nodeStartLine = childStartLine
				found = true
				break
			}
			childStartOffset = childEndOffset
			childStartLine += summary.Lines
		}

		if !found {
			// Shouldn't happen, but handle gracefully
			return false
		}
	}

	// Find position within leaf
	c.leafNode = node
	chunkStartOffset := nodeStartOffset

	for i, chunk := range node.chunks {
		chunkLen := ByteOffset(chunk.Len())
		chunkEndOffset := chunkStartOffset + chunkLen
		if chunkEndOffset > offset {
			c.chunkIdx = i
			c.chunkOff = int(offset - chunkStartOffset)

			// Verify we're at a UTF-8 boundary
			if c.chunkOff > 0 {
				text := chunk.String()
				if c.chunkOff < len(text) && !isUTF8Start(text[c.chunkOff]) {
					// Not at a UTF-8 boundary - adjust backward to find start
					for c.chunkOff > 0 && !isUTF8Start(text[c.chunkOff]) {
						c.chunkOff--
						c.offset--
					}
				}
			}
			return true
		}
		chunkStartOffset = chunkEndOffset
	}

	// Position at end of leaf
	c.chunkIdx = len(node.chunks) - 1
	if c.chunkIdx >= 0 {
		c.chunkOff = node.chunks[c.chunkIdx].Len()
	} else {
		c.chunkOff = 0
	}

	return true
}

// seekToEnd positions cursor at the end of the rope.
func (c *Cursor) seekToEnd() bool {
	c.path = c.path[:0]
	c.offset = c.rope.Len()
	c.pointSet = false

	if c.rope.root == nil {
		c.leafNode = nil
		return true
	}

	// Descend to rightmost leaf
	node := c.rope.root
	currentOffset := ByteOffset(0)
	currentLine := uint32(0)

	for !node.IsLeaf() {
		lastIdx := len(node.children) - 1
		for i := 0; i < lastIdx; i++ {
			currentOffset += node.childSummaries[i].Bytes
			currentLine += node.childSummaries[i].Lines
		}
		c.path = append(c.path, cursorFrame{
			node:     node,
			childIdx: lastIdx,
			offset:   currentOffset,
			line:     currentLine,
		})
		node = node.children[lastIdx]
	}

	c.leafNode = node
	if len(node.chunks) > 0 {
		c.chunkIdx = len(node.chunks) - 1
		c.chunkOff = node.chunks[c.chunkIdx].Len()
	} else {
		c.chunkIdx = 0
		c.chunkOff = 0
	}

	return true
}

// SeekLine moves the cursor to the start of the given line.
// Returns true if successful, false if line is out of range.
func (c *Cursor) SeekLine(line uint32) bool {
	if c.rope.root == nil {
		return line == 0
	}

	if line == 0 {
		c.seekToStart()
		return true
	}

	if line >= c.rope.LineCount() {
		return false
	}

	// Reset path
	c.path = c.path[:0]
	c.pointSet = false

	// Descend from root, tracking lines
	node := c.rope.root
	currentOffset := ByteOffset(0)
	currentLine := uint32(0)
	targetLines := line // We need to find 'line' newlines

	for !node.IsLeaf() {
		found := false
		for i, summary := range node.childSummaries {
			if currentLine+summary.Lines >= targetLines {
				c.path = append(c.path, cursorFrame{
					node:     node,
					childIdx: i,
					offset:   currentOffset,
					line:     currentLine,
				})
				node = node.children[i]
				found = true
				break
			}
			currentOffset += summary.Bytes
			currentLine += summary.Lines
		}

		if !found {
			return false
		}
	}

	// Find position within leaf
	c.leafNode = node
	remainingLines := targetLines - currentLine

	for i, chunk := range node.chunks {
		summary := chunk.Summary()
		if summary.Lines >= remainingLines {
			// Target line is in this chunk - use the newline index for O(1) lookup
			c.chunkIdx = i
			newlines := chunk.Newlines()
			pos := newlines.FindNthNewline(remainingLines)
			if pos < 0 {
				// Newline not found where expected - this shouldn't happen
				// if summaries are correct, but handle gracefully
				return false
			}
			c.chunkOff = pos + 1 // Position after newline
			c.offset = currentOffset + ByteOffset(c.chunkOff)
			c.point = Point{Line: line, Column: 0}
			c.pointSet = true
			return true
		}
		remainingLines -= summary.Lines
		currentOffset += ByteOffset(chunk.Len())
	}

	return false
}

// findNthNewline finds the byte position of the nth newline in s.
// Returns the position of the newline, or -1 if not found.
//
//nolint:unused // retained for future line-based cursor operations
func findNthNewline(s string, n uint32) int {
	if n == 0 {
		return -1
	}

	count := uint32(0)
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			count++
			if count == n {
				return i
			}
		}
	}
	return -1
}

// Rune returns the rune at the current position.
// Returns (0, 0) if at end or empty.
func (c *Cursor) Rune() (rune, int) {
	if c.leafNode == nil || c.chunkIdx >= len(c.leafNode.chunks) {
		return 0, 0
	}

	chunk := c.leafNode.chunks[c.chunkIdx]
	if c.chunkOff >= chunk.Len() {
		return 0, 0
	}

	return utf8.DecodeRuneInString(chunk.String()[c.chunkOff:])
}

// Byte returns the byte at the current position.
// Returns (0, false) if at end or empty.
func (c *Cursor) Byte() (byte, bool) {
	if c.leafNode == nil || c.chunkIdx >= len(c.leafNode.chunks) {
		return 0, false
	}

	chunk := c.leafNode.chunks[c.chunkIdx]
	if c.chunkOff >= chunk.Len() {
		return 0, false
	}

	return chunk.String()[c.chunkOff], true
}

// Next advances the cursor by one rune.
// Returns false if already at end.
func (c *Cursor) Next() bool {
	if c.offset >= c.rope.Len() {
		return false
	}

	r, size := c.Rune()
	if size == 0 {
		return false
	}

	// Update position
	c.offset += ByteOffset(size)
	c.chunkOff += size

	// Update point if tracking
	if c.pointSet {
		if r == '\n' {
			c.point.Line++
			c.point.Column = 0
		} else {
			c.point.Column += uint32(size)
		}
	}

	// Check if we need to move to next chunk
	if c.leafNode != nil && c.chunkIdx < len(c.leafNode.chunks) {
		if c.chunkOff >= c.leafNode.chunks[c.chunkIdx].Len() {
			c.advanceChunk()
		}
	}

	return true
}

// advanceChunk moves to the next chunk.
func (c *Cursor) advanceChunk() {
	c.chunkIdx++
	c.chunkOff = 0

	// Check if we need to move to next leaf
	if c.chunkIdx >= len(c.leafNode.chunks) {
		c.advanceLeaf()
	}
}

// advanceLeaf moves to the next leaf node.
func (c *Cursor) advanceLeaf() {
	// Walk back up the path until we can go right
	for len(c.path) > 0 {
		frame := c.path[len(c.path)-1]
		c.path = c.path[:len(c.path)-1]

		// Try to move to next sibling
		nextIdx := frame.childIdx + 1
		if nextIdx < len(frame.node.children) {
			// Calculate the absolute offset and line for the next sibling
			nextSiblingOffset := frame.offset + frame.node.childSummaries[frame.childIdx].Bytes
			nextSiblingLine := frame.line + frame.node.childSummaries[frame.childIdx].Lines

			// Can go right - push frame for next sibling
			c.path = append(c.path, cursorFrame{
				node:     frame.node,
				childIdx: nextIdx,
				offset:   nextSiblingOffset,
				line:     nextSiblingLine,
			})

			// Descend to leftmost leaf, tracking offsets properly
			node := frame.node.children[nextIdx]
			currentOffset := nextSiblingOffset
			currentLine := nextSiblingLine

			for !node.IsLeaf() {
				c.path = append(c.path, cursorFrame{
					node:     node,
					childIdx: 0,
					offset:   currentOffset,
					line:     currentLine,
				})
				// First child starts at same offset as parent
				node = node.children[0]
			}

			c.leafNode = node
			c.chunkIdx = 0
			c.chunkOff = 0
			return
		}
	}

	// Reached end of rope
	c.leafNode = nil
	c.chunkIdx = 0
	c.chunkOff = 0
}

// Prev moves the cursor back by one rune.
// Returns false if already at start.
func (c *Cursor) Prev() bool {
	if c.offset == 0 {
		return false
	}

	// Find the previous rune by looking at the byte before current position
	prevOffset := c.offset - 1

	// Handle UTF-8 by finding the start of the previous rune
	for prevOffset > 0 {
		b, ok := c.rope.ByteAt(prevOffset)
		if !ok {
			break
		}
		if isUTF8Start(b) {
			break
		}
		prevOffset--
	}

	// Seek to the new position
	c.SeekOffset(prevOffset)
	return true
}

// AtEnd returns true if the cursor is at the end of the rope.
func (c *Cursor) AtEnd() bool {
	return c.offset >= c.rope.Len()
}

// AtStart returns true if the cursor is at the start of the rope.
func (c *Cursor) AtStart() bool {
	return c.offset == 0
}

// Clone creates a copy of the cursor at the same position.
func (c *Cursor) Clone() *Cursor {
	newCursor := &Cursor{
		rope:     c.rope,
		path:     make([]cursorFrame, len(c.path)),
		offset:   c.offset,
		point:    c.point,
		pointSet: c.pointSet,
		leafNode: c.leafNode,
		chunkIdx: c.chunkIdx,
		chunkOff: c.chunkOff,
	}
	copy(newCursor.path, c.path)
	return newCursor
}
