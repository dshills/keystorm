package git

import (
	"sync"
	"time"
)

// StatusWatcher provides real-time status monitoring.
type StatusWatcher struct {
	repo     *Repository
	interval time.Duration

	mu        sync.RWMutex
	last      *Status
	callbacks []func(*Status)
	done      chan struct{}
	running   bool
}

// NewStatusWatcher creates a status watcher for a repository.
func NewStatusWatcher(repo *Repository, interval time.Duration) *StatusWatcher {
	if interval <= 0 {
		interval = time.Second
	}

	return &StatusWatcher{
		repo:     repo,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// OnChange registers a callback for status changes.
func (w *StatusWatcher) OnChange(fn func(*Status)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, fn)
}

// Start starts the status watcher.
func (w *StatusWatcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.done = make(chan struct{})
	w.mu.Unlock()

	go w.watchLoop()
}

// Stop stops the status watcher.
func (w *StatusWatcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.done)
	w.mu.Unlock()
}

// watchLoop polls for status changes.
func (w *StatusWatcher) watchLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			w.checkStatus()
		}
	}
}

// checkStatus checks for status changes and notifies callbacks.
func (w *StatusWatcher) checkStatus() {
	status, err := w.repo.Status()
	if err != nil {
		return
	}

	w.mu.Lock()
	changed := w.hasChanged(status)
	if changed {
		w.last = status
	}
	callbacks := make([]func(*Status), len(w.callbacks))
	copy(callbacks, w.callbacks)
	w.mu.Unlock()

	if changed {
		for _, cb := range callbacks {
			cb(status)
		}
	}
}

// hasChanged compares current status to last status.
func (w *StatusWatcher) hasChanged(current *Status) bool {
	if w.last == nil {
		return true
	}

	// Compare key fields
	if w.last.Branch != current.Branch {
		return true
	}
	if w.last.Ahead != current.Ahead || w.last.Behind != current.Behind {
		return true
	}
	if len(w.last.Staged) != len(current.Staged) {
		return true
	}
	if len(w.last.Unstaged) != len(current.Unstaged) {
		return true
	}
	if len(w.last.Untracked) != len(current.Untracked) {
		return true
	}
	if len(w.last.Conflicts) != len(current.Conflicts) {
		return true
	}

	// Deep compare staged files
	for i, fs := range w.last.Staged {
		if current.Staged[i].Path != fs.Path || current.Staged[i].Status != fs.Status {
			return true
		}
	}

	// Deep compare unstaged files
	for i, fs := range w.last.Unstaged {
		if current.Unstaged[i].Path != fs.Path || current.Unstaged[i].Status != fs.Status {
			return true
		}
	}

	// Deep compare untracked files
	for i, path := range w.last.Untracked {
		if current.Untracked[i] != path {
			return true
		}
	}

	return false
}

// StatusSummary provides a compact status representation.
type StatusSummary struct {
	// Branch is the current branch name.
	Branch string

	// IsDetached indicates detached HEAD state.
	IsDetached bool

	// HeadCommit is the current commit hash (short).
	HeadCommit string

	// Ahead is commits ahead of upstream.
	Ahead int

	// Behind is commits behind upstream.
	Behind int

	// StagedCount is the number of staged files.
	StagedCount int

	// UnstagedCount is the number of unstaged files.
	UnstagedCount int

	// UntrackedCount is the number of untracked files.
	UntrackedCount int

	// ConflictCount is the number of conflicted files.
	ConflictCount int

	// HasChanges indicates any uncommitted changes.
	HasChanges bool
}

// Summary returns a compact status summary.
func (s *Status) Summary() StatusSummary {
	return StatusSummary{
		Branch:         s.Branch,
		IsDetached:     s.IsDetached,
		HeadCommit:     s.HeadCommit,
		Ahead:          s.Ahead,
		Behind:         s.Behind,
		StagedCount:    len(s.Staged),
		UnstagedCount:  len(s.Unstaged),
		UntrackedCount: len(s.Untracked),
		ConflictCount:  len(s.Conflicts),
		HasChanges:     s.HasChanges(),
	}
}

// FormatBranch returns a formatted branch string for display.
// Examples: "main", "main ↑2", "main ↓1", "main ↑2↓1", "(detached)"
func (s *Status) FormatBranch() string {
	if s.IsDetached {
		return "(" + s.HeadCommit + ")"
	}

	result := s.Branch

	if s.Ahead > 0 && s.Behind > 0 {
		result += " ↑" + itoa(s.Ahead) + "↓" + itoa(s.Behind)
	} else if s.Ahead > 0 {
		result += " ↑" + itoa(s.Ahead)
	} else if s.Behind > 0 {
		result += " ↓" + itoa(s.Behind)
	}

	return result
}

// FormatChanges returns a formatted changes string for display.
// Examples: "+2 ~3 -1", "●3" (staged only), "○5" (unstaged only)
func (s *Status) FormatChanges() string {
	if !s.HasChanges() {
		return ""
	}

	var parts []string

	// Count by type
	var added, modified, deleted int
	for _, fs := range append(s.Staged, s.Unstaged...) {
		switch fs.Status {
		case StatusAdded:
			added++
		case StatusModified:
			modified++
		case StatusDeleted:
			deleted++
		}
	}

	if added > 0 {
		parts = append(parts, "+"+itoa(added))
	}
	if modified > 0 {
		parts = append(parts, "~"+itoa(modified))
	}
	if deleted > 0 {
		parts = append(parts, "-"+itoa(deleted))
	}

	if len(s.Untracked) > 0 {
		parts = append(parts, "?"+itoa(len(s.Untracked)))
	}

	if len(s.Conflicts) > 0 {
		parts = append(parts, "!"+itoa(len(s.Conflicts)))
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}

	return result
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	neg := i < 0
	if neg {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}

// RefreshStatus forces a status refresh, bypassing the cache.
func (r *Repository) RefreshStatus() (*Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear cache to force refresh
	r.statusCache = nil

	status, err := r.statusLocked()
	if err != nil {
		return nil, err
	}

	r.statusCache = status
	r.statusCacheTime = time.Now()
	return status, nil
}

// SetStatusCacheTTL updates the status cache TTL.
func (r *Repository) SetStatusCacheTTL(ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statusCacheTTL = ttl
}

// GetFileStatus returns the status for a specific file.
func (r *Repository) GetFileStatus(path string) (*FileStatus, error) {
	status, err := r.Status()
	if err != nil {
		return nil, err
	}

	// Check staged
	for _, fs := range status.Staged {
		if fs.Path == path {
			return &fs, nil
		}
	}

	// Check unstaged
	for _, fs := range status.Unstaged {
		if fs.Path == path {
			return &fs, nil
		}
	}

	// Check untracked
	for _, p := range status.Untracked {
		if p == path {
			return &FileStatus{
				Path:   path,
				Status: StatusUntracked,
			}, nil
		}
	}

	// Check conflicts
	for _, p := range status.Conflicts {
		if p == path {
			return &FileStatus{
				Path:   path,
				Status: StatusConflict,
			}, nil
		}
	}

	// File is unmodified
	return &FileStatus{
		Path:   path,
		Status: StatusUnmodified,
	}, nil
}
