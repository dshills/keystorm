package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// ServerStatus indicates the current state of a server.
type ServerStatus int

const (
	ServerStatusStopped ServerStatus = iota
	ServerStatusStarting
	ServerStatusInitializing
	ServerStatusReady
	ServerStatusShuttingDown
	ServerStatusError
)

// String returns a human-readable status name.
func (s ServerStatus) String() string {
	switch s {
	case ServerStatusStopped:
		return "stopped"
	case ServerStatusStarting:
		return "starting"
	case ServerStatusInitializing:
		return "initializing"
	case ServerStatusReady:
		return "ready"
	case ServerStatusShuttingDown:
		return "shutting down"
	case ServerStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Server represents a connection to a single language server.
type Server struct {
	mu sync.Mutex

	// Configuration
	config     ServerConfig
	languageID string

	// Process management
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// Transport
	transport *Transport

	// State
	status       atomic.Int32
	capabilities ServerCapabilities
	serverInfo   *InitializeServerInfo
	lastError    error

	// Document tracking
	documents   map[DocumentURI]*Document
	documentsMu sync.RWMutex

	// Diagnostics
	diagnostics   map[DocumentURI][]Diagnostic
	diagnosticsMu sync.RWMutex
	diagHandler   func(uri DocumentURI, diagnostics []Diagnostic)

	// Workspace
	workspaceFolders []WorkspaceFolder

	// Lifecycle
	ctx       context.Context
	cancel    context.CancelFunc
	exitCh    chan error
	closeOnce sync.Once
}

// Document represents an open document tracked by the server.
type Document struct {
	URI        DocumentURI
	LanguageID string
	Version    int
	Content    string
}

// ServerConfig defines how to start a language server.
type ServerConfig struct {
	// Command is the executable to run.
	Command string

	// Args are command-line arguments.
	Args []string

	// Env are additional environment variables.
	Env map[string]string

	// WorkDir is the working directory (defaults to workspace root).
	WorkDir string

	// InitializationOptions are sent during initialize.
	InitializationOptions any

	// Settings are sent via workspace/didChangeConfiguration.
	Settings any

	// FilePatterns that this server handles (e.g., "*.go").
	FilePatterns []string

	// LanguageIDs that this server handles (e.g., "go").
	LanguageIDs []string

	// Timeout for requests (default: 30s).
	Timeout time.Duration
}

// NewServer creates a new server instance (not yet started).
func NewServer(config ServerConfig, languageID string) *Server {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	s := &Server{
		config:      config,
		languageID:  languageID,
		documents:   make(map[DocumentURI]*Document),
		diagnostics: make(map[DocumentURI][]Diagnostic),
		exitCh:      make(chan error, 1),
	}
	s.status.Store(int32(ServerStatusStopped))
	return s
}

// Start starts the language server process and initializes it.
func (s *Server) Start(ctx context.Context, workspaceFolders []WorkspaceFolder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status() != ServerStatusStopped {
		return fmt.Errorf("server already started")
	}

	s.status.Store(int32(ServerStatusStarting))
	s.workspaceFolders = workspaceFolders

	// Create cancellable context
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start the process
	if err := s.startProcess(); err != nil {
		s.status.Store(int32(ServerStatusError))
		s.lastError = err
		return err
	}

	// Create transport
	s.transport = NewTransport(s.stdout, s.stdin, nil)

	// Register notification handlers
	s.registerNotificationHandlers()

	// Start transport read loop
	s.transport.Start(s.ctx)

	// Monitor process
	go s.monitorProcess()

	// Initialize the server
	s.status.Store(int32(ServerStatusInitializing))
	if err := s.initialize(s.ctx); err != nil {
		s.status.Store(int32(ServerStatusError))
		s.lastError = err
		s.stopProcess()
		return fmt.Errorf("initialize: %w", err)
	}

	s.status.Store(int32(ServerStatusReady))
	return nil
}

// startProcess starts the language server executable.
func (s *Server) startProcess() error {
	cmd := exec.CommandContext(s.ctx, s.config.Command, s.config.Args...)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.config.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Set working directory
	if s.config.WorkDir != "" {
		cmd.Dir = s.config.WorkDir
	} else if len(s.workspaceFolders) > 0 {
		cmd.Dir = URIToFilePath(s.workspaceFolders[0].URI)
	}

	// Get pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("start process: %w", err)
	}

	s.cmd = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.stderr = stderr

	return nil
}

// monitorProcess watches the process and signals when it exits.
func (s *Server) monitorProcess() {
	if s.cmd == nil {
		return
	}

	err := s.cmd.Wait()
	select {
	case s.exitCh <- err:
	default:
	}
}

// stopProcess stops the server process.
func (s *Server) stopProcess() {
	if s.transport != nil {
		s.transport.Close()
	}

	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.stderr != nil {
		s.stderr.Close()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
}

// initialize performs the LSP initialize handshake.
func (s *Server) initialize(ctx context.Context) error {
	// Build root URI
	var rootURI DocumentURI
	if len(s.workspaceFolders) > 0 {
		rootURI = s.workspaceFolders[0].URI
	}

	params := InitializeParams{
		ProcessID:             os.Getpid(),
		RootURI:               rootURI,
		Capabilities:          DefaultClientCapabilities(),
		InitializationOptions: s.config.InitializationOptions,
		WorkspaceFolders:      s.workspaceFolders,
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result InitializeResult
	if err := s.transport.Call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize request: %w", err)
	}

	s.capabilities = result.Capabilities
	s.serverInfo = result.ServerInfo

	// Send initialized notification
	if err := s.transport.Notify(ctx, "initialized", InitializedParams{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}

	return nil
}

// registerNotificationHandlers sets up handlers for server notifications.
func (s *Server) registerNotificationHandlers() {
	// Diagnostics
	s.transport.OnNotification("textDocument/publishDiagnostics", func(method string, params json.RawMessage) {
		var p PublishDiagnosticsParams
		if err := json.Unmarshal(params, &p); err != nil {
			return
		}

		s.diagnosticsMu.Lock()
		if len(p.Diagnostics) == 0 {
			delete(s.diagnostics, p.URI)
		} else {
			s.diagnostics[p.URI] = p.Diagnostics
		}
		handler := s.diagHandler
		s.diagnosticsMu.Unlock()

		if handler != nil {
			handler(p.URI, p.Diagnostics)
		}
	})

	// Log messages (optional - just consume them)
	s.transport.OnNotification("window/logMessage", func(method string, params json.RawMessage) {
		// Could log these somewhere
	})

	// Show message (optional)
	s.transport.OnNotification("window/showMessage", func(method string, params json.RawMessage) {
		// Could display these to user
	})
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := ServerStatus(s.status.Load())
	if status == ServerStatusStopped || status == ServerStatusShuttingDown {
		return nil
	}

	s.status.Store(int32(ServerStatusShuttingDown))

	// Send shutdown request
	if s.transport != nil && !s.transport.IsClosed() {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		_ = s.transport.Call(shutdownCtx, "shutdown", nil, nil)
		_ = s.transport.Notify(shutdownCtx, "exit", nil)
	}

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Stop process
	s.stopProcess()

	s.status.Store(int32(ServerStatusStopped))
	return nil
}

// Status returns the current server status.
func (s *Server) Status() ServerStatus {
	return ServerStatus(s.status.Load())
}

// Capabilities returns the server's capabilities.
func (s *Server) Capabilities() ServerCapabilities {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capabilities
}

// InitializeServerInfo returns information about the server from initialization.
func (s *Server) InitializeServerInfo() *InitializeServerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serverInfo
}

// LastError returns the last error that occurred.
func (s *Server) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

// LanguageID returns the language this server handles.
func (s *Server) LanguageID() string {
	return s.languageID
}

// ExitChannel returns a channel that receives when the process exits.
func (s *Server) ExitChannel() <-chan error {
	return s.exitCh
}

// OnDiagnostics registers a handler for diagnostic notifications.
func (s *Server) OnDiagnostics(handler func(uri DocumentURI, diagnostics []Diagnostic)) {
	s.diagnosticsMu.Lock()
	s.diagHandler = handler
	s.diagnosticsMu.Unlock()
}

// --- Document Management ---

// OpenDocument notifies the server that a document was opened.
func (s *Server) OpenDocument(ctx context.Context, path, languageID, content string) error {
	if s.Status() != ServerStatusReady {
		return ErrServerNotReady
	}

	uri := FilePathToURI(path)

	s.documentsMu.Lock()
	if _, exists := s.documents[uri]; exists {
		s.documentsMu.Unlock()
		return ErrDocumentAlreadyOpen
	}

	doc := &Document{
		URI:        uri,
		LanguageID: languageID,
		Version:    1,
		Content:    content,
	}
	s.documents[uri] = doc
	s.documentsMu.Unlock()

	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       content,
		},
	}

	return s.transport.Notify(ctx, "textDocument/didOpen", params)
}

