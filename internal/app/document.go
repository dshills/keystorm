package app

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/dshills/keystorm/internal/engine"
	"github.com/dshills/keystorm/internal/lsp"
	"github.com/dshills/keystorm/internal/renderer"
)

// Document represents an open file with its associated editor state.
type Document struct {
	// Path is the absolute file path (empty for scratch buffers).
	Path string

	// Name is the display name (filename or "Untitled").
	Name string

	// Engine is the text buffer and editing engine.
	Engine *engine.Engine

	// LanguageID is the detected language for LSP.
	LanguageID string

	// Modified indicates unsaved changes.
	modified atomic.Bool

	// ReadOnly indicates the document cannot be edited.
	ReadOnly bool

	// Version tracks document changes for LSP sync.
	version atomic.Int64

	// lspOpened tracks if document was opened with LSP.
	lspOpened atomic.Bool
}

// NewDocument creates a new document from a file path.
func NewDocument(path string, content []byte) *Document {
	name := filepath.Base(path)
	if path == "" {
		name = "Untitled"
	}

	eng := engine.New(engine.WithContent(string(content)))

	doc := &Document{
		Path:       path,
		Name:       name,
		Engine:     eng,
		LanguageID: lsp.DetectLanguageID(path),
	}

	return doc
}

// NewScratchDocument creates a new scratch (unsaved) document.
func NewScratchDocument() *Document {
	return &Document{
		Path:   "",
		Name:   "Untitled",
		Engine: engine.New(),
	}
}

// IsModified returns true if the document has unsaved changes.
func (d *Document) IsModified() bool {
	return d.modified.Load()
}

// SetModified sets the modified flag.
func (d *Document) SetModified(modified bool) {
	d.modified.Store(modified)
}

// IsScratch returns true if this is a scratch buffer (no file path).
func (d *Document) IsScratch() bool {
	return d.Path == ""
}

// Version returns the current document version for LSP.
func (d *Document) Version() int64 {
	return d.version.Load()
}

// IncrementVersion increments and returns the new version.
func (d *Document) IncrementVersion() int64 {
	return d.version.Add(1)
}

// IsLSPOpened returns true if the document was opened with LSP.
func (d *Document) IsLSPOpened() bool {
	return d.lspOpened.Load()
}

// SetLSPOpened marks the document as opened with LSP.
func (d *Document) SetLSPOpened(opened bool) {
	d.lspOpened.Store(opened)
}

// Content returns the full document content.
func (d *Document) Content() string {
	return d.Engine.Text()
}

// DocumentManager manages all open documents.
type DocumentManager struct {
	mu        sync.RWMutex
	documents map[string]*Document // path -> document
	active    *Document
	order     []string // tracks open order for navigation
	counter   int      // for generating scratch buffer names
}

// NewDocumentManager creates a new document manager.
func NewDocumentManager() *DocumentManager {
	return &DocumentManager{
		documents: make(map[string]*Document),
		order:     make([]string, 0),
	}
}

// Open opens a document from a file.
// Returns existing document if already open.
func (dm *DocumentManager) Open(path string) (*Document, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Check if already open
	if doc, exists := dm.documents[absPath]; exists {
		dm.active = doc
		return doc, nil
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	// Create document
	doc := NewDocument(absPath, content)
	dm.documents[absPath] = doc
	dm.order = append(dm.order, absPath)
	dm.active = doc

	return doc, nil
}

// CreateScratch creates a new scratch document.
func (dm *DocumentManager) CreateScratch() *Document {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.counter++
	doc := NewScratchDocument()

	// Use counter to create unique key
	key := scratchKey(dm.counter)
	if dm.counter > 1 {
		doc.Name = "Untitled-" + itoa(dm.counter)
	}

	dm.documents[key] = doc
	dm.order = append(dm.order, key)
	dm.active = doc

	return doc
}

// Close closes a document by path.
func (dm *DocumentManager) Close(path string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, exists := dm.documents[path]
	if !exists {
		return ErrDocumentNotFound
	}

	// Remove from documents map
	delete(dm.documents, path)

	// Remove from order
	for i, p := range dm.order {
		if p == path {
			dm.order = append(dm.order[:i], dm.order[i+1:]...)
			break
		}
	}

	// Update active document
	if dm.active == doc {
		if len(dm.order) > 0 {
			dm.active = dm.documents[dm.order[len(dm.order)-1]]
		} else {
			dm.active = nil
		}
	}

	return nil
}

// Active returns the currently active document.
func (dm *DocumentManager) Active() *Document {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.active
}

// SetActive sets the active document.
func (dm *DocumentManager) SetActive(doc *Document) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.active = doc
}

