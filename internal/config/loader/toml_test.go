package loader

import (
	"io/fs"
	"strings"
	"testing"
	"time"
)

// MemFS is an in-memory file system for testing.
type MemFS struct {
	files map[string][]byte
}

func NewMemFS() *MemFS {
	return &MemFS{files: make(map[string][]byte)}
}

func (m *MemFS) AddFile(path string, content string) {
	m.files[path] = []byte(content)
}

func (m *MemFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (m *MemFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return data, nil
}

func (m *MemFS) Stat(path string) (fs.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return &memFileInfo{name: path}, nil
	}
	return nil, fs.ErrNotExist
}

type memFileInfo struct {
	name string
}

func (f *memFileInfo) Name() string       { return f.name }
func (f *memFileInfo) Size() int64        { return 0 }
func (f *memFileInfo) Mode() fs.FileMode  { return 0644 }
func (f *memFileInfo) ModTime() time.Time { return time.Now() }
func (f *memFileInfo) IsDir() bool        { return false }
func (f *memFileInfo) Sys() any           { return nil }

func TestTOMLLoader_Load(t *testing.T) {
	memfs := NewMemFS()
	memfs.AddFile("/config.toml", `
[editor]
tabSize = 4
insertSpaces = true
wordWrap = "on"

[ui]
theme = "dark"
fontSize = 14
`)

	loader := NewTOMLLoaderWithFS(memfs, "/config.toml")
	config, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check editor section
	editor, ok := config["editor"].(map[string]any)
	if !ok {
		t.Fatal("expected editor to be a map")
	}

	if editor["tabSize"] != int64(4) {
		t.Errorf("tabSize = %v (%T), want 4", editor["tabSize"], editor["tabSize"])
	}
	if editor["insertSpaces"] != true {
		t.Errorf("insertSpaces = %v, want true", editor["insertSpaces"])
	}
	if editor["wordWrap"] != "on" {
		t.Errorf("wordWrap = %v, want 'on'", editor["wordWrap"])
	}

	// Check ui section
	ui, ok := config["ui"].(map[string]any)
	if !ok {
		t.Fatal("expected ui to be a map")
	}
	if ui["theme"] != "dark" {
		t.Errorf("theme = %v, want 'dark'", ui["theme"])
	}
}

func TestTOMLLoader_LoadNonExistent(t *testing.T) {
	memfs := NewMemFS()
	loader := NewTOMLLoaderWithFS(memfs, "/nonexistent.toml")

	config, err := loader.Load()
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got: %v", err)
	}
	if config != nil {
		t.Error("expected nil config for non-existent file")
	}
}

func TestTOMLLoader_LoadInvalid(t *testing.T) {
	memfs := NewMemFS()
	memfs.AddFile("/invalid.toml", `
[editor
tabSize = 4
`)

	loader := NewTOMLLoaderWithFS(memfs, "/invalid.toml")
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected parse error")
	}

	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if parseErr.Path != "/invalid.toml" {
		t.Errorf("Path = %q, want '/invalid.toml'", parseErr.Path)
	}
}

func TestTOMLLoader_LoadFromReader(t *testing.T) {
	loader := &TOMLLoader{}

	content := `
theme = "light"
fontSize = 12
`
	reader := strings.NewReader(content)
	config, err := loader.LoadFromReader(reader)
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	if config["theme"] != "light" {
		t.Errorf("theme = %v, want 'light'", config["theme"])
	}
	if config["fontSize"] != int64(12) {
		t.Errorf("fontSize = %v, want 12", config["fontSize"])
	}
}

func TestTOMLLoader_LoadWithIncludes(t *testing.T) {
	memfs := NewMemFS()
	memfs.AddFile("/config.toml", `
"@include" = ["base.toml"]

[editor]
tabSize = 2
`)
	memfs.AddFile("/base.toml", `
[editor]
tabSize = 4
insertSpaces = true

[ui]
theme = "dark"
`)

	loader := NewTOMLLoaderWithFS(memfs, "/config.toml")
	config, err := loader.LoadWithIncludes("/config.toml", 5)
	if err != nil {
		t.Fatalf("LoadWithIncludes failed: %v", err)
	}

	// Check that main file overrides included file
	editor, ok := config["editor"].(map[string]any)
	if !ok {
		t.Fatal("expected editor to be a map")
	}

	// tabSize should be 2 (from main file, overrides base.toml)
	if editor["tabSize"] != int64(2) {
		t.Errorf("tabSize = %v, want 2 (should override included)", editor["tabSize"])
	}

	// insertSpaces should be true (from base.toml)
	if editor["insertSpaces"] != true {
		t.Errorf("insertSpaces = %v, want true (from included file)", editor["insertSpaces"])
	}

	// ui.theme should be dark (from base.toml)
	ui, ok := config["ui"].(map[string]any)
	if !ok {
		t.Fatal("expected ui to be a map")
	}
	if ui["theme"] != "dark" {
		t.Errorf("theme = %v, want 'dark'", ui["theme"])
	}
}

func TestTOMLLoader_LoadWithIncludes_DepthExceeded(t *testing.T) {
	memfs := NewMemFS()
	memfs.AddFile("/a.toml", `"@include" = ["b.toml"]`)
	memfs.AddFile("/b.toml", `"@include" = ["c.toml"]`)
	memfs.AddFile("/c.toml", `"@include" = ["d.toml"]`)
	memfs.AddFile("/d.toml", `value = 1`)

	loader := NewTOMLLoaderWithFS(memfs, "/a.toml")

	// Should fail with depth 2
	_, err := loader.LoadWithIncludes("/a.toml", 2)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
	if !strings.Contains(err.Error(), "depth exceeded") {
		t.Errorf("expected 'depth exceeded' error, got: %v", err)
	}

	// Should succeed with depth 5
	config, err := loader.LoadWithIncludes("/a.toml", 5)
	if err != nil {
		t.Fatalf("expected success with depth 5, got: %v", err)
	}
	if config["value"] != int64(1) {
		t.Errorf("value = %v, want 1", config["value"])
	}
}

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
			name:     "simple merge",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMerge(tt.dst, tt.src)
			if !mapsEqual(result, tt.expected) {
				t.Errorf("DeepMerge() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestClone(t *testing.T) {
	original := map[string]any{
		"string": "value",
		"int":    42,
		"nested": map[string]any{
			"deep": "data",
		},
		"array": []any{"a", "b", "c"},
	}

	cloned := Clone(original)

	// Modify original
	original["string"] = "changed"
	original["nested"].(map[string]any)["deep"] = "modified"
	original["array"].([]any)[0] = "x"

	// Cloned should be unchanged
	if cloned["string"] != "value" {
		t.Error("clone was affected by original modification")
	}
	if cloned["nested"].(map[string]any)["deep"] != "data" {
		t.Error("nested clone was affected by original modification")
	}
	if cloned["array"].([]any)[0] != "a" {
		t.Error("array clone was affected by original modification")
	}
}

func TestClone_Nil(t *testing.T) {
	if Clone(nil) != nil {
		t.Error("Clone(nil) should return nil")
	}
}

// mapsEqual compares two maps for equality (simple version for tests).
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		switch ta := va.(type) {
		case map[string]any:
			tb, ok := vb.(map[string]any)
			if !ok || !mapsEqual(ta, tb) {
				return false
			}
		default:
			if va != vb {
				return false
			}
		}
	}
	return true
}
