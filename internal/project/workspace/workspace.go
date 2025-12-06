// Package workspace provides workspace management for multi-root project support.
// It handles workspace folders, configuration, and lifecycle management.
package workspace

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
)

// Common errors.
var (
	ErrNoFolders       = errors.New("workspace has no folders")
	ErrFolderNotFound  = errors.New("folder not found in workspace")
	ErrFolderExists    = errors.New("folder already in workspace")
	ErrInvalidPath     = errors.New("invalid folder path")
	ErrWorkspaceClosed = errors.New("workspace is closed")
)

// Workspace represents a collection of folders being edited.
// It supports both single-root and multi-root workspaces.
type Workspace struct {
	mu      sync.RWMutex
	folders []Folder
	config  *Config
	closed  bool

	// Callbacks
	onFolderAdd    []func(Folder)
	onFolderRemove []func(Folder)
	onChange       []func(ChangeEvent)
}

// Folder represents a single folder in the workspace.
type Folder struct {
	// URI is the folder path as a URI (file://)
	URI string
	// Path is the local file system path
	Path string
	// Name is the display name for the folder
	Name string
}

// ChangeEvent represents a workspace change.
type ChangeEvent struct {
	Type    ChangeType
	Folders []Folder
}

// ChangeType indicates the type of workspace change.
type ChangeType int

const (
	// ChangeFolderAdded indicates a folder was added.
	ChangeFolderAdded ChangeType = iota
	// ChangeFolderRemoved indicates a folder was removed.
	ChangeFolderRemoved
	// ChangeConfigUpdated indicates configuration was updated.
	ChangeConfigUpdated
)

// New creates a new empty workspace.
func New() *Workspace {
	return &Workspace{
		folders: make([]Folder, 0),
		config:  DefaultConfig(),
	}
}

// NewFromPath creates a workspace with a single root folder.
func NewFromPath(path string) (*Workspace, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	ws := New()
	folder := Folder{
		Path: absPath,
		URI:  PathToURI(absPath),
		Name: filepath.Base(absPath),
	}
	ws.folders = append(ws.folders, folder)
	return ws, nil
}

// NewFromPaths creates a multi-root workspace from multiple paths.
func NewFromPaths(paths ...string) (*Workspace, error) {
	if len(paths) == 0 {
		return nil, ErrNoFolders
	}

	ws := New()
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		folder := Folder{
			Path: absPath,
			URI:  PathToURI(absPath),
			Name: filepath.Base(absPath),
		}
		ws.folders = append(ws.folders, folder)
	}
	return ws, nil
}

// Open initializes the workspace with the given root paths.
func (w *Workspace) Open(ctx context.Context, roots ...string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWorkspaceClosed
	}

	if len(roots) == 0 {
		return ErrNoFolders
	}

	w.folders = make([]Folder, 0, len(roots))
	for _, root := range roots {
		absPath, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		folder := Folder{
			Path: absPath,
			URI:  PathToURI(absPath),
			Name: filepath.Base(absPath),
		}
		w.folders = append(w.folders, folder)
	}
	return nil
}

// Close closes the workspace.
func (w *Workspace) Close(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.closed = true
	w.folders = nil
	return nil
}

// IsClosed returns whether the workspace is closed.
func (w *Workspace) IsClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.closed
}

// Root returns the primary workspace root path.
// For multi-root workspaces, this returns the first folder.
func (w *Workspace) Root() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if len(w.folders) == 0 {
		return ""
	}
	return w.folders[0].Path
}

// Roots returns all workspace root paths.
func (w *Workspace) Roots() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	paths := make([]string, len(w.folders))
	for i, f := range w.folders {
		paths[i] = f.Path
	}
	return paths
}

// Folders returns all workspace folders.
func (w *Workspace) Folders() []Folder {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]Folder, len(w.folders))
	copy(result, w.folders)
	return result
}

// FolderCount returns the number of folders in the workspace.
func (w *Workspace) FolderCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.folders)
}

// IsMultiRoot returns true if the workspace has more than one root folder.
func (w *Workspace) IsMultiRoot() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.folders) > 1
}

// AddFolder adds a folder to the workspace.
func (w *Workspace) AddFolder(ctx context.Context, path string) error {
	w.mu.Lock()

	if w.closed {
		w.mu.Unlock()
		return ErrWorkspaceClosed
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		w.mu.Unlock()
		return err
	}

	// Check if folder already exists
	for _, f := range w.folders {
		if f.Path == absPath {
			w.mu.Unlock()
			return ErrFolderExists
		}
	}

	folder := Folder{
		Path: absPath,
		URI:  PathToURI(absPath),
		Name: filepath.Base(absPath),
	}
	w.folders = append(w.folders, folder)

	// Copy callbacks before releasing lock
	callbacks := make([]func(Folder), len(w.onFolderAdd))
	copy(callbacks, w.onFolderAdd)
	changeCallbacks := make([]func(ChangeEvent), len(w.onChange))
	copy(changeCallbacks, w.onChange)

	w.mu.Unlock()

	// Notify listeners (outside lock)
	for _, cb := range callbacks {
		cb(folder)
	}
	event := ChangeEvent{Type: ChangeFolderAdded, Folders: []Folder{folder}}
	for _, cb := range changeCallbacks {
		cb(event)
	}

	return nil
}

