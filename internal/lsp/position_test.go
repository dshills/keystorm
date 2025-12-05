package lsp

import (
	"testing"
)

func TestNewPositionConverter(t *testing.T) {
	pc := NewPositionConverter("hello\nworld")
	if pc == nil {
		t.Fatal("NewPositionConverter returned nil")
	}

	if pc.LineCount() != 2 {
		t.Errorf("Expected 2 lines, got %d", pc.LineCount())
	}
}

func TestPositionConverter_EmptyContent(t *testing.T) {
	pc := NewPositionConverter("")
	if pc.LineCount() != 1 {
		t.Errorf("Expected 1 line for empty content, got %d", pc.LineCount())
	}
}

func TestPositionConverter_SingleLine(t *testing.T) {
	pc := NewPositionConverter("hello")

	pos := pc.ByteOffsetToPosition(0)
	if pos.Line != 0 || pos.Character != 0 {
		t.Errorf("Expected (0,0), got (%d,%d)", pos.Line, pos.Character)
	}

	pos = pc.ByteOffsetToPosition(5)
	if pos.Line != 0 || pos.Character != 5 {
		t.Errorf("Expected (0,5), got (%d,%d)", pos.Line, pos.Character)
	}
}

func TestPositionConverter_MultiLine(t *testing.T) {
	pc := NewPositionConverter("line1\nline2\nline3")

	tests := []struct {
		byteOffset int
		line       int
		char       int
	}{
		{0, 0, 0},  // Start of line1
		{5, 0, 5},  // End of line1
		{6, 1, 0},  // Start of line2
		{11, 1, 5}, // End of line2
		{12, 2, 0}, // Start of line3
		{17, 2, 5}, // End of line3
	}

	for _, tt := range tests {
		pos := pc.ByteOffsetToPosition(tt.byteOffset)
		if pos.Line != tt.line || pos.Character != tt.char {
			t.Errorf("ByteOffset %d: expected (%d,%d), got (%d,%d)",
				tt.byteOffset, tt.line, tt.char, pos.Line, pos.Character)
		}
	}
}

func TestPositionConverter_PositionToByteOffset(t *testing.T) {
	pc := NewPositionConverter("line1\nline2\nline3")

	tests := []struct {
		line       int
		char       int
		byteOffset int
	}{
		{0, 0, 0},
		{0, 5, 5},
		{1, 0, 6},
		{1, 5, 11},
		{2, 0, 12},
		{2, 5, 17},
	}

	for _, tt := range tests {
		offset := pc.PositionToByteOffset(Position{Line: tt.line, Character: tt.char})
		if offset != tt.byteOffset {
			t.Errorf("Position (%d,%d): expected offset %d, got %d",
				tt.line, tt.char, tt.byteOffset, offset)
		}
	}
}

func TestPositionConverter_RoundTrip(t *testing.T) {
	content := "first line\nsecond line\nthird line"
	pc := NewPositionConverter(content)

	for i := 0; i <= len(content); i++ {
		pos := pc.ByteOffsetToPosition(i)
		offset := pc.PositionToByteOffset(pos)
		if offset != i && i < len(content) {
			t.Errorf("Round trip failed for offset %d: got %d (via pos %d,%d)",
				i, offset, pos.Line, pos.Character)
		}
	}
}

func TestPositionConverter_UTF16(t *testing.T) {
	// Test with emoji (4 bytes in UTF-8, 2 UTF-16 code units)
	content := "a😀b"
	pc := NewPositionConverter(content)

	// 'a' is at byte 0, UTF-16 offset 0
	// '😀' is at byte 1, UTF-16 offset 1 (takes 2 UTF-16 code units)
	// 'b' is at byte 5, UTF-16 offset 3

	pos := pc.ByteOffsetToPosition(0)
	if pos.Character != 0 {
		t.Errorf("Expected UTF-16 char 0 for byte 0, got %d", pos.Character)
	}

	pos = pc.ByteOffsetToPosition(1)
	if pos.Character != 1 {
		t.Errorf("Expected UTF-16 char 1 for byte 1, got %d", pos.Character)
	}

	pos = pc.ByteOffsetToPosition(5)
	if pos.Character != 3 {
		t.Errorf("Expected UTF-16 char 3 for byte 5, got %d", pos.Character)
	}
}

