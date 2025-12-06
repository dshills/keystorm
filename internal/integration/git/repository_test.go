package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryHead(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Empty repo - HEAD exists but points to unborn branch
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}

	if head.ShortName != "main" && head.ShortName != "master" {
		t.Errorf("expected main or master branch, got %s", head.ShortName)
	}

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	head, err = repo.Head()
	if err != nil {
		t.Fatalf("head after commit: %v", err)
	}

	if head.Hash == "" {
		t.Error("expected non-empty hash")
	}

	if len(head.Hash) != 40 {
		t.Errorf("expected 40-char hash, got %d", len(head.Hash))
	}
}

func TestRepositoryStatus(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Empty repo - clean status
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.HasChanges() {
		t.Error("expected clean status")
	}

	// Create untracked file
	createFile(t, dir, "untracked.txt", "content")

	status, err = repo.RefreshStatus()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d", len(status.Untracked))
	}

	if status.Untracked[0] != "untracked.txt" {
		t.Errorf("expected untracked.txt, got %s", status.Untracked[0])
	}
}

func TestRepositoryStatusStaged(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Create and stage a file
	createFile(t, dir, "staged.txt", "content")
	gitCmd(t, dir, "add", "staged.txt")

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Errorf("expected 1 staged file, got %d", len(status.Staged))
	}

	if status.Staged[0].Path != "staged.txt" {
		t.Errorf("expected staged.txt, got %s", status.Staged[0].Path)
	}

	if status.Staged[0].Status != StatusAdded {
		t.Errorf("expected StatusAdded, got %v", status.Staged[0].Status)
	}
}

func TestRepositoryStatusModified(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "original")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Modify the file
	createFile(t, dir, "file.txt", "modified")

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Unstaged) != 1 {
		t.Errorf("expected 1 unstaged file, got %d", len(status.Unstaged))
	}

	if status.Unstaged[0].Path != "file.txt" {
		t.Errorf("expected file.txt, got %s", status.Unstaged[0].Path)
	}

	if status.Unstaged[0].Status != StatusModified {
		t.Errorf("expected StatusModified, got %v", status.Unstaged[0].Status)
	}
}

func TestRepositoryStatusCaching(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// First call
	status1, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	// Create file (won't be seen due to caching)
	createFile(t, dir, "new.txt", "content")

	// Second call - should return cached result
	status2, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	// Both should be the same (cached)
	if len(status1.Untracked) != len(status2.Untracked) {
		t.Error("expected cached result")
	}

	// Force refresh
	status3, err := repo.RefreshStatus()
	if err != nil {
		t.Fatalf("refresh status: %v", err)
	}

	if len(status3.Untracked) != 1 {
		t.Errorf("expected 1 untracked after refresh, got %d", len(status3.Untracked))
	}
}

func TestRepositoryStatusBranch(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	// Should be on main or master
	if status.Branch != "main" && status.Branch != "master" {
		t.Errorf("expected main or master, got %s", status.Branch)
	}

	if status.IsDetached {
		t.Error("expected not detached")
	}
}

func TestRepositoryGetFileStatus(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "tracked.txt", "content")
	gitCmd(t, dir, "add", "tracked.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Create untracked file
	createFile(t, dir, "untracked.txt", "content")

	// Modify tracked file
	createFile(t, dir, "tracked.txt", "modified")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Test untracked file
	fs, err := repo.GetFileStatus("untracked.txt")
	if err != nil {
		t.Fatalf("get file status: %v", err)
	}
	if fs.Status != StatusUntracked {
		t.Errorf("expected StatusUntracked, got %v", fs.Status)
	}

	// Test modified file
	fs, err = repo.GetFileStatus("tracked.txt")
	if err != nil {
		t.Fatalf("get file status: %v", err)
	}
	if fs.Status != StatusModified {
		t.Errorf("expected StatusModified, got %v", fs.Status)
	}

	// Test non-existent file (should be unmodified)
	fs, err = repo.GetFileStatus("nonexistent.txt")
	if err != nil {
		t.Fatalf("get file status: %v", err)
	}
	if fs.Status != StatusUnmodified {
		t.Errorf("expected StatusUnmodified, got %v", fs.Status)
	}
}

func TestDiscoverRepository(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create nested directories
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Discover from nested path
	root, err := discoverRepository(nested)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if root != dir {
		t.Errorf("expected %s, got %s", dir, root)
	}
}

func TestDiscoverRepositoryNotFound(t *testing.T) {
	dir, err := os.MkdirTemp("", "not-git-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.RemoveAll(dir)

	_, err = discoverRepository(dir)
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}
