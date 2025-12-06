package registry

import (
	"fmt"
	"strings"
	"time"
)

// Accessor provides type-safe access to configuration values.
// It wraps a value store (typically a map) and uses the registry
// for type validation and defaults.
type Accessor struct {
	registry *Registry
	values   ValueStore
}

// ValueStore is the interface for accessing raw configuration values.
type ValueStore interface {
	// GetValue returns the value at the given path.
	// Returns nil, false if the path doesn't exist.
	GetValue(path string) (any, bool)
}

// MapValueStore wraps a nested map as a ValueStore.
type MapValueStore struct {
	data map[string]any
}

// NewMapValueStore creates a ValueStore from a nested map.
func NewMapValueStore(data map[string]any) *MapValueStore {
	return &MapValueStore{data: data}
}

// GetValue returns the value at the given dot-separated path.
func (s *MapValueStore) GetValue(path string) (any, bool) {
	return getByPath(s.data, path)
}

// NewAccessor creates a new type-safe accessor.
func NewAccessor(registry *Registry, values ValueStore) *Accessor {
	return &Accessor{
		registry: registry,
		values:   values,
	}
}

// Get returns the raw value at the given path.
// If the value is not set, returns the default from the registry.
// Returns ErrSettingNotFound if the setting is not registered.
func (a *Accessor) Get(path string) (any, error) {
	// Try to get the actual value
	if val, ok := a.values.GetValue(path); ok {
		return val, nil
	}

	// Fall back to default
	setting := a.registry.Get(path)
	if setting == nil {
		return nil, fmt.Errorf("%w: %s", ErrSettingNotFound, path)
	}

	return setting.Default, nil
}

// GetString returns a string value at the given path.
func (a *Accessor) GetString(path string) (string, error) {
	val, err := a.Get(path)
	if err != nil {
		return "", err
	}

	if val == nil {
		return "", nil
	}

	s, ok := val.(string)
	if !ok {
		return "", &TypeError{
			Path:     path,
			Expected: "string",
			Actual:   fmt.Sprintf("%T", val),
		}
	}

	return s, nil
}

// GetInt returns an integer value at the given path.
func (a *Accessor) GetInt(path string) (int, error) {
	val, err := a.Get(path)
	if err != nil {
		return 0, err
	}

	if val == nil {
		return 0, nil
	}

	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case int32:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, &TypeError{
			Path:     path,
			Expected: "integer",
			Actual:   fmt.Sprintf("%T", val),
		}
	}
}

// GetInt64 returns an int64 value at the given path.
func (a *Accessor) GetInt64(path string) (int64, error) {
	val, err := a.Get(path)
	if err != nil {
		return 0, err
	}

	if val == nil {
		return 0, nil
	}

	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, &TypeError{
			Path:     path,
			Expected: "integer",
			Actual:   fmt.Sprintf("%T", val),
		}
	}
}

// GetFloat64 returns a float64 value at the given path.
func (a *Accessor) GetFloat64(path string) (float64, error) {
	val, err := a.Get(path)
	if err != nil {
		return 0, err
	}

	if val == nil {
		return 0, nil
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, &TypeError{
			Path:     path,
			Expected: "number",
			Actual:   fmt.Sprintf("%T", val),
		}
	}
}

// GetBool returns a boolean value at the given path.
func (a *Accessor) GetBool(path string) (bool, error) {
	val, err := a.Get(path)
	if err != nil {
		return false, err
	}

	if val == nil {
		return false, nil
	}

	b, ok := val.(bool)
	if !ok {
		return false, &TypeError{
			Path:     path,
			Expected: "boolean",
			Actual:   fmt.Sprintf("%T", val),
		}
	}

	return b, nil
}

// GetStringSlice returns a string slice value at the given path.
func (a *Accessor) GetStringSlice(path string) ([]string, error) {
	val, err := a.Get(path)
	if err != nil {
		return nil, err
	}

	if val == nil {
		return nil, nil
	}

	switch v := val.(type) {
	case []string:
		return v, nil
	case []any:
		result := make([]string, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, &TypeError{
					Path:     path,
					Expected: "string array",
					Actual:   fmt.Sprintf("array with %T element", item),
				}
			}
			result[i] = s
		}
		return result, nil
	default:
		return nil, &TypeError{
			Path:     path,
			Expected: "string array",
			Actual:   fmt.Sprintf("%T", val),
		}
	}
}

// GetDuration returns a time.Duration value at the given path.
// Accepts both duration strings (e.g., "500ms") and integers (milliseconds).
func (a *Accessor) GetDuration(path string) (time.Duration, error) {
	val, err := a.Get(path)
	if err != nil {
		return 0, err
	}

	if val == nil {
		return 0, nil
	}

	switch v := val.(type) {
	case time.Duration:
		return v, nil
	case string:
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf("invalid duration string at %s: %w", path, err)
		}
		return d, nil
	case int:
		return time.Duration(v) * time.Millisecond, nil
	case int64:
		return time.Duration(v) * time.Millisecond, nil
	case float64:
		return time.Duration(v) * time.Millisecond, nil
	default:
		return 0, &TypeError{
			Path:     path,
			Expected: "duration",
			Actual:   fmt.Sprintf("%T", val),
		}
	}
}

// GetMap returns a map value at the given path.
func (a *Accessor) GetMap(path string) (map[string]any, error) {
	val, err := a.Get(path)
	if err != nil {
		return nil, err
	}

	if val == nil {
		return nil, nil
	}

	m, ok := val.(map[string]any)
	if !ok {
		return nil, &TypeError{
			Path:     path,
			Expected: "object",
			Actual:   fmt.Sprintf("%T", val),
		}
	}

	return m, nil
}

// TypeError is returned when a type conversion fails.
type TypeError struct {
	Path     string
	Expected string
	Actual   string
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("type error at %s: expected %s, got %s", e.Path, e.Expected, e.Actual)
}

// ErrSettingNotFound is returned when a setting is not registered.
var ErrSettingNotFound = fmt.Errorf("setting not found")

// getByPath navigates a nested map using a dot-separated path.
func getByPath(data map[string]any, path string) (any, bool) {
	if data == nil {
		return nil, false
	}

	parts := strings.Split(path, ".")
	current := any(data)

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		val, exists := m[part]
		if !exists {
			return nil, false
		}

		current = val
	}

	return current, true
}

// setByPath sets a value in a nested map using a dot-separated path.
// Creates intermediate maps as needed.
func setByPath(data map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := data

	// Navigate/create intermediate maps
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			// Create intermediate map
			next := make(map[string]any)
			current[part] = next
			current = next
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
}
