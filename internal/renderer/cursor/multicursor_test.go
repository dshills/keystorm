package cursor

import (
	"testing"
)

func TestMultiCursorManagerNew(t *testing.T) {
	m := NewMultiCursorManager()

	if m == nil {
		t.Fatal("NewMultiCursorManager() returned nil")
	}
	if m.CursorCount() != 0 {
		t.Errorf("New manager should have 0 cursors, got %d", m.CursorCount())
	}
	if m.HasMultiple() {
		t.Error("New manager should not have multiple cursors")
	}
}

func TestMultiCursorManagerSetPrimary(t *testing.T) {
	m := NewMultiCursorManager()

	m.SetPrimary(5, 10)

	count := m.CursorCount()
	if count != 1 {
		t.Fatalf("SetPrimary should result in 1 cursor, got %d", count)
	}

	primary, ok := m.Primary()
	if !ok {
		t.Fatal("Primary() should return true after SetPrimary")
	}
	if primary.Position.Line != 5 || primary.Position.Column != 10 {
		t.Errorf("Primary position = (%d, %d), want (5, 10)", primary.Position.Line, primary.Position.Column)
	}
	if !primary.IsPrimary {
		t.Error("Primary cursor should have IsPrimary = true")
	}
}

func TestMultiCursorManagerSetPrimaryClearsSecondary(t *testing.T) {
	m := NewMultiCursorManager()

	m.SetPrimary(0, 0)
	m.AddCursor(1, 0)
	m.AddCursor(2, 0)

	if m.CursorCount() != 3 {
		t.Fatalf("Should have 3 cursors, got %d", m.CursorCount())
	}

	m.SetPrimary(5, 5)

	if m.CursorCount() != 1 {
		t.Errorf("SetPrimary should clear secondary cursors, got %d", m.CursorCount())
	}
}

func TestMultiCursorManagerAddCursor(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)

	idx := m.AddCursor(5, 10)
	if idx != 1 {
		t.Errorf("AddCursor returned index %d, want 1", idx)
	}

	count := m.CursorCount()
	if count != 2 {
		t.Errorf("CursorCount() = %d, want 2", count)
	}

	if !m.HasMultiple() {
		t.Error("HasMultiple() should return true with 2 cursors")
	}

	cursors := m.Cursors()
	if cursors[1].Position.Line != 5 || cursors[1].Position.Column != 10 {
		t.Errorf("Added cursor at wrong position")
	}
	if cursors[1].IsPrimary {
		t.Error("Added cursor should not be primary")
	}
}

func TestMultiCursorManagerAddCursorDuplicate(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 10)

	// Try to add at same position
	idx := m.AddCursor(5, 10)
	if idx != -1 {
		t.Errorf("AddCursor should return -1 for duplicate, got %d", idx)
	}
	if m.CursorCount() != 1 {
		t.Errorf("Duplicate cursor should not be added")
	}
}

func TestMultiCursorManagerAddCursorAbove(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 10)

	ok := m.AddCursorAbove()
	if !ok {
		t.Error("AddCursorAbove should return true")
	}
	if m.CursorCount() != 2 {
		t.Errorf("CursorCount() = %d, want 2", m.CursorCount())
	}

	cursors := m.Cursors()
	// Find the added cursor
	var found bool
	for _, c := range cursors {
		if c.Position.Line == 4 && c.Position.Column == 10 {
			found = true
			break
		}
	}
	if !found {
		t.Error("AddCursorAbove should add cursor at line 4, col 10")
	}
}

func TestMultiCursorManagerAddCursorAboveAtTop(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 10)

	ok := m.AddCursorAbove()
	if ok {
		t.Error("AddCursorAbove should return false at line 0")
	}
}

func TestMultiCursorManagerAddCursorAboveEmpty(t *testing.T) {
	m := NewMultiCursorManager()

	ok := m.AddCursorAbove()
	if ok {
		t.Error("AddCursorAbove should return false with no cursors")
	}
}

func TestMultiCursorManagerAddCursorBelow(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 10)

	ok := m.AddCursorBelow()
	if !ok {
		t.Error("AddCursorBelow should return true")
	}

	cursors := m.Cursors()
	var found bool
	for _, c := range cursors {
		if c.Position.Line == 6 && c.Position.Column == 10 {
			found = true
			break
		}
	}
	if !found {
		t.Error("AddCursorBelow should add cursor at line 6, col 10")
	}
}

