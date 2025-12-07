package event

import (
	"context"
	"sync"
	"testing"

	"github.com/dshills/keystorm/internal/event/topic"
)

func TestSubscriptionState_String(t *testing.T) {
	tests := []struct {
		state    SubscriptionState
		expected string
	}{
		{SubscriptionStateActive, "active"},
		{SubscriptionStatePaused, "paused"},
		{SubscriptionStateCancelled, "cancelled"},
		{SubscriptionState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("SubscriptionState.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewSubscription(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	sub := newSubscription("sub-1", topic.Topic("buffer.content.inserted"), handler)

	if sub.ID() != "sub-1" {
		t.Errorf("expected ID sub-1, got %v", sub.ID())
	}
	if sub.Topic() != topic.Topic("buffer.content.inserted") {
		t.Errorf("expected topic buffer.content.inserted, got %v", sub.Topic())
	}
	if !sub.IsActive() {
		t.Error("expected subscription to be active")
	}
	if sub.Handler() == nil {
		t.Error("expected handler to be set")
	}
}

func TestNewSubscription_WithOptions(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	filter := func(event any) bool { return true }

	sub := newSubscription(
		"sub-2",
		topic.Topic("config.changed"),
		handler,
		WithPriority(PriorityHigh),
		WithDeliveryMode(DeliveryAsync),
		WithFilter(filter),
		WithOnce(),
	)

	config := sub.Config()
	if config.Priority != PriorityHigh {
		t.Errorf("expected priority PriorityHigh, got %v", config.Priority)
	}
	if config.DeliveryMode != DeliveryAsync {
		t.Errorf("expected delivery mode DeliveryAsync, got %v", config.DeliveryMode)
	}
	if config.Filter == nil {
		t.Error("expected filter to be set")
	}
	if !config.Once {
		t.Error("expected once to be true")
	}
}

func TestDefaultSubscriptionConfig(t *testing.T) {
	config := DefaultSubscriptionConfig()

	if config.Priority != PriorityNormal {
		t.Errorf("expected priority PriorityNormal, got %v", config.Priority)
	}
	if config.DeliveryMode != DeliverySync {
		t.Errorf("expected delivery mode DeliverySync, got %v", config.DeliveryMode)
	}
	if config.Filter != nil {
		t.Error("expected filter to be nil")
	}
	if config.Once {
		t.Error("expected once to be false")
	}
}

func TestSubscription_Lifecycle(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	sub := newSubscription("sub-3", topic.Topic("test"), handler)

	// Initial state should be active
	if sub.State() != SubscriptionStateActive {
		t.Errorf("expected state Active, got %v", sub.State())
	}
	if !sub.IsActive() {
		t.Error("expected IsActive to be true")
	}

	// Pause
	sub.Pause()
	if sub.State() != SubscriptionStatePaused {
		t.Errorf("expected state Paused, got %v", sub.State())
	}
	if !sub.IsPaused() {
		t.Error("expected IsPaused to be true")
	}
	if sub.IsActive() {
		t.Error("expected IsActive to be false when paused")
	}

	// Resume
	sub.Resume()
	if sub.State() != SubscriptionStateActive {
		t.Errorf("expected state Active after resume, got %v", sub.State())
	}
	if !sub.IsActive() {
		t.Error("expected IsActive to be true after resume")
	}

	// Cancel
	sub.Cancel()
	if sub.State() != SubscriptionStateCancelled {
		t.Errorf("expected state Cancelled, got %v", sub.State())
	}
	if !sub.IsCancelled() {
		t.Error("expected IsCancelled to be true")
	}
	if sub.IsActive() {
		t.Error("expected IsActive to be false when cancelled")
	}
}

func TestSubscription_PauseResume_OnlyCancelled(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	sub := newSubscription("sub-4", topic.Topic("test"), handler)

	// Cancel first
	sub.Cancel()

	// Try to pause - should have no effect
	sub.Pause()
	if sub.State() != SubscriptionStateCancelled {
		t.Error("pause should not change cancelled state")
	}

	// Try to resume - should have no effect
	sub.Resume()
	if sub.State() != SubscriptionStateCancelled {
		t.Error("resume should not change cancelled state")
	}
}

func TestSubscription_ResumeOnlyFromPaused(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	sub := newSubscription("sub-5", topic.Topic("test"), handler)

	// Try to resume from active - should have no effect
	sub.Resume()
	if sub.State() != SubscriptionStateActive {
		t.Error("resume from active should have no effect")
	}
}

func TestSubscription_ShouldDeliver(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	t.Run("active subscription delivers", func(t *testing.T) {
		sub := newSubscription("sub-1", topic.Topic("test"), handler)
		if !sub.ShouldDeliver("event") {
			t.Error("active subscription should deliver")
		}
	})

	t.Run("paused subscription does not deliver", func(t *testing.T) {
		sub := newSubscription("sub-2", topic.Topic("test"), handler)
		sub.Pause()
		if sub.ShouldDeliver("event") {
			t.Error("paused subscription should not deliver")
		}
	})

	t.Run("cancelled subscription does not deliver", func(t *testing.T) {
		sub := newSubscription("sub-3", topic.Topic("test"), handler)
		sub.Cancel()
		if sub.ShouldDeliver("event") {
			t.Error("cancelled subscription should not deliver")
		}
	})

	t.Run("filter allows event", func(t *testing.T) {
		sub := newSubscription("sub-4", topic.Topic("test"), handler,
			WithFilter(func(event any) bool {
				return event == "allowed"
			}),
		)
		if !sub.ShouldDeliver("allowed") {
			t.Error("filter should allow 'allowed' event")
		}
	})

	t.Run("filter blocks event", func(t *testing.T) {
		sub := newSubscription("sub-5", topic.Topic("test"), handler,
			WithFilter(func(event any) bool {
				return event == "allowed"
			}),
		)
		if sub.ShouldDeliver("blocked") {
			t.Error("filter should block 'blocked' event")
		}
	})
}

func TestSubscription_Concurrent(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})

	sub := newSubscription("sub-concurrent", topic.Topic("test"), handler)

	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent state checks
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = sub.IsActive()
				_ = sub.IsPaused()
				_ = sub.IsCancelled()
				_ = sub.State()
			}
		}()
	}

	// Concurrent pause/resume
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				sub.Pause()
				sub.Resume()
			}
		}()
	}

	// Concurrent ShouldDeliver
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = sub.ShouldDeliver("event")
			}
		}()
	}

	wg.Wait()
}

func BenchmarkSubscription_IsActive(b *testing.B) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})
	sub := newSubscription("bench", topic.Topic("test"), handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sub.IsActive()
	}
}

func BenchmarkSubscription_ShouldDeliver(b *testing.B) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})
	sub := newSubscription("bench", topic.Topic("test"), handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sub.ShouldDeliver("event")
	}
}

func BenchmarkSubscription_ShouldDeliver_WithFilter(b *testing.B) {
	handler := HandlerFunc(func(ctx context.Context, event any) error {
		return nil
	})
	sub := newSubscription("bench", topic.Topic("test"), handler,
		WithFilter(func(event any) bool { return true }),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sub.ShouldDeliver("event")
	}
}
