// Package project provides project management functionality.
package project

import (
	"context"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
	"github.com/dshills/keystorm/internal/project/filestore"
	"github.com/dshills/keystorm/internal/project/index"
)

// Provider defines the interface for project operations exposed to plugins.
// This interface provides a subset of Project functionality suitable for plugins.
type Provider interface {
	// Workspace operations
	IsOpen() bool
	Roots() []string

	// File operations
	OpenFile(ctx context.Context, path string) (*filestore.Document, error)
	SaveFile(ctx context.Context, path string) error
	CloseFile(ctx context.Context, path string) error
	CreateFile(ctx context.Context, path string, content []byte) error
	DeleteFile(ctx context.Context, path string) error

	// Directory operations
	ListDirectory(ctx context.Context, path string) ([]index.FileInfo, error)

	// Search operations
	FindFiles(ctx context.Context, pattern string, opts FindOptions) ([]FileMatch, error)
	SearchContent(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error)

	// Graph operations
	RelatedFiles(ctx context.Context, path string) ([]RelatedFile, error)

	// Document operations
	OpenDocuments() []*filestore.Document
	DirtyDocuments() []*filestore.Document

	// Status operations
	IndexStatus() IndexStatus
	WatcherStatus() WatcherStatus
}

// Module implements the ks.project API module for Lua plugins.
type Module struct {
	provider Provider
}

// NewModule creates a new project module for plugin integration.
func NewModule(provider Provider) *Module {
	return &Module{provider: provider}
}

// Name returns the module name.
func (m *Module) Name() string {
	return "project"
}

// RequiredCapability returns the capability required for this module.
func (m *Module) RequiredCapability() security.Capability {
	return security.CapabilityProject
}

// Register registers the module into the Lua state.
func (m *Module) Register(L *lua.LState) error {
	mod := L.NewTable()

	// Workspace operations
	L.SetField(mod, "is_open", L.NewFunction(m.isOpen))
	L.SetField(mod, "roots", L.NewFunction(m.roots))

	// File operations
	L.SetField(mod, "open_file", L.NewFunction(m.openFile))
	L.SetField(mod, "save_file", L.NewFunction(m.saveFile))
	L.SetField(mod, "close_file", L.NewFunction(m.closeFile))
	L.SetField(mod, "create_file", L.NewFunction(m.createFile))
	L.SetField(mod, "delete_file", L.NewFunction(m.deleteFile))

	// Directory operations
	L.SetField(mod, "list_directory", L.NewFunction(m.listDirectory))

	// Search operations
	L.SetField(mod, "find_files", L.NewFunction(m.findFiles))
	L.SetField(mod, "search_content", L.NewFunction(m.searchContent))

	// Graph operations
	L.SetField(mod, "related_files", L.NewFunction(m.relatedFiles))

	// Document operations
	L.SetField(mod, "open_documents", L.NewFunction(m.openDocuments))
	L.SetField(mod, "dirty_documents", L.NewFunction(m.dirtyDocuments))

	// Status operations
	L.SetField(mod, "index_status", L.NewFunction(m.indexStatus))
	L.SetField(mod, "watcher_status", L.NewFunction(m.watcherStatus))

	L.SetGlobal("_ks_project", mod)
	return nil
}

// Workspace operations

// is_open() -> bool
// Returns true if a workspace is open.
func (m *Module) isOpen(L *lua.LState) int {
	L.Push(lua.LBool(m.provider.IsOpen()))
	return 1
}

// roots() -> table
// Returns the workspace root directories.
func (m *Module) roots(L *lua.LState) int {
	roots := m.provider.Roots()
	tbl := L.NewTable()
	for i, root := range roots {
		L.RawSetInt(tbl, i+1, lua.LString(root))
	}
	L.Push(tbl)
	return 1
}

// File operations

