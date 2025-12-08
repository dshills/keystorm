package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/event"
)

func TestIntegrationTopics(t *testing.T) {
	// Verify all topic strings are non-empty
	topics := AllIntegrationTopics()
	if len(topics) == 0 {
		t.Fatal("AllIntegrationTopics returned empty slice")
	}

	for i, topic := range topics {
		if topic == "" {
			t.Errorf("topic[%d] is empty", i)
		}
	}
}

func TestWildcardTopics(t *testing.T) {
	wildcards := WildcardTopics()
	if len(wildcards) == 0 {
		t.Fatal("WildcardTopics returned empty slice")
	}

	// Verify expected wildcard patterns
	expected := map[string]bool{
		"integration.*": true,
		"terminal.*":    true,
		"git.*":         true,
		"debug.*":       true,
		"task.*":        true,
	}

	for _, w := range wildcards {
		if !expected[string(w)] {
			t.Errorf("unexpected wildcard: %s", w)
		}
	}
}

func TestNewEventBridge(t *testing.T) {
	bus := event.NewBus()
	bridge := NewEventBridge(bus)

	if bridge == nil {
		t.Fatal("NewEventBridge returned nil")
	}

	if bridge.Bus() != bus {
		t.Error("Bus() returned different bus")
	}

	if bridge.Publisher() == nil {
		t.Error("Publisher() returned nil")
	}
}

func TestEventBridge_Publish(t *testing.T) {
	bus := event.NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	bridge := NewEventBridge(bus)
	defer bridge.Close()

	var received bool
	var mu sync.Mutex

	// Subscribe to test topic
	bridge.Subscribe("test.event", func(data map[string]any) {
		mu.Lock()
		received = true
		mu.Unlock()
	})

	// Publish event
	bridge.Publish("test.event", map[string]any{"key": "value"})

	// Wait for async delivery
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if !received {
		t.Error("Event was not received")
	}
	mu.Unlock()
}

func TestEventBridge_Subscribe(t *testing.T) {
	bus := event.NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	bridge := NewEventBridge(bus)
	defer bridge.Close()

	id := bridge.Subscribe("test.topic", func(data map[string]any) {})
	if id == "" {
		t.Error("Subscribe returned empty ID")
	}

	if bridge.SubscriptionCount() != 1 {
		t.Errorf("SubscriptionCount = %d, want 1", bridge.SubscriptionCount())
	}
}

func TestEventBridge_Unsubscribe(t *testing.T) {
	bus := event.NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	bridge := NewEventBridge(bus)
	defer bridge.Close()

	id := bridge.Subscribe("test.topic", func(data map[string]any) {})
	if bridge.SubscriptionCount() != 1 {
		t.Fatalf("SubscriptionCount = %d, want 1", bridge.SubscriptionCount())
	}

	ok := bridge.Unsubscribe(id)
	if !ok {
		t.Error("Unsubscribe returned false")
	}

	if bridge.SubscriptionCount() != 0 {
		t.Errorf("SubscriptionCount = %d, want 0", bridge.SubscriptionCount())
	}
}

func TestEventBridge_SetManager(t *testing.T) {
	bus := event.NewBus()
	bridge := NewEventBridge(bus)
	defer bridge.Close()

	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer m.Close()

	// Should not panic
	bridge.SetManager(m)

	// Manager's event bus should be set to bridge's publisher
	if m.EventBus() != bridge.Publisher() {
		t.Error("Manager's event bus was not set to bridge's publisher")
	}
}

func TestEventBridge_Close(t *testing.T) {
	bus := event.NewBus()
	bridge := NewEventBridge(bus)

	// Subscribe to something
	bridge.Subscribe("test.topic", func(data map[string]any) {})

	// Close should succeed
	err := bridge.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Second close should be idempotent
	err = bridge.Close()
	if err != nil {
		t.Errorf("Second Close returned error: %v", err)
	}

	// Subscribe after close should return empty ID
	id := bridge.Subscribe("another.topic", func(data map[string]any) {})
	if id != "" {
		t.Error("Subscribe after close should return empty ID")
	}
}

func TestEventBridge_PublishSync(t *testing.T) {
	bus := event.NewBus()
	if err := bus.Start(); err != nil {
		t.Fatalf("bus.Start failed: %v", err)
	}
	defer bus.Stop(context.Background())

	bridge := NewEventBridge(bus)
	defer bridge.Close()

	// Subscribe with sync delivery
	var received bool
	_, err := bus.SubscribeFunc("test.sync", func(ctx context.Context, evt any) error {
		received = true
		return nil
	}, event.WithDeliveryMode(event.DeliverySync))
	if err != nil {
		t.Fatalf("SubscribeFunc failed: %v", err)
	}

	// PublishSync should work
	err = bridge.PublishSync("test.sync", map[string]any{"key": "value"})
	if err != nil {
		t.Errorf("PublishSync returned error: %v", err)
	}

	// Event should be received immediately
	if !received {
		t.Error("Event was not received synchronously")
	}
}

func TestIntegrationTopics_Names(t *testing.T) {
	// Verify specific topic names match expected values
	tests := []struct {
		name     string
		topic    string
		expected string
	}{
		{"ManagerStarted", string(IntegrationTopics.ManagerStarted), "integration.started"},
		{"ManagerStopping", string(IntegrationTopics.ManagerStopping), "integration.stopping"},
		{"ManagerStopped", string(IntegrationTopics.ManagerStopped), "integration.stopped"},
		{"WorkspaceChanged", string(IntegrationTopics.WorkspaceChanged), "integration.workspace.changed"},
		{"TerminalCreated", string(IntegrationTopics.TerminalCreated), "terminal.created"},
		{"GitStatusChanged", string(IntegrationTopics.GitStatusChanged), "git.status.changed"},
		{"DebugSessionStarted", string(IntegrationTopics.DebugSessionStarted), "debug.session.started"},
		{"TaskStarted", string(IntegrationTopics.TaskStarted), "task.started"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.topic != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.topic, tt.expected)
			}
		})
	}
}
