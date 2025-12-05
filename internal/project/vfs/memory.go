package vfs

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Standard error values for MemFS operations.
// These align with POSIX errors for consistency with OSFS.
var (
	errIsDir    = syscall.EISDIR
	errNotDir   = syscall.ENOTDIR
	errNotEmpty = syscall.ENOTEMPTY
	errNotUnder = errors.New("target is not under base")
)

// MemFS implements VFS using an in-memory file system.
// It is primarily used for testing but can also be used for
// temporary workspaces or staging areas.
//
// MemFS is safe for concurrent use.
type MemFS struct {
	mu    sync.RWMutex
	files map[string]*memFile
	dirs  map[string]bool
}

type memFile struct {
	content []byte
	mode    fs.FileMode
	modTime time.Time
}

// NewMemFS creates a new in-memory file system.
func NewMemFS() *MemFS {
	return &MemFS{
		files: make(map[string]*memFile),
		dirs:  map[string]bool{"/": true},
	}
}

// Ensure MemFS implements VFS.
var _ VFS = (*MemFS)(nil)

// Open opens a file for reading.
func (m *MemFS) Open(filePath string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filePath = m.cleanPath(filePath)
	f, ok := m.files[filePath]
	if !ok {
		if m.dirs[filePath] {
			return nil, &fs.PathError{Op: "open", Path: filePath, Err: errIsDir}
		}
		return nil, &fs.PathError{Op: "open", Path: filePath, Err: fs.ErrNotExist}
	}

	return io.NopCloser(bytes.NewReader(f.content)), nil
}

// ReadFile reads the entire file content.
func (m *MemFS) ReadFile(filePath string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filePath = m.cleanPath(filePath)
	f, ok := m.files[filePath]
	if !ok {
		if m.dirs[filePath] {
			return nil, &fs.PathError{Op: "read", Path: filePath, Err: errIsDir}
		}
		return nil, &fs.PathError{Op: "read", Path: filePath, Err: fs.ErrNotExist}
	}

	// Return a copy to prevent modification
	content := make([]byte, len(f.content))
	copy(content, f.content)
	return content, nil
}

// Stat returns file information.
func (m *MemFS) Stat(filePath string) (FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filePath = m.cleanPath(filePath)

	if f, ok := m.files[filePath]; ok {
		return NewFileInfo(
			filePath,
			path.Base(filePath),
			int64(len(f.content)),
			f.mode,
			f.modTime,
			false,
		), nil
	}

	if m.dirs[filePath] {
		return NewFileInfo(
			filePath,
			path.Base(filePath),
			0,
			fs.ModeDir|0755,
			time.Now(),
			true,
		), nil
	}

	return FileInfo{}, &fs.PathError{Op: "stat", Path: filePath, Err: fs.ErrNotExist}
}

// ReadDir reads a directory and returns its entries.
func (m *MemFS) ReadDir(dirPath string) ([]FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dirPath = m.cleanPath(dirPath)

	if !m.dirs[dirPath] {
		if _, ok := m.files[dirPath]; ok {
			return nil, &fs.PathError{Op: "readdir", Path: dirPath, Err: errNotDir}
		}
		return nil, &fs.PathError{Op: "readdir", Path: dirPath, Err: fs.ErrNotExist}
	}

	var entries []FileInfo
	seen := make(map[string]bool)

	// Add files in this directory
	prefix := dirPath
	if prefix != "/" {
		prefix += "/"
	}

	for filePath, f := range m.files {
		if !strings.HasPrefix(filePath, prefix) {
			continue
		}
		rest := strings.TrimPrefix(filePath, prefix)
		if rest == "" || strings.Contains(rest, "/") {
			continue // Not a direct child
		}
		entries = append(entries, NewFileInfo(
			filePath,
			rest,
			int64(len(f.content)),
			f.mode,
			f.modTime,
			false,
		))
		seen[rest] = true
	}

	// Add subdirectories
	for d := range m.dirs {
		if !strings.HasPrefix(d, prefix) {
			continue
		}
		rest := strings.TrimPrefix(d, prefix)
		if rest == "" || strings.Contains(rest, "/") {
			continue // Not a direct child
		}
		if seen[rest] {
			continue
		}
		entries = append(entries, NewFileInfo(
			d,
			rest,
			0,
			fs.ModeDir|0755,
			time.Now(),
			true,
		))
	}

	// Sort by name
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	return entries, nil
}

