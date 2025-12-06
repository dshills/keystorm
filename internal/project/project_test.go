package project

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/project/graph"
	"github.com/dshills/keystorm/internal/project/vfs"
	"github.com/dshills/keystorm/internal/project/workspace"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.IsOpen() {
		t.Error("New project should not be open")
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := Config{
		MaxFileSize:  1024,
		IndexWorkers: 2,
	}
	p := New(WithConfig(cfg))
	if p.config.MaxFileSize != 1024 {
		t.Errorf("MaxFileSize = %d, want 1024", p.config.MaxFileSize)
	}
	if p.config.IndexWorkers != 2 {
		t.Errorf("IndexWorkers = %d, want 2", p.config.IndexWorkers)
	}
}

func TestNewWithVFS(t *testing.T) {
	memfs := vfs.NewMemFS()
	p := New(WithVFS(memfs))
	if p.vfs != memfs {
		t.Error("VFS was not set correctly")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxFileSize <= 0 {
		t.Error("DefaultConfig MaxFileSize should be positive")
	}
	if cfg.IndexWorkers <= 0 {
		t.Error("DefaultConfig IndexWorkers should be positive")
	}
	if len(cfg.ExcludePatterns) == 0 {
		t.Error("DefaultConfig should have exclude patterns")
	}
}

func TestProject_OpenClose(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/testroot", 0755)
	_ = memfs.WriteFile("/testroot/file.txt", []byte("hello"), 0644)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))

	// Open should succeed
	ctx := context.Background()
	err := p.Open(ctx, "/testroot")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if !p.IsOpen() {
		t.Error("IsOpen() should return true after Open()")
	}

	if p.Root() != "/testroot" {
		t.Errorf("Root() = %q, want /testroot", p.Root())
	}

	// Open again should fail
	err = p.Open(ctx, "/testroot")
	if err != ErrAlreadyOpen {
		t.Errorf("Second Open() error = %v, want ErrAlreadyOpen", err)
	}

	// Close should succeed
	err = p.Close(ctx)
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if p.IsOpen() {
		t.Error("IsOpen() should return false after Close()")
	}

	// Close again should fail
	err = p.Close(ctx)
	if err != ErrNotOpen {
		t.Errorf("Second Close() error = %v, want ErrNotOpen", err)
	}
}

func TestProject_OpenNoFolders(t *testing.T) {
	p := New()
	err := p.Open(context.Background())
	if err != workspace.ErrNoFolders {
		t.Errorf("Open() with no folders error = %v, want ErrNoFolders", err)
	}
}

func TestProject_MultiRoot(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/root1", 0755)
	_ = memfs.Mkdir("/root2", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))

	ctx := context.Background()
	err := p.Open(ctx, "/root1", "/root2")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	roots := p.Roots()
	if len(roots) != 2 {
		t.Errorf("Roots() len = %d, want 2", len(roots))
	}

	_ = p.Close(ctx)
}

func TestProject_IsInWorkspace(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)
	_ = memfs.Mkdir("/workspace/src", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))

	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	tests := []struct {
		path string
		want bool
	}{
		{"/workspace/src/file.go", true},
		{"/workspace/file.txt", true},
		{"/other/file.txt", false},
		{"/workspaceother/file.txt", false},
	}

	for _, tt := range tests {
		if got := p.IsInWorkspace(tt.path); got != tt.want {
			t.Errorf("IsInWorkspace(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}

	_ = p.Close(ctx)
}

func TestProject_FileOperations(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)
	_ = memfs.WriteFile("/workspace/existing.txt", []byte("content"), 0644)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	// OpenFile
	doc, err := p.OpenFile(ctx, "/workspace/existing.txt")
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if doc == nil {
		t.Fatal("OpenFile() returned nil document")
	}

	// GetDocument
	doc2, ok := p.GetDocument("/workspace/existing.txt")
	if !ok {
		t.Error("GetDocument() should return true for open file")
	}
	if doc2 != doc {
		t.Error("GetDocument() should return same document")
	}

	// OpenDocuments
	docs := p.OpenDocuments()
	if len(docs) != 1 {
		t.Errorf("OpenDocuments() len = %d, want 1", len(docs))
	}

	// IsDirty - should be false initially
	if p.IsDirty("/workspace/existing.txt") {
		t.Error("IsDirty() should be false for unchanged file")
	}

	// CloseFile
	err = p.CloseFile(ctx, "/workspace/existing.txt")
	if err != nil {
		t.Errorf("CloseFile() error = %v", err)
	}

	_ = p.Close(ctx)
}

func TestProject_CreateDeleteFile(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	// CreateFile
	err := p.CreateFile(ctx, "/workspace/new.txt", []byte("new content"))
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}

	// Verify file exists
	if !memfs.Exists("/workspace/new.txt") {
		t.Error("Created file should exist")
	}

	// CreateFile duplicate should fail
	err = p.CreateFile(ctx, "/workspace/new.txt", []byte("duplicate"))
	if err == nil {
		t.Error("CreateFile() on existing file should error")
	}

	// DeleteFile
	err = p.DeleteFile(ctx, "/workspace/new.txt")
	if err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}

	if memfs.Exists("/workspace/new.txt") {
		t.Error("Deleted file should not exist")
	}

	_ = p.Close(ctx)
}

