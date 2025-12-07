package event

import "errors"

// Sentinel errors for the event bus.
var (
	// ErrBusNotRunning is returned when operations are attempted on a stopped bus.
	ErrBusNotRunning = errors.New("event bus is not running")

	// ErrBusAlreadyRunning is returned when Start is called on a running bus.
	ErrBusAlreadyRunning = errors.New("event bus is already running")

	// ErrQueueFull is returned when the async queue is full and cannot accept more events.
	ErrQueueFull = errors.New("event queue is full")

	// ErrInvalidEvent is returned when an event is malformed or missing required fields.
	ErrInvalidEvent = errors.New("invalid event")

	// ErrInvalidTopic is returned when a topic is empty or malformed.
	ErrInvalidTopic = errors.New("invalid topic")

	// ErrInvalidSubscription is returned when a subscription is invalid.
	ErrInvalidSubscription = errors.New("invalid subscription")

	// ErrSubscriptionNotFound is returned when trying to unsubscribe a non-existent subscription.
	ErrSubscriptionNotFound = errors.New("subscription not found")

	// ErrHandlerTimeout is returned when a handler exceeds its timeout.
	ErrHandlerTimeout = errors.New("handler timeout exceeded")

	// ErrHandlerPanic is returned when a handler panics.
	ErrHandlerPanic = errors.New("handler panicked")

	// ErrNilHandler is returned when a nil handler is provided.
	ErrNilHandler = errors.New("handler cannot be nil")

	// ErrShutdownTimeout is returned when graceful shutdown times out.
	ErrShutdownTimeout = errors.New("shutdown timeout exceeded")
)

// HandlerError wraps an error from a handler with additional context.
type HandlerError struct {
	// SubscriptionID is the ID of the subscription whose handler failed.
	SubscriptionID string

	// Topic is the topic the handler was subscribed to.
	Topic string

	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *HandlerError) Error() string {
	return "handler error for subscription " + e.SubscriptionID + " on topic " + e.Topic + ": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *HandlerError) Unwrap() error {
	return e.Err
}

// PanicError wraps a panic value as an error.
type PanicError struct {
	// SubscriptionID is the ID of the subscription whose handler panicked.
	SubscriptionID string

	// Topic is the topic the handler was subscribed to.
	Topic string

	// Value is the value passed to panic().
	Value any

	// Stack is the stack trace at the time of the panic.
	Stack string
}

// Error implements the error interface.
func (e *PanicError) Error() string {
	return "handler panic for subscription " + e.SubscriptionID + " on topic " + e.Topic
}

// Is allows errors.Is to match PanicError with ErrHandlerPanic.
func (e *PanicError) Is(target error) bool {
	return target == ErrHandlerPanic
}
