package project

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/project/filestore"
	"github.com/dshills/keystorm/internal/project/graph"
	"github.com/dshills/keystorm/internal/project/index"
	"github.com/dshills/keystorm/internal/project/search"
	"github.com/dshills/keystorm/internal/project/vfs"
	"github.com/dshills/keystorm/internal/project/watcher"
	"github.com/dshills/keystorm/internal/project/workspace"
)

// Project is the main interface for workspace and file operations.
// It provides a unified API for file management, search, and project intelligence.
type Project interface {
	// Lifecycle
	Open(ctx context.Context, roots ...string) error
	Close(ctx context.Context) error
	IsOpen() bool

	// Workspace info
	Root() string
	Roots() []string
	IsInWorkspace(path string) bool
	Workspace() *workspace.Workspace

	// File operations
	OpenFile(ctx context.Context, path string) (*filestore.Document, error)
	SaveFile(ctx context.Context, path string) error
	SaveFileAs(ctx context.Context, oldPath, newPath string) error
	CloseFile(ctx context.Context, path string) error
	CreateFile(ctx context.Context, path string, content []byte) error
	DeleteFile(ctx context.Context, path string) error
	RenameFile(ctx context.Context, oldPath, newPath string) error
	ReloadFile(ctx context.Context, path string) error

	// Directory operations
	CreateDirectory(ctx context.Context, path string) error
	DeleteDirectory(ctx context.Context, path string, recursive bool) error
	ListDirectory(ctx context.Context, path string) ([]index.FileInfo, error)

	// Search
	FindFiles(ctx context.Context, query string, opts FindOptions) ([]FileMatch, error)
	SearchContent(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error)

	// Project graph
	Graph() graph.Graph
	RelatedFiles(ctx context.Context, path string) ([]RelatedFile, error)

	// Open documents
	OpenDocuments() []*filestore.Document
	GetDocument(path string) (*filestore.Document, bool)
	IsDirty(path string) bool
	DirtyDocuments() []*filestore.Document

	// Events
	OnFileChange(handler func(FileChangeEvent))
	OnWorkspaceChange(handler func(workspace.ChangeEvent))

	// Status
	IndexStatus() IndexStatus
	WatcherStatus() WatcherStatus
}

// FindOptions configures file search behavior.
type FindOptions struct {
	MaxResults    int
	FileTypes     []string
	IncludeDirs   bool
	CaseSensitive bool
	PathPrefix    string
	MatchMode     search.MatchMode
	BoostRecent   bool
}

// SearchOptions configures content search behavior.
type SearchOptions struct {
	CaseSensitive bool
	WholeWord     bool
	UseRegex      bool
	IncludePaths  []string
	ExcludePaths  []string
	FileTypes     []string
	MaxResults    int
	ContextLines  int
}

// FileMatch represents a file search result.
type FileMatch struct {
	Path  string
	Name  string
	Score float64
	Info  index.FileInfo
}

// ContentMatch represents a content search result.
type ContentMatch struct {
	Path       string
	Line       int
	Column     int
	Text       string
	Context    []string
	Highlights []Range
}

// Range represents a text range.
type Range struct {
	Start int
	End   int
}

// RelatedFile represents a file related to another file.
type RelatedFile struct {
	Path         string
	Relationship string
	Score        float64
}

// FileChangeEvent represents a file system change.
type FileChangeEvent struct {
	Type      FileChangeType
	Path      string
	OldPath   string // For renames
	Timestamp time.Time
}

// FileChangeType indicates the type of file change.
type FileChangeType int

const (
	FileChangeCreated FileChangeType = iota
	FileChangeModified
	FileChangeDeleted
	FileChangeRenamed
)

// IndexStatus indicates the indexer state.
type IndexStatus struct {
	Status         string
	TotalFiles     int
	IndexedFiles   int
	ErrorFiles     int
	BytesProcessed int64
	StartTime      time.Time
	LastUpdateTime time.Time
}

// WatcherStatus provides watcher status information.
type WatcherStatus struct {
	WatchedPaths  int
	PendingEvents int
	TotalEvents   int64
	Errors        int64
	LastError     error
	StartTime     time.Time
}