func TestPositionConverter_RuneOffset(t *testing.T) {
	content := "hello\nworld"
	pc := NewPositionConverter(content)

	tests := []struct {
		runeOffset int
		line       int
		char       int
	}{
		{0, 0, 0},
		{5, 0, 5},
		{6, 1, 0},
		{11, 1, 5},
	}

	for _, tt := range tests {
		pos := pc.RuneOffsetToPosition(tt.runeOffset)
		if pos.Line != tt.line || pos.Character != tt.char {
			t.Errorf("RuneOffset %d: expected (%d,%d), got (%d,%d)",
				tt.runeOffset, tt.line, tt.char, pos.Line, pos.Character)
		}
	}
}

func TestPositionConverter_LineContent(t *testing.T) {
	pc := NewPositionConverter("first\nsecond\nthird")

	tests := []struct {
		line    int
		content string
	}{
		{0, "first"},
		{1, "second"},
		{2, "third"},
		{-1, ""},
		{10, ""},
	}

	for _, tt := range tests {
		content := pc.LineContent(tt.line)
		if content != tt.content {
			t.Errorf("Line %d: expected %q, got %q", tt.line, tt.content, content)
		}
	}
}

func TestPositionConverter_LineByteRange(t *testing.T) {
	pc := NewPositionConverter("abc\ndefgh\ni")

	tests := []struct {
		line       int
		start, end int
	}{
		{0, 0, 3},
		{1, 4, 9},
		{2, 10, 11},
	}

	for _, tt := range tests {
		start, end := pc.LineByteRange(tt.line)
		if start != tt.start || end != tt.end {
			t.Errorf("Line %d: expected range [%d,%d), got [%d,%d)",
				tt.line, tt.start, tt.end, start, end)
		}
	}
}

func TestPositionConverter_RangeToByteOffsets(t *testing.T) {
	pc := NewPositionConverter("hello\nworld")

	rng := Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 0, Character: 5},
	}

	start, end := pc.RangeToByteOffsets(rng)
	if start != 0 || end != 5 {
		t.Errorf("Expected [0,5), got [%d,%d)", start, end)
	}

	rng = Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 1, Character: 5},
	}

	start, end = pc.RangeToByteOffsets(rng)
	if start != 0 || end != 11 {
		t.Errorf("Expected [0,11), got [%d,%d)", start, end)
	}
}

func TestPositionConverter_ByteOffsetsToRange(t *testing.T) {
	pc := NewPositionConverter("hello\nworld")

	rng := pc.ByteOffsetsToRange(0, 5)
	if rng.Start.Line != 0 || rng.Start.Character != 0 {
		t.Errorf("Expected start (0,0), got (%d,%d)", rng.Start.Line, rng.Start.Character)
	}
	if rng.End.Line != 0 || rng.End.Character != 5 {
		t.Errorf("Expected end (0,5), got (%d,%d)", rng.End.Line, rng.End.Character)
	}
}

func TestPositionConverter_BoundaryConditions(t *testing.T) {
	pc := NewPositionConverter("hello")

	// Negative offset
	pos := pc.ByteOffsetToPosition(-10)
	if pos.Line != 0 || pos.Character != 0 {
		t.Errorf("Negative offset: expected (0,0), got (%d,%d)", pos.Line, pos.Character)
	}

	// Offset beyond content
	pos = pc.ByteOffsetToPosition(100)
	if pos.Line != 0 {
		t.Errorf("Beyond content: expected line 0, got %d", pos.Line)
	}

	// Negative line
	offset := pc.PositionToByteOffset(Position{Line: -1, Character: 0})
	if offset != 0 {
		t.Errorf("Negative line: expected offset 0, got %d", offset)
	}

	// Line beyond content
	offset = pc.PositionToByteOffset(Position{Line: 100, Character: 0})
	if offset != 5 {
		t.Errorf("Beyond line count: expected offset 5, got %d", offset)
	}
}

