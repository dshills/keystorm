// Package editor provides handlers for text editing operations.
package editor

import (
	"sort"
	"strings"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for indent operations.
const (
	ActionIndent       = "editor.indent"       // >> - indent line
	ActionOutdent      = "editor.outdent"      // << - outdent line
	ActionAutoIndent   = "editor.autoIndent"   // = - auto-indent selection
	ActionIndentBlock  = "editor.indentBlock"  // >} - indent block
	ActionOutdentBlock = "editor.outdentBlock" // <{ - outdent block
)

// Default indentation settings.
const (
	DefaultTabWidth   = 4
	DefaultUseTabs    = false
	DefaultIndentSize = 4
)

// IndentHandler handles indentation operations.
type IndentHandler struct {
	tabWidth   int
	useTabs    bool
	indentSize int
}

// NewIndentHandler creates a new indent handler with default settings.
func NewIndentHandler() *IndentHandler {
	return &IndentHandler{
		tabWidth:   DefaultTabWidth,
		useTabs:    DefaultUseTabs,
		indentSize: DefaultIndentSize,
	}
}

// NewIndentHandlerWithConfig creates an indent handler with custom settings.
func NewIndentHandlerWithConfig(tabWidth, indentSize int, useTabs bool) *IndentHandler {
	return &IndentHandler{
		tabWidth:   tabWidth,
		useTabs:    useTabs,
		indentSize: indentSize,
	}
}

// Namespace returns the editor namespace.
func (h *IndentHandler) Namespace() string {
	return "editor"
}

// CanHandle returns true if this handler can process the action.
func (h *IndentHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionIndent, ActionOutdent, ActionAutoIndent,
		ActionIndentBlock, ActionOutdentBlock:
		return true
	}
	return false
}

// HandleAction processes an indent action.
func (h *IndentHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	count := ctx.GetCount()

	switch action.Name {
	case ActionIndent:
		return h.indent(ctx, count)
	case ActionOutdent:
		return h.outdent(ctx, count)
	case ActionAutoIndent:
		return h.autoIndent(ctx)
	case ActionIndentBlock:
		return h.indentBlock(ctx, count)
	case ActionOutdentBlock:
		return h.outdentBlock(ctx, count)
	default:
		return handler.Errorf("unknown indent action: %s", action.Name)
	}
}

// indent adds indentation to lines.
func (h *IndentHandler) indent(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors
	lineCount := engine.LineCount()

	if lineCount == 0 {
		return handler.NoOp()
	}

	if ctx.History != nil {
		ctx.History.BeginGroup("indent")
		defer ctx.History.EndGroup()
	}

	// Get the indentation string
	indentStr := h.getIndentString()
	// Repeat for count
	fullIndent := strings.Repeat(indentStr, count)
	indentLen := buffer.ByteOffset(len(fullIndent))

	// Collect unique lines to indent
	lineSet := make(map[uint32]bool)
	selections := cursors.All()
	for _, sel := range selections {
		r := sel.Range()
		startPoint := engine.OffsetToPoint(r.Start)
		endPoint := engine.OffsetToPoint(r.End)

		for line := startPoint.Line; line <= endPoint.Line; line++ {
			lineSet[line] = true
		}
	}

	// Convert to sorted slice (descending to maintain offsets)
	lines := make([]uint32, 0, len(lineSet))
	for line := range lineSet {
		lines = append(lines, line)
	}
	sortLinesDescending(lines)

	// Indent each line
	for _, line := range lines {
		lineStart := engine.LineStartOffset(line)

		// Skip empty lines
		lineText := engine.LineText(line)
		if len(lineText) == 0 {
			continue
		}

		// Insert indentation at line start
		_, err := engine.Insert(lineStart, fullIndent)
		if err != nil {
			return handler.Error(err)
		}
	}

	// Update cursor positions
	cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)
		// Adjust for lines that were indented
		adjustment := buffer.ByteOffset(0)
		for _, line := range lines {
			if point.Line > line {
				adjustment += indentLen
			} else if point.Line == line {
				adjustment += indentLen
				break
			}
		}
		return cursor.Selection{
			Anchor: sel.Anchor + adjustment,
			Head:   sel.Head + adjustment,
		}
	})

	// Collect affected lines
	affectedLines := make([]uint32, 0, len(lines))
	for _, line := range lines {
		affectedLines = append(affectedLines, line)
	}

	return handler.Success().WithRedrawLines(affectedLines...)
}

// outdent removes indentation from lines.
func (h *IndentHandler) outdent(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors
	lineCount := engine.LineCount()

	if lineCount == 0 {
		return handler.NoOp()
	}

	if ctx.History != nil {
		ctx.History.BeginGroup("outdent")
		defer ctx.History.EndGroup()
	}

	// Calculate how much to remove (one indent unit * count)
	removeAmount := h.indentSize * count

	// Collect unique lines to outdent
	lineSet := make(map[uint32]bool)
	selections := cursors.All()
	for _, sel := range selections {
		r := sel.Range()
		startPoint := engine.OffsetToPoint(r.Start)
		endPoint := engine.OffsetToPoint(r.End)

		for line := startPoint.Line; line <= endPoint.Line; line++ {
			lineSet[line] = true
		}
	}

	// Convert to sorted slice (descending)
	lines := make([]uint32, 0, len(lineSet))
	for line := range lineSet {
		lines = append(lines, line)
	}
	sortLinesDescending(lines)

	// Outdent each line
	for _, line := range lines {
		lineStart := engine.LineStartOffset(line)
		lineText := engine.LineText(line)

		// Count leading whitespace
		leadingWS := 0
		for _, r := range lineText {
			if r == ' ' {
				leadingWS++
			} else if r == '\t' {
				leadingWS += h.tabWidth
			} else {
				break
			}
		}

		// Determine how much to actually remove
		toRemove := removeAmount
		if toRemove > leadingWS {
			toRemove = leadingWS
		}
		if toRemove == 0 {
			continue
		}

		// Find the byte offset for the whitespace to remove
		byteCount := 0
		removed := 0
		for i, r := range lineText {
			if removed >= toRemove {
				break
			}
			if r == ' ' {
				removed++
				byteCount = i + 1
			} else if r == '\t' {
				removed += h.tabWidth
				byteCount = i + 1
			} else {
				break
			}
		}

		if byteCount > 0 {
			_, err := engine.Delete(lineStart, lineStart+buffer.ByteOffset(byteCount))
			if err != nil {
				return handler.Error(err)
			}
		}
	}

	// Update cursor positions (adjustments would be complex - just redraw)
	affectedLines := make([]uint32, 0, len(lines))
	for _, line := range lines {
		affectedLines = append(affectedLines, line)
	}

	return handler.Success().WithRedrawLines(affectedLines...)
}

