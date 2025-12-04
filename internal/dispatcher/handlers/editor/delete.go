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

// Action names for delete operations.
const (
	ActionDeleteChar      = "editor.deleteChar"      // x - delete char under cursor
	ActionDeleteCharBack  = "editor.deleteCharBack"  // X - delete char before cursor
	ActionDeleteLine      = "editor.deleteLine"      // dd - delete entire line
	ActionDeleteToEnd     = "editor.deleteToEnd"     // D - delete to end of line
	ActionDeleteSelection = "editor.deleteSelection" // delete selected text
	ActionDeleteWord      = "editor.deleteWord"      // dw - delete word
	ActionDeleteWordBack  = "editor.deleteWordBack"  // db - delete word backward
)

// DeleteHandler handles text deletion operations.
type DeleteHandler struct{}

// NewDeleteHandler creates a new delete handler.
func NewDeleteHandler() *DeleteHandler {
	return &DeleteHandler{}
}

// Namespace returns the editor namespace.
func (h *DeleteHandler) Namespace() string {
	return "editor"
}

// CanHandle returns true if this handler can process the action.
func (h *DeleteHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionDeleteChar, ActionDeleteCharBack, ActionDeleteLine,
		ActionDeleteToEnd, ActionDeleteSelection, ActionDeleteWord,
		ActionDeleteWordBack:
		return true
	}
	return false
}

// HandleAction processes a delete action.
func (h *DeleteHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	count := ctx.GetCount()

	switch action.Name {
	case ActionDeleteChar:
		return h.deleteChar(ctx, count)
	case ActionDeleteCharBack:
		return h.deleteCharBack(ctx, count)
	case ActionDeleteLine:
		return h.deleteLine(ctx, count)
	case ActionDeleteToEnd:
		return h.deleteToEnd(ctx)
	case ActionDeleteSelection:
		return h.deleteSelection(ctx)
	case ActionDeleteWord:
		return h.deleteWord(ctx, count)
	case ActionDeleteWordBack:
		return h.deleteWordBack(ctx, count)
	default:
		return handler.Errorf("unknown delete action: %s", action.Name)
	}
}

// deleteChar deletes count characters at cursor position (like 'x' in Vim).
func (h *DeleteHandler) deleteChar(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("deleteChar")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverse(selections)

	var affectedLines []uint32
	var deletedParts []string

	for _, sel := range selections {
		start := sel.Head
		end := start

		// Get fresh text state for each iteration
		text := engine.Text()
		textLen := buffer.ByteOffset(len(text))

		for i := 0; i < count && end < textLen; i++ {
			// Move forward by one rune using proper UTF-8 decoding
			end = nextRuneEndUTF8(text, end, textLen)
		}

		if start == end {
			continue
		}

		// Accumulate deleted text (will be in reverse buffer order due to iteration order)
		deletedParts = append(deletedParts, engine.TextRange(start, end))

		// Delete the text
		_, err := engine.Delete(start, end)
		if err != nil {
			return handler.Error(err)
		}

		point := engine.OffsetToPoint(start)
		affectedLines = append(affectedLines, point.Line)
	}

	// Reverse to get buffer order (since we processed in reverse)
	reverseStrings(deletedParts)
	deletedText := joinStrings(deletedParts)

	return handler.Success().
		WithRedrawLines(uniqueLines(affectedLines)...).
		WithRegisterContent(deletedText)
}

// deleteCharBack deletes count characters before cursor (like 'X' in Vim).
func (h *DeleteHandler) deleteCharBack(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("deleteCharBack")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverse(selections)

	var affectedLines []uint32
	var deletedParts []string

	for i, sel := range selections {
		end := sel.Head
		start := end

		// Get fresh text for proper UTF-8 handling
		text := engine.Text()

		for j := 0; j < count && start > 0; j++ {
			start = prevRuneStartUTF8(text, start)
		}

		if start == end {
			continue
		}

		// Accumulate deleted text
		deletedParts = append(deletedParts, engine.TextRange(start, end))

		// Delete the text
		_, err := engine.Delete(start, end)
		if err != nil {
			return handler.Error(err)
		}

		// Update cursor position
		selections[i] = sel.MoveTo(start)

		point := engine.OffsetToPoint(start)
		affectedLines = append(affectedLines, point.Line)
	}

	cursors.SetAll(selections)

	// Reverse to get buffer order
	reverseStrings(deletedParts)
	deletedText := joinStrings(deletedParts)

	return handler.Success().
		WithRedrawLines(uniqueLines(affectedLines)...).
		WithRegisterContent(deletedText)
}

