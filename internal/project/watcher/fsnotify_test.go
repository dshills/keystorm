package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFSNotifyWatcher(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	if w.events == nil {
		t.Error("events channel should not be nil")
	}
	if w.errors == nil {
		t.Error("errors channel should not be nil")
	}
}

func TestFSNotifyWatcher_WatchUnwatch(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()

	// Watch directory
	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch error = %v", err)
	}

	if !w.IsWatching(tmpDir) {
		t.Error("should be watching tmpDir")
	}

	// Watch again should error
	if err := w.Watch(tmpDir); err != ErrAlreadyWatching {
		t.Errorf("Watch again error = %v, want ErrAlreadyWatching", err)
	}

	// Unwatch
	if err := w.Unwatch(tmpDir); err != nil {
		t.Fatalf("Unwatch error = %v", err)
	}

	if w.IsWatching(tmpDir) {
		t.Error("should not be watching tmpDir after Unwatch")
	}

	// Unwatch again should error
	if err := w.Unwatch(tmpDir); err != ErrNotWatching {
		t.Errorf("Unwatch again error = %v, want ErrNotWatching", err)
	}
}

func TestFSNotifyWatcher_WatchNonexistent(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	err = w.Watch("/nonexistent/path/that/does/not/exist")
	if err != ErrPathNotExist {
		t.Errorf("Watch nonexistent error = %v, want ErrPathNotExist", err)
	}
}

func TestFSNotifyWatcher_WatchRecursive(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()

	// Create subdirectories
	subDir1 := filepath.Join(tmpDir, "sub1")
	subDir2 := filepath.Join(tmpDir, "sub1", "sub2")
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	// Watch recursively
	if err := w.WatchRecursive(tmpDir); err != nil {
		t.Fatalf("WatchRecursive error = %v", err)
	}

	// All directories should be watched
	if !w.IsWatching(tmpDir) {
		t.Error("should be watching tmpDir")
	}
	if !w.IsWatching(subDir1) {
		t.Error("should be watching sub1")
	}
	if !w.IsWatching(subDir2) {
		t.Error("should be watching sub2")
	}
}

func TestFSNotifyWatcher_WatchedPaths(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Mkdir error = %v", err)
	}

	_ = w.Watch(tmpDir)
	_ = w.Watch(subDir)

	paths := w.WatchedPaths()
	if len(paths) != 2 {
		t.Errorf("WatchedPaths count = %d, want 2", len(paths))
	}
}

func TestFSNotifyWatcher_Stats(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	_ = w.Watch(tmpDir)

	stats := w.Stats()
	if stats.WatchedPaths != 1 {
		t.Errorf("WatchedPaths = %d, want 1", stats.WatchedPaths)
	}
	if stats.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
}

func TestFSNotifyWatcher_Close(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}

	tmpDir := t.TempDir()
	_ = w.Watch(tmpDir)

	// Close should succeed
	if err := w.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}

	// Operations after close should error
	if err := w.Watch(tmpDir); err != ErrWatcherClosed {
		t.Errorf("Watch after close error = %v, want ErrWatcherClosed", err)
	}

	// Close again should be safe
	if err := w.Close(); err != nil {
		t.Errorf("Close again error = %v", err)
	}
}

func TestFSNotifyWatcher_FileEvents(t *testing.T) {
	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch error = %v", err)
	}

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Wait for create event - may receive multiple events, drain until we get create
	gotCreate := false
	timeout := time.After(2 * time.Second)
createLoop:
	for {
		select {
		case event := <-w.Events():
			if event.Path == testFile && event.Op.Has(OpCreate) {
				gotCreate = true
				break createLoop
			}
		case <-timeout:
			break createLoop
		}
	}
	if !gotCreate {
		t.Error("timeout waiting for create event")
	}

	// Give a small delay to let any pending events clear
	time.Sleep(100 * time.Millisecond)

	// Drain any remaining events from the create operation
drainCreate:
	for {
		select {
		case <-w.Events():
		default:
			break drainCreate
		}
	}

	// Write to file (append to trigger write event)
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile error = %v", err)
	}
	_, _ = f.WriteString(" world")
	_ = f.Close()

	// Wait for write event
	gotWrite := false
	timeout = time.After(2 * time.Second)
writeLoop:
	for {
		select {
		case event := <-w.Events():
			if event.Path == testFile && event.Op.Has(OpWrite) {
				gotWrite = true
				break writeLoop
			}
		case <-timeout:
			break writeLoop
		}
	}
	if !gotWrite {
		t.Error("timeout waiting for write event")
	}

	// Give a small delay
	time.Sleep(100 * time.Millisecond)

	// Drain any remaining events