// autoIndent automatically indents lines based on context.
func (h *IndentHandler) autoIndent(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors
	lineCount := engine.LineCount()

	if lineCount == 0 {
		return handler.NoOp()
	}

	if ctx.History != nil {
		ctx.History.BeginGroup("autoIndent")
		defer ctx.History.EndGroup()
	}

	// Collect unique lines to auto-indent
	lineSet := make(map[uint32]bool)
	selections := cursors.All()
	for _, sel := range selections {
		r := sel.Range()
		startPoint := engine.OffsetToPoint(r.Start)
		endPoint := engine.OffsetToPoint(r.End)

		for line := startPoint.Line; line <= endPoint.Line; line++ {
			lineSet[line] = true
		}
	}

	// Convert to sorted slice (ascending for auto-indent to use previous line)
	lines := make([]uint32, 0, len(lineSet))
	for line := range lineSet {
		lines = append(lines, line)
	}
	sortLinesAscending(lines)

	var affectedLines []uint32

	// Auto-indent each line based on previous line
	for _, line := range lines {
		// Get previous line's indentation
		var targetIndent string
		if line > 0 {
			prevLineText := engine.LineText(line - 1)
			targetIndent = getLeadingWhitespace(prevLineText)

			// Increase indent if previous line ends with {, [, or (
			trimmed := strings.TrimRight(prevLineText, " \t")
			if len(trimmed) > 0 {
				lastChar := trimmed[len(trimmed)-1]
				if lastChar == '{' || lastChar == '[' || lastChar == '(' {
					targetIndent += h.getIndentString()
				}
			}
		}

		// Get current line's content without leading whitespace
		lineText := engine.LineText(line)
		contentStart := len(getLeadingWhitespace(lineText))
		content := ""
		if contentStart < len(lineText) {
			content = lineText[contentStart:]
		}

		// Decrease indent if line starts with }, ], or )
		if len(content) > 0 {
			firstChar := content[0]
			if firstChar == '}' || firstChar == ']' || firstChar == ')' {
				targetIndent = removeOneIndent(targetIndent, h.indentSize, h.tabWidth)
			}
		}

		// Replace the line's indentation
		lineStart := engine.LineStartOffset(line)
		oldIndentLen := buffer.ByteOffset(len(getLeadingWhitespace(lineText)))

		if oldIndentLen > 0 {
			_, err := engine.Delete(lineStart, lineStart+oldIndentLen)
			if err != nil {
				return handler.Error(err)
			}
		}

		if len(targetIndent) > 0 {
			_, err := engine.Insert(lineStart, targetIndent)
			if err != nil {
				return handler.Error(err)
			}
		}

		affectedLines = append(affectedLines, line)
	}

	return handler.Success().WithRedrawLines(uniqueLines(affectedLines)...)
}

// indentBlock indents a block of lines (paragraph or selection).
func (h *IndentHandler) indentBlock(ctx *execctx.ExecutionContext, count int) handler.Result {
	// For now, same as indent - could be extended to handle paragraph motions
	return h.indent(ctx, count)
}

// outdentBlock outdents a block of lines.
func (h *IndentHandler) outdentBlock(ctx *execctx.ExecutionContext, count int) handler.Result {
	// For now, same as outdent
	return h.outdent(ctx, count)
}

// getIndentString returns the string to use for one level of indentation.
func (h *IndentHandler) getIndentString() string {
	if h.useTabs {
		return "\t"
	}
	return strings.Repeat(" ", h.indentSize)
}

// getLeadingWhitespace returns the leading whitespace of a string.
func getLeadingWhitespace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

// removeOneIndent removes one level of indentation from a whitespace string.
func removeOneIndent(ws string, indentSize, tabWidth int) string {
	if len(ws) == 0 {
		return ws
	}

	// Check if it starts with a tab
	if ws[0] == '\t' {
		return ws[1:]
	}

	// Remove indentSize spaces
	spaces := 0
	cutoff := 0
	for i, r := range ws {
		if r == ' ' {
			spaces++
			if spaces >= indentSize {
				cutoff = i + 1
				break
			}
		} else if r == '\t' {
			cutoff = i + 1
			break
		}
	}

	if cutoff > 0 && cutoff <= len(ws) {
		return ws[cutoff:]
	}
	return ""
}

// sortLinesDescending sorts line numbers in descending order.
func sortLinesDescending(lines []uint32) {
	sort.Slice(lines, func(i, j int) bool {
		return lines[i] > lines[j]
	})
}

// sortLinesAscending sorts line numbers in ascending order.
func sortLinesAscending(lines []uint32) {
	sort.Slice(lines, func(i, j int) bool {
		return lines[i] < lines[j]
	})
}
