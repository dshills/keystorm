package schema

import (
	"testing"
)

func TestValidator_Validate_TypeChecks(t *testing.T) {
	tests := []struct {
		name      string
		schema    *Schema
		data      map[string]any
		wantError bool
	}{
		{
			name:      "valid string",
			schema:    &Schema{Type: SchemaType{Types: []string{"object"}}, Properties: map[string]*Schema{"name": {Type: SchemaType{Types: []string{"string"}}}}},
			data:      map[string]any{"name": "test"},
			wantError: false,
		},
		{
			name:      "invalid string (got int)",
			schema:    &Schema{Type: SchemaType{Types: []string{"object"}}, Properties: map[string]*Schema{"name": {Type: SchemaType{Types: []string{"string"}}}}},
			data:      map[string]any{"name": 123},
			wantError: true,
		},
		{
			name:      "valid integer",
			schema:    &Schema{Type: SchemaType{Types: []string{"object"}}, Properties: map[string]*Schema{"count": {Type: SchemaType{Types: []string{"integer"}}}}},
			data:      map[string]any{"count": 42},
			wantError: false,
		},
		{
			name:      "invalid integer (got float)",
			schema:    &Schema{Type: SchemaType{Types: []string{"object"}}, Properties: map[string]*Schema{"count": {Type: SchemaType{Types: []string{"integer"}}}}},
			data:      map[string]any{"count": 3.14},
			wantError: true,
		},
		{
			name:      "valid boolean",
			schema:    &Schema{Type: SchemaType{Types: []string{"object"}}, Properties: map[string]*Schema{"enabled": {Type: SchemaType{Types: []string{"boolean"}}}}},
			data:      map[string]any{"enabled": true},
			wantError: false,
		},
		{
			name:      "valid array",
			schema:    &Schema{Type: SchemaType{Types: []string{"object"}}, Properties: map[string]*Schema{"items": {Type: SchemaType{Types: []string{"array"}}}}},
			data:      map[string]any{"items": []any{"a", "b"}},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.schema)
			err := v.Validate(tt.data)
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidator_Validate_Enum(t *testing.T) {
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"level": {
				Type: SchemaType{Types: []string{"string"}},
				Enum: []any{"debug", "info", "warn", "error"},
			},
		},
	}

	v := NewValidator(schema)

	// Valid enum value
	err := v.Validate(map[string]any{"level": "info"})
	if err != nil {
		t.Errorf("expected valid enum to pass: %v", err)
	}

	// Invalid enum value
	err = v.Validate(map[string]any{"level": "invalid"})
	if err == nil {
		t.Error("expected invalid enum to fail")
	}
}

func TestValidator_Validate_Range(t *testing.T) {
	min := float64(1)
	max := float64(16)
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"tabSize": {
				Type:    SchemaType{Types: []string{"integer"}},
				Minimum: &min,
				Maximum: &max,
			},
		},
	}

	v := NewValidator(schema)

	// Valid in range
	if err := v.Validate(map[string]any{"tabSize": 4}); err != nil {
		t.Errorf("expected value in range to pass: %v", err)
	}

	// Below minimum
	if err := v.Validate(map[string]any{"tabSize": 0}); err == nil {
		t.Error("expected value below minimum to fail")
	}

	// Above maximum
	if err := v.Validate(map[string]any{"tabSize": 100}); err == nil {
		t.Error("expected value above maximum to fail")
	}
}

func TestValidator_Validate_StringLength(t *testing.T) {
	minLen := 2
	maxLen := 10
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"name": {
				Type:      SchemaType{Types: []string{"string"}},
				MinLength: &minLen,
				MaxLength: &maxLen,
			},
		},
	}

	v := NewValidator(schema)

	// Valid length
	if err := v.Validate(map[string]any{"name": "test"}); err != nil {
		t.Errorf("expected valid length to pass: %v", err)
	}

	// Too short
	if err := v.Validate(map[string]any{"name": "a"}); err == nil {
		t.Error("expected too short string to fail")
	}

	// Too long
	if err := v.Validate(map[string]any{"name": "this is way too long"}); err == nil {
		t.Error("expected too long string to fail")
	}
}

