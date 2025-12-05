package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ActionsService provides high-level code actions, formatting, and refactoring features.
// It wraps the basic Server methods with filtering, caching, and convenient query methods.
type ActionsService struct {
	mu      sync.RWMutex
	manager *Manager

	// Formatting settings
	formatOnSave   bool
	formatOnType   bool
	formatOptions  FormattingOptions
	formatExcludes []string // glob patterns for files to exclude from formatting

	// Code action settings
	codeActionKinds []CodeActionKind // kinds to include (empty = all)

	// Rename settings
	renameConfirmation bool // whether to preview rename before applying

	// Signature help tracking
	activeSignature *signatureState

	// Cache for code actions
	codeActionCache    map[actionCacheKey]*actionCacheEntry
	codeActionCacheAge int64 // seconds
}

// actionCacheKey identifies a cache entry by file and position/range.
type actionCacheKey struct {
	path      string
	startLine int
	startChar int
	endLine   int
	endChar   int
}

// actionCacheEntry stores cached code actions.
type actionCacheEntry struct {
	actions   []CodeAction
	timestamp int64
}

// signatureState tracks active signature help state.
type signatureState struct {
	path            string
	pos             Position
	help            *SignatureHelp
	activeSignature int
	activeParameter int
	timestamp       int64
}

// ActionsOption configures the ActionsService.
type ActionsOption func(*ActionsService)

// WithFormatOnSave enables/disables format on save.
func WithFormatOnSave(enable bool) ActionsOption {
	return func(as *ActionsService) {
		as.formatOnSave = enable
	}
}

// WithFormatOnType enables/disables format on type.
func WithFormatOnType(enable bool) ActionsOption {
	return func(as *ActionsService) {
		as.formatOnType = enable
	}
}

// WithFormattingOptions sets default formatting options.
func WithFormattingOptions(opts FormattingOptions) ActionsOption {
	return func(as *ActionsService) {
		as.formatOptions = opts
	}
}

// WithFormatExcludes sets glob patterns for files to exclude from formatting.
func WithFormatExcludes(patterns []string) ActionsOption {
	return func(as *ActionsService) {
		as.formatExcludes = patterns
	}
}

// WithCodeActionKinds sets which code action kinds to include.
func WithCodeActionKinds(kinds []CodeActionKind) ActionsOption {
	return func(as *ActionsService) {
		as.codeActionKinds = kinds
	}
}

// WithRenameConfirmation enables/disables rename preview confirmation.
func WithRenameConfirmation(enable bool) ActionsOption {
	return func(as *ActionsService) {
		as.renameConfirmation = enable
	}
}

// WithCodeActionCacheAge sets the code action cache max age in seconds.
func WithCodeActionCacheAge(seconds int64) ActionsOption {
	return func(as *ActionsService) {
		as.codeActionCacheAge = seconds
	}
}

// NewActionsService creates a new actions service.
func NewActionsService(manager *Manager, opts ...ActionsOption) *ActionsService {
	as := &ActionsService{
		manager:            manager,
		formatOnSave:       false,
		formatOnType:       false,
		formatOptions:      DefaultFormattingOptions(),
		formatExcludes:     nil,
		codeActionKinds:    nil, // all kinds
		renameConfirmation: true,
		codeActionCache:    make(map[actionCacheKey]*actionCacheEntry),
		codeActionCacheAge: 10, // 10 seconds
	}

	for _, opt := range opts {
		opt(as)
	}

	return as
}

// DefaultFormattingOptions returns sensible default formatting options.
func DefaultFormattingOptions() FormattingOptions {
	return FormattingOptions{
		TabSize:                4,
		InsertSpaces:           false, // tabs by default
		TrimTrailingWhitespace: true,
		InsertFinalNewline:     true,
		TrimFinalNewlines:      true,
	}
}

// --- Code Actions ---

// CodeActionResult contains categorized code actions.
type CodeActionResult struct {
	// All actions returned
	All []CodeAction

	// Actions grouped by kind
	QuickFixes   []CodeAction
	Refactors    []CodeAction
	SourceFixes  []CodeAction
	OtherActions []CodeAction

	// Count statistics
	TotalCount int
}

