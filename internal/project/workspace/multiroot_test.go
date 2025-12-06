package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadWorkspaceFile(t *testing.T) {
	tmpDir := t.TempDir()
	wsFile := filepath.Join(tmpDir, "test.code-workspace")

	content := `{
		"folders": [
			{"path": "project1"},
			{"path": "project2", "name": "My Project"}
		],
		"settings": {
			"editor.tabSize": 2
		}
	}`

	if err := os.WriteFile(wsFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loaded, err := LoadWorkspaceFile(wsFile)
	if err != nil {
		t.Fatalf("LoadWorkspaceFile error: %v", err)
	}

	if len(loaded.Folders) != 2 {
		t.Errorf("Expected 2 folders, got %d", len(loaded.Folders))
	}

	if loaded.Folders[0].Path != "project1" {
		t.Errorf("Folder[0].Path = %q, want project1", loaded.Folders[0].Path)
	}

	if loaded.Folders[1].Name != "My Project" {
		t.Errorf("Folder[1].Name = %q, want 'My Project'", loaded.Folders[1].Name)
	}
}

func TestSaveWorkspaceFile(t *testing.T) {
	tmpDir := t.TempDir()
	wsFile := filepath.Join(tmpDir, "test.code-workspace")

	wsData := &WorkspaceFile{
		Folders: []WorkspaceFolderEntry{
			{Path: "project1"},
			{Path: "project2", Name: "Test"},
		},
		Settings: map[string]any{
			"editor.tabSize": 4,
		},
	}

	err := SaveWorkspaceFile(wsFile, wsData)
	if err != nil {
		t.Fatalf("SaveWorkspaceFile error: %v", err)
	}

	// Verify by loading
	loaded, err := LoadWorkspaceFile(wsFile)
	if err != nil {
		t.Fatalf("LoadWorkspaceFile error: %v", err)
	}

	if len(loaded.Folders) != 2 {
		t.Errorf("Expected 2 folders, got %d", len(loaded.Folders))
	}
}

func TestOpenFromWorkspaceFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project directories
	project1 := filepath.Join(tmpDir, "project1")
	project2 := filepath.Join(tmpDir, "project2")
	_ = os.MkdirAll(project1, 0o755)
	_ = os.MkdirAll(project2, 0o755)

	// Create workspace file
	wsFile := filepath.Join(tmpDir, "test.code-workspace")
	content := `{
		"folders": [
			{"path": "project1"},
			{"path": "project2", "name": "Custom Name"}
		],
		"settings": {
			"editor.tabSize": 2,
			"editor.insertSpaces": false
		}
	}`
	if err := os.WriteFile(wsFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write workspace file: %v", err)
	}

	ws, err := OpenFromWorkspaceFile(wsFile)
	if err != nil {
		t.Fatalf("OpenFromWorkspaceFile error: %v", err)
	}

	if len(ws.Folders()) != 2 {
		t.Errorf("Expected 2 folders, got %d", len(ws.Folders()))
	}

	folders := ws.Folders()

	// Check first folder
	if folders[0].Path != project1 {
		t.Errorf("Folder[0].Path = %q, want %q", folders[0].Path, project1)
	}

	// Check second folder has custom name
	if folders[1].Name != "Custom Name" {
		t.Errorf("Folder[1].Name = %q, want 'Custom Name'", folders[1].Name)
	}

	// Check settings were applied
	config := ws.Config()
	if config.EditorSettings.TabSize != 2 {
		t.Errorf("TabSize = %d, want 2", config.EditorSettings.TabSize)
	}
	if config.EditorSettings.InsertSpaces != false {
		t.Error("InsertSpaces should be false")
	}
}

