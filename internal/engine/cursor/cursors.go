package cursor

import "sort"

// CursorSet manages multiple cursors/selections.
// Selections are kept sorted by position and non-overlapping.
// The first selection is considered the "primary" selection.
type CursorSet struct {
	selections []Selection
}

// NewCursorSet creates a cursor set with a single selection.
func NewCursorSet(initial Selection) *CursorSet {
	return &CursorSet{
		selections: []Selection{initial},
	}
}

// NewCursorSetAt creates a cursor set with a single cursor at the given offset.
func NewCursorSetAt(offset ByteOffset) *CursorSet {
	return &CursorSet{
		selections: []Selection{NewCursorSelection(offset)},
	}
}

// NewCursorSetFromSlice creates a cursor set from a slice of selections.
// The selections will be normalized (sorted and merged).
func NewCursorSetFromSlice(selections []Selection) *CursorSet {
	if len(selections) == 0 {
		return &CursorSet{
			selections: []Selection{NewCursorSelection(0)},
		}
	}
	cs := &CursorSet{
		selections: make([]Selection, len(selections)),
	}
	copy(cs.selections, selections)
	cs.normalize()
	return cs
}

// Primary returns the primary (first) selection.
func (cs *CursorSet) Primary() Selection {
	if len(cs.selections) == 0 {
		return Selection{}
	}
	return cs.selections[0]
}

// PrimaryCursor returns the head offset of the primary selection.
func (cs *CursorSet) PrimaryCursor() ByteOffset {
	if len(cs.selections) == 0 {
		return 0
	}
	return cs.selections[0].Head
}

// All returns a copy of all selections.
// The returned slice is safe to modify without affecting the CursorSet.
func (cs *CursorSet) All() []Selection {
	result := make([]Selection, len(cs.selections))
	copy(result, cs.selections)
	return result
}

// Count returns the number of cursors/selections.
func (cs *CursorSet) Count() int {
	return len(cs.selections)
}

// IsMulti returns true if there are multiple selections.
func (cs *CursorSet) IsMulti() bool {
	return len(cs.selections) > 1
}

// Get returns the selection at the given index.
// Returns an empty selection if index is out of range.
func (cs *CursorSet) Get(index int) Selection {
	if index < 0 || index >= len(cs.selections) {
		return Selection{}
	}
	return cs.selections[index]
}

// Add adds a new selection, merging with overlapping ones.
func (cs *CursorSet) Add(sel Selection) {
	cs.selections = append(cs.selections, sel)
	cs.normalize()
}

// AddAll adds multiple selections.
func (cs *CursorSet) AddAll(sels []Selection) {
	cs.selections = append(cs.selections, sels...)
	cs.normalize()
}

// SetPrimary sets the primary selection, keeping others.
// Note: After normalization (sorting/merging), the primary selection
// will be the one with the lowest start position, which may not be
// the selection passed to this method if it overlaps with others.
func (cs *CursorSet) SetPrimary(sel Selection) {
	if len(cs.selections) == 0 {
		cs.selections = []Selection{sel}
	} else {
		cs.selections[0] = sel
	}
	cs.normalize()
}

// Set replaces all selections with a single selection.
func (cs *CursorSet) Set(sel Selection) {
	cs.selections = []Selection{sel}
}

// SetAll replaces all selections.
func (cs *CursorSet) SetAll(sels []Selection) {
	if len(sels) == 0 {
		cs.selections = []Selection{NewCursorSelection(0)}
		return
	}
	cs.selections = make([]Selection, len(sels))
	copy(cs.selections, sels)
	cs.normalize()
}

// Clear removes all selections except primary.
func (cs *CursorSet) Clear() {
	if len(cs.selections) > 1 {
		cs.selections = cs.selections[:1]
	}
}

