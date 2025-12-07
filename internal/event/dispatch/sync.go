package dispatch

import (
	"context"
	"sync/atomic"
	"time"
)

// SyncDispatcher executes handlers synchronously in the caller's goroutine.
// It provides panic recovery and context support.
type SyncDispatcher struct {
	executor *Executor
	timeout  time.Duration

	// Stats
	dispatched  atomic.Uint64
	succeeded   atomic.Uint64
	failed      atomic.Uint64
	panicked    atomic.Uint64
	skipped     atomic.Uint64
	totalTimeNs atomic.Int64
}

// NewSyncDispatcher creates a new synchronous dispatcher.
func NewSyncDispatcher(opts ...SyncOption) *SyncDispatcher {
	d := &SyncDispatcher{
		executor: NewExecutor(),
		timeout:  0, // No timeout by default
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// SyncOption configures a SyncDispatcher.
type SyncOption func(*SyncDispatcher)

// WithPanicHandler sets the panic handler for the dispatcher.
func WithPanicHandler(h PanicHandler) SyncOption {
	return func(d *SyncDispatcher) {
		d.executor = NewExecutor(WithExecutorPanicHandler(h))
	}
}

// WithTimeout sets a default timeout for handler execution.
func WithTimeout(timeout time.Duration) SyncOption {
	return func(d *SyncDispatcher) {
		d.timeout = timeout
	}
}

// Dispatch executes a handler synchronously with the given event.
// It blocks until the handler completes, times out, or panics.
func (d *SyncDispatcher) Dispatch(ctx context.Context, event any, handler Handler) Result {
	d.dispatched.Add(1)

	var result Result
	if d.timeout > 0 {
		result = d.executor.ExecuteWithTimeout(ctx, event, handler, d.timeout)
	} else {
		result = d.executor.Execute(ctx, event, handler)
	}

	// Update stats
	d.totalTimeNs.Add(result.Duration.Nanoseconds())

	switch {
	case result.Skipped:
		d.skipped.Add(1)
	case result.Panicked:
		d.panicked.Add(1)
	case result.Error != nil:
		d.failed.Add(1)
	case result.Success:
		d.succeeded.Add(1)
	}

	return result
}

// DispatchAll executes multiple handlers sequentially.
// Returns results for all handlers in order.
func (d *SyncDispatcher) DispatchAll(ctx context.Context, event any, handlers []Handler) []Result {
	results := make([]Result, len(handlers))

	for i, handler := range handlers {
		results[i] = d.Dispatch(ctx, event, handler)

		// Stop if context is cancelled
		select {
		case <-ctx.Done():
			for j := i + 1; j < len(handlers); j++ {
				results[j] = Result{
					Success: false,
					Error:   ctx.Err(),
					Skipped: true,
				}
			}
			return results
		default:
		}
	}

	return results
}

// DispatchUntilError executes handlers until one returns an error or panics.
// Returns the results up to and including the failure, or all results if successful.
func (d *SyncDispatcher) DispatchUntilError(ctx context.Context, event any, handlers []Handler) []Result {
	var results []Result

	for _, handler := range handlers {
		result := d.Dispatch(ctx, event, handler)
		results = append(results, result)

		if !result.IsSuccess() {
			break
		}
	}

	return results
}

// Stats returns dispatch statistics.
// Note: Stats are read without a mutex, so values may be slightly inconsistent
// if stats are being updated concurrently.
func (d *SyncDispatcher) Stats() SyncDispatcherStats {
	dispatched := d.dispatched.Load()
	totalNs := d.totalTimeNs.Load()

	var avgNs int64
	if dispatched > 0 {
		avgNs = totalNs / int64(dispatched)
	}

	return SyncDispatcherStats{
		Dispatched:    dispatched,
		Succeeded:     d.succeeded.Load(),
		Failed:        d.failed.Load(),
		Panicked:      d.panicked.Load(),
		Skipped:       d.skipped.Load(),
		TotalDuration: time.Duration(totalNs),
		AvgDuration:   time.Duration(avgNs),
	}
}

// ResetStats resets all statistics to zero.
func (d *SyncDispatcher) ResetStats() {
	d.dispatched.Store(0)
	d.succeeded.Store(0)
	d.failed.Store(0)
	d.panicked.Store(0)
	d.skipped.Store(0)
	d.totalTimeNs.Store(0)
}

// SyncDispatcherStats contains statistics for a sync dispatcher.
type SyncDispatcherStats struct {
	// Dispatched is the total number of dispatch calls.
	Dispatched uint64

	// Succeeded is the number of successful handler executions.
	Succeeded uint64

	// Failed is the number of handlers that returned errors.
	Failed uint64

	// Panicked is the number of handlers that panicked.
	Panicked uint64

	// Skipped is the number of handlers skipped (e.g., context cancelled).
	Skipped uint64

	// TotalDuration is the cumulative time spent in handlers.
	TotalDuration time.Duration

	// AvgDuration is the average handler execution time.
	AvgDuration time.Duration
}
