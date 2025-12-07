package event

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

func TestNewBus(t *testing.T) {
	bus := NewBus()
	if bus == nil {
		t.Fatal("NewBus() returned nil")
	}
}

func TestBus_StartStop(t *testing.T) {
	bus := NewBus()

	// Should start successfully
	if err := bus.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if !bus.IsRunning() {
		t.Error("expected bus to be running after Start()")
	}

	// Should fail to start again
	if err := bus.Start(); err != ErrBusAlreadyRunning {
		t.Errorf("expected ErrBusAlreadyRunning, got %v", err)
	}

	// Should stop successfully
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := bus.Stop(ctx); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if bus.IsRunning() {
		t.Error("expected bus to not be running after Stop()")
	}

	// Should fail to stop again
	if err := bus.Stop(ctx); err != ErrBusNotRunning {
		t.Errorf("expected ErrBusNotRunning, got %v", err)
	}
}

func TestBus_PauseResume(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	if bus.IsPaused() {
		t.Error("expected bus to not be paused initially")
	}

	bus.Pause()
	if !bus.IsPaused() {
		t.Error("expected bus to be paused after Pause()")
	}

	bus.Resume()
	if bus.IsPaused() {
		t.Error("expected bus to not be paused after Resume()")
	}
}

func TestBus_Subscribe(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	sub, err := bus.Subscribe(topic.Topic("test.event"), handler)
	if err != nil {
		t.Fatalf("Subscribe() failed: %v", err)
	}
	if sub == nil {
		t.Fatal("Subscribe() returned nil subscription")
	}
	if sub.Topic() != topic.Topic("test.event") {
		t.Errorf("expected topic 'test.event', got '%s'", sub.Topic())
	}
	if !sub.IsActive() {
		t.Error("expected subscription to be active")
	}
}

func TestBus_Subscribe_NilHandler(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	_, err := bus.Subscribe(topic.Topic("test.event"), nil)
	if err != ErrNilHandler {
		t.Errorf("expected ErrNilHandler, got %v", err)
	}
}

func TestBus_Subscribe_EmptyTopic(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	_, err := bus.Subscribe(topic.Topic(""), handler)
	if err != ErrInvalidTopic {
		t.Errorf("expected ErrInvalidTopic, got %v", err)
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	sub, _ := bus.Subscribe(topic.Topic("test.event"), handler)

	err := bus.Unsubscribe(sub)
	if err != nil {
		t.Fatalf("Unsubscribe() failed: %v", err)
	}

	// Subscription should be cancelled
	if sub.IsActive() {
		t.Error("expected subscription to be cancelled after Unsubscribe()")
	}

	// Should fail to unsubscribe again
	err = bus.Unsubscribe(sub)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestBus_PublishSync(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	received := make(chan struct{}, 1)

	_, err := bus.SubscribeFunc(topic.Topic("test.event"),
		func(ctx context.Context, event any) error {
			received <- struct{}{}
			return nil
		},
		WithDeliveryMode(DeliverySync),
	)
	if err != nil {
		t.Fatalf("Subscribe() failed: %v", err)
	}

	event := NewEvent(topic.Topic("test.event"), "payload", "test")
	err = bus.PublishSync(context.Background(), event)
	if err != nil {
		t.Fatalf("PublishSync() failed: %v", err)
	}

	select {
	case <-received:
		// Success - handler was called synchronously
	default:
		t.Fatal("handler was not called synchronously")
	}
}

func TestBus_PublishAsync(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	received := make(chan struct{}, 1)

	_, err := bus.SubscribeFunc(topic.Topic("test.event"),
		func(ctx context.Context, event any) error {
			received <- struct{}{}
			return nil
		},
		WithDeliveryMode(DeliveryAsync),
	)
	if err != nil {
		t.Fatalf("Subscribe() failed: %v", err)
	}

	event := NewEvent(topic.Topic("test.event"), "payload", "test")
	err = bus.PublishAsync(context.Background(), event)
	if err != nil {
		t.Fatalf("PublishAsync() failed: %v", err)
	}

	select {
	case <-received:
		// Success
	case <-time.After(time.Second):
		t.Fatal("handler was not called within timeout")
	}
}

func TestBus_Publish_NotRunning(t *testing.T) {
	bus := NewBus()

	event := NewEvent(topic.Topic("test.event"), "payload", "test")
	err := bus.Publish(context.Background(), event)
	if err != ErrBusNotRunning {
		t.Errorf("expected ErrBusNotRunning, got %v", err)
	}
}

func TestBus_Publish_Paused(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	received := make(chan struct{}, 1)

	bus.SubscribeFunc(topic.Topic("test.event"),
		func(ctx context.Context, event any) error {
			received <- struct{}{}
			return nil
		},
		WithDeliveryMode(DeliverySync),
	)

	bus.Pause()

	event := NewEvent(topic.Topic("test.event"), "payload", "test")
	err := bus.PublishSync(context.Background(), event)
	if err != nil {
		t.Fatalf("PublishSync() should not fail when paused, got: %v", err)
	}

	select {
	case <-received:
		t.Fatal("handler should not be called when paused")
	default:
		// Success - event was silently dropped
	}
}

func TestBus_WildcardSubscription(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	var received atomic.Int32

	// Subscribe to buffer.*
	bus.SubscribeFunc(topic.Topic("buffer.*"),
		func(ctx context.Context, event any) error {
			received.Add(1)
			return nil
		},
		WithDeliveryMode(DeliverySync),
	)

	// Publish different buffer events
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("buffer.inserted"), struct{}{}, "test"))
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("buffer.deleted"), struct{}{}, "test"))
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("cursor.moved"), struct{}{}, "test")) // Should not match

	if received.Load() != 2 {
		t.Errorf("expected 2 events received, got %d", received.Load())
	}
}