// CloseDocument notifies the server that a document was closed.
func (s *Server) CloseDocument(ctx context.Context, path string) error {
	if s.Status() != ServerStatusReady {
		return ErrServerNotReady
	}

	uri := FilePathToURI(path)

	s.documentsMu.Lock()
	if _, exists := s.documents[uri]; !exists {
		s.documentsMu.Unlock()
		return ErrDocumentNotOpen
	}
	delete(s.documents, uri)
	s.documentsMu.Unlock()

	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	return s.transport.Notify(ctx, "textDocument/didClose", params)
}

// ChangeDocument sends document changes to the server.
func (s *Server) ChangeDocument(ctx context.Context, path string, changes []TextDocumentContentChangeEvent) error {
	if s.Status() != ServerStatusReady {
		return ErrServerNotReady
	}

	uri := FilePathToURI(path)

	s.documentsMu.Lock()
	doc, exists := s.documents[uri]
	if !exists {
		s.documentsMu.Unlock()
		return ErrDocumentNotOpen
	}
	doc.Version++
	version := doc.Version

	// Update cached content (for full sync, take the last change)
	for _, change := range changes {
		if change.Range == nil {
			doc.Content = change.Text
		}
		// For incremental sync, we'd need to apply the range edit
	}
	s.documentsMu.Unlock()

	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: TextDocumentIdentifier{URI: uri},
			Version:                version,
		},
		ContentChanges: changes,
	}

	return s.transport.Notify(ctx, "textDocument/didChange", params)
}