// SetActiveByPath sets the active document by path.
func (dm *DocumentManager) SetActiveByPath(path string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, exists := dm.documents[path]
	if !exists {
		return ErrDocumentNotFound
	}
	dm.active = doc
	return nil
}

// Get returns a document by path.
func (dm *DocumentManager) Get(path string) (*Document, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	doc, exists := dm.documents[path]
	return doc, exists
}

// All returns all open documents.
func (dm *DocumentManager) All() []*Document {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	docs := make([]*Document, 0, len(dm.documents))
	for _, path := range dm.order {
		if doc, exists := dm.documents[path]; exists {
			docs = append(docs, doc)
		}
	}
	return docs
}

// Count returns the number of open documents.
func (dm *DocumentManager) Count() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return len(dm.documents)
}

// DirtyDocuments returns all documents with unsaved changes.
func (dm *DocumentManager) DirtyDocuments() []*Document {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var dirty []*Document
	for _, doc := range dm.documents {
		if doc.IsModified() {
			dirty = append(dirty, doc)
		}
	}
	return dirty
}

// HasDirty returns true if any document has unsaved changes.
func (dm *DocumentManager) HasDirty() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, doc := range dm.documents {
		if doc.IsModified() {
			return true
		}
	}
	return false
}

// Next returns the next document in order (for buffer switching).
func (dm *DocumentManager) Next() *Document {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if len(dm.order) == 0 || dm.active == nil {
		return nil
	}

	// Find current index
	currentIdx := -1
	var currentPath string
	for _, path := range dm.order {
		if dm.documents[path] == dm.active {
			currentPath = path
			break
		}
	}

	for i, path := range dm.order {
		if path == currentPath {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return dm.active
	}

	// Wrap around
	nextIdx := (currentIdx + 1) % len(dm.order)
	dm.active = dm.documents[dm.order[nextIdx]]
	return dm.active
}

// Previous returns the previous document in order.
func (dm *DocumentManager) Previous() *Document {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if len(dm.order) == 0 || dm.active == nil {
		return nil
	}

	// Find current index
	currentIdx := -1
	var currentPath string
	for _, path := range dm.order {
		if dm.documents[path] == dm.active {
			currentPath = path
			break
		}
	}

	for i, path := range dm.order {
		if path == currentPath {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return dm.active
	}

	// Wrap around
	prevIdx := currentIdx - 1
	if prevIdx < 0 {
		prevIdx = len(dm.order) - 1
	}
	dm.active = dm.documents[dm.order[prevIdx]]
	return dm.active
}

// scratchKey generates a key for scratch buffers.
func scratchKey(n int) string {
	return "::scratch::" + itoa(n)
}

// scratchKey generates a key for scratch buffers.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// DocumentCursorProvider implements renderer.CursorProvider for a document.
// It adapts the engine's cursor position to the renderer's expected interface.
type DocumentCursorProvider struct {
	dm *DocumentManager
}

// NewDocumentCursorProvider creates a cursor provider for the document manager.
func NewDocumentCursorProvider(dm *DocumentManager) *DocumentCursorProvider {
	return &DocumentCursorProvider{dm: dm}
}

// PrimaryCursor returns the primary cursor position as (line, column).
func (p *DocumentCursorProvider) PrimaryCursor() (line uint32, col uint32) {
	doc := p.dm.Active()
	if doc == nil || doc.Engine == nil {
		return 0, 0
	}

	offset := doc.Engine.PrimaryCursor()
	point := doc.Engine.OffsetToPoint(engine.ByteOffset(offset))
	return point.Line, point.Column
}

// Selections returns all active selections for rendering.
func (p *DocumentCursorProvider) Selections() []renderer.Selection {
	doc := p.dm.Active()
	if doc == nil || doc.Engine == nil {
		return nil
	}

	// Get primary selection
	sel := doc.Engine.PrimarySelection()
	if sel.IsEmpty() {
		return nil
	}

	startPoint := doc.Engine.OffsetToPoint(engine.ByteOffset(sel.Start()))
	endPoint := doc.Engine.OffsetToPoint(engine.ByteOffset(sel.End()))

	return []renderer.Selection{
		{
			StartLine: startPoint.Line,
			StartCol:  startPoint.Column,
			EndLine:   endPoint.Line,
			EndCol:    endPoint.Column,
			IsPrimary: true,
		},
	}
}
