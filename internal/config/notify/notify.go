// Package notify provides change notification for configuration updates.
//
// The notify package implements an observer pattern that allows components
// to subscribe to configuration changes and receive callbacks when settings
// are modified.
package notify

import (
	"sync"
)

// ChangeType represents the type of configuration change.
type ChangeType int

const (
	// ChangeSet indicates a value was set or updated.
	ChangeSet ChangeType = iota

	// ChangeDelete indicates a value was deleted.
	ChangeDelete

	// ChangeReload indicates the entire configuration was reloaded.
	ChangeReload
)

// String returns the change type name.
func (c ChangeType) String() string {
	switch c {
	case ChangeSet:
		return "set"
	case ChangeDelete:
		return "delete"
	case ChangeReload:
		return "reload"
	default:
		return "unknown"
	}
}

// Change represents a configuration change event.
type Change struct {
	// Path is the dot-separated path to the changed setting.
	// Empty for reload events.
	Path string

	// Type is the type of change.
	Type ChangeType

	// OldValue is the previous value (may be nil).
	OldValue any

	// NewValue is the new value (may be nil for deletes).
	NewValue any

	// Source identifies where the change came from.
	Source string
}

// Observer is called when configuration changes occur.
type Observer func(change Change)

// Subscription represents an active observer subscription.
type Subscription struct {
	id       uint64
	path     string
	observer Observer
	notifier *Notifier
}

// Unsubscribe removes this subscription.
func (s *Subscription) Unsubscribe() {
	if s.notifier != nil {
		s.notifier.unsubscribe(s.id)
	}
}

// Notifier manages configuration change subscriptions.
type Notifier struct {
	mu sync.RWMutex

	// Global observers that receive all changes
	globalObservers map[uint64]Observer

	// Path-specific observers
	pathObservers map[string]map[uint64]Observer

	// Next subscription ID
	nextID uint64

	// Whether to notify synchronously or asynchronously
	async bool

	// Buffer for async notifications
	buffer chan Change

	// Done channel for shutdown
	done chan struct{}

	// Wait group for async goroutine
	wg sync.WaitGroup

	// Closed flag for idempotent Close
	closed bool
}

// Option configures a Notifier.
type Option func(*Notifier)

// WithAsync enables asynchronous notification delivery.
func WithAsync(bufferSize int) Option {
	return func(n *Notifier) {
		if bufferSize > 0 {
			n.async = true
			n.buffer = make(chan Change, bufferSize)
		}
	}
}

// New creates a new Notifier.
func New(opts ...Option) *Notifier {
	n := &Notifier{
		globalObservers: make(map[uint64]Observer),
		pathObservers:   make(map[string]map[uint64]Observer),
		done:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(n)
	}

	if n.async {
		n.wg.Add(1)
		go n.processAsync()
	}

	return n
}

// Subscribe registers an observer for all changes.
func (n *Notifier) Subscribe(observer Observer) *Subscription {
	n.mu.Lock()
	defer n.mu.Unlock()

	id := n.nextID
	n.nextID++
	n.globalObservers[id] = observer

	return &Subscription{
		id:       id,
		observer: observer,
		notifier: n,
	}
}

// SubscribePath registers an observer for changes to a specific path.
// The observer is called for exact matches and for parent paths.
// For example, subscribing to "editor" receives changes to "editor.tabSize".
func (n *Notifier) SubscribePath(path string, observer Observer) *Subscription {
	n.mu.Lock()
	defer n.mu.Unlock()

	id := n.nextID
	n.nextID++

	if n.pathObservers[path] == nil {
		n.pathObservers[path] = make(map[uint64]Observer)
	}
	n.pathObservers[path][id] = observer

	return &Subscription{
		id:       id,
		path:     path,
		observer: observer,
		notifier: n,
	}
}

// Notify sends a change notification to all relevant observers.
func (n *Notifier) Notify(change Change) {
	n.mu.RLock()
	if n.closed {
		n.mu.RUnlock()
		return
	}
	n.mu.RUnlock()

	if n.async {
		select {
		case n.buffer <- change:
		case <-n.done:
		}
		return
	}

	n.deliverChange(change)
}

