package vfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// OSFS implements VFS using the operating system's file system.
type OSFS struct{}

// NewOSFS creates a new OS file system.
func NewOSFS() *OSFS {
	return &OSFS{}
}

// Ensure OSFS implements VFS.
var _ VFS = (*OSFS)(nil)

// Open opens a file for reading.
func (f *OSFS) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

// ReadFile reads the entire file content.
func (f *OSFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Stat returns file information.
func (f *OSFS) Stat(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}
	return osFileInfoToVFS(path, info), nil
}

// ReadDir reads a directory and returns its entries.
func (f *OSFS) ReadDir(path string) ([]FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	infos := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't stat
		}
		entryPath := filepath.Join(path, entry.Name())
		infos = append(infos, osFileInfoToVFS(entryPath, info))
	}
	return infos, nil
}

// WriteFile writes data to a file, creating it if necessary.
func (f *OSFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// Create creates a file for writing.
func (f *OSFS) Create(path string) (io.WriteCloser, error) {
	return os.Create(path)
}

// Mkdir creates a directory.
func (f *OSFS) Mkdir(path string, perm fs.FileMode) error {
	return os.Mkdir(path, perm)
}

// MkdirAll creates a directory and all parent directories.
func (f *OSFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Remove removes a file or empty directory.
func (f *OSFS) Remove(path string) error {
	return os.Remove(path)
}

// RemoveAll removes a path and all its contents.
func (f *OSFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Rename renames (moves) a file or directory.
func (f *OSFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// Abs returns the absolute path.
func (f *OSFS) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Rel returns the relative path from base to target.
func (f *OSFS) Rel(basePath, targetPath string) (string, error) {
	return filepath.Rel(basePath, targetPath)
}

// Join joins path elements.
func (f *OSFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Dir returns the directory portion of a path.
func (f *OSFS) Dir(path string) string {
	return filepath.Dir(path)
}

// Base returns the last element of a path.
func (f *OSFS) Base(path string) string {
	return filepath.Base(path)
}

// Ext returns the file extension.
func (f *OSFS) Ext(path string) string {
	return filepath.Ext(path)
}

// Clean returns the cleaned path.
func (f *OSFS) Clean(path string) string {
	return filepath.Clean(path)
}

// Exists returns true if the path exists.
func (f *OSFS) Exists(path string) bool {
	_, err := os.Stat(path)
	// Return true unless we confirm the file doesn't exist.
	// Permission errors mean we can't determine existence, but the path may exist.
	return !errors.Is(err, os.ErrNotExist)
}

// IsDir returns true if the path is a directory.
func (f *OSFS) IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsRegular returns true if the path is a regular file.
func (f *OSFS) IsRegular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// Glob returns paths matching the pattern.
func (f *OSFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// Walk walks the file tree rooted at root.
func (f *OSFS) Walk(root string, fn WalkFunc) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fn(path, FileInfo{}, err)
		}
		return fn(path, osFileInfoToVFS(path, info), nil)
	})
}

// WalkDir walks the file tree rooted at root.
func (f *OSFS) WalkDir(root string, fn WalkDirFunc) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fn(path, nil, err)
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return fn(path, nil, infoErr)
		}
		return fn(path, NewDirEntry(osFileInfoToVFS(path, info)), nil)
	})
}

// osFileInfoToVFS converts os.FileInfo to vfs.FileInfo.
func osFileInfoToVFS(path string, info os.FileInfo) FileInfo {
	return NewFileInfo(
		path,
		info.Name(),
		info.Size(),
		info.Mode(),
		info.ModTime(),
		info.IsDir(),
	)
}

// FileInfoFromOS creates a FileInfo from os.FileInfo.
// This is useful when converting results from other packages.
func FileInfoFromOS(path string, info os.FileInfo) FileInfo {
	return osFileInfoToVFS(path, info)
}

// TempDir creates a temporary directory and returns its path.
// The caller is responsible for removing the directory when done.
func TempDir(pattern string) (string, error) {
	return os.MkdirTemp("", pattern)
}

// TempFile creates a temporary file and returns its path.
// The file is closed after creation. The caller is responsible
// for removing the file when done.
func TempFile(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	return name, nil
}

// osFileInfo wraps vfs.FileInfo to implement os.FileInfo interface.
type osFileInfo struct {
	fi FileInfo
}

// Name returns the base name.
func (o *osFileInfo) Name() string { return o.fi.Name() }

// Size returns the file size.
func (o *osFileInfo) Size() int64 { return o.fi.Size() }

// Mode returns the file mode.
func (o *osFileInfo) Mode() fs.FileMode { return o.fi.Mode() }

// ModTime returns the modification time.
func (o *osFileInfo) ModTime() time.Time { return o.fi.ModTime() }

// IsDir returns true if this is a directory.
func (o *osFileInfo) IsDir() bool { return o.fi.IsDir() }

// Sys returns nil.
func (o *osFileInfo) Sys() any { return nil }

// ToOSFileInfo converts a vfs.FileInfo to os.FileInfo.
func ToOSFileInfo(fi FileInfo) os.FileInfo {
	return &osFileInfo{fi: fi}
}
