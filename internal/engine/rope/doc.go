// Package rope provides an immutable rope data structure for efficient text storage and manipulation.
//
// A rope is a binary tree where leaf nodes contain text chunks and internal nodes
// store aggregated metrics (byte count, line count, etc.). This implementation uses
// a B+ tree variant for better cache locality and worst-case performance.
//
// # Key Features
//
//   - O(log n) insertion, deletion, and access operations
//   - Immutable operations return new ropes; originals are never modified
//   - Efficient line/column indexing via aggregated metrics
//   - Copy-on-write semantics enable cheap snapshots
//   - Thread-safe for concurrent read access
//   - Memory pooling for reduced GC pressure
//   - Newline index caching for fast line navigation
//
// # Basic Usage
//
// Create and modify ropes:
//
//	// Create from string
//	r := rope.FromString("hello world")
//
//	// Insert text
//	r = r.Insert(5, ",")  // "hello, world"
//
//	// Delete text
//	r = r.Delete(0, 6)    // "world"
//
//	// Replace text
//	r = r.Replace(0, 5, "universe")  // "universe"
//
//	// Get content
//	text := r.String()  // full text
//	slice := r.Slice(0, 4)  // "univ"
//
// # Immutability
//
// All operations return new ropes without modifying the original:
//
//	original := rope.FromString("hello")
//	modified := original.Insert(5, " world")
//
//	fmt.Println(original.String())  // "hello" (unchanged)
//	fmt.Println(modified.String())  // "hello world"
//
// This enables cheap snapshots and safe concurrent access.
//
// # Line Operations
//
// The rope efficiently tracks line information:
//
//	r := rope.FromString("line 1\nline 2\nline 3")
//
//	// Get line count
//	count := r.LineCount()  // 3
//
//	// Get line text
//	text := r.LineText(1)  // "line 2"
//
//	// Get line offsets
//	start := r.LineStartOffset(1)  // 7
//	end := r.LineEndOffset(1)      // 13
//
// # Position Conversion
//
// Convert between byte offsets and line/column positions:
//
//	r := rope.FromString("hello\nworld")
//
//	// Offset to point
//	point := r.OffsetToPoint(6)  // Point{Line: 1, Column: 0}
//
//	// Point to offset
//	offset := r.PointToOffset(rope.Point{Line: 1, Column: 0})  // 6
//
// # Cursor Navigation
//
// Use cursors for efficient sequential access:
//
//	r := rope.FromString("hello world")
//	cursor := rope.NewCursor(r)
//
//	// Iterate over runes
//	for cursor.Next() {
//	    r, _ := cursor.Rune()
//	    fmt.Printf("%c", r)
//	}
//
//	// Seek to specific positions
//	cursor.SeekOffset(5)   // seek to byte offset
//	cursor.SeekLine(0)     // seek to line start
//
// # Building Ropes Efficiently
//
// For building ropes incrementally, use the Builder:
//
//	builder := rope.NewBuilder()
//	builder.WriteString("hello ")
//	builder.WriteString("world")
//	r := builder.Build()  // "hello world"
//
// The builder is more efficient than repeated Insert calls when
// building text from many small pieces.
//
// # Iteration
//
// Several iterators are available for traversing rope content:
//
//	r := rope.FromString("hello\nworld")
//
//	// Iterate over lines
//	lines := r.Lines()
//	for lines.Next() {
//	    fmt.Println(lines.Text())
//	}
//
//	// Iterate over chunks (raw storage units)
//	chunks := r.Chunks()
//	for chunks.Next() {
//	    fmt.Print(chunks.Chunk().String())
//	}
//
//	// Iterate over runes
//	runes := r.Runes()
//	for runes.Next() {
//	    fmt.Printf("%c", runes.Rune())
//	}
//
// # Performance Characteristics
//
// Operation complexities for a rope of n bytes with l lines:
//
//   - FromString:      O(n)
//   - Insert:          O(log n)
//   - Delete:          O(log n)
//   - Replace:         O(log n)
//   - Slice:           O(log n + k) where k is the slice length
//   - LineText:        O(log l + k) where k is the line length
//   - OffsetToPoint:   O(log n)
//   - PointToOffset:   O(log l)
//   - String:          O(n)
//   - Len:             O(1)
//   - LineCount:       O(1)
//
// # Memory Efficiency
//
// The rope uses structural sharing, so operations like Insert create
// new nodes only along the path from root to the modification point.
// Unchanged subtrees are shared between the old and new rope.
//
// The package includes a node pool to reduce GC pressure during
// high-frequency editing operations.
//
// # Thread Safety
//
// Ropes are safe for concurrent read access. The immutable design
// means multiple goroutines can safely read from the same rope
// without synchronization. For concurrent writes, external
// synchronization is required.
package rope
