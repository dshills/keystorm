package vfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestVFSInterface runs a suite of tests against any VFS implementation.
// This ensures both OSFS and MemFS behave consistently.
func TestVFSInterface(t *testing.T) {
	// Test with MemFS
	t.Run("MemFS", func(t *testing.T) {
		fs := NewMemFS()
		testVFSOperations(t, fs, "/")
	})

	// Test with OSFS
	t.Run("OSFS", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "vfs_test_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		fs := NewOSFS()
		testVFSOperations(t, fs, tmpDir)
	})
}

func testVFSOperations(t *testing.T, vfs VFS, root string) {
	t.Run("WriteFile_ReadFile", func(t *testing.T) {
		path := vfs.Join(root, "test.txt")
		content := []byte("hello world")

		err := vfs.WriteFile(path, content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		got, err := vfs.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(got) != string(content) {
			t.Errorf("content mismatch: got %q, want %q", got, content)
		}
	})

	t.Run("Stat", func(t *testing.T) {
		path := vfs.Join(root, "stat_test.txt")
		content := []byte("test content")

		err := vfs.WriteFile(path, content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		info, err := vfs.Stat(path)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.Name() != "stat_test.txt" {
			t.Errorf("Name: got %q, want %q", info.Name(), "stat_test.txt")
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("Size: got %d, want %d", info.Size(), len(content))
		}

		if info.IsDir() {
			t.Error("IsDir: expected false for file")
		}
	})

	t.Run("Mkdir_ReadDir", func(t *testing.T) {
		dirPath := vfs.Join(root, "testdir")

		err := vfs.Mkdir(dirPath, 0755)
		if err != nil {
			t.Fatalf("Mkdir failed: %v", err)
		}

		// Create files in directory
		vfs.WriteFile(vfs.Join(dirPath, "a.txt"), []byte("a"), 0644)
		vfs.WriteFile(vfs.Join(dirPath, "b.txt"), []byte("b"), 0644)

		entries, err := vfs.ReadDir(dirPath)
		if err != nil {
			t.Fatalf("ReadDir failed: %v", err)
		}

		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}

		// Check entries are sorted
		if len(entries) >= 2 {
			if entries[0].Name() != "a.txt" {
				t.Errorf("first entry: got %q, want %q", entries[0].Name(), "a.txt")
			}
			if entries[1].Name() != "b.txt" {
				t.Errorf("second entry: got %q, want %q", entries[1].Name(), "b.txt")
			}
		}
	})

	t.Run("MkdirAll", func(t *testing.T) {
		deepPath := vfs.Join(root, "deep", "nested", "dir")

		err := vfs.MkdirAll(deepPath, 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		if !vfs.IsDir(deepPath) {
			t.Error("directory was not created")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		path := vfs.Join(root, "to_remove.txt")

		err := vfs.WriteFile(path, []byte("delete me"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if !vfs.Exists(path) {
			t.Fatal("file should exist before removal")
		}

		err = vfs.Remove(path)
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		if vfs.Exists(path) {
			t.Error("file should not exist after removal")
		}
	})

	t.Run("RemoveAll", func(t *testing.T) {
		dirPath := vfs.Join(root, "remove_all_test")
		vfs.MkdirAll(dirPath, 0755)
		vfs.WriteFile(vfs.Join(dirPath, "file1.txt"), []byte("1"), 0644)
		vfs.MkdirAll(vfs.Join(dirPath, "subdir"), 0755)
		vfs.WriteFile(vfs.Join(dirPath, "subdir", "file2.txt"), []byte("2"), 0644)

		err := vfs.RemoveAll(dirPath)
		if err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}

		if vfs.Exists(dirPath) {
			t.Error("directory should not exist after RemoveAll")
		}
	})

	t.Run("Rename", func(t *testing.T) {
		oldPath := vfs.Join(root, "old_name.txt")
		newPath := vfs.Join(root, "new_name.txt")

		vfs.WriteFile(oldPath, []byte("rename me"), 0644)

		err := vfs.Rename(oldPath, newPath)
		if err != nil {
			t.Fatalf("Rename failed: %v", err)
		}

		if vfs.Exists(oldPath) {
			t.Error("old path should not exist after rename")
		}

		if !vfs.Exists(newPath) {
			t.Error("new path should exist after rename")
		}

		content, _ := vfs.ReadFile(newPath)
		if string(content) != "rename me" {
			t.Errorf("content changed after rename: got %q", content)
		}
	})

	t.Run("Exists_IsDir_IsRegular", func(t *testing.T) {
		filePath := vfs.Join(root, "exist_test.txt")
		dirPath := vfs.Join(root, "exist_dir")

		vfs.WriteFile(filePath, []byte("test"), 0644)
		vfs.Mkdir(dirPath, 0755)

		// Test Exists
		if !vfs.Exists(filePath) {
			t.Error("Exists: expected true for existing file")
		}
		if !vfs.Exists(dirPath) {
			t.Error("Exists: expected true for existing dir")
		}
		if vfs.Exists(vfs.Join(root, "nonexistent")) {
			t.Error("Exists: expected false for nonexistent path")
		}

		// Test IsDir
		if vfs.IsDir(filePath) {
			t.Error("IsDir: expected false for file")
		}
		if !vfs.IsDir(dirPath) {
			t.Error("IsDir: expected true for directory")
		}

		// Test IsRegular
		if !vfs.IsRegular(filePath) {
			t.Error("IsRegular: expected true for file")
		}
		if vfs.IsRegular(dirPath) {
			t.Error("IsRegular: expected false for directory")
		}
	})

	t.Run("PathOperations", func(t *testing.T) {
		// Test Join
		joined := vfs.Join("a", "b", "c")
		expected := filepath.Join("a", "b", "c")
		if joined != expected {
			t.Errorf("Join: got %q, want %q", joined, expected)
		}

		// Test Dir
		dir := vfs.Dir("/a/b/c.txt")
		if dir != "/a/b" && dir != "\\a\\b" { // Handle Windows
			t.Errorf("Dir: got %q", dir)
		}

		// Test Base
		base := vfs.Base("/a/b/c.txt")
		if base != "c.txt" {
			t.Errorf("Base: got %q, want %q", base, "c.txt")
		}

		// Test Ext
		ext := vfs.Ext("file.txt")
		if ext != ".txt" {
			t.Errorf("Ext: got %q, want %q", ext, ".txt")
		}
	})

	t.Run("Walk", func(t *testing.T) {
		walkRoot := vfs.Join(root, "walk_test")
		vfs.MkdirAll(walkRoot, 0755)
		vfs.WriteFile(vfs.Join(walkRoot, "a.txt"), []byte("a"), 0644)
		vfs.MkdirAll(vfs.Join(walkRoot, "subdir"), 0755)
		vfs.WriteFile(vfs.Join(walkRoot, "subdir", "b.txt"), []byte("b"), 0644)

		var paths []string
		err := vfs.Walk(walkRoot, func(path string, info FileInfo, err error) error {
			if err != nil {
				return err
			}
			paths = append(paths, info.Name())
			return nil
		})

		if err != nil {
			t.Fatalf("Walk failed: %v", err)
		}

		// Should have at least: walk_test, a.txt, subdir, b.txt
		if len(paths) < 4 {
			t.Errorf("expected at least 4 paths, got %d: %v", len(paths), paths)
		}
	})
}

func TestFileInfo(t *testing.T) {
	now := time.Now()
	fi := NewFileInfo("/path/to/file.txt", "file.txt", 1234, 0644, now, false)

	if fi.Path() != "/path/to/file.txt" {
		t.Errorf("Path: got %q", fi.Path())
	}
	if fi.Name() != "file.txt" {
		t.Errorf("Name: got %q", fi.Name())
	}
	if fi.Size() != 1234 {
		t.Errorf("Size: got %d", fi.Size())
	}
	if fi.Mode() != 0644 {
		t.Errorf("Mode: got %v", fi.Mode())
	}
	if fi.IsDir() {
		t.Error("IsDir: expected false")
	}
	if !fi.IsRegular() {
		t.Error("IsRegular: expected true")
	}

	// Test directory
	dirFI := NewFileInfo("/path/to/dir", "dir", 0, fs.ModeDir|0755, now, true)
	if !dirFI.IsDir() {
		t.Error("IsDir: expected true for directory")
	}
	if dirFI.IsRegular() {
		t.Error("IsRegular: expected false for directory")
	}
}

func TestDirEntry(t *testing.T) {
	fi := NewFileInfo("/path/to/file.txt", "file.txt", 100, 0644, time.Now(), false)
	de := NewDirEntry(fi)

	if de.Name() != "file.txt" {
		t.Errorf("Name: got %q", de.Name())
	}
	if de.IsDir() {
		t.Error("IsDir: expected false")
	}

	info, err := de.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Size() != 100 {
		t.Errorf("Info.Size: got %d", info.Size())
	}
}
