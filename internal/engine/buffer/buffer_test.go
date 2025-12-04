package buffer

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestNewBuffer(t *testing.T) {
	b := NewBuffer()

	if !b.IsEmpty() {
		t.Error("new buffer should be empty")
	}

	if b.Len() != 0 {
		t.Errorf("expected length 0, got %d", b.Len())
	}

	if b.LineCount() != 1 {
		t.Errorf("expected 1 line, got %d", b.LineCount())
	}
}

func TestNewBufferFromString(t *testing.T) {
	text := "Hello, World!"
	b := NewBufferFromString(text)

	if b.Text() != text {
		t.Errorf("expected %q, got %q", text, b.Text())
	}

	if b.Len() != int64(len(text)) {
		t.Errorf("expected length %d, got %d", len(text), b.Len())
	}
}

func TestNewBufferFromStringMultiline(t *testing.T) {
	text := "line1\nline2\nline3"
	b := NewBufferFromString(text)

	if b.LineCount() != 3 {
		t.Errorf("expected 3 lines, got %d", b.LineCount())
	}

	if b.LineText(0) != "line1" {
		t.Errorf("expected line1, got %q", b.LineText(0))
	}

	if b.LineText(1) != "line2" {
		t.Errorf("expected line2, got %q", b.LineText(1))
	}

	if b.LineText(2) != "line3" {
		t.Errorf("expected line3, got %q", b.LineText(2))
	}
}

func TestBufferInsert(t *testing.T) {
	b := NewBufferFromString("Hello World")

	end, err := b.Insert(5, ",")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if end != 6 {
		t.Errorf("expected end position 6, got %d", end)
	}

	if b.Text() != "Hello, World" {
		t.Errorf("expected 'Hello, World', got %q", b.Text())
	}
}

func TestBufferInsertAtStart(t *testing.T) {
	b := NewBufferFromString("World")

	_, err := b.Insert(0, "Hello ")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if b.Text() != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", b.Text())
	}
}

func TestBufferInsertAtEnd(t *testing.T) {
	b := NewBufferFromString("Hello")

	_, err := b.Insert(5, " World")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if b.Text() != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", b.Text())
	}
}

func TestBufferInsertOutOfRange(t *testing.T) {
	b := NewBufferFromString("Hello")

	_, err := b.Insert(100, "X")
	if !errors.Is(err, ErrOffsetOutOfRange) {
		t.Errorf("expected ErrOffsetOutOfRange, got %v", err)
	}

	_, err = b.Insert(-1, "X")
	if !errors.Is(err, ErrOffsetOutOfRange) {
		t.Errorf("expected ErrOffsetOutOfRange, got %v", err)
	}
}

func TestBufferDelete(t *testing.T) {
	b := NewBufferFromString("Hello, World!")

	err := b.Delete(5, 7)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if b.Text() != "HelloWorld!" {
		t.Errorf("expected 'HelloWorld!', got %q", b.Text())
	}
}

func TestBufferDeleteInvalidRange(t *testing.T) {
	b := NewBufferFromString("Hello")

	err := b.Delete(3, 2)
	if !errors.Is(err, ErrRangeInvalid) {
		t.Errorf("expected ErrRangeInvalid, got %v", err)
	}

	err = b.Delete(0, 100)
	if !errors.Is(err, ErrRangeInvalid) {
		t.Errorf("expected ErrRangeInvalid, got %v", err)
	}
}

func TestBufferReplace(t *testing.T) {
	b := NewBufferFromString("Hello World")

	end, err := b.Replace(6, 11, "Go")
	if err != nil {
		t.Fatalf("replace failed: %v", err)
	}

	if end != 8 {
		t.Errorf("expected end position 8, got %d", end)
	}

	if b.Text() != "Hello Go" {
		t.Errorf("expected 'Hello Go', got %q", b.Text())
	}
}

