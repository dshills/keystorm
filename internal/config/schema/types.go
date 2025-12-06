package schema

// Common type constants for JSON Schema.
const (
	TypeNameString  = "string"
	TypeNameNumber  = "number"
	TypeNameInteger = "integer"
	TypeNameBoolean = "boolean"
	TypeNameArray   = "array"
	TypeNameObject  = "object"
	TypeNameNull    = "null"
)

// Common format constants.
const (
	FormatDuration = "duration"
	FormatURI      = "uri"
	FormatEmail    = "email"
	FormatRegex    = "regex"
	FormatColor    = "color"
	FormatPath     = "path"
)

// Scope constants for Keystorm settings.
const (
	ScopeGlobal    = "global"
	ScopeWorkspace = "workspace"
	ScopeLanguage  = "language"
	ScopeResource  = "resource"
	ScopeAll       = "all"
)

// Builder provides a fluent API for constructing schemas.
type Builder struct {
	schema *Schema
}

// NewBuilder creates a new schema builder.
func NewBuilder() *Builder {
	return &Builder{
		schema: &Schema{},
	}
}

// Build returns the constructed schema.
func (b *Builder) Build() *Schema {
	return b.schema
}

// ID sets the schema ID.
func (b *Builder) ID(id string) *Builder {
	b.schema.ID = id
	return b
}

// Title sets the schema title.
func (b *Builder) Title(title string) *Builder {
	b.schema.Title = title
	return b
}

// Description sets the schema description.
func (b *Builder) Description(desc string) *Builder {
	b.schema.Description = desc
	return b
}

// Type sets the schema type.
func (b *Builder) Type(types ...string) *Builder {
	b.schema.Type = SchemaType{Types: types}
	return b
}

// Default sets the default value.
func (b *Builder) Default(value any) *Builder {
	b.schema.Default = value
	return b
}

// Enum sets allowed values.
func (b *Builder) Enum(values ...any) *Builder {
	b.schema.Enum = values
	return b
}

// Const sets a constant required value.
func (b *Builder) Const(value any) *Builder {
	b.schema.Const = value
	return b
}

// Minimum sets the minimum value for numbers.
func (b *Builder) Minimum(min float64) *Builder {
	b.schema.Minimum = &min
	return b
}

// Maximum sets the maximum value for numbers.
func (b *Builder) Maximum(max float64) *Builder {
	b.schema.Maximum = &max
	return b
}

// ExclusiveMinimum sets the exclusive minimum.
func (b *Builder) ExclusiveMinimum(min float64) *Builder {
	b.schema.ExclusiveMinimum = &min
	return b
}

// ExclusiveMaximum sets the exclusive maximum.
func (b *Builder) ExclusiveMaximum(max float64) *Builder {
	b.schema.ExclusiveMaximum = &max
	return b
}

// MultipleOf sets the multiple requirement.
func (b *Builder) MultipleOf(value float64) *Builder {
	b.schema.MultipleOf = &value
	return b
}

// MinLength sets the minimum string length.
func (b *Builder) MinLength(length int) *Builder {
	b.schema.MinLength = &length
	return b
}

// MaxLength sets the maximum string length.
func (b *Builder) MaxLength(length int) *Builder {
	b.schema.MaxLength = &length
	return b
}

// Pattern sets the regex pattern for strings.
func (b *Builder) Pattern(pattern string) *Builder {
	b.schema.Pattern = pattern
	return b
}

// Format sets the semantic format.
func (b *Builder) Format(format string) *Builder {
	b.schema.Format = format
	return b
}

// MinItems sets the minimum array length.
func (b *Builder) MinItems(count int) *Builder {
	b.schema.MinItems = &count
	return b
}

// MaxItems sets the maximum array length.
func (b *Builder) MaxItems(count int) *Builder {
	b.schema.MaxItems = &count
	return b
}

// UniqueItems requires array items to be unique.
func (b *Builder) UniqueItems() *Builder {
	b.schema.UniqueItems = true
	return b
}

// Items sets the schema for array items.
func (b *Builder) Items(schema *Schema) *Builder {
	b.schema.Items = schema
	return b
}

// Property adds a property to an object schema.
func (b *Builder) Property(name string, schema *Schema) *Builder {
	if b.schema.Properties == nil {
		b.schema.Properties = make(map[string]*Schema)
	}
	b.schema.Properties[name] = schema
	return b
}

// Required marks properties as required.
func (b *Builder) Required(names ...string) *Builder {
	b.schema.Required = append(b.schema.Required, names...)
	return b
}

// AdditionalProperties sets whether additional properties are allowed.
func (b *Builder) AdditionalProperties(allowed bool) *Builder {
	b.schema.AdditionalProperties = &allowed
	return b
}

// AllOf requires all schemas to match.
func (b *Builder) AllOf(schemas ...*Schema) *Builder {
	b.schema.AllOf = schemas
	return b
}

// AnyOf requires at least one schema to match.
func (b *Builder) AnyOf(schemas ...*Schema) *Builder {
	b.schema.AnyOf = schemas
	return b
}

// OneOf requires exactly one schema to match.
func (b *Builder) OneOf(schemas ...*Schema) *Builder {
	b.schema.OneOf = schemas
	return b
}

// Not inverts the schema.
func (b *Builder) Not(schema *Schema) *Builder {
	b.schema.Not = schema
	return b
}

// Ref sets a reference to another schema.
func (b *Builder) Ref(ref string) *Builder {
	b.schema.Ref = ref
	return b
}

// Deprecated marks the setting as deprecated.
func (b *Builder) Deprecated(message string) *Builder {
	b.schema.Deprecated = true
	b.schema.DeprecationMessage = message
	return b
}

// Scope sets the Keystorm setting scope.
func (b *Builder) Scope(scope string) *Builder {
	b.schema.Scope = scope
	return b
}

// Tags sets categorization tags.
func (b *Builder) Tags(tags ...string) *Builder {
	b.schema.Tags = tags
	return b
}

// Order sets the display order.
func (b *Builder) Order(order int) *Builder {
	b.schema.Order = order
	return b
}

// Convenience functions for creating common schema types

// String creates a string schema.
func String() *Builder {
	return NewBuilder().Type(TypeNameString)
}

// Integer creates an integer schema.
func Integer() *Builder {
	return NewBuilder().Type(TypeNameInteger)
}

// Number creates a number schema.
func Number() *Builder {
	return NewBuilder().Type(TypeNameNumber)
}

// Boolean creates a boolean schema.
func Boolean() *Builder {
	return NewBuilder().Type(TypeNameBoolean)
}

// Array creates an array schema.
func Array() *Builder {
	return NewBuilder().Type(TypeNameArray)
}

// Object creates an object schema.
func Object() *Builder {
	return NewBuilder().Type(TypeNameObject)
}

// StringEnum creates a string enum schema.
func StringEnum(values ...string) *Builder {
	anyValues := make([]any, len(values))
	for i, v := range values {
		anyValues[i] = v
	}
	return String().Enum(anyValues...)
}

// IntRange creates an integer schema with min/max.
func IntRange(min, max int) *Builder {
	return Integer().Minimum(float64(min)).Maximum(float64(max))
}
