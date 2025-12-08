package topic

// Matcher provides efficient topic pattern matching using a trie data structure.
// It is safe for concurrent use.
//
// Matcher wraps the Trie type with a more convenient API for the event bus.
type Matcher struct {
	trie *Trie
}

// NewMatcher creates a new topic matcher.
func NewMatcher() *Matcher {
	return &Matcher{
		trie: NewTrie(),
	}
}

// Add adds a pattern to the matcher.
// The pattern may contain wildcards (* and **).
func (m *Matcher) Add(pattern Topic) {
	m.trie.Insert(pattern)
}

// Remove removes a pattern from the matcher.
func (m *Matcher) Remove(pattern Topic) {
	m.trie.Delete(pattern)
}

// Has returns true if the pattern exists in the matcher.
func (m *Matcher) Has(pattern Topic) bool {
	return m.trie.Contains(pattern)
}

// Match returns all patterns that match the given topic.
// The topic should not contain wildcards - it represents an actual event topic.
func (m *Matcher) Match(eventTopic Topic) []Topic {
	return m.trie.Match(eventTopic)
}

// MatchExact returns true if there is an exact pattern match (no wildcards).
func (m *Matcher) MatchExact(topic Topic) bool {
	return m.trie.MatchExact(topic)
}

// Patterns returns all patterns in the matcher.
func (m *Matcher) Patterns() []Topic {
	return m.trie.All()
}

// Count returns the number of patterns in the matcher.
func (m *Matcher) Count() int {
	return m.trie.Size()
}

// Clear removes all patterns from the matcher.
func (m *Matcher) Clear() {
	m.trie.Clear()
}
