package git

import (
	"testing"
	"time"
)

func TestStatusSummary(t *testing.T) {
	status := &Status{
		Branch:    "main",
		Ahead:     2,
		Behind:    1,
		Staged:    []FileStatus{{Path: "a"}, {Path: "b"}},
		Unstaged:  []FileStatus{{Path: "c"}},
		Untracked: []string{"d", "e"},
		Conflicts: []string{"f"},
	}

	summary := status.Summary()

	if summary.Branch != "main" {
		t.Errorf("expected main, got %s", summary.Branch)
	}

	if summary.Ahead != 2 {
		t.Errorf("expected ahead 2, got %d", summary.Ahead)
	}

	if summary.Behind != 1 {
		t.Errorf("expected behind 1, got %d", summary.Behind)
	}

	if summary.StagedCount != 2 {
		t.Errorf("expected 2 staged, got %d", summary.StagedCount)
	}

	if summary.UnstagedCount != 1 {
		t.Errorf("expected 1 unstaged, got %d", summary.UnstagedCount)
	}

	if summary.UntrackedCount != 2 {
		t.Errorf("expected 2 untracked, got %d", summary.UntrackedCount)
	}

	if summary.ConflictCount != 1 {
		t.Errorf("expected 1 conflict, got %d", summary.ConflictCount)
	}

	if !summary.HasChanges {
		t.Error("expected HasChanges to be true")
	}
}

func TestStatusFormatBranch(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{
			name:   "simple branch",
			status: Status{Branch: "main"},
			want:   "main",
		},
		{
			name:   "ahead only",
			status: Status{Branch: "main", Ahead: 2},
			want:   "main ↑2",
		},
		{
			name:   "behind only",
			status: Status{Branch: "main", Behind: 3},
			want:   "main ↓3",
		},
		{
			name:   "ahead and behind",
			status: Status{Branch: "main", Ahead: 2, Behind: 1},
			want:   "main ↑2↓1",
		},
		{
			name:   "detached",
			status: Status{IsDetached: true, HeadCommit: "abc1234"},
			want:   "(abc1234)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.FormatBranch()
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestStatusFormatChanges(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{
			name:   "no changes",
			status: Status{},
			want:   "",
		},
		{
			name: "added files",
			status: Status{
				Staged: []FileStatus{{Status: StatusAdded}, {Status: StatusAdded}},
			},
			want: "+2",
		},
		{
			name: "modified files",
			status: Status{
				Unstaged: []FileStatus{{Status: StatusModified}},
			},
			want: "~1",
		},
		{
			name: "deleted files",
			status: Status{
				Staged: []FileStatus{{Status: StatusDeleted}},
			},
			want: "-1",
		},
		{
			name: "untracked files",
			status: Status{
				Untracked: []string{"a", "b", "c"},
			},
			want: "?3",
		},
		{
			name: "conflicts",
			status: Status{
				Conflicts: []string{"a"},
			},
			want: "!1",
		},
		{
			name: "mixed changes",
			status: Status{
				Staged:    []FileStatus{{Status: StatusAdded}, {Status: StatusModified}},
				Unstaged:  []FileStatus{{Status: StatusDeleted}},
				Untracked: []string{"a"},
			},
			want: "+1 ~1 -1 ?1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.FormatChanges()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusWatcher(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	watcher := NewStatusWatcher(repo, 50*time.Millisecond)

	changed := make(chan *Status, 1)
	watcher.OnChange(func(s *Status) {
		select {
		case changed <- s:
		default:
		}
	})

	watcher.Start()
	defer watcher.Stop()

	// Create a file to trigger change
	createFile(t, dir, "new.txt", "content")

	// Wait for change notification
	select {
	case status := <-changed:
		if len(status.Untracked) != 1 {
			t.Errorf("expected 1 untracked, got %d", len(status.Untracked))
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for status change")
	}
}

func TestStatusWatcherStartStop(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	watcher := NewStatusWatcher(repo, 100*time.Millisecond)

	// Start and stop should not panic
	watcher.Start()
	watcher.Start() // Double start should be safe
	watcher.Stop()
	watcher.Stop() // Double stop should be safe
}

func TestStatusWatcherNoChanges(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	watcher := NewStatusWatcher(repo, 50*time.Millisecond)

	callCount := 0
	watcher.OnChange(func(s *Status) {
		callCount++
	})

	watcher.Start()
	defer watcher.Stop()

	// Wait a bit - should get one initial call, then no more
	time.Sleep(200 * time.Millisecond)

	// Should only have been called once (initial state)
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{123, "123"},
		{-456, "-456"},
		{1000000, "1000000"},
	}

	for _, tt := range tests {
		got := itoa(tt.input)
		if got != tt.want {
			t.Errorf("itoa(%d) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestSetStatusCacheTTL(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Default TTL
	if repo.statusCacheTTL != time.Second {
		t.Errorf("expected default TTL of 1s, got %v", repo.statusCacheTTL)
	}

	// Update TTL
	repo.SetStatusCacheTTL(5 * time.Second)

	if repo.statusCacheTTL != 5*time.Second {
		t.Errorf("expected TTL of 5s, got %v", repo.statusCacheTTL)
	}
}

func TestRefreshStatus(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Get initial status
	status1, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	// Create file
	createFile(t, dir, "file.txt", "content")

	// Status should still show old cached value
	status2, err := repo.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if len(status2.Untracked) != len(status1.Untracked) {
		t.Error("expected cached status")
	}

	// Force refresh
	status3, err := repo.RefreshStatus()
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if len(status3.Untracked) != 1 {
		t.Errorf("expected 1 untracked after refresh, got %d", len(status3.Untracked))
	}
}
