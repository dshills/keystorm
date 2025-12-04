package history

import (
	"errors"
	"testing"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
)

// Helper to create a test buffer and cursor set
func newTestBufferAndCursors(text string, cursorPos ByteOffset) (*buffer.Buffer, *cursor.CursorSet) {
	buf := buffer.NewBufferFromString(text)
	cursors := cursor.NewCursorSetAt(cursorPos)
	return buf, cursors
}

// Operation Tests

func TestNewOperation(t *testing.T) {
	op := NewOperation(Range{Start: 5, End: 10}, "hello", "world")
	if op.Range.Start != 5 || op.Range.End != 10 {
		t.Error("wrong range")
	}
	if op.OldText != "hello" || op.NewText != "world" {
		t.Error("wrong text")
	}
	if op.Timestamp.IsZero() {
		t.Error("timestamp not set")
	}
}

func TestOperationIsInsert(t *testing.T) {
	insert := NewInsertOperation(5, "hello")
	if !insert.IsInsert() {
		t.Error("should be insert")
	}
	if insert.IsDelete() || insert.IsReplace() {
		t.Error("should not be delete or replace")
	}
}

func TestOperationIsDelete(t *testing.T) {
	del := NewDeleteOperation(Range{Start: 5, End: 10}, "hello")
	if !del.IsDelete() {
		t.Error("should be delete")
	}
	if del.IsInsert() || del.IsReplace() {
		t.Error("should not be insert or replace")
	}
}

func TestOperationIsReplace(t *testing.T) {
	replace := NewReplaceOperation(Range{Start: 5, End: 10}, "hello", "world")
	if !replace.IsReplace() {
		t.Error("should be replace")
	}
	if replace.IsInsert() || replace.IsDelete() {
		t.Error("should not be insert or delete")
	}
}

