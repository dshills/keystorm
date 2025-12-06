// Package watcher provides file watching for configuration live reload.
//
// The watcher monitors configuration files for changes and triggers
// reload callbacks when modifications are detected.
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event represents a file change event.
type Event struct {
	// Path is the absolute path to the changed file.
	Path string

	// Op is the operation that triggered the event.
	Op Operation

	// Time is when the event occurred.
	Time time.Time
}

// Operation represents the type of file operation.
type Operation int

const (
	// OpWrite indicates the file was modified.
	OpWrite Operation = iota

	// OpCreate indicates a new file was created.
	OpCreate

	// OpRemove indicates the file was deleted.
	OpRemove

	// OpRename indicates the file was renamed.
	OpRename
)

// String returns the operation name.
func (op Operation) String() string {
	switch op {
	case OpWrite:
		return "write"
	case OpCreate:
		return "create"
	case OpRemove:
		return "remove"
	case OpRename:
		return "rename"
	default:
		return "unknown"
	}
}

// Handler is called when a file change is detected.
type Handler func(event Event)

// Watcher monitors files for changes.
type Watcher struct {
	mu sync.RWMutex

	// Watched files and their last modification times
	files map[string]time.Time

	// Handlers to call on file changes
	handlers []Handler

	// Polling interval
	interval time.Duration

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for shutdown
	wg sync.WaitGroup

	// Running state
	running bool

	// Debounce settings
	debounce     time.Duration
	pendingMu    sync.Mutex
	pendingFiles map[string]pendingEvent
}

// pendingEvent stores a pending event with its operation for debouncing.
type pendingEvent struct {
	Op   Operation
	Time time.Time
}

// Option configures a Watcher.
type Option func(*Watcher)

// WithInterval sets the polling interval.
func WithInterval(d time.Duration) Option {
	return func(w *Watcher) {
		if d > 0 {
			w.interval = d
		}
	}
}

// WithDebounce sets the debounce duration for rapid changes.
func WithDebounce(d time.Duration) Option {
	return func(w *Watcher) {
		if d >= 0 {
			w.debounce = d
		}
	}
}

// New creates a new file watcher.
func New(opts ...Option) *Watcher {
	w := &Watcher{
		files:        make(map[string]time.Time),
		handlers:     make([]Handler, 0),
		interval:     500 * time.Millisecond,
		debounce:     100 * time.Millisecond,
		pendingFiles: make(map[string]pendingEvent),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Watch adds a file to the watch list.
func (w *Watcher) Watch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Get initial modification time if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, we'll watch for creation
			w.files[absPath] = time.Time{}
			return nil
		}
		return err
	}

	w.files[absPath] = info.ModTime()
	return nil
}

// Unwatch removes a file from the watch list.
func (w *Watcher) Unwatch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.files, absPath)
	return nil
}

// WatchDir adds all files in a directory matching a pattern.
func (w *Watcher) WatchDir(dir string, pattern string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	matches, err := filepath.Glob(filepath.Join(absDir, pattern))
	if err != nil {
		return err
	}

	for _, path := range matches {
		if err := w.Watch(path); err != nil {
			return err
		}
	}

	return nil
}

// OnChange registers a handler for file change events.
func (w *Watcher) OnChange(handler Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers = append(w.handlers, handler)
}

// Start begins watching files for changes.
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.running = true
	w.mu.Unlock()

	w.wg.Add(1)
	go w.pollLoop()

	if w.debounce > 0 {
		w.wg.Add(1)
		go w.debounceLoop()
	}
}

// Stop stops watching files.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.cancel()
	w.running = false
	w.mu.Unlock()

	w.wg.Wait()
}

// IsRunning returns whether the watcher is active.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// WatchedFiles returns the list of watched files.
func (w *Watcher) WatchedFiles() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	files := make([]string, 0, len(w.files))
	for path := range w.files {
		files = append(files, path)
	}
	return files
}

// pollLoop checks files for changes at regular intervals.
func (w *Watcher) pollLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.checkFiles()
		}
	}
}

// checkFiles checks all watched files for changes.
func (w *Watcher) checkFiles() {
	w.mu.RLock()
	files := make(map[string]time.Time, len(w.files))
	for path, modTime := range w.files {
		files[path] = modTime
	}
	w.mu.RUnlock()

	for path, lastMod := range files {
		event := w.checkFile(path, lastMod)
		if event != nil {
			if w.debounce > 0 {
				w.queueEvent(*event)
			} else {
				w.emitEvent(*event)
			}
		}
	}
}

