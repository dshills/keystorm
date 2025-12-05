package vfs

import (
	"io"
	"testing"
)

func TestMemFS_AddFile(t *testing.T) {
	fs := NewMemFS()

	// AddFile should create parent directories
	err := fs.AddFile("/a/b/c/file.txt", "content")
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	if !fs.Exists("/a/b/c/file.txt") {
		t.Error("file should exist")
	}

	if !fs.IsDir("/a/b/c") {
		t.Error("parent directory should exist")
	}

	if !fs.IsDir("/a/b") {
		t.Error("grandparent directory should exist")
	}
}

func TestMemFS_Create(t *testing.T) {
	fs := NewMemFS()

	w, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = w.Write([]byte("hello "))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	_, err = w.Write([]byte("world"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	content, err := fs.ReadFile("/test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(content) != "hello world" {
		t.Errorf("content: got %q, want %q", content, "hello world")
	}
}

func TestMemFS_Open(t *testing.T) {
	fs := NewMemFS()
	fs.AddFile("/test.txt", "file content")

	r, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer r.Close()

	content, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if string(content) != "file content" {
		t.Errorf("content: got %q", content)
	}
}

func TestMemFS_OpenDirectory(t *testing.T) {
	fs := NewMemFS()
	fs.Mkdir("/dir", 0755)

	_, err := fs.Open("/dir")
	if err == nil {
		t.Error("expected error when opening directory")
	}
}

func TestMemFS_OpenNonExistent(t *testing.T) {
	fs := NewMemFS()

	_, err := fs.Open("/nonexistent")
	if err == nil {
		t.Error("expected error when opening nonexistent file")
	}
}

func TestMemFS_ReadFileModification(t *testing.T) {
	fs := NewMemFS()
	original := "original content"
	fs.AddFile("/test.txt", original)

	// Read the file
	content, _ := fs.ReadFile("/test.txt")

	// Modify the returned slice
	content[0] = 'X'

	// Original should be unchanged
	content2, _ := fs.ReadFile("/test.txt")
	if string(content2) != original {
		t.Error("modifying returned slice affected stored content")
	}
}

func TestMemFS_WriteFileModification(t *testing.T) {
	fs := NewMemFS()
	data := []byte("original")
	fs.WriteFile("/test.txt", data, 0644)

	// Modify the original slice
	data[0] = 'X'

	// Stored content should be unchanged
	content, _ := fs.ReadFile("/test.txt")
	if string(content) != "original" {
		t.Error("modifying original slice affected stored content")
	}
}

func TestMemFS_Files(t *testing.T) {
	fs := NewMemFS()
	fs.AddFile("/a.txt", "a")
	fs.AddFile("/b.txt", "b")
	fs.AddFile("/c/d.txt", "d")

	files := fs.Files()
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}

	// Should be sorted
	if files[0] != "/a.txt" {
		t.Errorf("files[0]: got %q", files[0])
	}
}

func TestMemFS_Dirs(t *testing.T) {
	fs := NewMemFS()
	fs.MkdirAll("/a/b/c", 0755)

	dirs := fs.Dirs()

	// Should include /, /a, /a/b, /a/b/c
	if len(dirs) != 4 {
		t.Errorf("expected 4 dirs, got %d: %v", len(dirs), dirs)
	}
}

func TestMemFS_Glob(t *testing.T) {
	fs := NewMemFS()
	fs.AddFile("/test1.txt", "1")
	fs.AddFile("/test2.txt", "2")
	fs.AddFile("/other.go", "go")

	matches, err := fs.Glob("/*.txt")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d: %v", len(matches), matches)
	}
}

func TestMemFS_RemoveNonEmptyDir(t *testing.T) {
	fs := NewMemFS()
	fs.MkdirAll("/dir", 0755)
	fs.AddFile("/dir/file.txt", "content")

	err := fs.Remove("/dir")
	if err == nil {
		t.Error("expected error removing non-empty directory")
	}
}

func TestMemFS_MkdirExisting(t *testing.T) {
	fs := NewMemFS()
	fs.Mkdir("/dir", 0755)

	err := fs.Mkdir("/dir", 0755)
	if err == nil {
		t.Error("expected error creating existing directory")
	}
}

func TestMemFS_MkdirAllExistingFile(t *testing.T) {
	fs := NewMemFS()
	fs.AddFile("/path", "content")

	err := fs.MkdirAll("/path/subdir", 0755)
	if err == nil {
		t.Error("expected error creating directory over file")
	}
}

func TestMemFS_RenameDirectory(t *testing.T) {
	fs := NewMemFS()
	fs.MkdirAll("/old/subdir", 0755)
	fs.AddFile("/old/file.txt", "file")
	fs.AddFile("/old/subdir/nested.txt", "nested")

	err := fs.Rename("/old", "/new")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	if fs.Exists("/old") {
		t.Error("old path should not exist")
	}

	if !fs.Exists("/new") {
		t.Error("/new should exist")
	}

	if !fs.Exists("/new/file.txt") {
		t.Error("/new/file.txt should exist")
	}

	if !fs.Exists("/new/subdir/nested.txt") {
		t.Error("/new/subdir/nested.txt should exist")
	}
}

func TestMemFS_Rel(t *testing.T) {
	fs := NewMemFS()

	tests := []struct {
		base   string
		target string
		want   string
	}{
		{"/a/b", "/a/b/c/d", "c/d"},
		{"/a/b", "/a/b", "."},
		{"/", "/a/b", "a/b"},
	}

	for _, tt := range tests {
		got, err := fs.Rel(tt.base, tt.target)
		if err != nil {
			t.Errorf("Rel(%q, %q) error: %v", tt.base, tt.target, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Rel(%q, %q) = %q, want %q", tt.base, tt.target, got, tt.want)
		}
	}
}

func TestMemFS_ConcurrentAccess(t *testing.T) {
	fs := NewMemFS()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			fs.WriteFile("/concurrent.txt", []byte("data"), 0644)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			fs.ReadFile("/concurrent.txt")
		}
		done <- true
	}()

	// Stat goroutine
	go func() {
		for i := 0; i < 100; i++ {
			fs.Stat("/concurrent.txt")
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}

func TestMemFS_CreateParentNotExist(t *testing.T) {
	fs := NewMemFS()

	_, err := fs.Create("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error creating file in nonexistent directory")
	}
}

func TestMemFS_WriteParentNotExist(t *testing.T) {
	fs := NewMemFS()

	err := fs.WriteFile("/nonexistent/file.txt", []byte("data"), 0644)
	if err == nil {
		t.Error("expected error writing file in nonexistent directory")
	}
}

func TestMemFS_RemoveAllNonExistent(t *testing.T) {
	fs := NewMemFS()

	// RemoveAll should succeed for non-existent paths
	err := fs.RemoveAll("/nonexistent")
	if err != nil {
		t.Errorf("RemoveAll on non-existent should succeed: %v", err)
	}
}
