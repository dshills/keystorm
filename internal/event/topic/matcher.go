package topic

import "sync"

// Matcher provides efficient topic pattern matching using a trie data structure.
// It is safe for concurrent use.
type Matcher struct {
	mu   sync.RWMutex
	root *trieNode
}

// trieNode represents a node in the pattern trie.
type trieNode struct {
	children map[string]*trieNode
	patterns []Topic // Patterns that terminate at this node
}

// newTrieNode creates a new trie node.
func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[string]*trieNode),
	}
}

// NewMatcher creates a new topic matcher.
func NewMatcher() *Matcher {
	return &Matcher{
		root: newTrieNode(),
	}
}

// Add adds a pattern to the matcher.
// The pattern may contain wildcards (* and **).
func (m *Matcher) Add(pattern Topic) {
	if pattern == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	segments := pattern.Segments()
	node := m.root

	for _, seg := range segments {
		if node.children[seg] == nil {
			node.children[seg] = newTrieNode()
		}
		node = node.children[seg]
	}

	// Add pattern to leaf node (avoid duplicates)
	for _, p := range node.patterns {
		if p == pattern {
			return
		}
	}
	node.patterns = append(node.patterns, pattern)
}

// Remove removes a pattern from the matcher.
func (m *Matcher) Remove(pattern Topic) {
	if pattern == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	segments := pattern.Segments()
	node := m.root

	// Navigate to the node
	for _, seg := range segments {
		if node.children[seg] == nil {
			return // Pattern not found
		}
		node = node.children[seg]
	}

	// Remove pattern from leaf
	for i, p := range node.patterns {
		if p == pattern {
			node.patterns = append(node.patterns[:i], node.patterns[i+1:]...)
			break
		}
	}
}

// Has returns true if the pattern exists in the matcher.
func (m *Matcher) Has(pattern Topic) bool {
	if pattern == "" {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	segments := pattern.Segments()
	node := m.root

	for _, seg := range segments {
		if node.children[seg] == nil {
			return false
		}
		node = node.children[seg]
	}

	for _, p := range node.patterns {
		if p == pattern {
			return true
		}
	}
	return false
}

// Match returns all patterns that match the given topic.
// The topic should not contain wildcards - it represents an actual event topic.
func (m *Matcher) Match(eventTopic Topic) []Topic {
	if eventTopic == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []Topic
	segments := eventTopic.Segments()

	m.matchRecursive(m.root, segments, 0, &matches)

	return matches
}

// matchRecursive performs recursive pattern matching.
func (m *Matcher) matchRecursive(node *trieNode, segments []string, depth int, matches *[]Topic) {
	if node == nil {
		return
	}

	// If we've consumed all segments, collect patterns at this node
	if depth == len(segments) {
		*matches = append(*matches, node.patterns...)

		// Also check for ** wildcard that matches zero additional segments
		if child := node.children[WildcardMulti]; child != nil {
			// ** at the end can match zero segments
			m.matchRecursive(child, segments, depth, matches)
		}
		return
	}

	segment := segments[depth]

	// Exact match - continue down the tree
	if child := node.children[segment]; child != nil {
		m.matchRecursive(child, segments, depth+1, matches)
	}

	// Single wildcard (*) matches any one segment
	if child := node.children[WildcardSingle]; child != nil {
		m.matchRecursive(child, segments, depth+1, matches)
	}

	// Multi wildcard (**) matches zero or more segments
	if child := node.children[WildcardMulti]; child != nil {
		// Try matching 0, 1, 2, ... remaining segments
		for i := depth; i <= len(segments); i++ {
			m.matchRecursive(child, segments, i, matches)
		}
	}
}

// MatchExact returns true if there is an exact pattern match (no wildcards).
func (m *Matcher) MatchExact(topic Topic) bool {
	if topic == "" {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	segments := topic.Segments()
	node := m.root

	for _, seg := range segments {
		child := node.children[seg]
		if child == nil {
			return false
		}
		node = child
	}

	return len(node.patterns) > 0
}

// Patterns returns all patterns in the matcher.
func (m *Matcher) Patterns() []Topic {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var patterns []Topic
	m.collectPatterns(m.root, &patterns)
	return patterns
}

// collectPatterns recursively collects all patterns from the trie.
func (m *Matcher) collectPatterns(node *trieNode, patterns *[]Topic) {
	if node == nil {
		return
	}

	*patterns = append(*patterns, node.patterns...)

	for _, child := range node.children {
		m.collectPatterns(child, patterns)
	}
}

// Count returns the number of patterns in the matcher.
func (m *Matcher) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	m.countPatterns(m.root, &count)
	return count
}

// countPatterns recursively counts patterns in the trie.
func (m *Matcher) countPatterns(node *trieNode, count *int) {
	if node == nil {
		return
	}

	*count += len(node.patterns)

	for _, child := range node.children {
		m.countPatterns(child, count)
	}
}

// Clear removes all patterns from the matcher.
func (m *Matcher) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.root = newTrieNode()
}
