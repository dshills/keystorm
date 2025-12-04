package tracking

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/rope"
)

// Helper to create a revision ID for testing
func testRevisionID(n uint64) RevisionID {
	return RevisionID(n)
}

// TestChangeTypes tests change type creation and properties
func TestChangeTypes(t *testing.T) {
	t.Run("insert change", func(t *testing.T) {
		c := NewInsertChange(10, "hello", testRevisionID(1))

		if c.Type != ChangeInsert {
			t.Errorf("expected ChangeInsert, got %v", c.Type)
		}
		if !c.IsInsert() {
			t.Error("IsInsert should return true")
		}
		if c.Range.Start != 10 || c.Range.End != 10 {
			t.Errorf("expected range [10:10), got %v", c.Range)
		}
		if c.NewRange.Start != 10 || c.NewRange.End != 15 {
			t.Errorf("expected new range [10:15), got %v", c.NewRange)
		}
		if c.NewText != "hello" {
			t.Errorf("expected NewText 'hello', got %q", c.NewText)
		}
		if c.OldText != "" {
			t.Errorf("expected empty OldText, got %q", c.OldText)
		}
		if c.Delta() != 5 {
			t.Errorf("expected delta 5, got %d", c.Delta())
		}
	})

	t.Run("delete change", func(t *testing.T) {
		c := NewDeleteChange(10, 15, "hello", testRevisionID(2))

		if c.Type != ChangeDelete {
			t.Errorf("expected ChangeDelete, got %v", c.Type)
		}
		if !c.IsDelete() {
			t.Error("IsDelete should return true")
		}
		if c.Range.Start != 10 || c.Range.End != 15 {
			t.Errorf("expected range [10:15), got %v", c.Range)
		}
		if c.NewRange.Start != 10 || c.NewRange.End != 10 {
			t.Errorf("expected new range [10:10), got %v", c.NewRange)
		}
		if c.OldText != "hello" {
			t.Errorf("expected OldText 'hello', got %q", c.OldText)
		}
		if c.NewText != "" {
			t.Errorf("expected empty NewText, got %q", c.NewText)
		}
		if c.Delta() != -5 {
			t.Errorf("expected delta -5, got %d", c.Delta())
		}
	})

	t.Run("replace change", func(t *testing.T) {
		c := NewReplaceChange(10, 15, "hello", "world!", testRevisionID(3))

		if c.Type != ChangeReplace {
			t.Errorf("expected ChangeReplace, got %v", c.Type)
		}
		if !c.IsReplace() {
			t.Error("IsReplace should return true")
		}
		if c.Range.Start != 10 || c.Range.End != 15 {
			t.Errorf("expected range [10:15), got %v", c.Range)
		}
		if c.NewRange.Start != 10 || c.NewRange.End != 16 {
			t.Errorf("expected new range [10:16), got %v", c.NewRange)
		}
		if c.OldText != "hello" {
			t.Errorf("expected OldText 'hello', got %q", c.OldText)
		}
		if c.NewText != "world!" {
			t.Errorf("expected NewText 'world!', got %q", c.NewText)
		}
		if c.Delta() != 1 {
			t.Errorf("expected delta 1, got %d", c.Delta())
		}
	})

	t.Run("change inversion", func(t *testing.T) {
		original := NewInsertChange(10, "hello", testRevisionID(1))
		inverted := original.Invert()

		if inverted.Type != ChangeDelete {
			t.Errorf("expected inverted type ChangeDelete, got %v", inverted.Type)
		}
		if inverted.OldText != "hello" {
			t.Errorf("expected inverted OldText 'hello', got %q", inverted.OldText)
		}
		if inverted.NewText != "" {
			t.Errorf("expected inverted NewText '', got %q", inverted.NewText)
		}
	})
}

