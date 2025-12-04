// Package operator provides handlers for Vim-style operator commands.
// Operators work with motions and text objects to perform operations
// on text ranges (e.g., delete word, change inside quotes, yank paragraph).
package operator

import (
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for operator operations.
const (
	ActionDelete     = "operator.delete"     // d - delete operator
	ActionChange     = "operator.change"     // c - change operator (delete + insert)
	ActionYank       = "operator.yank"       // y - yank (copy) operator
	ActionIndent     = "operator.indent"     // > - indent operator
	ActionOutdent    = "operator.outdent"    // < - outdent operator
	ActionLowercase  = "operator.lowercase"  // gu - lowercase operator
	ActionUppercase  = "operator.uppercase"  // gU - uppercase operator
	ActionToggleCase = "operator.toggleCase" // g~ - toggle case operator
	ActionFormat     = "operator.format"     // gq - format operator
)

// OperatorHandler handles Vim-style operator commands.
type OperatorHandler struct{}

// NewOperatorHandler creates a new operator handler.
func NewOperatorHandler() *OperatorHandler {
	return &OperatorHandler{}
}

// Namespace returns the operator namespace.
func (h *OperatorHandler) Namespace() string {
	return "operator"
}

// CanHandle returns true if this handler can process the action.
func (h *OperatorHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionDelete, ActionChange, ActionYank,
		ActionIndent, ActionOutdent,
		ActionLowercase, ActionUppercase, ActionToggleCase,
		ActionFormat:
		return true
	}
	return false
}

// HandleAction processes an operator action.
func (h *OperatorHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Operators need either a motion, text object, or visual selection
	operatorRange, err := h.resolveOperatorRange(action, ctx)
	if err != nil {
		return handler.Error(err)
	}

	switch action.Name {
	case ActionDelete:
		return h.delete(ctx, operatorRange, action.Args.Register)
	case ActionChange:
		return h.change(ctx, operatorRange, action.Args.Register)
	case ActionYank:
		return h.yank(ctx, operatorRange, action.Args.Register)
	case ActionIndent:
		return h.indent(ctx, operatorRange)
	case ActionOutdent:
		return h.outdent(ctx, operatorRange)
	case ActionLowercase:
		return h.lowercase(ctx, operatorRange)
	case ActionUppercase:
		return h.uppercase(ctx, operatorRange)
	case ActionToggleCase:
		return h.toggleCase(ctx, operatorRange)
	case ActionFormat:
		return h.format(ctx, operatorRange)
	default:
		return handler.Errorf("unknown operator action: %s", action.Name)
	}
}

// OperatorRange represents the range for an operator operation.
type OperatorRange struct {
	Start    buffer.ByteOffset
	End      buffer.ByteOffset
	Linewise bool // If true, operation applies to entire lines
}

// resolveOperatorRange determines the range for the operator.
func (h *OperatorHandler) resolveOperatorRange(action input.Action, ctx *execctx.ExecutionContext) (OperatorRange, error) {
	// Check for visual selection first
	if ctx.Cursors != nil {
		sel := ctx.Cursors.Primary()
		if !sel.IsEmpty() {
			r := sel.Range()
			return OperatorRange{
				Start:    r.Start,
				End:      r.End,
				Linewise: false, // Could be determined from mode (visual-line)
			}, nil
		}
	}

	// Check for motion
	if action.Args.Motion != nil {
		return h.resolveMotionRange(action.Args.Motion, ctx)
	}

	// Check for text object
	if action.Args.TextObject != nil {
		return h.resolveTextObjectRange(action.Args.TextObject, ctx)
	}

	return OperatorRange{}, execctx.ErrMissingMotion
}

