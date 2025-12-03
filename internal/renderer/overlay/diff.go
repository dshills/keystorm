package overlay

import (
	"github.com/dshills/keystorm/internal/renderer/core"
)

// DiffOperation represents the type of diff operation.
type DiffOperation uint8

const (
	DiffOpEqual DiffOperation = iota
	DiffOpInsert
	DiffOpDelete
	DiffOpReplace
)

// String returns the string representation of the diff operation.
func (op DiffOperation) String() string {
	switch op {
	case DiffOpEqual:
		return "equal"
	case DiffOpInsert:
		return "insert"
	case DiffOpDelete:
		return "delete"
	case DiffOpReplace:
		return "replace"
	default:
		return "unknown"
	}
}

// DiffHunk represents a single diff change.
type DiffHunk struct {
	// Operation is the type of change.
	Operation DiffOperation

	// OldRange is the range in the original content.
	OldRange Range

	// NewRange is the range in the new content.
	NewRange Range

	// OldLines contains the original lines (for delete/replace).
	OldLines []string

	// NewLines contains the new lines (for insert/replace).
	NewLines []string
}

// DiffPreview represents an inline diff preview overlay.
// It shows proposed changes from AI suggestions or refactoring operations.
type DiffPreview struct {
	*BaseOverlay

	// hunks contains the diff changes.
	hunks []DiffHunk

	// addStyle is the style for additions.
	addStyle core.Style

	// deleteStyle is the style for deletions.
	deleteStyle core.Style

	// modifyStyle is the style for modifications.
	modifyStyle core.Style

	// showLineNumbers shows line numbers in the diff.
	showLineNumbers bool

	// collapsed indicates if the diff is collapsed to a summary.
	collapsed bool

	// accepted tracks if this diff has been accepted.
	accepted bool

	// rejected tracks if this diff has been rejected.
	rejected bool
}

// NewDiffPreview creates a new diff preview overlay.
func NewDiffPreview(id string, hunks []DiffHunk, config Config) *DiffPreview {
	// Calculate the overall range from hunks
	rng := calculateDiffRange(hunks)

	return &DiffPreview{
		BaseOverlay: NewBaseOverlay(id, TypeDiffAdd, PriorityHigh, rng),
		hunks:       hunks,
		addStyle:    config.DiffAddStyle,
		deleteStyle: config.DiffDeleteStyle,
		modifyStyle: config.DiffModifyStyle,
	}
}

// NewDiffPreviewSimple creates a diff preview from old and new text.
func NewDiffPreviewSimple(id string, startLine uint32, oldLines, newLines []string, config Config) *DiffPreview {
	hunks := computeSimpleDiff(startLine, oldLines, newLines)
	return NewDiffPreview(id, hunks, config)
}

// Hunks returns the diff hunks.
func (d *DiffPreview) Hunks() []DiffHunk {
	return d.hunks
}

// HunkCount returns the number of diff hunks.
func (d *DiffPreview) HunkCount() int {
	return len(d.hunks)
}

// AdditionCount returns the number of added lines.
func (d *DiffPreview) AdditionCount() int {
	count := 0
	for _, h := range d.hunks {
		if h.Operation == DiffOpInsert || h.Operation == DiffOpReplace {
			count += len(h.NewLines)
		}
	}
	return count
}

// DeletionCount returns the number of deleted lines.
func (d *DiffPreview) DeletionCount() int {
	count := 0
	for _, h := range d.hunks {
		if h.Operation == DiffOpDelete || h.Operation == DiffOpReplace {
			count += len(h.OldLines)
		}
	}
	return count
}

// SetCollapsed sets whether the diff is collapsed.
func (d *DiffPreview) SetCollapsed(collapsed bool) {
	d.collapsed = collapsed
}

// IsCollapsed returns true if the diff is collapsed.
func (d *DiffPreview) IsCollapsed() bool {
	return d.collapsed
}

// Accept accepts the diff and applies the changes.
func (d *DiffPreview) Accept() {
	d.accepted = true
	d.visible = false
}

// Reject rejects the diff.
func (d *DiffPreview) Reject() {
	d.rejected = true
	d.visible = false
}

// IsAccepted returns true if the diff was accepted.
func (d *DiffPreview) IsAccepted() bool {
	return d.accepted
}

// IsRejected returns true if the diff was rejected.
func (d *DiffPreview) IsRejected() bool {
	return d.rejected
}

// AcceptHunk accepts a single hunk by index.
func (d *DiffPreview) AcceptHunk(index int) bool {
	if index < 0 || index >= len(d.hunks) {
		return false
	}
	// Mark hunk as accepted by removing it
	d.hunks = append(d.hunks[:index], d.hunks[index+1:]...)
	if len(d.hunks) == 0 {
		d.Accept()
	}
	return true
}

