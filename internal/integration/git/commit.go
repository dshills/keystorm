package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// commitLogFormat is the format string for parsing git log output.
// Format: Hash, ShortHash, Subject, AuthorName, AuthorEmail, AuthorTime, CommitterName, CommitterEmail, CommitTime, Parents
const commitLogFormat = "%H%n%h%n%s%n%an%n%ae%n%at%n%cn%n%ce%n%ct%n%P"

// Commit creates a new commit with the given message.
func (r *Repository) Commit(message string, opts CommitOptions) (*Commit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get status once for all checks (avoids redundant git calls and race conditions)
	status, err := r.statusLocked()
	if err != nil {
		return nil, fmt.Errorf("check status: %w", err)
	}

	// Check for conflicts first
	if status.HasConflicts() {
		return nil, ErrConflict
	}

	// Check if there are staged changes (unless AllowEmpty or Amend)
	if !opts.AllowEmpty && !opts.Amend {
		if !status.HasStagedChanges() {
			return nil, ErrNothingToCommit
		}
	}

	// Build commit command
	args := []string{"commit", "-m", message}

	if opts.Amend {
		args = append(args, "--amend")
	}

	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}

	if opts.Author != "" && opts.AuthorEmail != "" {
		args = append(args, "--author", fmt.Sprintf("%s <%s>", opts.Author, opts.AuthorEmail))
	}

	if opts.SignOff {
		args = append(args, "--signoff")
	}

	// Execute commit
	output, err := r.git(args...)
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Invalidate cache
	r.statusCache = nil

	// Get the new commit info
	commit, err := r.getHeadCommit()
	if err != nil {
		// Commit succeeded but we couldn't get details
		// Return minimal commit with a wrapped error for visibility
		return &Commit{
			Message: message,
		}, fmt.Errorf("commit succeeded but failed to retrieve details: %w", err)
	}

	// Publish event
	r.publishEvent("git.commit.created", map[string]any{
		"hash":    commit.Hash,
		"message": message,
		"amend":   opts.Amend,
		"output":  output,
	})

	return commit, nil
}

// getHeadCommit returns the HEAD commit.
func (r *Repository) getHeadCommit() (*Commit, error) {
	output, err := r.git("log", "-1", "--format="+commitLogFormat)
	if err != nil {
		return nil, fmt.Errorf("get head commit: %w", err)
	}

	return parseCommitOutput(output)
}

// parseCommitOutput parses git log output into a Commit.
// Expected format (10 lines): Hash, ShortHash, Subject, AuthorName, AuthorEmail,
// AuthorTime, CommitterName, CommitterEmail, CommitTime, Parents
func parseCommitOutput(output string) (*Commit, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 9 {
		return nil, fmt.Errorf("invalid commit output: expected at least 9 lines, got %d", len(lines))
	}

	authorTime, err := strconv.ParseInt(lines[5], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse author time: %w", err)
	}
	commitTime, err := strconv.ParseInt(lines[8], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse commit time: %w", err)
	}

	commit := &Commit{
		Hash:           lines[0],
		ShortHash:      lines[1],
		Message:        lines[2],
		Author:         lines[3],
		AuthorEmail:    lines[4],
		AuthorTime:     time.Unix(authorTime, 0),
		Committer:      lines[6],
		CommitterEmail: lines[7],
		CommitTime:     time.Unix(commitTime, 0),
	}

	// Parse parent hashes
	if len(lines) > 9 && lines[9] != "" {
		commit.Parents = strings.Fields(lines[9])
	}

	return commit, nil
}

