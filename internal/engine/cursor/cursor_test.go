package cursor

import (
	"testing"
)

// Cursor Tests

func TestNewCursor(t *testing.T) {
	c := NewCursor(10)
	if c.Offset() != 10 {
		t.Errorf("expected offset 10, got %d", c.Offset())
	}
}

func TestNewCursorNegative(t *testing.T) {
	c := NewCursor(-5)
	if c.Offset() != 0 {
		t.Errorf("negative offset should clamp to 0, got %d", c.Offset())
	}
}

func TestCursorMoveTo(t *testing.T) {
	c := NewCursor(10)
	c2 := c.MoveTo(20)

	if c.Offset() != 10 {
		t.Error("original cursor should be unchanged")
	}
	if c2.Offset() != 20 {
		t.Errorf("expected offset 20, got %d", c2.Offset())
	}
}

func TestCursorMoveBy(t *testing.T) {
	c := NewCursor(10)

	c2 := c.MoveBy(5)
	if c2.Offset() != 15 {
		t.Errorf("expected offset 15, got %d", c2.Offset())
	}

	c3 := c.MoveBy(-5)
	if c3.Offset() != 5 {
		t.Errorf("expected offset 5, got %d", c3.Offset())
	}

	c4 := c.MoveBy(-20)
	if c4.Offset() != 0 {
		t.Errorf("expected offset 0 (clamped), got %d", c4.Offset())
	}
}

func TestCursorClamp(t *testing.T) {
	c := NewCursor(50)

	c2 := c.Clamp(30)
	if c2.Offset() != 30 {
		t.Errorf("expected clamped offset 30, got %d", c2.Offset())
	}

	c3 := c.Clamp(100)
	if c3.Offset() != 50 {
		t.Errorf("expected unchanged offset 50, got %d", c3.Offset())
	}
}

func TestCursorCompare(t *testing.T) {
	c1 := NewCursor(10)
	c2 := NewCursor(20)
	c3 := NewCursor(10)

	if c1.Compare(c2) != -1 {
		t.Error("c1 should be less than c2")
	}
	if c2.Compare(c1) != 1 {
		t.Error("c2 should be greater than c1")
	}
	if c1.Compare(c3) != 0 {
		t.Error("c1 should equal c3")
	}
}

func TestCursorToSelection(t *testing.T) {
	c := NewCursor(10)
	sel := c.ToSelection()

	if sel.Anchor != 10 || sel.Head != 10 {
		t.Error("cursor selection should have anchor == head == offset")
	}
	if !sel.IsEmpty() {
		t.Error("cursor selection should be empty")
	}
}

// Selection Tests

func TestNewSelection(t *testing.T) {
	sel := NewSelection(10, 20)

	if sel.Anchor != 10 {
		t.Errorf("expected anchor 10, got %d", sel.Anchor)
	}
	if sel.Head != 20 {
		t.Errorf("expected head 20, got %d", sel.Head)
	}
}

func TestNewCursorSelection(t *testing.T) {
	sel := NewCursorSelection(15)

	if sel.Anchor != 15 || sel.Head != 15 {
		t.Error("cursor selection should have anchor == head")
	}
	if !sel.IsEmpty() {
		t.Error("cursor selection should be empty")
	}
}

func TestSelectionIsEmpty(t *testing.T) {
	empty := NewCursorSelection(10)
	if !empty.IsEmpty() {
		t.Error("should be empty")
	}

	notEmpty := NewSelection(10, 20)
	if notEmpty.IsEmpty() {
		t.Error("should not be empty")
	}
}

func TestSelectionLen(t *testing.T) {
	sel := NewSelection(10, 20)
	if sel.Len() != 10 {
		t.Errorf("expected len 10, got %d", sel.Len())
	}

	backward := NewSelection(20, 10)
	if backward.Len() != 10 {
		t.Errorf("backward selection len should be 10, got %d", backward.Len())
	}
}

func TestSelectionRange(t *testing.T) {
	forward := NewSelection(10, 20)
	r := forward.Range()
	if r.Start != 10 || r.End != 20 {
		t.Errorf("expected range [10:20), got [%d:%d)", r.Start, r.End)
	}

	backward := NewSelection(20, 10)
	r = backward.Range()
	if r.Start != 10 || r.End != 20 {
		t.Errorf("backward range should be normalized to [10:20), got [%d:%d)", r.Start, r.End)
	}
}

