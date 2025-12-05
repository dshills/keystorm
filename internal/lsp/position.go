package lsp

import (
	"unicode/utf16"
)

// PositionConverter handles conversions between different position representations.
// LSP uses 0-based line/column positions with UTF-16 code units for columns.
// This converter handles the translation between byte offsets, rune offsets,
// and LSP positions.
type PositionConverter struct {
	content string
	lines   []lineInfo
}

// lineInfo stores information about a line for efficient position conversion.
type lineInfo struct {
	byteOffset int // Byte offset of line start
	runeOffset int // Rune offset of line start
	byteLen    int // Length in bytes
	runeLen    int // Length in runes
	utf16Len   int // Length in UTF-16 code units
}

// NewPositionConverter creates a new converter for the given content.
func NewPositionConverter(content string) *PositionConverter {
	pc := &PositionConverter{
		content: content,
	}
	pc.buildLineIndex()
	return pc
}

// buildLineIndex creates an index of all lines for fast position lookup.
func (pc *PositionConverter) buildLineIndex() {
	pc.lines = nil

	runeOffset := 0
	lineStart := 0
	runeLineStart := 0

	for i, r := range pc.content {
		if r == '\n' {
			lineByteLen := i - lineStart
			lineRuneLen := runeOffset - runeLineStart
			utf16Len := utf16LenForString(pc.content[lineStart:i])

			pc.lines = append(pc.lines, lineInfo{
				byteOffset: lineStart,
				runeOffset: runeLineStart,
				byteLen:    lineByteLen,
				runeLen:    lineRuneLen,
				utf16Len:   utf16Len,
			})

			lineStart = i + 1
			runeLineStart = runeOffset + 1
		}
		runeOffset++
	}

	// Handle last line (may not end with newline)
	lineByteLen := len(pc.content) - lineStart
	lineRuneLen := runeOffset - runeLineStart
	var utf16Len int
	if lineStart < len(pc.content) {
		utf16Len = utf16LenForString(pc.content[lineStart:])
	}

	pc.lines = append(pc.lines, lineInfo{
		byteOffset: lineStart,
		runeOffset: runeLineStart,
		byteLen:    lineByteLen,
		runeLen:    lineRuneLen,
		utf16Len:   utf16Len,
	})
}

// ByteOffsetToPosition converts a byte offset to an LSP Position.
func (pc *PositionConverter) ByteOffsetToPosition(byteOffset int) Position {
	if byteOffset < 0 {
		return Position{Line: 0, Character: 0}
	}

	// Find the line containing this offset
	lineNum := 0
	for i, line := range pc.lines {
		if byteOffset < line.byteOffset+line.byteLen+1 { // +1 for newline
			lineNum = i
			break
		}
		if i == len(pc.lines)-1 {
			lineNum = i
		}
	}

	line := pc.lines[lineNum]

	// Calculate character within line (UTF-16 offset)
	charOffset := byteOffset - line.byteOffset
	if charOffset < 0 {
		charOffset = 0
	}
	if charOffset > line.byteLen {
		charOffset = line.byteLen
	}

	// Convert byte offset within line to UTF-16 offset
	lineContent := pc.content[line.byteOffset : line.byteOffset+line.byteLen]
	utf16Char := byteToUTF16Offset(lineContent, charOffset)

	return Position{
		Line:      lineNum,
		Character: utf16Char,
	}
}

// PositionToByteOffset converts an LSP Position to a byte offset.
func (pc *PositionConverter) PositionToByteOffset(pos Position) int {
	if pos.Line < 0 {
		return 0
	}
	if pos.Line >= len(pc.lines) {
		return len(pc.content)
	}

	line := pc.lines[pos.Line]

	// Convert UTF-16 character offset to byte offset within line
	lineContent := pc.content[line.byteOffset : line.byteOffset+line.byteLen]
	byteChar := utf16ToByteOffset(lineContent, pos.Character)

	return line.byteOffset + byteChar
}