// TestChangeSet tests change set operations
func TestChangeSet(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		cs := NewChangeSet(testRevisionID(1))

		if !cs.IsEmpty() {
			t.Error("new change set should be empty")
		}

		cs.Add(NewInsertChange(0, "hello", testRevisionID(2)))
		cs.Add(NewInsertChange(5, " world", testRevisionID(3)))

		if cs.IsEmpty() {
			t.Error("change set should not be empty")
		}
		if cs.Len() != 2 {
			t.Errorf("expected length 2, got %d", cs.Len())
		}
		if cs.TotalDelta() != 11 {
			t.Errorf("expected total delta 11, got %d", cs.TotalDelta())
		}
		if cs.StartRevision != testRevisionID(1) {
			t.Errorf("expected start revision 1, got %d", cs.StartRevision)
		}
		if cs.EndRevision != testRevisionID(3) {
			t.Errorf("expected end revision 3, got %d", cs.EndRevision)
		}
	})

	t.Run("summary", func(t *testing.T) {
		cs := NewChangeSet(testRevisionID(1))
		cs.Add(NewInsertChange(0, "hello", testRevisionID(2)))
		cs.Add(NewDeleteChange(0, 5, "world", testRevisionID(3)))

		summary := cs.Summary()
		if summary == "" {
			t.Error("summary should not be empty")
		}
	})
}

// TestRevision tests revision operations
func TestRevision(t *testing.T) {
	t.Run("create revision", func(t *testing.T) {
		rp := rope.FromString("hello world")
		rev := NewRevision(testRevisionID(1), rp)

		if rev.ID != testRevisionID(1) {
			t.Errorf("expected ID 1, got %d", rev.ID)
		}
		if rev.Timestamp.IsZero() {
			t.Error("timestamp should not be zero")
		}
		if rev.Text() != "hello world" {
			t.Errorf("expected text 'hello world', got %q", rev.Text())
		}
		if rev.Len() != 11 {
			t.Errorf("expected len 11, got %d", rev.Len())
		}
	})
}

// TestRevisionStore tests revision storage
func TestRevisionStore(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		store := newRevisionStore(10)

		rp := rope.FromString("test")
		rev := NewRevision(testRevisionID(1), rp)
		store.Add(rev)

		got, ok := store.Get(testRevisionID(1))
		if !ok {
			t.Error("revision not found")
		}
		if got.Text() != "test" {
			t.Errorf("expected text 'test', got %q", got.Text())
		}
	})

	t.Run("capacity limit", func(t *testing.T) {
		store := newRevisionStore(3)

		for i := 1; i <= 5; i++ {
			rp := rope.FromString("test")
			store.Add(NewRevision(testRevisionID(uint64(i)), rp))
		}

		// Should only have 3 most recent
		if store.Len() != 3 {
			t.Errorf("expected 3 revisions, got %d", store.Len())
		}

		// Oldest should be evicted
		if _, ok := store.Get(testRevisionID(1)); ok {
			t.Error("revision 1 should have been evicted")
		}
		if _, ok := store.Get(testRevisionID(2)); ok {
			t.Error("revision 2 should have been evicted")
		}
	})
}

// TestSnapshot tests snapshot operations
func TestSnapshot(t *testing.T) {
	t.Run("create snapshot", func(t *testing.T) {
		rp := rope.FromString("hello world")
		snap := NewSnapshot("test_snapshot", rp, testRevisionID(5))

		if snap.Name != "test_snapshot" {
			t.Errorf("expected name 'test_snapshot', got %q", snap.Name)
		}
		if snap.Revision != testRevisionID(5) {
			t.Errorf("expected revision 5, got %d", snap.Revision)
		}
		if snap.Text() != "hello world" {
			t.Errorf("expected text 'hello world', got %q", snap.Text())
		}
		if snap.LineCount() != 1 {
			t.Errorf("expected 1 line, got %d", snap.LineCount())
		}
	})
}

