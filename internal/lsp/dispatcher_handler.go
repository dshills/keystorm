package lsp

import (
	"context"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for LSP operations.
const (
	// Code navigation
	ActionGotoDefinition     = "lsp.gotoDefinition"
	ActionGotoTypeDefinition = "lsp.gotoTypeDefinition"
	ActionGotoImplementation = "lsp.gotoImplementation"
	ActionFindReferences     = "lsp.findReferences"

	// Code intelligence
	ActionHover         = "lsp.hover"
	ActionCompletion    = "lsp.completion"
	ActionSignatureHelp = "lsp.signatureHelp"

	// Symbols
	ActionDocumentSymbols  = "lsp.documentSymbols"
	ActionWorkspaceSymbols = "lsp.workspaceSymbols"

	// Code actions
	ActionCodeAction    = "lsp.codeAction"
	ActionApplyCodeEdit = "lsp.applyCodeEdit"

	// Formatting
	ActionFormat          = "lsp.format"
	ActionFormatRange     = "lsp.formatRange"
	ActionFormatOnType    = "lsp.formatOnType"
	ActionOrganizeImports = "lsp.organizeImports"

	// Refactoring
	ActionRename          = "lsp.rename"
	ActionPrepareRename   = "lsp.prepareRename"
	ActionExtractVariable = "lsp.extractVariable"
	ActionExtractFunction = "lsp.extractFunction"

	// Diagnostics
	ActionNextDiagnostic = "lsp.nextDiagnostic"
	ActionPrevDiagnostic = "lsp.prevDiagnostic"
	ActionShowDiagnostic = "lsp.showDiagnostic"

	// Server management
	ActionRestartServer = "lsp.restartServer"
	ActionServerStatus  = "lsp.serverStatus"
)

// Handler provides LSP operations as dispatcher actions.
// It implements the dispatcher's NamespaceHandler interface for the "lsp" namespace.
//
// Handler is NOT safe for concurrent use. SetClient should only be called
// during initialization or when the handler is not being used by the dispatcher.
// The actions map is populated once at construction and never modified afterward.
type Handler struct {
	client         *Client
	requestTimeout time.Duration

	// Actions registered by name (immutable after construction)
	actions map[string]func(action input.Action, ctx *execctx.ExecutionContext) handler.Result
}

// HandlerOption configures the LSP handler.
type HandlerOption func(*Handler)

// WithLSPClient sets the LSP client for the handler.
func WithLSPClient(client *Client) HandlerOption {
	return func(h *Handler) {
		h.client = client
	}
}

// WithHandlerTimeout sets the timeout for LSP requests.
func WithHandlerTimeout(timeout time.Duration) HandlerOption {
	return func(h *Handler) {
		h.requestTimeout = timeout
	}
}

// NewHandler creates a new LSP dispatcher handler.
func NewHandler(opts ...HandlerOption) *Handler {
	h := &Handler{
		requestTimeout: 5 * time.Second,
		actions:        make(map[string]func(action input.Action, ctx *execctx.ExecutionContext) handler.Result),
	}

	for _, opt := range opts {
		opt(h)
	}

	// Register all actions
	h.registerActions()

	return h
}

// SetClient sets or updates the LSP client.
func (h *Handler) SetClient(client *Client) {
	h.client = client
}

// Namespace implements handler.NamespaceHandler.
func (h *Handler) Namespace() string {
	return "lsp"
}

// CanHandle implements handler.NamespaceHandler.
func (h *Handler) CanHandle(actionName string) bool {
	_, ok := h.actions[actionName]
	return ok
}

// HandleAction implements handler.NamespaceHandler.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	fn, ok := h.actions[action.Name]
	if !ok {
		return handler.Errorf("unknown LSP action: %s", action.Name)
	}
	return fn(action, ctx)
}

