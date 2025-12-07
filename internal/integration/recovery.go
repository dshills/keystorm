package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RecoveryStrategy defines how to handle operation failures.
type RecoveryStrategy int

const (
	// RecoveryNone means no recovery - fail immediately.
	RecoveryNone RecoveryStrategy = iota

	// RecoveryRetry retries the operation with backoff.
	RecoveryRetry

	// RecoveryFallback uses a fallback value on failure.
	RecoveryFallback

	// RecoveryCircuitBreaker uses circuit breaker pattern.
	RecoveryCircuitBreaker
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts.
	MaxAttempts int

	// InitialDelay is the initial delay between retries.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// BackoffMultiplier multiplies the delay after each attempt.
	BackoffMultiplier float64

	// RetryableErrors defines which errors should trigger a retry.
	// If nil, all errors are retried.
	RetryableErrors func(error) bool
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          5 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// Retry executes fn with retry logic based on the config.
// If MaxAttempts is <= 0, it defaults to 1 attempt.
func Retry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if cfg.RetryableErrors != nil && !cfg.RetryableErrors(err) {
			var zero T
			return zero, fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't wait after last attempt
		if attempt == maxAttempts {
			break
		}

		// Wait with backoff
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-time.After(delay):
		}

		// Increase delay
		delay = time.Duration(float64(delay) * cfg.BackoffMultiplier)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	var zero T
	return zero, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// RetryFunc is a convenience wrapper for Retry with no return value.
func RetryFunc(ctx context.Context, cfg RetryConfig, fn func() error) error {
	_, err := Retry(ctx, cfg, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// CircuitClosed means the circuit is operating normally.
	CircuitClosed CircuitBreakerState = iota

	// CircuitOpen means the circuit is tripped and rejecting calls.
	CircuitOpen

	// CircuitHalfOpen means the circuit is testing if the service recovered.
	CircuitHalfOpen
)

// String returns the string representation of the state.
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures the circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit.
	FailureThreshold int

	// SuccessThreshold is the number of successes in half-open state
	// before closing the circuit.
	SuccessThreshold int

	// Timeout is how long the circuit stays open before transitioning to half-open.
	Timeout time.Duration

	// OnStateChange is called when the circuit state changes.
	OnStateChange func(from, to CircuitBreakerState)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu     sync.RWMutex
	config CircuitBreakerConfig

	state           CircuitBreakerState
	failures        int
	successes       int
	lastFailure     time.Time
	lastStateChange time.Time
}

// ErrCircuitOpen is returned when the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:          cfg,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
	}
}

// Execute runs fn through the circuit breaker.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// ExecuteWithResult runs fn through the circuit breaker and returns its result.
func ExecuteWithResult[T any](cb *CircuitBreaker, fn func() (T, error)) (T, error) {
	if !cb.allowRequest() {
		var zero T
		return zero, ErrCircuitOpen
	}

	result, err := fn()
	cb.recordResult(err)
	return result, err
}

// allowRequest determines if a request should be allowed.
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.transitionTo(CircuitHalfOpen)
			return true
		}
		return false

	case CircuitHalfOpen:
		return true

	default:
		return false
	}
}

// recordResult records the result of an operation.
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failed operation.
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailure = time.Now()
	cb.successes = 0

	switch cb.state {
	case CircuitClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.transitionTo(CircuitOpen)
		}

	case CircuitHalfOpen:
		// Any failure in half-open state opens the circuit again
		cb.transitionTo(CircuitOpen)
	}
}

// onSuccess handles a successful operation.
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case CircuitClosed:
		// Reset failure count on success
		cb.failures = 0

	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	}
}

// transitionTo transitions to a new state (must hold lock).
func (cb *CircuitBreaker) transitionTo(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Reset counters on state change
	cb.failures = 0
	cb.successes = 0

	if cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(oldState, newState)
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state != CircuitClosed {
		cb.transitionTo(CircuitClosed)
	}
}

// Stats returns circuit breaker statistics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:           cb.state,
		Failures:        cb.failures,
		Successes:       cb.successes,
		LastFailure:     cb.lastFailure,
		LastStateChange: cb.lastStateChange,
	}
}

// CircuitBreakerStats contains circuit breaker statistics.
type CircuitBreakerStats struct {
	State           CircuitBreakerState
	Failures        int
	Successes       int
	LastFailure     time.Time
	LastStateChange time.Time
}

// Fallback executes fn and falls back to fallbackValue on error.
func Fallback[T any](fn func() (T, error), fallbackValue T) T {
	result, err := fn()
	if err != nil {
		return fallbackValue
	}
	return result
}

// FallbackFunc executes fn and falls back to fallbackFn on error.
func FallbackFunc[T any](fn func() (T, error), fallbackFn func(error) T) T {
	result, err := fn()
	if err != nil {
		return fallbackFn(err)
	}
	return result
}

// SafeGo runs fn in a goroutine with panic recovery.
func SafeGo(fn func(), onPanic func(recovered any)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if onPanic != nil {
					onPanic(r)
				}
			}
		}()
		fn()
	}()
}

// SafeGoWithContext runs fn in a goroutine with context and panic recovery.
func SafeGoWithContext(ctx context.Context, fn func(context.Context), onPanic func(recovered any)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if onPanic != nil {
					onPanic(r)
				}
			}
		}()
		fn(ctx)
	}()
}

// Timeout executes fn with a timeout.
//
// Important: fn MUST respect the passed context and return when ctx.Done() is closed.
// If fn ignores the context and blocks indefinitely, the goroutine will leak.
// The function returns immediately when the timeout expires, but fn continues
// running in the background until it returns.
func Timeout[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultCh := make(chan T, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := fn(ctx)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- result
		}
	}()

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		var zero T
		return zero, err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// TimeoutFunc is a convenience wrapper for Timeout with no return value.
func TimeoutFunc(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	_, err := Timeout(ctx, timeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}
