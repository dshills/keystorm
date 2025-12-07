package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Repository represents a git repository.
type Repository struct {
	path string

	mu sync.RWMutex

	// Status cache
	statusCache     *Status
	statusCacheTime time.Time
	statusCacheTTL  time.Duration

	// Event publishing
	eventBus EventPublisher
}

// openRepository opens an existing git repository.
func openRepository(path string, cacheTTL time.Duration, eventBus EventPublisher) (*Repository, error) {
	// Verify it's a git repository
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotRepository
		}
		return nil, fmt.Errorf("stat .git: %w", err)
	}

	// .git can be a directory or a file (for worktrees)
	if !info.IsDir() {
		// Could be a worktree, read the gitdir from the file
		content, err := os.ReadFile(gitDir)
		if err != nil {
			return nil, fmt.Errorf("read .git file: %w", err)
		}
		if !bytes.HasPrefix(content, []byte("gitdir:")) {
			return nil, ErrNotRepository
		}
	}

	return &Repository{
		path:           path,
		statusCacheTTL: cacheTTL,
		eventBus:       eventBus,
	}, nil
}

// discoverRepository finds the repository root from any path within it.
func discoverRepository(path string) (string, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}

	// Walk up the directory tree
	current := absPath
	for {
		gitDir := filepath.Join(current, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return current, nil
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root
			return "", ErrRepositoryNotFound
		}
		current = parent
	}
}

// Path returns the repository root path.
func (r *Repository) Path() string {
	return r.path
}

// Head returns the current HEAD reference.
func (r *Repository) Head() (*Reference, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Read HEAD
	headPath := filepath.Join(r.path, ".git", "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		return nil, fmt.Errorf("read HEAD: %w", err)
	}

	content = bytes.TrimSpace(content)

	// Check if it's a symbolic reference
	if bytes.HasPrefix(content, []byte("ref: ")) {
		refName := string(content[5:])
		shortName := strings.TrimPrefix(refName, "refs/heads/")

		// Read the referenced commit
		hash, err := r.resolveRef(refName)
		if err != nil {
			// New repository with no commits
			if strings.Contains(err.Error(), "no such file") {
				return &Reference{
					Name:      refName,
					ShortName: shortName,
				}, nil
			}
			return nil, err
		}

		return &Reference{
			Name:      refName,
			ShortName: shortName,
			Hash:      hash,
		}, nil
	}

	// Detached HEAD - content is a commit hash
	hash := string(content)
	return &Reference{
		Name:      hash,
		ShortName: hash[:7],
		Hash:      hash,
	}, nil
}