// open_file(path) -> table, error
// Opens a file and returns a document table or nil with error message.
func (m *Module) openFile(L *lua.LState) int {
	path := L.CheckString(1)

	doc, err := m.provider.OpenFile(context.Background(), path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	L.Push(m.documentToTable(L, doc))
	L.Push(lua.LNil)
	return 2
}

// save_file(path) -> nil, error
// Saves a file. Returns nil on success, or nil with error message.
func (m *Module) saveFile(L *lua.LState) int {
	path := L.CheckString(1)

	err := m.provider.SaveFile(context.Background(), path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	L.Push(lua.LTrue)
	L.Push(lua.LNil)
	return 2
}

// close_file(path) -> nil, error
// Closes a file. Returns nil on success, or nil with error message.
func (m *Module) closeFile(L *lua.LState) int {
	path := L.CheckString(1)

	err := m.provider.CloseFile(context.Background(), path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	L.Push(lua.LTrue)
	L.Push(lua.LNil)
	return 2
}

// create_file(path, content?) -> nil, error
// Creates a new file with optional content.
func (m *Module) createFile(L *lua.LState) int {
	path := L.CheckString(1)
	var content []byte
	if L.GetTop() >= 2 && L.Get(2) != lua.LNil {
		content = []byte(L.CheckString(2))
	}

	err := m.provider.CreateFile(context.Background(), path, content)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	L.Push(lua.LTrue)
	L.Push(lua.LNil)
	return 2
}

// delete_file(path) -> nil, error
// Deletes a file.
func (m *Module) deleteFile(L *lua.LState) int {
	path := L.CheckString(1)

	err := m.provider.DeleteFile(context.Background(), path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	L.Push(lua.LTrue)
	L.Push(lua.LNil)
	return 2
}

// Directory operations

// list_directory(path) -> table, error
// Lists directory contents.
func (m *Module) listDirectory(L *lua.LState) int {
	path := L.CheckString(1)

	entries, err := m.provider.ListDirectory(context.Background(), path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	tbl := L.NewTable()
	for i, entry := range entries {
		entryTbl := L.NewTable()
		L.SetField(entryTbl, "name", lua.LString(entry.Name))
		L.SetField(entryTbl, "path", lua.LString(entry.Path))
		L.SetField(entryTbl, "is_dir", lua.LBool(entry.IsDir))
		L.SetField(entryTbl, "size", lua.LNumber(entry.Size))
		L.RawSetInt(tbl, i+1, entryTbl)
	}

	L.Push(tbl)
	L.Push(lua.LNil)
	return 2
}

// Search operations

// find_files(pattern, opts?) -> table, error
// Finds files matching a pattern.
func (m *Module) findFiles(L *lua.LState) int {
	pattern := L.CheckString(1)

	opts := FindOptions{
		MaxResults: 100, // default
	}

	// Parse options table if provided
	if L.GetTop() >= 2 && L.Get(2) != lua.LNil {
		optsTbl := L.CheckTable(2)
		if maxResults := optsTbl.RawGetString("max_results"); maxResults != lua.LNil {
			if n, ok := maxResults.(lua.LNumber); ok {
				opts.MaxResults = int(n)
			}
		}
	}

	matches, err := m.provider.FindFiles(context.Background(), pattern, opts)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	tbl := L.NewTable()
	for i, match := range matches {
		matchTbl := L.NewTable()
		L.SetField(matchTbl, "path", lua.LString(match.Path))
		L.SetField(matchTbl, "name", lua.LString(match.Name))
		L.SetField(matchTbl, "score", lua.LNumber(match.Score))
		L.RawSetInt(tbl, i+1, matchTbl)
	}

	L.Push(tbl)
	L.Push(lua.LNil)
	return 2
}

// search_content(query, opts?) -> table, error
// Searches file contents for a query string.
func (m *Module) searchContent(L *lua.LState) int {
	query := L.CheckString(1)

	opts := SearchOptions{
		MaxResults: 100, // default
	}

	// Parse options table if provided
	if L.GetTop() >= 2 && L.Get(2) != lua.LNil {
		optsTbl := L.CheckTable(2)
		if maxResults := optsTbl.RawGetString("max_results"); maxResults != lua.LNil {
			if n, ok := maxResults.(lua.LNumber); ok {
				opts.MaxResults = int(n)
			}
		}
		if caseSensitive := optsTbl.RawGetString("case_sensitive"); caseSensitive != lua.LNil {
			if b, ok := caseSensitive.(lua.LBool); ok {
				opts.CaseSensitive = bool(b)
			}
		}
		if wholeWord := optsTbl.RawGetString("whole_word"); wholeWord != lua.LNil {
			if b, ok := wholeWord.(lua.LBool); ok {
				opts.WholeWord = bool(b)
			}
		}
		if useRegex := optsTbl.RawGetString("use_regex"); useRegex != lua.LNil {
			if b, ok := useRegex.(lua.LBool); ok {
				opts.UseRegex = bool(b)
			}
		}
	}

	matches, err := m.provider.SearchContent(context.Background(), query, opts)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	tbl := L.NewTable()
	for i, match := range matches {
		matchTbl := L.NewTable()
		L.SetField(matchTbl, "path", lua.LString(match.Path))
		L.SetField(matchTbl, "line", lua.LNumber(match.Line))
		L.SetField(matchTbl, "column", lua.LNumber(match.Column))
		L.SetField(matchTbl, "text", lua.LString(match.Text))
		L.RawSetInt(tbl, i+1, matchTbl)
	}

	L.Push(tbl)
	L.Push(lua.LNil)
	return 2
}

// Graph operations

// related_files(path) -> table, error
// Returns files related to the given file.
func (m *Module) relatedFiles(L *lua.LState) int {
	path := L.CheckString(1)

	related, err := m.provider.RelatedFiles(context.Background(), path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	tbl := L.NewTable()
	for i, rel := range related {
		relTbl := L.NewTable()
		L.SetField(relTbl, "path", lua.LString(rel.Path))
		L.SetField(relTbl, "relationship", lua.LString(rel.Relationship))
		L.SetField(relTbl, "score", lua.LNumber(rel.Score))
		L.RawSetInt(tbl, i+1, relTbl)
	}

	L.Push(tbl)
	L.Push(lua.LNil)
	return 2
}

// Document operations

// open_documents() -> table
// Returns all open documents.
func (m *Module) openDocuments(L *lua.LState) int {
	docs := m.provider.OpenDocuments()

	tbl := L.NewTable()
	for i, doc := range docs {
		L.RawSetInt(tbl, i+1, m.documentToTable(L, doc))
	}

	L.Push(tbl)
	return 1
}

// dirty_documents() -> table
// Returns all documents with unsaved changes.
func (m *Module) dirtyDocuments(L *lua.LState) int {
	docs := m.provider.DirtyDocuments()

	tbl := L.NewTable()
	for i, doc := range docs {
		L.RawSetInt(tbl, i+1, m.documentToTable(L, doc))
	}

	L.Push(tbl)
	return 1
}

// Status operations

// index_status() -> table
// Returns the indexing status.
func (m *Module) indexStatus(L *lua.LState) int {
	status := m.provider.IndexStatus()

	tbl := L.NewTable()
	L.SetField(tbl, "status", lua.LString(status.Status))
	L.SetField(tbl, "indexed_files", lua.LNumber(status.IndexedFiles))
	L.SetField(tbl, "total_files", lua.LNumber(status.TotalFiles))
	L.SetField(tbl, "error_files", lua.LNumber(status.ErrorFiles))
	L.SetField(tbl, "bytes_processed", lua.LNumber(status.BytesProcessed))

	L.Push(tbl)
	return 1
}

// watcher_status() -> table
// Returns the file watcher status.
func (m *Module) watcherStatus(L *lua.LState) int {
	status := m.provider.WatcherStatus()

	tbl := L.NewTable()
	L.SetField(tbl, "watched_paths", lua.LNumber(status.WatchedPaths))
	L.SetField(tbl, "pending_events", lua.LNumber(status.PendingEvents))
	L.SetField(tbl, "total_events", lua.LNumber(status.TotalEvents))
	L.SetField(tbl, "errors", lua.LNumber(status.Errors))
	if status.LastError != nil {
		L.SetField(tbl, "last_error", lua.LString(status.LastError.Error()))
	} else {
		L.SetField(tbl, "last_error", lua.LNil)
	}

	L.Push(tbl)
	return 1
}

// Helper functions

// documentToTable converts a filestore.Document to a Lua table.
func (m *Module) documentToTable(L *lua.LState, doc *filestore.Document) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "path", lua.LString(doc.Path))
	L.SetField(tbl, "is_dirty", lua.LBool(doc.IsDirty()))
	L.SetField(tbl, "version", lua.LNumber(doc.GetVersion()))
	L.SetField(tbl, "language", lua.LString(doc.LanguageID))
	return tbl
}

// Adapter adapts a Project to the Provider interface.
type Adapter struct {
	project Project
}

// NewAdapter creates a new adapter that wraps a Project.
func NewAdapter(proj Project) *Adapter {
	return &Adapter{project: proj}
}

// IsOpen returns true if a workspace is open.
func (a *Adapter) IsOpen() bool {
	return a.project.IsOpen()
}

// Roots returns the workspace root directories.
func (a *Adapter) Roots() []string {
	return a.project.Roots()
}

// OpenFile opens a file and returns its document.
func (a *Adapter) OpenFile(ctx context.Context, path string) (*filestore.Document, error) {
	return a.project.OpenFile(ctx, path)
}

// SaveFile saves a file.
func (a *Adapter) SaveFile(ctx context.Context, path string) error {
	return a.project.SaveFile(ctx, path)
}

// CloseFile closes a file.
func (a *Adapter) CloseFile(ctx context.Context, path string) error {
	return a.project.CloseFile(ctx, path)
}

// CreateFile creates a new file.
func (a *Adapter) CreateFile(ctx context.Context, path string, content []byte) error {
	return a.project.CreateFile(ctx, path, content)
}

// DeleteFile deletes a file.
func (a *Adapter) DeleteFile(ctx context.Context, path string) error {
	return a.project.DeleteFile(ctx, path)
}

// ListDirectory lists directory contents.
func (a *Adapter) ListDirectory(ctx context.Context, path string) ([]index.FileInfo, error) {
	return a.project.ListDirectory(ctx, path)
}

// FindFiles finds files matching a pattern.
func (a *Adapter) FindFiles(ctx context.Context, pattern string, opts FindOptions) ([]FileMatch, error) {
	return a.project.FindFiles(ctx, pattern, opts)
}

// SearchContent searches file contents.
func (a *Adapter) SearchContent(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error) {
	return a.project.SearchContent(ctx, query, opts)
}

// RelatedFiles returns files related to the given file.
func (a *Adapter) RelatedFiles(ctx context.Context, path string) ([]RelatedFile, error) {
	return a.project.RelatedFiles(ctx, path)
}

// OpenDocuments returns all open documents.
func (a *Adapter) OpenDocuments() []*filestore.Document {
	return a.project.OpenDocuments()
}

// DirtyDocuments returns documents with unsaved changes.
func (a *Adapter) DirtyDocuments() []*filestore.Document {
	return a.project.DirtyDocuments()
}

// IndexStatus returns the indexing status.
func (a *Adapter) IndexStatus() IndexStatus {
	return a.project.IndexStatus()
}

// WatcherStatus returns the file watcher status.
func (a *Adapter) WatcherStatus() WatcherStatus {
	return a.project.WatcherStatus()
}

// Ensure Adapter implements Provider.
var _ Provider = (*Adapter)(nil)
