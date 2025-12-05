package lsp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Client provides a high-level interface for LSP operations.
// It coordinates multiple language servers and provides unified access
// to completions, diagnostics, navigation, code actions, and formatting.
//
// Client is the main entry point for the LSP package and should be used
// by higher-level components (editor, plugins) instead of directly using
// Manager or individual services.
type Client struct {
	mu     sync.RWMutex
	status ClientStatus

	// Core components
	manager *Manager

	// High-level services
	completion  *CompletionService
	diagnostics *DiagnosticsService
	navigation  *NavigationService
	actions     *ActionsService

	// Configuration
	config ClientConfig

	// Event callbacks
	onDiagnostics func(path string, diagnostics []Diagnostic)
	// Note: Server lifecycle callbacks are reserved for future use.
	// They will be wired when Manager adds support for server events.
	onServerStarted func(languageID string)
	onServerStopped func(languageID string, err error)
}

// ClientStatus represents the client's lifecycle state.
type ClientStatus int

const (
	// ClientStatusStopped indicates the client is not running.
	ClientStatusStopped ClientStatus = iota
	// ClientStatusStarting indicates the client is starting up.
	ClientStatusStarting
	// ClientStatusReady indicates the client is ready for requests.
	ClientStatusReady
	// ClientStatusShuttingDown indicates the client is shutting down.
	ClientStatusShuttingDown
)

// String returns a human-readable status string.
func (s ClientStatus) String() string {
	switch s {
	case ClientStatusStopped:
		return "stopped"
	case ClientStatusStarting:
		return "starting"
	case ClientStatusReady:
		return "ready"
	case ClientStatusShuttingDown:
		return "shutting down"
	default:
		return "unknown"
	}
}

// ClientConfig contains configuration for the LSP client.
type ClientConfig struct {
	// Server configurations by language ID
	Servers map[string]ServerConfig

	// Workspace folders
	WorkspaceFolders []WorkspaceFolder

	// Request timeout for LSP requests
	RequestTimeout time.Duration

	// Completion settings
	CompletionMaxResults int
	CompletionCacheAge   int64 // seconds

	// Formatting settings
	FormatOnSave   bool
	FormatOnType   bool
	FormatOptions  FormattingOptions
	FormatExcludes []string

	// Code action settings
	CodeActionKinds    []CodeActionKind
	CodeActionCacheAge int64 // seconds

	// Rename settings
	RenameConfirmation bool

	// Diagnostics settings
	DiagnosticsDebounce time.Duration
	DiagnosticsEnabled  bool

	// Auto-detect servers if none configured
	AutoDetectServers bool
}

// DefaultClientConfig returns a default client configuration.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Servers:              make(map[string]ServerConfig),
		WorkspaceFolders:     nil,
		RequestTimeout:       10 * time.Second,
		CompletionMaxResults: 100,
		CompletionCacheAge:   5,
		FormatOnSave:         false,
		FormatOnType:         false,
		FormatOptions:        DefaultFormattingOptions(),
		FormatExcludes:       nil,
		CodeActionKinds:      nil, // all kinds
		CodeActionCacheAge:   10,
		RenameConfirmation:   true,
		DiagnosticsDebounce:  100 * time.Millisecond,
		DiagnosticsEnabled:   true,
		AutoDetectServers:    true,
	}
}

// ClientOption configures the client.
type ClientOption func(*Client)

// WithClientConfig sets the full client configuration.
func WithClientConfig(config ClientConfig) ClientOption {
	return func(c *Client) {
		c.config = config
	}
}

// WithServers sets the server configurations.
func WithServers(servers map[string]ServerConfig) ClientOption {
	return func(c *Client) {
		c.config.Servers = servers
	}
}

// WithWorkspaceFolders sets the workspace folders.
func WithWorkspaceFolders(folders []WorkspaceFolder) ClientOption {
	return func(c *Client) {
		c.config.WorkspaceFolders = folders
	}
}