// checkFile checks a single file for changes.
func (w *Watcher) checkFile(path string, lastMod time.Time) *Event {
	info, err := os.Stat(path)

	// File was deleted
	if os.IsNotExist(err) {
		if !lastMod.IsZero() {
			// File existed before, now it's gone
			w.mu.Lock()
			w.files[path] = time.Time{}
			w.mu.Unlock()

			return &Event{
				Path: path,
				Op:   OpRemove,
				Time: time.Now(),
			}
		}
		return nil
	}

	if err != nil {
		return nil
	}

	currentMod := info.ModTime()

	// File was created
	if lastMod.IsZero() && !currentMod.IsZero() {
		w.mu.Lock()
		w.files[path] = currentMod
		w.mu.Unlock()

		return &Event{
			Path: path,
			Op:   OpCreate,
			Time: time.Now(),
		}
	}

	// File was modified
	if !currentMod.Equal(lastMod) {
		w.mu.Lock()
		w.files[path] = currentMod
		w.mu.Unlock()

		return &Event{
			Path: path,
			Op:   OpWrite,
			Time: time.Now(),
		}
	}

	return nil
}

// queueEvent queues an event for debounced delivery.
// It coalesces events intelligently:
// - create + write => create (first seen operation wins for creation)
// - write + write => write (latest time)
// - any + remove => remove (deletion takes precedence)
func (w *Watcher) queueEvent(event Event) {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	existing, exists := w.pendingFiles[event.Path]
	if !exists {
		w.pendingFiles[event.Path] = pendingEvent{Op: event.Op, Time: event.Time}
		return
	}

	// Coalesce events
	switch event.Op {
	case OpRemove:
		// Remove always takes precedence
		w.pendingFiles[event.Path] = pendingEvent{Op: OpRemove, Time: event.Time}
	case OpCreate:
		// If we already have create, keep it; otherwise use new op
		if existing.Op != OpCreate {
			w.pendingFiles[event.Path] = pendingEvent{Op: OpCreate, Time: event.Time}
		} else {
			// Update time for existing create
			w.pendingFiles[event.Path] = pendingEvent{Op: OpCreate, Time: event.Time}
		}
	case OpWrite:
		// Write doesn't override create or remove
		if existing.Op == OpWrite {
			w.pendingFiles[event.Path] = pendingEvent{Op: OpWrite, Time: event.Time}
		} else {
			// Keep existing op but update time
			w.pendingFiles[event.Path] = pendingEvent{Op: existing.Op, Time: event.Time}
		}
	default:
		// For rename or unknown, just update
		w.pendingFiles[event.Path] = pendingEvent{Op: event.Op, Time: event.Time}
	}
}

// debounceLoop processes debounced events.
func (w *Watcher) debounceLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.debounce)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.processPendingEvents()
		}
	}
}

// processPendingEvents emits events that have been stable.
func (w *Watcher) processPendingEvents() {
	w.pendingMu.Lock()
	now := time.Now()
	stableThreshold := now.Add(-w.debounce)

	var toEmit []Event
	for path, pending := range w.pendingFiles {
		if pending.Time.Before(stableThreshold) {
			toEmit = append(toEmit, Event{
				Path: path,
				Op:   pending.Op,
				Time: pending.Time,
			})
			delete(w.pendingFiles, path)
		}
	}
	w.pendingMu.Unlock()

	for _, event := range toEmit {
		w.emitEvent(event)
	}
}

// emitEvent calls all handlers with the event.
// Handlers are called with panic recovery to prevent a panicking handler
// from crashing the watcher goroutine.
func (w *Watcher) emitEvent(event Event) {
	w.mu.RLock()
	handlers := make([]Handler, len(w.handlers))
	copy(handlers, w.handlers)
	w.mu.RUnlock()

	for _, handler := range handlers {
		w.safeCallHandler(handler, event)
	}
}

// safeCallHandler calls a handler with panic recovery.
func (w *Watcher) safeCallHandler(handler Handler, event Event) {
	defer func() {
		// Recover from panics to keep the watcher running
		_ = recover()
	}()
	handler(event)
}
