package project

import (
	"errors"
	"fmt"
)

// Standard errors returned by the project package.
var (
	// ErrNotOpen indicates no workspace is currently open.
	ErrNotOpen = errors.New("no workspace open")

	// ErrAlreadyOpen indicates a workspace is already open.
	ErrAlreadyOpen = errors.New("workspace already open")

	// ErrNotFound indicates a file or directory was not found.
	ErrNotFound = errors.New("not found")

	// ErrNotInWorkspace indicates the path is outside the workspace.
	ErrNotInWorkspace = errors.New("path not in workspace")

	// ErrIsDirectory indicates the path is a directory, not a file.
	ErrIsDirectory = errors.New("path is a directory")

	// ErrNotDirectory indicates the path is a file, not a directory.
	ErrNotDirectory = errors.New("path is not a directory")

	// ErrAlreadyExists indicates the file or directory already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrReadOnly indicates the file is read-only.
	ErrReadOnly = errors.New("file is read-only")

	// ErrFileTooLarge indicates the file exceeds the maximum size limit.
	ErrFileTooLarge = errors.New("file too large")

	// ErrBinaryFile indicates the file appears to be binary.
	ErrBinaryFile = errors.New("binary file")

	// ErrDocumentNotOpen indicates the document is not open.
	ErrDocumentNotOpen = errors.New("document not open")

	// ErrDocumentDirty indicates the document has unsaved changes.
	ErrDocumentDirty = errors.New("document has unsaved changes")

	// ErrIndexing indicates an indexing operation is in progress.
	ErrIndexing = errors.New("indexing in progress")

	// ErrWatcherFailed indicates the file watcher failed.
	ErrWatcherFailed = errors.New("file watcher failed")

	// ErrEncodingUnsupported indicates the file encoding is not supported.
	ErrEncodingUnsupported = errors.New("unsupported encoding")
)

// PathError represents an error associated with a file path.
type PathError struct {
	Op   string // Operation that failed (open, read, write, etc.)
	Path string // File path
	Err  error  // Underlying error
}

// Error implements the error interface.
func (e *PathError) Error() string {
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

// Unwrap returns the underlying error.
func (e *PathError) Unwrap() error {
	return e.Err
}

// NewPathError creates a new PathError.
func NewPathError(op, path string, err error) *PathError {
	return &PathError{Op: op, Path: path, Err: err}
}

// WorkspaceError represents an error related to workspace operations.
type WorkspaceError struct {
	Root string // Workspace root path
	Err  error  // Underlying error
}

// Error implements the error interface.
func (e *WorkspaceError) Error() string {
	return fmt.Sprintf("workspace %s: %v", e.Root, e.Err)
}

// Unwrap returns the underlying error.
func (e *WorkspaceError) Unwrap() error {
	return e.Err
}

// IndexError represents an error during indexing.
type IndexError struct {
	Path string // File being indexed (empty for general errors)
	Err  error  // Underlying error
}

// Error implements the error interface.
func (e *IndexError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("index %s: %v", e.Path, e.Err)
	}
	return fmt.Sprintf("index: %v", e.Err)
}

// Unwrap returns the underlying error.
func (e *IndexError) Unwrap() error {
	return e.Err
}

// IsNotFound returns true if the error indicates a file was not found.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsNotInWorkspace returns true if the error indicates path is outside workspace.
func IsNotInWorkspace(err error) bool {
	return errors.Is(err, ErrNotInWorkspace)
}

// IsDirty returns true if the error indicates document has unsaved changes.
func IsDirty(err error) bool {
	return errors.Is(err, ErrDocumentDirty)
}
