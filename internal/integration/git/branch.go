package git

import (
	"fmt"
	"strings"
)

// Branch represents a git branch.
type Branch struct {
	// Name is the branch name (e.g., "main", "feature/foo").
	Name string

	// FullName is the full reference name (e.g., "refs/heads/main").
	FullName string

	// Hash is the commit hash this branch points to.
	Hash string

	// Upstream is the upstream branch (e.g., "origin/main").
	Upstream string

	// IsHead indicates if this is the current branch.
	IsHead bool

	// IsRemote indicates if this is a remote tracking branch.
	IsRemote bool

	// Ahead is the number of commits ahead of upstream.
	Ahead int

	// Behind is the number of commits behind upstream.
	Behind int
}

// ListBranches returns all local branches.
func (r *Repository) ListBranches() ([]Branch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.listBranchesLocked(false)
}

// ListAllBranches returns all local and remote branches.
func (r *Repository) ListAllBranches() ([]Branch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.listBranchesLocked(true)
}

// listBranchesLocked lists branches (caller must hold lock).
func (r *Repository) listBranchesLocked(includeRemote bool) ([]Branch, error) {
	// Format: refname, objectname, upstream, HEAD indicator, upstream tracking
	format := "%(refname)%00%(objectname)%00%(upstream:short)%00%(HEAD)%00%(upstream:track)"

	args := []string{"for-each-ref", "--format=" + format, "refs/heads"}
	if includeRemote {
		args = append(args, "refs/remotes")
	}

	output, err := r.git(args...)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	var branches []Branch
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\x00")
		if len(parts) < 5 {
			continue
		}

		branch := Branch{
			FullName: parts[0],
			Hash:     parts[1],
			Upstream: parts[2],
			IsHead:   parts[3] == "*",
		}

		// Extract short name from full refname
		if strings.HasPrefix(branch.FullName, "refs/heads/") {
			branch.Name = strings.TrimPrefix(branch.FullName, "refs/heads/")
		} else if strings.HasPrefix(branch.FullName, "refs/remotes/") {
			branch.Name = strings.TrimPrefix(branch.FullName, "refs/remotes/")
			branch.IsRemote = true
		}

		// Parse upstream tracking info: [ahead N, behind M] or [ahead N] or [behind M]
		if track := parts[4]; track != "" {
			track = strings.Trim(track, "[]")
			for _, part := range strings.Split(track, ", ") {
				if strings.HasPrefix(part, "ahead ") {
					fmt.Sscanf(part, "ahead %d", &branch.Ahead)
				} else if strings.HasPrefix(part, "behind ") {
					fmt.Sscanf(part, "behind %d", &branch.Behind)
				}
			}
		}

		branches = append(branches, branch)
	}

	return branches, nil
}

// GetBranch returns information about a specific branch.
func (r *Repository) GetBranch(name string) (*Branch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	branches, err := r.listBranchesLocked(true)
	if err != nil {
		return nil, err
	}

	for _, b := range branches {
		if b.Name == name {
			return &b, nil
		}
	}

	return nil, fmt.Errorf("branch not found: %s", name)
}

// CurrentBranch returns the current branch name.
// Returns empty string if in detached HEAD state.
func (r *Repository) CurrentBranch() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	output, err := r.git("symbolic-ref", "--short", "HEAD")
	if err != nil {
		// Check if we're in detached HEAD state
		if strings.Contains(err.Error(), "not a symbolic ref") {
			return "", nil
		}
		return "", fmt.Errorf("get current branch: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// CreateBranch creates a new branch at the given start point.
// If startPoint is empty, the branch is created at HEAD.
func (r *Repository) CreateBranch(name string, startPoint string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"branch", name}
	if startPoint != "" {
		args = append(args, startPoint)
	}

	if _, err := r.git(args...); err != nil {
		return fmt.Errorf("create branch %s: %w", name, err)
	}

	r.publishEvent("git.branch.created", map[string]any{
		"name":       name,
		"startPoint": startPoint,
	})

	return nil
}

// CreateBranchAndSwitch creates a new branch and switches to it.
func (r *Repository) CreateBranchAndSwitch(name string, startPoint string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"checkout", "-b", name}
	if startPoint != "" {
		args = append(args, startPoint)
	}

	if _, err := r.git(args...); err != nil {
		return fmt.Errorf("create and switch to branch %s: %w", name, err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.branch.created", map[string]any{
		"name":       name,
		"startPoint": startPoint,
		"switched":   true,
	})

	return nil
}

