package api

import (
	"fmt"
	"sync"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockEventProvider implements EventProvider for testing.
type mockEventProvider struct {
	mu            sync.Mutex
	subscriptions map[string]subscriptionEntry
	emitted       []emittedEvent
	nextID        int
}

type subscriptionEntry struct {
	eventType string
	handler   func(data map[string]any)
}

type emittedEvent struct {
	eventType string
	data      map[string]any
}

func newMockEventProvider() *mockEventProvider {
	return &mockEventProvider{
		subscriptions: make(map[string]subscriptionEntry),
	}
}

func (m *mockEventProvider) Subscribe(eventType string, handler func(data map[string]any)) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("sub-%d", m.nextID)
	m.subscriptions[id] = subscriptionEntry{
		eventType: eventType,
		handler:   handler,
	}
	return id
}

func (m *mockEventProvider) Unsubscribe(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.subscriptions[id]
	if exists {
		delete(m.subscriptions, id)
	}
	return exists
}

func (m *mockEventProvider) Emit(eventType string, data map[string]any) {
	m.mu.Lock()
	m.emitted = append(m.emitted, emittedEvent{eventType, data})
	// Copy handlers to avoid holding lock during callback
	var handlers []func(data map[string]any)
	for _, entry := range m.subscriptions {
		if entry.eventType == eventType {
			handlers = append(handlers, entry.handler)
		}
	}
	m.mu.Unlock()

	// Call handlers outside lock
	for _, h := range handlers {
		h(data)
	}
}

func (m *mockEventProvider) GetEmitted() []emittedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]emittedEvent, len(m.emitted))
	copy(result, m.emitted)
	return result
}

func (m *mockEventProvider) SubscriptionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subscriptions)
}

func setupEventTest(t *testing.T, ep *mockEventProvider) (*lua.LState, *EventModule) {
	t.Helper()

	ctx := &Context{Event: ep}
	mod := NewEventModule(ctx, "testplugin")

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestEventModuleName(t *testing.T) {
	ctx := &Context{}
	mod := NewEventModule(ctx, "test")
	if mod.Name() != "event" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "event")
	}
}

func TestEventModuleCapability(t *testing.T) {
	ctx := &Context{}
	mod := NewEventModule(ctx, "test")
	if mod.RequiredCapability() != security.CapabilityEvent {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityEvent)
	}
}