func TestProject_RenameFile(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)
	_ = memfs.WriteFile("/workspace/old.txt", []byte("content"), 0644)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	err := p.RenameFile(ctx, "/workspace/old.txt", "/workspace/new.txt")
	if err != nil {
		t.Fatalf("RenameFile() error = %v", err)
	}

	if memfs.Exists("/workspace/old.txt") {
		t.Error("Old file should not exist after rename")
	}
	if !memfs.Exists("/workspace/new.txt") {
		t.Error("New file should exist after rename")
	}

	_ = p.Close(ctx)
}

func TestProject_DirectoryOperations(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	// CreateDirectory
	err := p.CreateDirectory(ctx, "/workspace/newdir")
	if err != nil {
		t.Fatalf("CreateDirectory() error = %v", err)
	}

	if !memfs.IsDir("/workspace/newdir") {
		t.Error("Created directory should exist")
	}

	// Create a file in the directory
	_ = memfs.WriteFile("/workspace/newdir/file.txt", []byte("content"), 0644)

	// ListDirectory
	entries, err := p.ListDirectory(ctx, "/workspace/newdir")
	if err != nil {
		t.Fatalf("ListDirectory() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("ListDirectory() len = %d, want 1", len(entries))
	}

	// DeleteDirectory recursive
	err = p.DeleteDirectory(ctx, "/workspace/newdir", true)
	if err != nil {
		t.Fatalf("DeleteDirectory() error = %v", err)
	}

	if memfs.Exists("/workspace/newdir") {
		t.Error("Deleted directory should not exist")
	}

	_ = p.Close(ctx)
}

func TestProject_NotOpenErrors(t *testing.T) {
	p := New()
	ctx := context.Background()

	// All operations should fail when not open
	_, err := p.OpenFile(ctx, "/test.txt")
	if err != ErrNotOpen {
		t.Errorf("OpenFile() when closed error = %v, want ErrNotOpen", err)
	}

	err = p.SaveFile(ctx, "/test.txt")
	if err != ErrNotOpen {
		t.Errorf("SaveFile() when closed error = %v, want ErrNotOpen", err)
	}

	err = p.CloseFile(ctx, "/test.txt")
	if err != ErrNotOpen {
		t.Errorf("CloseFile() when closed error = %v, want ErrNotOpen", err)
	}

	err = p.CreateFile(ctx, "/test.txt", nil)
	if err != ErrNotOpen {
		t.Errorf("CreateFile() when closed error = %v, want ErrNotOpen", err)
	}

	err = p.DeleteFile(ctx, "/test.txt")
	if err != ErrNotOpen {
		t.Errorf("DeleteFile() when closed error = %v, want ErrNotOpen", err)
	}

	_, err = p.FindFiles(ctx, "test", FindOptions{})
	if err != ErrNotOpen {
		t.Errorf("FindFiles() when closed error = %v, want ErrNotOpen", err)
	}

	_, err = p.SearchContent(ctx, "test", SearchOptions{})
	if err != ErrNotOpen {
		t.Errorf("SearchContent() when closed error = %v, want ErrNotOpen", err)
	}
}

func TestProject_Graph(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = true

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	g := p.Graph()
	if g == nil {
		t.Error("Graph() should not return nil when graph is enabled")
	}

	_ = p.Close(ctx)
}

func TestProject_IndexStatus(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	status := p.IndexStatus()
	if status.Status == "" {
		t.Error("IndexStatus().Status should not be empty")
	}

	_ = p.Close(ctx)
}

func TestProject_WatcherStatus(t *testing.T) {
	p := New()

	// Before opening, watcher status should be empty
	status := p.WatcherStatus()
	// Just check it doesn't panic
	_ = status
}

func TestProject_EventHandlers(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))

	var fileChangeCalled bool
	var workspaceChangeCalled bool

	p.OnFileChange(func(e FileChangeEvent) {
		fileChangeCalled = true
	})
	p.OnWorkspaceChange(func(e workspace.ChangeEvent) {
		workspaceChangeCalled = true
	})

	// Handlers should be registered
	if len(p.fileChangeHandlers) != 1 {
		t.Error("OnFileChange handler not registered")
	}
	if len(p.workspaceChangeHandlers) != 1 {
		t.Error("OnWorkspaceChange handler not registered")
	}

	_ = fileChangeCalled
	_ = workspaceChangeCalled
}

