package event

import (
	"context"
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// timeNow is a variable to allow testing with fixed timestamps.
var timeNow = time.Now

// Publisher provides a simplified API for publishing events.
// It wraps a Bus and provides convenience methods for common publishing patterns.
type Publisher struct {
	bus    Bus
	source string
}

// NewPublisher creates a new Publisher wrapping the given bus.
// The source parameter identifies where events originate (e.g., "buffer", "lsp").
func NewPublisher(bus Bus, source string) *Publisher {
	return &Publisher{
		bus:    bus,
		source: source,
	}
}

// Publish sends an event asynchronously using the default delivery mode.
// The event must implement TopicProvider or be an Envelope.
func (p *Publisher) Publish(ctx context.Context, event any) error {
	return p.bus.Publish(ctx, event)
}

// PublishSync sends an event synchronously.
// The call blocks until all sync handlers complete.
func (p *Publisher) PublishSync(ctx context.Context, event any) error {
	return p.bus.PublishSync(ctx, event)
}

// PublishAsync sends an event asynchronously.
// The event is queued for delivery by worker goroutines.
func (p *Publisher) PublishAsync(ctx context.Context, event any) error {
	return p.bus.PublishAsync(ctx, event)
}

// PublishTyped creates and publishes a typed event.
// This is a convenience method that creates an Event[T] with the publisher's source.
func (p *Publisher) PublishTyped(ctx context.Context, eventType topic.Topic, payload any) error {
	env := Envelope{
		Topic:   eventType,
		Payload: payload,
		Metadata: Metadata{
			ID:        generateID(),
			Source:    p.source,
			Timestamp: timeNow(),
			Version:   1,
		},
	}
	return p.bus.Publish(ctx, env)
}

// PublishTypedSync creates and publishes a typed event synchronously.
func (p *Publisher) PublishTypedSync(ctx context.Context, eventType topic.Topic, payload any) error {
	env := Envelope{
		Topic:   eventType,
		Payload: payload,
		Metadata: Metadata{
			ID:        generateID(),
			Source:    p.source,
			Timestamp: timeNow(),
			Version:   1,
		},
	}
	return p.bus.PublishSync(ctx, env)
}

// PublishTypedAsync creates and publishes a typed event asynchronously.
func (p *Publisher) PublishTypedAsync(ctx context.Context, eventType topic.Topic, payload any) error {
	env := Envelope{
		Topic:   eventType,
		Payload: payload,
		Metadata: Metadata{
			ID:        generateID(),
			Source:    p.source,
			Timestamp: timeNow(),
			Version:   1,
		},
	}
	return p.bus.PublishAsync(ctx, env)
}

// PublishEvent creates and publishes a typed Event[T].
// This provides full type safety for the payload.
func PublishEvent[T any](ctx context.Context, p *Publisher, eventType topic.Topic, payload T) error {
	event := NewEvent(eventType, payload, p.source)
	return p.bus.Publish(ctx, event)
}

// PublishEventSync creates and publishes a typed Event[T] synchronously.
func PublishEventSync[T any](ctx context.Context, p *Publisher, eventType topic.Topic, payload T) error {
	event := NewEvent(eventType, payload, p.source)
	return p.bus.PublishSync(ctx, event)
}

// PublishEventAsync creates and publishes a typed Event[T] asynchronously.
func PublishEventAsync[T any](ctx context.Context, p *Publisher, eventType topic.Topic, payload T) error {
	event := NewEvent(eventType, payload, p.source)
	return p.bus.PublishAsync(ctx, event)
}

// PublishWithCorrelation publishes an event with a correlation ID.
// Useful for tracking related events across operations.
func (p *Publisher) PublishWithCorrelation(ctx context.Context, eventType topic.Topic, payload any, correlationID string) error {
	env := Envelope{
		Topic:   eventType,
		Payload: payload,
		Metadata: Metadata{
			ID:            generateID(),
			Source:        p.source,
			Timestamp:     timeNow(),
			Version:       1,
			CorrelationID: correlationID,
		},
	}
	return p.bus.Publish(ctx, env)
}

// PublishWithCausation publishes an event with a causation ID.
// Useful for tracking event chains where one event causes another.
func (p *Publisher) PublishWithCausation(ctx context.Context, eventType topic.Topic, payload any, causationID string) error {
	env := Envelope{
		Topic:   eventType,
		Payload: payload,
		Metadata: Metadata{
			ID:          generateID(),
			Source:      p.source,
			Timestamp:   timeNow(),
			Version:     1,
			CausationID: causationID,
		},
	}
	return p.bus.Publish(ctx, env)
}

// Source returns the publisher's source identifier.
func (p *Publisher) Source() string {
	return p.source
}

// Bus returns the underlying bus.
func (p *Publisher) Bus() Bus {
	return p.bus
}