// deleteLine deletes count lines including the current line (like 'dd' in Vim).
func (h *DeleteHandler) deleteLine(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors
	lineCount := engine.LineCount()

	if lineCount == 0 {
		return handler.NoOp()
	}

	if ctx.History != nil {
		ctx.History.BeginGroup("deleteLine")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverse(selections)

	var deletedParts []string

	for i, sel := range selections {
		// Get fresh line count after each deletion
		currentLineCount := engine.LineCount()
		if currentLineCount == 0 {
			break
		}

		point := engine.OffsetToPoint(sel.Head)
		startLine := point.Line
		endLine := startLine + uint32(count)
		if endLine > currentLineCount {
			endLine = currentLineCount
		}

		// Get line start and end offsets
		start := engine.LineStartOffset(startLine)
		var end buffer.ByteOffset
		if endLine >= currentLineCount {
			end = engine.Len()
		} else {
			end = engine.LineStartOffset(endLine)
		}

		// Accumulate deleted text
		deletedParts = append(deletedParts, engine.TextRange(start, end))

		// Delete the lines
		_, err := engine.Delete(start, end)
		if err != nil {
			return handler.Error(err)
		}

		// Position cursor at start of line (or last line if at end)
		newLineCount := engine.LineCount()
		if newLineCount == 0 {
			selections[i] = cursor.NewCursorSelection(0)
		} else {
			targetLine := startLine
			if targetLine >= newLineCount {
				targetLine = newLineCount - 1
			}
			newOffset := engine.LineStartOffset(targetLine)
			selections[i] = sel.MoveTo(newOffset)
		}
	}

	cursors.SetAll(selections)

	// Reverse to get buffer order
	reverseStrings(deletedParts)
	deletedText := joinStrings(deletedParts)

	return handler.Success().
		WithRedraw().
		WithRegisterContent(deletedText)
}

// deleteToEnd deletes from cursor to end of line (like 'D' in Vim).
func (h *DeleteHandler) deleteToEnd(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("deleteToEnd")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverse(selections)

	var affectedLines []uint32
	var deletedParts []string

	for _, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		start := sel.Head
		end := engine.LineEndOffset(point.Line)

		if start >= end {
			continue
		}

		// Accumulate deleted text
		deletedParts = append(deletedParts, engine.TextRange(start, end))

		// Delete to end of line
		_, err := engine.Delete(start, end)
		if err != nil {
			return handler.Error(err)
		}

		affectedLines = append(affectedLines, point.Line)
	}

	// Reverse to get buffer order
	reverseStrings(deletedParts)
	deletedText := joinStrings(deletedParts)

	return handler.Success().
		WithRedrawLines(uniqueLines(affectedLines)...).
		WithRegisterContent(deletedText)
}

// deleteSelection deletes the selected text.
func (h *DeleteHandler) deleteSelection(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("deleteSelection")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverse(selections)

	var deletedParts []string

	for i, sel := range selections {
		if sel.IsEmpty() {
			continue
		}

		r := sel.Range()

		// Accumulate deleted text
		deletedParts = append(deletedParts, engine.TextRange(r.Start, r.End))

		// Delete the selection
		_, err := engine.Delete(r.Start, r.End)
		if err != nil {
			return handler.Error(err)
		}

		// Collapse cursor to start of selection
		selections[i] = sel.MoveTo(r.Start)
	}

	cursors.SetAll(selections)

	// Reverse to get buffer order
	reverseStrings(deletedParts)
	deletedText := joinStrings(deletedParts)

	return handler.Success().
		WithRedraw().
		WithRegisterContent(deletedText)
}

// deleteWord deletes count words forward (like 'dw' in Vim).
func (h *DeleteHandler) deleteWord(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("deleteWord")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverse(selections)

	var affectedLines []uint32
	var deletedParts []string

	for _, sel := range selections {
		start := sel.Head
		end := start

		// Get fresh text for this iteration
		text := engine.Text()
		textLen := buffer.ByteOffset(len(text))

		// Find end of count words
		for i := 0; i < count && end < textLen; i++ {
			end = findNextWordStartUTF8(text, end, textLen)
		}

		if start == end {
			continue
		}

		// Accumulate deleted text
		deletedParts = append(deletedParts, engine.TextRange(start, end))

		// Delete the words
		_, err := engine.Delete(start, end)
		if err != nil {
			return handler.Error(err)
		}

		startPoint := engine.OffsetToPoint(start)
		affectedLines = append(affectedLines, startPoint.Line)
	}

	// Reverse to get buffer order
	reverseStrings(deletedParts)
	deletedText := joinStrings(deletedParts)

	return handler.Success().
		WithRedrawLines(uniqueLines(affectedLines)...).
		WithRegisterContent(deletedText)
}

