package event

import (
	"context"
	"sync"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Subscriber provides a simplified API for subscribing to events.
// It manages multiple subscriptions and provides cleanup on close.
type Subscriber struct {
	bus           Bus
	subscriptions []Subscription
	mu            sync.Mutex
	closed        bool
}

// NewSubscriber creates a new Subscriber wrapping the given bus.
func NewSubscriber(bus Bus) *Subscriber {
	return &Subscriber{
		bus:           bus,
		subscriptions: make([]Subscription, 0),
	}
}

// Subscribe creates a subscription for the given topic pattern.
// The subscription is tracked for cleanup when Close is called.
func (s *Subscriber) Subscribe(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) (Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrSubscriberClosed
	}

	sub, err := s.bus.Subscribe(topicPattern, handler, opts...)
	if err != nil {
		return nil, err
	}

	s.subscriptions = append(s.subscriptions, sub)
	return sub, nil
}

// SubscribeFunc creates a subscription with a function handler.
func (s *Subscriber) SubscribeFunc(topicPattern topic.Topic, fn HandlerFunc, opts ...SubscriptionOption) (Subscription, error) {
	return s.Subscribe(topicPattern, fn, opts...)
}

// SubscribeTyped creates a type-safe subscription for Event[T].
// The handler will only be called for events that match the type.
func SubscribeTyped[T any](s *Subscriber, topicPattern topic.Topic, handler TypedHandlerFunc[T], opts ...SubscriptionOption) (Subscription, error) {
	wrappedHandler := AsHandlerFunc(handler)
	return s.Subscribe(topicPattern, wrappedHandler, opts...)
}

// SubscribePayload creates a subscription that extracts and handles the payload directly.
// This is useful when you don't need access to the full Event[T] wrapper.
func SubscribePayload[T any](s *Subscriber, topicPattern topic.Topic, handler func(ctx context.Context, payload T) error, opts ...SubscriptionOption) (Subscription, error) {
	wrappedHandler := HandlerFunc(func(ctx context.Context, event any) error {
		// Try typed Event[T] first
		if e, ok := event.(Event[T]); ok {
			return handler(ctx, e.Payload)
		}
		// Try Envelope with typed payload
		if env, ok := event.(Envelope); ok {
			if payload, ok := env.Payload.(T); ok {
				return handler(ctx, payload)
			}
		}
		// Try direct payload
		if payload, ok := event.(T); ok {
			return handler(ctx, payload)
		}
		// Type mismatch - skip silently
		return nil
	})
	return s.Subscribe(topicPattern, wrappedHandler, opts...)
}

// SubscribeOnce creates a one-time subscription that auto-cancels after the first event.
func (s *Subscriber) SubscribeOnce(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) (Subscription, error) {
	opts = append(opts, WithOnce())
	return s.Subscribe(topicPattern, handler, opts...)
}

// SubscribeOnceFunc creates a one-time subscription with a function handler.
func (s *Subscriber) SubscribeOnceFunc(topicPattern topic.Topic, fn HandlerFunc, opts ...SubscriptionOption) (Subscription, error) {
	opts = append(opts, WithOnce())
	return s.SubscribeFunc(topicPattern, fn, opts...)
}

// SubscribeAsync creates an asynchronous subscription.
// Events will be delivered via the async worker pool.
func (s *Subscriber) SubscribeAsync(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) (Subscription, error) {
	opts = append(opts, WithDeliveryMode(DeliveryAsync))
	return s.Subscribe(topicPattern, handler, opts...)
}

// SubscribeAsyncFunc creates an asynchronous subscription with a function handler.
func (s *Subscriber) SubscribeAsyncFunc(topicPattern topic.Topic, fn HandlerFunc, opts ...SubscriptionOption) (Subscription, error) {
	opts = append(opts, WithDeliveryMode(DeliveryAsync))
	return s.SubscribeFunc(topicPattern, fn, opts...)
}

