// Package search provides handlers for search and replace operations.
package search

import (
	"regexp"
	"strings"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for search operations.
const (
	ActionSearchForward      = "search.forward"      // / - forward search
	ActionSearchBackward     = "search.backward"     // ? - backward search
	ActionSearchNext         = "search.next"         // n - find next match
	ActionSearchPrev         = "search.prev"         // N - find previous match
	ActionSearchWordForward  = "search.wordForward"  // * - search word under cursor forward
	ActionSearchWordBackward = "search.wordBackward" // # - search word under cursor backward
	ActionReplace            = "search.replace"      // :s - replace in range
	ActionReplaceAll         = "search.replaceAll"   // :%s - replace all
	ActionClearSearch        = "search.clear"        // clear search highlight
)

// SearchState holds the current search state.
// This is stored in the execution context data.
type SearchState struct {
	// Pattern is the current search pattern.
	Pattern string
	// Regex is the compiled regular expression.
	Regex *regexp.Regexp
	// Forward indicates the search direction.
	Forward bool
	// CaseSensitive indicates case-sensitive search.
	CaseSensitive bool
}

const searchStateKey = "_search_state"

// Handler implements namespace-based search handling.
type Handler struct{}

// NewHandler creates a new search handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Namespace returns the search namespace.
func (h *Handler) Namespace() string {
	return "search"
}

// CanHandle returns true if this handler can process the action.
func (h *Handler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionSearchForward, ActionSearchBackward, ActionSearchNext, ActionSearchPrev,
		ActionSearchWordForward, ActionSearchWordBackward, ActionReplace, ActionReplaceAll,
		ActionClearSearch:
		return true
	}
	return false
}

// HandleAction processes a search action.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	switch action.Name {
	case ActionSearchForward:
		return h.searchForward(action, ctx)
	case ActionSearchBackward:
		return h.searchBackward(action, ctx)
	case ActionSearchNext:
		return h.searchNext(ctx)
	case ActionSearchPrev:
		return h.searchPrev(ctx)
	case ActionSearchWordForward:
		return h.searchWordForward(ctx)
	case ActionSearchWordBackward:
		return h.searchWordBackward(ctx)
	case ActionReplace:
		return h.replace(action, ctx)
	case ActionReplaceAll:
		return h.replaceAll(action, ctx)
	case ActionClearSearch:
		return h.clearSearch(ctx)
	default:
		return handler.Errorf("unknown search action: %s", action.Name)
	}
}

// searchForward initiates or continues a forward search.
func (h *Handler) searchForward(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	pattern := action.Args.SearchPattern
	if pattern == "" {
		// No pattern provided - this would typically open a search prompt
		return handler.NoOpWithMessage("search: pattern required")
	}

	state, err := compilePattern(pattern, true, true)
	if err != nil {
		return handler.Errorf("search: invalid pattern: %v", err)
	}

	// Store search state for repeat operations
	ctx.SetData(searchStateKey, state)

	// Find next match from current position
	return h.findNext(ctx, state)
}

// searchBackward initiates or continues a backward search.
func (h *Handler) searchBackward(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	pattern := action.Args.SearchPattern
	if pattern == "" {
		return handler.NoOpWithMessage("search: pattern required")
	}

	state, err := compilePattern(pattern, false, true)
	if err != nil {
		return handler.Errorf("search: invalid pattern: %v", err)
	}

	ctx.SetData(searchStateKey, state)
	return h.findPrev(ctx, state)
}

// searchNext finds the next match using the current search pattern.
func (h *Handler) searchNext(ctx *execctx.ExecutionContext) handler.Result {
	state := getSearchState(ctx)
	if state == nil {
		return handler.NoOpWithMessage("search: no previous search")
	}

	if state.Forward {
		return h.findNext(ctx, state)
	}
	return h.findPrev(ctx, state)
}

// searchPrev finds the previous match using the current search pattern.
func (h *Handler) searchPrev(ctx *execctx.ExecutionContext) handler.Result {
	state := getSearchState(ctx)
	if state == nil {
		return handler.NoOpWithMessage("search: no previous search")
	}

	// Reverse direction
	if state.Forward {
		return h.findPrev(ctx, state)
	}
	return h.findNext(ctx, state)
}

