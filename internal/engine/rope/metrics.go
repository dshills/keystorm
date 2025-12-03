package rope

import "unicode/utf8"

// ByteOffset represents an absolute byte position in the rope.
type ByteOffset uint64

// Point represents a line/column position.
// Line and Column are both 0-indexed.
type Point struct {
	Line   uint32
	Column uint32
}

// TextSummary holds aggregated metrics for a text span.
// This is the "summary" type for our SumTree, implementing monoid operations.
type TextSummary struct {
	// Bytes is the UTF-8 byte count.
	Bytes ByteOffset

	// UTF16Units is the UTF-16 code unit count (for LSP compatibility).
	UTF16Units uint64

	// Lines is the number of newline characters.
	Lines uint32

	// LongestLine is the byte length of the longest line.
	LongestLine uint32

	// FirstLineLen is the byte length of the first line (excluding newline).
	FirstLineLen uint32

	// LastLineLen is the byte length of the last line (excluding newline).
	LastLineLen uint32

	// Flags indicate text properties for fast paths.
	Flags TextFlags
}

// TextFlags indicate text properties for optimization fast paths.
type TextFlags uint8

const (
	// FlagASCII indicates all characters are ASCII (< 128).
	FlagASCII TextFlags = 1 << iota

	// FlagHasNewlines indicates the text contains newline characters.
	FlagHasNewlines

	// FlagHasTabs indicates the text contains tab characters.
	FlagHasTabs
)

// Add combines two summaries (monoid operation).
// This is called when concatenating rope sections.
func (s TextSummary) Add(other TextSummary) TextSummary {
	if s.Bytes == 0 {
		return other
	}
	if other.Bytes == 0 {
		return s
	}

	result := TextSummary{
		Bytes:      s.Bytes + other.Bytes,
		UTF16Units: s.UTF16Units + other.UTF16Units,
		Lines:      s.Lines + other.Lines,
		Flags:      s.Flags & other.Flags, // AND for flags (all must have property)
	}

	// Update line length tracking
	if other.Lines > 0 {
		// Other has newlines, so longest line could be from either
		result.LongestLine = max(s.LongestLine, other.LongestLine)
		result.FirstLineLen = s.FirstLineLen
		result.LastLineLen = other.LastLineLen
	} else {
		// Other has no newlines, extends last line of s
		combined := s.LastLineLen + other.LastLineLen
		result.LongestLine = max(s.LongestLine, combined)
		if s.Lines == 0 {
			result.FirstLineLen = combined
		} else {
			result.FirstLineLen = s.FirstLineLen
		}
		result.LastLineLen = combined
	}

	// Combine flags properly
	if s.Flags&FlagHasNewlines != 0 || other.Flags&FlagHasNewlines != 0 {
		result.Flags |= FlagHasNewlines
	}
	if s.Flags&FlagHasTabs != 0 || other.Flags&FlagHasTabs != 0 {
		result.Flags |= FlagHasTabs
	}

	return result
}

// Zero returns the identity element for the summary monoid.
func (TextSummary) Zero() TextSummary {
	return TextSummary{Flags: FlagASCII}
}

// IsZero returns true if this is the zero/identity summary.
func (s TextSummary) IsZero() bool {
	return s.Bytes == 0
}

// ComputeSummary calculates metrics for a string.
func ComputeSummary(s string) TextSummary {
	if len(s) == 0 {
		return TextSummary{Flags: FlagASCII}
	}

	var sum TextSummary
	sum.Bytes = ByteOffset(len(s))
	sum.Flags = FlagASCII // Start optimistic

	var lineLen uint32

	for _, r := range s {
		// UTF-16 code units
		if r <= 0xFFFF {
			sum.UTF16Units++
		} else {
			sum.UTF16Units += 2 // Surrogate pair
		}

		// ASCII check
		if r > 127 {
			sum.Flags &^= FlagASCII
		}

		// Line counting
		if r == '\n' {
			sum.Lines++
			if lineLen > sum.LongestLine {
				sum.LongestLine = lineLen
			}
			if sum.Lines == 1 {
				sum.FirstLineLen = lineLen
			}
			lineLen = 0
			sum.Flags |= FlagHasNewlines
		} else {
			lineLen += uint32(utf8.RuneLen(r))
			if r == '\t' {
				sum.Flags |= FlagHasTabs
			}
		}
	}

	// Handle last line
	sum.LastLineLen = lineLen
	if sum.Lines == 0 {
		sum.FirstLineLen = lineLen
		sum.LongestLine = lineLen
	} else if lineLen > sum.LongestLine {
		sum.LongestLine = lineLen
	}

	return sum
}

// CountLines returns the number of newlines in a string.
func CountLines(s string) uint32 {
	var count uint32
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}

// FindNthNewline finds the byte position of the nth newline (1-indexed).
// Returns -1 if not found.
func FindNthNewline(s string, n uint32) int {
	if n == 0 {
		return -1
	}

	var count uint32
	for i, c := range s {
		if c == '\n' {
			count++
			if count == n {
				return i
			}
		}
	}
	return -1
}

// OffsetToLineColumn converts a byte offset to line/column within a string.
func OffsetToLineColumn(s string, offset int) Point {
	if offset <= 0 {
		return Point{Line: 0, Column: 0}
	}
	if offset >= len(s) {
		offset = len(s)
	}

	var line uint32
	var lastNewline int = -1

	for i, c := range s[:offset] {
		if c == '\n' {
			line++
			lastNewline = i
		}
	}

	return Point{
		Line:   line,
		Column: uint32(offset - lastNewline - 1),
	}
}
