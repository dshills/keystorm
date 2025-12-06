package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Validator validates configuration against a schema.
type Validator struct {
	schema *Schema

	// Options
	strictMode       bool // Fail on unknown properties
	collectAllErrors bool // Continue validation after first error
	maxErrors        int  // Maximum errors to collect (0 = unlimited)

	// Pattern cache
	patternCache sync.Map // map[string]*regexp.Regexp
}

// NewValidator creates a validator for the given schema.
func NewValidator(schema *Schema) *Validator {
	return &Validator{
		schema:           schema,
		collectAllErrors: true,
		maxErrors:        100,
	}
}

// WithStrictMode enables strict mode (unknown properties are errors).
func (v *Validator) WithStrictMode(strict bool) *Validator {
	v.strictMode = strict
	return v
}

// WithCollectAllErrors sets whether to collect all errors or stop at first.
func (v *Validator) WithCollectAllErrors(collect bool) *Validator {
	v.collectAllErrors = collect
	return v
}

// WithMaxErrors sets the maximum number of errors to collect.
func (v *Validator) WithMaxErrors(max int) *Validator {
	v.maxErrors = max
	return v
}

// Validate validates configuration data against the schema.
func (v *Validator) Validate(data map[string]any) error {
	if v.schema == nil {
		return nil
	}

	errs := &ValidationErrors{}
	v.validateValue("", data, v.schema, errs)
	return errs.AsError()
}

// ValidatePath validates a single value at a given path.
func (v *Validator) ValidatePath(path string, value any) error {
	if v.schema == nil {
		return nil
	}

	propSchema := v.schema.GetProperty(path)
	if propSchema == nil {
		if v.strictMode {
			return NewUnknownPropertyError(path)
		}
		return nil
	}

	errs := &ValidationErrors{}
	v.validateValue(path, value, propSchema, errs)
	return errs.AsError()
}

// validateValue validates a value against a schema.
func (v *Validator) validateValue(path string, value any, schema *Schema, errs *ValidationErrors) {
	if schema == nil || (v.maxErrors > 0 && errs.Len() >= v.maxErrors) {
		return
	}

	// Handle $ref
	if schema.Ref != "" {
		refSchema := v.resolveRef(schema.Ref)
		if refSchema != nil {
			v.validateValue(path, value, refSchema, errs)
		}
		return
	}

	// Handle allOf
	if len(schema.AllOf) > 0 {
		for _, s := range schema.AllOf {
			v.validateValue(path, value, s, errs)
		}
	}

	// Handle anyOf
	if len(schema.AnyOf) > 0 {
		matched := false
		for _, s := range schema.AnyOf {
			testErrs := &ValidationErrors{}
			v.validateValue(path, value, s, testErrs)
			if !testErrs.HasErrors() {
				matched = true
				break
			}
		}
		if !matched {
			errs.Add(path, "value does not match any of the allowed schemas")
		}
	}

	// Handle oneOf
	if len(schema.OneOf) > 0 {
		matchCount := 0
		for _, s := range schema.OneOf {
			testErrs := &ValidationErrors{}
			v.validateValue(path, value, s, testErrs)
			if !testErrs.HasErrors() {
				matchCount++
			}
		}
		if matchCount == 0 {
			errs.Add(path, "value does not match any of the allowed schemas")
		} else if matchCount > 1 {
			errs.Add(path, "value matches more than one schema (must match exactly one)")
		}
	}

	// Handle not
	if schema.Not != nil {
		testErrs := &ValidationErrors{}
		v.validateValue(path, value, schema.Not, testErrs)
		if !testErrs.HasErrors() {
			errs.Add(path, "value should not match the schema")
		}
	}

	// Handle const
	if schema.Const != nil {
		if !valuesEqual(value, schema.Const) {
			errs.Add(path, fmt.Sprintf("value must be %v", schema.Const))
		}
	}

	// Handle enum
	if len(schema.Enum) > 0 {
		v.validateEnum(path, value, schema.Enum, errs)
	}

	// Type validation
	if !schema.Type.IsEmpty() {
		v.validateType(path, value, schema, errs)
	}
}

// validateType validates the value against the expected type(s).
func (v *Validator) validateType(path string, value any, schema *Schema, errs *ValidationErrors) {
	if value == nil {
		if !schema.Type.Is("null") {
			errs.AddError(NewTypeError(path, schema.Type.String(), value))
		}
		return
	}

	matched := false
	for _, typ := range schema.Type.Types {
		if v.matchesType(value, typ) {
			matched = true
			// Validate type-specific constraints
			switch typ {
			case "string":
				v.validateString(path, value.(string), schema, errs)
			case "number", "integer":
				v.validateNumber(path, value, schema, typ == "integer", errs)
			case "array":
				v.validateArray(path, value, schema, errs)
			case "object":
				v.validateObject(path, value, schema, errs)
			}
			break
		}
	}

	if !matched {
		errs.AddError(NewTypeError(path, schema.Type.String(), value))
	}
}