// GetCodeActions returns code actions for a position or range.
func (as *ActionsService) GetCodeActions(ctx context.Context, path string, rng Range, diagnostics []Diagnostic) (*CodeActionResult, error) {
	server, err := as.getServer(ctx, path)
	if err != nil {
		return nil, err
	}

	// Normalize path for cache key
	cleanPath := filepath.Clean(path)

	// Check cache
	key := actionCacheKey{
		path:      cleanPath,
		startLine: rng.Start.Line,
		startChar: rng.Start.Character,
		endLine:   rng.End.Line,
		endChar:   rng.End.Character,
	}
	now := time.Now().Unix()

	as.mu.RLock()
	cacheAge := as.codeActionCacheAge
	if entry, ok := as.codeActionCache[key]; ok {
		if now-entry.timestamp < cacheAge {
			as.mu.RUnlock()
			return as.categorizeActions(entry.actions), nil
		}
	}
	kinds := as.codeActionKinds
	as.mu.RUnlock()

	actions, err := server.CodeActions(ctx, path, rng, diagnostics)
	if err != nil {
		return nil, err
	}

	// Filter by kinds if specified
	if len(kinds) > 0 {
		actions = as.filterActionsByKindWith(actions, kinds)
	}

	// Cache result
	as.mu.Lock()
	as.codeActionCache[key] = &actionCacheEntry{
		actions:   actions,
		timestamp: now,
	}
	as.mu.Unlock()

	return as.categorizeActions(actions), nil
}

// GetCodeActionsAtPosition returns code actions at a specific position.
func (as *ActionsService) GetCodeActionsAtPosition(ctx context.Context, path string, pos Position, diagnostics []Diagnostic) (*CodeActionResult, error) {
	// Create a zero-width range at the position
	rng := Range{Start: pos, End: pos}
	return as.GetCodeActions(ctx, path, rng, diagnostics)
}

// GetQuickFixes returns only quick fix actions for the given diagnostics.
func (as *ActionsService) GetQuickFixes(ctx context.Context, path string, rng Range, diagnostics []Diagnostic) ([]CodeAction, error) {
	result, err := as.GetCodeActions(ctx, path, rng, diagnostics)
	if err != nil {
		return nil, err
	}
	return result.QuickFixes, nil
}

// GetRefactorings returns only refactoring actions for the given range.
func (as *ActionsService) GetRefactorings(ctx context.Context, path string, rng Range) ([]CodeAction, error) {
	result, err := as.GetCodeActions(ctx, path, rng, nil)
	if err != nil {
		return nil, err
	}
	return result.Refactors, nil
}

// GetOrganizeImports returns the organize imports action if available.
func (as *ActionsService) GetOrganizeImports(ctx context.Context, path string) (*CodeAction, error) {
	// Use full document range. MaxInt32 is safe and widely supported by LSP servers.
	fullRange := Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 1<<31 - 1, Character: 0},
	}

	result, err := as.GetCodeActions(ctx, path, fullRange, nil)
	if err != nil {
		return nil, err
	}

	for i := range result.SourceFixes {
		if result.SourceFixes[i].Kind == CodeActionKindSourceOrganizeImports {
			return &result.SourceFixes[i], nil
		}
	}

	return nil, nil // No organize imports action available
}

// ApplyCodeAction applies a code action's workspace edit.
func (as *ActionsService) ApplyCodeAction(ctx context.Context, action CodeAction) (*ApplyEditResult, error) {
	if action.Edit == nil {
		return nil, fmt.Errorf("code action has no edit")
	}

	return as.ApplyWorkspaceEdit(ctx, *action.Edit)
}

// --- Formatting ---

// FormatResult contains the result of a formatting operation.
type FormatResult struct {
	// Text edits to apply
	Edits []TextEdit

	// Number of edits
	EditCount int

	// Whether formatting was skipped (e.g., excluded file)
	Skipped    bool
	SkipReason string
}

// FormatDocument formats an entire document.
func (as *ActionsService) FormatDocument(ctx context.Context, path string) (*FormatResult, error) {
	// Check exclusions (with lock)
	as.mu.RLock()
	excluded := as.isExcludedFromFormattingLocked(path)
	opts := as.formatOptions
	as.mu.RUnlock()

	if excluded {
		return &FormatResult{
			Skipped:    true,
			SkipReason: "file excluded from formatting",
		}, nil
	}

	server, err := as.getServer(ctx, path)
	if err != nil {
		return nil, err
	}

	edits, err := server.Format(ctx, path, opts)
	if err != nil {
		return nil, err
	}

	return &FormatResult{
		Edits:     edits,
		EditCount: len(edits),
	}, nil
}

