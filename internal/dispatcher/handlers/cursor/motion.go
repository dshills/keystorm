package cursor

import (
	"unicode"
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for word/line/paragraph motions.
const (
	// Word motions
	ActionWordForward       = "cursor.wordForward"
	ActionWordBackward      = "cursor.wordBackward"
	ActionWordEndForward    = "cursor.wordEndForward"
	ActionBigWordForward    = "cursor.bigWordForward"
	ActionBigWordBackward   = "cursor.bigWordBackward"
	ActionBigWordEndForward = "cursor.bigWordEndForward"

	// Line motions
	ActionFirstNonBlank   = "cursor.firstNonBlank"
	ActionGotoLine        = "cursor.gotoLine"
	ActionGotoColumn      = "cursor.gotoColumn"
	ActionMatchingBracket = "cursor.matchingBracket"
	ActionGotoPercent     = "cursor.gotoPercent"

	// Paragraph/sentence motions
	ActionParagraphForward  = "cursor.paragraphForward"
	ActionParagraphBackward = "cursor.paragraphBackward"
	ActionSentenceForward   = "cursor.sentenceForward"
	ActionSentenceBackward  = "cursor.sentenceBackward"

	// Screen motions
	ActionScreenTop    = "cursor.screenTop"
	ActionScreenMiddle = "cursor.screenMiddle"
	ActionScreenBottom = "cursor.screenBottom"
)

// MotionHandler handles word, paragraph, and other motion-based cursor movements.
type MotionHandler struct{}

// NewMotionHandler creates a new motion handler.
func NewMotionHandler() *MotionHandler {
	return &MotionHandler{}
}

// Namespace returns the cursor namespace.
func (h *MotionHandler) Namespace() string {
	return "cursor"
}

// CanHandle returns true if this handler can process the action.
func (h *MotionHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionWordForward, ActionWordBackward, ActionWordEndForward,
		ActionBigWordForward, ActionBigWordBackward, ActionBigWordEndForward,
		ActionFirstNonBlank, ActionGotoLine, ActionGotoColumn, ActionMatchingBracket, ActionGotoPercent,
		ActionParagraphForward, ActionParagraphBackward,
		ActionSentenceForward, ActionSentenceBackward,
		ActionScreenTop, ActionScreenMiddle, ActionScreenBottom:
		return true
	}
	return false
}

// HandleAction processes a motion action.
func (h *MotionHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	count := ctx.GetCount()

	switch action.Name {
	// Word motions
	case ActionWordForward:
		return h.wordForward(ctx, count, false)
	case ActionWordBackward:
		return h.wordBackward(ctx, count, false)
	case ActionWordEndForward:
		return h.wordEndForward(ctx, count, false)
	case ActionBigWordForward:
		return h.wordForward(ctx, count, true)
	case ActionBigWordBackward:
		return h.wordBackward(ctx, count, true)
	case ActionBigWordEndForward:
		return h.wordEndForward(ctx, count, true)

	// Line motions
	case ActionFirstNonBlank:
		return h.firstNonBlank(ctx)
	case ActionGotoLine:
		return h.gotoLine(ctx, count)
	case ActionGotoColumn:
		return h.gotoColumn(ctx, count)
	case ActionMatchingBracket:
		return h.matchingBracket(ctx)
	case ActionGotoPercent:
		return h.gotoPercent(ctx, count)

	// Paragraph/sentence motions
	case ActionParagraphForward:
		return h.paragraphForward(ctx, count)
	case ActionParagraphBackward:
		return h.paragraphBackward(ctx, count)
	case ActionSentenceForward:
		return h.sentenceForward(ctx, count)
	case ActionSentenceBackward:
		return h.sentenceBackward(ctx, count)

	// Screen motions
	case ActionScreenTop:
		return h.screenTop(ctx)
	case ActionScreenMiddle:
		return h.screenMiddle(ctx)
	case ActionScreenBottom:
		return h.screenBottom(ctx)

	default:
		return handler.Errorf("unknown motion action: %s", action.Name)
	}
}

