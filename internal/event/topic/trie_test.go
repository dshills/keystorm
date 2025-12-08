package topic

import (
	"fmt"
	"sync"
	"testing"
)

func TestTrie_ZeroValue(t *testing.T) {
	// Test that zero-value Trie is safe to use
	var trie Trie

	// All methods should not panic and return sensible defaults
	if trie.Contains(Topic("test")) {
		t.Error("Contains should return false for zero-value trie")
	}
	if trie.Delete(Topic("test")) {
		t.Error("Delete should return false for zero-value trie")
	}
	if matches := trie.Match(Topic("test")); len(matches) != 0 {
		t.Error("Match should return nil/empty for zero-value trie")
	}
	if trie.MatchExact(Topic("test")) {
		t.Error("MatchExact should return false for zero-value trie")
	}
	if patterns := trie.All(); len(patterns) != 0 {
		t.Error("All should return nil/empty for zero-value trie")
	}
	if trie.Size() != 0 {
		t.Error("Size should return 0 for zero-value trie")
	}
	if trie.NodeCount() != 0 {
		t.Error("NodeCount should return 0 for zero-value trie")
	}

	// Insert should work and initialize the trie
	if !trie.Insert(Topic("test.pattern")) {
		t.Error("Insert should succeed on zero-value trie")
	}
	if trie.Size() != 1 {
		t.Errorf("Size() = %d after insert, want 1", trie.Size())
	}
	if !trie.Contains(Topic("test.pattern")) {
		t.Error("Contains should return true after insert")
	}
}

func TestTrie_Insert(t *testing.T) {
	trie := NewTrie()

	tests := []struct {
		pattern  Topic
		expected bool
	}{
		{Topic("buffer.content.inserted"), true},
		{Topic("buffer.content.deleted"), true},
		{Topic("config.changed"), true},
		{Topic("buffer.content.inserted"), false}, // duplicate
		{Topic(""), false},                        // empty
	}

	for _, tt := range tests {
		t.Run(tt.pattern.String(), func(t *testing.T) {
			got := trie.Insert(tt.pattern)
			if got != tt.expected {
				t.Errorf("Insert(%q) = %v, want %v", tt.pattern, got, tt.expected)
			}
		})
	}

	if trie.Size() != 3 {
		t.Errorf("Size() = %d, want 3", trie.Size())
	}
}

func TestTrie_Delete(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.content.inserted"))
	trie.Insert(Topic("buffer.content.deleted"))

	tests := []struct {
		pattern  Topic
		expected bool
	}{
		{Topic("buffer.content.inserted"), true},
		{Topic("buffer.content.inserted"), false}, // already deleted
		{Topic("cursor.moved"), false},            // never existed
		{Topic(""), false},                        // empty
	}

	for _, tt := range tests {
		t.Run(tt.pattern.String(), func(t *testing.T) {
			got := trie.Delete(tt.pattern)
			if got != tt.expected {
				t.Errorf("Delete(%q) = %v, want %v", tt.pattern, got, tt.expected)
			}
		})
	}

	if trie.Size() != 1 {
		t.Errorf("Size() = %d, want 1", trie.Size())
	}
}

