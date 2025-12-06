package schema

import (
	"testing"
)

func TestBuilder_Build(t *testing.T) {
	schema := NewBuilder().
		Type("string").
		Description("A test string").
		Default("default").
		Build()

	if !schema.Type.Is("string") {
		t.Error("expected type to be string")
	}
	if schema.Description != "A test string" {
		t.Errorf("description = %q, want 'A test string'", schema.Description)
	}
	if schema.Default != "default" {
		t.Errorf("default = %v, want 'default'", schema.Default)
	}
}

func TestBuilder_NumericConstraints(t *testing.T) {
	schema := NewBuilder().
		Type("integer").
		Minimum(1).
		Maximum(100).
		ExclusiveMinimum(0).
		ExclusiveMaximum(101).
		MultipleOf(5).
		Build()

	if schema.Minimum == nil || *schema.Minimum != 1 {
		t.Error("expected minimum to be 1")
	}
	if schema.Maximum == nil || *schema.Maximum != 100 {
		t.Error("expected maximum to be 100")
	}
	if schema.ExclusiveMinimum == nil || *schema.ExclusiveMinimum != 0 {
		t.Error("expected exclusiveMinimum to be 0")
	}
	if schema.ExclusiveMaximum == nil || *schema.ExclusiveMaximum != 101 {
		t.Error("expected exclusiveMaximum to be 101")
	}
	if schema.MultipleOf == nil || *schema.MultipleOf != 5 {
		t.Error("expected multipleOf to be 5")
	}
}

func TestBuilder_StringConstraints(t *testing.T) {
	schema := NewBuilder().
		Type("string").
		MinLength(1).
		MaxLength(100).
		Pattern(`^[a-z]+$`).
		Format("email").
		Build()

	if schema.MinLength == nil || *schema.MinLength != 1 {
		t.Error("expected minLength to be 1")
	}
	if schema.MaxLength == nil || *schema.MaxLength != 100 {
		t.Error("expected maxLength to be 100")
	}
	if schema.Pattern != `^[a-z]+$` {
		t.Errorf("pattern = %q, want '^[a-z]+$'", schema.Pattern)
	}
	if schema.Format != "email" {
		t.Errorf("format = %q, want 'email'", schema.Format)
	}
}

func TestBuilder_ArrayConstraints(t *testing.T) {
	itemSchema := String().Build()
	schema := NewBuilder().
		Type("array").
		MinItems(1).
		MaxItems(10).
		UniqueItems().
		Items(itemSchema).
		Build()

	if schema.MinItems == nil || *schema.MinItems != 1 {
		t.Error("expected minItems to be 1")
	}
	if schema.MaxItems == nil || *schema.MaxItems != 10 {
		t.Error("expected maxItems to be 10")
	}
	if !schema.UniqueItems {
		t.Error("expected uniqueItems to be true")
	}
	if schema.Items == nil {
		t.Error("expected items schema to be set")
	}
}

func TestBuilder_ObjectConstraints(t *testing.T) {
	nameSchema := String().Build()
	ageSchema := Integer().Minimum(0).Build()

	schema := NewBuilder().
		Type("object").
		Property("name", nameSchema).
		Property("age", ageSchema).
		Required("name").
		AdditionalProperties(false).
		Build()

	if len(schema.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(schema.Properties))
	}
	if schema.Properties["name"] == nil {
		t.Error("expected 'name' property")
	}
	if schema.Properties["age"] == nil {
		t.Error("expected 'age' property")
	}
	if len(schema.Required) != 1 || schema.Required[0] != "name" {
		t.Error("expected 'name' to be required")
	}
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties != false {
		t.Error("expected additionalProperties to be false")
	}
}