// SubscribeCritical creates a critical-priority subscription.
// Critical handlers execute first and are intended for core functionality.
func (s *Subscriber) SubscribeCritical(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) (Subscription, error) {
	opts = append(opts, WithPriority(PriorityCritical))
	return s.Subscribe(topicPattern, handler, opts...)
}

// SubscribeLow creates a low-priority subscription.
// Low-priority handlers execute last and are intended for metrics/logging.
func (s *Subscriber) SubscribeLow(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) (Subscription, error) {
	opts = append(opts, WithPriority(PriorityLow))
	return s.Subscribe(topicPattern, handler, opts...)
}

// SubscribeWithFilter creates a subscription with a filter predicate.
// The handler is only called for events that pass the filter.
func (s *Subscriber) SubscribeWithFilter(topicPattern topic.Topic, handler Handler, filter FilterFunc, opts ...SubscriptionOption) (Subscription, error) {
	opts = append(opts, WithFilter(filter))
	return s.Subscribe(topicPattern, handler, opts...)
}

// Unsubscribe removes a specific subscription.
func (s *Subscriber) Unsubscribe(sub Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from our tracking list
	for i, tracked := range s.subscriptions {
		if tracked.ID() == sub.ID() {
			s.subscriptions = append(s.subscriptions[:i], s.subscriptions[i+1:]...)
			break
		}
	}

	return s.bus.Unsubscribe(sub)
}

// UnsubscribeAll removes all subscriptions managed by this subscriber.
func (s *Subscriber) UnsubscribeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subscriptions {
		_ = s.bus.Unsubscribe(sub)
	}
	s.subscriptions = s.subscriptions[:0]
}

// Close cancels all subscriptions and prevents new ones.
// This should be called when the owning component is being shut down.
func (s *Subscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Cancel all tracked subscriptions
	for _, sub := range s.subscriptions {
		_ = s.bus.Unsubscribe(sub)
	}
	s.subscriptions = nil

	return nil
}

// Count returns the number of active subscriptions.
func (s *Subscriber) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subscriptions)
}

// IsClosed returns true if the subscriber has been closed.
func (s *Subscriber) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Bus returns the underlying bus.
func (s *Subscriber) Bus() Bus {
	return s.bus
}

// SubscriptionGroup manages a group of related subscriptions.
// Useful for components that need to subscribe to multiple topics.
type SubscriptionGroup struct {
	subscriber *Subscriber
	subs       []Subscription
	mu         sync.Mutex
}

// NewSubscriptionGroup creates a new subscription group.
func NewSubscriptionGroup(subscriber *Subscriber) *SubscriptionGroup {
	return &SubscriptionGroup{
		subscriber: subscriber,
		subs:       make([]Subscription, 0),
	}
}

// Add creates a subscription and adds it to the group.
func (g *SubscriptionGroup) Add(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	sub, err := g.subscriber.Subscribe(topicPattern, handler, opts...)
	if err != nil {
		return err
	}

	g.subs = append(g.subs, sub)
	return nil
}

// AddFunc creates a subscription with a function handler and adds it to the group.
func (g *SubscriptionGroup) AddFunc(topicPattern topic.Topic, fn HandlerFunc, opts ...SubscriptionOption) error {
	return g.Add(topicPattern, fn, opts...)
}

// PauseAll pauses all subscriptions in the group.
func (g *SubscriptionGroup) PauseAll() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, sub := range g.subs {
		sub.Pause()
	}
}

// ResumeAll resumes all subscriptions in the group.
func (g *SubscriptionGroup) ResumeAll() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, sub := range g.subs {
		sub.Resume()
	}
}

// CancelAll cancels all subscriptions in the group.
func (g *SubscriptionGroup) CancelAll() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, sub := range g.subs {
		_ = g.subscriber.Unsubscribe(sub)
	}
	g.subs = g.subs[:0]
}

// Count returns the number of subscriptions in the group.
func (g *SubscriptionGroup) Count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.subs)
}