// DeleteBranch deletes a branch.
// Set force to true to delete unmerged branches.
func (r *Repository) DeleteBranch(name string, force bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	flag := "-d"
	if force {
		flag = "-D"
	}

	if _, err := r.git("branch", flag, name); err != nil {
		return fmt.Errorf("delete branch %s: %w", name, err)
	}

	r.publishEvent("git.branch.deleted", map[string]any{
		"name":  name,
		"force": force,
	})

	return nil
}

// RenameBranch renames a branch.
func (r *Repository) RenameBranch(oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("branch", "-m", oldName, newName); err != nil {
		return fmt.Errorf("rename branch %s to %s: %w", oldName, newName, err)
	}

	r.publishEvent("git.branch.renamed", map[string]any{
		"oldName": oldName,
		"newName": newName,
	})

	return nil
}

// SwitchBranch switches to an existing branch.
func (r *Repository) SwitchBranch(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("checkout", name); err != nil {
		return fmt.Errorf("switch to branch %s: %w", name, err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.branch.switched", map[string]any{
		"name": name,
	})

	return nil
}

// SwitchBranchDetach switches to a commit in detached HEAD state.
func (r *Repository) SwitchBranchDetach(ref string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("checkout", "--detach", ref); err != nil {
		return fmt.Errorf("detach to %s: %w", ref, err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.branch.detached", map[string]any{
		"ref": ref,
	})

	return nil
}

// MergeBranch merges the given branch into the current branch.
func (r *Repository) MergeBranch(name string, opts MergeOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"merge", name}

	if opts.NoFF {
		args = append(args, "--no-ff")
	}
	if opts.FFOnly {
		args = append(args, "--ff-only")
	}
	if opts.Squash {
		args = append(args, "--squash")
	}
	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}

	output, err := r.git(args...)
	if err != nil {
		// Check for merge conflicts
		if strings.Contains(output, "CONFLICT") || strings.Contains(err.Error(), "CONFLICT") {
			return ErrConflict
		}
		return fmt.Errorf("merge branch %s: %w", name, err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.branch.merged", map[string]any{
		"name":   name,
		"squash": opts.Squash,
	})

	return nil
}

// MergeOptions configures merge behavior.
type MergeOptions struct {
	// NoFF creates a merge commit even for fast-forward merges.
	NoFF bool

	// FFOnly only allows fast-forward merges.
	FFOnly bool

	// Squash squashes all commits into one.
	Squash bool

	// Message is the merge commit message.
	Message string
}

// AbortMerge aborts an in-progress merge.
func (r *Repository) AbortMerge() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("merge", "--abort"); err != nil {
		return fmt.Errorf("abort merge: %w", err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.merge.aborted", nil)

	return nil
}

// RebaseBranch rebases the current branch onto the given branch.
func (r *Repository) RebaseBranch(onto string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("rebase", onto); err != nil {
		// Check for rebase conflicts
		if strings.Contains(err.Error(), "CONFLICT") {
			return ErrConflict
		}
		return fmt.Errorf("rebase onto %s: %w", onto, err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.branch.rebased", map[string]any{
		"onto": onto,
	})

	return nil
}

// AbortRebase aborts an in-progress rebase.
func (r *Repository) AbortRebase() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("rebase", "--abort"); err != nil {
		return fmt.Errorf("abort rebase: %w", err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.rebase.aborted", nil)

	return nil
}

// ContinueRebase continues a paused rebase after resolving conflicts.
func (r *Repository) ContinueRebase() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("rebase", "--continue"); err != nil {
		return fmt.Errorf("continue rebase: %w", err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.rebase.continued", nil)

	return nil
}

// SkipRebase skips the current commit in a rebase.
func (r *Repository) SkipRebase() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("rebase", "--skip"); err != nil {
		return fmt.Errorf("skip rebase: %w", err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.rebase.skipped", nil)

	return nil
}
