package event

import (
	"context"
	"sync"
	"testing"

	"github.com/dshills/keystorm/internal/event/topic"
)

func newTestHandler() Handler {
	return HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}
}

func TestRegistry_Add(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("sub-1", topic.Topic("buffer.content.inserted"), newTestHandler())
	sub2 := newSubscription("sub-2", topic.Topic("config.changed"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)

	if r.Count() != 2 {
		t.Errorf("expected count 2, got %d", r.Count())
	}
}

func TestRegistry_Add_SameTopic(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("sub-1", topic.Topic("buffer.content.inserted"), newTestHandler())
	sub2 := newSubscription("sub-2", topic.Topic("buffer.content.inserted"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)

	if r.Count() != 2 {
		t.Errorf("expected count 2, got %d", r.Count())
	}

	byTopic := r.GetByTopic(topic.Topic("buffer.content.inserted"))
	if len(byTopic) != 2 {
		t.Errorf("expected 2 subscriptions for topic, got %d", len(byTopic))
	}
}

func TestRegistry_Add_PriorityOrder(t *testing.T) {
	r := NewRegistry()

	// Add in non-priority order
	subLow := newSubscription("low", topic.Topic("test"), newTestHandler(), WithPriority(PriorityLow))
	subHigh := newSubscription("high", topic.Topic("test"), newTestHandler(), WithPriority(PriorityHigh))
	subNormal := newSubscription("normal", topic.Topic("test"), newTestHandler(), WithPriority(PriorityNormal))
	subCritical := newSubscription("critical", topic.Topic("test"), newTestHandler(), WithPriority(PriorityCritical))

	r.Add(subLow)
	r.Add(subHigh)
	r.Add(subNormal)
	r.Add(subCritical)

	// Should be sorted by priority
	subs := r.GetByTopic(topic.Topic("test"))
	if len(subs) != 4 {
		t.Fatalf("expected 4 subscriptions, got %d", len(subs))
	}

	expectedOrder := []string{"critical", "high", "normal", "low"}
	for i, sub := range subs {
		if sub.ID() != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], sub.ID())
		}
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("sub-1", topic.Topic("test"), newTestHandler())
	sub2 := newSubscription("sub-2", topic.Topic("test"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)

	if !r.Remove("sub-1") {
		t.Error("expected Remove to return true for existing subscription")
	}

	if r.Count() != 1 {
		t.Errorf("expected count 1 after removal, got %d", r.Count())
	}

	// Try to remove non-existent
	if r.Remove("sub-1") {
		t.Error("expected Remove to return false for non-existent subscription")
	}

	if r.Remove("non-existent") {
		t.Error("expected Remove to return false for never-added subscription")
	}
}

func TestRegistry_Remove_LastForTopic(t *testing.T) {
	r := NewRegistry()

	sub := newSubscription("sub-1", topic.Topic("test"), newTestHandler())
	r.Add(sub)
	r.Remove("sub-1")

	// Topic should be cleaned up
	topics := r.Topics()
	for _, tp := range topics {
		if tp == topic.Topic("test") {
			t.Error("expected topic to be removed when last subscription removed")
		}
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	sub := newSubscription("sub-1", topic.Topic("test"), newTestHandler())
	r.Add(sub)

	got, exists := r.Get("sub-1")
	if !exists {
		t.Error("expected subscription to exist")
	}
	if got.ID() != "sub-1" {
		t.Errorf("expected ID sub-1, got %s", got.ID())
	}

	_, exists = r.Get("non-existent")
	if exists {
		t.Error("expected non-existent subscription to not exist")
	}
}

func TestRegistry_GetByTopic(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("sub-1", topic.Topic("buffer.changed"), newTestHandler())
	sub2 := newSubscription("sub-2", topic.Topic("buffer.changed"), newTestHandler())
	sub3 := newSubscription("sub-3", topic.Topic("config.changed"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)
	r.Add(sub3)

	bufferSubs := r.GetByTopic(topic.Topic("buffer.changed"))
	if len(bufferSubs) != 2 {
		t.Errorf("expected 2 buffer subscriptions, got %d", len(bufferSubs))
	}

	configSubs := r.GetByTopic(topic.Topic("config.changed"))
	if len(configSubs) != 1 {
		t.Errorf("expected 1 config subscription, got %d", len(configSubs))
	}

	noneSubs := r.GetByTopic(topic.Topic("cursor.moved"))
	if len(noneSubs) != 0 {
		t.Errorf("expected 0 cursor subscriptions, got %d", len(noneSubs))
	}
}

func TestRegistry_GetByTopic_ReturnsCopy(t *testing.T) {
	r := NewRegistry()

	sub := newSubscription("sub-1", topic.Topic("test"), newTestHandler())
	r.Add(sub)

	subs := r.GetByTopic(topic.Topic("test"))
	subs[0] = nil // Modify the slice

	// Original should be unaffected
	subs2 := r.GetByTopic(topic.Topic("test"))
	if subs2[0] == nil {
		t.Error("modifying returned slice should not affect registry")
	}
}

func TestRegistry_Match_ExactTopic(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("sub-1", topic.Topic("buffer.content.inserted"), newTestHandler())
	sub2 := newSubscription("sub-2", topic.Topic("config.changed"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)

	matches := r.Match(topic.Topic("buffer.content.inserted"))
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].ID() != "sub-1" {
		t.Errorf("expected sub-1, got %s", matches[0].ID())
	}
}

func TestRegistry_Match_Wildcard(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("exact", topic.Topic("buffer.content.inserted"), newTestHandler())
	sub2 := newSubscription("wildcard", topic.Topic("buffer.*"), newTestHandler())
	sub3 := newSubscription("multi", topic.Topic("buffer.**"), newTestHandler())
	sub4 := newSubscription("global", topic.Topic("**"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)
	r.Add(sub3)
	r.Add(sub4)

	// buffer.content.inserted should match: exact, multi, global (not wildcard - different segment count)
	matches := r.Match(topic.Topic("buffer.content.inserted"))

	matchIDs := make(map[string]bool)
	for _, m := range matches {
		matchIDs[m.ID()] = true
	}

	if !matchIDs["exact"] {
		t.Error("expected exact match")
	}
	if !matchIDs["multi"] {
		t.Error("expected multi-wildcard match")
	}
	if !matchIDs["global"] {
		t.Error("expected global wildcard match")
	}
}

func TestRegistry_Match_PriorityOrder(t *testing.T) {
	r := NewRegistry()

	// Add subscriptions with different priorities to different patterns
	subLow := newSubscription("low", topic.Topic("buffer.**"), newTestHandler(), WithPriority(PriorityLow))
	subHigh := newSubscription("high", topic.Topic("buffer.content.inserted"), newTestHandler(), WithPriority(PriorityHigh))
	subCritical := newSubscription("critical", topic.Topic("**"), newTestHandler(), WithPriority(PriorityCritical))

	r.Add(subLow)
	r.Add(subHigh)
	r.Add(subCritical)

	matches := r.Match(topic.Topic("buffer.content.inserted"))
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}

	// Should be sorted by priority across all patterns
	expectedOrder := []string{"critical", "high", "low"}
	for i, m := range matches {
		if m.ID() != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], m.ID())
		}
	}
}