func TestEventOn(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		received = nil
		sub_id = _ks_event.on("buffer.change", function(data)
			received = data
		end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Check subscription ID was returned
	subID := L.GetGlobal("sub_id")
	if subID == lua.LNil {
		t.Fatal("subscription ID should not be nil")
	}
	if _, ok := subID.(lua.LString); !ok {
		t.Fatalf("subscription ID should be a string, got %T", subID)
	}

	// Verify subscription was created
	if ep.SubscriptionCount() != 1 {
		t.Errorf("subscription count = %d, want 1", ep.SubscriptionCount())
	}

	// Emit an event and check handler was called
	ep.Emit("buffer.change", map[string]any{"file": "test.go"})

	// Give the handler time to execute
	time.Sleep(10 * time.Millisecond)

	received := L.GetGlobal("received")
	if received == lua.LNil {
		t.Fatal("handler should have received data")
	}
}

func TestEventOnWithData(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		received_file = nil
		received_line = nil
		_ks_event.on("buffer.change", function(data)
			received_file = data.file
			received_line = data.line
		end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Emit an event with data
	ep.Emit("buffer.change", map[string]any{
		"file": "test.go",
		"line": 42,
	})

	// Give the handler time to execute
	time.Sleep(10 * time.Millisecond)

	file := L.GetGlobal("received_file")
	if file.(lua.LString) != "test.go" {
		t.Errorf("received_file = %v, want 'test.go'", file)
	}

	line := L.GetGlobal("received_line")
	if line.(lua.LNumber) != 42 {
		t.Errorf("received_line = %v, want 42", line)
	}
}

func TestEventOff(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		sub_id = _ks_event.on("test.event", function(data) end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if ep.SubscriptionCount() != 1 {
		t.Fatalf("subscription count = %d, want 1", ep.SubscriptionCount())
	}

	// Unsubscribe
	err = L.DoString(`
		result = _ks_event.off(sub_id)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Error("off should return true for existing subscription")
	}

	if ep.SubscriptionCount() != 0 {
		t.Errorf("subscription count = %d, want 0", ep.SubscriptionCount())
	}
}

func TestEventOffNotFound(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		result = _ks_event.off("nonexistent")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LFalse {
		t.Error("off should return false for nonexistent subscription")
	}
}

func TestEventOnce(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		call_count = 0
		_ks_event.once("test.event", function(data)
			call_count = call_count + 1
		end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Emit twice
	ep.Emit("test.event", nil)
	time.Sleep(10 * time.Millisecond)
	ep.Emit("test.event", nil)
	time.Sleep(10 * time.Millisecond)

	callCount := L.GetGlobal("call_count")
	if callCount.(lua.LNumber) != 1 {
		t.Errorf("call_count = %v, want 1 (once should only fire once)", callCount)
	}

	// Subscription should be removed
	if ep.SubscriptionCount() != 0 {
		t.Errorf("subscription count = %d, want 0 after once fired", ep.SubscriptionCount())
	}
}

func TestEventEmit(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		_ks_event.emit("custom", { message = "hello", count = 5 })
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	emitted := ep.GetEmitted()
	if len(emitted) != 1 {
		t.Fatalf("emitted event count = %d, want 1", len(emitted))
	}

	// Check event type is prefixed with plugin namespace
	if emitted[0].eventType != "plugin.testplugin.custom" {
		t.Errorf("event type = %q, want %q", emitted[0].eventType, "plugin.testplugin.custom")
	}

	// Check data
	if emitted[0].data["message"] != "hello" {
		t.Errorf("data.message = %v, want 'hello'", emitted[0].data["message"])
	}
	if emitted[0].data["count"] != float64(5) {
		t.Errorf("data.count = %v, want 5", emitted[0].data["count"])
	}
	if emitted[0].data["source"] != "plugin:testplugin" {
		t.Errorf("data.source = %v, want 'plugin:testplugin'", emitted[0].data["source"])
	}
}

func TestEventEmitWithoutData(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		_ks_event.emit("simple")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	emitted := ep.GetEmitted()
	if len(emitted) != 1 {
		t.Fatalf("emitted event count = %d, want 1", len(emitted))
	}

	// Data should have source and event_type even without explicit data
	if emitted[0].data["source"] != "plugin:testplugin" {
		t.Errorf("data.source = %v, want 'plugin:testplugin'", emitted[0].data["source"])
	}
}

func TestEventOnEmptyType(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		_ks_event.on("", function() end)
	`)
	if err == nil {
		t.Error("on with empty event type should error")
	}
}

func TestEventEmitEmptyType(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		_ks_event.emit("")
	`)
	if err == nil {
		t.Error("emit with empty event type should error")
	}
}

func TestEventNilProvider(t *testing.T) {
	ctx := &Context{Event: nil}
	mod := NewEventModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// on should error with nil provider
	err := L.DoString(`
		_ks_event.on("test", function() end)
	`)
	if err == nil {
		t.Error("on should error when provider is nil")
	}

	// off should return false with nil provider
	err = L.DoString(`
		result = _ks_event.off("any")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}
	result := L.GetGlobal("result")
	if result != lua.LFalse {
		t.Error("off should return false when provider is nil")
	}

	// emit should error with nil provider
	err = L.DoString(`
		_ks_event.emit("test")
	`)
	if err == nil {
		t.Error("emit should error when provider is nil")
	}
}

func TestEventCleanup(t *testing.T) {
	ep := newMockEventProvider()
	L, mod := setupEventTest(t, ep)

	// Create multiple subscriptions
	err := L.DoString(`
		_ks_event.on("event1", function() end)
		_ks_event.on("event2", function() end)
		_ks_event.on("event3", function() end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if ep.SubscriptionCount() != 3 {
		t.Fatalf("subscription count = %d, want 3", ep.SubscriptionCount())
	}

	// Cleanup should unsubscribe all
	mod.Cleanup()

	if ep.SubscriptionCount() != 0 {
		t.Errorf("subscription count after cleanup = %d, want 0", ep.SubscriptionCount())
	}
}

func TestEventMultipleSubscriptions(t *testing.T) {
	ep := newMockEventProvider()
	L, _ := setupEventTest(t, ep)

	err := L.DoString(`
		count1 = 0
		count2 = 0
		_ks_event.on("same.event", function() count1 = count1 + 1 end)
		_ks_event.on("same.event", function() count2 = count2 + 1 end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Emit event
	ep.Emit("same.event", nil)
	time.Sleep(10 * time.Millisecond)

	count1 := L.GetGlobal("count1")
	count2 := L.GetGlobal("count2")

	if count1.(lua.LNumber) != 1 {
		t.Errorf("count1 = %v, want 1", count1)
	}
	if count2.(lua.LNumber) != 1 {
		t.Errorf("count2 = %v, want 1", count2)
	}
}
