package git

import (
	"fmt"
	"strconv"
	"strings"
)

// DiffHunk represents a single hunk in a diff.
type DiffHunk struct {
	// OldStart is the starting line in the old file.
	OldStart int

	// OldLines is the number of lines from the old file.
	OldLines int

	// NewStart is the starting line in the new file.
	NewStart int

	// NewLines is the number of lines in the new file.
	NewLines int

	// Header is the hunk header (e.g., "@@ -1,3 +1,4 @@").
	Header string

	// Lines are the diff lines in this hunk.
	Lines []DiffLine
}

// DiffLine represents a single line in a diff.
type DiffLine struct {
	// Type is the line type: ' ' (context), '+' (addition), '-' (deletion).
	Type byte

	// Content is the line content (without the type prefix).
	Content string

	// OldLineNo is the line number in the old file (0 for additions).
	OldLineNo int

	// NewLineNo is the line number in the new file (0 for deletions).
	NewLineNo int
}

// FileDiff represents the diff for a single file.
type FileDiff struct {
	// OldPath is the path in the old version.
	OldPath string

	// NewPath is the path in the new version.
	NewPath string

	// Status is the change type (added, modified, deleted, renamed).
	Status StatusCode

	// IsBinary indicates if this is a binary file.
	IsBinary bool

	// Hunks contains the diff hunks.
	Hunks []DiffHunk

	// Stats contains line statistics.
	Stats DiffStats
}

// DiffStats contains diff statistics.
type DiffStats struct {
	// Additions is the number of added lines.
	Additions int

	// Deletions is the number of deleted lines.
	Deletions int
}

// Diff represents a complete diff.
type Diff struct {
	// Files contains the per-file diffs.
	Files []FileDiff

	// Stats contains aggregate statistics.
	Stats DiffStats
}

// DiffStaged returns the diff of staged changes.
func (r *Repository) DiffStaged() (*Diff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.diffLocked("--cached")
}

// DiffUnstaged returns the diff of unstaged changes.
func (r *Repository) DiffUnstaged() (*Diff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.diffLocked()
}

// DiffAll returns the diff of all changes (staged + unstaged).
func (r *Repository) DiffAll() (*Diff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.diffLocked("HEAD")
}

// DiffFile returns the diff for a specific file.
func (r *Repository) DiffFile(path string, staged bool) (*Diff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if staged {
		return r.diffLocked("--cached", "--", path)
	}
	return r.diffLocked("--", path)
}

// DiffCommits returns the diff between two commits.
func (r *Repository) DiffCommits(from, to string) (*Diff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.diffLocked(from + ".." + to)
}

// DiffCommit returns the diff for a single commit.
func (r *Repository) DiffCommit(hash string) (*Diff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// For the first commit, we need special handling
	output, err := r.git("diff-tree", "--root", "-p", "-M", "--no-commit-id", hash)
	if err != nil {
		return nil, fmt.Errorf("diff commit %s: %w", hash, err)
	}

	return parseDiff(output), nil
}

// DiffBranches returns the diff between two branches.
func (r *Repository) DiffBranches(from, to string) (*Diff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.diffLocked(from + "..." + to)
}

// diffLocked executes git diff with the given args (caller must hold lock).
func (r *Repository) diffLocked(args ...string) (*Diff, error) {
	fullArgs := append([]string{"diff", "-M"}, args...)
	output, err := r.git(fullArgs...)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	return parseDiff(output), nil
}

// DiffRaw returns the raw diff output as a string.
func (r *Repository) DiffRaw(opts DiffOptions) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	args := []string{"diff"}

	if opts.Staged {
		args = append(args, "--cached")
	}

	if opts.Stat {
		args = append(args, "--stat")
	}

	if opts.NumStat {
		args = append(args, "--numstat")
	}

	if opts.NameOnly {
		args = append(args, "--name-only")
	}

	if opts.NameStatus {
		args = append(args, "--name-status")
	}

	if opts.Context >= 0 {
		args = append(args, fmt.Sprintf("-U%d", opts.Context))
	}

	if opts.IgnoreWhitespace {
		args = append(args, "-w")
	}

	if opts.IgnoreSpaceChange {
		args = append(args, "-b")
	}

	if opts.From != "" {
		args = append(args, opts.From)
	}

	if opts.To != "" {
		args = append(args, opts.To)
	}

	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}

	output, err := r.git(args...)
	if err != nil {
		return "", fmt.Errorf("diff: %w", err)
	}

	return output, nil
}

