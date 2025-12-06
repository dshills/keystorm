package project

import (
	perrors "github.com/dshills/keystorm/internal/project/errors"
)

// Re-export error types from errors subpackage for backwards compatibility.
// New code should import github.com/dshills/keystorm/internal/project/errors directly.

// Standard errors - re-exported from errors package.
var (
	ErrNotOpen             = perrors.ErrNotOpen
	ErrAlreadyOpen         = perrors.ErrAlreadyOpen
	ErrNotFound            = perrors.ErrNotFound
	ErrNotInWorkspace      = perrors.ErrNotInWorkspace
	ErrIsDirectory         = perrors.ErrIsDirectory
	ErrNotDirectory        = perrors.ErrNotDirectory
	ErrAlreadyExists       = perrors.ErrAlreadyExists
	ErrReadOnly            = perrors.ErrReadOnly
	ErrFileTooLarge        = perrors.ErrFileTooLarge
	ErrBinaryFile          = perrors.ErrBinaryFile
	ErrDocumentNotOpen     = perrors.ErrDocumentNotOpen
	ErrDocumentDirty       = perrors.ErrDocumentDirty
	ErrIndexing            = perrors.ErrIndexing
	ErrWatcherFailed       = perrors.ErrWatcherFailed
	ErrEncodingUnsupported = perrors.ErrEncodingUnsupported
	ErrInvalidQuery        = perrors.ErrInvalidQuery
)

// PathError is re-exported from the errors package.
type PathError = perrors.PathError

// WorkspaceError is re-exported from the errors package.
type WorkspaceError = perrors.WorkspaceError

// IndexError is re-exported from the errors package.
type IndexError = perrors.IndexError

// NewPathError creates a new PathError.
func NewPathError(op, path string, err error) *PathError {
	return perrors.NewPathError(op, path, err)
}

// IsNotFound returns true if the error indicates a file was not found.
func IsNotFound(err error) bool {
	return perrors.IsNotFound(err)
}

// IsNotInWorkspace returns true if the error indicates path is outside workspace.
func IsNotInWorkspace(err error) bool {
	return perrors.IsNotInWorkspace(err)
}

// IsDirty returns true if the error indicates document has unsaved changes.
func IsDirty(err error) bool {
	return perrors.IsDirty(err)
}