// resolveMotionRange calculates the range for a motion.
func (h *OperatorHandler) resolveMotionRange(motion *input.Motion, ctx *execctx.ExecutionContext) (OperatorRange, error) {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return OperatorRange{}, execctx.ErrMissingEngine
	}

	engine := ctx.Engine
	sel := ctx.Cursors.Primary()
	start := sel.Head
	text := engine.Text()
	textLen := buffer.ByteOffset(len(text))

	count := motion.Count
	if count <= 0 {
		count = 1
	}

	var end buffer.ByteOffset
	linewise := false

	switch motion.Name {
	case "word", "w":
		end = h.findWordEnd(text, start, textLen, count, false)
	case "WORD", "W":
		end = h.findWordEnd(text, start, textLen, count, true)
	case "wordEnd", "e":
		end = h.findWordEndPosition(text, start, textLen, count, false)
		if motion.Inclusive {
			end = nextRuneEnd(text, end, textLen)
		}
	case "WORDEND", "E":
		end = h.findWordEndPosition(text, start, textLen, count, true)
		if motion.Inclusive {
			end = nextRuneEnd(text, end, textLen)
		}
	case "wordBack", "b":
		end = h.findWordBackward(text, start, count, false)
		// Swap start and end for backward motion
		start, end = end, start
	case "WORDBACK", "B":
		end = h.findWordBackward(text, start, count, true)
		start, end = end, start
	case "line", "l":
		linewise = true
		point := engine.OffsetToPoint(start)
		lineStart := engine.LineStartOffset(point.Line)
		endLine := point.Line + uint32(count)
		if endLine > engine.LineCount() {
			endLine = engine.LineCount()
		}
		var lineEnd buffer.ByteOffset
		if endLine >= engine.LineCount() {
			lineEnd = engine.Len()
		} else {
			lineEnd = engine.LineStartOffset(endLine)
		}
		start = lineStart
		end = lineEnd
	case "lineEnd", "$":
		point := engine.OffsetToPoint(start)
		end = engine.LineEndOffset(point.Line)
	case "lineStart", "0":
		point := engine.OffsetToPoint(start)
		end = engine.LineStartOffset(point.Line)
		start, end = end, start
	case "firstNonBlank", "^":
		point := engine.OffsetToPoint(start)
		lineStart := engine.LineStartOffset(point.Line)
		lineText := engine.LineText(point.Line)
		end = lineStart
		for i, r := range lineText {
			if !unicode.IsSpace(r) {
				end = lineStart + buffer.ByteOffset(i)
				break
			}
		}
		if end < start {
			start, end = end, start
		}
	case "paragraph", "}":
		end = h.findParagraphEnd(text, start, textLen, count)
		linewise = true
	case "paragraphBack", "{":
		end = h.findParagraphStart(text, start, count)
		start, end = end, start
		linewise = true
	case "documentEnd", "G":
		end = engine.Len()
		linewise = true
	case "documentStart", "gg":
		end = 0
		start, end = end, start
		linewise = true
	default:
		return OperatorRange{}, handler.Errorf("unknown motion: %s", motion.Name).Error
	}

	// Handle direction
	if motion.Direction == input.DirBackward && start < end {
		start, end = end, start
	}

	return OperatorRange{
		Start:    start,
		End:      end,
		Linewise: linewise,
	}, nil
}

// resolveTextObjectRange calculates the range for a text object.
func (h *OperatorHandler) resolveTextObjectRange(textObj *input.TextObject, ctx *execctx.ExecutionContext) (OperatorRange, error) {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return OperatorRange{}, execctx.ErrMissingEngine
	}

	engine := ctx.Engine
	sel := ctx.Cursors.Primary()
	offset := sel.Head
	text := engine.Text()

	switch textObj.Name {
	case "word", "w":
		start, end := h.findWordBounds(text, offset, textObj.Inner, false)
		return OperatorRange{Start: start, End: end}, nil
	case "WORD", "W":
		start, end := h.findWordBounds(text, offset, textObj.Inner, true)
		return OperatorRange{Start: start, End: end}, nil
	case "sentence", "s":
		start, end := h.findSentenceBounds(text, offset, textObj.Inner)
		return OperatorRange{Start: start, End: end}, nil
	case "paragraph", "p":
		start, end := h.findParagraphBounds(text, offset, textObj.Inner)
		return OperatorRange{Start: start, End: end, Linewise: true}, nil
	case "quote", `"`, "'", "`":
		delimiter := textObj.Delimiter
		if delimiter == 0 {
			delimiter = '"'
		}
		start, end := h.findQuoteBounds(text, offset, delimiter, textObj.Inner)
		return OperatorRange{Start: start, End: end}, nil
	case "paren", "(", ")":
		start, end := h.findBracketBounds(text, offset, '(', ')', textObj.Inner)
		return OperatorRange{Start: start, End: end}, nil
	case "bracket", "[", "]":
		start, end := h.findBracketBounds(text, offset, '[', ']', textObj.Inner)
		return OperatorRange{Start: start, End: end}, nil
	case "brace", "{", "}":
		start, end := h.findBracketBounds(text, offset, '{', '}', textObj.Inner)
		return OperatorRange{Start: start, End: end}, nil
	case "angle", "<", ">":
		start, end := h.findBracketBounds(text, offset, '<', '>', textObj.Inner)
		return OperatorRange{Start: start, End: end}, nil
	case "tag", "t":
		start, end := h.findTagBounds(text, offset, textObj.Inner)
		return OperatorRange{Start: start, End: end}, nil
	default:
		return OperatorRange{}, handler.Errorf("unknown text object: %s", textObj.Name).Error
	}
}

