package engine

import (
	"github.com/dshills/keystorm/internal/engine/buffer"
)

// Default configuration values.
const (
	DefaultTabWidth       = 4
	DefaultMaxUndoEntries = 1000
	DefaultMaxChanges     = 10000
	DefaultMaxRevisions   = 100
)

// Option configures an Engine during creation.
type Option func(*Engine)

// WithContent sets the initial content of the engine.
func WithContent(content string) Option {
	return func(e *Engine) {
		e.initContent = content
	}
}

// WithTabWidth sets the tab width for the engine.
func WithTabWidth(width int) Option {
	return func(e *Engine) {
		if width > 0 {
			e.tabWidth = width
		}
	}
}

// WithLineEnding sets the line ending style for the engine.
func WithLineEnding(ending buffer.LineEnding) Option {
	return func(e *Engine) {
		e.lineEnding = ending
	}
}

// WithMaxUndoEntries sets the maximum number of undo history entries.
func WithMaxUndoEntries(maxEntries int) Option {
	return func(e *Engine) {
		if maxEntries > 0 {
			e.maxUndoEntries = maxEntries
		}
	}
}

// WithMaxChanges sets the maximum number of tracked changes.
func WithMaxChanges(maxChanges int) Option {
	return func(e *Engine) {
		if maxChanges > 0 {
			e.maxChanges = maxChanges
		}
	}
}

// WithMaxRevisions sets the maximum number of stored revisions.
func WithMaxRevisions(maxRevisions int) Option {
	return func(e *Engine) {
		if maxRevisions > 0 {
			e.maxRevisions = maxRevisions
		}
	}
}

// WithReadOnly creates a read-only engine.
// Write operations will return ErrReadOnly.
func WithReadOnly() Option {
	return func(e *Engine) {
		e.readOnly = true
	}
}
