package event

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewPublisher(t *testing.T) {
	bus := NewBus()
	pub := NewPublisher(bus, "test-source")

	if pub == nil {
		t.Fatal("NewPublisher returned nil")
	}
	if pub.Source() != "test-source" {
		t.Errorf("Source = %q, want %q", pub.Source(), "test-source")
	}
	if pub.Bus() != bus {
		t.Error("Bus() returned different bus")
	}
}

func TestPublisher_Publish(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	pub := NewPublisher(bus, "test")

	var received bool
	var mu sync.Mutex

	// Publish() uses async delivery by default, so subscribe with DeliveryAsync
	_, err := bus.SubscribeFunc("test.event", func(ctx context.Context, event any) error {
		mu.Lock()
		received = true
		mu.Unlock()
		return nil
	}, WithDeliveryMode(DeliveryAsync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	env := Envelope{Topic: "test.event", Payload: "hello"}
	if err := pub.Publish(context.Background(), env); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Give async delivery time to complete
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if !received {
		t.Error("Event was not received")
	}
	mu.Unlock()
}

func TestPublisher_PublishSync(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	pub := NewPublisher(bus, "test")

	var received bool
	_, err := bus.SubscribeFunc("test.sync", func(ctx context.Context, event any) error {
		received = true
		return nil
	}, WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	env := Envelope{Topic: "test.sync", Payload: "hello"}
	if err := pub.PublishSync(context.Background(), env); err != nil {
		t.Errorf("PublishSync failed: %v", err)
	}

	if !received {
		t.Error("Event was not received synchronously")
	}
}

func TestPublisher_PublishTyped(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	pub := NewPublisher(bus, "test-source")

	var receivedEnv Envelope
	var mu sync.Mutex

	_, err := bus.SubscribeFunc("test.typed", func(ctx context.Context, event any) error {
		mu.Lock()
		if env, ok := event.(Envelope); ok {
			receivedEnv = env
		}
		mu.Unlock()
		return nil
	}, WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	payload := map[string]string{"key": "value"}
	if err := pub.PublishTypedSync(context.Background(), "test.typed", payload); err != nil {
		t.Errorf("PublishTypedSync failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedEnv.Topic != "test.typed" {
		t.Errorf("Topic = %q, want %q", receivedEnv.Topic, "test.typed")
	}
	if receivedEnv.Metadata.Source != "test-source" {
		t.Errorf("Source = %q, want %q", receivedEnv.Metadata.Source, "test-source")
	}
	if receivedEnv.Metadata.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestPublishEvent(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	pub := NewPublisher(bus, "test")

	type TestPayload struct {
		Message string
	}

	var receivedPayload TestPayload
	var mu sync.Mutex

	_, err := bus.SubscribeFunc("test.generic", func(ctx context.Context, event any) error {
		mu.Lock()
		if e, ok := event.(Event[TestPayload]); ok {
			receivedPayload = e.Payload
		}
		mu.Unlock()
		return nil
	}, WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := PublishEventSync(context.Background(), pub, "test.generic", TestPayload{Message: "hello"}); err != nil {
		t.Errorf("PublishEventSync failed: %v", err)
	}

	mu.Lock()
	if receivedPayload.Message != "hello" {
		t.Errorf("Message = %q, want %q", receivedPayload.Message, "hello")
	}
	mu.Unlock()
}

func TestPublisher_PublishWithCorrelation(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	pub := NewPublisher(bus, "test")

	var receivedCorrelation string
	var mu sync.Mutex

	// PublishWithCorrelation uses async delivery, so subscribe with DeliveryAsync
	_, err := bus.SubscribeFunc("test.corr", func(ctx context.Context, event any) error {
		mu.Lock()
		if env, ok := event.(Envelope); ok {
			receivedCorrelation = env.Metadata.CorrelationID
		}
		mu.Unlock()
		return nil
	}, WithDeliveryMode(DeliveryAsync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := pub.PublishWithCorrelation(context.Background(), "test.corr", "data", "corr-123"); err != nil {
		t.Errorf("PublishWithCorrelation failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if receivedCorrelation != "corr-123" {
		t.Errorf("CorrelationID = %q, want %q", receivedCorrelation, "corr-123")
	}
	mu.Unlock()
}

func TestPublisher_PublishWithCausation(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	pub := NewPublisher(bus, "test")

	var receivedCausation string
	var mu sync.Mutex

	// PublishWithCausation uses async delivery, so subscribe with DeliveryAsync
	_, err := bus.SubscribeFunc("test.cause", func(ctx context.Context, event any) error {
		mu.Lock()
		if env, ok := event.(Envelope); ok {
			receivedCausation = env.Metadata.CausationID
		}
		mu.Unlock()
		return nil
	}, WithDeliveryMode(DeliveryAsync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := pub.PublishWithCausation(context.Background(), "test.cause", "data", "cause-456"); err != nil {
		t.Errorf("PublishWithCausation failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if receivedCausation != "cause-456" {
		t.Errorf("CausationID = %q, want %q", receivedCausation, "cause-456")
	}
	mu.Unlock()
}
