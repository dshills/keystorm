package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	w := New()
	if w == nil {
		t.Fatal("New() returned nil")
	}
	if w.interval != 500*time.Millisecond {
		t.Errorf("default interval = %v, want 500ms", w.interval)
	}
	if w.debounce != 100*time.Millisecond {
		t.Errorf("default debounce = %v, want 100ms", w.debounce)
	}
}

func TestNew_WithOptions(t *testing.T) {
	w := New(
		WithInterval(200*time.Millisecond),
		WithDebounce(50*time.Millisecond),
	)

	if w.interval != 200*time.Millisecond {
		t.Errorf("interval = %v, want 200ms", w.interval)
	}
	if w.debounce != 50*time.Millisecond {
		t.Errorf("debounce = %v, want 50ms", w.debounce)
	}
}

func TestOperation_String(t *testing.T) {
	tests := []struct {
		op   Operation
		want string
	}{
		{OpWrite, "write"},
		{OpCreate, "create"},
		{OpRemove, "remove"},
		{OpRename, "rename"},
		{Operation(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestWatcher_Watch(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.toml")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	w := New()

	// Watch existing file
	if err := w.Watch(tmpFile); err != nil {
		t.Errorf("Watch() error = %v", err)
	}

	files := w.WatchedFiles()
	if len(files) != 1 {
		t.Errorf("WatchedFiles() = %d files, want 1", len(files))
	}

	// Watch non-existent file (should succeed - watching for creation)
	nonExistent := filepath.Join(tmpDir, "nonexistent.toml")
	if err := w.Watch(nonExistent); err != nil {
		t.Errorf("Watch() for non-existent file error = %v", err)
	}

	files = w.WatchedFiles()
	if len(files) != 2 {
		t.Errorf("WatchedFiles() = %d files, want 2", len(files))
	}
}

func TestWatcher_Unwatch(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.toml")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	w := New()
	_ = w.Watch(tmpFile)

	if err := w.Unwatch(tmpFile); err != nil {
		t.Errorf("Unwatch() error = %v", err)
	}

	files := w.WatchedFiles()
	if len(files) != 0 {
		t.Errorf("WatchedFiles() = %d files, want 0", len(files))
	}
}

func TestWatcher_WatchDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	for _, name := range []string{"a.toml", "b.toml", "c.json"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	w := New()
	if err := w.WatchDir(tmpDir, "*.toml"); err != nil {
		t.Errorf("WatchDir() error = %v", err)
	}

	files := w.WatchedFiles()
	if len(files) != 2 {
		t.Errorf("WatchedFiles() = %d files, want 2", len(files))
	}
}

func TestWatcher_StartStop(t *testing.T) {
	w := New(WithInterval(50 * time.Millisecond))

	if w.IsRunning() {
		t.Error("IsRunning() = true before Start()")
	}

	w.Start()
	if !w.IsRunning() {
		t.Error("IsRunning() = false after Start()")
	}

	// Start again should be idempotent
	w.Start()
	if !w.IsRunning() {
		t.Error("IsRunning() = false after second Start()")
	}

	w.Stop()
	if w.IsRunning() {
		t.Error("IsRunning() = true after Stop()")
	}

	// Stop again should be idempotent
	w.Stop()
	if w.IsRunning() {
		t.Error("IsRunning() = true after second Stop()")
	}
}

func TestWatcher_DetectsFileModification(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.toml")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w := New(
		WithInterval(20*time.Millisecond),
		WithDebounce(0), // Disable debounce for faster test
	)

	var eventReceived atomic.Bool
	var receivedEvent Event
	var mu sync.Mutex

	w.OnChange(func(event Event) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
		eventReceived.Store(true)
	})

	_ = w.Watch(tmpFile)
	w.Start()
	defer w.Stop()

	// Modify the file
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(tmpFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for event
	deadline := time.Now().Add(500 * time.Millisecond)
	for !eventReceived.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !eventReceived.Load() {
		t.Fatal("did not receive file change event")
	}

	mu.Lock()
	if receivedEvent.Op != OpWrite {
		t.Errorf("event.Op = %v, want OpWrite", receivedEvent.Op)
	}
	if receivedEvent.Path != tmpFile {
		t.Errorf("event.Path = %q, want %q", receivedEvent.Path, tmpFile)
	}
	mu.Unlock()
}

func TestWatcher_DetectsFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "new.toml")

	w := New(
		WithInterval(20*time.Millisecond),
		WithDebounce(0),
	)

	var eventReceived atomic.Bool
	var receivedEvent Event
	var mu sync.Mutex

	w.OnChange(func(event Event) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
		eventReceived.Store(true)
	})

	// Watch non-existent file
	_ = w.Watch(tmpFile)
	w.Start()
	defer w.Stop()

	// Create the file
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(tmpFile, []byte("created"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for event
	deadline := time.Now().Add(500 * time.Millisecond)
	for !eventReceived.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !eventReceived.Load() {
		t.Fatal("did not receive file creation event")
	}

	mu.Lock()
	if receivedEvent.Op != OpCreate {
		t.Errorf("event.Op = %v, want OpCreate", receivedEvent.Op)
	}
	mu.Unlock()
}

func TestWatcher_DetectsFileDeletion(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "delete.toml")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w := New(
		WithInterval(20*time.Millisecond),
		WithDebounce(0),
	)

	var eventReceived atomic.Bool
	var receivedEvent Event
	var mu sync.Mutex

	w.OnChange(func(event Event) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
		eventReceived.Store(true)
	})

	_ = w.Watch(tmpFile)
	w.Start()
	defer w.Stop()

	// Delete the file
	time.Sleep(50 * time.Millisecond)
	if err := os.Remove(tmpFile); err != nil {
		t.Fatal(err)
	}

	// Wait for event
	deadline := time.Now().Add(500 * time.Millisecond)
	for !eventReceived.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !eventReceived.Load() {
		t.Fatal("did not receive file deletion event")
	}

	mu.Lock()
	if receivedEvent.Op != OpRemove {
		t.Errorf("event.Op = %v, want OpRemove", receivedEvent.Op)
	}
	mu.Unlock()
}

func TestWatcher_Debounce(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "debounce.toml")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w := New(
		WithInterval(10*time.Millisecond),
		WithDebounce(100*time.Millisecond),
	)

	var eventCount atomic.Int32

	w.OnChange(func(event Event) {
		eventCount.Add(1)
	})

	_ = w.Watch(tmpFile)
	w.Start()
	defer w.Stop()

	// Rapid modifications
	time.Sleep(30 * time.Millisecond)
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(tmpFile, []byte("modified"), 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to settle
	time.Sleep(200 * time.Millisecond)

	// Should have received only 1 debounced event (or possibly 2 at boundaries)
	count := eventCount.Load()
	if count > 2 {
		t.Errorf("received %d events, expected 1-2 (debounced)", count)
	}
}

func TestWatcher_MultipleHandlers(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "multi.toml")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w := New(
		WithInterval(20*time.Millisecond),
		WithDebounce(0),
	)

	var count1, count2 atomic.Int32

	w.OnChange(func(event Event) {
		count1.Add(1)
	})
	w.OnChange(func(event Event) {
		count2.Add(1)
	})

	_ = w.Watch(tmpFile)
	w.Start()
	defer w.Stop()

	// Modify the file
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(tmpFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for events
	time.Sleep(100 * time.Millisecond)

	if count1.Load() < 1 {
		t.Error("handler 1 did not receive event")
	}
	if count2.Load() < 1 {
		t.Error("handler 2 did not receive event")
	}
}
