package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewApplication(t *testing.T) {
	opts := Options{
		WorkspacePath: t.TempDir(),
	}

	app, err := New(opts)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if app == nil {
		t.Fatal("New() returned nil")
	}
	defer app.Shutdown()

	// Verify core components are initialized
	if app.eventBus == nil {
		t.Error("expected eventBus to be initialized")
	}
	if app.config == nil {
		t.Error("expected config to be initialized")
	}
	if app.modeManager == nil {
		t.Error("expected modeManager to be initialized")
	}
	if app.dispatcher == nil {
		t.Error("expected dispatcher to be initialized")
	}
	if app.documents == nil {
		t.Error("expected documents to be initialized")
	}
}

func TestApplication_IsRunning(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	if app.IsRunning() {
		t.Error("expected IsRunning() to be false before Run()")
	}
}

func TestApplication_ShutdownIdempotent(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Should be safe to call multiple times
	app.Shutdown()
	app.Shutdown()
	app.Shutdown()
}

func TestApplication_SetBackend(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	// Should not error before running
	err = app.SetBackend(nil)
	if err != nil {
		t.Errorf("SetBackend() failed: %v", err)
	}
}

func TestApplication_Accessors(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	// Test all accessor methods
	if app.EventBus() == nil {
		t.Error("EventBus() returned nil")
	}
	if app.Config() == nil {
		t.Error("Config() returned nil")
	}
	if app.ModeManager() == nil {
		t.Error("ModeManager() returned nil")
	}
	if app.Dispatcher() == nil {
		t.Error("Dispatcher() returned nil")
	}
	if app.Documents() == nil {
		t.Error("Documents() returned nil")
	}
	// Logger should return default if not set
	if app.Logger() == nil {
		t.Error("Logger() returned nil")
	}
	// Metrics should return default if not set
	if app.Metrics() == nil {
		t.Error("Metrics() returned nil")
	}
}

func TestApplication_OpenFiles(t *testing.T) {
	// Create temp files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "test1.txt")
	file2 := filepath.Join(tmpDir, "test2.go")

	if err := os.WriteFile(file1, []byte("Hello World"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	opts := Options{
		Files: []string{file1, file2},
	}

	app, err := New(opts)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	// Should have opened files
	if app.Documents().Count() != 2 {
		t.Errorf("expected 2 documents, got %d", app.Documents().Count())
	}
}

func TestApplication_NoFiles_CreatesScratch(t *testing.T) {
	opts := Options{
		Files: []string{},
	}

	app, err := New(opts)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	// Should have created scratch buffer
	if app.Documents().Count() != 1 {
		t.Errorf("expected 1 scratch document, got %d", app.Documents().Count())
	}

	doc := app.Documents().Active()
	if doc == nil {
		t.Fatal("expected active document")
	}
	if !doc.IsScratch() {
		t.Error("expected scratch document")
	}
}

func TestApplication_RunWithoutBackend(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Start in background
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Should be running
	if !app.IsRunning() {
		t.Error("expected app to be running")
	}

	// Trigger shutdown
	app.Shutdown()

	// Should exit cleanly
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not exit within timeout")
	}
}

func TestApplication_RunTwice(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Start first instance
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Second call should fail
	err = app.Run()
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}

	app.Shutdown()
	<-done
}

func TestApplication_PublishModeChange(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	ctx := context.Background()
	err = app.PublishModeChange(ctx, "normal", "insert")
	if err != nil {
		t.Errorf("PublishModeChange() failed: %v", err)
	}
}

func TestApplication_PublishFileEvent(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Shutdown()

	ctx := context.Background()
	err = app.PublishFileEvent(ctx, TopicFileOpened, "/path/to/file.go")
	if err != nil {
		t.Errorf("PublishFileEvent() failed: %v", err)
	}
}
