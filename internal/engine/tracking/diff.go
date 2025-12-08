package tracking

import (
	"strconv"
	"strings"

	"github.com/dshills/keystorm/internal/engine/rope"
)

// DiffOptions configures diff computation.
type DiffOptions struct {
	// ContextLines is the number of unchanged lines to include
	// around each change for context. Default is 3.
	ContextLines int

	// IgnoreCase performs case-insensitive comparison.
	IgnoreCase bool

	// IgnoreWhitespace ignores leading/trailing whitespace on each line.
	IgnoreWhitespace bool

	// IgnoreBlankLines treats blank lines as equal.
	IgnoreBlankLines bool

	// MaxLines limits the maximum number of lines to diff.
	// If exceeded, a heuristic diff is used. Default is 10000.
	// Set to 0 to disable the limit.
	MaxLines int

	// MaxMemoryMB limits memory usage for diff computation.
	// If the estimated memory exceeds this, a heuristic diff is used.
	// Default is 100MB. Set to 0 to disable the limit.
	MaxMemoryMB int
}

// DefaultDiffOptions returns default diff options.
func DefaultDiffOptions() DiffOptions {
	return DiffOptions{
		ContextLines: 3,
		MaxLines:     10000,
		MaxMemoryMB:  100,
	}
}

// Default limits for diff computation.
const (
	// DefaultMaxDiffLines is the default maximum lines for Myers diff.
	DefaultMaxDiffLines = 10000

	// DefaultMaxDiffMemoryMB is the default memory limit in megabytes.
	DefaultMaxDiffMemoryMB = 100
)

// DiffType indicates the type of a diff hunk.
type DiffType uint8

const (
	// DiffEqual indicates unchanged lines.
	DiffEqual DiffType = iota

	// DiffInsert indicates added lines.
	DiffInsert

	// DiffDelete indicates removed lines.
	DiffDelete
)

