package fuzzy

import (
	"sort"
	"strings"
	"sync"
)

// Item represents a searchable item.
type Item struct {
	// Text is the string to match against.
	Text string

	// Data is arbitrary data associated with this item.
	Data any
}

// Result represents a match result with scoring information.
type Result struct {
	// Item is the matched item.
	Item Item

	// Score is the match score (higher is better).
	Score int

	// Matches contains the rune indices of matched characters.
	Matches []int
}

// Matcher performs fuzzy string matching.
type Matcher struct {
	mu      sync.RWMutex
	cache   *Cache
	scorer  Scorer
	options Options
}

// Options configures the matcher behavior.
type Options struct {
	// CacheSize is the maximum number of cached query results.
	// Set to 0 to disable caching.
	CacheSize int

	// MinScore is the minimum score for a match to be included.
	// Default is 0 (include all matches).
	MinScore int

	// CaseSensitive enables case-sensitive matching.
	// Default is false (case-insensitive).
	CaseSensitive bool
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		CacheSize:     1000,
		MinScore:      0,
		CaseSensitive: false,
	}
}

// NewMatcher creates a new fuzzy matcher with the given options.
func NewMatcher(opts Options) *Matcher {
	var cache *Cache
	if opts.CacheSize > 0 {
		cache = NewCache(opts.CacheSize)
	}

	return &Matcher{
		cache:   cache,
		scorer:  DefaultScorer{},
		options: opts,
	}
}

// SetScorer sets a custom scoring algorithm.
func (m *Matcher) SetScorer(scorer Scorer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scorer = scorer
}

// Match finds items matching the query and returns results sorted by score.
func (m *Matcher) Match(query string, items []Item, limit int) []Result {
	// Normalize query
	if !m.options.CaseSensitive {
		query = strings.ToLower(query)
	}
	query = strings.TrimSpace(query)

	// Empty query returns first items with zero score
	if query == "" {
		return m.emptyQueryResults(items, limit)
	}

	// Check cache
	if m.cache != nil {
		if cached := m.cache.Get(query); cached != nil {
			return m.applyLimit(cached, limit)
		}
	}

	// Convert query to runes once
	queryRunes := []rune(query)

	// Match all items
	results := make([]Result, 0, len(items))
	for _, item := range items {
		score, matches := m.matchItem(queryRunes, item.Text)
		if score > m.options.MinScore {
			results = append(results, Result{
				Item:    item,
				Score:   score,
				Matches: matches,
			})
		}
	}

	// Sort by score descending, then by text for deterministic ordering
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Item.Text < results[j].Item.Text
	})

	// Cache results
	if m.cache != nil {
		m.cache.Set(query, results)
	}

	return m.applyLimit(results, limit)
}

// MatchWithHighlight matches items and returns results with highlighted text.
// The highlight function is called for each matched character position.
func (m *Matcher) MatchWithHighlight(query string, items []Item, limit int, highlight func(text string, matches []int) string) []struct {
	Result
	Highlighted string
} {
	results := m.Match(query, items, limit)

	highlighted := make([]struct {
		Result
		Highlighted string
	}, len(results))

	for i, r := range results {
		highlighted[i].Result = r
		highlighted[i].Highlighted = highlight(r.Item.Text, r.Matches)
	}

	return highlighted
}

// matchItem scores a single item against the query.
// Returns score and matched character indices (rune indices).
func (m *Matcher) matchItem(queryRunes []rune, text string) (int, []int) {
	if text == "" || len(queryRunes) == 0 {
		return 0, nil
	}

	// Convert text to runes for proper UTF-8 handling
	var textRunes []rune
	if m.options.CaseSensitive {
		textRunes = []rune(text)
	} else {
		textRunes = []rune(strings.ToLower(text))
	}
	originalRunes := []rune(text) // Keep original case for boundary detection

	// Find matching character positions using greedy left-to-right scan
	matches := make([]int, 0, len(queryRunes))
	queryIdx := 0

	for i := 0; i < len(textRunes) && queryIdx < len(queryRunes); i++ {
		if textRunes[i] == queryRunes[queryIdx] {
			matches = append(matches, i)
			queryIdx++
		}
	}

	// All query characters must match
	if queryIdx != len(queryRunes) {
		return 0, nil
	}

	// Calculate score
	m.mu.RLock()
	scorer := m.scorer
	m.mu.RUnlock()

	score := scorer.Score(queryRunes, originalRunes, textRunes, matches)
	return score, matches
}

// emptyQueryResults returns results for an empty query.
func (m *Matcher) emptyQueryResults(items []Item, limit int) []Result {
	count := len(items)
	if limit > 0 && limit < count {
		count = limit
	}

	results := make([]Result, count)
	for i := 0; i < count; i++ {
		results[i] = Result{
			Item:  items[i],
			Score: 0,
		}
	}
	return results
}

// applyLimit returns at most limit results.
func (m *Matcher) applyLimit(results []Result, limit int) []Result {
	if limit <= 0 || limit >= len(results) {
		return results
	}
	return results[:limit]
}

// ClearCache clears the result cache.
func (m *Matcher) ClearCache() {
	if m.cache != nil {
		m.cache.Clear()
	}
}