// Remove removes the selection at the given index.
// If it's the last selection, it's replaced with a cursor at position 0.
func (cs *CursorSet) Remove(index int) {
	if index < 0 || index >= len(cs.selections) {
		return
	}
	cs.selections = append(cs.selections[:index], cs.selections[index+1:]...)
	if len(cs.selections) == 0 {
		cs.selections = []Selection{NewCursorSelection(0)}
	}
}

// RemoveLast removes the last added selection.
func (cs *CursorSet) RemoveLast() {
	if len(cs.selections) > 1 {
		cs.selections = cs.selections[:len(cs.selections)-1]
	}
}

// ForEach calls f for each selection with its index.
func (cs *CursorSet) ForEach(f func(index int, sel Selection)) {
	for i, sel := range cs.selections {
		f(i, sel)
	}
}

// Map applies f to each selection and returns the results.
func (cs *CursorSet) Map(f func(sel Selection) Selection) []Selection {
	result := make([]Selection, len(cs.selections))
	for i, sel := range cs.selections {
		result[i] = f(sel)
	}
	return result
}

// MapInPlace applies f to each selection in place.
func (cs *CursorSet) MapInPlace(f func(sel Selection) Selection) {
	for i, sel := range cs.selections {
		cs.selections[i] = f(sel)
	}
	cs.normalize()
}

// HasSelection returns true if any selection is non-empty (has extent).
func (cs *CursorSet) HasSelection() bool {
	for _, sel := range cs.selections {
		if !sel.IsEmpty() {
			return true
		}
	}
	return false
}

// CollapseAll collapses all selections to cursors at their heads.
func (cs *CursorSet) CollapseAll() {
	for i, sel := range cs.selections {
		cs.selections[i] = sel.Collapse()
	}
	cs.normalize()
}

// Clamp clamps all selections to the valid range [0, maxOffset].
func (cs *CursorSet) Clamp(maxOffset ByteOffset) {
	for i, sel := range cs.selections {
		cs.selections[i] = sel.Clamp(maxOffset)
	}
	cs.normalize()
}

// Clone returns a deep copy of the cursor set.
func (cs *CursorSet) Clone() *CursorSet {
	clone := &CursorSet{
		selections: make([]Selection, len(cs.selections)),
	}
	copy(clone.selections, cs.selections)
	return clone
}

// Ranges returns all selection ranges (for operations like delete).
func (cs *CursorSet) Ranges() []Range {
	ranges := make([]Range, len(cs.selections))
	for i, sel := range cs.selections {
		ranges[i] = sel.Range()
	}
	return ranges
}

// SelectionRanges returns ranges only for non-empty selections.
func (cs *CursorSet) SelectionRanges() []Range {
	var ranges []Range
	for _, sel := range cs.selections {
		if !sel.IsEmpty() {
			ranges = append(ranges, sel.Range())
		}
	}
	return ranges
}

// normalize sorts selections and merges overlapping/adjacent ones.
func (cs *CursorSet) normalize() {
	if len(cs.selections) <= 1 {
		return
	}

	// Sort by start position
	sort.Slice(cs.selections, func(i, j int) bool {
		si, sj := cs.selections[i].Start(), cs.selections[j].Start()
		if si != sj {
			return si < sj
		}
		// If same start, sort by end (larger ranges first)
		return cs.selections[i].End() > cs.selections[j].End()
	})

	// Merge overlapping or adjacent selections
	merged := cs.selections[:1]
	for _, sel := range cs.selections[1:] {
		last := &merged[len(merged)-1]
		if sel.Start() <= last.End() {
			// Overlapping or adjacent: merge
			*last = last.Merge(sel)
		} else {
			merged = append(merged, sel)
		}
	}
	cs.selections = merged
}

// Equals returns true if two cursor sets have the same selections.
func (cs *CursorSet) Equals(other *CursorSet) bool {
	if other == nil {
		return false
	}
	if cs.Count() != other.Count() {
		return false
	}
	for i, sel := range cs.selections {
		if !sel.Equals(other.selections[i]) {
			return false
		}
	}
	return true
}