func TestBus_Priority(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	var order []string
	var mu sync.Mutex

	// Subscribe with different priorities (out of order)
	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			mu.Lock()
			order = append(order, "normal")
			mu.Unlock()
			return nil
		},
		WithPriority(PriorityNormal),
		WithDeliveryMode(DeliverySync),
	)

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			mu.Lock()
			order = append(order, "critical")
			mu.Unlock()
			return nil
		},
		WithPriority(PriorityCritical),
		WithDeliveryMode(DeliverySync),
	)

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			mu.Lock()
			order = append(order, "low")
			mu.Unlock()
			return nil
		},
		WithPriority(PriorityLow),
		WithDeliveryMode(DeliverySync),
	)

	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))

	expected := []string{"critical", "normal", "low"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d handlers, got %d", len(expected), len(order))
	}
	for i, e := range expected {
		if order[i] != e {
			t.Errorf("position %d: expected %s, got %s", i, e, order[i])
		}
	}
}

func TestBus_Filter(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	var received atomic.Int32

	// Subscribe with filter that only accepts certain events
	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			received.Add(1)
			return nil
		},
		WithDeliveryMode(DeliverySync),
		WithFilter(func(event any) bool {
			e, ok := event.(Event[string])
			return ok && e.Payload == "accept"
		}),
	)

	// Publish events
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), "accept", "test"))
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), "reject", "test"))
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), "accept", "test"))

	if received.Load() != 2 {
		t.Errorf("expected 2 events received (filtered), got %d", received.Load())
	}
}

func TestBus_Once(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	var received atomic.Int32

	sub, _ := bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			received.Add(1)
			return nil
		},
		WithDeliveryMode(DeliverySync),
		WithOnce(),
	)

	// Publish multiple events
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))

	if received.Load() != 1 {
		t.Errorf("expected 1 event received (once), got %d", received.Load())
	}

	// Subscription should be cancelled
	if sub.IsActive() {
		t.Error("expected subscription to be cancelled after once")
	}
}

func TestBus_HandlerError(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	handlerErr := errors.New("handler error")
	var executed atomic.Int32

	// Subscribe two handlers - first returns error, second should still run
	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			executed.Add(1)
			return handlerErr
		},
		WithDeliveryMode(DeliverySync),
		WithPriority(PriorityCritical),
	)

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			executed.Add(1)
			return nil
		},
		WithDeliveryMode(DeliverySync),
		WithPriority(PriorityNormal),
	)

	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))

	// Both handlers should have executed
	if executed.Load() != 2 {
		t.Errorf("expected 2 handlers executed, got %d", executed.Load())
	}

	// Stats should reflect the error
	stats := bus.Stats()
	if stats.HandlerErrors == 0 {
		t.Error("expected handler errors to be tracked")
	}
}

