package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BlameLine represents a single line in a blame output.
type BlameLine struct {
	// Hash is the commit hash that introduced this line.
	Hash string

	// Author is the author who introduced this line.
	Author string

	// AuthorEmail is the author's email.
	AuthorEmail string

	// AuthorTime is when the author created the commit.
	AuthorTime time.Time

	// Committer is the committer name.
	Committer string

	// CommitterEmail is the committer's email.
	CommitterEmail string

	// CommitTime is when the commit was created.
	CommitTime time.Time

	// Summary is the commit message summary.
	Summary string

	// LineNo is the line number (1-based).
	LineNo int

	// OriginalLineNo is the original line number in the source commit.
	OriginalLineNo int

	// Content is the line content.
	Content string

	// IsBoundary indicates if this is a boundary commit.
	IsBoundary bool
}

// BlameResult represents the complete blame for a file.
type BlameResult struct {
	// Path is the file path.
	Path string

	// Lines contains the blame information for each line.
	Lines []BlameLine
}

// Blame returns blame information for a file.
func (r *Repository) Blame(path string, opts BlameOptions) (*BlameResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	args := []string{"blame", "--porcelain"}

	if opts.Rev != "" {
		args = append(args, opts.Rev)
	}

	if opts.StartLine > 0 && opts.EndLine > 0 {
		args = append(args, fmt.Sprintf("-L%d,%d", opts.StartLine, opts.EndLine))
	}

	if opts.IgnoreWhitespace {
		args = append(args, "-w")
	}

	if opts.IgnoreRevisions != "" {
		args = append(args, "--ignore-revs-file", opts.IgnoreRevisions)
	}

	args = append(args, "--", path)

	output, err := r.git(args...)
	if err != nil {
		return nil, fmt.Errorf("blame %s: %w", path, err)
	}

	return parseBlame(path, output), nil
}

// BlameOptions configures blame behavior.
type BlameOptions struct {
	// Rev is the revision to blame (default: HEAD).
	Rev string

	// StartLine limits blame to lines starting at this line (1-based).
	StartLine int

	// EndLine limits blame to lines ending at this line.
	EndLine int

	// IgnoreWhitespace ignores whitespace changes.
	IgnoreWhitespace bool

	// IgnoreRevisions is a file containing revisions to ignore.
	IgnoreRevisions string
}

// BlameRange returns blame for a specific line range.
func (r *Repository) BlameRange(path string, startLine, endLine int) (*BlameResult, error) {
	return r.Blame(path, BlameOptions{
		StartLine: startLine,
		EndLine:   endLine,
	})
}

// BlameLine returns blame for a specific line.
func (r *Repository) BlameLine(path string, lineNo int) (*BlameLine, error) {
	result, err := r.Blame(path, BlameOptions{
		StartLine: lineNo,
		EndLine:   lineNo,
	})
	if err != nil {
		return nil, err
	}

	if len(result.Lines) == 0 {
		return nil, fmt.Errorf("line %d not found in %s", lineNo, path)
	}

	return &result.Lines[0], nil
}

