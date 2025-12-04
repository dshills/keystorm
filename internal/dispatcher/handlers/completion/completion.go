// Package completion provides handlers for completion operations.
package completion

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for completion operations.
const (
	ActionTrigger       = "completion.trigger"       // Ctrl+Space - trigger completion
	ActionAccept        = "completion.accept"        // Tab/Enter - accept selected completion
	ActionAcceptWord    = "completion.acceptWord"    // Accept word part only
	ActionCancel        = "completion.cancel"        // Escape - cancel completion
	ActionNext          = "completion.next"          // Ctrl+N - next completion
	ActionPrev          = "completion.prev"          // Ctrl+P - previous completion
	ActionPageDown      = "completion.pageDown"      // Page down in completions
	ActionPageUp        = "completion.pageUp"        // Page up in completions
	ActionWordComplete  = "completion.wordComplete"  // Ctrl+N in insert - word completion
	ActionLineComplete  = "completion.lineComplete"  // Ctrl+X Ctrl+L - line completion
	ActionPathComplete  = "completion.pathComplete"  // Ctrl+X Ctrl+F - path completion
	ActionOmniComplete  = "completion.omniComplete"  // Ctrl+X Ctrl+O - omni completion (LSP)
	ActionSignatureHelp = "completion.signatureHelp" // Signature help
)

// CompletionKind indicates the type of completion item.
type CompletionKind int

const (
	KindText CompletionKind = iota
	KindMethod
	KindFunction
	KindConstructor
	KindField
	KindVariable
	KindClass
	KindInterface
	KindModule
	KindProperty
	KindUnit
	KindValue
	KindEnum
	KindKeyword
	KindSnippet
	KindColor
	KindFile
	KindReference
	KindFolder
	KindEnumMember
	KindConstant
	KindStruct
	KindEvent
	KindOperator
	KindTypeParameter
)

// CompletionItem represents a single completion suggestion.
type CompletionItem struct {
	// Label is the display text for the completion.
	Label string
	// Kind indicates the type of completion.
	Kind CompletionKind
	// Detail provides additional information.
	Detail string
	// Documentation provides extended documentation.
	Documentation string
	// InsertText is the text to insert (if different from Label).
	InsertText string
	// FilterText is the text to use for filtering.
	FilterText string
	// SortText is the text to use for sorting.
	SortText string
	// Preselect indicates this item should be selected by default.
	Preselect bool
	// TextEditRange indicates the range to replace (if any).
	TextEditRange *buffer.Range
}

// CompletionState holds the current completion session state.
type CompletionState struct {
	// Items holds the completion items.
	Items []CompletionItem
	// Selected is the currently selected index.
	Selected int
	// Prefix is the text prefix being completed.
	Prefix string
	// StartOffset is where the completion started.
	StartOffset buffer.ByteOffset
	// Active indicates if a completion session is active.
	Active bool
}

// CompletionProvider provides completion suggestions.
type CompletionProvider interface {
	// GetCompletions returns completion items at the given position.
	GetCompletions(ctx *execctx.ExecutionContext, offset buffer.ByteOffset) ([]CompletionItem, error)
	// GetWordCompletions returns word completions from the buffer.
	GetWordCompletions(ctx *execctx.ExecutionContext, prefix string) ([]CompletionItem, error)
	// GetLineCompletions returns line completions.
	GetLineCompletions(ctx *execctx.ExecutionContext, prefix string) ([]CompletionItem, error)
	// GetPathCompletions returns file path completions.
	GetPathCompletions(ctx *execctx.ExecutionContext, prefix string) ([]CompletionItem, error)
	// GetSignatureHelp returns signature help at the position.
	GetSignatureHelp(ctx *execctx.ExecutionContext, offset buffer.ByteOffset) (string, error)
}

const (
	completionStateKey    = "_completion_state"
	completionProviderKey = "_completion_provider"
)

// Handler implements namespace-based completion handling.
type Handler struct {
	provider CompletionProvider
}