// wordForward moves to the beginning of the next word.
// If bigWord is true, treats WORD (whitespace-delimited) instead of word.
func (h *MotionHandler) wordForward(ctx *execctx.ExecutionContext, count int, bigWord bool) handler.Result {
	engine := ctx.Engine
	text := engine.Text()
	maxOffset := engine.Len()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		offset := sel.Head

		for i := 0; i < count && offset < maxOffset; i++ {
			offset = findNextWordStart(text, offset, maxOffset, bigWord)
		}

		if ctx.HasSelection() {
			return sel.Extend(offset)
		}
		return sel.MoveTo(offset)
	})

	return handler.Success().WithRedraw()
}

// wordBackward moves to the beginning of the previous word.
func (h *MotionHandler) wordBackward(ctx *execctx.ExecutionContext, count int, bigWord bool) handler.Result {
	engine := ctx.Engine
	text := engine.Text()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		offset := sel.Head

		for i := 0; i < count && offset > 0; i++ {
			offset = findPrevWordStart(text, offset, bigWord)
		}

		if ctx.HasSelection() {
			return sel.Extend(offset)
		}
		return sel.MoveTo(offset)
	})

	return handler.Success().WithRedraw()
}

// wordEndForward moves to the end of the current or next word.
func (h *MotionHandler) wordEndForward(ctx *execctx.ExecutionContext, count int, bigWord bool) handler.Result {
	engine := ctx.Engine
	text := engine.Text()
	maxOffset := engine.Len()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		offset := sel.Head

		for i := 0; i < count && offset < maxOffset; i++ {
			offset = findWordEnd(text, offset, maxOffset, bigWord)
		}

		if ctx.HasSelection() {
			return sel.Extend(offset)
		}
		return sel.MoveTo(offset)
	})

	return handler.Success().WithRedraw()
}

// firstNonBlank moves to the first non-blank character on the line.
func (h *MotionHandler) firstNonBlank(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	text := engine.Text()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)
		lineStart := engine.LineStartOffset(point.Line)
		lineEnd := engine.LineEndOffset(point.Line)

		// Find first non-blank character
		offset := lineStart
		for offset < lineEnd {
			r, size := utf8.DecodeRuneInString(text[offset:])
			if !unicode.IsSpace(r) {
				break
			}
			offset += buffer.ByteOffset(size)
		}

		if ctx.HasSelection() {
			return sel.Extend(offset)
		}
		return sel.MoveTo(offset)
	})

	return handler.Success().WithRedraw()
}

// gotoLine moves to a specific line (1-indexed from user perspective).
func (h *MotionHandler) gotoLine(ctx *execctx.ExecutionContext, lineNum int) handler.Result {
	engine := ctx.Engine
	lineCount := int(engine.LineCount())

	// Clamp line number (convert from 1-indexed to 0-indexed)
	targetLine := lineNum - 1
	if targetLine < 0 {
		targetLine = 0
	}
	if targetLine >= lineCount {
		targetLine = lineCount - 1
	}

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := engine.LineStartOffset(uint32(targetLine))

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// gotoColumn moves to a specific column on the current line.
func (h *MotionHandler) gotoColumn(ctx *execctx.ExecutionContext, col int) handler.Result {
	engine := ctx.Engine

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)

		// Clamp column (convert from 1-indexed to 0-indexed)
		targetCol := uint32(col - 1)
		lineLen := engine.LineLen(point.Line)
		if targetCol > lineLen {
			targetCol = lineLen
		}

		newOffset := engine.PointToOffset(buffer.Point{Line: point.Line, Column: targetCol})

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// matchingBracket finds the matching bracket under or after cursor.
func (h *MotionHandler) matchingBracket(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	text := engine.Text()
	maxOffset := engine.Len()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		offset := sel.Head

		// Find a bracket at or after cursor on current line
		point := engine.OffsetToPoint(offset)
		lineEnd := engine.LineEndOffset(point.Line)

		var bracketOffset buffer.ByteOffset = -1
		var bracket rune
		searchOffset := offset

		for searchOffset < lineEnd {
			r, size := utf8.DecodeRuneInString(text[searchOffset:])
			if isBracket(r) {
				bracketOffset = searchOffset
				bracket = r
				break
			}
			searchOffset += buffer.ByteOffset(size)
		}

		if bracketOffset < 0 {
			// No bracket found
			return sel
		}

		// Find matching bracket
		matchOffset, found := findMatchingBracket(text, bracketOffset, maxOffset, bracket)
		if !found {
			return sel
		}

		if ctx.HasSelection() {
			return sel.Extend(matchOffset)
		}
		return sel.MoveTo(matchOffset)
	})

	return handler.Success().WithRedraw()
}

