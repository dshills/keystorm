package event

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

func TestNewBusAdapter(t *testing.T) {
	bus := NewBus()
	adapter := NewBusAdapter(bus, "test-source")

	if adapter == nil {
		t.Fatal("NewBusAdapter returned nil")
	}
	if adapter.Bus() != bus {
		t.Error("Bus() returned different bus")
	}
}

func TestBusAdapter_Publish(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	adapter := NewBusAdapter(bus, "adapter-source")

	var received atomic.Bool
	var receivedData map[string]any
	var mu sync.Mutex

	_, err := bus.SubscribeFunc("test.legacy", func(ctx context.Context, event any) error {
		received.Store(true)
		mu.Lock()
		if env, ok := event.(Envelope); ok {
			if data, ok := env.Payload.(map[string]any); ok {
				receivedData = data
			}
		}
		mu.Unlock()
		return nil
	}, WithDeliveryMode(DeliveryAsync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish using legacy interface
	adapter.Publish("test.legacy", map[string]any{"key": "value"})

	// Wait for async delivery
	time.Sleep(100 * time.Millisecond)

	if !received.Load() {
		t.Error("Event was not received")
	}

	mu.Lock()
	if receivedData == nil {
		t.Error("receivedData is nil")
	} else if receivedData["key"] != "value" {
		t.Errorf("receivedData[key] = %v, want %q", receivedData["key"], "value")
	}
	mu.Unlock()
}

func TestBusAdapter_PublishSync(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	adapter := NewBusAdapter(bus, "adapter-source")

	var received bool
	_, err := bus.SubscribeFunc("test.sync", func(ctx context.Context, event any) error {
		received = true
		return nil
	}, WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	err = adapter.PublishSync("test.sync", map[string]any{"key": "value"})
	if err != nil {
		t.Errorf("PublishSync failed: %v", err)
	}

	if !received {
		t.Error("Event was not received synchronously")
	}
}

func TestBusAdapter_Close(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	adapter := NewBusAdapter(bus, "test")

	if err := adapter.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// PublishSync should fail after close
	err := adapter.PublishSync("test.event", map[string]any{})
	if err != ErrAdapterClosed {
		t.Errorf("PublishSync after close: err = %v, want ErrAdapterClosed", err)
	}
}

func TestNewIntegrationSubscriber(t *testing.T) {
	bus := NewBus()
	sub := NewIntegrationSubscriber(bus)

	if sub == nil {
		t.Fatal("NewIntegrationSubscriber returned nil")
	}
	if sub.SubscriptionCount() != 0 {
		t.Errorf("SubscriptionCount = %d, want 0", sub.SubscriptionCount())
	}
}

func TestIntegrationSubscriber_Subscribe(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewIntegrationSubscriber(bus)

	var received atomic.Bool
	var receivedData map[string]any
	var mu sync.Mutex

	id := sub.Subscribe("test.legacy", func(data map[string]any) {
		received.Store(true)
		mu.Lock()
		receivedData = data
		mu.Unlock()
	})

	if id == "" {
		t.Fatal("Subscribe returned empty ID")
	}
	if sub.SubscriptionCount() != 1 {
		t.Errorf("SubscriptionCount = %d, want 1", sub.SubscriptionCount())
	}

	// Publish using typed bus with map payload
	env := Envelope{
		Topic:   "test.legacy",
		Payload: map[string]any{"message": "hello"},
	}
	if err := bus.PublishAsync(context.Background(), env); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Wait for async delivery
	time.Sleep(100 * time.Millisecond)

	if !received.Load() {
		t.Error("Event was not received")
	}

	mu.Lock()
	if receivedData == nil {
		t.Error("receivedData is nil")
	} else if receivedData["message"] != "hello" {
		t.Errorf("receivedData[message] = %v, want %q", receivedData["message"], "hello")
	}
	mu.Unlock()
}

func TestIntegrationSubscriber_Unsubscribe(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewIntegrationSubscriber(bus)

	var count atomic.Int32
	id := sub.Subscribe("test.unsub", func(data map[string]any) {
		count.Add(1)
	})

	// First event
	env := Envelope{Topic: "test.unsub", Payload: map[string]any{}}
	_ = bus.PublishAsync(context.Background(), env)
	time.Sleep(50 * time.Millisecond)

	if count.Load() != 1 {
		t.Errorf("count = %d, want 1", count.Load())
	}

	// Unsubscribe
	if !sub.Unsubscribe(id) {
		t.Error("Unsubscribe returned false")
	}
	if sub.SubscriptionCount() != 0 {
		t.Errorf("SubscriptionCount = %d, want 0", sub.SubscriptionCount())
	}

	// Second event should not be received
	_ = bus.PublishAsync(context.Background(), env)
	time.Sleep(50 * time.Millisecond)

	if count.Load() != 1 {
		t.Errorf("count = %d, want 1 after unsubscribe", count.Load())
	}
}

func TestIntegrationSubscriber_Close(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewIntegrationSubscriber(bus)
	sub.Subscribe("test.close", func(data map[string]any) {})

	if err := sub.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// New subscriptions should fail
	id := sub.Subscribe("test.after", func(data map[string]any) {})
	if id != "" {
		t.Error("Subscribe after close should return empty ID")
	}
}

func TestExtractLegacyData(t *testing.T) {
	tests := []struct {
		name     string
		event    any
		wantNil  bool
		wantKeys []string
	}{
		{
			name: "envelope with map payload",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"key": "value"},
			},
			wantKeys: []string{"key"},
		},
		{
			name: "envelope with struct payload",
			event: Envelope{
				Topic:   "test.topic",
				Payload: struct{ Field string }{Field: "value"},
				Metadata: Metadata{
					ID:     "123",
					Source: "test",
				},
			},
			wantKeys: []string{"payload", "topic", "id", "source"},
		},
		{
			name:     "direct map",
			event:    map[string]any{"direct": "map"},
			wantKeys: []string{"direct"},
		},
		{
			name:     "topic provider",
			event:    NewEvent("test.topic", struct{ Data string }{Data: "value"}, "source"),
			wantKeys: []string{"topic", "payload"},
		},
		{
			name:     "unknown type",
			event:    "just a string",
			wantKeys: []string{"payload"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := extractLegacyData(tt.event)

			if tt.wantNil {
				if data != nil {
					t.Errorf("extractLegacyData() = %v, want nil", data)
				}
				return
			}

			if data == nil {
				t.Fatal("extractLegacyData() returned nil")
			}

			for _, key := range tt.wantKeys {
				if _, ok := data[key]; !ok {
					t.Errorf("extractLegacyData() missing key %q", key)
				}
			}
		})
	}
}

func TestBridge(t *testing.T) {
	typedBus := NewBus()
	if err := typedBus.Start(); err != nil {
		t.Fatalf("typedBus.Start failed: %v", err)
	}
	defer typedBus.Stop(context.Background())

	// Create a mock legacy publisher
	var legacyReceived []map[string]any
	var mu sync.Mutex
	mockLegacy := &mockEventPublisher{
		publishFunc: func(eventType string, data map[string]any) {
			mu.Lock()
			legacyReceived = append(legacyReceived, data)
			mu.Unlock()
		},
	}

	bridge := NewBridge(typedBus, mockLegacy, BridgeConfig{
		ForwardTopics: []topic.Topic{"test.*"},
	})

	if err := bridge.Start(); err != nil {
		t.Fatalf("bridge.Start failed: %v", err)
	}
	defer bridge.Close()

	// Publish to typed bus
	env := Envelope{
		Topic:   "test.forward",
		Payload: map[string]any{"forwarded": true},
	}
	if err := typedBus.PublishAsync(context.Background(), env); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Wait for async delivery and forwarding
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(legacyReceived) != 1 {
		t.Errorf("legacyReceived has %d events, want 1", len(legacyReceived))
	} else if legacyReceived[0]["forwarded"] != true {
		t.Errorf("forwarded event missing expected data")
	}
	mu.Unlock()
}

// mockEventPublisher implements EventPublisher for testing.
type mockEventPublisher struct {
	publishFunc func(eventType string, data map[string]any)
}

func (m *mockEventPublisher) Publish(eventType string, data map[string]any) {
	if m.publishFunc != nil {
		m.publishFunc(eventType, data)
	}
}