func TestIsPositionBefore(t *testing.T) {
	tests := []struct {
		a, b     Position
		expected bool
	}{
		{Position{0, 0}, Position{0, 1}, true},
		{Position{0, 1}, Position{0, 0}, false},
		{Position{0, 5}, Position{1, 0}, true},
		{Position{1, 0}, Position{0, 5}, false},
		{Position{0, 0}, Position{0, 0}, false},
	}

	for _, tt := range tests {
		result := IsPositionBefore(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("IsPositionBefore(%v, %v): expected %v, got %v",
				tt.a, tt.b, tt.expected, result)
		}
	}
}

func TestIsPositionAfter(t *testing.T) {
	tests := []struct {
		a, b     Position
		expected bool
	}{
		{Position{0, 1}, Position{0, 0}, true},
		{Position{0, 0}, Position{0, 1}, false},
		{Position{1, 0}, Position{0, 5}, true},
		{Position{0, 5}, Position{1, 0}, false},
		{Position{0, 0}, Position{0, 0}, false},
	}

	for _, tt := range tests {
		result := IsPositionAfter(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("IsPositionAfter(%v, %v): expected %v, got %v",
				tt.a, tt.b, tt.expected, result)
		}
	}
}

func TestIsPositionEqual(t *testing.T) {
	tests := []struct {
		a, b     Position
		expected bool
	}{
		{Position{0, 0}, Position{0, 0}, true},
		{Position{1, 5}, Position{1, 5}, true},
		{Position{0, 0}, Position{0, 1}, false},
		{Position{0, 0}, Position{1, 0}, false},
	}

	for _, tt := range tests {
		result := IsPositionEqual(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("IsPositionEqual(%v, %v): expected %v, got %v",
				tt.a, tt.b, tt.expected, result)
		}
	}
}

func TestIsPositionInRange(t *testing.T) {
	rng := Range{
		Start: Position{Line: 1, Character: 5},
		End:   Position{Line: 3, Character: 10},
	}

	tests := []struct {
		pos      Position
		expected bool
	}{
		{Position{0, 0}, false},  // Before range
		{Position{1, 0}, false},  // Same line but before
		{Position{1, 5}, true},   // At start
		{Position{2, 0}, true},   // Middle
		{Position{3, 10}, true},  // At end
		{Position{3, 11}, false}, // Same line but after
		{Position{4, 0}, false},  // After range
	}

	for _, tt := range tests {
		result := IsPositionInRange(tt.pos, rng)
		if result != tt.expected {
			t.Errorf("IsPositionInRange(%v, %v): expected %v, got %v",
				tt.pos, rng, tt.expected, result)
		}
	}
}

func TestComparePositions(t *testing.T) {
	tests := []struct {
		a, b     Position
		expected int
	}{
		{Position{0, 0}, Position{0, 0}, 0},
		{Position{0, 0}, Position{0, 1}, -1},
		{Position{0, 1}, Position{0, 0}, 1},
		{Position{0, 5}, Position{1, 0}, -1},
		{Position{1, 0}, Position{0, 5}, 1},
	}

	for _, tt := range tests {
		result := ComparePositions(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("ComparePositions(%v, %v): expected %d, got %d",
				tt.a, tt.b, tt.expected, result)
		}
	}
}

func TestRangesOverlap(t *testing.T) {
	tests := []struct {
		a, b     Range
		expected bool
	}{
		// No overlap - a before b
		{
			Range{Start: Position{0, 0}, End: Position{0, 5}},
			Range{Start: Position{0, 5}, End: Position{0, 10}},
			false,
		},
		// No overlap - b before a
		{
			Range{Start: Position{0, 5}, End: Position{0, 10}},
			Range{Start: Position{0, 0}, End: Position{0, 5}},
			false,
		},
		// Overlap
		{
			Range{Start: Position{0, 0}, End: Position{0, 7}},
			Range{Start: Position{0, 5}, End: Position{0, 10}},
			true,
		},
		// a contains b
		{
			Range{Start: Position{0, 0}, End: Position{0, 10}},
			Range{Start: Position{0, 2}, End: Position{0, 8}},
			true,
		},
		// Same range
		{
			Range{Start: Position{0, 0}, End: Position{0, 10}},
			Range{Start: Position{0, 0}, End: Position{0, 10}},
			true,
		},
	}

	for _, tt := range tests {
		result := RangesOverlap(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("RangesOverlap(%v, %v): expected %v, got %v",
				tt.a, tt.b, tt.expected, result)
		}
	}
}

