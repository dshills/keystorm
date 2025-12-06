package index

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewIncrementalIndexer(t *testing.T) {
	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	config := DefaultIncrementalConfig()

	ii := NewIncrementalIndexer(fileIndex, contentIndex, config)
	if ii == nil {
		t.Fatal("NewIncrementalIndexer returned nil")
	}

	if ii.Status() != IndexStatusIdle {
		t.Errorf("Initial status = %v, want IndexStatusIdle", ii.Status())
	}
}

func TestNewIncrementalIndexer_DefaultWorkers(t *testing.T) {
	config := IncrementalConfig{Workers: 0} // Should default to 4
	ii := NewIncrementalIndexer(NewFileIndex(), nil, config)

	if ii.workers != 4 {
		t.Errorf("workers = %d, want 4 (default)", ii.workers)
	}
}

func TestDefaultIncrementalConfig(t *testing.T) {
	config := DefaultIncrementalConfig()

	if config.Workers != 4 {
		t.Errorf("Workers = %d, want 4", config.Workers)
	}
	if config.MaxFileSize != 10*1024*1024 {
		t.Errorf("MaxFileSize = %d, want 10MB", config.MaxFileSize)
	}
	if !config.IndexContent {
		t.Error("IndexContent should be true by default")
	}
	if config.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", config.BatchSize)
	}
	if len(config.ExcludePatterns) == 0 {
		t.Error("ExcludePatterns should have defaults")
	}
}