// DiffOptions configures diff generation.
type DiffOptions struct {
	// Staged shows diff of staged changes.
	Staged bool

	// From is the starting ref (commit, branch, etc.).
	From string

	// To is the ending ref.
	To string

	// Paths limits diff to specific paths.
	Paths []string

	// Context is the number of context lines.
	Context int

	// Stat shows diffstat.
	Stat bool

	// NumStat shows numstat.
	NumStat bool

	// NameOnly shows only file names.
	NameOnly bool

	// NameStatus shows file names with status.
	NameStatus bool

	// IgnoreWhitespace ignores all whitespace.
	IgnoreWhitespace bool

	// IgnoreSpaceChange ignores space changes.
	IgnoreSpaceChange bool
}

// DiffStat returns diff statistics.
func (r *Repository) DiffStat(staged bool) ([]FileDiffStat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	args := []string{"diff", "--numstat"}
	if staged {
		args = append(args, "--cached")
	}

	output, err := r.git(args...)
	if err != nil {
		return nil, fmt.Errorf("diff stat: %w", err)
	}

	var stats []FileDiffStat
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		stat := FileDiffStat{
			Path: parts[2],
		}

		// Binary files show as "-" for additions/deletions
		if parts[0] != "-" {
			stat.Additions, _ = strconv.Atoi(parts[0])
		} else {
			stat.IsBinary = true
		}
		if parts[1] != "-" {
			stat.Deletions, _ = strconv.Atoi(parts[1])
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// FileDiffStat contains statistics for a single file.
type FileDiffStat struct {
	// Path is the file path.
	Path string

	// Additions is the number of added lines.
	Additions int

	// Deletions is the number of deleted lines.
	Deletions int

	// IsBinary indicates if this is a binary file.
	IsBinary bool
}

// parseDiff parses git diff output into a Diff struct.
func parseDiff(output string) *Diff {
	diff := &Diff{}
	if output == "" {
		return diff
	}

	lines := strings.Split(output, "\n")
	var currentFile *FileDiff
	var currentHunk *DiffHunk
	oldLine, newLine := 0, 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// File header: diff --git a/path b/path
		if strings.HasPrefix(line, "diff --git ") {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				diff.Files = append(diff.Files, *currentFile)
			}
			currentFile = &FileDiff{}
			currentHunk = nil

			// Parse paths from header
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				currentFile.OldPath = strings.TrimPrefix(parts[2], "a/")
				currentFile.NewPath = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		// Index line
		if strings.HasPrefix(line, "index ") {
			continue
		}

		// Old file mode
		if strings.HasPrefix(line, "old mode ") {
			continue
		}

		// New file mode
		if strings.HasPrefix(line, "new mode ") {
			continue
		}

		// New file
		if strings.HasPrefix(line, "new file mode ") {
			currentFile.Status = StatusAdded
			continue
		}

		// Deleted file
		if strings.HasPrefix(line, "deleted file mode ") {
			currentFile.Status = StatusDeleted
			continue
		}

		// Renamed file
		if strings.HasPrefix(line, "similarity index ") {
			currentFile.Status = StatusRenamed
			continue
		}

		if strings.HasPrefix(line, "rename from ") {
			currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
			continue
		}

		if strings.HasPrefix(line, "rename to ") {
			currentFile.NewPath = strings.TrimPrefix(line, "rename to ")
			continue
		}

		// Binary file
		if strings.HasPrefix(line, "Binary files ") {
			currentFile.IsBinary = true
			continue
		}

		// Old file path
		if strings.HasPrefix(line, "--- ") {
			path := strings.TrimPrefix(line, "--- ")
			if path != "/dev/null" {
				currentFile.OldPath = strings.TrimPrefix(path, "a/")
			}
			continue
		}

		// New file path
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimPrefix(line, "+++ ")
			if path != "/dev/null" {
				currentFile.NewPath = strings.TrimPrefix(path, "b/")
			}
			continue
		}

		// Hunk header: @@ -start,count +start,count @@
		if strings.HasPrefix(line, "@@ ") {
			if currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}

			currentHunk = &DiffHunk{
				Header: line,
			}

			// Parse hunk header
			parts := strings.SplitN(line, "@@", 3)
			if len(parts) >= 2 {
				ranges := strings.TrimSpace(parts[1])
				rangesParts := strings.Fields(ranges)

				for _, r := range rangesParts {
					if strings.HasPrefix(r, "-") {
						nums := strings.Split(strings.TrimPrefix(r, "-"), ",")
						if len(nums) >= 1 {
							currentHunk.OldStart, _ = strconv.Atoi(nums[0])
						}
						if len(nums) >= 2 {
							currentHunk.OldLines, _ = strconv.Atoi(nums[1])
						} else {
							currentHunk.OldLines = 1
						}
					} else if strings.HasPrefix(r, "+") {
						nums := strings.Split(strings.TrimPrefix(r, "+"), ",")
						if len(nums) >= 1 {
							currentHunk.NewStart, _ = strconv.Atoi(nums[0])
						}
						if len(nums) >= 2 {
							currentHunk.NewLines, _ = strconv.Atoi(nums[1])
						} else {
							currentHunk.NewLines = 1
						}
					}
				}
			}

			oldLine = currentHunk.OldStart
			newLine = currentHunk.NewStart

			// Set default status to modified if not set
			if currentFile.Status == 0 {
				currentFile.Status = StatusModified
			}
			continue
		}

		// Diff lines
		if currentHunk != nil && len(line) > 0 {
			diffLine := DiffLine{
				Type:    line[0],
				Content: "",
			}
			if len(line) > 1 {
				diffLine.Content = line[1:]
			}

			switch line[0] {
			case '+':
				diffLine.NewLineNo = newLine
				newLine++
				currentFile.Stats.Additions++
				diff.Stats.Additions++
			case '-':
				diffLine.OldLineNo = oldLine
				oldLine++
				currentFile.Stats.Deletions++
				diff.Stats.Deletions++
			case ' ':
				diffLine.OldLineNo = oldLine
				diffLine.NewLineNo = newLine
				oldLine++
				newLine++
			case '\\':
				// "\ No newline at end of file"
				diffLine.Content = line
			default:
				continue
			}

			currentHunk.Lines = append(currentHunk.Lines, diffLine)
		}
	}

	// Add last file and hunk
	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		diff.Files = append(diff.Files, *currentFile)
	}

	return diff
}

