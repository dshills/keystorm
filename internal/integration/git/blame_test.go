package git

import (
	"fmt"
	"testing"
)

func TestBlame(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	result, err := repo.Blame("file.txt", BlameOptions{})
	if err != nil {
		t.Fatalf("blame: %v", err)
	}

	if result.Path != "file.txt" {
		t.Errorf("expected file.txt, got %s", result.Path)
	}

	if len(result.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(result.Lines))
	}

	// All lines should have the same commit
	hash := result.Lines[0].Hash
	for i, line := range result.Lines {
		if line.Hash != hash {
			t.Errorf("line %d has different hash", i)
		}
		if line.LineNo != i+1 {
			t.Errorf("expected line %d, got %d", i+1, line.LineNo)
		}
	}
}

func TestBlameMultipleCommits(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify middle line
	createFile(t, dir, "file.txt", "line1\nmodified\nline3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "modify line 2")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	result, err := repo.Blame("file.txt", BlameOptions{})
	if err != nil {
		t.Fatalf("blame: %v", err)
	}

	if len(result.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result.Lines))
	}

	// Line 1 and 3 should have same hash (initial commit)
	// Line 2 should have different hash (second commit)
	if result.Lines[0].Hash != result.Lines[2].Hash {
		t.Error("expected lines 1 and 3 to have same hash")
	}

	if result.Lines[1].Hash == result.Lines[0].Hash {
		t.Error("expected line 2 to have different hash")
	}

	// Verify content
	if result.Lines[0].Content != "line1" {
		t.Errorf("expected 'line1', got '%s'", result.Lines[0].Content)
	}

	if result.Lines[1].Content != "modified" {
		t.Errorf("expected 'modified', got '%s'", result.Lines[1].Content)
	}
}

func TestBlameRange(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3\nline4\nline5")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	result, err := repo.BlameRange("file.txt", 2, 4)
	if err != nil {
		t.Fatalf("blame range: %v", err)
	}

	if len(result.Lines) != 3 {
		t.Errorf("expected 3 lines (2-4), got %d", len(result.Lines))
	}

	// First line should be line 2
	if result.Lines[0].LineNo != 2 {
		t.Errorf("expected line 2, got %d", result.Lines[0].LineNo)
	}

	// Last line should be line 4
	if result.Lines[2].LineNo != 4 {
		t.Errorf("expected line 4, got %d", result.Lines[2].LineNo)
	}
}

func TestBlameLine(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	line, err := repo.BlameLine("file.txt", 2)
	if err != nil {
		t.Fatalf("blame line: %v", err)
	}

	if line.LineNo != 2 {
		t.Errorf("expected line 2, got %d", line.LineNo)
	}

	if line.Content != "line2" {
		t.Errorf("expected 'line2', got '%s'", line.Content)
	}

	if line.Author != "Test User" {
		t.Errorf("expected 'Test User', got '%s'", line.Author)
	}
}

func TestBlameAuthorInfo(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial commit")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	result, err := repo.Blame("file.txt", BlameOptions{})
	if err != nil {
		t.Fatalf("blame: %v", err)
	}

	if len(result.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result.Lines))
	}

	line := result.Lines[0]

	if line.Author != "Test User" {
		t.Errorf("expected 'Test User', got '%s'", line.Author)
	}

	if line.AuthorEmail != "test@example.com" {
		t.Errorf("expected 'test@example.com', got '%s'", line.AuthorEmail)
	}

	if line.Summary != "initial commit" {
		t.Errorf("expected 'initial commit', got '%s'", line.Summary)
	}

	if line.AuthorTime.IsZero() {
		t.Error("expected non-zero author time")
	}
}

func TestGetLastModifier(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	author, hash, err := repo.GetLastModifier("file.txt", 1)
	if err != nil {
		t.Fatalf("get last modifier: %v", err)
	}

	if author != "Test User" {
		t.Errorf("expected 'Test User', got '%s'", author)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestGetFileHistory(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create commits
	createFile(t, dir, "file.txt", "v1")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "version 1")

	createFile(t, dir, "file.txt", "v2")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "version 2")

	createFile(t, dir, "file.txt", "v3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "version 3")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	commits, err := repo.GetFileHistory("file.txt", 0)
	if err != nil {
		t.Fatalf("get file history: %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("expected 3 commits, got %d", len(commits))
	}

	// Most recent first
	if commits[0].Message != "version 3" {
		t.Errorf("expected 'version 3', got '%s'", commits[0].Message)
	}
}

func TestGetFileHistoryMaxCount(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create commits with different content each time
	for i := 1; i <= 5; i++ {
		createFile(t, dir, "file.txt", fmt.Sprintf("version %d", i))
		gitCmd(t, dir, "add", "file.txt")
		gitCmd(t, dir, "commit", "-m", fmt.Sprintf("commit %d", i))
	}

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	commits, err := repo.GetFileHistory("file.txt", 2)
	if err != nil {
		t.Fatalf("get file history: %v", err)
	}

	if len(commits) != 2 {
		t.Errorf("expected 2 commits, got %d", len(commits))
	}
}
