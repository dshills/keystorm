package dispatcher

import "errors"

// Dispatcher errors.
var (
	// ErrNoHandler indicates no handler was found for an action.
	ErrNoHandler = errors.New("dispatcher: no handler for action")

	// ErrDispatcherStopped indicates the dispatcher has been stopped.
	ErrDispatcherStopped = errors.New("dispatcher: dispatcher is stopped")

	// ErrActionCancelled indicates the action was cancelled by a hook.
	ErrActionCancelled = errors.New("dispatcher: action cancelled by hook")

	// ErrTimeout indicates the handler execution timed out.
	ErrTimeout = errors.New("dispatcher: handler timeout")

	// ErrPanic indicates the handler panicked.
	ErrPanic = errors.New("dispatcher: handler panic")

	// ErrInvalidAction indicates the action is invalid.
	ErrInvalidAction = errors.New("dispatcher: invalid action")

	// ErrAsyncNotEnabled indicates async dispatch is not enabled.
	ErrAsyncNotEnabled = errors.New("dispatcher: async dispatch not enabled")
)