// String returns a human-readable representation of the diff type.
func (dt DiffType) String() string {
	switch dt {
	case DiffEqual:
		return "equal"
	case DiffInsert:
		return "insert"
	case DiffDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// LineDiff represents a line-based diff between two texts.
type LineDiff struct {
	// Type indicates whether this hunk is equal, insert, or delete.
	Type DiffType

	// OldStart is the starting line number in the old text (0-indexed).
	OldStart int

	// OldCount is the number of lines in the old text.
	OldCount int

	// NewStart is the starting line number in the new text (0-indexed).
	NewStart int

	// NewCount is the number of lines in the new text.
	NewCount int

	// Lines contains the actual line content.
	// For DiffEqual: context lines (from either version)
	// For DiffInsert: inserted lines
	// For DiffDelete: deleted lines
	Lines []string
}

// IsEmpty returns true if this diff has no lines.
func (ld LineDiff) IsEmpty() bool {
	return len(ld.Lines) == 0
}

// DiffResult contains the complete result of a diff operation.
type DiffResult struct {
	// Hunks are the individual diff hunks.
	Hunks []LineDiff

	// OldLineCount is the total line count in the old text.
	OldLineCount int

	// NewLineCount is the total line count in the new text.
	NewLineCount int
}

// HasChanges returns true if there are any differences.
func (dr DiffResult) HasChanges() bool {
	for _, hunk := range dr.Hunks {
		if hunk.Type != DiffEqual {
			return true
		}
	}
	return false
}

// InsertedLines returns the total number of inserted lines.
func (dr DiffResult) InsertedLines() int {
	count := 0
	for _, hunk := range dr.Hunks {
		for _, line := range hunk.Lines {
			if len(line) > 0 && line[0] == '+' {
				count++
			}
		}
	}
	return count
}

// DeletedLines returns the total number of deleted lines.
func (dr DiffResult) DeletedLines() int {
	count := 0
	for _, hunk := range dr.Hunks {
		for _, line := range hunk.Lines {
			if len(line) > 0 && line[0] == '-' {
				count++
			}
		}
	}
	return count
}

// ComputeLineDiff computes line-based diff between two ropes.
// Uses Myers diff algorithm for optimal results.
func ComputeLineDiff(oldRope, newRope rope.Rope, opts DiffOptions) DiffResult {
	oldLines := toLines(oldRope)
	newLines := toLines(newRope)

	return computeLineDiffFromLines(oldLines, newLines, opts)
}

// ComputeLineDiffStrings computes line-based diff between two strings.
func ComputeLineDiffStrings(oldStr, newStr string, opts DiffOptions) DiffResult {
	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	return computeLineDiffFromLines(oldLines, newLines, opts)
}

// toLines extracts lines from a rope efficiently.
func toLines(r rope.Rope) []string {
	if r.Len() == 0 {
		return []string{}
	}

	var lines []string
	iter := r.Lines()
	for iter.Next() {
		lines = append(lines, iter.Text())
	}
	return lines
}

// computeLineDiffFromLines computes diff from pre-split lines.
func computeLineDiffFromLines(oldLines, newLines []string, opts DiffOptions) DiffResult {
	n := len(oldLines)
	m := len(newLines)

	// Check line count limits
	maxLines := opts.MaxLines
	if maxLines == 0 {
		maxLines = DefaultMaxDiffLines
	}
	if maxLines > 0 && (n > maxLines || m > maxLines) {
		// Fall back to heuristic diff for large inputs
		return heuristicDiff(oldLines, newLines, opts)
	}

	// Check estimated memory usage
	// Myers algorithm uses O((n+m) * d) memory where d is the edit distance
	// In worst case, d = n + m, so memory is O((n+m)^2) integers
	// Each V vector copy is 2*(n+m)+1 integers = 8 bytes each on 64-bit
	maxMemMB := opts.MaxMemoryMB
	if maxMemMB == 0 {
		maxMemMB = DefaultMaxDiffMemoryMB
	}
	if maxMemMB > 0 {
		maxD := n + m
		// Estimated memory: maxD iterations * (2*maxD+1) * 8 bytes per int
		estimatedBytes := int64(maxD) * int64(2*maxD+1) * 8
		estimatedMB := estimatedBytes / (1024 * 1024)
		if estimatedMB > int64(maxMemMB) {
			// Fall back to heuristic diff for memory-intensive inputs
			return heuristicDiff(oldLines, newLines, opts)
		}
	}

	// Run Myers diff
	script := myersDiff(oldLines, newLines, opts)

	// Convert edit script to hunks with context
	hunks := buildHunks(oldLines, newLines, script, opts.ContextLines)

	return DiffResult{
		Hunks:        hunks,
		OldLineCount: n,
		NewLineCount: m,
	}
}

// heuristicDiff provides a simple line-by-line diff for large inputs.
// It's less optimal than Myers but uses O(n+m) memory.
func heuristicDiff(oldLines, newLines []string, opts DiffOptions) DiffResult {
	n := len(oldLines)
	m := len(newLines)

	// Build a map of old lines for quick lookup
	oldLineMap := make(map[string][]int)
	for i, line := range oldLines {
		key := normalizeLineForDiff(line, opts)
		oldLineMap[key] = append(oldLineMap[key], i)
	}

	// Track which old lines have been matched
	matched := make([]bool, n)
	newMatched := make([]bool, m)

	// First pass: find exact matches
	for j, line := range newLines {
		key := normalizeLineForDiff(line, opts)
		if indices, ok := oldLineMap[key]; ok {
			for _, i := range indices {
				if !matched[i] {
					matched[i] = true
					newMatched[j] = true
					break
				}
			}
		}
	}

	// Build edit script from matches
	var ops []editOp
	i, j := 0, 0
	for i < n || j < m {
		// Skip matched lines (they're equal)
		for i < n && j < m && matched[i] && newMatched[j] {
			ops = append(ops, editOp{op: DiffEqual, oldIndex: i, newIndex: j})
			i++
			j++
		}

		// Handle unmatched old lines (deletions)
		for i < n && !matched[i] {
			ops = append(ops, editOp{op: DiffDelete, oldIndex: i})
			i++
		}

		// Handle unmatched new lines (insertions)
		for j < m && !newMatched[j] {
			ops = append(ops, editOp{op: DiffInsert, newIndex: j})
			j++
		}
	}

	hunks := buildHunks(oldLines, newLines, ops, opts.ContextLines)

	return DiffResult{
		Hunks:        hunks,
		OldLineCount: n,
		NewLineCount: m,
	}
}

// normalizeLineForDiff normalizes a line based on diff options.
func normalizeLineForDiff(line string, opts DiffOptions) string {
	if opts.IgnoreCase {
		line = strings.ToLower(line)
	}
	if opts.IgnoreWhitespace {
		line = strings.TrimSpace(line)
	}
	return line
}

// editOp represents a single edit operation in the diff.
type editOp struct {
	op       DiffType
	oldIndex int
	newIndex int
}

// myersDiff implements the Myers diff algorithm.
// Returns a sequence of edit operations.
func myersDiff(oldLines, newLines []string, opts DiffOptions) []editOp {
	n := len(oldLines)
	m := len(newLines)

	// Handle trivial cases
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		ops := make([]editOp, m)
		for i := 0; i < m; i++ {
			ops[i] = editOp{op: DiffInsert, newIndex: i}
		}
		return ops
	}
	if m == 0 {
		ops := make([]editOp, n)
		for i := 0; i < n; i++ {
			ops[i] = editOp{op: DiffDelete, oldIndex: i}
		}
		return ops
	}

	// Myers algorithm using slice-based V vector for efficiency
	maxD := n + m
	offset := maxD // V[-max..max] maps to slice[0..2*max]
	v := make([]int, 2*maxD+1)

	// Initialize V[1] = 0 as per Myers algorithm
	v[offset+1] = 0

	var trace [][]int

	// Forward pass to find shortest edit path
outer:
	for d := 0; d <= maxD; d++ {
		// Save trace BEFORE processing this d (we need state from previous iteration)
		vCopy := make([]int, len(v))
		copy(vCopy, v)
		trace = append(trace, vCopy)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}

			y := x - k

			// Extend diagonal (equal elements)
			for x < n && y < m && linesEqual(oldLines[x], newLines[y], opts) {
				x++
				y++
			}

			v[offset+k] = x

			if x >= n && y >= m {
				// Save final state
				vFinal := make([]int, len(v))
				copy(vFinal, v)
				trace = append(trace, vFinal)
				break outer
			}
		}
	}

	// Backtrack to build edit script
	return backtrackSlice(trace, oldLines, newLines, offset, opts)
}