// SaveDocument notifies the server that a document was saved.
func (s *Server) SaveDocument(ctx context.Context, path string, content string) error {
	if s.Status() != ServerStatusReady {
		return ErrServerNotReady
	}

	uri := FilePathToURI(path)

	params := DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Text:         content,
	}

	return s.transport.Notify(ctx, "textDocument/didSave", params)
}

// IsDocumentOpen returns true if the document is open.
func (s *Server) IsDocumentOpen(path string) bool {
	uri := FilePathToURI(path)
	s.documentsMu.RLock()
	_, exists := s.documents[uri]
	s.documentsMu.RUnlock()
	return exists
}

// GetDocument returns a copy of the document if open.
func (s *Server) GetDocument(path string) (*Document, bool) {
	uri := FilePathToURI(path)
	s.documentsMu.RLock()
	defer s.documentsMu.RUnlock()

	doc, exists := s.documents[uri]
	if !exists {
		return nil, false
	}

	// Return a copy
	return &Document{
		URI:        doc.URI,
		LanguageID: doc.LanguageID,
		Version:    doc.Version,
		Content:    doc.Content,
	}, true
}

// OpenDocuments returns all open documents.
func (s *Server) OpenDocuments() []*Document {
	s.documentsMu.RLock()
	defer s.documentsMu.RUnlock()

	docs := make([]*Document, 0, len(s.documents))
	for _, doc := range s.documents {
		docs = append(docs, &Document{
			URI:        doc.URI,
			LanguageID: doc.LanguageID,
			Version:    doc.Version,
			Content:    doc.Content,
		})
	}
	return docs
}

// --- Diagnostics ---

// Diagnostics returns the current diagnostics for a file.
func (s *Server) Diagnostics(path string) []Diagnostic {
	uri := FilePathToURI(path)
	s.diagnosticsMu.RLock()
	defer s.diagnosticsMu.RUnlock()
	return s.diagnostics[uri]
}

// AllDiagnostics returns diagnostics for all files.
func (s *Server) AllDiagnostics() map[string][]Diagnostic {
	s.diagnosticsMu.RLock()
	defer s.diagnosticsMu.RUnlock()

	result := make(map[string][]Diagnostic, len(s.diagnostics))
	for uri, diags := range s.diagnostics {
		result[URIToFilePath(uri)] = diags
	}
	return result
}

// --- LSP Requests ---