func TestTrie_Contains(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.content.inserted"))
	trie.Insert(Topic("buffer.*"))

	tests := []struct {
		pattern  Topic
		expected bool
	}{
		{Topic("buffer.content.inserted"), true},
		{Topic("buffer.*"), true},
		{Topic("buffer.content.deleted"), false},
		{Topic("cursor.moved"), false},
		{Topic(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern.String(), func(t *testing.T) {
			got := trie.Contains(tt.pattern)
			if got != tt.expected {
				t.Errorf("Contains(%q) = %v, want %v", tt.pattern, got, tt.expected)
			}
		})
	}
}

func TestTrie_Match_Exact(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.content.inserted"))
	trie.Insert(Topic("buffer.content.deleted"))
	trie.Insert(Topic("config.changed"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("buffer.content.inserted"), 1},
		{Topic("config.changed"), 1},
		{Topic("cursor.moved"), 0},
		{Topic(""), 0},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := trie.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, want %d: %v",
					tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestTrie_Match_SingleWildcard(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.*.inserted"))
	trie.Insert(Topic("buffer.content.deleted"))
	trie.Insert(Topic("*.changed"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("buffer.content.inserted"), 1},
		{Topic("buffer.text.inserted"), 1},
		{Topic("buffer.content.deleted"), 1},
		{Topic("config.changed"), 1},
		{Topic("cursor.changed"), 1},
		{Topic("buffer.changed"), 1}, // *.changed matches
		{Topic("cursor.moved"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := trie.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, want %d: %v",
					tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestTrie_Match_MultiWildcard(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.**"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("buffer"), 1},
		{Topic("buffer.content"), 1},
		{Topic("buffer.content.inserted"), 1},
		{Topic("buffer.a.b.c.d"), 1},
		{Topic("cursor.moved"), 0},
		{Topic("config.changed"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := trie.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, want %d: %v",
					tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestTrie_Match_MultiWildcardPrefix(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("**.inserted"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("inserted"), 1},
		{Topic("buffer.inserted"), 1},
		{Topic("buffer.content.inserted"), 1},
		{Topic("a.b.c.d.inserted"), 1},
		{Topic("buffer.deleted"), 0},
		{Topic("insertedx"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := trie.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, want %d: %v",
					tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestTrie_Match_GlobalWildcard(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("**"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("anything"), 1},
		{Topic("buffer.content.inserted"), 1},
		{Topic("a"), 1},
		{Topic("a.b.c.d.e.f"), 1},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := trie.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, want %d: %v",
					tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestTrie_Match_MultiplePatterns(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.content.inserted"))
	trie.Insert(Topic("buffer.*.inserted"))
	trie.Insert(Topic("buffer.**"))
	trie.Insert(Topic("**"))

	matches := trie.Match(Topic("buffer.content.inserted"))

	// Should match all 4 patterns
	if len(matches) != 4 {
		t.Errorf("expected 4 matches, got %d: %v", len(matches), matches)
	}
}

func TestTrie_Match_ComplexPatterns(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("a.*.c.**.e"))
	trie.Insert(Topic("**.middle.**"))
	trie.Insert(Topic("start.**.end"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("a.b.c.e"), 1},               // a.*.c.**.e matches
		{Topic("a.b.c.d.e"), 1},             // a.*.c.**.e matches
		{Topic("a.b.c.d.d.e"), 1},           // a.*.c.**.e matches
		{Topic("something.middle.else"), 1}, // **.middle.** matches
		{Topic("x.y.middle.z"), 1},          // **.middle.** matches
		{Topic("start.x.y.z.end"), 1},       // start.**.end matches
		{Topic("start.end"), 1},             // start.**.end matches (** matches zero)
		{Topic("nomatch"), 0},               // nothing matches
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := trie.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, want %d: %v",
					tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestTrie_MatchExact(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.content.inserted"))
	trie.Insert(Topic("buffer.*"))

	tests := []struct {
		topic    Topic
		expected bool
	}{
		{Topic("buffer.content.inserted"), true},
		{Topic("buffer.*"), true},
		{Topic("buffer.content.deleted"), false},
		{Topic("buffer"), false},
		{Topic(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			got := trie.MatchExact(tt.topic)
			if got != tt.expected {
				t.Errorf("MatchExact(%q) = %v, want %v", tt.topic, got, tt.expected)
			}
		})
	}
}

func TestTrie_All(t *testing.T) {
	trie := NewTrie()

	patterns := []Topic{
		Topic("buffer.content.inserted"),
		Topic("buffer.content.deleted"),
		Topic("config.changed"),
	}

	for _, p := range patterns {
		trie.Insert(p)
	}

	got := trie.All()

	if len(got) != len(patterns) {
		t.Errorf("All() returned %d patterns, want %d", len(got), len(patterns))
	}

	// Check all patterns are present (order may vary)
	patternSet := make(map[Topic]bool)
	for _, p := range got {
		patternSet[p] = true
	}
	for _, p := range patterns {
		if !patternSet[p] {
			t.Errorf("All() missing pattern %q", p)
		}
	}
}

func TestTrie_Size(t *testing.T) {
	trie := NewTrie()

	if trie.Size() != 0 {
		t.Errorf("Size() = %d, want 0", trie.Size())
	}

	trie.Insert(Topic("buffer.content.inserted"))
	if trie.Size() != 1 {
		t.Errorf("Size() = %d, want 1", trie.Size())
	}

	trie.Insert(Topic("config.changed"))
	if trie.Size() != 2 {
		t.Errorf("Size() = %d, want 2", trie.Size())
	}

	trie.Delete(Topic("buffer.content.inserted"))
	if trie.Size() != 1 {
		t.Errorf("Size() = %d, want 1", trie.Size())
	}
}

func TestTrie_Clear(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Topic("buffer.content.inserted"))
	trie.Insert(Topic("config.changed"))

	trie.Clear()

	if trie.Size() != 0 {
		t.Errorf("Size() = %d after clear, want 0", trie.Size())
	}
	if trie.Contains(Topic("buffer.content.inserted")) {
		t.Error("Trie should be empty after clear")
	}
}

func TestTrie_NodeCount(t *testing.T) {
	trie := NewTrie()

	initial := trie.NodeCount()
	if initial != 1 {
		t.Errorf("NodeCount() = %d for empty trie, want 1 (root)", initial)
	}

	// "a.b.c" should add 3 nodes
	trie.Insert(Topic("a.b.c"))
	if trie.NodeCount() != 4 { // root + 3 segments
		t.Errorf("NodeCount() = %d, want 4", trie.NodeCount())
	}

	// "a.b.d" should add 1 node (shares a.b)
	trie.Insert(Topic("a.b.d"))
	if trie.NodeCount() != 5 {
		t.Errorf("NodeCount() = %d, want 5", trie.NodeCount())
	}
}

func TestTrie_Concurrent(t *testing.T) {
	trie := NewTrie()

	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent inserts
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				trie.Insert(Topic("buffer.content.inserted"))
				trie.Insert(Topic("config.changed"))
			}
		}(i)
	}

	// Concurrent matches
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = trie.Match(Topic("buffer.content.inserted"))
			}
		}()
	}

	// Concurrent deletes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				trie.Delete(Topic("buffer.content.inserted"))
			}
		}()
	}

	wg.Wait()
}

// Node pruning tests

func TestTrie_Delete_PrunesEmptyNodes(t *testing.T) {
	trie := NewTrie()

	// Insert a pattern that creates nodes: a -> b -> c
	trie.Insert(Topic("a.b.c"))
	if trie.NodeCount() != 4 { // root + 3 nodes
		t.Errorf("NodeCount() = %d, want 4", trie.NodeCount())
	}

	// Delete the pattern - should prune all empty nodes
	trie.Delete(Topic("a.b.c"))
	if trie.NodeCount() != 1 { // only root remains
		t.Errorf("NodeCount() = %d after delete, want 1 (only root)", trie.NodeCount())
	}
}

func TestTrie_Delete_PreservesSharedNodes(t *testing.T) {
	trie := NewTrie()

	// Insert patterns that share nodes: a -> b -> c and a -> b -> d
	trie.Insert(Topic("a.b.c"))
	trie.Insert(Topic("a.b.d"))
	if trie.NodeCount() != 5 { // root + a + b + c + d
		t.Errorf("NodeCount() = %d, want 5", trie.NodeCount())
	}

	// Delete one pattern - should only prune the leaf
	trie.Delete(Topic("a.b.c"))
	if trie.NodeCount() != 4 { // root + a + b + d
		t.Errorf("NodeCount() = %d after delete, want 4", trie.NodeCount())
	}
	if !trie.Contains(Topic("a.b.d")) {
		t.Error("a.b.d should still exist")
	}
}

// Deduplication tests

func TestTrie_Match_NoDuplicates(t *testing.T) {
	trie := NewTrie()

	// Insert patterns that could potentially match the same topic multiple ways
	trie.Insert(Topic("**"))
	trie.Insert(Topic("a.**"))
	trie.Insert(Topic("**.c"))
	trie.Insert(Topic("a.*.c"))

	// This topic could potentially match "**" multiple ways through different recursion paths
	matches := trie.Match(Topic("a.b.c"))

	// Check for duplicates
	seen := make(map[Topic]int)
	for _, m := range matches {
		seen[m]++
		if seen[m] > 1 {
			t.Errorf("Duplicate pattern in matches: %q (appeared %d times)", m, seen[m])
		}
	}

	// Should match all 4 patterns
	if len(matches) != 4 {
		t.Errorf("Match returned %d matches, want 4: %v", len(matches), matches)
	}
}

func TestTrie_Match_ComplexWildcards_NoDuplicates(t *testing.T) {
	trie := NewTrie()

	// Many overlapping ** patterns
	trie.Insert(Topic("**"))
	trie.Insert(Topic("a.**"))
	trie.Insert(Topic("**.d"))
	trie.Insert(Topic("a.**.d"))

	// Long topic that could match through many paths
	matches := trie.Match(Topic("a.b.c.d"))

	// Check for duplicates
	seen := make(map[Topic]int)
	for _, m := range matches {
		seen[m]++
		if seen[m] > 1 {
			t.Errorf("Duplicate pattern in matches: %q (appeared %d times)", m, seen[m])
		}
	}
}

// linearMatcher provides a simple linear search implementation for benchmarking.
// It stores patterns in a slice and matches by iterating through all of them.
// This is the baseline implementation to compare against the trie.
type linearMatcher struct {
	mu       sync.RWMutex
	patterns []Topic
}

// newLinearMatcher creates a new linear matcher for benchmarking.
func newLinearMatcher() *linearMatcher {
	return &linearMatcher{
		patterns: make([]Topic, 0),
	}
}

// Add adds a pattern to the matcher.
func (lm *linearMatcher) Add(pattern Topic) bool {
	if pattern == "" {
		return false
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Check for duplicates
	for _, p := range lm.patterns {
		if p == pattern {
			return false
		}
	}
	lm.patterns = append(lm.patterns, pattern)
	return true
}

// Remove removes a pattern from the matcher.
func (lm *linearMatcher) Remove(pattern Topic) bool {
	if pattern == "" {
		return false
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	for i, p := range lm.patterns {
		if p == pattern {
			lm.patterns = append(lm.patterns[:i], lm.patterns[i+1:]...)
			return true
		}
	}
	return false
}

// Match returns all patterns that match the given topic using linear search.
// Time complexity: O(n * k) where n is the number of patterns and k is segments.
func (lm *linearMatcher) Match(eventTopic Topic) []Topic {
	if eventTopic == "" {
		return nil
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var matches []Topic
	for _, pattern := range lm.patterns {
		if eventTopic.Matches(pattern) {
			matches = append(matches, pattern)
		}
	}
	return matches
}

// Size returns the number of patterns.
func (lm *linearMatcher) Size() int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return len(lm.patterns)
}

// Clear removes all patterns.
func (lm *linearMatcher) Clear() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.patterns = nil // Release memory
}

// linearMatcher tests (for benchmarking comparison)

func TestLinearMatcher_Add(t *testing.T) {
	lm := newLinearMatcher()

	if !lm.Add(Topic("buffer.content.inserted")) {
		t.Error("Add should return true for new pattern")
	}
	if lm.Add(Topic("buffer.content.inserted")) {
		t.Error("Add should return false for duplicate")
	}
	if lm.Add(Topic("")) {
		t.Error("Add should return false for empty pattern")
	}

	if lm.Size() != 1 {
		t.Errorf("Size() = %d, want 1", lm.Size())
	}
}

func TestLinearMatcher_Remove(t *testing.T) {
	lm := newLinearMatcher()

	lm.Add(Topic("buffer.content.inserted"))

	if !lm.Remove(Topic("buffer.content.inserted")) {
		t.Error("Remove should return true for existing pattern")
	}
	if lm.Remove(Topic("buffer.content.inserted")) {
		t.Error("Remove should return false for non-existing pattern")
	}
	if lm.Remove(Topic("")) {
		t.Error("Remove should return false for empty pattern")
	}
}

func TestLinearMatcher_Match(t *testing.T) {
	lm := newLinearMatcher()

	lm.Add(Topic("buffer.content.inserted"))
	lm.Add(Topic("buffer.*"))
	lm.Add(Topic("buffer.**"))
	lm.Add(Topic("**"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("buffer.content.inserted"), 3}, // buffer.** and **
		{Topic("buffer.content"), 3},          // buffer.* (2 segments), buffer.**, **
		{Topic("config.changed"), 1},          // only **
		{Topic(""), 0},
	}

	for _, tt := range tests {
		tt := tt // capture loop variable
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := lm.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, want %d: %v",
					tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestLinearMatcher_Clear(t *testing.T) {
	lm := newLinearMatcher()

	lm.Add(Topic("buffer.content.inserted"))
	lm.Add(Topic("config.changed"))

	lm.Clear()

	if lm.Size() != 0 {
		t.Errorf("Size() = %d after clear, want 0", lm.Size())
	}
}

// Benchmarks comparing Trie vs LinearMatcher

func BenchmarkTrie_Insert(b *testing.B) {
	trie := NewTrie()
	pattern := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Insert(pattern)
	}
}

func BenchmarkLinearMatcher_Add(b *testing.B) {
	lm := newLinearMatcher()
	pattern := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lm.Add(pattern)
	}
}

func BenchmarkTrie_Match_FewPatterns(b *testing.B) {
	trie := NewTrie()
	trie.Insert(Topic("buffer.content.inserted"))
	trie.Insert(Topic("buffer.content.deleted"))
	trie.Insert(Topic("config.changed"))
	trie.Insert(Topic("cursor.moved"))
	trie.Insert(Topic("project.file.opened"))

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.Match(topic)
	}
}

func BenchmarkLinearMatcher_Match_FewPatterns(b *testing.B) {
	lm := newLinearMatcher()
	lm.Add(Topic("buffer.content.inserted"))
	lm.Add(Topic("buffer.content.deleted"))
	lm.Add(Topic("config.changed"))
	lm.Add(Topic("cursor.moved"))
	lm.Add(Topic("project.file.opened"))

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.Match(topic)
	}
}

func BenchmarkTrie_Match_ManyPatterns(b *testing.B) {
	trie := NewTrie()

	// Add many patterns
	categories := []string{"buffer", "cursor", "config", "project", "plugin", "lsp", "terminal", "git", "debug", "task"}
	for _, cat := range categories {
		trie.Insert(Topic(cat + ".changed"))
		trie.Insert(Topic(cat + ".created"))
		trie.Insert(Topic(cat + ".deleted"))
		trie.Insert(Topic(cat + ".*"))
		trie.Insert(Topic(cat + ".**"))
	}

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.Match(topic)
	}
}

func BenchmarkLinearMatcher_Match_ManyPatterns(b *testing.B) {
	lm := newLinearMatcher()

	// Add many patterns
	categories := []string{"buffer", "cursor", "config", "project", "plugin", "lsp", "terminal", "git", "debug", "task"}
	for _, cat := range categories {
		lm.Add(Topic(cat + ".changed"))
		lm.Add(Topic(cat + ".created"))
		lm.Add(Topic(cat + ".deleted"))
		lm.Add(Topic(cat + ".*"))
		lm.Add(Topic(cat + ".**"))
	}

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.Match(topic)
	}
}

func BenchmarkTrie_Match_VeryManyPatterns(b *testing.B) {
	trie := NewTrie()

	// Add 1000 patterns
	for i := 0; i < 100; i++ {
		for j := 0; j < 10; j++ {
			trie.Insert(Topic(fmt.Sprintf("category%d.subcategory%d.action", i, j)))
		}
	}

	topic := Topic("category50.subcategory5.action")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.Match(topic)
	}
}

func BenchmarkLinearMatcher_Match_VeryManyPatterns(b *testing.B) {
	lm := newLinearMatcher()

	// Add 1000 patterns
	for i := 0; i < 100; i++ {
		for j := 0; j < 10; j++ {
			lm.Add(Topic(fmt.Sprintf("category%d.subcategory%d.action", i, j)))
		}
	}

	topic := Topic("category50.subcategory5.action")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.Match(topic)
	}
}

