package event

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewSubscriber(t *testing.T) {
	bus := NewBus()
	sub := NewSubscriber(bus)

	if sub == nil {
		t.Fatal("NewSubscriber returned nil")
	}
	if sub.Bus() != bus {
		t.Error("Bus() returned different bus")
	}
	if sub.Count() != 0 {
		t.Errorf("Count = %d, want 0", sub.Count())
	}
}

func TestSubscriber_Subscribe(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	var received bool
	subscription, err := sub.SubscribeFunc("test.event", func(ctx context.Context, event any) error {
		received = true
		return nil
	}, WithDeliveryMode(DeliverySync))

	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if subscription == nil {
		t.Fatal("Subscription is nil")
	}
	if sub.Count() != 1 {
		t.Errorf("Count = %d, want 1", sub.Count())
	}

	// Publish an event
	env := Envelope{Topic: "test.event", Payload: "hello"}
	if err := bus.PublishSync(context.Background(), env); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	if !received {
		t.Error("Event was not received")
	}
}

func TestSubscriber_SubscribeTyped(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	type TestPayload struct {
		Message string
	}

	var receivedPayload TestPayload
	_, err := SubscribeTyped(sub, "test.typed", func(ctx context.Context, event Event[TestPayload]) error {
		receivedPayload = event.Payload
		return nil
	}, WithDeliveryMode(DeliverySync))

	if err != nil {
		t.Fatalf("SubscribeTyped failed: %v", err)
	}

	// Publish a typed event
	event := NewEvent("test.typed", TestPayload{Message: "hello"}, "test")
	if err := bus.PublishSync(context.Background(), event); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	if receivedPayload.Message != "hello" {
		t.Errorf("Message = %q, want %q", receivedPayload.Message, "hello")
	}
}

func TestSubscriber_SubscribePayload(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	type TestPayload struct {
		Value int
	}

	var receivedValue int
	_, err := SubscribePayload(sub, "test.payload", func(ctx context.Context, payload TestPayload) error {
		receivedValue = payload.Value
		return nil
	}, WithDeliveryMode(DeliverySync))

	if err != nil {
		t.Fatalf("SubscribePayload failed: %v", err)
	}

	// Publish a typed event
	event := NewEvent("test.payload", TestPayload{Value: 42}, "test")
	if err := bus.PublishSync(context.Background(), event); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	if receivedValue != 42 {
		t.Errorf("Value = %d, want 42", receivedValue)
	}
}

func TestSubscriber_SubscribeOnce(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	var count int
	_, err := sub.SubscribeOnceFunc("test.once", func(ctx context.Context, event any) error {
		count++
		return nil
	}, WithDeliveryMode(DeliverySync))

	if err != nil {
		t.Fatalf("SubscribeOnce failed: %v", err)
	}

	// Publish multiple events
	env := Envelope{Topic: "test.once", Payload: "hello"}
	for i := 0; i < 3; i++ {
		_ = bus.PublishSync(context.Background(), env)
	}

	// Should only receive once
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestSubscriber_SubscribeAsync(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	var received atomic.Bool
	_, err := sub.SubscribeAsyncFunc("test.async", func(ctx context.Context, event any) error {
		received.Store(true)
		return nil
	})

	if err != nil {
		t.Fatalf("SubscribeAsync failed: %v", err)
	}

	env := Envelope{Topic: "test.async", Payload: "hello"}
	if err := bus.PublishAsync(context.Background(), env); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Wait for async delivery
	time.Sleep(100 * time.Millisecond)

	if !received.Load() {
		t.Error("Event was not received asynchronously")
	}
}

func TestSubscriber_SubscribeCritical(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	var order []string
	var mu sync.Mutex

	// Subscribe with different priorities
	_, err := sub.SubscribeLow("test.priority", HandlerFunc(func(ctx context.Context, event any) error {
		mu.Lock()
		order = append(order, "low")
		mu.Unlock()
		return nil
	}), WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("SubscribeLow failed: %v", err)
	}

	_, err = sub.SubscribeCritical("test.priority", HandlerFunc(func(ctx context.Context, event any) error {
		mu.Lock()
		order = append(order, "critical")
		mu.Unlock()
		return nil
	}), WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("SubscribeCritical failed: %v", err)
	}

	env := Envelope{Topic: "test.priority", Payload: "hello"}
	if err := bus.PublishSync(context.Background(), env); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Critical should execute before low
	if len(order) != 2 {
		t.Fatalf("order has %d elements, want 2", len(order))
	}
	if order[0] != "critical" {
		t.Errorf("order[0] = %q, want %q", order[0], "critical")
	}
	if order[1] != "low" {
		t.Errorf("order[1] = %q, want %q", order[1], "low")
	}
}

func TestSubscriber_SubscribeWithFilter(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	var received []string
	var mu sync.Mutex

	filter := func(event any) bool {
		if env, ok := event.(Envelope); ok {
			if s, ok := env.Payload.(string); ok {
				return s == "allow"
			}
		}
		return false
	}

	_, err := sub.SubscribeWithFilter("test.filter", HandlerFunc(func(ctx context.Context, event any) error {
		mu.Lock()
		if env, ok := event.(Envelope); ok {
			received = append(received, env.Payload.(string))
		}
		mu.Unlock()
		return nil
	}), filter, WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("SubscribeWithFilter failed: %v", err)
	}

	// Publish events
	_ = bus.PublishSync(context.Background(), Envelope{Topic: "test.filter", Payload: "allow"})
	_ = bus.PublishSync(context.Background(), Envelope{Topic: "test.filter", Payload: "deny"})
	_ = bus.PublishSync(context.Background(), Envelope{Topic: "test.filter", Payload: "allow"})

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Errorf("received %d events, want 2", len(received))
	}
}