// TestSnapshotManager tests snapshot manager operations
func TestSnapshotManager(t *testing.T) {
	t.Run("create and get", func(t *testing.T) {
		sm := NewSnapshotManager()
		rp := rope.FromString("hello")

		id := sm.Create("test", rp, testRevisionID(1))

		snap, ok := sm.Get(id)
		if !ok {
			t.Error("snapshot not found by ID")
		}
		if snap.Text() != "hello" {
			t.Errorf("expected text 'hello', got %q", snap.Text())
		}

		snap2, ok := sm.GetByName("test")
		if !ok {
			t.Error("snapshot not found by name")
		}
		if snap2.ID != id {
			t.Error("IDs should match")
		}
	})

	t.Run("replace by name", func(t *testing.T) {
		sm := NewSnapshotManager()

		id1 := sm.Create("test", rope.FromString("first"), testRevisionID(1))
		id2 := sm.Create("test", rope.FromString("second"), testRevisionID(2))

		if sm.Count() != 1 {
			t.Errorf("expected 1 snapshot, got %d", sm.Count())
		}

		if _, ok := sm.Get(id1); ok {
			t.Error("old snapshot should be removed")
		}

		snap, ok := sm.Get(id2)
		if !ok {
			t.Error("new snapshot should exist")
		}
		if snap.Text() != "second" {
			t.Errorf("expected text 'second', got %q", snap.Text())
		}
	})

	t.Run("delete", func(t *testing.T) {
		sm := NewSnapshotManager()
		id := sm.Create("test", rope.FromString("hello"), testRevisionID(1))

		sm.Delete(id)

		if sm.Count() != 0 {
			t.Errorf("expected 0 snapshots, got %d", sm.Count())
		}
	})

	t.Run("prune by count", func(t *testing.T) {
		sm := NewSnapshotManager()

		for i := 0; i < 5; i++ {
			sm.Create("", rope.FromString("test"), testRevisionID(uint64(i)))
			time.Sleep(time.Millisecond) // Ensure different timestamps
		}

		removed := sm.PruneKeepN(2)

		if removed != 3 {
			t.Errorf("expected 3 removed, got %d", removed)
		}
		if sm.Count() != 2 {
			t.Errorf("expected 2 remaining, got %d", sm.Count())
		}
	})
}

