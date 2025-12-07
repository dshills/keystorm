package event

import (
	"sort"
	"sync"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Registry manages subscriptions organized by topic pattern.
// It is thread-safe for concurrent access.
type Registry struct {
	mu      sync.RWMutex
	subs    map[topic.Topic][]*subscription
	byID    map[string]*subscription
	matcher *topic.Matcher
}

// NewRegistry creates a new subscription registry.
func NewRegistry() *Registry {
	return &Registry{
		subs:    make(map[topic.Topic][]*subscription),
		byID:    make(map[string]*subscription),
		matcher: topic.NewMatcher(),
	}
}

// Add adds a subscription for a topic pattern.
// The subscription is inserted in priority order (lower priority values first).
func (r *Registry) Add(sub *subscription) {
	r.mu.Lock()
	defer r.mu.Unlock()

	topicPattern := sub.Topic()

	// Add to topic-based map
	subs := r.subs[topicPattern]
	subs = append(subs, sub)

	// Sort by priority (lower values first)
	sort.Slice(subs, func(i, j int) bool {
		return subs[i].Config().Priority < subs[j].Config().Priority
	})

	r.subs[topicPattern] = subs

	// Add to ID-based map
	r.byID[sub.ID()] = sub

	// Add pattern to matcher
	r.matcher.Add(topicPattern)
}

// Remove removes a subscription by ID.
func (r *Registry) Remove(subID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	sub, exists := r.byID[subID]
	if !exists {
		return false
	}

	topicPattern := sub.Topic()

	// Remove from topic-based map
	subs := r.subs[topicPattern]
	for i, s := range subs {
		if s.ID() == subID {
			r.subs[topicPattern] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// Clean up empty topic entries
	if len(r.subs[topicPattern]) == 0 {
		delete(r.subs, topicPattern)
		r.matcher.Remove(topicPattern)
	}

	// Remove from ID-based map
	delete(r.byID, subID)

	return true
}

// Get returns a subscription by ID.
func (r *Registry) Get(subID string) (*subscription, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sub, exists := r.byID[subID]
	return sub, exists
}

// GetByTopic returns all subscriptions for a specific topic pattern.
// Returns a copy to prevent modification during iteration.
func (r *Registry) GetByTopic(topicPattern topic.Topic) []*subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subs := r.subs[topicPattern]
	if len(subs) == 0 {
		return nil
	}

	// Return copy to prevent races
	result := make([]*subscription, len(subs))
	copy(result, subs)
	return result
}

// Match returns all subscriptions that match the given event topic.
// Subscriptions are returned in priority order (across all matching patterns).
// This handles exact matches and wildcard patterns.
func (r *Registry) Match(eventTopic topic.Topic) []*subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get all matching patterns
	patterns := r.matcher.Match(eventTopic)
	if len(patterns) == 0 {
		return nil
	}

	// Collect all subscriptions from matching patterns
	var all []*subscription
	for _, pattern := range patterns {
		all = append(all, r.subs[pattern]...)
	}

	if len(all) == 0 {
		return nil
	}

	// Sort by priority (lower values first)
	sort.Slice(all, func(i, j int) bool {
		return all[i].Config().Priority < all[j].Config().Priority
	})

	// Return copy
	result := make([]*subscription, len(all))
	copy(result, all)
	return result
}

// MatchActive returns all active subscriptions that match the given event topic.
// This filters out paused and cancelled subscriptions.
func (r *Registry) MatchActive(eventTopic topic.Topic) []*subscription {
	all := r.Match(eventTopic)
	if len(all) == 0 {
		return nil
	}

	result := make([]*subscription, 0, len(all))
	for _, sub := range all {
		if sub.IsActive() {
			result = append(result, sub)
		}
	}
	return result
}

// Count returns the total number of subscriptions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.byID)
}

// CountByTopic returns the number of subscriptions for a specific topic pattern.
func (r *Registry) CountByTopic(topicPattern topic.Topic) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.subs[topicPattern])
}

// CountActive returns the number of active subscriptions.
func (r *Registry) CountActive() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, sub := range r.byID {
		if sub.IsActive() {
			count++
		}
	}
	return count
}

// All returns all subscriptions.
// Returns a copy to prevent modification during iteration.
func (r *Registry) All() []*subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.byID) == 0 {
		return nil
	}

	result := make([]*subscription, 0, len(r.byID))
	for _, sub := range r.byID {
		result = append(result, sub)
	}
	return result
}

// Topics returns all topic patterns with active subscriptions.
func (r *Registry) Topics() []topic.Topic {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.subs) == 0 {
		return nil
	}

	topics := make([]topic.Topic, 0, len(r.subs))
	for t := range r.subs {
		topics = append(topics, t)
	}
	return topics
}

// Clear removes all subscriptions.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.subs = make(map[topic.Topic][]*subscription)
	r.byID = make(map[string]*subscription)
	r.matcher.Clear()
}

// RemoveCancelled removes all cancelled subscriptions from the registry.
// Returns the number of subscriptions removed.
func (r *Registry) RemoveCancelled() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	for id, sub := range r.byID {
		if sub.IsCancelled() {
			topicPattern := sub.Topic()

			// Remove from topic-based map
			subs := r.subs[topicPattern]
			for i, s := range subs {
				if s.ID() == id {
					r.subs[topicPattern] = append(subs[:i], subs[i+1:]...)
					break
				}
			}

			// Clean up empty topic entries
			if len(r.subs[topicPattern]) == 0 {
				delete(r.subs, topicPattern)
				r.matcher.Remove(topicPattern)
			}

			// Remove from ID-based map
			delete(r.byID, id)
			removed++
		}
	}

	return removed
}