// FormatRange formats a range within a document.
func (as *ActionsService) FormatRange(ctx context.Context, path string, rng Range) (*FormatResult, error) {
	// Check exclusions (with lock)
	as.mu.RLock()
	excluded := as.isExcludedFromFormattingLocked(path)
	opts := as.formatOptions
	as.mu.RUnlock()

	if excluded {
		return &FormatResult{
			Skipped:    true,
			SkipReason: "file excluded from formatting",
		}, nil
	}

	server, err := as.getServer(ctx, path)
	if err != nil {
		return nil, err
	}

	edits, err := server.FormatRange(ctx, path, rng, opts)
	if err != nil {
		return nil, err
	}

	return &FormatResult{
		Edits:     edits,
		EditCount: len(edits),
	}, nil
}

// FormatOnSave formats a document if format-on-save is enabled.
// Returns nil result if format-on-save is disabled.
func (as *ActionsService) FormatOnSave(ctx context.Context, path string) (*FormatResult, error) {
	as.mu.RLock()
	enabled := as.formatOnSave
	as.mu.RUnlock()

	if !enabled {
		return nil, nil
	}
	return as.FormatDocument(ctx, path)
}

// ShouldFormatOnSave returns whether format-on-save is enabled and the file is not excluded.
func (as *ActionsService) ShouldFormatOnSave(path string) bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.formatOnSave && !as.isExcludedFromFormattingLocked(path)
}

// SetFormatOnSave enables/disables format on save.
func (as *ActionsService) SetFormatOnSave(enable bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.formatOnSave = enable
}

// SetFormattingOptions sets the formatting options.
func (as *ActionsService) SetFormattingOptions(opts FormattingOptions) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.formatOptions = opts
}

// GetFormattingOptions returns the current formatting options.
func (as *ActionsService) GetFormattingOptions() FormattingOptions {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.formatOptions
}

// --- Rename ---

// RenameResult contains the result of a rename operation.
type RenameResult struct {
	// The workspace edit to apply
	Edit *WorkspaceEdit

	// Affected files
	AffectedFiles []string

	// Number of changes per file
	ChangesPerFile map[string]int

	// Total number of changes
	TotalChanges int

	// Preview information
	Preview *RenamePreview
}

// RenamePreview contains preview information for a rename.
type RenamePreview struct {
	OldName string
	NewName string
	Changes []RenameChange
}

// RenameChange represents a single rename change.
type RenameChange struct {
	FilePath     string
	RelativePath string
	Line         int
	Character    int
	OldText      string
	NewText      string
}

// PrepareRename checks if rename is valid at the given position and returns the range to rename.
func (as *ActionsService) PrepareRename(ctx context.Context, path string, pos Position) (*Range, string, error) {
	server, err := as.getServer(ctx, path)
	if err != nil {
		return nil, "", err
	}

	// Check if server supports prepareRename
	caps := server.Capabilities()
	if caps.RenameProvider == nil {
		return nil, "", ErrNotSupported
	}

	// For now, we don't have PrepareRename in the server, so we return a simple range
	// A real implementation would call textDocument/prepareRename
	return &Range{Start: pos, End: pos}, "", nil
}

// Rename performs a rename operation.
func (as *ActionsService) Rename(ctx context.Context, path string, pos Position, newName string) (*RenameResult, error) {
	server, err := as.getServer(ctx, path)
	if err != nil {
		return nil, err
	}

	edit, err := server.Rename(ctx, path, pos, newName)
	if err != nil {
		return nil, err
	}

	if edit == nil {
		return nil, fmt.Errorf("rename returned no edit")
	}

	result := &RenameResult{
		Edit:           edit,
		AffectedFiles:  make([]string, 0),
		ChangesPerFile: make(map[string]int),
	}

	// Analyze the edit
	for uri, edits := range edit.Changes {
		filePath := URIToFilePath(uri)
		result.AffectedFiles = append(result.AffectedFiles, filePath)
		result.ChangesPerFile[filePath] = len(edits)
		result.TotalChanges += len(edits)
	}

	// Sort affected files for consistent ordering
	sort.Strings(result.AffectedFiles)

	return result, nil
}

