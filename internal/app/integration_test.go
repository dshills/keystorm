package app

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/event"
)

// TestIntegration_OpenEditSaveFlow tests the complete file open, edit, save flow.
func TestIntegration_OpenEditSaveFlow(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "Hello World"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create application with the file
	app, err := New(Options{
		Files: []string{testFile},
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	// Verify file was opened
	doc := app.Documents().Active()
	if doc == nil {
		t.Fatal("expected active document")
	}
	if doc.Content() != initialContent {
		t.Errorf("expected content '%s', got '%s'", initialContent, doc.Content())
	}

	// Simulate edit via engine
	doc.Engine.Insert(6, "Beautiful ")
	doc.SetModified(true)

	// Verify content changed
	expectedContent := "Hello Beautiful World"
	if doc.Content() != expectedContent {
		t.Errorf("expected content '%s', got '%s'", expectedContent, doc.Content())
	}
	if !doc.IsModified() {
		t.Error("expected document to be modified")
	}
}

// TestIntegration_MultipleDocuments tests managing multiple open documents.
func TestIntegration_MultipleDocuments(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files
	files := make([]string, 3)
	for i := range files {
		files[i] = filepath.Join(tmpDir, "file"+itoa(i+1)+".txt")
		if err := os.WriteFile(files[i], []byte("content "+itoa(i+1)), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	app, err := New(Options{
		Files: files,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	// Should have 3 documents
	if app.Documents().Count() != 3 {
		t.Errorf("expected 3 documents, got %d", app.Documents().Count())
	}

	// Navigate through documents
	dm := app.Documents()
	initial := dm.Active()

	next := dm.Next()
	if next == initial {
		t.Error("expected Next() to return different document")
	}

	prev := dm.Previous()
	if prev != initial {
		t.Error("expected Previous() to return to initial document")
	}
}

// TestIntegration_EventBusSubscriptions tests that event subscriptions work correctly.
func TestIntegration_EventBusSubscriptions(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	var modeChangeReceived atomic.Bool

	// Subscribe to mode changes
	_, err = app.EventBus().SubscribeFunc(
		TopicModeChanged,
		func(ctx context.Context, ev any) error {
			modeChangeReceived.Store(true)
			return nil
		},
		event.WithDeliveryMode(event.DeliverySync),
	)
	if err != nil {
		t.Fatalf("Subscribe() failed: %v", err)
	}

	// Publish mode change
	ctx := context.Background()
	err = app.PublishModeChange(ctx, "normal", "insert")
	if err != nil {
		t.Fatalf("PublishModeChange() failed: %v", err)
	}

	// Verify event was received
	if !modeChangeReceived.Load() {
		t.Error("expected mode change event to be received")
	}
}

// TestIntegration_ConfigAccess tests that config is properly wired.
func TestIntegration_ConfigAccess(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	config := app.Config()
	if config == nil {
		t.Fatal("expected config to be set")
	}

	// Should be able to get editor settings
	tabSize, err := config.GetInt("editor.tabSize")
	if err != nil {
		t.Errorf("GetInt() failed: %v", err)
	}
	if tabSize == 0 {
		t.Error("expected editor.tabSize to have a default value")
	}
}

// TestIntegration_ModeManagerAccess tests that mode manager is properly wired.
func TestIntegration_ModeManagerAccess(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	mm := app.ModeManager()
	if mm == nil {
		t.Fatal("expected mode manager to be set")
	}

	// Should have modes registered
	modes := mm.Modes()
	if len(modes) == 0 {
		t.Error("expected modes to be registered")
	}
}

// TestIntegration_ConcurrentAccess tests thread-safe access to app components.
func TestIntegration_ConcurrentAccess(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	done := make(chan struct{})

	// Start multiple goroutines accessing different components
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			_ = app.Documents().Active()
			_ = app.Documents().Count()
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			_ = app.EventBus()
			_ = app.Config()
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			_ = app.ModeManager()
			_ = app.Dispatcher()
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			_ = app.Logger()
			_ = app.Metrics()
		}
	}()

	// Wait for all goroutines with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 4; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("timeout waiting for goroutines")
		}
	}
}

// TestIntegration_DocumentModificationTracking tests that modifications are tracked correctly.
func TestIntegration_DocumentModificationTracking(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	app, err := New(Options{
		Files: []string{testFile},
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	doc := app.Documents().Active()

	// Initially not modified
	if doc.IsModified() {
		t.Error("expected document to not be modified initially")
	}
	if app.Documents().HasDirty() {
		t.Error("expected no dirty documents initially")
	}

	// Make a change
	doc.Engine.Insert(0, "prefix ")
	doc.SetModified(true)

	// Now should be modified
	if !doc.IsModified() {
		t.Error("expected document to be modified")
	}
	if !app.Documents().HasDirty() {
		t.Error("expected dirty documents")
	}

	dirty := app.Documents().DirtyDocuments()
	if len(dirty) != 1 {
		t.Errorf("expected 1 dirty document, got %d", len(dirty))
	}
}

// TestIntegration_LSPLanguageDetection tests that language detection works for documents.
func TestIntegration_LSPLanguageDetection(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"test.go":   "go",
		"test.py":   "python",
		"test.js":   "javascript",
		"test.ts":   "typescript",
		"test.rs":   "rust",
		"test.c":    "c",
		"test.cpp":  "cpp",
		"test.java": "java",
		"test.rb":   "ruby",
		"test.md":   "markdown",
	}

	for filename, expectedLang := range files {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		doc := NewDocument(path, []byte("content"))
		if doc.LanguageID != expectedLang {
			t.Errorf("file %s: expected language '%s', got '%s'", filename, expectedLang, doc.LanguageID)
		}
	}
}

// TestIntegration_MetricsCollection tests that metrics are collected during operation.
func TestIntegration_MetricsCollection(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	metrics := app.Metrics()

	// Record some metrics
	metrics.RecordFrame(16 * time.Millisecond)
	metrics.RecordFrame(17 * time.Millisecond)
	metrics.RecordInput(1 * time.Millisecond)
	metrics.RecordRender(5 * time.Millisecond)
	metrics.RecordEvent(100 * time.Microsecond)

	snapshot := metrics.Snapshot()

	if snapshot.FrameCount != 2 {
		t.Errorf("expected 2 frames, got %d", snapshot.FrameCount)
	}
	if snapshot.InputCount != 1 {
		t.Errorf("expected 1 input, got %d", snapshot.InputCount)
	}
	if snapshot.RenderCount != 1 {
		t.Errorf("expected 1 render, got %d", snapshot.RenderCount)
	}
	if snapshot.EventCount != 1 {
		t.Errorf("expected 1 event, got %d", snapshot.EventCount)
	}
}

// TestIntegration_LoggingInfrastructure tests that logging works correctly.
func TestIntegration_LoggingInfrastructure(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	logger := app.Logger()
	if logger == nil {
		t.Fatal("expected logger to be available")
	}

	// These should not panic
	app.LogDebug("debug message")
	app.LogInfo("info message")
	app.LogWarn("warn message")
	app.LogError("error message")
}

// TestIntegration_ShutdownCleanup tests that shutdown cleans up properly.
func TestIntegration_ShutdownCleanup(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Start running in background
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Verify running
	if !app.IsRunning() {
		t.Error("expected app to be running")
	}

	// Trigger shutdown
	app.Shutdown()

	// Wait for clean exit
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not complete within timeout")
	}

	// Verify not running
	if app.IsRunning() {
		t.Error("expected app to not be running after shutdown")
	}
}

// TestIntegration_VersionTracking tests document version tracking for LSP.
func TestIntegration_VersionTracking(t *testing.T) {
	doc := NewScratchDocument()

	if doc.Version() != 0 {
		t.Errorf("expected initial version 0, got %d", doc.Version())
	}

	// Simulate edits incrementing version
	v1 := doc.IncrementVersion()
	v2 := doc.IncrementVersion()
	v3 := doc.IncrementVersion()

	if v1 != 1 || v2 != 2 || v3 != 3 {
		t.Errorf("expected versions 1, 2, 3, got %d, %d, %d", v1, v2, v3)
	}
	if doc.Version() != 3 {
		t.Errorf("expected current version 3, got %d", doc.Version())
	}
}
