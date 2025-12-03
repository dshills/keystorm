// Package rope provides an immutable rope data structure for efficient text storage and manipulation.
//
// A rope is a binary tree where leaf nodes contain text chunks and internal nodes
// store aggregated metrics (byte count, line count, etc.). This implementation uses
// a B+ tree variant for better cache locality and worst-case performance.
//
// Key features:
//   - O(log n) insertion, deletion, and access operations
//   - Immutable operations return new ropes; originals are never modified
//   - Efficient line/column indexing via aggregated metrics
//   - Copy-on-write semantics enable cheap snapshots
//   - Thread-safe for concurrent read access
//
// Basic usage:
//
//	r := rope.FromString("hello world")
//	r = r.Insert(5, ",")           // "hello, world"
//	r = r.Delete(0, 6)             // "world"
//	text := r.String()             // "world"
//
// The rope is designed to handle large files efficiently while maintaining
// fast random access and modification times.
package rope
