package fuzzy

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMatcherBasic(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := []Item{
		{Text: "main.go", Data: 1},
		{Text: "handler.go", Data: 2},
		{Text: "config.go", Data: 3},
		{Text: "utils.go", Data: 4},
	}

	tests := []struct {
		query       string
		wantFirst   string
		wantMatches int
	}{
		{"main", "main.go", 1},
		{"go", "main.go", 4}, // All have .go, shorter text scores higher
		{"han", "handler.go", 1},
		{"xyz", "", 0},
		{"", "main.go", 4}, // Empty returns all
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := matcher.Match(tt.query, items, 10)
			if len(results) != tt.wantMatches {
				t.Errorf("query %q: got %d matches, want %d", tt.query, len(results), tt.wantMatches)
			}
			if tt.wantMatches > 0 && results[0].Item.Text != tt.wantFirst {
				t.Errorf("query %q: got first %q, want %q", tt.query, results[0].Item.Text, tt.wantFirst)
			}
		})
	}
}

func TestMatcherCaseInsensitive(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := []Item{
		{Text: "MainController.go"},
		{Text: "main.go"},
	}

	// Case-insensitive by default
	results := matcher.Match("main", items, 10)
	if len(results) != 2 {
		t.Errorf("expected 2 matches, got %d", len(results))
	}

	// main.go should score higher (shorter, exact prefix)
	if results[0].Item.Text != "main.go" {
		t.Errorf("expected main.go first, got %s", results[0].Item.Text)
	}
}

func TestMatcherCaseSensitive(t *testing.T) {
	opts := DefaultOptions()
	opts.CaseSensitive = true
	matcher := NewMatcher(opts)

	items := []Item{
		{Text: "MainController.go"},
		{Text: "main.go"},
	}

	// Only lowercase should match
	results := matcher.Match("main", items, 10)
	if len(results) != 1 {
		t.Errorf("expected 1 match, got %d", len(results))
	}
	if results[0].Item.Text != "main.go" {
		t.Errorf("expected main.go, got %s", results[0].Item.Text)
	}
}

func TestMatcherFuzzyMatch(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := []Item{
		{Text: "FileController.go"},
		{Text: "file.go"},
		{Text: "config.go"},
	}

	// "fc" should match FileController (F_ile C_ontroller)
	results := matcher.Match("fc", items, 10)
	if len(results) == 0 {
		t.Fatal("expected matches for 'fc'")
	}
	if results[0].Item.Text != "FileController.go" {
		t.Errorf("expected FileController.go first, got %s", results[0].Item.Text)
	}
}

func TestMatcherLimit(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := make([]Item, 100)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("file%d.go", i)}
	}

	results := matcher.Match("file", items, 5)
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestMatcherUTF8(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := []Item{
		{Text: "日本語ファイル.txt"},
		{Text: "中文文件.txt"},
		{Text: "한국어파일.txt"},
		{Text: "Файл.txt"},
	}

	tests := []struct {
		query     string
		wantFirst string
	}{
		{"日本", "日本語ファイル.txt"},
		{"文件", "中文文件.txt"},
		{"파일", "한국어파일.txt"},
		{"Фай", "Файл.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := matcher.Match(tt.query, items, 10)
			if len(results) == 0 {
				t.Fatalf("expected match for %q", tt.query)
			}
			if results[0].Item.Text != tt.wantFirst {
				t.Errorf("expected %q, got %q", tt.wantFirst, results[0].Item.Text)
			}
		})
	}
}

func TestMatcherWordBoundary(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := []Item{
		{Text: "getUserById"},
		{Text: "getuser"},
	}

	// "gub" should prefer getUserById due to word boundary matches
	results := matcher.Match("gub", items, 10)
	if len(results) == 0 {
		t.Fatal("expected matches")
	}
	if results[0].Item.Text != "getUserById" {
		t.Errorf("expected getUserById first (word boundary), got %s", results[0].Item.Text)
	}
}

func TestMatcherDeterministicOrder(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := []Item{
		{Text: "alpha.go"},
		{Text: "bravo.go"},
		{Text: "charlie.go"},
	}

	// Run multiple times to verify deterministic ordering
	for i := 0; i < 5; i++ {
		results := matcher.Match("go", items, 10)
		if len(results) != 3 {
			t.Fatal("expected 3 results")
		}
		// All have same score for "go", should be alphabetical
		if results[0].Item.Text != "alpha.go" {
			t.Errorf("iteration %d: expected alpha.go first, got %s", i, results[0].Item.Text)
		}
	}
}