func TestValidator_Validate_Pattern(t *testing.T) {
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"email": {
				Type:    SchemaType{Types: []string{"string"}},
				Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			},
		},
	}

	v := NewValidator(schema)

	// Valid pattern
	if err := v.Validate(map[string]any{"email": "test@example.com"}); err != nil {
		t.Errorf("expected valid pattern to pass: %v", err)
	}

	// Invalid pattern
	if err := v.Validate(map[string]any{"email": "not-an-email"}); err == nil {
		t.Error("expected invalid pattern to fail")
	}
}

func TestValidator_Validate_Required(t *testing.T) {
	schema := &Schema{
		Type:     SchemaType{Types: []string{"object"}},
		Required: []string{"name", "id"},
		Properties: map[string]*Schema{
			"name": {Type: SchemaType{Types: []string{"string"}}},
			"id":   {Type: SchemaType{Types: []string{"integer"}}},
		},
	}

	v := NewValidator(schema)

	// All required present
	if err := v.Validate(map[string]any{"name": "test", "id": 1}); err != nil {
		t.Errorf("expected valid data to pass: %v", err)
	}

	// Missing required field
	if err := v.Validate(map[string]any{"name": "test"}); err == nil {
		t.Error("expected missing required field to fail")
	}
}

func TestValidator_Validate_Array(t *testing.T) {
	minItems := 1
	maxItems := 5
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"tags": {
				Type:        SchemaType{Types: []string{"array"}},
				MinItems:    &minItems,
				MaxItems:    &maxItems,
				UniqueItems: true,
				Items: &Schema{
					Type: SchemaType{Types: []string{"string"}},
				},
			},
		},
	}

	v := NewValidator(schema)

	// Valid array
	if err := v.Validate(map[string]any{"tags": []any{"a", "b", "c"}}); err != nil {
		t.Errorf("expected valid array to pass: %v", err)
	}

	// Empty array (below minItems)
	if err := v.Validate(map[string]any{"tags": []any{}}); err == nil {
		t.Error("expected empty array to fail minItems check")
	}

	// Too many items
	if err := v.Validate(map[string]any{"tags": []any{"a", "b", "c", "d", "e", "f"}}); err == nil {
		t.Error("expected too many items to fail")
	}

	// Duplicate items
	if err := v.Validate(map[string]any{"tags": []any{"a", "b", "a"}}); err == nil {
		t.Error("expected duplicate items to fail uniqueItems check")
	}

	// Invalid item type
	if err := v.Validate(map[string]any{"tags": []any{"a", 123}}); err == nil {
		t.Error("expected invalid item type to fail")
	}
}

func TestValidator_Validate_NestedObject(t *testing.T) {
	min := float64(1)
	max := float64(16)
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"editor": {
				Type: SchemaType{Types: []string{"object"}},
				Properties: map[string]*Schema{
					"tabSize": {
						Type:    SchemaType{Types: []string{"integer"}},
						Minimum: &min,
						Maximum: &max,
					},
					"insertSpaces": {
						Type: SchemaType{Types: []string{"boolean"}},
					},
				},
			},
		},
	}

	v := NewValidator(schema)

	// Valid nested
	data := map[string]any{
		"editor": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
	}
	if err := v.Validate(data); err != nil {
		t.Errorf("expected valid nested object to pass: %v", err)
	}

	// Invalid nested value
	data = map[string]any{
		"editor": map[string]any{
			"tabSize": 100, // Out of range
		},
	}
	if err := v.Validate(data); err == nil {
		t.Error("expected invalid nested value to fail")
	}
}

