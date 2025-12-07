package event

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Event represents an event in the system.
// Events are immutable once created.
type Event[T any] struct {
	// Type is the hierarchical event type (e.g., "buffer.content.inserted").
	Type topic.Topic

	// Payload contains the event-specific data.
	Payload T

	// Metadata contains standard event information.
	Metadata Metadata
}

// Metadata contains standard information attached to every event.
type Metadata struct {
	// ID is a unique identifier for this event instance.
	ID string

	// Timestamp is when the event was created.
	Timestamp time.Time

	// Source identifies the module that published the event.
	Source string

	// CorrelationID links related events (e.g., request/response).
	CorrelationID string

	// CausationID links to the event that caused this one.
	CausationID string

	// Version is the schema version of the payload.
	Version int
}

// NewEvent creates a new event with the given type and payload.
func NewEvent[T any](eventType topic.Topic, payload T, source string) Event[T] {
	return Event[T]{
		Type:    eventType,
		Payload: payload,
		Metadata: Metadata{
			ID:        generateID(),
			Timestamp: time.Now(),
			Source:    source,
			Version:   1,
		},
	}
}

// NewEventWithMetadata creates a new event with custom metadata.
func NewEventWithMetadata[T any](eventType topic.Topic, payload T, meta Metadata) Event[T] {
	if meta.ID == "" {
		meta.ID = generateID()
	}
	if meta.Timestamp.IsZero() {
		meta.Timestamp = time.Now()
	}
	if meta.Version == 0 {
		meta.Version = 1
	}
	return Event[T]{
		Type:     eventType,
		Payload:  payload,
		Metadata: meta,
	}
}

// EventTopic returns the event's topic for type-erased handling.
func (e Event[T]) EventTopic() topic.Topic {
	return e.Type
}

// EventMetadata returns the event's metadata for type-erased handling.
func (e Event[T]) EventMetadata() Metadata {
	return e.Metadata
}

// WithCorrelation returns a copy of the event with a correlation ID set.
func (e Event[T]) WithCorrelation(correlationID string) Event[T] {
	e.Metadata.CorrelationID = correlationID
	return e
}

// WithCausation returns a copy of the event with a causation ID set.
func (e Event[T]) WithCausation(causationID string) Event[T] {
	e.Metadata.CausationID = causationID
	return e
}

// WithSource returns a copy of the event with a different source.
func (e Event[T]) WithSource(source string) Event[T] {
	e.Metadata.Source = source
	return e
}

// TopicProvider is implemented by types that can provide their topic.
type TopicProvider interface {
	EventTopic() topic.Topic
}

// MetadataProvider is implemented by types that can provide their metadata.
type MetadataProvider interface {
	EventMetadata() Metadata
}

// generateID generates a unique event ID.
func generateID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// Envelope wraps any event for type-erased handling.
// This is useful when the bus needs to handle events of unknown types.
type Envelope struct {
	// Topic is the event topic.
	Topic topic.Topic

	// Payload is the type-erased event payload.
	Payload any

	// Metadata is the event metadata.
	Metadata Metadata
}

// NewEnvelope creates a new envelope from a typed event.
func NewEnvelope[T any](e Event[T]) Envelope {
	return Envelope{
		Topic:    e.Type,
		Payload:  e.Payload,
		Metadata: e.Metadata,
	}
}

// ToEnvelope converts a TopicProvider to an Envelope.
// Returns an empty Envelope if the event doesn't implement the required interfaces.
func ToEnvelope(event any) Envelope {
	tp, ok := event.(TopicProvider)
	if !ok {
		return Envelope{}
	}

	env := Envelope{
		Topic:   tp.EventTopic(),
		Payload: event,
	}

	if mp, ok := event.(MetadataProvider); ok {
		env.Metadata = mp.EventMetadata()
	}

	return env
}
