package filestore

import (
	"context"
	"io/fs"
	"sync"
	"time"

	perrors "github.com/dshills/keystorm/internal/project/errors"
	"github.com/dshills/keystorm/internal/project/vfs"
)

// FileStore manages open documents in the editor.
// It provides thread-safe access to documents and tracks their state.
type FileStore struct {
	mu        sync.RWMutex
	documents map[string]*Document
	vfs       vfs.VFS

	// Configuration
	maxFileSize int64 // Maximum file size to open (0 = unlimited)

	// Event handlers
	onOpen   []func(doc *Document)
	onClose  []func(path string)
	onSave   []func(doc *Document)
	onDirty  []func(doc *Document, dirty bool)
	onReload []func(doc *Document)
}

// NewFileStore creates a new FileStore.
func NewFileStore(vfs vfs.VFS) *FileStore {
	return &FileStore{
		documents:   make(map[string]*Document),
		vfs:         vfs,
		maxFileSize: 10 * 1024 * 1024, // 10MB default
	}
}

// Option configures a FileStore.
type Option func(*FileStore)

// WithMaxFileSize sets the maximum file size.
func WithMaxFileSize(size int64) Option {
	return func(fs *FileStore) {
		fs.maxFileSize = size
	}
}

// NewFileStoreWithOptions creates a new FileStore with options.
func NewFileStoreWithOptions(vfs vfs.VFS, opts ...Option) *FileStore {
	store := NewFileStore(vfs)
	for _, opt := range opts {
		opt(store)
	}
	return store
}

// Open opens a file and returns its Document.
// If the file is already open, returns the existing Document.
func (s *FileStore) Open(ctx context.Context, path string) (*Document, error) {
	// Clean the path
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return nil, &perrors.PathError{Op: "open", Path: path, Err: err}
	}

	// Check if already open
	s.mu.RLock()
	if doc, ok := s.documents[absPath]; ok {
		s.mu.RUnlock()
		return doc, nil
	}
	s.mu.RUnlock()

	// Check if file exists
	info, err := s.vfs.Stat(absPath)
	if err != nil {
		// Preserve the underlying VFS error for better debugging
		return nil, &perrors.PathError{Op: "open", Path: path, Err: err}
	}

	// Check if it's a directory
	if info.IsDir() {
		return nil, &perrors.PathError{Op: "open", Path: path, Err: perrors.ErrIsDirectory}
	}

	// Check file size
	if s.maxFileSize > 0 && info.Size() > s.maxFileSize {
		return nil, &perrors.PathError{Op: "open", Path: path, Err: perrors.ErrFileTooLarge}
	}

	// Read file content
	content, err := s.vfs.ReadFile(absPath)
	if err != nil {
		return nil, &perrors.PathError{Op: "open", Path: path, Err: err}
	}

	// Check if binary
	if vfs.IsBinary(content) {
		return nil, &perrors.PathError{Op: "open", Path: path, Err: perrors.ErrBinaryFile}
	}

	// Create document
	doc := NewDocument(absPath, content, info.ModTime())

	// Store document
	s.mu.Lock()
	// Double-check in case another goroutine opened it
	if existing, ok := s.documents[absPath]; ok {
		s.mu.Unlock()
		return existing, nil
	}
	s.documents[absPath] = doc
	s.mu.Unlock()

	// Notify handlers (copy slice to avoid races during iteration)
	s.mu.RLock()
	handlers := make([]func(doc *Document), len(s.onOpen))
	copy(handlers, s.onOpen)
	s.mu.RUnlock()
	for _, handler := range handlers {
		handler(doc)
	}

	return doc, nil
}