// searchWordForward searches for the word under the cursor forward.
func (h *Handler) searchWordForward(ctx *execctx.ExecutionContext) handler.Result {
	word, err := h.wordUnderCursor(ctx)
	if err != nil {
		return handler.Error(err)
	}
	if word == "" {
		return handler.NoOpWithMessage("search: no word under cursor")
	}

	// Create word boundary pattern
	pattern := `\b` + regexp.QuoteMeta(word) + `\b`
	state, err := compilePattern(pattern, true, true)
	if err != nil {
		return handler.Errorf("search: invalid pattern: %v", err)
	}

	ctx.SetData(searchStateKey, state)
	return h.findNext(ctx, state)
}

// searchWordBackward searches for the word under the cursor backward.
func (h *Handler) searchWordBackward(ctx *execctx.ExecutionContext) handler.Result {
	word, err := h.wordUnderCursor(ctx)
	if err != nil {
		return handler.Error(err)
	}
	if word == "" {
		return handler.NoOpWithMessage("search: no word under cursor")
	}

	pattern := `\b` + regexp.QuoteMeta(word) + `\b`
	state, err := compilePattern(pattern, false, true)
	if err != nil {
		return handler.Errorf("search: invalid pattern: %v", err)
	}

	ctx.SetData(searchStateKey, state)
	return h.findPrev(ctx, state)
}

// replace performs search and replace in a range or current line.
func (h *Handler) replace(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	pattern := action.Args.SearchPattern
	replacement := action.Args.GetString("replacement")
	if pattern == "" {
		return handler.NoOpWithMessage("replace: pattern required")
	}

	state, err := compilePattern(pattern, true, true)
	if err != nil {
		return handler.Errorf("replace: invalid pattern: %v", err)
	}

	// Get range from action or default to current line
	startLine := uint32(action.Args.GetInt("startLine"))
	endLine := uint32(action.Args.GetInt("endLine"))

	engine := ctx.Engine
	if endLine == 0 {
		// Default to current line
		if ctx.Cursors != nil {
			pos := engine.OffsetToPoint(ctx.Cursors.Primary().Head)
			startLine = pos.Line
			endLine = pos.Line + 1
		}
	}

	return h.replaceInRange(ctx, state, replacement, startLine, endLine, action.Args.GetBool("global"))
}

// replaceAll performs search and replace in the entire buffer.
func (h *Handler) replaceAll(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	pattern := action.Args.SearchPattern
	replacement := action.Args.GetString("replacement")
	if pattern == "" {
		return handler.NoOpWithMessage("replace: pattern required")
	}

	state, err := compilePattern(pattern, true, true)
	if err != nil {
		return handler.Errorf("replace: invalid pattern: %v", err)
	}

	engine := ctx.Engine
	return h.replaceInRange(ctx, state, replacement, 0, engine.LineCount(), action.Args.GetBool("global"))
}

// clearSearch clears the search state.
func (h *Handler) clearSearch(ctx *execctx.ExecutionContext) handler.Result {
	ctx.SetData(searchStateKey, nil)
	return handler.Success().WithMessage("search cleared")
}

// findNext finds the next match after the current cursor position.
func (h *Handler) findNext(ctx *execctx.ExecutionContext, state *SearchState) handler.Result {
	engine := ctx.Engine
	text := engine.Text()
	textLen := len(text)

	// Get current position
	startOffset := 0
	if ctx.Cursors != nil {
		startOffset = int(ctx.Cursors.Primary().Head) + 1 // Start after cursor
	}

	if startOffset >= textLen {
		startOffset = 0 // Wrap to beginning
	}

	// Search from current position to end
	loc := state.Regex.FindStringIndex(text[startOffset:])
	if loc != nil {
		matchStart := buffer.ByteOffset(startOffset + loc[0])
		return h.moveCursorToMatch(ctx, matchStart).
			WithMessage("search: " + state.Pattern)
	}

	// Wrap around: search from beginning to current position
	if startOffset > 0 {
		loc = state.Regex.FindStringIndex(text[:startOffset])
		if loc != nil {
			matchStart := buffer.ByteOffset(loc[0])
			return h.moveCursorToMatch(ctx, matchStart).
				WithMessage("search: " + state.Pattern + " (wrapped)")
		}
	}

	return handler.NoOpWithMessage("search: pattern not found: " + state.Pattern)
}