// gotoPercent moves to a percentage position in the file.
func (h *MotionHandler) gotoPercent(ctx *execctx.ExecutionContext, percent int) handler.Result {
	engine := ctx.Engine
	lineCount := int(engine.LineCount())

	// Handle empty buffer
	if lineCount == 0 {
		return handler.NoOp()
	}

	// Clamp percentage
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}

	// Calculate target line
	targetLine := (lineCount * percent) / 100
	if targetLine >= lineCount {
		targetLine = lineCount - 1
	}

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := engine.LineStartOffset(uint32(targetLine))

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// paragraphForward moves forward to the next paragraph boundary.
func (h *MotionHandler) paragraphForward(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	lineCount := engine.LineCount()

	// Handle empty buffer
	if lineCount == 0 {
		return handler.NoOp()
	}

	lastLine := lineCount - 1

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)
		line := point.Line

		for i := 0; i < count && line < lastLine; i++ {
			// Skip non-empty lines
			for line < lastLine && !isEmptyLine(engine, line) {
				line++
			}
			// Skip empty lines
			for line < lastLine && isEmptyLine(engine, line) {
				line++
			}
		}

		newOffset := engine.LineStartOffset(line)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// paragraphBackward moves backward to the previous paragraph boundary.
func (h *MotionHandler) paragraphBackward(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)
		line := point.Line

		for i := 0; i < count && line > 0; i++ {
			// Skip empty lines
			for line > 0 && isEmptyLine(engine, line) {
				line--
			}
			// Skip non-empty lines
			for line > 0 && !isEmptyLine(engine, line) {
				line--
			}
		}

		newOffset := engine.LineStartOffset(line)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// sentenceForward moves forward to the start of the next sentence.
func (h *MotionHandler) sentenceForward(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	text := engine.Text()
	maxOffset := engine.Len()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		offset := sel.Head

		for i := 0; i < count && offset < maxOffset; i++ {
			offset = findNextSentenceStart(text, offset, maxOffset)
		}

		if ctx.HasSelection() {
			return sel.Extend(offset)
		}
		return sel.MoveTo(offset)
	})

	return handler.Success().WithRedraw()
}

// sentenceBackward moves backward to the start of the previous sentence.
func (h *MotionHandler) sentenceBackward(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	text := engine.Text()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		offset := sel.Head

		for i := 0; i < count && offset > 0; i++ {
			offset = findPrevSentenceStart(text, offset)
		}

		if ctx.HasSelection() {
			return sel.Extend(offset)
		}
		return sel.MoveTo(offset)
	})

	return handler.Success().WithRedraw()
}