func TestBus_HandlerPanic(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	var executed atomic.Int32

	// Subscribe two handlers - first panics, second should still run
	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			executed.Add(1)
			panic("test panic")
		},
		WithDeliveryMode(DeliverySync),
		WithPriority(PriorityCritical),
	)

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			executed.Add(1)
			return nil
		},
		WithDeliveryMode(DeliverySync),
		WithPriority(PriorityNormal),
	)

	// Should not panic
	bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))

	// Both handlers should have executed
	if executed.Load() != 2 {
		t.Errorf("expected 2 handlers executed, got %d", executed.Load())
	}

	// Stats should reflect the panic
	stats := bus.Stats()
	if stats.HandlerPanics == 0 {
		t.Error("expected handler panics to be tracked")
	}
}

func TestBus_Stats(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			return nil
		},
		WithDeliveryMode(DeliverySync),
	)

	// Publish some events
	for i := 0; i < 5; i++ {
		bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))
	}

	stats := bus.Stats()
	if stats.EventsPublished != 5 {
		t.Errorf("expected 5 events published, got %d", stats.EventsPublished)
	}
	if stats.EventsDelivered != 5 {
		t.Errorf("expected 5 events delivered, got %d", stats.EventsDelivered)
	}
	if stats.HandlersExecuted != 5 {
		t.Errorf("expected 5 handlers executed, got %d", stats.HandlersExecuted)
	}
	if stats.ActiveSubscribers != 1 {
		t.Errorf("expected 1 active subscriber, got %d", stats.ActiveSubscribers)
	}
}

func TestBus_ConcurrentPublish(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	var received atomic.Int32

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error {
			received.Add(1)
			return nil
		},
		WithDeliveryMode(DeliverySync),
	)

	// Publish concurrently
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.PublishSync(context.Background(), NewEvent(topic.Topic("test"), struct{}{}, "test"))
		}()
	}
	wg.Wait()

	if received.Load() != 100 {
		t.Errorf("expected 100 events received, got %d", received.Load())
	}
}

func TestBus_ConcurrentSubscribe(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	var subscribed atomic.Int32
	var wg sync.WaitGroup

	// Subscribe concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := bus.SubscribeFunc(topic.Topic("test"),
				func(ctx context.Context, event any) error {
					return nil
				},
			)
			if err == nil {
				subscribed.Add(1)
			}
		}()
	}
	wg.Wait()

	if subscribed.Load() != 100 {
		t.Errorf("expected 100 subscriptions, got %d", subscribed.Load())
	}

	stats := bus.Stats()
	if stats.ActiveSubscribers != 100 {
		t.Errorf("expected 100 active subscribers, got %d", stats.ActiveSubscribers)
	}
}

func TestBus_Envelope(t *testing.T) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	received := make(chan struct{}, 1)

	bus.SubscribeFunc(topic.Topic("test.event"),
		func(ctx context.Context, event any) error {
			received <- struct{}{}
			return nil
		},
		WithDeliveryMode(DeliverySync),
	)

	// Publish using Envelope
	env := Envelope{
		Topic:   topic.Topic("test.event"),
		Payload: "payload",
		Metadata: Metadata{
			ID:     "test-id",
			Source: "test",
		},
	}
	err := bus.PublishSync(context.Background(), env)
	if err != nil {
		t.Fatalf("PublishSync() with Envelope failed: %v", err)
	}

	select {
	case <-received:
		// Success
	default:
		t.Fatal("handler was not called for Envelope")
	}
}

func BenchmarkBus_PublishSync(b *testing.B) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error { return nil },
		WithDeliveryMode(DeliverySync),
	)

	event := NewEvent(topic.Topic("test"), struct{}{}, "bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.PublishSync(ctx, event)
	}
}

func BenchmarkBus_PublishAsync(b *testing.B) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	bus.SubscribeFunc(topic.Topic("test"),
		func(ctx context.Context, event any) error { return nil },
		WithDeliveryMode(DeliveryAsync),
	)

	event := NewEvent(topic.Topic("test"), struct{}{}, "bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.PublishAsync(ctx, event)
	}
}

func BenchmarkBus_Subscribe(b *testing.B) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	handler := HandlerFunc(func(ctx context.Context, event any) error { return nil })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Subscribe(topic.Topic("test"), handler)
	}
}

func BenchmarkBus_ManySubscribers(b *testing.B) {
	bus := NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	// Add many subscribers
	for i := 0; i < 100; i++ {
		bus.SubscribeFunc(topic.Topic("test"),
			func(ctx context.Context, event any) error { return nil },
			WithDeliveryMode(DeliverySync),
		)
	}

	event := NewEvent(topic.Topic("test"), struct{}{}, "bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.PublishSync(ctx, event)
	}
}