// RuneOffsetToPosition converts a rune offset to an LSP Position.
func (pc *PositionConverter) RuneOffsetToPosition(runeOffset int) Position {
	if runeOffset < 0 {
		return Position{Line: 0, Character: 0}
	}

	// Find the line containing this rune offset
	lineNum := 0
	for i, line := range pc.lines {
		nextLineRune := line.runeOffset + line.runeLen + 1 // +1 for newline
		if i == len(pc.lines)-1 {
			nextLineRune = line.runeOffset + line.runeLen
		}
		if runeOffset < nextLineRune {
			lineNum = i
			break
		}
		if i == len(pc.lines)-1 {
			lineNum = i
		}
	}

	line := pc.lines[lineNum]

	// Calculate rune offset within line
	runeInLine := runeOffset - line.runeOffset
	if runeInLine < 0 {
		runeInLine = 0
	}
	if runeInLine > line.runeLen {
		runeInLine = line.runeLen
	}

	// Convert rune offset to UTF-16 offset
	lineContent := pc.content[line.byteOffset : line.byteOffset+line.byteLen]
	utf16Char := runeToUTF16Offset(lineContent, runeInLine)

	return Position{
		Line:      lineNum,
		Character: utf16Char,
	}
}

// PositionToRuneOffset converts an LSP Position to a rune offset.
func (pc *PositionConverter) PositionToRuneOffset(pos Position) int {
	if pos.Line < 0 {
		return 0
	}
	if pos.Line >= len(pc.lines) {
		// Return total rune count
		total := 0
		for _, line := range pc.lines {
			total += line.runeLen
			if line.byteOffset+line.byteLen < len(pc.content) {
				total++ // newline
			}
		}
		return total
	}

	line := pc.lines[pos.Line]

	// Convert UTF-16 offset to rune offset within line
	lineContent := pc.content[line.byteOffset : line.byteOffset+line.byteLen]
	runeChar := utf16ToRuneOffset(lineContent, pos.Character)

	return line.runeOffset + runeChar
}

// RangeToByteOffsets converts an LSP Range to start and end byte offsets.
func (pc *PositionConverter) RangeToByteOffsets(rng Range) (start, end int) {
	start = pc.PositionToByteOffset(rng.Start)
	end = pc.PositionToByteOffset(rng.End)
	return
}

// ByteOffsetsToRange converts start and end byte offsets to an LSP Range.
func (pc *PositionConverter) ByteOffsetsToRange(start, end int) Range {
	return Range{
		Start: pc.ByteOffsetToPosition(start),
		End:   pc.ByteOffsetToPosition(end),
	}
}

// LineCount returns the number of lines.
func (pc *PositionConverter) LineCount() int {
	return len(pc.lines)
}

// LineByteRange returns the byte range for a line (excluding newline).
func (pc *PositionConverter) LineByteRange(lineNum int) (start, end int) {
	if lineNum < 0 || lineNum >= len(pc.lines) {
		return 0, 0
	}
	line := pc.lines[lineNum]
	return line.byteOffset, line.byteOffset + line.byteLen
}

// LineContent returns the content of a line (excluding newline).
func (pc *PositionConverter) LineContent(lineNum int) string {
	if lineNum < 0 || lineNum >= len(pc.lines) {
		return ""
	}
	line := pc.lines[lineNum]
	return pc.content[line.byteOffset : line.byteOffset+line.byteLen]
}

// --- UTF-16 conversion helpers ---

// utf16LenForString returns the length in UTF-16 code units.
func utf16LenForString(s string) int {
	count := 0
	for _, r := range s {
		if r >= 0x10000 {
			count += 2 // Surrogate pair
		} else {
			count++
		}
	}
	return count
}

// byteToUTF16Offset converts a byte offset within a string to UTF-16 offset.
func byteToUTF16Offset(s string, byteOff int) int {
	if byteOff <= 0 {
		return 0
	}
	if byteOff >= len(s) {
		return utf16LenForString(s)
	}

	utf16Off := 0
	for i, r := range s {
		if i >= byteOff {
			break
		}
		if r >= 0x10000 {
			utf16Off += 2
		} else {
			utf16Off++
		}
	}
	return utf16Off
}

// utf16ToByteOffset converts a UTF-16 offset to byte offset within a string.
func utf16ToByteOffset(s string, utf16Off int) int {
	if utf16Off <= 0 {
		return 0
	}

	utf16Count := 0
	for i, r := range s {
		if utf16Count >= utf16Off {
			return i
		}
		if r >= 0x10000 {
			utf16Count += 2
		} else {
			utf16Count++
		}
	}
	return len(s)
}

