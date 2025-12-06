package layer

import (
	"reflect"
	"testing"
)

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		name     string
		dst      map[string]any
		src      map[string]any
		expected map[string]any
	}{
		{
			name:     "nil dst",
			dst:      nil,
			src:      map[string]any{"a": 1},
			expected: map[string]any{"a": 1},
		},
		{
			name:     "nil src",
			dst:      map[string]any{"a": 1},
			src:      nil,
			expected: map[string]any{"a": 1},
		},
		{
			name:     "simple merge - no overlap",
			dst:      map[string]any{"a": 1},
			src:      map[string]any{"b": 2},
			expected: map[string]any{"a": 1, "b": 2},
		},
		{
			name:     "src overrides dst",
			dst:      map[string]any{"a": 1},
			src:      map[string]any{"a": 2},
			expected: map[string]any{"a": 2},
		},
		{
			name: "nested merge",
			dst: map[string]any{
				"editor": map[string]any{
					"tabSize": 4,
				},
			},
			src: map[string]any{
				"editor": map[string]any{
					"insertSpaces": true,
				},
			},
			expected: map[string]any{
				"editor": map[string]any{
					"tabSize":      4,
					"insertSpaces": true,
				},
			},
		},
		{
			name: "nested override",
			dst: map[string]any{
				"editor": map[string]any{
					"tabSize": 4,
				},
			},
			src: map[string]any{
				"editor": map[string]any{
					"tabSize": 2,
				},
			},
			expected: map[string]any{
				"editor": map[string]any{
					"tabSize": 2,
				},
			},
		},
		{
			name: "deep nested merge",
			dst: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"a": 1,
					},
				},
			},
			src: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"b": 2,
					},
				},
			},
			expected: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"a": 1,
						"b": 2,
					},
				},
			},
		},
		{
			name: "non-map overwrites map",
			dst: map[string]any{
				"value": map[string]any{"a": 1},
			},
			src: map[string]any{
				"value": "string",
			},
			expected: map[string]any{
				"value": "string",
			},
		},
		{
			name: "map overwrites non-map",
			dst: map[string]any{
				"value": "string",
			},
			src: map[string]any{
				"value": map[string]any{"a": 1},
			},
			expected: map[string]any{
				"value": map[string]any{"a": 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMerge(tt.dst, tt.src)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("DeepMerge() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetByPath(t *testing.T) {
	data := map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
			"nested": map[string]any{
				"deep": "value",
			},
		},
		"simple": "string",
	}

	tests := []struct {
		path     string
		expected any
		found    bool
	}{
		{"editor.tabSize", 4, true},
		{"editor.nested.deep", "value", true},
		{"simple", "string", true},
		{"nonexistent", nil, false},
		{"editor.nonexistent", nil, false},
		{"editor.tabSize.invalid", nil, false},
	}

	for _, tt := range tests {
		val, found := GetByPath(data, tt.path)
		if found != tt.found {
			t.Errorf("GetByPath(%q): found = %v, want %v", tt.path, found, tt.found)
		}
		if found && val != tt.expected {
			t.Errorf("GetByPath(%q) = %v, want %v", tt.path, val, tt.expected)
		}
	}
}

func TestGetByPath_NilData(t *testing.T) {
	val, found := GetByPath(nil, "any.path")
	if found {
		t.Error("expected found = false for nil data")
	}
	if val != nil {
		t.Error("expected nil value for nil data")
	}
}

func TestSetByPath(t *testing.T) {
	data := make(map[string]any)

	SetByPath(data, "editor.tabSize", 4)
	SetByPath(data, "editor.insertSpaces", true)
	SetByPath(data, "ui.theme", "dark")
	SetByPath(data, "deep.nested.path.value", "test")

	// Verify structure
	if val, _ := GetByPath(data, "editor.tabSize"); val != 4 {
		t.Errorf("editor.tabSize = %v, want 4", val)
	}
	if val, _ := GetByPath(data, "editor.insertSpaces"); val != true {
		t.Errorf("editor.insertSpaces = %v, want true", val)
	}
	if val, _ := GetByPath(data, "ui.theme"); val != "dark" {
		t.Errorf("ui.theme = %v, want 'dark'", val)
	}
	if val, _ := GetByPath(data, "deep.nested.path.value"); val != "test" {
		t.Errorf("deep.nested.path.value = %v, want 'test'", val)
	}
}

func TestSetByPath_Overwrite(t *testing.T) {
	data := map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
		},
	}

	SetByPath(data, "editor.tabSize", 2)

	if val, _ := GetByPath(data, "editor.tabSize"); val != 2 {
		t.Errorf("editor.tabSize = %v, want 2", val)
	}
}