// registerActions registers all LSP action handlers.
func (h *Handler) registerActions() {
	// Navigation
	h.actions[ActionGotoDefinition] = h.handleGotoDefinition
	h.actions[ActionGotoTypeDefinition] = h.handleGotoTypeDefinition
	h.actions[ActionGotoImplementation] = h.handleGotoImplementation
	h.actions[ActionFindReferences] = h.handleFindReferences

	// Intelligence
	h.actions[ActionHover] = h.handleHover
	h.actions[ActionCompletion] = h.handleCompletion
	h.actions[ActionSignatureHelp] = h.handleSignatureHelp

	// Symbols
	h.actions[ActionDocumentSymbols] = h.handleDocumentSymbols
	h.actions[ActionWorkspaceSymbols] = h.handleWorkspaceSymbols

	// Code actions
	h.actions[ActionCodeAction] = h.handleCodeAction
	h.actions[ActionApplyCodeEdit] = h.handleApplyCodeEdit

	// Formatting
	h.actions[ActionFormat] = h.handleFormat
	h.actions[ActionFormatRange] = h.handleFormatRange
	h.actions[ActionFormatOnType] = h.handleFormatOnType
	h.actions[ActionOrganizeImports] = h.handleOrganizeImports

	// Refactoring
	h.actions[ActionRename] = h.handleRename
	h.actions[ActionPrepareRename] = h.handlePrepareRename
	h.actions[ActionExtractVariable] = h.handleExtractVariable
	h.actions[ActionExtractFunction] = h.handleExtractFunction

	// Diagnostics
	h.actions[ActionNextDiagnostic] = h.handleNextDiagnostic
	h.actions[ActionPrevDiagnostic] = h.handlePrevDiagnostic
	h.actions[ActionShowDiagnostic] = h.handleShowDiagnostic

	// Server management
	h.actions[ActionRestartServer] = h.handleRestartServer
	h.actions[ActionServerStatus] = h.handleServerStatus
}

// --- Helper Methods ---

// getContext creates a context with timeout.
func (h *Handler) getContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), h.requestTimeout)
}

// ensureClient checks that a client is configured.
func (h *Handler) ensureClient() error {
	if h.client == nil {
		return ErrNotStarted
	}
	return nil
}

// getPositionFromContext extracts cursor position from execution context.
// Returns zero position if cursors are nil, empty, or engine is unavailable.
func (h *Handler) getPositionFromContext(ctx *execctx.ExecutionContext) Position {
	if ctx.Cursors == nil || ctx.Cursors.Count() == 0 {
		return Position{}
	}

	if ctx.Engine == nil {
		return Position{}
	}

	primary := ctx.Cursors.Primary()
	point := ctx.Engine.OffsetToPoint(buffer.ByteOffset(primary.Start()))
	return Position{
		Line:      int(point.Line),
		Character: int(point.Column),
	}
}

// getFilePath extracts the file path from execution context.
func (h *Handler) getFilePath(ctx *execctx.ExecutionContext) string {
	return ctx.FilePath
}

// positionToOffset converts an LSP Position to a byte offset.
func (h *Handler) positionToOffset(ctx *execctx.ExecutionContext, pos Position) buffer.ByteOffset {
	if ctx.Engine == nil {
		return 0
	}
	return ctx.Engine.PointToOffset(positionToPoint(pos))
}

// navigationResultToHandler converts a NavigationResult to a handler result.
func (h *Handler) navigationResultToHandler(result *NavigationResult) handler.Result {
	if result == nil || len(result.Locations) == 0 {
		return handler.NoOpWithMessage("no results found")
	}

	if len(result.Locations) == 1 {
		loc := result.Locations[0]
		return handler.Success().
			WithScrollTo(uint32(loc.Range.Start.Line), uint32(loc.Range.Start.Character), true).
			WithData("location", loc)
	}

	// Multiple results - return data for picker UI
	return handler.Success().
		WithMessage("multiple results").
		WithData("locations", result.Locations).
		WithData("count", len(result.Locations))
}

// --- Navigation Handlers ---

func (h *Handler) handleGotoDefinition(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	result, err := h.client.GoToDefinition(reqCtx, path, pos)
	if err != nil {
		return handler.Error(err)
	}

	return h.navigationResultToHandler(result)
}

func (h *Handler) handleGotoTypeDefinition(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	result, err := h.client.GoToTypeDefinition(reqCtx, path, pos)
	if err != nil {
		return handler.Error(err)
	}

	return h.navigationResultToHandler(result)
}

func (h *Handler) handleGotoImplementation(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	result, err := h.client.GoToImplementation(reqCtx, path, pos)
	if err != nil {
		return handler.Error(err)
	}

	return h.navigationResultToHandler(result)
}