func TestSelectionStartEnd(t *testing.T) {
	forward := NewSelection(10, 20)
	if forward.Start() != 10 || forward.End() != 20 {
		t.Error("forward selection Start/End incorrect")
	}

	backward := NewSelection(20, 10)
	if backward.Start() != 10 || backward.End() != 20 {
		t.Error("backward selection Start/End incorrect")
	}
}

func TestSelectionDirection(t *testing.T) {
	forward := NewSelection(10, 20)
	if !forward.IsForward() {
		t.Error("should be forward")
	}
	if forward.IsBackward() {
		t.Error("should not be backward")
	}

	backward := NewSelection(20, 10)
	if backward.IsForward() {
		t.Error("should not be forward")
	}
	if !backward.IsBackward() {
		t.Error("should be backward")
	}
}

func TestSelectionExtend(t *testing.T) {
	sel := NewCursorSelection(10)
	extended := sel.Extend(20)

	if extended.Anchor != 10 {
		t.Error("anchor should remain at 10")
	}
	if extended.Head != 20 {
		t.Error("head should be at 20")
	}
}

func TestSelectionCollapse(t *testing.T) {
	sel := NewSelection(10, 20)

	collapsed := sel.Collapse()
	if collapsed.Anchor != 20 || collapsed.Head != 20 {
		t.Error("collapse should move to head")
	}

	toStart := sel.CollapseToStart()
	if toStart.Anchor != 10 || toStart.Head != 10 {
		t.Error("collapseToStart should move to start")
	}

	toEnd := sel.CollapseToEnd()
	if toEnd.Anchor != 20 || toEnd.Head != 20 {
		t.Error("collapseToEnd should move to end")
	}
}

func TestSelectionFlip(t *testing.T) {
	sel := NewSelection(10, 20)
	flipped := sel.Flip()

	if flipped.Anchor != 20 || flipped.Head != 10 {
		t.Error("flip should swap anchor and head")
	}
}

func TestSelectionNormalize(t *testing.T) {
	backward := NewSelection(20, 10)
	normalized := backward.Normalize()

	if normalized.Anchor != 10 || normalized.Head != 20 {
		t.Error("normalize should make selection forward")
	}
	if !normalized.IsForward() {
		t.Error("normalized should be forward")
	}
}

func TestSelectionContains(t *testing.T) {
	sel := NewSelection(10, 20)

	if !sel.Contains(15) {
		t.Error("selection should contain 15")
	}
	if !sel.Contains(10) {
		t.Error("selection should contain start (10)")
	}
	if sel.Contains(20) {
		t.Error("selection should not contain end (20, exclusive)")
	}
	if sel.Contains(5) {
		t.Error("selection should not contain 5")
	}

	empty := NewCursorSelection(10)
	if empty.Contains(10) {
		t.Error("empty selection should not contain anything")
	}
}

func TestSelectionOverlaps(t *testing.T) {
	sel1 := NewSelection(10, 20)
	sel2 := NewSelection(15, 25)
	sel3 := NewSelection(25, 35)
	sel4 := NewSelection(5, 15)

	if !sel1.Overlaps(sel2) {
		t.Error("sel1 should overlap sel2")
	}
	if sel1.Overlaps(sel3) {
		t.Error("sel1 should not overlap sel3")
	}
	if !sel1.Overlaps(sel4) {
		t.Error("sel1 should overlap sel4")
	}
}

func TestSelectionTouches(t *testing.T) {
	sel1 := NewSelection(10, 20)
	sel2 := NewSelection(20, 30)
	sel3 := NewSelection(25, 35)

	if !sel1.Touches(sel2) {
		t.Error("sel1 should touch sel2 (adjacent)")
	}
	if sel1.Touches(sel3) {
		t.Error("sel1 should not touch sel3")
	}
}

func TestSelectionMerge(t *testing.T) {
	sel1 := NewSelection(10, 20)
	sel2 := NewSelection(15, 30)

	merged := sel1.Merge(sel2)
	if merged.Start() != 10 || merged.End() != 30 {
		t.Errorf("merged should be [10:30), got [%d:%d)", merged.Start(), merged.End())
	}
}

