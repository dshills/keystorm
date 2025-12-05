package lsp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager coordinates multiple language servers.
// It provides a single entry point for LSP operations,
// automatically routing requests to the appropriate server
// based on file type.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*Server // languageID -> server
	configs map[string]ServerConfig

	// Supervised servers (crash recovery enabled)
	supervisors map[string]*Supervisor // languageID -> supervisor

	workspaceFolders []WorkspaceFolder
	diagnosticsCb    func(uri DocumentURI, diagnostics []Diagnostic)
	supervisorCb     func(event SupervisorEvent)

	// Options
	requestTimeout   time.Duration
	supervisionMode  bool
	supervisorConfig SupervisorConfig
}

// ManagerOption configures the manager.
type ManagerOption func(*Manager)

// WithRequestTimeout sets the default timeout for LSP requests.
func WithRequestTimeout(d time.Duration) ManagerOption {
	return func(m *Manager) {
		m.requestTimeout = d
	}
}

// WithDiagnosticsCallback sets a callback for diagnostics updates.
func WithDiagnosticsCallback(cb func(uri DocumentURI, diagnostics []Diagnostic)) ManagerOption {
	return func(m *Manager) {
		m.diagnosticsCb = cb
	}
}

// WithSupervision enables crash recovery supervision for servers.
func WithSupervision(config SupervisorConfig) ManagerOption {
	return func(m *Manager) {
		m.supervisionMode = true
		m.supervisorConfig = config
	}
}

// WithSupervisorCallback sets a callback for supervisor events.
func WithSupervisorCallback(cb func(event SupervisorEvent)) ManagerOption {
	return func(m *Manager) {
		m.supervisorCb = cb
	}
}

// NewManager creates a new LSP manager.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		servers:          make(map[string]*Server),
		configs:          make(map[string]ServerConfig),
		supervisors:      make(map[string]*Supervisor),
		requestTimeout:   10 * time.Second,
		supervisorConfig: DefaultSupervisorConfig(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// RegisterServer registers a server configuration for a language.
func (m *Manager) RegisterServer(languageID string, config ServerConfig) {
	m.mu.Lock()
	m.configs[languageID] = config
	m.mu.Unlock()
}

// SetWorkspaceFolders sets the workspace folders for all servers.
func (m *Manager) SetWorkspaceFolders(folders []WorkspaceFolder) {
	m.mu.Lock()
	m.workspaceFolders = folders
	m.mu.Unlock()
}

// WorkspaceRoot returns the root path of the first workspace folder, or empty string if none.
func (m *Manager) WorkspaceRoot() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.workspaceFolders) > 0 {
		return URIToFilePath(m.workspaceFolders[0].URI)
	}
	return ""
}

// getOrStartServer returns the server for a language, starting it if needed.
func (m *Manager) getOrStartServer(ctx context.Context, languageID string) (*Server, error) {
	// Check for supervised mode
	if m.supervisionMode {
		return m.getOrStartSupervisedServer(ctx, languageID)
	}

	m.mu.RLock()
	server, exists := m.servers[languageID]
	m.mu.RUnlock()

	if exists && server.Status() == ServerStatusReady {
		return server, nil
	}

	// Need to start server
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if server, exists = m.servers[languageID]; exists && server.Status() == ServerStatusReady {
		return server, nil
	}

	config, hasConfig := m.configs[languageID]
	if !hasConfig {
		return nil, &ServerError{LanguageID: languageID, Err: ErrNoServer}
	}

	// Create and start server
	server = NewServer(config, languageID)

	// Set up diagnostics forwarding
	if m.diagnosticsCb != nil {
		server.OnDiagnostics(m.diagnosticsCb)
	}

	if err := server.Start(ctx, m.workspaceFolders); err != nil {
		return nil, &ServerError{LanguageID: languageID, Err: err}
	}

	m.servers[languageID] = server
	return server, nil
}

