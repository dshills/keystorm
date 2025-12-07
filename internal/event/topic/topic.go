package topic

import "strings"

// Topic represents a hierarchical event type using dot notation.
// Examples: "buffer.content.inserted", "config.changed", "plugin.vim-surround.activated"
type Topic string

// Wildcard constants for pattern matching.
const (
	// WildcardSingle matches exactly one segment.
	WildcardSingle = "*"

	// WildcardMulti matches zero or more segments.
	WildcardMulti = "**"

	// Separator is the character used to separate topic segments.
	Separator = "."
)

// String returns the topic as a string.
func (t Topic) String() string {
	return string(t)
}

// Segments returns the topic split by the separator.
func (t Topic) Segments() []string {
	if t == "" {
		return nil
	}
	return strings.Split(string(t), Separator)
}

// SegmentCount returns the number of segments in the topic.
func (t Topic) SegmentCount() int {
	if t == "" {
		return 0
	}
	return strings.Count(string(t), Separator) + 1
}

// Parent returns the parent topic by removing the last segment.
// Returns an empty topic if there is no parent.
//
// Example: "buffer.content.inserted" -> "buffer.content"
func (t Topic) Parent() Topic {
	s := string(t)
	idx := strings.LastIndex(s, Separator)
	if idx < 0 {
		return ""
	}
	return Topic(s[:idx])
}

// Child returns a child topic by appending a segment.
//
// Example: "buffer".Child("content") -> "buffer.content"
func (t Topic) Child(segment string) Topic {
	if t == "" {
		return Topic(segment)
	}
	return Topic(string(t) + Separator + segment)
}

// Base returns the last segment of the topic.
//
// Example: "buffer.content.inserted" -> "inserted"
func (t Topic) Base() string {
	s := string(t)
	idx := strings.LastIndex(s, Separator)
	if idx < 0 {
		return s
	}
	return s[idx+1:]
}

// HasPrefix returns true if the topic starts with the given prefix.
func (t Topic) HasPrefix(prefix Topic) bool {
	if prefix == "" {
		return true
	}
	s := string(t)
	p := string(prefix)
	if !strings.HasPrefix(s, p) {
		return false
	}
	// Ensure we're matching complete segments
	if len(s) == len(p) {
		return true
	}
	return s[len(p)] == '.'
}

// IsWildcard returns true if the topic contains any wildcard characters.
func (t Topic) IsWildcard() bool {
	return strings.Contains(string(t), WildcardSingle)
}

// IsValid returns true if the topic is valid.
// A valid topic:
//   - Is not empty
//   - Does not start or end with a separator
//   - Does not contain consecutive separators
//   - Does not contain empty segments
func (t Topic) IsValid() bool {
	s := string(t)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, Separator) || strings.HasSuffix(s, Separator) {
		return false
	}
	if strings.Contains(s, Separator+Separator) {
		return false
	}
	// Check for empty segments
	for _, seg := range t.Segments() {
		if seg == "" {
			return false
		}
	}
	return true
}

// Matches returns true if this topic matches the given pattern.
// The pattern may contain wildcards:
//   - "*" matches exactly one segment
//   - "**" matches zero or more segments
func (t Topic) Matches(pattern Topic) bool {
	return matchSegments(t.Segments(), pattern.Segments())
}

// matchSegments performs recursive pattern matching on topic segments.
func matchSegments(topic, pattern []string) bool {
	ti, pi := 0, 0

	for pi < len(pattern) {
		if pattern[pi] == WildcardMulti {
			// ** matches zero or more segments
			// Try matching 0, 1, 2, ... remaining topic segments
			for ti <= len(topic) {
				if matchSegments(topic[ti:], pattern[pi+1:]) {
					return true
				}
				ti++
			}
			return false
		}

		// Need a topic segment to match against
		if ti >= len(topic) {
			return false
		}

		if pattern[pi] == WildcardSingle {
			// * matches exactly one segment
			ti++
			pi++
		} else if pattern[pi] == topic[ti] {
			// Exact match
			ti++
			pi++
		} else {
			// No match
			return false
		}
	}

	// Pattern consumed - topic must also be consumed
	return ti == len(topic)
}

// Join joins multiple segments into a topic.
func Join(segments ...string) Topic {
	return Topic(strings.Join(segments, Separator))
}

// FromString creates a Topic from a string.
// This is mainly for clarity when converting from string literals.
func FromString(s string) Topic {
	return Topic(s)
}

// Split splits a topic string into segments.
// This is a convenience function that doesn't require creating a Topic first.
func Split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, Separator)
}