func BenchmarkTrie_Match_WithWildcards(b *testing.B) {
	trie := NewTrie()
	trie.Insert(Topic("buffer.*"))
	trie.Insert(Topic("buffer.*.inserted"))
	trie.Insert(Topic("*.content.*"))
	trie.Insert(Topic("buffer.**"))
	trie.Insert(Topic("**.inserted"))

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.Match(topic)
	}
}

func BenchmarkLinearMatcher_Match_WithWildcards(b *testing.B) {
	lm := newLinearMatcher()
	lm.Add(Topic("buffer.*"))
	lm.Add(Topic("buffer.*.inserted"))
	lm.Add(Topic("*.content.*"))
	lm.Add(Topic("buffer.**"))
	lm.Add(Topic("**.inserted"))

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.Match(topic)
	}
}

func BenchmarkTrie_Match_GlobalWildcard(b *testing.B) {
	trie := NewTrie()
	trie.Insert(Topic("**"))

	topic := Topic("buffer.content.inserted.something.deep")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.Match(topic)
	}
}

func BenchmarkLinearMatcher_Match_GlobalWildcard(b *testing.B) {
	lm := newLinearMatcher()
	lm.Add(Topic("**"))

	topic := Topic("buffer.content.inserted.something.deep")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.Match(topic)
	}
}

