package git

import (
	"fmt"
	"strings"
)

// Remote represents a git remote.
type Remote struct {
	// Name is the remote name (e.g., "origin").
	Name string

	// FetchURL is the URL used for fetching.
	FetchURL string

	// PushURL is the URL used for pushing.
	PushURL string
}

// ListRemotes returns all configured remotes.
func (r *Repository) ListRemotes() ([]Remote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	output, err := r.git("remote", "-v")
	if err != nil {
		return nil, fmt.Errorf("list remotes: %w", err)
	}

	// Parse output: name\turl (fetch|push)
	remotes := make(map[string]*Remote)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		name := parts[0]
		url := parts[1]
		kind := strings.Trim(parts[2], "()")

		remote, ok := remotes[name]
		if !ok {
			remote = &Remote{Name: name}
			remotes[name] = remote
		}

		switch kind {
		case "fetch":
			remote.FetchURL = url
		case "push":
			remote.PushURL = url
		}
	}

	result := make([]Remote, 0, len(remotes))
	for _, remote := range remotes {
		result = append(result, *remote)
	}

	return result, nil
}

// GetRemote returns information about a specific remote.
func (r *Repository) GetRemote(name string) (*Remote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fetchURL, err := r.git("remote", "get-url", name)
	if err != nil {
		return nil, fmt.Errorf("get remote %s: %w", name, err)
	}

	pushURL, err := r.git("remote", "get-url", "--push", name)
	if err != nil {
		pushURL = fetchURL
	}

	return &Remote{
		Name:     name,
		FetchURL: strings.TrimSpace(fetchURL),
		PushURL:  strings.TrimSpace(pushURL),
	}, nil
}

// AddRemote adds a new remote.
func (r *Repository) AddRemote(name, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("remote", "add", name, url); err != nil {
		return fmt.Errorf("add remote %s: %w", name, err)
	}

	r.publishEvent("git.remote.added", map[string]any{
		"name": name,
		"url":  url,
	})

	return nil
}

// RemoveRemote removes a remote.
func (r *Repository) RemoveRemote(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("remote", "remove", name); err != nil {
		return fmt.Errorf("remove remote %s: %w", name, err)
	}

	r.publishEvent("git.remote.removed", map[string]any{
		"name": name,
	})

	return nil
}

// RenameRemote renames a remote.
func (r *Repository) RenameRemote(oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("remote", "rename", oldName, newName); err != nil {
		return fmt.Errorf("rename remote %s to %s: %w", oldName, newName, err)
	}

	r.publishEvent("git.remote.renamed", map[string]any{
		"oldName": oldName,
		"newName": newName,
	})

	return nil
}

// SetRemoteURL sets the URL for a remote.
func (r *Repository) SetRemoteURL(name, url string, push bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"remote", "set-url"}
	if push {
		args = append(args, "--push")
	}
	args = append(args, name, url)

	if _, err := r.git(args...); err != nil {
		return fmt.Errorf("set remote URL %s: %w", name, err)
	}

	return nil
}

// Fetch fetches from a remote.
func (r *Repository) Fetch(opts FetchOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"fetch"}

	if opts.All {
		args = append(args, "--all")
	} else if opts.Remote != "" {
		args = append(args, opts.Remote)
		if opts.RefSpec != "" {
			args = append(args, opts.RefSpec)
		}
	}

	if opts.Prune {
		args = append(args, "--prune")
	}

	if opts.Tags {
		args = append(args, "--tags")
	}

	if opts.Depth > 0 {
		args = append(args, fmt.Sprintf("--depth=%d", opts.Depth))
	}

	output, err := r.git(args...)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	r.publishEvent("git.fetch.completed", map[string]any{
		"remote": opts.Remote,
		"all":    opts.All,
		"output": output,
	})

	return nil
}

// FetchOptions configures fetch behavior.
type FetchOptions struct {
	// Remote is the remote to fetch from.
	Remote string

	// RefSpec is the refspec to fetch.
	RefSpec string

	// All fetches from all remotes.
	All bool

	// Prune removes remote-tracking references that no longer exist.
	Prune bool

	// Tags fetches tags.
	Tags bool

	// Depth limits fetch to the specified number of commits.
	Depth int
}

// Pull fetches and integrates changes from a remote.
func (r *Repository) Pull(opts PullOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"pull"}

	if opts.Remote != "" {
		args = append(args, opts.Remote)
		if opts.Branch != "" {
			args = append(args, opts.Branch)
		}
	}

	if opts.Rebase {
		args = append(args, "--rebase")
	}

	if opts.FFOnly {
		args = append(args, "--ff-only")
	}

	if opts.NoFF {
		args = append(args, "--no-ff")
	}

	output, err := r.git(args...)
	if err != nil {
		// Check for merge conflicts
		if strings.Contains(output, "CONFLICT") || strings.Contains(err.Error(), "CONFLICT") {
			return ErrConflict
		}
		return fmt.Errorf("pull: %w", err)
	}

	// Invalidate status cache
	r.statusCache = nil

	r.publishEvent("git.pull.completed", map[string]any{
		"remote": opts.Remote,
		"branch": opts.Branch,
		"rebase": opts.Rebase,
		"output": output,
	})

	return nil
}