// WithWorkspaceRoot sets a single workspace root path.
func WithWorkspaceRoot(path string) ClientOption {
	return func(c *Client) {
		c.config.WorkspaceFolders = []WorkspaceFolder{WorkspaceFolderFromPath(path)}
	}
}

// WithClientRequestTimeout sets the request timeout.
func WithClientRequestTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.config.RequestTimeout = d
	}
}

// WithClientFormatOnSave enables/disables format on save.
func WithClientFormatOnSave(enable bool) ClientOption {
	return func(c *Client) {
		c.config.FormatOnSave = enable
	}
}

// WithClientFormatOptions sets the formatting options.
func WithClientFormatOptions(opts FormattingOptions) ClientOption {
	return func(c *Client) {
		c.config.FormatOptions = opts
	}
}

// WithAutoDetectServers enables/disables auto-detection of servers.
func WithAutoDetectServers(enable bool) ClientOption {
	return func(c *Client) {
		c.config.AutoDetectServers = enable
	}
}

// WithDiagnosticsCallback sets a callback for diagnostics updates.
func WithClientDiagnosticsCallback(cb func(path string, diagnostics []Diagnostic)) ClientOption {
	return func(c *Client) {
		c.onDiagnostics = cb
	}
}

// WithServerStartedCallback sets a callback for server start events.
func WithServerStartedCallback(cb func(languageID string)) ClientOption {
	return func(c *Client) {
		c.onServerStarted = cb
	}
}

// WithServerStoppedCallback sets a callback for server stop events.
func WithServerStoppedCallback(cb func(languageID string, err error)) ClientOption {
	return func(c *Client) {
		c.onServerStopped = cb
	}
}

// NewClient creates a new LSP client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		status: ClientStatusStopped,
		config: DefaultClientConfig(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start initializes and starts the LSP client.
// This registers server configurations and prepares the client for requests.
// Servers are started lazily when files of that language are opened.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status != ClientStatusStopped {
		c.mu.Unlock()
		return ErrAlreadyStarted
	}
	c.status = ClientStatusStarting
	c.mu.Unlock()

	// Create manager with diagnostics forwarding
	managerOpts := []ManagerOption{
		WithRequestTimeout(c.config.RequestTimeout),
	}
	if c.onDiagnostics != nil {
		managerOpts = append(managerOpts, WithDiagnosticsCallback(func(uri DocumentURI, diags []Diagnostic) {
			c.onDiagnostics(URIToFilePath(uri), diags)
		}))
	}
	c.manager = NewManager(managerOpts...)

	// Set workspace folders
	if len(c.config.WorkspaceFolders) > 0 {
		c.manager.SetWorkspaceFolders(c.config.WorkspaceFolders)
	}

	// Register configured servers
	for langID, serverConfig := range c.config.Servers {
		c.manager.RegisterServer(langID, serverConfig)
	}

	// Auto-detect servers if enabled and none configured
	if c.config.AutoDetectServers && len(c.config.Servers) == 0 {
		detected := AutoDetectServers()
		for langID, serverConfig := range detected {
			c.manager.RegisterServer(langID, serverConfig)
		}
	}

	// Create high-level services
	c.completion = NewCompletionService(c.manager,
		WithMaxResults(c.config.CompletionMaxResults),
		WithCacheTimeout(time.Duration(c.config.CompletionCacheAge)*time.Second),
	)

	c.diagnostics = NewDiagnosticsService(c.manager,
		WithDiagnosticsDebounce(c.config.DiagnosticsDebounce),
	)

	c.navigation = NewNavigationService(c.manager)

	c.actions = NewActionsService(c.manager,
		WithFormatOnSave(c.config.FormatOnSave),
		WithFormatOnType(c.config.FormatOnType),
		WithFormattingOptions(c.config.FormatOptions),
		WithFormatExcludes(c.config.FormatExcludes),
		WithCodeActionKinds(c.config.CodeActionKinds),
		WithCodeActionCacheAge(c.config.CodeActionCacheAge),
		WithRenameConfirmation(c.config.RenameConfirmation),
	)

	c.mu.Lock()
	c.status = ClientStatusReady
	c.mu.Unlock()

	return nil
}

