package index

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileIndex_SaveLoad(t *testing.T) {
	idx := NewFileIndex()

	// Add some files
	now := time.Now().Truncate(time.Nanosecond)
	_ = idx.Add("/project/main.go", FileInfo{
		Name:    "main.go",
		Size:    1024,
		ModTime: now,
		IsDir:   false,
		Mode:    0644,
	})
	_ = idx.Add("/project/src", FileInfo{
		Name:    "src",
		Size:    0,
		ModTime: now,
		IsDir:   true,
		Mode:    0755,
	})
	_ = idx.Add("/project/link", FileInfo{
		Name:      "link",
		Size:      100,
		ModTime:   now,
		IsSymlink: true,
		Mode:      0777,
	})

	// Save to buffer
	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	// Create new index and load
	idx2 := NewFileIndex()
	if err := idx2.Load(&buf); err != nil {
		t.Fatalf("Load error = %v", err)
	}

	// Verify count
	if idx2.Count() != 3 {
		t.Errorf("Count() = %d, want 3", idx2.Count())
	}

	// Verify file
	info, ok := idx2.Get("/project/main.go")
	if !ok {
		t.Fatal("main.go not found")
	}
	if info.Name != "main.go" {
		t.Errorf("Name = %q, want main.go", info.Name)
	}
	if info.Size != 1024 {
		t.Errorf("Size = %d, want 1024", info.Size)
	}
	if info.IsDir {
		t.Error("IsDir should be false")
	}
	if info.Mode != 0644 {
		t.Errorf("Mode = %o, want 0644", info.Mode)
	}

	// Verify directory
	info, ok = idx2.Get("/project/src")
	if !ok {
		t.Fatal("src not found")
	}
	if !info.IsDir {
		t.Error("src IsDir should be true")
	}
	if info.Mode != 0755 {
		t.Errorf("Mode = %o, want 0755", info.Mode)
	}

	// Verify symlink
	info, ok = idx2.Get("/project/link")
	if !ok {
		t.Fatal("link not found")
	}
	if !info.IsSymlink {
		t.Error("link IsSymlink should be true")
	}

	idx.Close()
	idx2.Close()
}

func TestFileIndex_SaveLoad_Empty(t *testing.T) {
	idx := NewFileIndex()

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	idx2 := NewFileIndex()
	if err := idx2.Load(&buf); err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if idx2.Count() != 0 {
		t.Errorf("Count() = %d, want 0", idx2.Count())
	}

	idx.Close()
	idx2.Close()
}

func TestFileIndex_Load_InvalidMagic(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	buf := bytes.NewBufferString("XXXX") // Invalid magic
	err := idx.Load(buf)
	if err != ErrInvalidFormat {
		t.Errorf("Load error = %v, want ErrInvalidFormat", err)
	}
}

func TestFileIndex_Load_VersionMismatch(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	// Create buffer with correct magic but wrong version
	var buf bytes.Buffer
	buf.Write(persistMagic)
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Invalid version (big number)

	err := idx.Load(&buf)
	if err != ErrVersionMismatch {
		t.Errorf("Load error = %v, want ErrVersionMismatch", err)
	}
}

func TestFileIndex_SaveLoad_ModTime(t *testing.T) {
	idx := NewFileIndex()

	// Use a specific time with nanoseconds
	modTime := time.Date(2024, 6, 15, 10, 30, 45, 123456789, time.UTC)
	_ = idx.Add("/test.go", FileInfo{
		Name:    "test.go",
		ModTime: modTime,
	})

	var buf bytes.Buffer
	_ = idx.Save(&buf)

	idx2 := NewFileIndex()
	_ = idx2.Load(&buf)

	info, _ := idx2.Get("/test.go")
	if !info.ModTime.Equal(modTime) {
		t.Errorf("ModTime = %v, want %v", info.ModTime, modTime)
	}

	idx.Close()
	idx2.Close()
}

func TestFileIndex_SaveLoad_Indexes(t *testing.T) {
	idx := NewFileIndex()

	// Add files with same name in different directories
	_ = idx.Add("/project/src/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/project/cmd/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/project/src/util.go", FileInfo{Name: "util.go"})

	var buf bytes.Buffer
	_ = idx.Save(&buf)

	idx2 := NewFileIndex()
	_ = idx2.Load(&buf)

	// Test name index is rebuilt
	files := idx2.GetByName("main.go")
	if len(files) != 2 {
		t.Errorf("GetByName(main.go) = %d files, want 2", len(files))
	}

	// Test directory index is rebuilt
	files = idx2.GetByDirectory("/project/src")
	if len(files) != 2 {
		t.Errorf("GetByDirectory(/project/src) = %d files, want 2", len(files))
	}

	idx.Close()
	idx2.Close()
}