func TestBufferApplyEdit(t *testing.T) {
	b := NewBufferFromString("Hello World")

	edit := NewEdit(Range{Start: 0, End: 5}, "Hi")
	result, err := b.ApplyEdit(edit)
	if err != nil {
		t.Fatalf("apply edit failed: %v", err)
	}

	if b.Text() != "Hi World" {
		t.Errorf("expected 'Hi World', got %q", b.Text())
	}

	if result.OldText != "Hello" {
		t.Errorf("expected old text 'Hello', got %q", result.OldText)
	}

	if result.Delta != -3 {
		t.Errorf("expected delta -3, got %d", result.Delta)
	}
}

func TestBufferApplyEdits(t *testing.T) {
	b := NewBufferFromString("Hello World")

	// Edits must be in reverse order
	edits := []Edit{
		NewEdit(Range{Start: 6, End: 11}, "Go"),     // "World" -> "Go"
		NewEdit(Range{Start: 0, End: 5}, "Goodbye"), // "Hello" -> "Goodbye"
	}

	err := b.ApplyEdits(edits)
	if err != nil {
		t.Fatalf("apply edits failed: %v", err)
	}

	if b.Text() != "Goodbye Go" {
		t.Errorf("expected 'Goodbye Go', got %q", b.Text())
	}
}

func TestBufferApplyEditsOverlap(t *testing.T) {
	b := NewBufferFromString("Hello World")

	// These edits overlap
	edits := []Edit{
		NewEdit(Range{Start: 3, End: 8}, "X"),
		NewEdit(Range{Start: 5, End: 10}, "Y"),
	}

	err := b.ApplyEdits(edits)
	if !errors.Is(err, ErrEditsOverlap) {
		t.Errorf("expected ErrEditsOverlap, got %v", err)
	}
}

func TestBufferLineOperations(t *testing.T) {
	text := "first line\nsecond line\nthird line"
	b := NewBufferFromString(text)

	if b.LineCount() != 3 {
		t.Errorf("expected 3 lines, got %d", b.LineCount())
	}

	tests := []struct {
		line     uint32
		expected string
	}{
		{0, "first line"},
		{1, "second line"},
		{2, "third line"},
	}

	for _, tt := range tests {
		got := b.LineText(tt.line)
		if got != tt.expected {
			t.Errorf("LineText(%d) = %q, want %q", tt.line, got, tt.expected)
		}
	}
}

func TestBufferLineStartEnd(t *testing.T) {
	text := "abc\ndefgh\nij"
	b := NewBufferFromString(text)

	tests := []struct {
		line          uint32
		expectedStart ByteOffset
		expectedEnd   ByteOffset
	}{
		{0, 0, 3},
		{1, 4, 9},
		{2, 10, 12},
	}

	for _, tt := range tests {
		start := b.LineStartOffset(tt.line)
		end := b.LineEndOffset(tt.line)

		if start != tt.expectedStart {
			t.Errorf("LineStartOffset(%d) = %d, want %d", tt.line, start, tt.expectedStart)
		}
		if end != tt.expectedEnd {
			t.Errorf("LineEndOffset(%d) = %d, want %d", tt.line, end, tt.expectedEnd)
		}
	}
}

func TestBufferOffsetToPoint(t *testing.T) {
	text := "abc\ndefgh\nij"
	b := NewBufferFromString(text)

	tests := []struct {
		offset   ByteOffset
		expected Point
	}{
		{0, Point{Line: 0, Column: 0}},
		{2, Point{Line: 0, Column: 2}},
		{3, Point{Line: 0, Column: 3}},
		{4, Point{Line: 1, Column: 0}},
		{7, Point{Line: 1, Column: 3}},
		{10, Point{Line: 2, Column: 0}},
	}

	for _, tt := range tests {
		got := b.OffsetToPoint(tt.offset)
		if got != tt.expected {
			t.Errorf("OffsetToPoint(%d) = %v, want %v", tt.offset, got, tt.expected)
		}
	}
}

func TestBufferPointToOffset(t *testing.T) {
	text := "abc\ndefgh\nij"
	b := NewBufferFromString(text)

	tests := []struct {
		point    Point
		expected ByteOffset
	}{
		{Point{Line: 0, Column: 0}, 0},
		{Point{Line: 0, Column: 2}, 2},
		{Point{Line: 1, Column: 0}, 4},
		{Point{Line: 1, Column: 3}, 7},
		{Point{Line: 2, Column: 0}, 10},
	}

	for _, tt := range tests {
		got := b.PointToOffset(tt.point)
		if got != tt.expected {
			t.Errorf("PointToOffset(%v) = %d, want %d", tt.point, got, tt.expected)
		}
	}
}