// ApplyPatch applies a diff/patch to the working tree.
func (r *Repository) ApplyPatch(patch string, opts ApplyOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := []string{"apply"}

	if opts.Check {
		args = append(args, "--check")
	}

	if opts.Index {
		args = append(args, "--index")
	}

	if opts.Cached {
		args = append(args, "--cached")
	}

	if opts.Reverse {
		args = append(args, "-R")
	}

	if opts.ThreeWay {
		args = append(args, "-3")
	}

	args = append(args, "-")

	// Run git apply with patch on stdin
	cmd := newGitCommand(r.path, args...)
	execCmd := cmd.toExecCmd()
	execCmd.Stdin = strings.NewReader(patch)

	var stderr strings.Builder
	execCmd.Stderr = &stderr

	if err := execCmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return fmt.Errorf("apply patch: %w: %s", err, stderrStr)
		}
		return fmt.Errorf("apply patch: %w", err)
	}

	// Invalidate status cache
	r.statusCache = nil

	return nil
}

// ApplyOptions configures patch application.
type ApplyOptions struct {
	// Check only checks if the patch applies.
	Check bool

	// Index applies to both index and working tree.
	Index bool

	// Cached applies to index only.
	Cached bool

	// Reverse reverses the patch.
	Reverse bool

	// ThreeWay attempts three-way merge.
	ThreeWay bool
}