// NewHandler creates a new completion handler.
func NewHandler() *Handler {
	return &Handler{}
}

// NewHandlerWithProvider creates a handler with a completion provider.
func NewHandlerWithProvider(p CompletionProvider) *Handler {
	return &Handler{provider: p}
}

// Namespace returns the completion namespace.
func (h *Handler) Namespace() string {
	return "completion"
}

// CanHandle returns true if this handler can process the action.
func (h *Handler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionTrigger, ActionAccept, ActionAcceptWord, ActionCancel,
		ActionNext, ActionPrev, ActionPageDown, ActionPageUp,
		ActionWordComplete, ActionLineComplete, ActionPathComplete,
		ActionOmniComplete, ActionSignatureHelp:
		return true
	}
	return false
}

// HandleAction processes a completion action.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	switch action.Name {
	case ActionTrigger:
		return h.trigger(ctx)
	case ActionAccept:
		return h.accept(ctx, false)
	case ActionAcceptWord:
		return h.accept(ctx, true)
	case ActionCancel:
		return h.cancel(ctx)
	case ActionNext:
		return h.navigate(ctx, 1)
	case ActionPrev:
		return h.navigate(ctx, -1)
	case ActionPageDown:
		return h.navigate(ctx, 10)
	case ActionPageUp:
		return h.navigate(ctx, -10)
	case ActionWordComplete:
		return h.wordComplete(ctx)
	case ActionLineComplete:
		return h.lineComplete(ctx)
	case ActionPathComplete:
		return h.pathComplete(ctx)
	case ActionOmniComplete:
		return h.omniComplete(ctx)
	case ActionSignatureHelp:
		return h.signatureHelp(ctx)
	default:
		return handler.Errorf("unknown completion action: %s", action.Name)
	}
}

// getProvider returns the completion provider.
func (h *Handler) getProvider(ctx *execctx.ExecutionContext) CompletionProvider {
	if h.provider != nil {
		return h.provider
	}
	if v, ok := ctx.GetData(completionProviderKey); ok {
		if p, ok := v.(CompletionProvider); ok {
			return p
		}
	}
	return nil
}

// getState returns the current completion state.
func (h *Handler) getState(ctx *execctx.ExecutionContext) *CompletionState {
	if v, ok := ctx.GetData(completionStateKey); ok {
		if state, ok := v.(*CompletionState); ok {
			return state
		}
	}
	return nil
}

// setState sets the completion state.
func (h *Handler) setState(ctx *execctx.ExecutionContext, state *CompletionState) {
	ctx.SetData(completionStateKey, state)
}

// trigger starts a completion session.
func (h *Handler) trigger(ctx *execctx.ExecutionContext) handler.Result {
	provider := h.getProvider(ctx)
	if provider == nil {
		return handler.NoOpWithMessage("completion: no provider")
	}

	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	offset := ctx.Cursors.Primary().Head
	items, err := provider.GetCompletions(ctx, offset)
	if err != nil {
		return handler.Error(err)
	}

	if len(items) == 0 {
		return handler.NoOpWithMessage("completion: no completions")
	}

	// Find the prefix (word before cursor)
	prefix := ""
	startOffset := offset
	if ctx.Engine != nil {
		text := ctx.Engine.Text()
		start := int(offset)
		for start > 0 && isWordChar(rune(text[start-1])) {
			start--
		}
		prefix = text[start:offset]
		startOffset = buffer.ByteOffset(start)
	}

	state := &CompletionState{
		Items:       items,
		Selected:    findPreselected(items),
		Prefix:      prefix,
		StartOffset: startOffset,
		Active:      true,
	}

	h.setState(ctx, state)

	return handler.Success().
		WithData("completions", items).
		WithData("selected", state.Selected).
		WithMessage("completion: " + itoa(len(items)) + " items")
}

