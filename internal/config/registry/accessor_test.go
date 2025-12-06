package registry

import (
	"testing"
	"time"
)

func TestMapValueStore_GetValue(t *testing.T) {
	data := map[string]any{
		"editor": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
		"ui": map[string]any{
			"theme": "dark",
			"nested": map[string]any{
				"deep": "value",
			},
		},
	}

	store := NewMapValueStore(data)

	tests := []struct {
		path     string
		expected any
		found    bool
	}{
		{"editor.tabSize", 4, true},
		{"editor.insertSpaces", true, true},
		{"ui.theme", "dark", true},
		{"ui.nested.deep", "value", true},
		{"nonexistent", nil, false},
		{"editor.nonexistent", nil, false},
	}

	for _, tt := range tests {
		val, found := store.GetValue(tt.path)
		if found != tt.found {
			t.Errorf("GetValue(%q): found = %v, want %v", tt.path, found, tt.found)
		}
		if found && val != tt.expected {
			t.Errorf("GetValue(%q) = %v, want %v", tt.path, val, tt.expected)
		}
	}
}

func TestAccessor_Get(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path:    "editor.tabSize",
		Type:    TypeInt,
		Default: 4,
	})
	registry.MustRegister(Setting{
		Path:    "editor.insertSpaces",
		Type:    TypeBool,
		Default: true,
	})

	data := map[string]any{
		"editor": map[string]any{
			"tabSize": 2, // Override default
		},
	}

	accessor := NewAccessor(registry, NewMapValueStore(data))

	// Get value that exists in store
	val, err := accessor.Get("editor.tabSize")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != 2 {
		t.Errorf("editor.tabSize = %v, want 2", val)
	}

	// Get value that falls back to default
	val, err = accessor.Get("editor.insertSpaces")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != true {
		t.Errorf("editor.insertSpaces = %v, want true", val)
	}

	// Get unregistered setting
	_, err = accessor.Get("unknown.setting")
	if err == nil {
		t.Error("expected error for unknown setting")
	}
}

func TestAccessor_GetString(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path:    "ui.theme",
		Type:    TypeString,
		Default: "dark",
	})

	data := map[string]any{
		"ui": map[string]any{
			"theme": "light",
		},
	}

	accessor := NewAccessor(registry, NewMapValueStore(data))

	val, err := accessor.GetString("ui.theme")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if val != "light" {
		t.Errorf("ui.theme = %q, want 'light'", val)
	}
}

func TestAccessor_GetInt(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path:    "editor.tabSize",
		Type:    TypeInt,
		Default: 4,
	})

	tests := []struct {
		name     string
		data     map[string]any
		expected int
	}{
		{
			name:     "int value",
			data:     map[string]any{"editor": map[string]any{"tabSize": 2}},
			expected: 2,
		},
		{
			name:     "int64 value",
			data:     map[string]any{"editor": map[string]any{"tabSize": int64(8)}},
			expected: 8,
		},
		{
			name:     "float64 value (from JSON)",
			data:     map[string]any{"editor": map[string]any{"tabSize": float64(4)}},
			expected: 4,
		},
		{
			name:     "default value",
			data:     map[string]any{},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor := NewAccessor(registry, NewMapValueStore(tt.data))
			val, err := accessor.GetInt("editor.tabSize")
			if err != nil {
				t.Fatalf("GetInt failed: %v", err)
			}
			if val != tt.expected {
				t.Errorf("got %d, want %d", val, tt.expected)
			}
		})
	}
}

func TestAccessor_GetInt_TypeError(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path: "test.value",
		Type: TypeInt,
	})

	data := map[string]any{
		"test": map[string]any{
			"value": "not an int",
		},
	}

	accessor := NewAccessor(registry, NewMapValueStore(data))
	_, err := accessor.GetInt("test.value")
	if err == nil {
		t.Error("expected type error")
	}

	typeErr, ok := err.(*TypeError)
	if !ok {
		t.Errorf("expected *TypeError, got %T", err)
	} else {
		if typeErr.Expected != "integer" {
			t.Errorf("expected 'integer', got %q", typeErr.Expected)
		}
	}
}

func TestAccessor_GetFloat64(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path:    "ui.lineHeight",
		Type:    TypeFloat,
		Default: 1.5,
	})

	tests := []struct {
		name     string
		data     map[string]any
		expected float64
	}{
		{
			name:     "float64 value",
			data:     map[string]any{"ui": map[string]any{"lineHeight": 2.0}},
			expected: 2.0,
		},
		{
			name:     "int value",
			data:     map[string]any{"ui": map[string]any{"lineHeight": 2}},
			expected: 2.0,
		},
		{
			name:     "default value",
			data:     map[string]any{},
			expected: 1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor := NewAccessor(registry, NewMapValueStore(tt.data))
			val, err := accessor.GetFloat64("ui.lineHeight")
			if err != nil {
				t.Fatalf("GetFloat64 failed: %v", err)
			}
			if val != tt.expected {
				t.Errorf("got %f, want %f", val, tt.expected)
			}
		})
	}
}