// Close closes a document.
// Returns an error if the document has unsaved changes and force is false.
func (s *FileStore) Close(ctx context.Context, path string, force bool) error {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return &perrors.PathError{Op: "close", Path: path, Err: err}
	}

	s.mu.Lock()
	doc, ok := s.documents[absPath]
	if !ok {
		s.mu.Unlock()
		return &perrors.PathError{Op: "close", Path: path, Err: perrors.ErrDocumentNotOpen}
	}

	// Check for unsaved changes
	if !force && doc.IsDirty() {
		s.mu.Unlock()
		return &perrors.PathError{Op: "close", Path: path, Err: perrors.ErrDocumentDirty}
	}

	// Mark as closed and remove from store
	doc.MarkClosed()
	delete(s.documents, absPath)
	s.mu.Unlock()

	// Notify handlers (copy slice to avoid races during iteration)
	s.mu.RLock()
	closeHandlers := make([]func(path string), len(s.onClose))
	copy(closeHandlers, s.onClose)
	s.mu.RUnlock()
	for _, handler := range closeHandlers {
		handler(absPath)
	}

	return nil
}

// Get returns a document by path if it is open.
func (s *FileStore) Get(path string) (*Document, bool) {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[absPath]
	return doc, ok
}

// IsOpen returns true if the file is open.
func (s *FileStore) IsOpen(path string) bool {
	_, ok := s.Get(path)
	return ok
}

// IsDirty returns true if the file is open and has unsaved changes.
func (s *FileStore) IsDirty(path string) bool {
	doc, ok := s.Get(path)
	if !ok {
		return false
	}
	return doc.IsDirty()
}

// OpenDocuments returns all open documents.
func (s *FileStore) OpenDocuments() []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := make([]*Document, 0, len(s.documents))
	for _, doc := range s.documents {
		docs = append(docs, doc)
	}
	return docs
}

// DirtyDocuments returns all documents with unsaved changes.
func (s *FileStore) DirtyDocuments() []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var dirty []*Document
	for _, doc := range s.documents {
		if doc.IsDirty() {
			dirty = append(dirty, doc)
		}
	}
	return dirty
}

// Count returns the number of open documents.
func (s *FileStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.documents)
}

// Save saves a document to disk.
func (s *FileStore) Save(ctx context.Context, path string) error {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return &perrors.PathError{Op: "save", Path: path, Err: err}
	}

	s.mu.RLock()
	doc, ok := s.documents[absPath]
	s.mu.RUnlock()

	if !ok {
		return &perrors.PathError{Op: "save", Path: path, Err: perrors.ErrDocumentNotOpen}
	}

	if doc.ReadOnly {
		return &perrors.PathError{Op: "save", Path: path, Err: perrors.ErrReadOnly}
	}

	// Get content prepared for saving
	content := doc.ContentForSave()

	// Write to disk
	if err := s.vfs.WriteFile(absPath, content, 0644); err != nil {
		return &perrors.PathError{Op: "save", Path: path, Err: err}
	}

	// Get the new modification time
	info, err := s.vfs.Stat(absPath)
	if err != nil {
		// File was saved but we can't get the mod time - use current time
		doc.MarkSaved(time.Now())
	} else {
		doc.MarkSaved(info.ModTime())
	}

	// Notify handlers (copy slice to avoid races during iteration)
	s.mu.RLock()
	saveHandlers := make([]func(doc *Document), len(s.onSave))
	copy(saveHandlers, s.onSave)
	s.mu.RUnlock()
	for _, handler := range saveHandlers {
		handler(doc)
	}

	return nil
}

