// Package fuzzy provides fast fuzzy string matching for editor features.
//
// The package implements a general-purpose fuzzy matching algorithm optimized
// for common editor use cases like file pickers, symbol search, and buffer
// switching. It is designed to handle 10k+ items with sub-50ms latency.
//
// # Features
//
//   - Fuzzy matching with intelligent scoring
//   - Result caching for repeated queries
//   - Async matching for large item sets
//   - Proper UTF-8/rune handling
//   - Configurable scoring weights
//
// # Scoring Algorithm
//
// The scorer favors matches based on several factors:
//   - Consecutive character matches (bonus)
//   - Word boundary matches (start of word, camelCase transitions)
//   - Prefix matches (query at start of text)
//   - Shorter text (more specific matches)
//   - Minimal gaps between matched characters
//
// # Usage
//
// Basic usage:
//
//	matcher := fuzzy.NewMatcher(fuzzy.DefaultOptions())
//	items := []fuzzy.Item{
//	    {Text: "main.go", Data: file1},
//	    {Text: "handler.go", Data: file2},
//	}
//	results := matcher.Match("main", items, 10)
//	for _, r := range results {
//	    fmt.Printf("%s (score: %d)\n", r.Item.Text, r.Score)
//	}
//
// For large item sets, use async matching:
//
//	results, cancel := matcher.MatchAsync(query, items, 10)
//	defer cancel()
//	for result := range results {
//	    // Process results as they arrive
//	}
//
// # Thread Safety
//
// The Matcher is safe for concurrent use. The cache is internally synchronized.
//
// # Performance
//
// Target performance characteristics:
//   - < 50ms for 10,000 items
//   - O(n*m) where n = items, m = avg text length
//   - Cache hit ratio improves repeated queries significantly
package fuzzy