// Shutdown gracefully shuts down the LSP client and all servers.
func (c *Client) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if c.status != ClientStatusReady {
		status := c.status
		c.mu.Unlock()
		if status == ClientStatusStopped {
			return nil
		}
		return fmt.Errorf("client in invalid state for shutdown: %s", status)
	}
	c.status = ClientStatusShuttingDown
	c.mu.Unlock()

	// Clear caches
	if c.completion != nil {
		c.completion.ClearCache()
	}
	if c.diagnostics != nil {
		c.diagnostics.Clear()
	}
	if c.actions != nil {
		c.actions.ClearCodeActionCache()
		c.actions.ClearSignatureHelp()
	}

	// Shutdown manager and all servers
	var err error
	if c.manager != nil {
		err = c.manager.Shutdown(ctx)
	}

	c.mu.Lock()
	c.status = ClientStatusStopped
	c.manager = nil
	c.completion = nil
	c.diagnostics = nil
	c.navigation = nil
	c.actions = nil
	c.mu.Unlock()

	return err
}

// Status returns the current client status.
func (c *Client) Status() ClientStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// IsReady returns true if the client is ready to handle requests.
func (c *Client) IsReady() bool {
	return c.Status() == ClientStatusReady
}

// --- Document Management ---

// OpenDocument opens a document for LSP tracking.
func (c *Client) OpenDocument(ctx context.Context, path, content string) error {
	svc, err := c.getServices()
	if err != nil {
		return err
	}
	return svc.manager.OpenDocument(ctx, path, content)
}

// CloseDocument closes a document.
func (c *Client) CloseDocument(ctx context.Context, path string) error {
	svc, err := c.getServices()
	if err != nil {
		return err
	}

	// Clear related caches
	svc.completion.InvalidateCache(path)
	svc.diagnostics.ClearFile(path)
	svc.actions.InvalidateCodeActionCache(path)

	return svc.manager.CloseDocument(ctx, path)
}

// ChangeDocument notifies LSP of document changes.
func (c *Client) ChangeDocument(ctx context.Context, path string, changes []TextDocumentContentChangeEvent) error {
	svc, err := c.getServices()
	if err != nil {
		return err
	}

	// Invalidate related caches
	svc.completion.InvalidateCache(path)
	svc.actions.InvalidateCodeActionCache(path)

	return svc.manager.ChangeDocument(ctx, path, changes)
}

// SaveDocument notifies LSP of a document save and optionally formats.
func (c *Client) SaveDocument(ctx context.Context, path string) (*FormatResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}

	// Format on save if enabled
	return svc.actions.FormatOnSave(ctx, path)
}

// IsAvailable checks if LSP is available for a file type.
func (c *Client) IsAvailable(path string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.manager == nil {
		return false
	}
	return c.manager.IsAvailable(path)
}

// --- Completion ---

// Complete returns code completions at a position.
func (c *Client) Complete(ctx context.Context, path string, pos Position, prefix string) (*CompletionResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.completion.Complete(ctx, path, pos, prefix)
}

// CompleteWithTrigger returns completions triggered by a character.
func (c *Client) CompleteWithTrigger(ctx context.Context, path string, pos Position, triggerChar string) (*CompletionResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.completion.CompleteWithTrigger(ctx, path, pos, triggerChar)
}

// ResolveCompletion resolves additional details for a completion item.
func (c *Client) ResolveCompletion(ctx context.Context, path string, item CompletionItem) (*CompletionItem, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.completion.ResolveItem(ctx, path, item)
}

// --- Diagnostics ---

// Diagnostics returns current diagnostics for a file.
func (c *Client) Diagnostics(path string) []Diagnostic {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.diagnostics == nil {
		return nil
	}
	return c.diagnostics.GetDiagnostics(path)
}

// AllDiagnostics returns diagnostics for all open files.
func (c *Client) AllDiagnostics() map[string][]Diagnostic {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.diagnostics == nil {
		return nil
	}
	return c.diagnostics.AllDiagnostics()
}

