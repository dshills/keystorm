package loader

import (
	"os"
	"testing"
	"time"
)

func TestEnvLoader_Load(t *testing.T) {
	// Set test environment variables
	os.Setenv("KEYSTORM_TAB_SIZE", "2")
	os.Setenv("KEYSTORM_THEME", "light")
	os.Setenv("KEYSTORM_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("KEYSTORM_TAB_SIZE")
		os.Unsetenv("KEYSTORM_THEME")
		os.Unsetenv("KEYSTORM_LOG_LEVEL")
	}()

	loader := NewEnvLoader("KEYSTORM_")
	config, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check mapped variable
	if val, ok := getByPath(config, "logging.level"); !ok || val != "debug" {
		t.Errorf("logging.level = %v, want 'debug'", val)
	}

	// Check mapped variable (ui.theme)
	if val, ok := getByPath(config, "ui.theme"); !ok || val != "light" {
		t.Errorf("ui.theme = %v, want 'light'", val)
	}

	// Check mapped variable with int conversion
	if val, ok := getByPath(config, "editor.tabSize"); !ok || val != int64(2) {
		t.Errorf("editor.tabSize = %v (%T), want 2", val, val)
	}
}

func TestEnvLoader_LoadUnmapped(t *testing.T) {
	// Set unmapped environment variable
	os.Setenv("KEYSTORM_CUSTOM_SETTING", "value")
	defer os.Unsetenv("KEYSTORM_CUSTOM_SETTING")

	loader := NewEnvLoader("KEYSTORM_")
	config, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should be converted to custom.setting
	if val, ok := getByPath(config, "custom.setting"); !ok || val != "value" {
		t.Errorf("custom.setting = %v, want 'value'", val)
	}
}

func TestEnvLoader_envToPath(t *testing.T) {
	loader := NewEnvLoader("KEYSTORM_")

	tests := []struct {
		env      string
		expected string
	}{
		{"KEYSTORM_EDITOR_TAB_SIZE", "editor.tabSize"},
		{"KEYSTORM_UI_THEME", "ui.theme"},
		{"KEYSTORM_SIMPLE", "simple"},
		{"KEYSTORM_DEEP_NESTED_PATH", "deep.nestedPath"},
	}

	for _, tt := range tests {
		got := loader.envToPath(tt.env)
		if got != tt.expected {
			t.Errorf("envToPath(%q) = %q, want %q", tt.env, got, tt.expected)
		}
	}
}

func TestEnvLoader_parseValue(t *testing.T) {
	loader := NewEnvLoader("KEYSTORM_")

	tests := []struct {
		input    string
		expected any
	}{
		// Booleans
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"on", true},
		{"1", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"no", false},
		{"off", false},
		{"0", false},

		// Integers
		{"42", int64(42)},
		{"-10", int64(-10)},
		{"999999", int64(999999)},

		// Floats (only with decimal point)
		{"3.14", 3.14},
		{"-2.5", -2.5},

		// Durations
		{"500ms", 500 * time.Millisecond},
		{"1s", time.Second},
		{"5m", 5 * time.Minute},

		// JSON arrays
		{`["a","b","c"]`, []any{"a", "b", "c"}},

		// JSON objects
		{`{"key":"value"}`, map[string]any{"key": "value"}},

		// Strings (default)
		{"hello", "hello"},
		{"hello world", "hello world"},
		{"", ""},
	}

	for _, tt := range tests {
		got := loader.parseValue(tt.input)

		// Special handling for slices and maps
		switch expected := tt.expected.(type) {
		case []any:
			gotSlice, ok := got.([]any)
			if !ok {
				t.Errorf("parseValue(%q) = %T, want []any", tt.input, got)
				continue
			}
			if len(gotSlice) != len(expected) {
				t.Errorf("parseValue(%q) slice length = %d, want %d", tt.input, len(gotSlice), len(expected))
			}
		case map[string]any:
			gotMap, ok := got.(map[string]any)
			if !ok {
				t.Errorf("parseValue(%q) = %T, want map[string]any", tt.input, got)
				continue
			}
			if len(gotMap) != len(expected) {
				t.Errorf("parseValue(%q) map length = %d, want %d", tt.input, len(gotMap), len(expected))
			}
		default:
			if got != tt.expected {
				t.Errorf("parseValue(%q) = %v (%T), want %v (%T)",
					tt.input, got, got, tt.expected, tt.expected)
			}
		}
	}
}

func TestEnvLoader_AddRemoveMapping(t *testing.T) {
	loader := NewEnvLoader("KEYSTORM_")

	// Add custom mapping
	loader.AddMapping("CUSTOM_VAR", "custom.path")

	os.Setenv("CUSTOM_VAR", "custom_value")
	defer os.Unsetenv("CUSTOM_VAR")

	config, _ := loader.Load()

	if val, ok := getByPath(config, "custom.path"); !ok || val != "custom_value" {
		t.Errorf("custom.path = %v, want 'custom_value'", val)
	}

	// Remove mapping
	loader.RemoveMapping("CUSTOM_VAR")
}

func TestNewEnvLoaderWithMapping(t *testing.T) {
	customMapping := map[string]string{
		"MY_VAR": "my.setting",
	}

	loader := NewEnvLoaderWithMapping("MY_", customMapping)

	os.Setenv("MY_VAR", "test_value")
	defer os.Unsetenv("MY_VAR")

	config, _ := loader.Load()

	if val, ok := getByPath(config, "my.setting"); !ok || val != "test_value" {
		t.Errorf("my.setting = %v, want 'test_value'", val)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	os.Setenv("TEST_EXISTS", "exists")
	defer os.Unsetenv("TEST_EXISTS")

	// Existing variable
	if val := GetEnvOrDefault("TEST_EXISTS", "default"); val != "exists" {
		t.Errorf("GetEnvOrDefault = %q, want 'exists'", val)
	}

	// Non-existing variable
	if val := GetEnvOrDefault("TEST_NOT_EXISTS", "default"); val != "default" {
		t.Errorf("GetEnvOrDefault = %q, want 'default'", val)
	}
}

func TestExpandEnvInString(t *testing.T) {
	os.Setenv("TEST_VAR", "world")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		input    string
		expected string
	}{
		{"hello $TEST_VAR", "hello world"},
		{"hello ${TEST_VAR}", "hello world"},
		{"$TEST_VAR!", "world!"},
		{"no vars", "no vars"},
	}

	for _, tt := range tests {
		got := ExpandEnvInString(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandEnvInString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// Helper to get value by path
func getByPath(data map[string]any, path string) (any, bool) {
	parts := splitPath(path)
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

func splitPath(path string) []string {
	var result []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
