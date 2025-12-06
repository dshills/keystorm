package git

import (
	"sync"
	"sync/atomic"
	"time"
)

// StatusCode represents the status of a file in the working tree.
type StatusCode int

const (
	// StatusUnmodified indicates the file is unchanged.
	StatusUnmodified StatusCode = iota
	// StatusModified indicates the file has been modified.
	StatusModified
	// StatusAdded indicates the file is newly added.
	StatusAdded
	// StatusDeleted indicates the file has been deleted.
	StatusDeleted
	// StatusRenamed indicates the file has been renamed.
	StatusRenamed
	// StatusCopied indicates the file has been copied.
	StatusCopied
	// StatusUntracked indicates the file is not tracked by git.
	StatusUntracked
	// StatusIgnored indicates the file is ignored by git.
	StatusIgnored
	// StatusConflict indicates a merge conflict.
	StatusConflict
)

// String returns the string representation of a StatusCode.
func (s StatusCode) String() string {
	switch s {
	case StatusUnmodified:
		return "unmodified"
	case StatusModified:
		return "modified"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusCopied:
		return "copied"
	case StatusUntracked:
		return "untracked"
	case StatusIgnored:
		return "ignored"
	case StatusConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// FileStatus represents the status of a single file.
type FileStatus struct {
	// Path is the file path relative to repository root.
	Path string

	// OldPath is the original path for renamed files.
	OldPath string

	// Status indicates the type of change.
	Status StatusCode

	// Staged indicates whether this change is staged.
	Staged bool
}

// Status represents the working tree status.
type Status struct {
	// Branch is the current branch name.
	Branch string

	// Upstream is the upstream branch name (e.g., "origin/main").
	Upstream string

	// Ahead is the number of commits ahead of upstream.
	Ahead int

	// Behind is the number of commits behind upstream.
	Behind int

	// Staged contains staged changes.
	Staged []FileStatus

	// Unstaged contains unstaged changes.
	Unstaged []FileStatus

	// Untracked contains untracked file paths.
	Untracked []string

	// Conflicts contains paths with merge conflicts.
	Conflicts []string

	// IsDetached indicates detached HEAD state.
	IsDetached bool

	// HeadCommit is the current HEAD commit hash (short).
	HeadCommit string
}

// HasChanges returns true if there are any changes (staged, unstaged, untracked, or conflicts).
func (s *Status) HasChanges() bool {
	return len(s.Staged) > 0 || len(s.Unstaged) > 0 || len(s.Untracked) > 0 || len(s.Conflicts) > 0
}

// HasStagedChanges returns true if there are staged changes.
func (s *Status) HasStagedChanges() bool {
	return len(s.Staged) > 0
}

// HasConflicts returns true if there are merge conflicts.
func (s *Status) HasConflicts() bool {
	return len(s.Conflicts) > 0
}

// Reference represents a git reference (branch, tag, commit).
type Reference struct {
	// Name is the reference name (e.g., "refs/heads/main").
	Name string

	// ShortName is the short name (e.g., "main").
	ShortName string

	// Hash is the commit hash this reference points to.
	Hash string

	// IsTag indicates if this is a tag reference.
	IsTag bool
}

// Commit represents a git commit.
type Commit struct {
	// Hash is the full commit hash.
	Hash string

	// ShortHash is the abbreviated commit hash.
	ShortHash string

	// Message is the commit message.
	Message string

	// Author is the commit author name.
	Author string

	// AuthorEmail is the commit author email.
	AuthorEmail string

	// AuthorTime is when the author created the commit.
	AuthorTime time.Time

	// Committer is the committer name.
	Committer string

	// CommitterEmail is the committer email.
	CommitterEmail string

	// CommitTime is when the commit was created.
	CommitTime time.Time

	// Parents are the parent commit hashes.
	Parents []string
}

// CommitOptions configures commit creation.
type CommitOptions struct {
	// Author overrides the commit author.
	Author string

	// AuthorEmail overrides the author email.
	AuthorEmail string

	// Amend amends the previous commit.
	Amend bool

	// AllowEmpty allows creating an empty commit.
	AllowEmpty bool

	// SignOff adds a Signed-off-by line.
	SignOff bool
}

// EventPublisher publishes git events.
type EventPublisher interface {
	Publish(eventType string, data map[string]any)
}

// Manager manages git repositories.
type Manager struct {
	mu     sync.RWMutex
	repos  map[string]*Repository
	closed atomic.Bool

	// Configuration
	statusCacheTTL time.Duration

	// Event publishing
	eventBus EventPublisher
}

// ManagerConfig configures a git manager.
type ManagerConfig struct {
	// StatusCacheTTL is how long to cache status results.
	// Defaults to 1 second.
	StatusCacheTTL time.Duration

	// EventBus for publishing git events.
	EventBus EventPublisher
}

// NewManager creates a new git manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.StatusCacheTTL <= 0 {
		cfg.StatusCacheTTL = time.Second
	}

	return &Manager{
		repos:          make(map[string]*Repository),
		statusCacheTTL: cfg.StatusCacheTTL,
		eventBus:       cfg.EventBus,
	}
}

// Open opens a repository at the given path.
// The path must be the repository root (containing .git).
func (m *Manager) Open(path string) (*Repository, error) {
	if m.closed.Load() {
		return nil, ErrManagerClosed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already open
	if repo, ok := m.repos[path]; ok {
		return repo, nil
	}

	// Open the repository
	repo, err := openRepository(path, m.statusCacheTTL, m.eventBus)
	if err != nil {
		return nil, err
	}

	m.repos[path] = repo
	return repo, nil
}

// Discover finds and opens the repository containing the given path.
// It walks up the directory tree looking for a .git directory.
func (m *Manager) Discover(path string) (*Repository, error) {
	if m.closed.Load() {
		return nil, ErrManagerClosed
	}

	// Find the repository root
	root, err := discoverRepository(path)
	if err != nil {
		return nil, err
	}

	return m.Open(root)
}

// IsRepository checks if the path is inside a git repository.
func (m *Manager) IsRepository(path string) bool {
	_, err := discoverRepository(path)
	return err == nil
}

// Close closes the manager and all open repositories.
func (m *Manager) Close() error {
	if m.closed.Swap(true) {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, repo := range m.repos {
		repo.close()
	}
	m.repos = make(map[string]*Repository)

	return nil
}

// publishEvent publishes an event if an event bus is configured.
func (m *Manager) publishEvent(eventType string, data map[string]any) {
	if m.eventBus != nil {
		if data == nil {
			data = make(map[string]any)
		}
		data["timestamp"] = time.Now().UnixMilli()
		m.eventBus.Publish(eventType, data)
	}
}
