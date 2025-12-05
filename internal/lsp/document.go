package lsp

import (
	"context"
	"sync"
	"time"
)

// DocumentManager tracks open documents and synchronizes changes with LSP servers.
// It provides centralized document management with change debouncing and
// integration between the editor's buffer system and LSP servers.
type DocumentManager struct {
	mu        sync.RWMutex
	documents map[DocumentURI]*ManagedDocument
	manager   *Manager

	// Change debouncing
	debounceDelay time.Duration
	pendingTimers map[DocumentURI]*time.Timer

	// Callbacks
	onDiagnostics func(uri DocumentURI, diagnostics []Diagnostic)
}

// ManagedDocument represents an open document with its state and metadata.
type ManagedDocument struct {
	URI        DocumentURI
	Path       string
	LanguageID string
	Version    int
	Content    string

	// Tracking
	OpenedAt   time.Time
	ModifiedAt time.Time
	IsDirty    bool

	// Sync state
	SyncKind TextDocumentSyncKind
	LastSync time.Time
}

// DocumentManagerOption configures the document manager.
type DocumentManagerOption func(*DocumentManager)

// WithDebounceDelay sets the delay before sending changes to the server.
func WithDebounceDelay(d time.Duration) DocumentManagerOption {
	return func(dm *DocumentManager) {
		dm.debounceDelay = d
	}
}

// WithDiagnosticsHandler sets a callback for diagnostics updates.
func WithDiagnosticsHandler(handler func(uri DocumentURI, diagnostics []Diagnostic)) DocumentManagerOption {
	return func(dm *DocumentManager) {
		dm.onDiagnostics = handler
	}
}

// NewDocumentManager creates a new document manager.
func NewDocumentManager(mgr *Manager, opts ...DocumentManagerOption) *DocumentManager {
	dm := &DocumentManager{
		documents:     make(map[DocumentURI]*ManagedDocument),
		manager:       mgr,
		debounceDelay: 300 * time.Millisecond,
		pendingTimers: make(map[DocumentURI]*time.Timer),
	}

	for _, opt := range opts {
		opt(dm)
	}

	return dm
}

// OpenDocument opens a document for editing.
// This notifies the appropriate LSP server that the document is now open.
func (dm *DocumentManager) OpenDocument(path, languageID, content string) error {
	uri := FilePathToURI(path)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Check if already open
	if _, exists := dm.documents[uri]; exists {
		return ErrDocumentAlreadyOpen
	}

	now := time.Now()
	doc := &ManagedDocument{
		URI:        uri,
		Path:       path,
		LanguageID: languageID,
		Version:    1,
		Content:    content,
		OpenedAt:   now,
		ModifiedAt: now,
		IsDirty:    false,
		SyncKind:   TextDocumentSyncKindFull, // Default to full sync
		LastSync:   now,
	}

	dm.documents[uri] = doc

	// Notify LSP server (if available)
	if dm.manager != nil {
		go dm.manager.OpenDocument(timeoutCtx(), path, content)
	}

	return nil
}

// CloseDocument closes a document.
// This notifies the LSP server that the document is no longer being edited.
func (dm *DocumentManager) CloseDocument(path string) error {
	uri := FilePathToURI(path)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, exists := dm.documents[uri]
	if !exists {
		return ErrDocumentNotOpen
	}

	// Cancel any pending timer
	if timer, ok := dm.pendingTimers[uri]; ok {
		timer.Stop()
		delete(dm.pendingTimers, uri)
	}

	delete(dm.documents, uri)

	// Notify LSP server
	if dm.manager != nil {
		go dm.manager.CloseDocument(timeoutCtx(), doc.Path)
	}

	return nil
}

// ChangeDocument records a change to a document.
// Changes are debounced before being sent to the LSP server.
func (dm *DocumentManager) ChangeDocument(path string, changes []TextDocumentContentChangeEvent) error {
	uri := FilePathToURI(path)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, exists := dm.documents[uri]
	if !exists {
		return ErrDocumentNotOpen
	}

	doc.Version++
	doc.ModifiedAt = time.Now()
	doc.IsDirty = true

	// Apply changes to cached content
	for _, change := range changes {
		if change.Range == nil {
			// Full document replacement
			doc.Content = change.Text
		} else {
			// Incremental change - apply to content
			doc.Content = applyTextChange(doc.Content, *change.Range, change.Text)
		}
	}

	// Cancel existing timer if any
	if timer, ok := dm.pendingTimers[uri]; ok {
		timer.Stop()
	}

	// Schedule debounced sync
	dm.pendingTimers[uri] = time.AfterFunc(dm.debounceDelay, func() {
		dm.syncDocument(uri)
	})

	return nil
}