func TestWorkspace_SaveToWorkspaceFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project directories
	project1 := filepath.Join(tmpDir, "project1")
	project2 := filepath.Join(tmpDir, "project2")
	_ = os.MkdirAll(project1, 0o755)
	_ = os.MkdirAll(project2, 0o755)

	ws, _ := NewFromPaths(project1, project2)

	// Save to workspace file
	wsFile := filepath.Join(tmpDir, "saved.code-workspace")
	err := ws.SaveToWorkspaceFile(wsFile)
	if err != nil {
		t.Fatalf("SaveToWorkspaceFile error: %v", err)
	}

	// Verify by loading
	loaded, err := LoadWorkspaceFile(wsFile)
	if err != nil {
		t.Fatalf("LoadWorkspaceFile error: %v", err)
	}

	if len(loaded.Folders) != 2 {
		t.Errorf("Expected 2 folders, got %d", len(loaded.Folders))
	}

	// Paths should be relative
	if loaded.Folders[0].Path != "project1" {
		t.Errorf("Folder[0].Path = %q, want project1", loaded.Folders[0].Path)
	}
}

func TestMultiRootManager_FindCommonRoot(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string // relative paths under tmpBase
		want    string   // expected suffix of common root
		wantNil bool
	}{
		{
			name:  "single folder",
			paths: []string{"home/user/project"},
			want:  "home/user/project",
		},
		{
			name:  "common parent",
			paths: []string{"home/user/project1", "home/user/project2"},
			want:  "home/user",
		},
		{
			name:  "nested common",
			paths: []string{"home/user/work/a", "home/user/work/b", "home/user/work/c"},
			want:  "home/user/work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directories
			tmpBase := t.TempDir()
			paths := make([]string, len(tt.paths))
			for i, p := range tt.paths {
				// Create path relative to temp dir (no leading slash)
				fullPath := filepath.Join(tmpBase, p)
				_ = os.MkdirAll(fullPath, 0o755)
				paths[i] = fullPath
			}

			ws, err := NewFromPaths(paths...)
			if err != nil {
				t.Fatalf("NewFromPaths error: %v", err)
			}

			mgr := NewMultiRootManager(ws)
			got := mgr.FindCommonRoot()

			if tt.wantNil && got != "" {
				t.Errorf("FindCommonRoot() = %q, want empty", got)
			}

			// Check the common root ends with expected suffix
			if !tt.wantNil && tt.want != "" {
				expectedSuffix := filepath.Join(tmpBase, tt.want)
				if got != expectedSuffix {
					t.Errorf("FindCommonRoot() = %q, want %q", got, expectedSuffix)
				}
			}
		})
	}
}

func TestMultiRootManager_SetPrimaryFolder(t *testing.T) {
	tmpDir := t.TempDir()
	project1 := filepath.Join(tmpDir, "project1")
	project2 := filepath.Join(tmpDir, "project2")
	_ = os.MkdirAll(project1, 0o755)
	_ = os.MkdirAll(project2, 0o755)

	ws, _ := NewFromPaths(project1, project2)
	mgr := NewMultiRootManager(ws)

	// Set project2 as primary
	err := mgr.SetPrimaryFolder(project2)
	if err != nil {
		t.Fatalf("SetPrimaryFolder error: %v", err)
	}

	if ws.Root() != project2 {
		t.Errorf("Root() = %q, want %q", ws.Root(), project2)
	}
}

func TestMultiRootManager_SetPrimaryFolder_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)
	mgr := NewMultiRootManager(ws)

	err := mgr.SetPrimaryFolder("/nonexistent")
	if err != ErrFolderNotFound {
		t.Errorf("Expected ErrFolderNotFound, got %v", err)
	}
}

func TestMultiRootManager_RenameFolder(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)
	mgr := NewMultiRootManager(ws)

	err := mgr.RenameFolder(tmpDir, "My Project")
	if err != nil {
		t.Fatalf("RenameFolder error: %v", err)
	}

	folders := ws.Folders()
	if folders[0].Name != "My Project" {
		t.Errorf("Name = %q, want 'My Project'", folders[0].Name)
	}
}

