package schema

import (
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	// With path
	err := &ValidationError{Path: "editor.tabSize", Message: "must be between 1 and 16"}
	expected := "editor.tabSize: must be between 1 and 16"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}

	// Without path
	err = &ValidationError{Message: "invalid configuration"}
	if err.Error() != "invalid configuration" {
		t.Errorf("got %q, want 'invalid configuration'", err.Error())
	}
}

func TestValidationErrors_Error(t *testing.T) {
	errs := &ValidationErrors{}

	// No errors
	if errs.Error() != "no validation errors" {
		t.Errorf("got %q for empty errors", errs.Error())
	}

	// Single error
	errs.Add("path", "message")
	if !strings.Contains(errs.Error(), "path: message") {
		t.Errorf("single error should contain the error: %q", errs.Error())
	}

	// Multiple errors
	errs.Add("path2", "message2")
	if !strings.Contains(errs.Error(), "2 validation errors") {
		t.Errorf("multiple errors should show count: %q", errs.Error())
	}
}

func TestValidationErrors_Add(t *testing.T) {
	errs := &ValidationErrors{}
	errs.Add("test.path", "test message")

	if len(errs.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs.Errors))
	}
	if errs.Errors[0].Path != "test.path" {
		t.Errorf("path = %q, want 'test.path'", errs.Errors[0].Path)
	}
	if errs.Errors[0].Message != "test message" {
		t.Errorf("message = %q, want 'test message'", errs.Errors[0].Message)
	}
}

func TestValidationErrors_AddWithValue(t *testing.T) {
	errs := &ValidationErrors{}
	errs.AddWithValue("test.path", "invalid value", 42)

	if len(errs.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs.Errors))
	}
	if errs.Errors[0].Value != 42 {
		t.Errorf("value = %v, want 42", errs.Errors[0].Value)
	}
}

func TestValidationErrors_AddError(t *testing.T) {
	errs := &ValidationErrors{}
	err := &ValidationError{Path: "test", Message: "error", Value: "val", Expected: "exp"}
	errs.AddError(err)

	if len(errs.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs.Errors))
	}
	if errs.Errors[0] != err {
		t.Error("expected same error instance")
	}
}

func TestValidationErrors_Merge(t *testing.T) {
	errs1 := &ValidationErrors{}
	errs1.Add("path1", "message1")

	errs2 := &ValidationErrors{}
	errs2.Add("path2", "message2")
	errs2.Add("path3", "message3")

	errs1.Merge(errs2)

	if len(errs1.Errors) != 3 {
		t.Errorf("expected 3 errors after merge, got %d", len(errs1.Errors))
	}

	// Merge nil
	errs1.Merge(nil)
	if len(errs1.Errors) != 3 {
		t.Error("merge nil should not affect errors")
	}
}

func TestValidationErrors_HasErrors(t *testing.T) {
	errs := &ValidationErrors{}
	if errs.HasErrors() {
		t.Error("expected HasErrors() = false for empty")
	}

	errs.Add("path", "message")
	if !errs.HasErrors() {
		t.Error("expected HasErrors() = true after adding error")
	}
}

func TestValidationErrors_Len(t *testing.T) {
	errs := &ValidationErrors{}
	if errs.Len() != 0 {
		t.Errorf("expected Len() = 0, got %d", errs.Len())
	}

	errs.Add("p1", "m1")
	errs.Add("p2", "m2")
	if errs.Len() != 2 {
		t.Errorf("expected Len() = 2, got %d", errs.Len())
	}
}

func TestValidationErrors_Clear(t *testing.T) {
	errs := &ValidationErrors{}
	errs.Add("p1", "m1")
	errs.Add("p2", "m2")

	errs.Clear()
	if errs.Len() != 0 {
		t.Errorf("expected Len() = 0 after Clear, got %d", errs.Len())
	}
}

func TestValidationErrors_AsError(t *testing.T) {
	errs := &ValidationErrors{}

	// Empty returns nil
	if errs.AsError() != nil {
		t.Error("expected AsError() = nil for empty")
	}

	// Non-empty returns self
	errs.Add("path", "message")
	if errs.AsError() == nil {
		t.Error("expected AsError() != nil after adding error")
	}
}

