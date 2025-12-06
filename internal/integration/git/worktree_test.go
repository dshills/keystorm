package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryStage(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Create files
	createFile(t, dir, "file1.txt", "content1")
	createFile(t, dir, "file2.txt", "content2")

	// Stage one file
	if err := repo.Stage("file1.txt"); err != nil {
		t.Fatalf("stage: %v", err)
	}

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Errorf("expected 1 staged file, got %d", len(status.Staged))
	}

	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d", len(status.Untracked))
	}
}

func TestRepositoryStageMultiple(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Create files
	createFile(t, dir, "file1.txt", "content1")
	createFile(t, dir, "file2.txt", "content2")
	createFile(t, dir, "file3.txt", "content3")

	// Stage multiple files
	if err := repo.Stage("file1.txt", "file2.txt"); err != nil {
		t.Fatalf("stage: %v", err)
	}

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Staged) != 2 {
		t.Errorf("expected 2 staged files, got %d", len(status.Staged))
	}

	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d", len(status.Untracked))
	}
}

func TestRepositoryStageAll(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Create files
	createFile(t, dir, "file1.txt", "content1")
	createFile(t, dir, "file2.txt", "content2")

	// Stage all
	if err := repo.StageAll(); err != nil {
		t.Fatalf("stage all: %v", err)
	}

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Staged) != 2 {
		t.Errorf("expected 2 staged files, got %d", len(status.Staged))
	}

	if len(status.Untracked) != 0 {
		t.Errorf("expected 0 untracked files, got %d", len(status.Untracked))
	}
}

func TestRepositoryStageNotFound(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Stage non-existent file
	err = repo.Stage("nonexistent.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestRepositoryUnstage(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Create and stage file
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")

	// Verify staged
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(status.Staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(status.Staged))
	}

	// Unstage
	if err := repo.Unstage("file.txt"); err != nil {
		t.Fatalf("unstage: %v", err)
	}

	// Verify unstaged
	status, err = repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Staged) != 0 {
		t.Errorf("expected 0 staged files, got %d", len(status.Staged))
	}

	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d", len(status.Untracked))
	}
}

func TestRepositoryUnstageAll(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Create and stage files
	createFile(t, dir, "file1.txt", "content1")
	createFile(t, dir, "file2.txt", "content2")
	gitCmd(t, dir, "add", "-A")

	// Unstage all
	if err := repo.UnstageAll(); err != nil {
		t.Fatalf("unstage all: %v", err)
	}

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Staged) != 0 {
		t.Errorf("expected 0 staged files, got %d", len(status.Staged))
	}
}

func TestRepositoryDiscard(t *testing.T) {
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

	// Modify file
	createFile(t, dir, "file.txt", "modified")

	// Verify modified
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(status.Unstaged) != 1 {
		t.Fatalf("expected 1 unstaged file, got %d", len(status.Unstaged))
	}

	// Discard changes
	if err := repo.Discard("file.txt"); err != nil {
		t.Fatalf("discard: %v", err)
	}

	// Verify clean
	status, err = repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.HasChanges() {
		t.Error("expected clean status after discard")
	}

	// Verify content restored
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(content) != "original" {
		t.Errorf("expected 'original', got '%s'", content)
	}
}

func TestRepositoryClean(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "tracked.txt", "content")
	gitCmd(t, dir, "add", "tracked.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Create untracked files
	createFile(t, dir, "untracked1.txt", "content")
	createFile(t, dir, "untracked2.txt", "content")

	// Clean specific file
	if err := repo.Clean("untracked1.txt"); err != nil {
		t.Fatalf("clean: %v", err)
	}

	// Verify one file removed
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d", len(status.Untracked))
	}
}

func TestRepositoryStash(t *testing.T) {
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

	// Modify file
	createFile(t, dir, "file.txt", "modified")

	// Stash
	if err := repo.Stash("test stash"); err != nil {
		t.Fatalf("stash: %v", err)
	}

	// Verify clean
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.HasChanges() {
		t.Error("expected clean status after stash")
	}

	// Verify content restored
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(content) != "original" {
		t.Errorf("expected 'original', got '%s'", content)
	}

	// List stashes
	stashes, err := repo.StashList()
	if err != nil {
		t.Fatalf("stash list: %v", err)
	}

	if len(stashes) != 1 {
		t.Errorf("expected 1 stash, got %d", len(stashes))
	}
}

func TestRepositoryStashPop(t *testing.T) {
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

	// Modify and stash
	createFile(t, dir, "file.txt", "modified")
	if err := repo.Stash("test stash"); err != nil {
		t.Fatalf("stash: %v", err)
	}

	// Pop stash
	if err := repo.StashPop(); err != nil {
		t.Fatalf("stash pop: %v", err)
	}

	// Verify changes restored
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if !status.HasChanges() {
		t.Error("expected changes after stash pop")
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(content) != "modified" {
		t.Errorf("expected 'modified', got '%s'", content)
	}
}

func TestRepositoryStageDeletion(t *testing.T) {
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

	// Delete the file
	os.Remove(filepath.Join(dir, "file.txt"))

	// Stage the deletion
	if err := repo.Stage("file.txt"); err != nil {
		t.Fatalf("stage deletion: %v", err)
	}

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Errorf("expected 1 staged file, got %d", len(status.Staged))
	}

	if status.Staged[0].Status != StatusDeleted {
		t.Errorf("expected StatusDeleted, got %v", status.Staged[0].Status)
	}
}
