package rope

import "strings"

// Tree structure constants
const (
	// MinChildren is the minimum children per internal node (except root).
	MinChildren = 4

	// MaxChildren is the maximum children per internal node before splitting.
	MaxChildren = 8

	// MaxChunksPerLeaf is the maximum chunks in a leaf node.
	MaxChunksPerLeaf = 4
)

// Node represents a node in the rope B+ tree.
// Leaf nodes (height == 0) contain text chunks.
// Internal nodes (height > 0) contain child node references.
type Node struct {
	height  uint8       // 0 for leaves, >0 for internal
	summary TextSummary // Aggregated metrics for entire subtree

	// Internal node fields (height > 0)
	children       []*Node       // Child nodes
	childSummaries []TextSummary // Per-child summaries for efficient seeking

	// Leaf node fields (height == 0)
	chunks []Chunk // Text chunks in this leaf
}

// newLeafNode creates an empty leaf node.
func newLeafNode() *Node {
	return &Node{
		height: 0,
		chunks: make([]Chunk, 0, MaxChunksPerLeaf),
	}
}

// newLeafNodeWithChunks creates a leaf node with the given chunks.
func newLeafNodeWithChunks(chunks []Chunk) *Node {
	n := &Node{
		height: 0,
		chunks: chunks,
	}
	n.recomputeSummary()
	return n
}

// newInternalNode creates an internal node with the given children.
func newInternalNode(children []*Node) *Node {
	if len(children) == 0 {
		return newLeafNode()
	}

	height := children[0].height + 1
	summaries := make([]TextSummary, len(children))
	var total TextSummary

	for i, child := range children {
		summaries[i] = child.summary
		total = total.Add(child.summary)
	}

	return &Node{
		height:         height,
		summary:        total,
		children:       children,
		childSummaries: summaries,
	}
}

// IsLeaf returns true if this is a leaf node.
func (n *Node) IsLeaf() bool {
	return n.height == 0
}

// Len returns the byte length of text in this subtree.
func (n *Node) Len() ByteOffset {
	return n.summary.Bytes
}

// LineCount returns the number of lines in this subtree.
func (n *Node) LineCount() uint32 {
	return n.summary.Lines + 1
}

// recomputeSummary recalculates the summary from children or chunks.
func (n *Node) recomputeSummary() {
	if n.IsLeaf() {
		n.summary = TextSummary{Flags: FlagASCII}
		for _, chunk := range n.chunks {
			n.summary = n.summary.Add(chunk.Summary())
		}
	} else {
		n.summary = TextSummary{Flags: FlagASCII}
		n.childSummaries = make([]TextSummary, len(n.children))
		for i, child := range n.children {
			n.childSummaries[i] = child.summary
			n.summary = n.summary.Add(child.summary)
		}
	}
}

// clone creates a shallow copy of the node.
func (n *Node) clone() *Node {
	if n.IsLeaf() {
		chunks := make([]Chunk, len(n.chunks))
		copy(chunks, n.chunks)
		return &Node{
			height:  0,
			summary: n.summary,
			chunks:  chunks,
		}
	}

	children := make([]*Node, len(n.children))
	copy(children, n.children)
	summaries := make([]TextSummary, len(n.childSummaries))
	copy(summaries, n.childSummaries)

	return &Node{
		height:         n.height,
		summary:        n.summary,
		children:       children,
		childSummaries: summaries,
	}
}

// appendTo appends all text in this subtree to the builder.
func (n *Node) appendTo(sb *strings.Builder) {
	if n.IsLeaf() {
		for _, chunk := range n.chunks {
			sb.WriteString(chunk.String())
		}
		return
	}

	for _, child := range n.children {
		child.appendTo(sb)
	}
}

// textInRange extracts text in the byte range [start, end).
func (n *Node) textInRange(start, end ByteOffset) string {
	if start >= end || start >= n.Len() {
		return ""
	}
	if end > n.Len() {
		end = n.Len()
	}

	var sb strings.Builder
	sb.Grow(int(end - start))
	n.appendRange(&sb, start, end)
	return sb.String()
}

