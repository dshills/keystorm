package watcher

import (
	"sync"
	"testing"
	"time"
)

// mockWatcher is a simple mock for testing DebouncedWatcher.
type mockWatcher struct {
	mu       sync.Mutex
	events   chan Event
	errors   chan error
	watching map[string]bool
	closed   bool
}

func newMockWatcher() *mockWatcher {
	return &mockWatcher{
		events:   make(chan Event, 100),
		errors:   make(chan error, 100),
		watching: make(map[string]bool),
	}
}

func (m *mockWatcher) Watch(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watching[path] = true
	return nil
}

func (m *mockWatcher) WatchRecursive(path string) error {
	return m.Watch(path)
}

func (m *mockWatcher) Unwatch(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.watching, path)
	return nil
}

func (m *mockWatcher) Events() <-chan Event {
	return m.events
}

func (m *mockWatcher) Errors() <-chan error {
	return m.errors
}

func (m *mockWatcher) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.events)
		close(m.errors)
	}
	return nil
}

func (m *mockWatcher) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Stats{
		WatchedPaths: len(m.watching),
		StartTime:    time.Now(),
	}
}

func (m *mockWatcher) IsWatching(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watching[path]
}

func (m *mockWatcher) WatchedPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	paths := make([]string, 0, len(m.watching))
	for p := range m.watching {
		paths = append(paths, p)
	}
	return paths
}

// sendEvent sends an event to the mock watcher.
func (m *mockWatcher) sendEvent(event Event) {
	m.events <- event
}

// sendError sends an error to the mock watcher.
func (m *mockWatcher) sendError(err error) {
	m.errors <- err
}

func TestNewDebouncedWatcher(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 50*time.Millisecond)
	defer dw.Close()

	if dw.inner != mock {
		t.Error("inner watcher not set")
	}
	if dw.delay != 50*time.Millisecond {
		t.Errorf("delay = %v, want 50ms", dw.delay)
	}
}

func TestNewDebouncedWatcher_DefaultDelay(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 0)
	defer dw.Close()

	if dw.delay != 100*time.Millisecond {
		t.Errorf("delay = %v, want 100ms (default)", dw.delay)
	}
}

func TestDebouncedWatcher_PassThrough(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 50*time.Millisecond)
	defer dw.Close()

	// Test Watch passes through
	if err := dw.Watch("/test"); err != nil {
		t.Errorf("Watch error = %v", err)
	}
	if !mock.IsWatching("/test") {
		t.Error("mock should be watching /test")
	}

	// Test IsWatching passes through
	if !dw.IsWatching("/test") {
		t.Error("dw should report watching /test")
	}

	// Test WatchedPaths passes through
	paths := dw.WatchedPaths()
	if len(paths) != 1 || paths[0] != "/test" {
		t.Errorf("WatchedPaths = %v, want [/test]", paths)
	}

	// Test Unwatch passes through
	if err := dw.Unwatch("/test"); err != nil {
		t.Errorf("Unwatch error = %v", err)
	}
	if mock.IsWatching("/test") {
		t.Error("mock should not be watching /test after Unwatch")
	}
}

func TestDebouncedWatcher_SingleEvent(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 50*time.Millisecond)
	defer dw.Close()

	// Send a single event
	event := Event{
		Path:      "/test/file.txt",
		Op:        OpWrite,
		Timestamp: time.Now(),
	}
	mock.sendEvent(event)

	// Should receive after debounce delay
	select {
	case received := <-dw.Events():
		if received.Path != event.Path {
			t.Errorf("received.Path = %q, want %q", received.Path, event.Path)
		}
		if received.Op != OpWrite {
			t.Errorf("received.Op = %v, want OpWrite", received.Op)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for debounced event")
	}
}

func TestDebouncedWatcher_EventCoalescing(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 100*time.Millisecond)
	defer dw.Close()

	path := "/test/file.txt"
	now := time.Now()

	// Send multiple events for the same path rapidly
	mock.sendEvent(Event{Path: path, Op: OpCreate, Timestamp: now})
	time.Sleep(20 * time.Millisecond)
	mock.sendEvent(Event{Path: path, Op: OpWrite, Timestamp: now.Add(20 * time.Millisecond)})
	time.Sleep(20 * time.Millisecond)
	mock.sendEvent(Event{Path: path, Op: OpWrite, Timestamp: now.Add(40 * time.Millisecond)})

	// Should receive only ONE coalesced event
	select {
	case received := <-dw.Events():
		// Should have both OpCreate and OpWrite
		if !received.Op.Has(OpCreate) {
			t.Error("coalesced event should have OpCreate")
		}
		if !received.Op.Has(OpWrite) {
			t.Error("coalesced event should have OpWrite")
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timeout waiting for coalesced event")
	}

	// Make sure no more events come through
	select {
	case extra := <-dw.Events():
		t.Errorf("received unexpected extra event: %+v", extra)
	case <-time.After(150 * time.Millisecond):
		// Good, no extra events
	}
}