// screenTop moves cursor to the top of the visible screen.
func (h *MotionHandler) screenTop(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine

	var targetLine uint32
	if ctx.Renderer != nil {
		start, _ := ctx.Renderer.VisibleLineRange()
		targetLine = start
	}

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := engine.LineStartOffset(targetLine)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// screenMiddle moves cursor to the middle of the visible screen.
func (h *MotionHandler) screenMiddle(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	lineCount := engine.LineCount()

	// Handle empty buffer
	if lineCount == 0 {
		return handler.NoOp()
	}

	var targetLine uint32
	if ctx.Renderer != nil {
		start, end := ctx.Renderer.VisibleLineRange()
		targetLine = start + (end-start)/2
	} else {
		targetLine = lineCount / 2
	}

	if targetLine >= lineCount {
		targetLine = lineCount - 1
	}

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := engine.LineStartOffset(targetLine)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// screenBottom moves cursor to the bottom of the visible screen.
func (h *MotionHandler) screenBottom(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	lineCount := engine.LineCount()

	// Handle empty buffer
	if lineCount == 0 {
		return handler.NoOp()
	}

	var targetLine uint32
	if ctx.Renderer != nil {
		_, end := ctx.Renderer.VisibleLineRange()
		targetLine = end
	} else {
		targetLine = lineCount - 1
	}

	if targetLine >= lineCount {
		targetLine = lineCount - 1
	}

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := engine.LineStartOffset(targetLine)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// Helper functions

// findNextWordStart finds the start of the next word.
func findNextWordStart(text string, offset, maxOffset buffer.ByteOffset, bigWord bool) buffer.ByteOffset {
	textLen := buffer.ByteOffset(len(text))
	if maxOffset > textLen {
		maxOffset = textLen
	}
	if offset >= maxOffset {
		return maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	// First, skip current word (if any)
	inWord := false
	for offset < maxOffset {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if size == 0 {
			break
		}
		isWS := unicode.IsSpace(r)
		isWordChar := isWordCharacter(r, bigWord)

		if !inWord && isWordChar {
			inWord = true
		} else if inWord && !isWordChar {
			// Reached end of word
			break
		}

		offset += buffer.ByteOffset(size)
		if !inWord && !isWS {
			// Skip punctuation
			continue
		}
	}

	// Then, skip whitespace to find next word
	for offset < maxOffset {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if !unicode.IsSpace(r) {
			break
		}
		offset += buffer.ByteOffset(size)
	}

	return offset
}

// findPrevWordStart finds the start of the previous word.
func findPrevWordStart(text string, offset buffer.ByteOffset, bigWord bool) buffer.ByteOffset {
	textLen := buffer.ByteOffset(len(text))
	if offset <= 0 {
		return 0
	}
	if offset > textLen {
		offset = textLen
	}

	// Move back one character to start
	offset = prevRuneStart(text, offset)

	// Skip whitespace backwards
	for offset > 0 {
		r, _ := utf8.DecodeRuneInString(text[offset:])
		if !unicode.IsSpace(r) {
			break
		}
		offset = prevRuneStart(text, offset)
	}

	// Find the start of the word
	for offset > 0 {
		prevOffset := prevRuneStart(text, offset)
		r, _ := utf8.DecodeRuneInString(text[prevOffset:])
		if !isWordCharacter(r, bigWord) {
			break
		}
		offset = prevOffset
	}

	return offset
}

// findWordEnd finds the end of the current or next word.
func findWordEnd(text string, offset, maxOffset buffer.ByteOffset, bigWord bool) buffer.ByteOffset {
	textLen := buffer.ByteOffset(len(text))
	if maxOffset > textLen {
		maxOffset = textLen
	}
	if offset >= maxOffset {
		return maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	// Move forward one to get off current position
	_, size := utf8.DecodeRuneInString(text[offset:])
	if size == 0 {
		return offset
	}
	offset += buffer.ByteOffset(size)

	// Skip whitespace
	for offset < maxOffset {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if !unicode.IsSpace(r) {
			break
		}
		offset += buffer.ByteOffset(size)
	}

	// Find end of word
	for offset < maxOffset {
		_, size := utf8.DecodeRuneInString(text[offset:])
		nextOffset := offset + buffer.ByteOffset(size)

		if nextOffset >= maxOffset {
			return offset
		}

		nextR, _ := utf8.DecodeRuneInString(text[nextOffset:])
		if !isWordCharacter(nextR, bigWord) {
			return offset
		}

		offset = nextOffset
	}

	return offset
}

// isWordCharacter returns true if r is a word character.
// For bigWord, only whitespace is not a word character.
func isWordCharacter(r rune, bigWord bool) bool {
	if bigWord {
		return !unicode.IsSpace(r)
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// isBracket returns true if r is a bracket character.
func isBracket(r rune) bool {
	switch r {
	case '(', ')', '[', ']', '{', '}', '<', '>':
		return true
	}
	return false
}

// matchingBracketFor returns the matching bracket, direction, and whether it's valid.
// Returns (matchRune, isForward, isValid).
func matchingBracketFor(r rune) (rune, bool, bool) {
	switch r {
	case '(':
		return ')', true, true
	case ')':
		return '(', false, true
	case '[':
		return ']', true, true
	case ']':
		return '[', false, true
	case '{':
		return '}', true, true
	case '}':
		return '{', false, true
	case '<':
		return '>', true, true
	case '>':
		return '<', false, true
	}
	return 0, false, false
}

// findMatchingBracket finds the matching bracket.
// Returns (offset, found) where found indicates if a match was found.
func findMatchingBracket(text string, offset, maxOffset buffer.ByteOffset, bracket rune) (buffer.ByteOffset, bool) {
	match, forward, valid := matchingBracketFor(bracket)
	if !valid {
		return 0, false
	}

	textLen := buffer.ByteOffset(len(text))
	if offset < 0 || offset >= textLen {
		return 0, false
	}
	if maxOffset > textLen {
		maxOffset = textLen
	}

	depth := 1

	if forward {
		// Move forward - skip opening bracket
		_, size := utf8.DecodeRuneInString(text[offset:])
		if size == 0 {
			return 0, false
		}
		offset += buffer.ByteOffset(size)

		for offset < maxOffset && depth > 0 {
			r, size := utf8.DecodeRuneInString(text[offset:])
			if size == 0 {
				break
			}
			if r == bracket {
				depth++
			} else if r == match {
				depth--
				if depth == 0 {
					return offset, true
				}
			}
			offset += buffer.ByteOffset(size)
		}
	} else {
		// Move backward
		offset = prevRuneStart(text, offset)

		for depth > 0 {
			if offset >= textLen {
				break
			}
			r, _ := utf8.DecodeRuneInString(text[offset:])
			if r == bracket {
				depth++
			} else if r == match {
				depth--
				if depth == 0 {
					return offset, true
				}
			}
			if offset == 0 {
				break
			}
			offset = prevRuneStart(text, offset)
		}
	}

	return 0, false // No match found
}

// isEmptyLine returns true if the line is empty or only whitespace.
func isEmptyLine(engine execctx.EngineInterface, line uint32) bool {
	lineText := engine.LineText(line)
	for _, r := range lineText {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// findNextSentenceStart finds the start of the next sentence.
func findNextSentenceStart(text string, offset, maxOffset buffer.ByteOffset) buffer.ByteOffset {
	textLen := buffer.ByteOffset(len(text))
	if maxOffset > textLen {
		maxOffset = textLen
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= maxOffset {
		return maxOffset
	}

	// A sentence ends with '.', '!', or '?' followed by whitespace
	foundEnd := false

	for offset < maxOffset {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if size == 0 {
			break
		}

		if r == '.' || r == '!' || r == '?' {
			foundEnd = true
		} else if foundEnd && !unicode.IsSpace(r) {
			return offset
		} else if foundEnd && r == '\n' {
			// Double newline also ends sentence
			nextOffset := offset + buffer.ByteOffset(size)
			if nextOffset < maxOffset {
				nextR, _ := utf8.DecodeRuneInString(text[nextOffset:])
				if nextR == '\n' {
					return nextOffset
				}
			}
		}

		offset += buffer.ByteOffset(size)
	}

	return maxOffset
}

// findPrevSentenceStart finds the start of the previous sentence.
func findPrevSentenceStart(text string, offset buffer.ByteOffset) buffer.ByteOffset {
	textLen := buffer.ByteOffset(len(text))
	if offset <= 0 {
		return 0
	}
	if offset > textLen {
		offset = textLen
	}

	// Move back to start
	offset = prevRuneStart(text, offset)

	// Skip whitespace
	for offset > 0 {
		r, _ := utf8.DecodeRuneInString(text[offset:])
		if !unicode.IsSpace(r) {
			break
		}
		offset = prevRuneStart(text, offset)
	}

	// Skip to before sentence ending
	for offset > 0 {
		r, _ := utf8.DecodeRuneInString(text[offset:])
		if r == '.' || r == '!' || r == '?' {
			break
		}
		offset = prevRuneStart(text, offset)
	}

	// Find the actual start of this sentence
	for offset > 0 {
		prevOffset := prevRuneStart(text, offset)
		r, _ := utf8.DecodeRuneInString(text[prevOffset:])
		if r == '.' || r == '!' || r == '?' {
			// Skip trailing whitespace of previous sentence
			r2, _ := utf8.DecodeRuneInString(text[offset:])
			for offset < buffer.ByteOffset(len(text)) && unicode.IsSpace(r2) {
				_, size := utf8.DecodeRuneInString(text[offset:])
				offset += buffer.ByteOffset(size)
				if offset < buffer.ByteOffset(len(text)) {
					r2, _ = utf8.DecodeRuneInString(text[offset:])
				}
			}
			return offset
		}
		offset = prevOffset
	}

	return 0
}