// matchesType checks if a value matches a JSON Schema type.
func (v *Validator) matchesType(value any, typ string) bool {
	switch typ {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		return isNumber(value)
	case "integer":
		return isInteger(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		return isArray(value)
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "null":
		return value == nil
	default:
		return false
	}
}

// validateString validates string-specific constraints.
func (v *Validator) validateString(path string, value string, schema *Schema, errs *ValidationErrors) {
	// MinLength
	if schema.MinLength != nil && len(value) < *schema.MinLength {
		errs.Add(path, fmt.Sprintf("string length %d is less than minimum %d", len(value), *schema.MinLength))
	}

	// MaxLength
	if schema.MaxLength != nil && len(value) > *schema.MaxLength {
		errs.Add(path, fmt.Sprintf("string length %d is greater than maximum %d", len(value), *schema.MaxLength))
	}

	// Pattern
	if schema.Pattern != "" {
		if !v.matchPattern(value, schema.Pattern) {
			errs.AddError(NewPatternError(path, value, schema.Pattern))
		}
	}

	// Format
	if schema.Format != "" {
		v.validateFormat(path, value, schema.Format, errs)
	}
}

// validateNumber validates numeric constraints.
func (v *Validator) validateNumber(path string, value any, schema *Schema, requireInt bool, errs *ValidationErrors) {
	f := toFloat64(value)

	if requireInt && !isInteger(value) {
		errs.Add(path, fmt.Sprintf("expected integer, got %v", value))
		return
	}

	// Minimum
	if schema.Minimum != nil && f < *schema.Minimum {
		errs.AddError(NewRangeError(path, value, schema.Minimum, schema.Maximum))
	}

	// Maximum
	if schema.Maximum != nil && f > *schema.Maximum {
		errs.AddError(NewRangeError(path, value, schema.Minimum, schema.Maximum))
	}

	// ExclusiveMinimum
	if schema.ExclusiveMinimum != nil && f <= *schema.ExclusiveMinimum {
		errs.Add(path, fmt.Sprintf("value must be greater than %v", *schema.ExclusiveMinimum))
	}

	// ExclusiveMaximum
	if schema.ExclusiveMaximum != nil && f >= *schema.ExclusiveMaximum {
		errs.Add(path, fmt.Sprintf("value must be less than %v", *schema.ExclusiveMaximum))
	}

	// MultipleOf
	if schema.MultipleOf != nil && *schema.MultipleOf != 0 {
		remainder := math.Mod(f, *schema.MultipleOf)
		if math.Abs(remainder) > 1e-10 {
			errs.Add(path, fmt.Sprintf("value must be a multiple of %v", *schema.MultipleOf))
		}
	}
}

// validateArray validates array constraints.
func (v *Validator) validateArray(path string, value any, schema *Schema, errs *ValidationErrors) {
	arr := toSlice(value)
	if arr == nil {
		return
	}

	// MinItems
	if schema.MinItems != nil && len(arr) < *schema.MinItems {
		errs.Add(path, fmt.Sprintf("array has %d items, minimum is %d", len(arr), *schema.MinItems))
	}

	// MaxItems
	if schema.MaxItems != nil && len(arr) > *schema.MaxItems {
		errs.Add(path, fmt.Sprintf("array has %d items, maximum is %d", len(arr), *schema.MaxItems))
	}

	// UniqueItems
	if schema.UniqueItems {
		seen := make(map[string]bool)
		for i, item := range arr {
			// Use JSON marshaling for reliable key generation
			keyBytes, err := json.Marshal(item)
			var key string
			if err != nil {
				key = fmt.Sprintf("%v", item)
			} else {
				key = string(keyBytes)
			}
			if seen[key] {
				errs.Add(path, fmt.Sprintf("array items must be unique, duplicate at index %d", i))
				break
			}
			seen[key] = true
		}
	}

	// Items validation
	if schema.Items != nil {
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			v.validateValue(itemPath, item, schema.Items, errs)
		}
	}
}

// validateObject validates object constraints.
func (v *Validator) validateObject(path string, value any, schema *Schema, errs *ValidationErrors) {
	obj, ok := value.(map[string]any)
	if !ok {
		return
	}

	// Required properties
	for _, req := range schema.Required {
		if _, exists := obj[req]; !exists {
			propPath := joinPath(path, req)
			errs.AddError(NewRequiredError(propPath))
		}
	}

	// Validate each property
	for name, propValue := range obj {
		propPath := joinPath(path, name)

		if propSchema, ok := schema.Properties[name]; ok {
			v.validateValue(propPath, propValue, propSchema, errs)
		} else if v.strictMode && !schema.AllowsAdditionalProperties() {
			errs.AddError(NewUnknownPropertyError(propPath))
		}
	}
}