func TestDebouncedWatcher_DifferentPaths(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 50*time.Millisecond)
	defer dw.Close()

	now := time.Now()

	// Send events for different paths
	mock.sendEvent(Event{Path: "/file1.txt", Op: OpWrite, Timestamp: now})
	mock.sendEvent(Event{Path: "/file2.txt", Op: OpWrite, Timestamp: now})

	// Should receive TWO separate events
	received := make(map[string]bool)
	timeout := time.After(200 * time.Millisecond)

	for i := 0; i < 2; i++ {
		select {
		case event := <-dw.Events():
			received[event.Path] = true
		case <-timeout:
			t.Fatalf("timeout waiting for event %d", i+1)
		}
	}

	if !received["/file1.txt"] {
		t.Error("should have received event for /file1.txt")
	}
	if !received["/file2.txt"] {
		t.Error("should have received event for /file2.txt")
	}
}

func TestDebouncedWatcher_ErrorForwarding(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 50*time.Millisecond)
	defer dw.Close()

	// Send an error
	testErr := ErrWatcherClosed
	mock.sendError(testErr)

	// Should receive error immediately (no debouncing for errors)
	select {
	case err := <-dw.Errors():
		if err != testErr {
			t.Errorf("received error = %v, want %v", err, testErr)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for error")
	}
}

func TestDebouncedWatcher_Flush(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 500*time.Millisecond) // Long delay
	defer dw.Close()

	// Send events
	mock.sendEvent(Event{Path: "/file1.txt", Op: OpWrite, Timestamp: time.Now()})
	mock.sendEvent(Event{Path: "/file2.txt", Op: OpWrite, Timestamp: time.Now()})

	// Wait a bit for events to be processed
	time.Sleep(50 * time.Millisecond)

	// PendingCount should be 2
	if count := dw.PendingCount(); count != 2 {
		t.Errorf("PendingCount = %d, want 2", count)
	}

	// Flush immediately
	dw.Flush()

	// Should receive events immediately
	received := 0
	timeout := time.After(100 * time.Millisecond)

	for {
		select {
		case <-dw.Events():
			received++
			if received == 2 {
				goto done
			}
		case <-timeout:
			t.Errorf("timeout, received only %d events", received)
			goto done
		}
	}
done:

	if received != 2 {
		t.Errorf("received %d events, want 2", received)
	}

	// PendingCount should now be 0
	if count := dw.PendingCount(); count != 0 {
		t.Errorf("PendingCount after Flush = %d, want 0", count)
	}
}

func TestDebouncedWatcher_SetDelay(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 50*time.Millisecond)
	defer dw.Close()

	dw.SetDelay(200 * time.Millisecond)

	if dw.delay != 200*time.Millisecond {
		t.Errorf("delay = %v, want 200ms", dw.delay)
	}
}

func TestDebouncedWatcher_Stats(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 500*time.Millisecond)
	defer dw.Close()

	_ = dw.Watch("/test")

	// Send an event to create pending
	mock.sendEvent(Event{Path: "/test/file.txt", Op: OpWrite, Timestamp: time.Now()})
	time.Sleep(50 * time.Millisecond)

	stats := dw.Stats()
	if stats.WatchedPaths != 1 {
		t.Errorf("WatchedPaths = %d, want 1", stats.WatchedPaths)
	}
	if stats.PendingEvents != 1 {
		t.Errorf("PendingEvents = %d, want 1", stats.PendingEvents)
	}
}

func TestDebouncedWatcher_Close(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 50*time.Millisecond)

	// Send pending events
	mock.sendEvent(Event{Path: "/file.txt", Op: OpWrite, Timestamp: time.Now()})
	time.Sleep(20 * time.Millisecond)

	// Close should work
	if err := dw.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}

	// Close again should be safe
	if err := dw.Close(); err != nil {
		t.Errorf("Close again error = %v", err)
	}
}

func TestDebouncedWatcher_CloseWithPending(t *testing.T) {
	mock := newMockWatcher()
	dw := NewDebouncedWatcher(mock, 1*time.Second) // Long delay

	// Send events
	mock.sendEvent(Event{Path: "/file1.txt", Op: OpWrite, Timestamp: time.Now()})
	mock.sendEvent(Event{Path: "/file2.txt", Op: OpWrite, Timestamp: time.Now()})
	time.Sleep(50 * time.Millisecond)

	// Close should cancel pending events
	if err := dw.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}

	// Events channel should be closed
	_, ok := <-dw.Events()
	if ok {
		t.Error("Events channel should be closed")
	}
}

func TestDebouncedWatcher_ImplementsWatcher(t *testing.T) {
	mock := newMockWatcher()
	var w Watcher = NewDebouncedWatcher(mock, 50*time.Millisecond)
	defer w.Close()

	// Just verify it compiles and implements the interface
	_ = w.Watch("/test")
	_ = w.WatchRecursive("/test2")
	_ = w.Unwatch("/test")
	_ = w.Events()
	_ = w.Errors()
	_ = w.Stats()
	_ = w.IsWatching("/test")
	_ = w.WatchedPaths()
}
