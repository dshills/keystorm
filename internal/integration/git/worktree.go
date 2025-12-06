package git

import (
	"fmt"
	"os"
	"path/filepath"
)

// Stage stages files for commit.
// If no paths are provided, nothing is staged.
func (r *Repository) Stage(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate paths exist
	for _, p := range paths {
		fullPath := filepath.Join(r.path, p)
		if _, err := os.Stat(fullPath); err != nil {
			// File might be deleted, check if it's tracked
			if os.IsNotExist(err) {
				// Check if it's a deleted tracked file
				output, gitErr := r.git("ls-files", "--error-unmatch", p)
				if gitErr != nil || output == "" {
					return fmt.Errorf("%w: %s", ErrPathNotFound, p)
				}
			} else {
				return fmt.Errorf("stat %s: %w", p, err)
			}
		}
	}

	// Stage the files
	args := append([]string{"add", "--"}, paths...)
	if _, err := r.git(args...); err != nil {
		return fmt.Errorf("stage files: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "stage",
		"paths":  paths,
	})

	return nil
}

// StageAll stages all changes (tracked and untracked).
func (r *Repository) StageAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("add", "-A"); err != nil {
		return fmt.Errorf("stage all: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "stage_all",
	})

	return nil
}

// Unstage unstages files from the index.
// If no paths are provided, nothing is unstaged.
func (r *Repository) Unstage(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Use git reset to unstage
	args := append([]string{"reset", "HEAD", "--"}, paths...)
	if _, err := r.git(args...); err != nil {
		// Check if this is an initial commit (no HEAD yet)
		if _, headErr := r.git("rev-parse", "HEAD"); headErr != nil {
			// Initial commit - use rm --cached
			rmArgs := append([]string{"rm", "--cached", "--"}, paths...)
			if _, rmErr := r.git(rmArgs...); rmErr != nil {
				return fmt.Errorf("unstage files: %w", rmErr)
			}
		} else {
			return fmt.Errorf("unstage files: %w", err)
		}
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "unstage",
		"paths":  paths,
	})

	return nil
}

// UnstageAll unstages all staged changes.
func (r *Repository) UnstageAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("reset", "HEAD"); err != nil {
		// Check if this is an initial commit
		if _, headErr := r.git("rev-parse", "HEAD"); headErr != nil {
			// Initial commit - unstage everything
			if _, rmErr := r.git("rm", "--cached", "-r", "."); rmErr != nil {
				return fmt.Errorf("unstage all: %w", rmErr)
			}
		} else {
			return fmt.Errorf("unstage all: %w", err)
		}
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "unstage_all",
	})

	return nil
}

// Discard discards changes to files, restoring them to the last commit.
// This only affects unstaged changes.
func (r *Repository) Discard(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Use checkout to restore files
	args := append([]string{"checkout", "--"}, paths...)
	if _, err := r.git(args...); err != nil {
		return fmt.Errorf("discard changes: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "discard",
		"paths":  paths,
	})

	return nil
}

// DiscardAll discards all unstaged changes.
func (r *Repository) DiscardAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Checkout all tracked files
	if _, err := r.git("checkout", "--", "."); err != nil {
		return fmt.Errorf("discard all: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "discard_all",
	})

	return nil
}

// Clean removes untracked files.
func (r *Repository) Clean(paths ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"clean", "-f"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}

	if _, err := r.git(args...); err != nil {
		return fmt.Errorf("clean: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "clean",
		"paths":  paths,
	})

	return nil
}

// CleanAll removes all untracked files and directories.
func (r *Repository) CleanAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// -f force, -d directories, -x ignored files too
	if _, err := r.git("clean", "-fd"); err != nil {
		return fmt.Errorf("clean all: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "clean_all",
	})

	return nil
}

// Stash stashes working tree changes.
func (r *Repository) Stash(message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}

	if _, err := r.git(args...); err != nil {
		return fmt.Errorf("stash: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action":  "stash",
		"message": message,
	})

	return nil
}

// StashPop pops the most recent stash.
func (r *Repository) StashPop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.git("stash", "pop"); err != nil {
		return fmt.Errorf("stash pop: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Publish event
	r.publishEvent("git.status.changed", map[string]any{
		"action": "stash_pop",
	})

	return nil
}

// StashList returns the list of stashes.
func (r *Repository) StashList() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lines, err := r.gitLines("stash", "list")
	if err != nil {
		return nil, fmt.Errorf("stash list: %w", err)
	}

	return lines, nil
}
