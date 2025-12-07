package dispatch

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncDispatcher_StartStop(t *testing.T) {
	d := NewAsyncDispatcher()

	// Should start successfully
	if err := d.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if !d.IsRunning() {
		t.Error("expected dispatcher to be running after Start()")
	}

	// Should fail to start again
	if err := d.Start(); err != ErrAlreadyRunning {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}

	// Should stop successfully
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.Stop(ctx); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if d.IsRunning() {
		t.Error("expected dispatcher to not be running after Stop()")
	}

	// Should fail to stop again
	if err := d.Stop(ctx); err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestAsyncDispatcher_Enqueue_NotRunning(t *testing.T) {
	d := NewAsyncDispatcher()

	handler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})

	err := d.Enqueue(context.Background(), "event", handler)
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestAsyncDispatcher_Enqueue_Success(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(100),
		WithWorkerCount(2),
	)
	d.Start()
	defer d.Stop(context.Background())

	executed := make(chan struct{})
	handler := newTestHandler(func(ctx context.Context, event any) error {
		close(executed)
		return nil
	})

	err := d.Enqueue(context.Background(), "test-event", handler)
	if err != nil {
		t.Fatalf("Enqueue() failed: %v", err)
	}

	select {
	case <-executed:
		// Success
	case <-time.After(time.Second):
		t.Fatal("handler was not executed within timeout")
	}
}

func TestAsyncDispatcher_QueueFull(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(2),
		WithWorkerCount(1),
	)
	d.Start()

	// Create a slow handler to block the worker
	blocker := make(chan struct{})
	defer close(blocker) // Ensure cleanup
	started := make(chan struct{})

	slowHandler := newTestHandler(func(ctx context.Context, event any) error {
		select {
		case <-started:
			// Already signaled
		default:
			close(started)
		}
		<-blocker
		return nil
	})

	// Enqueue first item and wait for worker to pick it up
	err := d.Enqueue(context.Background(), "event", slowHandler)
	if err != nil {
		t.Fatalf("Enqueue() 0 failed: %v", err)
	}

	// Wait for worker to start processing the first task
	select {
	case <-started:
		// Worker has started processing
	case <-time.After(time.Second):
		t.Fatal("worker did not start processing within timeout")
	}

	// Now fill the queue (queue size is 2)
	for i := 1; i <= 2; i++ {
		err := d.Enqueue(context.Background(), "event", slowHandler)
		if err != nil {
			t.Fatalf("Enqueue() %d failed: %v", i, err)
		}
	}

	// Next enqueue should fail
	err = d.Enqueue(context.Background(), "event", slowHandler)
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	// Verify dropped stat
	stats := d.Stats()
	if stats.Dropped != 1 {
		t.Errorf("expected 1 dropped, got %d", stats.Dropped)
	}

	// Stop with short timeout (blocker will be closed by defer)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d.Stop(ctx)
}

func TestAsyncDispatcher_HandlerExecution(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(100),
		WithWorkerCount(4),
	)
	d.Start()
	defer d.Stop(context.Background())

	const count = 100
	var executed atomic.Int32
	var wg sync.WaitGroup
	wg.Add(count)

	handler := newTestHandler(func(ctx context.Context, event any) error {
		executed.Add(1)
		wg.Done()
		return nil
	})

	for i := 0; i < count; i++ {
		err := d.Enqueue(context.Background(), i, handler)
		if err != nil {
			t.Fatalf("Enqueue() failed: %v", err)
		}
	}

	// Wait for all handlers to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if executed.Load() != count {
			t.Errorf("expected %d executed, got %d", count, executed.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for handlers, executed: %d", executed.Load())
	}
}

func TestAsyncDispatcher_HandlerError(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(10),
		WithWorkerCount(2),
	)
	d.Start()
	defer d.Stop(context.Background())

	expectedErr := errors.New("handler error")
	executed := make(chan struct{})

	handler := newTestHandler(func(ctx context.Context, event any) error {
		defer close(executed)
		return expectedErr
	})

	err := d.Enqueue(context.Background(), "event", handler)
	if err != nil {
		t.Fatalf("Enqueue() failed: %v", err)
	}

	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Fatal("handler was not executed")
	}

	// Give stats time to update
	time.Sleep(10 * time.Millisecond)

	stats := d.Stats()
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
}

