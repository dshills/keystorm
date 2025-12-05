package filestore

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/project/vfs"
)

func TestSyncManager_StartStop(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewSyncManager(store, 50*time.Millisecond)

	if sm.IsRunning() {
		t.Error("sync manager should not be running initially")
	}

	sm.Start()
	if !sm.IsRunning() {
		t.Error("sync manager should be running after Start")
	}

	// Starting again should be safe
	sm.Start()
	if !sm.IsRunning() {
		t.Error("sync manager should still be running")
	}

	sm.Stop()
	if sm.IsRunning() {
		t.Error("sync manager should not be running after Stop")
	}

	// Stopping again should be safe
	sm.Stop()
}

func TestSyncManager_OnExternalChange(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewSyncManager(store, 50*time.Millisecond)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	store.Open(ctx, "/test/file.go")

	var changedDoc *Document
	sm.OnExternalChange(func(doc *Document) {
		changedDoc = doc
	})

	// Modify file
	time.Sleep(10 * time.Millisecond)
	memfs.WriteFile("/test/file.go", []byte("modified"), 0644)

	// Manual check
	sm.CheckNow(ctx)

	if changedDoc == nil {
		t.Error("external change handler should have been called")
	}
	if changedDoc != nil && changedDoc.Path != "/test/file.go" {
		t.Errorf("changed doc path = %q, want %q", changedDoc.Path, "/test/file.go")
	}
}

func TestSyncManager_OnConflict(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewSyncManager(store, 50*time.Millisecond)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	doc, _ := store.Open(ctx, "/test/file.go")

	// Make local change
	doc.SetContent([]byte("local change"))

	var conflictDoc *Document
	sm.OnConflict(func(doc *Document) {
		conflictDoc = doc
	})

	// Make external change
	time.Sleep(10 * time.Millisecond)
	memfs.WriteFile("/test/file.go", []byte("external change"), 0644)

	// Manual check
	sm.CheckNow(ctx)

	if conflictDoc == nil {
		t.Error("conflict handler should have been called")
	}
}

func TestSyncManager_AutoCheck(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewSyncManager(store, 50*time.Millisecond)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	store.Open(ctx, "/test/file.go")

	changeCalls := 0
	sm.OnExternalChange(func(doc *Document) {
		changeCalls++
	})

	sm.Start()
	defer sm.Stop()

	// Modify file
	time.Sleep(10 * time.Millisecond)
	memfs.WriteFile("/test/file.go", []byte("modified"), 0644)

	// Wait for auto-check
	time.Sleep(100 * time.Millisecond)

	if changeCalls == 0 {
		t.Error("external change should have been detected automatically")
	}
}

func TestAutoReloadSyncManager_AutoReloadClean(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewAutoReloadSyncManager(store, 50*time.Millisecond, AutoReloadClean)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	doc, _ := store.Open(ctx, "/test/file.go")

	sm.Start()
	defer sm.Stop()

	// Modify file externally
	time.Sleep(10 * time.Millisecond)
	memfs.WriteFile("/test/file.go", []byte("modified"), 0644)

	// Wait for auto-check and reload
	time.Sleep(100 * time.Millisecond)

	// Document should have been reloaded
	if string(doc.GetContent()) != "modified" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "modified")
	}
}

func TestAutoReloadSyncManager_NoAutoReloadDirty(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewAutoReloadSyncManager(store, 50*time.Millisecond, AutoReloadClean)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	doc, _ := store.Open(ctx, "/test/file.go")

	// Make local change
	doc.SetContent([]byte("local change"))

	sm.Start()
	defer sm.Stop()

	// Modify file externally
	time.Sleep(10 * time.Millisecond)
	memfs.WriteFile("/test/file.go", []byte("external change"), 0644)

	// Wait for auto-check
	time.Sleep(100 * time.Millisecond)

	// Document should NOT have been auto-reloaded (it's dirty)
	if string(doc.GetContent()) != "local change" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "local change")
	}
}

func TestAutoReloadSyncManager_SetPolicy(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewAutoReloadSyncManager(store, 50*time.Millisecond, AutoReloadNever)

	if sm.GetPolicy() != AutoReloadNever {
		t.Errorf("policy = %v, want %v", sm.GetPolicy(), AutoReloadNever)
	}

	sm.SetPolicy(AutoReloadClean)

	if sm.GetPolicy() != AutoReloadClean {
		t.Errorf("policy = %v, want %v", sm.GetPolicy(), AutoReloadClean)
	}
}