func TestProject_Workspace(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	ws := p.Workspace()
	if ws == nil {
		t.Fatal("Workspace() should not return nil when open")
	}

	if ws.Root() != "/workspace" {
		t.Errorf("Workspace().Root() = %q, want /workspace", ws.Root())
	}

	_ = p.Close(ctx)

	// After close, workspace should be nil
	ws = p.Workspace()
	if ws != nil {
		t.Error("Workspace() should return nil after close")
	}
}

func TestFileChangeType(t *testing.T) {
	types := []FileChangeType{
		FileChangeCreated,
		FileChangeModified,
		FileChangeDeleted,
		FileChangeRenamed,
	}

	for i, typ := range types {
		if int(typ) != i {
			t.Errorf("FileChangeType %d has wrong value %d", i, typ)
		}
	}
}

func TestIndexStatus_Fields(t *testing.T) {
	status := IndexStatus{
		Status:         "indexing",
		TotalFiles:     100,
		IndexedFiles:   50,
		ErrorFiles:     5,
		BytesProcessed: 1024,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
	}

	if status.Status != "indexing" {
		t.Error("Status field mismatch")
	}
	if status.TotalFiles != 100 {
		t.Error("TotalFiles field mismatch")
	}
}

func TestWatcherStatus_Fields(t *testing.T) {
	status := WatcherStatus{
		WatchedPaths:  10,
		PendingEvents: 5,
		TotalEvents:   100,
		Errors:        2,
	}

	if status.WatchedPaths != 10 {
		t.Error("WatchedPaths field mismatch")
	}
}

func TestFileMatch_Fields(t *testing.T) {
	match := FileMatch{
		Path:  "/test/file.go",
		Name:  "file.go",
		Score: 0.95,
	}

	if match.Path != "/test/file.go" {
		t.Error("Path field mismatch")
	}
	if match.Name != "file.go" {
		t.Error("Name field mismatch")
	}
	if match.Score != 0.95 {
		t.Error("Score field mismatch")
	}
}

func TestContentMatch_Fields(t *testing.T) {
	match := ContentMatch{
		Path:   "/test/file.go",
		Line:   10,
		Column: 5,
		Text:   "test content",
	}

	if match.Path != "/test/file.go" {
		t.Error("Path field mismatch")
	}
	if match.Line != 10 {
		t.Error("Line field mismatch")
	}
}

func TestRelatedFile_Fields(t *testing.T) {
	related := RelatedFile{
		Path:         "/test/related.go",
		Relationship: "imports",
		Score:        0.8,
	}

	if related.Path != "/test/related.go" {
		t.Error("Path field mismatch")
	}
	if related.Relationship != "imports" {
		t.Error("Relationship field mismatch")
	}
}

func TestFileChangeEvent_Fields(t *testing.T) {
	event := FileChangeEvent{
		Type:      FileChangeModified,
		Path:      "/test/file.go",
		OldPath:   "",
		Timestamp: time.Now(),
	}

	if event.Type != FileChangeModified {
		t.Error("Type field mismatch")
	}
	if event.Path != "/test/file.go" {
		t.Error("Path field mismatch")
	}
}

func TestProject_RelatedFiles(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)
	_ = memfs.WriteFile("/workspace/main.go", []byte("package main"), 0644)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = true

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	// Give the graph builder a moment
	time.Sleep(50 * time.Millisecond)

	// Test RelatedFiles - should not error even if no relations
	related, err := p.RelatedFiles(ctx, "/workspace/main.go")
	if err != nil {
		t.Errorf("RelatedFiles() error = %v", err)
	}
	// May be empty, that's fine
	_ = related

	_ = p.Close(ctx)
}

func TestProject_RelatedFilesNotOpen(t *testing.T) {
	p := New()
	ctx := context.Background()

	_, err := p.RelatedFiles(ctx, "/test.go")
	if err != ErrNotOpen {
		t.Errorf("RelatedFiles() when closed error = %v, want ErrNotOpen", err)
	}
}