func TestAccessor_GetBool(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path:    "editor.insertSpaces",
		Type:    TypeBool,
		Default: true,
	})

	data := map[string]any{
		"editor": map[string]any{
			"insertSpaces": false,
		},
	}

	accessor := NewAccessor(registry, NewMapValueStore(data))
	val, err := accessor.GetBool("editor.insertSpaces")
	if err != nil {
		t.Fatalf("GetBool failed: %v", err)
	}
	if val != false {
		t.Error("expected false")
	}
}

func TestAccessor_GetStringSlice(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path:    "files.exclude",
		Type:    TypeArray,
		Default: []string{".git", "node_modules"},
	})

	tests := []struct {
		name     string
		data     map[string]any
		expected []string
	}{
		{
			name: "[]string value",
			data: map[string]any{
				"files": map[string]any{
					"exclude": []string{"*.log", "*.tmp"},
				},
			},
			expected: []string{"*.log", "*.tmp"},
		},
		{
			name: "[]any value (from TOML)",
			data: map[string]any{
				"files": map[string]any{
					"exclude": []any{"*.log", "*.tmp"},
				},
			},
			expected: []string{"*.log", "*.tmp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor := NewAccessor(registry, NewMapValueStore(tt.data))
			val, err := accessor.GetStringSlice("files.exclude")
			if err != nil {
				t.Fatalf("GetStringSlice failed: %v", err)
			}
			if len(val) != len(tt.expected) {
				t.Fatalf("length = %d, want %d", len(val), len(tt.expected))
			}
			for i, v := range val {
				if v != tt.expected[i] {
					t.Errorf("val[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestAccessor_GetDuration(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path:    "input.keyTimeout",
		Type:    TypeDuration,
		Default: "500ms",
	})

	tests := []struct {
		name     string
		data     map[string]any
		expected time.Duration
	}{
		{
			name:     "string duration",
			data:     map[string]any{"input": map[string]any{"keyTimeout": "1s"}},
			expected: time.Second,
		},
		{
			name:     "int milliseconds",
			data:     map[string]any{"input": map[string]any{"keyTimeout": 250}},
			expected: 250 * time.Millisecond,
		},
		{
			name:     "float64 milliseconds",
			data:     map[string]any{"input": map[string]any{"keyTimeout": float64(100)}},
			expected: 100 * time.Millisecond,
		},
		{
			name:     "default value",
			data:     map[string]any{},
			expected: 500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor := NewAccessor(registry, NewMapValueStore(tt.data))
			val, err := accessor.GetDuration("input.keyTimeout")
			if err != nil {
				t.Fatalf("GetDuration failed: %v", err)
			}
			if val != tt.expected {
				t.Errorf("got %v, want %v", val, tt.expected)
			}
		})
	}
}

func TestAccessor_GetMap(t *testing.T) {
	registry := New()
	registry.MustRegister(Setting{
		Path: "lsp.settings",
		Type: TypeObject,
	})

	data := map[string]any{
		"lsp": map[string]any{
			"settings": map[string]any{
				"gopls": map[string]any{
					"usePlaceholders": true,
				},
			},
		},
	}

	accessor := NewAccessor(registry, NewMapValueStore(data))
	val, err := accessor.GetMap("lsp.settings")
	if err != nil {
		t.Fatalf("GetMap failed: %v", err)
	}

	gopls, ok := val["gopls"].(map[string]any)
	if !ok {
		t.Fatal("expected gopls to be a map")
	}

	if gopls["usePlaceholders"] != true {
		t.Error("expected usePlaceholders to be true")
	}
}

func TestSetByPath(t *testing.T) {
	data := make(map[string]any)

	setByPath(data, "editor.tabSize", 4)
	setByPath(data, "editor.insertSpaces", true)
	setByPath(data, "ui.theme", "dark")

	// Check structure
	editor, ok := data["editor"].(map[string]any)
	if !ok {
		t.Fatal("expected editor to be a map")
	}
	if editor["tabSize"] != 4 {
		t.Errorf("editor.tabSize = %v, want 4", editor["tabSize"])
	}
	if editor["insertSpaces"] != true {
		t.Errorf("editor.insertSpaces = %v, want true", editor["insertSpaces"])
	}

	ui, ok := data["ui"].(map[string]any)
	if !ok {
		t.Fatal("expected ui to be a map")
	}
	if ui["theme"] != "dark" {
		t.Errorf("ui.theme = %v, want 'dark'", ui["theme"])
	}
}
