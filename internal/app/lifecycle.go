// Package app provides the main application structure and coordination.
package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/dshills/keystorm/internal/engine/buffer"
)

// SaveDocument saves the active document to disk.
func (app *Application) SaveDocument() error {
	doc := app.documents.Active()
	if doc == nil {
		return ErrNoActiveDocument
	}

	if doc.IsScratch() {
		return ErrNoFilePath
	}

	if doc.ReadOnly {
		return ErrReadOnly
	}

	// Get document content
	content := doc.Content()

	// Write to file
	if err := os.WriteFile(doc.Path, []byte(content), 0644); err != nil {
		return &FileError{Op: "save", Path: doc.Path, Err: err}
	}

	// Clear modified flag
	doc.SetModified(false)

	return nil
}

// SaveDocumentAs saves the active document to a new path.
func (app *Application) SaveDocumentAs(path string) error {
	doc := app.documents.Active()
	if doc == nil {
		return ErrNoActiveDocument
	}

	// Get document content
	content := doc.Content()

	// Write to file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return &FileError{Op: "save", Path: path, Err: err}
	}

	// Update document path and name
	doc.Path = path
	doc.Name = pathBase(path)
	doc.SetModified(false)

	return nil
}

// CloseDocument closes the specified document.
// Returns ErrUnsavedChanges if document has unsaved changes and force is false.
func (app *Application) CloseDocument(doc *Document, force bool) error {
	if doc == nil {
		return ErrNoActiveDocument
	}

	if doc.IsModified() && !force {
		return ErrUnsavedChanges
	}

	// Close LSP document if needed
	if doc.IsLSPOpened() && app.lsp != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		app.lsp.CloseDocument(ctx, doc.Path)
	}

	// Remove from document manager
	var key string
	if doc.IsScratch() {
		// Find scratch key
		for k, d := range app.documents.documents {
			if d == doc {
				key = k
				break
			}
		}
	} else {
		key = doc.Path
	}

	if key != "" {
		return app.documents.Close(key)
	}

	return nil
}

// CloseActiveDocument closes the active document.
func (app *Application) CloseActiveDocument(force bool) error {
	return app.CloseDocument(app.documents.Active(), force)
}

// OpenFile opens a file and creates a document for it.
func (app *Application) OpenFile(path string) (*Document, error) {
	// Use document manager to open
	doc, err := app.documents.Open(path)
	if err != nil {
		return nil, &FileError{Op: "open", Path: path, Err: err}
	}

	// Notify LSP if available
	if app.lsp != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// OpenDocument takes (ctx, path, content) - languageID is detected internally
		if err := app.lsp.OpenDocument(ctx, doc.Path, doc.Content()); err != nil {
			// Non-fatal, continue without LSP
			_ = err
		} else {
			doc.SetLSPOpened(true)
		}
	}

	return doc, nil
}

// Quit initiates application shutdown.
// Returns ErrUnsavedChanges if there are unsaved changes and force is false.
func (app *Application) Quit(force bool) error {
	if !force && app.documents.HasDirty() {
		return ErrUnsavedChanges
	}

	app.Shutdown()
	return nil
}

// ForceQuit forces immediate shutdown, discarding unsaved changes.
func (app *Application) ForceQuit() {
	app.Shutdown()
}

// ConfirmQuit checks if quit is safe (no unsaved changes).
func (app *Application) ConfirmQuit() bool {
	return !app.documents.HasDirty()
}

// pathBase returns the base name of a path.
func pathBase(path string) string {
	return filepath.Base(path)
}

// FileError represents a file operation error.
type FileError struct {
	Op   string
	Path string
	Err  error
}

func (e *FileError) Error() string {
	if e.Err == nil {
		return e.Op + " " + e.Path
	}
	return e.Op + " " + e.Path + ": " + e.Err.Error()
}

func (e *FileError) Unwrap() error {
	return e.Err
}

// ErrNoFilePath indicates the document has no file path.
var ErrNoFilePath = &FileError{Op: "save", Err: errNoPath}

var errNoPath = constError("no file path")

// ErrReadOnly indicates the document is read-only.
var ErrReadOnly = constError("document is read-only")

// constError is a simple constant error type.
type constError string

func (e constError) Error() string { return string(e) }

// EngineAdapter adapts engine.Engine to execctx.EngineInterface.
// This bridges the gap between our engine implementation and what the dispatcher expects.
type EngineAdapter struct {
	engine EngineInterface
}

// EngineInterface defines the subset of engine.Engine methods we need.
type EngineInterface interface {
	Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error)
	Delete(start, end buffer.ByteOffset) (buffer.EditResult, error)
	Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error)
	Text() string
	TextRange(start, end buffer.ByteOffset) string
	LineText(line uint32) string
	Len() buffer.ByteOffset
	LineCount() uint32
	LineStartOffset(line uint32) buffer.ByteOffset
	LineEndOffset(line uint32) buffer.ByteOffset
	LineLen(line uint32) uint32
	OffsetToPoint(offset buffer.ByteOffset) buffer.Point
	PointToOffset(point buffer.Point) buffer.ByteOffset
	RevisionID() buffer.RevisionID
}