func (h *Handler) handleFindReferences(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	result, err := h.client.FindReferences(reqCtx, path, pos)
	if err != nil {
		return handler.Error(err)
	}

	return h.navigationResultToHandler(result)
}

// --- Intelligence Handlers ---

func (h *Handler) handleHover(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	hover, err := h.client.Hover(reqCtx, path, pos)
	if err != nil {
		return handler.Error(err)
	}

	if hover == nil || hover.Contents.Value == "" {
		return handler.NoOpWithMessage("no hover information")
	}

	return handler.Success().
		WithMessage(hover.Contents.Value).
		WithData("hover", hover)
}

func (h *Handler) handleCompletion(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	// Get prefix from action args (optional)
	prefix := action.Args.GetString("prefix")

	// Check for trigger character - use CompleteWithTrigger if present
	triggerChar := action.Args.GetString("triggerCharacter")
	var result *CompletionResult
	var err error
	if triggerChar != "" {
		result, err = h.client.CompleteWithTrigger(reqCtx, path, pos, triggerChar)
	} else {
		result, err = h.client.Complete(reqCtx, path, pos, prefix)
	}
	if err != nil {
		return handler.Error(err)
	}

	if result == nil || len(result.Items) == 0 {
		return handler.NoOpWithMessage("no completions")
	}

	return handler.Success().
		WithMessage("completion").
		WithData("completion", result).
		WithData("count", len(result.Items))
}

func (h *Handler) handleSignatureHelp(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	result, err := h.client.SignatureHelp(reqCtx, path, pos)
	if err != nil {
		return handler.Error(err)
	}

	if result == nil || !result.HasActiveSignature {
		return handler.NoOpWithMessage("no signature help")
	}

	return handler.Success().
		WithData("signatureHelp", result)
}

// --- Symbol Handlers ---

func (h *Handler) handleDocumentSymbols(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)

	symbols, err := h.client.DocumentSymbols(reqCtx, path)
	if err != nil {
		return handler.Error(err)
	}

	if len(symbols) == 0 {
		return handler.NoOpWithMessage("no symbols found")
	}

	return handler.Success().
		WithData("symbols", symbols).
		WithData("count", len(symbols))
}

func (h *Handler) handleWorkspaceSymbols(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	// Get query from action args
	query := action.Args.GetString("query")

	// Get languageID from action args or detect from file
	languageID := action.Args.GetString("language")
	if languageID == "" {
		languageID = DetectLanguageID(h.getFilePath(ctx))
	}

	symbols, err := h.client.WorkspaceSymbols(reqCtx, query, languageID)
	if err != nil {
		return handler.Error(err)
	}

	if len(symbols) == 0 {
		return handler.NoOpWithMessage("no symbols found")
	}

	return handler.Success().
		WithData("symbols", symbols).
		WithData("count", len(symbols))
}

// --- Code Action Handlers ---

func (h *Handler) handleCodeAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	// Build range from cursor position or selection
	r := Range{Start: pos, End: pos}
	if ctx.Cursors != nil && ctx.Cursors.HasSelection() {
		primary := ctx.Cursors.Primary()
		if ctx.Engine != nil {
			startPoint := ctx.Engine.OffsetToPoint(buffer.ByteOffset(primary.Start()))
			endPoint := ctx.Engine.OffsetToPoint(buffer.ByteOffset(primary.End()))
			r.Start = Position{Line: int(startPoint.Line), Character: int(startPoint.Column)}
			r.End = Position{Line: int(endPoint.Line), Character: int(endPoint.Column)}
		}
	}

	// Get diagnostics from action args if available
	var diags []Diagnostic
	if v, ok := action.Args.Get("diagnostics"); ok {
		if d, ok := v.([]Diagnostic); ok {
			diags = d
		}
	}

	result, err := h.client.CodeActions(reqCtx, path, r, diags)
	if err != nil {
		return handler.Error(err)
	}

	if result == nil || len(result.All) == 0 {
		return handler.NoOpWithMessage("no code actions available")
	}

	return handler.Success().
		WithData("codeActions", result).
		WithData("count", result.TotalCount)
}