// DiagnosticsAtLine returns diagnostics at a specific line.
func (c *Client) DiagnosticsAtLine(path string, line int) []Diagnostic {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.diagnostics == nil {
		return nil
	}
	return c.diagnostics.GetDiagnosticsAtLine(path, line)
}

// DiagnosticsAtPosition returns diagnostics at a specific position.
func (c *Client) DiagnosticsAtPosition(path string, pos Position) []Diagnostic {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.diagnostics == nil {
		return nil
	}
	return c.diagnostics.GetDiagnosticsAtPosition(path, pos)
}

// ErrorCount returns the count of errors across all files.
func (c *Client) ErrorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.diagnostics == nil {
		return 0
	}
	return c.diagnostics.Summary().TotalErrors
}

// WarningCount returns the count of warnings across all files.
func (c *Client) WarningCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.diagnostics == nil {
		return 0
	}
	return c.diagnostics.Summary().TotalWarns
}

// DiagnosticsSummary returns overall diagnostics statistics.
func (c *Client) DiagnosticsSummary() DiagnosticSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.diagnostics == nil {
		return DiagnosticSummary{}
	}
	return c.diagnostics.Summary()
}

// --- Navigation ---

// Hover returns hover information at a position.
func (c *Client) Hover(ctx context.Context, path string, pos Position) (*Hover, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.manager.Hover(ctx, path, pos)
}

// GoToDefinition navigates to the definition of a symbol.
func (c *Client) GoToDefinition(ctx context.Context, path string, pos Position) (*NavigationResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.GoToDefinition(ctx, path, pos)
}

// GoToTypeDefinition navigates to the type definition.
func (c *Client) GoToTypeDefinition(ctx context.Context, path string, pos Position) (*NavigationResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.GoToTypeDefinition(ctx, path, pos)
}

// GoToImplementation navigates to implementations.
func (c *Client) GoToImplementation(ctx context.Context, path string, pos Position) (*NavigationResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.FindImplementations(ctx, path, pos)
}

// FindReferences finds all references to a symbol.
func (c *Client) FindReferences(ctx context.Context, path string, pos Position) (*NavigationResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.FindReferences(ctx, path, pos)
}

// DocumentSymbols returns all symbols in a document.
func (c *Client) DocumentSymbols(ctx context.Context, path string) ([]DocumentSymbol, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.GetDocumentSymbols(ctx, path)
}

// DocumentSymbolTree returns symbols as a hierarchical tree.
func (c *Client) DocumentSymbolTree(ctx context.Context, path string) (*SymbolTree, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.GetSymbolTree(ctx, path)
}

// WorkspaceSymbols searches for symbols across the workspace.
func (c *Client) WorkspaceSymbols(ctx context.Context, query, languageID string) ([]SymbolInformation, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.SearchWorkspaceSymbols(ctx, query, languageID)
}

// SymbolAtPosition returns the symbol at a specific position.
func (c *Client) SymbolAtPosition(ctx context.Context, path string, pos Position) (*DocumentSymbol, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.navigation.GetSymbolAtPosition(ctx, path, pos)
}

// --- Code Actions and Formatting ---

// CodeActions returns available code actions for a range.
func (c *Client) CodeActions(ctx context.Context, path string, rng Range, diagnostics []Diagnostic) (*CodeActionResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.GetCodeActions(ctx, path, rng, diagnostics)
}

// QuickFixes returns quick fix actions for the given diagnostics.
func (c *Client) QuickFixes(ctx context.Context, path string, rng Range, diagnostics []Diagnostic) ([]CodeAction, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.GetQuickFixes(ctx, path, rng, diagnostics)
}

// Refactorings returns available refactorings for a range.
func (c *Client) Refactorings(ctx context.Context, path string, rng Range) ([]CodeAction, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.GetRefactorings(ctx, path, rng)
}

// OrganizeImports returns the organize imports action if available.
func (c *Client) OrganizeImports(ctx context.Context, path string) (*CodeAction, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.GetOrganizeImports(ctx, path)
}

