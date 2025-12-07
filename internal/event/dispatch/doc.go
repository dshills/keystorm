// Package dispatch provides event dispatching mechanisms for the event bus.
//
// The dispatch package implements both synchronous and asynchronous event
// delivery with panic recovery, context support, and configurable timeouts.
//
// # Dispatchers
//
// Two dispatcher implementations are provided:
//
//   - SyncDispatcher: Executes handlers synchronously in the caller's goroutine.
//     Used for critical paths where latency matters (buffer changes, cursor moves).
//
//   - AsyncDispatcher: Executes handlers asynchronously using a worker pool.
//     Used for non-critical handlers (metrics, plugins, integrations).
//
// # Panic Recovery
//
// All dispatchers recover from panics in handlers, preventing a misbehaving
// handler from crashing the entire editor. Panics are reported via a
// configurable PanicHandler callback.
//
// # Context Support
//
// Dispatchers respect context cancellation and deadlines. If a context is
// cancelled before or during handler execution, the dispatch returns
// context.Canceled or context.DeadlineExceeded.
//
// # Usage
//
// Synchronous dispatch:
//
//	dispatcher := dispatch.NewSyncDispatcher()
//	result := dispatcher.Dispatch(ctx, event, handler)
//	if !result.IsSuccess() {
//	    // Handle error or panic
//	}
//
// With panic handler:
//
//	dispatcher := dispatch.NewSyncDispatcher(
//	    dispatch.WithPanicHandler(func(event any, err any, stack []byte) {
//	        log.Printf("panic in handler: %v\n%s", err, stack)
//	    }),
//	)
//
// # Result Handling
//
// The Result type captures the outcome of handler execution including
// success/failure status, error details, execution duration, and panic
// information if applicable.
package dispatch
