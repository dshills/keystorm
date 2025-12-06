package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxFileSize != 10*1024*1024 {
		t.Errorf("MaxFileSize = %d, want 10MB", config.MaxFileSize)
	}

	if config.IndexingConcurrency != 4 {
		t.Errorf("IndexingConcurrency = %d, want 4", config.IndexingConcurrency)
	}

	if len(config.ExcludePatterns) == 0 {
		t.Error("ExcludePatterns should not be empty")
	}

	if len(config.FileAssociations) == 0 {
		t.Error("FileAssociations should not be empty")
	}

	if config.EditorSettings.TabSize != 4 {
		t.Errorf("TabSize = %d, want 4", config.EditorSettings.TabSize)
	}
}

func TestLoadConfig_NotExists(t *testing.T) {
	tmpDir := t.TempDir()

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	// Should return default config
	if config.MaxFileSize != 10*1024*1024 {
		t.Error("Should return default config when file doesn't exist")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		ExcludePatterns:     []string{"**/test/**"},
		MaxFileSize:         5 * 1024 * 1024,
		IndexingConcurrency: 8,
		EditorSettings: EditorSettings{
			TabSize:      2,
			InsertSpaces: false,
		},
	}

	err := SaveConfig(tmpDir, config)
	if err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load and verify
	loaded, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if loaded.MaxFileSize != config.MaxFileSize {
		t.Errorf("MaxFileSize = %d, want %d", loaded.MaxFileSize, config.MaxFileSize)
	}

	if loaded.IndexingConcurrency != config.IndexingConcurrency {
		t.Errorf("IndexingConcurrency = %d, want %d", loaded.IndexingConcurrency, config.IndexingConcurrency)
	}

	if loaded.EditorSettings.TabSize != config.EditorSettings.TabSize {
		t.Errorf("TabSize = %d, want %d", loaded.EditorSettings.TabSize, config.EditorSettings.TabSize)
	}
}

func TestMergeConfig(t *testing.T) {
	base := &Config{
		ExcludePatterns:     []string{"**/.git/**"},
		MaxFileSize:         10 * 1024 * 1024,
		IndexingConcurrency: 4,
		FileAssociations:    map[string]string{"*.go": "go"},
		EditorSettings: EditorSettings{
			TabSize:      4,
			InsertSpaces: true,
		},
	}

	override := &Config{
		ExcludePatterns:     []string{"**/vendor/**"},
		MaxFileSize:         5 * 1024 * 1024,
		IndexingConcurrency: 0, // Should not override
		FileAssociations:    map[string]string{"*.ts": "typescript"},
		EditorSettings: EditorSettings{
			TabSize:      2,
			InsertSpaces: false,
		},
	}

	result := MergeConfig(base, override)

	// Exclude patterns should be merged
	if len(result.ExcludePatterns) != 2 {
		t.Errorf("Expected 2 exclude patterns, got %d", len(result.ExcludePatterns))
	}

	// MaxFileSize should be overridden
	if result.MaxFileSize != 5*1024*1024 {
		t.Errorf("MaxFileSize = %d, want 5MB", result.MaxFileSize)
	}

	// IndexingConcurrency should not be overridden (0 value)
	if result.IndexingConcurrency != 4 {
		t.Errorf("IndexingConcurrency = %d, want 4", result.IndexingConcurrency)
	}

	// FileAssociations should be merged
	if len(result.FileAssociations) != 2 {
		t.Errorf("Expected 2 file associations, got %d", len(result.FileAssociations))
	}

	// Editor settings should be overridden
	if result.EditorSettings.TabSize != 2 {
		t.Errorf("TabSize = %d, want 2", result.EditorSettings.TabSize)
	}
	if result.EditorSettings.InsertSpaces != false {
		t.Error("InsertSpaces should be false")
	}
}

func TestConfig_IsExcluded(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		path string
		want bool
	}{
		{"/project/.git/config", true},
		{"/project/node_modules/package/index.js", true},
		{"/project/src/main.go", false},
		{"/project/vendor/module/file.go", true},
		{"/project/.DS_Store", true},
	}

	for _, tt := range tests {
		got := config.IsExcluded(tt.path)
		if got != tt.want {
			t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestConfig_IsSearchExcluded(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		path string
		want bool
	}{
		{"/project/.git/config", true},       // From ExcludePatterns
		{"/project/bundle.min.js", true},     // From SearchExcludePatterns
		{"/project/package-lock.json", true}, // From SearchExcludePatterns
		{"/project/src/main.go", false},
	}

	for _, tt := range tests {
		got := config.IsSearchExcluded(tt.path)
		if got != tt.want {
			t.Errorf("IsSearchExcluded(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestConfig_GetLanguageID(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		path string
		want string
	}{
		{"/project/main.go", "go"},
		{"/project/app.ts", "typescript"},
		{"/project/Component.tsx", "typescriptreact"},
		{"/project/script.py", "python"},
		{"/project/Makefile", "makefile"},
		{"/project/Dockerfile", "dockerfile"},
		{"/project/unknown.xyz", ""},
	}

	for _, tt := range tests {
		got := config.GetLanguageID(tt.path)
		if got != tt.want {
			t.Errorf("GetLanguageID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestMatchGlobPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/.git/**", "/project/.git/config", true},
		{"**/.git/**", "/project/src/main.go", false},
		{"**/node_modules/**", "/project/node_modules/pkg/index.js", true},
		{"*.go", "main.go", true},
		{"*.go", "main.ts", false},
		{"src/**", "src/main.go", true},
		{"src/**", "lib/main.go", false},
		{"**/test/**", "/project/src/test/main_test.go", true},
		{"**/*.min.js", "/project/bundle.min.js", true},
		{"**/*.min.js", "/project/bundle.js", false},
	}

	for _, tt := range tests {
		got := matchGlobPattern(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("matchGlobPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestMergeStringSlices(t *testing.T) {
	base := []string{"a", "b", "c"}
	override := []string{"b", "d", "e"}

	result := mergeStringSlices(base, override)

	expected := []string{"a", "b", "c", "d", "e"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("mergeStringSlices = %v, want %v", result, expected)
	}
}

func TestMergeStringMaps(t *testing.T) {
	base := map[string]string{"a": "1", "b": "2"}
	override := map[string]string{"b": "3", "c": "4"}

	result := mergeStringMaps(base, override)

	if result["a"] != "1" {
		t.Errorf("result[a] = %q, want 1", result["a"])
	}
	if result["b"] != "3" { // Override wins
		t.Errorf("result[b] = %q, want 3", result["b"])
	}
	if result["c"] != "4" {
		t.Errorf("result[c] = %q, want 4", result["c"])
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"a/b/c", []string{"a", "b", "c"}},
		{"/a/b/c", []string{"a", "b", "c"}},
		{"a", []string{"a"}},
		{"", nil},
		{"a//b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		got := splitPath(tt.path)
		if len(got) == 0 && len(tt.want) == 0 {
			continue // Both empty, pass
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