// WriteFile writes data to a file, creating it if necessary.
func (m *MemFS) WriteFile(filePath string, data []byte, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath = m.cleanPath(filePath)

	// Check if path is a directory
	if m.dirs[filePath] {
		return &fs.PathError{Op: "write", Path: filePath, Err: errIsDir}
	}

	// Ensure parent directory exists
	dir := path.Dir(filePath)
	if dir != "/" && !m.dirs[dir] {
		return &fs.PathError{Op: "write", Path: filePath, Err: fs.ErrNotExist}
	}

	// Make a copy of the data
	content := make([]byte, len(data))
	copy(content, data)

	m.files[filePath] = &memFile{
		content: content,
		mode:    perm,
		modTime: time.Now(),
	}
	return nil
}

// Create creates a file for writing.
func (m *MemFS) Create(filePath string) (io.WriteCloser, error) {
	filePath = m.cleanPath(filePath)

	// Check parent exists
	dir := path.Dir(filePath)
	m.mu.RLock()
	if dir != "/" && !m.dirs[dir] {
		m.mu.RUnlock()
		return nil, &fs.PathError{Op: "create", Path: filePath, Err: fs.ErrNotExist}
	}
	if m.dirs[filePath] {
		m.mu.RUnlock()
		return nil, &fs.PathError{Op: "create", Path: filePath, Err: errIsDir}
	}
	m.mu.RUnlock()

	return &memWriter{fs: m, path: filePath}, nil
}

// Mkdir creates a directory.
func (m *MemFS) Mkdir(dirPath string, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dirPath = m.cleanPath(dirPath)

	// Check if already exists
	if m.dirs[dirPath] {
		return &fs.PathError{Op: "mkdir", Path: dirPath, Err: fs.ErrExist}
	}
	if _, ok := m.files[dirPath]; ok {
		return &fs.PathError{Op: "mkdir", Path: dirPath, Err: fs.ErrExist}
	}

	// Check parent exists
	parent := path.Dir(dirPath)
	if parent != "/" && !m.dirs[parent] {
		return &fs.PathError{Op: "mkdir", Path: dirPath, Err: fs.ErrNotExist}
	}

	m.dirs[dirPath] = true
	return nil
}

