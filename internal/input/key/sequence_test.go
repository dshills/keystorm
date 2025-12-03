package key

import (
	"testing"
)

func TestSequenceBasicOperations(t *testing.T) {
	seq := NewSequence()
	if !seq.IsEmpty() {
		t.Error("NewSequence should be empty")
	}
	if seq.Len() != 0 {
		t.Error("NewSequence length should be 0")
	}

	seq.Add(NewRuneEvent('g', ModNone))
	if seq.IsEmpty() {
		t.Error("Sequence should not be empty after Add")
	}
	if seq.Len() != 1 {
		t.Error("Sequence length should be 1 after Add")
	}

	seq.Add(NewRuneEvent('g', ModNone))
	if seq.Len() != 2 {
		t.Error("Sequence length should be 2 after second Add")
	}

	seq.Clear()
	if !seq.IsEmpty() {
		t.Error("Sequence should be empty after Clear")
	}
}

func TestSequenceFirstLast(t *testing.T) {
	seq := NewSequence()
	if seq.First() != nil {
		t.Error("First on empty sequence should return nil")
	}
	if seq.Last() != nil {
		t.Error("Last on empty sequence should return nil")
	}

	seq.Add(NewRuneEvent('a', ModNone))
	seq.Add(NewRuneEvent('b', ModNone))
	seq.Add(NewRuneEvent('c', ModNone))

	if seq.First().Rune != 'a' {
		t.Errorf("First() = %q, want 'a'", seq.First().Rune)
	}
	if seq.Last().Rune != 'c' {
		t.Errorf("Last() = %q, want 'c'", seq.Last().Rune)
	}
}

func TestSequenceAt(t *testing.T) {
	seq := NewSequenceFrom(
		NewRuneEvent('a', ModNone),
		NewRuneEvent('b', ModNone),
		NewRuneEvent('c', ModNone),
	)

	if seq.At(-1) != nil {
		t.Error("At(-1) should return nil")
	}
	if seq.At(3) != nil {
		t.Error("At(3) should return nil for length 3")
	}
	if seq.At(0).Rune != 'a' {
		t.Errorf("At(0) = %q, want 'a'", seq.At(0).Rune)
	}
	if seq.At(1).Rune != 'b' {
		t.Errorf("At(1) = %q, want 'b'", seq.At(1).Rune)
	}
}

