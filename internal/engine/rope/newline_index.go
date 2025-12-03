package rope

// NewlineIndex provides fast O(1) lookup of newline positions within a chunk.
// It uses a compact representation inspired by Zed's u128 optimization:
// - For chunks with <= 4 newlines: inline small array (no allocation)
// - For chunks with > 4 newlines: heap-allocated slice
//
// This optimization is critical for fast line navigation in large files.
//
// Type sizing rationale:
//   - uint16 for positions: MaxChunkSize is 256 bytes, so positions fit in uint8,
//     but uint16 provides headroom and better alignment
//   - uint8 for count: A 256-byte chunk can have at most 256 newlines
type NewlineIndex struct {
	// For small counts (common case), store positions inline
	inline [4]uint16 // Up to 4 positions (0-65535 byte offset in chunk)
	count  uint8     // Number of newlines (0-255, sufficient for MaxChunkSize)

	// For larger counts, use heap-allocated slice
	positions []uint16 // Only allocated when count > 4
}

// MaxInlineNewlines is the number of newline positions stored inline.
const MaxInlineNewlines = 4

// ComputeNewlineIndex scans a string and builds a newline index.
func ComputeNewlineIndex(s string) NewlineIndex {
	var idx NewlineIndex

	// First pass: count newlines
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			count++
		}
	}

	if count == 0 {
		return idx
	}

	// Clamp count to uint8 range (shouldn't happen with proper chunk sizes)
	if count > 255 {
		count = 255
	}
	idx.count = uint8(count)

	// Allocate if needed
	if count > MaxInlineNewlines {
		idx.positions = make([]uint16, 0, count)
	}

	// Second pass: record positions
	recorded := 0
	for i := 0; i < len(s) && recorded < count; i++ {
		if s[i] == '\n' {
			pos := uint16(i)
			if recorded < MaxInlineNewlines {
				idx.inline[recorded] = pos
			}
			if count > MaxInlineNewlines {
				idx.positions = append(idx.positions, pos)
			}
			recorded++
		}
	}

	return idx
}

// Count returns the number of newlines.
func (idx *NewlineIndex) Count() uint32 {
	return uint32(idx.count)
}

// Position returns the byte offset of the nth newline (0-indexed).
// Returns -1 if n is out of range.
func (idx *NewlineIndex) Position(n uint32) int {
	if n >= uint32(idx.count) {
		return -1
	}

	if idx.count <= MaxInlineNewlines {
		return int(idx.inline[n])
	}

	return int(idx.positions[n])
}

// FindNthNewline returns the byte position of the nth newline (1-indexed).
// This matches the convention used elsewhere in the rope package.
// Returns -1 if n is 0 or not found.
func (idx *NewlineIndex) FindNthNewline(n uint32) int {
	if n == 0 || n > uint32(idx.count) {
		return -1
	}
	return idx.Position(n - 1)
}

// SearchLine returns the byte offset of the start of line `line` within this chunk.
// Line 0 returns 0 (start of chunk).
// For line > 0, returns the position after the (line)th newline, or -1 if not found.
//
// Example: For text "abc\ndef\nghi", newlines are at positions 3 and 7.
//   - SearchLine(0) returns 0 (start of chunk/line 0)
//   - SearchLine(1) returns 4 (start of line 1, after first newline)
//   - SearchLine(2) returns 8 (start of line 2, after second newline)
func (idx *NewlineIndex) SearchLine(line uint32) int {
	if line == 0 {
		return 0 // Line 0 starts at byte 0
	}

	pos := idx.FindNthNewline(line)
	if pos < 0 {
		return -1
	}
	return pos + 1 // Return position after the newline
}

// Contains returns true if the index has at least `lines` newlines.
// Used for quickly checking if a chunk contains a target line.
func (idx *NewlineIndex) Contains(lines uint32) bool {
	return uint32(idx.count) >= lines
}

// LastNewlinePosition returns the position of the last newline, or -1 if none.
func (idx *NewlineIndex) LastNewlinePosition() int {
	if idx.count == 0 {
		return -1
	}
	return idx.Position(uint32(idx.count) - 1)
}

// NewlineBefore returns the position of the last newline before the given offset.
// Returns -1 if no newline exists before that offset.
func (idx *NewlineIndex) NewlineBefore(offset int) int {
	if idx.count == 0 {
		return -1
	}

	// Binary search for the largest newline position < offset
	positions := idx.allPositions()

	// Linear search for small counts (usually faster due to cache)
	if len(positions) <= 8 {
		for i := len(positions) - 1; i >= 0; i-- {
			if int(positions[i]) < offset {
				return int(positions[i])
			}
		}
		return -1
	}

	// Binary search for larger counts
	lo, hi := 0, len(positions)-1
	result := -1

	for lo <= hi {
		mid := (lo + hi) / 2
		pos := int(positions[mid])
		if pos < offset {
			result = pos
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	return result
}

// NewlineAfter returns the position of the first newline at or after the given offset.
// Returns -1 if no newline exists at or after that offset.
func (idx *NewlineIndex) NewlineAfter(offset int) int {
	if idx.count == 0 {
		return -1
	}

	positions := idx.allPositions()

	// Linear search for small counts
	if len(positions) <= 8 {
		for _, pos := range positions {
			if int(pos) >= offset {
				return int(pos)
			}
		}
		return -1
	}

	// Binary search for larger counts
	lo, hi := 0, len(positions)-1
	result := -1

	for lo <= hi {
		mid := (lo + hi) / 2
		pos := int(positions[mid])
		if pos >= offset {
			result = pos
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}

	return result
}

// allPositions returns a slice of all positions.
// For inline storage, returns a slice of the inline array.
func (idx *NewlineIndex) allPositions() []uint16 {
	if idx.count <= MaxInlineNewlines {
		return idx.inline[:idx.count]
	}
	return idx.positions
}
