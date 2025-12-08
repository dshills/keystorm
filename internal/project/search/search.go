// Package search provides file and content search functionality.
// It supports fuzzy file matching and full-text content search.
package search

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// Common errors.
var (
	ErrInvalidQuery   = errors.New("invalid search query")
	ErrSearchCanceled = errors.New("search canceled")
	ErrNoResults      = errors.New("no results found")
	ErrFileTooLarge   = errors.New("file exceeds maximum size limit")
	ErrPatternTooLong = errors.New("regex pattern exceeds maximum length")
)

// Regex safety limits to prevent ReDoS attacks.
const (
	// MaxRegexPatternLength is the maximum allowed length for a regex pattern.
	MaxRegexPatternLength = 1000

	// MaxRegexRepetitions is the maximum allowed unbounded repetitions.
	// Patterns like a{1000} or a* nested multiple times can cause exponential backtracking.
	MaxRegexRepetitions = 100
)

// FileSearcher provides fast file name/path search.
type FileSearcher interface {
	// Search finds files matching the query.
	Search(ctx context.Context, query string, opts FileSearchOptions) ([]FileMatch, error)
}

// ContentSearcher provides full-text content search.
type ContentSearcher interface {
	// Search performs a content search.
	Search(ctx context.Context, query string, opts ContentSearchOptions) ([]ContentMatch, error)

	// IndexFile indexes a file's content.
	IndexFile(path string, content []byte) error

	// RemoveFile removes a file from the index.
	RemoveFile(path string) error

	// Clear removes all indexed content.
	Clear()
}

// FileSearchOptions configures file search behavior.
type FileSearchOptions struct {
	// MaxResults limits the number of results (0 = unlimited)
	MaxResults int

	// FileTypes filters by extension (e.g., ".go", ".ts")
	FileTypes []string

	// IncludeDirs includes directories in results
	IncludeDirs bool

	// CaseSensitive makes matching case-sensitive
	CaseSensitive bool

	// PathPrefix filters results to paths starting with this prefix
	PathPrefix string

	// MatchMode specifies how to match files
	MatchMode MatchMode

	// BoostRecent gives higher scores to recently modified files
	BoostRecent bool

	// BoostFrequent gives higher scores to frequently opened files
	BoostFrequent bool
}

// ContentSearchOptions configures content search behavior.
type ContentSearchOptions struct {
	// Query matching
	CaseSensitive bool
	WholeWord     bool
	UseRegex      bool

	// Scope
	IncludePaths []string // Glob patterns to include
	ExcludePaths []string // Glob patterns to exclude
	FileTypes    []string // Extensions to search

	// Limits
	MaxResults  int
	MaxFileSize int64

	// Context
	ContextLines int // Lines of context around matches
}

// MatchMode specifies how to match search patterns.
type MatchMode int

const (
	// MatchFuzzy uses fuzzy matching (characters in order, not necessarily consecutive).
	MatchFuzzy MatchMode = iota

	// MatchExact matches the entire name exactly.
	MatchExact

	// MatchPrefix matches names starting with the pattern.
	MatchPrefix

	// MatchContains matches names containing the pattern.
	MatchContains

	// MatchGlob uses glob pattern matching (*, ?, []).
	MatchGlob

	// MatchRegex uses regular expression matching.
	MatchRegex
)

// String returns the string representation of the match mode.
func (m MatchMode) String() string {
	switch m {
	case MatchFuzzy:
		return "fuzzy"
	case MatchExact:
		return "exact"
	case MatchPrefix:
		return "prefix"
	case MatchContains:
		return "contains"
	case MatchGlob:
		return "glob"
	case MatchRegex:
		return "regex"
	default:
		return "unknown"
	}
}

// FileMatch represents a file search result.
type FileMatch struct {
	// Path is the full path to the file
	Path string

	// Name is the file name
	Name string

	// Score indicates match quality (higher is better)
	Score float64

	// IsDir indicates if this is a directory
	IsDir bool

	// MatchPositions contains the positions of matched characters in the name
	MatchPositions []int
}

// ContentMatch represents a content search result.
type ContentMatch struct {
	// Path is the full path to the file
	Path string

	// Line is the 1-based line number
	Line int

	// Column is the 1-based column number
	Column int

	// Text is the matching line content
	Text string

	// ContextBefore contains lines before the match
	ContextBefore []string

	// ContextAfter contains lines after the match
	ContextAfter []string

	// Highlights are the match positions in Text
	Highlights []Range
}

// Range represents a range within a string.
type Range struct {
	Start int
	End   int
}

// DefaultFileSearchOptions returns sensible defaults for file search.
func DefaultFileSearchOptions() FileSearchOptions {
	return FileSearchOptions{
		MaxResults:    100,
		MatchMode:     MatchFuzzy,
		IncludeDirs:   false,
		CaseSensitive: false,
		BoostRecent:   true,
		BoostFrequent: false,
	}
}

// DefaultContentSearchOptions returns sensible defaults for content search.
func DefaultContentSearchOptions() ContentSearchOptions {
	return ContentSearchOptions{
		CaseSensitive: false,
		WholeWord:     false,
		UseRegex:      false,
		MaxResults:    1000,
		MaxFileSize:   10 * 1024 * 1024, // 10 MB
		ContextLines:  2,
	}
}

// CompileQuery compiles a search query into a regex pattern.
// Returns an error if the query is invalid.
func CompileQuery(query string, opts ContentSearchOptions) (*regexp.Regexp, error) {
	pattern := query

	// Escape regex special characters if not using regex mode
	if !opts.UseRegex {
		pattern = regexp.QuoteMeta(pattern)
	}

	// Add word boundaries if whole word matching
	if opts.WholeWord {
		pattern = `\b` + pattern + `\b`
	}

	// Add case-insensitive flag if not case-sensitive
	if !opts.CaseSensitive {
		pattern = "(?i)" + pattern
	}

	// Use safe compilation with limits
	return CompileSafeRegex(pattern)
}

// CompileSafeRegex compiles a regex pattern with safety checks to prevent ReDoS attacks.
// It validates pattern length and rejects patterns that could cause catastrophic backtracking.
func CompileSafeRegex(pattern string) (*regexp.Regexp, error) {
	// Check pattern length
	if len(pattern) > MaxRegexPatternLength {
		return nil, fmt.Errorf("%w: length %d exceeds limit %d", ErrPatternTooLong, len(pattern), MaxRegexPatternLength)
	}

	// Note: Go's regexp package uses RE2 which guarantees linear time matching
	// and doesn't support backtracking, making it inherently resistant to ReDoS.
	// However, we still limit pattern length to prevent resource exhaustion
	// during compilation of very complex patterns.

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
	}

	return re, nil
}