func TestBufferUTF16Conversion(t *testing.T) {
	// Test with emoji (surrogate pair in UTF-16)
	text := "a😀b"
	b := NewBufferFromString(text)

	// 'a' at offset 0 -> UTF-16 column 0
	p := b.OffsetToPointUTF16(0)
	if p.Column != 0 {
		t.Errorf("expected UTF-16 column 0 for 'a', got %d", p.Column)
	}

	// '😀' starts at offset 1 -> UTF-16 column 1
	p = b.OffsetToPointUTF16(1)
	if p.Column != 1 {
		t.Errorf("expected UTF-16 column 1 for emoji start, got %d", p.Column)
	}

	// 'b' at offset 5 (1 + 4 bytes for emoji) -> UTF-16 column 3 (1 + 2 for surrogate pair)
	p = b.OffsetToPointUTF16(5)
	if p.Column != 3 {
		t.Errorf("expected UTF-16 column 3 for 'b', got %d", p.Column)
	}
}

func TestBufferSnapshot(t *testing.T) {
	b := NewBufferFromString("Hello")
	snap := b.Snapshot()

	// Modify the buffer
	b.Insert(5, " World")

	// Snapshot should have original content
	if snap.Text() != "Hello" {
		t.Errorf("snapshot should have 'Hello', got %q", snap.Text())
	}

	// Buffer should have new content
	if b.Text() != "Hello World" {
		t.Errorf("buffer should have 'Hello World', got %q", b.Text())
	}
}

func TestBufferSnapshotOperations(t *testing.T) {
	text := "abc\ndefgh\nij"
	b := NewBufferFromString(text)
	snap := b.Snapshot()

	if snap.Len() != int64(len(text)) {
		t.Errorf("expected len %d, got %d", len(text), snap.Len())
	}

	if snap.LineCount() != 3 {
		t.Errorf("expected 3 lines, got %d", snap.LineCount())
	}

	if snap.LineText(1) != "defgh" {
		t.Errorf("expected 'defgh', got %q", snap.LineText(1))
	}

	p := snap.OffsetToPoint(7)
	if p.Line != 1 || p.Column != 3 {
		t.Errorf("expected (1:3), got %v", p)
	}
}

func TestBufferLineEndingNormalization(t *testing.T) {
	// Test CRLF to LF conversion
	b := NewBufferFromString("line1\r\nline2\r\n")

	if b.Text() != "line1\nline2\n" {
		t.Errorf("CRLF not normalized to LF: got %q", b.Text())
	}

	// Test CR to LF conversion
	b = NewBufferFromString("line1\rline2\r")

	if b.Text() != "line1\nline2\n" {
		t.Errorf("CR not normalized to LF: got %q", b.Text())
	}
}

func TestBufferWithCRLFLineEnding(t *testing.T) {
	b := NewBufferFromString("line1\nline2", WithCRLF())

	if b.Text() != "line1\r\nline2" {
		t.Errorf("expected CRLF, got %q", b.Text())
	}

	// Insert with normalization
	b.Insert(int64(len(b.Text())), "\nline3")
	expected := "line1\r\nline2\r\nline3"
	if b.Text() != expected {
		t.Errorf("expected %q, got %q", expected, b.Text())
	}
}

func TestBufferRevisionID(t *testing.T) {
	b := NewBuffer()
	rev1 := b.RevisionID()

	b.Insert(0, "Hello")
	rev2 := b.RevisionID()

	if rev1 == rev2 {
		t.Error("revision ID should change after insert")
	}

	b.Delete(0, 5)
	rev3 := b.RevisionID()

	if rev2 == rev3 {
		t.Error("revision ID should change after delete")
	}
}

func TestBufferConcurrentRead(t *testing.T) {
	b := NewBufferFromString("Hello World")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Text()
			_ = b.Len()
			_ = b.LineCount()
		}()
	}
	wg.Wait()
}