// RemoveFolder removes a folder from the workspace.
func (w *Workspace) RemoveFolder(ctx context.Context, path string) error {
	w.mu.Lock()

	if w.closed {
		w.mu.Unlock()
		return ErrWorkspaceClosed
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		w.mu.Unlock()
		return err
	}

	idx := -1
	var removed Folder
	for i, f := range w.folders {
		if f.Path == absPath {
			idx = i
			removed = f
			break
		}
	}

	if idx == -1 {
		w.mu.Unlock()
		return ErrFolderNotFound
	}

	// Remove folder
	w.folders = append(w.folders[:idx], w.folders[idx+1:]...)

	// Copy callbacks before releasing lock
	callbacks := make([]func(Folder), len(w.onFolderRemove))
	copy(callbacks, w.onFolderRemove)
	changeCallbacks := make([]func(ChangeEvent), len(w.onChange))
	copy(changeCallbacks, w.onChange)

	w.mu.Unlock()

	// Notify listeners (outside lock)
	for _, cb := range callbacks {
		cb(removed)
	}
	event := ChangeEvent{Type: ChangeFolderRemoved, Folders: []Folder{removed}}
	for _, cb := range changeCallbacks {
		cb(event)
	}

	return nil
}

// GetFolder returns the folder at the given path.
func (w *Workspace) GetFolder(path string) (Folder, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return Folder{}, false
	}

	for _, f := range w.folders {
		if f.Path == absPath {
			return f, true
		}
	}
	return Folder{}, false
}

// GetFolderByURI returns the folder with the given URI.
func (w *Workspace) GetFolderByURI(uri string) (Folder, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, f := range w.folders {
		if f.URI == uri {
			return f, true
		}
	}
	return Folder{}, false
}

// IsInWorkspace checks if a path is within any workspace folder.
func (w *Workspace) IsInWorkspace(path string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, f := range w.folders {
		if isSubPath(f.Path, absPath) {
			return true
		}
	}
	return false
}

// ContainingFolder returns the workspace folder that contains the given path.
func (w *Workspace) ContainingFolder(path string) (Folder, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return Folder{}, false
	}

	for _, f := range w.folders {
		if isSubPath(f.Path, absPath) {
			return f, true
		}
	}
	return Folder{}, false
}

// RelativePath returns the path relative to its containing workspace folder.
func (w *Workspace) RelativePath(path string) (string, error) {
	folder, ok := w.ContainingFolder(path)
	if !ok {
		return "", ErrFolderNotFound
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return filepath.Rel(folder.Path, absPath)
}

// Config returns the workspace configuration.
func (w *Workspace) Config() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// SetConfig sets the workspace configuration.
func (w *Workspace) SetConfig(config *Config) {
	w.mu.Lock()

	w.config = config

	// Copy callbacks before releasing lock
	changeCallbacks := make([]func(ChangeEvent), len(w.onChange))
	copy(changeCallbacks, w.onChange)

	w.mu.Unlock()

	// Notify listeners (outside lock)
	event := ChangeEvent{Type: ChangeConfigUpdated}
	for _, cb := range changeCallbacks {
		cb(event)
	}
}

// OnFolderAdd registers a callback for when a folder is added.
func (w *Workspace) OnFolderAdd(fn func(Folder)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onFolderAdd = append(w.onFolderAdd, fn)
}

// OnFolderRemove registers a callback for when a folder is removed.
func (w *Workspace) OnFolderRemove(fn func(Folder)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onFolderRemove = append(w.onFolderRemove, fn)
}

// OnChange registers a callback for any workspace change.
func (w *Workspace) OnChange(fn func(ChangeEvent)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = append(w.onChange, fn)
}

// PathToURI converts a file path to a file:// URI.
func PathToURI(path string) string {
	// Ensure path is absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Convert to forward slashes
	absPath = filepath.ToSlash(absPath)

	// URL-encode the path
	u := url.URL{
		Scheme: "file",
		Path:   absPath,
	}
	return u.String()
}

// URIToPath converts a file:// URI to a file path.
func URIToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	if u.Scheme != "file" {
		return "", ErrInvalidPath
	}

	// Decode percent-encoded characters in the path
	decodedPath, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}

	// Convert URL path to native path
	path := filepath.FromSlash(decodedPath)

	// On Windows, remove leading slash if path starts with drive letter
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	return path, nil
}

// isSubPath checks if child is a subpath of parent.
func isSubPath(parent, child string) bool {
	// Normalize paths
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	// Ensure parent ends with separator for proper prefix matching
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}

	// Check if child is exactly parent (without trailing separator)
	parentWithoutSep := strings.TrimSuffix(parent, string(filepath.Separator))
	if child == parentWithoutSep {
		return true
	}

	return strings.HasPrefix(child, parent)
}