// delete deletes the text in the range.
func (h *OperatorHandler) delete(ctx *execctx.ExecutionContext, opRange OperatorRange, register rune) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	// Get text for register
	deletedText := engine.TextRange(opRange.Start, opRange.End)

	// Perform delete
	_, err := engine.Delete(opRange.Start, opRange.End)
	if err != nil {
		return handler.Error(err)
	}

	// Position cursor at start of deleted region
	cursors.SetPrimary(cursor.NewCursorSelection(opRange.Start))

	// Track affected lines
	startPoint := engine.OffsetToPoint(opRange.Start)

	result := handler.Success().
		WithRegisterContent(deletedText).
		WithLinewise(opRange.Linewise).
		WithRedrawLines(startPoint.Line)

	return result
}

// change deletes the text and enters insert mode.
func (h *OperatorHandler) change(ctx *execctx.ExecutionContext, opRange OperatorRange, register rune) handler.Result {
	result := h.delete(ctx, opRange, register)
	if result.Status != handler.StatusOK {
		return result
	}

	// Enter insert mode
	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("insert"); err != nil {
			return handler.Error(err)
		}
	}

	return result.WithModeChange("insert")
}

// yank copies the text to a register.
func (h *OperatorHandler) yank(ctx *execctx.ExecutionContext, opRange OperatorRange, register rune) handler.Result {
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	engine := ctx.Engine

	// Get text for register
	yankedText := engine.TextRange(opRange.Start, opRange.End)

	return handler.Success().
		WithRegisterContent(yankedText).
		WithLinewise(opRange.Linewise)
}

// indent adds indentation to lines in the range.
func (h *OperatorHandler) indent(ctx *execctx.ExecutionContext, opRange OperatorRange) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine

	// Get lines in range
	startPoint := engine.OffsetToPoint(opRange.Start)
	endPoint := engine.OffsetToPoint(opRange.End)

	// Adjust end point if at start of line
	if opRange.End > opRange.Start {
		prevPoint := engine.OffsetToPoint(opRange.End - 1)
		if prevPoint.Line < endPoint.Line {
			endPoint = prevPoint
		}
	}

	// Collect unique lines
	lines := make([]uint32, 0, endPoint.Line-startPoint.Line+1)
	for line := startPoint.Line; line <= endPoint.Line; line++ {
		lines = append(lines, line)
	}

	// Sort descending to maintain offsets
	sort.Slice(lines, func(i, j int) bool {
		return lines[i] > lines[j]
	})

	// Indent each line
	indentStr := "\t" // TODO: Use editor settings for indent string
	for _, line := range lines {
		lineStart := engine.LineStartOffset(line)

		// Skip empty lines
		lineText := engine.LineText(line)
		if len(lineText) == 0 {
			continue
		}

		_, err := engine.Insert(lineStart, indentStr)
		if err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithRedraw()
}

