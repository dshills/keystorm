package rope

import (
	"strings"
	"testing"
	"testing/quick"
)

func TestNew(t *testing.T) {
	r := New()
	if r.Len() != 0 {
		t.Errorf("New rope should have length 0, got %d", r.Len())
	}
	if !r.IsEmpty() {
		t.Error("New rope should be empty")
	}
	if r.String() != "" {
		t.Errorf("New rope String() should be empty, got %q", r.String())
	}
	if r.LineCount() != 1 {
		t.Errorf("New rope should have 1 line, got %d", r.LineCount())
	}
}

func TestFromString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"single char", "a"},
		{"short string", "hello"},
		{"with newline", "hello\nworld"},
		{"multiple newlines", "a\nb\nc\nd"},
		{"unicode", "hello 世界 🌍"},
		{"long string", strings.Repeat("abcdefghij", 100)},
		{"very long string", strings.Repeat("x", 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FromString(tt.input)
			if r.String() != tt.input {
				t.Errorf("String() = %q, want %q", r.String(), tt.input)
			}
			if r.Len() != ByteOffset(len(tt.input)) {
				t.Errorf("Len() = %d, want %d", r.Len(), len(tt.input))
			}
		})
	}
}

func TestInsert(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		offset   ByteOffset
		text     string
		expected string
	}{
		{"insert at start", "world", 0, "hello ", "hello world"},
		{"insert at end", "hello", 5, " world", "hello world"},
		{"insert in middle", "helloworld", 5, " ", "hello world"},
		{"insert into empty", "", 0, "hello", "hello"},
		{"insert empty string", "hello", 3, "", "hello"},
		{"insert unicode", "hello", 5, " 世界", "hello 世界"},
		{"insert at unicode boundary", "世界", 3, "!", "世!界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FromString(tt.initial)
			r = r.Insert(tt.offset, tt.text)
			if got := r.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		start    ByteOffset
		end      ByteOffset
		expected string
	}{
		{"delete from start", "hello world", 0, 6, "world"},
		{"delete from end", "hello world", 5, 11, "hello"},
		{"delete from middle", "hello world", 5, 6, "helloworld"},
		{"delete all", "hello", 0, 5, ""},
		{"delete nothing", "hello", 3, 3, "hello"},
		{"delete beyond end", "hello", 0, 100, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FromString(tt.initial)
			r = r.Delete(tt.start, tt.end)
			if got := r.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReplace(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		start    ByteOffset
		end      ByteOffset
		text     string
		expected string
	}{
		{"replace word", "hello world", 6, 11, "universe", "hello universe"},
		{"replace with shorter", "hello world", 0, 5, "hi", "hi world"},
		{"replace with longer", "hi world", 0, 2, "hello", "hello world"},
		{"replace all", "hello", 0, 5, "world", "world"},
		{"replace nothing with insert", "hello", 5, 5, " world", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FromString(tt.initial)
			r = r.Replace(tt.start, tt.end, tt.text)
			if got := r.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		offset        ByteOffset
		expectedLeft  string
		expectedRight string
	}{
		{"split at start", "hello", 0, "", "hello"},
		{"split at end", "hello", 5, "hello", ""},
		{"split in middle", "hello", 3, "hel", "lo"},
		{"split empty", "", 0, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FromString(tt.input)
			left, right := r.Split(tt.offset)
			if left.String() != tt.expectedLeft {
				t.Errorf("left = %q, want %q", left.String(), tt.expectedLeft)
			}
			if right.String() != tt.expectedRight {
				t.Errorf("right = %q, want %q", right.String(), tt.expectedRight)
			}
		})
	}
}

func TestConcat(t *testing.T) {
	tests := []struct {
		name     string
		left     string
		right    string
		expected string
	}{
		{"concat two strings", "hello ", "world", "hello world"},
		{"concat with empty left", "", "hello", "hello"},
		{"concat with empty right", "hello", "", "hello"},
		{"concat two empty", "", "", ""},
		{"concat long strings", strings.Repeat("a", 1000), strings.Repeat("b", 1000), strings.Repeat("a", 1000) + strings.Repeat("b", 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := FromString(tt.left)
			right := FromString(tt.right)
			result := left.Concat(right)
			if result.String() != tt.expected {
				t.Errorf("got %q, want %q", result.String(), tt.expected)
			}
		})
	}
}

func TestSlice(t *testing.T) {
	text := "hello world"
	r := FromString(text)

	tests := []struct {
		name     string
		start    ByteOffset
		end      ByteOffset
		expected string
	}{
		{"full slice", 0, 11, "hello world"},
		{"first word", 0, 5, "hello"},
		{"last word", 6, 11, "world"},
		{"middle", 3, 8, "lo wo"},
		{"empty slice", 5, 5, ""},
		{"beyond end", 6, 100, "world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Slice(tt.start, tt.end)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLineCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint32
	}{
		{"empty", "", 1},
		{"no newlines", "hello", 1},
		{"one newline", "hello\n", 2},
		{"two lines", "hello\nworld", 2},
		{"three lines", "a\nb\nc", 3},
		{"trailing newline", "a\nb\n", 3},
		{"only newlines", "\n\n\n", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FromString(tt.input)
			if got := r.LineCount(); got != tt.expected {
				t.Errorf("LineCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestLineText(t *testing.T) {
	r := FromString("hello\nworld\nfoo")

	tests := []struct {
		line     uint32
		expected string
	}{
		{0, "hello"},
		{1, "world"},
		{2, "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := r.LineText(tt.line)
			if got != tt.expected {
				t.Errorf("LineText(%d) = %q, want %q", tt.line, got, tt.expected)
			}
		})
	}
}

func TestLineStartOffset(t *testing.T) {
	r := FromString("hello\nworld\nfoo")

	tests := []struct {
		line     uint32
		expected ByteOffset
	}{
		{0, 0},
		{1, 6},
		{2, 12},
	}

	for _, tt := range tests {
		got := r.LineStartOffset(tt.line)
		if got != tt.expected {
			t.Errorf("LineStartOffset(%d) = %d, want %d", tt.line, got, tt.expected)
		}
	}
}

func TestOffsetToPoint(t *testing.T) {
	r := FromString("hello\nworld\nfoo")

	tests := []struct {
		offset   ByteOffset
		expected Point
	}{
		{0, Point{0, 0}},
		{5, Point{0, 5}},
		{6, Point{1, 0}},
		{11, Point{1, 5}},
		{12, Point{2, 0}},
		{15, Point{2, 3}},
	}

	for _, tt := range tests {
		got := r.OffsetToPoint(tt.offset)
		if got != tt.expected {
			t.Errorf("OffsetToPoint(%d) = %+v, want %+v", tt.offset, got, tt.expected)
		}
	}
}

func TestPointToOffset(t *testing.T) {
	r := FromString("hello\nworld\nfoo")

	tests := []struct {
		point    Point
		expected ByteOffset
	}{
		{Point{0, 0}, 0},
		{Point{0, 5}, 5},
		{Point{1, 0}, 6},
		{Point{1, 5}, 11},
		{Point{2, 0}, 12},
		{Point{2, 3}, 15},
	}

	for _, tt := range tests {
		got := r.PointToOffset(tt.point)
		if got != tt.expected {
			t.Errorf("PointToOffset(%+v) = %d, want %d", tt.point, got, tt.expected)
		}
	}
}

func TestByteAt(t *testing.T) {
	r := FromString("hello")

	tests := []struct {
		offset   ByteOffset
		expected byte
		ok       bool
	}{
		{0, 'h', true},
		{4, 'o', true},
		{5, 0, false},
		{100, 0, false},
	}

	for _, tt := range tests {
		b, ok := r.ByteAt(tt.offset)
		if b != tt.expected || ok != tt.ok {
			t.Errorf("ByteAt(%d) = (%c, %v), want (%c, %v)", tt.offset, b, ok, tt.expected, tt.ok)
		}
	}
}

func TestImmutability(t *testing.T) {
	original := FromString("hello")
	modified := original.Insert(5, " world")

	if original.String() != "hello" {
		t.Errorf("Original was modified: %q", original.String())
	}
	if modified.String() != "hello world" {
		t.Errorf("Modified is wrong: %q", modified.String())
	}
}

func TestLargeRope(t *testing.T) {
	// Create a large rope
	text := strings.Repeat("abcdefghij\n", 10000)
	r := FromString(text)

	if r.String() != text {
		t.Error("Large rope content mismatch")
	}

	// Test operations on large rope
	r = r.Insert(50000, "INSERTED")
	if !strings.Contains(r.String(), "INSERTED") {
		t.Error("Insert into large rope failed")
	}

	// Test line access
	lineText := r.LineText(5000)
	if len(lineText) == 0 {
		t.Error("Failed to get line from large rope")
	}
}

func TestChunkIterator(t *testing.T) {
	text := strings.Repeat("hello world ", 100)
	r := FromString(text)

	var result strings.Builder
	iter := r.Chunks()
	for iter.Next() {
		result.WriteString(iter.Chunk().String())
	}

	if result.String() != text {
		t.Error("Chunk iterator did not produce correct output")
	}
}

func TestLineIterator(t *testing.T) {
	text := "line1\nline2\nline3"
	r := FromString(text)

	expected := []string{"line1", "line2", "line3"}
	var got []string

	iter := r.Lines()
	for iter.Next() {
		got = append(got, iter.Text())
	}

	if len(got) != len(expected) {
		t.Errorf("Got %d lines, want %d", len(got), len(expected))
	}

	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("Line %d: got %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestRuneIterator(t *testing.T) {
	text := "hello 世界"
	r := FromString(text)

	var runes []rune
	iter := r.Runes()
	for iter.Next() {
		runes = append(runes, iter.Rune())
	}

	expected := []rune(text)
	if len(runes) != len(expected) {
		t.Errorf("Got %d runes, want %d", len(runes), len(expected))
	}

	for i := range expected {
		if runes[i] != expected[i] {
			t.Errorf("Rune %d: got %c, want %c", i, runes[i], expected[i])
		}
	}
}

func TestCursor(t *testing.T) {
	r := FromString("hello\nworld")

	cursor := NewCursor(r)
	if cursor.Offset() != 0 {
		t.Errorf("Initial offset = %d, want 0", cursor.Offset())
	}

	// Test seeking
	if !cursor.SeekOffset(6) {
		t.Error("SeekOffset failed")
	}
	if cursor.Offset() != 6 {
		t.Errorf("After seek, offset = %d, want 6", cursor.Offset())
	}

	// Test rune reading
	r2, size := cursor.Rune()
	if r2 != 'w' || size != 1 {
		t.Errorf("Rune() = (%c, %d), want (w, 1)", r2, size)
	}

	// Test Next
	if !cursor.Next() {
		t.Error("Next() returned false")
	}
	if cursor.Offset() != 7 {
		t.Errorf("After Next, offset = %d, want 7", cursor.Offset())
	}

	// Test SeekLine
	if !cursor.SeekLine(1) {
		t.Error("SeekLine failed")
	}
	if cursor.Offset() != 6 {
		t.Errorf("After SeekLine(1), offset = %d, want 6", cursor.Offset())
	}
}

func TestBuilder(t *testing.T) {
	b := NewBuilder()
	b.WriteString("hello")
	b.WriteString(" ")
	b.WriteString("world")

	r := b.Build()
	if r.String() != "hello world" {
		t.Errorf("Builder produced %q, want %q", r.String(), "hello world")
	}

	// Builder should be reset after Build
	if b.Len() != 0 {
		t.Error("Builder not reset after Build")
	}
}

func TestFromLines(t *testing.T) {
	lines := []string{"hello", "world", "foo"}
	r := FromLines(lines)

	expected := "hello\nworld\nfoo"
	if r.String() != expected {
		t.Errorf("FromLines produced %q, want %q", r.String(), expected)
	}
}

func TestJoin(t *testing.T) {
	ropes := []Rope{
		FromString("a"),
		FromString("b"),
		FromString("c"),
	}
	result := Join(ropes, ", ")
	expected := "a, b, c"

	if result.String() != expected {
		t.Errorf("Join produced %q, want %q", result.String(), expected)
	}
}

func TestEquals(t *testing.T) {
	r1 := FromString("hello")
	r2 := FromString("hello")
	r3 := FromString("world")

	if !r1.Equals(r2) {
		t.Error("Equal ropes should be equal")
	}
	if r1.Equals(r3) {
		t.Error("Different ropes should not be equal")
	}
}

// Property-based tests

func TestInsertDeleteProperty(t *testing.T) {
	f := func(s string, offset int, insert string) bool {
		if len(s) == 0 {
			offset = 0
		} else {
			offset = offset % (len(s) + 1)
			if offset < 0 {
				offset = -offset
			}
		}

		r := FromString(s)
		r = r.Insert(ByteOffset(offset), insert)
		r = r.Delete(ByteOffset(offset), ByteOffset(offset+len(insert)))
		return r.String() == s
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestConcatSplitProperty(t *testing.T) {
	f := func(s string, offset int) bool {
		if len(s) == 0 {
			return true
		}
		offset = offset % (len(s) + 1)
		if offset < 0 {
			offset = -offset
		}

		r := FromString(s)
		left, right := r.Split(ByteOffset(offset))
		result := left.Concat(right)
		return result.String() == s
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestLenProperty(t *testing.T) {
	f := func(s string) bool {
		r := FromString(s)
		return int(r.Len()) == len(s)
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestLineCountProperty(t *testing.T) {
	f := func(s string) bool {
		r := FromString(s)
		expectedLines := uint32(1)
		for _, c := range s {
			if c == '\n' {
				expectedLines++
			}
		}
		return r.LineCount() == expectedLines
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

// TextSummary tests

func TestComputeSummary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		bytes    ByteOffset
		lines    uint32
		hasASCII bool
	}{
		{"empty", "", 0, 0, true},
		{"ascii", "hello", 5, 0, true},
		{"with newline", "hello\n", 6, 1, true},
		{"unicode", "世界", 6, 0, false},
		{"mixed", "hello 世界", 12, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := ComputeSummary(tt.input)
			if sum.Bytes != tt.bytes {
				t.Errorf("Bytes = %d, want %d", sum.Bytes, tt.bytes)
			}
			if sum.Lines != tt.lines {
				t.Errorf("Lines = %d, want %d", sum.Lines, tt.lines)
			}
			isASCII := sum.Flags&FlagASCII != 0
			if isASCII != tt.hasASCII {
				t.Errorf("ASCII flag = %v, want %v", isASCII, tt.hasASCII)
			}
		})
	}
}

func TestSummaryAdd(t *testing.T) {
	s1 := ComputeSummary("hello\n")
	s2 := ComputeSummary("world")

	combined := s1.Add(s2)

	if combined.Bytes != 11 {
		t.Errorf("Combined bytes = %d, want 11", combined.Bytes)
	}
	if combined.Lines != 1 {
		t.Errorf("Combined lines = %d, want 1", combined.Lines)
	}
}