func TestAutoReloadSyncManager_ResolveConflict_KeepLocal(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewAutoReloadSyncManager(store, 50*time.Millisecond, AutoReloadNever)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	doc, _ := store.Open(ctx, "/test/file.go")
	doc.SetContent([]byte("local change"))

	err := sm.ResolveConflict(ctx, "/test/file.go", ResolveKeepLocal)
	if err != nil {
		t.Fatalf("ResolveConflict failed: %v", err)
	}

	// Content should be unchanged
	if string(doc.GetContent()) != "local change" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "local change")
	}
}

func TestAutoReloadSyncManager_ResolveConflict_UseExternal(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewAutoReloadSyncManager(store, 50*time.Millisecond, AutoReloadNever)
	ctx := context.Background()

	memfs.AddFile("/test/file.go", "original")
	doc, _ := store.Open(ctx, "/test/file.go")
	doc.SetContent([]byte("local change"))

	// Modify file externally
	memfs.WriteFile("/test/file.go", []byte("external change"), 0644)

	err := sm.ResolveConflict(ctx, "/test/file.go", ResolveUseExternal)
	if err != nil {
		t.Fatalf("ResolveConflict failed: %v", err)
	}

	// Content should be external version
	if string(doc.GetContent()) != "external change" {
		t.Errorf("content = %q, want %q", doc.GetContent(), "external change")
	}
}

func TestAutoReloadSyncManager_SaveAll(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewAutoReloadSyncManager(store, 50*time.Millisecond, AutoReloadNever)
	ctx := context.Background()

	memfs.AddFile("/test/a.go", "a")
	memfs.AddFile("/test/b.go", "b")

	docA, _ := store.Open(ctx, "/test/a.go")
	docB, _ := store.Open(ctx, "/test/b.go")

	docA.SetContent([]byte("modified a"))
	docB.SetContent([]byte("modified b"))

	result := sm.SaveAll(ctx)

	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failures, got %d", len(result.Failed))
	}
	if len(result.Saved) != 2 {
		t.Errorf("expected 2 saved, got %d", len(result.Saved))
	}

	// Verify content on disk
	contentA, _ := memfs.ReadFile("/test/a.go")
	contentB, _ := memfs.ReadFile("/test/b.go")

	if string(contentA) != "modified a" {
		t.Errorf("disk content a = %q, want %q", contentA, "modified a")
	}
	if string(contentB) != "modified b" {
		t.Errorf("disk content b = %q, want %q", contentB, "modified b")
	}
}

func TestAutoReloadSyncManager_SavePaths(t *testing.T) {
	memfs := vfs.NewMemFS()
	store := NewFileStore(memfs)
	sm := NewAutoReloadSyncManager(store, 50*time.Millisecond, AutoReloadNever)
	ctx := context.Background()

	memfs.AddFile("/test/a.go", "a")
	memfs.AddFile("/test/b.go", "b")

	docA, _ := store.Open(ctx, "/test/a.go")
	docB, _ := store.Open(ctx, "/test/b.go")

	docA.SetContent([]byte("modified a"))
	docB.SetContent([]byte("modified b"))

	// Only save a.go
	result := sm.SavePaths(ctx, []string{"/test/a.go"})

	if len(result.Saved) != 1 {
		t.Errorf("expected 1 saved, got %d", len(result.Saved))
	}

	// a.go should be saved
	contentA, _ := memfs.ReadFile("/test/a.go")
	if string(contentA) != "modified a" {
		t.Errorf("disk content a = %q, want %q", contentA, "modified a")
	}

	// b.go should still have old content
	contentB, _ := memfs.ReadFile("/test/b.go")
	if string(contentB) != "b" {
		t.Errorf("disk content b = %q, want %q", contentB, "b")
	}
}

func TestDocumentWatcher(t *testing.T) {
	memfs := vfs.NewMemFS()
	memfs.AddFile("/test/file.go", "content")

	store := NewFileStore(memfs)
	doc, _ := store.Open(context.Background(), "/test/file.go")

	watcher := NewDocumentWatcher(doc)

	info, _ := memfs.Stat("/test/file.go")
	if watcher.Check(info.ModTime()) {
		t.Error("no changes expected for same mod time")
	}

	// Modify file
	time.Sleep(10 * time.Millisecond)
	memfs.WriteFile("/test/file.go", []byte("modified"), 0644)

	info, _ = memfs.Stat("/test/file.go")
	if !watcher.Check(info.ModTime()) {
		t.Error("changes expected for different mod time")
	}

	lastCheck, checkCount := watcher.Stats()
	if checkCount != 2 {
		t.Errorf("checkCount = %d, want 2", checkCount)
	}
	if time.Since(lastCheck) > time.Second {
		t.Error("lastCheck should be recent")
	}
}
