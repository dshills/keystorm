package rope

import "sync"

// NodePool provides efficient allocation and recycling of rope nodes.
// It uses sync.Pool for thread-safe pooling with per-P caches.
//
// Usage note: The pool is optional and used internally by rope operations.
// It's primarily beneficial for:
// - High-frequency edit operations
// - Building ropes from many small pieces
// - Reducing GC pressure in interactive editors
type NodePool struct {
	leafPool     sync.Pool
	internalPool sync.Pool
}

// DefaultPool is the global node pool used by rope operations.
// It can be replaced with a custom pool if needed.
var DefaultPool = NewNodePool()

// NewNodePool creates a new node pool.
func NewNodePool() *NodePool {
	return &NodePool{
		leafPool: sync.Pool{
			New: func() interface{} {
				return &Node{
					height: 0,
					chunks: make([]Chunk, 0, MaxChunksPerLeaf),
				}
			},
		},
		internalPool: sync.Pool{
			New: func() interface{} {
				return &Node{
					height:         1,
					children:       make([]*Node, 0, MaxChildren),
					childSummaries: make([]TextSummary, 0, MaxChildren),
				}
			},
		},
	}
}

// GetLeaf retrieves a leaf node from the pool.
// The node is reset to empty state.
func (p *NodePool) GetLeaf() *Node {
	n := p.leafPool.Get().(*Node)
	n.height = 0
	n.summary = TextSummary{}
	n.chunks = n.chunks[:0]
	n.children = nil
	n.childSummaries = nil
	return n
}

// GetInternal retrieves an internal node from the pool.
// The node is reset to empty state.
func (p *NodePool) GetInternal(height uint8) *Node {
	n := p.internalPool.Get().(*Node)
	n.height = height
	n.summary = TextSummary{}
	n.chunks = nil
	n.children = n.children[:0]
	n.childSummaries = n.childSummaries[:0]
	return n
}

// PutLeaf returns a leaf node to the pool for reuse.
// The node should not be used after calling this method.
func (p *NodePool) PutLeaf(n *Node) {
	if n == nil || !n.IsLeaf() {
		return
	}
	// Clear references to allow GC of chunk data
	for i := range n.chunks {
		n.chunks[i] = Chunk{}
	}
	n.chunks = n.chunks[:0]
	p.leafPool.Put(n)
}

// PutInternal returns an internal node to the pool for reuse.
// The node should not be used after calling this method.
func (p *NodePool) PutInternal(n *Node) {
	if n == nil || n.IsLeaf() {
		return
	}
	// Clear references to allow GC of children
	for i := range n.children {
		n.children[i] = nil
	}
	n.children = n.children[:0]
	n.childSummaries = n.childSummaries[:0]
	p.internalPool.Put(n)
}

// Put returns a node to the appropriate pool based on its type.
func (p *NodePool) Put(n *Node) {
	if n == nil {
		return
	}
	if n.IsLeaf() {
		p.PutLeaf(n)
	} else {
		p.PutInternal(n)
	}
}

// ChunkSlicePool provides efficient allocation of chunk slices.
var ChunkSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]Chunk, 0, MaxChunksPerLeaf*2)
		return &s
	},
}

// GetChunkSlice retrieves a chunk slice from the pool.
func GetChunkSlice() *[]Chunk {
	s := ChunkSlicePool.Get().(*[]Chunk)
	*s = (*s)[:0]
	return s
}

// PutChunkSlice returns a chunk slice to the pool.
func PutChunkSlice(s *[]Chunk) {
	if s == nil {
		return
	}
	// Clear references
	for i := range *s {
		(*s)[i] = Chunk{}
	}
	*s = (*s)[:0]
	ChunkSlicePool.Put(s)
}

// NodeSlicePool provides efficient allocation of node pointer slices.
var NodeSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]*Node, 0, MaxChildren*2)
		return &s
	},
}

// GetNodeSlice retrieves a node slice from the pool.
func GetNodeSlice() *[]*Node {
	s := NodeSlicePool.Get().(*[]*Node)
	*s = (*s)[:0]
	return s
}

// PutNodeSlice returns a node slice to the pool.
func PutNodeSlice(s *[]*Node) {
	if s == nil {
		return
	}
	// Clear references
	for i := range *s {
		(*s)[i] = nil
	}
	*s = (*s)[:0]
	NodeSlicePool.Put(s)
}

// StringBuilderPool provides efficient allocation of string builders.
var StringBuilderPool = sync.Pool{
	New: func() interface{} {
		return new(stringBuilderWrapper)
	},
}

// stringBuilderWrapper wraps strings.Builder for pooling.
// We use a wrapper because strings.Builder has specific reset requirements.
type stringBuilderWrapper struct {
	buf []byte
}

// GetStringBuilder retrieves a string builder from the pool.
// Returns a slice that can be appended to.
func GetStringBuilder(capacity int) *stringBuilderWrapper {
	w := StringBuilderPool.Get().(*stringBuilderWrapper)
	if cap(w.buf) < capacity {
		w.buf = make([]byte, 0, capacity)
	} else {
		w.buf = w.buf[:0]
	}
	return w
}

// PutStringBuilder returns a string builder to the pool.
func PutStringBuilder(w *stringBuilderWrapper) {
	if w == nil {
		return
	}
	// Only keep reasonably sized buffers
	if cap(w.buf) <= 64*1024 {
		w.buf = w.buf[:0]
		StringBuilderPool.Put(w)
	}
}

// Write appends bytes to the builder.
func (w *stringBuilderWrapper) Write(p []byte) {
	w.buf = append(w.buf, p...)
}

// WriteString appends a string to the builder.
func (w *stringBuilderWrapper) WriteString(s string) {
	w.buf = append(w.buf, s...)
}

// String returns the accumulated string.
func (w *stringBuilderWrapper) String() string {
	return string(w.buf)
}

// Len returns the current length.
func (w *stringBuilderWrapper) Len() int {
	return len(w.buf)
}

// Reset clears the builder.
func (w *stringBuilderWrapper) Reset() {
	w.buf = w.buf[:0]
}