func TestRegistry_MatchActive(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("active", topic.Topic("test"), newTestHandler())
	sub2 := newSubscription("paused", topic.Topic("test"), newTestHandler())
	sub3 := newSubscription("cancelled", topic.Topic("test"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)
	r.Add(sub3)

	sub2.Pause()
	sub3.Cancel()

	matches := r.MatchActive(topic.Topic("test"))
	if len(matches) != 1 {
		t.Errorf("expected 1 active match, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].ID() != "active" {
		t.Errorf("expected active subscription, got %s", matches[0].ID())
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()

	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}

	r.Add(newSubscription("1", topic.Topic("test"), newTestHandler()))
	r.Add(newSubscription("2", topic.Topic("test"), newTestHandler()))
	r.Add(newSubscription("3", topic.Topic("other"), newTestHandler()))

	if r.Count() != 3 {
		t.Errorf("expected count 3, got %d", r.Count())
	}
}

func TestRegistry_CountByTopic(t *testing.T) {
	r := NewRegistry()

	r.Add(newSubscription("1", topic.Topic("test"), newTestHandler()))
	r.Add(newSubscription("2", topic.Topic("test"), newTestHandler()))
	r.Add(newSubscription("3", topic.Topic("other"), newTestHandler()))

	if r.CountByTopic(topic.Topic("test")) != 2 {
		t.Errorf("expected 2 for test topic, got %d", r.CountByTopic(topic.Topic("test")))
	}
	if r.CountByTopic(topic.Topic("other")) != 1 {
		t.Errorf("expected 1 for other topic, got %d", r.CountByTopic(topic.Topic("other")))
	}
	if r.CountByTopic(topic.Topic("none")) != 0 {
		t.Errorf("expected 0 for none topic, got %d", r.CountByTopic(topic.Topic("none")))
	}
}

func TestRegistry_CountActive(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("1", topic.Topic("test"), newTestHandler())
	sub2 := newSubscription("2", topic.Topic("test"), newTestHandler())
	sub3 := newSubscription("3", topic.Topic("test"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)
	r.Add(sub3)

	if r.CountActive() != 3 {
		t.Errorf("expected 3 active, got %d", r.CountActive())
	}

	sub2.Pause()
	if r.CountActive() != 2 {
		t.Errorf("expected 2 active after pause, got %d", r.CountActive())
	}

	sub3.Cancel()
	if r.CountActive() != 1 {
		t.Errorf("expected 1 active after cancel, got %d", r.CountActive())
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()

	if all := r.All(); len(all) != 0 {
		t.Errorf("expected empty slice for empty registry, got %d", len(all))
	}

	r.Add(newSubscription("1", topic.Topic("a"), newTestHandler()))
	r.Add(newSubscription("2", topic.Topic("b"), newTestHandler()))
	r.Add(newSubscription("3", topic.Topic("c"), newTestHandler()))

	all := r.All()
	if len(all) != 3 {
		t.Errorf("expected 3 subscriptions, got %d", len(all))
	}

	// Verify all IDs are present
	ids := make(map[string]bool)
	for _, s := range all {
		ids[s.ID()] = true
	}
	for _, id := range []string{"1", "2", "3"} {
		if !ids[id] {
			t.Errorf("expected subscription %s in All()", id)
		}
	}
}

func TestRegistry_Topics(t *testing.T) {
	r := NewRegistry()

	if topics := r.Topics(); len(topics) != 0 {
		t.Errorf("expected empty topics for empty registry, got %d", len(topics))
	}

	r.Add(newSubscription("1", topic.Topic("buffer.changed"), newTestHandler()))
	r.Add(newSubscription("2", topic.Topic("buffer.changed"), newTestHandler()))
	r.Add(newSubscription("3", topic.Topic("config.changed"), newTestHandler()))

	topics := r.Topics()
	if len(topics) != 2 {
		t.Errorf("expected 2 unique topics, got %d", len(topics))
	}

	topicSet := make(map[topic.Topic]bool)
	for _, t := range topics {
		topicSet[t] = true
	}
	if !topicSet[topic.Topic("buffer.changed")] {
		t.Error("expected buffer.changed topic")
	}
	if !topicSet[topic.Topic("config.changed")] {
		t.Error("expected config.changed topic")
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := NewRegistry()

	r.Add(newSubscription("1", topic.Topic("test"), newTestHandler()))
	r.Add(newSubscription("2", topic.Topic("other"), newTestHandler()))

	r.Clear()

	if r.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", r.Count())
	}
	if len(r.Topics()) != 0 {
		t.Errorf("expected no topics after clear, got %d", len(r.Topics()))
	}
}

func TestRegistry_RemoveCancelled(t *testing.T) {
	r := NewRegistry()

	sub1 := newSubscription("active", topic.Topic("test"), newTestHandler())
	sub2 := newSubscription("cancelled1", topic.Topic("test"), newTestHandler())
	sub3 := newSubscription("cancelled2", topic.Topic("other"), newTestHandler())
	sub4 := newSubscription("paused", topic.Topic("test"), newTestHandler())

	r.Add(sub1)
	r.Add(sub2)
	r.Add(sub3)
	r.Add(sub4)

	sub2.Cancel()
	sub3.Cancel()
	sub4.Pause()

	removed := r.RemoveCancelled()
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
	if r.Count() != 2 {
		t.Errorf("expected count 2 after RemoveCancelled, got %d", r.Count())
	}

	// Verify correct ones remain
	if _, exists := r.Get("active"); !exists {
		t.Error("expected active subscription to remain")
	}
	if _, exists := r.Get("paused"); !exists {
		t.Error("expected paused subscription to remain")
	}
	if _, exists := r.Get("cancelled1"); exists {
		t.Error("expected cancelled1 to be removed")
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := NewRegistry()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				id := topic.Topic("buffer.content.inserted")
				sub := newSubscription(
					topic.Topic("sub").Child("test").Child(id.String()).String(),
					id,
					newTestHandler(),
				)
				r.Add(sub)
			}
		}(i)
	}

	// Concurrent matches
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = r.Match(topic.Topic("buffer.content.inserted"))
				_ = r.MatchActive(topic.Topic("buffer.content.inserted"))
			}
		}()
	}

	// Concurrent counts
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = r.Count()
				_ = r.CountActive()
				_ = r.Topics()
			}
		}()
	}

	wg.Wait()
}

