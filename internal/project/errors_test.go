package project

import (
	"errors"
	"testing"
)

func TestPathError(t *testing.T) {
	err := NewPathError("read", "/path/to/file.txt", ErrNotFound)

	want := "read /path/to/file.txt: not found"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}

	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is should return true for underlying error")
	}

	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Error("errors.As should work for PathError")
	}

	if pathErr.Op != "read" {
		t.Errorf("Op = %q, want %q", pathErr.Op, "read")
	}

	if pathErr.Path != "/path/to/file.txt" {
		t.Errorf("Path = %q, want %q", pathErr.Path, "/path/to/file.txt")
	}
}

func TestWorkspaceError(t *testing.T) {
	underlying := errors.New("some error")
	err := &WorkspaceError{Root: "/workspace", Err: underlying}

	want := "workspace /workspace: some error"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}

	if !errors.Is(err, underlying) {
		t.Error("errors.Is should return true for underlying error")
	}
}

func TestIndexError(t *testing.T) {
	tests := []struct {
		name string
		err  *IndexError
		want string
	}{
		{
			name: "with path",
			err:  &IndexError{Path: "/file.go", Err: ErrFileTooLarge},
			want: "index /file.go: file too large",
		},
		{
			name: "without path",
			err:  &IndexError{Err: ErrIndexing},
			want: "index: indexing in progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct ErrNotFound",
			err:  ErrNotFound,
			want: true,
		},
		{
			name: "wrapped ErrNotFound",
			err:  NewPathError("read", "/file", ErrNotFound),
			want: true,
		},
		{
			name: "different error",
			err:  ErrIsDirectory,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNotInWorkspace(t *testing.T) {
	if !IsNotInWorkspace(ErrNotInWorkspace) {
		t.Error("IsNotInWorkspace(ErrNotInWorkspace) should be true")
	}

	wrapped := NewPathError("open", "/outside", ErrNotInWorkspace)
	if !IsNotInWorkspace(wrapped) {
		t.Error("IsNotInWorkspace should work with wrapped error")
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

func TestErrorMessages(t *testing.T) {
	// Just verify all errors have reasonable messages
	errs := []error{
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
	}

	for _, err := range errs {
		if err.Error() == "" {
			t.Errorf("error %T has empty message", err)
		}
	}
}