func TestOperationBytesDelta(t *testing.T) {
	tests := []struct {
		name     string
		op       *Operation
		expected int
	}{
		{"insert", NewInsertOperation(0, "hello"), 5},
		{"delete", NewDeleteOperation(Range{Start: 0, End: 5}, "hello"), -5},
		{"replace longer", NewReplaceOperation(Range{Start: 0, End: 3}, "abc", "hello"), 2},
		{"replace shorter", NewReplaceOperation(Range{Start: 0, End: 5}, "hello", "hi"), -3},
		{"replace same", NewReplaceOperation(Range{Start: 0, End: 5}, "hello", "world"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.op.BytesDelta(); got != tt.expected {
				t.Errorf("BytesDelta() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestOperationInvert(t *testing.T) {
	op := NewReplaceOperation(Range{Start: 5, End: 10}, "hello", "world")
	op.CursorsBefore = []Selection{cursor.NewCursorSelection(5)}
	op.CursorsAfter = []Selection{cursor.NewCursorSelection(10)}

	inv := op.Invert()

	if inv.Range.Start != 5 || inv.Range.End != 10 {
		t.Error("inverted range wrong")
	}
	if inv.OldText != "world" || inv.NewText != "hello" {
		t.Error("inverted text wrong")
	}
	if len(inv.CursorsBefore) != 1 || inv.CursorsBefore[0].Head != 10 {
		t.Error("inverted cursors before wrong")
	}
	if len(inv.CursorsAfter) != 1 || inv.CursorsAfter[0].Head != 5 {
		t.Error("inverted cursors after wrong")
	}
}

func TestOperationClone(t *testing.T) {
	op := NewReplaceOperation(Range{Start: 5, End: 10}, "hello", "world")
	op.CursorsBefore = []Selection{cursor.NewCursorSelection(5)}
	op.CursorsAfter = []Selection{cursor.NewCursorSelection(10)}

	clone := op.Clone()

	// Modify original
	op.Range.Start = 100
	op.CursorsBefore[0] = cursor.NewCursorSelection(100)

	// Clone should be unchanged
	if clone.Range.Start != 5 {
		t.Error("clone range was modified")
	}
	if clone.CursorsBefore[0].Head != 5 {
		t.Error("clone cursors were modified")
	}
}

// InsertCommand Tests

func TestInsertCommandExecute(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewInsertCommand(" there")

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "hello there world" {
		t.Errorf("got %q, want %q", buf.Text(), "hello there world")
	}

	// Cursor should be at end of inserted text
	if cursors.PrimaryCursor() != 11 {
		t.Errorf("cursor at %d, want 11", cursors.PrimaryCursor())
	}
}

func TestInsertCommandUndo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewInsertCommand(" there")

	cmd.Execute(buf, cursors)
	err := cmd.Undo(buf, cursors)
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	if buf.Text() != "hello world" {
		t.Errorf("got %q, want %q", buf.Text(), "hello world")
	}

	if cursors.PrimaryCursor() != 5 {
		t.Errorf("cursor at %d, want 5", cursors.PrimaryCursor())
	}
}

func TestInsertCommandWithSelection(t *testing.T) {
	buf, _ := newTestBufferAndCursors("hello world", 0)
	cursors := cursor.NewCursorSet(cursor.NewSelection(0, 5)) // Select "hello"
	cmd := NewInsertCommand("hi")

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "hi world" {
		t.Errorf("got %q, want %q", buf.Text(), "hi world")
	}

	if cursors.PrimaryCursor() != 2 {
		t.Errorf("cursor at %d, want 2", cursors.PrimaryCursor())
	}
}

func TestInsertCommandDescription(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"a", "Type 'a'"},
		{"\n", "Insert newline"},
		{"\t", "Insert tab"},
		{"hello", `Insert "hello"`},
		{"a very long string that exceeds the limit", "Insert 41 characters"},
	}

	for _, tt := range tests {
		cmd := NewInsertCommand(tt.text)
		if got := cmd.Description(); got != tt.expected {
			t.Errorf("Description for %q = %q, want %q", tt.text, got, tt.expected)
		}
	}
}

// DeleteCommand Tests

func TestDeleteCommandBackspace(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewDeleteCommand(DeleteBackward)

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "hell world" {
		t.Errorf("got %q, want %q", buf.Text(), "hell world")
	}

	if cursors.PrimaryCursor() != 4 {
		t.Errorf("cursor at %d, want 4", cursors.PrimaryCursor())
	}
}

func TestDeleteCommandForward(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewDeleteCommand(DeleteForward)

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "helloworld" {
		t.Errorf("got %q, want %q", buf.Text(), "helloworld")
	}

	if cursors.PrimaryCursor() != 5 {
		t.Errorf("cursor at %d, want 5", cursors.PrimaryCursor())
	}
}

func TestDeleteCommandWithSelection(t *testing.T) {
	buf, _ := newTestBufferAndCursors("hello world", 0)
	cursors := cursor.NewCursorSet(cursor.NewSelection(0, 5)) // Select "hello"
	cmd := NewDeleteCommand(DeleteBackward)                   // Direction doesn't matter with selection

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != " world" {
		t.Errorf("got %q, want %q", buf.Text(), " world")
	}

	if cursors.PrimaryCursor() != 0 {
		t.Errorf("cursor at %d, want 0", cursors.PrimaryCursor())
	}
}

func TestDeleteCommandUndo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewDeleteCommand(DeleteBackward)

	cmd.Execute(buf, cursors)
	err := cmd.Undo(buf, cursors)
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	if buf.Text() != "hello world" {
		t.Errorf("got %q, want %q", buf.Text(), "hello world")
	}

	if cursors.PrimaryCursor() != 5 {
		t.Errorf("cursor at %d, want 5", cursors.PrimaryCursor())
	}
}

func TestDeleteCommandN(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewDeleteCommandN(DeleteBackward, 3)

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "he world" {
		t.Errorf("got %q, want %q", buf.Text(), "he world")
	}
}

// ReplaceCommand Tests

func TestReplaceCommandExecute(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 0)
	cmd := NewReplaceCommand(Range{Start: 0, End: 5}, "hi")

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "hi world" {
		t.Errorf("got %q, want %q", buf.Text(), "hi world")
	}
}

func TestReplaceCommandUndo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 0)
	cmd := NewReplaceCommand(Range{Start: 0, End: 5}, "hi")

	cmd.Execute(buf, cursors)
	err := cmd.Undo(buf, cursors)
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	if buf.Text() != "hello world" {
		t.Errorf("got %q, want %q", buf.Text(), "hello world")
	}
}

// CompoundCommand Tests