drainWrite:
	for {
		select {
		case <-w.Events():
		default:
			break drainWrite
		}
	}

	// Remove file
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Remove error = %v", err)
	}

	// Wait for remove event
	gotRemove := false
	timeout = time.After(2 * time.Second)
removeLoop:
	for {
		select {
		case event := <-w.Events():
			if event.Path == testFile && event.Op.Has(OpRemove) {
				gotRemove = true
				break removeLoop
			}
		case <-timeout:
			break removeLoop
		}
	}
	if !gotRemove {
		t.Error("timeout waiting for remove event")
	}
}

func TestFSNotifyWatcher_IgnorePatterns(t *testing.T) {
	w, err := NewFSNotifyWatcher(
		WithIgnorePatterns([]string{"*.log", "temp/"}),
	)
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch error = %v", err)
	}

	// Create a .log file (should be ignored)
	logFile := filepath.Join(tmpDir, "debug.log")
	if err := os.WriteFile(logFile, []byte("log"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Create a regular file (should not be ignored)
	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("txt"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// We should only receive the txt file event
	gotTxt := false
	gotLog := false

	timeout := time.After(1 * time.Second)
	for {
		select {
		case event := <-w.Events():
			if event.Path == txtFile {
				gotTxt = true
			}
			if event.Path == logFile {
				gotLog = true
			}
		case <-timeout:
			goto done
		}
	}
done:

	if !gotTxt {
		t.Error("should have received event for test.txt")
	}
	if gotLog {
		t.Error("should NOT have received event for debug.log (ignored)")
	}
}

func TestFSNotifyWatcher_IgnoreHidden(t *testing.T) {
	w, err := NewFSNotifyWatcher(
		WithIgnoreHidden(true),
	)
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch error = %v", err)
	}

	// Create a hidden file (should be ignored)
	hiddenFile := filepath.Join(tmpDir, ".hidden")
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("regular"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	gotRegular := false
	gotHidden := false

	timeout := time.After(1 * time.Second)
	for {
		select {
		case event := <-w.Events():
			if event.Path == regularFile {
				gotRegular = true
			}
			if event.Path == hiddenFile {
				gotHidden = true
			}
		case <-timeout:
			goto done
		}
	}
done:

	if !gotRegular {
		t.Error("should have received event for regular.txt")
	}
	if gotHidden {
		t.Error("should NOT have received event for .hidden (ignored)")
	}
}

func TestFSNotifyWatcher_MaxWatches(t *testing.T) {
	w, err := NewFSNotifyWatcher(
		WithMaxWatches(2),
	)
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	dir3 := filepath.Join(tmpDir, "dir3")

	for _, d := range []string{dir1, dir2, dir3} {
		if err := os.Mkdir(d, 0755); err != nil {
			t.Fatalf("Mkdir error = %v", err)
		}
	}

	// First two should succeed
	if err := w.Watch(dir1); err != nil {
		t.Errorf("Watch dir1 error = %v", err)
	}
	if err := w.Watch(dir2); err != nil {
		t.Errorf("Watch dir2 error = %v", err)
	}

	// Third should fail
	if err := w.Watch(dir3); err == nil {
		t.Error("Watch dir3 should fail (max watches reached)")
	}
}

func TestFSNotifyWatcher_EventFilter(t *testing.T) {
	writeOnly := func(e Event) bool {
		return e.Op.Has(OpWrite)
	}

	w, err := NewFSNotifyWatcher(
		WithEventFilter(writeOnly),
	)
	if err != nil {
		t.Fatalf("NewFSNotifyWatcher error = %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch error = %v", err)
	}

	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file (should be filtered out)
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Give time for create event to be processed
	time.Sleep(100 * time.Millisecond)

	// Write to file (should pass filter)
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// We should only receive write event, not create
	gotCreate := false
	gotWrite := false

	timeout := time.After(1 * time.Second)
	for {
		select {
		case event := <-w.Events():
			if event.Op.Has(OpCreate) {
				gotCreate = true
			}
			if event.Op.Has(OpWrite) {
				gotWrite = true
			}
		case <-timeout:
			goto done
		}
	}
done:

	if gotCreate {
		t.Error("should NOT have received create event (filtered)")
	}
	if !gotWrite {
		t.Error("should have received write event")
	}
}