// linesEqual compares two lines respecting diff options.
func linesEqual(a, b string, opts DiffOptions) bool {
	if opts.IgnoreCase {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	if opts.IgnoreWhitespace {
		a = strings.TrimSpace(a)
		b = strings.TrimSpace(b)
	}
	if opts.IgnoreBlankLines && a == "" && b == "" {
		return true
	}
	return a == b
}

// backtrackSlice reconstructs the edit script from the trace using slice-based V.
func backtrackSlice(trace [][]int, oldLines, newLines []string, offset int, _ DiffOptions) []editOp {
	n := len(oldLines)
	m := len(newLines)

	if len(trace) == 0 {
		return nil
	}

	x := n
	y := m
	var ops []editOp

	// trace has d+1 entries for edit distance d (d=0,1,...,d plus final state)
	// We iterate backwards from the final edit distance
	for d := len(trace) - 2; d >= 0; d-- {
		v := trace[d]
		k := x - y

		var prevK int
		if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[offset+prevK]
		prevY := prevX - prevK

		// Walk back diagonals (equal elements)
		for x > prevX && y > prevY {
			x--
			y--
			ops = append(ops, editOp{op: DiffEqual, oldIndex: x, newIndex: y})
		}

		if d > 0 {
			if x > prevX {
				x--
				ops = append(ops, editOp{op: DiffDelete, oldIndex: x})
			} else if y > prevY {
				y--
				ops = append(ops, editOp{op: DiffInsert, newIndex: y})
			}
		}
	}

	// Reverse the ops (we built them backwards)
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}

	return ops
}

