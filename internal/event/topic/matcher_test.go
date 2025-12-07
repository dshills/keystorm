package topic

import (
	"sync"
	"testing"
)

func TestMatcher_Add(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.content.deleted"))
	m.Add(Topic("config.changed"))

	if !m.Has(Topic("buffer.content.inserted")) {
		t.Error("expected matcher to have buffer.content.inserted")
	}
	if !m.Has(Topic("buffer.content.deleted")) {
		t.Error("expected matcher to have buffer.content.deleted")
	}
	if !m.Has(Topic("config.changed")) {
		t.Error("expected matcher to have config.changed")
	}
	if m.Has(Topic("cursor.moved")) {
		t.Error("expected matcher to not have cursor.moved")
	}
}

func TestMatcher_Add_Duplicate(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.content.inserted"))

	if m.Count() != 1 {
		t.Errorf("expected count 1, got %d", m.Count())
	}
}

func TestMatcher_Add_Empty(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic(""))

	if m.Count() != 0 {
		t.Errorf("expected count 0 after adding empty topic, got %d", m.Count())
	}
}

func TestMatcher_Remove(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.content.deleted"))

	m.Remove(Topic("buffer.content.inserted"))

	if m.Has(Topic("buffer.content.inserted")) {
		t.Error("expected matcher to not have buffer.content.inserted after removal")
	}
	if !m.Has(Topic("buffer.content.deleted")) {
		t.Error("expected matcher to still have buffer.content.deleted")
	}
}

func TestMatcher_Remove_NonExistent(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))

	// Should not panic
	m.Remove(Topic("cursor.moved"))
	m.Remove(Topic("buffer.content.deleted"))

	if !m.Has(Topic("buffer.content.inserted")) {
		t.Error("expected matcher to still have buffer.content.inserted")
	}
}

func TestMatcher_Match_Exact(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.content.deleted"))
	m.Add(Topic("config.changed"))

	matches := m.Match(Topic("buffer.content.inserted"))

	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0] != Topic("buffer.content.inserted") {
		t.Errorf("expected match buffer.content.inserted, got %v", matches[0])
	}

	// No match
	matches = m.Match(Topic("cursor.moved"))
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestMatcher_Match_SingleWildcard(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.*.inserted"))
	m.Add(Topic("buffer.content.deleted"))
	m.Add(Topic("*.changed"))

	tests := []struct {
		topic         Topic
		expectedCount int
	}{
		{Topic("buffer.content.inserted"), 1},
		{Topic("buffer.text.inserted"), 1},
		{Topic("buffer.content.deleted"), 1},
		{Topic("config.changed"), 1},
		{Topic("cursor.changed"), 1},
		{Topic("buffer.changed"), 1}, // *.changed matches buffer.changed (2 segments each)
		{Topic("cursor.moved"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.topic.String(), func(t *testing.T) {
			matches := m.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, expected %d: %v", tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestMatcher_Match_MultiWildcard(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.**"))

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
			matches := m.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, expected %d: %v", tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestMatcher_Match_MultiWildcardAtEnd(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("**.inserted"))

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
			matches := m.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, expected %d: %v", tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestMatcher_Match_GlobalWildcard(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("**"))

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
			matches := m.Match(tt.topic)
			if len(matches) != tt.expectedCount {
				t.Errorf("Match(%q) returned %d matches, expected %d: %v", tt.topic, len(matches), tt.expectedCount, matches)
			}
		})
	}
}

func TestMatcher_Match_MultiplePatterns(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.*.inserted"))
	m.Add(Topic("buffer.**"))
	m.Add(Topic("**"))

	matches := m.Match(Topic("buffer.content.inserted"))

	// Should match all 4 patterns
	if len(matches) != 4 {
		t.Errorf("expected 4 matches, got %d: %v", len(matches), matches)
	}
}

func TestMatcher_Match_Empty(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))

	matches := m.Match(Topic(""))
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for empty topic, got %d", len(matches))
	}
}

