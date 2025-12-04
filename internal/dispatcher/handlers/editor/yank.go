// Package editor provides handlers for text editing operations.
package editor

import (
	"sort"
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for yank/paste operations.
const (
	ActionYankSelection = "editor.yankSelection" // y - yank selection
	ActionYankLine      = "editor.yankLine"      // yy - yank entire line
	ActionYankToEnd     = "editor.yankToEnd"     // Y - yank to end of line
	ActionYankWord      = "editor.yankWord"      // yw - yank word
	ActionPasteAfter    = "editor.pasteAfter"    // p - paste after cursor
	ActionPasteBefore   = "editor.pasteBefore"   // P - paste before cursor
)

// YankHandler handles yank (copy) and paste operations.
type YankHandler struct{}

// NewYankHandler creates a new yank handler.
func NewYankHandler() *YankHandler {
	return &YankHandler{}
}

// Namespace returns the editor namespace.
func (h *YankHandler) Namespace() string {
	return "editor"
}

// CanHandle returns true if this handler can process the action.
func (h *YankHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionYankSelection, ActionYankLine, ActionYankToEnd,
		ActionYankWord, ActionPasteAfter, ActionPasteBefore:
		return true
	}
	return false
}

// HandleAction processes a yank/paste action.
func (h *YankHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Yank operations only need engine and cursors
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	count := ctx.GetCount()

	switch action.Name {
	case ActionYankSelection:
		return h.yankSelection(ctx)
	case ActionYankLine:
		return h.yankLine(ctx, count)
	case ActionYankToEnd:
		return h.yankToEnd(ctx)
	case ActionYankWord:
		return h.yankWord(ctx, count)
	case ActionPasteAfter:
		return h.pasteAfter(ctx, action.Args.Text, count)
	case ActionPasteBefore:
		return h.pasteBefore(ctx, action.Args.Text, count)
	default:
		return handler.Errorf("unknown yank action: %s", action.Name)
	}
}

// yankSelection yanks the selected text.
// For multi-cursor, accumulates text from all selections in buffer order.
func (h *YankHandler) yankSelection(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	// Sort selections by position for consistent ordering in register
	selections := cursors.All()
	sortSelectionsForward(selections)

	var yankedParts []string
	for _, sel := range selections {
		if sel.IsEmpty() {
			continue
		}

		r := sel.Range()
		yankedParts = append(yankedParts, engine.TextRange(r.Start, r.End))
	}

	if len(yankedParts) == 0 {
		return handler.NoOp()
	}

	yankedText := joinStringsYank(yankedParts)
	return handler.Success().WithRegisterContent(yankedText)
}

// yankLine yanks count lines including the current line.
func (h *YankHandler) yankLine(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors
	lineCount := engine.LineCount()

	if lineCount == 0 {
		return handler.NoOp()
	}

	// Sort selections for consistent ordering
	selections := cursors.All()
	sortSelectionsForward(selections)

	var yankedParts []string
	for _, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		startLine := point.Line
		endLine := startLine + uint32(count)
		if endLine > lineCount {
			endLine = lineCount
		}

		// Get line start and end offsets
		start := engine.LineStartOffset(startLine)
		var end buffer.ByteOffset
		if endLine >= lineCount {
			end = engine.Len()
		} else {
			end = engine.LineStartOffset(endLine)
		}

		yankedParts = append(yankedParts, engine.TextRange(start, end))
	}

	if len(yankedParts) == 0 {
		return handler.NoOp()
	}

	yankedText := joinStringsYank(yankedParts)
	return handler.Success().
		WithRegisterContent(yankedText).
		WithLinewise(true)
}

// yankToEnd yanks from cursor to end of line.
func (h *YankHandler) yankToEnd(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	// Sort selections for consistent ordering
	selections := cursors.All()
	sortSelectionsForward(selections)

	var yankedParts []string
	for _, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		start := sel.Head
		end := engine.LineEndOffset(point.Line)

		if start >= end {
			continue
		}

		yankedParts = append(yankedParts, engine.TextRange(start, end))
	}

	if len(yankedParts) == 0 {
		return handler.NoOp()
	}

	yankedText := joinStringsYank(yankedParts)
	return handler.Success().WithRegisterContent(yankedText)
}

