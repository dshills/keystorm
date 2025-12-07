package event

import (
	"context"
	"errors"
	"testing"

	"github.com/dshills/keystorm/internal/event/topic"
)

func TestPriority_String(t *testing.T) {
	tests := []struct {
		priority Priority
		expected string
	}{
		{PriorityCritical, "critical"}, // 0
		{PriorityHigh, "high"},         // 100
		{PriorityNormal, "normal"},     // 200
		{PriorityLow, "low"},           // 300
		{Priority(-10), "critical"},    // -10 <= 0 -> critical
		{Priority(50), "high"},         // 0 < 50 <= 100 -> high
		{Priority(150), "normal"},      // 100 < 150 <= 200 -> normal
		{Priority(250), "low"},         // 200 < 250 -> low
		{Priority(400), "low"},         // 300 < 400 -> low
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.priority.String(); got != tt.expected {
				t.Errorf("Priority.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDeliveryMode_String(t *testing.T) {
	tests := []struct {
		mode     DeliveryMode
		expected string
	}{
		{DeliverySync, "sync"},
		{DeliveryAsync, "async"},
		{DeliveryMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("DeliveryMode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHandlerFunc(t *testing.T) {
	called := false
	var receivedEvent any

	handler := HandlerFunc(func(ctx context.Context, event any) error {
		called = true
		receivedEvent = event
		return nil
	})

	err := handler.Handle(context.Background(), "test-event")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if receivedEvent != "test-event" {
		t.Errorf("expected event 'test-event', got %v", receivedEvent)
	}
}

func TestHandlerFunc_Error(t *testing.T) {
	expectedErr := errors.New("test error")

	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return expectedErr
	})

	err := handler.Handle(context.Background(), "test-event")

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestTypedHandlerFunc(t *testing.T) {
	type TestPayload struct {
		Value string
	}

	called := false
	var receivedPayload TestPayload

	handler := TypedHandlerFunc[TestPayload](func(ctx context.Context, event Event[TestPayload]) error {
		called = true
		receivedPayload = event.Payload
		return nil
	})

	evt := NewEvent(topic.Topic("test"), TestPayload{Value: "hello"}, "source")
	err := handler.Handle(context.Background(), evt)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if receivedPayload.Value != "hello" {
		t.Errorf("expected payload value 'hello', got %v", receivedPayload.Value)
	}
}

func TestAsHandler(t *testing.T) {
	type TestPayload struct {
		Value int
	}

	called := false
	var receivedValue int

	typedHandler := TypedHandlerFunc[TestPayload](func(ctx context.Context, event Event[TestPayload]) error {
		called = true
		receivedValue = event.Payload.Value
		return nil
	})

	handler := AsHandler(typedHandler)

	// Test with matching type
	evt := NewEvent(topic.Topic("test"), TestPayload{Value: 42}, "source")
	err := handler.Handle(context.Background(), evt)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if receivedValue != 42 {
		t.Errorf("expected value 42, got %v", receivedValue)
	}
}

func TestAsHandler_TypeMismatch(t *testing.T) {
	type TestPayload struct {
		Value int
	}

	called := false

	typedHandler := TypedHandlerFunc[TestPayload](func(ctx context.Context, event Event[TestPayload]) error {
		called = true
		return nil
	})

	handler := AsHandler(typedHandler)

	// Test with non-matching type (plain string instead of Event[TestPayload])
	err := handler.Handle(context.Background(), "wrong type")

	if err != nil {
		t.Errorf("unexpected error for type mismatch: %v", err)
	}
	if called {
		t.Error("handler should not be called for type mismatch")
	}
}

func TestAsHandlerFunc(t *testing.T) {
	type TestPayload struct {
		Name string
	}

	called := false

	fn := TypedHandlerFunc[TestPayload](func(ctx context.Context, event Event[TestPayload]) error {
		called = true
		return nil
	})

	handler := AsHandlerFunc(fn)

	evt := NewEvent(topic.Topic("test"), TestPayload{Name: "test"}, "source")
	_ = handler.Handle(context.Background(), evt)

	if !called {
		t.Error("handler was not called")
	}
}

func TestFilterFunc(t *testing.T) {
	type TestPayload struct {
		Important bool
	}

	filter := FilterFunc(func(event any) bool {
		if evt, ok := event.(Event[TestPayload]); ok {
			return evt.Payload.Important
		}
		return false
	})

	importantEvt := NewEvent(topic.Topic("test"), TestPayload{Important: true}, "source")
	normalEvt := NewEvent(topic.Topic("test"), TestPayload{Important: false}, "source")

	if !filter(importantEvt) {
		t.Error("filter should return true for important event")
	}
	if filter(normalEvt) {
		t.Error("filter should return false for normal event")
	}
	if filter("not an event") {
		t.Error("filter should return false for non-event")
	}
}

func BenchmarkHandlerFunc(b *testing.B) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, "event")
	}
}

func BenchmarkTypedHandler(b *testing.B) {
	type TestPayload struct {
		Value int
	}

	handler := AsHandler(TypedHandlerFunc[TestPayload](func(ctx context.Context, event Event[TestPayload]) error {
		return nil
	}))

	ctx := context.Background()
	evt := NewEvent(topic.Topic("test"), TestPayload{Value: 42}, "source")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, evt)
	}
}
