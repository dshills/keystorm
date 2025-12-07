package integration

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_Success(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	result, err := Retry(ctx, cfg, func() (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Errorf("Retry error: %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
}

func TestRetry_EventualSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts:       5,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	var attempts int

	result, err := Retry(ctx, cfg, func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("not yet")
		}
		return 42, nil
	})

	if err != nil {
		t.Errorf("Retry error: %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetry_AllFail(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	var attempts int

	_, err := Retry(ctx, cfg, func() (int, error) {
		attempts++
		return 0, errors.New("always fail")
	})

	if err == nil {
		t.Error("expected error")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetry_NonRetryable(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		RetryableErrors: func(err error) bool {
			return !errors.Is(err, errNonRetryable)
		},
	}

	var attempts int

	_, err := Retry(ctx, cfg, func() (int, error) {
		attempts++
		return 0, errNonRetryable
	})

	if err == nil {
		t.Error("expected error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (non-retryable)", attempts)
	}
}

var errNonRetryable = errors.New("non-retryable error")

func TestRetry_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := RetryConfig{
		MaxAttempts:       10,
		InitialDelay:      100 * time.Millisecond,
		BackoffMultiplier: 1.0,
	}

	var attempts int

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := Retry(ctx, cfg, func() (int, error) {
		attempts++
		return 0, errors.New("fail")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRetryFunc(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	var called bool
	err := RetryFunc(ctx, cfg, func() error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("RetryFunc error: %v", err)
	}
	if !called {
		t.Error("function was not called")
	}
}

func TestCircuitBreaker_Closed(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Execute error: %v", err)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("State = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_Open(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Trigger failures to open circuit
	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("State = %v, want Open", cb.State())
	}

	// Next call should be rejected
	err := cb.Execute(func() error {
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("State = %v, want Open", cb.State())
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Next call should transition to half-open
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Execute error: %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("State = %v, want HalfOpen", cb.State())
	}

	// One more success should close circuit
	_ = cb.Execute(func() error {
		return nil
	})

	if cb.State() != CircuitClosed {
		t.Errorf("State = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Failure in half-open should reopen
	_ = cb.Execute(func() error {
		return errors.New("fail again")
	})

	if cb.State() != CircuitOpen {
		t.Errorf("State = %v, want Open", cb.State())
	}
}

func TestCircuitBreaker_StateChange(t *testing.T) {
	var transitions []string
	var mu atomic.Value
	mu.Store(transitions)

	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		OnStateChange: func(from, to CircuitBreakerState) {
			trans := mu.Load().([]string)
			trans = append(trans, from.String()+"->"+to.String())
			mu.Store(trans)
		},
	}
	cb := NewCircuitBreaker(cfg)

	// Open circuit
	_ = cb.Execute(func() error { return errors.New("fail") })
	_ = cb.Execute(func() error { return errors.New("fail") })

	// Wait for callback
	time.Sleep(20 * time.Millisecond)

	trans := mu.Load().([]string)
	if len(trans) < 1 {
		t.Error("expected state change callback")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(cfg)

	// Open circuit
	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("State = %v, want Open", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Errorf("State after reset = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(cfg)

	// Execute some operations
	_ = cb.Execute(func() error { return nil })
	_ = cb.Execute(func() error { return errors.New("fail") })

	stats := cb.Stats()

	if stats.State != CircuitClosed {
		t.Errorf("State = %v, want Closed", stats.State)
	}
	if stats.Failures != 1 {
		t.Errorf("Failures = %d, want 1", stats.Failures)
	}
}

func TestExecuteWithResult(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	result, err := ExecuteWithResult(cb, func() (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Errorf("ExecuteWithResult error: %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
}

func TestFallback(t *testing.T) {
	// Success case
	result := Fallback(func() (int, error) {
		return 42, nil
	}, 0)
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}

	// Failure case
	result = Fallback(func() (int, error) {
		return 0, errors.New("fail")
	}, 99)
	if result != 99 {
		t.Errorf("result = %d, want 99 (fallback)", result)
	}
}

func TestFallbackFunc(t *testing.T) {
	result := FallbackFunc(func() (int, error) {
		return 0, errors.New("custom error")
	}, func(err error) int {
		if err.Error() == "custom error" {
			return 100
		}
		return 0
	})

	if result != 100 {
		t.Errorf("result = %d, want 100", result)
	}
}

func TestSafeGo(t *testing.T) {
	var recovered atomic.Value

	done := make(chan bool)

	SafeGo(func() {
		panic("test panic")
	}, func(r any) {
		recovered.Store(r)
		done <- true
	})

	select {
	case <-done:
		// Good
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for panic recovery")
	}

	if recovered.Load() != "test panic" {
		t.Errorf("recovered = %v, want 'test panic'", recovered.Load())
	}
}

func TestSafeGoWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ran atomic.Bool

	done := make(chan bool)

	SafeGoWithContext(ctx, func(ctx context.Context) {
		ran.Store(true)
		done <- true
	}, nil)

	select {
	case <-done:
		// Good
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	if !ran.Load() {
		t.Error("function was not called")
	}
}

func TestTimeout_Success(t *testing.T) {
	ctx := context.Background()

	result, err := Timeout(ctx, time.Second, func(ctx context.Context) (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Errorf("Timeout error: %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
}

func TestTimeout_Exceeded(t *testing.T) {
	ctx := context.Background()

	_, err := Timeout(ctx, 50*time.Millisecond, func(ctx context.Context) (int, error) {
		time.Sleep(200 * time.Millisecond)
		return 42, nil
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestTimeout_Error(t *testing.T) {
	ctx := context.Background()

	_, err := Timeout(ctx, time.Second, func(ctx context.Context) (int, error) {
		return 0, errors.New("operation failed")
	})

	if err == nil {
		t.Error("expected error")
	}
	if err.Error() != "operation failed" {
		t.Errorf("error = %v, want 'operation failed'", err)
	}
}

func TestTimeoutFunc(t *testing.T) {
	ctx := context.Background()

	err := TimeoutFunc(ctx, time.Second, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("TimeoutFunc error: %v", err)
	}
}

func TestCircuitBreakerState_String(t *testing.T) {
	tests := []struct {
		state CircuitBreakerState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitBreakerState(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