// accept accepts the currently selected completion.
func (h *Handler) accept(ctx *execctx.ExecutionContext, wordOnly bool) handler.Result {
	state := h.getState(ctx)
	if state == nil || !state.Active || len(state.Items) == 0 {
		return handler.NoOp()
	}

	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	selected := state.Items[state.Selected]
	insertText := selected.InsertText
	if insertText == "" {
		insertText = selected.Label
	}

	if wordOnly {
		// Only insert up to the first non-word character
		for i, r := range insertText {
			if !isWordChar(r) {
				insertText = insertText[:i]
				break
			}
		}
	}

	// Replace the prefix with the completion
	startOffset := state.StartOffset
	endOffset := ctx.Cursors.Primary().Head

	if selected.TextEditRange != nil {
		startOffset = selected.TextEditRange.Start
		endOffset = selected.TextEditRange.End
	}

	_, err := ctx.Engine.Replace(startOffset, endOffset, insertText)
	if err != nil {
		return handler.Error(err)
	}

	// Move cursor to end of inserted text
	newOffset := startOffset + buffer.ByteOffset(len(insertText))
	sel := ctx.Cursors.Primary().MoveTo(newOffset)
	ctx.Cursors.SetPrimary(sel)

	// Clear completion state
	h.setState(ctx, nil)

	return handler.Success().WithRedraw()
}

// cancel cancels the completion session.
func (h *Handler) cancel(ctx *execctx.ExecutionContext) handler.Result {
	state := h.getState(ctx)
	if state == nil || !state.Active {
		return handler.NoOp()
	}

	h.setState(ctx, nil)
	return handler.Success().WithData("completionCancelled", true)
}

// navigate moves the selection in the completion list.
func (h *Handler) navigate(ctx *execctx.ExecutionContext, delta int) handler.Result {
	state := h.getState(ctx)
	if state == nil || !state.Active || len(state.Items) == 0 {
		return handler.NoOp()
	}

	state.Selected += delta
	if state.Selected < 0 {
		state.Selected = len(state.Items) - 1
	} else if state.Selected >= len(state.Items) {
		state.Selected = 0
	}

	return handler.Success().
		WithData("completions", state.Items).
		WithData("selected", state.Selected)
}

// wordComplete triggers word completion from buffer content.
func (h *Handler) wordComplete(ctx *execctx.ExecutionContext) handler.Result {
	provider := h.getProvider(ctx)
	if provider == nil {
		// Fall back to simple word completion
		return h.simpleWordComplete(ctx)
	}

	prefix := h.getCurrentWordPrefix(ctx)
	if prefix == "" {
		return handler.NoOp()
	}

	items, err := provider.GetWordCompletions(ctx, prefix)
	if err != nil {
		return handler.Error(err)
	}

	return h.setupCompletionState(ctx, items, prefix)
}

// lineComplete triggers line completion.
func (h *Handler) lineComplete(ctx *execctx.ExecutionContext) handler.Result {
	provider := h.getProvider(ctx)
	if provider == nil {
		return handler.NoOpWithMessage("completion: no provider")
	}

	prefix := h.getCurrentLinePrefix(ctx)
	items, err := provider.GetLineCompletions(ctx, prefix)
	if err != nil {
		return handler.Error(err)
	}

	return h.setupCompletionState(ctx, items, prefix)
}

// pathComplete triggers path completion.
func (h *Handler) pathComplete(ctx *execctx.ExecutionContext) handler.Result {
	provider := h.getProvider(ctx)
	if provider == nil {
		return handler.NoOpWithMessage("completion: no provider")
	}

	prefix := h.getCurrentWordPrefix(ctx)
	items, err := provider.GetPathCompletions(ctx, prefix)
	if err != nil {
		return handler.Error(err)
	}

	return h.setupCompletionState(ctx, items, prefix)
}

// omniComplete triggers LSP-based completion.
func (h *Handler) omniComplete(ctx *execctx.ExecutionContext) handler.Result {
	return h.trigger(ctx)
}