func TestMultiCursorManagerRemoveCursor(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)
	m.AddCursor(1, 0)
	m.AddCursor(2, 0)

	ok := m.RemoveCursor(1)
	if !ok {
		t.Error("RemoveCursor should return true")
	}
	if m.CursorCount() != 2 {
		t.Errorf("CursorCount() = %d, want 2", m.CursorCount())
	}
}

func TestMultiCursorManagerRemoveCursorInvalidIndex(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)

	ok := m.RemoveCursor(-1)
	if ok {
		t.Error("RemoveCursor(-1) should return false")
	}

	ok = m.RemoveCursor(10)
	if ok {
		t.Error("RemoveCursor(10) should return false")
	}
}

func TestMultiCursorManagerRemoveLastCursor(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)

	ok := m.RemoveCursor(0)
	if ok {
		t.Error("Should not remove the last cursor")
	}
	if m.CursorCount() != 1 {
		t.Error("Last cursor should remain")
	}
}

func TestMultiCursorManagerRemovePrimaryCursor(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)
	m.AddCursor(1, 0)
	m.AddCursor(2, 0)

	// Primary is at index 0
	ok := m.RemoveCursor(0)
	if !ok {
		t.Error("RemoveCursor should return true")
	}

	// Another cursor should become primary
	primary, ok := m.Primary()
	if !ok {
		t.Fatal("Should have a primary cursor after removal")
	}
	if !primary.IsPrimary {
		t.Error("New primary should have IsPrimary = true")
	}
}

func TestMultiCursorManagerRemoveSecondary(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)
	m.AddCursor(1, 0)
	m.AddCursor(2, 0)

	m.RemoveSecondary()

	if m.CursorCount() != 1 {
		t.Errorf("RemoveSecondary should leave 1 cursor, got %d", m.CursorCount())
	}

	primary, _ := m.Primary()
	if primary.Position.Line != 0 {
		t.Error("Primary cursor should remain after RemoveSecondary")
	}
}

func TestMultiCursorManagerMovePrimary(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)

	m.MovePrimary(10, 20)

	primary, _ := m.Primary()
	if primary.Position.Line != 10 || primary.Position.Column != 20 {
		t.Errorf("Primary position = (%d, %d), want (10, 20)", primary.Position.Line, primary.Position.Column)
	}
}

func TestMultiCursorManagerMovePrimaryEmpty(t *testing.T) {
	m := NewMultiCursorManager()

	m.MovePrimary(10, 20)

	// Should create a cursor
	if m.CursorCount() != 1 {
		t.Error("MovePrimary on empty manager should create cursor")
	}
}

func TestMultiCursorManagerMoveAll(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 10)
	m.AddCursor(6, 10)
	m.AddCursor(7, 10)

	m.MoveAll(1, 2)

	cursors := m.Cursors()
	for _, c := range cursors {
		if c.Position.Column != 12 {
			t.Errorf("Column should be 12, got %d", c.Position.Column)
		}
	}
}

func TestMultiCursorManagerMoveAllNegative(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(1, 1)

	// Move negative by more than current position
	m.MoveAll(-5, -5)

	primary, _ := m.Primary()
	if primary.Position.Line != 0 || primary.Position.Column != 0 {
		t.Errorf("Position should clamp to (0, 0), got (%d, %d)", primary.Position.Line, primary.Position.Column)
	}
}

func TestMultiCursorManagerMoveAllToColumn(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 5)
	m.AddCursor(1, 10)
	m.AddCursor(2, 15)

	m.MoveAllToColumn(0)

	cursors := m.Cursors()
	for _, c := range cursors {
		if c.Position.Column != 0 {
			t.Errorf("All cursors should be at column 0")
		}
	}
}

func TestMultiCursorManagerSetPrimaryIndex(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)
	m.AddCursor(1, 0)
	m.AddCursor(2, 0)

	ok := m.SetPrimaryIndex(2)
	if !ok {
		t.Error("SetPrimaryIndex should return true")
	}

	primary, _ := m.Primary()
	if primary.Position.Line != 2 {
		t.Errorf("Primary should be at line 2, got %d", primary.Position.Line)
	}
}