// ReplaceContent replaces the entire document content.
// This is a convenience method for full document updates.
func (dm *DocumentManager) ReplaceContent(path, content string) error {
	return dm.ChangeDocument(path, []TextDocumentContentChangeEvent{
		{Text: content},
	})
}

// syncDocument sends pending changes to the LSP server.
func (dm *DocumentManager) syncDocument(uri DocumentURI) {
	dm.mu.Lock()
	doc, exists := dm.documents[uri]
	if !exists {
		dm.mu.Unlock()
		return
	}

	// Clear pending timer
	delete(dm.pendingTimers, uri)

	// Capture state
	content := doc.Content
	version := doc.Version
	path := doc.Path
	syncKind := doc.SyncKind

	doc.LastSync = time.Now()
	dm.mu.Unlock()

	// Send to LSP server
	if dm.manager == nil {
		return
	}

	var changes []TextDocumentContentChangeEvent

	switch syncKind {
	case TextDocumentSyncKindFull:
		// Send entire document
		changes = []TextDocumentContentChangeEvent{{Text: content}}
	case TextDocumentSyncKindIncremental:
		// TODO: Track incremental changes
		// For now, fall back to full sync
		changes = []TextDocumentContentChangeEvent{{Text: content}}
	default:
		return // TextDocumentSyncKindNone
	}

	_ = version // Version is managed by the server
	dm.manager.ChangeDocument(timeoutCtx(), path, changes)
}

// FlushPending immediately sends any pending changes to the LSP server.
func (dm *DocumentManager) FlushPending(path string) {
	uri := FilePathToURI(path)

	dm.mu.Lock()
	if timer, ok := dm.pendingTimers[uri]; ok {
		timer.Stop()
		delete(dm.pendingTimers, uri)
	}
	dm.mu.Unlock()

	dm.syncDocument(uri)
}

// FlushAll immediately sends all pending changes to LSP servers.
func (dm *DocumentManager) FlushAll() {
	dm.mu.Lock()
	uris := make([]DocumentURI, 0, len(dm.pendingTimers))
	for uri, timer := range dm.pendingTimers {
		timer.Stop()
		uris = append(uris, uri)
	}
	dm.pendingTimers = make(map[DocumentURI]*time.Timer)
	dm.mu.Unlock()

	for _, uri := range uris {
		dm.syncDocument(uri)
	}
}

// SaveDocument notifies the LSP server that a document was saved.
func (dm *DocumentManager) SaveDocument(path string) error {
	uri := FilePathToURI(path)

	dm.mu.Lock()
	doc, exists := dm.documents[uri]
	if !exists {
		dm.mu.Unlock()
		return ErrDocumentNotOpen
	}

	content := doc.Content
	doc.IsDirty = false
	dm.mu.Unlock()

	// Flush any pending changes first
	dm.FlushPending(path)

	// Notify LSP server
	if dm.manager != nil {
		server, err := dm.manager.ServerForFile(timeoutCtx(), path)
		if err == nil {
			server.SaveDocument(timeoutCtx(), path, content)
		}
	}

	return nil
}

// GetDocument returns the managed document for a path.
func (dm *DocumentManager) GetDocument(path string) (*ManagedDocument, bool) {
	uri := FilePathToURI(path)

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	doc, exists := dm.documents[uri]
	if !exists {
		return nil, false
	}

	// Return a copy
	copy := *doc
	return &copy, true
}

// GetContent returns the current content of a document.
func (dm *DocumentManager) GetContent(path string) (string, bool) {
	doc, exists := dm.GetDocument(path)
	if !exists {
		return "", false
	}
	return doc.Content, true
}

// GetVersion returns the current version of a document.
func (dm *DocumentManager) GetVersion(path string) (int, bool) {
	doc, exists := dm.GetDocument(path)
	if !exists {
		return 0, false
	}
	return doc.Version, true
}

// IsOpen returns true if a document is open.
func (dm *DocumentManager) IsOpen(path string) bool {
	uri := FilePathToURI(path)

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	_, exists := dm.documents[uri]
	return exists
}