func TestMatcher_MatchExact(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.*"))

	if !m.MatchExact(Topic("buffer.content.inserted")) {
		t.Error("expected exact match for buffer.content.inserted")
	}
	if m.MatchExact(Topic("buffer.content.deleted")) {
		t.Error("expected no exact match for buffer.content.deleted")
	}
}

func TestMatcher_Patterns(t *testing.T) {
	m := NewMatcher()

	patterns := []Topic{
		Topic("buffer.content.inserted"),
		Topic("buffer.content.deleted"),
		Topic("config.changed"),
	}

	for _, p := range patterns {
		m.Add(p)
	}

	got := m.Patterns()

	if len(got) != len(patterns) {
		t.Errorf("expected %d patterns, got %d", len(patterns), len(got))
	}

	// Check all patterns are present (order may vary)
	patternSet := make(map[Topic]bool)
	for _, p := range got {
		patternSet[p] = true
	}
	for _, p := range patterns {
		if !patternSet[p] {
			t.Errorf("expected pattern %q to be in result", p)
		}
	}
}

func TestMatcher_Count(t *testing.T) {
	m := NewMatcher()

	if m.Count() != 0 {
		t.Errorf("expected count 0, got %d", m.Count())
	}

	m.Add(Topic("buffer.content.inserted"))
	if m.Count() != 1 {
		t.Errorf("expected count 1, got %d", m.Count())
	}

	m.Add(Topic("config.changed"))
	if m.Count() != 2 {
		t.Errorf("expected count 2, got %d", m.Count())
	}

	m.Remove(Topic("buffer.content.inserted"))
	if m.Count() != 1 {
		t.Errorf("expected count 1 after removal, got %d", m.Count())
	}
}

func TestMatcher_Clear(t *testing.T) {
	m := NewMatcher()

	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("config.changed"))

	m.Clear()

	if m.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", m.Count())
	}
	if m.Has(Topic("buffer.content.inserted")) {
		t.Error("expected matcher to be empty after clear")
	}
}

func TestMatcher_Concurrent(t *testing.T) {
	m := NewMatcher()

	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				m.Add(Topic("buffer.content.inserted"))
				m.Add(Topic("config.changed"))
			}
		}(i)
	}

	// Concurrent matches
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = m.Match(Topic("buffer.content.inserted"))
			}
		}()
	}

	// Concurrent removes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				m.Remove(Topic("buffer.content.inserted"))
			}
		}()
	}

	wg.Wait()
}

func BenchmarkMatcher_Add(b *testing.B) {
	m := NewMatcher()
	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Add(topic)
	}
}

func BenchmarkMatcher_Match_Exact(b *testing.B) {
	m := NewMatcher()
	m.Add(Topic("buffer.content.inserted"))
	m.Add(Topic("buffer.content.deleted"))
	m.Add(Topic("config.changed"))
	m.Add(Topic("cursor.moved"))
	m.Add(Topic("project.file.opened"))

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Match(topic)
	}
}

func BenchmarkMatcher_Match_Wildcard(b *testing.B) {
	m := NewMatcher()
	m.Add(Topic("buffer.*"))
	m.Add(Topic("buffer.*.inserted"))
	m.Add(Topic("*.content.*"))

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Match(topic)
	}
}

func BenchmarkMatcher_Match_MultiWildcard(b *testing.B) {
	m := NewMatcher()
	m.Add(Topic("buffer.**"))
	m.Add(Topic("**.inserted"))
	m.Add(Topic("**"))

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Match(topic)
	}
}

func BenchmarkMatcher_Match_ManyPatterns(b *testing.B) {
	m := NewMatcher()

	// Add many patterns
	categories := []string{"buffer", "cursor", "config", "project", "plugin", "lsp", "terminal", "git", "debug", "task"}
	for _, cat := range categories {
		m.Add(Topic(cat + ".changed"))
		m.Add(Topic(cat + ".created"))
		m.Add(Topic(cat + ".deleted"))
		m.Add(Topic(cat + ".*"))
		m.Add(Topic(cat + ".**"))
	}

	topic := Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Match(topic)
	}
}