// PullOptions configures pull behavior.
type PullOptions struct {
	// Remote is the remote to pull from.
	Remote string

	// Branch is the branch to pull.
	Branch string

	// Rebase rebases instead of merging.
	Rebase bool

	// FFOnly only allows fast-forward merges.
	FFOnly bool

	// NoFF creates a merge commit even for fast-forward merges.
	NoFF bool
}

// Push pushes changes to a remote.
func (r *Repository) Push(opts PushOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"push"}

	if opts.Remote != "" {
		args = append(args, opts.Remote)
		if opts.RefSpec != "" {
			args = append(args, opts.RefSpec)
		}
	}

	if opts.SetUpstream {
		args = append(args, "-u")
	}

	if opts.Force {
		args = append(args, "--force")
	}

	if opts.ForceWithLease {
		args = append(args, "--force-with-lease")
	}

	if opts.Tags {
		args = append(args, "--tags")
	}

	if opts.Delete {
		args = append(args, "--delete")
	}

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	output, err := r.git(args...)
	if err != nil {
		// Check for common push errors
		if strings.Contains(err.Error(), "rejected") {
			return ErrPushRejected
		}
		if strings.Contains(err.Error(), "no upstream") {
			return ErrNoUpstream
		}
		return fmt.Errorf("push: %w", err)
	}

	r.publishEvent("git.push.completed", map[string]any{
		"remote":  opts.Remote,
		"refSpec": opts.RefSpec,
		"force":   opts.Force,
		"output":  output,
	})

	return nil
}

// PushOptions configures push behavior.
type PushOptions struct {
	// Remote is the remote to push to.
	Remote string

	// RefSpec is the refspec to push.
	RefSpec string

	// SetUpstream sets upstream tracking.
	SetUpstream bool

	// Force forces the push.
	Force bool

	// ForceWithLease is a safer force push.
	ForceWithLease bool

	// Tags pushes tags.
	Tags bool

	// Delete deletes the remote ref.
	Delete bool

	// DryRun performs a dry run.
	DryRun bool
}

// SetUpstream sets the upstream branch for the current branch.
func (r *Repository) SetUpstream(remote, branch string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	upstream := remote + "/" + branch
	if _, err := r.git("branch", "--set-upstream-to", upstream); err != nil {
		return fmt.Errorf("set upstream to %s: %w", upstream, err)
	}

	return nil
}

// UnsetUpstream removes the upstream branch for the current branch.
func (r *Repository) UnsetUpstream() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("branch", "--unset-upstream"); err != nil {
		return fmt.Errorf("unset upstream: %w", err)
	}

	return nil
}

// GetUpstream returns the upstream branch for the current branch.
func (r *Repository) GetUpstream() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	output, err := r.git("rev-parse", "--abbrev-ref", "@{upstream}")
	if err != nil {
		if strings.Contains(err.Error(), "no upstream") {
			return "", ErrNoUpstream
		}
		return "", fmt.Errorf("get upstream: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// PruneRemote prunes stale remote-tracking branches.
func (r *Repository) PruneRemote(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("remote", "prune", name); err != nil {
		return fmt.Errorf("prune remote %s: %w", name, err)
	}

	r.publishEvent("git.remote.pruned", map[string]any{
		"name": name,
	})

	return nil
}

// Clone clones a repository.
func Clone(url, path string, opts CloneOptions) error {
	args := []string{"clone", url, path}

	if opts.Branch != "" {
		args = append(args, "-b", opts.Branch)
	}

	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	if opts.Bare {
		args = append(args, "--bare")
	}

	if opts.Mirror {
		args = append(args, "--mirror")
	}

	if opts.Recursive {
		args = append(args, "--recursive")
	}

	// Execute git clone directly (not through a repository)
	cmd := newGitCommand(path, args...)
	if _, err := cmd.run(); err != nil {
		return fmt.Errorf("clone %s: %w", url, err)
	}

	return nil
}

// CloneOptions configures clone behavior.
type CloneOptions struct {
	// Branch is the branch to clone.
	Branch string

	// Depth limits clone to the specified number of commits.
	Depth int

	// Bare creates a bare repository.
	Bare bool

	// Mirror creates a mirror clone.
	Mirror bool

	// Recursive clones submodules.
	Recursive bool
}
