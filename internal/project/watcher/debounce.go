package watcher

import (
	"sync"
	"time"
)

// DebouncedWatcher wraps a Watcher with event debouncing.
// Multiple rapid changes to the same file are coalesced into one event.
type DebouncedWatcher struct {
	inner Watcher
	delay time.Duration

	mu       sync.Mutex
	pending  map[string]*pendingEvent
	events   chan Event
	errors   chan error
	closed   bool
	closeCh  chan struct{}
	closedWg sync.WaitGroup
}

// pendingEvent tracks a debounced event.
type pendingEvent struct {
	event Event
	timer *time.Timer
	ops   Op // Combined operations
}

// NewDebouncedWatcher creates a debounced watcher wrapper.
// Events are delayed by the specified duration and coalesced.
// Multiple operations on the same path within the delay window are merged.
func NewDebouncedWatcher(inner Watcher, delay time.Duration) *DebouncedWatcher {
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}

	dw := &DebouncedWatcher{
		inner:   inner,
		delay:   delay,
		pending: make(map[string]*pendingEvent),
		events:  make(chan Event, 100),
		errors:  make(chan error, 100),
		closeCh: make(chan struct{}),
	}

	dw.closedWg.Add(1)
	go dw.processLoop()

	return dw
}

// Watch starts watching a path.
func (dw *DebouncedWatcher) Watch(path string) error {
	return dw.inner.Watch(path)
}

// WatchRecursive starts watching a directory recursively.
func (dw *DebouncedWatcher) WatchRecursive(path string) error {
	return dw.inner.WatchRecursive(path)
}

// Unwatch stops watching a path.
func (dw *DebouncedWatcher) Unwatch(path string) error {
	return dw.inner.Unwatch(path)
}

// Events returns the debounced event channel.
func (dw *DebouncedWatcher) Events() <-chan Event {
	return dw.events
}

// Errors returns the error channel.
func (dw *DebouncedWatcher) Errors() <-chan error {
	return dw.errors
}

// Close stops the debounced watcher.
func (dw *DebouncedWatcher) Close() error {
	dw.mu.Lock()
	if dw.closed {
		dw.mu.Unlock()
		return nil
	}
	dw.closed = true
	close(dw.closeCh)

	// Cancel all pending timers
	for path, p := range dw.pending {
		p.timer.Stop()
		delete(dw.pending, path)
	}
	dw.mu.Unlock()

	// Wait for processLoop to finish
	dw.closedWg.Wait()

	// Close channels
	close(dw.events)
	close(dw.errors)

	// Close inner watcher
	return dw.inner.Close()
}

// Stats returns watcher statistics.
func (dw *DebouncedWatcher) Stats() Stats {
	dw.mu.Lock()
	pendingCount := len(dw.pending)
	dw.mu.Unlock()

	stats := dw.inner.Stats()
	stats.PendingEvents = pendingCount
	return stats
}

// IsWatching returns true if the path is being watched.
func (dw *DebouncedWatcher) IsWatching(path string) bool {
	return dw.inner.IsWatching(path)
}

// WatchedPaths returns all watched paths.
func (dw *DebouncedWatcher) WatchedPaths() []string {
	return dw.inner.WatchedPaths()
}

// processLoop handles incoming events from the inner watcher.
func (dw *DebouncedWatcher) processLoop() {
	defer dw.closedWg.Done()

	for {
		select {
		case <-dw.closeCh:
			return

		case event, ok := <-dw.inner.Events():
			if !ok {
				return
			}
			dw.handleEvent(event)

		case err, ok := <-dw.inner.Errors():
			if !ok {
				return
			}
			dw.forwardError(err)
		}
	}
}

// handleEvent processes an incoming event with debouncing.
func (dw *DebouncedWatcher) handleEvent(event Event) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if dw.closed {
		return
	}

	// Check if we already have a pending event for this path
	if p, exists := dw.pending[event.Path]; exists {
		// Coalesce: combine operations and reset timer
		p.ops |= event.Op
		p.event.Op = p.ops
		p.event.Timestamp = event.Timestamp
		p.timer.Reset(dw.delay)
		return
	}

	// Create new pending event
	p := &pendingEvent{
		event: event,
		ops:   event.Op,
	}

	// Create timer that will fire the event after delay
	p.timer = time.AfterFunc(dw.delay, func() {
		dw.fireEvent(event.Path)
	})

	dw.pending[event.Path] = p
}

// fireEvent sends a pending event and removes it from the map.
func (dw *DebouncedWatcher) fireEvent(path string) {
	dw.mu.Lock()
	p, exists := dw.pending[path]
	if !exists {
		dw.mu.Unlock()
		return
	}
	delete(dw.pending, path)
	event := p.event
	dw.mu.Unlock()

	// Send the event
	select {
	case dw.events <- event:
	case <-dw.closeCh:
		return
	default:
		// Channel full, drop event
	}
}

// forwardError forwards an error from the inner watcher.
func (dw *DebouncedWatcher) forwardError(err error) {
	select {
	case dw.errors <- err:
	case <-dw.closeCh:
	default:
		// Channel full, drop error
	}
}

// Flush immediately fires all pending events.
// Useful for testing or when you need immediate notification.
func (dw *DebouncedWatcher) Flush() {
	dw.mu.Lock()
	paths := make([]string, 0, len(dw.pending))
	for path, p := range dw.pending {
		p.timer.Stop()
		paths = append(paths, path)
	}
	dw.mu.Unlock()

	// Fire all pending events
	for _, path := range paths {
		dw.fireEvent(path)
	}
}

// SetDelay updates the debounce delay.
// Does not affect currently pending events.
func (dw *DebouncedWatcher) SetDelay(delay time.Duration) {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	dw.delay = delay
}

// PendingCount returns the number of pending events.
func (dw *DebouncedWatcher) PendingCount() int {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	return len(dw.pending)
}

// Ensure DebouncedWatcher implements Watcher.
var _ Watcher = (*DebouncedWatcher)(nil)
