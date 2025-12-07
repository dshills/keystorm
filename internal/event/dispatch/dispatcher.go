package dispatch

import (
	"context"
	"time"
)

// Handler is the interface for event handlers.
// This mirrors the event.Handler interface to avoid circular imports.
type Handler interface {
	Handle(ctx context.Context, event any) error
}

// Dispatcher is the interface for event dispatchers.
type Dispatcher interface {
	// Dispatch executes a handler with the given event.
	// Returns a Result containing execution details.
	Dispatch(ctx context.Context, event any, handler Handler) Result
}

// Result represents the outcome of a handler execution.
type Result struct {
	// Success is true if the handler completed without error or panic.
	Success bool

	// Error is the error returned by the handler, if any.
	Error error

	// Panicked is true if the handler panicked.
	Panicked bool

	// PanicValue is the value passed to panic(), if Panicked is true.
	PanicValue any

	// PanicStack is the stack trace at the point of panic.
	PanicStack []byte

	// Duration is how long the handler took to execute.
	Duration time.Duration

	// Skipped is true if the handler was not executed (e.g., context cancelled).
	Skipped bool
}

// IsSuccess returns true if the result indicates successful execution.
func (r Result) IsSuccess() bool {
	return r.Success && !r.Panicked && r.Error == nil
}

// IsError returns true if the result indicates an error (not panic).
func (r Result) IsError() bool {
	return r.Error != nil && !r.Panicked
}

// IsPanic returns true if the result indicates a panic.
func (r Result) IsPanic() bool {
	return r.Panicked
}

// PanicHandler is called when a handler panics during execution.
// It receives the event being processed, the panic value, and the stack trace.
type PanicHandler func(event any, panicValue any, stack []byte)

// ErrorHandler is called when a handler returns an error.
type ErrorHandler func(event any, handler Handler, err error)

// defaultPanicHandler is a no-op panic handler.
func defaultPanicHandler(event any, panicValue any, stack []byte) {
	// Default: silently recover
	// In production, this would typically log to a structured logger
}