// yankWord yanks count words forward.
func (h *YankHandler) yankWord(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	text := engine.Text()
	textLen := buffer.ByteOffset(len(text))

	// Sort selections for consistent ordering
	selections := cursors.All()
	sortSelectionsForward(selections)

	var yankedParts []string
	for _, sel := range selections {
		start := sel.Head
		end := start

		// Find end of count words using UTF-8 safe function
		for i := 0; i < count && end < textLen; i++ {
			end = findNextWordStartYank(text, end, textLen)
		}

		if start == end {
			continue
		}

		yankedParts = append(yankedParts, engine.TextRange(start, end))
	}

	if len(yankedParts) == 0 {
		return handler.NoOp()
	}

	yankedText := joinStringsYank(yankedParts)
	return handler.Success().WithRegisterContent(yankedText)
}

// pasteAfter pastes text after cursor position.
func (h *YankHandler) pasteAfter(ctx *execctx.ExecutionContext, text string, count int) handler.Result {
	if text == "" {
		return handler.NoOp()
	}

	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("pasteAfter")
		defer ctx.History.EndGroup()
	}

	// Build repeated text once (base paste content)
	basePasteText := ""
	for i := 0; i < count; i++ {
		basePasteText += text
	}

	selections := cursors.All()
	sortSelectionsReverseYank(selections)

	var affectedLines []uint32
	isLinewise := len(text) > 0 && text[len(text)-1] == '\n'

	for i, sel := range selections {
		// Get fresh engine state for each iteration
		engineText := engine.Text()
		engineLen := buffer.ByteOffset(len(engineText))

		// Create a local copy of paste text for this iteration
		pasteText := basePasteText

		// Calculate insert position
		insertOffset := sel.Head
		if !isLinewise && insertOffset < engineLen {
			// For characterwise paste, insert after current character
			_, size := utf8.DecodeRuneInString(engineText[insertOffset:])
			if size > 0 {
				insertOffset += buffer.ByteOffset(size)
			}
		}

		if isLinewise {
			// For linewise paste, insert at start of next line
			point := engine.OffsetToPoint(sel.Head)
			currentLineCount := engine.LineCount()
			if point.Line+1 < currentLineCount {
				insertOffset = engine.LineStartOffset(point.Line + 1)
			} else {
				// Insert at end of buffer
				insertOffset = engine.Len()
				// Add newline before if buffer doesn't end with one
				currentEngineText := engine.Text()
				currentEngineLen := buffer.ByteOffset(len(currentEngineText))
				if currentEngineLen > 0 && currentEngineText[currentEngineLen-1] != '\n' {
					pasteText = "\n" + pasteText
				}
			}
		}

		// Insert the text
		result, err := engine.Insert(insertOffset, pasteText)
		if err != nil {
			return handler.Error(err)
		}

		// Update cursor using result from insert
		newOffset := result.NewRange.End
		if isLinewise {
			// For linewise, position at start of first pasted line
			newOffset = result.NewRange.Start
		}
		selections[i] = sel.MoveTo(newOffset)

		// Track affected lines
		startPoint := engine.OffsetToPoint(result.NewRange.Start)
		endPoint := engine.OffsetToPoint(result.NewRange.End)
		for line := startPoint.Line; line <= endPoint.Line; line++ {
			affectedLines = append(affectedLines, line)
		}
	}

	// Reverse selections to restore original order before setting
	reverseSelectionsYank(selections)
	cursors.SetAll(selections)

	return handler.Success().WithRedrawLines(uniqueLinesYank(affectedLines)...)
}