// ApplyCodeAction applies a code action.
func (c *Client) ApplyCodeAction(ctx context.Context, action CodeAction) (*ApplyEditResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.ApplyCodeAction(ctx, action)
}

// Format formats an entire document.
func (c *Client) Format(ctx context.Context, path string) (*FormatResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.FormatDocument(ctx, path)
}

// FormatRange formats a range within a document.
func (c *Client) FormatRange(ctx context.Context, path string, rng Range) (*FormatResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.FormatRange(ctx, path, rng)
}

// --- Rename ---

// PrepareRename checks if rename is valid at a position.
func (c *Client) PrepareRename(ctx context.Context, path string, pos Position) (*Range, string, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, "", err
	}
	return svc.actions.PrepareRename(ctx, path, pos)
}

// Rename performs a rename refactoring.
func (c *Client) Rename(ctx context.Context, path string, pos Position, newName string) (*RenameResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.Rename(ctx, path, pos, newName)
}

// RenameWithPreview performs a rename and returns preview information.
func (c *Client) RenameWithPreview(ctx context.Context, path string, pos Position, oldName, newName string) (*RenameResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.RenameWithPreview(ctx, path, pos, oldName, newName)
}

// NeedsRenameConfirmation returns whether rename should show confirmation.
func (c *Client) NeedsRenameConfirmation() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.actions == nil {
		return true
	}
	return c.actions.NeedsRenameConfirmation()
}

// --- Signature Help ---

// SignatureHelp returns signature help at a position.
func (c *Client) SignatureHelp(ctx context.Context, path string, pos Position) (*SignatureHelpResult, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.GetSignatureHelp(ctx, path, pos)
}

// ActiveSignature returns the currently tracked active signature.
func (c *Client) ActiveSignature() *SignatureHelpResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady || c.actions == nil {
		return nil
	}
	return c.actions.GetActiveSignature()
}

// ClearSignatureHelp clears the active signature help state.
func (c *Client) ClearSignatureHelp() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status == ClientStatusReady && c.actions != nil {
		c.actions.ClearSignatureHelp()
	}
}

// SignatureTriggerCharacters returns characters that trigger signature help.
func (c *Client) SignatureTriggerCharacters(ctx context.Context, path string) ([]string, error) {
	svc, err := c.getServices()
	if err != nil {
		return nil, err
	}
	return svc.actions.GetSignatureTriggerCharacters(ctx, path)
}

// --- Configuration ---

// SetFormatOnSave enables or disables format on save.
func (c *Client) SetFormatOnSave(enable bool) {
	c.mu.Lock()
	c.config.FormatOnSave = enable
	if c.actions != nil {
		c.actions.SetFormatOnSave(enable)
	}
	c.mu.Unlock()
}

// SetFormattingOptions updates the formatting options.
func (c *Client) SetFormattingOptions(opts FormattingOptions) {
	c.mu.Lock()
	c.config.FormatOptions = opts
	if c.actions != nil {
		c.actions.SetFormattingOptions(opts)
	}
	c.mu.Unlock()
}

// GetFormattingOptions returns the current formatting options.
func (c *Client) GetFormattingOptions() FormattingOptions {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.FormatOptions
}

// Config returns a copy of the current client configuration.
func (c *Client) Config() ClientConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return a deep copy to prevent external modification
	config := c.config
	if c.config.Servers != nil {
		config.Servers = make(map[string]ServerConfig, len(c.config.Servers))
		for k, v := range c.config.Servers {
			config.Servers[k] = v
		}
	}
	if c.config.WorkspaceFolders != nil {
		config.WorkspaceFolders = make([]WorkspaceFolder, len(c.config.WorkspaceFolders))
		copy(config.WorkspaceFolders, c.config.WorkspaceFolders)
	}
	if c.config.FormatExcludes != nil {
		config.FormatExcludes = make([]string, len(c.config.FormatExcludes))
		copy(config.FormatExcludes, c.config.FormatExcludes)
	}
	if c.config.CodeActionKinds != nil {
		config.CodeActionKinds = make([]CodeActionKind, len(c.config.CodeActionKinds))
		copy(config.CodeActionKinds, c.config.CodeActionKinds)
	}
	return config
}

