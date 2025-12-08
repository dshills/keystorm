// Package index provides fast file lookup and path indexing.
// It supports multiple match types including exact, prefix, suffix,
// contains, fuzzy, glob, and regex matching.
package index

import (
	"errors"
	"io"
	"os"
	"time"
)

// Common errors.
var (
	ErrNotFound       = errors.New("entry not found")
	ErrAlreadyExists  = errors.New("entry already exists")
	ErrInvalidQuery   = errors.New("invalid query")
	ErrIndexClosed    = errors.New("index is closed")
	ErrPatternTooLong = errors.New("regex pattern exceeds maximum length")
)

// Regex safety limits to prevent resource exhaustion.
const (
	// MaxRegexPatternLength is the maximum allowed length for a regex pattern.
	MaxRegexPatternLength = 1000
)

// FileInfo represents information about a file or directory.
type FileInfo struct {
	Path      string
	Name      string
	Size      int64
	ModTime   time.Time
	IsDir     bool
	IsSymlink bool
	Mode      os.FileMode
}

// Index provides fast file lookup.
type Index interface {
	// Add adds a file to the index.
	Add(path string, info FileInfo) error

	// Remove removes a file from the index.
	Remove(path string) error

	// Update updates an existing file in the index.
	Update(path string, info FileInfo) error

	// Get retrieves file info by path.
	Get(path string) (FileInfo, bool)

	// Has checks if a path exists in the index.
	Has(path string) bool

	// Count returns the number of indexed files.
	Count() int

	// All returns all indexed paths.
	All() []string

	// Query searches the index.
	Query(q Query) ([]Result, error)

	// Clear removes all entries.
	Clear()

	// Persistence
	Save(w io.Writer) error
	Load(r io.Reader) error

	// Close releases resources.
	Close() error
}

// Query defines search parameters.
type Query struct {
	// Pattern is the search pattern
	Pattern string

	// MatchType specifies how to match
	MatchType MatchType

	// FileTypes filters by extension (e.g., ".go", ".ts")
	FileTypes []string

	// MaxResults limits the number of results (0 = unlimited)
	MaxResults int

	// IncludeDirs includes directories in results
	IncludeDirs bool

	// CaseSensitive makes matching case-sensitive
	CaseSensitive bool

	// PathPrefix filters results to paths starting with this prefix
	PathPrefix string
}

// MatchType indicates how to match search patterns.
type MatchType int

const (
	// MatchExact matches the entire path or name exactly.
	MatchExact MatchType = iota

	// MatchPrefix matches paths/names starting with the pattern.
	MatchPrefix

	// MatchSuffix matches paths/names ending with the pattern.
	MatchSuffix

	// MatchContains matches paths/names containing the pattern.
	MatchContains

	// MatchFuzzy uses fuzzy matching (characters in order, not necessarily consecutive).
	MatchFuzzy

	// MatchGlob uses glob pattern matching (*, ?, []).
	MatchGlob

	// MatchRegex uses regular expression matching.
	MatchRegex
)

// String returns the string representation of the match type.
func (mt MatchType) String() string {
	switch mt {
	case MatchExact:
		return "exact"
	case MatchPrefix:
		return "prefix"
	case MatchSuffix:
		return "suffix"
	case MatchContains:
		return "contains"
	case MatchFuzzy:
		return "fuzzy"
	case MatchGlob:
		return "glob"
	case MatchRegex:
		return "regex"
	default:
		return "unknown"
	}
}

// Result represents a search match.
type Result struct {
	Path  string
	Info  FileInfo
	Score float64 // Match quality (for ranking)
}

// DefaultQuery returns a Query with default values.
func DefaultQuery(pattern string) Query {
	return Query{
		Pattern:       pattern,
		MatchType:     MatchFuzzy,
		MaxResults:    100,
		IncludeDirs:   false,
		CaseSensitive: false,
	}
}

// Option configures an Index.
type Option func(*Config)

// Config holds index configuration.
type Config struct {
	// InitialCapacity is the initial size hint for the index
	InitialCapacity int

	// CaseSensitive makes all operations case-sensitive
	CaseSensitive bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		InitialCapacity: 10000,
		CaseSensitive:   false,
	}
}

// WithInitialCapacity sets the initial capacity.
func WithInitialCapacity(capacity int) Option {
	return func(c *Config) {
		c.InitialCapacity = capacity
	}
}

// WithCaseSensitive sets case sensitivity.
func WithCaseSensitive(sensitive bool) Option {
	return func(c *Config) {
		c.CaseSensitive = sensitive
	}
}