func TestSubscriber_Unsubscribe(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	var count int
	subscription, _ := sub.SubscribeFunc("test.unsub", func(ctx context.Context, event any) error {
		count++
		return nil
	}, WithDeliveryMode(DeliverySync))

	// First event
	env := Envelope{Topic: "test.unsub", Payload: "hello"}
	_ = bus.PublishSync(context.Background(), env)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Unsubscribe
	if err := sub.Unsubscribe(subscription); err != nil {
		t.Errorf("Unsubscribe failed: %v", err)
	}
	if sub.Count() != 0 {
		t.Errorf("Count = %d, want 0", sub.Count())
	}

	// Second event should not be received
	_ = bus.PublishSync(context.Background(), env)
	if count != 1 {
		t.Errorf("count = %d, want 1 after unsubscribe", count)
	}
}

func TestSubscriber_UnsubscribeAll(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	// Create multiple subscriptions
	for i := 0; i < 3; i++ {
		_, _ = sub.SubscribeFunc("test.all", func(ctx context.Context, event any) error {
			return nil
		})
	}

	if sub.Count() != 3 {
		t.Errorf("Count = %d, want 3", sub.Count())
	}

	sub.UnsubscribeAll()

	if sub.Count() != 0 {
		t.Errorf("Count = %d after UnsubscribeAll, want 0", sub.Count())
	}
}

func TestSubscriber_Close(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)

	_, _ = sub.SubscribeFunc("test.close", func(ctx context.Context, event any) error {
		return nil
	})

	if sub.IsClosed() {
		t.Error("Subscriber should not be closed initially")
	}

	if err := sub.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !sub.IsClosed() {
		t.Error("Subscriber should be closed after Close()")
	}

	// New subscriptions should fail
	_, err := sub.SubscribeFunc("test.after", func(ctx context.Context, event any) error {
		return nil
	})
	if err != ErrSubscriberClosed {
		t.Errorf("Subscribe after close: err = %v, want ErrSubscriberClosed", err)
	}
}

func TestSubscriptionGroup(t *testing.T) {
	bus := NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	sub := NewSubscriber(bus)
	group := NewSubscriptionGroup(sub)

	// Add multiple subscriptions
	var count atomic.Int32

	err := group.AddFunc("test.group.a", func(ctx context.Context, event any) error {
		count.Add(1)
		return nil
	}, WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	err = group.AddFunc("test.group.b", func(ctx context.Context, event any) error {
		count.Add(1)
		return nil
	}, WithDeliveryMode(DeliverySync))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if group.Count() != 2 {
		t.Errorf("Count = %d, want 2", group.Count())
	}

	// Pause all
	group.PauseAll()
	_ = bus.PublishSync(context.Background(), Envelope{Topic: "test.group.a", Payload: "hello"})
	_ = bus.PublishSync(context.Background(), Envelope{Topic: "test.group.b", Payload: "hello"})

	if count.Load() != 0 {
		t.Errorf("count = %d after pause, want 0", count.Load())
	}

	// Resume all
	group.ResumeAll()
	_ = bus.PublishSync(context.Background(), Envelope{Topic: "test.group.a", Payload: "hello"})
	_ = bus.PublishSync(context.Background(), Envelope{Topic: "test.group.b", Payload: "hello"})

	if count.Load() != 2 {
		t.Errorf("count = %d after resume, want 2", count.Load())
	}

	// Cancel all
	group.CancelAll()
	if group.Count() != 0 {
		t.Errorf("Count = %d after CancelAll, want 0", group.Count())
	}
}