// DefaultProject is the standard implementation of Project.
type DefaultProject struct {
	mu sync.RWMutex

	// Core components
	vfs             vfs.VFS
	workspace       *workspace.Workspace
	fileStore       *filestore.FileStore
	fileIndex       index.Index
	contIndex       *index.ContentIndex
	increIndex      *index.IncrementalIndexer
	graph           graph.Graph
	watcher         watcher.Watcher
	fileSearcher    *search.FuzzySearcher
	contentSearcher *search.ContentSearch

	// State
	open   bool
	config Config

	// Goroutine lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Event handlers
	fileChangeHandlers      []func(FileChangeEvent)
	workspaceChangeHandlers []func(workspace.ChangeEvent)
}

// Config holds project configuration.
type Config struct {
	// MaxFileSize is the maximum file size to open (bytes)
	MaxFileSize int64

	// IndexWorkers is the number of parallel indexing workers
	IndexWorkers int

	// WatchDebounceDelay is the delay for debouncing file watch events
	WatchDebounceDelay time.Duration

	// ExcludePatterns are glob patterns to exclude from indexing/watching
	ExcludePatterns []string

	// EnableContentIndex enables content indexing for search
	EnableContentIndex bool

	// EnableGraph enables project graph building
	EnableGraph bool
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxFileSize:        10 * 1024 * 1024, // 10MB
		IndexWorkers:       4,
		WatchDebounceDelay: 100 * time.Millisecond,
		ExcludePatterns: []string{
			"**/.git/**",
			"**/node_modules/**",
			"**/vendor/**",
			"**/__pycache__/**",
			"**/dist/**",
			"**/build/**",
		},
		EnableContentIndex: true,
		EnableGraph:        true,
	}
}

// Option configures a DefaultProject.
type Option func(*DefaultProject)

// WithConfig sets the project configuration.
func WithConfig(cfg Config) Option {
	return func(p *DefaultProject) {
		p.config = cfg
	}
}

// WithVFS sets a custom VFS implementation.
func WithVFS(v vfs.VFS) Option {
	return func(p *DefaultProject) {
		p.vfs = v
	}
}

// WithWatcher sets a custom watcher implementation.
func WithWatcher(w watcher.Watcher) Option {
	return func(p *DefaultProject) {
		p.watcher = w
	}
}

// New creates a new DefaultProject with the given options.
func New(opts ...Option) *DefaultProject {
	p := &DefaultProject{
		config: DefaultConfig(),
	}

	for _, opt := range opts {
		opt(p)
	}

	// Initialize VFS if not provided
	if p.vfs == nil {
		p.vfs = vfs.NewOSFS()
	}

	return p
}

// Open opens a workspace with the given roots.
func (p *DefaultProject) Open(ctx context.Context, roots ...string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.open {
		return ErrAlreadyOpen
	}

	if len(roots) == 0 {
		return workspace.ErrNoFolders
	}

	// Create internal context for goroutine lifecycle management
	// This context is independent of the caller's context to allow goroutines
	// to continue running until Close() is called
	p.ctx, p.cancel = context.WithCancel(context.Background())

	// Create workspace
	var err error
	if len(roots) == 1 {
		p.workspace, err = workspace.NewFromPath(roots[0])
	} else {
		p.workspace, err = workspace.NewFromPaths(roots...)
	}
	if err != nil {
		p.cancel() // Clean up context
		return &WorkspaceError{Root: roots[0], Err: err}
	}

	// Initialize file store
	p.fileStore = filestore.NewFileStoreWithOptions(p.vfs, filestore.WithMaxFileSize(p.config.MaxFileSize))

	// Initialize file index
	p.fileIndex = index.NewFileIndex()

	// Initialize content index if enabled
	if p.config.EnableContentIndex {
		p.contIndex = index.NewContentIndex(index.DefaultContentIndexConfig())
	}

	// Initialize graph if enabled
	if p.config.EnableGraph {
		p.graph = graph.New()
	}

	// Initialize incremental indexer
	incConfig := index.DefaultIncrementalConfig()
	incConfig.Workers = p.config.IndexWorkers
	incConfig.ExcludePatterns = p.config.ExcludePatterns
	incConfig.IndexContent = p.config.EnableContentIndex
	p.increIndex = index.NewIncrementalIndexer(p.fileIndex, p.contIndex, incConfig)

	// Initialize search components
	p.fileSearcher = search.NewFuzzySearcher(p.fileIndex)
	p.contentSearcher = search.NewContentSearch(p.vfs)

	// Initialize watcher if not provided
	if p.watcher == nil {
		fsWatcher, err := watcher.NewFSNotifyWatcher()
		if err != nil {
			// Continue without watcher - log warning in real implementation
			p.watcher = nil
		} else {
			// Wrap with debouncing
			p.watcher = watcher.NewDebouncedWatcher(fsWatcher, p.config.WatchDebounceDelay)
		}
	}

	// Start watching workspace roots
	if p.watcher != nil {
		for _, folder := range p.workspace.Folders() {
			if err := p.watcher.WatchRecursive(folder.Path); err != nil {
				// Log warning but continue
			}
		}

		// Start processing watcher events
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.processWatcherEvents(p.ctx)
		}()
	}

	// Start background indexing
	indexRoots := make([]string, 0, len(roots))
	for _, folder := range p.workspace.Folders() {
		indexRoots = append(indexRoots, folder.Path)
	}
	if err := p.increIndex.Start(p.ctx, indexRoots...); err != nil {
		// Log warning but continue
	}

	// Build initial graph if enabled
	if p.config.EnableGraph && p.graph != nil {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.buildGraph(p.ctx, indexRoots)
		}()
	}

	p.open = true
	return nil
}

