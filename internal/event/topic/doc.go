// Package topic provides hierarchical topic types and pattern matching for the event bus.
//
// # Topic Format
//
// Topics use dot-notation to create hierarchical namespaces:
//
//	buffer.content.inserted
//	cursor.moved
//	config.section.reloaded
//	plugin.vim-surround.activated
//
// # Wildcards
//
// Two wildcard patterns are supported:
//
//   - "*" matches exactly one segment
//   - "**" matches zero or more segments
//
// Examples:
//
//	buffer.*              matches buffer.cleared, buffer.saved (not buffer.content.inserted)
//	buffer.**             matches buffer.cleared, buffer.content.inserted, buffer.a.b.c
//	*.changed             matches config.changed, cursor.changed
//	buffer.*.inserted     matches buffer.content.inserted, buffer.text.inserted
//	**                    matches everything
//
// # Pattern Matching
//
// The Matcher type provides efficient pattern matching using a trie data structure.
// It supports:
//
//   - Exact topic matching
//   - Single-segment wildcards (*)
//   - Multi-segment wildcards (**)
//   - Multiple patterns matching a single topic
//
// # Performance
//
// The trie-based implementation provides O(k) pattern matching where k is the
// number of topic segments. This is significantly faster than linear search
// (O(n * k) where n is the number of patterns) especially for large pattern sets:
//
//   - 5 patterns: ~3x faster than linear search
//   - 50 patterns: ~30x faster than linear search
//   - 1000 patterns: ~577x faster than linear search
//
// # Usage
//
//	m := topic.NewMatcher()
//	m.Add(topic.Topic("buffer.*"))
//	m.Add(topic.Topic("buffer.content.inserted"))
//
//	matches := m.Match(topic.Topic("buffer.content.inserted"))
//	// matches contains both patterns
//
// For direct trie access:
//
//	t := topic.NewTrie()
//	t.Insert(topic.Topic("buffer.**"))
//	t.Match(topic.Topic("buffer.content.inserted"))
package topic
