package integration

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBus_Subscribe(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	id := bus.Subscribe("test.event", func(data map[string]any) {})

	if id == "" {
		t.Error("Subscribe should return a non-empty ID")
	}

	if bus.SubscriptionCount() != 1 {
		t.Errorf("SubscriptionCount() = %d, want 1", bus.SubscriptionCount())
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	id := bus.Subscribe("test.event", func(data map[string]any) {})

	// Unsubscribe should return true for existing subscription
	if !bus.Unsubscribe(id) {
		t.Error("Unsubscribe should return true for existing subscription")
	}

	if bus.SubscriptionCount() != 0 {
		t.Errorf("SubscriptionCount() = %d after unsubscribe, want 0", bus.SubscriptionCount())
	}

	// Unsubscribe again should return false
	if bus.Unsubscribe(id) {
		t.Error("Unsubscribe should return false for non-existing subscription")
	}
}

func TestEventBus_Emit(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var received bool
	var receivedData map[string]any
	var mu sync.Mutex

	bus.Subscribe("test.event", func(data map[string]any) {
		mu.Lock()
		received = true
		receivedData = data
		mu.Unlock()
	})

	testData := map[string]any{"key": "value"}
	bus.Emit("test.event", testData)

	mu.Lock()
	if !received {
		t.Error("Handler was not called")
	}
	if receivedData["key"] != "value" {
		t.Errorf("Received data = %v, want key='value'", receivedData)
	}
	mu.Unlock()
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var count atomic.Int32

	bus.Subscribe("test.event", func(data map[string]any) {
		count.Add(1)
	})
	bus.Subscribe("test.event", func(data map[string]any) {
		count.Add(1)
	})

	bus.Emit("test.event", nil)

	if count.Load() != 2 {
		t.Errorf("Expected 2 handlers called, got %d", count.Load())
	}
}

func TestEventBus_WildcardSubscription(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var events []string
	var mu sync.Mutex

	bus.Subscribe("git.*", func(data map[string]any) {
		mu.Lock()
		events = append(events, data["type"].(string))
		mu.Unlock()
	})

	bus.Emit("git.commit", map[string]any{"type": "git.commit"})
	bus.Emit("git.push", map[string]any{"type": "git.push"})
	bus.Emit("terminal.output", map[string]any{"type": "terminal.output"})

	mu.Lock()
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d: %v", len(events), events)
	}
	mu.Unlock()
}

func TestEventBus_WildcardAndExact(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var count atomic.Int32

	bus.Subscribe("git.*", func(data map[string]any) {
		count.Add(1)
	})
	bus.Subscribe("git.commit", func(data map[string]any) {
		count.Add(10)
	})

	bus.Emit("git.commit", nil)

	// Both handlers should be called (1 + 10 = 11)
	if count.Load() != 11 {
		t.Errorf("Expected count 11, got %d", count.Load())
	}
}

func TestEventBus_EmitAsync(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var done sync.WaitGroup
	done.Add(2)

	bus.Subscribe("async.test", func(data map[string]any) {
		time.Sleep(10 * time.Millisecond)
		done.Done()
	})
	bus.Subscribe("async.test", func(data map[string]any) {
		time.Sleep(10 * time.Millisecond)
		done.Done()
	})

	start := time.Now()
	bus.EmitAsync("async.test", nil)

	// EmitAsync should return immediately
	if time.Since(start) > 5*time.Millisecond {
		t.Error("EmitAsync should return immediately")
	}

	// Wait for handlers to complete
	done.Wait()
}

func TestEventBus_HandlerPanicRecovery(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var secondHandlerCalled atomic.Bool

	bus.Subscribe("panic.test", func(data map[string]any) {
		panic("test panic")
	})
	bus.Subscribe("panic.test", func(data map[string]any) {
		secondHandlerCalled.Store(true)
	})

	// Should not panic and should call second handler
	bus.Emit("panic.test", nil)

	if !secondHandlerCalled.Load() {
		t.Error("Second handler should still be called after first handler panics")
	}
}

func TestEventBus_Close(t *testing.T) {
	bus := NewEventBus()

	bus.Subscribe("test", func(data map[string]any) {})
	bus.Close()

	// Subscribe after close should return empty ID
	id := bus.Subscribe("test", func(data map[string]any) {})
	if id != "" {
		t.Error("Subscribe after close should return empty ID")
	}

	// Emit after close should not panic
	bus.Emit("test", nil)
}

func TestEventBus_ConcurrentAccess(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var wg sync.WaitGroup
	var count atomic.Int32

	// Multiple goroutines subscribing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Subscribe("concurrent.test", func(data map[string]any) {
				count.Add(1)
			})
		}()
	}
	wg.Wait()

	// Multiple goroutines emitting
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Emit("concurrent.test", nil)
		}()
	}
	wg.Wait()

	// Should have processed all events (10 subscribers * 10 events = 100)
	if count.Load() != 100 {
		t.Errorf("Expected 100 handler calls, got %d", count.Load())
	}
}

func TestEventBus_Publish(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	var received atomic.Bool

	bus.Subscribe("publish.test", func(data map[string]any) {
		received.Store(true)
	})

	// Publish is alias for Emit
	bus.Publish("publish.test", nil)

	if !received.Load() {
		t.Error("Publish should trigger handler")
	}
}

func TestEventBus_SubscribersFor(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	bus.Subscribe("event.a", func(data map[string]any) {})
	bus.Subscribe("event.a", func(data map[string]any) {})
	bus.Subscribe("event.b", func(data map[string]any) {})

	if bus.SubscribersFor("event.a") != 2 {
		t.Errorf("SubscribersFor('event.a') = %d, want 2", bus.SubscribersFor("event.a"))
	}
	if bus.SubscribersFor("event.b") != 1 {
		t.Errorf("SubscribersFor('event.b') = %d, want 1", bus.SubscribersFor("event.b"))
	}
	if bus.SubscribersFor("event.c") != 0 {
		t.Errorf("SubscribersFor('event.c') = %d, want 0", bus.SubscribersFor("event.c"))
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern   string
		eventType string
		want      bool
	}{
		{"git.*", "git.commit", true},
		{"git.*", "git.push", true},
		{"git.*", "git", false},
		{"git.*", "gitcommit", false},
		{"git.*", "terminal.output", false},
		{"terminal.*", "terminal.created", true},
		{"a.b.*", "a.b.c", true},
		{"a.b.*", "a.b", false},
		{"exact.match", "exact.match", true},
		{"exact.match", "exact.match.extra", false},
	}

	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.eventType)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.eventType, got, tt.want)
		}
	}
}

func TestIsWildcard(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{"git.*", true},
		{"git.commit", false},
		{".*", true},
		{"", false},
		{"a", false},
		{"a.*", true},
	}

	for _, tt := range tests {
		got := isWildcard(tt.eventType)
		if got != tt.want {
			t.Errorf("isWildcard(%q) = %v, want %v", tt.eventType, got, tt.want)
		}
	}
}