// resolveRef resolves a reference to its commit hash.
func (r *Repository) resolveRef(refName string) (string, error) {
	// Try reading from .git/refs
	refPath := filepath.Join(r.path, ".git", refName)
	content, err := os.ReadFile(refPath)
	if err == nil {
		return strings.TrimSpace(string(content)), nil
	}

	// Try packed-refs
	packedRefsPath := filepath.Join(r.path, ".git", "packed-refs")
	packedRefs, err := os.ReadFile(packedRefsPath)
	if err != nil {
		return "", fmt.Errorf("resolve ref %s: %w", refName, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(packedRefs))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == refName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("ref not found: %s", refName)
}

// git executes a git command in the repository.
func (r *Repository) git(args ...string) (string, error) {
	cmd := newGitCommand(r.path, args...)
	return cmd.run()
}

// gitCommand represents a git command to execute outside a repository context.
// This is used by Clone and other operations that don't require an existing repo.
type gitCommand struct {
	dir  string
	args []string
}

// newGitCommand creates a new git command.
func newGitCommand(dir string, args ...string) *gitCommand {
	return &gitCommand{dir: dir, args: args}
}

// run executes the git command.
func (c *gitCommand) run() (string, error) {
	cmd := exec.Command("git", c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(c.args, " "), strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// toExecCmd converts gitCommand to an exec.Cmd for custom stdin/stdout handling.
func (c *gitCommand) toExecCmd() *exec.Cmd {
	cmd := exec.Command("git", c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	return cmd
}

// gitLines executes a git command and returns output lines.
func (r *Repository) gitLines(args ...string) ([]string, error) {
	output, err := r.git(args...)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return nil, nil
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// Status returns the working tree status.
// Results are cached for performance.
func (r *Repository) Status() (*Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache
	if r.statusCache != nil && time.Since(r.statusCacheTime) < r.statusCacheTTL {
		return r.statusCache, nil
	}

	status, err := r.statusLocked()
	if err != nil {
		return nil, err
	}

	r.statusCache = status
	r.statusCacheTime = time.Now()
	return status, nil
}

// statusLocked fetches fresh status (caller must hold lock).
func (r *Repository) statusLocked() (*Status, error) {
	status := &Status{}

	// Get branch info
	branchOutput, err := r.git("branch", "--show-current")
	if err == nil {
		status.Branch = strings.TrimSpace(branchOutput)
	}

	if status.Branch == "" {
		// Detached HEAD
		status.IsDetached = true
		head, err := r.Head()
		if err == nil && head.Hash != "" {
			status.HeadCommit = head.Hash[:7]
		}
	}

	// Get ahead/behind counts
	if status.Branch != "" && !status.IsDetached {
		upstream, err := r.git("rev-parse", "--abbrev-ref", status.Branch+"@{upstream}")
		if err == nil {
			status.Upstream = strings.TrimSpace(upstream)

			// Get ahead/behind
			revList, err := r.git("rev-list", "--left-right", "--count", status.Branch+"..."+status.Upstream)
			if err == nil {
				parts := strings.Fields(revList)
				if len(parts) >= 2 {
					status.Ahead, _ = strconv.Atoi(parts[0])
					status.Behind, _ = strconv.Atoi(parts[1])
				}
			}
		}
	}

	// Get file status using porcelain v2
	output, err := r.git("status", "--porcelain=v2", "--branch", "--untracked-files=all")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		switch line[0] {
		case '#':
			// Header line - already handled above
			continue
		case '1':
			// Ordinary changed entry
			fs := parseOrdinaryEntry(line)
			if fs != nil {
				if fs.Staged {
					status.Staged = append(status.Staged, *fs)
				} else {
					status.Unstaged = append(status.Unstaged, *fs)
				}
			}
		case '2':
			// Renamed or copied entry
			fs := parseRenamedEntry(line)
			if fs != nil {
				if fs.Staged {
					status.Staged = append(status.Staged, *fs)
				} else {
					status.Unstaged = append(status.Unstaged, *fs)
				}
			}
		case 'u':
			// Unmerged entry (conflict)
			path := parseUnmergedEntry(line)
			if path != "" {
				status.Conflicts = append(status.Conflicts, path)
			}
		case '?':
			// Untracked
			if len(line) > 2 {
				status.Untracked = append(status.Untracked, line[2:])
			}
		case '!':
			// Ignored (we don't normally see this unless requested)
			continue
		}
	}

	return status, scanner.Err()
}

// parseOrdinaryEntry parses a porcelain v2 ordinary entry.
// Format: 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
func parseOrdinaryEntry(line string) *FileStatus {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return nil
	}

	xy := fields[1]
	path := fields[8]

	// Handle paths with spaces
	if idx := strings.Index(line, fields[8]); idx > 0 {
		path = line[idx:]
	}

	// X = index status, Y = worktree status
	indexStatus := xy[0]
	worktreeStatus := xy[1]

	var result []FileStatus

	// Check index (staged) changes
	if indexStatus != '.' {
		fs := FileStatus{
			Path:   path,
			Status: charToStatus(indexStatus),
			Staged: true,
		}
		result = append(result, fs)
	}

	// Check worktree (unstaged) changes
	if worktreeStatus != '.' {
		fs := FileStatus{
			Path:   path,
			Status: charToStatus(worktreeStatus),
			Staged: false,
		}
		result = append(result, fs)
	}

	// Return staged first if exists, otherwise unstaged
	if len(result) > 0 {
		return &result[0]
	}
	return nil
}

// parseRenamedEntry parses a porcelain v2 renamed/copied entry.
// Format: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <path><tab><origPath>
func parseRenamedEntry(line string) *FileStatus {
	// Find the tab separator between new and old path
	tabIdx := strings.LastIndex(line, "\t")
	if tabIdx == -1 {
		return nil
	}

	fields := strings.Fields(line[:tabIdx])
	if len(fields) < 10 {
		return nil
	}

	xy := fields[1]
	newPath := fields[9]
	oldPath := line[tabIdx+1:]

	// Handle paths with spaces
	if idx := strings.Index(line[:tabIdx], fields[9]); idx > 0 {
		newPath = line[idx:tabIdx]
	}

	indexStatus := xy[0]
	worktreeStatus := xy[1]

	staged := indexStatus != '.'

	status := StatusRenamed
	if fields[8][0] == 'C' {
		status = StatusCopied
	}

	return &FileStatus{
		Path:    newPath,
		OldPath: oldPath,
		Status:  status,
		Staged:  staged || worktreeStatus == '.',
	}
}

// parseUnmergedEntry parses a porcelain v2 unmerged entry.
// Format: u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
func parseUnmergedEntry(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 11 {
		return ""
	}

	path := fields[10]

	// Handle paths with spaces
	if idx := strings.Index(line, fields[10]); idx > 0 {
		path = line[idx:]
	}

	return path
}

// charToStatus converts a porcelain status character to StatusCode.
func charToStatus(c byte) StatusCode {
	switch c {
	case 'M':
		return StatusModified
	case 'A':
		return StatusAdded
	case 'D':
		return StatusDeleted
	case 'R':
		return StatusRenamed
	case 'C':
		return StatusCopied
	case 'T': // Type change
		return StatusModified
	case 'U':
		return StatusConflict
	default:
		return StatusUnmodified
	}
}

// invalidateStatusCache invalidates the status cache.
func (r *Repository) invalidateStatusCache() {
	r.mu.Lock()
	r.statusCache = nil
	r.mu.Unlock()
}

// close closes the repository.
func (r *Repository) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statusCache = nil
}

// publishEvent publishes an event if an event bus is configured.
func (r *Repository) publishEvent(eventType string, data map[string]any) {
	if r.eventBus != nil {
		if data == nil {
			data = make(map[string]any)
		}
		data["repository"] = r.path
		data["timestamp"] = time.Now().UnixMilli()
		r.eventBus.Publish(eventType, data)
	}
}