// RenameWithPreview performs a rename and includes preview information.
func (as *ActionsService) RenameWithPreview(ctx context.Context, path string, pos Position, oldName, newName string) (*RenameResult, error) {
	result, err := as.Rename(ctx, path, pos, newName)
	if err != nil {
		return nil, err
	}

	// Build preview
	preview := &RenamePreview{
		OldName: oldName,
		NewName: newName,
		Changes: make([]RenameChange, 0),
	}

	workspaceRoot := ""
	if as.manager != nil {
		workspaceRoot = as.manager.WorkspaceRoot()
	}

	for uri, edits := range result.Edit.Changes {
		filePath := URIToFilePath(uri)
		relativePath := filePath
		if workspaceRoot != "" {
			if rel, err := filepath.Rel(workspaceRoot, filePath); err == nil {
				relativePath = rel
			}
		}

		for _, edit := range edits {
			preview.Changes = append(preview.Changes, RenameChange{
				FilePath:     filePath,
				RelativePath: relativePath,
				Line:         edit.Range.Start.Line + 1,
				Character:    edit.Range.Start.Character + 1,
				OldText:      oldName,
				NewText:      edit.NewText,
			})
		}
	}

	// Sort changes by file and position
	sort.Slice(preview.Changes, func(i, j int) bool {
		if preview.Changes[i].FilePath != preview.Changes[j].FilePath {
			return preview.Changes[i].FilePath < preview.Changes[j].FilePath
		}
		if preview.Changes[i].Line != preview.Changes[j].Line {
			return preview.Changes[i].Line < preview.Changes[j].Line
		}
		return preview.Changes[i].Character < preview.Changes[j].Character
	})

	result.Preview = preview
	return result, nil
}

// NeedsRenameConfirmation returns whether rename should show a confirmation dialog.
func (as *ActionsService) NeedsRenameConfirmation() bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.renameConfirmation
}

// --- Signature Help ---

// SignatureHelpResult contains enhanced signature help information.
type SignatureHelpResult struct {
	// The signature help from the server
	Help *SignatureHelp

	// Active signature information
	ActiveSignature *SignatureDisplay

	// All signatures formatted for display
	Signatures []SignatureDisplay

	// Whether there's an active signature
	HasActiveSignature bool
}

// SignatureDisplay contains formatted signature information.
type SignatureDisplay struct {
	// Full signature label
	Label string

	// Documentation (if any)
	Documentation string

	// Parameters
	Parameters []ParameterDisplay

	// Index of the active parameter
	ActiveParameterIndex int

	// The active parameter (if any)
	ActiveParameter *ParameterDisplay
}

// ParameterDisplay contains formatted parameter information.
type ParameterDisplay struct {
	// Parameter label
	Label string

	// Documentation (if any)
	Documentation string

	// Whether this is the active parameter
	IsActive bool
}

// GetSignatureHelp returns signature help at the given position.
func (as *ActionsService) GetSignatureHelp(ctx context.Context, path string, pos Position) (*SignatureHelpResult, error) {
	server, err := as.getServer(ctx, path)
	if err != nil {
		return nil, err
	}

	help, err := server.SignatureHelp(ctx, path, pos)
	if err != nil {
		return nil, err
	}

	if help == nil || len(help.Signatures) == 0 {
		return &SignatureHelpResult{}, nil
	}

	// Track state
	as.mu.Lock()
	as.activeSignature = &signatureState{
		path:            path,
		pos:             pos,
		help:            help,
		activeSignature: help.ActiveSignature,
		activeParameter: help.ActiveParameter,
		timestamp:       time.Now().Unix(),
	}
	as.mu.Unlock()

	return as.buildSignatureResult(help), nil
}

// GetActiveSignature returns the currently active signature (if tracking).
func (as *ActionsService) GetActiveSignature() *SignatureHelpResult {
	as.mu.RLock()
	if as.activeSignature == nil || as.activeSignature.help == nil {
		as.mu.RUnlock()
		return nil
	}
	// Copy the help pointer while holding the lock to avoid races
	help := as.activeSignature.help
	as.mu.RUnlock()

	return as.buildSignatureResult(help)
}

// ClearSignatureHelp clears the active signature help state.
func (as *ActionsService) ClearSignatureHelp() {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.activeSignature = nil
}

// GetSignatureTriggerCharacters returns characters that trigger signature help.
func (as *ActionsService) GetSignatureTriggerCharacters(ctx context.Context, path string) ([]string, error) {
	server, err := as.getServer(ctx, path)
	if err != nil {
		return nil, err
	}

	caps := server.Capabilities()
	if caps.SignatureHelpProvider == nil {
		return nil, nil
	}

	return caps.SignatureHelpProvider.TriggerCharacters, nil
}

