package filestore

import (
	"context"
	"testing"
	"time"

	perrors "github.com/dshills/keystorm/internal/project/errors"
	"github.com/dshills/keystorm/internal/project/vfs"
)

func setupTestStore(t *testing.T) (*FileStore, *vfs.MemFS) {
	t.Helper()
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	return store, memfs
}

func TestFileStore_Open(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	// Create a test file
	memfs.AddFile("/test/file.go", "package main\n")

	doc, err := store.Open(ctx, "/test/file.go")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if doc.Path != "/test/file.go" {
		t.Errorf("Path = %q, want %q", doc.Path, "/test/file.go")
	}

	if string(doc.GetContent()) != "package main\n" {
		t.Errorf("Content = %q", doc.GetContent())
	}

	if doc.LanguageID != "go" {
		t.Errorf("LanguageID = %q, want %q", doc.LanguageID, "go")
	}
}

func TestFileStore_Open_AlreadyOpen(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "content")

	doc1, err := store.Open(ctx, "/test/file.go")
	if err != nil {
		t.Fatalf("first Open failed: %v", err)
	}

	doc2, err := store.Open(ctx, "/test/file.go")
	if err != nil {
		t.Fatalf("second Open failed: %v", err)
	}

	if doc1 != doc2 {
		t.Error("opening same file twice should return same document")
	}
}

func TestFileStore_Open_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	_, err := store.Open(ctx, "/nonexistent/file.go")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}

	var pathErr *perrors.PathError
	if !isPathError(err, &pathErr) {
		t.Errorf("expected PathError, got %T", err)
	}
}

func TestFileStore_Open_Directory(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.Mkdir("/testdir", 0755)

	_, err := store.Open(ctx, "/testdir")
	if err == nil {
		t.Fatal("expected error for directory")
	}
}

func TestFileStore_Open_TooLarge(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStoreWithOptions(memfs, WithMaxFileSize(100))
	ctx := context.Background()

	// Create a file larger than max
	largeContent := make([]byte, 200)
	memfs.WriteFile("/large.txt", largeContent, 0644)

	_, err := store.Open(ctx, "/large.txt")
	if err == nil {
		t.Fatal("expected error for large file")
	}
}

func TestFileStore_Open_Binary(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	// Create a binary file with null bytes
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03}
	memfs.WriteFile("/binary.dat", binaryContent, 0644)

	_, err := store.Open(ctx, "/binary.dat")
	if err == nil {
		t.Fatal("expected error for binary file")
	}
}

func TestFileStore_Close(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "content")

	_, err := store.Open(ctx, "/test/file.go")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if !store.IsOpen("/test/file.go") {
		t.Error("file should be open")
	}

	err = store.Close(ctx, "/test/file.go", false)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if store.IsOpen("/test/file.go") {
		t.Error("file should be closed")
	}
}

func TestFileStore_Close_Dirty(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")

	doc, _ := store.Open(ctx, "/test/file.go")
	doc.SetContent([]byte("modified"))

	err := store.Close(ctx, "/test/file.go", false)
	if err == nil {
		t.Fatal("expected error closing dirty document")
	}

	// Force close should work
	err = store.Close(ctx, "/test/file.go", true)
	if err != nil {
		t.Fatalf("force Close failed: %v", err)
	}
}

func TestFileStore_Save(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")

	doc, _ := store.Open(ctx, "/test/file.go")
	doc.SetContent([]byte("modified"))

	if !doc.IsDirty() {
		t.Error("document should be dirty")
	}

	err := store.Save(ctx, "/test/file.go")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if doc.IsDirty() {
		t.Error("document should not be dirty after save")
	}

	// Verify content on disk
	diskContent, _ := memfs.ReadFile("/test/file.go")
	if string(diskContent) != "modified" {
		t.Errorf("disk content = %q, want %q", diskContent, "modified")
	}
}

func TestFileStore_SaveAs(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/old.go", "content")

	doc, _ := store.Open(ctx, "/test/old.go")

	err := store.SaveAs(ctx, "/test/old.go", "/test/new.go")
	if err != nil {
		t.Fatalf("SaveAs failed: %v", err)
	}

	// Document should now have new path
	if doc.Path != "/test/new.go" {
		t.Errorf("doc.Path = %q, want %q", doc.Path, "/test/new.go")
	}

	// Old path should not be open
	if store.IsOpen("/test/old.go") {
		t.Error("old path should not be open")
	}

	// New path should be open
	if !store.IsOpen("/test/new.go") {
		t.Error("new path should be open")
	}

	// New file should exist on disk
	if !memfs.Exists("/test/new.go") {
		t.Error("new file should exist on disk")
	}
}

