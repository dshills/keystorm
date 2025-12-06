package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testRepo creates a temporary git repository for testing.
func testRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	dir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("git init: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("git config email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("git config name: %v", err)
	}

	return dir, cleanup
}

// createFile creates a file in the repo.
func createFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

// gitCmd runs a git command in the repo.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestNewManager(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	if mgr.statusCacheTTL != time.Second {
		t.Errorf("expected default TTL of 1s, got %v", mgr.statusCacheTTL)
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	mgr := NewManager(ManagerConfig{
		StatusCacheTTL: 5 * time.Second,
	})

	if mgr.statusCacheTTL != 5*time.Second {
		t.Errorf("expected TTL of 5s, got %v", mgr.statusCacheTTL)
	}
}

func TestManagerOpen(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if repo.Path() != dir {
		t.Errorf("expected path %s, got %s", dir, repo.Path())
	}
}

func TestManagerOpenNotRepository(t *testing.T) {
	dir, err := os.MkdirTemp("", "not-git-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	_, err = mgr.Open(dir)
	if err != ErrNotRepository {
		t.Errorf("expected ErrNotRepository, got %v", err)
	}
}

func TestManagerDiscover(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create subdirectory
	subdir := filepath.Join(dir, "src", "pkg")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Discover(subdir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if repo.Path() != dir {
		t.Errorf("expected path %s, got %s", dir, repo.Path())
	}
}

func TestManagerDiscoverNotFound(t *testing.T) {
	dir, err := os.MkdirTemp("", "not-git-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	_, err = mgr.Discover(dir)
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}

func TestManagerIsRepository(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	if !mgr.IsRepository(dir) {
		t.Error("expected IsRepository to return true")
	}

	notGitDir, _ := os.MkdirTemp("", "not-git-*")
	defer os.RemoveAll(notGitDir)

	if mgr.IsRepository(notGitDir) {
		t.Error("expected IsRepository to return false")
	}
}

func TestManagerClose(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})

	_, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := mgr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err = mgr.Open(dir)
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed, got %v", err)
	}
}

func TestStatusCode(t *testing.T) {
	tests := []struct {
		code StatusCode
		want string
	}{
		{StatusUnmodified, "unmodified"},
		{StatusModified, "modified"},
		{StatusAdded, "added"},
		{StatusDeleted, "deleted"},
		{StatusRenamed, "renamed"},
		{StatusCopied, "copied"},
		{StatusUntracked, "untracked"},
		{StatusIgnored, "ignored"},
		{StatusConflict, "conflict"},
		{StatusCode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.code.String()
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestStatusHasChanges(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"empty", Status{}, false},
		{"staged", Status{Staged: []FileStatus{{Path: "a"}}}, true},
		{"unstaged", Status{Unstaged: []FileStatus{{Path: "a"}}}, true},
		{"untracked", Status{Untracked: []string{"a"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.HasChanges()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusHasStagedChanges(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"empty", Status{}, false},
		{"staged", Status{Staged: []FileStatus{{Path: "a"}}}, true},
		{"only unstaged", Status{Unstaged: []FileStatus{{Path: "a"}}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.HasStagedChanges()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusHasConflicts(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"empty", Status{}, false},
		{"conflicts", Status{Conflicts: []string{"a"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.HasConflicts()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
