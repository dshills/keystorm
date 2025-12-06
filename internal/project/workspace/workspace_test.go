package workspace

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	ws := New()
	if ws == nil {
		t.Fatal("New() returned nil")
	}
	if len(ws.Folders()) != 0 {
		t.Errorf("New workspace should have 0 folders, got %d", len(ws.Folders()))
	}
	if ws.Config() == nil {
		t.Error("New workspace should have default config")
	}
}

func TestNewFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	ws, err := NewFromPath(tmpDir)
	if err != nil {
		t.Fatalf("NewFromPath error: %v", err)
	}

	if len(ws.Folders()) != 1 {
		t.Errorf("Expected 1 folder, got %d", len(ws.Folders()))
	}

	absPath, _ := filepath.Abs(tmpDir)
	if ws.Root() != absPath {
		t.Errorf("Root() = %q, want %q", ws.Root(), absPath)
	}
}

func TestNewFromPaths(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	ws, err := NewFromPaths(tmpDir1, tmpDir2)
	if err != nil {
		t.Fatalf("NewFromPaths error: %v", err)
	}

	if len(ws.Folders()) != 2 {
		t.Errorf("Expected 2 folders, got %d", len(ws.Folders()))
	}

	if !ws.IsMultiRoot() {
		t.Error("IsMultiRoot() should be true")
	}

	roots := ws.Roots()
	if len(roots) != 2 {
		t.Errorf("Roots() returned %d paths, want 2", len(roots))
	}
}

func TestNewFromPaths_NoFolders(t *testing.T) {
	_, err := NewFromPaths()
	if err != ErrNoFolders {
		t.Errorf("Expected ErrNoFolders, got %v", err)
	}
}

func TestWorkspace_Open(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New()

	err := ws.Open(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	if len(ws.Folders()) != 1 {
		t.Errorf("Expected 1 folder after Open, got %d", len(ws.Folders()))
	}
}

func TestWorkspace_Open_NoFolders(t *testing.T) {
	ws := New()
	err := ws.Open(context.Background())
	if err != ErrNoFolders {
		t.Errorf("Expected ErrNoFolders, got %v", err)
	}
}

func TestWorkspace_Close(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)

	err := ws.Close(context.Background())
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if !ws.IsClosed() {
		t.Error("IsClosed() should be true after Close")
	}

	if ws.Root() != "" {
		t.Error("Root() should be empty after Close")
	}
}

func TestWorkspace_AddFolder(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	ws, _ := NewFromPath(tmpDir1)

	err := ws.AddFolder(context.Background(), tmpDir2)
	if err != nil {
		t.Fatalf("AddFolder error: %v", err)
	}

	if len(ws.Folders()) != 2 {
		t.Errorf("Expected 2 folders, got %d", len(ws.Folders()))
	}

	if !ws.IsMultiRoot() {
		t.Error("IsMultiRoot() should be true after adding second folder")
	}
}

func TestWorkspace_AddFolder_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)

	err := ws.AddFolder(context.Background(), tmpDir)
	if err != ErrFolderExists {
		t.Errorf("Expected ErrFolderExists, got %v", err)
	}
}

func TestWorkspace_AddFolder_Callback(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	ws, _ := NewFromPath(tmpDir1)

	var addedFolder Folder
	ws.OnFolderAdd(func(f Folder) {
		addedFolder = f
	})

	_ = ws.AddFolder(context.Background(), tmpDir2)

	absPath, _ := filepath.Abs(tmpDir2)
	if addedFolder.Path != absPath {
		t.Errorf("Callback received wrong folder: %q, want %q", addedFolder.Path, absPath)
	}
}

func TestWorkspace_RemoveFolder(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	ws, _ := NewFromPaths(tmpDir1, tmpDir2)

	err := ws.RemoveFolder(context.Background(), tmpDir2)
	if err != nil {
		t.Fatalf("RemoveFolder error: %v", err)
	}

	if len(ws.Folders()) != 1 {
		t.Errorf("Expected 1 folder, got %d", len(ws.Folders()))
	}
}

func TestWorkspace_RemoveFolder_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)

	err := ws.RemoveFolder(context.Background(), "/nonexistent")
	if err != ErrFolderNotFound {
		t.Errorf("Expected ErrFolderNotFound, got %v", err)
	}
}

func TestWorkspace_RemoveFolder_Callback(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	ws, _ := NewFromPaths(tmpDir1, tmpDir2)

	var removedFolder Folder
	ws.OnFolderRemove(func(f Folder) {
		removedFolder = f
	})

	_ = ws.RemoveFolder(context.Background(), tmpDir2)

	absPath, _ := filepath.Abs(tmpDir2)
	if removedFolder.Path != absPath {
		t.Errorf("Callback received wrong folder: %q, want %q", removedFolder.Path, absPath)
	}
}