// outdent removes indentation from lines in the range.
func (h *OperatorHandler) outdent(ctx *execctx.ExecutionContext, opRange OperatorRange) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine

	// Get lines in range
	startPoint := engine.OffsetToPoint(opRange.Start)
	endPoint := engine.OffsetToPoint(opRange.End)

	// Collect unique lines
	lines := make([]uint32, 0, endPoint.Line-startPoint.Line+1)
	for line := startPoint.Line; line <= endPoint.Line; line++ {
		lines = append(lines, line)
	}

	// Sort descending to maintain offsets
	sort.Slice(lines, func(i, j int) bool {
		return lines[i] > lines[j]
	})

	// Outdent each line
	tabWidth := 4 // TODO: Use editor settings
	for _, line := range lines {
		lineStart := engine.LineStartOffset(line)
		lineText := engine.LineText(line)

		if len(lineText) == 0 {
			continue
		}

		// Count leading whitespace to remove
		removeCount := 0
		removed := 0
		for i, r := range lineText {
			if removed >= tabWidth {
				break
			}
			if r == '\t' {
				removeCount = i + 1
				break
			} else if r == ' ' {
				removed++
				removeCount = i + 1
			} else {
				break
			}
		}

		if removeCount > 0 {
			_, err := engine.Delete(lineStart, lineStart+buffer.ByteOffset(removeCount))
			if err != nil {
				return handler.Error(err)
			}
		}
	}

	return handler.Success().WithRedraw()
}

// lowercase converts text in range to lowercase.
func (h *OperatorHandler) lowercase(ctx *execctx.ExecutionContext, opRange OperatorRange) handler.Result {
	return h.transformCase(ctx, opRange, unicode.ToLower)
}

// uppercase converts text in range to uppercase.
func (h *OperatorHandler) uppercase(ctx *execctx.ExecutionContext, opRange OperatorRange) handler.Result {
	return h.transformCase(ctx, opRange, unicode.ToUpper)
}

// toggleCase toggles the case of text in range.
func (h *OperatorHandler) toggleCase(ctx *execctx.ExecutionContext, opRange OperatorRange) handler.Result {
	return h.transformCase(ctx, opRange, func(r rune) rune {
		if unicode.IsUpper(r) {
			return unicode.ToLower(r)
		}
		return unicode.ToUpper(r)
	})
}

// transformCase applies a case transformation to text in range.
func (h *OperatorHandler) transformCase(ctx *execctx.ExecutionContext, opRange OperatorRange, transform func(rune) rune) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine

	// Get text in range
	text := engine.TextRange(opRange.Start, opRange.End)

	// Transform case
	var result []rune
	for _, r := range text {
		result = append(result, transform(r))
	}
	newText := string(result)

	// Replace text
	_, err := engine.Replace(opRange.Start, opRange.End, newText)
	if err != nil {
		return handler.Error(err)
	}

	startPoint := engine.OffsetToPoint(opRange.Start)
	return handler.Success().WithRedrawLines(startPoint.Line)
}

// format formats text in range.
func (h *OperatorHandler) format(ctx *execctx.ExecutionContext, opRange OperatorRange) handler.Result {
	// Basic formatting: wrap lines at 80 characters
	// TODO: Use editor settings and more sophisticated formatting
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	// For now, just report success without modification
	// Full implementation would invoke language-specific formatters
	return handler.Success()
}

// Motion helper functions

// findWordEnd finds the end of count words forward.
func (h *OperatorHandler) findWordEnd(text string, offset, maxOffset buffer.ByteOffset, count int, bigWord bool) buffer.ByteOffset {
	for i := 0; i < count && offset < maxOffset; i++ {
		// Skip current word
		for offset < maxOffset && h.isWordChar(text, offset, bigWord) {
			offset = nextRuneEnd(text, offset, maxOffset)
		}
		// Skip whitespace/non-word
		for offset < maxOffset && !h.isWordChar(text, offset, bigWord) {
			if text[offset] == '\n' {
				offset++
				break
			}
			offset = nextRuneEnd(text, offset, maxOffset)
		}
	}
	return offset
}

// findWordEndPosition finds the end position of the current/next word.
func (h *OperatorHandler) findWordEndPosition(text string, offset, maxOffset buffer.ByteOffset, count int, bigWord bool) buffer.ByteOffset {
	for i := 0; i < count && offset < maxOffset; i++ {
		// Move forward one to start searching
		if offset < maxOffset {
			offset = nextRuneEnd(text, offset, maxOffset)
		}
		// Skip whitespace
		for offset < maxOffset && unicode.IsSpace(getRune(text, offset)) {
			offset = nextRuneEnd(text, offset, maxOffset)
		}
		// Move to end of word
		for offset < maxOffset && h.isWordChar(text, offset, bigWord) {
			nextOff := nextRuneEnd(text, offset, maxOffset)
			if nextOff >= maxOffset || !h.isWordChar(text, nextOff, bigWord) {
				break
			}
			offset = nextOff
		}
	}
	return offset
}

