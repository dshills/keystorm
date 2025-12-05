// Package watcher provides file system watching capabilities for the project module.
//
// The watcher detects external file system changes (create, modify, delete, rename)
// and notifies subscribers. It supports event debouncing to coalesce rapid changes
// and ignore pattern matching to exclude unwanted paths.
package watcher

import (
	"context"
	"errors"
	"time"
)

// Common errors returned by watcher operations.
var (
	ErrWatcherClosed   = errors.New("watcher is closed")
	ErrAlreadyWatching = errors.New("path is already being watched")
	ErrNotWatching     = errors.New("path is not being watched")
	ErrPathNotExist    = errors.New("path does not exist")
)

// Op represents the type of file system operation.
type Op uint32

const (
	// OpCreate indicates a file or directory was created.
	OpCreate Op = 1 << iota
	// OpWrite indicates a file was written to.
	OpWrite
	// OpRemove indicates a file or directory was removed.
	OpRemove
	// OpRename indicates a file or directory was renamed.
	OpRename
	// OpChmod indicates file permissions were changed.
	OpChmod
)

// String returns a human-readable representation of the operation.
func (op Op) String() string {
	switch op {
	case OpCreate:
		return "CREATE"
	case OpWrite:
		return "WRITE"
	case OpRemove:
		return "REMOVE"
	case OpRename:
		return "RENAME"
	case OpChmod:
		return "CHMOD"
	default:
		return "UNKNOWN"
	}
}

// Has returns true if the operation includes the given op.
func (op Op) Has(o Op) bool {
	return op&o == o
}

// Event represents a file system change event.
type Event struct {
	// Path is the absolute path of the affected file or directory.
	Path string

	// Op is the operation that occurred.
	Op Op

	// Timestamp is when the event occurred.
	Timestamp time.Time
}

// Stats provides watcher status information.
type Stats struct {
	// WatchedPaths is the number of paths being watched.
	WatchedPaths int

	// PendingEvents is the number of events waiting to be delivered.
	PendingEvents int

	// TotalEvents is the total number of events processed.
	TotalEvents int64

	// Errors is the total number of errors encountered.
	Errors int64

	// LastError is the most recent error, if any.
	LastError error

	// StartTime is when the watcher was started.
	StartTime time.Time
}

// Watcher monitors file system changes.
type Watcher interface {
	// Watch starts watching a path (file or directory).
	// For directories, it watches the directory itself and its immediate children.
	// Returns ErrAlreadyWatching if the path is already being watched.
	Watch(path string) error

	// WatchRecursive starts watching a directory and all subdirectories.
	// Returns ErrPathNotExist if the path doesn't exist or isn't a directory.
	WatchRecursive(path string) error

	// Unwatch stops watching a path.
	// Returns ErrNotWatching if the path isn't being watched.
	Unwatch(path string) error

	// Events returns the channel of file change events.
	// The channel is closed when the watcher is closed.
	Events() <-chan Event

	// Errors returns the channel of watcher errors.
	// The channel is closed when the watcher is closed.
	Errors() <-chan error

	// Close stops the watcher and releases resources.
	// After Close, Events() and Errors() channels will be closed.
	Close() error

	// Stats returns watcher statistics.
	Stats() Stats

	// IsWatching returns true if the path is being watched.
	IsWatching(path string) bool

	// WatchedPaths returns all paths being watched.
	WatchedPaths() []string
}

// Handler is a function that handles file system events.
type Handler func(event Event)

// ErrorHandler is a function that handles watcher errors.
type ErrorHandler func(err error)

// EventFilter is a function that filters events.
// Return true to keep the event, false to discard it.
type EventFilter func(event Event) bool

// Config holds watcher configuration options.
type Config struct {
	// DebounceDelay is the delay before delivering events.
	// Events within this window are coalesced.
	// Default: 100ms
	DebounceDelay time.Duration

	// BufferSize is the size of the event and error channels.
	// Default: 100
	BufferSize int

	// IgnorePatterns are gitignore-style patterns for paths to ignore.
	IgnorePatterns []string

	// IgnoreHidden ignores hidden files (starting with .).
	// Default: false
	IgnoreHidden bool

	// FollowSymlinks follows symbolic links when watching.
	// Default: false
	FollowSymlinks bool

	// MaxWatches is the maximum number of paths to watch.
	// 0 means unlimited.
	// Default: 0
	MaxWatches int

	// EventFilter is an optional filter for events.
	EventFilter EventFilter
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DebounceDelay: 100 * time.Millisecond,
		BufferSize:    100,
		IgnoreHidden:  false,
		MaxWatches:    0,
	}
}

// WatcherOption configures a watcher.
type WatcherOption func(*Config)

// WithDebounceDelay sets the debounce delay.
func WithDebounceDelay(d time.Duration) WatcherOption {
	return func(c *Config) {
		c.DebounceDelay = d
	}
}

// WithBufferSize sets the channel buffer size.
func WithBufferSize(size int) WatcherOption {
	return func(c *Config) {
		c.BufferSize = size
	}
}

// WithIgnorePatterns sets the ignore patterns.
func WithIgnorePatterns(patterns []string) WatcherOption {
	return func(c *Config) {
		c.IgnorePatterns = patterns
	}
}

// WithIgnoreHidden enables ignoring hidden files.
func WithIgnoreHidden(ignore bool) WatcherOption {
	return func(c *Config) {
		c.IgnoreHidden = ignore
	}
}

// WithFollowSymlinks enables following symbolic links.
func WithFollowSymlinks(follow bool) WatcherOption {
	return func(c *Config) {
		c.FollowSymlinks = follow
	}
}

// WithMaxWatches sets the maximum number of watches.
func WithMaxWatches(max int) WatcherOption {
	return func(c *Config) {
		c.MaxWatches = max
	}
}

// WithEventFilter sets the event filter.
func WithEventFilter(filter EventFilter) WatcherOption {
	return func(c *Config) {
		c.EventFilter = filter
	}
}

// EventDispatcher manages event handlers and dispatches events.
type EventDispatcher struct {
	handlers      []Handler
	errorHandlers []ErrorHandler
}

// NewEventDispatcher creates a new event dispatcher.
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{}
}

// OnEvent registers a handler for file events.
func (d *EventDispatcher) OnEvent(handler Handler) {
	d.handlers = append(d.handlers, handler)
}

// OnError registers a handler for errors.
func (d *EventDispatcher) OnError(handler ErrorHandler) {
	d.errorHandlers = append(d.errorHandlers, handler)
}

// Dispatch sends an event to all handlers.
func (d *EventDispatcher) Dispatch(event Event) {
	for _, handler := range d.handlers {
		handler(event)
	}
}

// DispatchError sends an error to all error handlers.
func (d *EventDispatcher) DispatchError(err error) {
	for _, handler := range d.errorHandlers {
		handler(err)
	}
}

// Run starts listening to a watcher and dispatches events until ctx is cancelled.
func (d *EventDispatcher) Run(ctx context.Context, w Watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.Events():
			if !ok {
				return
			}
			d.Dispatch(event)
		case err, ok := <-w.Errors():
			if !ok {
				return
			}
			d.DispatchError(err)
		}
	}
}
