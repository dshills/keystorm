package lsp

import (
	"sync"
	"testing"
	"time"
)

func TestNewDocumentManager(t *testing.T) {
	dm := NewDocumentManager(nil)
	if dm == nil {
		t.Fatal("NewDocumentManager returned nil")
	}

	if dm.debounceDelay != 300*time.Millisecond {
		t.Errorf("Expected default debounce 300ms, got %v", dm.debounceDelay)
	}
}

func TestDocumentManager_WithOptions(t *testing.T) {
	var callbackCalled bool
	callback := func(uri DocumentURI, diags []Diagnostic) {
		callbackCalled = true
	}

	dm := NewDocumentManager(nil,
		WithDebounceDelay(100*time.Millisecond),
		WithDiagnosticsHandler(callback),
	)

	if dm.debounceDelay != 100*time.Millisecond {
		t.Errorf("Expected debounce 100ms, got %v", dm.debounceDelay)
	}

	if dm.onDiagnostics == nil {
		t.Error("Expected diagnostics callback to be set")
	}

	_ = callbackCalled // Unused in this test
}

func TestDocumentManager_OpenClose(t *testing.T) {
	dm := NewDocumentManager(nil)

	// Open document
	err := dm.OpenDocument("/test/file.go", "go", "package main")
	if err != nil {
		t.Fatalf("OpenDocument error: %v", err)
	}

	// Check it's open
	if !dm.IsOpen("/test/file.go") {
		t.Error("Document should be open")
	}

	// Get document
	doc, ok := dm.GetDocument("/test/file.go")
	if !ok {
		t.Fatal("GetDocument failed")
	}

	if doc.LanguageID != "go" {
		t.Errorf("Expected languageID 'go', got %s", doc.LanguageID)
	}

	if doc.Content != "package main" {
		t.Errorf("Expected content 'package main', got %s", doc.Content)
	}

	if doc.Version != 1 {
		t.Errorf("Expected version 1, got %d", doc.Version)
	}

	// Try opening again - should fail
	err = dm.OpenDocument("/test/file.go", "go", "different")
	if err != ErrDocumentAlreadyOpen {
		t.Errorf("Expected ErrDocumentAlreadyOpen, got %v", err)
	}

	// Close document
	err = dm.CloseDocument("/test/file.go")
	if err != nil {
		t.Fatalf("CloseDocument error: %v", err)
	}

	// Check it's closed
	if dm.IsOpen("/test/file.go") {
		t.Error("Document should be closed")
	}

	// Close again - should fail
	err = dm.CloseDocument("/test/file.go")
	if err != ErrDocumentNotOpen {
		t.Errorf("Expected ErrDocumentNotOpen, got %v", err)
	}
}

func TestDocumentManager_ChangeDocument(t *testing.T) {
	dm := NewDocumentManager(nil, WithDebounceDelay(50*time.Millisecond))

	// Open document
	dm.OpenDocument("/test/file.go", "go", "line1\nline2\nline3")

	// Make a change
	err := dm.ReplaceContent("/test/file.go", "new content")
	if err != nil {
		t.Fatalf("ReplaceContent error: %v", err)
	}

	// Check content updated
	content, ok := dm.GetContent("/test/file.go")
	if !ok {
		t.Fatal("GetContent failed")
	}

	if content != "new content" {
		t.Errorf("Expected 'new content', got %s", content)
	}

	// Check version incremented
	version, _ := dm.GetVersion("/test/file.go")
	if version != 2 {
		t.Errorf("Expected version 2, got %d", version)
	}

	// Check dirty flag
	if !dm.IsDirty("/test/file.go") {
		t.Error("Document should be dirty")
	}

	dm.CloseDocument("/test/file.go")
}

func TestDocumentManager_IncrementalChange(t *testing.T) {
	dm := NewDocumentManager(nil)

	// Open document
	dm.OpenDocument("/test/file.go", "go", "line1\nline2\nline3\n")

	// Make incremental change - replace "line2" with "modified"
	change := TextDocumentContentChangeEvent{
		Range: &Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 5},
		},
		Text: "modified",
	}

	err := dm.ChangeDocument("/test/file.go", []TextDocumentContentChangeEvent{change})
	if err != nil {
		t.Fatalf("ChangeDocument error: %v", err)
	}

	content, _ := dm.GetContent("/test/file.go")
	expected := "line1\nmodified\nline3\n"
	if content != expected {
		t.Errorf("Expected %q, got %q", expected, content)
	}

	dm.CloseDocument("/test/file.go")
}