func TestRangeContains(t *testing.T) {
	outer := Range{
		Start: Position{Line: 1, Character: 0},
		End:   Position{Line: 5, Character: 10},
	}

	tests := []struct {
		inner    Range
		expected bool
	}{
		// Inner fully contained
		{
			Range{Start: Position{2, 0}, End: Position{4, 5}},
			true,
		},
		// Inner equals outer
		{
			Range{Start: Position{1, 0}, End: Position{5, 10}},
			true,
		},
		// Inner starts before outer
		{
			Range{Start: Position{0, 0}, End: Position{3, 0}},
			false,
		},
		// Inner ends after outer
		{
			Range{Start: Position{2, 0}, End: Position{6, 0}},
			false,
		},
	}

	for _, tt := range tests {
		result := RangeContains(outer, tt.inner)
		if result != tt.expected {
			t.Errorf("RangeContains(%v, %v): expected %v, got %v",
				outer, tt.inner, tt.expected, result)
		}
	}
}

func TestExpandRange(t *testing.T) {
	a := Range{
		Start: Position{Line: 2, Character: 5},
		End:   Position{Line: 4, Character: 10},
	}
	b := Range{
		Start: Position{Line: 1, Character: 0},
		End:   Position{Line: 3, Character: 15},
	}

	result := ExpandRange(a, b)

	if result.Start.Line != 1 || result.Start.Character != 0 {
		t.Errorf("Expected start (1,0), got (%d,%d)", result.Start.Line, result.Start.Character)
	}
	if result.End.Line != 4 || result.End.Character != 10 {
		t.Errorf("Expected end (4,10), got (%d,%d)", result.End.Line, result.End.Character)
	}
}

func TestUTF16LenForString(t *testing.T) {
	tests := []struct {
		s        string
		expected int
	}{
		{"", 0},
		{"hello", 5},
		{"日本語", 3},          // 3 CJK characters, each 1 UTF-16 code unit
		{"😀", 2},            // Emoji is a surrogate pair (2 UTF-16 code units)
		{"a😀b", 4},          // 1 + 2 + 1
		{"hello😀world", 12}, // 5 + 2 + 5
	}

	for _, tt := range tests {
		result := utf16LenForString(tt.s)
		if result != tt.expected {
			t.Errorf("utf16LenForString(%q): expected %d, got %d", tt.s, tt.expected, result)
		}
	}
}

func TestByteToUTF16Offset(t *testing.T) {
	// "a😀b" - 'a' is 1 byte, '😀' is 4 bytes, 'b' is 1 byte
	s := "a😀b"

	tests := []struct {
		byteOff  int
		expected int
	}{
		{0, 0}, // Before 'a'
		{1, 1}, // After 'a', before emoji
		{5, 3}, // After emoji, before 'b'
		{6, 4}, // After 'b'
	}

	for _, tt := range tests {
		result := byteToUTF16Offset(s, tt.byteOff)
		if result != tt.expected {
			t.Errorf("byteToUTF16Offset(%q, %d): expected %d, got %d",
				s, tt.byteOff, tt.expected, result)
		}
	}
}

func TestUTF16ToByteOffset(t *testing.T) {
	// "a😀b" - 'a' is 1 byte, '😀' is 4 bytes, 'b' is 1 byte
	s := "a😀b"

	tests := []struct {
		utf16Off int
		expected int
	}{
		{0, 0}, // Before 'a'
		{1, 1}, // After 'a'
		{3, 5}, // After emoji (UTF-16 offset 3 = after 2 code units for emoji)
		{4, 6}, // After 'b'
	}

	for _, tt := range tests {
		result := utf16ToByteOffset(s, tt.utf16Off)
		if result != tt.expected {
			t.Errorf("utf16ToByteOffset(%q, %d): expected %d, got %d",
				s, tt.utf16Off, tt.expected, result)
		}
	}
}

