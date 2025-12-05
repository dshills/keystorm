package filestore

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/project/vfs"
)

func TestNewDocument(t *testing.T) {
	content := []byte("hello world\nline 2\n")
	modTime := time.Now()

	doc := NewDocument("/test/file.go", content, modTime)

	if doc.Path != "/test/file.go" {
		t.Errorf("Path = %q, want %q", doc.Path, "/test/file.go")
	}

	if doc.Version != 1 {
		t.Errorf("Version = %d, want 1", doc.Version)
	}

	if string(doc.Content) != string(content) {
		t.Errorf("Content = %q, want %q", doc.Content, content)
	}

	if doc.LanguageID != "go" {
		t.Errorf("LanguageID = %q, want %q", doc.LanguageID, "go")
	}

	if doc.IsDirty() {
		t.Error("new document should not be dirty")
	}
}

func TestNewDocument_WithBOM(t *testing.T) {
	// UTF-8 BOM + content
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...)
	doc := NewDocument("/test/file.txt", content, time.Now())

	// BOM should be stripped
	if string(doc.Content) != "hello" {
		t.Errorf("Content = %q, want %q", doc.Content, "hello")
	}

	if doc.Encoding != vfs.EncodingUTF8BOM {
		t.Errorf("Encoding = %v, want %v", doc.Encoding, vfs.EncodingUTF8BOM)
	}
}

func TestDocument_IsDirty(t *testing.T) {
	content := []byte("original")
	doc := NewDocument("/test/file.txt", content, time.Now())

	if doc.IsDirty() {
		t.Error("new document should not be dirty")
	}

	// Modify content
	doc.SetContent([]byte("modified"))

	if !doc.IsDirty() {
		t.Error("modified document should be dirty")
	}

	// Set back to original
	doc.SetContent([]byte("original"))

	if doc.IsDirty() {
		t.Error("document with original content should not be dirty")
	}
}

func TestDocument_SetContent(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("original"), time.Now())
	initialVersion := doc.GetVersion()

	doc.SetContent([]byte("new content"))

	if doc.GetVersion() != initialVersion+1 {
		t.Errorf("Version = %d, want %d", doc.GetVersion(), initialVersion+1)
	}

	if string(doc.GetContent()) != "new content" {
		t.Errorf("Content = %q, want %q", doc.GetContent(), "new content")
	}
}

func TestDocument_ApplyEdit(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("hello world"), time.Now())
	initialVersion := doc.GetVersion()

	// Replace "world" with "go"
	err := doc.ApplyEdit(6, 11, []byte("go"))
	if err != nil {
		t.Fatalf("ApplyEdit failed: %v", err)
	}

	if doc.GetVersion() != initialVersion+1 {
		t.Errorf("Version = %d, want %d", doc.GetVersion(), initialVersion+1)
	}

	if string(doc.GetContent()) != "hello go" {
		t.Errorf("Content = %q, want %q", doc.GetContent(), "hello go")
	}
}

func TestDocument_ApplyEdit_Insert(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("helloworld"), time.Now())

	// Insert " " between hello and world
	err := doc.ApplyEdit(5, 5, []byte(" "))
	if err != nil {
		t.Fatalf("ApplyEdit failed: %v", err)
	}

	if string(doc.GetContent()) != "hello world" {
		t.Errorf("Content = %q, want %q", doc.GetContent(), "hello world")
	}
}

func TestDocument_ApplyEdit_Delete(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("hello world"), time.Now())

	// Delete " world"
	err := doc.ApplyEdit(5, 11, []byte{})
	if err != nil {
		t.Fatalf("ApplyEdit failed: %v", err)
	}

	if string(doc.GetContent()) != "hello" {
		t.Errorf("Content = %q, want %q", doc.GetContent(), "hello")
	}
}

func TestDocument_ApplyEdit_InvalidBounds(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("hello"), time.Now())
	initialVersion := doc.GetVersion()

	tests := []struct {
		name  string
		start int
		end   int
	}{
		{"negative start", -1, 3},
		{"negative end", 0, -1},
		{"start > len", 10, 11},
		{"end > len", 0, 10},
		{"start > end", 3, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := doc.ApplyEdit(tt.start, tt.end, []byte("x"))
			if err != ErrInvalidEditRange {
				t.Errorf("ApplyEdit(%d, %d) error = %v, want ErrInvalidEditRange", tt.start, tt.end, err)
			}
		})
	}

	// Ensure version didn't change for invalid edits
	if doc.GetVersion() != initialVersion {
		t.Errorf("Version changed for invalid edit: got %d, want %d", doc.GetVersion(), initialVersion)
	}
}