func TestFileIndex_SaveToFile_LoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "test.index")

	idx := NewFileIndex()
	_ = idx.Add("/test.go", FileInfo{Name: "test.go", Size: 100})

	// Save to file
	if err := idx.SaveToFile(indexPath); err != nil {
		t.Fatalf("SaveToFile error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("Index file not created: %v", err)
	}

	// Load from file
	idx2 := NewFileIndex()
	if err := idx2.LoadFromFile(indexPath); err != nil {
		t.Fatalf("LoadFromFile error = %v", err)
	}

	if idx2.Count() != 1 {
		t.Errorf("Count() = %d, want 1", idx2.Count())
	}

	info, _ := idx2.Get("/test.go")
	if info.Size != 100 {
		t.Errorf("Size = %d, want 100", info.Size)
	}

	idx.Close()
	idx2.Close()
}

func TestFileIndex_LoadFromFile_NotExists(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	err := idx.LoadFromFile("/nonexistent/path/index.bin")
	if err == nil {
		t.Error("LoadFromFile should error for nonexistent file")
	}
}

func TestFileIndex_SaveLoad_LargeIndex(t *testing.T) {
	idx := NewFileIndex()

	// Add many files
	count := 1000
	for i := 0; i < count; i++ {
		path := sprintf("/project/src/file%d.go", i)
		_ = idx.Add(path, FileInfo{
			Name:    sprintf("file%d.go", i),
			Size:    int64(i * 100),
			ModTime: time.Now(),
			Mode:    0644,
		})
	}

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	idx2 := NewFileIndex()
	if err := idx2.Load(&buf); err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if idx2.Count() != count {
		t.Errorf("Count() = %d, want %d", idx2.Count(), count)
	}

	// Verify a few entries
	info, ok := idx2.Get("/project/src/file500.go")
	if !ok {
		t.Error("file500.go not found")
	}
	if info.Size != 50000 {
		t.Errorf("file500.go Size = %d, want 50000", info.Size)
	}

	idx.Close()
	idx2.Close()
}

func TestFileIndex_SaveLoad_SpecialCharacters(t *testing.T) {
	idx := NewFileIndex()

	// Path with special characters
	_ = idx.Add("/project/src/my file (1).go", FileInfo{Name: "my file (1).go"})
	_ = idx.Add("/project/src/日本語.go", FileInfo{Name: "日本語.go"})

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	idx2 := NewFileIndex()
	if err := idx2.Load(&buf); err != nil {
		t.Fatalf("Load error = %v", err)
	}

	_, ok := idx2.Get("/project/src/my file (1).go")
	if !ok {
		t.Error("file with spaces not found")
	}

	_, ok = idx2.Get("/project/src/日本語.go")
	if !ok {
		t.Error("file with unicode not found")
	}

	idx.Close()
	idx2.Close()
}

func TestFileIndex_Save_Closed(t *testing.T) {
	idx := NewFileIndex()
	_ = idx.Add("/test.go", FileInfo{Name: "test.go"})
	idx.Close()

	var buf bytes.Buffer
	err := idx.Save(&buf)
	if err != ErrIndexClosed {
		t.Errorf("Save after close error = %v, want ErrIndexClosed", err)
	}
}

func TestFileIndex_Load_Closed(t *testing.T) {
	// First create valid data
	idx := NewFileIndex()
	_ = idx.Add("/test.go", FileInfo{Name: "test.go"})

	var buf bytes.Buffer
	_ = idx.Save(&buf)
	idx.Close()

	// Try to load into closed index
	idx2 := NewFileIndex()
	idx2.Close()

	err := idx2.Load(&buf)
	if err != ErrIndexClosed {
		t.Errorf("Load into closed index error = %v, want ErrIndexClosed", err)
	}
}

func BenchmarkFileIndex_Save(b *testing.B) {
	idx := NewFileIndex()
	for i := 0; i < 10000; i++ {
		path := sprintf("/project/src/file%d.go", i)
		_ = idx.Add(path, FileInfo{Name: sprintf("file%d.go", i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = idx.Save(&buf)
	}
}

func BenchmarkFileIndex_Load(b *testing.B) {
	idx := NewFileIndex()
	for i := 0; i < 10000; i++ {
		path := sprintf("/project/src/file%d.go", i)
		_ = idx.Add(path, FileInfo{Name: sprintf("file%d.go", i)})
	}

	var buf bytes.Buffer
	_ = idx.Save(&buf)
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx2 := NewFileIndex()
		_ = idx2.Load(bytes.NewReader(data))
	}
}
