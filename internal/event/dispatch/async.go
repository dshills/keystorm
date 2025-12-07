package dispatch

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// AsyncDispatcher executes handlers asynchronously using a worker pool.
// It provides bounded queuing, graceful shutdown, and configurable timeouts.
type AsyncDispatcher struct {
	// Configuration
	queueSize   int
	workerCount int
	timeout     time.Duration

	// State
	mu      sync.Mutex // protects queue creation/destruction
	queue   chan asyncTask
	running atomic.Bool
	wg      sync.WaitGroup

	// Handlers
	panicHandler PanicHandler

	// Stats
	enqueued    atomic.Uint64
	processed   atomic.Uint64
	succeeded   atomic.Uint64
	failed      atomic.Uint64
	panicked    atomic.Uint64
	dropped     atomic.Uint64
	timedOut    atomic.Uint64
	totalTimeNs atomic.Int64
}

// asyncTask represents a task to be executed asynchronously.
type asyncTask struct {
	ctx     context.Context
	event   any
	handler Handler
	timeout time.Duration
}

// NewAsyncDispatcher creates a new asynchronous dispatcher.
func NewAsyncDispatcher(opts ...AsyncOption) *AsyncDispatcher {
	d := &AsyncDispatcher{
		queueSize:    10000,
		workerCount:  10,
		timeout:      5 * time.Second,
		panicHandler: defaultPanicHandler,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// AsyncOption configures an AsyncDispatcher.
type AsyncOption func(*AsyncDispatcher)

// WithQueueSize sets the task queue size.
func WithQueueSize(size int) AsyncOption {
	return func(d *AsyncDispatcher) {
		if size > 0 {
			d.queueSize = size
		}
	}
}

// WithWorkerCount sets the number of worker goroutines.
func WithWorkerCount(count int) AsyncOption {
	return func(d *AsyncDispatcher) {
		if count > 0 {
			d.workerCount = count
		}
	}
}

// WithAsyncTimeout sets the default handler execution timeout.
func WithAsyncTimeout(timeout time.Duration) AsyncOption {
	return func(d *AsyncDispatcher) {
		d.timeout = timeout
	}
}

// WithAsyncPanicHandler sets the panic handler for async execution.
func WithAsyncPanicHandler(h PanicHandler) AsyncOption {
	return func(d *AsyncDispatcher) {
		d.panicHandler = h
	}
}

// Start starts the worker pool.
func (d *AsyncDispatcher) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running.Load() {
		return ErrAlreadyRunning
	}

	d.queue = make(chan asyncTask, d.queueSize)
	d.running.Store(true)

	// Start workers
	for i := 0; i < d.workerCount; i++ {
		d.wg.Add(1)
		go d.worker()
	}

	return nil
}

// Stop stops the worker pool gracefully.
// It waits for all queued tasks to complete or until context is cancelled.
func (d *AsyncDispatcher) Stop(ctx context.Context) error {
	d.mu.Lock()
	if !d.running.Load() {
		d.mu.Unlock()
		return ErrNotRunning
	}

	d.running.Store(false)
	// Close the queue to signal workers to stop
	close(d.queue)
	d.mu.Unlock()

	// Wait for workers to finish with context timeout
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Enqueue adds a task to the queue for asynchronous execution.
// Returns ErrQueueFull if the queue is at capacity.
func (d *AsyncDispatcher) Enqueue(ctx context.Context, event any, handler Handler) error {
	return d.EnqueueWithTimeout(ctx, event, handler, d.timeout)
}

// EnqueueWithTimeout adds a task with a specific timeout.
func (d *AsyncDispatcher) EnqueueWithTimeout(ctx context.Context, event any, handler Handler, timeout time.Duration) error {
	if !d.running.Load() {
		return ErrNotRunning
	}

	task := asyncTask{
		ctx:     ctx,
		event:   event,
		handler: handler,
		timeout: timeout,
	}

	select {
	case d.queue <- task:
		d.enqueued.Add(1)
		return nil
	default:
		d.dropped.Add(1)
		return ErrQueueFull
	}
}

// worker processes tasks from the queue.
func (d *AsyncDispatcher) worker() {
	defer d.wg.Done()

	executor := NewExecutor(WithExecutorPanicHandler(d.panicHandler))

	for task := range d.queue {
		d.executeTask(executor, task)
	}
}

// executeTask executes a single task with timeout and panic recovery.
func (d *AsyncDispatcher) executeTask(executor *Executor, task asyncTask) {
	d.processed.Add(1)
	start := time.Now()

	// This variable tracks if the executor handled the task (panic or not).
	// Used to avoid double-counting panics if an unexpected panic escapes.
	var executorHandled bool

	// Fallback panic recovery for panics that escape the executor (should be rare).
	// Note: panics caught here are NOT double-counted - executorHandled will be false.
	defer func() {
		if r := recover(); r != nil {
			if !executorHandled {
				d.panicked.Add(1)
			}
			if d.panicHandler != nil {
				stack := debug.Stack()
				func() {
					defer func() { _ = recover() }()
					d.panicHandler(task.event, r, stack)
				}()
			}
		}
		d.totalTimeNs.Add(time.Since(start).Nanoseconds())
	}()

	// Check if context is already cancelled
	select {
	case <-task.ctx.Done():
		d.failed.Add(1)
		return
	default:
	}

	// Execute with timeout
	var result Result
	if task.timeout > 0 {
		result = executor.ExecuteWithTimeout(task.ctx, task.event, task.handler, task.timeout)
	} else {
		result = executor.Execute(task.ctx, task.event, task.handler)
	}
	executorHandled = true

	// Update stats based on result
	switch {
	case result.Skipped:
		// Skipped due to context cancellation before execution
		d.failed.Add(1)
	case result.Panicked:
		d.panicked.Add(1)
	case result.Error != nil:
		if result.Error == context.DeadlineExceeded {
			d.timedOut.Add(1)
		}
		d.failed.Add(1)
	case result.Success:
		d.succeeded.Add(1)
	}
}

// QueueDepth returns the current number of tasks in the queue.
// Returns 0 if the dispatcher is not running.
func (d *AsyncDispatcher) QueueDepth() int {
	if !d.running.Load() {
		return 0
	}
	// Queue is guaranteed to exist when running is true
	return len(d.queue)
}

// Stats returns dispatcher statistics.
func (d *AsyncDispatcher) Stats() AsyncDispatcherStats {
	processed := d.processed.Load()
	totalNs := d.totalTimeNs.Load()

	var avgNs int64
	if processed > 0 {
		avgNs = totalNs / int64(processed)
	}

	return AsyncDispatcherStats{
		Enqueued:      d.enqueued.Load(),
		Processed:     processed,
		Succeeded:     d.succeeded.Load(),
		Failed:        d.failed.Load(),
		Panicked:      d.panicked.Load(),
		Dropped:       d.dropped.Load(),
		TimedOut:      d.timedOut.Load(),
		QueueDepth:    d.QueueDepth(),
		TotalDuration: time.Duration(totalNs),
		AvgDuration:   time.Duration(avgNs),
	}
}

// ResetStats resets all statistics to zero.
// For consistent results, call this when the dispatcher is stopped.
func (d *AsyncDispatcher) ResetStats() {
	d.enqueued.Store(0)
	d.processed.Store(0)
	d.succeeded.Store(0)
	d.failed.Store(0)
	d.panicked.Store(0)
	d.dropped.Store(0)
	d.timedOut.Store(0)
	d.totalTimeNs.Store(0)
}

// AsyncDispatcherStats contains statistics for an async dispatcher.
type AsyncDispatcherStats struct {
	// Enqueued is the total number of tasks added to the queue.
	Enqueued uint64

	// Processed is the number of tasks that have been processed.
	Processed uint64

	// Succeeded is the number of successful handler executions.
	Succeeded uint64

	// Failed is the number of handlers that returned errors.
	Failed uint64

	// Panicked is the number of handlers that panicked.
	Panicked uint64

	// Dropped is the number of tasks dropped due to queue being full.
	Dropped uint64

	// TimedOut is the number of handlers that timed out.
	TimedOut uint64

	// QueueDepth is the current number of tasks waiting in the queue.
	QueueDepth int

	// TotalDuration is the cumulative time spent processing tasks.
	TotalDuration time.Duration

	// AvgDuration is the average task processing time.
	AvgDuration time.Duration
}

// IsRunning returns true if the dispatcher is running.
func (d *AsyncDispatcher) IsRunning() bool {
	return d.running.Load()
}
