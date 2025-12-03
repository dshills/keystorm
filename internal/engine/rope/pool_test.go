package rope

import (
	"sync"
	"testing"
)

func TestNodePoolGetLeaf(t *testing.T) {
	pool := NewNodePool()

	n := pool.GetLeaf()
	if n == nil {
		t.Fatal("expected non-nil node")
	}
	if !n.IsLeaf() {
		t.Error("expected leaf node")
	}
	if len(n.chunks) != 0 {
		t.Errorf("expected empty chunks, got %d", len(n.chunks))
	}
}

func TestNodePoolGetInternal(t *testing.T) {
	pool := NewNodePool()

	n := pool.GetInternal(2)
	if n == nil {
		t.Fatal("expected non-nil node")
	}
	if n.IsLeaf() {
		t.Error("expected internal node")
	}
	if n.height != 2 {
		t.Errorf("expected height 2, got %d", n.height)
	}
	if len(n.children) != 0 {
		t.Errorf("expected empty children, got %d", len(n.children))
	}
}

func TestNodePoolPutAndGet(t *testing.T) {
	pool := NewNodePool()

	// Get a leaf, modify it, put it back
	n1 := pool.GetLeaf()
	n1.chunks = append(n1.chunks, NewChunk("test"))
	n1.recomputeSummary()
	pool.PutLeaf(n1)

	// Get another leaf - might be the same one (reused)
	n2 := pool.GetLeaf()
	if len(n2.chunks) != 0 {
		t.Errorf("expected empty chunks after pool reuse, got %d", len(n2.chunks))
	}
}

func TestNodePoolConcurrent(t *testing.T) {
	pool := NewNodePool()

	var wg sync.WaitGroup
	iterations := 1000

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Get, use, put leaf
				leaf := pool.GetLeaf()
				leaf.chunks = append(leaf.chunks, NewChunk("test"))
				pool.PutLeaf(leaf)

				// Get, use, put internal
				internal := pool.GetInternal(1)
				internal.children = append(internal.children, pool.GetLeaf())
				pool.PutInternal(internal)
			}
		}()
	}

	wg.Wait()
}

func TestChunkSlicePool(t *testing.T) {
	s := GetChunkSlice()
	if s == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(*s) != 0 {
		t.Errorf("expected empty slice, got len %d", len(*s))
	}

	*s = append(*s, NewChunk("test1"), NewChunk("test2"))
	PutChunkSlice(s)

	// Get another one
	s2 := GetChunkSlice()
	if len(*s2) != 0 {
		t.Errorf("expected empty slice after pool reuse, got %d", len(*s2))
	}
	PutChunkSlice(s2)
}

func TestNodeSlicePool(t *testing.T) {
	s := GetNodeSlice()
	if s == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(*s) != 0 {
		t.Errorf("expected empty slice, got len %d", len(*s))
	}

	*s = append(*s, newLeafNode(), newLeafNode())
	PutNodeSlice(s)

	// Get another one
	s2 := GetNodeSlice()
	if len(*s2) != 0 {
		t.Errorf("expected empty slice after pool reuse, got %d", len(*s2))
	}
	PutNodeSlice(s2)
}

func TestStringBuilderPool(t *testing.T) {
	w := GetStringBuilder(100)
	if w == nil {
		t.Fatal("expected non-nil wrapper")
	}
	if w.Len() != 0 {
		t.Errorf("expected empty builder, got len %d", w.Len())
	}

	w.WriteString("hello ")
	w.WriteString("world")
	if w.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", w.String())
	}

	PutStringBuilder(w)

	// Get another one
	w2 := GetStringBuilder(50)
	if w2.Len() != 0 {
		t.Errorf("expected empty builder after pool reuse, got %d", w2.Len())
	}
	PutStringBuilder(w2)
}

func BenchmarkPooledNodeAllocation(b *testing.B) {
	pool := NewNodePool()

	b.Run("leaf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			n := pool.GetLeaf()
			pool.PutLeaf(n)
		}
	})

	b.Run("internal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			n := pool.GetInternal(1)
			pool.PutInternal(n)
		}
	})
}

func BenchmarkUnpooledNodeAllocation(b *testing.B) {
	b.Run("leaf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = &Node{
				height: 0,
				chunks: make([]Chunk, 0, MaxChunksPerLeaf),
			}
		}
	})

	b.Run("internal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = &Node{
				height:         1,
				children:       make([]*Node, 0, MaxChildren),
				childSummaries: make([]TextSummary, 0, MaxChildren),
			}
		}
	})
}