func TestAsyncDispatcher_HandlerPanic(t *testing.T) {
	var panicHandlerCalled atomic.Bool
	var capturedPanicValue atomic.Value

	d := NewAsyncDispatcher(
		WithQueueSize(10),
		WithWorkerCount(2),
		WithAsyncPanicHandler(func(event any, panicValue any, stack []byte) {
			panicHandlerCalled.Store(true)
			capturedPanicValue.Store(panicValue)
		}),
	)
	d.Start()
	defer d.Stop(context.Background())

	handler := newTestHandler(func(ctx context.Context, event any) error {
		panic("test panic")
	})

	err := d.Enqueue(context.Background(), "event", handler)
	if err != nil {
		t.Fatalf("Enqueue() failed: %v", err)
	}

	// Wait for handler to execute
	time.Sleep(100 * time.Millisecond)

	if !panicHandlerCalled.Load() {
		t.Error("panic handler was not called")
	}
	if capturedPanicValue.Load() != "test panic" {
		t.Errorf("expected panic value 'test panic', got %v", capturedPanicValue.Load())
	}

	stats := d.Stats()
	if stats.Panicked != 1 {
		t.Errorf("expected 1 panicked, got %d", stats.Panicked)
	}
}

func TestAsyncDispatcher_Timeout(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(10),
		WithWorkerCount(2),
		WithAsyncTimeout(50*time.Millisecond),
	)
	d.Start()
	defer d.Stop(context.Background())

	executed := make(chan struct{})
	handler := newTestHandler(func(ctx context.Context, event any) error {
		select {
		case <-ctx.Done():
			close(executed)
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	err := d.Enqueue(context.Background(), "event", handler)
	if err != nil {
		t.Fatalf("Enqueue() failed: %v", err)
	}

	select {
	case <-executed:
		// Handler was cancelled due to timeout
	case <-time.After(time.Second):
		t.Fatal("handler should have timed out")
	}

	// Give stats time to update
	time.Sleep(10 * time.Millisecond)

	stats := d.Stats()
	if stats.TimedOut != 1 {
		t.Errorf("expected 1 timed out, got %d", stats.TimedOut)
	}
}

func TestAsyncDispatcher_ContextCancellation(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(10),
		WithWorkerCount(2),
	)
	d.Start()
	defer d.Stop(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before enqueue

	executed := make(chan struct{})
	handler := newTestHandler(func(ctx context.Context, event any) error {
		close(executed)
		return nil
	})

	err := d.Enqueue(ctx, "event", handler)
	if err != nil {
		t.Fatalf("Enqueue() failed: %v", err)
	}

	// The handler should still be called but should fail due to cancelled context
	time.Sleep(100 * time.Millisecond)

	stats := d.Stats()
	if stats.Processed != 1 {
		t.Errorf("expected 1 processed, got %d", stats.Processed)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
}

func TestAsyncDispatcher_GracefulShutdown(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(100),
		WithWorkerCount(2),
	)
	d.Start()

	var executed atomic.Int32
	handler := newTestHandler(func(ctx context.Context, event any) error {
		time.Sleep(10 * time.Millisecond)
		executed.Add(1)
		return nil
	})

	// Enqueue several tasks
	const count = 10
	for i := 0; i < count; i++ {
		d.Enqueue(context.Background(), i, handler)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// All tasks should have been processed
	if executed.Load() != count {
		t.Errorf("expected %d executed, got %d", count, executed.Load())
	}
}

func TestAsyncDispatcher_ShutdownTimeout(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(10),
		WithWorkerCount(1),
	)
	d.Start()

	// Create a handler that blocks forever
	blocker := make(chan struct{})
	handler := newTestHandler(func(ctx context.Context, event any) error {
		<-blocker
		return nil
	})

	// Enqueue a task that will block
	d.Enqueue(context.Background(), "event", handler)

	// Short shutdown timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := d.Stop(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	// Clean up
	close(blocker)
}

func TestAsyncDispatcher_Stats(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(100),
		WithWorkerCount(4),
	)
	d.Start()
	defer d.Stop(context.Background())

	var wg sync.WaitGroup

	// Success handler
	wg.Add(1)
	successHandler := newTestHandler(func(ctx context.Context, event any) error {
		wg.Done()
		return nil
	})

	// Error handler
	wg.Add(1)
	errorHandler := newTestHandler(func(ctx context.Context, event any) error {
		wg.Done()
		return errors.New("error")
	})

	// Panic handler
	wg.Add(1)
	panicHandler := newTestHandler(func(ctx context.Context, event any) error {
		defer wg.Done()
		panic("panic")
	})

	d.Enqueue(context.Background(), "success", successHandler)
	d.Enqueue(context.Background(), "error", errorHandler)
	d.Enqueue(context.Background(), "panic", panicHandler)

	// Wait for all handlers
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handlers")
	}

	// Give stats time to update
	time.Sleep(50 * time.Millisecond)

	stats := d.Stats()
	if stats.Enqueued != 3 {
		t.Errorf("expected 3 enqueued, got %d", stats.Enqueued)
	}
	if stats.Processed != 3 {
		t.Errorf("expected 3 processed, got %d", stats.Processed)
	}
	if stats.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", stats.Succeeded)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
	if stats.Panicked != 1 {
		t.Errorf("expected 1 panicked, got %d", stats.Panicked)
	}
	if stats.TotalDuration == 0 {
		t.Error("expected non-zero total duration")
	}
}

