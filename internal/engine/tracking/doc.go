// Package tracking provides change tracking and snapshot management for AI context.
//
// This package enables efficient tracking of buffer changes over time, supporting:
//   - Revision-based change queries ("what changed since revision X?")
//   - Named snapshots for checkpointing buffer state
//   - Line-level diff computation using Myers algorithm
//   - Efficient storage through structural sharing of immutable ropes
//
// # Core Components
//
// The package consists of several interconnected types:
//
//   - [Change]: Represents a single change (insert, delete, or replace)
//   - [Snapshot]: A named checkpoint of buffer state
//   - [Tracker]: Main type that orchestrates change recording and queries
//   - [LineDiff]: Line-based diff result for AI context generation
//
// # Usage
//
// Create a tracker and record changes as they happen:
//
//	tracker := tracking.NewTracker()
//
//	// Record a change
//	change := tracking.Change{
//	    Type:     tracking.ChangeInsert,
//	    Range:    buffer.Range{Start: 0, End: 0},
//	    NewRange: buffer.Range{Start: 0, End: 5},
//	    NewText:  "hello",
//	}
//	tracker.RecordChange(revisionID, change, ropeSnapshot)
//
//	// Query changes since a revision
//	changes := tracker.ChangesSince(oldRevisionID)
//
// # Snapshots
//
// Create named snapshots for important checkpoints:
//
//	// Create a snapshot before AI operation
//	snapID := tracker.CreateSnapshot("before_ai_edit", rope, revisionID)
//
//	// Later, get changes since that snapshot
//	changes, err := tracker.DiffSinceSnapshot(snapID, currentRope)
//
// # Diffing
//
// Compute line-level diffs for AI context:
//
//	diffs := tracking.ComputeLineDiff(oldRope, newRope, tracking.DiffOptions{
//	    ContextLines: 3,
//	})
//
// # Thread Safety
//
// All Tracker operations are thread-safe through internal locking.
// Snapshots are immutable and can be freely shared across goroutines.
//
// # Performance
//
// The tracking system is designed for efficiency:
//   - Snapshots are O(1) due to rope structural sharing
//   - Change history is bounded by a configurable maximum
//   - Ring buffer storage minimizes allocation overhead
package tracking