func TestValidator_Validate_Format(t *testing.T) {
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"timeout": {
				Type:   SchemaType{Types: []string{"string"}},
				Format: "duration",
			},
		},
	}

	v := NewValidator(schema)

	// Valid duration
	if err := v.Validate(map[string]any{"timeout": "5s"}); err != nil {
		t.Errorf("expected valid duration to pass: %v", err)
	}
	if err := v.Validate(map[string]any{"timeout": "100ms"}); err != nil {
		t.Errorf("expected valid duration to pass: %v", err)
	}

	// Invalid duration
	if err := v.Validate(map[string]any{"timeout": "not-a-duration"}); err == nil {
		t.Error("expected invalid duration to fail")
	}
}

func TestValidator_Validate_OneOf(t *testing.T) {
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"value": {
				OneOf: []*Schema{
					{Type: SchemaType{Types: []string{"string"}}},
					{Type: SchemaType{Types: []string{"integer"}}},
				},
			},
		},
	}

	v := NewValidator(schema)

	// String (matches first)
	if err := v.Validate(map[string]any{"value": "test"}); err != nil {
		t.Errorf("expected string to match oneOf: %v", err)
	}

	// Integer (matches second)
	if err := v.Validate(map[string]any{"value": 42}); err != nil {
		t.Errorf("expected integer to match oneOf: %v", err)
	}

	// Boolean (matches neither)
	if err := v.Validate(map[string]any{"value": true}); err == nil {
		t.Error("expected boolean to fail oneOf")
	}
}

func TestValidator_Validate_AnyOf(t *testing.T) {
	min := float64(0)
	max := float64(100)
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"value": {
				AnyOf: []*Schema{
					{Type: SchemaType{Types: []string{"string"}}},
					{Type: SchemaType{Types: []string{"integer"}}, Minimum: &min, Maximum: &max},
				},
			},
		},
	}

	v := NewValidator(schema)

	// Matches string
	if err := v.Validate(map[string]any{"value": "test"}); err != nil {
		t.Errorf("expected string to match anyOf: %v", err)
	}

	// Matches integer in range
	if err := v.Validate(map[string]any{"value": 50}); err != nil {
		t.Errorf("expected integer to match anyOf: %v", err)
	}

	// Matches neither (boolean)
	if err := v.Validate(map[string]any{"value": true}); err == nil {
		t.Error("expected boolean to fail anyOf")
	}
}

func TestValidator_ValidatePath(t *testing.T) {
	min := float64(1)
	max := float64(16)
	schema := &Schema{
		Properties: map[string]*Schema{
			"editor": {
				Properties: map[string]*Schema{
					"tabSize": {
						Type:    SchemaType{Types: []string{"integer"}},
						Minimum: &min,
						Maximum: &max,
					},
				},
			},
		},
	}

	v := NewValidator(schema)

	// Valid value for path
	if err := v.ValidatePath("editor.tabSize", 4); err != nil {
		t.Errorf("expected valid value to pass: %v", err)
	}

	// Invalid value for path
	if err := v.ValidatePath("editor.tabSize", 100); err == nil {
		t.Error("expected invalid value to fail")
	}

	// Unknown path (non-strict mode)
	if err := v.ValidatePath("unknown.path", "value"); err != nil {
		t.Errorf("expected unknown path to pass in non-strict mode: %v", err)
	}

	// Unknown path (strict mode)
	v.WithStrictMode(true)
	if err := v.ValidatePath("unknown.path", "value"); err == nil {
		t.Error("expected unknown path to fail in strict mode")
	}
}

func TestValidator_WithOptions(t *testing.T) {
	schema := &Schema{}
	v := NewValidator(schema)

	// Chain options
	v.WithStrictMode(true).WithCollectAllErrors(false).WithMaxErrors(10)

	if !v.strictMode {
		t.Error("expected strictMode to be true")
	}
	if v.collectAllErrors {
		t.Error("expected collectAllErrors to be false")
	}
	if v.maxErrors != 10 {
		t.Errorf("expected maxErrors to be 10, got %d", v.maxErrors)
	}
}

