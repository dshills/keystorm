// Package app provides the main application structure and coordination.
package app

import (
	"errors"
	"fmt"
)

// Application errors.
var (
	// ErrQuit signals that the application should exit normally.
	ErrQuit = errors.New("quit requested")

	// ErrAlreadyRunning indicates the application is already running.
	ErrAlreadyRunning = errors.New("application already running")

	// ErrNotRunning indicates the application is not running.
	ErrNotRunning = errors.New("application not running")

	// ErrNoActiveDocument indicates no document is currently active.
	ErrNoActiveDocument = errors.New("no active document")

	// ErrDocumentNotFound indicates a document was not found.
	ErrDocumentNotFound = errors.New("document not found")

	// ErrDocumentAlreadyOpen indicates a document is already open.
	ErrDocumentAlreadyOpen = errors.New("document already open")

	// ErrUnsavedChanges indicates there are unsaved changes.
	ErrUnsavedChanges = errors.New("unsaved changes")

	// ErrInitialization indicates an initialization failure.
	ErrInitialization = errors.New("initialization failed")

	// ErrShutdownTimeout indicates shutdown timed out.
	ErrShutdownTimeout = errors.New("shutdown timed out")

	// ErrInvalidOperation indicates an operation that cannot be performed.
	ErrInvalidOperation = errors.New("invalid operation")

	// ErrComponentNotAvailable indicates a required component is not available.
	ErrComponentNotAvailable = errors.New("component not available")
)

// OperationError represents an error that occurred during a specific operation.
type OperationError struct {
	Op      string // Operation name (e.g., "save", "open", "close")
	Target  string // Target of the operation (e.g., file path, document name)
	Context string // Additional context
	Err     error  // Underlying error
}

// NewOperationError creates a new OperationError.
func NewOperationError(op, target string, err error) *OperationError {
	return &OperationError{
		Op:     op,
		Target: target,
		Err:    err,
	}
}

// WithContext adds context to the error.
// Safe to call on nil receiver - returns nil.
func (e *OperationError) WithContext(ctx string) *OperationError {
	if e == nil {
		return nil
	}
	e.Context = ctx
	return e
}

func (e *OperationError) Error() string {
	if e == nil {
		return ""
	}

	var msg string
	if e.Target != "" {
		msg = fmt.Sprintf("%s %s", e.Op, e.Target)
	} else {
		msg = e.Op
	}

	if e.Context != "" {
		msg = fmt.Sprintf("%s (%s)", msg, e.Context)
	}

	if e.Err != nil {
		msg = fmt.Sprintf("%s: %v", msg, e.Err)
	}

	return msg
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is implements errors.Is for OperationError.
// Matches both the wrapper itself and the wrapped error.
func (e *OperationError) Is(target error) bool {
	if e == nil {
		return false
	}
	// Check if target is the same wrapper instance
	if t, ok := target.(*OperationError); ok {
		return e == t
	}
	// Check the wrapped error
	return errors.Is(e.Err, target)
}

// ComponentError represents an error from a specific component.
type ComponentError struct {
	Component string // Component name (e.g., "lsp", "renderer", "dispatcher")
	Action    string // Action being performed
	Err       error  // Underlying error
}

// NewComponentError creates a new ComponentError.
func NewComponentError(component, action string, err error) *ComponentError {
	return &ComponentError{
		Component: component,
		Action:    action,
		Err:       err,
	}
}

func (e *ComponentError) Error() string {
	if e == nil {
		return ""
	}

	if e.Action != "" {
		if e.Err != nil {
			return fmt.Sprintf("%s: %s: %v", e.Component, e.Action, e.Err)
		}
		return fmt.Sprintf("%s: %s", e.Component, e.Action)
	}

	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Component, e.Err)
	}

	return e.Component
}

func (e *ComponentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is implements errors.Is for ComponentError.
// Matches both the wrapper itself and the wrapped error.
func (e *ComponentError) Is(target error) bool {
	if e == nil {
		return false
	}
	// Check if target is the same wrapper instance
	if t, ok := target.(*ComponentError); ok {
		return e == t
	}
	// Check the wrapped error
	return errors.Is(e.Err, target)
}

// RecoveredPanicError wraps a panic value as an error.
// SECURITY NOTE: The Error() method includes the full stack trace and panic value.
// Be cautious about exposing this in production logs or user-facing error messages
// as it may leak sensitive information about internal code structure.
type RecoveredPanicError struct {
	Value any
	Stack string
}

// NewRecoveredPanicError creates a new RecoveredPanicError.
func NewRecoveredPanicError(value any, stack string) *RecoveredPanicError {
	return &RecoveredPanicError{
		Value: value,
		Stack: stack,
	}
}

func (e *RecoveredPanicError) Error() string {
	if e == nil {
		return ""
	}
	if e.Stack != "" {
		return fmt.Sprintf("panic: %v\n%s", e.Value, e.Stack)
	}
	return fmt.Sprintf("panic: %v", e.Value)
}

// ErrorList collects multiple errors.
// NOTE: ErrorList is NOT safe for concurrent use. If concurrent access is needed,
// callers must provide their own synchronization.
type ErrorList struct {
	errors []error
}

// NewErrorList creates a new ErrorList.
func NewErrorList() *ErrorList {
	return &ErrorList{
		errors: make([]error, 0),
	}
}

// Add adds an error to the list. Nil errors are ignored.
func (e *ErrorList) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

// HasErrors returns true if there are any errors.
func (e *ErrorList) HasErrors() bool {
	return len(e.errors) > 0
}

// Len returns the number of errors.
func (e *ErrorList) Len() int {
	return len(e.errors)
}

// Errors returns a copy of the error slice.
// The returned slice is safe to modify without affecting the ErrorList.
func (e *ErrorList) Errors() []error {
	if e == nil || len(e.errors) == 0 {
		return nil
	}
	out := make([]error, len(e.errors))
	copy(out, e.errors)
	return out
}

// Error returns a combined error message.
func (e *ErrorList) Error() string {
	if e == nil || len(e.errors) == 0 {
		return ""
	}

	if len(e.errors) == 1 {
		return e.errors[0].Error()
	}

	return fmt.Sprintf("%d errors: first: %v", len(e.errors), e.errors[0])
}

// AsError returns nil if there are no errors, otherwise returns the ErrorList.
func (e *ErrorList) AsError() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}

// First returns the first error, or nil if empty.
func (e *ErrorList) First() error {
	if len(e.errors) == 0 {
		return nil
	}
	return e.errors[0]
}

// WrapError wraps an error with additional context if it's not nil.
// The format string uses fmt.Sprintf verbs (e.g., %s, %d) - do not use %w
// as wrapping is handled internally.
func WrapError(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", msg, err)
}