func TestBufferConcurrentReadWrite(t *testing.T) {
	b := NewBufferFromString("Hello")

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				b.Insert(0, "X")
			}
		}()
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = b.Text()
			}
		}()
	}

	wg.Wait()

	// Should have 100 X's plus "Hello"
	text := b.Text()
	xCount := strings.Count(text, "X")
	if xCount != 100 {
		t.Errorf("expected 100 X's, got %d", xCount)
	}
}

func TestDetectLineEnding(t *testing.T) {
	tests := []struct {
		text     string
		expected LineEnding
	}{
		{"no newlines", LineEndingLF},
		{"unix\nstyle\n", LineEndingLF},
		{"windows\r\nstyle\r\n", LineEndingCRLF},
		{"old mac\rstyle\r", LineEndingCR},
		{"mixed\r\nmore\nlines", LineEndingCRLF}, // CRLF wins
	}

	for _, tt := range tests {
		got := DetectLineEnding(tt.text)
		if got != tt.expected {
			t.Errorf("DetectLineEnding(%q) = %v, want %v", tt.text, got, tt.expected)
		}
	}
}

func TestPointOperations(t *testing.T) {
	p1 := Point{Line: 1, Column: 5}
	p2 := Point{Line: 1, Column: 10}
	p3 := Point{Line: 2, Column: 0}

	if !p1.Before(p2) {
		t.Error("p1 should be before p2")
	}

	if !p2.Before(p3) {
		t.Error("p2 should be before p3")
	}

	if p2.Before(p1) {
		t.Error("p2 should not be before p1")
	}

	if p1.Compare(p1) != 0 {
		t.Error("point should equal itself")
	}
}

func TestRangeOperations(t *testing.T) {
	r1 := Range{Start: 0, End: 10}
	r2 := Range{Start: 5, End: 15}
	r3 := Range{Start: 20, End: 30}

	if !r1.Overlaps(r2) {
		t.Error("r1 should overlap r2")
	}

	if r1.Overlaps(r3) {
		t.Error("r1 should not overlap r3")
	}

	if !r1.Contains(5) {
		t.Error("r1 should contain 5")
	}

	if r1.Contains(10) {
		t.Error("r1 should not contain 10 (exclusive end)")
	}

	intersection := r1.Intersect(r2)
	if intersection.Start != 5 || intersection.End != 10 {
		t.Errorf("intersection should be [5:10), got %v", intersection)
	}

	union := r1.Union(r2)
	if union.Start != 0 || union.End != 15 {
		t.Errorf("union should be [0:15), got %v", union)
	}
}

func TestEditOperations(t *testing.T) {
	insert := NewInsert(5, "Hello")
	if !insert.IsInsert() {
		t.Error("should be insert")
	}

	del := NewDelete(0, 5)
	if !del.IsDelete() {
		t.Error("should be delete")
	}

	replace := NewEdit(Range{Start: 0, End: 5}, "World")
	if !replace.IsReplace() {
		t.Error("should be replace")
	}

	if insert.Delta() != 5 {
		t.Errorf("insert delta should be 5, got %d", insert.Delta())
	}

	if del.Delta() != -5 {
		t.Errorf("delete delta should be -5, got %d", del.Delta())
	}
}

func TestChangeInvert(t *testing.T) {
	insertChange := Change{
		Type:     ChangeInsert,
		Range:    Range{Start: 5, End: 5},
		NewRange: Range{Start: 5, End: 10},
		NewText:  "Hello",
	}

	inverted := insertChange.Invert()
	if inverted.Type != ChangeDelete {
		t.Error("inverted insert should be delete")
	}
	if inverted.OldText != "Hello" {
		t.Error("inverted should have original new text as old text")
	}

	deleteChange := Change{
		Type:    ChangeDelete,
		Range:   Range{Start: 0, End: 5},
		OldText: "Hello",
	}

	inverted = deleteChange.Invert()
	if inverted.Type != ChangeInsert {
		t.Error("inverted delete should be insert")
	}
	if inverted.NewText != "Hello" {
		t.Error("inverted should have original old text as new text")
	}
}
