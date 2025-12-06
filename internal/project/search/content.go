package search

import (
	"bufio"
	"bytes"
	"context"
	"io"
	stdpath "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/dshills/keystorm/internal/project/vfs"
)

// ContentSearch implements ContentSearcher for full-text content search.
type ContentSearch struct {
	mu  sync.RWMutex
	vfs vfs.VFS

	// Cached file contents for fast searching
	cache map[string][]byte
}

// NewContentSearch creates a new content searcher.
func NewContentSearch(fs vfs.VFS) *ContentSearch {
	return &ContentSearch{
		vfs:   fs,
		cache: make(map[string][]byte),
	}
}

// Search performs a content search across files.
func (cs *ContentSearch) Search(ctx context.Context, query string, opts ContentSearchOptions) ([]ContentMatch, error) {
	if query == "" {
		return nil, ErrInvalidQuery
	}

	// Compile query pattern
	re, err := CompileQuery(query, opts)
	if err != nil {
		return nil, err
	}

	// Take a snapshot of the cache to avoid holding lock during search
	snapshot := cs.snapshotCache()

	var results []ContentMatch

	// Search through cached files
	for path, content := range snapshot {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil, ErrSearchCanceled
		default:
		}

		// Apply file filters
		if !cs.matchesFilters(path, opts) {
			continue
		}

		// Check file size
		if opts.MaxFileSize > 0 && int64(len(content)) > opts.MaxFileSize {
			continue
		}

		// Search file content
		matches := cs.searchFile(path, content, re, opts)
		results = append(results, matches...)

		// Check max results
		if opts.MaxResults > 0 && len(results) >= opts.MaxResults {
			results = results[:opts.MaxResults]
			break
		}
	}

	return results, nil
}

// snapshotCache returns a shallow copy of the cache for lock-free iteration.
func (cs *ContentSearch) snapshotCache() map[string][]byte {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	snapshot := make(map[string][]byte, len(cs.cache))
	for k, v := range cs.cache {
		snapshot[k] = v
	}
	return snapshot
}

// SearchFiles searches specific files for content matches.
func (cs *ContentSearch) SearchFiles(ctx context.Context, paths []string, query string, opts ContentSearchOptions) ([]ContentMatch, error) {
	if query == "" {
		return nil, ErrInvalidQuery
	}

	// Compile query pattern
	re, err := CompileQuery(query, opts)
	if err != nil {
		return nil, err
	}

	var results []ContentMatch

	for _, path := range paths {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil, ErrSearchCanceled
		default:
		}

		// Read file content
		content, err := cs.vfs.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		// Check file size
		if opts.MaxFileSize > 0 && int64(len(content)) > opts.MaxFileSize {
			continue
		}

		// Search file content
		matches := cs.searchFile(path, content, re, opts)
		results = append(results, matches...)

		// Check max results
		if opts.MaxResults > 0 && len(results) >= opts.MaxResults {
			results = results[:opts.MaxResults]
			break
		}
	}

	return results, nil
}

// SearchReader searches content from a reader.
func (cs *ContentSearch) SearchReader(ctx context.Context, path string, r io.Reader, query string, opts ContentSearchOptions) ([]ContentMatch, error) {
	if query == "" {
		return nil, ErrInvalidQuery
	}

	// Compile query pattern
	re, err := CompileQuery(query, opts)
	if err != nil {
		return nil, err
	}

	// Limit read size if MaxFileSize is set
	var content []byte
	if opts.MaxFileSize > 0 {
		limitedReader := io.LimitReader(r, opts.MaxFileSize+1)
		content, err = io.ReadAll(limitedReader)
		if err != nil {
			return nil, err
		}
		// If we read more than MaxFileSize, the file is too large
		if int64(len(content)) > opts.MaxFileSize {
			return nil, ErrFileTooLarge
		}
	} else {
		content, err = io.ReadAll(r)
		if err != nil {
			return nil, err
		}
	}

	return cs.searchFile(path, content, re, opts), nil
}

// IndexFile indexes a file's content for searching.
func (cs *ContentSearch) IndexFile(path string, content []byte) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.cache[path] = content
	return nil
}

// RemoveFile removes a file from the index.
func (cs *ContentSearch) RemoveFile(path string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.cache, path)
	return nil
}

// Clear removes all indexed content.
func (cs *ContentSearch) Clear() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.cache = make(map[string][]byte)
}

// IndexedCount returns the number of indexed files.
func (cs *ContentSearch) IndexedCount() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return len(cs.cache)
}

