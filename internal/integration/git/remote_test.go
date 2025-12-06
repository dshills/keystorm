package git

import (
	"strings"
	"testing"
)

func TestListRemotes(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// No remotes initially
	remotes, err := repo.ListRemotes()
	if err != nil {
		t.Fatalf("list remotes: %v", err)
	}

	if len(remotes) != 0 {
		t.Errorf("expected 0 remotes, got %d", len(remotes))
	}
}

func TestAddRemote(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Add remote
	if err := repo.AddRemote("origin", "https://github.com/example/repo.git"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	remotes, err := repo.ListRemotes()
	if err != nil {
		t.Fatalf("list remotes: %v", err)
	}

	if len(remotes) != 1 {
		t.Errorf("expected 1 remote, got %d", len(remotes))
	}

	if remotes[0].Name != "origin" {
		t.Errorf("expected origin, got %s", remotes[0].Name)
	}
}

func TestGetRemote(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Add remote
	gitCmd(t, dir, "remote", "add", "origin", "https://github.com/example/repo.git")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	remote, err := repo.GetRemote("origin")
	if err != nil {
		t.Fatalf("get remote: %v", err)
	}

	if remote.Name != "origin" {
		t.Errorf("expected origin, got %s", remote.Name)
	}

	// URL may be rewritten by git config (insteadOf), so just check it's not empty
	if remote.FetchURL == "" {
		t.Error("expected non-empty URL")
	}
}

func TestRemoveRemote(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Add remote
	gitCmd(t, dir, "remote", "add", "origin", "https://github.com/example/repo.git")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Remove remote
	if err := repo.RemoveRemote("origin"); err != nil {
		t.Fatalf("remove remote: %v", err)
	}

	remotes, err := repo.ListRemotes()
	if err != nil {
		t.Fatalf("list remotes: %v", err)
	}

	if len(remotes) != 0 {
		t.Errorf("expected 0 remotes after removal, got %d", len(remotes))
	}
}

func TestRenameRemote(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Add remote
	gitCmd(t, dir, "remote", "add", "origin", "https://github.com/example/repo.git")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Rename remote
	if err := repo.RenameRemote("origin", "upstream"); err != nil {
		t.Fatalf("rename remote: %v", err)
	}

	remote, err := repo.GetRemote("upstream")
	if err != nil {
		t.Fatalf("get remote: %v", err)
	}

	if remote.Name != "upstream" {
		t.Errorf("expected upstream, got %s", remote.Name)
	}
}

func TestSetRemoteURL(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Add remote
	gitCmd(t, dir, "remote", "add", "origin", "https://github.com/example/repo.git")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Change URL
	newURL := "https://github.com/other/repo.git"
	if err := repo.SetRemoteURL("origin", newURL, false); err != nil {
		t.Fatalf("set remote URL: %v", err)
	}

	remote, err := repo.GetRemote("origin")
	if err != nil {
		t.Fatalf("get remote: %v", err)
	}

	// URL may be rewritten by git config (insteadOf), so check it contains "other/repo"
	if !strings.Contains(remote.FetchURL, "other/repo") {
		t.Errorf("expected URL to contain 'other/repo', got %s", remote.FetchURL)
	}
}

func TestGetUpstreamNoUpstream(t *testing.T) {
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

	_, err = repo.GetUpstream()
	if err != ErrNoUpstream {
		t.Errorf("expected ErrNoUpstream, got %v", err)
	}
}