func TestSelectionClamp(t *testing.T) {
	sel := NewSelection(10, 50)
	clamped := sel.Clamp(30)

	if clamped.Anchor != 10 || clamped.Head != 30 {
		t.Errorf("expected clamped to [10:30], got [%d:%d]", clamped.Anchor, clamped.Head)
	}
}

// CursorSet Tests

func TestNewCursorSet(t *testing.T) {
	sel := NewCursorSelection(10)
	cs := NewCursorSet(sel)

	if cs.Count() != 1 {
		t.Errorf("expected count 1, got %d", cs.Count())
	}
	if cs.Primary().Head != 10 {
		t.Error("primary should be at offset 10")
	}
}

func TestCursorSetAdd(t *testing.T) {
	cs := NewCursorSetAt(10)
	cs.Add(NewCursorSelection(30))

	if cs.Count() != 2 {
		t.Errorf("expected count 2, got %d", cs.Count())
	}
}

func TestCursorSetAddMerge(t *testing.T) {
	cs := NewCursorSet(NewSelection(10, 20))
	cs.Add(NewSelection(15, 25))

	if cs.Count() != 1 {
		t.Errorf("overlapping selections should merge, got count %d", cs.Count())
	}

	sel := cs.Primary()
	if sel.Start() != 10 || sel.End() != 25 {
		t.Errorf("merged selection should be [10:25), got [%d:%d)", sel.Start(), sel.End())
	}
}

func TestCursorSetNormalize(t *testing.T) {
	// Create with unsorted selections
	cs := NewCursorSetFromSlice([]Selection{
		NewSelection(30, 40),
		NewSelection(10, 20),
		NewSelection(50, 60),
	})

	if cs.Count() != 3 {
		t.Errorf("expected 3 selections, got %d", cs.Count())
	}

	// Should be sorted
	sels := cs.All()
	if sels[0].Start() != 10 || sels[1].Start() != 30 || sels[2].Start() != 50 {
		t.Error("selections should be sorted by start position")
	}
}

func TestCursorSetClear(t *testing.T) {
	cs := NewCursorSetAt(10)
	cs.Add(NewCursorSelection(20))
	cs.Add(NewCursorSelection(30))

	if cs.Count() != 3 {
		t.Errorf("expected 3 cursors, got %d", cs.Count())
	}

	cs.Clear()

	if cs.Count() != 1 {
		t.Errorf("after clear, expected 1 cursor, got %d", cs.Count())
	}
}

func TestCursorSetClamp(t *testing.T) {
	cs := NewCursorSetFromSlice([]Selection{
		NewSelection(10, 20),
		NewSelection(40, 60),
	})

	cs.Clamp(50)

	sels := cs.All()
	if sels[1].End() != 50 {
		t.Errorf("second selection should be clamped to 50, got %d", sels[1].End())
	}
}

func TestCursorSetHasSelection(t *testing.T) {
	cursorsOnly := NewCursorSetFromSlice([]Selection{
		NewCursorSelection(10),
		NewCursorSelection(20),
	})
	if cursorsOnly.HasSelection() {
		t.Error("cursors only should not have selection")
	}

	withSelection := NewCursorSetFromSlice([]Selection{
		NewCursorSelection(10),
		NewSelection(20, 30),
	})
	if !withSelection.HasSelection() {
		t.Error("should have selection")
	}
}

func TestCursorSetClone(t *testing.T) {
	cs := NewCursorSetFromSlice([]Selection{
		NewSelection(10, 20),
		NewSelection(30, 40),
	})

	clone := cs.Clone()

	// Modify original
	cs.Add(NewCursorSelection(50))

	if clone.Count() != 2 {
		t.Error("clone should not be affected by original modifications")
	}
}

func TestCursorSetEqualsNil(t *testing.T) {
	cs := NewCursorSetAt(10)
	if cs.Equals(nil) {
		t.Error("Equals(nil) should return false")
	}
}

// Transform Tests

func TestTransformOffsetInsertBefore(t *testing.T) {
	// Insert "Hello" (5 chars) at offset 0
	edit := Edit{
		Range:   Range{Start: 0, End: 0},
		NewText: "Hello",
	}

	offset := TransformOffset(10, edit)
	if offset != 15 {
		t.Errorf("offset should shift right by 5, got %d", offset)
	}
}

