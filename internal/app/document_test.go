package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDocument(t *testing.T) {
	doc := NewDocument("/path/to/file.go", []byte("package main"))

	if doc.Path != "/path/to/file.go" {
		t.Errorf("expected path '/path/to/file.go', got '%s'", doc.Path)
	}
	if doc.Name != "file.go" {
		t.Errorf("expected name 'file.go', got '%s'", doc.Name)
	}
	if doc.Engine == nil {
		t.Error("expected engine to be initialized")
	}
	if doc.LanguageID != "go" {
		t.Errorf("expected language 'go', got '%s'", doc.LanguageID)
	}
	if doc.IsModified() {
		t.Error("expected document to not be modified initially")
	}
	if doc.IsScratch() {
		t.Error("expected document to not be scratch")
	}
}

func TestNewDocument_EmptyPath(t *testing.T) {
	doc := NewDocument("", []byte("content"))

	if doc.Path != "" {
		t.Errorf("expected empty path, got '%s'", doc.Path)
	}
	if doc.Name != "Untitled" {
		t.Errorf("expected name 'Untitled', got '%s'", doc.Name)
	}
	if !doc.IsScratch() {
		t.Error("expected document to be scratch")
	}
}

func TestNewScratchDocument(t *testing.T) {
	doc := NewScratchDocument()

	if doc.Path != "" {
		t.Errorf("expected empty path, got '%s'", doc.Path)
	}
	if doc.Name != "Untitled" {
		t.Errorf("expected name 'Untitled', got '%s'", doc.Name)
	}
	if doc.Engine == nil {
		t.Error("expected engine to be initialized")
	}
	if !doc.IsScratch() {
		t.Error("expected document to be scratch")
	}
}

func TestDocument_Modified(t *testing.T) {
	doc := NewScratchDocument()

	if doc.IsModified() {
		t.Error("expected document to not be modified initially")
	}

	doc.SetModified(true)
	if !doc.IsModified() {
		t.Error("expected document to be modified")
	}

	doc.SetModified(false)
	if doc.IsModified() {
		t.Error("expected document to not be modified")
	}
}

func TestDocument_Version(t *testing.T) {
	doc := NewScratchDocument()

	if doc.Version() != 0 {
		t.Errorf("expected initial version 0, got %d", doc.Version())
	}

	v1 := doc.IncrementVersion()
	if v1 != 1 {
		t.Errorf("expected version 1, got %d", v1)
	}
	if doc.Version() != 1 {
		t.Errorf("expected version 1, got %d", doc.Version())
	}

	v2 := doc.IncrementVersion()
	if v2 != 2 {
		t.Errorf("expected version 2, got %d", v2)
	}
}

func TestDocument_LSPOpened(t *testing.T) {
	doc := NewScratchDocument()

	if doc.IsLSPOpened() {
		t.Error("expected LSP to not be opened initially")
	}

	doc.SetLSPOpened(true)
	if !doc.IsLSPOpened() {
		t.Error("expected LSP to be opened")
	}

	doc.SetLSPOpened(false)
	if doc.IsLSPOpened() {
		t.Error("expected LSP to not be opened")
	}
}

func TestDocument_Content(t *testing.T) {
	content := "Hello, World!"
	doc := NewDocument("/path/to/file.txt", []byte(content))

	if doc.Content() != content {
		t.Errorf("expected content '%s', got '%s'", content, doc.Content())
	}
}

func TestDocumentManager_CreateScratch(t *testing.T) {
	dm := NewDocumentManager()

	doc1 := dm.CreateScratch()
	if doc1 == nil {
		t.Fatal("CreateScratch() returned nil")
	}
	if doc1.Name != "Untitled" {
		t.Errorf("expected name 'Untitled', got '%s'", doc1.Name)
	}

	doc2 := dm.CreateScratch()
	if doc2 == nil {
		t.Fatal("CreateScratch() returned nil")
	}
	if doc2.Name != "Untitled-2" {
		t.Errorf("expected name 'Untitled-2', got '%s'", doc2.Name)
	}

	if dm.Count() != 2 {
		t.Errorf("expected 2 documents, got %d", dm.Count())
	}
}

func TestDocumentManager_Open(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	dm := NewDocumentManager()

	doc, err := dm.Open(testFile)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	if doc == nil {
		t.Fatal("Open() returned nil document")
	}
	if doc.Name != "test.txt" {
		t.Errorf("expected name 'test.txt', got '%s'", doc.Name)
	}
	if doc.Content() != "test content" {
		t.Errorf("expected content 'test content', got '%s'", doc.Content())
	}
}

func TestDocumentManager_Open_AlreadyOpen(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	dm := NewDocumentManager()

	doc1, err := dm.Open(testFile)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	// Open again should return same document
	doc2, err := dm.Open(testFile)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	if doc1 != doc2 {
		t.Error("expected same document for duplicate Open()")
	}
	if dm.Count() != 1 {
		t.Errorf("expected 1 document, got %d", dm.Count())
	}
}