// Close closes the workspace.
func (p *DefaultProject) Close(ctx context.Context) error {
	p.mu.Lock()

	if !p.open {
		p.mu.Unlock()
		return ErrNotOpen
	}

	// Cancel context to signal goroutines to stop
	if p.cancel != nil {
		p.cancel()
	}

	// Stop watcher first (closes event channels which unblocks processWatcherEvents)
	if p.watcher != nil {
		p.watcher.Close()
		p.watcher = nil
	}

	// Stop indexer
	if p.increIndex != nil {
		p.increIndex.Stop()
	}

	// Release lock before waiting for goroutines (they may need to acquire lock)
	p.mu.Unlock()

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutines finished cleanly
	case <-ctx.Done():
		// Context cancelled while waiting - goroutines will still be signalled to stop
		// via the cancelled p.ctx but we won't wait for them
	}

	// Reacquire lock for final cleanup
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close file store
	if p.fileStore != nil {
		_ = p.fileStore.CloseAll(ctx, true)
	}

	// Close file index
	if p.fileIndex != nil {
		p.fileIndex.Close()
	}

	// Clear graph
	if p.graph != nil {
		p.graph.Clear()
	}

	p.workspace = nil
	p.ctx = nil
	p.cancel = nil
	p.open = false
	return nil
}

// IsOpen returns true if the workspace is open.
func (p *DefaultProject) IsOpen() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.open
}

// Root returns the primary workspace root.
func (p *DefaultProject) Root() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.workspace == nil {
		return ""
	}
	return p.workspace.Root()
}

// Roots returns all workspace roots.
func (p *DefaultProject) Roots() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.workspace == nil {
		return nil
	}

	folders := p.workspace.Folders()
	roots := make([]string, len(folders))
	for i, f := range folders {
		roots[i] = f.Path
	}
	return roots
}

// IsInWorkspace returns true if the path is within the workspace.
func (p *DefaultProject) IsInWorkspace(path string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.workspace == nil {
		return false
	}
	return p.workspace.IsInWorkspace(path)
}

// Workspace returns the underlying workspace.
func (p *DefaultProject) Workspace() *workspace.Workspace {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.workspace
}

// OpenFile opens a file and returns its Document.
func (p *DefaultProject) OpenFile(ctx context.Context, path string) (*filestore.Document, error) {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return nil, ErrNotOpen
	}
	ws := p.workspace
	store := p.fileStore
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return nil, NewPathError("open", path, ErrNotInWorkspace)
	}

	return store.Open(ctx, path)
}

// SaveFile saves an open document to disk.
func (p *DefaultProject) SaveFile(ctx context.Context, path string) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	store := p.fileStore
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return NewPathError("save", path, ErrNotInWorkspace)
	}

	return store.Save(ctx, path)
}

// SaveFileAs saves a document to a new path.
func (p *DefaultProject) SaveFileAs(ctx context.Context, oldPath, newPath string) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	store := p.fileStore
	p.mu.RUnlock()

	// Validate both paths are in workspace
	if ws != nil {
		if !ws.IsInWorkspace(oldPath) {
			return NewPathError("saveas", oldPath, ErrNotInWorkspace)
		}
		if !ws.IsInWorkspace(newPath) {
			return NewPathError("saveas", newPath, ErrNotInWorkspace)
		}
	}

	return store.SaveAs(ctx, oldPath, newPath)
}

// CloseFile closes an open document.
func (p *DefaultProject) CloseFile(ctx context.Context, path string) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	store := p.fileStore
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return NewPathError("close", path, ErrNotInWorkspace)
	}

	return store.Close(ctx, path, false)
}

