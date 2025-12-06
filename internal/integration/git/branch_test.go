package git

import (
	"testing"
)

func TestListBranches(t *testing.T) {
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

	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}

	if len(branches) != 1 {
		t.Errorf("expected 1 branch, got %d", len(branches))
	}

	// Should be main or master
	if branches[0].Name != "main" && branches[0].Name != "master" {
		t.Errorf("expected main or master, got %s", branches[0].Name)
	}

	if !branches[0].IsHead {
		t.Error("expected branch to be HEAD")
	}
}

func TestCreateBranch(t *testing.T) {
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

	// Create new branch
	if err := repo.CreateBranch("feature", ""); err != nil {
		t.Fatalf("create branch: %v", err)
	}

	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}

	if len(branches) != 2 {
		t.Errorf("expected 2 branches, got %d", len(branches))
	}

	// Find feature branch
	found := false
	for _, b := range branches {
		if b.Name == "feature" {
			found = true
			break
		}
	}
	if !found {
		t.Error("feature branch not found")
	}
}

func TestCreateBranchAndSwitch(t *testing.T) {
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

	// Create and switch to new branch
	if err := repo.CreateBranchAndSwitch("feature", ""); err != nil {
		t.Fatalf("create and switch branch: %v", err)
	}

	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("current branch: %v", err)
	}

	if current != "feature" {
		t.Errorf("expected feature, got %s", current)
	}
}

func TestDeleteBranch(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Create feature branch
	gitCmd(t, dir, "branch", "feature")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Delete branch
	if err := repo.DeleteBranch("feature", false); err != nil {
		t.Fatalf("delete branch: %v", err)
	}

	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}

	for _, b := range branches {
		if b.Name == "feature" {
			t.Error("feature branch should have been deleted")
		}
	}
}

func TestRenameBranch(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Create feature branch
	gitCmd(t, dir, "branch", "feature")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Rename branch
	if err := repo.RenameBranch("feature", "feature-renamed"); err != nil {
		t.Fatalf("rename branch: %v", err)
	}

	branch, err := repo.GetBranch("feature-renamed")
	if err != nil {
		t.Fatalf("get branch: %v", err)
	}

	if branch.Name != "feature-renamed" {
		t.Errorf("expected feature-renamed, got %s", branch.Name)
	}
}

func TestSwitchBranch(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Create feature branch
	gitCmd(t, dir, "branch", "feature")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Switch to feature
	if err := repo.SwitchBranch("feature"); err != nil {
		t.Fatalf("switch branch: %v", err)
	}

	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("current branch: %v", err)
	}

	if current != "feature" {
		t.Errorf("expected feature, got %s", current)
	}
}

func TestCurrentBranch(t *testing.T) {
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

	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("current branch: %v", err)
	}

	if current != "main" && current != "master" {
		t.Errorf("expected main or master, got %s", current)
	}
}

func TestSwitchBranchDetach(t *testing.T) {
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

	// Detach to HEAD commit
	if err := repo.SwitchBranchDetach(head.Hash); err != nil {
		t.Fatalf("detach: %v", err)
	}

	// Current branch should be empty (detached)
	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("current branch: %v", err)
	}

	if current != "" {
		t.Errorf("expected empty (detached), got %s", current)
	}
}

func TestMergeBranch(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit on main
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Create feature branch with changes
	gitCmd(t, dir, "checkout", "-b", "feature")
	createFile(t, dir, "feature.txt", "feature content")
	gitCmd(t, dir, "add", "feature.txt")
	gitCmd(t, dir, "commit", "-m", "feature commit")

	// Switch back to main
	gitCmd(t, dir, "checkout", "main")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Merge feature into main
	if err := repo.MergeBranch("feature", MergeOptions{}); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Verify feature.txt exists
	status, err := repo.RefreshStatus()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.HasChanges() {
		t.Error("expected clean status after merge")
	}
}