// SaveAs saves a document to a new path.
func (s *FileStore) SaveAs(ctx context.Context, oldPath, newPath string) error {
	oldAbsPath, err := s.vfs.Abs(oldPath)
	if err != nil {
		return &perrors.PathError{Op: "saveas", Path: oldPath, Err: err}
	}

	newAbsPath, err := s.vfs.Abs(newPath)
	if err != nil {
		return &perrors.PathError{Op: "saveas", Path: newPath, Err: err}
	}

	// Snapshot state while holding lock, then release before I/O
	s.mu.Lock()
	doc, ok := s.documents[oldAbsPath]
	if !ok {
		s.mu.Unlock()
		return &perrors.PathError{Op: "saveas", Path: oldPath, Err: perrors.ErrDocumentNotOpen}
	}

	// Check if new path already has an open document
	if _, exists := s.documents[newAbsPath]; exists {
		s.mu.Unlock()
		return &perrors.PathError{Op: "saveas", Path: newPath, Err: perrors.ErrAlreadyOpen}
	}
	s.mu.Unlock()

	// Get content prepared for saving (outside lock - ContentForSave makes its own copy)
	content := doc.ContentForSave()

	// Write to new path (I/O outside lock)
	if err := s.vfs.WriteFile(newAbsPath, content, 0644); err != nil {
		return &perrors.PathError{Op: "saveas", Path: newPath, Err: err}
	}

	// Get the new modification time
	info, err := s.vfs.Stat(newAbsPath)
	modTime := time.Now()
	if err == nil {
		modTime = info.ModTime()
	}

	// Re-acquire lock to update store state
	s.mu.Lock()
	// Re-check conditions after re-acquiring lock
	if _, stillExists := s.documents[oldAbsPath]; !stillExists {
		s.mu.Unlock()
		// Document was closed while we were writing - clean up orphaned file
		_ = s.vfs.Remove(newAbsPath)
		return &perrors.PathError{Op: "saveas", Path: oldPath, Err: perrors.ErrDocumentNotOpen}
	}
	if _, exists := s.documents[newAbsPath]; exists {
		s.mu.Unlock()
		// Another document appeared at new path - clean up
		_ = s.vfs.Remove(newAbsPath)
		return &perrors.PathError{Op: "saveas", Path: newPath, Err: perrors.ErrAlreadyOpen}
	}

	// Update document path and state
	doc.mu.Lock()
	doc.Path = newAbsPath
	doc.LanguageID = detectLanguageID(newAbsPath)
	doc.mu.Unlock()
	doc.MarkSaved(modTime)

	// Update store
	delete(s.documents, oldAbsPath)
	s.documents[newAbsPath] = doc
	s.mu.Unlock()

	// Notify handlers (copy slice to avoid races during iteration)
	s.mu.RLock()
	saveAsHandlers := make([]func(doc *Document), len(s.onSave))
	copy(saveAsHandlers, s.onSave)
	s.mu.RUnlock()
	for _, handler := range saveAsHandlers {
		handler(doc)
	}

	return nil
}

// Reload reloads a document from disk.
// If the document is dirty and force is false, returns an error.
func (s *FileStore) Reload(ctx context.Context, path string, force bool) error {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return &perrors.PathError{Op: "reload", Path: path, Err: err}
	}

	s.mu.RLock()
	doc, ok := s.documents[absPath]
	s.mu.RUnlock()

	if !ok {
		return &perrors.PathError{Op: "reload", Path: path, Err: perrors.ErrDocumentNotOpen}
	}

	if !force && doc.IsDirty() {
		return &perrors.PathError{Op: "reload", Path: path, Err: perrors.ErrDocumentDirty}
	}

	// Read current content from disk
	content, err := s.vfs.ReadFile(absPath)
	if err != nil {
		return &perrors.PathError{Op: "reload", Path: path, Err: err}
	}

	info, err := s.vfs.Stat(absPath)
	if err != nil {
		return &perrors.PathError{Op: "reload", Path: path, Err: err}
	}

	// Reload the document
	if doc.Reload(content, info.ModTime()) {
		// Notify handlers if content changed (copy slice to avoid races)
		s.mu.RLock()
		reloadHandlers := make([]func(doc *Document), len(s.onReload))
		copy(reloadHandlers, s.onReload)
		s.mu.RUnlock()
		for _, handler := range reloadHandlers {
			handler(doc)
		}
	}

	return nil
}