// CreateFile creates a new file.
func (p *DefaultProject) CreateFile(ctx context.Context, path string, content []byte) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	fs := p.vfs
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return NewPathError("create", path, ErrNotInWorkspace)
	}

	// Check if file exists
	if fs.Exists(path) {
		return NewPathError("create", path, ErrAlreadyExists)
	}

	// Create parent directories
	dir := filepath.Dir(path)
	if err := fs.MkdirAll(dir, 0755); err != nil {
		return NewPathError("create", path, err)
	}

	// Write file
	if err := fs.WriteFile(path, content, 0644); err != nil {
		return NewPathError("create", path, err)
	}

	return nil
}

// DeleteFile deletes a file.
func (p *DefaultProject) DeleteFile(ctx context.Context, path string) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	fs := p.vfs
	store := p.fileStore
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return NewPathError("delete", path, ErrNotInWorkspace)
	}

	// Close if open
	_ = store.Close(ctx, path, true)

	// Delete file
	if err := fs.Remove(path); err != nil {
		return NewPathError("delete", path, err)
	}

	return nil
}

// RenameFile renames a file.
func (p *DefaultProject) RenameFile(ctx context.Context, oldPath, newPath string) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	fs := p.vfs
	store := p.fileStore
	p.mu.RUnlock()

	// Validate both paths are in workspace
	if ws != nil {
		if !ws.IsInWorkspace(oldPath) {
			return NewPathError("rename", oldPath, ErrNotInWorkspace)
		}
		if !ws.IsInWorkspace(newPath) {
			return NewPathError("rename", newPath, ErrNotInWorkspace)
		}
	}

	// Close if open
	_ = store.Close(ctx, oldPath, true)

	// Rename file
	if err := fs.Rename(oldPath, newPath); err != nil {
		return NewPathError("rename", oldPath, err)
	}

	return nil
}

// ReloadFile reloads a file from disk.
func (p *DefaultProject) ReloadFile(ctx context.Context, path string) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	store := p.fileStore
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return NewPathError("reload", path, ErrNotInWorkspace)
	}

	return store.Reload(ctx, path, false)
}

// CreateDirectory creates a directory.
func (p *DefaultProject) CreateDirectory(ctx context.Context, path string) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	fs := p.vfs
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return NewPathError("mkdir", path, ErrNotInWorkspace)
	}

	if err := fs.MkdirAll(path, 0755); err != nil {
		return NewPathError("mkdir", path, err)
	}
	return nil
}

// DeleteDirectory deletes a directory.
func (p *DefaultProject) DeleteDirectory(ctx context.Context, path string, recursive bool) error {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return ErrNotOpen
	}
	ws := p.workspace
	fs := p.vfs
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return NewPathError("rmdir", path, ErrNotInWorkspace)
	}

	var err error
	if recursive {
		err = fs.RemoveAll(path)
	} else {
		err = fs.Remove(path)
	}
	if err != nil {
		return NewPathError("rmdir", path, err)
	}
	return nil
}

// ListDirectory lists directory contents.
func (p *DefaultProject) ListDirectory(ctx context.Context, path string) ([]index.FileInfo, error) {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return nil, ErrNotOpen
	}
	ws := p.workspace
	fs := p.vfs
	p.mu.RUnlock()

	// Validate path is in workspace
	if ws != nil && !ws.IsInWorkspace(path) {
		return nil, NewPathError("readdir", path, ErrNotInWorkspace)
	}

	entries, err := fs.ReadDir(path)
	if err != nil {
		return nil, NewPathError("readdir", path, err)
	}

	infos := make([]index.FileInfo, len(entries))
	for i, e := range entries {
		infos[i] = index.FileInfo{
			Path:    filepath.Join(path, e.Name()),
			Name:    e.Name(),
			Size:    e.Size(),
			ModTime: e.ModTime(),
			IsDir:   e.IsDir(),
			Mode:    e.Mode(),
		}
	}
	return infos, nil
}

