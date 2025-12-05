package watcher

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FSNotifyWatcher implements Watcher using fsnotify.
type FSNotifyWatcher struct {
	mu sync.RWMutex

	// fsnotify watcher
	watcher *fsnotify.Watcher

	// Configuration
	config Config

	// Tracked paths
	paths map[string]bool

	// Output channels
	events chan Event
	errors chan error

	// Stats
	startTime   time.Time
	totalEvents int64
	totalErrors int64
	lastError   error

	// Lifecycle
	closed   bool
	closeCh  chan struct{}
	closedWg sync.WaitGroup

	// Ignore matcher
	ignore *IgnorePatterns
}

// NewFSNotifyWatcher creates a new fsnotify-based watcher.
func NewFSNotifyWatcher(opts ...WatcherOption) (*FSNotifyWatcher, error) {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	bufSize := config.BufferSize
	if bufSize <= 0 {
		bufSize = 100
	}

	w := &FSNotifyWatcher{
		watcher:   fsw,
		config:    config,
		paths:     make(map[string]bool),
		events:    make(chan Event, bufSize),
		errors:    make(chan error, bufSize),
		startTime: time.Now(),
		closeCh:   make(chan struct{}),
		ignore:    NewIgnorePatterns(),
	}

	// Add ignore patterns
	for _, pattern := range config.IgnorePatterns {
		_ = w.ignore.AddPattern(pattern)
	}

	// Start event processing loop
	w.closedWg.Add(1)
	go w.processLoop()

	return w, nil
}

// Watch starts watching a path.
func (w *FSNotifyWatcher) Watch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWatcherClosed
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return ErrPathNotExist
		}
		return err
	}

	// Check if already watching
	if w.paths[absPath] {
		return ErrAlreadyWatching
	}

	// Check max watches
	if w.config.MaxWatches > 0 && len(w.paths) >= w.config.MaxWatches {
		return errors.New("maximum watch limit reached")
	}

	// Add to fsnotify
	if err := w.watcher.Add(absPath); err != nil {
		return err
	}

	w.paths[absPath] = true
	return nil
}

// WatchRecursive watches a directory and all subdirectories.
func (w *FSNotifyWatcher) WatchRecursive(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Check if directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrPathNotExist
		}
		return err
	}
	if !info.IsDir() {
		return w.Watch(absPath)
	}

	// Walk and watch all directories
	return filepath.WalkDir(absPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip ignored paths
		isDir := d.IsDir()
		if w.shouldIgnore(p, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Only watch directories (fsnotify will notify about file changes in watched dirs)
		if isDir {
			if watchErr := w.Watch(p); watchErr != nil {
				// Ignore "already watching" errors during recursive walk
				if watchErr != ErrAlreadyWatching {
					// Log but continue
					w.recordError(watchErr)
				}
			}
		}

		return nil
	})
}

// Unwatch stops watching a path.
func (w *FSNotifyWatcher) Unwatch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWatcherClosed
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if !w.paths[absPath] {
		return ErrNotWatching
	}

	if err := w.watcher.Remove(absPath); err != nil {
		return err
	}

	delete(w.paths, absPath)
	return nil
}

// Events returns the event channel.
func (w *FSNotifyWatcher) Events() <-chan Event {
	return w.events
}

// Errors returns the error channel.
func (w *FSNotifyWatcher) Errors() <-chan error {
	return w.errors
}

// Close stops the watcher.
func (w *FSNotifyWatcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	close(w.closeCh)
	w.mu.Unlock()

	// Wait for processLoop to finish
	w.closedWg.Wait()

	// Close channels
	close(w.events)
	close(w.errors)

	// Close fsnotify watcher
	return w.watcher.Close()
}

// Stats returns watcher statistics.
func (w *FSNotifyWatcher) Stats() Stats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return Stats{
		WatchedPaths:  len(w.paths),
		PendingEvents: len(w.events),
		TotalEvents:   atomic.LoadInt64(&w.totalEvents),
		Errors:        atomic.LoadInt64(&w.totalErrors),
		LastError:     w.lastError,
		StartTime:     w.startTime,
	}
}

// IsWatching returns true if the path is being watched.
func (w *FSNotifyWatcher) IsWatching(path string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return w.paths[absPath]
}

// WatchedPaths returns all watched paths.
func (w *FSNotifyWatcher) WatchedPaths() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	paths := make([]string, 0, len(w.paths))
	for p := range w.paths {
		paths = append(paths, p)
	}
	return paths
}

// processLoop handles incoming fsnotify events.
func (w *FSNotifyWatcher) processLoop() {
	defer w.closedWg.Done()

	for {
		select {
		case <-w.closeCh:
			return

		case fsEvent, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleFSEvent(fsEvent)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.recordError(err)
			w.sendError(err)
		}
	}
}

// handleFSEvent converts and dispatches an fsnotify event.
func (w *FSNotifyWatcher) handleFSEvent(fsEvent fsnotify.Event) {
	// Convert operation
	op := convertOp(fsEvent.Op)
	if op == 0 {
		return // Unknown operation
	}

	// Check ignore patterns
	if w.shouldIgnore(fsEvent.Name, false) {
		return
	}

	// Create event
	event := Event{
		Path:      fsEvent.Name,
		Op:        op,
		Timestamp: time.Now(),
	}

	// Apply event filter
	if w.config.EventFilter != nil {
		if !w.config.EventFilter(event) {
			return
		}
	}

	// Send event
	w.sendEvent(event)

	// Handle directory creation - auto-watch new directories
	if op == OpCreate {
		info, err := os.Stat(fsEvent.Name)
		if err == nil && info.IsDir() {
			// Auto-watch new directories if parent is being watched
			_ = w.Watch(fsEvent.Name)
		}
	}
}

// convertOp converts fsnotify.Op to watcher.Op.
func convertOp(fsOp fsnotify.Op) Op {
	var op Op
	if fsOp.Has(fsnotify.Create) {
		op |= OpCreate
	}
	if fsOp.Has(fsnotify.Write) {
		op |= OpWrite
	}
	if fsOp.Has(fsnotify.Remove) {
		op |= OpRemove
	}
	if fsOp.Has(fsnotify.Rename) {
		op |= OpRename
	}
	if fsOp.Has(fsnotify.Chmod) {
		op |= OpChmod
	}
	return op
}

// shouldIgnore checks if a path should be ignored.
func (w *FSNotifyWatcher) shouldIgnore(path string, isDir bool) bool {
	// Check hidden files
	if w.config.IgnoreHidden {
		base := filepath.Base(path)
		if len(base) > 0 && base[0] == '.' {
			return true
		}
	}

	// Check ignore patterns
	return w.ignore.Match(path, isDir)
}

// sendEvent sends an event to the output channel.
func (w *FSNotifyWatcher) sendEvent(event Event) {
	select {
	case w.events <- event:
		atomic.AddInt64(&w.totalEvents, 1)
	default:
		// Channel full, drop event
		w.recordError(errors.New("event channel full, dropping event"))
	}
}

// sendError sends an error to the output channel.
func (w *FSNotifyWatcher) sendError(err error) {
	select {
	case w.errors <- err:
	default:
		// Channel full, drop error
	}
}

// recordError records an error in stats.
func (w *FSNotifyWatcher) recordError(err error) {
	atomic.AddInt64(&w.totalErrors, 1)
	w.mu.Lock()
	w.lastError = err
	w.mu.Unlock()
}

// Ensure FSNotifyWatcher implements Watcher.
var _ Watcher = (*FSNotifyWatcher)(nil)
