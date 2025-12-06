package schema

import (
	"encoding/json"
	"testing"
)

func TestSchemaType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single type", `"string"`, []string{"string"}},
		{"array types", `["string", "null"]`, []string{"string", "null"}},
		{"number type", `"number"`, []string{"number"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var st SchemaType
			if err := json.Unmarshal([]byte(tt.input), &st); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if len(st.Types) != len(tt.expected) {
				t.Fatalf("got %d types, want %d", len(st.Types), len(tt.expected))
			}
			for i, exp := range tt.expected {
				if st.Types[i] != exp {
					t.Errorf("type[%d] = %q, want %q", i, st.Types[i], exp)
				}
			}
		})
	}
}

func TestSchemaType_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		types    []string
		expected string
	}{
		{"single type", []string{"string"}, `"string"`},
		{"multiple types", []string{"string", "null"}, `["string","null"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := SchemaType{Types: tt.types}
			data, err := json.Marshal(st)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("got %s, want %s", string(data), tt.expected)
			}
		})
	}
}

func TestSchemaType_Is(t *testing.T) {
	st := SchemaType{Types: []string{"string", "null"}}

	if !st.Is("string") {
		t.Error("expected Is('string') to be true")
	}
	if !st.Is("null") {
		t.Error("expected Is('null') to be true")
	}
	if st.Is("number") {
		t.Error("expected Is('number') to be false")
	}
}

func TestSchemaType_IsEmpty(t *testing.T) {
	empty := SchemaType{}
	if !empty.IsEmpty() {
		t.Error("expected empty type to be empty")
	}

	nonEmpty := SchemaType{Types: []string{"string"}}
	if nonEmpty.IsEmpty() {
		t.Error("expected non-empty type to not be empty")
	}
}

func TestSchema_GetProperty(t *testing.T) {
	schema := &Schema{
		Properties: map[string]*Schema{
			"editor": {
				Properties: map[string]*Schema{
					"tabSize": {
						Type:    SchemaType{Types: []string{"integer"}},
						Minimum: floatPtr(1),
						Maximum: floatPtr(16),
					},
				},
			},
		},
	}

	// Direct property
	editor := schema.GetProperty("editor")
	if editor == nil {
		t.Fatal("expected to find 'editor' property")
	}

	// Nested property
	tabSize := schema.GetProperty("editor.tabSize")
	if tabSize == nil {
		t.Fatal("expected to find 'editor.tabSize' property")
	}
	if !tabSize.Type.Is("integer") {
		t.Error("expected tabSize type to be integer")
	}

	// Non-existent
	if schema.GetProperty("nonexistent") != nil {
		t.Error("expected nil for non-existent property")
	}
	if schema.GetProperty("editor.nonexistent") != nil {
		t.Error("expected nil for non-existent nested property")
	}
}

func TestSchema_HasProperty(t *testing.T) {
	schema := &Schema{
		Properties: map[string]*Schema{
			"editor": {
				Properties: map[string]*Schema{
					"tabSize": {},
				},
			},
		},
	}

	if !schema.HasProperty("editor") {
		t.Error("expected HasProperty('editor') to be true")
	}
	if !schema.HasProperty("editor.tabSize") {
		t.Error("expected HasProperty('editor.tabSize') to be true")
	}
	if schema.HasProperty("nonexistent") {
		t.Error("expected HasProperty('nonexistent') to be false")
	}
}

func TestSchema_IsRequired(t *testing.T) {
	schema := &Schema{
		Required: []string{"name", "path"},
	}

	if !schema.IsRequired("name") {
		t.Error("expected 'name' to be required")
	}
	if !schema.IsRequired("path") {
		t.Error("expected 'path' to be required")
	}
	if schema.IsRequired("optional") {
		t.Error("expected 'optional' to not be required")
	}
}

func TestSchema_AllowsAdditionalProperties(t *testing.T) {
	// Default (nil) allows additional
	s1 := &Schema{}
	if !s1.AllowsAdditionalProperties() {
		t.Error("expected default to allow additional properties")
	}

	// Explicit true
	s2 := &Schema{AdditionalProperties: boolPtr(true)}
	if !s2.AllowsAdditionalProperties() {
		t.Error("expected explicit true to allow additional properties")
	}

	// Explicit false
	s3 := &Schema{AdditionalProperties: boolPtr(false)}
	if s3.AllowsAdditionalProperties() {
		t.Error("expected explicit false to not allow additional properties")
	}
}

func TestParse(t *testing.T) {
	jsonSchema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name"]
	}`

	schema, err := Parse([]byte(jsonSchema))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if !schema.Type.Is("object") {
		t.Error("expected type to be object")
	}
	if len(schema.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(schema.Properties))
	}
	if !schema.IsRequired("name") {
		t.Error("expected 'name' to be required")
	}
}

func TestLoadEmbedded(t *testing.T) {
	schema, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	if schema == nil {
		t.Fatal("expected non-nil schema")
	}

	// Check some known properties
	if !schema.HasProperty("editor") {
		t.Error("expected 'editor' property in embedded schema")
	}
	if !schema.HasProperty("editor.tabSize") {
		t.Error("expected 'editor.tabSize' property in embedded schema")
	}
	if !schema.HasProperty("ai") {
		t.Error("expected 'ai' property in embedded schema")
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"", nil},
		{"simple", []string{"simple"}},
		{"editor.tabSize", []string{"editor", "tabSize"}},
		{"deep.nested.path", []string{"deep", "nested", "path"}},
	}

	for _, tt := range tests {
		result := splitPath(tt.path)
		if len(result) != len(tt.expected) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.path, result, tt.expected)
			continue
		}
		for i, exp := range tt.expected {
			if result[i] != exp {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.path, i, result[i], exp)
			}
		}
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}