func TestCompoundCommandExecute(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewCompoundCommand("test",
		NewInsertCommand(" there"),
		NewInsertCommand("!"),
	)

	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "hello there! world" {
		t.Errorf("got %q, want %q", buf.Text(), "hello there! world")
	}
}

func TestCompoundCommandUndo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello world", 5)
	cmd := NewCompoundCommand("test",
		NewInsertCommand(" there"),
		NewInsertCommand("!"),
	)

	cmd.Execute(buf, cursors)
	err := cmd.Undo(buf, cursors)
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	if buf.Text() != "hello world" {
		t.Errorf("got %q, want %q", buf.Text(), "hello world")
	}
}

// History Tests

func TestHistoryPushAndUndo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	cmd := NewInsertCommand(" world")
	history.Execute(cmd, buf, cursors)

	if buf.Text() != "hello world" {
		t.Errorf("after execute: got %q", buf.Text())
	}

	err := history.Undo(buf, cursors)
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	if buf.Text() != "hello" {
		t.Errorf("after undo: got %q", buf.Text())
	}
}

func TestHistoryRedo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	cmd := NewInsertCommand(" world")
	history.Execute(cmd, buf, cursors)
	history.Undo(buf, cursors)

	err := history.Redo(buf, cursors)
	if err != nil {
		t.Fatalf("Redo failed: %v", err)
	}

	if buf.Text() != "hello world" {
		t.Errorf("after redo: got %q", buf.Text())
	}
}

func TestHistoryRedoClearedOnPush(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	history.Execute(NewInsertCommand(" world"), buf, cursors)
	history.Undo(buf, cursors)

	if !history.CanRedo() {
		t.Error("should be able to redo")
	}

	// New command clears redo stack
	history.Execute(NewInsertCommand("!"), buf, cursors)

	if history.CanRedo() {
		t.Error("redo should be cleared after new command")
	}
}

func TestHistoryMaxEntries(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("", 0)
	history := NewHistory(3)

	for i := 0; i < 5; i++ {
		history.Execute(NewInsertCommand("x"), buf, cursors)
	}

	if history.UndoCount() != 3 {
		t.Errorf("undo count = %d, want 3", history.UndoCount())
	}
}

func TestHistoryCanUndoRedo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	if history.CanUndo() {
		t.Error("should not be able to undo initially")
	}
	if history.CanRedo() {
		t.Error("should not be able to redo initially")
	}

	history.Execute(NewInsertCommand(" world"), buf, cursors)

	if !history.CanUndo() {
		t.Error("should be able to undo after execute")
	}
	if history.CanRedo() {
		t.Error("should not be able to redo after execute")
	}

	history.Undo(buf, cursors)

	if history.CanUndo() {
		t.Error("should not be able to undo after undoing single command")
	}
	if !history.CanRedo() {
		t.Error("should be able to redo after undo")
	}
}

func TestHistoryErrors(t *testing.T) {
	history := NewHistory(100)
	buf, cursors := newTestBufferAndCursors("hello", 0)

	if err := history.Undo(buf, cursors); !errors.Is(err, ErrNothingToUndo) {
		t.Errorf("expected ErrNothingToUndo, got %v", err)
	}

	if err := history.Redo(buf, cursors); !errors.Is(err, ErrNothingToRedo) {
		t.Errorf("expected ErrNothingToRedo, got %v", err)
	}
}

func TestHistoryClear(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	history.Execute(NewInsertCommand(" world"), buf, cursors)
	history.Clear()

	if history.CanUndo() || history.CanRedo() {
		t.Error("history should be empty after clear")
	}
}

// Grouping Tests

func TestHistoryGrouping(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	history.BeginGroup("test group")
	history.Execute(NewInsertCommand(" "), buf, cursors)
	history.Execute(NewInsertCommand("world"), buf, cursors)
	history.EndGroup()

	if buf.Text() != "hello world" {
		t.Errorf("got %q", buf.Text())
	}

	// Single undo should revert both commands
	history.Undo(buf, cursors)

	if buf.Text() != "hello" {
		t.Errorf("after undo: got %q, want %q", buf.Text(), "hello")
	}

	if history.CanUndo() {
		t.Error("should have only one undo entry for group")
	}
}

func TestHistoryCancelGroup(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	history.BeginGroup("test group")
	history.Execute(NewInsertCommand(" world"), buf, cursors)
	history.CancelGroup()

	// Buffer is modified but no undo entry created
	if buf.Text() != "hello world" {
		t.Errorf("got %q", buf.Text())
	}

	if history.CanUndo() {
		t.Error("canceled group should not create undo entry")
	}
}

