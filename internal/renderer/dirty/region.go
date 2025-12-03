// Package dirty provides efficient dirty region tracking for incremental rendering.
// It tracks which parts of the display need to be redrawn and coalesces
// adjacent or overlapping regions for optimal update performance.
package dirty

// Region represents a rectangular region of the display that needs redrawing.
type Region struct {
	// StartLine is the first line of the region (inclusive).
	StartLine uint32

	// EndLine is the last line of the region (inclusive).
	EndLine uint32

	// StartCol is the first column of the region (inclusive).
	// Ignored when FullWidth is true.
	StartCol uint32

	// EndCol is the last column of the region (exclusive).
	// Ignored when FullWidth is true.
	EndCol uint32

	// FullWidth indicates the region spans the entire width.
	// When true, StartCol and EndCol are ignored.
	FullWidth bool
}

// NewLineRegion creates a region covering full lines.
func NewLineRegion(startLine, endLine uint32) Region {
	if endLine < startLine {
		startLine, endLine = endLine, startLine
	}
	return Region{
		StartLine: startLine,
		EndLine:   endLine,
		FullWidth: true,
	}
}

// NewSingleLine creates a region for a single full line.
func NewSingleLine(line uint32) Region {
	return Region{
		StartLine: line,
		EndLine:   line,
		FullWidth: true,
	}
}

// NewColumnRegion creates a region covering a specific column range.
func NewColumnRegion(line, startCol, endCol uint32) Region {
	if endCol < startCol {
		startCol, endCol = endCol, startCol
	}
	return Region{
		StartLine: line,
		EndLine:   line,
		StartCol:  startCol,
		EndCol:    endCol,
		FullWidth: false,
	}
}

// NewRectRegion creates a rectangular region.
func NewRectRegion(startLine, endLine, startCol, endCol uint32) Region {
	if endLine < startLine {
		startLine, endLine = endLine, startLine
	}
	if endCol < startCol {
		startCol, endCol = endCol, startCol
	}
	return Region{
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
		FullWidth: false,
	}
}

// IsEmpty returns true if the region covers no area.
func (r Region) IsEmpty() bool {
	if r.StartLine > r.EndLine {
		return true
	}
	if !r.FullWidth && r.StartCol >= r.EndCol {
		return true
	}
	return false
}

// LineCount returns the number of lines covered by the region.
func (r Region) LineCount() uint32 {
	if r.StartLine > r.EndLine {
		return 0
	}
	return r.EndLine - r.StartLine + 1
}

// ContainsLine returns true if the region covers the given line.
func (r Region) ContainsLine(line uint32) bool {
	return line >= r.StartLine && line <= r.EndLine
}

// Contains returns true if the region contains the given position.
func (r Region) Contains(line, col uint32) bool {
	if !r.ContainsLine(line) {
		return false
	}
	if r.FullWidth {
		return true
	}
	return col >= r.StartCol && col < r.EndCol
}

// Overlaps returns true if two regions overlap.
func (r Region) Overlaps(other Region) bool {
	// Check line overlap
	if r.EndLine < other.StartLine || r.StartLine > other.EndLine {
		return false
	}

	// If either is full width, they overlap if lines overlap
	if r.FullWidth || other.FullWidth {
		return true
	}

	// Check column overlap
	if r.EndCol <= other.StartCol || r.StartCol >= other.EndCol {
		return false
	}

	return true
}

// Adjacent returns true if two regions are adjacent (can be merged).
func (r Region) Adjacent(other Region) bool {
	// Check if regions are vertically adjacent
	// Guard against uint32 overflow by checking value before adding
	if (r.EndLine < ^uint32(0) && r.EndLine+1 == other.StartLine) ||
		(other.EndLine < ^uint32(0) && other.EndLine+1 == r.StartLine) {
		// For full-width regions, vertical adjacency is enough
		if r.FullWidth && other.FullWidth {
			return true
		}
		// For column regions, columns must match
		if !r.FullWidth && !other.FullWidth {
			return r.StartCol == other.StartCol && r.EndCol == other.EndCol
		}
		// Mixed full-width and column regions on adjacent lines are not mergeable
		return false
	}

	// Check if regions overlap on the same lines (can merge columns)
	// Only check horizontal adjacency for non-full-width regions
	if !r.FullWidth && !other.FullWidth {
		if r.StartLine <= other.EndLine && r.EndLine >= other.StartLine {
			// Horizontally adjacent
			if r.EndCol == other.StartCol || other.EndCol == r.StartCol {
				return true
			}
		}
	}

	return false
}

// Merge combines two regions into a single region that covers both.
// Returns the merged region and true if merging is beneficial.
func (r Region) Merge(other Region) (Region, bool) {
	if !r.Overlaps(other) && !r.Adjacent(other) {
		return Region{}, false
	}

	merged := Region{
		StartLine: min(r.StartLine, other.StartLine),
		EndLine:   max(r.EndLine, other.EndLine),
	}

	// If either is full width, result is full width
	if r.FullWidth || other.FullWidth {
		merged.FullWidth = true
	} else {
		merged.StartCol = min(r.StartCol, other.StartCol)
		merged.EndCol = max(r.EndCol, other.EndCol)
	}

	return merged, true
}

// Expand extends the region to include the given position.
func (r Region) Expand(line, col uint32) Region {
	result := r

	if line < result.StartLine {
		result.StartLine = line
	}
	if line > result.EndLine {
		result.EndLine = line
	}

	if !result.FullWidth {
		if col < result.StartCol {
			result.StartCol = col
		}
		if col >= result.EndCol {
			result.EndCol = col + 1
		}
	}

	return result
}

// ExpandLines extends the region to include the given line range.
func (r Region) ExpandLines(startLine, endLine uint32) Region {
	result := r

	if startLine < result.StartLine {
		result.StartLine = startLine
	}
	if endLine > result.EndLine {
		result.EndLine = endLine
	}

	return result
}

// Intersect returns the intersection of two regions.
// Returns an empty region if they don't overlap.
func (r Region) Intersect(other Region) Region {
	if !r.Overlaps(other) {
		// Return a region that satisfies IsEmpty() (StartLine > EndLine)
		return Region{StartLine: 1, EndLine: 0}
	}

	result := Region{
		StartLine: max(r.StartLine, other.StartLine),
		EndLine:   min(r.EndLine, other.EndLine),
	}

	if r.FullWidth && other.FullWidth {
		result.FullWidth = true
	} else if r.FullWidth {
		result.StartCol = other.StartCol
		result.EndCol = other.EndCol
	} else if other.FullWidth {
		result.StartCol = r.StartCol
		result.EndCol = r.EndCol
	} else {
		result.StartCol = max(r.StartCol, other.StartCol)
		result.EndCol = min(r.EndCol, other.EndCol)
	}

	return result
}

// Equals returns true if two regions are identical.
func (r Region) Equals(other Region) bool {
	return r.StartLine == other.StartLine &&
		r.EndLine == other.EndLine &&
		r.StartCol == other.StartCol &&
		r.EndCol == other.EndCol &&
		r.FullWidth == other.FullWidth
}

// min returns the smaller of two uint32 values.
func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// max returns the larger of two uint32 values.
func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}