// TestTracker tests the main tracker operations
func TestTracker(t *testing.T) {
	t.Run("record and query changes", func(t *testing.T) {
		tracker := NewTracker()
		rp := rope.FromString("hello")

		c1 := NewInsertChange(0, "hi ", testRevisionID(1))
		tracker.RecordChange(testRevisionID(1), c1, rp)

		c2 := NewInsertChange(3, "world", testRevisionID(2))
		tracker.RecordChange(testRevisionID(2), c2, rp.Insert(0, "hi "))

		changes := tracker.ChangesSince(testRevisionID(0))
		if len(changes) != 2 {
			t.Errorf("expected 2 changes, got %d", len(changes))
		}

		changes = tracker.ChangesSince(testRevisionID(1))
		if len(changes) != 1 {
			t.Errorf("expected 1 change after revision 1, got %d", len(changes))
		}
	})

	t.Run("latest changes", func(t *testing.T) {
		tracker := NewTracker()
		rp := rope.FromString("")

		for i := 1; i <= 5; i++ {
			c := NewInsertChange(0, "x", testRevisionID(uint64(i)))
			tracker.RecordChange(testRevisionID(uint64(i)), c, rp)
		}

		latest := tracker.LatestChanges(3)
		if len(latest) != 3 {
			t.Errorf("expected 3 changes, got %d", len(latest))
		}

		// Should be in chronological order
		if latest[0].RevisionID != testRevisionID(3) {
			t.Errorf("expected first change revision 3, got %d", latest[0].RevisionID)
		}
	})

	t.Run("snapshot integration", func(t *testing.T) {
		tracker := NewTracker()
		rp := rope.FromString("hello")

		// Create a snapshot
		snapID := tracker.CreateSnapshot("before_edit", rp, testRevisionID(0))

		// Make some changes
		c := NewInsertChange(5, " world", testRevisionID(1))
		tracker.RecordChange(testRevisionID(1), c, rp)

		// Query changes since snapshot
		changes, err := tracker.DiffSinceSnapshot(snapID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(changes) != 1 {
			t.Errorf("expected 1 change, got %d", len(changes))
		}
	})

	t.Run("change set building", func(t *testing.T) {
		tracker := NewTracker()
		rp := rope.FromString("")

		for i := 1; i <= 3; i++ {
			c := NewInsertChange(buffer.ByteOffset(i-1), "x", testRevisionID(uint64(i)))
			tracker.RecordChange(testRevisionID(uint64(i)), c, rp)
			rp = rp.Insert(rope.ByteOffset(i-1), "x")
		}

		cs := tracker.BuildChangeSet(testRevisionID(0))
		if cs.Len() != 3 {
			t.Errorf("expected 3 changes in set, got %d", cs.Len())
		}
		if cs.TotalDelta() != 3 {
			t.Errorf("expected total delta 3, got %d", cs.TotalDelta())
		}
	})

	t.Run("ring buffer overflow", func(t *testing.T) {
		tracker := NewTracker(WithMaxChanges(5))
		rp := rope.FromString("")

		for i := 1; i <= 10; i++ {
			c := NewInsertChange(0, "x", testRevisionID(uint64(i)))
			tracker.RecordChange(testRevisionID(uint64(i)), c, rp)
		}

		if tracker.ChangeCount() != 5 {
			t.Errorf("expected 5 changes (max), got %d", tracker.ChangeCount())
		}

		// Oldest changes should be gone
		changes := tracker.ChangesSince(testRevisionID(0))
		if len(changes) != 5 {
			t.Errorf("expected 5 changes, got %d", len(changes))
		}

		// Check that we have the most recent
		if changes[0].RevisionID != testRevisionID(6) {
			t.Errorf("expected oldest remaining revision 6, got %d", changes[0].RevisionID)
		}
	})
}

// TestLineDiff tests the Myers diff algorithm
func TestLineDiff(t *testing.T) {
	t.Run("identical content", func(t *testing.T) {
		oldRope := rope.FromString("hello\nworld")
		newRope := rope.FromString("hello\nworld")

		result := ComputeLineDiff(oldRope, newRope, DefaultDiffOptions())

		if result.HasChanges() {
			t.Error("identical content should have no changes")
		}
	})

	t.Run("simple insert", func(t *testing.T) {
		oldRope := rope.FromString("line1\nline3")
		newRope := rope.FromString("line1\nline2\nline3")

		result := ComputeLineDiff(oldRope, newRope, DefaultDiffOptions())

		if !result.HasChanges() {
			t.Error("should have changes")
		}
		if result.InsertedLines() != 1 {
			t.Errorf("expected 1 inserted line, got %d", result.InsertedLines())
		}
	})

	t.Run("simple delete", func(t *testing.T) {
		oldRope := rope.FromString("line1\nline2\nline3")
		newRope := rope.FromString("line1\nline3")

		result := ComputeLineDiff(oldRope, newRope, DefaultDiffOptions())

		if !result.HasChanges() {
			t.Error("should have changes")
		}
		if result.DeletedLines() != 1 {
			t.Errorf("expected 1 deleted line, got %d", result.DeletedLines())
		}
	})

	t.Run("string diff", func(t *testing.T) {
		result := ComputeLineDiffStrings("a\nb\nc", "a\nX\nc", DefaultDiffOptions())

		if !result.HasChanges() {
			t.Error("should have changes")
		}
	})

	t.Run("empty old", func(t *testing.T) {
		oldRope := rope.FromString("")
		newRope := rope.FromString("hello\nworld")

		result := ComputeLineDiff(oldRope, newRope, DefaultDiffOptions())

		if !result.HasChanges() {
			t.Error("should have changes")
		}
		if result.InsertedLines() != 2 {
			t.Errorf("expected 2 inserted lines, got %d", result.InsertedLines())
		}
	})

	t.Run("empty new", func(t *testing.T) {
		oldRope := rope.FromString("hello\nworld")
		newRope := rope.FromString("")

		result := ComputeLineDiff(oldRope, newRope, DefaultDiffOptions())

		if !result.HasChanges() {
			t.Error("should have changes")
		}
		if result.DeletedLines() != 2 {
			t.Errorf("expected 2 deleted lines, got %d", result.DeletedLines())
		}
	})

	t.Run("ignore case", func(t *testing.T) {
		opts := DiffOptions{IgnoreCase: true}
		result := ComputeLineDiffStrings("HELLO", "hello", opts)

		if result.HasChanges() {
			t.Error("should have no changes with case ignored")
		}
	})

	t.Run("ignore whitespace", func(t *testing.T) {
		opts := DiffOptions{IgnoreWhitespace: true}
		result := ComputeLineDiffStrings("  hello  ", "hello", opts)

		if result.HasChanges() {
			t.Error("should have no changes with whitespace ignored")
		}
	})
}

// TestUnifiedDiff tests unified diff output
func TestUnifiedDiff(t *testing.T) {
	oldRope := rope.FromString("line1\nline2\nline3")
	newRope := rope.FromString("line1\nmodified\nline3")

	result := ComputeLineDiff(oldRope, newRope, DefaultDiffOptions())
	unified := UnifiedDiff(result, "old.txt", "new.txt")

	if unified == "" {
		t.Error("unified diff should not be empty")
	}
	if len(unified) < 10 {
		t.Error("unified diff seems too short")
	}
}

// TestAIContext tests AI context generation
func TestAIContext(t *testing.T) {
	tracker := NewTracker()
	rp := rope.FromString("hello")

	// Create initial snapshot
	tracker.CreateSnapshot("before_edit", rp, testRevisionID(0))

	// Make some changes
	c := NewInsertChange(5, " world", testRevisionID(1))
	tracker.RecordChange(testRevisionID(1), c, rp)

	newRope := rp.Insert(5, " world")

	opts := AIContextOptions{
		SinceRevision:    testRevisionID(0),
		MaxChanges:       10,
		IncludeDiff:      true,
		DiffFromSnapshot: "before_edit",
		DiffOptions:      DefaultDiffOptions(),
	}

	ctx := tracker.GetAIContext(newRope, opts)

	if len(ctx.Changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(ctx.Changes))
	}
	if ctx.Summary == "" {
		t.Error("summary should not be empty")
	}
	if !ctx.HasDiff {
		t.Error("should have diff")
	}
}