// FindFiles searches for files matching the query.
func (p *DefaultProject) FindFiles(ctx context.Context, query string, opts FindOptions) ([]FileMatch, error) {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return nil, ErrNotOpen
	}
	searcher := p.fileSearcher
	fileIdx := p.fileIndex
	p.mu.RUnlock()

	// Check if search components are available
	if searcher == nil || fileIdx == nil {
		return nil, nil
	}

	searchOpts := search.FileSearchOptions{
		MaxResults:    opts.MaxResults,
		FileTypes:     opts.FileTypes,
		IncludeDirs:   opts.IncludeDirs,
		CaseSensitive: opts.CaseSensitive,
		PathPrefix:    opts.PathPrefix,
		MatchMode:     opts.MatchMode,
		BoostRecent:   opts.BoostRecent,
	}

	results, err := searcher.Search(ctx, query, searchOpts)
	if err != nil {
		return nil, err
	}

	matches := make([]FileMatch, len(results))
	for i, r := range results {
		info, _ := fileIdx.Get(r.Path)
		matches[i] = FileMatch{
			Path:  r.Path,
			Name:  r.Name,
			Score: r.Score,
			Info:  info,
		}
	}
	return matches, nil
}

// SearchContent searches file contents.
func (p *DefaultProject) SearchContent(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error) {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return nil, ErrNotOpen
	}
	searcher := p.contentSearcher
	p.mu.RUnlock()

	// Check if content searcher is available
	if searcher == nil {
		return nil, nil
	}

	searchOpts := search.ContentSearchOptions{
		CaseSensitive: opts.CaseSensitive,
		WholeWord:     opts.WholeWord,
		UseRegex:      opts.UseRegex,
		IncludePaths:  opts.IncludePaths,
		ExcludePaths:  opts.ExcludePaths,
		FileTypes:     opts.FileTypes,
		MaxResults:    opts.MaxResults,
		ContextLines:  opts.ContextLines,
	}

	results, err := searcher.Search(ctx, query, searchOpts)
	if err != nil {
		return nil, err
	}

	matches := make([]ContentMatch, len(results))
	for i, r := range results {
		highlights := make([]Range, len(r.Highlights))
		for j, h := range r.Highlights {
			highlights[j] = Range{Start: h.Start, End: h.End}
		}
		matches[i] = ContentMatch{
			Path:       r.Path,
			Line:       r.Line,
			Column:     r.Column,
			Text:       r.Text,
			Highlights: highlights,
		}
	}
	return matches, nil
}

// Graph returns the project graph.
func (p *DefaultProject) Graph() graph.Graph {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.graph
}

// RelatedFiles returns files related to the given path.
func (p *DefaultProject) RelatedFiles(ctx context.Context, path string) ([]RelatedFile, error) {
	p.mu.RLock()
	if !p.open {
		p.mu.RUnlock()
		return nil, ErrNotOpen
	}
	g := p.graph
	p.mu.RUnlock()

	if g == nil {
		return nil, nil
	}

	// Find node for path
	node, found := g.FindNodeByPath(path)
	if !found {
		return nil, nil
	}

	// Get related nodes
	related := g.RelatedNodes(node.ID, 2)
	results := make([]RelatedFile, 0, len(related))

	for _, rel := range related {
		if rel.Path != "" && rel.Path != path {
			relationship := "related"
			// Determine relationship type from edges
			edges := g.GetEdges(node.ID)
			for _, e := range edges {
				if e.To == rel.ID {
					relationship = e.Type.String()
					break
				}
			}
			results = append(results, RelatedFile{
				Path:         rel.Path,
				Relationship: relationship,
				Score:        1.0,
			})
		}
	}

	return results, nil
}

// OpenDocuments returns all open documents.
func (p *DefaultProject) OpenDocuments() []*filestore.Document {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.fileStore == nil {
		return nil
	}
	return p.fileStore.OpenDocuments()
}

// GetDocument returns an open document by path.
func (p *DefaultProject) GetDocument(path string) (*filestore.Document, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.fileStore == nil {
		return nil, false
	}
	return p.fileStore.Get(path)
}

// IsDirty returns true if the document has unsaved changes.
func (p *DefaultProject) IsDirty(path string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.fileStore == nil {
		return false
	}
	doc, ok := p.fileStore.Get(path)
	if !ok {
		return false
	}
	return doc.IsDirty()
}

// DirtyDocuments returns all documents with unsaved changes.
func (p *DefaultProject) DirtyDocuments() []*filestore.Document {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.fileStore == nil {
		return nil
	}
	return p.fileStore.DirtyDocuments()
}