// NewEngineAdapter creates a new engine adapter.
func NewEngineAdapter(engine EngineInterface) *EngineAdapter {
	return &EngineAdapter{engine: engine}
}

// Insert inserts text at the given offset.
func (a *EngineAdapter) Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error) {
	return a.engine.Insert(offset, text)
}

// Delete removes text between start and end offsets.
func (a *EngineAdapter) Delete(start, end buffer.ByteOffset) (buffer.EditResult, error) {
	return a.engine.Delete(start, end)
}

// Replace replaces text between start and end with new text.
func (a *EngineAdapter) Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error) {
	return a.engine.Replace(start, end, text)
}

// Text returns the full document text.
func (a *EngineAdapter) Text() string {
	return a.engine.Text()
}

// TextRange returns text in the given range.
func (a *EngineAdapter) TextRange(start, end buffer.ByteOffset) string {
	return a.engine.TextRange(start, end)
}

// LineText returns the text of the given line.
func (a *EngineAdapter) LineText(line uint32) string {
	return a.engine.LineText(line)
}

// Len returns the total byte length.
func (a *EngineAdapter) Len() buffer.ByteOffset {
	return a.engine.Len()
}

// LineCount returns the number of lines.
func (a *EngineAdapter) LineCount() uint32 {
	return a.engine.LineCount()
}

// LineStartOffset returns the start offset of a line.
func (a *EngineAdapter) LineStartOffset(line uint32) buffer.ByteOffset {
	return a.engine.LineStartOffset(line)
}

// LineEndOffset returns the end offset of a line.
func (a *EngineAdapter) LineEndOffset(line uint32) buffer.ByteOffset {
	return a.engine.LineEndOffset(line)
}

// LineLen returns the length of a line.
func (a *EngineAdapter) LineLen(line uint32) uint32 {
	return a.engine.LineLen(line)
}

// OffsetToPoint converts a byte offset to a point (line, column).
func (a *EngineAdapter) OffsetToPoint(offset buffer.ByteOffset) buffer.Point {
	return a.engine.OffsetToPoint(offset)
}

// PointToOffset converts a point to a byte offset.
func (a *EngineAdapter) PointToOffset(point buffer.Point) buffer.ByteOffset {
	return a.engine.PointToOffset(point)
}

// RevisionID returns the current revision ID.
func (a *EngineAdapter) RevisionID() buffer.RevisionID {
	return a.engine.RevisionID()
}

// Snapshot returns a read-only snapshot - not implemented yet.
func (a *EngineAdapter) Snapshot() interface{} {
	// TODO: Implement snapshot functionality
	return nil
}

// ModeManagerAdapter adapts mode.Manager to execctx.ModeManagerInterface.
type ModeManagerAdapter struct {
	manager ModeManagerInterface
}

// ModeManagerInterface defines the mode manager methods we need.
type ModeManagerInterface interface {
	Current() ModeInterface
	SetInitialMode(name string) error
}

// ModeInterface defines a mode's methods.
type ModeInterface interface {
	Name() string
	DisplayName() string
}

// NewModeManagerAdapter creates a new mode manager adapter.
func NewModeManagerAdapter(manager ModeManagerInterface) *ModeManagerAdapter {
	return &ModeManagerAdapter{manager: manager}
}

// Current returns the current mode.
func (a *ModeManagerAdapter) Current() ModeAdapterInterface {
	if a.manager == nil {
		return nil
	}
	m := a.manager.Current()
	if m == nil {
		return nil
	}
	return &modeAdapter{mode: m}
}

// CurrentName returns the current mode name.
func (a *ModeManagerAdapter) CurrentName() string {
	if a.manager == nil || a.manager.Current() == nil {
		return ""
	}
	return a.manager.Current().Name()
}

// Switch switches to a named mode.
func (a *ModeManagerAdapter) Switch(name string) error {
	if a.manager == nil {
		return nil
	}
	return a.manager.SetInitialMode(name)
}

// Push pushes a new mode onto the stack.
func (a *ModeManagerAdapter) Push(name string) error {
	// Mode push is not yet implemented in mode.Manager
	return a.Switch(name)
}

// Pop pops the current mode from the stack.
func (a *ModeManagerAdapter) Pop() error {
	// Mode pop is not yet implemented in mode.Manager
	return nil
}

// IsMode returns true if the current mode matches the given name.
func (a *ModeManagerAdapter) IsMode(name string) bool {
	return a.CurrentName() == name
}

// IsAnyMode returns true if the current mode matches any of the given names.
func (a *ModeManagerAdapter) IsAnyMode(names ...string) bool {
	current := a.CurrentName()
	for _, name := range names {
		if current == name {
			return true
		}
	}
	return false
}

// ModeAdapterInterface is the adapted mode interface.
type ModeAdapterInterface interface {
	Name() string
	DisplayName() string
}

// modeAdapter wraps a Mode to implement ModeAdapterInterface.
type modeAdapter struct {
	mode ModeInterface
}

func (a *modeAdapter) Name() string {
	if a.mode == nil {
		return ""
	}
	return a.mode.Name()
}

func (a *modeAdapter) DisplayName() string {
	if a.mode == nil {
		return ""
	}
	return a.mode.DisplayName()
}