// validateEnum checks if value is in the allowed enum values.
func (v *Validator) validateEnum(path string, value any, allowed []any, errs *ValidationErrors) {
	for _, a := range allowed {
		if valuesEqual(value, a) {
			return
		}
	}
	errs.AddError(NewEnumError(path, value, allowed))
}

// validateFormat validates string formats.
func (v *Validator) validateFormat(path, value, format string, errs *ValidationErrors) {
	switch format {
	case "duration":
		if _, err := time.ParseDuration(value); err != nil {
			errs.Add(path, fmt.Sprintf("invalid duration format: %s", value))
		}
	case "uri", "url":
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") &&
			!strings.HasPrefix(value, "file://") {
			errs.Add(path, fmt.Sprintf("invalid URI format: %s", value))
		}
	case "email":
		if _, err := mail.ParseAddress(value); err != nil {
			errs.Add(path, fmt.Sprintf("invalid email format: %s", value))
		}
	case "regex":
		if _, err := regexp.Compile(value); err != nil {
			errs.Add(path, fmt.Sprintf("invalid regex: %s", value))
		}
	case "color":
		if !isValidColor(value) {
			errs.Add(path, fmt.Sprintf("invalid color format: %s", value))
		}
	case "path":
		// Basic path validation - just check it's not empty
		if value == "" {
			errs.Add(path, "path cannot be empty")
		}
	}
}

// resolveRef resolves a $ref to its schema.
func (v *Validator) resolveRef(ref string) *Schema {
	if v.schema == nil || v.schema.Defs == nil {
		return nil
	}

	// Handle #/$defs/Name format
	if strings.HasPrefix(ref, "#/$defs/") {
		name := strings.TrimPrefix(ref, "#/$defs/")
		return v.schema.Defs[name]
	}

	return nil
}

// matchPattern checks if a string matches a regex pattern.
func (v *Validator) matchPattern(value, pattern string) bool {
	if cached, ok := v.patternCache.Load(pattern); ok {
		return cached.(*regexp.Regexp).MatchString(value)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	v.patternCache.Store(pattern, re)
	return re.MatchString(value)
}

// Helper functions

func isNumber(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func isInteger(v any) bool {
	switch val := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return float32(int32(val)) == val
	case float64:
		return float64(int64(val)) == val
	default:
		return false
	}
}

func isArray(v any) bool {
	switch v.(type) {
	case []any, []string, []int, []int64, []float64, []bool:
		return true
	default:
		return false
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int8:
		return float64(val)
	case int16:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint8:
		return float64(val)
	case uint16:
		return float64(val)
	case uint32:
		return float64(val)
	case uint64:
		return float64(val)
	case float32:
		return float64(val)
	case float64:
		return val
	default:
		return 0
	}
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	case float32:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

func toSlice(v any) []any {
	switch val := v.(type) {
	case []any:
		return val
	case []string:
		result := make([]any, len(val))
		for i, s := range val {
			result[i] = s
		}
		return result
	case []int:
		result := make([]any, len(val))
		for i, n := range val {
			result[i] = n
		}
		return result
	case []int64:
		result := make([]any, len(val))
		for i, n := range val {
			result[i] = n
		}
		return result
	case []float64:
		result := make([]any, len(val))
		for i, n := range val {
			result[i] = n
		}
		return result
	default:
		return nil
	}
}

func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle numeric comparison with precision preservation
	if isNumber(a) && isNumber(b) {
		// If both are integers, compare as int64 to preserve precision
		// But check for uint64 overflow first
		if isInteger(a) && isInteger(b) {
			// Check for uint64 values that would overflow int64
			if isLargeUint64(a) || isLargeUint64(b) {
				return toFloat64(a) == toFloat64(b)
			}
			return toInt64(a) == toInt64(b)
		}
		return toFloat64(a) == toFloat64(b)
	}

	return a == b
}

func isLargeUint64(v any) bool {
	if val, ok := v.(uint64); ok {
		return val > 9223372036854775807 // math.MaxInt64
	}
	return false
}

func joinPath(base, name string) string {
	if base == "" {
		return name
	}
	return base + "." + name
}

func isValidColor(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Hex color
	if s[0] == '#' {
		s = s[1:]
		if len(s) != 3 && len(s) != 6 && len(s) != 8 {
			return false
		}
		for _, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}
	// Named colors (basic)
	namedColors := map[string]bool{
		"black": true, "white": true, "red": true, "green": true, "blue": true,
		"yellow": true, "cyan": true, "magenta": true, "gray": true, "grey": true,
	}
	return namedColors[strings.ToLower(s)]
}