// MkdirAll creates a directory and all parent directories.
func (m *MemFS) MkdirAll(dirPath string, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dirPath = m.cleanPath(dirPath)

	// Check if it's a file
	if _, ok := m.files[dirPath]; ok {
		return &fs.PathError{Op: "mkdir", Path: dirPath, Err: errNotDir}
	}

	// Create all directories in path
	parts := strings.Split(strings.Trim(dirPath, "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current += "/" + part
		if _, ok := m.files[current]; ok {
			return &fs.PathError{Op: "mkdir", Path: current, Err: errNotDir}
		}
		m.dirs[current] = true
	}

	return nil
}

// Remove removes a file or empty directory.
func (m *MemFS) Remove(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath = m.cleanPath(filePath)

	// Check if it's a file
	if _, ok := m.files[filePath]; ok {
		delete(m.files, filePath)
		return nil
	}

	// Check if it's a directory
	if !m.dirs[filePath] {
		return &fs.PathError{Op: "remove", Path: filePath, Err: fs.ErrNotExist}
	}

	// Check if directory is empty
	prefix := filePath
	if prefix != "/" {
		prefix += "/"
	}
	for f := range m.files {
		if strings.HasPrefix(f, prefix) {
			return &fs.PathError{Op: "remove", Path: filePath, Err: errNotEmpty}
		}
	}
	for d := range m.dirs {
		if d != filePath && strings.HasPrefix(d, prefix) {
			return &fs.PathError{Op: "remove", Path: filePath, Err: errNotEmpty}
		}
	}

	delete(m.dirs, filePath)
	return nil
}

// RemoveAll removes a path and all its contents.
func (m *MemFS) RemoveAll(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath = m.cleanPath(filePath)

	// Remove files
	if _, ok := m.files[filePath]; ok {
		delete(m.files, filePath)
		return nil
	}

	// Remove directory and contents
	if !m.dirs[filePath] {
		return nil // RemoveAll succeeds if path doesn't exist
	}

	prefix := filePath
	if prefix != "/" {
		prefix += "/"
	}

	// Remove all files under this directory
	for f := range m.files {
		if strings.HasPrefix(f, prefix) {
			delete(m.files, f)
		}
	}

	// Remove all subdirectories
	for d := range m.dirs {
		if d == filePath || strings.HasPrefix(d, prefix) {
			delete(m.dirs, d)
		}
	}

	return nil
}

// Rename renames (moves) a file or directory.
func (m *MemFS) Rename(oldPath, newPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldPath = m.cleanPath(oldPath)
	newPath = m.cleanPath(newPath)

	// Check if old path exists
	f, isFile := m.files[oldPath]
	isDir := m.dirs[oldPath]
	if !isFile && !isDir {
		return &fs.PathError{Op: "rename", Path: oldPath, Err: fs.ErrNotExist}
	}

	// Check new parent exists
	newParent := path.Dir(newPath)
	if newParent != "/" && !m.dirs[newParent] {
		return &fs.PathError{Op: "rename", Path: newPath, Err: fs.ErrNotExist}
	}

	if isFile {
		m.files[newPath] = f
		delete(m.files, oldPath)
		return nil
	}

	// Rename directory and all contents
	oldPrefix := oldPath
	if oldPrefix != "/" {
		oldPrefix += "/"
	}
	newPrefix := newPath
	if newPrefix != "/" {
		newPrefix += "/"
	}

	// Rename files
	for filePath, content := range m.files {
		if strings.HasPrefix(filePath, oldPrefix) {
			relPath := strings.TrimPrefix(filePath, oldPrefix)
			newFilePath := path.Join(newPath, relPath)
			m.files[newFilePath] = content
			delete(m.files, filePath)
		}
	}

	// Rename directories
	for d := range m.dirs {
		if d == oldPath || strings.HasPrefix(d, oldPrefix) {
			relPath := strings.TrimPrefix(d, oldPath)
			var newDir string
			if relPath == "" {
				newDir = newPath
			} else {
				newDir = path.Join(newPath, relPath)
			}
			m.dirs[newDir] = true
			delete(m.dirs, d)
		}
	}

	return nil
}

// Abs returns the absolute path (already absolute in MemFS).
func (m *MemFS) Abs(filePath string) (string, error) {
	return m.cleanPath(filePath), nil
}

// Rel returns the relative path from base to target.
func (m *MemFS) Rel(basePath, targetPath string) (string, error) {
	basePath = m.cleanPath(basePath)
	targetPath = m.cleanPath(targetPath)

	if !strings.HasPrefix(targetPath, basePath) {
		return "", errNotUnder
	}

	rel := strings.TrimPrefix(targetPath, basePath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return ".", nil
	}
	return rel, nil
}

// Join joins path elements.
func (m *MemFS) Join(elem ...string) string {
	return path.Join(elem...)
}

// Dir returns the directory portion of a path.
func (m *MemFS) Dir(filePath string) string {
	return path.Dir(m.cleanPath(filePath))
}

// Base returns the last element of a path.
func (m *MemFS) Base(filePath string) string {
	return path.Base(filePath)
}

// Ext returns the file extension.
func (m *MemFS) Ext(filePath string) string {
	return path.Ext(filePath)
}

// Clean returns the cleaned path.
func (m *MemFS) Clean(filePath string) string {
	return m.cleanPath(filePath)
}

// Exists returns true if the path exists.
func (m *MemFS) Exists(filePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filePath = m.cleanPath(filePath)
	_, isFile := m.files[filePath]
	return isFile || m.dirs[filePath]
}

// IsDir returns true if the path is a directory.
func (m *MemFS) IsDir(filePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.dirs[m.cleanPath(filePath)]
}

// IsRegular returns true if the path is a regular file.
func (m *MemFS) IsRegular(filePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.files[m.cleanPath(filePath)]
	return ok
}

// Glob returns paths matching the pattern.
func (m *MemFS) Glob(pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []string

	// Check files
	for filePath := range m.files {
		matched, err := path.Match(pattern, filePath)
		if err != nil {
			return nil, err
		}
		if matched {
			matches = append(matches, filePath)
		}
	}

	// Check directories
	for dirPath := range m.dirs {
		matched, err := path.Match(pattern, dirPath)
		if err != nil {
			return nil, err
		}
		if matched {
			matches = append(matches, dirPath)
		}
	}

	sort.Strings(matches)
	return matches, nil
}

// Walk walks the file tree rooted at root.
func (m *MemFS) Walk(root string, fn WalkFunc) error {
	return m.WalkDir(root, func(filePath string, d DirEntry, err error) error {
		if err != nil {
			return fn(filePath, FileInfo{}, err)
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return fn(filePath, FileInfo{}, infoErr)
		}
		return fn(filePath, info, nil)
	})
}

// WalkDir walks the file tree rooted at root.
func (m *MemFS) WalkDir(root string, fn WalkDirFunc) error {
	root = m.cleanPath(root)

	// Get root info
	info, err := m.Stat(root)
	if err != nil {
		return fn(root, nil, err)
	}

	return m.walkDir(root, NewDirEntry(info), fn)
}

func (m *MemFS) walkDir(dirPath string, d DirEntry, fn WalkDirFunc) error {
	if err := fn(dirPath, d, nil); err != nil {
		if err == SkipDir && d.IsDir() {
			return nil
		}
		return err
	}

	if !d.IsDir() {
		return nil
	}

	entries, err := m.ReadDir(dirPath)
	if err != nil {
		return fn(dirPath, d, err)
	}

	for _, entry := range entries {
		entryPath := m.Join(dirPath, entry.Name())
		if err := m.walkDir(entryPath, NewDirEntry(entry), fn); err != nil {
			if err == SkipDir {
				continue
			}
			return err
		}
	}

	return nil
}

// cleanPath normalizes a path.
func (m *MemFS) cleanPath(p string) string {
	p = path.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// memWriter implements io.WriteCloser for MemFS.Create().
type memWriter struct {
	fs   *MemFS
	path string
	buf  bytes.Buffer
}

func (w *memWriter) Write(p []byte) (n int, err error) {
	return w.buf.Write(p)
}

func (w *memWriter) Close() error {
	return w.fs.WriteFile(w.path, w.buf.Bytes(), 0644)
}

// AddFile is a convenience method for adding files during setup.
func (m *MemFS) AddFile(filePath string, content string) error {
	// Ensure parent directories exist
	dir := path.Dir(m.cleanPath(filePath))
	if dir != "/" {
		if err := m.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return m.WriteFile(filePath, []byte(content), 0644)
}

// Files returns all file paths in the file system.
// Useful for testing and debugging.
func (m *MemFS) Files() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]string, 0, len(m.files))
	for f := range m.files {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// Dirs returns all directory paths in the file system.
// Useful for testing and debugging.
func (m *MemFS) Dirs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dirs := make([]string, 0, len(m.dirs))
	for d := range m.dirs {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}