func TestRuneToUTF16Offset(t *testing.T) {
	// "a😀b" - 3 runes, but emoji is 2 UTF-16 code units
	s := "a😀b"

	tests := []struct {
		runeOff  int
		expected int
	}{
		{0, 0}, // Before 'a'
		{1, 1}, // After 'a'
		{2, 3}, // After emoji (1 + 2)
		{3, 4}, // After 'b' (1 + 2 + 1)
	}

	for _, tt := range tests {
		result := runeToUTF16Offset(s, tt.runeOff)
		if result != tt.expected {
			t.Errorf("runeToUTF16Offset(%q, %d): expected %d, got %d",
				s, tt.runeOff, tt.expected, result)
		}
	}
}

func TestUTF16ToRuneOffset(t *testing.T) {
	// "a😀b" - 3 runes, but emoji is 2 UTF-16 code units
	s := "a😀b"

	tests := []struct {
		utf16Off int
		expected int
	}{
		{0, 0}, // Before 'a'
		{1, 1}, // After 'a'
		{3, 2}, // After emoji
		{4, 3}, // After 'b'
	}

	for _, tt := range tests {
		result := utf16ToRuneOffset(s, tt.utf16Off)
		if result != tt.expected {
			t.Errorf("utf16ToRuneOffset(%q, %d): expected %d, got %d",
				s, tt.utf16Off, tt.expected, result)
		}
	}
}

func TestStandaloneConversionFunctions(t *testing.T) {
	content := "hello\nworld"

	// ByteOffsetToLSPPosition
	pos := ByteOffsetToLSPPosition(content, 6)
	if pos.Line != 1 || pos.Character != 0 {
		t.Errorf("ByteOffsetToLSPPosition: expected (1,0), got (%d,%d)", pos.Line, pos.Character)
	}

	// LSPPositionToByteOffset
	offset := LSPPositionToByteOffset(content, Position{Line: 1, Character: 0})
	if offset != 6 {
		t.Errorf("LSPPositionToByteOffset: expected 6, got %d", offset)
	}

	// RuneOffsetToLSPPosition
	pos = RuneOffsetToLSPPosition(content, 6)
	if pos.Line != 1 || pos.Character != 0 {
		t.Errorf("RuneOffsetToLSPPosition: expected (1,0), got (%d,%d)", pos.Line, pos.Character)
	}

	// LSPPositionToRuneOffset
	runeOff := LSPPositionToRuneOffset(content, Position{Line: 1, Character: 0})
	if runeOff != 6 {
		t.Errorf("LSPPositionToRuneOffset: expected 6, got %d", runeOff)
	}
}

func TestEncodeDecodeUTF16(t *testing.T) {
	original := "hello 日本語 😀"

	encoded := EncodeUTF16(original)
	decoded := DecodeUTF16(encoded)

	if decoded != original {
		t.Errorf("Round trip failed: expected %q, got %q", original, decoded)
	}
}

func TestPositionConverter_TrailingNewline(t *testing.T) {
	// Content ending with newline
	pc := NewPositionConverter("line1\nline2\n")

	if pc.LineCount() != 3 {
		t.Errorf("Expected 3 lines (including empty line after trailing newline), got %d", pc.LineCount())
	}

	// Last line should be empty
	lastLine := pc.LineContent(2)
	if lastLine != "" {
		t.Errorf("Expected empty last line, got %q", lastLine)
	}
}

func TestPositionConverter_MultiByteCharacters(t *testing.T) {
	// Japanese text: "日本語" (3 characters, 9 bytes)
	pc := NewPositionConverter("日本語")

	if pc.LineCount() != 1 {
		t.Errorf("Expected 1 line, got %d", pc.LineCount())
	}

	// Each Japanese character is 3 bytes but 1 UTF-16 code unit
	// So byte 0 = char 0, byte 3 = char 1, byte 6 = char 2

	tests := []struct {
		byteOff int
		char    int
	}{
		{0, 0},
		{3, 1},
		{6, 2},
		{9, 3},
	}

	for _, tt := range tests {
		pos := pc.ByteOffsetToPosition(tt.byteOff)
		if pos.Character != tt.char {
			t.Errorf("ByteOffset %d: expected char %d, got %d", tt.byteOff, tt.char, pos.Character)
		}
	}
}