func TestDocumentManager_FlushPending(t *testing.T) {
	dm := NewDocumentManager(nil, WithDebounceDelay(1*time.Second))

	dm.OpenDocument("/test/file.go", "go", "initial")
	dm.ReplaceContent("/test/file.go", "changed")

	// Should have a pending timer
	dm.mu.RLock()
	pending := len(dm.pendingTimers)
	dm.mu.RUnlock()

	if pending != 1 {
		t.Errorf("Expected 1 pending timer, got %d", pending)
	}

	// Flush
	dm.FlushPending("/test/file.go")

	// Should have no pending timers
	dm.mu.RLock()
	pending = len(dm.pendingTimers)
	dm.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected 0 pending timers after flush, got %d", pending)
	}

	dm.CloseDocument("/test/file.go")
}

func TestDocumentManager_OpenDocuments(t *testing.T) {
	dm := NewDocumentManager(nil)

	dm.OpenDocument("/test/a.go", "go", "a")
	dm.OpenDocument("/test/b.py", "python", "b")
	dm.OpenDocument("/test/c.js", "javascript", "c")

	uris := dm.OpenDocuments()
	if len(uris) != 3 {
		t.Errorf("Expected 3 open documents, got %d", len(uris))
	}

	paths := dm.OpenDocumentPaths()
	if len(paths) != 3 {
		t.Errorf("Expected 3 paths, got %d", len(paths))
	}

	dm.CloseAll()

	if len(dm.OpenDocuments()) != 0 {
		t.Error("Expected 0 documents after CloseAll")
	}
}

func TestDocumentManager_Stats(t *testing.T) {
	dm := NewDocumentManager(nil)

	dm.OpenDocument("/test/a.go", "go", "a")
	dm.OpenDocument("/test/b.go", "go", "b")
	dm.OpenDocument("/test/c.py", "python", "c")
	dm.ReplaceContent("/test/a.go", "modified")

	stats := dm.Stats()

	if stats.TotalOpen != 3 {
		t.Errorf("Expected 3 open, got %d", stats.TotalOpen)
	}

	if stats.TotalDirty != 1 {
		t.Errorf("Expected 1 dirty, got %d", stats.TotalDirty)
	}

	if stats.ByLanguage["go"] != 2 {
		t.Errorf("Expected 2 Go files, got %d", stats.ByLanguage["go"])
	}

	if stats.ByLanguage["python"] != 1 {
		t.Errorf("Expected 1 Python file, got %d", stats.ByLanguage["python"])
	}

	dm.CloseAll()
}

func TestDocumentManager_ConcurrentAccess(t *testing.T) {
	dm := NewDocumentManager(nil, WithDebounceDelay(10*time.Millisecond))

	dm.OpenDocument("/test/file.go", "go", "initial")

	var wg sync.WaitGroup

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				dm.GetContent("/test/file.go")
				dm.IsOpen("/test/file.go")
				dm.IsDirty("/test/file.go")
			}
		}()
	}

	// Concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				dm.ReplaceContent("/test/file.go", "content from writer")
			}
		}(i)
	}

	wg.Wait()
	dm.CloseDocument("/test/file.go")
}

func TestApplyTextChange(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		rng      Range
		newText  string
		expected string
	}{
		{
			name:    "replace word",
			content: "hello world",
			rng: Range{
				Start: Position{Line: 0, Character: 6},
				End:   Position{Line: 0, Character: 11},
			},
			newText:  "universe",
			expected: "hello universe",
		},
		{
			name:    "insert at beginning",
			content: "world",
			rng: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 0},
			},
			newText:  "hello ",
			expected: "hello world",
		},
		{
			name:    "multiline replace",
			content: "line1\nline2\nline3",
			rng: Range{
				Start: Position{Line: 0, Character: 5},
				End:   Position{Line: 2, Character: 0},
			},
			newText:  "\nnew\n",
			expected: "line1\nnew\nline3",
		},
		{
			name:    "delete",
			content: "hello world",
			rng: Range{
				Start: Position{Line: 0, Character: 5},
				End:   Position{Line: 0, Character: 11},
			},
			newText:  "",
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyTextChange(tt.content, tt.rng, tt.newText)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		content  string
		expected []string
	}{
		{"", []string{""}},
		{"line", []string{"line"}},
		{"line1\nline2", []string{"line1", "line2"}},
		{"line1\nline2\n", []string{"line1", "line2", ""}},
		{"\n", []string{"", ""}},
		{"\n\n", []string{"", "", ""}},
	}

	for _, tt := range tests {
		result := splitLines(tt.content)
		if len(result) != len(tt.expected) {
			t.Errorf("splitLines(%q): got %d lines, expected %d", tt.content, len(result), len(tt.expected))
			continue
		}
		for i, line := range result {
			if line != tt.expected[i] {
				t.Errorf("splitLines(%q)[%d]: got %q, expected %q", tt.content, i, line, tt.expected[i])
			}
		}
	}
}