func (h *Handler) handleApplyCodeEdit(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// This action applies a workspace edit received from a code action.
	// The edit should be provided in action.Args.Extra["edit"].
	//
	// LIMITATION: Currently only applies edits to the current file.
	// Multi-file workspace edits (e.g., from rename operations that affect
	// multiple files) require coordination with the editor's buffer management
	// system and are not yet supported. The caller should check the
	// workspaceEdit.Changes map for additional files and handle them separately.
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	editVal, ok := action.Args.Get("edit")
	if !ok {
		return handler.Errorf("no edit provided in action args")
	}

	edit, ok := editVal.(WorkspaceEdit)
	if !ok {
		return handler.Errorf("invalid edit type in action args")
	}

	if ctx.Engine == nil {
		return handler.Errorf("no engine available")
	}

	path := h.getFilePath(ctx)
	uri := FilePathToURI(path)

	changes, hasChanges := edit.Changes[uri]
	if !hasChanges || len(changes) == 0 {
		// Check if there are changes for other files (multi-file edit)
		if len(edit.Changes) > 0 {
			return handler.NoOpWithMessage("workspace edit affects other files; multi-file edits not yet supported")
		}
		return handler.NoOpWithMessage("no changes for current file")
	}

	// Apply edits in reverse order (to preserve positions)
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		startOffset := h.positionToOffset(ctx, change.Range.Start)
		endOffset := h.positionToOffset(ctx, change.Range.End)

		_, err := ctx.Engine.Replace(
			startOffset,
			endOffset,
			change.NewText,
		)
		if err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithRedraw()
}

// --- Formatting Handlers ---

func (h *Handler) handleFormat(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)

	result, err := h.client.Format(reqCtx, path)
	if err != nil {
		return handler.Error(err)
	}

	if result == nil || result.Skipped || len(result.Edits) == 0 {
		if result != nil && result.Skipped {
			return handler.NoOpWithMessage(result.SkipReason)
		}
		return handler.NoOpWithMessage("no formatting changes")
	}

	// Apply edits
	return h.applyTextEdits(ctx, result.Edits)
}

func (h *Handler) handleFormatRange(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	// Get range from selection or action data
	r := Range{Start: pos, End: pos}
	if ctx.Cursors != nil && ctx.Cursors.HasSelection() {
		primary := ctx.Cursors.Primary()
		if ctx.Engine != nil {
			startPoint := ctx.Engine.OffsetToPoint(buffer.ByteOffset(primary.Start()))
			endPoint := ctx.Engine.OffsetToPoint(buffer.ByteOffset(primary.End()))
			r.Start = Position{Line: int(startPoint.Line), Character: int(startPoint.Column)}
			r.End = Position{Line: int(endPoint.Line), Character: int(endPoint.Column)}
		}
	}

	result, err := h.client.FormatRange(reqCtx, path, r)
	if err != nil {
		return handler.Error(err)
	}

	if result == nil || result.Skipped || len(result.Edits) == 0 {
		if result != nil && result.Skipped {
			return handler.NoOpWithMessage(result.SkipReason)
		}
		return handler.NoOpWithMessage("no formatting changes")
	}

	return h.applyTextEdits(ctx, result.Edits)
}

func (h *Handler) handleFormatOnType(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}
	// FormatOnType is not yet implemented at the Client level
	// This handler serves as a placeholder for future implementation
	return handler.NoOpWithMessage("format on type not yet available")
}

func (h *Handler) handleOrganizeImports(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)

	// Use the dedicated OrganizeImports method
	ca, err := h.client.OrganizeImports(reqCtx, path)
	if err != nil {
		return handler.Error(err)
	}

	if ca == nil {
		return handler.NoOpWithMessage("organize imports not available")
	}

	// Apply the code action edit
	if ca.Edit != nil {
		return h.handleApplyCodeEdit(input.Action{
			Args: input.ActionArgs{
				Extra: map[string]interface{}{"edit": *ca.Edit},
			},
		}, ctx)
	}

	return handler.NoOpWithMessage("no edits from organize imports")
}

