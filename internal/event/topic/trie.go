package topic

import "sync"

// Trie is a thread-safe trie data structure optimized for topic pattern matching.
// It provides O(k) lookup where k is the number of topic segments.
//
// The trie stores patterns with wildcards (* and **) and can efficiently
// find all patterns that match a given concrete topic.
type Trie struct {
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

// isEmpty returns true if the node has no children and no patterns.
func (n *trieNode) isEmpty() bool {
	return len(n.children) == 0 && len(n.patterns) == 0
}

// NewTrie creates a new topic pattern trie.
func NewTrie() *Trie {
	return &Trie{
		root: newTrieNode(),
	}
}

// Insert adds a pattern to the trie.
// The pattern may contain wildcards (* and **).
// Returns true if the pattern was added, false if it already existed.
func (t *Trie) Insert(pattern Topic) bool {
	if pattern == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Initialize root if zero-value Trie is used
	if t.root == nil {
		t.root = newTrieNode()
	}

	segments := pattern.Segments()
	node := t.root

	for _, seg := range segments {
		if node.children[seg] == nil {
			node.children[seg] = newTrieNode()
		}
		node = node.children[seg]
	}

	// Check for duplicates
	for _, p := range node.patterns {
		if p == pattern {
			return false
		}
	}
	node.patterns = append(node.patterns, pattern)
	return true
}

// pathEntry tracks a node and the key used to reach it during traversal.
type pathEntry struct {
	node *trieNode
	key  string // the segment key used to reach this node from parent
}

// Delete removes a pattern from the trie and prunes empty nodes.
// Returns true if the pattern was removed, false if it didn't exist.
func (t *Trie) Delete(pattern Topic) bool {
	if pattern == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Nothing to delete if root doesn't exist
	if t.root == nil {
		return false
	}

	segments := pattern.Segments()

	// Track the path for pruning, storing both nodes and the keys used to reach them
	path := make([]pathEntry, 0, len(segments)+1)
	path = append(path, pathEntry{node: t.root, key: ""}) // root has no key

	node := t.root
	for _, seg := range segments {
		child := node.children[seg]
		if child == nil {
			return false // Pattern not found
		}
		path = append(path, pathEntry{node: child, key: seg})
		node = child
	}

	// Remove pattern from leaf
	found := false
	for i, p := range node.patterns {
		if p == pattern {
			node.patterns = append(node.patterns[:i], node.patterns[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return false
	}

	// Prune empty nodes from leaf back to root
	for i := len(path) - 1; i > 0; i-- {
		if !path[i].node.isEmpty() {
			break
		}
		// Remove this node from parent using the stored key
		parent := path[i-1].node
		delete(parent.children, path[i].key)
	}

	return true
}

// Contains returns true if the exact pattern exists in the trie.
func (t *Trie) Contains(pattern Topic) bool {
	if pattern == "" {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == nil {
		return false
	}

	segments := pattern.Segments()
	node := t.root

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

// matchState tracks the state during recursive matching to avoid duplicates.
type matchState struct {
	seen    map[Topic]struct{}
	matches []Topic
	visited map[visitKey]struct{} // memoization to avoid revisiting (node, depth) pairs
}

// visitKey is a composite key for memoization of (node pointer, depth) pairs.
// Using a struct as map key is efficient and avoids nested map allocations.
type visitKey struct {
	node  *trieNode
	depth int
}

// Match returns all patterns that match the given concrete topic.
// The topic should not contain wildcards - it represents an actual event topic.
// The returned patterns are unique (no duplicates).
//
// Time complexity: O(k * m) where k is the number of topic segments and m is
// the number of wildcard branches to explore. For most practical cases with
// limited wildcards, this approaches O(k).
func (t *Trie) Match(eventTopic Topic) []Topic {
	if eventTopic == "" {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == nil {
		return nil
	}

	state := &matchState{
		seen:    make(map[Topic]struct{}),
		matches: nil,
		visited: make(map[visitKey]struct{}),
	}
	segments := eventTopic.Segments()

	t.matchRecursive(t.root, segments, 0, state)

	return state.matches
}

// matchRecursive performs recursive pattern matching through the trie.
// Uses memoization via state.visited to avoid exponential blowup with ** wildcards.
func (t *Trie) matchRecursive(node *trieNode, segments []string, depth int, state *matchState) {
	if node == nil {
		return
	}

	// Check if we've already visited this (node, depth) combination
	key := visitKey{node: node, depth: depth}
	if _, seen := state.visited[key]; seen {
		return
	}
	state.visited[key] = struct{}{}

	// If we've consumed all segments, collect patterns at this node
	if depth == len(segments) {
		t.addPatterns(node.patterns, state)

		// Also check for ** wildcard that matches zero additional segments
		if child := node.children[WildcardMulti]; child != nil {
			t.matchRecursive(child, segments, depth, state)
		}
		return
	}

	segment := segments[depth]

	// Exact match - continue down the tree
	if child := node.children[segment]; child != nil {
		t.matchRecursive(child, segments, depth+1, state)
	}

	// Single wildcard (*) matches any one segment
	if child := node.children[WildcardSingle]; child != nil {
		t.matchRecursive(child, segments, depth+1, state)
	}

	// Multi wildcard (**) matches zero or more segments
	if child := node.children[WildcardMulti]; child != nil {
		// Try matching 0, 1, 2, ... remaining segments
		// Memoization prevents exponential blowup
		for i := depth; i <= len(segments); i++ {
			t.matchRecursive(child, segments, i, state)
		}
	}
}

// addPatterns adds patterns to the match state, avoiding duplicates.
func (t *Trie) addPatterns(patterns []Topic, state *matchState) {
	for _, p := range patterns {
		if _, seen := state.seen[p]; !seen {
			state.seen[p] = struct{}{}
			state.matches = append(state.matches, p)
		}
	}
}

// MatchExact returns true if there is a pattern that exactly matches the topic
// (without wildcard expansion).
func (t *Trie) MatchExact(topic Topic) bool {
	if topic == "" {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == nil {
		return false
	}

	segments := topic.Segments()
	node := t.root

	for _, seg := range segments {
		child := node.children[seg]
		if child == nil {
			return false
		}
		node = child
	}

	return len(node.patterns) > 0
}

// All returns all patterns stored in the trie.
func (t *Trie) All() []Topic {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var patterns []Topic
	t.collectPatterns(t.root, &patterns)
	return patterns
}

// collectPatterns recursively collects all patterns from the trie.
func (t *Trie) collectPatterns(node *trieNode, patterns *[]Topic) {
	if node == nil {
		return
	}

	*patterns = append(*patterns, node.patterns...)

	for _, child := range node.children {
		t.collectPatterns(child, patterns)
	}
}

// Size returns the number of patterns in the trie.
func (t *Trie) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	t.countPatterns(t.root, &count)
	return count
}

// countPatterns recursively counts patterns in the trie.
func (t *Trie) countPatterns(node *trieNode, count *int) {
	if node == nil {
		return
	}

	*count += len(node.patterns)

	for _, child := range node.children {
		t.countPatterns(child, count)
	}
}

// Clear removes all patterns from the trie.
func (t *Trie) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.root = newTrieNode()
}

// NodeCount returns the total number of nodes in the trie.
// This is useful for memory analysis.
func (t *Trie) NodeCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	t.countNodes(t.root, &count)
	return count
}

// countNodes recursively counts nodes in the trie.
func (t *Trie) countNodes(node *trieNode, count *int) {
	if node == nil {
		return
	}

	*count++

	for _, child := range node.children {
		t.countNodes(child, count)
	}
}
