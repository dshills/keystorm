package layer

import (
	"testing"
)

func TestNewLayer(t *testing.T) {
	l := NewLayer("test", SourceUserGlobal, PriorityUserGlobal)

	if l.Name != "test" {
		t.Errorf("Name = %q, want 'test'", l.Name)
	}
	if l.Source != SourceUserGlobal {
		t.Errorf("Source = %v, want SourceUserGlobal", l.Source)
	}
	if l.Priority != PriorityUserGlobal {
		t.Errorf("Priority = %d, want %d", l.Priority, PriorityUserGlobal)
	}
	if l.Data == nil {
		t.Error("Data should be initialized")
	}
}

func TestNewLayerWithData(t *testing.T) {
	data := map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
		},
	}

	l := NewLayerWithData("test", SourceWorkspace, PriorityWorkspace, data)

	if l.Data == nil {
		t.Fatal("Data should not be nil")
	}

	editor, ok := l.Data["editor"].(map[string]any)
	if !ok {
		t.Fatal("editor should be a map")
	}
	if editor["tabSize"] != 4 {
		t.Errorf("tabSize = %v, want 4", editor["tabSize"])
	}
}

func TestLayer_Clone(t *testing.T) {
	original := NewLayerWithData("original", SourceUserGlobal, PriorityUserGlobal, map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
			"nested": map[string]any{
				"deep": "value",
			},
		},
		"array": []any{"a", "b"},
	})
	original.Path = "/path/to/config"
	original.ReadOnly = true

	cloned := original.Clone()

	// Verify properties copied
	if cloned.Name != original.Name {
		t.Errorf("Name not cloned")
	}
	if cloned.Priority != original.Priority {
		t.Errorf("Priority not cloned")
	}
	if cloned.Source != original.Source {
		t.Errorf("Source not cloned")
	}
	if cloned.Path != original.Path {
		t.Errorf("Path not cloned")
	}
	if cloned.ReadOnly != original.ReadOnly {
		t.Errorf("ReadOnly not cloned")
	}

	// Modify original and verify clone is independent
	original.Data["editor"].(map[string]any)["tabSize"] = 8
	original.Data["editor"].(map[string]any)["nested"].(map[string]any)["deep"] = "modified"

	editor := cloned.Data["editor"].(map[string]any)
	if editor["tabSize"] != 4 {
		t.Error("Clone should be independent - tabSize was modified")
	}
	if editor["nested"].(map[string]any)["deep"] != "value" {
		t.Error("Clone should be independent - nested value was modified")
	}
}

func TestSource_String(t *testing.T) {
	tests := []struct {
		source   Source
		expected string
	}{
		{SourceBuiltin, "builtin"},
		{SourceUserGlobal, "user"},
		{SourceWorkspace, "workspace"},
		{SourceLanguage, "language"},
		{SourceEnv, "environment"},
		{SourceArgs, "arguments"},
		{SourcePlugin, "plugin"},
		{SourceSession, "session"},
		{Source(255), "unknown"},
	}

	for _, tt := range tests {
		got := tt.source.String()
		if got != tt.expected {
			t.Errorf("Source(%d).String() = %q, want %q", tt.source, got, tt.expected)
		}
	}
}

func TestCloneMap(t *testing.T) {
	original := map[string]any{
		"string": "value",
		"int":    42,
		"nested": map[string]any{
			"deep": "data",
		},
		"array": []any{"a", "b", map[string]any{"c": "d"}},
	}

	cloned := cloneMap(original)

	// Verify deep copy
	original["string"] = "changed"
	original["nested"].(map[string]any)["deep"] = "modified"
	original["array"].([]any)[0] = "x"
	original["array"].([]any)[2].(map[string]any)["c"] = "e"

	if cloned["string"] != "value" {
		t.Error("string was not cloned properly")
	}
	if cloned["nested"].(map[string]any)["deep"] != "data" {
		t.Error("nested map was not cloned properly")
	}
	if cloned["array"].([]any)[0] != "a" {
		t.Error("array was not cloned properly")
	}
	if cloned["array"].([]any)[2].(map[string]any)["c"] != "d" {
		t.Error("nested array map was not cloned properly")
	}
}

func TestCloneMap_Nil(t *testing.T) {
	if cloneMap(nil) != nil {
		t.Error("cloneMap(nil) should return nil")
	}
}