// applyTextEdits applies a list of text edits to the engine.
func (h *Handler) applyTextEdits(ctx *execctx.ExecutionContext, edits []TextEdit) handler.Result {
	if ctx.Engine == nil {
		return handler.Errorf("no engine available")
	}

	// Apply edits in reverse order to preserve positions
	for i := len(edits) - 1; i >= 0; i-- {
		edit := edits[i]
		startOffset := h.positionToOffset(ctx, edit.Range.Start)
		endOffset := h.positionToOffset(ctx, edit.Range.End)

		_, err := ctx.Engine.Replace(
			startOffset,
			endOffset,
			edit.NewText,
		)
		if err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithRedraw()
}

// --- Refactoring Handlers ---

func (h *Handler) handleRename(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	// Get new name from action args
	newName := action.Args.GetString("newName")
	if newName == "" {
		return handler.Errorf("no new name provided")
	}

	result, err := h.client.Rename(reqCtx, path, pos, newName)
	if err != nil {
		return handler.Error(err)
	}

	if result == nil || result.Edit == nil {
		return handler.NoOpWithMessage("rename failed: no edits returned")
	}

	// Return the edit for the caller to apply
	return handler.Success().
		WithData("renameResult", result).
		WithData("workspaceEdit", result.Edit).
		WithData("affectedFiles", result.AffectedFiles).
		WithMessage("rename ready")
}

func (h *Handler) handlePrepareRename(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	rng, placeholder, err := h.client.PrepareRename(reqCtx, path, pos)
	if err != nil {
		return handler.Error(err)
	}

	if rng == nil {
		return handler.NoOpWithMessage("rename not available here")
	}

	return handler.Success().
		WithData("range", rng).
		WithData("placeholder", placeholder)
}

func (h *Handler) handleExtractVariable(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Extract variable is typically a code action - use Refactorings method
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	r := h.getSelectionRange(ctx)

	refactorings, err := h.client.Refactorings(reqCtx, path, r)
	if err != nil {
		return handler.Error(err)
	}

	// Find extract variable action
	for _, ca := range refactorings {
		if ca.Title == "Extract variable" || ca.Kind == CodeActionKindRefactorExtract {
			if ca.Edit != nil {
				return h.handleApplyCodeEdit(input.Action{
					Args: input.ActionArgs{
						Extra: map[string]interface{}{"edit": *ca.Edit},
					},
				}, ctx)
			}
		}
	}

	return handler.NoOpWithMessage("extract variable not available")
}

func (h *Handler) handleExtractFunction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Extract function is typically a code action - use Refactorings method
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	path := h.getFilePath(ctx)
	r := h.getSelectionRange(ctx)

	refactorings, err := h.client.Refactorings(reqCtx, path, r)
	if err != nil {
		return handler.Error(err)
	}

	// Find extract function action
	for _, ca := range refactorings {
		if ca.Title == "Extract function" || ca.Title == "Extract method" {
			if ca.Edit != nil {
				return h.handleApplyCodeEdit(input.Action{
					Args: input.ActionArgs{
						Extra: map[string]interface{}{"edit": *ca.Edit},
					},
				}, ctx)
			}
		}
	}

	return handler.NoOpWithMessage("extract function not available")
}

// getSelectionRange returns the selection range from context.
func (h *Handler) getSelectionRange(ctx *execctx.ExecutionContext) Range {
	pos := h.getPositionFromContext(ctx)
	r := Range{Start: pos, End: pos}

	if ctx.Cursors != nil && ctx.Cursors.HasSelection() {
		primary := ctx.Cursors.Primary()
		if ctx.Engine != nil {
			startPoint := ctx.Engine.OffsetToPoint(buffer.ByteOffset(primary.Start()))
			endPoint := ctx.Engine.OffsetToPoint(buffer.ByteOffset(primary.End()))
			r.Start = Position{Line: int(startPoint.Line), Character: int(startPoint.Column)}
			r.End = Position{Line: int(endPoint.Line), Character: int(endPoint.Column)}
		}
	}

	return r
}

// --- Diagnostics Handlers ---

func (h *Handler) handleNextDiagnostic(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	diags := h.client.Diagnostics(path)
	if len(diags) == 0 {
		return handler.NoOpWithMessage("no diagnostics")
	}

	// Find next diagnostic after current position
	for _, diag := range diags {
		if diag.Range.Start.Line > pos.Line ||
			(diag.Range.Start.Line == pos.Line && diag.Range.Start.Character > pos.Character) {
			return handler.Success().
				WithScrollTo(uint32(diag.Range.Start.Line), uint32(diag.Range.Start.Character), true).
				WithData("diagnostic", diag).
				WithMessage(diag.Message)
		}
	}

	// Wrap around to first diagnostic
	first := diags[0]
	return handler.Success().
		WithScrollTo(uint32(first.Range.Start.Line), uint32(first.Range.Start.Character), true).
		WithData("diagnostic", first).
		WithMessage(first.Message)
}