// UpdateContent updates the content of an open document.
// This is typically called from the editor when the buffer changes.
func (s *FileStore) UpdateContent(path string, content []byte) error {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return &perrors.PathError{Op: "update", Path: path, Err: err}
	}

	s.mu.RLock()
	doc, ok := s.documents[absPath]
	s.mu.RUnlock()

	if !ok {
		return &perrors.PathError{Op: "update", Path: path, Err: perrors.ErrDocumentNotOpen}
	}

	wasDirty := doc.IsDirty()
	doc.SetContent(content)
	isDirty := doc.IsDirty()

	// Notify if dirty state changed (copy slice to avoid races)
	if wasDirty != isDirty {
		s.mu.RLock()
		dirtyHandlers := make([]func(doc *Document, dirty bool), len(s.onDirty))
		copy(dirtyHandlers, s.onDirty)
		s.mu.RUnlock()
		for _, handler := range dirtyHandlers {
			handler(doc, isDirty)
		}
	}

	return nil
}

// ApplyEdit applies an incremental edit to an open document.
func (s *FileStore) ApplyEdit(path string, startOffset, endOffset int, newText []byte) error {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return &perrors.PathError{Op: "edit", Path: path, Err: err}
	}

	s.mu.RLock()
	doc, ok := s.documents[absPath]
	s.mu.RUnlock()

	if !ok {
		return &perrors.PathError{Op: "edit", Path: path, Err: perrors.ErrDocumentNotOpen}
	}

	wasDirty := doc.IsDirty()
	if err := doc.ApplyEdit(startOffset, endOffset, newText); err != nil {
		return &perrors.PathError{Op: "edit", Path: path, Err: err}
	}
	isDirty := doc.IsDirty()

	// Notify if dirty state changed (copy slice to avoid races)
	if wasDirty != isDirty {
		s.mu.RLock()
		editDirtyHandlers := make([]func(doc *Document, dirty bool), len(s.onDirty))
		copy(editDirtyHandlers, s.onDirty)
		s.mu.RUnlock()
		for _, handler := range editDirtyHandlers {
			handler(doc, isDirty)
		}
	}

	return nil
}

// CheckExternalChanges checks if any open documents have been modified externally.
// Returns documents that have external changes.
func (s *FileStore) CheckExternalChanges() []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var changed []*Document
	for path, doc := range s.documents {
		info, err := s.vfs.Stat(path)
		if err != nil {
			// File may have been deleted
			continue
		}
		if doc.HasExternalChanges(info.ModTime()) {
			changed = append(changed, doc)
		}
	}
	return changed
}

// CloseAll closes all open documents.
// If force is false, returns an error if any document is dirty.
func (s *FileStore) CloseAll(ctx context.Context, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for dirty documents first
	if !force {
		for _, doc := range s.documents {
			if doc.IsDirty() {
				return perrors.ErrDocumentDirty
			}
		}
	}

	// Copy close handlers before iteration to avoid races
	closeAllHandlers := make([]func(path string), len(s.onClose))
	copy(closeAllHandlers, s.onClose)

	// Close all documents
	for path, doc := range s.documents {
		doc.MarkClosed()
		delete(s.documents, path)

		// Notify handlers (using copied slice)
		for _, handler := range closeAllHandlers {
			handler(path)
		}
	}

	return nil
}

// CreateFile creates a new file and opens it.
func (s *FileStore) CreateFile(ctx context.Context, path string, content []byte) (*Document, error) {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return nil, &perrors.PathError{Op: "create", Path: path, Err: err}
	}

	// Check if file already exists
	if s.vfs.Exists(absPath) {
		return nil, &perrors.PathError{Op: "create", Path: path, Err: perrors.ErrAlreadyExists}
	}

	// Check if parent directory exists
	parent := s.vfs.Dir(absPath)
	if !s.vfs.IsDir(parent) {
		return nil, &perrors.PathError{Op: "create", Path: path, Err: perrors.ErrNotFound}
	}

	// Create the file
	if err := s.vfs.WriteFile(absPath, content, 0644); err != nil {
		return nil, &perrors.PathError{Op: "create", Path: path, Err: err}
	}

	// Open it
	return s.Open(ctx, absPath)
}