// NotifySet is a convenience method for set changes.
func (n *Notifier) NotifySet(path string, oldValue, newValue any, source string) {
	n.Notify(Change{
		Path:     path,
		Type:     ChangeSet,
		OldValue: oldValue,
		NewValue: newValue,
		Source:   source,
	})
}

// NotifyDelete is a convenience method for delete changes.
func (n *Notifier) NotifyDelete(path string, oldValue any, source string) {
	n.Notify(Change{
		Path:     path,
		Type:     ChangeDelete,
		OldValue: oldValue,
		Source:   source,
	})
}

// NotifyReload is a convenience method for reload events.
func (n *Notifier) NotifyReload(source string) {
	n.Notify(Change{
		Type:   ChangeReload,
		Source: source,
	})
}

// Close shuts down the notifier. It is safe to call Close multiple times.
func (n *Notifier) Close() {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.closed = true
	n.mu.Unlock()

	close(n.done)
	n.wg.Wait()
}

// unsubscribe removes an observer by ID.
func (n *Notifier) unsubscribe(id uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.globalObservers, id)

	for path, observers := range n.pathObservers {
		delete(observers, id)
		if len(observers) == 0 {
			delete(n.pathObservers, path)
		}
	}
}

// deliverChange sends a change to all matching observers.
func (n *Notifier) deliverChange(change Change) {
	n.mu.RLock()

	// Collect matching observers
	var observers []Observer

	// All global observers
	for _, obs := range n.globalObservers {
		observers = append(observers, obs)
	}

	// Path-specific observers
	if change.Path != "" {
		// Exact path match
		if pathObs, ok := n.pathObservers[change.Path]; ok {
			for _, obs := range pathObs {
				observers = append(observers, obs)
			}
		}

		// Parent path matches (e.g., "editor" matches "editor.tabSize")
		for path, pathObs := range n.pathObservers {
			if isParentPath(path, change.Path) {
				for _, obs := range pathObs {
					observers = append(observers, obs)
				}
			}
		}
	} else {
		// Reload event - notify all path observers too
		for _, pathObs := range n.pathObservers {
			for _, obs := range pathObs {
				observers = append(observers, obs)
			}
		}
	}

	n.mu.RUnlock()

	// Call observers outside the lock
	for _, obs := range observers {
		obs(change)
	}
}

// processAsync handles asynchronous notification delivery.
func (n *Notifier) processAsync() {
	defer n.wg.Done()

	for {
		select {
		case change := <-n.buffer:
			n.deliverChange(change)
		case <-n.done:
			// Drain remaining buffered changes
			for {
				select {
				case change := <-n.buffer:
					n.deliverChange(change)
				default:
					return
				}
			}
		}
	}
}

// isParentPath checks if parent is a parent path of child.
// e.g., "editor" is parent of "editor.tabSize".
func isParentPath(parent, child string) bool {
	if len(parent) >= len(child) {
		return false
	}
	if parent == "" {
		return true
	}
	return len(child) > len(parent) && child[:len(parent)] == parent && child[len(parent)] == '.'
}

// Batch collects multiple changes and delivers them as a group.
type Batch struct {
	notifier *Notifier
	changes  []Change
	mu       sync.Mutex
}

// NewBatch creates a new batch for collecting changes.
func (n *Notifier) NewBatch() *Batch {
	return &Batch{
		notifier: n,
		changes:  make([]Change, 0),
	}
}

// Add adds a change to the batch.
func (b *Batch) Add(change Change) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.changes = append(b.changes, change)
}

// Set adds a set change to the batch.
func (b *Batch) Set(path string, oldValue, newValue any, source string) {
	b.Add(Change{
		Path:     path,
		Type:     ChangeSet,
		OldValue: oldValue,
		NewValue: newValue,
		Source:   source,
	})
}

// Commit sends all batched changes to observers.
func (b *Batch) Commit() {
	b.mu.Lock()
	changes := b.changes
	b.changes = make([]Change, 0)
	b.mu.Unlock()

	for _, change := range changes {
		b.notifier.Notify(change)
	}
}

// Discard clears the batch without sending notifications.
func (b *Batch) Discard() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.changes = make([]Change, 0)
}

// Len returns the number of pending changes.
func (b *Batch) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.changes)
}