func TestDeleteByPath(t *testing.T) {
	data := map[string]any{
		"editor": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
		"ui": map[string]any{
			"theme": "dark",
		},
	}

	// Delete existing value
	if !DeleteByPath(data, "editor.tabSize") {
		t.Error("expected DeleteByPath to return true for existing value")
	}
	if _, found := GetByPath(data, "editor.tabSize"); found {
		t.Error("editor.tabSize should be deleted")
	}

	// insertSpaces should still exist
	if _, found := GetByPath(data, "editor.insertSpaces"); !found {
		t.Error("editor.insertSpaces should still exist")
	}

	// Delete non-existent value
	if DeleteByPath(data, "nonexistent.path") {
		t.Error("expected DeleteByPath to return false for non-existent value")
	}

	// Delete from nil
	if DeleteByPath(nil, "any.path") {
		t.Error("expected DeleteByPath to return false for nil data")
	}
}

func TestFlattenMap(t *testing.T) {
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
		"simple": "string",
	}

	flattened := FlattenMap(data)

	expected := map[string]any{
		"editor.tabSize":      4,
		"editor.insertSpaces": true,
		"ui.theme":            "dark",
		"ui.nested.deep":      "value",
		"simple":              "string",
	}

	if len(flattened) != len(expected) {
		t.Errorf("flattened has %d keys, want %d", len(flattened), len(expected))
	}

	for k, v := range expected {
		if flattened[k] != v {
			t.Errorf("flattened[%q] = %v, want %v", k, flattened[k], v)
		}
	}
}

func TestUnflattenMap(t *testing.T) {
	flattened := map[string]any{
		"editor.tabSize":      4,
		"editor.insertSpaces": true,
		"ui.theme":            "dark",
	}

	unflattened := UnflattenMap(flattened)

	if val, _ := GetByPath(unflattened, "editor.tabSize"); val != 4 {
		t.Errorf("editor.tabSize = %v, want 4", val)
	}
	if val, _ := GetByPath(unflattened, "editor.insertSpaces"); val != true {
		t.Errorf("editor.insertSpaces = %v, want true", val)
	}
	if val, _ := GetByPath(unflattened, "ui.theme"); val != "dark" {
		t.Errorf("ui.theme = %v, want 'dark'", val)
	}
}

func TestDiffMaps(t *testing.T) {
	old := map[string]any{
		"editor": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
		"removed": "value",
	}

	new := map[string]any{
		"editor": map[string]any{
			"tabSize":      2, // modified
			"insertSpaces": true,
		},
		"added": "new", // added
	}

	added, modified, removed := DiffMaps(old, new)

	// Check added
	if len(added) != 1 || added[0] != "added" {
		t.Errorf("added = %v, want [added]", added)
	}

	// Check modified
	if len(modified) != 1 || modified[0] != "editor.tabSize" {
		t.Errorf("modified = %v, want [editor.tabSize]", modified)
	}

	// Check removed
	if len(removed) != 1 || removed[0] != "removed" {
		t.Errorf("removed = %v, want [removed]", removed)
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        any
		b        any
		expected bool
	}{
		{"nil nil", nil, nil, true},
		{"nil non-nil", nil, 1, false},
		{"non-nil nil", 1, nil, false},
		{"same int", 1, 1, true},
		{"different int", 1, 2, false},
		{"same string", "a", "a", true},
		{"different string", "a", "b", false},
		{"same map", map[string]any{"a": 1}, map[string]any{"a": 1}, true},
		{"different map", map[string]any{"a": 1}, map[string]any{"a": 2}, false},
		{"same slice", []any{1, 2}, []any{1, 2}, true},
		{"different slice", []any{1, 2}, []any{1, 3}, false},
		{"different length slice", []any{1}, []any{1, 2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valuesEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