func BenchmarkRegistry_Add(b *testing.B) {
	r := NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub := newSubscription("sub", topic.Topic("buffer.content.inserted"), newTestHandler())
		r.Add(sub)
	}
}

func BenchmarkRegistry_Match_Exact(b *testing.B) {
	r := NewRegistry()

	// Add some subscriptions
	topics := []string{
		"buffer.content.inserted",
		"buffer.content.deleted",
		"config.changed",
		"cursor.moved",
		"project.file.opened",
	}
	for i, t := range topics {
		sub := newSubscription(topic.Topic(t).Child(string(rune('0'+i))).String(), topic.Topic(t), newTestHandler())
		r.Add(sub)
	}

	eventTopic := topic.Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Match(eventTopic)
	}
}

func BenchmarkRegistry_Match_Wildcard(b *testing.B) {
	r := NewRegistry()

	// Add wildcard patterns
	patterns := []string{
		"buffer.**",
		"buffer.*",
		"**.inserted",
		"**",
	}
	for i, p := range patterns {
		sub := newSubscription(topic.Topic(p).Child(string(rune('0'+i))).String(), topic.Topic(p), newTestHandler())
		r.Add(sub)
	}

	eventTopic := topic.Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Match(eventTopic)
	}
}

func BenchmarkRegistry_Match_ManySubscriptions(b *testing.B) {
	r := NewRegistry()

	// Add many subscriptions across many patterns
	categories := []string{"buffer", "cursor", "config", "project", "plugin", "lsp", "terminal", "git", "debug", "task"}
	for _, cat := range categories {
		for j := 0; j < 10; j++ {
			t := topic.Topic(cat).Child("event").Child(string(rune('a' + j)))
			sub := newSubscription(t.String(), t, newTestHandler())
			r.Add(sub)
		}
		// Add wildcards
		sub := newSubscription(cat+"-wild", topic.Topic(cat+".**"), newTestHandler())
		r.Add(sub)
	}

	eventTopic := topic.Topic("buffer.content.inserted")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Match(eventTopic)
	}
}