func TestTransformOffsetInsertAfter(t *testing.T) {
	// Insert at offset 20, cursor at 10
	edit := Edit{
		Range:   Range{Start: 20, End: 20},
		NewText: "Hello",
	}

	offset := TransformOffset(10, edit)
	if offset != 10 {
		t.Errorf("offset should be unchanged, got %d", offset)
	}
}

func TestTransformOffsetDeleteBefore(t *testing.T) {
	// Delete 5 chars at offset 0-5
	edit := Edit{
		Range:   Range{Start: 0, End: 5},
		NewText: "",
	}

	offset := TransformOffset(10, edit)
	if offset != 5 {
		t.Errorf("offset should shift left by 5, got %d", offset)
	}
}

func TestTransformOffsetDeleteSpanning(t *testing.T) {
	// Delete chars from 5 to 15, cursor at 10
	edit := Edit{
		Range:   Range{Start: 5, End: 15},
		NewText: "",
	}

	offset := TransformOffset(10, edit)
	if offset != 5 {
		t.Errorf("offset should move to start of deletion, got %d", offset)
	}
}

func TestTransformOffsetReplace(t *testing.T) {
	// Replace 5 chars with 10 chars at 0-5
	edit := Edit{
		Range:   Range{Start: 0, End: 5},
		NewText: "0123456789",
	}

	offset := TransformOffset(10, edit)
	// Cursor was at 10, delete shifted it to 5, insert of 10 shifts it to 15
	if offset != 15 {
		t.Errorf("expected offset 15, got %d", offset)
	}
}

func TestTransformSelection(t *testing.T) {
	sel := NewSelection(10, 20)

	// Insert 5 chars at offset 0
	edit := Edit{
		Range:   Range{Start: 0, End: 0},
		NewText: "Hello",
	}

	transformed := TransformSelection(sel, edit)
	if transformed.Anchor != 15 || transformed.Head != 25 {
		t.Errorf("selection should shift by 5, got [%d:%d]", transformed.Anchor, transformed.Head)
	}
}

func TestTransformCursorSet(t *testing.T) {
	cs := NewCursorSetFromSlice([]Selection{
		NewCursorSelection(10),
		NewCursorSelection(20),
		NewCursorSelection(30),
	})

	// Insert 5 chars at offset 0
	edit := Edit{
		Range:   Range{Start: 0, End: 0},
		NewText: "Hello",
	}

	TransformCursorSet(cs, edit)

	sels := cs.All()
	if sels[0].Head != 15 || sels[1].Head != 25 || sels[2].Head != 35 {
		t.Error("all cursors should shift by 5")
	}
}

func TestTransformCursorSetMulti(t *testing.T) {
	cs := NewCursorSetAt(50)

	// Multiple edits applied in order
	edits := []Edit{
		{Range: Range{Start: 0, End: 0}, NewText: "AAAAA"},   // +5
		{Range: Range{Start: 10, End: 15}, NewText: ""},      // -5
		{Range: Range{Start: 20, End: 20}, NewText: "BBBBB"}, // +5
	}

	TransformCursorSetMulti(cs, edits)

	// Net effect: +5, cursor at 50 should end at 55
	if cs.PrimaryCursor() != 55 {
		t.Errorf("expected cursor at 55, got %d", cs.PrimaryCursor())
	}
}

func TestComputeEditDelta(t *testing.T) {
	insert := Edit{Range: Range{Start: 0, End: 0}, NewText: "Hello"}
	if ComputeEditDelta(insert) != 5 {
		t.Error("insert delta should be 5")
	}

	del := Edit{Range: Range{Start: 0, End: 10}, NewText: ""}
	if ComputeEditDelta(del) != -10 {
		t.Error("delete delta should be -10")
	}

	replace := Edit{Range: Range{Start: 0, End: 5}, NewText: "HelloWorld"}
	if ComputeEditDelta(replace) != 5 {
		t.Error("replace delta should be 5 (10 - 5)")
	}
}