func (h *Handler) handlePrevDiagnostic(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	diags := h.client.Diagnostics(path)
	if len(diags) == 0 {
		return handler.NoOpWithMessage("no diagnostics")
	}

	// Find previous diagnostic before current position
	for i := len(diags) - 1; i >= 0; i-- {
		diag := diags[i]
		if diag.Range.Start.Line < pos.Line ||
			(diag.Range.Start.Line == pos.Line && diag.Range.Start.Character < pos.Character) {
			return handler.Success().
				WithScrollTo(uint32(diag.Range.Start.Line), uint32(diag.Range.Start.Character), true).
				WithData("diagnostic", diag).
				WithMessage(diag.Message)
		}
	}

	// Wrap around to last diagnostic
	last := diags[len(diags)-1]
	return handler.Success().
		WithScrollTo(uint32(last.Range.Start.Line), uint32(last.Range.Start.Character), true).
		WithData("diagnostic", last).
		WithMessage(last.Message)
}

func (h *Handler) handleShowDiagnostic(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	path := h.getFilePath(ctx)
	pos := h.getPositionFromContext(ctx)

	diags := h.client.Diagnostics(path)
	if len(diags) == 0 {
		return handler.NoOpWithMessage("no diagnostics")
	}

	// Find diagnostic at current position
	for _, diag := range diags {
		if pos.Line >= diag.Range.Start.Line && pos.Line <= diag.Range.End.Line {
			if pos.Line > diag.Range.Start.Line || pos.Character >= diag.Range.Start.Character {
				if pos.Line < diag.Range.End.Line || pos.Character <= diag.Range.End.Character {
					return handler.Success().
						WithData("diagnostic", diag).
						WithMessage(diag.Message)
				}
			}
		}
	}

	return handler.NoOpWithMessage("no diagnostic at cursor")
}

// --- Server Management Handlers ---

func (h *Handler) handleRestartServer(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	// Get language from action args or detect from file
	languageID := action.Args.GetString("language")
	if languageID == "" {
		languageID = DetectLanguageID(h.getFilePath(ctx))
	}

	if languageID == "" {
		return handler.Errorf("could not determine language")
	}

	reqCtx, cancel := h.getContext()
	defer cancel()

	if err := h.client.RestartServer(reqCtx, languageID); err != nil {
		return handler.Error(err)
	}

	return handler.SuccessWithMessage("server restarted: " + languageID)
}

func (h *Handler) handleServerStatus(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := h.ensureClient(); err != nil {
		return handler.Error(err)
	}

	// Get language from action args or detect from file
	languageID := action.Args.GetString("language")
	if languageID == "" {
		languageID = DetectLanguageID(h.getFilePath(ctx))
	}

	if languageID == "" {
		return handler.Errorf("could not determine language")
	}

	status := h.client.ServerStatus(languageID)

	return handler.Success().
		WithData("status", status.String()).
		WithData("language", languageID).
		WithMessage(languageID + ": " + status.String())
}

// --- Utility Functions ---

// positionToPoint converts an LSP Position to an engine Point.
func positionToPoint(pos Position) buffer.Point {
	return buffer.Point{
		Line:   uint32(pos.Line),
		Column: uint32(pos.Character),
	}
}

// ListActions returns all supported LSP action names.
func ListActions() []string {
	return []string{
		// Navigation
		ActionGotoDefinition,
		ActionGotoTypeDefinition,
		ActionGotoImplementation,
		ActionFindReferences,
		// Intelligence
		ActionHover,
		ActionCompletion,
		ActionSignatureHelp,
		// Symbols
		ActionDocumentSymbols,
		ActionWorkspaceSymbols,
		// Code actions
		ActionCodeAction,
		ActionApplyCodeEdit,
		// Formatting
		ActionFormat,
		ActionFormatRange,
		ActionFormatOnType,
		ActionOrganizeImports,
		// Refactoring
		ActionRename,
		ActionPrepareRename,
		ActionExtractVariable,
		ActionExtractFunction,
		// Diagnostics
		ActionNextDiagnostic,
		ActionPrevDiagnostic,
		ActionShowDiagnostic,
		// Server management
		ActionRestartServer,
		ActionServerStatus,
	}
}
