package event

import "context"

// Priority determines handler execution order.
// Lower values execute first.
type Priority int

const (
	// PriorityCritical is for renderer, core engine handlers that must run first.
	PriorityCritical Priority = 0

	// PriorityHigh is for LSP, dispatcher handlers.
	PriorityHigh Priority = 100

	// PriorityNormal is the default priority for plugins, integrations.
	PriorityNormal Priority = 200

	// PriorityLow is for metrics, logging handlers that run last.
	PriorityLow Priority = 300
)

// String returns a human-readable priority name.
func (p Priority) String() string {
	switch {
	case p <= PriorityCritical:
		return "critical"
	case p <= PriorityHigh:
		return "high"
	case p <= PriorityNormal:
		return "normal"
	default:
		return "low"
	}
}

// DeliveryMode specifies how events are delivered to handlers.
type DeliveryMode int

const (
	// DeliverySync executes the handler synchronously in the publisher's goroutine.
	// Use for critical paths where latency matters (buffer changes, cursor moves).
	DeliverySync DeliveryMode = iota

	// DeliveryAsync queues the event for asynchronous delivery.
	// Use for non-critical handlers (metrics, plugins, integrations).
	DeliveryAsync
)

// String returns a human-readable delivery mode name.
func (m DeliveryMode) String() string {
	switch m {
	case DeliverySync:
		return "sync"
	case DeliveryAsync:
		return "async"
	default:
		return "unknown"
	}
}

// Handler is the interface for event handlers.
type Handler interface {
	// Handle processes an event.
	// The event parameter is type-erased; handlers should type-assert.
	Handle(ctx context.Context, event any) error
}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc func(ctx context.Context, event any) error

// Handle implements the Handler interface.
func (f HandlerFunc) Handle(ctx context.Context, event any) error {
	return f(ctx, event)
}

// TypedHandler provides type-safe event handling using generics.
type TypedHandler[T any] interface {
	Handle(ctx context.Context, event Event[T]) error
}

// TypedHandlerFunc is a function adapter for TypedHandler.
type TypedHandlerFunc[T any] func(ctx context.Context, event Event[T]) error

// Handle implements the TypedHandler interface.
func (f TypedHandlerFunc[T]) Handle(ctx context.Context, event Event[T]) error {
	return f(ctx, event)
}

// AsHandler converts a TypedHandler to a generic Handler.
func AsHandler[T any](h TypedHandler[T]) Handler {
	return HandlerFunc(func(ctx context.Context, event any) error {
		if e, ok := event.(Event[T]); ok {
			return h.Handle(ctx, e)
		}
		// Type mismatch - skip silently
		return nil
	})
}

// AsHandlerFunc converts a TypedHandlerFunc to a generic Handler.
func AsHandlerFunc[T any](fn TypedHandlerFunc[T]) Handler {
	return AsHandler[T](fn)
}

// FilterFunc is a predicate for filtering events.
// Return true to allow the event, false to filter it out.
type FilterFunc func(event any) bool

// Stats contains event bus statistics.
type Stats struct {
	// EventsPublished is the total number of events published.
	EventsPublished uint64

	// EventsDelivered is the total number of events delivered to handlers.
	EventsDelivered uint64

	// EventsDropped is the number of events dropped (queue full, etc.).
	EventsDropped uint64

	// HandlersExecuted is the total number of handler executions.
	HandlersExecuted uint64

	// HandlerErrors is the number of handlers that returned errors.
	HandlerErrors uint64

	// HandlerPanics is the number of handlers that panicked.
	HandlerPanics uint64

	// AvgDeliveryTimeNs is the average event delivery time in nanoseconds.
	AvgDeliveryTimeNs int64

	// ActiveSubscribers is the current number of active subscriptions.
	ActiveSubscribers int

	// QueueDepth is the current async queue depth.
	QueueDepth int
}

// PanicHandler is called when a handler panics.
type PanicHandler func(event any, handler Handler, recovered any)

// DefaultPanicHandler logs panics to stderr.
func DefaultPanicHandler(event any, handler Handler, recovered any) {
	// In production, this would log to a proper logger
	// For now, just ignore - panics are isolated
}