func TestBuilder_Composition(t *testing.T) {
	s1 := String().Build()
	s2 := Integer().Build()

	// AllOf
	schema := NewBuilder().AllOf(s1, s2).Build()
	if len(schema.AllOf) != 2 {
		t.Errorf("expected 2 allOf schemas, got %d", len(schema.AllOf))
	}

	// AnyOf
	schema = NewBuilder().AnyOf(s1, s2).Build()
	if len(schema.AnyOf) != 2 {
		t.Errorf("expected 2 anyOf schemas, got %d", len(schema.AnyOf))
	}

	// OneOf
	schema = NewBuilder().OneOf(s1, s2).Build()
	if len(schema.OneOf) != 2 {
		t.Errorf("expected 2 oneOf schemas, got %d", len(schema.OneOf))
	}

	// Not
	schema = NewBuilder().Not(s1).Build()
	if schema.Not == nil {
		t.Error("expected not schema to be set")
	}
}

func TestBuilder_Keystorm(t *testing.T) {
	schema := NewBuilder().
		Deprecated("Use newSetting instead").
		Scope("global").
		Tags("editor", "experimental").
		Order(10).
		Build()

	if !schema.Deprecated {
		t.Error("expected deprecated to be true")
	}
	if schema.DeprecationMessage != "Use newSetting instead" {
		t.Errorf("deprecation message = %q", schema.DeprecationMessage)
	}
	if schema.Scope != "global" {
		t.Errorf("scope = %q, want 'global'", schema.Scope)
	}
	if len(schema.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(schema.Tags))
	}
	if schema.Order != 10 {
		t.Errorf("order = %d, want 10", schema.Order)
	}
}

func TestBuilder_Enum(t *testing.T) {
	schema := NewBuilder().
		Type("string").
		Enum("debug", "info", "warn", "error").
		Build()

	if len(schema.Enum) != 4 {
		t.Errorf("expected 4 enum values, got %d", len(schema.Enum))
	}
}

func TestBuilder_Const(t *testing.T) {
	schema := NewBuilder().Const("fixed").Build()
	if schema.Const != "fixed" {
		t.Errorf("const = %v, want 'fixed'", schema.Const)
	}
}

func TestBuilder_Ref(t *testing.T) {
	schema := NewBuilder().Ref("#/$defs/MyType").Build()
	if schema.Ref != "#/$defs/MyType" {
		t.Errorf("ref = %q, want '#/$defs/MyType'", schema.Ref)
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// String
	s := String().Build()
	if !s.Type.Is("string") {
		t.Error("String() should create string type")
	}

	// Integer
	i := Integer().Build()
	if !i.Type.Is("integer") {
		t.Error("Integer() should create integer type")
	}

	// Number
	n := Number().Build()
	if !n.Type.Is("number") {
		t.Error("Number() should create number type")
	}

	// Boolean
	b := Boolean().Build()
	if !b.Type.Is("boolean") {
		t.Error("Boolean() should create boolean type")
	}

	// Array
	a := Array().Build()
	if !a.Type.Is("array") {
		t.Error("Array() should create array type")
	}

	// Object
	o := Object().Build()
	if !o.Type.Is("object") {
		t.Error("Object() should create object type")
	}
}

func TestStringEnum(t *testing.T) {
	schema := StringEnum("a", "b", "c").Build()

	if !schema.Type.Is("string") {
		t.Error("StringEnum should create string type")
	}
	if len(schema.Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(schema.Enum))
	}
}

func TestIntRange(t *testing.T) {
	schema := IntRange(1, 100).Build()

	if !schema.Type.Is("integer") {
		t.Error("IntRange should create integer type")
	}
	if schema.Minimum == nil || *schema.Minimum != 1 {
		t.Error("expected minimum to be 1")
	}
	if schema.Maximum == nil || *schema.Maximum != 100 {
		t.Error("expected maximum to be 100")
	}
}

func TestBuilder_ID(t *testing.T) {
	schema := NewBuilder().ID("https://example.com/schema.json").Build()
	if schema.ID != "https://example.com/schema.json" {
		t.Errorf("ID = %q, want 'https://example.com/schema.json'", schema.ID)
	}
}

func TestBuilder_Title(t *testing.T) {
	schema := NewBuilder().Title("Test Schema").Build()
	if schema.Title != "Test Schema" {
		t.Errorf("Title = %q, want 'Test Schema'", schema.Title)
	}
}
