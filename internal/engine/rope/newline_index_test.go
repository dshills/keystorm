package rope

import "testing"

func TestNewlineIndexEmpty(t *testing.T) {
	idx := ComputeNewlineIndex("")
	if idx.Count() != 0 {
		t.Errorf("expected count 0, got %d", idx.Count())
	}
	if pos := idx.Position(0); pos != -1 {
		t.Errorf("expected position -1, got %d", pos)
	}
}

func TestNewlineIndexNoNewlines(t *testing.T) {
	idx := ComputeNewlineIndex("hello world")
	if idx.Count() != 0 {
		t.Errorf("expected count 0, got %d", idx.Count())
	}
}

func TestNewlineIndexSingleNewline(t *testing.T) {
	idx := ComputeNewlineIndex("hello\nworld")
	if idx.Count() != 1 {
		t.Errorf("expected count 1, got %d", idx.Count())
	}
	if pos := idx.Position(0); pos != 5 {
		t.Errorf("expected position 5, got %d", pos)
	}
}

func TestNewlineIndexMultipleNewlines(t *testing.T) {
	idx := ComputeNewlineIndex("a\nb\nc\nd\ne")
	if idx.Count() != 4 {
		t.Errorf("expected count 4, got %d", idx.Count())
	}

	expected := []int{1, 3, 5, 7}
	for i, exp := range expected {
		if pos := idx.Position(uint32(i)); pos != exp {
			t.Errorf("position %d: expected %d, got %d", i, exp, pos)
		}
	}
}

func TestNewlineIndexMoreThanInline(t *testing.T) {
	// More than MaxInlineNewlines (4)
	idx := ComputeNewlineIndex("a\nb\nc\nd\ne\nf\ng")
	if idx.Count() != 6 {
		t.Errorf("expected count 6, got %d", idx.Count())
	}

	expected := []int{1, 3, 5, 7, 9, 11}
	for i, exp := range expected {
		if pos := idx.Position(uint32(i)); pos != exp {
			t.Errorf("position %d: expected %d, got %d", i, exp, pos)
		}
	}
}

func TestNewlineIndexFindNthNewline(t *testing.T) {
	idx := ComputeNewlineIndex("abc\ndef\nghi\njkl")

	tests := []struct {
		n        uint32
		expected int
	}{
		{0, -1}, // 0 is invalid (1-indexed)
		{1, 3},  // first newline
		{2, 7},  // second newline
		{3, 11}, // third newline
		{4, -1}, // out of range
	}

	for _, tt := range tests {
		if pos := idx.FindNthNewline(tt.n); pos != tt.expected {
			t.Errorf("FindNthNewline(%d): expected %d, got %d", tt.n, tt.expected, pos)
		}
	}
}

func TestNewlineIndexSearchLine(t *testing.T) {
	idx := ComputeNewlineIndex("abc\ndef\nghi")

	tests := []struct {
		line     uint32
		expected int
	}{
		{0, 0},  // line 0 starts at 0
		{1, 4},  // line 1 starts after first newline
		{2, 8},  // line 2 starts after second newline
		{3, -1}, // out of range
	}

	for _, tt := range tests {
		if pos := idx.SearchLine(tt.line); pos != tt.expected {
			t.Errorf("SearchLine(%d): expected %d, got %d", tt.line, tt.expected, pos)
		}
	}
}

func TestNewlineIndexNewlineBefore(t *testing.T) {
	idx := ComputeNewlineIndex("abc\ndef\nghi")

	tests := []struct {
		offset   int
		expected int
	}{
		{0, -1},  // nothing before 0
		{3, -1},  // nothing before position 3
		{4, 3},   // newline at 3 is before 4
		{5, 3},   // newline at 3 is before 5
		{7, 3},   // newline at 3 is before 7
		{8, 7},   // newline at 7 is before 8
		{100, 7}, // newline at 7 is the last one
	}

	for _, tt := range tests {
		if pos := idx.NewlineBefore(tt.offset); pos != tt.expected {
			t.Errorf("NewlineBefore(%d): expected %d, got %d", tt.offset, tt.expected, pos)
		}
	}
}

func TestNewlineIndexNewlineAfter(t *testing.T) {
	idx := ComputeNewlineIndex("abc\ndef\nghi")

	tests := []struct {
		offset   int
		expected int
	}{
		{0, 3},  // first newline at 3
		{3, 3},  // at position 3, return 3
		{4, 7},  // next newline at 7
		{7, 7},  // at position 7, return 7
		{8, -1}, // no newline after 8
		{100, -1},
	}

	for _, tt := range tests {
		if pos := idx.NewlineAfter(tt.offset); pos != tt.expected {
			t.Errorf("NewlineAfter(%d): expected %d, got %d", tt.offset, tt.expected, pos)
		}
	}
}

func TestNewlineIndexLastNewlinePosition(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"", -1},
		{"no newline", -1},
		{"hello\n", 5},
		{"a\nb\nc", 3},
		{"\n\n\n", 2},
	}

	for _, tt := range tests {
		idx := ComputeNewlineIndex(tt.text)
		if pos := idx.LastNewlinePosition(); pos != tt.expected {
			t.Errorf("LastNewlinePosition(%q): expected %d, got %d", tt.text, tt.expected, pos)
		}
	}
}

func TestNewlineIndexContains(t *testing.T) {
	idx := ComputeNewlineIndex("a\nb\nc\nd")

	tests := []struct {
		lines    uint32
		expected bool
	}{
		{0, true},
		{1, true},
		{2, true},
		{3, true},
		{4, false},
		{100, false},
	}

	for _, tt := range tests {
		if result := idx.Contains(tt.lines); result != tt.expected {
			t.Errorf("Contains(%d): expected %v, got %v", tt.lines, tt.expected, result)
		}
	}
}

func BenchmarkNewlineIndexCompute(b *testing.B) {
	// Typical chunk with a few newlines
	text := "This is line one\nThis is line two\nThis is line three\nAnd line four\n"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeNewlineIndex(text)
	}
}

func BenchmarkNewlineIndexPosition(b *testing.B) {
	text := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj"
	idx := ComputeNewlineIndex(text)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Position(uint32(i % 10))
	}
}

func BenchmarkNewlineIndexNewlineBefore(b *testing.B) {
	text := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj"
	idx := ComputeNewlineIndex(text)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.NewlineBefore(i % 20)
	}
}