func BenchmarkTrie_Match_DeepTopic(b *testing.B) {
	trie := NewTrie()
	trie.Insert(Topic("a.b.c.d.e.f.g.h.i.j"))
	trie.Insert(Topic("a.b.c.d.*.*.*.*.*.*"))
	trie.Insert(Topic("a.**"))

	topic := Topic("a.b.c.d.e.f.g.h.i.j")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.Match(topic)
	}
}

func BenchmarkLinearMatcher_Match_DeepTopic(b *testing.B) {
	lm := newLinearMatcher()
	lm.Add(Topic("a.b.c.d.e.f.g.h.i.j"))
	lm.Add(Topic("a.b.c.d.*.*.*.*.*.*"))
	lm.Add(Topic("a.**"))

	topic := Topic("a.b.c.d.e.f.g.h.i.j")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.Match(topic)
	}
}

// Memory benchmarks

func BenchmarkTrie_Memory(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		trie := NewTrie()
		for j := 0; j < 100; j++ {
			trie.Insert(Topic(fmt.Sprintf("category%d.subcategory.action", j)))
		}
	}
}

func BenchmarkLinearMatcher_Memory(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		lm := newLinearMatcher()
		for j := 0; j < 100; j++ {
			lm.Add(Topic(fmt.Sprintf("category%d.subcategory.action", j)))
		}
	}
}