func TestIndexStatus_String(t *testing.T) {
	tests := []struct {
		status IndexStatus
		want   string
	}{
		{IndexStatusIdle, "idle"},
		{IndexStatusIndexing, "indexing"},
		{IndexStatusError, "error"},
		{IndexStatusStopped, "stopped"},
		{IndexStatus(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("IndexStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestIndexProgress_Copy(t *testing.T) {
	p := &IndexProgress{
		TotalFiles:     100,
		IndexedFiles:   50,
		ErrorFiles:     5,
		BytesProcessed: 1024000,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
		CurrentFile:    "/path/to/file.go",
	}

	copy := p.Copy()

	if copy.TotalFiles != p.TotalFiles {
		t.Errorf("TotalFiles = %d, want %d", copy.TotalFiles, p.TotalFiles)
	}
	if copy.IndexedFiles != p.IndexedFiles {
		t.Errorf("IndexedFiles = %d, want %d", copy.IndexedFiles, p.IndexedFiles)
	}
	if copy.CurrentFile != p.CurrentFile {
		t.Errorf("CurrentFile = %q, want %q", copy.CurrentFile, p.CurrentFile)
	}
}

func TestIndexProgress_PercentComplete(t *testing.T) {
	tests := []struct {
		total   int
		indexed int
		want    float64
	}{
		{0, 0, 0},
		{100, 50, 50},
		{100, 100, 100},
		{10, 3, 30},
	}

	for _, tt := range tests {
		p := &IndexProgress{
			TotalFiles:   tt.total,
			IndexedFiles: tt.indexed,
		}
		if got := p.PercentComplete(); got != tt.want {
			t.Errorf("PercentComplete() with %d/%d = %f, want %f", tt.indexed, tt.total, got, tt.want)
		}
	}
}

func TestIncrementalIndexer_Start(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some test files
	for i := 0; i < 5; i++ {
		file := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".go")
		content := "package main\n\nfunc main() {}\n"
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	config := DefaultIncrementalConfig()
	config.Workers = 2

	ii := NewIncrementalIndexer(fileIndex, contentIndex, config)

	// Track events
	var events []IndexEventType
	var mu sync.Mutex
	ii.OnEvent(func(e IndexEvent) {
		mu.Lock()
		events = append(events, e.Type)
		mu.Unlock()
	})

	err = ii.Start(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for indexing to complete
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for indexing to complete")
		default:
			if ii.Status() == IndexStatusIdle {
				goto done
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
done:

	// Check events
	mu.Lock()
	defer mu.Unlock()

	hasStarted := false
	hasCompleted := false
	for _, e := range events {
		if e == IndexEventStarted {
			hasStarted = true
		}
		if e == IndexEventCompleted {
			hasCompleted = true
		}
	}

	if !hasStarted {
		t.Error("Should have emitted IndexEventStarted")
	}
	if !hasCompleted {
		t.Error("Should have emitted IndexEventCompleted")
	}

	// Check files were indexed
	if fileIndex.Count() != 5 {
		t.Errorf("File index count = %d, want 5", fileIndex.Count())
	}
	if contentIndex.DocumentCount() != 5 {
		t.Errorf("Content index document count = %d, want 5", contentIndex.DocumentCount())
	}
}

func TestIncrementalIndexer_Start_AlreadyRunning(t *testing.T) {
	ii := NewIncrementalIndexer(NewFileIndex(), nil, DefaultIncrementalConfig())
	ii.status.Store(int32(IndexStatusIndexing))

	err := ii.Start(context.Background())
	if err != ErrIndexerBusy {
		t.Errorf("Start() error = %v, want ErrIndexerBusy", err)
	}
}

func TestIncrementalIndexer_Start_Stopped(t *testing.T) {
	ii := NewIncrementalIndexer(NewFileIndex(), nil, DefaultIncrementalConfig())
	ii.status.Store(int32(IndexStatusStopped))

	err := ii.Start(context.Background())
	if err != ErrIndexerStopped {
		t.Errorf("Start() error = %v, want ErrIndexerStopped", err)
	}
}

func TestIncrementalIndexer_Stop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create many files to ensure indexing takes time
	for i := 0; i < 100; i++ {
		file := filepath.Join(tmpDir, "file"+string(rune('0'+i%10))+string(rune('0'+i/10))+".go")
		if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	ii := NewIncrementalIndexer(NewFileIndex(), nil, DefaultIncrementalConfig())

	_ = ii.Start(context.Background(), tmpDir)
	ii.Stop()

	if ii.Status() != IndexStatusStopped {
		t.Errorf("Status after Stop() = %v, want IndexStatusStopped", ii.Status())
	}
}

func TestIncrementalIndexer_ProcessChange_Created(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file
	file := filepath.Join(tmpDir, "new.go")
	if err := os.WriteFile(file, []byte("package main\n\nfunc hello() {}"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	ii := NewIncrementalIndexer(fileIndex, contentIndex, DefaultIncrementalConfig())

	err = ii.ProcessChange(FileChangeEvent{
		Type:      FileChangeCreated,
		Path:      file,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("ProcessChange() error = %v", err)
	}

	if !fileIndex.Has(file) {
		t.Error("File should be in file index after ProcessChange")
	}
	if !contentIndex.HasDocument(file) {
		t.Error("File should be in content index after ProcessChange")
	}
}

func TestIncrementalIndexer_ProcessChange_Modified(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file := filepath.Join(tmpDir, "modified.go")
	if err := os.WriteFile(file, []byte("package main\n\nfunc old() {}"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	ii := NewIncrementalIndexer(fileIndex, contentIndex, DefaultIncrementalConfig())

	// Initial index
	_ = ii.ProcessChange(FileChangeEvent{Type: FileChangeCreated, Path: file})

	// Verify old content is indexed
	results, _ := contentIndex.Search(context.Background(), "old", ContentSearchOptions{})
	if len(results) != 1 {
		t.Errorf("Initial search for 'old' returned %d results, want 1", len(results))
	}

	// Modify file
	if err := os.WriteFile(file, []byte("package main\n\nfunc newfunction() {}"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Process modification
	err = ii.ProcessChange(FileChangeEvent{
		Type:      FileChangeModified,
		Path:      file,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("ProcessChange() error = %v", err)
	}

	// Old content should be gone
	results, _ = contentIndex.Search(context.Background(), "old", ContentSearchOptions{})
	if len(results) != 0 {
		t.Errorf("Search for 'old' after modify returned %d results, want 0", len(results))
	}

	// New content should be indexed
	results, _ = contentIndex.Search(context.Background(), "newfunction", ContentSearchOptions{})
	if len(results) != 1 {
		t.Errorf("Search for 'newfunction' returned %d results, want 1", len(results))
	}
}

func TestIncrementalIndexer_ProcessChange_Deleted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file := filepath.Join(tmpDir, "deleted.go")
	if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	ii := NewIncrementalIndexer(fileIndex, contentIndex, DefaultIncrementalConfig())

	// Initial index
	_ = ii.ProcessChange(FileChangeEvent{Type: FileChangeCreated, Path: file})

	// Verify indexed
	if !fileIndex.Has(file) {
		t.Error("File should be in index before delete")
	}

	// Delete file from disk
	os.Remove(file)

	// Process deletion
	err = ii.ProcessChange(FileChangeEvent{
		Type:      FileChangeDeleted,
		Path:      file,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("ProcessChange() error = %v", err)
	}

	if fileIndex.Has(file) {
		t.Error("File should not be in file index after delete")
	}
	if contentIndex.HasDocument(file) {
		t.Error("File should not be in content index after delete")
	}
}

func TestIncrementalIndexer_ProcessChange_Renamed(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldFile := filepath.Join(tmpDir, "old.go")
	newFile := filepath.Join(tmpDir, "new.go")
	if err := os.WriteFile(oldFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	ii := NewIncrementalIndexer(fileIndex, contentIndex, DefaultIncrementalConfig())

	// Initial index
	_ = ii.ProcessChange(FileChangeEvent{Type: FileChangeCreated, Path: oldFile})

	// Rename file
	os.Rename(oldFile, newFile)

	// Process rename
	err = ii.ProcessChange(FileChangeEvent{
		Type:      FileChangeRenamed,
		Path:      newFile,
		OldPath:   oldFile,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("ProcessChange() error = %v", err)
	}

	if fileIndex.Has(oldFile) {
		t.Error("Old path should not be in file index after rename")
	}
	if !fileIndex.Has(newFile) {
		t.Error("New path should be in file index after rename")
	}
	if contentIndex.HasDocument(oldFile) {
		t.Error("Old path should not be in content index after rename")
	}
	if !contentIndex.HasDocument(newFile) {
		t.Error("New path should be in content index after rename")
	}
}

func TestIncrementalIndexer_Rebuild(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files
	for i := 0; i < 3; i++ {
		file := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".go")
		if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	config := DefaultIncrementalConfig()
	config.Workers = 2
	ii := NewIncrementalIndexer(fileIndex, contentIndex, config)

	// Add some extra data to indexes
	_ = fileIndex.Add("/extra/file.go", FileInfo{Path: "/extra/file.go"})
	_ = contentIndex.IndexDocument("/extra/file.go", []byte("extra content"))

	err = ii.Rebuild(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	// Wait for rebuild to complete
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for rebuild to complete")
		default:
			if ii.Status() == IndexStatusIdle {
				goto done
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
done:

	// Extra file should be gone
	if fileIndex.Has("/extra/file.go") {
		t.Error("Extra file should be cleared on rebuild")
	}

	// Only files in tmpDir should be indexed
	if fileIndex.Count() != 3 {
		t.Errorf("File index count = %d, want 3", fileIndex.Count())
	}
}

func TestIncrementalIndexer_SaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	file := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(file, []byte("package main\n\nfunc hello() {}"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	fileIndex := NewFileIndex()
	contentIndex := NewContentIndex(DefaultContentIndexConfig())
	ii := NewIncrementalIndexer(fileIndex, contentIndex, DefaultIncrementalConfig())

	// Index the file
	_ = ii.ProcessChange(FileChangeEvent{Type: FileChangeCreated, Path: file})

	// Save
	var fileBuf, contentBuf bytes.Buffer
	err = ii.Save(&fileBuf, &contentBuf)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create new indexer and load
	fileIndex2 := NewFileIndex()
	contentIndex2 := NewContentIndex(DefaultContentIndexConfig())
	ii2 := NewIncrementalIndexer(fileIndex2, contentIndex2, DefaultIncrementalConfig())

	err = ii2.Load(&fileBuf, &contentBuf)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if fileIndex2.Count() != 1 {
		t.Errorf("Loaded file index count = %d, want 1", fileIndex2.Count())
	}
	if contentIndex2.DocumentCount() != 1 {
		t.Errorf("Loaded content index count = %d, want 1", contentIndex2.DocumentCount())
	}

	// Search should work on loaded index
	results, err := contentIndex2.Search(context.Background(), "hello", ContentSearchOptions{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search() on loaded index returned %d results, want 1", len(results))
	}
}

func TestIncrementalIndexer_ExcludePatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create vendor directory with file
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}
	vendorFile := filepath.Join(vendorDir, "lib.go")
	if err := os.WriteFile(vendorFile, []byte("package lib"), 0644); err != nil {
		t.Fatalf("Failed to write vendor file: %v", err)
	}

	// Create regular file
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write main file: %v", err)
	}

	fileIndex := NewFileIndex()
	config := DefaultIncrementalConfig()
	config.Workers = 1
	ii := NewIncrementalIndexer(fileIndex, nil, config)

	err = ii.Start(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for completion
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for indexing")
		default:
			if ii.Status() == IndexStatusIdle {
				goto done
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
done:

	// Vendor file should be excluded
	if fileIndex.Has(vendorFile) {
		t.Error("Vendor file should be excluded")
	}

	// Main file should be indexed
	if !fileIndex.Has(mainFile) {
		t.Error("Main file should be indexed")
	}
}

func TestIncrementalIndexer_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files
	for i := 0; i < 50; i++ {
		file := filepath.Join(tmpDir, "file"+string(rune('0'+i%10))+string(rune('0'+i/10))+".go")
		if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	ii := NewIncrementalIndexer(NewFileIndex(), nil, DefaultIncrementalConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = ii.Start(ctx, tmpDir)
	// Start should succeed, but indexing will be cancelled
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait a bit for indexing goroutine to notice cancellation
	time.Sleep(100 * time.Millisecond)
}

func TestMatchExcludePattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/.git/**", "/project/.git/config", true},
		{"**/.git/**", "/project/src/main.go", false},
		{"**/node_modules/**", "/project/node_modules/pkg/index.js", true},
		{"**/vendor/**", "/project/vendor/lib/lib.go", true},
		{"**/vendor/**", "/project/src/vendor.go", false},
		{"**/__pycache__/**", "/project/__pycache__/module.pyc", true},
		{"*.log", "debug.log", true},
		{"*.log", "main.go", false},
	}

	for _, tt := range tests {
		got := matchExcludePattern(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("matchExcludePattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestContainsDoublestar(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"**/.git/**", true},
		{"*.go", false},
		{"path/to/**", true},
		{"**/file", true},
		{"no stars", false},
	}

	for _, tt := range tests {
		if got := containsDoublestar(tt.input); got != tt.want {
			t.Errorf("containsDoublestar(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSplitOnDoublestar(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"**/.git/**", []string{"", "/.git/", ""}},
		{"path/**", []string{"path/", ""}},
		{"**/file", []string{"", "/file"}},
		{"no stars", []string{"no stars"}},
	}

	for _, tt := range tests {
		got := splitOnDoublestar(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitOnDoublestar(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitOnDoublestar(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestTrimSlashes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/path/", "path"},
		{"path", "path"},
		{"///path///", "path"},
		{"/", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := trimSlashes(tt.input); got != tt.want {
			t.Errorf("trimSlashes(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContainsPath(t *testing.T) {
	tests := []struct {
		path    string
		segment string
		want    bool
	}{
		{"/project/vendor/lib.go", "vendor", true},
		{"/project/src/main.go", "vendor", false},
		{"/project/.git/config", ".git", true},
		{"/project/vendor.go", "vendor", false}, // Not a path segment
		{"", "vendor", false},
		{"/project/vendor", "", false},
	}

	for _, tt := range tests {
		got := containsPath(tt.path, tt.segment)
		if got != tt.want {
			t.Errorf("containsPath(%q, %q) = %v, want %v", tt.path, tt.segment, got, tt.want)
		}
	}
}

func TestFileChangeType(t *testing.T) {
	// Just verify constants are defined correctly
	tests := []struct {
		t    FileChangeType
		want int
	}{
		{FileChangeCreated, 0},
		{FileChangeModified, 1},
		{FileChangeDeleted, 2},
		{FileChangeRenamed, 3},
	}

	for _, tt := range tests {
		if int(tt.t) != tt.want {
			t.Errorf("FileChangeType = %d, want %d", tt.t, tt.want)
		}
	}
}

func TestIndexEventType(t *testing.T) {
	// Verify constants are defined correctly
	tests := []struct {
		t    IndexEventType
		want int
	}{
		{IndexEventStarted, 0},
		{IndexEventProgress, 1},
		{IndexEventFileIndexed, 2},
		{IndexEventFileError, 3},
		{IndexEventCompleted, 4},
		{IndexEventError, 5},
	}

	for _, tt := range tests {
		if int(tt.t) != tt.want {
			t.Errorf("IndexEventType = %d, want %d", tt.t, tt.want)
		}
	}
}

func TestIncrementalIndexer_Progress(t *testing.T) {
	ii := NewIncrementalIndexer(NewFileIndex(), nil, DefaultIncrementalConfig())

	progress := ii.Progress()
	if progress.TotalFiles != 0 {
		t.Errorf("Initial TotalFiles = %d, want 0", progress.TotalFiles)
	}
}

func TestIncrementalIndexer_OnEvent(t *testing.T) {
	ii := NewIncrementalIndexer(NewFileIndex(), nil, DefaultIncrementalConfig())

	called := false
	ii.OnEvent(func(e IndexEvent) {
		called = true
	})

	// Emit an event manually
	ii.emitEvent(IndexEvent{Type: IndexEventStarted})

	if !called {
		t.Error("Event handler should have been called")
	}
}

func TestIncrementalIndexer_MultipleEventHandlers(t *testing.T) {
	ii := NewIncrementalIndexer(NewFileIndex(), nil, DefaultIncrementalConfig())

	count := 0
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		ii.OnEvent(func(e IndexEvent) {
			mu.Lock()
			count++
			mu.Unlock()
		})
	}

	ii.emitEvent(IndexEvent{Type: IndexEventStarted})

	mu.Lock()
	defer mu.Unlock()
	if count != 3 {
		t.Errorf("Event handlers called = %d, want 3", count)
	}
}
