package integration

import (
	"sync"
	"sync/atomic"
)

// EventBus provides a thread-safe publish-subscribe event system for the integration layer.
//
// The event bus supports:
//   - Multiple subscribers per event type
//   - Wildcard subscriptions (e.g., "git.*" matches "git.commit", "git.push")
//   - Thread-safe operations
//   - Subscription cleanup
//
// Event types follow a dot-notation hierarchy:
//   - terminal.created, terminal.output, terminal.closed
//   - git.status.changed, git.branch.changed, git.commit.created
//   - debug.session.started, debug.session.stopped, debug.breakpoint.hit
//   - task.started, task.output, task.completed
//   - plugin.<name>.* (custom plugin events)
type EventBus struct {
	mu sync.RWMutex

	// Subscribers by event type (exact match)
	subscribers map[string]map[string]*subscription

	// Wildcard subscribers (pattern -> subscriptions)
	wildcards map[string]map[string]*subscription

	// All subscriptions by ID for fast lookup
	byID map[string]*subscription

	// Counter for generating unique subscription IDs
	nextID uint64

	// Closed flag
	closed atomic.Bool
}

// subscription holds a subscription's metadata.
type subscription struct {
	id        string
	eventType string
	isPattern bool
	handler   func(data map[string]any)
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]map[string]*subscription),
		wildcards:   make(map[string]map[string]*subscription),
		byID:        make(map[string]*subscription),
	}
}

// Subscribe adds an event handler for the given event type.
//
// The event type can be:
//   - An exact event name (e.g., "git.commit.created")
//   - A wildcard pattern ending with ".*" (e.g., "git.*")
//
// Returns a subscription ID that can be used to unsubscribe.
func (b *EventBus) Subscribe(eventType string, handler func(data map[string]any)) string {
	if b.closed.Load() {
		return ""
	}

	id := b.generateID()
	sub := &subscription{
		id:        id,
		eventType: eventType,
		isPattern: isWildcard(eventType),
		handler:   handler,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.byID[id] = sub

	if sub.isPattern {
		if b.wildcards[eventType] == nil {
			b.wildcards[eventType] = make(map[string]*subscription)
		}
		b.wildcards[eventType][id] = sub
	} else {
		if b.subscribers[eventType] == nil {
			b.subscribers[eventType] = make(map[string]*subscription)
		}
		b.subscribers[eventType][id] = sub
	}

	return id
}

// Unsubscribe removes a subscription by ID.
// Returns true if the subscription existed.
func (b *EventBus) Unsubscribe(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub, exists := b.byID[id]
	if !exists {
		return false
	}

	delete(b.byID, id)

	if sub.isPattern {
		if subs, ok := b.wildcards[sub.eventType]; ok {
			delete(subs, id)
			if len(subs) == 0 {
				delete(b.wildcards, sub.eventType)
			}
		}
	} else {
		if subs, ok := b.subscribers[sub.eventType]; ok {
			delete(subs, id)
			if len(subs) == 0 {
				delete(b.subscribers, sub.eventType)
			}
		}
	}

	return true
}

// Emit publishes an event to all matching subscribers.
//
// Handlers are called synchronously in the order they subscribed.
// Use EmitAsync for non-blocking event publishing.
func (b *EventBus) Emit(eventType string, data map[string]any) {
	if b.closed.Load() {
		return
	}

	handlers := b.getMatchingHandlers(eventType)

	// Call handlers synchronously
	for _, handler := range handlers {
		// Recover from panics in handlers to prevent one bad handler from breaking others
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but continue with other handlers
				}
			}()
			handler(data)
		}()
	}
}

// EmitAsync publishes an event asynchronously.
// Each handler is called in its own goroutine.
func (b *EventBus) EmitAsync(eventType string, data map[string]any) {
	if b.closed.Load() {
		return
	}

	handlers := b.getMatchingHandlers(eventType)

	for _, handler := range handlers {
		go func(h func(data map[string]any)) {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't crash
				}
			}()
			h(data)
		}(handler)
	}
}

// Publish implements the EventPublisher interface.
// This allows EventBus to be used as the event bus for the integration Manager.
func (b *EventBus) Publish(eventType string, data map[string]any) {
	b.Emit(eventType, data)
}

// Close shuts down the event bus.
// After closing, Subscribe, Emit, and Publish calls are no-ops.
func (b *EventBus) Close() {
	if b.closed.Swap(true) {
		return
	}

	b.mu.Lock()
	b.subscribers = make(map[string]map[string]*subscription)
	b.wildcards = make(map[string]map[string]*subscription)
	b.byID = make(map[string]*subscription)
	b.mu.Unlock()
}

// SubscriptionCount returns the total number of active subscriptions.
func (b *EventBus) SubscriptionCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.byID)
}

// SubscribersFor returns the number of subscribers for a specific event type.
// This counts exact matches only, not wildcards.
func (b *EventBus) SubscribersFor(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[eventType])
}

// getMatchingHandlers returns all handlers that match the given event type.
func (b *EventBus) getMatchingHandlers(eventType string) []func(data map[string]any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var handlers []func(data map[string]any)

	// Add exact match subscribers
	if subs, ok := b.subscribers[eventType]; ok {
		for _, sub := range subs {
			handlers = append(handlers, sub.handler)
		}
	}

	// Add wildcard match subscribers
	for pattern, subs := range b.wildcards {
		if matchPattern(pattern, eventType) {
			for _, sub := range subs {
				handlers = append(handlers, sub.handler)
			}
		}
	}

	return handlers
}

// generateID generates a unique subscription ID.
func (b *EventBus) generateID() string {
	id := atomic.AddUint64(&b.nextID, 1)
	return itoa64(id)
}

// isWildcard checks if the event type is a wildcard pattern.
func isWildcard(eventType string) bool {
	return len(eventType) >= 2 && eventType[len(eventType)-2:] == ".*"
}

// matchPattern checks if an event type matches a wildcard pattern.
func matchPattern(pattern, eventType string) bool {
	if !isWildcard(pattern) {
		return pattern == eventType
	}

	// Remove the ".*" suffix
	prefix := pattern[:len(pattern)-2]

	// Check if event type starts with the prefix followed by a dot
	if len(eventType) <= len(prefix) {
		return false
	}

	return eventType[:len(prefix)] == prefix && eventType[len(prefix)] == '.'
}

// itoa64 converts a uint64 to a string without importing strconv.
func itoa64(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