// getOrStartSupervisedServer returns a supervised server, starting it if needed.
func (m *Manager) getOrStartSupervisedServer(ctx context.Context, languageID string) (*Server, error) {
	m.mu.RLock()
	supervisor, exists := m.supervisors[languageID]
	m.mu.RUnlock()

	if exists {
		if supervisor.IsReady() {
			return supervisor.Server(), nil
		}
		// Check if supervisor has permanently failed
		if supervisor.State() == SupervisorStateFailed {
			return nil, &ServerError{LanguageID: languageID, Err: ErrSupervisorFailed}
		}
		// Server might be restarting, return the server anyway
		server := supervisor.Server()
		if server != nil {
			return server, nil
		}
		return nil, &ServerError{LanguageID: languageID, Err: ErrServerNotReady}
	}

	// Need to start supervisor
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if supervisor, exists = m.supervisors[languageID]; exists {
		if supervisor.IsReady() {
			return supervisor.Server(), nil
		}
		server := supervisor.Server()
		if server != nil {
			return server, nil
		}
		return nil, &ServerError{LanguageID: languageID, Err: ErrServerNotReady}
	}

	config, hasConfig := m.configs[languageID]
	if !hasConfig {
		return nil, &ServerError{LanguageID: languageID, Err: ErrNoServer}
	}

	// Create supervisor
	supervisor = NewSupervisor(config, languageID, m.supervisorConfig)

	// Set up diagnostics forwarding
	if m.diagnosticsCb != nil {
		supervisor.OnDiagnostics(m.diagnosticsCb)
	}

	// Start event forwarding
	if m.supervisorCb != nil {
		go m.forwardSupervisorEvents(supervisor)
	}

	if err := supervisor.Start(ctx, m.workspaceFolders); err != nil {
		return nil, &ServerError{LanguageID: languageID, Err: err}
	}

	m.supervisors[languageID] = supervisor
	return supervisor.Server(), nil
}

// forwardSupervisorEvents forwards supervisor events to the callback.
func (m *Manager) forwardSupervisorEvents(supervisor *Supervisor) {
	for event := range supervisor.Events() {
		if m.supervisorCb != nil {
			m.supervisorCb(event)
		}
	}
}

// ServerForFile returns the server for a file, starting it if needed.
func (m *Manager) ServerForFile(ctx context.Context, path string) (*Server, error) {
	languageID := DetectLanguageID(path)
	if languageID == "" {
		return nil, ErrNoServer
	}
	return m.getOrStartServer(ctx, languageID)
}

// ServerForLanguage returns the server for a language, starting it if needed.
func (m *Manager) ServerForLanguage(ctx context.Context, languageID string) (*Server, error) {
	return m.getOrStartServer(ctx, languageID)
}

// OpenDocument opens a document with the appropriate server.
func (m *Manager) OpenDocument(ctx context.Context, path, content string) error {
	languageID := DetectLanguageID(path)
	if languageID == "" {
		return nil // No server for this file type
	}

	server, err := m.getOrStartServer(ctx, languageID)
	if err != nil {
		return err
	}

	return server.OpenDocument(ctx, path, languageID, content)
}

// CloseDocument closes a document.
func (m *Manager) CloseDocument(ctx context.Context, path string) error {
	languageID := DetectLanguageID(path)
	if languageID == "" {
		return nil
	}

	m.mu.RLock()
	server, exists := m.servers[languageID]
	m.mu.RUnlock()

	if !exists || server.Status() != ServerStatusReady {
		return nil
	}

	return server.CloseDocument(ctx, path)
}

// ChangeDocument notifies the server of document changes.
func (m *Manager) ChangeDocument(ctx context.Context, path string, changes []TextDocumentContentChangeEvent) error {
	languageID := DetectLanguageID(path)
	if languageID == "" {
		return nil
	}

	m.mu.RLock()
	server, exists := m.servers[languageID]
	m.mu.RUnlock()

	if !exists || server.Status() != ServerStatusReady {
		return nil
	}

	return server.ChangeDocument(ctx, path, changes)
}

// Completion requests completions at a position.
func (m *Manager) Completion(ctx context.Context, path string, pos Position) (*CompletionList, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.Completion(ctx, path, pos)
}

// Hover requests hover information at a position.
func (m *Manager) Hover(ctx context.Context, path string, pos Position) (*Hover, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.Hover(ctx, path, pos)
}

// Definition requests go-to-definition at a position.
func (m *Manager) Definition(ctx context.Context, path string, pos Position) ([]Location, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.Definition(ctx, path, pos)
}

// TypeDefinition requests go-to-type-definition at a position.
func (m *Manager) TypeDefinition(ctx context.Context, path string, pos Position) ([]Location, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.TypeDefinition(ctx, path, pos)
}

// References requests find-references at a position.
func (m *Manager) References(ctx context.Context, path string, pos Position, includeDecl bool) ([]Location, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.References(ctx, path, pos, includeDecl)
}

// DocumentSymbols requests document symbols.
func (m *Manager) DocumentSymbols(ctx context.Context, path string) ([]DocumentSymbol, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.DocumentSymbols(ctx, path)
}