// OnFileChange registers a handler for file change events.
func (p *DefaultProject) OnFileChange(handler func(FileChangeEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fileChangeHandlers = append(p.fileChangeHandlers, handler)
}

// OnWorkspaceChange registers a handler for workspace change events.
func (p *DefaultProject) OnWorkspaceChange(handler func(workspace.ChangeEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.workspaceChangeHandlers = append(p.workspaceChangeHandlers, handler)
}

// IndexStatus returns the current indexing status.
func (p *DefaultProject) IndexStatus() IndexStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.increIndex == nil {
		return IndexStatus{Status: "idle"}
	}

	progress := p.increIndex.Progress()
	status := p.increIndex.Status()

	return IndexStatus{
		Status:         status.String(),
		TotalFiles:     progress.TotalFiles,
		IndexedFiles:   progress.IndexedFiles,
		ErrorFiles:     progress.ErrorFiles,
		BytesProcessed: progress.BytesProcessed,
		StartTime:      progress.StartTime,
		LastUpdateTime: progress.LastUpdateTime,
	}
}

// WatcherStatus returns the current watcher status.
func (p *DefaultProject) WatcherStatus() WatcherStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.watcher == nil {
		return WatcherStatus{}
	}

	stats := p.watcher.Stats()
	return WatcherStatus{
		WatchedPaths:  stats.WatchedPaths,
		PendingEvents: stats.PendingEvents,
		TotalEvents:   stats.TotalEvents,
		Errors:        stats.Errors,
		LastError:     stats.LastError,
		StartTime:     stats.StartTime,
	}
}

// Save persists the project indexes.
func (p *DefaultProject) Save(fileIndexWriter, contentIndexWriter io.Writer) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.increIndex != nil {
		return p.increIndex.Save(fileIndexWriter, contentIndexWriter)
	}
	return nil
}

// Load restores the project indexes.
func (p *DefaultProject) Load(fileIndexReader, contentIndexReader io.Reader) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.increIndex != nil {
		return p.increIndex.Load(fileIndexReader, contentIndexReader)
	}
	return nil
}

// processWatcherEvents processes file system events from the watcher.
func (p *DefaultProject) processWatcherEvents(ctx context.Context) {
	if p.watcher == nil {
		return
	}

	events := p.watcher.Events()
	errors := p.watcher.Errors()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			p.handleWatchEvent(event)
		case err, ok := <-errors:
			if !ok {
				return
			}
			// Log error in real implementation
			_ = err
		}
	}
}

// handleWatchEvent handles a single watcher event.
func (p *DefaultProject) handleWatchEvent(event watcher.Event) {
	var changeType FileChangeType
	switch {
	case event.Op.Has(watcher.OpCreate):
		changeType = FileChangeCreated
	case event.Op.Has(watcher.OpWrite):
		changeType = FileChangeModified
	case event.Op.Has(watcher.OpRemove):
		changeType = FileChangeDeleted
	case event.Op.Has(watcher.OpRename):
		changeType = FileChangeRenamed
	default:
		return
	}

	// Get incremental indexer and handlers under lock
	p.mu.RLock()
	increIndex := p.increIndex
	handlers := make([]func(FileChangeEvent), len(p.fileChangeHandlers))
	copy(handlers, p.fileChangeHandlers)
	p.mu.RUnlock()

	// Update incremental index (outside lock to avoid blocking)
	if increIndex != nil {
		indexEvent := index.FileChangeEvent{
			Type:      index.FileChangeType(changeType),
			Path:      event.Path,
			Timestamp: event.Timestamp,
		}
		_ = increIndex.ProcessChange(indexEvent)
	}

	// Emit event to handlers (outside lock to avoid deadlock)
	changeEvent := FileChangeEvent{
		Type:      changeType,
		Path:      event.Path,
		Timestamp: event.Timestamp,
	}

	for _, h := range handlers {
		h(changeEvent)
	}
}

// buildGraph builds the project graph in the background.
func (p *DefaultProject) buildGraph(ctx context.Context, roots []string) {
	p.mu.RLock()
	g := p.graph
	cfg := p.config
	p.mu.RUnlock()

	if g == nil {
		return
	}

	builder := graph.NewBuilder(cfg.IndexWorkers)
	builder.SetIgnorePatterns(cfg.ExcludePatterns)

	for _, root := range roots {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		built, err := builder.Build(ctx, root)
		if err != nil {
			continue
		}

		// Merge nodes and edges into main graph
		p.mu.Lock()
		// Re-check graph is still valid (could be cleared during Close)
		if p.graph != nil {
			for _, node := range built.AllNodes() {
				_ = p.graph.AddNode(node)
			}
			for _, edge := range built.AllEdges() {
				_ = p.graph.AddEdge(edge)
			}
		}
		p.mu.Unlock()
	}
}

// Ensure DefaultProject implements Project interface.
var _ Project = (*DefaultProject)(nil)