// Completion requests completion items at a position.
func (s *Server) Completion(ctx context.Context, path string, pos Position) (*CompletionList, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if s.capabilities.CompletionProvider == nil {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     pos,
		},
		Context: &CompletionContext{
			TriggerKind: CompletionTriggerKindInvoked,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result json.RawMessage
	if err := s.transport.Call(ctx, "textDocument/completion", params, &result); err != nil {
		return nil, err
	}

	return ParseCompletionResult(result)
}

// Hover requests hover information at a position.
func (s *Server) Hover(ctx context.Context, path string, pos Position) (*Hover, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.HoverProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := HoverParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     pos,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result *Hover
	if err := s.transport.Call(ctx, "textDocument/hover", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Definition returns the definition location(s) for a symbol.
func (s *Server) Definition(ctx context.Context, path string, pos Position) ([]Location, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.DefinitionProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result json.RawMessage
	if err := s.transport.Call(ctx, "textDocument/definition", params, &result); err != nil {
		return nil, err
	}

	return ParseLocationResult(result)
}

// TypeDefinition returns the type definition location(s).
func (s *Server) TypeDefinition(ctx context.Context, path string, pos Position) ([]Location, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.TypeDefinitionProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result json.RawMessage
	if err := s.transport.Call(ctx, "textDocument/typeDefinition", params, &result); err != nil {
		return nil, err
	}

	return ParseLocationResult(result)
}

// References finds all references to the symbol at a position.
func (s *Server) References(ctx context.Context, path string, pos Position, includeDecl bool) ([]Location, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.ReferencesProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     pos,
		},
		Context: ReferenceContext{
			IncludeDeclaration: includeDecl,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result []Location
	if err := s.transport.Call(ctx, "textDocument/references", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// DocumentSymbols returns symbols in a document.
func (s *Server) DocumentSymbols(ctx context.Context, path string) ([]DocumentSymbol, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.DocumentSymbolProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result []DocumentSymbol
	if err := s.transport.Call(ctx, "textDocument/documentSymbol", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// WorkspaceSymbols searches for symbols in the workspace.
func (s *Server) WorkspaceSymbols(ctx context.Context, query string) ([]SymbolInformation, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.WorkspaceSymbolProvider) {
		return nil, ErrNotSupported
	}

	params := WorkspaceSymbolParams{
		Query: query,
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result []SymbolInformation
	if err := s.transport.Call(ctx, "workspace/symbol", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// CodeActions returns available code actions for a range.
func (s *Server) CodeActions(ctx context.Context, path string, rng Range, diags []Diagnostic) ([]CodeAction, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.CodeActionProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        rng,
		Context: CodeActionContext{
			Diagnostics: diags,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result []CodeAction
	if err := s.transport.Call(ctx, "textDocument/codeAction", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Format formats an entire document.
func (s *Server) Format(ctx context.Context, path string, opts FormattingOptions) ([]TextEdit, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.DocumentFormattingProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := DocumentFormattingParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Options:      opts,
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result []TextEdit
	if err := s.transport.Call(ctx, "textDocument/formatting", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// FormatRange formats a range within a document.
func (s *Server) FormatRange(ctx context.Context, path string, rng Range, opts FormattingOptions) ([]TextEdit, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.DocumentRangeFormattingProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := DocumentRangeFormattingParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        rng,
		Options:      opts,
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result []TextEdit
	if err := s.transport.Call(ctx, "textDocument/rangeFormatting", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Rename renames a symbol.
func (s *Server) Rename(ctx context.Context, path string, pos Position, newName string) (*WorkspaceEdit, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if !HasCapability(s.capabilities.RenameProvider) {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     pos,
		},
		NewName: newName,
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result *WorkspaceEdit
	if err := s.transport.Call(ctx, "textDocument/rename", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// SignatureHelp returns signature help information.
func (s *Server) SignatureHelp(ctx context.Context, path string, pos Position) (*SignatureHelp, error) {
	if s.Status() != ServerStatusReady {
		return nil, ErrServerNotReady
	}

	if s.capabilities.SignatureHelpProvider == nil {
		return nil, ErrNotSupported
	}

	uri := FilePathToURI(path)

	params := SignatureHelpParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     pos,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result *SignatureHelp
	if err := s.transport.Call(ctx, "textDocument/signatureHelp", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// --- Helpers ---

// MatchesFile returns true if this server handles the given file.
func (s *Server) MatchesFile(path string) bool {
	// Check language ID
	langID := DetectLanguageID(path)
	for _, id := range s.config.LanguageIDs {
		if id == langID {
			return true
		}
	}

	// Check file patterns
	base := filepath.Base(path)
	for _, pattern := range s.config.FilePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	return false
}