// --- Workspace Edit Application ---

// ApplyEditResult contains the result of applying a workspace edit.
type ApplyEditResult struct {
	// Whether the edit was applied successfully
	Applied bool

	// Files that were modified
	ModifiedFiles []string

	// Error message if not applied
	FailureReason string
}

// ApplyWorkspaceEdit applies a workspace edit.
// Note: This is a placeholder - actual implementation depends on the editor's buffer system.
func (as *ActionsService) ApplyWorkspaceEdit(ctx context.Context, edit WorkspaceEdit) (*ApplyEditResult, error) {
	result := &ApplyEditResult{
		ModifiedFiles: make([]string, 0),
	}

	// Collect files from edit.Changes
	fileSet := make(map[string]bool)
	for uri := range edit.Changes {
		fileSet[URIToFilePath(uri)] = true
	}

	// Also collect files from DocumentChanges if present
	// DocumentChanges can contain TextDocumentEdit or resource operations (CreateFile, RenameFile, DeleteFile)
	for _, change := range edit.DocumentChanges {
		if changeMap, ok := change.(map[string]any); ok {
			// Check if it's a TextDocumentEdit (has textDocument field)
			if textDoc, ok := changeMap["textDocument"].(map[string]any); ok {
				if uri, ok := textDoc["uri"].(string); ok {
					fileSet[URIToFilePath(DocumentURI(uri))] = true
				}
			}
			// Check for resource operations
			if kind, ok := changeMap["kind"].(string); ok {
				switch kind {
				case "create", "delete":
					if uri, ok := changeMap["uri"].(string); ok {
						fileSet[URIToFilePath(DocumentURI(uri))] = true
					}
				case "rename":
					if oldUri, ok := changeMap["oldUri"].(string); ok {
						fileSet[URIToFilePath(DocumentURI(oldUri))] = true
					}
					if newUri, ok := changeMap["newUri"].(string); ok {
						fileSet[URIToFilePath(DocumentURI(newUri))] = true
					}
				}
			}
		}
	}

	// Convert set to slice
	for file := range fileSet {
		result.ModifiedFiles = append(result.ModifiedFiles, file)
	}

	// Sort for consistent ordering
	sort.Strings(result.ModifiedFiles)

	// In a real implementation, this would apply edits through the engine/buffer system
	// For now, we just report what would be changed
	result.Applied = true

	return result, nil
}

// --- Cache Management ---

// InvalidateCodeActionCache invalidates code action cache for a file.
func (as *ActionsService) InvalidateCodeActionCache(path string) {
	cleanPath := filepath.Clean(path)

	as.mu.Lock()
	defer as.mu.Unlock()

	for key := range as.codeActionCache {
		if key.path == cleanPath {
			delete(as.codeActionCache, key)
		}
	}
}

// ClearCodeActionCache clears all code action cache.
func (as *ActionsService) ClearCodeActionCache() {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.codeActionCache = make(map[actionCacheKey]*actionCacheEntry)
}

// --- Helper Methods ---

func (as *ActionsService) getServer(ctx context.Context, path string) (*Server, error) {
	if as.manager == nil {
		return nil, ErrNoServerForFile
	}
	server, err := as.manager.ServerForFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("getting server for file %s: %w", path, err)
	}
	return server, nil
}

func (as *ActionsService) categorizeActions(actions []CodeAction) *CodeActionResult {
	result := &CodeActionResult{
		All:        actions,
		TotalCount: len(actions),
	}

	for _, action := range actions {
		switch {
		case strings.HasPrefix(string(action.Kind), string(CodeActionKindQuickFix)):
			result.QuickFixes = append(result.QuickFixes, action)
		case strings.HasPrefix(string(action.Kind), string(CodeActionKindRefactor)):
			result.Refactors = append(result.Refactors, action)
		case strings.HasPrefix(string(action.Kind), string(CodeActionKindSource)):
			result.SourceFixes = append(result.SourceFixes, action)
		default:
			result.OtherActions = append(result.OtherActions, action)
		}
	}

	return result
}