func TestHistoryGroupScope(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	func() {
		scope := history.GroupScope("test")
		defer scope.End()

		history.Execute(NewInsertCommand(" "), buf, cursors)
		history.Execute(NewInsertCommand("world"), buf, cursors)
	}()

	history.Undo(buf, cursors)

	if buf.Text() != "hello" {
		t.Errorf("after undo: got %q", buf.Text())
	}
}

func TestHistoryExecuteGrouped(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	err := history.ExecuteGrouped("test",
		buf, cursors,
		NewInsertCommand(" "),
		NewInsertCommand("world"),
	)
	if err != nil {
		t.Fatalf("ExecuteGrouped failed: %v", err)
	}

	if history.UndoCount() != 1 {
		t.Errorf("undo count = %d, want 1", history.UndoCount())
	}
}

// Info Tests

func TestHistoryUndoInfo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	history.Execute(NewInsertCommand(" world"), buf, cursors)

	info := history.UndoInfo()
	if len(info) != 1 {
		t.Fatalf("got %d entries, want 1", len(info))
	}

	if info[0].Description != `Insert " world"` {
		t.Errorf("description = %q", info[0].Description)
	}

	if info[0].Timestamp.IsZero() {
		t.Error("timestamp not set")
	}
}

func TestHistoryPeekUndo(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	_, ok := history.PeekUndo()
	if ok {
		t.Error("PeekUndo should return false when empty")
	}

	history.Execute(NewInsertCommand(" world"), buf, cursors)

	info, ok := history.PeekUndo()
	if !ok {
		t.Error("PeekUndo should return true")
	}
	if info.Description != `Insert " world"` {
		t.Errorf("description = %q", info.Description)
	}

	// Stack should be unchanged
	if history.UndoCount() != 1 {
		t.Error("PeekUndo should not modify stack")
	}
}

// Checkpoint Tests

func TestHistoryCheckpoint(t *testing.T) {
	buf, cursors := newTestBufferAndCursors("hello", 5)
	history := NewHistory(100)

	cp := history.CreateCheckpoint()

	history.Execute(NewInsertCommand(" "), buf, cursors)
	history.Execute(NewInsertCommand("world"), buf, cursors)
	history.Execute(NewInsertCommand("!"), buf, cursors)

	if buf.Text() != "hello world!" {
		t.Errorf("got %q", buf.Text())
	}

	err := history.UndoToCheckpoint(cp, buf, cursors)
	if err != nil {
		t.Fatalf("UndoToCheckpoint failed: %v", err)
	}

	if buf.Text() != "hello" {
		t.Errorf("after undo to checkpoint: got %q", buf.Text())
	}
}

// Multi-cursor Tests

func TestInsertMultiCursor(t *testing.T) {
	buf := buffer.NewBufferFromString("aa bb cc")
	// Cursors at positions after each letter pair
	cursors := cursor.NewCursorSetFromSlice([]cursor.Selection{
		cursor.NewCursorSelection(2),
		cursor.NewCursorSelection(5),
		cursor.NewCursorSelection(8),
	})

	cmd := NewInsertCommand("!")
	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "aa! bb! cc!" {
		t.Errorf("got %q, want %q", buf.Text(), "aa! bb! cc!")
	}

	// All cursors should be after their inserted text
	sels := cursors.All()
	expected := []ByteOffset{3, 7, 11}
	for i, sel := range sels {
		if sel.Head != expected[i] {
			t.Errorf("cursor %d at %d, want %d", i, sel.Head, expected[i])
		}
	}
}

func TestDeleteMultiCursor(t *testing.T) {
	buf := buffer.NewBufferFromString("aa! bb! cc!")
	cursors := cursor.NewCursorSetFromSlice([]cursor.Selection{
		cursor.NewCursorSelection(3),
		cursor.NewCursorSelection(7),
		cursor.NewCursorSelection(11),
	})

	cmd := NewDeleteCommand(DeleteBackward)
	err := cmd.Execute(buf, cursors)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if buf.Text() != "aa bb cc" {
		t.Errorf("got %q, want %q", buf.Text(), "aa bb cc")
	}
}
