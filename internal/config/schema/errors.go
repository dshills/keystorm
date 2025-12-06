package schema

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation failure.
type ValidationError struct {
	// Path is the dot-separated path to the invalid value.
	Path string

	// Message describes what's wrong.
	Message string

	// Value is the invalid value (may be nil).
	Value any

	// Expected describes what was expected.
	Expected string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidationErrors collects multiple validation errors.
type ValidationErrors struct {
	Errors []*ValidationError
}

// Error implements the error interface.
func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("%d validation errors:\n  - %s", len(e.Errors), strings.Join(msgs, "\n  - "))
}

// Add adds a validation error.
func (e *ValidationErrors) Add(path, message string) {
	e.Errors = append(e.Errors, &ValidationError{
		Path:    path,
		Message: message,
	})
}

// AddWithValue adds a validation error with the invalid value.
func (e *ValidationErrors) AddWithValue(path, message string, value any) {
	e.Errors = append(e.Errors, &ValidationError{
		Path:    path,
		Message: message,
		Value:   value,
	})
}

// AddError adds an existing ValidationError.
func (e *ValidationErrors) AddError(err *ValidationError) {
	e.Errors = append(e.Errors, err)
}

// Merge adds all errors from another ValidationErrors.
func (e *ValidationErrors) Merge(other *ValidationErrors) {
	if other == nil {
		return
	}
	e.Errors = append(e.Errors, other.Errors...)
}

// HasErrors returns true if there are any errors.
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// Len returns the number of errors.
func (e *ValidationErrors) Len() int {
	return len(e.Errors)
}

// Clear removes all errors and releases memory.
func (e *ValidationErrors) Clear() {
	e.Errors = nil
}

// AsError returns nil if no errors, otherwise returns self.
func (e *ValidationErrors) AsError() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}

// ErrorsForPath returns all errors for a specific path.
func (e *ValidationErrors) ErrorsForPath(path string) []*ValidationError {
	var result []*ValidationError
	for _, err := range e.Errors {
		if err.Path == path {
			result = append(result, err)
		}
	}
	return result
}

// ErrorsUnderPath returns all errors for a path and its children.
func (e *ValidationErrors) ErrorsUnderPath(path string) []*ValidationError {
	var result []*ValidationError
	prefix := path + "."
	for _, err := range e.Errors {
		if err.Path == path || strings.HasPrefix(err.Path, prefix) {
			result = append(result, err)
		}
	}
	return result
}

// NewValidationError creates a new validation error.
func NewValidationError(path, message string) *ValidationError {
	return &ValidationError{
		Path:    path,
		Message: message,
	}
}

// NewTypeError creates a validation error for type mismatch.
func NewTypeError(path string, expected string, actual any) *ValidationError {
	return &ValidationError{
		Path:     path,
		Message:  fmt.Sprintf("expected %s, got %T", expected, actual),
		Value:    actual,
		Expected: expected,
	}
}

// NewEnumError creates a validation error for invalid enum value.
func NewEnumError(path string, value any, allowed []any) *ValidationError {
	return &ValidationError{
		Path:     path,
		Message:  fmt.Sprintf("value %v is not one of allowed values: %v", value, allowed),
		Value:    value,
		Expected: fmt.Sprintf("one of %v", allowed),
	}
}

// NewRangeError creates a validation error for out-of-range value.
func NewRangeError(path string, value any, min, max *float64) *ValidationError {
	var expected string
	switch {
	case min != nil && max != nil:
		expected = fmt.Sprintf("between %v and %v", *min, *max)
	case min != nil:
		expected = fmt.Sprintf(">= %v", *min)
	case max != nil:
		expected = fmt.Sprintf("<= %v", *max)
	default:
		expected = "valid range"
	}
	return &ValidationError{
		Path:     path,
		Message:  fmt.Sprintf("value %v is out of range", value),
		Value:    value,
		Expected: expected,
	}
}

// NewPatternError creates a validation error for pattern mismatch.
func NewPatternError(path string, value, pattern string) *ValidationError {
	return &ValidationError{
		Path:     path,
		Message:  fmt.Sprintf("value does not match pattern: %s", pattern),
		Value:    value,
		Expected: fmt.Sprintf("pattern: %s", pattern),
	}
}

// NewRequiredError creates a validation error for missing required field.
func NewRequiredError(path string) *ValidationError {
	return &ValidationError{
		Path:    path,
		Message: "required field is missing",
	}
}

// NewUnknownPropertyError creates a validation error for unknown property.
func NewUnknownPropertyError(path string) *ValidationError {
	return &ValidationError{
		Path:    path,
		Message: "unknown property",
	}
}