// filterActionsByKindWith filters actions by the given kinds (with prefix matching).
func (as *ActionsService) filterActionsByKindWith(actions []CodeAction, kinds []CodeActionKind) []CodeAction {
	if len(kinds) == 0 {
		return actions
	}

	kindSet := make(map[CodeActionKind]bool)
	for _, k := range kinds {
		kindSet[k] = true
	}

	var filtered []CodeAction
	for _, action := range actions {
		// Check if action kind matches any configured kind (with prefix matching)
		for kind := range kindSet {
			if strings.HasPrefix(string(action.Kind), string(kind)) {
				filtered = append(filtered, action)
				break
			}
		}
	}

	return filtered
}

// isExcludedFromFormattingLocked checks if a path is excluded from formatting.
// Must be called with as.mu held (read or write lock).
func (as *ActionsService) isExcludedFromFormattingLocked(path string) bool {
	if len(as.formatExcludes) == 0 {
		return false
	}

	cleanPath := filepath.Clean(path)
	filename := filepath.Base(cleanPath)

	for _, pattern := range as.formatExcludes {
		// Match against filename
		if matched, err := filepath.Match(pattern, filename); err == nil && matched {
			return true
		}
		// Also try matching against full path
		if matched, err := filepath.Match(pattern, cleanPath); err == nil && matched {
			return true
		}
	}

	return false
}

func (as *ActionsService) buildSignatureResult(help *SignatureHelp) *SignatureHelpResult {
	result := &SignatureHelpResult{
		Help:       help,
		Signatures: make([]SignatureDisplay, len(help.Signatures)),
	}

	for i, sig := range help.Signatures {
		display := SignatureDisplay{
			Label:                sig.Label,
			Documentation:        extractDocumentation(sig.Documentation),
			Parameters:           make([]ParameterDisplay, len(sig.Parameters)),
			ActiveParameterIndex: sig.ActiveParameter,
		}

		// Determine active parameter index.
		// LSP spec: SignatureInformation.activeParameter takes precedence if set (>= 0).
		// If the signature doesn't specify, fall back to SignatureHelp.activeParameter.
		// Both default to 0 which is valid (first parameter), so we use the signature's
		// value for the active signature, otherwise use the help's global value.
		activeParam := help.ActiveParameter
		if i == help.ActiveSignature {
			// For the active signature, prefer the signature-level activeParameter
			activeParam = sig.ActiveParameter
		}

		for j, param := range sig.Parameters {
			paramDisplay := ParameterDisplay{
				Label:         extractParameterLabel(param.Label),
				Documentation: extractDocumentation(param.Documentation),
				IsActive:      j == activeParam,
			}
			display.Parameters[j] = paramDisplay

			if paramDisplay.IsActive {
				display.ActiveParameter = &display.Parameters[j]
			}
		}

		result.Signatures[i] = display

		// Set active signature
		if i == help.ActiveSignature {
			result.ActiveSignature = &result.Signatures[i]
			result.HasActiveSignature = true
		}
	}

	return result
}

// extractDocumentation extracts string documentation from various formats.
func extractDocumentation(doc any) string {
	if doc == nil {
		return ""
	}

	switch v := doc.(type) {
	case string:
		return v
	case map[string]any:
		// MarkupContent
		if value, ok := v["value"].(string); ok {
			return value
		}
	}

	return ""
}

// extractParameterLabel extracts the label string from parameter label.
func extractParameterLabel(label any) string {
	if label == nil {
		return ""
	}

	switch v := label.(type) {
	case string:
		return v
	case []any:
		// [start, end] range - we'd need the signature label to extract
		// For now, return empty
		return ""
	}

	return ""
}

// --- Utility Functions ---

// CodeActionKindString returns a human-readable name for a code action kind.
func CodeActionKindString(kind CodeActionKind) string {
	switch kind {
	case CodeActionKindQuickFix:
		return "Quick Fix"
	case CodeActionKindRefactor:
		return "Refactor"
	case CodeActionKindRefactorExtract:
		return "Extract"
	case CodeActionKindRefactorInline:
		return "Inline"
	case CodeActionKindRefactorRewrite:
		return "Rewrite"
	case CodeActionKindSource:
		return "Source"
	case CodeActionKindSourceOrganizeImports:
		return "Organize Imports"
	default:
		if kind == "" {
			return "Action"
		}
		return string(kind)
	}
}

// FormatCodeAction formats a code action for display.
func FormatCodeAction(action CodeAction) string {
	kindStr := CodeActionKindString(action.Kind)
	if action.IsPreferred {
		return fmt.Sprintf("[%s] %s (preferred)", kindStr, action.Title)
	}
	return fmt.Sprintf("[%s] %s", kindStr, action.Title)
}

