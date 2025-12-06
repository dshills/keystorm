package git

import (
	"strings"
	"testing"
)

func TestDiffUnstaged(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify file
	createFile(t, dir, "file.txt", "line1\nmodified\nline3")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	diff, err := repo.DiffUnstaged()
	if err != nil {
		t.Fatalf("diff unstaged: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(diff.Files))
	}

	if diff.Files[0].NewPath != "file.txt" {
		t.Errorf("expected file.txt, got %s", diff.Files[0].NewPath)
	}

	if diff.Stats.Additions != 1 {
		t.Errorf("expected 1 addition, got %d", diff.Stats.Additions)
	}

	if diff.Stats.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", diff.Stats.Deletions)
	}
}

func TestDiffStaged(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify and stage file
	createFile(t, dir, "file.txt", "line1\nmodified\nline3")
	gitCmd(t, dir, "add", "file.txt")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	diff, err := repo.DiffStaged()
	if err != nil {
		t.Fatalf("diff staged: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(diff.Files))
	}

	if diff.Files[0].Status != StatusModified {
		t.Errorf("expected StatusModified, got %v", diff.Files[0].Status)
	}
}

func TestDiffNewFile(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "old.txt", "content")
	gitCmd(t, dir, "add", "old.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Add new file and stage
	createFile(t, dir, "new.txt", "new content")
	gitCmd(t, dir, "add", "new.txt")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	diff, err := repo.DiffStaged()
	if err != nil {
		t.Fatalf("diff staged: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(diff.Files))
	}

	if diff.Files[0].Status != StatusAdded {
		t.Errorf("expected StatusAdded, got %v", diff.Files[0].Status)
	}
}

func TestDiffDeletedFile(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Delete and stage
	gitCmd(t, dir, "rm", "file.txt")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	diff, err := repo.DiffStaged()
	if err != nil {
		t.Fatalf("diff staged: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(diff.Files))
	}

	if diff.Files[0].Status != StatusDeleted {
		t.Errorf("expected StatusDeleted, got %v", diff.Files[0].Status)
	}
}

func TestDiffCommit(t *testing.T) {
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

	head, _ := repo.Head()
	diff, err := repo.DiffCommit(head.Hash)
	if err != nil {
		t.Fatalf("diff commit: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(diff.Files))
	}
}

func TestDiffHunks(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit with multiple lines
	createFile(t, dir, "file.txt", "line1\nline2\nline3\nline4\nline5")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify middle line
	createFile(t, dir, "file.txt", "line1\nline2\nmodified\nline4\nline5")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	diff, err := repo.DiffUnstaged()
	if err != nil {
		t.Fatalf("diff: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(diff.Files))
	}

	if len(diff.Files[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(diff.Files[0].Hunks))
	}

	hunk := diff.Files[0].Hunks[0]
	if !strings.HasPrefix(hunk.Header, "@@") {
		t.Errorf("expected hunk header to start with @@, got %s", hunk.Header)
	}
}

func TestDiffRaw(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify file
	createFile(t, dir, "file.txt", "modified")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	raw, err := repo.DiffRaw(DiffOptions{})
	if err != nil {
		t.Fatalf("diff raw: %v", err)
	}

	if !strings.Contains(raw, "file.txt") {
		t.Error("expected diff to contain file.txt")
	}

	if !strings.Contains(raw, "-content") {
		t.Error("expected diff to contain deletion")
	}

	if !strings.Contains(raw, "+modified") {
		t.Error("expected diff to contain addition")
	}
}

func TestDiffStat(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "line1\nline2\nline3\n")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify file: change line2 to modified, add newline
	createFile(t, dir, "file.txt", "line1\nmodified\nline3\nnewline\n")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	stats, err := repo.DiffStat(false)
	if err != nil {
		t.Fatalf("diff stat: %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("expected 1 file stat, got %d", len(stats))
	}

	if stats[0].Path != "file.txt" {
		t.Errorf("expected file.txt, got %s", stats[0].Path)
	}

	// Expect: -line2 +modified +newline = 2 additions, 1 deletion
	if stats[0].Additions != 2 {
		t.Errorf("expected 2 additions, got %d", stats[0].Additions)
	}

	if stats[0].Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", stats[0].Deletions)
	}
}

func TestDiffFile(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit with multiple files
	createFile(t, dir, "file1.txt", "content1")
	createFile(t, dir, "file2.txt", "content2")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify both files
	createFile(t, dir, "file1.txt", "modified1")
	createFile(t, dir, "file2.txt", "modified2")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Diff only file1
	diff, err := repo.DiffFile("file1.txt", false)
	if err != nil {
		t.Fatalf("diff file: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(diff.Files))
	}

	if diff.Files[0].NewPath != "file1.txt" {
		t.Errorf("expected file1.txt, got %s", diff.Files[0].NewPath)
	}
}

func TestDiffNoChanges(t *testing.T) {
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

	// No changes
	diff, err := repo.DiffUnstaged()
	if err != nil {
		t.Fatalf("diff: %v", err)
	}

	if len(diff.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(diff.Files))
	}
}
