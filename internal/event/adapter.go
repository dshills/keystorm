package event

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dshills/keystorm/internal/event/topic"
)

// EventPublisher is the interface expected by the integration layer.
// This matches the existing integration.EventBus Publish signature.
type EventPublisher interface {
	Publish(eventType string, data map[string]any)
}

// BusAdapter adapts the new event.Bus to the integration layer's EventPublisher interface.
// It provides backward compatibility while allowing gradual migration to typed events.
type BusAdapter struct {
	bus    Bus
	source string
	closed atomic.Bool
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBusAdapter creates a new adapter wrapping the given bus.
// The source parameter identifies the origin of events (e.g., "integration").
func NewBusAdapter(bus Bus, source string) *BusAdapter {
	ctx, cancel := context.WithCancel(context.Background())
	return &BusAdapter{
		bus:    bus,
		source: source,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Publish implements the EventPublisher interface.
// It converts the string-based event type to topic.Topic and wraps the data in an Envelope.
func (a *BusAdapter) Publish(eventType string, data map[string]any) {
	if a.closed.Load() {
		return
	}

	env := Envelope{
		Topic:   topic.Topic(eventType),
		Payload: data,
		Metadata: Metadata{
			ID:        generateID(),
			Source:    a.source,
			Timestamp: timeNow(),
			Version:   1,
		},
	}

	// Use async publish to match existing integration behavior
	_ = a.bus.PublishAsync(a.ctx, env)
}

// PublishSync publishes an event synchronously.
func (a *BusAdapter) PublishSync(eventType string, data map[string]any) error {
	if a.closed.Load() {
		return ErrAdapterClosed
	}

	env := Envelope{
		Topic:   topic.Topic(eventType),
		Payload: data,
		Metadata: Metadata{
			ID:        generateID(),
			Source:    a.source,
			Timestamp: timeNow(),
			Version:   1,
		},
	}

	return a.bus.PublishSync(a.ctx, env)
}

// Close shuts down the adapter.
func (a *BusAdapter) Close() error {
	if a.closed.Swap(true) {
		return nil
	}
	a.cancel()
	return nil
}

// Bus returns the underlying event bus.
func (a *BusAdapter) Bus() Bus {
	return a.bus
}

// IntegrationSubscriber provides backward-compatible subscription for integration layer.
// It subscribes to the new typed bus but delivers events in the legacy format.
type IntegrationSubscriber struct {
	subscriber *Subscriber
	closed     atomic.Bool
	mu         sync.Mutex
	subIDs     map[string]Subscription
}

// NewIntegrationSubscriber creates a subscriber that works with legacy handlers.
func NewIntegrationSubscriber(bus Bus) *IntegrationSubscriber {
	return &IntegrationSubscriber{
		subscriber: NewSubscriber(bus),
		subIDs:     make(map[string]Subscription),
	}
}

// Subscribe registers a legacy-style handler for the given event type pattern.
// Returns a subscription ID for later unsubscription.
func (s *IntegrationSubscriber) Subscribe(eventType string, handler func(data map[string]any)) string {
	if s.closed.Load() {
		return ""
	}

	// Create a wrapper that extracts the payload
	wrappedHandler := HandlerFunc(func(ctx context.Context, event any) error {
		data := extractLegacyData(event)
		if data != nil {
			handler(data)
		}
		return nil
	})

	// Subscribe with async delivery to match existing behavior
	sub, err := s.subscriber.SubscribeAsync(topic.Topic(eventType), wrappedHandler)
	if err != nil {
		return ""
	}

	s.mu.Lock()
	s.subIDs[sub.ID()] = sub
	s.mu.Unlock()

	return sub.ID()
}

// Unsubscribe removes a subscription by ID.
func (s *IntegrationSubscriber) Unsubscribe(id string) bool {
	s.mu.Lock()
	sub, exists := s.subIDs[id]
	if exists {
		delete(s.subIDs, id)
	}
	s.mu.Unlock()

	if !exists {
		return false
	}

	return s.subscriber.Unsubscribe(sub) == nil
}

// Close shuts down the subscriber and removes all subscriptions.
func (s *IntegrationSubscriber) Close() error {
	if s.closed.Swap(true) {
		return nil
	}

	s.mu.Lock()
	// Unsubscribe all subscriptions before clearing
	for _, sub := range s.subIDs {
		_ = s.subscriber.Unsubscribe(sub)
	}
	s.subIDs = make(map[string]Subscription)
	s.mu.Unlock()

	return s.subscriber.Close()
}

// SubscriptionCount returns the number of active subscriptions.
func (s *IntegrationSubscriber) SubscriptionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subIDs)
}

// extractLegacyData extracts a map[string]any payload from various event types.
func extractLegacyData(event any) map[string]any {
	switch e := event.(type) {
	case Envelope:
		// If payload is already a map, use it directly
		if data, ok := e.Payload.(map[string]any); ok {
			return data
		}
		// Otherwise wrap the payload with metadata
		return map[string]any{
			"payload":   e.Payload,
			"topic":     string(e.Topic),
			"id":        e.Metadata.ID,
			"source":    e.Metadata.Source,
			"timestamp": e.Metadata.Timestamp,
		}
	case map[string]any:
		return e
	default:
		// Try to extract from TopicProvider
		if tp, ok := event.(TopicProvider); ok {
			return map[string]any{
				"topic":   string(tp.EventTopic()),
				"payload": event,
			}
		}
		// Last resort: wrap as payload
		return map[string]any{
			"payload": event,
		}
	}
}

// Bridge connects the integration layer's EventBus to the new typed event Bus.
// It allows events published on either bus to be delivered to subscribers on both.
type Bridge struct {
	typedBus          Bus
	legacyBus         EventPublisher
	adapter           *BusAdapter
	subscriber        *IntegrationSubscriber
	forwardSubscriber *Subscriber // Subscriber for typed -> legacy forwarding
	legacySubIDs      []string
	mu                sync.Mutex
	closed            atomic.Bool
	forwardTopics     []topic.Topic // Topics to forward from typed to legacy
	backwardTopics    []string      // Topics to forward from legacy to typed
}

// BridgeConfig configures a Bridge.
type BridgeConfig struct {
	// ForwardTopics are topic patterns to forward from typed bus to legacy bus.
	// Use wildcards like "terminal.*" or "**" for all.
	ForwardTopics []topic.Topic

	// BackwardTopics are event types to forward from legacy bus to typed bus.
	// These should match the string patterns used by the integration layer.
	BackwardTopics []string
}

// NewBridge creates a bidirectional bridge between buses.
func NewBridge(typedBus Bus, legacyBus EventPublisher, config BridgeConfig) *Bridge {
	return &Bridge{
		typedBus:       typedBus,
		legacyBus:      legacyBus,
		adapter:        NewBusAdapter(typedBus, "bridge"),
		subscriber:     NewIntegrationSubscriber(typedBus),
		forwardTopics:  config.ForwardTopics,
		backwardTopics: config.BackwardTopics,
	}
}

// Start activates the bridge.
// This sets up forwarding in both directions based on the configuration.
func (b *Bridge) Start() error {
	if b.closed.Load() {
		return ErrAdapterClosed
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Set up forward (typed -> legacy) subscriptions
	b.forwardSubscriber = NewSubscriber(b.typedBus)
	for _, t := range b.forwardTopics {
		_, err := b.forwardSubscriber.SubscribeAsyncFunc(t, func(ctx context.Context, event any) error {
			if b.closed.Load() {
				return nil
			}
			data := extractLegacyData(event)
			var eventType string
			if tp, ok := event.(TopicProvider); ok {
				eventType = string(tp.EventTopic())
			} else if env, ok := event.(Envelope); ok {
				eventType = string(env.Topic)
			}
			if eventType != "" && data != nil {
				b.legacyBus.Publish(eventType, data)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Note: Backward (legacy -> typed) forwarding requires the legacy bus
	// to support subscription, which the integration.EventBus does.
	// The integration code should set up these subscriptions explicitly.

	return nil
}

// Close shuts down the bridge.
func (b *Bridge) Close() error {
	if b.closed.Swap(true) {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	_ = b.adapter.Close()
	_ = b.subscriber.Close()
	if b.forwardSubscriber != nil {
		_ = b.forwardSubscriber.Close()
	}

	return nil
}

// TypedBus returns the typed event bus.
func (b *Bridge) TypedBus() Bus {
	return b.typedBus
}

// LegacyPublisher returns an EventPublisher that forwards to the typed bus.
func (b *Bridge) LegacyPublisher() EventPublisher {
	return b.adapter
}