func TestScorerConsecutiveBonus(t *testing.T) {
	scorer := DefaultScorer{}

	// "abc" matching "abc" consecutively vs "a_b_c" with gaps
	query := []rune("abc")

	text1 := []rune("abc")
	matches1 := []int{0, 1, 2}
	score1 := scorer.Score(query, text1, text1, matches1)

	text2 := []rune("a_b_c")
	matches2 := []int{0, 2, 4}
	score2 := scorer.Score(query, text2, text2, matches2)

	if score1 <= score2 {
		t.Errorf("consecutive match should score higher: %d vs %d", score1, score2)
	}
}

func TestScorerPrefixBonus(t *testing.T) {
	scorer := DefaultScorer{}

	query := []rune("test")

	// Prefix match
	text1 := []rune("testing")
	matches1 := []int{0, 1, 2, 3}
	score1 := scorer.Score(query, text1, text1, matches1)

	// Non-prefix match
	text2 := []rune("_testing")
	matches2 := []int{1, 2, 3, 4}
	score2 := scorer.Score(query, text2, text2, matches2)

	if score1 <= score2 {
		t.Errorf("prefix match should score higher: %d vs %d", score1, score2)
	}
}

func TestWeightedScorer(t *testing.T) {
	scorer := DefaultWeights()

	query := []rune("abc")
	text := []rune("abc")
	matches := []int{0, 1, 2}

	score := scorer.Score(query, text, text, matches)
	if score <= 0 {
		t.Errorf("expected positive score, got %d", score)
	}
}

func TestFilePathScorer(t *testing.T) {
	scorer := NewFilePathScorer()

	query := []rune("main")

	// Filename match - "main" is in the filename
	path1 := []rune("src/pkg/main.go")
	matches1 := findMatches(query, path1)
	score1 := scorer.Score(query, path1, path1, matches1)

	// Verify filename match gets bonus
	// "main" matches at indices 8,9,10,11 which is after the last separator at 7
	if len(matches1) != 4 {
		t.Fatalf("expected 4 matches, got %d", len(matches1))
	}

	// Verify we get a reasonable score
	if score1 <= 0 {
		t.Errorf("expected positive score, got %d", score1)
	}

	// Filename with same query should score higher than longer path
	path2 := []rune("src/packages/something/main.go")
	matches2 := findMatches(query, path2)
	score2 := scorer.Score(query, path2, path2, matches2)

	// Shorter path should score higher (same filename position)
	if score1 <= score2 {
		t.Errorf("shorter path should score higher: %d vs %d", score1, score2)
	}
}

// findMatches helper for testing
func findMatches(query, text []rune) []int {
	matches := make([]int, 0, len(query))
	qi := 0
	for i := 0; i < len(text) && qi < len(query); i++ {
		if text[i] == query[qi] {
			matches = append(matches, i)
			qi++
		}
	}
	return matches
}

func TestCacheBasic(t *testing.T) {
	cache := NewCache(10)

	// Set and get
	results := []Result{
		{Item: Item{Text: "test"}, Score: 100},
	}
	cache.Set("query", results)

	got := cache.Get("query")
	if got == nil {
		t.Fatal("expected cached result")
	}
	if len(got) != 1 || got[0].Item.Text != "test" {
		t.Errorf("unexpected cached result: %+v", got)
	}

	// Miss
	if cache.Get("other") != nil {
		t.Error("expected cache miss")
	}
}

func TestCacheLRU(t *testing.T) {
	cache := NewCache(3)

	// Fill cache
	cache.Set("a", []Result{{Item: Item{Text: "a"}}})
	cache.Set("b", []Result{{Item: Item{Text: "b"}}})
	cache.Set("c", []Result{{Item: Item{Text: "c"}}})

	// Access "a" to make it recently used
	cache.Get("a")

	// Add new item, should evict "b" (least recently used)
	cache.Set("d", []Result{{Item: Item{Text: "d"}}})

	if cache.Get("b") != nil {
		t.Error("expected 'b' to be evicted")
	}
	if cache.Get("a") == nil {
		t.Error("expected 'a' to still be cached")
	}
	if cache.Get("c") == nil {
		t.Error("expected 'c' to still be cached")
	}
	if cache.Get("d") == nil {
		t.Error("expected 'd' to be cached")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewCache(10)

	cache.Set("a", []Result{})
	cache.Set("b", []Result{})

	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("expected empty cache, got %d items", cache.Len())
	}
	if cache.Get("a") != nil {
		t.Error("expected cache miss after clear")
	}
}

func TestCacheResultCopy(t *testing.T) {
	cache := NewCache(10)

	original := []Result{
		{Item: Item{Text: "test"}, Score: 100, Matches: []int{0, 1}},
	}
	cache.Set("query", original)

	// Modify original
	original[0].Score = 999

	// Get should return copy
	got := cache.Get("query")
	if got[0].Score == 999 {
		t.Error("cache should store copy, not reference")
	}
}

