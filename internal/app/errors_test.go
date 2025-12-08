package app

import (
	"errors"
	"testing"
)

func TestOperationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *OperationError
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "op only",
			err:      &OperationError{Op: "save"},
			expected: "save",
		},
		{
			name:     "op and target",
			err:      &OperationError{Op: "open", Target: "/path/file.txt"},
			expected: "open /path/file.txt",
		},
		{
			name:     "op, target, and context",
			err:      &OperationError{Op: "open", Target: "/path/file.txt", Context: "permission denied"},
			expected: "open /path/file.txt (permission denied)",
		},
		{
			name:     "full error chain",
			err:      &OperationError{Op: "open", Target: "/path/file.txt", Context: "read failed", Err: errors.New("io error")},
			expected: "open /path/file.txt (read failed): io error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = '%s', expected '%s'", result, tt.expected)
			}
		})
	}
}

func TestOperationError_WithContext(t *testing.T) {
	err := NewOperationError("save", "/path/file.txt", nil)
	err = err.WithContext("disk full")

	if err.Context != "disk full" {
		t.Errorf("expected context 'disk full', got '%s'", err.Context)
	}
}

func TestOperationError_WithContext_Nil(t *testing.T) {
	var err *OperationError
	result := err.WithContext("context")
	if result != nil {
		t.Error("expected nil result for nil receiver")
	}
}

func TestOperationError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := NewOperationError("save", "file.txt", inner)

	if err.Unwrap() != inner {
		t.Error("Unwrap() did not return inner error")
	}
}

func TestOperationError_Unwrap_Nil(t *testing.T) {
	var err *OperationError
	if err.Unwrap() != nil {
		t.Error("expected nil from Unwrap() on nil receiver")
	}
}

func TestOperationError_Is(t *testing.T) {
	sentinel := errors.New("sentinel error")
	err := NewOperationError("save", "file.txt", sentinel)

	// Should match wrapped error
	if !errors.Is(err, sentinel) {
		t.Error("expected errors.Is to match wrapped sentinel")
	}

	// Should match same instance
	if !errors.Is(err, err) {
		t.Error("expected errors.Is to match same instance")
	}

	// Should not match different error
	other := errors.New("other error")
	if errors.Is(err, other) {
		t.Error("expected errors.Is to not match different error")
	}
}

func TestOperationError_Is_Nil(t *testing.T) {
	var err *OperationError
	if err.Is(errors.New("any")) {
		t.Error("expected Is() to return false for nil receiver")
	}
}

func TestComponentError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ComponentError
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "component only",
			err:      &ComponentError{Component: "lsp"},
			expected: "lsp",
		},
		{
			name:     "component and action",
			err:      &ComponentError{Component: "lsp", Action: "initialize"},
			expected: "lsp: initialize",
		},
		{
			name:     "component, action, and error",
			err:      &ComponentError{Component: "lsp", Action: "connect", Err: errors.New("timeout")},
			expected: "lsp: connect: timeout",
		},
		{
			name:     "component and error only",
			err:      &ComponentError{Component: "lsp", Err: errors.New("failed")},
			expected: "lsp: failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = '%s', expected '%s'", result, tt.expected)
			}
		})
	}
}

func TestComponentError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := NewComponentError("lsp", "connect", inner)

	if err.Unwrap() != inner {
		t.Error("Unwrap() did not return inner error")
	}
}

func TestComponentError_Unwrap_Nil(t *testing.T) {
	var err *ComponentError
	if err.Unwrap() != nil {
		t.Error("expected nil from Unwrap() on nil receiver")
	}
}

func TestComponentError_Is(t *testing.T) {
	sentinel := errors.New("sentinel error")
	err := NewComponentError("lsp", "connect", sentinel)

	// Should match wrapped error
	if !errors.Is(err, sentinel) {
		t.Error("expected errors.Is to match wrapped sentinel")
	}

	// Should match same instance
	if !errors.Is(err, err) {
		t.Error("expected errors.Is to match same instance")
	}
}

func TestComponentError_Is_Nil(t *testing.T) {
	var err *ComponentError
	if err.Is(errors.New("any")) {
		t.Error("expected Is() to return false for nil receiver")
	}
}

func TestRecoveredPanicError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *RecoveredPanicError
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "value only",
			err:      &RecoveredPanicError{Value: "panic message"},
			expected: "panic: panic message",
		},
		{
			name:     "value with stack",
			err:      &RecoveredPanicError{Value: "panic", Stack: "goroutine 1..."},
			expected: "panic: panic\ngoroutine 1...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = '%s', expected '%s'", result, tt.expected)
			}
		})
	}
}

