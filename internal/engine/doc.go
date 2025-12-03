// Package engine provides the core text editor engine for Keystorm.
//
// The engine package serves as the main facade, combining buffer management,
// cursor handling, undo/redo operations, and change tracking into a unified,
// thread-safe API suitable for building text editors with AI integration.
//
// # Architecture
//
// The engine is built on several sub-packages:
//
//   - rope: B+ tree rope for efficient text storage (O(log n) operations)
//   - buffer: Buffer abstraction with position conversion and edit operations
//   - cursor: Multi-cursor and selection management
//   - history: Command-based undo/redo system
//   - tracking: Change tracking and snapshots for AI context
//
// # Thread Safety
//
// All Engine operations are thread-safe. The engine uses a read-write mutex
// to allow concurrent reads while serializing writes. Multiple goroutines
// can safely call read operations like Text(), LineText(), or OffsetToPoint()
// simultaneously.
//
// # Basic Usage
//
// Create an engine and perform basic edits:
//
//	// Create a new engine
//	e := engine.New()
//
//	// Insert text
//	e.Insert(0, "Hello, World!")
//
//	// Read content
//	text := e.Text() // "Hello, World!"
//
//	// Replace text
//	e.Replace(7, 12, "Go") // "Hello, Go!"
//
//	// Undo the replacement
//	e.Undo() // "Hello, World!"
//
// # Loading Files
//
// Create an engine from existing content:
//
//	// From a string
//	e := engine.New(engine.WithContent("initial content"))
//
//	// From a reader (file, network, etc.)
//	f, _ := os.Open("file.txt")
//	defer f.Close()
//	e, _ := engine.NewFromReader(f)
//
// # Multi-Cursor Support
//
// The engine supports multiple cursors for simultaneous edits:
//
//	e := engine.New(engine.WithContent("foo bar foo"))
//
//	// Add multiple cursors
//	e.SetPrimaryCursor(0)
//	e.AddCursor(8)
//
//	// Execute a command that affects all cursors
//	cmd := history.NewInsertCommand("X")
//	e.Execute(cmd)
//
//	// Result: "Xfoo bar Xfoo"
//
// # Undo/Redo
//
// The engine maintains full undo/redo history:
//
//	e := engine.New()
//	e.Insert(0, "Hello")
//	e.Insert(5, " World")
//
//	e.Undo() // Removes " World"
//	e.Undo() // Removes "Hello"
//	e.Redo() // Restores "Hello"
//
// Group multiple operations into a single undo unit:
//
//	e.BeginUndoGroup("format code")
//	e.Replace(0, 5, "fn")
//	e.Insert(2, " main()")
//	e.EndUndoGroup()
//
//	e.Undo() // Undoes both operations at once
//
// # Change Tracking for AI Context
//
// The engine tracks changes for AI context generation:
//
//	e := engine.New(engine.WithContent("original"))
//
//	// Create a snapshot before AI interaction
//	snapID := e.CreateSnapshot("before_ai_edit")
//
//	// ... user makes edits ...
//	e.Replace(0, 8, "modified")
//
//	// Get changes since snapshot
//	changes, _ := e.DiffSinceSnapshot(snapID)
//
//	// Or get a line-level diff
//	diff, _ := e.ComputeDiffSinceSnapshot(snapID, engine.DiffOptions{
//	    ContextLines: 3,
//	})
//
//	// Get AI-friendly context
//	ctx := e.GetAIContext(tracking.AIContextOptions{
//	    SinceRevision:    0,
//	    MaxChanges:       100,
//	    IncludeDiff:      true,
//	    DiffFromSnapshot: "before_ai_edit",
//	})
//
// # Configuration
//
// Configure the engine at creation time:
//
//	e := engine.New(
//	    engine.WithContent("initial"),
//	    engine.WithTabWidth(4),
//	    engine.WithLineEnding(engine.LineEndingLF),
//	    engine.WithMaxUndoEntries(1000),
//	    engine.WithMaxChanges(10000),
//	)
//
// Or modify configuration at runtime:
//
//	e.SetTabWidth(2)
//	e.SetLineEnding(engine.LineEndingCRLF)
//
// # Read-Only Mode
//
// Create a read-only engine that rejects write operations:
//
//	e := engine.New(
//	    engine.WithContent("read-only content"),
//	    engine.WithReadOnly(),
//	)
//
//	_, err := e.Insert(0, "text")
//	// err == engine.ErrReadOnly
//
// # Position Conversion
//
// Convert between different position representations:
//
//	e := engine.New(engine.WithContent("line 1\nline 2"))
//
//	// Byte offset to line/column
//	point := e.OffsetToPoint(7) // Point{Line: 1, Column: 0}
//
//	// Line/column to byte offset
//	offset := e.PointToOffset(engine.Point{Line: 1, Column: 0}) // 7
//
//	// UTF-16 positions (for LSP compatibility)
//	utf16Point := e.OffsetToPointUTF16(offset)
//	offset = e.PointUTF16ToOffset(utf16Point)
//
// # Snapshots
//
// Snapshots provide efficient read-only views of buffer state:
//
//	e := engine.New(engine.WithContent("original"))
//
//	// Buffer snapshot (lightweight, uses structural sharing)
//	snap := e.Snapshot()
//	text := snap.Text()
//
//	// Named snapshots for long-term storage
//	id := e.CreateSnapshot("checkpoint")
//	// ... edits ...
//	snapText, _ := e.GetSnapshotText(id)
//
// # Error Handling
//
// The package defines several error types:
//
//   - ErrOffsetOutOfRange: Invalid byte offset
//   - ErrRangeInvalid: Invalid range (e.g., end < start)
//   - ErrEditsOverlap: Batch edits overlap or are not in reverse order
//   - ErrNothingToUndo: Undo stack is empty
//   - ErrNothingToRedo: Redo stack is empty
//   - ErrSnapshotNotFound: Requested snapshot does not exist
//   - ErrReadOnly: Write operation on read-only engine
package engine