// deleteWordBack deletes count words backward (like 'db' in Vim).
func (h *DeleteHandler) deleteWordBack(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("deleteWordBack")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverse(selections)

	var affectedLines []uint32
	var deletedParts []string

	for i, sel := range selections {
		end := sel.Head
		start := end

		// Get fresh text for this iteration
		text := engine.Text()

		// Find start of count words backward
		for j := 0; j < count && start > 0; j++ {
			start = findPrevWordStartUTF8(text, start)
		}

		if start == end {
			continue
		}

		// Accumulate deleted text
		deletedParts = append(deletedParts, engine.TextRange(start, end))

		// Delete the words
		_, err := engine.Delete(start, end)
		if err != nil {
			return handler.Error(err)
		}

		// Update cursor position
		selections[i] = sel.MoveTo(start)

		startPoint := engine.OffsetToPoint(start)
		affectedLines = append(affectedLines, startPoint.Line)
	}

	cursors.SetAll(selections)

	// Reverse to get buffer order
	reverseStrings(deletedParts)
	deletedText := joinStrings(deletedParts)

	return handler.Success().
		WithRedrawLines(uniqueLines(affectedLines)...).
		WithRegisterContent(deletedText)
}

// nextRuneEndUTF8 returns the offset after the next rune using proper UTF-8 decoding.
func nextRuneEndUTF8(text string, offset, maxOffset buffer.ByteOffset) buffer.ByteOffset {
	if offset >= maxOffset {
		return maxOffset
	}

	textLen := buffer.ByteOffset(len(text))
	if offset >= textLen {
		return textLen
	}

	// Decode the rune at offset to get its size
	_, size := utf8.DecodeRuneInString(text[offset:])
	if size == 0 {
		return offset
	}

	newOffset := offset + buffer.ByteOffset(size)
	if newOffset > maxOffset {
		return maxOffset
	}
	return newOffset
}

// prevRuneStartUTF8 finds the start of the previous rune before offset.
func prevRuneStartUTF8(text string, offset buffer.ByteOffset) buffer.ByteOffset {
	if offset <= 0 {
		return 0
	}

	textLen := buffer.ByteOffset(len(text))
	if offset > textLen {
		offset = textLen
	}

	// DecodeLastRuneInString gives us the rune and its size
	_, size := utf8.DecodeLastRuneInString(text[:offset])
	if size == 0 {
		return 0
	}

	return offset - buffer.ByteOffset(size)
}

// findNextWordStartUTF8 finds the start of the next word using proper UTF-8 iteration.
func findNextWordStartUTF8(text string, offset, maxOffset buffer.ByteOffset) buffer.ByteOffset {
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

		if isWordChar(r) {
			if foundNonWord {
				// Found start of next word
				return pos
			}
			inWord = true
		} else if isWhitespace(r) {
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

// findPrevWordStartUTF8 finds the start of the previous word using proper UTF-8 handling.
func findPrevWordStartUTF8(text string, offset buffer.ByteOffset) buffer.ByteOffset {
	if offset <= 0 {
		return 0
	}

	textLen := buffer.ByteOffset(len(text))
	if offset > textLen {
		offset = textLen
	}

	// Work backwards through the string
	// First skip any trailing whitespace
	for offset > 0 {
		_, size := utf8.DecodeLastRuneInString(text[:offset])
		if size == 0 {
			break
		}
		r, _ := utf8.DecodeRuneInString(text[offset-buffer.ByteOffset(size):])
		if !isWhitespace(r) {
			break
		}
		offset -= buffer.ByteOffset(size)
	}

	// Now skip word characters to find the start of the word
	for offset > 0 {
		_, size := utf8.DecodeLastRuneInString(text[:offset])
		if size == 0 {
			break
		}
		r, _ := utf8.DecodeRuneInString(text[offset-buffer.ByteOffset(size):])
		if !isWordChar(r) {
			break
		}
		offset -= buffer.ByteOffset(size)
	}

	return offset
}

// isWordChar returns true if r is a word character (alphanumeric or underscore).
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

// isWhitespace returns true if r is whitespace.
func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// reverseStrings reverses a slice of strings in place.
func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// joinStrings joins strings without separator.
func joinStrings(parts []string) string {
	result := ""
	for _, s := range parts {
		result += s
	}
	return result
}

// sortSelectionsReverse sorts selections by position in descending order.
// This ensures edits don't affect subsequent cursor positions.
func sortSelectionsReverse(selections []cursor.Selection) {
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].Head > selections[j].Head
	})
}
