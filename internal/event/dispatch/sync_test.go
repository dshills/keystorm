package dispatch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// testHandler is a simple handler for testing.
type testHandler struct {
	fn func(ctx context.Context, event any) error
}

func (h *testHandler) Handle(ctx context.Context, event any) error {
	return h.fn(ctx, event)
}

func newTestHandler(fn func(ctx context.Context, event any) error) Handler {
	return &testHandler{fn: fn}
}

func TestResult_IsSuccess(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected bool
	}{
		{"success", Result{Success: true}, true},
		{"error", Result{Success: false, Error: errors.New("error")}, false},
		{"panic", Result{Success: false, Panicked: true}, false},
		{"skipped", Result{Success: false, Skipped: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsSuccess(); got != tt.expected {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResult_IsError(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected bool
	}{
		{"success", Result{Success: true}, false},
		{"error", Result{Success: false, Error: errors.New("error")}, true},
		{"panic", Result{Success: false, Panicked: true, PanicValue: "panic"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsError(); got != tt.expected {
				t.Errorf("IsError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResult_IsPanic(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected bool
	}{
		{"success", Result{Success: true}, false},
		{"error", Result{Success: false, Error: errors.New("error")}, false},
		{"panic", Result{Success: false, Panicked: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsPanic(); got != tt.expected {
				t.Errorf("IsPanic() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExecutor_Execute_Success(t *testing.T) {
	executor := NewExecutor()

	var called bool
	var receivedEvent any

	handler := newTestHandler(func(ctx context.Context, event any) error {
		called = true
		receivedEvent = event
		return nil
	})

	result := executor.Execute(context.Background(), "test-event", handler)

	if !result.IsSuccess() {
		t.Errorf("expected success, got %+v", result)
	}
	if !called {
		t.Error("handler was not called")
	}
	if receivedEvent != "test-event" {
		t.Errorf("expected event 'test-event', got %v", receivedEvent)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestExecutor_Execute_Error(t *testing.T) {
	executor := NewExecutor()
	expectedErr := errors.New("handler error")

	handler := newTestHandler(func(ctx context.Context, event any) error {
		return expectedErr
	})

	result := executor.Execute(context.Background(), "test-event", handler)

	if result.IsSuccess() {
		t.Error("expected failure")
	}
	if !result.IsError() {
		t.Error("expected IsError() to be true")
	}
	if result.Error != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, result.Error)
	}
}

func TestExecutor_Execute_Panic(t *testing.T) {
	var panicHandlerCalled bool
	var capturedPanicValue any

	executor := NewExecutor(
		WithExecutorPanicHandler(func(event any, panicValue any, stack []byte) {
			panicHandlerCalled = true
			capturedPanicValue = panicValue
		}),
	)

	handler := newTestHandler(func(ctx context.Context, event any) error {
		panic("test panic")
	})

	result := executor.Execute(context.Background(), "test-event", handler)

	if result.IsSuccess() {
		t.Error("expected failure")
	}
	if !result.IsPanic() {
		t.Error("expected IsPanic() to be true")
	}
	if result.PanicValue != "test panic" {
		t.Errorf("expected panic value 'test panic', got %v", result.PanicValue)
	}
	if len(result.PanicStack) == 0 {
		t.Error("expected non-empty stack trace")
	}
	if !panicHandlerCalled {
		t.Error("panic handler was not called")
	}
	if capturedPanicValue != "test panic" {
		t.Errorf("panic handler received wrong value: %v", capturedPanicValue)
	}
}

func TestExecutor_Execute_ContextCancelled(t *testing.T) {
	executor := NewExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before execution

	handler := newTestHandler(func(ctx context.Context, event any) error {
		t.Error("handler should not be called")
		return nil
	})

	result := executor.Execute(ctx, "test-event", handler)

	if result.IsSuccess() {
		t.Error("expected failure")
	}
	if !result.Skipped {
		t.Error("expected Skipped to be true")
	}
	if !errors.Is(result.Error, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", result.Error)
	}
}

func TestExecutor_ExecuteWithTimeout_Success(t *testing.T) {
	executor := NewExecutor()

	handler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})

	result := executor.ExecuteWithTimeout(context.Background(), "test-event", handler, 1*time.Second)

	if !result.IsSuccess() {
		t.Errorf("expected success, got %+v", result)
	}
}

func TestExecutor_ExecuteWithTimeout_Slow(t *testing.T) {
	executor := NewExecutor()

	handler := newTestHandler(func(ctx context.Context, event any) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	result := executor.ExecuteWithTimeout(context.Background(), "test-event", handler, 50*time.Millisecond)

	if result.IsSuccess() {
		t.Error("expected failure due to timeout")
	}
	if !errors.Is(result.Error, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded error, got %v", result.Error)
	}
}

func TestExecutor_ExecuteAll(t *testing.T) {
	executor := NewExecutor()

	callOrder := []int{}
	handlers := []Handler{
		newTestHandler(func(ctx context.Context, event any) error {
			callOrder = append(callOrder, 1)
			return nil
		}),
		newTestHandler(func(ctx context.Context, event any) error {
			callOrder = append(callOrder, 2)
			return nil
		}),
		newTestHandler(func(ctx context.Context, event any) error {
			callOrder = append(callOrder, 3)
			return nil
		}),
	}

	results := executor.ExecuteAll(context.Background(), "test-event", handlers)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.IsSuccess() {
			t.Errorf("result %d: expected success", i)
		}
	}
	if len(callOrder) != 3 || callOrder[0] != 1 || callOrder[1] != 2 || callOrder[2] != 3 {
		t.Errorf("expected call order [1, 2, 3], got %v", callOrder)
	}
}

func TestExecutor_ExecuteAll_ContextCancelled(t *testing.T) {
	executor := NewExecutor()

	ctx, cancel := context.WithCancel(context.Background())

	handlers := []Handler{
		newTestHandler(func(ctx context.Context, event any) error {
			cancel() // Cancel after first handler
			return nil
		}),
		newTestHandler(func(ctx context.Context, event any) error {
			return nil
		}),
		newTestHandler(func(ctx context.Context, event any) error {
			return nil
		}),
	}

	results := executor.ExecuteAll(ctx, "test-event", handlers)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if !results[0].IsSuccess() {
		t.Error("first handler should succeed")
	}
	// Remaining handlers should be skipped
	for i := 1; i < len(results); i++ {
		if !results[i].Skipped {
			t.Errorf("result %d: expected Skipped", i)
		}
	}
}

func TestSyncDispatcher_Dispatch_Success(t *testing.T) {
	dispatcher := NewSyncDispatcher()

	handler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})

	result := dispatcher.Dispatch(context.Background(), "test-event", handler)

	if !result.IsSuccess() {
		t.Errorf("expected success, got %+v", result)
	}

	stats := dispatcher.Stats()
	if stats.Dispatched != 1 {
		t.Errorf("expected 1 dispatched, got %d", stats.Dispatched)
	}
	if stats.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", stats.Succeeded)
	}
}

func TestSyncDispatcher_Dispatch_Error(t *testing.T) {
	dispatcher := NewSyncDispatcher()
	expectedErr := errors.New("test error")

	handler := newTestHandler(func(ctx context.Context, event any) error {
		return expectedErr
	})

	result := dispatcher.Dispatch(context.Background(), "test-event", handler)

	if result.IsSuccess() {
		t.Error("expected failure")
	}
	if result.Error != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, result.Error)
	}

	stats := dispatcher.Stats()
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
}

func TestSyncDispatcher_Dispatch_Panic(t *testing.T) {
	var panicHandlerCalled bool

	dispatcher := NewSyncDispatcher(
		WithPanicHandler(func(event any, panicValue any, stack []byte) {
			panicHandlerCalled = true
		}),
	)

	handler := newTestHandler(func(ctx context.Context, event any) error {
		panic("boom")
	})

	result := dispatcher.Dispatch(context.Background(), "test-event", handler)

	if result.IsSuccess() {
		t.Error("expected failure")
	}
	if !result.IsPanic() {
		t.Error("expected panic")
	}
	if !panicHandlerCalled {
		t.Error("panic handler was not called")
	}

	stats := dispatcher.Stats()
	if stats.Panicked != 1 {
		t.Errorf("expected 1 panicked, got %d", stats.Panicked)
	}
}

func TestSyncDispatcher_Dispatch_WithTimeout(t *testing.T) {
	dispatcher := NewSyncDispatcher(
		WithTimeout(50 * time.Millisecond),
	)

	handler := newTestHandler(func(ctx context.Context, event any) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	result := dispatcher.Dispatch(context.Background(), "test-event", handler)

	if result.IsSuccess() {
		t.Error("expected timeout failure")
	}
	if !errors.Is(result.Error, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", result.Error)
	}
}

func TestSyncDispatcher_DispatchAll(t *testing.T) {
	dispatcher := NewSyncDispatcher()

	handlers := []Handler{
		newTestHandler(func(ctx context.Context, event any) error { return nil }),
		newTestHandler(func(ctx context.Context, event any) error { return nil }),
		newTestHandler(func(ctx context.Context, event any) error { return nil }),
	}

	results := dispatcher.DispatchAll(context.Background(), "test-event", handlers)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	stats := dispatcher.Stats()
	if stats.Dispatched != 3 {
		t.Errorf("expected 3 dispatched, got %d", stats.Dispatched)
	}
	if stats.Succeeded != 3 {
		t.Errorf("expected 3 succeeded, got %d", stats.Succeeded)
	}
}

func TestSyncDispatcher_DispatchUntilError(t *testing.T) {
	dispatcher := NewSyncDispatcher()
	expectedErr := errors.New("stop here")

	callCount := 0
	handlers := []Handler{
		newTestHandler(func(ctx context.Context, event any) error {
			callCount++
			return nil
		}),
		newTestHandler(func(ctx context.Context, event any) error {
			callCount++
			return expectedErr
		}),
		newTestHandler(func(ctx context.Context, event any) error {
			callCount++
			return nil
		}),
	}

	results := dispatcher.DispatchUntilError(context.Background(), "test-event", handlers)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if callCount != 2 {
		t.Errorf("expected 2 handlers called, got %d", callCount)
	}
	if results[1].Error != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, results[1].Error)
	}
}

func TestSyncDispatcher_Stats(t *testing.T) {
	dispatcher := NewSyncDispatcher()

	// Successful dispatch
	dispatcher.Dispatch(context.Background(), "event",
		newTestHandler(func(ctx context.Context, event any) error { return nil }))

	// Failed dispatch
	dispatcher.Dispatch(context.Background(), "event",
		newTestHandler(func(ctx context.Context, event any) error { return errors.New("error") }))

	// Panic dispatch
	dispatcher.Dispatch(context.Background(), "event",
		newTestHandler(func(ctx context.Context, event any) error { panic("panic") }))

	// Skipped dispatch (cancelled context)
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	dispatcher.Dispatch(cancelledCtx, "event",
		newTestHandler(func(ctx context.Context, event any) error { return nil }))

	stats := dispatcher.Stats()

	if stats.Dispatched != 4 {
		t.Errorf("expected 4 dispatched, got %d", stats.Dispatched)
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
	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", stats.Skipped)
	}
	if stats.TotalDuration == 0 {
		t.Error("expected non-zero total duration")
	}
	if stats.AvgDuration == 0 {
		t.Error("expected non-zero average duration")
	}
}

func TestSyncDispatcher_ResetStats(t *testing.T) {
	dispatcher := NewSyncDispatcher()

	dispatcher.Dispatch(context.Background(), "event",
		newTestHandler(func(ctx context.Context, event any) error { return nil }))

	dispatcher.ResetStats()

	stats := dispatcher.Stats()
	if stats.Dispatched != 0 {
		t.Errorf("expected 0 dispatched after reset, got %d", stats.Dispatched)
	}
	if stats.Succeeded != 0 {
		t.Errorf("expected 0 succeeded after reset, got %d", stats.Succeeded)
	}
}

func TestSyncDispatcher_Concurrent(t *testing.T) {
	dispatcher := NewSyncDispatcher()

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				dispatcher.Dispatch(context.Background(), "event",
					newTestHandler(func(ctx context.Context, event any) error { return nil }))
			}
		}()
	}

	wg.Wait()

	stats := dispatcher.Stats()
	expected := uint64(10 * iterations)
	if stats.Dispatched != expected {
		t.Errorf("expected %d dispatched, got %d", expected, stats.Dispatched)
	}
	if stats.Succeeded != expected {
		t.Errorf("expected %d succeeded, got %d", expected, stats.Succeeded)
	}
}

func BenchmarkExecutor_Execute(b *testing.B) {
	executor := NewExecutor()
	handler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.Execute(ctx, "event", handler)
	}
}

func BenchmarkSyncDispatcher_Dispatch(b *testing.B) {
	dispatcher := NewSyncDispatcher()
	handler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dispatcher.Dispatch(ctx, "event", handler)
	}
}

func BenchmarkSyncDispatcher_Dispatch_WithPanicRecovery(b *testing.B) {
	dispatcher := NewSyncDispatcher(
		WithPanicHandler(func(event any, panicValue any, stack []byte) {}),
	)
	handler := newTestHandler(func(ctx context.Context, event any) error {
		return nil
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dispatcher.Dispatch(ctx, "event", handler)
	}
}