// runeToUTF16Offset converts a rune offset to UTF-16 offset within a string.
func runeToUTF16Offset(s string, runeOff int) int {
	if runeOff <= 0 {
		return 0
	}

	runeCount := 0
	utf16Off := 0
	for _, r := range s {
		if runeCount >= runeOff {
			break
		}
		if r >= 0x10000 {
			utf16Off += 2
		} else {
			utf16Off++
		}
		runeCount++
	}
	return utf16Off
}

// utf16ToRuneOffset converts a UTF-16 offset to rune offset within a string.
func utf16ToRuneOffset(s string, utf16Off int) int {
	if utf16Off <= 0 {
		return 0
	}

	utf16Count := 0
	runeOff := 0
	for _, r := range s {
		if utf16Count >= utf16Off {
			break
		}
		if r >= 0x10000 {
			utf16Count += 2
		} else {
			utf16Count++
		}
		runeOff++
	}
	return runeOff
}

// --- Standalone conversion functions ---

// ByteOffsetToLSPPosition converts a byte offset in content to an LSP Position.
// This is a convenience function that creates a temporary converter.
func ByteOffsetToLSPPosition(content string, byteOffset int) Position {
	pc := NewPositionConverter(content)
	return pc.ByteOffsetToPosition(byteOffset)
}

// LSPPositionToByteOffset converts an LSP Position to a byte offset.
// This is a convenience function that creates a temporary converter.
func LSPPositionToByteOffset(content string, pos Position) int {
	pc := NewPositionConverter(content)
	return pc.PositionToByteOffset(pos)
}

// RuneOffsetToLSPPosition converts a rune offset to an LSP Position.
// This is a convenience function that creates a temporary converter.
func RuneOffsetToLSPPosition(content string, runeOffset int) Position {
	pc := NewPositionConverter(content)
	return pc.RuneOffsetToPosition(runeOffset)
}

// LSPPositionToRuneOffset converts an LSP Position to a rune offset.
// This is a convenience function that creates a temporary converter.
func LSPPositionToRuneOffset(content string, pos Position) int {
	pc := NewPositionConverter(content)
	return pc.PositionToRuneOffset(pos)
}

// IsPositionBefore returns true if a is before b.
func IsPositionBefore(a, b Position) bool {
	if a.Line < b.Line {
		return true
	}
	if a.Line > b.Line {
		return false
	}
	return a.Character < b.Character
}

// IsPositionAfter returns true if a is after b.
func IsPositionAfter(a, b Position) bool {
	return IsPositionBefore(b, a)
}

// IsPositionEqual returns true if a and b are equal.
func IsPositionEqual(a, b Position) bool {
	return a.Line == b.Line && a.Character == b.Character
}

// IsPositionInRange returns true if pos is within the range (inclusive).
func IsPositionInRange(pos Position, rng Range) bool {
	if IsPositionBefore(pos, rng.Start) {
		return false
	}
	if IsPositionAfter(pos, rng.End) {
		return false
	}
	return true
}

// ComparePositions returns -1 if a < b, 0 if a == b, 1 if a > b.
func ComparePositions(a, b Position) int {
	if a.Line < b.Line {
		return -1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Character < b.Character {
		return -1
	}
	if a.Character > b.Character {
		return 1
	}
	return 0
}

// RangesOverlap returns true if two ranges overlap.
func RangesOverlap(a, b Range) bool {
	// a ends before b starts
	if IsPositionBefore(a.End, b.Start) || IsPositionEqual(a.End, b.Start) {
		return false
	}
	// b ends before a starts
	if IsPositionBefore(b.End, a.Start) || IsPositionEqual(b.End, a.Start) {
		return false
	}
	return true
}

// RangeContains returns true if outer contains inner.
func RangeContains(outer, inner Range) bool {
	return !IsPositionBefore(inner.Start, outer.Start) &&
		!IsPositionAfter(inner.End, outer.End)
}

// ExpandRange creates a range that contains both input ranges.
func ExpandRange(a, b Range) Range {
	start := a.Start
	if IsPositionBefore(b.Start, a.Start) {
		start = b.Start
	}

	end := a.End
	if IsPositionAfter(b.End, a.End) {
		end = b.End
	}

	return Range{Start: start, End: end}
}

// EncodeUTF16 encodes a string to UTF-16.
func EncodeUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// DecodeUTF16 decodes UTF-16 to a string.
func DecodeUTF16(u []uint16) string {
	return string(utf16.Decode(u))
}
