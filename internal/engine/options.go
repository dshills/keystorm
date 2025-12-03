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
func WithMaxUndoEntries(max int) Option {
	return func(e *Engine) {
		if max > 0 {
			e.maxUndoEntries = max
		}
	}
}

// WithMaxChanges sets the maximum number of tracked changes.
func WithMaxChanges(max int) Option {
	return func(e *Engine) {
		if max > 0 {
			e.maxChanges = max
		}
	}
}

// WithMaxRevisions sets the maximum number of stored revisions.
func WithMaxRevisions(max int) Option {
	return func(e *Engine) {
		if max > 0 {
			e.maxRevisions = max
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
