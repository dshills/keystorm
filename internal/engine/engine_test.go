package engine

import (
	"strings"
	"sync"
	"testing"

	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/engine/history"
	"github.com/dshills/keystorm/internal/engine/tracking"
)

// ============================================================================
// Basic Operations
// ============================================================================

func TestNew(t *testing.T) {
	e := New()
	if e.Len() != 0 {
		t.Errorf("expected empty engine, got len %d", e.Len())
	}
	if e.Text() != "" {
		t.Errorf("expected empty text, got %q", e.Text())
	}
}

func TestNewWithContent(t *testing.T) {
	content := "Hello, World!"
	e := New(WithContent(content))

	if e.Text() != content {
		t.Errorf("expected %q, got %q", content, e.Text())
	}
	if e.Len() != ByteOffset(len(content)) {
		t.Errorf("expected len %d, got %d", len(content), e.Len())
	}
}

func TestNewFromReader(t *testing.T) {
	content := "Hello, World!"
	r := strings.NewReader(content)

	e, err := NewFromReader(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.Text() != content {
		t.Errorf("expected %q, got %q", content, e.Text())
	}
}

func TestInsert(t *testing.T) {
	e := New()

	end, err := e.Insert(0, "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if end != 5 {
		t.Errorf("expected end position 5, got %d", end)
	}
	if e.Text() != "Hello" {
		t.Errorf("expected %q, got %q", "Hello", e.Text())
	}

	end, err = e.Insert(5, ", World!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Text() != "Hello, World!" {
		t.Errorf("expected %q, got %q", "Hello, World!", e.Text())
	}
}

func TestInsertOutOfRange(t *testing.T) {
	e := New(WithContent("Hello"))

	_, err := e.Insert(100, "text")
	if err == nil {
		t.Error("expected error for out of range insert")
	}
}

func TestDelete(t *testing.T) {
	e := New(WithContent("Hello, World!"))

	err := e.Delete(5, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Text() != "HelloWorld!" {
		t.Errorf("expected %q, got %q", "HelloWorld!", e.Text())
	}
}

func TestReplace(t *testing.T) {
	e := New(WithContent("Hello, World!"))

	end, err := e.Replace(7, 12, "Go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if end != 9 {
		t.Errorf("expected end position 9, got %d", end)
	}
	if e.Text() != "Hello, Go!" {
		t.Errorf("expected %q, got %q", "Hello, Go!", e.Text())
	}
}

func TestApplyEdit(t *testing.T) {
	e := New(WithContent("Hello, World!"))

	result, err := e.ApplyEdit(Edit{
		Range:   Range{Start: 0, End: 5},
		NewText: "Hi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OldRange.Start != 0 || result.OldRange.End != 5 {
		t.Errorf("unexpected old range: %v", result.OldRange)
	}
	if result.NewRange.Start != 0 || result.NewRange.End != 2 {
		t.Errorf("unexpected new range: %v", result.NewRange)
	}
	if e.Text() != "Hi, World!" {
		t.Errorf("expected %q, got %q", "Hi, World!", e.Text())
	}
}

func TestApplyEdits(t *testing.T) {
	e := New(WithContent("foo bar baz"))

	// Edits must be in reverse order
	err := e.ApplyEdits([]Edit{
		{Range: Range{Start: 8, End: 11}, NewText: "qux"},
		{Range: Range{Start: 4, End: 7}, NewText: "XYZ"},
		{Range: Range{Start: 0, End: 3}, NewText: "ABC"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.Text() != "ABC XYZ qux" {
		t.Errorf("expected %q, got %q", "ABC XYZ qux", e.Text())
	}
}

// ============================================================================
// Read Operations
// ============================================================================

func TestLineOperations(t *testing.T) {
	e := New(WithContent("line 1\nline 2\nline 3"))

	if e.LineCount() != 3 {
		t.Errorf("expected 3 lines, got %d", e.LineCount())
	}

	if e.LineText(0) != "line 1" {
		t.Errorf("expected %q, got %q", "line 1", e.LineText(0))
	}
	if e.LineText(1) != "line 2" {
		t.Errorf("expected %q, got %q", "line 2", e.LineText(1))
	}
	if e.LineText(2) != "line 3" {
		t.Errorf("expected %q, got %q", "line 3", e.LineText(2))
	}

	if e.LineLen(0) != 6 {
		t.Errorf("expected line 0 len 6, got %d", e.LineLen(0))
	}
}

func TestTextRange(t *testing.T) {
	e := New(WithContent("Hello, World!"))

	text := e.TextRange(0, 5)
	if text != "Hello" {
		t.Errorf("expected %q, got %q", "Hello", text)
	}

	text = e.TextRange(7, 12)
	if text != "World" {
		t.Errorf("expected %q, got %q", "World", text)
	}
}

func TestByteAt(t *testing.T) {
	e := New(WithContent("Hello"))

	b, ok := e.ByteAt(0)
	if !ok || b != 'H' {
		t.Errorf("expected 'H', got %c (ok=%v)", b, ok)
	}

	_, ok = e.ByteAt(100)
	if ok {
		t.Error("expected ok=false for out of range")
	}
}

func TestRuneAt(t *testing.T) {
	e := New(WithContent("Hello"))

	r, size := e.RuneAt(0)
	if r != 'H' || size != 1 {
		t.Errorf("expected 'H' size 1, got %c size %d", r, size)
	}
}

// ============================================================================
// Position Conversion
// ============================================================================

func TestOffsetToPoint(t *testing.T) {
	e := New(WithContent("line 1\nline 2"))

	p := e.OffsetToPoint(0)
	if p.Line != 0 || p.Column != 0 {
		t.Errorf("expected (0,0), got (%d,%d)", p.Line, p.Column)
	}

	p = e.OffsetToPoint(7)
	if p.Line != 1 || p.Column != 0 {
		t.Errorf("expected (1,0), got (%d,%d)", p.Line, p.Column)
	}
}

func TestPointToOffset(t *testing.T) {
	e := New(WithContent("line 1\nline 2"))

	offset := e.PointToOffset(Point{Line: 0, Column: 0})
	if offset != 0 {
		t.Errorf("expected 0, got %d", offset)
	}

	offset = e.PointToOffset(Point{Line: 1, Column: 0})
	if offset != 7 {
		t.Errorf("expected 7, got %d", offset)
	}
}

func TestLineStartEndOffset(t *testing.T) {
	e := New(WithContent("line 1\nline 2"))

	start := e.LineStartOffset(1)
	if start != 7 {
		t.Errorf("expected 7, got %d", start)
	}

	end := e.LineEndOffset(0)
	if end != 6 {
		t.Errorf("expected 6, got %d", end)
	}
}

// ============================================================================
// Undo/Redo
// ============================================================================

func TestUndoRedo(t *testing.T) {
	e := New()

	e.Insert(0, "Hello")
	if e.Text() != "Hello" {
		t.Errorf("expected %q, got %q", "Hello", e.Text())
	}

	err := e.Undo()
	if err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if e.Text() != "" {
		t.Errorf("expected empty after undo, got %q", e.Text())
	}

	err = e.Redo()
	if err != nil {
		t.Fatalf("redo failed: %v", err)
	}
	if e.Text() != "Hello" {
		t.Errorf("expected %q after redo, got %q", "Hello", e.Text())
	}
}

func TestCanUndoRedo(t *testing.T) {
	e := New()

	if e.CanUndo() {
		t.Error("expected CanUndo=false for empty history")
	}
	if e.CanRedo() {
		t.Error("expected CanRedo=false for empty history")
	}

	e.Insert(0, "Hello")
	if !e.CanUndo() {
		t.Error("expected CanUndo=true after insert")
	}
	if e.CanRedo() {
		t.Error("expected CanRedo=false after insert")
	}

	e.Undo()
	if e.CanUndo() {
		t.Error("expected CanUndo=false after undo")
	}
	if !e.CanRedo() {
		t.Error("expected CanRedo=true after undo")
	}
}

func TestUndoGroup(t *testing.T) {
	e := New()

	e.BeginUndoGroup("format")
	e.Insert(0, "Hello")
	e.Insert(5, " World")
	e.EndUndoGroup()

	if e.Text() != "Hello World" {
		t.Errorf("expected %q, got %q", "Hello World", e.Text())
	}

	// Single undo should undo the entire group
	e.Undo()
	if e.Text() != "" {
		t.Errorf("expected empty after undo group, got %q", e.Text())
	}

	// Single redo should redo the entire group
	e.Redo()
	if e.Text() != "Hello World" {
		t.Errorf("expected %q after redo group, got %q", "Hello World", e.Text())
	}
}

func TestClearHistory(t *testing.T) {
	e := New()

	e.Insert(0, "Hello")
	e.Insert(5, " World")

	if e.UndoCount() != 2 {
		t.Errorf("expected undo count 2, got %d", e.UndoCount())
	}

	e.ClearHistory()

	if e.UndoCount() != 0 {
		t.Errorf("expected undo count 0 after clear, got %d", e.UndoCount())
	}
	if e.CanUndo() {
		t.Error("expected CanUndo=false after clear")
	}
}

// ============================================================================
// Command Execution
// ============================================================================

func TestExecuteCommand(t *testing.T) {
	e := New(WithContent("Hello World"))

	// Set cursor position for the command
	e.SetPrimaryCursor(5)

	cmd := history.NewInsertCommand(",")
	err := e.Execute(cmd)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if e.Text() != "Hello, World" {
		t.Errorf("expected %q, got %q", "Hello, World", e.Text())
	}
}

// ============================================================================
// Cursor Operations
// ============================================================================

func TestPrimaryCursor(t *testing.T) {
	e := New(WithContent("Hello"))

	if e.PrimaryCursor() != 0 {
		t.Errorf("expected cursor at 0, got %d", e.PrimaryCursor())
	}

	e.SetPrimaryCursor(5)
	if e.PrimaryCursor() != 5 {
		t.Errorf("expected cursor at 5, got %d", e.PrimaryCursor())
	}
}

func TestMultipleCursors(t *testing.T) {
	e := New(WithContent("Hello"))

	e.SetPrimaryCursor(0)
	e.AddCursor(5)

	if e.CursorCount() != 2 {
		t.Errorf("expected 2 cursors, got %d", e.CursorCount())
	}
	if !e.HasMultipleCursors() {
		t.Error("expected HasMultipleCursors=true")
	}

	e.ClearSecondary()
	if e.CursorCount() != 1 {
		t.Errorf("expected 1 cursor after clear, got %d", e.CursorCount())
	}
}

func TestCursorsClone(t *testing.T) {
	e := New(WithContent("Hello"))

	e.SetPrimaryCursor(2)
	cursors := e.Cursors()

	// Modifying the clone should not affect the engine
	cursors.Add(cursor.NewCursorSelection(4))

	if e.CursorCount() != 1 {
		t.Errorf("expected 1 cursor in engine, got %d", e.CursorCount())
	}
}

// ============================================================================
// Snapshot Operations
// ============================================================================

func TestCreateSnapshot(t *testing.T) {
	e := New(WithContent("Hello"))

	id := e.CreateSnapshot("test")

	snap, err := e.GetSnapshot(id)
	if err != nil {
		t.Fatalf("get snapshot failed: %v", err)
	}
	if snap.Text() != "Hello" {
		t.Errorf("expected snapshot text %q, got %q", "Hello", snap.Text())
	}
}

func TestGetSnapshotByName(t *testing.T) {
	e := New(WithContent("Hello"))

	e.CreateSnapshot("mysnap")

	snap, err := e.GetSnapshotByName("mysnap")
	if err != nil {
		t.Fatalf("get snapshot by name failed: %v", err)
	}
	if snap.Name != "mysnap" {
		t.Errorf("expected name %q, got %q", "mysnap", snap.Name)
	}
}

func TestSnapshotNotFound(t *testing.T) {
	e := New()

	_, err := e.GetSnapshot(999)
	if err == nil {
		t.Error("expected error for non-existent snapshot")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	e := New(WithContent("Hello"))

	id := e.CreateSnapshot("test")
	e.DeleteSnapshot(id)

	_, err := e.GetSnapshot(id)
	if err == nil {
		t.Error("expected error after deleting snapshot")
	}
}

func TestListSnapshots(t *testing.T) {
	e := New(WithContent("Hello"))

	e.CreateSnapshot("snap1")
	e.CreateSnapshot("snap2")

	list := e.ListSnapshots()
	if len(list) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(list))
	}
}

// ============================================================================
// Change Tracking
// ============================================================================

func TestRevisionID(t *testing.T) {
	e := New()
	initialRev := e.RevisionID()

	e.Insert(0, "Hello")
	afterInsertRev := e.RevisionID()

	if afterInsertRev <= initialRev {
		t.Error("expected revision to increase after insert")
	}
}

func TestChangesSince(t *testing.T) {
	e := New()
	initialRev := e.RevisionID()

	e.Insert(0, "Hello")
	e.Insert(5, " World")

	changes := e.ChangesSince(initialRev)
	if len(changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(changes))
	}
}

func TestLatestChanges(t *testing.T) {
	e := New()

	e.Insert(0, "A")
	e.Insert(1, "B")
	e.Insert(2, "C")

	changes := e.LatestChanges(2)
	if len(changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(changes))
	}
}

func TestChangeCount(t *testing.T) {
	e := New()

	e.Insert(0, "Hello")
	e.Insert(5, " World")

	if e.ChangeCount() != 2 {
		t.Errorf("expected change count 2, got %d", e.ChangeCount())
	}
}

// ============================================================================
// Diff Operations
// ============================================================================

func TestDiffSinceSnapshot(t *testing.T) {
	e := New(WithContent("Hello"))
	snapID := e.CreateSnapshot("before")

	e.Replace(0, 5, "Hi")

	changes, err := e.DiffSinceSnapshot(snapID)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if len(changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(changes))
	}
}

func TestComputeDiffSinceSnapshot(t *testing.T) {
	e := New(WithContent("line 1\nline 2"))
	snapID := e.CreateSnapshot("before")

	e.Replace(7, 13, "modified")

	diff, err := e.ComputeDiffSinceSnapshot(snapID, DiffOptions{ContextLines: 1})
	if err != nil {
		t.Fatalf("compute diff failed: %v", err)
	}
	if !diff.HasChanges() {
		t.Error("expected diff to have changes")
	}
}

// ============================================================================
// AI Context
// ============================================================================

func TestGetAIContext(t *testing.T) {
	e := New(WithContent("Hello"))
	e.CreateSnapshot("before")
	initialRev := e.RevisionID()

	e.Replace(0, 5, "Hi")

	ctx := e.GetAIContext(tracking.AIContextOptions{
		SinceRevision: initialRev,
		MaxChanges:    100,
	})

	if len(ctx.Changes) != 1 {
		t.Errorf("expected 1 change in context, got %d", len(ctx.Changes))
	}
	if ctx.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

// ============================================================================
// Configuration
// ============================================================================

func TestTabWidth(t *testing.T) {
	e := New(WithTabWidth(2))

	if e.TabWidth() != 2 {
		t.Errorf("expected tab width 2, got %d", e.TabWidth())
	}

	e.SetTabWidth(8)
	if e.TabWidth() != 8 {
		t.Errorf("expected tab width 8, got %d", e.TabWidth())
	}
}

func TestLineEnding(t *testing.T) {
	e := New(WithLineEnding(LineEndingCRLF))

	if e.LineEnding() != LineEndingCRLF {
		t.Errorf("expected CRLF, got %v", e.LineEnding())
	}

	e.SetLineEnding(LineEndingLF)
	if e.LineEnding() != LineEndingLF {
		t.Errorf("expected LF, got %v", e.LineEnding())
	}
}

func TestReadOnly(t *testing.T) {
	e := New(WithContent("Hello"), WithReadOnly())

	if !e.IsReadOnly() {
		t.Error("expected IsReadOnly=true")
	}

	_, err := e.Insert(0, "text")
	if err != ErrReadOnly {
		t.Errorf("expected ErrReadOnly, got %v", err)
	}

	err = e.Delete(0, 1)
	if err != ErrReadOnly {
		t.Errorf("expected ErrReadOnly, got %v", err)
	}

	_, err = e.Replace(0, 1, "x")
	if err != ErrReadOnly {
		t.Errorf("expected ErrReadOnly, got %v", err)
	}

	err = e.Undo()
	if err != ErrReadOnly {
		t.Errorf("expected ErrReadOnly, got %v", err)
	}
}

// ============================================================================
// Clear and Reset
// ============================================================================

func TestClear(t *testing.T) {
	e := New(WithContent("Hello"))
	e.Insert(5, " World")

	err := e.Clear()
	if err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if e.Text() != "" {
		t.Errorf("expected empty text after clear, got %q", e.Text())
	}
	if e.CanUndo() {
		t.Error("expected no undo after clear")
	}
	if e.ChangeCount() != 0 {
		t.Errorf("expected change count 0 after clear, got %d", e.ChangeCount())
	}
}

func TestSetContent(t *testing.T) {
	e := New(WithContent("Hello"))
	e.Insert(5, " World")

	err := e.SetContent("New content")
	if err != nil {
		t.Fatalf("set content failed: %v", err)
	}

	if e.Text() != "New content" {
		t.Errorf("expected %q, got %q", "New content", e.Text())
	}
	if e.CanUndo() {
		t.Error("expected no undo after set content")
	}
}

// ============================================================================
// Thread Safety
// ============================================================================

func TestConcurrentReads(t *testing.T) {
	e := New(WithContent("Hello, World!"))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.Text()
			_ = e.Len()
			_ = e.LineCount()
			_ = e.LineText(0)
			_ = e.OffsetToPoint(0)
		}()
	}
	wg.Wait()
}

func TestConcurrentReadWrite(t *testing.T) {
	e := New()

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				e.Insert(0, "x")
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = e.Text()
				_ = e.Len()
			}
		}()
	}

	wg.Wait()

	// Should have 100 "x" characters
	if e.Len() != 100 {
		t.Errorf("expected len 100, got %d", e.Len())
	}
}

// ============================================================================
// Snapshot/Buffer State
// ============================================================================

func TestSnapshot(t *testing.T) {
	e := New(WithContent("Hello"))

	snap := e.Snapshot()
	if snap.Text() != "Hello" {
		t.Errorf("expected snapshot text %q, got %q", "Hello", snap.Text())
	}

	// Modify engine
	e.Insert(5, " World")

	// Snapshot should still have old content
	if snap.Text() != "Hello" {
		t.Errorf("expected snapshot to be immutable, got %q", snap.Text())
	}
}

func TestRope(t *testing.T) {
	e := New(WithContent("Hello"))

	r := e.Rope()
	if r.String() != "Hello" {
		t.Errorf("expected rope text %q, got %q", "Hello", r.String())
	}
}