func TestValidationErrors_ErrorsForPath(t *testing.T) {
	errs := &ValidationErrors{}
	errs.Add("editor.tabSize", "too small")
	errs.Add("editor.tabSize", "not a number")
	errs.Add("editor.wordWrap", "invalid enum")

	pathErrors := errs.ErrorsForPath("editor.tabSize")
	if len(pathErrors) != 2 {
		t.Errorf("expected 2 errors for path, got %d", len(pathErrors))
	}

	pathErrors = errs.ErrorsForPath("editor.wordWrap")
	if len(pathErrors) != 1 {
		t.Errorf("expected 1 error for path, got %d", len(pathErrors))
	}

	pathErrors = errs.ErrorsForPath("nonexistent")
	if len(pathErrors) != 0 {
		t.Errorf("expected 0 errors for nonexistent path, got %d", len(pathErrors))
	}
}

func TestValidationErrors_ErrorsUnderPath(t *testing.T) {
	errs := &ValidationErrors{}
	errs.Add("editor", "invalid section")
	errs.Add("editor.tabSize", "too small")
	errs.Add("editor.wordWrap", "invalid enum")
	errs.Add("ui.theme", "unknown theme")

	underErrors := errs.ErrorsUnderPath("editor")
	if len(underErrors) != 3 {
		t.Errorf("expected 3 errors under 'editor', got %d", len(underErrors))
	}

	underErrors = errs.ErrorsUnderPath("ui")
	if len(underErrors) != 1 {
		t.Errorf("expected 1 error under 'ui', got %d", len(underErrors))
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("test.path", "test message")
	if err.Path != "test.path" {
		t.Errorf("path = %q, want 'test.path'", err.Path)
	}
	if err.Message != "test message" {
		t.Errorf("message = %q, want 'test message'", err.Message)
	}
}

func TestNewTypeError(t *testing.T) {
	err := NewTypeError("test.path", "string", 42)
	if err.Path != "test.path" {
		t.Errorf("path = %q, want 'test.path'", err.Path)
	}
	if !strings.Contains(err.Message, "string") {
		t.Error("message should mention expected type")
	}
	if !strings.Contains(err.Message, "int") {
		t.Error("message should mention actual type")
	}
	if err.Expected != "string" {
		t.Errorf("expected = %q, want 'string'", err.Expected)
	}
}

func TestNewEnumError(t *testing.T) {
	err := NewEnumError("test.path", "invalid", []any{"a", "b", "c"})
	if !strings.Contains(err.Message, "invalid") {
		t.Error("message should contain invalid value")
	}
	if !strings.Contains(err.Expected, "one of") {
		t.Error("expected should describe enum values")
	}
}

func TestNewRangeError(t *testing.T) {
	min := float64(1)
	max := float64(10)

	// Both min and max
	err := NewRangeError("path", 0, &min, &max)
	if !strings.Contains(err.Expected, "between") {
		t.Errorf("expected should mention 'between': %q", err.Expected)
	}

	// Only min
	err = NewRangeError("path", 0, &min, nil)
	if !strings.Contains(err.Expected, ">=") {
		t.Errorf("expected should mention '>=': %q", err.Expected)
	}

	// Only max
	err = NewRangeError("path", 100, nil, &max)
	if !strings.Contains(err.Expected, "<=") {
		t.Errorf("expected should mention '<=': %q", err.Expected)
	}
}

func TestNewPatternError(t *testing.T) {
	err := NewPatternError("test.path", "invalid", `^[a-z]+$`)
	if !strings.Contains(err.Message, "pattern") {
		t.Error("message should mention pattern")
	}
	if err.Value != "invalid" {
		t.Errorf("value = %v, want 'invalid'", err.Value)
	}
}

func TestNewRequiredError(t *testing.T) {
	err := NewRequiredError("test.path")
	if !strings.Contains(err.Message, "required") {
		t.Error("message should mention required")
	}
}

func TestNewUnknownPropertyError(t *testing.T) {
	err := NewUnknownPropertyError("test.unknown")
	if !strings.Contains(err.Message, "unknown") {
		t.Error("message should mention unknown")
	}
}