// Format requests document formatting.
func (m *Manager) Format(ctx context.Context, path string, opts FormattingOptions) ([]TextEdit, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.Format(ctx, path, opts)
}

// RangeFormat requests range formatting.
func (m *Manager) RangeFormat(ctx context.Context, path string, rng Range, opts FormattingOptions) ([]TextEdit, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.FormatRange(ctx, path, rng, opts)
}

// CodeActions requests code actions for a range.
func (m *Manager) CodeActions(ctx context.Context, path string, rng Range, diagnostics []Diagnostic) ([]CodeAction, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.CodeActions(ctx, path, rng, diagnostics)
}

// SignatureHelp requests signature help at a position.
func (m *Manager) SignatureHelp(ctx context.Context, path string, pos Position) (*SignatureHelp, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.SignatureHelp(ctx, path, pos)
}

// Rename requests a rename refactoring.
func (m *Manager) Rename(ctx context.Context, path string, pos Position, newName string) (*WorkspaceEdit, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.Rename(ctx, path, pos, newName)
}

// Diagnostics returns cached diagnostics for a document.
func (m *Manager) Diagnostics(ctx context.Context, path string) ([]Diagnostic, error) {
	server, err := m.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return server.Diagnostics(path), nil
}

// IsAvailable checks if LSP is available for a file.
func (m *Manager) IsAvailable(path string) bool {
	languageID := DetectLanguageID(path)
	if languageID == "" {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if we have a config
	if _, hasConfig := m.configs[languageID]; hasConfig {
		return true
	}

	// Check if server is running
	if server, exists := m.servers[languageID]; exists {
		return server.Status() == ServerStatusReady
	}

	return false
}

// Shutdown gracefully shuts down all servers and supervisors.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.servers = make(map[string]*Server)

	supervisors := make([]*Supervisor, 0, len(m.supervisors))
	for _, s := range m.supervisors {
		supervisors = append(supervisors, s)
	}
	m.supervisors = make(map[string]*Supervisor)
	m.mu.Unlock()

	var errs []error

	// Shutdown supervised servers first
	for _, s := range supervisors {
		if err := s.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// Shutdown unsupervised servers
	for _, s := range servers {
		if err := s.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ServerStatus returns the status of a language server.
func (m *Manager) ServerStatus(languageID string) ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check supervisors first
	if supervisor, exists := m.supervisors[languageID]; exists {
		if server := supervisor.Server(); server != nil {
			return server.Status()
		}
		return ServerStatusStopped
	}

	server, exists := m.servers[languageID]
	if !exists {
		return ServerStatusStopped
	}
	return server.Status()
}

// SupervisorStats returns statistics for a supervised server.
func (m *Manager) SupervisorStats(languageID string) (SupervisorStats, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	supervisor, exists := m.supervisors[languageID]
	if !exists {
		return SupervisorStats{}, false
	}
	return supervisor.Stats(), true
}

// IsSupervised returns true if supervision mode is enabled.
func (m *Manager) IsSupervised() bool {
	return m.supervisionMode
}

// RegisteredLanguages returns the list of languages with registered servers.
func (m *Manager) RegisteredLanguages() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	langs := make([]string, 0, len(m.configs))
	for lang := range m.configs {
		langs = append(langs, lang)
	}
	return langs
}

// DefaultServerConfigs returns default configurations for common language servers.
func DefaultServerConfigs() map[string]ServerConfig {
	return map[string]ServerConfig{
		"go": {
			Command: "gopls",
			Args:    []string{"serve"},
		},
		"rust": {
			Command: "rust-analyzer",
		},
		"typescript": {
			Command: "typescript-language-server",
			Args:    []string{"--stdio"},
		},
		"javascript": {
			Command: "typescript-language-server",
			Args:    []string{"--stdio"},
		},
		"python": {
			Command: "pylsp",
		},
		"c": {
			Command: "clangd",
		},
		"cpp": {
			Command: "clangd",
		},
	}
}

// AutoDetectServers detects available language servers on the system.
func AutoDetectServers() map[string]ServerConfig {
	defaults := DefaultServerConfigs()
	available := make(map[string]ServerConfig)

	for lang, config := range defaults {
		// Check if command exists
		if _, err := exec.LookPath(config.Command); err == nil {
			available[lang] = config
		}
	}

	return available
}

// ManagedServerInfo provides information about a running server.
type ManagedServerInfo struct {
	LanguageID   string
	Status       ServerStatus
	Capabilities ServerCapabilities
	DocumentURIs []DocumentURI
}

// ServerInfos returns information about all servers.
func (m *Manager) ServerInfos() []ManagedServerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]ManagedServerInfo, 0, len(m.servers))
	for langID, server := range m.servers {
		docs := server.OpenDocuments()
		uris := make([]DocumentURI, len(docs))
		for i, doc := range docs {
			uris[i] = doc.URI
		}
		info := ManagedServerInfo{
			LanguageID:   langID,
			Status:       server.Status(),
			Capabilities: server.Capabilities(),
			DocumentURIs: uris,
		}
		infos = append(infos, info)
	}
	return infos
}