// DeleteFile deletes a file.
// If the file is open, it must be closed first (or force must be true).
func (s *FileStore) DeleteFile(ctx context.Context, path string, force bool) error {
	absPath, err := s.vfs.Abs(path)
	if err != nil {
		return &perrors.PathError{Op: "delete", Path: path, Err: err}
	}

	// Check if file is open
	s.mu.Lock()
	doc, isOpen := s.documents[absPath]
	if isOpen {
		if !force && doc.IsDirty() {
			s.mu.Unlock()
			return &perrors.PathError{Op: "delete", Path: path, Err: perrors.ErrDocumentDirty}
		}
		// Close the document
		doc.MarkClosed()
		delete(s.documents, absPath)
	}
	s.mu.Unlock()

	// Delete from disk
	if err := s.vfs.Remove(absPath); err != nil {
		return &perrors.PathError{Op: "delete", Path: path, Err: err}
	}

	// Notify close handlers if was open (copy slice to avoid races)
	if isOpen {
		s.mu.RLock()
		deleteHandlers := make([]func(path string), len(s.onClose))
		copy(deleteHandlers, s.onClose)
		s.mu.RUnlock()
		for _, handler := range deleteHandlers {
			handler(absPath)
		}
	}

	return nil
}

// RenameFile renames a file and updates any open document.
func (s *FileStore) RenameFile(ctx context.Context, oldPath, newPath string) error {
	oldAbsPath, err := s.vfs.Abs(oldPath)
	if err != nil {
		return &perrors.PathError{Op: "rename", Path: oldPath, Err: err}
	}

	newAbsPath, err := s.vfs.Abs(newPath)
	if err != nil {
		return &perrors.PathError{Op: "rename", Path: newPath, Err: err}
	}

	// Check if new path already exists
	if s.vfs.Exists(newAbsPath) {
		return &perrors.PathError{Op: "rename", Path: newPath, Err: perrors.ErrAlreadyExists}
	}

	s.mu.Lock()
	doc, isOpen := s.documents[oldAbsPath]

	// Check if new path has an open document
	if _, exists := s.documents[newAbsPath]; exists {
		s.mu.Unlock()
		return &perrors.PathError{Op: "rename", Path: newPath, Err: perrors.ErrAlreadyOpen}
	}

	// Rename on disk
	if err := s.vfs.Rename(oldAbsPath, newAbsPath); err != nil {
		s.mu.Unlock()
		return &perrors.PathError{Op: "rename", Path: oldPath, Err: err}
	}

	// Update document if open
	if isOpen {
		doc.mu.Lock()
		doc.Path = newAbsPath
		doc.LanguageID = detectLanguageID(newAbsPath)
		doc.mu.Unlock()

		delete(s.documents, oldAbsPath)
		s.documents[newAbsPath] = doc
	}
	s.mu.Unlock()

	return nil
}

// Event handler registration

// OnOpen registers a handler called when a document is opened.
func (s *FileStore) OnOpen(handler func(doc *Document)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onOpen = append(s.onOpen, handler)
}

// OnClose registers a handler called when a document is closed.
func (s *FileStore) OnClose(handler func(path string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onClose = append(s.onClose, handler)
}

// OnSave registers a handler called when a document is saved.
func (s *FileStore) OnSave(handler func(doc *Document)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSave = append(s.onSave, handler)
}

// OnDirty registers a handler called when a document's dirty state changes.
func (s *FileStore) OnDirty(handler func(doc *Document, dirty bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDirty = append(s.onDirty, handler)
}

// OnReload registers a handler called when a document is reloaded.
func (s *FileStore) OnReload(handler func(doc *Document)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onReload = append(s.onReload, handler)
}

// Stats returns statistics about the file store.
type Stats struct {
	OpenCount  int
	DirtyCount int
	TotalSize  int64
}

// GetStats returns current file store statistics.
func (s *FileStore) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := Stats{
		OpenCount: len(s.documents),
	}

	for _, doc := range s.documents {
		if doc.IsDirty() {
			stats.DirtyCount++
		}
		stats.TotalSize += int64(doc.Size())
	}

	return stats
}

// Ensure FileStore's WriteFile permission is available
var _ fs.FileMode = 0644