// searchFile searches for matches within a single file.
func (cs *ContentSearch) searchFile(path string, content []byte, re *regexp.Regexp, opts ContentSearchOptions) []ContentMatch {
	var matches []ContentMatch

	// Split into lines
	lines := splitLines(content)

	for lineNum, line := range lines {
		// Find all matches on this line
		allMatches := re.FindAllStringIndex(line, -1)
		if len(allMatches) == 0 {
			continue
		}

		// Create highlights from matches
		highlights := make([]Range, len(allMatches))
		for i, m := range allMatches {
			highlights[i] = Range{Start: m[0], End: m[1]}
		}

		// Get context lines
		contextBefore := getContextBefore(lines, lineNum, opts.ContextLines)
		contextAfter := getContextAfter(lines, lineNum, opts.ContextLines)

		matches = append(matches, ContentMatch{
			Path:          path,
			Line:          lineNum + 1, // 1-based
			Column:        allMatches[0][0] + 1,
			Text:          line,
			ContextBefore: contextBefore,
			ContextAfter:  contextAfter,
			Highlights:    highlights,
		})
	}

	return matches
}

// matchesFilters checks if a file path matches the search filters.
func (cs *ContentSearch) matchesFilters(path string, opts ContentSearchOptions) bool {
	// Check include patterns
	if len(opts.IncludePaths) > 0 {
		matched := false
		for _, pattern := range opts.IncludePaths {
			if matchGlob(pattern, path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range opts.ExcludePaths {
		if matchGlob(pattern, path) {
			return false
		}
	}

	// Check file types
	if len(opts.FileTypes) > 0 {
		ext := filepath.Ext(path)
		found := false
		for _, ft := range opts.FileTypes {
			if strings.EqualFold(ext, ft) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// splitLines splits content into lines, preserving empty lines.
// Returns empty slice (not nil) for empty content.
func splitLines(content []byte) []string {
	if len(content) == 0 {
		return []string{}
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	// Increase buffer size for long lines (1MB max per line)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// If scanner error occurred (e.g., line too long), return what we have
	// The caller can still use partial results
	if len(lines) == 0 {
		return []string{}
	}
	return lines
}

// getContextBefore returns lines before the match.
func getContextBefore(lines []string, lineNum, count int) []string {
	if count <= 0 {
		return nil
	}

	start := lineNum - count
	if start < 0 {
		start = 0
	}

	if start >= lineNum {
		return nil
	}

	return lines[start:lineNum]
}

// getContextAfter returns lines after the match.
func getContextAfter(lines []string, lineNum, count int) []string {
	if count <= 0 {
		return nil
	}

	end := lineNum + 1 + count
	if end > len(lines) {
		end = len(lines)
	}

	if lineNum+1 >= end {
		return nil
	}

	return lines[lineNum+1 : end]
}

// matchGlob matches a path against a glob pattern.
// Handles cross-platform paths by normalizing to forward slashes.
func matchGlob(pattern, filePath string) bool {
	// Normalize to forward slashes for cross-platform compatibility
	pattern = filepath.ToSlash(pattern)
	filePath = filepath.ToSlash(filePath)

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		// Split by ** to get parts
		parts := strings.Split(pattern, "**")

		// Pattern like "**/vendor/**" splits to ["", "/vendor/", ""]
		// Pattern like "**/vendor" splits to ["", "/vendor"]
		// Pattern like "src/**" splits to ["src/", ""]

		if len(parts) == 3 && parts[0] == "" && parts[2] == "" {
			// Pattern like "**/vendor/**" - check if middle part is in path
			middle := parts[1]
			// middle is "/vendor/" - check if path contains this segment
			return strings.Contains(filePath, middle)
		}

		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			// Pattern like "**/suffix" - ends with suffix
			if prefix == "" && suffix != "" {
				return strings.HasSuffix(filePath, suffix) || strings.Contains(filePath, "/"+suffix+"/")
			}
			// Pattern like "prefix/**" - starts with prefix
			if suffix == "" && prefix != "" {
				return strings.HasPrefix(filePath, prefix) || strings.HasPrefix(filePath, "/"+prefix)
			}
			// Pattern like "prefix/**/suffix"
			if prefix != "" && suffix != "" {
				hasPrefix := strings.HasPrefix(filePath, prefix) || strings.HasPrefix(filePath, "/"+prefix)
				hasSuffix := strings.HasSuffix(filePath, suffix)
				return hasPrefix && hasSuffix
			}
			return true
		}
	}

	// Standard glob matching - use stdpath.Match for slash-normalized paths
	// (stdpath.Match always uses '/' as separator, unlike filepath.Match)
	// Match against base name first for patterns like "*.go"
	baseName := filePath[strings.LastIndex(filePath, "/")+1:]
	if matched, _ := stdpath.Match(pattern, baseName); matched {
		return true
	}
	// Also try matching the full path for patterns like "src/*.go"
	matched, _ := stdpath.Match(pattern, filePath)
	return matched
}

// Ensure ContentSearch implements ContentSearcher.
var _ ContentSearcher = (*ContentSearch)(nil)
