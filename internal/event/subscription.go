package event

import (
	"sync/atomic"

	"github.com/dshills/keystorm/internal/event/topic"
)

// SubscriptionState represents the state of a subscription.
type SubscriptionState int32

const (
	// SubscriptionStateActive means the subscription is receiving events.
	SubscriptionStateActive SubscriptionState = iota

	// SubscriptionStatePaused means the subscription is temporarily not receiving events.
	SubscriptionStatePaused

	// SubscriptionStateCancelled means the subscription has been permanently cancelled.
	SubscriptionStateCancelled
)

// String returns a human-readable state name.
func (s SubscriptionState) String() string {
	switch s {
	case SubscriptionStateActive:
		return "active"
	case SubscriptionStatePaused:
		return "paused"
	case SubscriptionStateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Subscription represents an active event subscription.
// It provides methods to control the subscription lifecycle.
type Subscription interface {
	// ID returns the unique subscription identifier.
	ID() string

	// Topic returns the subscribed topic pattern.
	Topic() topic.Topic

	// State returns the current subscription state.
	State() SubscriptionState

	// IsActive returns true if the subscription can receive events.
	IsActive() bool

	// IsPaused returns true if the subscription is paused.
	IsPaused() bool

	// Pause temporarily stops event delivery to this subscription.
	Pause()

	// Resume restarts event delivery after a pause.
	Resume()

	// Cancel permanently cancels the subscription.
	// After cancellation, the subscription cannot be resumed.
	Cancel()
}

// SubscriptionConfig contains configuration for a subscription.
type SubscriptionConfig struct {
	// Priority determines execution order (lower values execute first).
	Priority Priority

	// DeliveryMode specifies sync or async delivery.
	DeliveryMode DeliveryMode

	// Filter is an optional predicate to filter events.
	// If set, events are only delivered if Filter returns true.
	Filter FilterFunc

	// Once indicates the subscription should auto-cancel after the first event.
	Once bool
}

// DefaultSubscriptionConfig returns a default subscription configuration.
func DefaultSubscriptionConfig() SubscriptionConfig {
	return SubscriptionConfig{
		Priority:     PriorityNormal,
		DeliveryMode: DeliverySync,
		Filter:       nil,
		Once:         false,
	}
}

// SubscriptionOption is a function that configures a subscription.
type SubscriptionOption func(*SubscriptionConfig)

// WithPriority sets the subscription priority.
func WithPriority(p Priority) SubscriptionOption {
	return func(c *SubscriptionConfig) {
		c.Priority = p
	}
}

// WithDeliveryMode sets the delivery mode.
func WithDeliveryMode(m DeliveryMode) SubscriptionOption {
	return func(c *SubscriptionConfig) {
		c.DeliveryMode = m
	}
}

// WithFilter sets a filter predicate.
func WithFilter(f FilterFunc) SubscriptionOption {
	return func(c *SubscriptionConfig) {
		c.Filter = f
	}
}

// WithOnce sets the subscription to auto-cancel after the first event.
func WithOnce() SubscriptionOption {
	return func(c *SubscriptionConfig) {
		c.Once = true
	}
}

// subscription is the internal implementation of Subscription.
type subscription struct {
	id      string
	topic   topic.Topic
	handler Handler
	config  SubscriptionConfig
	state   atomic.Int32
}

// newSubscription creates a new subscription.
func newSubscription(id string, t topic.Topic, h Handler, opts ...SubscriptionOption) *subscription {
	config := DefaultSubscriptionConfig()
	for _, opt := range opts {
		opt(&config)
	}

	s := &subscription{
		id:      id,
		topic:   t,
		handler: h,
		config:  config,
	}
	s.state.Store(int32(SubscriptionStateActive))
	return s
}

// ID returns the subscription ID.
func (s *subscription) ID() string {
	return s.id
}

// Topic returns the subscribed topic pattern.
func (s *subscription) Topic() topic.Topic {
	return s.topic
}

// Handler returns the subscription's handler.
func (s *subscription) Handler() Handler {
	return s.handler
}

// Config returns the subscription configuration.
func (s *subscription) Config() SubscriptionConfig {
	return s.config
}

// State returns the current subscription state.
func (s *subscription) State() SubscriptionState {
	return SubscriptionState(s.state.Load())
}

// IsActive returns true if the subscription is active.
func (s *subscription) IsActive() bool {
	return s.State() == SubscriptionStateActive
}

// IsPaused returns true if the subscription is paused.
func (s *subscription) IsPaused() bool {
	return s.State() == SubscriptionStatePaused
}

// IsCancelled returns true if the subscription is cancelled.
func (s *subscription) IsCancelled() bool {
	return s.State() == SubscriptionStateCancelled
}

// Pause temporarily stops event delivery.
func (s *subscription) Pause() {
	// Only pause if currently active
	s.state.CompareAndSwap(int32(SubscriptionStateActive), int32(SubscriptionStatePaused))
}

// Resume restarts event delivery.
func (s *subscription) Resume() {
	// Only resume if currently paused
	s.state.CompareAndSwap(int32(SubscriptionStatePaused), int32(SubscriptionStateActive))
}

// Cancel permanently cancels the subscription.
func (s *subscription) Cancel() {
	s.state.Store(int32(SubscriptionStateCancelled))
}

// ShouldDeliver returns true if the event should be delivered to this subscription.
func (s *subscription) ShouldDeliver(event any) bool {
	// Check state
	if !s.IsActive() {
		return false
	}

	// Check filter
	if s.config.Filter != nil && !s.config.Filter(event) {
		return false
	}

	return true
}