// findWordBackward finds the start of count words backward.
func (h *OperatorHandler) findWordBackward(text string, offset buffer.ByteOffset, count int, bigWord bool) buffer.ByteOffset {
	for i := 0; i < count && offset > 0; i++ {
		// Move back one to start searching
		offset = prevRuneStart(text, offset)
		// Skip whitespace
		for offset > 0 && unicode.IsSpace(getRune(text, offset)) {
			offset = prevRuneStart(text, offset)
		}
		// Move to start of word
		for offset > 0 && h.isWordChar(text, offset, bigWord) {
			prevOff := prevRuneStart(text, offset)
			if !h.isWordChar(text, prevOff, bigWord) {
				break
			}
			offset = prevOff
		}
	}
	return offset
}

// findParagraphEnd finds the end of count paragraphs forward.
func (h *OperatorHandler) findParagraphEnd(text string, offset, maxOffset buffer.ByteOffset, count int) buffer.ByteOffset {
	for i := 0; i < count && offset < maxOffset; i++ {
		// Skip non-empty lines
		for offset < maxOffset {
			if text[offset] == '\n' {
				offset++
				if offset < maxOffset && text[offset] == '\n' {
					break
				}
			} else {
				offset++
			}
		}
		// Skip empty lines
		for offset < maxOffset && text[offset] == '\n' {
			offset++
		}
	}
	return offset
}

// findParagraphStart finds the start of count paragraphs backward.
func (h *OperatorHandler) findParagraphStart(text string, offset buffer.ByteOffset, count int) buffer.ByteOffset {
	for i := 0; i < count && offset > 0; i++ {
		offset--
		// Skip empty lines
		for offset > 0 && text[offset] == '\n' {
			offset--
		}
		// Skip non-empty lines until empty line
		for offset > 0 {
			if text[offset] == '\n' && offset > 0 && text[offset-1] == '\n' {
				break
			}
			offset--
		}
	}
	if offset > 0 {
		offset++ // Move past the newline
	}
	return offset
}

// Text object helper functions

// findWordBounds finds the boundaries of the word at offset.
func (h *OperatorHandler) findWordBounds(text string, offset buffer.ByteOffset, inner, bigWord bool) (buffer.ByteOffset, buffer.ByteOffset) {
	textLen := buffer.ByteOffset(len(text))
	if offset >= textLen {
		return offset, offset
	}

	// Find start of word
	start := offset
	for start > 0 && h.isWordChar(text, start-1, bigWord) {
		start--
	}

	// Find end of word
	end := offset
	for end < textLen && h.isWordChar(text, end, bigWord) {
		end = nextRuneEnd(text, end, textLen)
	}

	if !inner {
		// Include trailing whitespace
		for end < textLen && unicode.IsSpace(getRune(text, end)) && text[end] != '\n' {
			end = nextRuneEnd(text, end, textLen)
		}
	}

	return start, end
}

// findSentenceBounds finds the boundaries of the sentence at offset.
func (h *OperatorHandler) findSentenceBounds(text string, offset buffer.ByteOffset, inner bool) (buffer.ByteOffset, buffer.ByteOffset) {
	textLen := buffer.ByteOffset(len(text))

	// Find start of sentence (after previous sentence-ending punctuation)
	start := offset
	for start > 0 {
		r := getRune(text, start-1)
		if r == '.' || r == '!' || r == '?' {
			break
		}
		start--
	}
	// Skip whitespace after punctuation
	for start < textLen && unicode.IsSpace(getRune(text, start)) {
		start++
	}

	// Find end of sentence
	end := offset
	for end < textLen {
		r := getRune(text, end)
		end = nextRuneEnd(text, end, textLen)
		if r == '.' || r == '!' || r == '?' {
			break
		}
	}

	if !inner {
		// Include trailing whitespace
		for end < textLen && unicode.IsSpace(getRune(text, end)) {
			end = nextRuneEnd(text, end, textLen)
		}
	}

	return start, end
}

