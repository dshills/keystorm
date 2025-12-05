// Package vfs provides a virtual file system abstraction.
//
// The VFS interface allows swapping the underlying file system implementation,
// enabling testing with in-memory file systems and potential future support
// for remote file systems (SSH, cloud storage).
package vfs

import (
	"io"
	"io/fs"
	"time"
)

// VFS is a virtual file system abstraction.
// It provides a unified interface for file operations across different
// storage backends (OS file system, in-memory, remote).
type VFS interface {
	// Read operations

	// Open opens a file for reading.
	Open(path string) (io.ReadCloser, error)

	// ReadFile reads the entire file content.
	ReadFile(path string) ([]byte, error)

	// Stat returns file information.
	Stat(path string) (FileInfo, error)

	// ReadDir reads a directory and returns its entries.
	ReadDir(path string) ([]FileInfo, error)

	// Write operations

	// WriteFile writes data to a file, creating it if necessary.
	WriteFile(path string, data []byte, perm fs.FileMode) error

	// Create creates a file for writing.
	Create(path string) (io.WriteCloser, error)

	// Mkdir creates a directory.
	Mkdir(path string, perm fs.FileMode) error

	// MkdirAll creates a directory and all parent directories.
	MkdirAll(path string, perm fs.FileMode) error

	// Remove removes a file or empty directory.
	Remove(path string) error

	// RemoveAll removes a path and all its contents.
	RemoveAll(path string) error

	// Rename renames (moves) a file or directory.
	Rename(oldPath, newPath string) error

	// Path operations

	// Abs returns the absolute path.
	Abs(path string) (string, error)

	// Rel returns the relative path from base to target.
	Rel(basePath, targetPath string) (string, error)

	// Join joins path elements.
	Join(elem ...string) string

	// Dir returns the directory portion of a path.
	Dir(path string) string

	// Base returns the last element of a path.
	Base(path string) string

	// Ext returns the file extension.
	Ext(path string) string

	// Clean returns the cleaned path.
	Clean(path string) string

	// Queries

	// Exists returns true if the path exists.
	Exists(path string) bool

	// IsDir returns true if the path is a directory.
	IsDir(path string) bool

	// IsRegular returns true if the path is a regular file.
	IsRegular(path string) bool

	// Glob returns paths matching the pattern.
	Glob(pattern string) ([]string, error)

	// Walk walks the file tree rooted at root.
	Walk(root string, fn WalkFunc) error

	// WalkDir walks the file tree rooted at root (more efficient than Walk).
	WalkDir(root string, fn WalkDirFunc) error
}

// FileInfo describes a file or directory.
type FileInfo struct {
	path    string
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

// NewFileInfo creates a FileInfo from the given parameters.
func NewFileInfo(path, name string, size int64, mode fs.FileMode, modTime time.Time, isDir bool) FileInfo {
	return FileInfo{
		path:    path,
		name:    name,
		size:    size,
		mode:    mode,
		modTime: modTime,
		isDir:   isDir,
	}
}

// Path returns the full path.
func (fi FileInfo) Path() string { return fi.path }

// Name returns the base name.
func (fi FileInfo) Name() string { return fi.name }

// Size returns the file size in bytes.
func (fi FileInfo) Size() int64 { return fi.size }

// Mode returns the file mode.
func (fi FileInfo) Mode() fs.FileMode { return fi.mode }

// ModTime returns the modification time.
func (fi FileInfo) ModTime() time.Time { return fi.modTime }

// IsDir returns true if this is a directory.
func (fi FileInfo) IsDir() bool { return fi.isDir }

// IsRegular returns true if this is a regular file.
func (fi FileInfo) IsRegular() bool { return fi.mode.IsRegular() }

// IsSymlink returns true if this is a symbolic link.
func (fi FileInfo) IsSymlink() bool { return fi.mode&fs.ModeSymlink != 0 }

// Sys returns nil (for compatibility with fs.FileInfo).
func (fi FileInfo) Sys() any { return nil }

// WalkFunc is the type of function called by Walk.
type WalkFunc func(path string, info FileInfo, err error) error

// WalkDirFunc is the type of function called by WalkDir.
type WalkDirFunc func(path string, d DirEntry, err error) error

// DirEntry is the interface for directory entries.
type DirEntry interface {
	// Name returns the name of the file or directory.
	Name() string

	// IsDir returns true if this is a directory.
	IsDir() bool

	// Type returns the file mode type bits.
	Type() fs.FileMode

	// Info returns the FileInfo for this entry.
	Info() (FileInfo, error)
}

// dirEntry implements DirEntry.
type dirEntry struct {
	info FileInfo
}

// NewDirEntry creates a DirEntry from FileInfo.
func NewDirEntry(info FileInfo) DirEntry {
	return &dirEntry{info: info}
}

func (d *dirEntry) Name() string            { return d.info.Name() }
func (d *dirEntry) IsDir() bool             { return d.info.IsDir() }
func (d *dirEntry) Type() fs.FileMode       { return d.info.Mode().Type() }
func (d *dirEntry) Info() (FileInfo, error) { return d.info, nil }

// SkipDir is used as a return value from WalkFunc to indicate that
// the directory named in the call should be skipped.
var SkipDir = fs.SkipDir

// SkipAll is used as a return value from WalkFunc to indicate that
// all remaining files and directories should be skipped.
var SkipAll = fs.SkipAll