// RejectHunk rejects a single hunk by index.
func (d *DiffPreview) RejectHunk(index int) bool {
	if index < 0 || index >= len(d.hunks) {
		return false
	}
	// Remove the hunk
	d.hunks = append(d.hunks[:index], d.hunks[index+1:]...)
	if len(d.hunks) == 0 {
		d.Reject()
	}
	return true
}

// SpansForLine returns the overlay spans for a specific line.
func (d *DiffPreview) SpansForLine(line uint32) []Span {
	if !d.visible || d.accepted || d.rejected {
		return nil
	}

	if d.collapsed {
		return d.collapsedSpansForLine(line)
	}

	var spans []Span

	for _, hunk := range d.hunks {
		hunkSpans := d.spansForHunk(line, hunk)
		spans = append(spans, hunkSpans...)
	}

	return spans
}

// spansForHunk returns spans for a specific hunk on the given line.
func (d *DiffPreview) spansForHunk(line uint32, hunk DiffHunk) []Span {
	var spans []Span

	switch hunk.Operation {
	case DiffOpInsert:
		// Insertions are shown after the line before them
		if line == hunk.OldRange.Start.Line {
			for i, newLine := range hunk.NewLines {
				spans = append(spans, Span{
					StartCol:       0,
					Text:           "+ " + newLine,
					Style:          d.addStyle,
					ReplaceContent: false,
					AfterContent:   i == 0,
				})
			}
		}

	case DiffOpDelete:
		// Deletions are shown with strikethrough
		lineIdx := int(line - hunk.OldRange.Start.Line)
		if lineIdx >= 0 && lineIdx < len(hunk.OldLines) {
			spans = append(spans, Span{
				StartCol:       0,
				EndCol:         uint32(len(hunk.OldLines[lineIdx])),
				Style:          d.deleteStyle,
				ReplaceContent: true,
			})
		}

	case DiffOpReplace:
		lineIdx := int(line - hunk.OldRange.Start.Line)
		if lineIdx >= 0 && lineIdx < len(hunk.OldLines) {
			// Show old line with strikethrough
			spans = append(spans, Span{
				StartCol:       0,
				EndCol:         uint32(len(hunk.OldLines[lineIdx])),
				Style:          d.deleteStyle,
				ReplaceContent: true,
			})
		}
		// Show new lines after the old lines
		if line == hunk.OldRange.End.Line-1 || (hunk.OldRange.Start.Line == hunk.OldRange.End.Line && line == hunk.OldRange.Start.Line) {
			for _, newLine := range hunk.NewLines {
				spans = append(spans, Span{
					StartCol:     0,
					Text:         "+ " + newLine,
					Style:        d.addStyle,
					AfterContent: true,
				})
			}
		}
	}

	return spans
}

// collapsedSpansForLine returns spans for the collapsed view.
func (d *DiffPreview) collapsedSpansForLine(line uint32) []Span {
	// Only show summary on the first line of the range
	if line != d.rng.Start.Line {
		return nil
	}

	adds := d.AdditionCount()
	dels := d.DeletionCount()

	summary := formatDiffSummary(adds, dels)

	return []Span{
		{
			StartCol:     0,
			Text:         summary,
			Style:        d.modifyStyle,
			AfterContent: true,
		},
	}
}

// calculateDiffRange calculates the overall range from hunks.
func calculateDiffRange(hunks []DiffHunk) Range {
	if len(hunks) == 0 {
		return Range{}
	}

	rng := hunks[0].OldRange
	for _, h := range hunks[1:] {
		if h.OldRange.Start.Line < rng.Start.Line {
			rng.Start = h.OldRange.Start
		}
		if h.OldRange.End.Line > rng.End.Line {
			rng.End = h.OldRange.End
		}
	}

	return rng
}