func TestWorkspace_GetFolder(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)

	folder, ok := ws.GetFolder(tmpDir)
	if !ok {
		t.Error("GetFolder should return true for existing folder")
	}

	absPath, _ := filepath.Abs(tmpDir)
	if folder.Path != absPath {
		t.Errorf("Folder.Path = %q, want %q", folder.Path, absPath)
	}
}

func TestWorkspace_GetFolder_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)

	_, ok := ws.GetFolder("/nonexistent")
	if ok {
		t.Error("GetFolder should return false for nonexistent folder")
	}
}

func TestWorkspace_IsInWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	_ = os.MkdirAll(subDir, 0o755)

	ws, _ := NewFromPath(tmpDir)

	if !ws.IsInWorkspace(subDir) {
		t.Error("IsInWorkspace should return true for subdirectory")
	}

	if ws.IsInWorkspace("/some/other/path") {
		t.Error("IsInWorkspace should return false for external path")
	}
}

func TestWorkspace_ContainingFolder(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	_ = os.MkdirAll(subDir, 0o755)

	ws, _ := NewFromPath(tmpDir)

	folder, ok := ws.ContainingFolder(subDir)
	if !ok {
		t.Error("ContainingFolder should find folder")
	}

	absPath, _ := filepath.Abs(tmpDir)
	if folder.Path != absPath {
		t.Errorf("ContainingFolder = %q, want %q", folder.Path, absPath)
	}
}

func TestWorkspace_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	subFile := filepath.Join(tmpDir, "subdir", "file.go")
	_ = os.MkdirAll(filepath.Dir(subFile), 0o755)

	ws, _ := NewFromPath(tmpDir)

	relPath, err := ws.RelativePath(subFile)
	if err != nil {
		t.Fatalf("RelativePath error: %v", err)
	}

	expected := filepath.Join("subdir", "file.go")
	if relPath != expected {
		t.Errorf("RelativePath = %q, want %q", relPath, expected)
	}
}

func TestWorkspace_SetConfig(t *testing.T) {
	ws := New()

	newConfig := &Config{
		MaxFileSize: 5 * 1024 * 1024,
	}

	var changeEvent ChangeEvent
	ws.OnChange(func(e ChangeEvent) {
		changeEvent = e
	})

	ws.SetConfig(newConfig)

	if ws.Config().MaxFileSize != 5*1024*1024 {
		t.Error("Config not updated")
	}

	if changeEvent.Type != ChangeConfigUpdated {
		t.Errorf("ChangeEvent.Type = %v, want ChangeConfigUpdated", changeEvent.Type)
	}
}

func TestWorkspace_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ws.Root()
			_ = ws.Roots()
			_ = ws.Folders()
			_ = ws.IsMultiRoot()
			_ = ws.FolderCount()
			_ = ws.IsInWorkspace(tmpDir)
		}()
	}
	wg.Wait()
}

func TestWorkspace_OperationsOnClosed(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := NewFromPath(tmpDir)
	_ = ws.Close(context.Background())

	err := ws.AddFolder(context.Background(), tmpDir)
	if err != ErrWorkspaceClosed {
		t.Errorf("AddFolder on closed workspace: expected ErrWorkspaceClosed, got %v", err)
	}

	err = ws.RemoveFolder(context.Background(), tmpDir)
	if err != ErrWorkspaceClosed {
		t.Errorf("RemoveFolder on closed workspace: expected ErrWorkspaceClosed, got %v", err)
	}

	err = ws.Open(context.Background(), tmpDir)
	if err != ErrWorkspaceClosed {
		t.Errorf("Open on closed workspace: expected ErrWorkspaceClosed, got %v", err)
	}
}

func TestPathToURI(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/project", "file:///home/user/project"},
		{"/path/with spaces/file.go", "file:///path/with%20spaces/file.go"},
	}

	for _, tt := range tests {
		got := PathToURI(tt.path)
		if got != tt.want {
			t.Errorf("PathToURI(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		uri     string
		want    string
		wantErr bool
	}{
		{"file:///home/user/project", "/home/user/project", false},
		{"file:///path/with%20spaces/file.go", "/path/with spaces/file.go", false},
		{"https://example.com", "", true}, // Invalid scheme
	}

	for _, tt := range tests {
		got, err := URIToPath(tt.uri)
		if (err != nil) != tt.wantErr {
			t.Errorf("URIToPath(%q) error = %v, wantErr %v", tt.uri, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("URIToPath(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		parent string
		child  string
		want   bool
	}{
		{"/home/user", "/home/user/project", true},
		{"/home/user", "/home/user", true},
		{"/home/user", "/home/other", false},
		{"/home/user", "/home/username", false},
		{"/a/b", "/a/b/c/d", true},
		{"/a/b/c", "/a/b", false},
	}

	for _, tt := range tests {
		got := isSubPath(tt.parent, tt.child)
		if got != tt.want {
			t.Errorf("isSubPath(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.want)
		}
	}
}
