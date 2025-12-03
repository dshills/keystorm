// Package buffer provides a thread-safe text buffer built on top of the rope
// data structure. It serves as the primary interface for text manipulation
// in the editor engine.
//
// The buffer package provides:
//
//   - Thread-safe read/write access via sync.RWMutex
//   - Efficient text operations through the underlying rope
//   - Coordinate conversion between byte offsets and line/column positions
//   - UTF-16 coordinate support for LSP compatibility
//   - Read-only snapshots for concurrent access
//   - Line ending normalization
//   - Revision tracking for change management
//
// Basic usage:
//
//	// Create a buffer with some text
//	buf := buffer.NewBufferFromString("Hello, World!")
//
//	// Insert text
//	buf.Insert(7, "Beautiful ")  // "Hello, Beautiful World!"
//
//	// Delete text
//	buf.Delete(0, 7)  // "Beautiful World!"
//
//	// Get a snapshot for concurrent reading
//	snap := buf.Snapshot()
//	go func() {
//	    text := snap.Text()
//	    // Process text...
//	}()
//
// Position Types:
//
// The package provides several position types to handle different coordinate
// systems:
//
//   - ByteOffset: Raw byte position in the buffer
//   - Point: Line and column position (0-indexed, column in bytes)
//   - PointUTF16: Line and column position with UTF-16 code unit column
//     (for LSP compatibility)
//
// Thread Safety:
//
// All Buffer methods are thread-safe. Read operations acquire a read lock,
// while write operations acquire an exclusive write lock. For scenarios
// requiring multiple reads without the possibility of intervening writes,
// use Snapshot() to obtain a consistent read-only view.
package buffer
