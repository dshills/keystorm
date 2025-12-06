package registry

import (
	"testing"
)

func TestSettingType_String(t *testing.T) {
	tests := []struct {
		typ  SettingType
		want string
	}{
		{TypeString, "string"},
		{TypeInt, "integer"},
		{TypeFloat, "number"},
		{TypeBool, "boolean"},
		{TypeArray, "array"},
		{TypeObject, "object"},
		{TypeDuration, "duration"},
		{TypeEnum, "enum"},
		{SettingType(255), "unknown"},
	}

	for _, tt := range tests {
		got := tt.typ.String()
		if got != tt.want {
			t.Errorf("SettingType(%d).String() = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestSettingScope_String(t *testing.T) {
	tests := []struct {
		scope SettingScope
		want  string
	}{
		{ScopeGlobal, "[global]"},
		{ScopeWorkspace, "[workspace]"},
		{ScopeLanguage, "[language]"},
		{ScopeAll, "all"},
		{ScopeGlobal | ScopeWorkspace, "[global workspace]"},
		{0, "none"},
	}

	for _, tt := range tests {
		got := tt.scope.String()
		if got != tt.want {
			t.Errorf("SettingScope(%d).String() = %q, want %q", tt.scope, got, tt.want)
		}
	}
}

func TestSettingScope_HasScope(t *testing.T) {
	tests := []struct {
		scope    SettingScope
		check    SettingScope
		expected bool
	}{
		{ScopeAll, ScopeGlobal, true},
		{ScopeAll, ScopeWorkspace, true},
		{ScopeGlobal, ScopeWorkspace, false},
		{ScopeGlobal | ScopeWorkspace, ScopeGlobal, true},
		{ScopeGlobal | ScopeWorkspace, ScopeLanguage, false},
	}

	for _, tt := range tests {
		got := tt.scope.HasScope(tt.check)
		if got != tt.expected {
			t.Errorf("SettingScope(%d).HasScope(%d) = %v, want %v",
				tt.scope, tt.check, got, tt.expected)
		}
	}
}

func TestSetting_Validate_TypeString(t *testing.T) {
	s := &Setting{
		Path: "test.string",
		Type: TypeString,
	}

	// Valid
	if err := s.Validate("hello"); err != nil {
		t.Errorf("expected valid string, got error: %v", err)
	}

	// Invalid
	if err := s.Validate(123); err == nil {
		t.Error("expected error for non-string value")
	}
}

func TestSetting_Validate_TypeInt(t *testing.T) {
	s := &Setting{
		Path: "test.int",
		Type: TypeInt,
	}

	// Valid integer types
	for _, v := range []any{42, int64(42), int32(42), uint(42)} {
		if err := s.Validate(v); err != nil {
			t.Errorf("expected valid int for %T, got error: %v", v, err)
		}
	}

	// Invalid
	if err := s.Validate("42"); err == nil {
		t.Error("expected error for string value")
	}
}

func TestSetting_Validate_TypeBool(t *testing.T) {
	s := &Setting{
		Path: "test.bool",
		Type: TypeBool,
	}

	if err := s.Validate(true); err != nil {
		t.Errorf("expected valid bool, got error: %v", err)
	}

	if err := s.Validate("true"); err == nil {
		t.Error("expected error for string value")
	}
}

func TestSetting_Validate_Enum(t *testing.T) {
	s := &Setting{
		Path: "test.enum",
		Type: TypeEnum,
		Enum: []any{"on", "off", "auto"},
	}

	// Valid
	if err := s.Validate("on"); err != nil {
		t.Errorf("expected valid enum, got error: %v", err)
	}

	// Invalid
	if err := s.Validate("maybe"); err == nil {
		t.Error("expected error for invalid enum value")
	}
}

func TestSetting_Validate_Range(t *testing.T) {
	min := 1.0
	max := 100.0
	s := &Setting{
		Path:    "test.range",
		Type:    TypeInt,
		Minimum: &min,
		Maximum: &max,
	}

	// Valid
	if err := s.Validate(50); err != nil {
		t.Errorf("expected valid in-range value, got error: %v", err)
	}

	// Below minimum
	if err := s.Validate(0); err == nil {
		t.Error("expected error for value below minimum")
	}

	// Above maximum
	if err := s.Validate(101); err == nil {
		t.Error("expected error for value above maximum")
	}
}

func TestSetting_Validate_Pattern(t *testing.T) {
	s := &Setting{
		Path:    "test.pattern",
		Type:    TypeString,
		Pattern: "^[a-z]+$",
	}

	// Valid
	if err := s.Validate("hello"); err != nil {
		t.Errorf("expected valid pattern match, got error: %v", err)
	}

	// Invalid
	if err := s.Validate("Hello123"); err == nil {
		t.Error("expected error for pattern mismatch")
	}
}

func TestMinMaxValue(t *testing.T) {
	min := MinValue(1.0)
	max := MaxValue(100.0)

	if *min != 1.0 {
		t.Errorf("MinValue(1.0) = %v, want 1.0", *min)
	}

	if *max != 100.0 {
		t.Errorf("MaxValue(100.0) = %v, want 100.0", *max)
	}
}