func TestValidator_Validate_Const(t *testing.T) {
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"version": {
				Const: "1.0",
			},
		},
	}

	v := NewValidator(schema)

	// Matches const
	if err := v.Validate(map[string]any{"version": "1.0"}); err != nil {
		t.Errorf("expected const value to pass: %v", err)
	}

	// Doesn't match const
	if err := v.Validate(map[string]any{"version": "2.0"}); err == nil {
		t.Error("expected non-const value to fail")
	}
}

func TestValidator_Validate_MultipleOf(t *testing.T) {
	mult := float64(5)
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"value": {
				Type:       SchemaType{Types: []string{"integer"}},
				MultipleOf: &mult,
			},
		},
	}

	v := NewValidator(schema)

	// Is multiple
	if err := v.Validate(map[string]any{"value": 15}); err != nil {
		t.Errorf("expected multiple of 5 to pass: %v", err)
	}

	// Not multiple
	if err := v.Validate(map[string]any{"value": 7}); err == nil {
		t.Error("expected non-multiple to fail")
	}
}

func TestValidator_Validate_ExclusiveRange(t *testing.T) {
	exMin := float64(0)
	exMax := float64(100)
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"value": {
				Type:             SchemaType{Types: []string{"number"}},
				ExclusiveMinimum: &exMin,
				ExclusiveMaximum: &exMax,
			},
		},
	}

	v := NewValidator(schema)

	// In exclusive range
	if err := v.Validate(map[string]any{"value": 50.0}); err != nil {
		t.Errorf("expected value in range to pass: %v", err)
	}

	// At minimum (exclusive, should fail)
	if err := v.Validate(map[string]any{"value": 0.0}); err == nil {
		t.Error("expected value at exclusive minimum to fail")
	}

	// At maximum (exclusive, should fail)
	if err := v.Validate(map[string]any{"value": 100.0}); err == nil {
		t.Error("expected value at exclusive maximum to fail")
	}
}

func TestValidator_Validate_ColorFormat(t *testing.T) {
	schema := &Schema{
		Type: SchemaType{Types: []string{"object"}},
		Properties: map[string]*Schema{
			"color": {
				Type:   SchemaType{Types: []string{"string"}},
				Format: "color",
			},
		},
	}

	v := NewValidator(schema)

	// Valid hex colors
	for _, color := range []string{"#fff", "#FFF", "#ffffff", "#FFFFFF", "#ff0000ff"} {
		if err := v.Validate(map[string]any{"color": color}); err != nil {
			t.Errorf("expected %q to be valid color: %v", color, err)
		}
	}

	// Valid named colors
	for _, color := range []string{"red", "blue", "green", "black", "white"} {
		if err := v.Validate(map[string]any{"color": color}); err != nil {
			t.Errorf("expected %q to be valid color: %v", color, err)
		}
	}

	// Invalid colors
	for _, color := range []string{"#gg0000", "notacolor", "#12"} {
		if err := v.Validate(map[string]any{"color": color}); err == nil {
			t.Errorf("expected %q to be invalid color", color)
		}
	}
}

func TestIsInteger(t *testing.T) {
	tests := []struct {
		value    any
		expected bool
	}{
		{42, true},
		{int64(42), true},
		{3.0, true},   // Whole float
		{3.14, false}, // Non-whole float
		{"42", false},
		{true, false},
	}

	for _, tt := range tests {
		result := isInteger(tt.value)
		if result != tt.expected {
			t.Errorf("isInteger(%v) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestIsNumber(t *testing.T) {
	tests := []struct {
		value    any
		expected bool
	}{
		{42, true},
		{3.14, true},
		{float32(1.0), true},
		{"42", false},
		{true, false},
		{[]int{1, 2}, false},
	}

	for _, tt := range tests {
		result := isNumber(tt.value)
		if result != tt.expected {
			t.Errorf("isNumber(%v) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}
