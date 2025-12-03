package cursor

import (
	"fmt"

	"github.com/dshills/keystorm/internal/engine/buffer"
)

// Range is an alias for buffer.Range for convenience.
type Range = buffer.Range

// Selection represents a range of selected text.
// Anchor is where the selection started; Head is the current cursor position.
// When Anchor == Head, this represents a cursor with no selection.
// Selection is an immutable value type.
type Selection struct {
	Anchor ByteOffset // Where selection started
	Head   ByteOffset // Current cursor position (where typing occurs)
}

// NewSelection creates a selection from anchor to head.
func NewSelection(anchor, head ByteOffset) Selection {
	return Selection{Anchor: anchor, Head: head}
}

// NewCursorSelection creates a selection representing just a cursor (no extent).
func NewCursorSelection(offset ByteOffset) Selection {
	return Selection{Anchor: offset, Head: offset}
}

// NewRangeSelection creates a forward selection covering the given range.
func NewRangeSelection(r Range) Selection {
	return Selection{Anchor: r.Start, Head: r.End}
}

// IsEmpty returns true if the selection has no extent (just a cursor).
func (s Selection) IsEmpty() bool {
	return s.Anchor == s.Head
}

// Len returns the length of the selection in bytes.
func (s Selection) Len() ByteOffset {
	if s.Anchor <= s.Head {
		return s.Head - s.Anchor
	}
	return s.Anchor - s.Head
}

// Range returns the selection as a range (always Start <= End).
func (s Selection) Range() Range {
	if s.Anchor <= s.Head {
		return Range{Start: s.Anchor, End: s.Head}
	}
	return Range{Start: s.Head, End: s.Anchor}
}

// Start returns the lower bound of the selection.
func (s Selection) Start() ByteOffset {
	if s.Anchor <= s.Head {
		return s.Anchor
	}
	return s.Head
}

// End returns the upper bound of the selection.
func (s Selection) End() ByteOffset {
	if s.Anchor >= s.Head {
		return s.Anchor
	}
	return s.Head
}

// Cursor returns the head position (where typing would occur).
func (s Selection) Cursor() ByteOffset {
	return s.Head
}

// IsForward returns true if the selection extends forward (head >= anchor).
func (s Selection) IsForward() bool {
	return s.Head >= s.Anchor
}

// IsBackward returns true if the selection extends backward (head < anchor).
func (s Selection) IsBackward() bool {
	return s.Head < s.Anchor
}

// Extend returns a new selection extended to include the given offset.
// The anchor remains fixed; only the head moves.
func (s Selection) Extend(offset ByteOffset) Selection {
	return Selection{Anchor: s.Anchor, Head: offset}
}

// ExtendBy returns a new selection with head moved by delta bytes.
func (s Selection) ExtendBy(delta ByteOffset) Selection {
	return Selection{Anchor: s.Anchor, Head: s.Head + delta}
}

// MoveTo returns a new collapsed selection (cursor) at the given offset.
func (s Selection) MoveTo(offset ByteOffset) Selection {
	return Selection{Anchor: offset, Head: offset}
}

// MoveBy returns a new selection shifted by delta bytes (both anchor and head).
func (s Selection) MoveBy(delta ByteOffset) Selection {
	return Selection{Anchor: s.Anchor + delta, Head: s.Head + delta}
}

// Collapse collapses the selection to a cursor at the head.
func (s Selection) Collapse() Selection {
	return Selection{Anchor: s.Head, Head: s.Head}
}

// CollapseToStart collapses the selection to its start position.
func (s Selection) CollapseToStart() Selection {
	start := s.Start()
	return Selection{Anchor: start, Head: start}
}

// CollapseToEnd collapses the selection to its end position.
func (s Selection) CollapseToEnd() Selection {
	end := s.End()
	return Selection{Anchor: end, Head: end}
}

// Flip returns a selection with anchor and head swapped.
func (s Selection) Flip() Selection {
	return Selection{Anchor: s.Head, Head: s.Anchor}
}

// Normalize returns a forward selection (anchor <= head).
func (s Selection) Normalize() Selection {
	if s.Anchor <= s.Head {
		return s
	}
	return Selection{Anchor: s.Head, Head: s.Anchor}
}

// Contains returns true if the given offset is within the selection.
// For empty selections (cursors), this always returns false.
func (s Selection) Contains(offset ByteOffset) bool {
	start, end := s.Start(), s.End()
	return offset >= start && offset < end
}

// ContainsInclusive returns true if the offset is within [start, end].
func (s Selection) ContainsInclusive(offset ByteOffset) bool {
	start, end := s.Start(), s.End()
	return offset >= start && offset <= end
}

// Overlaps returns true if this selection overlaps with another.
func (s Selection) Overlaps(other Selection) bool {
	return s.Start() < other.End() && other.Start() < s.End()
}

// Touches returns true if selections overlap or are adjacent.
func (s Selection) Touches(other Selection) bool {
	return s.Start() <= other.End() && other.Start() <= s.End()
}

// Merge merges two overlapping or adjacent selections into one.
// Returns a forward selection covering both ranges.
// Note: The resulting selection is always forward (anchor <= head),
// so direction information from the original selections is not preserved.
func (s Selection) Merge(other Selection) Selection {
	start := s.Start()
	if other.Start() < start {
		start = other.Start()
	}
	end := s.End()
	if other.End() > end {
		end = other.End()
	}
	return Selection{Anchor: start, Head: end}
}

// Clamp returns a selection clamped to the valid range [0, maxOffset].
func (s Selection) Clamp(maxOffset ByteOffset) Selection {
	anchor := s.Anchor
	head := s.Head

	if anchor < 0 {
		anchor = 0
	} else if anchor > maxOffset {
		anchor = maxOffset
	}

	if head < 0 {
		head = 0
	} else if head > maxOffset {
		head = maxOffset
	}

	return Selection{Anchor: anchor, Head: head}
}

// String returns a string representation of the selection.
func (s Selection) String() string {
	if s.IsEmpty() {
		return fmt.Sprintf("Cursor(%d)", s.Head)
	}
	dir := "→"
	if s.IsBackward() {
		dir = "←"
	}
	return fmt.Sprintf("Selection(%d%s%d)", s.Anchor, dir, s.Head)
}

// Equals returns true if two selections have the same anchor and head.
func (s Selection) Equals(other Selection) bool {
	return s.Anchor == other.Anchor && s.Head == other.Head
}

// SameRange returns true if two selections cover the same range,
// regardless of direction.
func (s Selection) SameRange(other Selection) bool {
	return s.Start() == other.Start() && s.End() == other.End()
}
