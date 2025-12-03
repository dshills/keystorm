package rope

import (
	"testing"
	"unicode/utf8"
)

// FuzzFromString tests rope creation from arbitrary strings.
func FuzzFromString(f *testing.F) {
	// Add seed corpus
	f.Add("")
	f.Add("hello")
	f.Add("hello\nworld")
	f.Add("hello\r\nworld")
	f.Add("日本語")
	f.Add("emoji 🎉 test")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, s string) {
		// Skip invalid UTF-8 (rope requires valid UTF-8)
		if !utf8.ValidString(s) {
			return
		}

		r := FromString(s)

		// Verify length
		if int(r.Len()) != len(s) {
			t.Errorf("length mismatch: got %d, want %d", r.Len(), len(s))
		}

		// Verify content
		if r.String() != s {
			t.Errorf("content mismatch")
		}
	})
}

// FuzzInsert tests insert operations.
func FuzzInsert(f *testing.F) {
	f.Add("hello", 0, "x")
	f.Add("hello", 5, "x")
	f.Add("hello", 3, "world")
	f.Add("", 0, "test")
	f.Add("日本語", 3, "x")

	f.Fuzz(func(t *testing.T, initial string, offset int, insert string) {
		if !utf8.ValidString(initial) || !utf8.ValidString(insert) {
			return
		}

		r := FromString(initial)

		// Clamp offset to valid range
		if offset < 0 {
			offset = 0
		}
		if offset > len(initial) {
			offset = len(initial)
		}

		// Perform insert
		result := r.Insert(ByteOffset(offset), insert)

		// Verify result
		expected := initial[:offset] + insert + initial[offset:]
		if result.String() != expected {
			t.Errorf("insert mismatch at offset %d", offset)
		}
	})
}

// FuzzDelete tests delete operations.
func FuzzDelete(f *testing.F) {
	f.Add("hello world", 0, 5)
	f.Add("hello world", 6, 11)
	f.Add("hello world", 5, 6)
	f.Add("日本語", 0, 3)

	f.Fuzz(func(t *testing.T, initial string, start, end int) {
		if !utf8.ValidString(initial) {
			return
		}

		r := FromString(initial)

		// Clamp to valid range
		if start < 0 {
			start = 0
		}
		if end < start {
			end = start
		}
		if end > len(initial) {
			end = len(initial)
		}

		result := r.Delete(ByteOffset(start), ByteOffset(end))

		// Verify result
		expected := initial[:start] + initial[end:]
		if result.String() != expected {
			t.Errorf("delete mismatch: range [%d, %d)", start, end)
		}
	})
}

// FuzzReplace tests replace operations.
func FuzzReplace(f *testing.F) {
	f.Add("hello world", 0, 5, "hi")
	f.Add("hello world", 6, 11, "universe")
	f.Add("abcdef", 2, 4, "XYZ")

	f.Fuzz(func(t *testing.T, initial string, start, end int, replacement string) {
		if !utf8.ValidString(initial) || !utf8.ValidString(replacement) {
			return
		}

		r := FromString(initial)

		// Clamp to valid range
		if start < 0 {
			start = 0
		}
		if end < start {
			end = start
		}
		if end > len(initial) {
			end = len(initial)
		}

		result := r.Replace(ByteOffset(start), ByteOffset(end), replacement)

		// Verify result
		expected := initial[:start] + replacement + initial[end:]
		if result.String() != expected {
			t.Errorf("replace mismatch: range [%d, %d)", start, end)
		}
	})
}

// FuzzSplit tests split operations.
func FuzzSplit(f *testing.F) {
	f.Add("hello world", 0)
	f.Add("hello world", 5)
	f.Add("hello world", 11)
	f.Add("日本語", 3)

	f.Fuzz(func(t *testing.T, s string, offset int) {
		if !utf8.ValidString(s) {
			return
		}

		r := FromString(s)

		// Clamp to valid range
		if offset < 0 {
			offset = 0
		}
		if offset > len(s) {
			offset = len(s)
		}

		left, right := r.Split(ByteOffset(offset))

		// Verify parts
		if left.String() != s[:offset] {
			t.Errorf("left part mismatch at offset %d", offset)
		}
		if right.String() != s[offset:] {
			t.Errorf("right part mismatch at offset %d", offset)
		}

		// Verify concatenation reproduces original
		combined := left.Concat(right)
		if combined.String() != s {
			t.Errorf("split+concat does not reproduce original")
		}
	})
}

// FuzzConcat tests concatenation.
func FuzzConcat(f *testing.F) {
	f.Add("hello", "world")
	f.Add("", "world")
	f.Add("hello", "")
	f.Add("", "")
	f.Add("日本語", "abc")

	f.Fuzz(func(t *testing.T, s1, s2 string) {
		if !utf8.ValidString(s1) || !utf8.ValidString(s2) {
			return
		}

		r1 := FromString(s1)
		r2 := FromString(s2)
		combined := r1.Concat(r2)

		expected := s1 + s2
		if combined.String() != expected {
			t.Error("concat mismatch")
		}
		if int(combined.Len()) != len(expected) {
			t.Errorf("length mismatch: got %d, want %d", combined.Len(), len(expected))
		}
	})
}

