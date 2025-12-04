package key

import (
	"strings"
)

// Sequence represents a series of key events forming a command.
// Examples: "g g" (go to top), "d i w" (delete inner word), "C-x C-s" (save)
type Sequence struct {
	// Events contains the key events in order.
	Events []Event
}

// NewSequence creates an empty key sequence.
func NewSequence() *Sequence {
	return &Sequence{
		Events: make([]Event, 0, 4), // Most sequences are short
	}
}

// NewSequenceFrom creates a sequence from the given events.
func NewSequenceFrom(events ...Event) *Sequence {
	return &Sequence{
		Events: events,
	}
}

// Len returns the number of events in the sequence.
func (s *Sequence) Len() int {
	return len(s.Events)
}

// IsEmpty returns true if the sequence has no events.
func (s *Sequence) IsEmpty() bool {
	return len(s.Events) == 0
}

// Add appends an event to the sequence.
func (s *Sequence) Add(event Event) {
	s.Events = append(s.Events, event)
}

// Clear removes all events from the sequence.
func (s *Sequence) Clear() {
	s.Events = s.Events[:0]
}

// Last returns the last event, or nil if empty.
func (s *Sequence) Last() *Event {
	if len(s.Events) == 0 {
		return nil
	}
	return &s.Events[len(s.Events)-1]
}

// First returns the first event, or nil if empty.
func (s *Sequence) First() *Event {
	if len(s.Events) == 0 {
		return nil
	}
	return &s.Events[0]
}

// At returns the event at the given index, or nil if out of bounds.
func (s *Sequence) At(index int) *Event {
	if index < 0 || index >= len(s.Events) {
		return nil
	}
	return &s.Events[index]
}

// String returns a human-readable representation.
// Examples: "g g", "d i w", "C-s"
func (s *Sequence) String() string {
	if len(s.Events) == 0 {
		return ""
	}

	parts := make([]string, len(s.Events))
	for i, e := range s.Events {
		parts[i] = e.String()
	}
	return strings.Join(parts, " ")
}

// VimString returns a Vim-style representation.
// Examples: "gg", "diw", "<C-s>"
func (s *Sequence) VimString() string {
	if len(s.Events) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, e := range s.Events {
		sb.WriteString(e.VimString())
	}
	return sb.String()
}

// Equals returns true if two sequences are identical.
func (s *Sequence) Equals(other *Sequence) bool {
	if s == nil || other == nil {
		return s == other
	}
	if len(s.Events) != len(other.Events) {
		return false
	}
	for i, e := range s.Events {
		if !e.Equals(other.Events[i]) {
			return false
		}
	}
	return true
}

// HasPrefix returns true if this sequence starts with the given prefix.
func (s *Sequence) HasPrefix(prefix *Sequence) bool {
	if prefix == nil || prefix.IsEmpty() {
		return true
	}
	if len(prefix.Events) > len(s.Events) {
		return false
	}
	for i, e := range prefix.Events {
		if !e.Equals(s.Events[i]) {
			return false
		}
	}
	return true
}

// StartsWith returns true if this sequence starts with the given event.
func (s *Sequence) StartsWith(event Event) bool {
	if len(s.Events) == 0 {
		return false
	}
	return s.Events[0].Equals(event)
}

// Clone returns a copy of the sequence.
func (s *Sequence) Clone() *Sequence {
	if s == nil {
		return nil
	}
	events := make([]Event, len(s.Events))
	copy(events, s.Events)
	return &Sequence{Events: events}
}

// Slice returns a new sequence containing events from start to end (exclusive).
func (s *Sequence) Slice(start, end int) *Sequence {
	if start < 0 {
		start = 0
	}
	if end > len(s.Events) {
		end = len(s.Events)
	}
	if start >= end {
		return NewSequence()
	}
	events := make([]Event, end-start)
	copy(events, s.Events[start:end])
	return &Sequence{Events: events}
}

// Tail returns a new sequence without the first n events.
func (s *Sequence) Tail(n int) *Sequence {
	return s.Slice(n, len(s.Events))
}

// Head returns a new sequence with only the first n events.
func (s *Sequence) Head(n int) *Sequence {
	return s.Slice(0, n)
}

// Append creates a new sequence by appending events from another sequence.
func (s *Sequence) Append(other *Sequence) *Sequence {
	if other == nil || other.IsEmpty() {
		return s.Clone()
	}

	events := make([]Event, len(s.Events)+len(other.Events))
	copy(events, s.Events)
	copy(events[len(s.Events):], other.Events)
	return &Sequence{Events: events}
}

// ContainsOnlyRunes returns true if the sequence contains only rune events.
func (s *Sequence) ContainsOnlyRunes() bool {
	for _, e := range s.Events {
		if !e.IsRune() {
			return false
		}
	}
	return len(s.Events) > 0
}

// AsString returns the sequence as a string if it contains only unmodified runes.
// Returns empty string and false if any event is not a simple rune.
func (s *Sequence) AsString() (string, bool) {
	if len(s.Events) == 0 {
		return "", false
	}

	var sb strings.Builder
	for _, e := range s.Events {
		if !e.IsRune() || e.IsModified() {
			return "", false
		}
		sb.WriteRune(e.Rune)
	}
	return sb.String(), true
}

// ParseSequence parses a key sequence string into a Sequence.
// The string can contain space-separated keys or a continuous Vim-style sequence.
// Examples: "g g", "d i w", "<C-x><C-s>", "dd"
func ParseSequence(s string) (*Sequence, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return NewSequence(), nil
	}

	seq := NewSequence()

	// Check for space-separated format first
	if strings.Contains(s, " ") {
		parts := strings.Fields(s)
		for _, part := range parts {
			event, err := Parse(part)
			if err != nil {
				return nil, err
			}
			seq.Add(event)
		}
		return seq, nil
	}

	// Parse as continuous sequence
	i := 0
	for i < len(s) {
		// Check for Vim-style <...> notation
		if s[i] == '<' {
			// Find matching >
			end := strings.IndexByte(s[i:], '>')
			if end == -1 {
				// No closing >, treat as literal <
				event := NewRuneEvent('<', ModNone)
				seq.Add(event)
				i++
				continue
			}

			event, err := Parse(s[i : i+end+1])
			if err != nil {
				return nil, err
			}
			seq.Add(event)
			i += end + 1
		} else {
			// Single character
			r := rune(s[i])
			event := NewRuneEvent(r, ModNone)
			seq.Add(event)
			i++
		}
	}

	return seq, nil
}

// MustParseSequence parses a sequence string and panics on error.
// Use only for known-valid sequences in initialization code.
func MustParseSequence(s string) *Sequence {
	seq, err := ParseSequence(s)
	if err != nil {
		panic("invalid key sequence: " + s + ": " + err.Error())
	}
	return seq
}