// pasteBefore pastes text before cursor position.
func (h *YankHandler) pasteBefore(ctx *execctx.ExecutionContext, text string, count int) handler.Result {
	if text == "" {
		return handler.NoOp()
	}

	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("pasteBefore")
		defer ctx.History.EndGroup()
	}

	// Build repeated text once
	pasteText := ""
	for i := 0; i < count; i++ {
		pasteText += text
	}

	selections := cursors.All()
	sortSelectionsReverseYank(selections)

	var affectedLines []uint32
	isLinewise := len(text) > 0 && text[len(text)-1] == '\n'

	for i, sel := range selections {
		insertOffset := sel.Head

		if isLinewise {
			// For linewise paste, insert at start of current line
			point := engine.OffsetToPoint(sel.Head)
			insertOffset = engine.LineStartOffset(point.Line)
		}

		// Insert the text
		result, err := engine.Insert(insertOffset, pasteText)
		if err != nil {
			return handler.Error(err)
		}

		// Update cursor position using result from insert
		var newOffset buffer.ByteOffset
		if isLinewise {
			// Position at start of first pasted line
			newOffset = result.NewRange.Start
		} else {
			// Position at end of pasted text
			newOffset = result.NewRange.End
		}
		selections[i] = cursor.NewCursorSelection(newOffset)

		// Track affected lines
		startPoint := engine.OffsetToPoint(result.NewRange.Start)
		endPoint := engine.OffsetToPoint(result.NewRange.End)
		for line := startPoint.Line; line <= endPoint.Line; line++ {
			affectedLines = append(affectedLines, line)
		}
	}

	// Reverse selections to restore original order before setting
	reverseSelectionsYank(selections)
	cursors.SetAll(selections)

	return handler.Success().WithRedrawLines(uniqueLinesYank(affectedLines)...)
}

// sortSelectionsForward sorts selections by position in ascending order.
func sortSelectionsForward(selections []cursor.Selection) {
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].Head < selections[j].Head
	})
}

// sortSelectionsReverseYank sorts selections by position in descending order.
func sortSelectionsReverseYank(selections []cursor.Selection) {
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].Head > selections[j].Head
	})
}

// reverseSelectionsYank reverses the order of selections.
func reverseSelectionsYank(selections []cursor.Selection) {
	for i, j := 0, len(selections)-1; i < j; i, j = i+1, j-1 {
		selections[i], selections[j] = selections[j], selections[i]
	}
}

// joinStringsYank joins strings without separator.
func joinStringsYank(parts []string) string {
	result := ""
	for _, s := range parts {
		result += s
	}
	return result
}

// uniqueLinesYank returns unique line numbers from a slice.
func uniqueLinesYank(lines []uint32) []uint32 {
	if len(lines) == 0 {
		return nil
	}

	seen := make(map[uint32]bool)
	result := make([]uint32, 0, len(lines))

	for _, line := range lines {
		if !seen[line] {
			seen[line] = true
			result = append(result, line)
		}
	}

	return result
}

// findNextWordStartYank finds the start of the next word using proper UTF-8 iteration.
func findNextWordStartYank(text string, offset, maxOffset buffer.ByteOffset) buffer.ByteOffset {
	textLen := buffer.ByteOffset(len(text))
	if offset >= textLen || offset >= maxOffset {
		return min(textLen, maxOffset)
	}

	// Use for-range to properly iterate over runes
	inWord := false
	foundNonWord := false

	for i, r := range text[offset:] {
		pos := offset + buffer.ByteOffset(i)
		if pos >= maxOffset {
			return maxOffset
		}

		if isWordCharYank(r) {
			if foundNonWord {
				// Found start of next word
				return pos
			}
			inWord = true
		} else if isWhitespaceYank(r) {
			if inWord {
				// Exited word, now in whitespace
				foundNonWord = true
			}
		} else {
			// Punctuation or other non-word char
			if inWord {
				foundNonWord = true
			} else if foundNonWord {
				// Found non-word, non-whitespace after whitespace
				return pos
			}
		}
	}

	return min(textLen, maxOffset)
}

// isWordCharYank returns true if r is a word character.
func isWordCharYank(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

// isWhitespaceYank returns true if r is whitespace.
func isWhitespaceYank(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