// SortCodeActions sorts code actions by kind and preferred status.
// Actions are sorted: preferred first, then by kind order, then alphabetically by title.
func SortCodeActions(actions []CodeAction) {
	sort.Slice(actions, func(i, j int) bool {
		// Preferred actions first
		if actions[i].IsPreferred != actions[j].IsPreferred {
			return actions[i].IsPreferred
		}
		// Then by kind (quick fixes first)
		orderI := codeActionKindOrder(actions[i].Kind)
		orderJ := codeActionKindOrder(actions[j].Kind)
		if orderI != orderJ {
			return orderI < orderJ
		}
		// Tie-breaker: sort alphabetically by title for deterministic ordering
		return actions[i].Title < actions[j].Title
	})
}

func codeActionKindOrder(kind CodeActionKind) int {
	switch {
	case strings.HasPrefix(string(kind), string(CodeActionKindQuickFix)):
		return 0
	case strings.HasPrefix(string(kind), string(CodeActionKindRefactor)):
		return 1
	case strings.HasPrefix(string(kind), string(CodeActionKindSource)):
		return 2
	default:
		return 3
	}
}

// GroupCodeActionsByKind groups code actions by their kind.
func GroupCodeActionsByKind(actions []CodeAction) map[CodeActionKind][]CodeAction {
	result := make(map[CodeActionKind][]CodeAction)
	for _, action := range actions {
		// Get the base kind
		baseKind := action.Kind
		if idx := strings.Index(string(action.Kind), "."); idx > 0 {
			baseKind = CodeActionKind(string(action.Kind)[:idx])
		}
		result[baseKind] = append(result[baseKind], action)
	}
	return result
}

// FormatTextEdit formats a text edit for display.
func FormatTextEdit(edit TextEdit) string {
	if edit.Range.Start.Line == edit.Range.End.Line {
		return fmt.Sprintf("Line %d: %q", edit.Range.Start.Line+1, edit.NewText)
	}
	return fmt.Sprintf("Lines %d-%d: %q", edit.Range.Start.Line+1, edit.Range.End.Line+1, edit.NewText)
}

// CountWorkspaceEditChanges counts total changes in a workspace edit.
// Counts edits from both Changes and DocumentChanges.
func CountWorkspaceEditChanges(edit *WorkspaceEdit) int {
	if edit == nil {
		return 0
	}

	count := 0

	// Count from Changes
	for _, edits := range edit.Changes {
		count += len(edits)
	}

	// Count from DocumentChanges
	for _, change := range edit.DocumentChanges {
		if changeMap, ok := change.(map[string]any); ok {
			// TextDocumentEdit has "edits" array
			if edits, ok := changeMap["edits"].([]any); ok {
				count += len(edits)
			}
			// Resource operations (create, rename, delete) count as 1 each
			if _, ok := changeMap["kind"].(string); ok {
				count++
			}
		}
	}

	return count
}

// GetWorkspaceEditFiles returns all files affected by a workspace edit.
// Collects files from both Changes and DocumentChanges.
func GetWorkspaceEditFiles(edit *WorkspaceEdit) []string {
	if edit == nil {
		return nil
	}

	fileSet := make(map[string]bool)

	// Collect from Changes
	for uri := range edit.Changes {
		fileSet[URIToFilePath(uri)] = true
	}

	// Collect from DocumentChanges
	for _, change := range edit.DocumentChanges {
		if changeMap, ok := change.(map[string]any); ok {
			// TextDocumentEdit
			if textDoc, ok := changeMap["textDocument"].(map[string]any); ok {
				if uri, ok := textDoc["uri"].(string); ok {
					fileSet[URIToFilePath(DocumentURI(uri))] = true
				}
			}
			// Resource operations
			if kind, ok := changeMap["kind"].(string); ok {
				switch kind {
				case "create", "delete":
					if uri, ok := changeMap["uri"].(string); ok {
						fileSet[URIToFilePath(DocumentURI(uri))] = true
					}
				case "rename":
					if oldUri, ok := changeMap["oldUri"].(string); ok {
						fileSet[URIToFilePath(DocumentURI(oldUri))] = true
					}
					if newUri, ok := changeMap["newUri"].(string); ok {
						fileSet[URIToFilePath(DocumentURI(newUri))] = true
					}
				}
			}
		}
	}

	if len(fileSet) == 0 {
		return nil
	}

	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}