func TestDocument_MarkSaved(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("original"), time.Now())
	doc.SetContent([]byte("modified"))

	if !doc.IsDirty() {
		t.Error("document should be dirty before save")
	}

	newModTime := time.Now().Add(time.Second)
	doc.MarkSaved(newModTime)

	if doc.IsDirty() {
		t.Error("document should not be dirty after save")
	}

	if !doc.DiskModTime.Equal(newModTime) {
		t.Errorf("DiskModTime = %v, want %v", doc.DiskModTime, newModTime)
	}
}

func TestDocument_HasExternalChanges(t *testing.T) {
	diskModTime := time.Now()
	doc := NewDocument("/test/file.txt", []byte("content"), diskModTime)

	if doc.HasExternalChanges(diskModTime) {
		t.Error("no external changes expected for same mod time")
	}

	newDiskModTime := diskModTime.Add(time.Second)
	if !doc.HasExternalChanges(newDiskModTime) {
		t.Error("external changes expected for different mod time")
	}
}

func TestDocument_Reload(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("original"), time.Now())
	initialVersion := doc.GetVersion()

	// Reload with same content
	changed := doc.Reload([]byte("original"), time.Now())
	if changed {
		t.Error("reload with same content should not report change")
	}
	if doc.GetVersion() != initialVersion {
		t.Error("version should not change on reload with same content")
	}

	// Reload with different content
	changed = doc.Reload([]byte("new content"), time.Now())
	if !changed {
		t.Error("reload with different content should report change")
	}
	if doc.GetVersion() != initialVersion+1 {
		t.Errorf("Version = %d, want %d", doc.GetVersion(), initialVersion+1)
	}
	if string(doc.GetContent()) != "new content" {
		t.Errorf("Content = %q, want %q", doc.GetContent(), "new content")
	}
	if doc.IsDirty() {
		t.Error("document should not be dirty after reload")
	}
}

func TestDocument_MarkClosed(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("content"), time.Now())

	if doc.IsClosed() {
		t.Error("new document should not be closed")
	}

	doc.MarkClosed()

	if !doc.IsClosed() {
		t.Error("document should be closed after MarkClosed")
	}
}

func TestDocument_LineCount(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"", 0},
		{"single line", 1},
		{"line1\nline2", 2},
		{"line1\nline2\n", 2},
		{"line1\r\nline2\r\nline3", 3},
	}

	for _, tt := range tests {
		doc := NewDocument("/test/file.txt", []byte(tt.content), time.Now())
		if got := doc.LineCount(); got != tt.want {
			t.Errorf("LineCount(%q) = %d, want %d", tt.content, got, tt.want)
		}
	}
}

func TestDocument_ContentForSave_WithBOM(t *testing.T) {
	// Create document that was opened with BOM
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...)
	doc := NewDocument("/test/file.txt", content, time.Now())

	// Content should have BOM stripped for editing
	if string(doc.Content) != "hello" {
		t.Errorf("Content = %q, want %q", doc.Content, "hello")
	}

	// ContentForSave should re-add BOM
	saveContent := doc.ContentForSave()
	if len(saveContent) != len(content) {
		t.Errorf("ContentForSave length = %d, want %d", len(saveContent), len(content))
	}
	if saveContent[0] != 0xEF || saveContent[1] != 0xBB || saveContent[2] != 0xBF {
		t.Error("BOM should be present in ContentForSave output")
	}
}

func TestDocument_GetContent_IsCopy(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("original"), time.Now())

	content := doc.GetContent()
	content[0] = 'X'

	// Original should be unchanged
	if string(doc.GetContent()) != "original" {
		t.Error("modifying returned content should not affect document")
	}
}

func TestDocument_ConcurrentAccess(t *testing.T) {
	doc := NewDocument("/test/file.txt", []byte("initial content"), time.Now())

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			doc.SetContent([]byte("modified content"))
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = doc.GetContent()
			_ = doc.IsDirty()
			_ = doc.GetVersion()
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done
}

func TestDetectLanguageID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.go", "go"},
		{"/path/to/file.py", "python"},
		{"/path/to/file.js", "javascript"},
		{"/path/to/file.ts", "typescript"},
		{"/path/to/file.tsx", "typescriptreact"},
		{"/path/to/file.rs", "rust"},
		{"/path/to/file.java", "java"},
		{"/path/to/file.md", "markdown"},
		{"/path/to/file.json", "json"},
		{"/path/to/file.yaml", "yaml"},
		{"/path/to/file.yml", "yaml"},
		{"/path/to/file.html", "html"},
		{"/path/to/file.css", "css"},
		{"/path/to/file.sql", "sql"},
		{"/path/to/file.sh", "shellscript"},
		{"/path/to/file.unknown", "plaintext"},
		{"/path/to/noextension", "plaintext"},
	}

	for _, tt := range tests {
		got := detectLanguageID(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguageID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