func TestDocumentManager_Open_NotFound(t *testing.T) {
	dm := NewDocumentManager()

	_, err := dm.Open("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestDocumentManager_Close(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	dm := NewDocumentManager()

	doc, _ := dm.Open(testFile)

	err := dm.Close(doc.Path)
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	if dm.Count() != 0 {
		t.Errorf("expected 0 documents after close, got %d", dm.Count())
	}
}

func TestDocumentManager_Close_NotFound(t *testing.T) {
	dm := NewDocumentManager()

	err := dm.Close("/nonexistent/file.txt")
	if err != ErrDocumentNotFound {
		t.Errorf("expected ErrDocumentNotFound, got %v", err)
	}
}

func TestDocumentManager_Active(t *testing.T) {
	dm := NewDocumentManager()

	// No active document initially
	if dm.Active() != nil {
		t.Error("expected nil active document initially")
	}

	doc := dm.CreateScratch()
	active := dm.Active()
	if active != doc {
		t.Error("expected newly created document to be active")
	}
}

func TestDocumentManager_SetActive(t *testing.T) {
	dm := NewDocumentManager()

	doc1 := dm.CreateScratch()
	doc2 := dm.CreateScratch()

	dm.SetActive(doc1)
	if dm.Active() != doc1 {
		t.Error("expected doc1 to be active")
	}

	dm.SetActive(doc2)
	if dm.Active() != doc2 {
		t.Error("expected doc2 to be active")
	}
}

func TestDocumentManager_Get(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	dm := NewDocumentManager()

	doc, _ := dm.Open(testFile)

	found, exists := dm.Get(doc.Path)
	if !exists {
		t.Error("expected document to exist")
	}
	if found != doc {
		t.Error("expected same document")
	}

	_, exists = dm.Get("/nonexistent/path")
	if exists {
		t.Error("expected document to not exist")
	}
}

func TestDocumentManager_All(t *testing.T) {
	dm := NewDocumentManager()

	doc1 := dm.CreateScratch()
	doc2 := dm.CreateScratch()
	doc3 := dm.CreateScratch()

	all := dm.All()
	if len(all) != 3 {
		t.Errorf("expected 3 documents, got %d", len(all))
	}

	// Verify order (FIFO)
	if all[0] != doc1 || all[1] != doc2 || all[2] != doc3 {
		t.Error("expected documents in order of creation")
	}
}

func TestDocumentManager_DirtyDocuments(t *testing.T) {
	dm := NewDocumentManager()

	doc1 := dm.CreateScratch()
	doc2 := dm.CreateScratch()
	doc3 := dm.CreateScratch()

	doc1.SetModified(true)
	doc3.SetModified(true)

	dirty := dm.DirtyDocuments()
	if len(dirty) != 2 {
		t.Errorf("expected 2 dirty documents, got %d", len(dirty))
	}

	// Verify doc2 is not in dirty list
	for _, d := range dirty {
		if d == doc2 {
			t.Error("expected doc2 to not be in dirty list")
		}
	}
}

func TestDocumentManager_HasDirty(t *testing.T) {
	dm := NewDocumentManager()

	doc := dm.CreateScratch()

	if dm.HasDirty() {
		t.Error("expected no dirty documents")
	}

	doc.SetModified(true)
	if !dm.HasDirty() {
		t.Error("expected dirty documents")
	}
}

func TestDocumentManager_Next(t *testing.T) {
	dm := NewDocumentManager()

	doc1 := dm.CreateScratch()
	doc2 := dm.CreateScratch()
	doc3 := dm.CreateScratch()

	// Active is doc3 (last created)
	dm.SetActive(doc1)

	// Next should cycle to doc2
	next := dm.Next()
	if next != doc2 {
		t.Error("expected next to be doc2")
	}

	// Next should cycle to doc3
	next = dm.Next()
	if next != doc3 {
		t.Error("expected next to be doc3")
	}

	// Next should wrap to doc1
	next = dm.Next()
	if next != doc1 {
		t.Error("expected next to wrap to doc1")
	}
}

func TestDocumentManager_Previous(t *testing.T) {
	dm := NewDocumentManager()

	doc1 := dm.CreateScratch()
	doc2 := dm.CreateScratch()
	doc3 := dm.CreateScratch()

	// Active is doc3 (last created)
	dm.SetActive(doc3)

	// Previous should go to doc2
	prev := dm.Previous()
	if prev != doc2 {
		t.Error("expected previous to be doc2")
	}

	// Previous should go to doc1
	prev = dm.Previous()
	if prev != doc1 {
		t.Error("expected previous to be doc1")
	}

	// Previous should wrap to doc3
	prev = dm.Previous()
	if prev != doc3 {
		t.Error("expected previous to wrap to doc3")
	}
}

func TestDocumentManager_CloseUpdatesActive(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "test1.txt")
	file2 := filepath.Join(tmpDir, "test2.txt")
	if err := os.WriteFile(file1, []byte("test1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("test2"), 0644); err != nil {
		t.Fatal(err)
	}

	dm := NewDocumentManager()

	doc1, _ := dm.Open(file1)
	doc2, _ := dm.Open(file2)

	// doc2 is active (last opened)
	if dm.Active() != doc2 {
		t.Fatal("expected doc2 to be active")
	}

	// Close active document
	dm.Close(doc2.Path)

	// doc1 should now be active
	if dm.Active() != doc1 {
		t.Error("expected doc1 to be active after closing doc2")
	}
}

func TestScratchKey(t *testing.T) {
	key1 := scratchKey(1)
	key2 := scratchKey(2)

	if key1 == key2 {
		t.Error("scratch keys should be unique")
	}
	if key1 != "::scratch::1" {
		t.Errorf("expected '::scratch::1', got '%s'", key1)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{999999, "999999"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		if result != tt.expected {
			t.Errorf("itoa(%d) = '%s', expected '%s'", tt.input, result, tt.expected)
		}
	}
}