func TestNewRecoveredPanicError(t *testing.T) {
	err := NewRecoveredPanicError("test panic", "stack trace")
	if err.Value != "test panic" {
		t.Errorf("expected value 'test panic', got '%v'", err.Value)
	}
	if err.Stack != "stack trace" {
		t.Errorf("expected stack 'stack trace', got '%s'", err.Stack)
	}
}

func TestErrorList_Add(t *testing.T) {
	el := NewErrorList()

	el.Add(errors.New("error 1"))
	el.Add(nil) // Should be ignored
	el.Add(errors.New("error 2"))

	if el.Len() != 2 {
		t.Errorf("expected 2 errors, got %d", el.Len())
	}
}

func TestErrorList_HasErrors(t *testing.T) {
	el := NewErrorList()

	if el.HasErrors() {
		t.Error("expected no errors initially")
	}

	el.Add(errors.New("error"))
	if !el.HasErrors() {
		t.Error("expected HasErrors() to be true after Add()")
	}
}

func TestErrorList_Errors(t *testing.T) {
	el := NewErrorList()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	el.Add(err1)
	el.Add(err2)

	errs := el.Errors()
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}

	// Verify returned slice is a copy
	errs[0] = nil
	originalErrs := el.Errors()
	if originalErrs[0] == nil {
		t.Error("expected Errors() to return a copy")
	}
}

func TestErrorList_Errors_Empty(t *testing.T) {
	el := NewErrorList()

	errs := el.Errors()
	if errs != nil {
		t.Error("expected nil slice for empty error list")
	}
}

func TestErrorList_Error(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ErrorList)
		expected string
	}{
		{
			name:     "empty list",
			setup:    func(el *ErrorList) {},
			expected: "",
		},
		{
			name: "single error",
			setup: func(el *ErrorList) {
				el.Add(errors.New("single error"))
			},
			expected: "single error",
		},
		{
			name: "multiple errors",
			setup: func(el *ErrorList) {
				el.Add(errors.New("first error"))
				el.Add(errors.New("second error"))
			},
			expected: "2 errors: first: first error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			el := NewErrorList()
			tt.setup(el)
			result := el.Error()
			if result != tt.expected {
				t.Errorf("Error() = '%s', expected '%s'", result, tt.expected)
			}
		})
	}
}

func TestErrorList_Error_Nil(t *testing.T) {
	var el *ErrorList
	if el.Error() != "" {
		t.Error("expected empty string for nil ErrorList")
	}
}

func TestErrorList_AsError(t *testing.T) {
	el := NewErrorList()

	// Empty list should return nil
	if el.AsError() != nil {
		t.Error("expected nil for empty list")
	}

	el.Add(errors.New("error"))
	if el.AsError() == nil {
		t.Error("expected non-nil for non-empty list")
	}
}

func TestErrorList_First(t *testing.T) {
	el := NewErrorList()

	// Empty list should return nil
	if el.First() != nil {
		t.Error("expected nil for empty list")
	}

	err1 := errors.New("first")
	err2 := errors.New("second")
	el.Add(err1)
	el.Add(err2)

	if el.First() != err1 {
		t.Error("expected First() to return first error")
	}
}

func TestWrapError(t *testing.T) {
	inner := errors.New("inner error")
	wrapped := WrapError(inner, "context: %s", "value")

	if wrapped == nil {
		t.Fatal("expected non-nil wrapped error")
	}
	if !errors.Is(wrapped, inner) {
		t.Error("expected wrapped error to contain inner error")
	}
	expected := "context: value: inner error"
	if wrapped.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, wrapped.Error())
	}
}

func TestWrapError_Nil(t *testing.T) {
	wrapped := WrapError(nil, "context")
	if wrapped != nil {
		t.Error("expected nil for nil input")
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify all sentinel errors are distinct
	sentinels := []error{
		ErrQuit,
		ErrAlreadyRunning,
		ErrNotRunning,
		ErrNoActiveDocument,
		ErrDocumentNotFound,
		ErrDocumentAlreadyOpen,
		ErrUnsavedChanges,
		ErrInitialization,
		ErrShutdownTimeout,
		ErrInvalidOperation,
		ErrComponentNotAvailable,
	}

	for i, err1 := range sentinels {
		for j, err2 := range sentinels {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("sentinel errors %d and %d should be distinct", i, j)
			}
		}
	}
}