// WorkspaceFolderFromPath creates a workspace folder from a directory path.
func WorkspaceFolderFromPath(path string) WorkspaceFolder {
	absPath, _ := filepath.Abs(path)
	name := filepath.Base(absPath)
	return WorkspaceFolder{
		URI:  FilePathToURI(absPath),
		Name: name,
	}
}

// DetectWorkspaceFolders detects workspace folders from common project markers.
func DetectWorkspaceFolders(root string) []WorkspaceFolder {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return []WorkspaceFolder{WorkspaceFolderFromPath(root)}
	}

	// Common project markers
	markers := []string{
		"go.mod",
		"package.json",
		"Cargo.toml",
		"pyproject.toml",
		"setup.py",
		".git",
	}

	// Check root directory for markers
	for _, marker := range markers {
		markerPath := filepath.Join(absRoot, marker)
		if fileExists(markerPath) {
			return []WorkspaceFolder{WorkspaceFolderFromPath(absRoot)}
		}
	}

	// If no markers found, use root as workspace
	return []WorkspaceFolder{WorkspaceFolderFromPath(absRoot)}
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// LanguageIDForExtension returns the language ID for a file extension.
func LanguageIDForExtension(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	// Map of extension to language ID
	extMap := map[string]string{
		"go":     "go",
		"rs":     "rust",
		"ts":     "typescript",
		"tsx":    "typescriptreact",
		"js":     "javascript",
		"jsx":    "javascriptreact",
		"py":     "python",
		"c":      "c",
		"h":      "c",
		"cpp":    "cpp",
		"cc":     "cpp",
		"cxx":    "cpp",
		"hpp":    "cpp",
		"hxx":    "cpp",
		"java":   "java",
		"rb":     "ruby",
		"php":    "php",
		"swift":  "swift",
		"kt":     "kotlin",
		"kts":    "kotlin",
		"scala":  "scala",
		"lua":    "lua",
		"sh":     "shellscript",
		"bash":   "shellscript",
		"zsh":    "shellscript",
		"json":   "json",
		"yaml":   "yaml",
		"yml":    "yaml",
		"xml":    "xml",
		"html":   "html",
		"htm":    "html",
		"css":    "css",
		"scss":   "scss",
		"sass":   "sass",
		"less":   "less",
		"md":     "markdown",
		"sql":    "sql",
		"r":      "r",
		"cs":     "csharp",
		"fs":     "fsharp",
		"vb":     "vb",
		"dart":   "dart",
		"ex":     "elixir",
		"exs":    "elixir",
		"erl":    "erlang",
		"hrl":    "erlang",
		"hs":     "haskell",
		"lhs":    "haskell",
		"ml":     "ocaml",
		"mli":    "ocaml",
		"clj":    "clojure",
		"cljs":   "clojurescript",
		"vim":    "vim",
		"proto":  "protobuf",
		"tf":     "terraform",
		"tfvars": "terraform",
		"vue":    "vue",
		"svelte": "svelte",
		"zig":    "zig",
		"nim":    "nim",
		"cr":     "crystal",
		"jl":     "julia",
	}

	if langID, ok := extMap[ext]; ok {
		return langID
	}
	return ""
}

// RestartServer restarts a language server.
func (m *Manager) RestartServer(ctx context.Context, languageID string) error {
	m.mu.Lock()
	server, exists := m.servers[languageID]
	if exists {
		delete(m.servers, languageID)
	}
	m.mu.Unlock()

	// Shutdown existing server
	if exists && server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		server.Shutdown(shutdownCtx)
		cancel()
	}

	// Check if we have a config to restart
	m.mu.RLock()
	_, hasConfig := m.configs[languageID]
	m.mu.RUnlock()

	if !hasConfig {
		return fmt.Errorf("no configuration for language: %s", languageID)
	}

	// Start will happen automatically on next request
	return nil
}