// signatureHelp shows function signature help.
func (h *Handler) signatureHelp(ctx *execctx.ExecutionContext) handler.Result {
	provider := h.getProvider(ctx)
	if provider == nil {
		return handler.NoOpWithMessage("completion: no provider")
	}

	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	offset := ctx.Cursors.Primary().Head
	sig, err := provider.GetSignatureHelp(ctx, offset)
	if err != nil {
		return handler.Error(err)
	}

	if sig == "" {
		return handler.NoOpWithMessage("completion: no signature help")
	}

	return handler.Success().
		WithData("signatureHelp", sig).
		WithMessage(sig)
}

// simpleWordComplete does basic word completion from buffer.
func (h *Handler) simpleWordComplete(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	prefix := h.getCurrentWordPrefix(ctx)
	if prefix == "" {
		return handler.NoOp()
	}

	// Find all words in the buffer that match the prefix
	text := ctx.Engine.Text()
	words := findWordsWithPrefix(text, prefix)

	if len(words) == 0 {
		return handler.NoOpWithMessage("completion: no matches")
	}

	items := make([]CompletionItem, len(words))
	for i, w := range words {
		items[i] = CompletionItem{
			Label: w,
			Kind:  KindText,
		}
	}

	return h.setupCompletionState(ctx, items, prefix)
}

// setupCompletionState creates a completion state from items.
func (h *Handler) setupCompletionState(ctx *execctx.ExecutionContext, items []CompletionItem, prefix string) handler.Result {
	if len(items) == 0 {
		return handler.NoOpWithMessage("completion: no completions")
	}

	startOffset := buffer.ByteOffset(0)
	if ctx.Cursors != nil {
		startOffset = ctx.Cursors.Primary().Head - buffer.ByteOffset(len(prefix))
	}

	state := &CompletionState{
		Items:       items,
		Selected:    0,
		Prefix:      prefix,
		StartOffset: startOffset,
		Active:      true,
	}

	h.setState(ctx, state)

	return handler.Success().
		WithData("completions", items).
		WithData("selected", 0).
		WithMessage("completion: " + itoa(len(items)) + " items")
}

// getCurrentWordPrefix gets the word before the cursor.
func (h *Handler) getCurrentWordPrefix(ctx *execctx.ExecutionContext) string {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return ""
	}

	text := ctx.Engine.Text()
	offset := int(ctx.Cursors.Primary().Head)

	start := offset
	for start > 0 && isWordChar(rune(text[start-1])) {
		start--
	}

	if start == offset {
		return ""
	}

	return text[start:offset]
}

// getCurrentLinePrefix gets the text from line start to cursor.
func (h *Handler) getCurrentLinePrefix(ctx *execctx.ExecutionContext) string {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return ""
	}

	offset := ctx.Cursors.Primary().Head
	point := ctx.Engine.OffsetToPoint(offset)
	lineStart := ctx.Engine.LineStartOffset(point.Line)

	return ctx.Engine.TextRange(lineStart, offset)
}

// findWordsWithPrefix finds all words in text that start with prefix.
func findWordsWithPrefix(text, prefix string) []string {
	seen := make(map[string]bool)
	var words []string

	i := 0
	for i < len(text) {
		// Skip non-word characters
		for i < len(text) && !isWordChar(rune(text[i])) {
			i++
		}

		// Collect word
		start := i
		for i < len(text) && isWordChar(rune(text[i])) {
			i++
		}

		if start < i {
			word := text[start:i]
			if len(word) > len(prefix) && hasPrefix(word, prefix) && !seen[word] {
				seen[word] = true
				words = append(words, word)
			}
		}
	}

	return words
}

// hasPrefix checks if s starts with prefix (case-insensitive).
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if toLower(s[i]) != toLower(prefix[i]) {
			return false
		}
	}
	return true
}

// toLower converts a byte to lowercase.
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// isWordChar returns true if the rune is a word character.
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

// findPreselected returns the index of the preselected item, or 0.
func findPreselected(items []CompletionItem) int {
	for i, item := range items {
		if item.Preselect {
			return i
		}
	}
	return 0
}

// itoa converts an int to string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