func TestProject_RelatedFilesNoGraph(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")

	related, err := p.RelatedFiles(ctx, "/workspace/main.go")
	if err != nil {
		t.Errorf("RelatedFiles() error = %v", err)
	}
	if related != nil {
		t.Error("RelatedFiles() should return nil when graph is disabled")
	}

	_ = p.Close(ctx)
}

// Test that DefaultProject implements the Project interface
func TestDefaultProject_ImplementsProject(t *testing.T) {
	var _ Project = (*DefaultProject)(nil)
}

// Benchmark tests
func BenchmarkProject_OpenClose(b *testing.B) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	for i := 0; i < b.N; i++ {
		p := New(WithVFS(memfs), WithConfig(cfg))
		ctx := context.Background()
		_ = p.Open(ctx, "/workspace")
		_ = p.Close(ctx)
	}
}

func BenchmarkProject_OpenFile(b *testing.B) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)
	_ = memfs.WriteFile("/workspace/test.txt", []byte("content"), 0644)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")
	defer func() { _ = p.Close(ctx) }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, _ := p.OpenFile(ctx, "/workspace/test.txt")
		if doc != nil {
			_ = p.CloseFile(ctx, "/workspace/test.txt")
		}
	}
}

// Test path validation - operations outside workspace should fail
func TestProject_PathValidation(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)
	_ = memfs.WriteFile("/workspace/file.txt", []byte("hello"), 0644)
	_ = memfs.Mkdir("/outside", 0755)
	_ = memfs.WriteFile("/outside/file.txt", []byte("outside"), 0644)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")
	defer func() { _ = p.Close(ctx) }()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"OpenFile", func() error {
			_, err := p.OpenFile(ctx, "/outside/file.txt")
			return err
		}},
		{"SaveFile", func() error {
			return p.SaveFile(ctx, "/outside/file.txt")
		}},
		{"CloseFile", func() error {
			return p.CloseFile(ctx, "/outside/file.txt")
		}},
		{"ReloadFile", func() error {
			return p.ReloadFile(ctx, "/outside/file.txt")
		}},
		{"CreateFile", func() error {
			return p.CreateFile(ctx, "/outside/newfile.txt", []byte("content"))
		}},
		{"DeleteFile", func() error {
			return p.DeleteFile(ctx, "/outside/file.txt")
		}},
		{"RenameFile_OldPath", func() error {
			return p.RenameFile(ctx, "/outside/file.txt", "/workspace/renamed.txt")
		}},
		{"RenameFile_NewPath", func() error {
			return p.RenameFile(ctx, "/workspace/file.txt", "/outside/renamed.txt")
		}},
		{"SaveFileAs_OldPath", func() error {
			return p.SaveFileAs(ctx, "/outside/file.txt", "/workspace/copy.txt")
		}},
		{"SaveFileAs_NewPath", func() error {
			return p.SaveFileAs(ctx, "/workspace/file.txt", "/outside/copy.txt")
		}},
		{"CreateDirectory", func() error {
			return p.CreateDirectory(ctx, "/outside/newdir")
		}},
		{"DeleteDirectory", func() error {
			return p.DeleteDirectory(ctx, "/outside", false)
		}},
		{"ListDirectory", func() error {
			_, err := p.ListDirectory(ctx, "/outside")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !IsNotInWorkspace(err) {
				t.Errorf("%s: expected ErrNotInWorkspace, got %v", tt.name, err)
			}
		})
	}
}

// Test that paths inside workspace work correctly
func TestProject_PathValidation_Inside(t *testing.T) {
	memfs := vfs.NewMemFS()
	_ = memfs.Mkdir("/workspace", 0755)

	cfg := DefaultConfig()
	cfg.EnableContentIndex = false
	cfg.EnableGraph = false

	p := New(WithVFS(memfs), WithConfig(cfg))
	ctx := context.Background()
	_ = p.Open(ctx, "/workspace")
	defer func() { _ = p.Close(ctx) }()

	// Create file inside workspace should succeed
	err := p.CreateFile(ctx, "/workspace/newfile.txt", []byte("content"))
	if err != nil {
		t.Errorf("CreateFile inside workspace: unexpected error %v", err)
	}

	// Create directory inside workspace should succeed
	err = p.CreateDirectory(ctx, "/workspace/newdir")
	if err != nil {
		t.Errorf("CreateDirectory inside workspace: unexpected error %v", err)
	}

	// List directory inside workspace should succeed
	_, err = p.ListDirectory(ctx, "/workspace")
	if err != nil {
		t.Errorf("ListDirectory inside workspace: unexpected error %v", err)
	}
}

// Test helper to create a test graph
func createTestGraph() graph.Graph {
	g := graph.New()
	return g
}