// TestFromBufferEdit tests conversion from buffer edit results
func TestFromBufferEdit(t *testing.T) {
	result := buffer.EditResult{
		OldRange: buffer.Range{Start: 5, End: 10},
		NewRange: buffer.Range{Start: 5, End: 8},
		OldText:  "hello",
		Delta:    -2,
	}

	change := FromBufferEdit(result, "hi!", testRevisionID(1))

	if change.Type != ChangeReplace {
		t.Errorf("expected ChangeReplace, got %v", change.Type)
	}
	if change.OldText != "hello" {
		t.Errorf("expected OldText 'hello', got %q", change.OldText)
	}
	if change.NewText != "hi!" {
		t.Errorf("expected NewText 'hi!', got %q", change.NewText)
	}
}

// TestTrackerObserver tests the observer pattern
func TestTrackerObserver(t *testing.T) {
	tracker := NewTracker()
	observer := NewTrackerObserver(tracker)

	rp := rope.FromString("hello")
	c := NewInsertChange(5, " world", testRevisionID(1))

	observer.OnChange(testRevisionID(1), c, rp)

	if tracker.ChangeCount() != 1 {
		t.Errorf("expected 1 change, got %d", tracker.ChangeCount())
	}
}

// Benchmark tests
func BenchmarkTrackerRecordChange(b *testing.B) {
	tracker := NewTracker()
	rp := rope.FromString("hello world")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewInsertChange(0, "x", testRevisionID(uint64(i)))
		tracker.RecordChange(testRevisionID(uint64(i)), c, rp)
	}
}

func BenchmarkTrackerChangesSince(b *testing.B) {
	tracker := NewTracker()
	rp := rope.FromString("hello world")

	// Fill with changes
	for i := 0; i < 1000; i++ {
		c := NewInsertChange(0, "x", testRevisionID(uint64(i)))
		tracker.RecordChange(testRevisionID(uint64(i)), c, rp)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.ChangesSince(testRevisionID(500))
	}
}

func BenchmarkLineDiffSmall(b *testing.B) {
	oldRope := rope.FromString("line1\nline2\nline3\nline4\nline5")
	newRope := rope.FromString("line1\nmodified\nline3\nline4\nline5")
	opts := DefaultDiffOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeLineDiff(oldRope, newRope, opts)
	}
}

func BenchmarkLineDiffLarge(b *testing.B) {
	// Generate 1000 line files
	var oldLines, newLines string
	for i := 0; i < 1000; i++ {
		oldLines += "line content here\n"
		if i == 500 {
			newLines += "modified line\n"
		} else {
			newLines += "line content here\n"
		}
	}

	oldRope := rope.FromString(oldLines)
	newRope := rope.FromString(newLines)
	opts := DefaultDiffOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeLineDiff(oldRope, newRope, opts)
	}
}

func BenchmarkSnapshotCreate(b *testing.B) {
	sm := NewSnapshotManager()
	rp := rope.FromString("hello world")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Create("", rp, testRevisionID(uint64(i)))
	}
}