// Log returns commit history.
func (r *Repository) Log(opts LogOptions) ([]*Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build args in correct order: log [options] [ref] [--] [path]
	args := []string{"log", "--format=" + commitLogFormat + "%x00"}

	if opts.MaxCount > 0 {
		args = append(args, fmt.Sprintf("-n%d", opts.MaxCount))
	}

	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}

	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}

	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}

	// Add ref before path separator
	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}

	// Add path with separator
	if opts.Path != "" {
		args = append(args, "--", opts.Path)
	}

	output, err := r.git(args...)
	if err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}

	// Parse commits (separated by null bytes)
	entries := strings.Split(output, "\x00")

	// Pre-allocate based on expected count
	capacity := len(entries)
	if opts.MaxCount > 0 && opts.MaxCount < capacity {
		capacity = opts.MaxCount
	}
	commits := make([]*Commit, 0, capacity)

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		commit, err := parseCommitOutput(entry)
		if err != nil {
			// Log parse errors but continue - partial results are still useful
			continue
		}
		commits = append(commits, commit)
	}

	return commits, nil
}

// LogOptions configures log queries.
type LogOptions struct {
	// MaxCount limits the number of commits returned.
	MaxCount int

	// Ref is the starting reference (branch, tag, commit).
	Ref string

	// Path filters commits to those affecting a path.
	Path string

	// Author filters by author.
	Author string

	// Since filters commits after this date.
	Since string

	// Until filters commits before this date.
	Until string
}

// GetCommit retrieves a specific commit by hash.
func (r *Repository) GetCommit(hash string) (*Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	output, err := r.git("log", "-1", "--format="+commitLogFormat, hash)
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", hash, err)
	}

	return parseCommitOutput(output)
}

// GetCommitMessage returns the full commit message for a commit.
func (r *Repository) GetCommitMessage(hash string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	output, err := r.git("log", "-1", "--format=%B", hash)
	if err != nil {
		return "", fmt.Errorf("get commit message %s: %w", hash, err)
	}

	return strings.TrimSpace(output), nil
}

// GetCommitDiff returns the diff for a specific commit.
func (r *Repository) GetCommitDiff(hash string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	output, err := r.git("show", "--format=", "--patch", hash)
	if err != nil {
		return "", fmt.Errorf("get commit diff %s: %w", hash, err)
	}

	return output, nil
}

// GetCommitFiles returns the files changed in a commit.
func (r *Repository) GetCommitFiles(hash string) ([]FileStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use -z for null-terminated output to handle paths with special characters
	output, err := r.git("show", "--format=", "--name-status", "-z", hash)
	if err != nil {
		return nil, fmt.Errorf("get commit files %s: %w", hash, err)
	}

	var files []FileStatus
	// With -z, entries are null-separated: status\0path\0 (or status\0oldpath\0newpath\0 for renames)
	parts := strings.Split(output, "\x00")
	for i := 0; i < len(parts); {
		status := strings.TrimSpace(parts[i])
		if status == "" {
			i++
			continue
		}

		if i+1 >= len(parts) {
			break
		}

		fs := FileStatus{}

		switch status[0] {
		case 'A':
			fs.Status = StatusAdded
			fs.Path = parts[i+1]
			i += 2
		case 'M':
			fs.Status = StatusModified
			fs.Path = parts[i+1]
			i += 2
		case 'D':
			fs.Status = StatusDeleted
			fs.Path = parts[i+1]
			i += 2
		case 'R':
			fs.Status = StatusRenamed
			// Renamed: status\0oldpath\0newpath\0
			if i+2 < len(parts) {
				fs.OldPath = parts[i+1]
				fs.Path = parts[i+2]
				i += 3
			} else {
				fs.Path = parts[i+1]
				i += 2
			}
		case 'C':
			fs.Status = StatusCopied
			// Copied: status\0oldpath\0newpath\0
			if i+2 < len(parts) {
				fs.OldPath = parts[i+1]
				fs.Path = parts[i+2]
				i += 3
			} else {
				fs.Path = parts[i+1]
				i += 2
			}
		default:
			fs.Status = StatusModified
			fs.Path = parts[i+1]
			i += 2
		}

		if fs.Path != "" {
			files = append(files, fs)
		}
	}

	return files, nil
}
