package config

import (
	"errors"
	"fmt"
)

// Errors returned by configuration operations.
var (
	// ErrSettingNotFound indicates the setting path doesn't exist.
	ErrSettingNotFound = errors.New("setting not found")

	// ErrTypeMismatch indicates the value type doesn't match the expected type.
	ErrTypeMismatch = errors.New("type mismatch")

	// ErrValidationFailed indicates the value fails schema validation.
	ErrValidationFailed = errors.New("validation failed")

	// ErrFileNotFound indicates the configuration file doesn't exist.
	ErrFileNotFound = errors.New("config file not found")

	// ErrReadOnly indicates modification was attempted on a read-only layer.
	ErrReadOnly = errors.New("configuration layer is read-only")

	// ErrInvalidPath indicates an invalid setting path format.
	ErrInvalidPath = errors.New("invalid setting path")

	// ErrLayerNotFound indicates the specified layer doesn't exist.
	ErrLayerNotFound = errors.New("layer not found")

	// ErrSettingAlreadyRegistered indicates an attempt to register a duplicate setting.
	ErrSettingAlreadyRegistered = errors.New("setting already registered")

	// ErrIncludeDepthExceeded indicates too many nested @include directives.
	ErrIncludeDepthExceeded = errors.New("include depth exceeded")
)

// ParseError represents an error while parsing a configuration file.
type ParseError struct {
	// Path is the file path that failed to parse.
	Path string
	// Line is the line number where the error occurred (if available).
	Line int
	// Column is the column number where the error occurred (if available).
	Column int
	// Message describes the parse error.
	Message string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("parse error in %s at line %d, column %d: %s", e.Path, e.Line, e.Column, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("parse error in %s at line %d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("parse error in %s: %s", e.Path, e.Message)
}

// Unwrap returns the underlying error.
func (e *ParseError) Unwrap() error {
	return e.Err
}

// ValidationError describes a validation failure for a setting.
type ValidationError struct {
	// Path is the setting path that failed validation.
	Path string
	// Message describes the validation error.
	Message string
	// Value is the invalid value.
	Value any
	// Code categorizes the validation error.
	Code ValidationErrorCode
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (value: %v)", e.Path, e.Message, e.Value)
}

// ValidationErrorCode categorizes validation errors.
type ValidationErrorCode uint8

const (
	// ErrCodeUnknownSetting indicates an unrecognized setting path.
	ErrCodeUnknownSetting ValidationErrorCode = iota
	// ErrCodeTypeMismatch indicates the value type is wrong.
	ErrCodeTypeMismatch
	// ErrCodeOutOfRange indicates a numeric value is out of range.
	ErrCodeOutOfRange
	// ErrCodeInvalidEnum indicates the value is not in the allowed enum.
	ErrCodeInvalidEnum
	// ErrCodePatternMismatch indicates the value doesn't match the required pattern.
	ErrCodePatternMismatch
	// ErrCodeRequiredMissing indicates a required setting is missing.
	ErrCodeRequiredMissing
	// ErrCodeDeprecated indicates the setting is deprecated.
	ErrCodeDeprecated
)

// String returns a human-readable name for the error code.
func (c ValidationErrorCode) String() string {
	switch c {
	case ErrCodeUnknownSetting:
		return "unknown_setting"
	case ErrCodeTypeMismatch:
		return "type_mismatch"
	case ErrCodeOutOfRange:
		return "out_of_range"
	case ErrCodeInvalidEnum:
		return "invalid_enum"
	case ErrCodePatternMismatch:
		return "pattern_mismatch"
	case ErrCodeRequiredMissing:
		return "required_missing"
	case ErrCodeDeprecated:
		return "deprecated"
	default:
		return "unknown"
	}
}

// TypeError is returned when a type conversion fails.
type TypeError struct {
	// Path is the setting path.
	Path string
	// Expected is the expected type name.
	Expected string
	// Actual is the actual type name.
	Actual string
}

// Error implements the error interface.
func (e *TypeError) Error() string {
	return fmt.Sprintf("type error for %s: expected %s, got %s", e.Path, e.Expected, e.Actual)
}

// Is implements error matching for TypeError.
func (e *TypeError) Is(target error) bool {
	return target == ErrTypeMismatch
}