func TestSequenceString(t *testing.T) {
	tests := []struct {
		events []Event
		want   string
	}{
		{nil, ""},
		{[]Event{NewRuneEvent('g', ModNone), NewRuneEvent('g', ModNone)}, "g g"},
		{[]Event{NewRuneEvent('d', ModNone), NewRuneEvent('i', ModNone), NewRuneEvent('w', ModNone)}, "d i w"},
		{[]Event{NewRuneEvent('s', ModCtrl)}, "C-s"},
	}

	for _, tt := range tests {
		seq := &Sequence{Events: tt.events}
		if got := seq.String(); got != tt.want {
			t.Errorf("Sequence.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestSequenceVimString(t *testing.T) {
	tests := []struct {
		events []Event
		want   string
	}{
		{nil, ""},
		{[]Event{NewRuneEvent('g', ModNone), NewRuneEvent('g', ModNone)}, "gg"},
		{[]Event{NewRuneEvent('d', ModNone), NewRuneEvent('i', ModNone), NewRuneEvent('w', ModNone)}, "diw"},
		{[]Event{NewRuneEvent('s', ModCtrl)}, "<C-s>"},
		{[]Event{NewRuneEvent('x', ModCtrl), NewRuneEvent('s', ModCtrl)}, "<C-x><C-s>"},
	}

	for _, tt := range tests {
		seq := &Sequence{Events: tt.events}
		if got := seq.VimString(); got != tt.want {
			t.Errorf("Sequence.VimString() = %q, want %q", got, tt.want)
		}
	}
}

func TestSequenceEquals(t *testing.T) {
	seq1 := NewSequenceFrom(NewRuneEvent('g', ModNone), NewRuneEvent('g', ModNone))
	seq2 := NewSequenceFrom(NewRuneEvent('g', ModNone), NewRuneEvent('g', ModNone))
	seq3 := NewSequenceFrom(NewRuneEvent('g', ModNone))
	seq4 := NewSequenceFrom(NewRuneEvent('d', ModNone), NewRuneEvent('d', ModNone))

	if !seq1.Equals(seq2) {
		t.Error("Identical sequences should be equal")
	}
	if seq1.Equals(seq3) {
		t.Error("Different length sequences should not be equal")
	}
	if seq1.Equals(seq4) {
		t.Error("Different content sequences should not be equal")
	}
	if seq1.Equals(nil) {
		t.Error("Sequence should not equal nil")
	}

	var nilSeq *Sequence
	if !nilSeq.Equals(nil) {
		t.Error("nil should equal nil")
	}
}

func TestSequenceHasPrefix(t *testing.T) {
	seq := NewSequenceFrom(
		NewRuneEvent('d', ModNone),
		NewRuneEvent('i', ModNone),
		NewRuneEvent('w', ModNone),
	)

	prefix1 := NewSequenceFrom(NewRuneEvent('d', ModNone))
	prefix2 := NewSequenceFrom(NewRuneEvent('d', ModNone), NewRuneEvent('i', ModNone))
	noMatch := NewSequenceFrom(NewRuneEvent('g', ModNone))
	tooLong := NewSequenceFrom(
		NewRuneEvent('d', ModNone),
		NewRuneEvent('i', ModNone),
		NewRuneEvent('w', ModNone),
		NewRuneEvent('x', ModNone),
	)

	if !seq.HasPrefix(prefix1) {
		t.Error("'diw' should have prefix 'd'")
	}
	if !seq.HasPrefix(prefix2) {
		t.Error("'diw' should have prefix 'di'")
	}
	if seq.HasPrefix(noMatch) {
		t.Error("'diw' should not have prefix 'g'")
	}
	if seq.HasPrefix(tooLong) {
		t.Error("'diw' should not have prefix 'diwx' (too long)")
	}
	if !seq.HasPrefix(nil) {
		t.Error("Any sequence should have nil prefix")
	}
	if !seq.HasPrefix(NewSequence()) {
		t.Error("Any sequence should have empty prefix")
	}
}

func TestSequenceStartsWith(t *testing.T) {
	seq := NewSequenceFrom(NewRuneEvent('d', ModNone), NewRuneEvent('d', ModNone))

	if !seq.StartsWith(NewRuneEvent('d', ModNone)) {
		t.Error("'dd' should start with 'd'")
	}
	if seq.StartsWith(NewRuneEvent('g', ModNone)) {
		t.Error("'dd' should not start with 'g'")
	}
	if NewSequence().StartsWith(NewRuneEvent('a', ModNone)) {
		t.Error("Empty sequence should not start with anything")
	}
}

func TestSequenceClone(t *testing.T) {
	seq := NewSequenceFrom(NewRuneEvent('a', ModNone), NewRuneEvent('b', ModNone))
	clone := seq.Clone()

	if !seq.Equals(clone) {
		t.Error("Clone should equal original")
	}

	// Modify original, clone should be unaffected
	seq.Add(NewRuneEvent('c', ModNone))
	if seq.Equals(clone) {
		t.Error("Clone should be independent of original")
	}

	var nilSeq *Sequence
	if nilSeq.Clone() != nil {
		t.Error("Clone of nil should be nil")
	}
}

func TestSequenceSlice(t *testing.T) {
	seq := NewSequenceFrom(
		NewRuneEvent('a', ModNone),
		NewRuneEvent('b', ModNone),
		NewRuneEvent('c', ModNone),
		NewRuneEvent('d', ModNone),
	)

	slice := seq.Slice(1, 3)
	if slice.Len() != 2 {
		t.Errorf("Slice(1,3) length = %d, want 2", slice.Len())
	}
	if slice.At(0).Rune != 'b' || slice.At(1).Rune != 'c' {
		t.Error("Slice(1,3) should contain 'b', 'c'")
	}

	// Edge cases
	if seq.Slice(-1, 2).Len() != 2 {
		t.Error("Slice with negative start should clamp to 0")
	}
	if seq.Slice(0, 10).Len() != 4 {
		t.Error("Slice with end > len should clamp")
	}
	if !seq.Slice(3, 2).IsEmpty() {
		t.Error("Slice with start >= end should return empty")
	}
}

func TestSequenceTailHead(t *testing.T) {
	seq := NewSequenceFrom(
		NewRuneEvent('a', ModNone),
		NewRuneEvent('b', ModNone),
		NewRuneEvent('c', ModNone),
	)

	tail := seq.Tail(1)
	if tail.Len() != 2 || tail.At(0).Rune != 'b' {
		t.Error("Tail(1) should skip first element")
	}

	head := seq.Head(2)
	if head.Len() != 2 || head.At(1).Rune != 'b' {
		t.Error("Head(2) should contain first two elements")
	}
}

func TestSequenceAppend(t *testing.T) {
	seq1 := NewSequenceFrom(NewRuneEvent('a', ModNone))
	seq2 := NewSequenceFrom(NewRuneEvent('b', ModNone), NewRuneEvent('c', ModNone))

	result := seq1.Append(seq2)
	if result.Len() != 3 {
		t.Errorf("Append length = %d, want 3", result.Len())
	}
	if result.At(0).Rune != 'a' || result.At(1).Rune != 'b' || result.At(2).Rune != 'c' {
		t.Error("Append should concatenate sequences")
	}

	// Original should be unchanged
	if seq1.Len() != 1 {
		t.Error("Original sequence should be unchanged")
	}

	// Append nil/empty
	if seq1.Append(nil).Len() != 1 {
		t.Error("Append(nil) should return clone")
	}
	if seq1.Append(NewSequence()).Len() != 1 {
		t.Error("Append(empty) should return clone")
	}
}

func TestSequenceContainsOnlyRunes(t *testing.T) {
	runeOnly := NewSequenceFrom(
		NewRuneEvent('a', ModNone),
		NewRuneEvent('b', ModNone),
	)
	if !runeOnly.ContainsOnlyRunes() {
		t.Error("Sequence with only runes should return true")
	}

	withSpecial := NewSequenceFrom(
		NewRuneEvent('a', ModNone),
		NewSpecialEvent(KeyEscape, ModNone),
	)
	if withSpecial.ContainsOnlyRunes() {
		t.Error("Sequence with special key should return false")
	}

	empty := NewSequence()
	if empty.ContainsOnlyRunes() {
		t.Error("Empty sequence should return false")
	}
}

func TestSequenceAsString(t *testing.T) {
	seq := NewSequenceFrom(
		NewRuneEvent('h', ModNone),
		NewRuneEvent('e', ModNone),
		NewRuneEvent('l', ModNone),
		NewRuneEvent('l', ModNone),
		NewRuneEvent('o', ModNone),
	)

	str, ok := seq.AsString()
	if !ok || str != "hello" {
		t.Errorf("AsString() = %q, %v, want 'hello', true", str, ok)
	}

	// With modifier - should fail
	withMod := NewSequenceFrom(NewRuneEvent('a', ModCtrl))
	_, ok = withMod.AsString()
	if ok {
		t.Error("AsString with modifier should return false")
	}

	// With special key - should fail
	withSpecial := NewSequenceFrom(NewSpecialEvent(KeyEscape, ModNone))
	_, ok = withSpecial.AsString()
	if ok {
		t.Error("AsString with special key should return false")
	}

	// Empty - should fail
	_, ok = NewSequence().AsString()
	if ok {
		t.Error("AsString on empty should return false")
	}
}
