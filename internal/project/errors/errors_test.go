package errors

import (
	"errors"
	"testing"
)

func TestPathError(t *testing.T) {
	err := &PathError{
		Op:   "open",
		Path: "/test/file.txt",
		Err:  ErrNotFound,
	}

	// Test Error()
	errStr := err.Error()
	if errStr != "open /test/file.txt: not found" {
		t.Errorf("Error() = %q, want 'open /test/file.txt: not found'", errStr)
	}

	// Test Unwrap()
	if err.Unwrap() != ErrNotFound {
		t.Error("Unwrap() should return underlying error")
	}
}

func TestNewPathError(t *testing.T) {
	err := NewPathError("read", "/test.txt", ErrNotFound)
	if err.Op != "read" {
		t.Errorf("Op = %q, want 'read'", err.Op)
	}
	if err.Path != "/test.txt" {
		t.Errorf("Path = %q, want '/test.txt'", err.Path)
	}
	if err.Err != ErrNotFound {
		t.Error("Err should be ErrNotFound")
	}
}

func TestWorkspaceError(t *testing.T) {
	err := &WorkspaceError{
		Root: "/workspace",
		Err:  ErrNotOpen,
	}

	errStr := err.Error()
	if errStr != "workspace /workspace: no workspace open" {
		t.Errorf("Error() = %q, want 'workspace /workspace: no workspace open'", errStr)
	}

	if err.Unwrap() != ErrNotOpen {
		t.Error("Unwrap() should return underlying error")
	}
}

func TestIndexError(t *testing.T) {
	// With path
	err := &IndexError{
		Path: "/test/file.go",
		Err:  ErrIndexing,
	}

	errStr := err.Error()
	if errStr != "index /test/file.go: indexing in progress" {
		t.Errorf("Error() = %q, want 'index /test/file.go: indexing in progress'", errStr)
	}

	// Without path
	err2 := &IndexError{
		Err: ErrIndexing,
	}

	errStr2 := err2.Error()
	if errStr2 != "index: indexing in progress" {
		t.Errorf("Error() = %q, want 'index: indexing in progress'", errStr2)
	}

	if err.Unwrap() != ErrIndexing {
		t.Error("Unwrap() should return underlying error")
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("IsNotFound(ErrNotFound) should be true")
	}

	wrapped := NewPathError("open", "/test", ErrNotFound)
	if !IsNotFound(wrapped) {
		t.Error("IsNotFound should work with wrapped errors")
	}

	if IsNotFound(ErrNotOpen) {
		t.Error("IsNotFound(ErrNotOpen) should be false")
	}
}

func TestIsNotInWorkspace(t *testing.T) {
	if !IsNotInWorkspace(ErrNotInWorkspace) {
		t.Error("IsNotInWorkspace(ErrNotInWorkspace) should be true")
	}

	if IsNotInWorkspace(ErrNotFound) {
		t.Error("IsNotInWorkspace(ErrNotFound) should be false")
	}
}

func TestIsDirty(t *testing.T) {
	if !IsDirty(ErrDocumentDirty) {
		t.Error("IsDirty(ErrDocumentDirty) should be true")
	}

	if IsDirty(ErrNotFound) {
		t.Error("IsDirty(ErrNotFound) should be false")
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	allErrors := []error{
		ErrNotOpen,
		ErrAlreadyOpen,
		ErrNotFound,
		ErrNotInWorkspace,
		ErrIsDirectory,
		ErrNotDirectory,
		ErrAlreadyExists,
		ErrReadOnly,
		ErrFileTooLarge,
		ErrBinaryFile,
		ErrDocumentNotOpen,
		ErrDocumentDirty,
		ErrIndexing,
		ErrWatcherFailed,
		ErrEncodingUnsupported,
		ErrInvalidQuery,
	}

	for i, err1 := range allErrors {
		for j, err2 := range allErrors {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("Error %d and %d should be distinct", i, j)
			}
		}
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrNotOpen, "no workspace open"},
		{ErrAlreadyOpen, "workspace already open"},
		{ErrNotFound, "not found"},
		{ErrNotInWorkspace, "path not in workspace"},
		{ErrIsDirectory, "path is a directory"},
		{ErrNotDirectory, "path is not a directory"},
		{ErrAlreadyExists, "already exists"},
		{ErrReadOnly, "file is read-only"},
		{ErrFileTooLarge, "file too large"},
		{ErrBinaryFile, "binary file"},
		{ErrDocumentNotOpen, "document not open"},
		{ErrDocumentDirty, "document has unsaved changes"},
		{ErrIndexing, "indexing in progress"},
		{ErrWatcherFailed, "file watcher failed"},
		{ErrEncodingUnsupported, "unsupported encoding"},
		{ErrInvalidQuery, "invalid query"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.want {
			t.Errorf("%v.Error() = %q, want %q", tt.err, tt.err.Error(), tt.want)
		}
	}
}
