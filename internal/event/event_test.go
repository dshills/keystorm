package event

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// TestPayload is a simple test payload type.
type TestPayload struct {
	BufferID string
	Text     string
	Position int
}

func TestNewEvent(t *testing.T) {
	eventTopic := topic.Topic("buffer.content.inserted")
	payload := TestPayload{
		BufferID: "test-buffer",
		Text:     "hello",
		Position: 42,
	}
	source := "engine"

	evt := NewEvent(eventTopic, payload, source)

	if evt.Type != eventTopic {
		t.Errorf("expected topic %v, got %v", eventTopic, evt.Type)
	}
	if evt.Payload.BufferID != payload.BufferID {
		t.Errorf("expected BufferID %v, got %v", payload.BufferID, evt.Payload.BufferID)
	}
	if evt.Payload.Text != payload.Text {
		t.Errorf("expected Text %v, got %v", payload.Text, evt.Payload.Text)
	}
	if evt.Payload.Position != payload.Position {
		t.Errorf("expected Position %v, got %v", payload.Position, evt.Payload.Position)
	}
	if evt.Metadata.Source != source {
		t.Errorf("expected source %v, got %v", source, evt.Metadata.Source)
	}
	if evt.Metadata.ID == "" {
		t.Error("expected non-empty ID")
	}
	if evt.Metadata.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if evt.Metadata.Version != 1 {
		t.Errorf("expected version 1, got %v", evt.Metadata.Version)
	}
}

func TestNewEventWithMetadata(t *testing.T) {
	eventTopic := topic.Topic("config.changed")
	payload := "test"
	meta := Metadata{
		ID:            "custom-id",
		Timestamp:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Source:        "config",
		CorrelationID: "corr-123",
		CausationID:   "cause-456",
		Version:       2,
	}

	evt := NewEventWithMetadata(eventTopic, payload, meta)

	if evt.Metadata.ID != "custom-id" {
		t.Errorf("expected custom ID, got %v", evt.Metadata.ID)
	}
	if evt.Metadata.CorrelationID != "corr-123" {
		t.Errorf("expected correlation ID, got %v", evt.Metadata.CorrelationID)
	}
	if evt.Metadata.CausationID != "cause-456" {
		t.Errorf("expected causation ID, got %v", evt.Metadata.CausationID)
	}
	if evt.Metadata.Version != 2 {
		t.Errorf("expected version 2, got %v", evt.Metadata.Version)
	}
}

func TestNewEventWithMetadata_Defaults(t *testing.T) {
	eventTopic := topic.Topic("config.changed")
	payload := "test"
	meta := Metadata{
		Source: "test",
		// ID, Timestamp, Version are zero values
	}

	evt := NewEventWithMetadata(eventTopic, payload, meta)

	if evt.Metadata.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if evt.Metadata.Timestamp.IsZero() {
		t.Error("expected auto-set timestamp")
	}
	if evt.Metadata.Version != 1 {
		t.Errorf("expected default version 1, got %v", evt.Metadata.Version)
	}
}

func TestEvent_EventTopic(t *testing.T) {
	eventTopic := topic.Topic("buffer.content.inserted")
	evt := NewEvent(eventTopic, "payload", "source")

	if evt.EventTopic() != eventTopic {
		t.Errorf("expected topic %v, got %v", eventTopic, evt.EventTopic())
	}
}

func TestEvent_EventMetadata(t *testing.T) {
	evt := NewEvent(topic.Topic("test"), "payload", "source")

	meta := evt.EventMetadata()

	if meta.Source != "source" {
		t.Errorf("expected source 'source', got %v", meta.Source)
	}
	if meta.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestEvent_WithCorrelation(t *testing.T) {
	evt := NewEvent(topic.Topic("test"), "payload", "source")

	evt2 := evt.WithCorrelation("corr-123")

	if evt2.Metadata.CorrelationID != "corr-123" {
		t.Errorf("expected correlation ID 'corr-123', got %v", evt2.Metadata.CorrelationID)
	}
	// Original should be unchanged (immutability through copy)
	if evt.Metadata.CorrelationID != "" {
		t.Error("original event should not be modified")
	}
}

func TestEvent_WithCausation(t *testing.T) {
	evt := NewEvent(topic.Topic("test"), "payload", "source")

	evt2 := evt.WithCausation("cause-456")

	if evt2.Metadata.CausationID != "cause-456" {
		t.Errorf("expected causation ID 'cause-456', got %v", evt2.Metadata.CausationID)
	}
}

func TestEvent_WithSource(t *testing.T) {
	evt := NewEvent(topic.Topic("test"), "payload", "original")

	evt2 := evt.WithSource("new-source")

	if evt2.Metadata.Source != "new-source" {
		t.Errorf("expected source 'new-source', got %v", evt2.Metadata.Source)
	}
	if evt.Metadata.Source != "original" {
		t.Error("original event should not be modified")
	}
}

func TestNewEnvelope(t *testing.T) {
	eventTopic := topic.Topic("buffer.content.inserted")
	payload := TestPayload{BufferID: "test", Text: "hello", Position: 10}
	evt := NewEvent(eventTopic, payload, "engine")

	env := NewEnvelope(evt)

	if env.Topic != eventTopic {
		t.Errorf("expected topic %v, got %v", eventTopic, env.Topic)
	}
	if env.Metadata.Source != "engine" {
		t.Errorf("expected source 'engine', got %v", env.Metadata.Source)
	}

	// Payload should be the original payload
	p, ok := env.Payload.(TestPayload)
	if !ok {
		t.Error("expected payload to be TestPayload")
	}
	if p.BufferID != "test" {
		t.Errorf("expected BufferID 'test', got %v", p.BufferID)
	}
}

func TestToEnvelope(t *testing.T) {
	eventTopic := topic.Topic("config.changed")
	evt := NewEvent(eventTopic, "payload", "config")

	env := ToEnvelope(evt)

	if env.Topic != eventTopic {
		t.Errorf("expected topic %v, got %v", eventTopic, env.Topic)
	}
	if env.Metadata.Source != "config" {
		t.Errorf("expected source 'config', got %v", env.Metadata.Source)
	}
}

func TestToEnvelope_NonEvent(t *testing.T) {
	// A type that doesn't implement TopicProvider
	env := ToEnvelope("not an event")

	if env.Topic != "" {
		t.Errorf("expected empty topic for non-event, got %v", env.Topic)
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	count := 10000

	for i := 0; i < count; i++ {
		id := generateID()
		if ids[id] {
			t.Errorf("duplicate ID generated: %v", id)
		}
		ids[id] = true
	}
}

func TestGenerateID_Length(t *testing.T) {
	id := generateID()

	// 16 bytes = 32 hex characters
	if len(id) != 32 {
		t.Errorf("expected ID length 32, got %d", len(id))
	}
}

func BenchmarkNewEvent(b *testing.B) {
	eventTopic := topic.Topic("buffer.content.inserted")
	payload := TestPayload{BufferID: "test", Text: "hello", Position: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewEvent(eventTopic, payload, "engine")
	}
}

func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = generateID()
	}
}

func BenchmarkNewEnvelope(b *testing.B) {
	evt := NewEvent(topic.Topic("test"), "payload", "source")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewEnvelope(evt)
	}
}
