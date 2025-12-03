package buffer

import "fmt"

// Range represents a byte range in the buffer.
// Start is inclusive, End is exclusive: [Start, End).
type Range struct {
	Start ByteOffset // Inclusive start position
	End   ByteOffset // Exclusive end position
}

// NewRange creates a new Range from start and end offsets.
func NewRange(start, end ByteOffset) Range {
	return Range{Start: start, End: end}
}

// String returns a human-readable representation of the range.
func (r Range) String() string {
	return fmt.Sprintf("[%d:%d)", r.Start, r.End)
}

// Len returns the length of the range in bytes.
func (r Range) Len() ByteOffset {
	return r.End - r.Start
}

// IsEmpty returns true if the range has zero length.
func (r Range) IsEmpty() bool {
	return r.Start == r.End
}

// IsValid returns true if the range is valid (Start <= End).
func (r Range) IsValid() bool {
	return r.Start <= r.End
}

// Contains returns true if the given offset is within the range.
func (r Range) Contains(offset ByteOffset) bool {
	return offset >= r.Start && offset < r.End
}

// ContainsRange returns true if the given range is entirely within this range.
func (r Range) ContainsRange(other Range) bool {
	return other.Start >= r.Start && other.End <= r.End
}

// Overlaps returns true if this range overlaps with another range.
func (r Range) Overlaps(other Range) bool {
	return r.Start < other.End && other.Start < r.End
}

// Intersect returns the intersection of two ranges, or an empty range if they don't overlap.
func (r Range) Intersect(other Range) Range {
	start := r.Start
	if other.Start > start {
		start = other.Start
	}
	end := r.End
	if other.End < end {
		end = other.End
	}
	if start >= end {
		return Range{Start: start, End: start}
	}
	return Range{Start: start, End: end}
}

// Union returns the smallest range that contains both ranges.
func (r Range) Union(other Range) Range {
	start := r.Start
	if other.Start < start {
		start = other.Start
	}
	end := r.End
	if other.End > end {
		end = other.End
	}
	return Range{Start: start, End: end}
}

// Shift returns a new range shifted by the given delta.
func (r Range) Shift(delta ByteOffset) Range {
	return Range{
		Start: r.Start + delta,
		End:   r.End + delta,
	}
}

// PointRange represents a range using line/column positions.
type PointRange struct {
	Start Point // Inclusive start position
	End   Point // Exclusive end position
}

// NewPointRange creates a new PointRange from start and end points.
func NewPointRange(start, end Point) PointRange {
	return PointRange{Start: start, End: end}
}

// String returns a human-readable representation of the range.
func (r PointRange) String() string {
	return fmt.Sprintf("[%s:%s)", r.Start.String(), r.End.String())
}

// IsEmpty returns true if start equals end.
func (r PointRange) IsEmpty() bool {
	return r.Start.Compare(r.End) == 0
}

// IsValid returns true if start <= end.
func (r PointRange) IsValid() bool {
	return r.Start.Compare(r.End) <= 0
}

// Contains returns true if the given point is within the range.
func (r PointRange) Contains(p Point) bool {
	return p.Compare(r.Start) >= 0 && p.Compare(r.End) < 0
}

// IsSingleLine returns true if the range spans only one line.
func (r PointRange) IsSingleLine() bool {
	return r.Start.Line == r.End.Line
}

// PointRangeUTF16 represents a range using line/UTF-16 column positions.
// This is used for LSP compatibility.
type PointRangeUTF16 struct {
	Start PointUTF16 // Inclusive start position
	End   PointUTF16 // Exclusive end position
}

// NewPointRangeUTF16 creates a new PointRangeUTF16 from start and end points.
func NewPointRangeUTF16(start, end PointUTF16) PointRangeUTF16 {
	return PointRangeUTF16{Start: start, End: end}
}

// String returns a human-readable representation of the range.
func (r PointRangeUTF16) String() string {
	return fmt.Sprintf("[%s:%s)", r.Start.String(), r.End.String())
}

// IsEmpty returns true if start equals end.
func (r PointRangeUTF16) IsEmpty() bool {
	return r.Start.Compare(r.End) == 0
}

// IsValid returns true if start <= end.
func (r PointRangeUTF16) IsValid() bool {
	return r.Start.Compare(r.End) <= 0
}

// IsSingleLine returns true if the range spans only one line.
func (r PointRangeUTF16) IsSingleLine() bool {
	return r.Start.Line == r.End.Line
}