func TestFileStore_Reload(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")

	doc, _ := store.Open(ctx, "/test/file.go")

	// Modify disk content
	memfs.WriteFile("/test/file.go", []byte("external change"), 0644)

	err := store.Reload(ctx, "/test/file.go", false)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if string(doc.GetContent()) != "external change" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "external change")
	}
}

func TestFileStore_Reload_Dirty(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")

	doc, _ := store.Open(ctx, "/test/file.go")
	doc.SetContent([]byte("local change"))

	err := store.Reload(ctx, "/test/file.go", false)
	if err == nil {
		t.Fatal("expected error reloading dirty document")
	}

	// Force reload should work
	memfs.WriteFile("/test/file.go", []byte("external change"), 0644)
	err = store.Reload(ctx, "/test/file.go", true)
	if err != nil {
		t.Fatalf("force Reload failed: %v", err)
	}

	if string(doc.GetContent()) != "external change" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "external change")
	}
}

func TestFileStore_UpdateContent(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")

	doc, _ := store.Open(ctx, "/test/file.go")

	err := store.UpdateContent("/test/file.go", []byte("updated"))
	if err != nil {
		t.Fatalf("UpdateContent failed: %v", err)
	}

	if string(doc.GetContent()) != "updated" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "updated")
	}

	if !doc.IsDirty() {
		t.Error("document should be dirty")
	}
}

func TestFileStore_ApplyEdit(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "hello world")

	doc, _ := store.Open(ctx, "/test/file.go")

	err := store.ApplyEdit("/test/file.go", 6, 11, []byte("go"))
	if err != nil {
		t.Fatalf("ApplyEdit failed: %v", err)
	}

	if string(doc.GetContent()) != "hello go" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "hello go")
	}
}

func TestFileStore_OpenDocuments(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/a.go", "a")
	memfs.AddFile("/test/b.go", "b")
	memfs.AddFile("/test/c.go", "c")

	store.Open(ctx, "/test/a.go")
	store.Open(ctx, "/test/b.go")
	store.Open(ctx, "/test/c.go")

	docs := store.OpenDocuments()
	if len(docs) != 3 {
		t.Errorf("OpenDocuments count = %d, want 3", len(docs))
	}
}

func TestFileStore_DirtyDocuments(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/a.go", "a")
	memfs.AddFile("/test/b.go", "b")

	docA, _ := store.Open(ctx, "/test/a.go")
	store.Open(ctx, "/test/b.go")

	docA.SetContent([]byte("modified"))

	dirty := store.DirtyDocuments()
	if len(dirty) != 1 {
		t.Errorf("DirtyDocuments count = %d, want 1", len(dirty))
	}
	if dirty[0].Path != "/test/a.go" {
		t.Errorf("dirty doc path = %q, want %q", dirty[0].Path, "/test/a.go")
	}
}

func TestFileStore_CreateFile(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.Mkdir("/test", 0755)

	doc, err := store.CreateFile(ctx, "/test/new.go", []byte("package main"))
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	if doc.Path != "/test/new.go" {
		t.Errorf("Path = %q, want %q", doc.Path, "/test/new.go")
	}

	// File should exist on disk
	if !memfs.Exists("/test/new.go") {
		t.Error("file should exist on disk")
	}

	// Document should be open
	if !store.IsOpen("/test/new.go") {
		t.Error("document should be open")
	}
}

func TestFileStore_CreateFile_AlreadyExists(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/existing.go", "existing")

	_, err := store.CreateFile(ctx, "/test/existing.go", []byte("new"))
	if err == nil {
		t.Fatal("expected error creating file that already exists")
	}
}

func TestFileStore_DeleteFile(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "content")

	err := store.DeleteFile(ctx, "/test/file.go", false)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	if memfs.Exists("/test/file.go") {
		t.Error("file should not exist on disk")
	}
}

func TestFileStore_DeleteFile_Open(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "content")
	store.Open(ctx, "/test/file.go")

	err := store.DeleteFile(ctx, "/test/file.go", false)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Document should be closed
	if store.IsOpen("/test/file.go") {
		t.Error("document should be closed after delete")
	}
}

func TestFileStore_DeleteFile_Dirty(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	doc, _ := store.Open(ctx, "/test/file.go")
	doc.SetContent([]byte("modified"))

	err := store.DeleteFile(ctx, "/test/file.go", false)
	if err == nil {
		t.Fatal("expected error deleting dirty document")
	}

	// Force delete should work
	err = store.DeleteFile(ctx, "/test/file.go", true)
	if err != nil {
		t.Fatalf("force DeleteFile failed: %v", err)
	}
}

func TestFileStore_RenameFile(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/old.go", "content")

	err := store.RenameFile(ctx, "/test/old.go", "/test/new.go")
	if err != nil {
		t.Fatalf("RenameFile failed: %v", err)
	}

	if memfs.Exists("/test/old.go") {
		t.Error("old file should not exist")
	}
	if !memfs.Exists("/test/new.go") {
		t.Error("new file should exist")
	}
}