func TestMultiCursorManagerSetPrimaryIndexInvalid(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)

	ok := m.SetPrimaryIndex(5)
	if ok {
		t.Error("SetPrimaryIndex with invalid index should return false")
	}
}

func TestMultiCursorManagerCyclePrimary(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)
	m.AddCursor(1, 0)
	m.AddCursor(2, 0)

	m.CyclePrimary()

	primary, _ := m.Primary()
	if primary.Position.Line != 1 {
		t.Errorf("After cycling, primary should be at line 1, got %d", primary.Position.Line)
	}

	m.CyclePrimary()
	m.CyclePrimary()

	// Should wrap around
	primary, _ = m.Primary()
	if primary.Position.Line != 0 {
		t.Errorf("After wrapping, primary should be at line 0, got %d", primary.Position.Line)
	}
}

func TestMultiCursorManagerSortByPosition(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 0)
	m.AddCursor(1, 0)
	m.AddCursor(10, 0)
	m.AddCursor(3, 0)

	m.SortByPosition()

	cursors := m.Cursors()
	expectedLines := []uint32{1, 3, 5, 10}
	for i, c := range cursors {
		if c.Position.Line != expectedLines[i] {
			t.Errorf("Cursor %d at line %d, want %d", i, c.Position.Line, expectedLines[i])
		}
	}

	// Primary should still be primary (at line 5)
	primary, _ := m.Primary()
	if primary.Position.Line != 5 {
		t.Error("Primary cursor should remain primary after sort")
	}
}

func TestMultiCursorManagerDeduplicate(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 10)

	// Manually add duplicate (bypassing check)
	m.mu.Lock()
	m.cursors = append(m.cursors, Cursor{
		Position: Position{Line: 5, Column: 10},
		Visible:  true,
	})
	m.mu.Unlock()

	m.Deduplicate()

	if m.CursorCount() != 1 {
		t.Errorf("After deduplicate, should have 1 cursor, got %d", m.CursorCount())
	}
}

func TestMultiCursorManagerCursorsOnLine(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 0)
	m.AddCursor(5, 5)
	m.AddCursor(5, 10)
	m.AddCursor(6, 0)

	cursors := m.CursorsOnLine(5)
	if len(cursors) != 3 {
		t.Errorf("CursorsOnLine(5) returned %d cursors, want 3", len(cursors))
	}

	cursors = m.CursorsOnLine(6)
	if len(cursors) != 1 {
		t.Errorf("CursorsOnLine(6) returned %d cursors, want 1", len(cursors))
	}

	cursors = m.CursorsOnLine(100)
	if len(cursors) != 0 {
		t.Errorf("CursorsOnLine(100) returned %d cursors, want 0", len(cursors))
	}
}

func TestMultiCursorManagerClear(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(0, 0)
	m.AddCursor(1, 0)

	m.Clear()

	if m.CursorCount() != 0 {
		t.Errorf("Clear should remove all cursors, got %d", m.CursorCount())
	}
}

func TestMultiCursorManagerClone(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 10)
	m.AddCursor(6, 11)

	clone := m.Clone()

	if clone.CursorCount() != m.CursorCount() {
		t.Error("Clone should have same cursor count")
	}

	// Modify original
	m.SetPrimary(100, 100)

	// Clone should be unaffected
	primary, _ := clone.Primary()
	if primary.Position.Line != 5 {
		t.Error("Clone should be independent of original")
	}
}

func TestMultiCursorManagerPrimaryEmpty(t *testing.T) {
	m := NewMultiCursorManager()

	_, ok := m.Primary()
	if ok {
		t.Error("Primary() should return false for empty manager")
	}
}

func TestMultiCursorManagerCursorsReturnsCopy(t *testing.T) {
	m := NewMultiCursorManager()
	m.SetPrimary(5, 10)

	cursors := m.Cursors()
	cursors[0].Position.Line = 100

	// Original should be unaffected
	primary, _ := m.Primary()
	if primary.Position.Line == 100 {
		t.Error("Cursors() should return a copy")
	}
}