func TestEditsInReverseOrder(t *testing.T) {
	correct := []Edit{
		{Range: Range{Start: 30, End: 35}},
		{Range: Range{Start: 20, End: 25}},
		{Range: Range{Start: 10, End: 15}},
	}
	if !EditsInReverseOrder(correct) {
		t.Error("should be in reverse order")
	}

	incorrect := []Edit{
		{Range: Range{Start: 10, End: 15}},
		{Range: Range{Start: 20, End: 25}},
	}
	if EditsInReverseOrder(incorrect) {
		t.Error("should not be in reverse order")
	}
}

func TestSortEditsReverse(t *testing.T) {
	edits := []Edit{
		{Range: Range{Start: 10, End: 15}},
		{Range: Range{Start: 30, End: 35}},
		{Range: Range{Start: 20, End: 25}},
	}

	SortEditsReverse(edits)

	if edits[0].Range.Start != 30 || edits[1].Range.Start != 20 || edits[2].Range.Start != 10 {
		t.Error("edits should be sorted in descending order by start")
	}
}

// Edge case tests

func TestOverlappingSelectionsNormalize(t *testing.T) {
	cs := NewCursorSetFromSlice([]Selection{
		NewSelection(0, 20),
		NewSelection(10, 30),
		NewSelection(25, 40),
	})

	// All overlap, should merge to one
	if cs.Count() != 1 {
		t.Errorf("expected 1 merged selection, got %d", cs.Count())
	}

	sel := cs.Primary()
	if sel.Start() != 0 || sel.End() != 40 {
		t.Errorf("expected merged selection [0:40), got [%d:%d)", sel.Start(), sel.End())
	}
}

func TestAdjacentSelectionsNormalize(t *testing.T) {
	cs := NewCursorSetFromSlice([]Selection{
		NewSelection(0, 10),
		NewSelection(10, 20),
		NewSelection(20, 30),
	})

	// Adjacent selections should merge
	if cs.Count() != 1 {
		t.Errorf("expected 1 merged selection, got %d", cs.Count())
	}

	sel := cs.Primary()
	if sel.Start() != 0 || sel.End() != 30 {
		t.Errorf("expected merged selection [0:30), got [%d:%d)", sel.Start(), sel.End())
	}
}

func TestTransformDeleteEntireSelection(t *testing.T) {
	sel := NewSelection(10, 20)

	// Delete exactly the selection
	edit := Edit{
		Range:   Range{Start: 10, End: 20},
		NewText: "",
	}

	transformed := TransformSelection(sel, edit)

	// Both anchor and head should move to 10
	if transformed.Anchor != 10 || transformed.Head != 10 {
		t.Errorf("expected collapsed at 10, got [%d:%d]", transformed.Anchor, transformed.Head)
	}
}

func TestTransformInsertAtCursor(t *testing.T) {
	sel := NewCursorSelection(10)

	// Insert exactly at cursor position
	edit := Edit{
		Range:   Range{Start: 10, End: 10},
		NewText: "Hello",
	}

	transformed := TransformSelection(sel, edit)

	// Cursor should move to end of insertion
	if transformed.Head != 15 {
		t.Errorf("cursor should move to 15, got %d", transformed.Head)
	}
}

func TestMultiCursorEditing(t *testing.T) {
	// Simulate typing 'x' at multiple cursor positions
	cs := NewCursorSetFromSlice([]Selection{
		NewCursorSelection(10),
		NewCursorSelection(20),
		NewCursorSelection(30),
	})

	// Edits in reverse order (as buffer.ApplyEdits expects)
	edits := []Edit{
		{Range: Range{Start: 30, End: 30}, NewText: "x"},
		{Range: Range{Start: 20, End: 20}, NewText: "x"},
		{Range: Range{Start: 10, End: 10}, NewText: "x"},
	}

	TransformCursorSetMulti(cs, edits)

	// After inserting 'x' at each position, cursors should be after each 'x'
	sels := cs.All()
	if sels[0].Head != 11 {
		t.Errorf("first cursor should be at 11, got %d", sels[0].Head)
	}
	if sels[1].Head != 22 {
		t.Errorf("second cursor should be at 22 (10+1 + 10+1), got %d", sels[1].Head)
	}
	if sels[2].Head != 33 {
		t.Errorf("third cursor should be at 33, got %d", sels[2].Head)
	}
}