func TestMultiRootManager_ReorderFolders(t *testing.T) {
	tmpDir := t.TempDir()
	project1 := filepath.Join(tmpDir, "project1")
	project2 := filepath.Join(tmpDir, "project2")
	project3 := filepath.Join(tmpDir, "project3")
	_ = os.MkdirAll(project1, 0o755)
	_ = os.MkdirAll(project2, 0o755)
	_ = os.MkdirAll(project3, 0o755)

	ws, _ := NewFromPaths(project1, project2, project3)
	mgr := NewMultiRootManager(ws)

	// Reorder to 3, 1, 2
	err := mgr.ReorderFolders([]string{project3, project1, project2})
	if err != nil {
		t.Fatalf("ReorderFolders error: %v", err)
	}

	roots := ws.Roots()
	expected := []string{project3, project1, project2}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("Roots() = %v, want %v", roots, expected)
	}
}

func TestMultiRootManager_GetFoldersByName(t *testing.T) {
	tmpDir := t.TempDir()
	project1 := filepath.Join(tmpDir, "api-service")
	project2 := filepath.Join(tmpDir, "web-service")
	project3 := filepath.Join(tmpDir, "cli-tool")
	_ = os.MkdirAll(project1, 0o755)
	_ = os.MkdirAll(project2, 0o755)
	_ = os.MkdirAll(project3, 0o755)

	ws, _ := NewFromPaths(project1, project2, project3)
	mgr := NewMultiRootManager(ws)

	// Find all services
	matches := mgr.GetFoldersByName("*-service")
	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
	}
}

func TestApplySettingsToConfig(t *testing.T) {
	config := DefaultConfig()
	settings := map[string]any{
		"editor.tabSize":               float64(2),
		"editor.insertSpaces":          false,
		"files.trimTrailingWhitespace": true,
		"files.insertFinalNewline":     false,
		"files.encoding":               "utf-16",
		"files.eol":                    "\r\n",
		"files.exclude": map[string]any{
			"**/custom/**": true,
		},
		"files.associations": map[string]any{
			"*.myext": "javascript",
		},
	}

	applySettingsToConfig(config, settings)

	if config.EditorSettings.TabSize != 2 {
		t.Errorf("TabSize = %d, want 2", config.EditorSettings.TabSize)
	}

	if config.EditorSettings.InsertSpaces != false {
		t.Error("InsertSpaces should be false")
	}

	if config.EditorSettings.DefaultLineEnding != "crlf" {
		t.Errorf("DefaultLineEnding = %q, want crlf", config.EditorSettings.DefaultLineEnding)
	}

	if config.FileAssociations["*.myext"] != "javascript" {
		t.Error("File association not applied")
	}
}

func TestConfigToSettings(t *testing.T) {
	config := &Config{
		ExcludePatterns: []string{"**/.git/**"},
		EditorSettings: EditorSettings{
			TabSize:           4,
			InsertSpaces:      true,
			DefaultLineEnding: "lf",
		},
	}

	settings := configToSettings(config)

	if settings["editor.tabSize"] != 4 {
		t.Errorf("editor.tabSize = %v, want 4", settings["editor.tabSize"])
	}

	if settings["editor.insertSpaces"] != true {
		t.Error("editor.insertSpaces should be true")
	}

	if settings["files.eol"] != "\n" {
		t.Errorf("files.eol = %v, want \\n", settings["files.eol"])
	}

	exclude, ok := settings["files.exclude"].(map[string]any)
	if !ok {
		t.Error("files.exclude should be a map")
	} else if exclude["**/.git/**"] != true {
		t.Error("**/.git/** should be excluded")
	}
}

func TestSplitPathComponents(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/home/user/project", []string{"home", "user", "project"}},
		{"relative/path", []string{"relative", "path"}},
		{"/single", []string{"single"}},
	}

	for _, tt := range tests {
		got := splitPathComponents(tt.path)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitPathComponents(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestAppendIfNotExists(t *testing.T) {
	slice := []string{"a", "b", "c"}

	// Adding existing element
	result := appendIfNotExists(slice, "b")
	if len(result) != 3 {
		t.Errorf("Should not add duplicate: len = %d, want 3", len(result))
	}

	// Adding new element
	result = appendIfNotExists(slice, "d")
	if len(result) != 4 {
		t.Errorf("Should add new element: len = %d, want 4", len(result))
	}
}