// appendRange appends text in the byte range to the builder.
func (n *Node) appendRange(sb *strings.Builder, start, end ByteOffset) {
	if start >= end {
		return
	}

	if n.IsLeaf() {
		offset := ByteOffset(0)
		for _, chunk := range n.chunks {
			chunkLen := ByteOffset(chunk.Len())
			chunkEnd := offset + chunkLen

			if chunkEnd <= start {
				offset = chunkEnd
				continue
			}
			if offset >= end {
				break
			}

			// Calculate slice bounds within chunk
			sliceStart := 0
			if start > offset {
				sliceStart = int(start - offset)
			}
			sliceEnd := chunk.Len()
			if end < chunkEnd {
				sliceEnd = int(end - offset)
			}

			sb.WriteString(chunk.String()[sliceStart:sliceEnd])
			offset = chunkEnd
		}
		return
	}

	// Internal node
	offset := ByteOffset(0)
	for i, child := range n.children {
		childLen := n.childSummaries[i].Bytes
		childEnd := offset + childLen

		if childEnd <= start {
			offset = childEnd
			continue
		}
		if offset >= end {
			break
		}

		// Adjust range for child
		childStart := ByteOffset(0)
		if start > offset {
			childStart = start - offset
		}
		childEndAdj := childLen
		if end < childEnd {
			childEndAdj = end - offset
		}

		child.appendRange(sb, childStart, childEndAdj)
		offset = childEnd
	}
}

// split splits the node at the given byte offset.
// Returns two nodes: left contains [0, offset), right contains [offset, end).
func (n *Node) split(offset ByteOffset) (*Node, *Node) {
	if offset <= 0 {
		return newLeafNode(), n.clone()
	}
	if offset >= n.Len() {
		return n.clone(), newLeafNode()
	}

	if n.IsLeaf() {
		return n.splitLeaf(offset)
	}
	return n.splitInternal(offset)
}

// splitLeaf splits a leaf node at the given offset.
func (n *Node) splitLeaf(offset ByteOffset) (*Node, *Node) {
	var leftChunks, rightChunks []Chunk
	currentOffset := ByteOffset(0)

	for _, chunk := range n.chunks {
		chunkLen := ByteOffset(chunk.Len())

		if currentOffset+chunkLen <= offset {
			// Entire chunk goes to left
			leftChunks = append(leftChunks, chunk)
		} else if currentOffset >= offset {
			// Entire chunk goes to right
			rightChunks = append(rightChunks, chunk)
		} else {
			// Need to split this chunk
			splitPoint := int(offset - currentOffset)
			left, right := chunk.Split(splitPoint)
			if !left.IsEmpty() {
				leftChunks = append(leftChunks, left)
			}
			if !right.IsEmpty() {
				rightChunks = append(rightChunks, right)
			}
		}
		currentOffset += chunkLen
	}

	return newLeafNodeWithChunks(leftChunks), newLeafNodeWithChunks(rightChunks)
}

// splitInternal splits an internal node at the given offset.
func (n *Node) splitInternal(offset ByteOffset) (*Node, *Node) {
	var leftChildren, rightChildren []*Node
	currentOffset := ByteOffset(0)

	for i, child := range n.children {
		childLen := n.childSummaries[i].Bytes

		if currentOffset+childLen <= offset {
			// Entire child goes to left
			leftChildren = append(leftChildren, child)
		} else if currentOffset >= offset {
			// Entire child goes to right
			rightChildren = append(rightChildren, child)
		} else {
			// Need to split this child
			splitPoint := offset - currentOffset
			leftChild, rightChild := child.split(splitPoint)
			if leftChild.Len() > 0 {
				leftChildren = append(leftChildren, leftChild)
			}
			if rightChild.Len() > 0 {
				rightChildren = append(rightChildren, rightChild)
			}
		}
		currentOffset += childLen
	}

	return buildNodeFromChildren(leftChildren), buildNodeFromChildren(rightChildren)
}

