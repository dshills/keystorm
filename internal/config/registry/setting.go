// Package registry provides the settings registry for Keystorm configuration.
//
// The registry maintains definitions of all known settings with their types,
// defaults, validation rules, and metadata. It provides type-safe access
// to settings values.
package registry

import (
	"fmt"
	"regexp"
	"time"
)

// Setting defines a configuration setting with its metadata.
type Setting struct {
	// Path is the dot-separated path (e.g., "editor.tabSize").
	Path string

	// Type is the setting's data type.
	Type SettingType

	// Default is the default value.
	Default any

	// Description is human-readable documentation.
	Description string

	// Scope defines where this setting can be set.
	Scope SettingScope

	// Enum lists allowed values for enum types.
	Enum []any

	// Minimum for numeric types (nil means no minimum).
	Minimum *float64

	// Maximum for numeric types (nil means no maximum).
	Maximum *float64

	// Pattern for string validation (regex).
	Pattern string

	// Deprecated marks settings that should be migrated.
	Deprecated        bool
	DeprecatedMessage string
	ReplacedBy        string

	// Tags for filtering/grouping settings.
	Tags []string

	// compiledPattern is the compiled regex pattern (lazily initialized).
	compiledPattern *regexp.Regexp
}

// Validate checks if a value is valid for this setting.
func (s *Setting) Validate(value any) error {
	// Type check
	if err := s.validateType(value); err != nil {
		return err
	}

	// Enum check
	if len(s.Enum) > 0 {
		if !containsValue(s.Enum, value) {
			return fmt.Errorf("value must be one of: %v", s.Enum)
		}
	}

	// Range check for numeric types
	if s.Type == TypeInt || s.Type == TypeFloat {
		if err := s.validateRange(value); err != nil {
			return err
		}
	}

	// Pattern check for strings
	if s.Type == TypeString && s.Pattern != "" {
		if err := s.validatePattern(value); err != nil {
			return err
		}
	}

	return nil
}

// validateType checks if the value matches the expected type.
func (s *Setting) validateType(value any) error {
	switch s.Type {
	case TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case TypeInt:
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// Valid
		default:
			return fmt.Errorf("expected integer, got %T", value)
		}
	case TypeFloat:
		switch value.(type) {
		case float32, float64, int, int64:
			// Valid (integers are acceptable for float)
		default:
			return fmt.Errorf("expected number, got %T", value)
		}
	case TypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case TypeArray:
		switch value.(type) {
		case []any, []string, []int, []int64, []float64:
			// Valid
		default:
			return fmt.Errorf("expected array, got %T", value)
		}
	case TypeObject:
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	case TypeDuration:
		switch value.(type) {
		case time.Duration, string:
			// Valid (string will be parsed)
		default:
			return fmt.Errorf("expected duration, got %T", value)
		}
	case TypeEnum:
		// Enum validation handled separately
	}
	return nil
}

// validateRange checks if a numeric value is within the allowed range.
func (s *Setting) validateRange(value any) error {
	var f float64
	switch v := value.(type) {
	case int:
		f = float64(v)
	case int64:
		f = float64(v)
	case float32:
		f = float64(v)
	case float64:
		f = v
	default:
		return nil // Non-numeric, skip range check
	}

	if s.Minimum != nil && f < *s.Minimum {
		return fmt.Errorf("value %v is less than minimum %v", value, *s.Minimum)
	}
	if s.Maximum != nil && f > *s.Maximum {
		return fmt.Errorf("value %v is greater than maximum %v", value, *s.Maximum)
	}
	return nil
}

// validatePattern checks if a string value matches the required pattern.
func (s *Setting) validatePattern(value any) error {
	str, ok := value.(string)
	if !ok {
		return nil // Non-string, skip pattern check
	}

	if s.compiledPattern == nil {
		var err error
		s.compiledPattern, err = regexp.Compile(s.Pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
	}

	if !s.compiledPattern.MatchString(str) {
		return fmt.Errorf("value does not match pattern %s", s.Pattern)
	}
	return nil
}

// SettingType represents the data type of a setting.
type SettingType uint8

const (
	// TypeString represents a string value.
	TypeString SettingType = iota
	// TypeInt represents an integer value.
	TypeInt
	// TypeFloat represents a floating-point value.
	TypeFloat
	// TypeBool represents a boolean value.
	TypeBool
	// TypeArray represents an array value.
	TypeArray
	// TypeObject represents an object/map value.
	TypeObject
	// TypeDuration represents a time duration.
	TypeDuration
	// TypeEnum represents a value from a fixed set.
	TypeEnum
)

// String returns the string representation of the type.
func (t SettingType) String() string {
	switch t {
	case TypeString:
		return "string"
	case TypeInt:
		return "integer"
	case TypeFloat:
		return "number"
	case TypeBool:
		return "boolean"
	case TypeArray:
		return "array"
	case TypeObject:
		return "object"
	case TypeDuration:
		return "duration"
	case TypeEnum:
		return "enum"
	default:
		return "unknown"
	}
}

// SettingScope defines where a setting can be configured.
type SettingScope uint8

const (
	// ScopeGlobal indicates the setting can only be set in user global config.
	ScopeGlobal SettingScope = 1 << iota
	// ScopeWorkspace indicates the setting can be set at workspace level.
	ScopeWorkspace
	// ScopeLanguage indicates the setting can be overridden per-language.
	ScopeLanguage
	// ScopeResource indicates the setting can be overridden per-file (future).
	ScopeResource

	// ScopeAll allows the setting at any level.
	ScopeAll = ScopeGlobal | ScopeWorkspace | ScopeLanguage | ScopeResource
)

// String returns a string representation of the scope.
func (s SettingScope) String() string {
	if s == ScopeAll {
		return "all"
	}

	var scopes []string
	if s&ScopeGlobal != 0 {
		scopes = append(scopes, "global")
	}
	if s&ScopeWorkspace != 0 {
		scopes = append(scopes, "workspace")
	}
	if s&ScopeLanguage != 0 {
		scopes = append(scopes, "language")
	}
	if s&ScopeResource != 0 {
		scopes = append(scopes, "resource")
	}

	if len(scopes) == 0 {
		return "none"
	}
	return fmt.Sprintf("%v", scopes)
}

// HasScope checks if the setting supports the given scope.
func (s SettingScope) HasScope(scope SettingScope) bool {
	return s&scope != 0
}

// containsValue checks if a slice contains a value.
func containsValue(slice []any, value any) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// MinValue creates a pointer to a float64 for use as Minimum.
func MinValue(v float64) *float64 {
	return &v
}

// MaxValue creates a pointer to a float64 for use as Maximum.
func MaxValue(v float64) *float64 {
	return &v
}