// --- Server Management ---

// RegisterServer registers a server configuration for a language.
// Can be called before or after Start.
func (c *Client) RegisterServer(languageID string, config ServerConfig) {
	c.mu.Lock()
	if c.config.Servers == nil {
		c.config.Servers = make(map[string]ServerConfig)
	}
	c.config.Servers[languageID] = config
	if c.manager != nil {
		c.manager.RegisterServer(languageID, config)
	}
	c.mu.Unlock()
}

// RegisteredLanguages returns languages with registered servers.
func (c *Client) RegisteredLanguages() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.manager == nil {
		langs := make([]string, 0, len(c.config.Servers))
		for lang := range c.config.Servers {
			langs = append(langs, lang)
		}
		return langs
	}
	return c.manager.RegisteredLanguages()
}

// ServerStatus returns the status of a language server.
func (c *Client) ServerStatus(languageID string) ServerStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.manager == nil {
		return ServerStatusStopped
	}
	return c.manager.ServerStatus(languageID)
}

// ServerInfos returns information about all running servers.
func (c *Client) ServerInfos() []ManagedServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.manager == nil {
		return nil
	}
	return c.manager.ServerInfos()
}

// RestartServer restarts a language server.
func (c *Client) RestartServer(ctx context.Context, languageID string) error {
	svc, err := c.getServices()
	if err != nil {
		return err
	}
	return svc.manager.RestartServer(ctx, languageID)
}

// --- Helper Methods ---

// clientServices holds snapshots of client services for thread-safe access.
// This avoids race conditions between status checks and service use.
type clientServices struct {
	manager     *Manager
	completion  *CompletionService
	diagnostics *DiagnosticsService
	navigation  *NavigationService
	actions     *ActionsService
}

// getServices returns a snapshot of all services if the client is ready.
// The returned services are safe to use after the lock is released.
func (c *Client) getServices() (*clientServices, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status != ClientStatusReady {
		return nil, ErrNotStarted
	}
	return &clientServices{
		manager:     c.manager,
		completion:  c.completion,
		diagnostics: c.diagnostics,
		navigation:  c.navigation,
		actions:     c.actions,
	}, nil
}

// Manager returns the underlying manager for advanced use cases.
// Most users should use Client methods instead.
//
// Note: The returned pointer is a snapshot that may become invalid after
// Shutdown. Callers should not cache this pointer across operations.
func (c *Client) Manager() *Manager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manager
}

// CompletionService returns the completion service for advanced use cases.
//
// Note: The returned pointer is a snapshot that may become invalid after
// Shutdown. Callers should not cache this pointer across operations.
func (c *Client) CompletionService() *CompletionService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.completion
}

// DiagnosticsService returns the diagnostics service for advanced use cases.
//
// Note: The returned pointer is a snapshot that may become invalid after
// Shutdown. Callers should not cache this pointer across operations.
func (c *Client) DiagnosticsService() *DiagnosticsService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.diagnostics
}

// NavigationService returns the navigation service for advanced use cases.
//
// Note: The returned pointer is a snapshot that may become invalid after
// Shutdown. Callers should not cache this pointer across operations.
func (c *Client) NavigationService() *NavigationService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.navigation
}

// ActionsService returns the actions service for advanced use cases.
func (c *Client) ActionsService() *ActionsService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actions
}

// --- Utility Functions ---

// QuickStart creates and starts a client with auto-detected servers for a workspace.
// This is a convenience function for quickly getting started with LSP.
func QuickStart(ctx context.Context, workspacePath string) (*Client, error) {
	folders := DetectWorkspaceFolders(workspacePath)

	client := NewClient(
		WithWorkspaceFolders(folders),
		WithAutoDetectServers(true),
	)

	if err := client.Start(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

// ClientStatusString returns a human-readable string for a client status.
func ClientStatusString(status ClientStatus) string {
	return status.String()
}