// buildNodeFromChildren creates a balanced tree from a list of child nodes.
func buildNodeFromChildren(children []*Node) *Node {
	if len(children) == 0 {
		return newLeafNode()
	}
	if len(children) == 1 {
		return children[0]
	}
	if len(children) <= MaxChildren {
		return newInternalNode(children)
	}

	// Need to split into multiple levels
	var parents []*Node
	for i := 0; i < len(children); i += MaxChildren {
		end := i + MaxChildren
		if end > len(children) {
			end = len(children)
		}
		parents = append(parents, newInternalNode(children[i:end]))
	}

	return buildNodeFromChildren(parents)
}

// concat concatenates two nodes.
func concat(left, right *Node) *Node {
	if left == nil || left.Len() == 0 {
		if right == nil {
			return newLeafNode()
		}
		return right
	}
	if right == nil || right.Len() == 0 {
		return left
	}

	// If both are leaves, try to merge
	if left.IsLeaf() && right.IsLeaf() {
		return concatLeaves(left, right)
	}

	// Bring to same height by wrapping shorter one
	for left.height < right.height {
		left = newInternalNode([]*Node{left})
	}
	for right.height < left.height {
		right = newInternalNode([]*Node{right})
	}

	// Now both have same height, merge at this level
	return mergeNodes(left, right)
}

// concatLeaves concatenates two leaf nodes.
func concatLeaves(left, right *Node) *Node {
	totalChunks := len(left.chunks) + len(right.chunks)

	if totalChunks <= MaxChunksPerLeaf {
		// Can fit in one leaf
		chunks := make([]Chunk, 0, totalChunks)
		chunks = append(chunks, left.chunks...)
		chunks = append(chunks, right.chunks...)
		return newLeafNodeWithChunks(chunks)
	}

	// Need to create internal node
	return newInternalNode([]*Node{left.clone(), right.clone()})
}

// mergeNodes merges two nodes of the same height.
func mergeNodes(left, right *Node) *Node {
	if left.IsLeaf() {
		return concatLeaves(left, right)
	}

	// Combine children
	allChildren := make([]*Node, 0, len(left.children)+len(right.children))
	allChildren = append(allChildren, left.children...)
	allChildren = append(allChildren, right.children...)

	if len(allChildren) <= MaxChildren {
		return newInternalNode(allChildren)
	}

	// Need to split into multiple internal nodes
	return buildNodeFromChildren(allChildren)
}

// findChildByOffset finds the child containing the given byte offset.
// Returns the child index and the offset within that child.
func (n *Node) findChildByOffset(offset ByteOffset) (int, ByteOffset) {
	if n.IsLeaf() {
		return -1, 0
	}

	currentOffset := ByteOffset(0)
	for i, summary := range n.childSummaries {
		if currentOffset+summary.Bytes > offset {
			return i, offset - currentOffset
		}
		currentOffset += summary.Bytes
	}

	// Offset is at or past the end
	lastIdx := len(n.children) - 1
	return lastIdx, offset - (n.summary.Bytes - n.childSummaries[lastIdx].Bytes)
}

// findChildByLine finds the child containing the given line number.
// Returns the child index and the line number within that child.
func (n *Node) findChildByLine(line uint32) (int, uint32) {
	if n.IsLeaf() {
		return -1, 0
	}

	currentLine := uint32(0)
	for i, summary := range n.childSummaries {
		// Lines in a child = newlines in that child
		// Line N is in a child if currentLine <= N <= currentLine + summary.Lines
		if currentLine+summary.Lines >= line {
			return i, line - currentLine
		}
		currentLine += summary.Lines
	}

	// Line is in last child
	lastIdx := len(n.children) - 1
	lastChildStartLine := n.summary.Lines - n.childSummaries[lastIdx].Lines
	return lastIdx, line - lastChildStartLine
}