func TestAsyncMatcherBasic(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())
	asyncMatcher := NewAsyncMatcher(matcher, 2)

	items := make([]Item, 1000)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("file%d.go", i)}
	}

	ctx := context.Background()
	results := asyncMatcher.MatchParallel(ctx, "file1", items, 10)

	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}

	// First result should match "file1.go" best
	found := false
	for _, r := range results {
		if r.Item.Text == "file1.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file1.go in results")
	}
}

func TestAsyncMatcherCancel(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())
	asyncMatcher := NewAsyncMatcher(matcher, 2)

	items := make([]Item, 100000)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("file%d.go", i)}
	}

	ctx, cancel := context.WithCancel(context.Background())
	results, _ := asyncMatcher.MatchAsync(ctx, "file", items, 1000)

	// Cancel immediately
	cancel()

	// Should receive some results (or none) without hanging
	count := 0
	timeout := time.After(100 * time.Millisecond)
loop:
	for {
		select {
		case _, ok := <-results:
			if !ok {
				break loop
			}
			count++
		case <-timeout:
			break loop
		}
	}

	// Just verify it didn't hang
	t.Logf("received %d results before cancel/timeout", count)
}

func TestStreamingMatcher(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())
	streaming := NewStreamingMatcher(matcher)

	items := []Item{
		{Text: "main.go"},
		{Text: "handler.go"},
	}

	results := streaming.Search("main", items, 10)

	count := 0
	for range results {
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 result, got %d", count)
	}
}

func TestStreamingMatcherCancel(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())
	streaming := NewStreamingMatcher(matcher)

	items := make([]Item, 10000)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("file%d.go", i)}
	}

	// Start first search
	_ = streaming.Search("file1", items, 100)

	// Start second search (should cancel first)
	results := streaming.Search("file2", items, 10)

	// Consume results
	for range results {
	}

	// Verify last query
	if streaming.LastQuery() != "file2" {
		t.Errorf("expected last query 'file2', got %q", streaming.LastQuery())
	}
}

func TestMatcherWithHighlight(t *testing.T) {
	matcher := NewMatcher(DefaultOptions())

	items := []Item{
		{Text: "main.go"},
	}

	highlight := func(text string, matches []int) string {
		if len(matches) == 0 {
			return text
		}

		var sb strings.Builder
		runes := []rune(text)
		matchSet := make(map[int]bool)
		for _, m := range matches {
			matchSet[m] = true
		}

		for i, r := range runes {
			if matchSet[i] {
				sb.WriteString("[")
				sb.WriteRune(r)
				sb.WriteString("]")
			} else {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}

	results := matcher.MatchWithHighlight("main", items, 10, highlight)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}

	expected := "[m][a][i][n].go"
	if results[0].Highlighted != expected {
		t.Errorf("expected %q, got %q", expected, results[0].Highlighted)
	}
}

// Benchmarks

func BenchmarkMatchSmall(b *testing.B) {
	matcher := NewMatcher(DefaultOptions())

	items := make([]Item, 100)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("file%d.go", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match("file5", items, 10)
	}
}

func BenchmarkMatchMedium(b *testing.B) {
	matcher := NewMatcher(DefaultOptions())

	items := make([]Item, 1000)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("path/to/file%d.go", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match("file5", items, 10)
	}
}

func BenchmarkMatchLarge(b *testing.B) {
	matcher := NewMatcher(DefaultOptions())

	items := make([]Item, 10000)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("src/pkg/component/file%d.go", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match("file123", items, 10)
	}
}

func BenchmarkMatchWithCache(b *testing.B) {
	matcher := NewMatcher(DefaultOptions())

	items := make([]Item, 10000)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("file%d.go", i)}
	}

	// Warm up cache
	matcher.Match("file5", items, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match("file5", items, 10)
	}
}

func BenchmarkMatchParallel(b *testing.B) {
	matcher := NewMatcher(DefaultOptions())
	asyncMatcher := NewAsyncMatcher(matcher, 0)

	items := make([]Item, 10000)
	for i := range items {
		items[i] = Item{Text: fmt.Sprintf("src/pkg/component/file%d.go", i)}
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		asyncMatcher.MatchParallel(ctx, "file123", items, 10)
	}
}

func BenchmarkScorer(b *testing.B) {
	scorer := DefaultScorer{}

	query := []rune("main")
	text := []rune("maincontroller")
	matches := []int{0, 1, 2, 3}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scorer.Score(query, text, text, matches)
	}
}

func BenchmarkCache(b *testing.B) {
	cache := NewCache(1000)

	results := []Result{
		{Item: Item{Text: "test"}, Score: 100},
	}

	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.Set(fmt.Sprintf("query%d", i%100), results)
		}
	})

	// Pre-populate for Get benchmark
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("query%d", i), results)
	}

	b.Run("Get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.Get(fmt.Sprintf("query%d", i%100))
		}
	})
}
