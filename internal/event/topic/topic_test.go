package topic

import (
	"testing"
)

func TestTopic_String(t *testing.T) {
	tests := []struct {
		topic    Topic
		expected string
	}{
		{Topic("buffer.content.inserted"), "buffer.content.inserted"},
		{Topic("config.changed"), "config.changed"},
		{Topic(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.topic.String(); got != tt.expected {
				t.Errorf("Topic.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTopic_Segments(t *testing.T) {
	tests := []struct {
		topic    Topic
		expected []string
	}{
		{Topic("buffer.content.inserted"), []string{"buffer", "content", "inserted"}},
		{Topic("config.changed"), []string{"config", "changed"}},
		{Topic("single"), []string{"single"}},
		{Topic(""), nil},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			got := tt.topic.Segments()
			if len(got) != len(tt.expected) {
				t.Errorf("Topic.Segments() = %v, want %v", got, tt.expected)
				return
			}
			for i, seg := range got {
				if seg != tt.expected[i] {
					t.Errorf("Topic.Segments()[%d] = %v, want %v", i, seg, tt.expected[i])
				}
			}
		})
	}
}

func TestTopic_SegmentCount(t *testing.T) {
	tests := []struct {
		topic    Topic
		expected int
	}{
		{Topic("buffer.content.inserted"), 3},
		{Topic("config.changed"), 2},
		{Topic("single"), 1},
		{Topic(""), 0},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			if got := tt.topic.SegmentCount(); got != tt.expected {
				t.Errorf("Topic.SegmentCount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTopic_Parent(t *testing.T) {
	tests := []struct {
		topic    Topic
		expected Topic
	}{
		{Topic("buffer.content.inserted"), Topic("buffer.content")},
		{Topic("config.changed"), Topic("config")},
		{Topic("single"), Topic("")},
		{Topic(""), Topic("")},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			if got := tt.topic.Parent(); got != tt.expected {
				t.Errorf("Topic.Parent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTopic_Child(t *testing.T) {
	tests := []struct {
		topic    Topic
		segment  string
		expected Topic
	}{
		{Topic("buffer"), "content", Topic("buffer.content")},
		{Topic("buffer.content"), "inserted", Topic("buffer.content.inserted")},
		{Topic(""), "buffer", Topic("buffer")},
	}

	for _, tt := range tests {
		t.Run(tt.expected.String(), func(t *testing.T) {
			if got := tt.topic.Child(tt.segment); got != tt.expected {
				t.Errorf("Topic.Child() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTopic_Base(t *testing.T) {
	tests := []struct {
		topic    Topic
		expected string
	}{
		{Topic("buffer.content.inserted"), "inserted"},
		{Topic("config.changed"), "changed"},
		{Topic("single"), "single"},
		{Topic(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			if got := tt.topic.Base(); got != tt.expected {
				t.Errorf("Topic.Base() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTopic_HasPrefix(t *testing.T) {
	tests := []struct {
		topic    Topic
		prefix   Topic
		expected bool
	}{
		{Topic("buffer.content.inserted"), Topic("buffer"), true},
		{Topic("buffer.content.inserted"), Topic("buffer.content"), true},
		{Topic("buffer.content.inserted"), Topic("buffer.content.inserted"), true},
		{Topic("buffer.content.inserted"), Topic("buff"), false},    // Not a complete segment
		{Topic("buffer.content.inserted"), Topic("content"), false}, // Not a prefix
		{Topic("buffer.content.inserted"), Topic("buffer.text"), false},
		{Topic("buffer"), Topic("buffer.content"), false}, // Prefix longer than topic
		{Topic("buffer.content"), Topic(""), true},        // Empty prefix matches all
	}

	for _, tt := range tests {
		t.Run(tt.topic.String()+"_"+tt.prefix.String(), func(t *testing.T) {
			if got := tt.topic.HasPrefix(tt.prefix); got != tt.expected {
				t.Errorf("Topic.HasPrefix(%v) = %v, want %v", tt.prefix, got, tt.expected)
			}
		})
	}
}

func TestTopic_IsWildcard(t *testing.T) {
	tests := []struct {
		topic    Topic
		expected bool
	}{
		{Topic("buffer.*"), true},
		{Topic("buffer.**"), true},
		{Topic("*.changed"), true},
		{Topic("buffer.*.inserted"), true},
		{Topic("buffer.content.inserted"), false},
		{Topic("config.changed"), false},
		{Topic(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			if got := tt.topic.IsWildcard(); got != tt.expected {
				t.Errorf("Topic.IsWildcard() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTopic_IsValid(t *testing.T) {
	tests := []struct {
		topic    Topic
		expected bool
	}{
		{Topic("buffer.content.inserted"), true},
		{Topic("config.changed"), true},
		{Topic("single"), true},
		{Topic("buffer.*"), true},
		{Topic("buffer.**"), true},
		{Topic(""), false},
		{Topic(".buffer"), false},
		{Topic("buffer."), false},
		{Topic("buffer..content"), false},
		{Topic("."), false},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			if got := tt.topic.IsValid(); got != tt.expected {
				t.Errorf("Topic.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTopic_Matches(t *testing.T) {
	tests := []struct {
		topic    Topic
		pattern  Topic
		expected bool
	}{
		// Exact matches
		{Topic("buffer.content.inserted"), Topic("buffer.content.inserted"), true},
		{Topic("config.changed"), Topic("config.changed"), true},
		{Topic("single"), Topic("single"), true},

		// Non-matches
		{Topic("buffer.content.inserted"), Topic("buffer.content.deleted"), false},
		{Topic("buffer.content.inserted"), Topic("cursor.moved"), false},
		{Topic("buffer"), Topic("buffer.content"), false},

		// Single wildcard (*)
		{Topic("buffer.content.inserted"), Topic("buffer.*.inserted"), true},
		{Topic("buffer.text.inserted"), Topic("buffer.*.inserted"), true},
		{Topic("buffer.content.deleted"), Topic("buffer.*.inserted"), false},
		{Topic("config.changed"), Topic("*.changed"), true},
		{Topic("cursor.changed"), Topic("*.changed"), true},
		{Topic("buffer.content"), Topic("*.*"), true},
		{Topic("buffer.content.inserted"), Topic("*.*"), false},

		// Multi wildcard (**)
		{Topic("buffer.content.inserted"), Topic("buffer.**"), true},
		{Topic("buffer.content"), Topic("buffer.**"), true},
		{Topic("buffer"), Topic("buffer.**"), true},
		{Topic("cursor.moved"), Topic("buffer.**"), false},
		{Topic("buffer.content.inserted"), Topic("**"), true},
		{Topic("single"), Topic("**"), true},

		// Combined wildcards
		{Topic("buffer.content.inserted"), Topic("**.inserted"), true},
		{Topic("a.b.c.inserted"), Topic("**.inserted"), true},
		{Topic("inserted"), Topic("**.inserted"), true},
		{Topic("buffer.content.deleted"), Topic("**.inserted"), false},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String()+"_matches_"+tt.pattern.String(), func(t *testing.T) {
			if got := tt.topic.Matches(tt.pattern); got != tt.expected {
				t.Errorf("Topic(%q).Matches(%q) = %v, want %v", tt.topic, tt.pattern, got, tt.expected)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		segments []string
		expected Topic
	}{
		{[]string{"buffer", "content", "inserted"}, Topic("buffer.content.inserted")},
		{[]string{"config", "changed"}, Topic("config.changed")},
		{[]string{"single"}, Topic("single")},
		{[]string{}, Topic("")},
	}

	for _, tt := range tests {
		t.Run(tt.expected.String(), func(t *testing.T) {
			if got := Join(tt.segments...); got != tt.expected {
				t.Errorf("Join() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFromString(t *testing.T) {
	s := "buffer.content.inserted"
	topic := FromString(s)
	if topic.String() != s {
		t.Errorf("FromString(%q) = %v, want %v", s, topic, s)
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"buffer.content.inserted", []string{"buffer", "content", "inserted"}},
		{"single", []string{"single"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Split(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("Split(%q) = %v, want %v", tt.input, got, tt.expected)
				return
			}
			for i, seg := range got {
				if seg != tt.expected[i] {
					t.Errorf("Split(%q)[%d] = %v, want %v", tt.input, i, seg, tt.expected[i])
				}
			}
		})
	}
}

func BenchmarkTopic_Segments(b *testing.B) {
	topic := Topic("buffer.content.inserted")
	for i := 0; i < b.N; i++ {
		_ = topic.Segments()
	}
}

func BenchmarkTopic_Matches_Exact(b *testing.B) {
	topic := Topic("buffer.content.inserted")
	pattern := Topic("buffer.content.inserted")
	for i := 0; i < b.N; i++ {
		_ = topic.Matches(pattern)
	}
}

func BenchmarkTopic_Matches_Wildcard(b *testing.B) {
	topic := Topic("buffer.content.inserted")
	pattern := Topic("buffer.*.*")
	for i := 0; i < b.N; i++ {
		_ = topic.Matches(pattern)
	}
}

func BenchmarkTopic_Matches_MultiWildcard(b *testing.B) {
	topic := Topic("buffer.content.inserted")
	pattern := Topic("buffer.**")
	for i := 0; i < b.N; i++ {
		_ = topic.Matches(pattern)
	}
}
