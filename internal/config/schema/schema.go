// Package schema provides JSON Schema-based validation for Keystorm configuration.
//
// The schema package defines the structure and constraints for configuration
// settings, enabling validation against a formal schema definition.
package schema

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed keystorm.schema.json
var schemaFS embed.FS

// Schema represents a JSON Schema definition for configuration validation.
type Schema struct {
	// ID is the schema identifier ($id).
	ID string `json:"$id,omitempty"`

	// Schema is the JSON Schema version ($schema).
	SchemaVersion string `json:"$schema,omitempty"`

	// Title is a descriptive title.
	Title string `json:"title,omitempty"`

	// Description provides documentation.
	Description string `json:"description,omitempty"`

	// Type is the JSON type (string, number, integer, boolean, array, object, null).
	Type SchemaType `json:"type,omitempty"`

	// Properties defines object properties (for type: object).
	Properties map[string]*Schema `json:"properties,omitempty"`

	// AdditionalProperties controls whether extra properties are allowed.
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`

	// Required lists required property names.
	Required []string `json:"required,omitempty"`

	// Items defines the schema for array elements.
	Items *Schema `json:"items,omitempty"`

	// Enum lists allowed values.
	Enum []any `json:"enum,omitempty"`

	// Const defines a single allowed value.
	Const any `json:"const,omitempty"`

	// Default is the default value.
	Default any `json:"default,omitempty"`

	// Minimum for numeric types.
	Minimum *float64 `json:"minimum,omitempty"`

	// Maximum for numeric types.
	Maximum *float64 `json:"maximum,omitempty"`

	// ExclusiveMinimum for numeric types.
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`

	// ExclusiveMaximum for numeric types.
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`

	// MultipleOf requires numbers to be multiples of this value.
	MultipleOf *float64 `json:"multipleOf,omitempty"`

	// MinLength for strings.
	MinLength *int `json:"minLength,omitempty"`

	// MaxLength for strings.
	MaxLength *int `json:"maxLength,omitempty"`

	// Pattern is a regex pattern for strings.
	Pattern string `json:"pattern,omitempty"`

	// Format is a semantic format hint (e.g., "uri", "email", "duration").
	Format string `json:"format,omitempty"`

	// MinItems for arrays.
	MinItems *int `json:"minItems,omitempty"`

	// MaxItems for arrays.
	MaxItems *int `json:"maxItems,omitempty"`

	// UniqueItems requires array elements to be unique.
	UniqueItems bool `json:"uniqueItems,omitempty"`

	// AllOf requires all schemas to match.
	AllOf []*Schema `json:"allOf,omitempty"`

	// AnyOf requires at least one schema to match.
	AnyOf []*Schema `json:"anyOf,omitempty"`

	// OneOf requires exactly one schema to match.
	OneOf []*Schema `json:"oneOf,omitempty"`

	// Not inverts a schema.
	Not *Schema `json:"not,omitempty"`

	// If/Then/Else for conditional validation.
	If   *Schema `json:"if,omitempty"`
	Then *Schema `json:"then,omitempty"`
	Else *Schema `json:"else,omitempty"`

	// Ref references another schema ($ref).
	Ref string `json:"$ref,omitempty"`

	// Defs contains schema definitions ($defs).
	Defs map[string]*Schema `json:"$defs,omitempty"`

	// Keystorm-specific extensions
	// Scope defines where this setting can be configured.
	Scope string `json:"x-scope,omitempty"`

	// Deprecated marks the setting as deprecated.
	Deprecated bool `json:"deprecated,omitempty"`

	// DeprecationMessage explains why and what to use instead.
	DeprecationMessage string `json:"x-deprecation-message,omitempty"`

	// Tags for categorization.
	Tags []string `json:"x-tags,omitempty"`

	// Order for display ordering.
	Order int `json:"x-order,omitempty"`
}

// SchemaType represents JSON Schema type(s).
// Can be a single type or an array of types.
type SchemaType struct {
	Types []string
}

// UnmarshalJSON handles both single type and array of types.
func (t *SchemaType) UnmarshalJSON(data []byte) error {
	// Try single string first
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		t.Types = []string{single}
		return nil
	}

	// Try array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return fmt.Errorf("type must be string or array of strings: %w", err)
	}
	t.Types = arr
	return nil
}

// MarshalJSON outputs single type as string, multiple as array.
func (t SchemaType) MarshalJSON() ([]byte, error) {
	if len(t.Types) == 1 {
		return json.Marshal(t.Types[0])
	}
	return json.Marshal(t.Types)
}

// Is checks if the schema type includes the given type.
func (t SchemaType) Is(typ string) bool {
	for _, st := range t.Types {
		if st == typ {
			return true
		}
	}
	return false
}

// IsEmpty returns true if no types are defined.
func (t SchemaType) IsEmpty() bool {
	return len(t.Types) == 0
}

// String returns the type as a string.
func (t SchemaType) String() string {
	if len(t.Types) == 1 {
		return t.Types[0]
	}
	return fmt.Sprintf("%v", t.Types)
}

// schemaCache caches the loaded schema.
var (
	schemaCache     *Schema
	schemaCacheOnce sync.Once
	schemaCacheErr  error
)

// LoadEmbedded loads the embedded Keystorm configuration schema.
func LoadEmbedded() (*Schema, error) {
	schemaCacheOnce.Do(func() {
		data, err := schemaFS.ReadFile("keystorm.schema.json")
		if err != nil {
			schemaCacheErr = fmt.Errorf("failed to read embedded schema: %w", err)
			return
		}

		schemaCache = &Schema{}
		if err := json.Unmarshal(data, schemaCache); err != nil {
			schemaCacheErr = fmt.Errorf("failed to parse embedded schema: %w", err)
			schemaCache = nil
			return
		}
	})

	return schemaCache, schemaCacheErr
}

// Parse parses a JSON Schema from bytes.
func Parse(data []byte) (*Schema, error) {
	s := &Schema{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}
	return s, nil
}

// GetProperty returns the schema for a nested property path.
// Path is dot-separated (e.g., "editor.tabSize").
func (s *Schema) GetProperty(path string) *Schema {
	if s == nil || path == "" {
		return s
	}

	parts := splitPath(path)
	current := s

	for _, part := range parts {
		if current.Properties == nil {
			return nil
		}
		prop, ok := current.Properties[part]
		if !ok {
			return nil
		}
		current = prop
	}

	return current
}

// HasProperty checks if a property exists at the given path.
func (s *Schema) HasProperty(path string) bool {
	return s.GetProperty(path) != nil
}

// IsRequired checks if a property is required.
func (s *Schema) IsRequired(name string) bool {
	for _, req := range s.Required {
		if req == name {
			return true
		}
	}
	return false
}

// AllowsAdditionalProperties returns whether additional properties are allowed.
func (s *Schema) AllowsAdditionalProperties() bool {
	if s.AdditionalProperties == nil {
		return true // Default is true
	}
	return *s.AdditionalProperties
}

// splitPath splits a dot-separated path into parts.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}

	var parts []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