// findParagraphBounds finds the boundaries of the paragraph at offset.
func (h *OperatorHandler) findParagraphBounds(text string, offset buffer.ByteOffset, inner bool) (buffer.ByteOffset, buffer.ByteOffset) {
	textLen := buffer.ByteOffset(len(text))

	// Find start of paragraph
	start := offset
	for start > 0 {
		if text[start-1] == '\n' && (start < 2 || text[start-2] == '\n') {
			break
		}
		start--
	}

	// Find end of paragraph
	end := offset
	for end < textLen {
		if text[end] == '\n' && (end+1 >= textLen || text[end+1] == '\n') {
			end++
			break
		}
		end++
	}

	if !inner {
		// Include trailing empty lines
		for end < textLen && text[end] == '\n' {
			end++
		}
	}

	return start, end
}

// findQuoteBounds finds matching quotes around offset.
func (h *OperatorHandler) findQuoteBounds(text string, offset buffer.ByteOffset, quote rune, inner bool) (buffer.ByteOffset, buffer.ByteOffset) {
	textLen := buffer.ByteOffset(len(text))

	// Find opening quote (searching backward)
	start := offset
	for start > 0 {
		if getRune(text, start-1) == quote {
			start--
			break
		}
		start--
	}

	// Find closing quote (searching forward)
	end := offset
	for end < textLen {
		if getRune(text, end) == quote && end != start {
			end = nextRuneEnd(text, end, textLen)
			break
		}
		end = nextRuneEnd(text, end, textLen)
	}

	if inner && end > start+1 {
		// Exclude the quotes themselves
		start = nextRuneEnd(text, start, textLen)
		end = prevRuneStart(text, end)
	}

	return start, end
}

// findBracketBounds finds matching brackets around offset.
func (h *OperatorHandler) findBracketBounds(text string, offset buffer.ByteOffset, open, close rune, inner bool) (buffer.ByteOffset, buffer.ByteOffset) {
	textLen := buffer.ByteOffset(len(text))

	// Find opening bracket with nesting
	start := offset
	depth := 0
	for start > 0 {
		r := getRune(text, start)
		if r == close {
			depth++
		} else if r == open {
			if depth == 0 {
				break
			}
			depth--
		}
		start--
	}

	// Find closing bracket with nesting
	end := offset
	depth = 0
	for end < textLen {
		r := getRune(text, end)
		if r == open {
			depth++
		} else if r == close {
			if depth == 0 {
				end = nextRuneEnd(text, end, textLen)
				break
			}
			depth--
		}
		end = nextRuneEnd(text, end, textLen)
	}

	if inner && end > start+1 {
		// Exclude the brackets themselves
		start = nextRuneEnd(text, start, textLen)
		end = prevRuneStart(text, end)
	}

	return start, end
}

// findTagBounds finds matching XML/HTML tags around offset.
func (h *OperatorHandler) findTagBounds(text string, offset buffer.ByteOffset, inner bool) (buffer.ByteOffset, buffer.ByteOffset) {
	// Simplified implementation - just finds < and > pairs
	return h.findBracketBounds(text, offset, '<', '>', inner)
}

// isWordChar returns true if the character at offset is a word character.
func (h *OperatorHandler) isWordChar(text string, offset buffer.ByteOffset, bigWord bool) bool {
	if int(offset) >= len(text) {
		return false
	}
	r := getRune(text, offset)
	if bigWord {
		// WORD: non-whitespace
		return !unicode.IsSpace(r)
	}
	// word: alphanumeric + underscore
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// getRune returns the rune at the given byte offset.
func getRune(text string, offset buffer.ByteOffset) rune {
	if int(offset) >= len(text) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(text[offset:])
	return r
}

// nextRuneEnd returns the byte offset after the rune at offset.
func nextRuneEnd(text string, offset, maxOffset buffer.ByteOffset) buffer.ByteOffset {
	if offset >= maxOffset || int(offset) >= len(text) {
		return maxOffset
	}
	_, size := utf8.DecodeRuneInString(text[offset:])
	newOffset := offset + buffer.ByteOffset(size)
	if newOffset > maxOffset {
		return maxOffset
	}
	return newOffset
}

// prevRuneStart returns the byte offset of the rune before offset.
func prevRuneStart(text string, offset buffer.ByteOffset) buffer.ByteOffset {
	if offset <= 0 {
		return 0
	}
	if int(offset) > len(text) {
		offset = buffer.ByteOffset(len(text))
	}
	_, size := utf8.DecodeLastRuneInString(text[:offset])
	if size == 0 {
		return 0
	}
	return offset - buffer.ByteOffset(size)
}