// IsDirty returns true if a document has unsaved changes.
func (dm *DocumentManager) IsDirty(path string) bool {
	doc, exists := dm.GetDocument(path)
	if !exists {
		return false
	}
	return doc.IsDirty
}

// OpenDocuments returns all open document URIs.
func (dm *DocumentManager) OpenDocuments() []DocumentURI {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	uris := make([]DocumentURI, 0, len(dm.documents))
	for uri := range dm.documents {
		uris = append(uris, uri)
	}
	return uris
}

// OpenDocumentPaths returns all open document paths.
func (dm *DocumentManager) OpenDocumentPaths() []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	paths := make([]string, 0, len(dm.documents))
	for _, doc := range dm.documents {
		paths = append(paths, doc.Path)
	}
	return paths
}

// CloseAll closes all open documents.
func (dm *DocumentManager) CloseAll() {
	dm.mu.Lock()
	paths := make([]string, 0, len(dm.documents))
	for _, doc := range dm.documents {
		paths = append(paths, doc.Path)
	}
	dm.mu.Unlock()

	for _, path := range paths {
		dm.CloseDocument(path)
	}
}

// SetSyncKind sets the sync kind for a document.
func (dm *DocumentManager) SetSyncKind(path string, kind TextDocumentSyncKind) error {
	uri := FilePathToURI(path)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, exists := dm.documents[uri]
	if !exists {
		return ErrDocumentNotOpen
	}

	doc.SyncKind = kind
	return nil
}

// applyTextChange applies an incremental text change to content.
func applyTextChange(content string, rng Range, newText string) string {
	lines := splitLines(content)

	// Calculate start and end positions
	startLine := rng.Start.Line
	startChar := rng.Start.Character
	endLine := rng.End.Line
	endChar := rng.End.Character

	// Clamp to valid ranges
	if startLine < 0 {
		startLine = 0
	}
	if startLine >= len(lines) {
		// Appending to end
		return content + newText
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
		endChar = len(lines[endLine])
	}

	// Clamp character positions
	if startChar < 0 {
		startChar = 0
	}
	if startChar > len(lines[startLine]) {
		startChar = len(lines[startLine])
	}
	if endChar < 0 {
		endChar = 0
	}
	if endChar > len(lines[endLine]) {
		endChar = len(lines[endLine])
	}

	// Build result
	var result string

	// Content before the change
	for i := 0; i < startLine; i++ {
		result += lines[i] + "\n"
	}
	result += lines[startLine][:startChar]

	// Insert new text
	result += newText

	// Content after the change
	result += lines[endLine][endChar:]
	if endLine < len(lines)-1 {
		result += "\n"
	}
	for i := endLine + 1; i < len(lines); i++ {
		result += lines[i]
		if i < len(lines)-1 {
			result += "\n"
		}
	}

	return result
}

// splitLines splits content into lines, preserving empty lines.
func splitLines(content string) []string {
	if content == "" {
		return []string{""}
	}

	var lines []string
	var current string

	for _, ch := range content {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}

	// Add last line if not empty or if content didn't end with newline
	lines = append(lines, current)

	return lines
}

// timeoutCtx creates a context with a standard timeout for LSP operations.
// Note: The cancel function is intentionally not returned since these are
// fire-and-forget operations that should complete within the timeout.
func timeoutCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return ctx
}

// DocumentStats provides statistics about open documents.
type DocumentStats struct {
	TotalOpen     int
	TotalDirty    int
	ByLanguage    map[string]int
	PendingSyncs  int
	OldestOpenAge time.Duration
}

// Stats returns statistics about open documents.
func (dm *DocumentManager) Stats() DocumentStats {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	stats := DocumentStats{
		TotalOpen:  len(dm.documents),
		ByLanguage: make(map[string]int),
	}

	var oldestOpen time.Time
	now := time.Now()

	for _, doc := range dm.documents {
		if doc.IsDirty {
			stats.TotalDirty++
		}
		stats.ByLanguage[doc.LanguageID]++

		if oldestOpen.IsZero() || doc.OpenedAt.Before(oldestOpen) {
			oldestOpen = doc.OpenedAt
		}
	}

	if !oldestOpen.IsZero() {
		stats.OldestOpenAge = now.Sub(oldestOpen)
	}

	stats.PendingSyncs = len(dm.pendingTimers)

	return stats
}