// computeSimpleDiff computes a simple line-by-line diff.
func computeSimpleDiff(startLine uint32, oldLines, newLines []string) []DiffHunk {
	var hunks []DiffHunk

	// Simple LCS-based diff
	oldLen := len(oldLines)
	newLen := len(newLines)

	// Edge cases
	if oldLen == 0 && newLen == 0 {
		return hunks
	}

	if oldLen == 0 {
		// All insertions
		hunks = append(hunks, DiffHunk{
			Operation: DiffOpInsert,
			OldRange: Range{
				Start: Position{Line: startLine, Col: 0},
				End:   Position{Line: startLine, Col: 0},
			},
			NewRange: Range{
				Start: Position{Line: startLine, Col: 0},
				End:   Position{Line: startLine + uint32(newLen), Col: 0},
			},
			NewLines: newLines,
		})
		return hunks
	}

	if newLen == 0 {
		// All deletions
		hunks = append(hunks, DiffHunk{
			Operation: DiffOpDelete,
			OldRange: Range{
				Start: Position{Line: startLine, Col: 0},
				End:   Position{Line: startLine + uint32(oldLen), Col: 0},
			},
			NewRange: Range{
				Start: Position{Line: startLine, Col: 0},
				End:   Position{Line: startLine, Col: 0},
			},
			OldLines: oldLines,
		})
		return hunks
	}

	// Simple line-by-line comparison
	i, j := 0, 0
	for i < oldLen || j < newLen {
		if i < oldLen && j < newLen && oldLines[i] == newLines[j] {
			// Equal lines, skip
			i++
			j++
			continue
		}

		// Find the extent of the change
		oldStart := i
		newStart := j

		// Look for next matching line
		for i < oldLen {
			found := false
			for k := j; k < newLen; k++ {
				if oldLines[i] == newLines[k] {
					found = true
					break
				}
			}
			if found {
				break
			}
			i++
		}

		for j < newLen {
			found := false
			for k := oldStart; k < i; k++ {
				if newLines[j] == oldLines[k] {
					found = true
					break
				}
			}
			if found {
				break
			}
			j++
		}

		// Create hunk for this change
		if oldStart < i || newStart < j {
			op := DiffOpReplace
			if oldStart == i {
				op = DiffOpInsert
			} else if newStart == j {
				op = DiffOpDelete
			}

			hunks = append(hunks, DiffHunk{
				Operation: op,
				OldRange: Range{
					Start: Position{Line: startLine + uint32(oldStart), Col: 0},
					End:   Position{Line: startLine + uint32(i), Col: 0},
				},
				NewRange: Range{
					Start: Position{Line: startLine + uint32(newStart), Col: 0},
					End:   Position{Line: startLine + uint32(j), Col: 0},
				},
				OldLines: oldLines[oldStart:i],
				NewLines: newLines[newStart:j],
			})
		}
	}

	return hunks
}

// formatDiffSummary formats a diff summary string.
func formatDiffSummary(adds, dels int) string {
	if adds == 0 && dels == 0 {
		return "[no changes]"
	}

	result := "["
	if adds > 0 {
		result += "+" + itoa(adds)
	}
	if dels > 0 {
		if adds > 0 {
			result += " "
		}
		result += "-" + itoa(dels)
	}
	result += " lines]"

	return result
}

// itoa converts an int to a string without fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var buf [20]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

// InlineDiff represents a character-level inline diff within a single line.
type InlineDiff struct {
	*BaseOverlay

	// line is the line number.
	line uint32

	// segments contains the inline diff segments.
	segments []InlineDiffSegment

	// addStyle is the style for additions.
	addStyle core.Style

	// deleteStyle is the style for deletions.
	deleteStyle core.Style
}

// InlineDiffSegment represents a segment of an inline diff.
type InlineDiffSegment struct {
	// Operation is the diff operation.
	Operation DiffOperation

	// StartCol is the starting column.
	StartCol uint32

	// EndCol is the ending column.
	EndCol uint32

	// Text is the text content (for insertions).
	Text string
}

// NewInlineDiff creates a new inline diff overlay.
func NewInlineDiff(id string, line uint32, segments []InlineDiffSegment, config Config) *InlineDiff {
	// Calculate range
	var endCol uint32
	for _, seg := range segments {
		if seg.EndCol > endCol {
			endCol = seg.EndCol
		}
	}

	rng := Range{
		Start: Position{Line: line, Col: 0},
		End:   Position{Line: line, Col: endCol},
	}

	return &InlineDiff{
		BaseOverlay: NewBaseOverlay(id, TypeDiffModify, PriorityHigh, rng),
		line:        line,
		segments:    segments,
		addStyle:    config.DiffAddStyle,
		deleteStyle: config.DiffDeleteStyle,
	}
}

// SpansForLine returns the overlay spans for a specific line.
func (d *InlineDiff) SpansForLine(line uint32) []Span {
	if !d.visible || line != d.line {
		return nil
	}

	spans := make([]Span, 0, len(d.segments))

	for _, seg := range d.segments {
		var style core.Style
		switch seg.Operation {
		case DiffOpInsert:
			style = d.addStyle
		case DiffOpDelete:
			style = d.deleteStyle
		default:
			continue
		}

		span := Span{
			StartCol: seg.StartCol,
			EndCol:   seg.EndCol,
			Text:     seg.Text,
			Style:    style,
		}

		if seg.Operation == DiffOpInsert {
			span.AfterContent = true
		} else {
			span.ReplaceContent = true
		}

		spans = append(spans, span)
	}

	return spans
}