// findPrev finds the previous match before the current cursor position.
func (h *Handler) findPrev(ctx *execctx.ExecutionContext, state *SearchState) handler.Result {
	engine := ctx.Engine
	text := engine.Text()

	// Get current position
	endOffset := len(text)
	if ctx.Cursors != nil {
		endOffset = int(ctx.Cursors.Primary().Head)
	}

	if endOffset <= 0 {
		endOffset = len(text) // Wrap to end
	}

	// Find all matches before current position and take the last one
	matches := state.Regex.FindAllStringIndex(text[:endOffset], -1)
	if len(matches) > 0 {
		lastMatch := matches[len(matches)-1]
		matchStart := buffer.ByteOffset(lastMatch[0])
		return h.moveCursorToMatch(ctx, matchStart).
			WithMessage("search: " + state.Pattern)
	}

	// Wrap around: search from current position to end
	if endOffset < len(text) {
		matches = state.Regex.FindAllStringIndex(text[endOffset:], -1)
		if len(matches) > 0 {
			lastMatch := matches[len(matches)-1]
			matchStart := buffer.ByteOffset(endOffset + lastMatch[0])
			return h.moveCursorToMatch(ctx, matchStart).
				WithMessage("search: " + state.Pattern + " (wrapped)")
		}
	}

	return handler.NoOpWithMessage("search: pattern not found: " + state.Pattern)
}

// moveCursorToMatch moves the cursor to the match position.
func (h *Handler) moveCursorToMatch(ctx *execctx.ExecutionContext, offset buffer.ByteOffset) handler.Result {
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	sel := ctx.Cursors.Primary().MoveTo(offset)
	ctx.Cursors.SetPrimary(sel)

	// Calculate line for scrolling
	point := ctx.Engine.OffsetToPoint(offset)
	return handler.Success().WithScrollTo(point.Line, point.Column, true)
}

// replaceInRange replaces matches in the specified line range.
func (h *Handler) replaceInRange(ctx *execctx.ExecutionContext, state *SearchState, replacement string, startLine, endLine uint32, global bool) handler.Result {
	engine := ctx.Engine

	if ctx.History != nil {
		ctx.History.BeginGroup("replace")
		defer ctx.History.EndGroup()
	}

	replacements := 0
	affectedLines := make([]uint32, 0)

	// Process lines in reverse order to maintain offsets
	for line := int(endLine) - 1; line >= int(startLine); line-- {
		lineStart := engine.LineStartOffset(uint32(line))
		lineEnd := engine.LineEndOffset(uint32(line))
		lineText := engine.TextRange(lineStart, lineEnd)

		var newText string
		if global {
			newText = state.Regex.ReplaceAllString(lineText, replacement)
		} else {
			newText = state.Regex.ReplaceAllStringFunc(lineText, func(match string) string {
				if replacements == 0 || global {
					replacements++
					return replacement
				}
				return match
			})
		}

		if newText != lineText {
			_, err := engine.Replace(lineStart, lineEnd, newText)
			if err != nil {
				return handler.Error(err)
			}
			affectedLines = append(affectedLines, uint32(line))
			if !global {
				replacements++
			} else {
				replacements += strings.Count(lineText, state.Pattern)
			}
		}
	}

	if replacements == 0 {
		return handler.NoOpWithMessage("replace: no matches found")
	}

	return handler.Success().
		WithRedrawLines(affectedLines...).
		WithMessage("replaced " + string(rune('0'+replacements)) + " occurrence(s)")
}

// wordUnderCursor returns the word at the current cursor position.
func (h *Handler) wordUnderCursor(ctx *execctx.ExecutionContext) (string, error) {
	if ctx.Cursors == nil {
		return "", execctx.ErrMissingCursors
	}

	engine := ctx.Engine
	text := engine.Text()
	offset := int(ctx.Cursors.Primary().Head)
	textLen := len(text)

	if offset >= textLen {
		return "", nil
	}

	// Find word boundaries
	start := offset
	for start > 0 && isWordChar(rune(text[start-1])) {
		start--
	}

	end := offset
	for end < textLen && isWordChar(rune(text[end])) {
		end++
	}

	if start == end {
		return "", nil
	}

	return text[start:end], nil
}

// compilePattern compiles a search pattern into a regex.
func compilePattern(pattern string, forward, caseSensitive bool) (*SearchState, error) {
	flags := ""
	if !caseSensitive {
		flags = "(?i)"
	}

	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, err
	}

	return &SearchState{
		Pattern:       pattern,
		Regex:         re,
		Forward:       forward,
		CaseSensitive: caseSensitive,
	}, nil
}

// getSearchState retrieves the search state from the context.
func getSearchState(ctx *execctx.ExecutionContext) *SearchState {
	v, ok := ctx.GetData(searchStateKey)
	if !ok {
		return nil
	}
	state, ok := v.(*SearchState)
	if !ok {
		return nil
	}
	return state
}

// isWordChar returns true if the rune is a word character.
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}
