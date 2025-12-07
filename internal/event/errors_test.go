package event

import (
	"errors"
	"testing"
)

func TestHandlerError(t *testing.T) {
	underlyingErr := errors.New("something went wrong")
	err := &HandlerError{
		SubscriptionID: "sub-123",
		Topic:          "buffer.content.inserted",
		Err:            underlyingErr,
	}

	// Test Error() method
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
	if errStr != "handler error for subscription sub-123 on topic buffer.content.inserted: something went wrong" {
		t.Errorf("unexpected error string: %s", errStr)
	}

	// Test Unwrap()
	if err.Unwrap() != underlyingErr {
		t.Error("Unwrap() should return the underlying error")
	}

	// Test errors.Is with underlying error
	if !errors.Is(err, underlyingErr) {
		t.Error("errors.Is should match the underlying error")
	}
}

func TestPanicError(t *testing.T) {
	err := &PanicError{
		SubscriptionID: "sub-456",
		Topic:          "config.changed",
		Value:          "panic value",
		Stack:          "fake stack trace",
	}

	// Test Error() method
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
	if errStr != "handler panic for subscription sub-456 on topic config.changed" {
		t.Errorf("unexpected error string: %s", errStr)
	}

	// Test errors.Is with ErrHandlerPanic
	if !errors.Is(err, ErrHandlerPanic) {
		t.Error("errors.Is should match ErrHandlerPanic")
	}

	// Test that it doesn't match other errors
	if errors.Is(err, ErrBusNotRunning) {
		t.Error("errors.Is should not match unrelated errors")
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify all sentinel errors are distinct
	sentinelErrors := []error{
		ErrBusNotRunning,
		ErrBusAlreadyRunning,
		ErrQueueFull,
		ErrInvalidEvent,
		ErrInvalidTopic,
		ErrInvalidSubscription,
		ErrSubscriptionNotFound,
		ErrHandlerTimeout,
		ErrHandlerPanic,
		ErrNilHandler,
		ErrShutdownTimeout,
	}

	for i, err1 := range sentinelErrors {
		for j, err2 := range sentinelErrors {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("sentinel errors should be distinct: %v and %v", err1, err2)
			}
		}
	}
}

func TestSentinelErrors_NotNil(t *testing.T) {
	sentinelErrors := map[string]error{
		"ErrBusNotRunning":        ErrBusNotRunning,
		"ErrBusAlreadyRunning":    ErrBusAlreadyRunning,
		"ErrQueueFull":            ErrQueueFull,
		"ErrInvalidEvent":         ErrInvalidEvent,
		"ErrInvalidTopic":         ErrInvalidTopic,
		"ErrInvalidSubscription":  ErrInvalidSubscription,
		"ErrSubscriptionNotFound": ErrSubscriptionNotFound,
		"ErrHandlerTimeout":       ErrHandlerTimeout,
		"ErrHandlerPanic":         ErrHandlerPanic,
		"ErrNilHandler":           ErrNilHandler,
		"ErrShutdownTimeout":      ErrShutdownTimeout,
	}

	for name, err := range sentinelErrors {
		if err == nil {
			t.Errorf("%s should not be nil", name)
		}
		if err.Error() == "" {
			t.Errorf("%s should have a non-empty error message", name)
		}
	}
}
