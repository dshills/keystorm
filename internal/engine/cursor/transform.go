package cursor

import (
	"sort"

	"github.com/dshills/keystorm/internal/engine/buffer"
)

// Edit is an alias for buffer.Edit for convenience.
type Edit = buffer.Edit

// TransformOffset updates an offset after an edit.
// Returns the new offset position.
//
// Transformation rules:
//   - If edit is entirely before offset: adjust offset by the edit's delta
//   - If edit starts at or after offset: offset unchanged
//   - If edit spans offset: move offset to end of new text
func TransformOffset(offset ByteOffset, edit Edit) ByteOffset {
	// Edit is entirely before offset: adjust by delta
	if edit.Range.End <= offset {
		oldLen := edit.Range.End - edit.Range.Start
		newLen := ByteOffset(len(edit.NewText))
		return offset - oldLen + newLen
	}

	// Edit starts at or after offset: no change needed
	if edit.Range.Start >= offset {
		return offset
	}

	// Edit spans offset: move to end of new text
	return edit.Range.Start + ByteOffset(len(edit.NewText))
}

// TransformOffsetSticky is like TransformOffset but with a "sticky" behavior
// that determines how the offset behaves when the edit starts exactly at the offset.
// If sticky is true, the offset "sticks" to its position (stays at start of insert).
// If sticky is false, the offset moves with insertions (moves to end of insert).
func TransformOffsetSticky(offset ByteOffset, edit Edit, sticky bool) ByteOffset {
	// Edit is entirely before offset: adjust by delta
	if edit.Range.End <= offset {
		oldLen := edit.Range.End - edit.Range.Start
		newLen := ByteOffset(len(edit.NewText))
		return offset - oldLen + newLen
	}

	// For insertions at exactly the offset position
	if edit.Range.Start == offset && edit.Range.Start == edit.Range.End {
		if sticky {
			// Sticky: stay at current position
			return offset
		}
		// Non-sticky: move to end of insertion
		return offset + ByteOffset(len(edit.NewText))
	}

	// Edit starts after offset: no change needed
	if edit.Range.Start >= offset {
		return offset
	}

	// Edit spans offset: move to end of new text
	return edit.Range.Start + ByteOffset(len(edit.NewText))
}

// TransformCursor updates a cursor after an edit.
func TransformCursor(c Cursor, edit Edit) Cursor {
	return NewCursor(TransformOffset(c.offset, edit))
}

// TransformSelection updates a selection after an edit.
// Both anchor and head are transformed independently.
func TransformSelection(sel Selection, edit Edit) Selection {
	return Selection{
		Anchor: TransformOffset(sel.Anchor, edit),
		Head:   TransformOffset(sel.Head, edit),
	}
}

// TransformSelectionWithBias transforms a selection with specified bias for anchor and head.
// Anchor typically has sticky=true (stays at position for insertions at anchor).
// Head typically has sticky=false (moves with insertions at cursor).
func TransformSelectionWithBias(sel Selection, edit Edit, anchorSticky, headSticky bool) Selection {
	return Selection{
		Anchor: TransformOffsetSticky(sel.Anchor, edit, anchorSticky),
		Head:   TransformOffsetSticky(sel.Head, edit, headSticky),
	}
}

// TransformCursorSet updates all selections in a cursor set after an edit.
func TransformCursorSet(cs *CursorSet, edit Edit) {
	for i := range cs.selections {
		cs.selections[i] = TransformSelection(cs.selections[i], edit)
	}
	cs.normalize()
}

// TransformCursorSetMulti updates selections after multiple edits.
// Edits must be provided in the order they were applied.
// They will be processed in reverse order internally to maintain offset validity.
func TransformCursorSetMulti(cs *CursorSet, edits []Edit) {
	// Process edits in reverse order because each edit shifts positions
	// for everything after it
	for i := len(edits) - 1; i >= 0; i-- {
		TransformCursorSet(cs, edits[i])
	}
}

// TransformRanges updates a slice of ranges after an edit.
// Useful for transforming multiple independent ranges.
// Ranges are normalized to ensure Start <= End after transformation.
func TransformRanges(ranges []Range, edit Edit) []Range {
	result := make([]Range, len(ranges))
	for i, r := range ranges {
		start := TransformOffset(r.Start, edit)
		end := TransformOffset(r.End, edit)
		// Normalize: ensure Start <= End
		if start > end {
			start, end = end, start
		}
		result[i] = Range{Start: start, End: end}
	}
	return result
}

// AdjustForDeletion handles the special case of transforming offsets
// when text is deleted. If the offset is within the deleted range,
// it moves to the start of the deletion.
func AdjustForDeletion(offset ByteOffset, deleteRange Range) ByteOffset {
	// Before deletion: unchanged
	if offset <= deleteRange.Start {
		return offset
	}

	// Within deletion: move to start
	if offset < deleteRange.End {
		return deleteRange.Start
	}

	// After deletion: shift left
	return offset - (deleteRange.End - deleteRange.Start)
}

// AdjustForInsertion handles the special case of transforming offsets
// when text is inserted. Offsets at the insertion point move to the end
// of the inserted text.
func AdjustForInsertion(offset ByteOffset, insertOffset ByteOffset, insertLen ByteOffset) ByteOffset {
	// Before insertion: unchanged
	if offset < insertOffset {
		return offset
	}

	// At or after insertion: shift right
	return offset + insertLen
}

// ComputeEditDelta returns the change in document length from an edit.
func ComputeEditDelta(edit Edit) ByteOffset {
	return ByteOffset(len(edit.NewText)) - (edit.Range.End - edit.Range.Start)
}

// EditsInReverseOrder returns true if edits are sorted by descending start position.
// This is the required order for applying multiple edits atomically.
func EditsInReverseOrder(edits []Edit) bool {
	for i := 1; i < len(edits); i++ {
		if edits[i].Range.Start >= edits[i-1].Range.Start {
			return false
		}
	}
	return true
}

// SortEditsReverse sorts edits in descending order by start position.
// This mutates the input slice.
func SortEditsReverse(edits []Edit) {
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].Range.Start > edits[j].Range.Start
	})
}
