// Package loader provides configuration file loading for Keystorm.
//
// The loader package handles parsing configuration files in various formats
// (TOML, JSON) and loading environment variables into configuration maps.
package loader

import (
	"io"
	"io/fs"
	"os"
)

// Loader is the interface for configuration loaders.
type Loader interface {
	// Load reads configuration from the source and returns a map.
	// Returns nil, nil if the source doesn't exist (not an error).
	Load() (map[string]any, error)
}

// FileLoader is the interface for loaders that read from files.
type FileLoader interface {
	Loader
	// LoadFrom reads configuration from a specific path.
	LoadFrom(path string) (map[string]any, error)
}

// ReaderLoader is the interface for loaders that read from io.Reader.
type ReaderLoader interface {
	// LoadFromReader reads configuration from a reader.
	LoadFromReader(r io.Reader) (map[string]any, error)
}

// FileSystem is an abstraction for file system operations.
// This allows for easy testing with in-memory file systems.
type FileSystem interface {
	fs.FS
	// ReadFile reads the entire file at path.
	ReadFile(path string) ([]byte, error)
	// Stat returns file info for path.
	Stat(path string) (fs.FileInfo, error)
}

// OSFS implements FileSystem using the real OS file system.
type OSFS struct{}

// Open implements fs.FS.
func (OSFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

// ReadFile reads the entire file at path.
func (OSFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Stat returns file info for path.
func (OSFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// DefaultFS returns the default file system (OS).
func DefaultFS() FileSystem {
	return OSFS{}
}