func TestAsyncDispatcher_ResetStats(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(10),
		WithWorkerCount(2),
	)
	d.Start()
	defer d.Stop(context.Background())

	executed := make(chan struct{})
	handler := newTestHandler(func(ctx context.Context, event any) error {
		close(executed)
		return nil
	})

	d.Enqueue(context.Background(), "event", handler)
	<-executed

	// Give stats time to update
	time.Sleep(10 * time.Millisecond)

	// Stats should have values
	stats := d.Stats()
	if stats.Enqueued == 0 {
		t.Error("expected non-zero enqueued before reset")
	}

	// Reset stats
	d.ResetStats()

	stats = d.Stats()
	if stats.Enqueued != 0 {
		t.Errorf("expected 0 enqueued after reset, got %d", stats.Enqueued)
	}
	if stats.Processed != 0 {
		t.Errorf("expected 0 processed after reset, got %d", stats.Processed)
	}
	if stats.Succeeded != 0 {
		t.Errorf("expected 0 succeeded after reset, got %d", stats.Succeeded)
	}
}

func TestAsyncDispatcher_QueueDepth(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(100),
		WithWorkerCount(1),
	)
	d.Start()

	// Create a slow handler to let queue fill up
	blocker := make(chan struct{})
	defer close(blocker) // Ensure cleanup

	slowHandler := newTestHandler(func(ctx context.Context, event any) error {
		<-blocker
		return nil
	})

	// Enqueue first task to block the worker
	d.Enqueue(context.Background(), "blocking", slowHandler)

	// Wait for worker to pick up the task
	time.Sleep(10 * time.Millisecond)

	// Enqueue more tasks
	fastHandler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})
	for i := 0; i < 5; i++ {
		d.Enqueue(context.Background(), i, fastHandler)
	}

	depth := d.QueueDepth()
	if depth != 5 {
		t.Errorf("expected queue depth 5, got %d", depth)
	}

	// Stop with timeout (blocker will be closed by defer)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d.Stop(ctx)
}

func TestAsyncDispatcher_EnqueueWithTimeout(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(10),
		WithWorkerCount(2),
		WithAsyncTimeout(5*time.Second), // Default timeout
	)
	d.Start()
	defer d.Stop(context.Background())

	executed := make(chan struct{})
	handler := newTestHandler(func(ctx context.Context, event any) error {
		// Check that context has a deadline
		deadline, ok := ctx.Deadline()
		if !ok {
			close(executed)
			return errors.New("expected context to have deadline")
		}
		// Should have short timeout (50ms) not default (5s)
		if time.Until(deadline) > 100*time.Millisecond {
			close(executed)
			return errors.New("expected short timeout")
		}
		close(executed)
		return nil
	})

	// Use short timeout
	err := d.EnqueueWithTimeout(context.Background(), "event", handler, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("EnqueueWithTimeout() failed: %v", err)
	}

	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Fatal("handler was not executed")
	}
}

func TestAsyncDispatcher_ConcurrentEnqueue(t *testing.T) {
	d := NewAsyncDispatcher(
		WithQueueSize(10000),
		WithWorkerCount(10),
	)
	d.Start()
	defer d.Stop(context.Background())

	const goroutines = 10
	const perGoroutine = 100
	total := goroutines * perGoroutine

	var executed atomic.Int32
	var wg sync.WaitGroup
	wg.Add(total)

	handler := newTestHandler(func(ctx context.Context, event any) error {
		executed.Add(1)
		wg.Done()
		return nil
	})

	// Enqueue from multiple goroutines
	var enqueueWg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		enqueueWg.Add(1)
		go func() {
			defer enqueueWg.Done()
			for j := 0; j < perGoroutine; j++ {
				d.Enqueue(context.Background(), j, handler)
			}
		}()
	}

	// Wait for all enqueues to complete
	enqueueWg.Wait()

	// Wait for all handlers to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if executed.Load() != int32(total) {
			t.Errorf("expected %d executed, got %d", total, executed.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for handlers, executed: %d", executed.Load())
	}
}

func BenchmarkAsyncDispatcher_Enqueue(b *testing.B) {
	d := NewAsyncDispatcher(
		WithQueueSize(100000),
		WithWorkerCount(10),
	)
	d.Start()
	defer d.Stop(context.Background())

	handler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Enqueue(ctx, "event", handler)
	}
}

func BenchmarkAsyncDispatcher_Throughput(b *testing.B) {
	d := NewAsyncDispatcher(
		WithQueueSize(100000),
		WithWorkerCount(10),
	)
	d.Start()
	defer d.Stop(context.Background())

	var wg sync.WaitGroup
	handler := newTestHandler(func(ctx context.Context, event any) error {
		wg.Done()
		return nil
	})
	ctx := context.Background()

	wg.Add(b.N)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		d.Enqueue(ctx, "event", handler)
	}

	wg.Wait()
}
