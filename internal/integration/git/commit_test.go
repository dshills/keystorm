package git

import (
	"strings"
	"testing"
	"time"
)

func TestRepositoryCommit(t *testing.T) {
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
	if err := repo.Stage("file.txt"); err != nil {
		t.Fatalf("stage: %v", err)
	}

	// Commit
	commit, err := repo.Commit("test commit", CommitOptions{})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if commit.Hash == "" {
		t.Error("expected non-empty hash")
	}

	if commit.ShortHash == "" {
		t.Error("expected non-empty short hash")
	}

	if commit.Message != "test commit" {
		t.Errorf("expected 'test commit', got '%s'", commit.Message)
	}

	if commit.Author != "Test User" {
		t.Errorf("expected 'Test User', got '%s'", commit.Author)
	}

	if commit.AuthorEmail != "test@example.com" {
		t.Errorf("expected 'test@example.com', got '%s'", commit.AuthorEmail)
	}

	if commit.AuthorTime.IsZero() {
		t.Error("expected non-zero author time")
	}
}

func TestRepositoryCommitNothingStaged(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Try to commit with nothing staged
	_, err = repo.Commit("test commit", CommitOptions{})
	if err != ErrNothingToCommit {
		t.Errorf("expected ErrNothingToCommit, got %v", err)
	}
}

func TestRepositoryCommitAllowEmpty(t *testing.T) {
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

	// Commit with allow empty
	commit, err := repo.Commit("empty commit", CommitOptions{AllowEmpty: true})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if commit.Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestRepositoryCommitAmend(t *testing.T) {
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

	// Get original commit
	originalHead, _ := repo.Head()

	// Amend commit
	commit, err := repo.Commit("amended message", CommitOptions{Amend: true})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Verify different hash (rewritten commit)
	if commit.Hash == originalHead.Hash {
		t.Error("expected different hash after amend")
	}

	// Verify message changed
	if commit.Message != "amended message" {
		t.Errorf("expected 'amended message', got '%s'", commit.Message)
	}
}

func TestRepositoryLog(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create multiple commits
	for i := 1; i <= 3; i++ {
		createFile(t, dir, "file.txt", strings.Repeat("x", i))
		gitCmd(t, dir, "add", "file.txt")
		gitCmd(t, dir, "commit", "-m", "commit "+strings.Repeat("x", i))
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Get log
	commits, err := repo.Log(LogOptions{})
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("expected 3 commits, got %d", len(commits))
	}

	// Verify order (newest first)
	if commits[0].Message != "commit xxx" {
		t.Errorf("expected 'commit xxx', got '%s'", commits[0].Message)
	}
}

func TestRepositoryLogMaxCount(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create multiple commits
	for i := 1; i <= 5; i++ {
		createFile(t, dir, "file.txt", strings.Repeat("x", i))
		gitCmd(t, dir, "add", "file.txt")
		gitCmd(t, dir, "commit", "-m", "commit "+strings.Repeat("x", i))
	}

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Get limited log
	commits, err := repo.Log(LogOptions{MaxCount: 2})
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	if len(commits) != 2 {
		t.Errorf("expected 2 commits, got %d", len(commits))
	}
}

func TestRepositoryLogByPath(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create commits for different files
	createFile(t, dir, "file1.txt", "content1")
	gitCmd(t, dir, "add", "file1.txt")
	gitCmd(t, dir, "commit", "-m", "add file1")

	createFile(t, dir, "file2.txt", "content2")
	gitCmd(t, dir, "add", "file2.txt")
	gitCmd(t, dir, "commit", "-m", "add file2")

	createFile(t, dir, "file1.txt", "modified1")
	gitCmd(t, dir, "add", "file1.txt")
	gitCmd(t, dir, "commit", "-m", "modify file1")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Get log for file1 only
	commits, err := repo.Log(LogOptions{Path: "file1.txt"})
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	if len(commits) != 2 {
		t.Errorf("expected 2 commits for file1, got %d", len(commits))
	}
}

func TestRepositoryGetCommit(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "test commit")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Get HEAD commit
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}

	// Get commit by hash
	commit, err := repo.GetCommit(head.Hash)
	if err != nil {
		t.Fatalf("get commit: %v", err)
	}

	if commit.Hash != head.Hash {
		t.Errorf("expected %s, got %s", head.Hash, commit.Hash)
	}

	if commit.Message != "test commit" {
		t.Errorf("expected 'test commit', got '%s'", commit.Message)
	}
}

func TestRepositoryGetCommitMessage(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create commit with multiline message
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "Subject line\n\nBody paragraph.")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	head, _ := repo.Head()
	msg, err := repo.GetCommitMessage(head.Hash)
	if err != nil {
		t.Fatalf("get commit message: %v", err)
	}

	if !strings.Contains(msg, "Subject line") {
		t.Errorf("expected message to contain 'Subject line', got '%s'", msg)
	}
}

func TestRepositoryGetCommitFiles(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create commit with multiple file changes
	createFile(t, dir, "file1.txt", "content1")
	createFile(t, dir, "file2.txt", "content2")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "add files")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	head, _ := repo.Head()
	files, err := repo.GetCommitFiles(head.Hash)
	if err != nil {
		t.Fatalf("get commit files: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// All should be added
	for _, fs := range files {
		if fs.Status != StatusAdded {
			t.Errorf("expected StatusAdded for %s, got %v", fs.Path, fs.Status)
		}
	}
}

func TestRepositoryGetCommitDiff(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	head, _ := repo.Head()
	diff, err := repo.GetCommitDiff(head.Hash)
	if err != nil {
		t.Fatalf("get commit diff: %v", err)
	}

	if !strings.Contains(diff, "file.txt") {
		t.Errorf("expected diff to contain 'file.txt', got '%s'", diff)
	}

	if !strings.Contains(diff, "+line1") {
		t.Errorf("expected diff to contain '+line1', got '%s'", diff)
	}
}

func TestCommitParents(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create two commits
	createFile(t, dir, "file.txt", "content1")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "first")

	firstHead := gitCmd(t, dir, "rev-parse", "HEAD")
	firstHead = strings.TrimSpace(firstHead)

	createFile(t, dir, "file.txt", "content2")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "second")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	head, _ := repo.Head()
	commit, err := repo.GetCommit(head.Hash)
	if err != nil {
		t.Fatalf("get commit: %v", err)
	}

	if len(commit.Parents) != 1 {
		t.Errorf("expected 1 parent, got %d", len(commit.Parents))
	}

	if commit.Parents[0] != firstHead {
		t.Errorf("expected parent %s, got %s", firstHead, commit.Parents[0])
	}
}