// FuzzLineOperations tests line-related operations.
func FuzzLineOperations(f *testing.F) {
	f.Add("line1\nline2\nline3")
	f.Add("no newline")
	f.Add("\n\n\n")
	f.Add("")
	f.Add("日本語\n英語\n中国語")

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}

		r := FromString(s)

		// Verify line count
		lineCount := r.LineCount()
		if lineCount == 0 {
			t.Error("line count should be at least 1")
		}

		// Verify each line can be accessed
		for i := uint32(0); i < lineCount; i++ {
			start := r.LineStartOffset(i)
			end := r.LineEndOffset(i)

			if start > end {
				t.Errorf("line %d: start %d > end %d", i, start, end)
			}
			if start > r.Len() || end > r.Len() {
				t.Errorf("line %d: offsets out of range", i)
			}

			// Get line text
			text := r.LineText(i)
			_ = text // Just verify no panic
		}
	})
}

// FuzzOffsetToPoint tests coordinate conversion.
func FuzzOffsetToPoint(f *testing.F) {
	f.Add("line1\nline2\nline3", 0)
	f.Add("line1\nline2\nline3", 5)
	f.Add("line1\nline2\nline3", 6)
	f.Add("abc", 2)

	f.Fuzz(func(t *testing.T, s string, offset int) {
		if !utf8.ValidString(s) {
			return
		}

		r := FromString(s)

		// Clamp offset
		if offset < 0 {
			offset = 0
		}
		if offset > len(s) {
			offset = len(s)
		}

		// Convert offset to point
		point := r.OffsetToPoint(ByteOffset(offset))

		// Verify point is within valid range
		if point.Line >= r.LineCount() {
			t.Errorf("line %d >= lineCount %d", point.Line, r.LineCount())
		}

		// Convert back to offset
		backOffset := r.PointToOffset(point)

		// The back-converted offset should be <= original
		// (might be at start of line if original was at newline)
		if backOffset > ByteOffset(offset) {
			t.Errorf("round-trip offset mismatch: %d -> (%d,%d) -> %d",
				offset, point.Line, point.Column, backOffset)
		}
	})
}

// FuzzSlice tests slice operations.
func FuzzSlice(f *testing.F) {
	f.Add("hello world", 0, 5)
	f.Add("hello world", 6, 11)
	f.Add("hello world", 0, 11)
	f.Add("日本語", 0, 3)

	f.Fuzz(func(t *testing.T, s string, start, end int) {
		if !utf8.ValidString(s) {
			return
		}

		r := FromString(s)

		// Clamp to valid range
		if start < 0 {
			start = 0
		}
		if end < start {
			end = start
		}
		if end > len(s) {
			end = len(s)
		}

		slice := r.Slice(ByteOffset(start), ByteOffset(end))

		expected := s[start:end]
		if slice != expected {
			t.Errorf("slice mismatch: range [%d, %d)", start, end)
		}
	})
}

// FuzzByteAt tests byte access.
func FuzzByteAt(f *testing.F) {
	f.Add("hello", 0)
	f.Add("hello", 4)
	f.Add("hello", 5)
	f.Add("日本語", 0)

	f.Fuzz(func(t *testing.T, s string, offset int) {
		if !utf8.ValidString(s) {
			return
		}

		r := FromString(s)

		b, ok := r.ByteAt(ByteOffset(offset))

		if offset >= 0 && offset < len(s) {
			if !ok {
				t.Errorf("expected ok=true for offset %d", offset)
			}
			if b != s[offset] {
				t.Errorf("byte mismatch at offset %d", offset)
			}
		} else {
			if ok {
				t.Errorf("expected ok=false for offset %d", offset)
			}
		}
	})
}

// FuzzMultipleOperations tests sequences of operations.
func FuzzMultipleOperations(f *testing.F) {
	// op: 0=insert, 1=delete, 2=replace
	f.Add("hello", 0, 0, 5, "x")
	f.Add("hello", 1, 0, 3, "")
	f.Add("hello", 2, 1, 4, "abc")

	f.Fuzz(func(t *testing.T, initial string, op int, pos1, pos2 int, text string) {
		if !utf8.ValidString(initial) || !utf8.ValidString(text) {
			return
		}

		r := FromString(initial)

		// Clamp positions
		if pos1 < 0 {
			pos1 = 0
		}
		if pos2 < pos1 {
			pos2 = pos1
		}
		if pos1 > len(initial) {
			pos1 = len(initial)
		}
		if pos2 > len(initial) {
			pos2 = len(initial)
		}

		// Perform operation
		switch op % 3 {
		case 0:
			r = r.Insert(ByteOffset(pos1), text)
		case 1:
			r = r.Delete(ByteOffset(pos1), ByteOffset(pos2))
		case 2:
			r = r.Replace(ByteOffset(pos1), ByteOffset(pos2), text)
		}

		// Verify basic invariants
		if !utf8.ValidString(r.String()) {
			t.Error("result is not valid UTF-8")
		}
		if int(r.Len()) != len(r.String()) {
			t.Errorf("length mismatch: Len()=%d, len(String())=%d", r.Len(), len(r.String()))
		}
	})
}
