package dispatch

import (
	"context"
	"runtime/debug"
	"time"
)

// Executor handles the actual execution of event handlers with
// panic recovery and timing.
type Executor struct {
	panicHandler PanicHandler
}

// NewExecutor creates a new executor with the given options.
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		panicHandler: defaultPanicHandler,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ExecutorOption configures an Executor.
type ExecutorOption func(*Executor)

// WithExecutorPanicHandler sets the panic handler for the executor.
func WithExecutorPanicHandler(h PanicHandler) ExecutorOption {
	return func(e *Executor) {
		e.panicHandler = h
	}
}

// Execute runs a handler with the given event and returns the result.
// It recovers from panics and captures timing information.
func (e *Executor) Execute(ctx context.Context, event any, handler Handler) (result Result) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return Result{
			Success: false,
			Error:   ctx.Err(),
			Skipped: true,
		}
	default:
	}

	start := time.Now()

	// Set up panic recovery
	defer func() {
		result.Duration = time.Since(start)

		if r := recover(); r != nil {
			// Capture full stack trace using debug.Stack()
			stack := debug.Stack()

			result.Success = false
			result.Panicked = true
			result.PanicValue = r
			result.PanicStack = stack

			// Protect the panic handler call - don't let it crash the process
			if e.panicHandler != nil {
				func() {
					defer func() {
						// Silently recover if panic handler itself panics
						_ = recover()
					}()
					e.panicHandler(event, r, stack)
				}()
			}
		}
	}()

	// Execute the handler
	err := handler.Handle(ctx, event)

	if err != nil {
		result.Success = false
		result.Error = err
	} else {
		result.Success = true
	}

	return result
}

// ExecuteWithTimeout runs a handler with a timeout.
// If the handler doesn't complete within the timeout, the context is cancelled.
// Note: The handler must respect context cancellation for this to be effective.
func (e *Executor) ExecuteWithTimeout(ctx context.Context, event any, handler Handler, timeout time.Duration) Result {
	if timeout <= 0 {
		return e.Execute(ctx, event, handler)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return e.Execute(ctx, event, handler)
}

// ExecuteAll runs multiple handlers sequentially and returns all results.
// Execution stops early if the context is cancelled.
func (e *Executor) ExecuteAll(ctx context.Context, event any, handlers []Handler) []Result {
	results := make([]Result, len(handlers))

	for i, handler := range handlers {
		// Check context between handlers
		select {
		case <-ctx.Done():
			// Mark remaining handlers as skipped
			for j := i; j < len(handlers); j++ {
				results[j] = Result{
					Success: false,
					Error:   ctx.Err(),
					Skipped: true,
				}
			}
			return results
		default:
		}

		results[i] = e.Execute(ctx, event, handler)
	}

	return results
}

// ExecuteAllWithTimeout runs multiple handlers with a shared timeout.
func (e *Executor) ExecuteAllWithTimeout(ctx context.Context, event any, handlers []Handler, timeout time.Duration) []Result {
	if timeout <= 0 {
		return e.ExecuteAll(ctx, event, handlers)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return e.ExecuteAll(ctx, event, handlers)
}