func TestFileStore_RenameFile_Open(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/old.go", "content")
	doc, _ := store.Open(ctx, "/test/old.go")

	err := store.RenameFile(ctx, "/test/old.go", "/test/new.go")
	if err != nil {
		t.Fatalf("RenameFile failed: %v", err)
	}

	// Document path should be updated
	if doc.Path != "/test/new.go" {
		t.Errorf("doc.Path = %q, want %q", doc.Path, "/test/new.go")
	}

	// Old path should not be open
	if store.IsOpen("/test/old.go") {
		t.Error("old path should not be open")
	}

	// New path should be open
	if !store.IsOpen("/test/new.go") {
		t.Error("new path should be open")
	}
}

func TestFileStore_CheckExternalChanges(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	store.Open(ctx, "/test/file.go")

	// No changes yet
	changed := store.CheckExternalChanges()
	if len(changed) != 0 {
		t.Errorf("expected no changes, got %d", len(changed))
	}

	// Modify file on disk
	time.Sleep(10 * time.Millisecond) // Ensure different mod time
	memfs.WriteFile("/test/file.go", []byte("external change"), 0644)

	changed = store.CheckExternalChanges()
	if len(changed) != 1 {
		t.Errorf("expected 1 change, got %d", len(changed))
	}
}

func TestFileStore_CloseAll(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/a.go", "a")
	memfs.AddFile("/test/b.go", "b")

	store.Open(ctx, "/test/a.go")
	store.Open(ctx, "/test/b.go")

	if store.Count() != 2 {
		t.Errorf("Count = %d, want 2", store.Count())
	}

	err := store.CloseAll(ctx, false)
	if err != nil {
		t.Fatalf("CloseAll failed: %v", err)
	}

	if store.Count() != 0 {
		t.Errorf("Count = %d, want 0", store.Count())
	}
}

func TestFileStore_CloseAll_Dirty(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	doc, _ := store.Open(ctx, "/test/file.go")
	doc.SetContent([]byte("modified"))

	err := store.CloseAll(ctx, false)
	if err == nil {
		t.Fatal("expected error closing all with dirty documents")
	}

	// Force should work
	err = store.CloseAll(ctx, true)
	if err != nil {
		t.Fatalf("force CloseAll failed: %v", err)
	}
}

func TestFileStore_EventHandlers(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	var openedPath, closedPath, savedPath string

	store.OnOpen(func(doc *Document) {
		openedPath = doc.Path
	})
	store.OnClose(func(path string) {
		closedPath = path
	})
	store.OnSave(func(doc *Document) {
		savedPath = doc.Path
	})

	memfs.AddFile("/test/file.go", "content")

	// Test open handler
	store.Open(ctx, "/test/file.go")
	if openedPath != "/test/file.go" {
		t.Errorf("openedPath = %q, want %q", openedPath, "/test/file.go")
	}

	// Test save handler
	store.Save(ctx, "/test/file.go")
	if savedPath != "/test/file.go" {
		t.Errorf("savedPath = %q, want %q", savedPath, "/test/file.go")
	}

	// Test close handler
	store.Close(ctx, "/test/file.go", false)
	if closedPath != "/test/file.go" {
		t.Errorf("closedPath = %q, want %q", closedPath, "/test/file.go")
	}
}

func TestFileStore_GetStats(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	memfs.AddFile("/test/a.go", "content a")
	memfs.AddFile("/test/b.go", "content b")

	docA, _ := store.Open(ctx, "/test/a.go")
	store.Open(ctx, "/test/b.go")

	docA.SetContent([]byte("modified"))

	stats := store.GetStats()
	if stats.OpenCount != 2 {
		t.Errorf("OpenCount = %d, want 2", stats.OpenCount)
	}
	if stats.DirtyCount != 1 {
		t.Errorf("DirtyCount = %d, want 1", stats.DirtyCount)
	}
	if stats.TotalSize <= 0 {
		t.Error("TotalSize should be > 0")
	}
}

func TestFileStore_ConcurrentAccess(t *testing.T) {
	store, memfs := setupTestStore(t)
	ctx := context.Background()

	// Create test files
	for i := 0; i < 10; i++ {
		memfs.AddFile("/test/file"+string(rune('0'+i))+".go", "content")
	}

	done := make(chan bool)

	// Multiple goroutines opening files
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				path := "/test/file" + string(rune('0'+j)) + ".go"
				store.Open(ctx, path)
				store.IsOpen(path)
				store.IsDirty(path)
			}
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 5; i++ {
		<-done
	}
}

// Helper to check PathError
func isPathError(err error, target **perrors.PathError) bool {
	if pe, ok := err.(*perrors.PathError); ok {
		*target = pe
		return true
	}
	return false
}