// buildHunks converts an edit script into diff hunks with context.
func buildHunks(oldLines, newLines []string, ops []editOp, contextLines int) []LineDiff {
	if len(ops) == 0 {
		return nil
	}

	var hunks []LineDiff
	var currentHunk *LineDiff
	lastChangeOldLine := -1
	_ = lastChangeOldLine // used for context window tracking

	for _, op := range ops {
		switch op.op {
		case DiffEqual:
			// Check if we need to start a new hunk or continue the current one
			if currentHunk != nil {
				// Add this line as context if within context window
				distFromLastChange := op.oldIndex - lastChangeOldLine
				if distFromLastChange <= contextLines {
					currentHunk.Lines = append(currentHunk.Lines, oldLines[op.oldIndex])
					currentHunk.OldCount++
					currentHunk.NewCount++
				} else {
					// Too far from last change, finalize current hunk
					hunks = append(hunks, *currentHunk)
					currentHunk = nil
				}
			}

		case DiffDelete:
			if currentHunk == nil {
				// Start new hunk with leading context
				startOld := op.oldIndex - contextLines
				if startOld < 0 {
					startOld = 0
				}
				startNew := op.newIndex - contextLines
				if startNew < 0 {
					startNew = 0
				}

				currentHunk = &LineDiff{
					Type:     DiffDelete,
					OldStart: startOld,
					NewStart: startNew,
				}

				// Add leading context
				for i := startOld; i < op.oldIndex; i++ {
					currentHunk.Lines = append(currentHunk.Lines, oldLines[i])
					currentHunk.OldCount++
					currentHunk.NewCount++
				}
			}

			// Add deleted line
			currentHunk.Lines = append(currentHunk.Lines, "-"+oldLines[op.oldIndex])
			currentHunk.OldCount++
			currentHunk.Type = DiffDelete
			lastChangeOldLine = op.oldIndex

		case DiffInsert:
			if currentHunk == nil {
				// Start new hunk with leading context
				startOld := op.oldIndex - contextLines
				if startOld < 0 {
					startOld = 0
				}
				startNew := op.newIndex - contextLines
				if startNew < 0 {
					startNew = 0
				}

				currentHunk = &LineDiff{
					Type:     DiffInsert,
					OldStart: startOld,
					NewStart: startNew,
				}

				// Add leading context from old lines
				for i := startOld; i < op.oldIndex && i < len(oldLines); i++ {
					currentHunk.Lines = append(currentHunk.Lines, oldLines[i])
					currentHunk.OldCount++
					currentHunk.NewCount++
				}
			}

			// Add inserted line
			currentHunk.Lines = append(currentHunk.Lines, "+"+newLines[op.newIndex])
			currentHunk.NewCount++
			if currentHunk.Type == DiffEqual {
				currentHunk.Type = DiffInsert
			}
			lastChangeOldLine = op.oldIndex
		}
	}

	// Don't forget the last hunk
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

// UnifiedDiff returns the diff in unified diff format.
func UnifiedDiff(result DiffResult, oldName, newName string) string {
	if !result.HasChanges() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- ")
	sb.WriteString(oldName)
	sb.WriteString("\n")
	sb.WriteString("+++ ")
	sb.WriteString(newName)
	sb.WriteString("\n")

	for _, hunk := range result.Hunks {
		if hunk.Type == DiffEqual {
			continue
		}

		// Write hunk header
		sb.WriteString("@@ -")
		sb.WriteString(strconv.Itoa(hunk.OldStart + 1))
		sb.WriteString(",")
		sb.WriteString(strconv.Itoa(hunk.OldCount))
		sb.WriteString(" +")
		sb.WriteString(strconv.Itoa(hunk.NewStart + 1))
		sb.WriteString(",")
		sb.WriteString(strconv.Itoa(hunk.NewCount))
		sb.WriteString(" @@\n")

		// Write lines
		for _, line := range hunk.Lines {
			if len(line) > 0 && (line[0] == '+' || line[0] == '-') {
				sb.WriteString(line)
			} else {
				sb.WriteString(" ")
				sb.WriteString(line)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