// parseBlame parses git blame --porcelain output.
func parseBlame(path, output string) *BlameResult {
	result := &BlameResult{
		Path: path,
	}

	if output == "" {
		return result
	}

	lines := strings.Split(output, "\n")
	commits := make(map[string]*blameCommitInfo)
	var currentHash string
	var currentOrigLine, currentFinalLine int
	var currentContent string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}

		// First line of a group: <sha> <orig-line> <final-line> [count]
		if len(line) >= 40 && !strings.HasPrefix(line, "\t") {
			parts := strings.Fields(line)
			if len(parts) >= 3 && len(parts[0]) == 40 {
				currentHash = parts[0]
				currentOrigLine, _ = strconv.Atoi(parts[1])
				currentFinalLine, _ = strconv.Atoi(parts[2])

				// Initialize commit info if needed
				if _, ok := commits[currentHash]; !ok {
					commits[currentHash] = &blameCommitInfo{Hash: currentHash}
				}
				continue
			}
		}

		// Header lines
		if strings.HasPrefix(line, "author ") {
			if info, ok := commits[currentHash]; ok {
				info.Author = strings.TrimPrefix(line, "author ")
			}
			continue
		}

		if strings.HasPrefix(line, "author-mail ") {
			if info, ok := commits[currentHash]; ok {
				email := strings.TrimPrefix(line, "author-mail ")
				info.AuthorEmail = strings.Trim(email, "<>")
			}
			continue
		}

		if strings.HasPrefix(line, "author-time ") {
			if info, ok := commits[currentHash]; ok {
				ts, _ := strconv.ParseInt(strings.TrimPrefix(line, "author-time "), 10, 64)
				info.AuthorTime = time.Unix(ts, 0)
			}
			continue
		}

		if strings.HasPrefix(line, "committer ") {
			if info, ok := commits[currentHash]; ok {
				info.Committer = strings.TrimPrefix(line, "committer ")
			}
			continue
		}

		if strings.HasPrefix(line, "committer-mail ") {
			if info, ok := commits[currentHash]; ok {
				email := strings.TrimPrefix(line, "committer-mail ")
				info.CommitterEmail = strings.Trim(email, "<>")
			}
			continue
		}

		if strings.HasPrefix(line, "committer-time ") {
			if info, ok := commits[currentHash]; ok {
				ts, _ := strconv.ParseInt(strings.TrimPrefix(line, "committer-time "), 10, 64)
				info.CommitTime = time.Unix(ts, 0)
			}
			continue
		}

		if strings.HasPrefix(line, "summary ") {
			if info, ok := commits[currentHash]; ok {
				info.Summary = strings.TrimPrefix(line, "summary ")
			}
			continue
		}

		if strings.HasPrefix(line, "boundary") {
			if info, ok := commits[currentHash]; ok {
				info.IsBoundary = true
			}
			continue
		}

		// Skip other header lines
		if strings.HasPrefix(line, "author-tz ") ||
			strings.HasPrefix(line, "committer-tz ") ||
			strings.HasPrefix(line, "previous ") ||
			strings.HasPrefix(line, "filename ") {
			continue
		}

		// Content line (starts with tab)
		if strings.HasPrefix(line, "\t") {
			currentContent = strings.TrimPrefix(line, "\t")

			// Create blame line
			info := commits[currentHash]
			blameLine := BlameLine{
				Hash:           currentHash,
				Author:         info.Author,
				AuthorEmail:    info.AuthorEmail,
				AuthorTime:     info.AuthorTime,
				Committer:      info.Committer,
				CommitterEmail: info.CommitterEmail,
				CommitTime:     info.CommitTime,
				Summary:        info.Summary,
				LineNo:         currentFinalLine,
				OriginalLineNo: currentOrigLine,
				Content:        currentContent,
				IsBoundary:     info.IsBoundary,
			}
			result.Lines = append(result.Lines, blameLine)
		}
	}

	return result
}

// blameCommitInfo caches commit info during blame parsing.
type blameCommitInfo struct {
	Hash           string
	Author         string
	AuthorEmail    string
	AuthorTime     time.Time
	Committer      string
	CommitterEmail string
	CommitTime     time.Time
	Summary        string
	IsBoundary     bool
}

// GetLastModifier returns who last modified a specific line.
func (r *Repository) GetLastModifier(path string, lineNo int) (author string, hash string, err error) {
	line, err := r.BlameLine(path, lineNo)
	if err != nil {
		return "", "", err
	}
	return line.Author, line.Hash, nil
}

// GetFileHistory returns the commits that modified a file.
func (r *Repository) GetFileHistory(path string, maxCount int) ([]*Commit, error) {
	opts := LogOptions{
		Path: path,
	}
	if maxCount > 0 {
		opts.MaxCount = maxCount
	}
	return r.Log(opts)
}
