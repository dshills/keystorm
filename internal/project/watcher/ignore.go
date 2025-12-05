package watcher

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// IgnorePatterns manages gitignore-style file ignore rules.
// It supports patterns like:
//   - *.log       - match files ending in .log
//   - /build/     - match build directory at root
//   - **/node_modules/** - match node_modules anywhere
//   - !important.log - negate (don't ignore) important.log
type IgnorePatterns struct {
	mu       sync.RWMutex
	patterns []ignorePattern
}

// ignorePattern represents a single ignore pattern.
type ignorePattern struct {
	original string // Original pattern string
	pattern  string // Normalized pattern
	negation bool   // Pattern starts with !
	dirOnly  bool   // Pattern ends with /
	rooted   bool   // Pattern starts with /
}

// NewIgnorePatterns creates a new ignore pattern matcher.
func NewIgnorePatterns() *IgnorePatterns {
	return &IgnorePatterns{
		patterns: make([]ignorePattern, 0),
	}
}

// AddPattern adds an ignore pattern (gitignore syntax).
// Returns an error if the pattern is invalid.
func (ip *IgnorePatterns) AddPattern(pattern string) error {
	if pattern == "" || pattern == "#" {
		return nil // Skip empty or comment-only lines
	}

	// Skip comments
	if strings.HasPrefix(pattern, "#") {
		return nil
	}

	// Trim trailing spaces (unless escaped)
	pattern = strings.TrimRight(pattern, " \t")
	if pattern == "" {
		return nil
	}

	p := ignorePattern{
		original: pattern,
	}

	// Check for negation
	if strings.HasPrefix(pattern, "!") {
		p.negation = true
		pattern = pattern[1:]
	}

	// Check for directory-only
	if strings.HasSuffix(pattern, "/") {
		p.dirOnly = true
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Check for rooted pattern
	if strings.HasPrefix(pattern, "/") {
		p.rooted = true
		pattern = pattern[1:]
	}

	p.pattern = pattern

	ip.mu.Lock()
	ip.patterns = append(ip.patterns, p)
	ip.mu.Unlock()

	return nil
}

// AddPatterns adds multiple ignore patterns.
func (ip *IgnorePatterns) AddPatterns(patterns []string) error {
	for _, pattern := range patterns {
		if err := ip.AddPattern(pattern); err != nil {
			return err
		}
	}
	return nil
}

// AddFromFile loads patterns from a file (e.g., .gitignore).
// Each line is treated as a pattern.
func (ip *IgnorePatterns) AddFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if err := ip.AddPattern(line); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// Match returns true if the path should be ignored.
// basePath is used to make relative comparisons for rooted patterns.
func (ip *IgnorePatterns) Match(path string, isDir bool) bool {
	return ip.MatchRelative(path, "", isDir)
}

// MatchRelative checks if path should be ignored, relative to basePath.
func (ip *IgnorePatterns) MatchRelative(path, basePath string, isDir bool) bool {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	// Get relative path if basePath is provided
	relPath := path
	if basePath != "" {
		rel, err := filepath.Rel(basePath, path)
		if err == nil {
			relPath = rel
		}
	}

	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	// Check each pattern in order (later patterns can override earlier ones)
	ignored := false
	for _, p := range ip.patterns {
		if p.dirOnly && !isDir {
			continue // Pattern only applies to directories
		}

		matched := ip.matchPattern(p, relPath, isDir)
		if matched {
			ignored = !p.negation
		}
	}

	return ignored
}

// matchPattern checks if a path matches a single pattern.
func (ip *IgnorePatterns) matchPattern(p ignorePattern, relPath string, isDir bool) bool {
	pattern := p.pattern

	// Handle ** (match any path component)
	if strings.Contains(pattern, "**") {
		return ip.matchDoubleGlob(pattern, relPath)
	}

	// Handle rooted patterns - only match at root level
	if p.rooted {
		// For rooted patterns, check if the first path component matches
		// e.g., /build should only match "build" or "build/..." but not "src/build"
		parts := strings.Split(relPath, "/")
		if len(parts) > 0 {
			// Match pattern against first component or full path if pattern has /
			if strings.Contains(pattern, "/") {
				return ip.matchGlob(pattern, relPath)
			}
			return ip.matchGlob(pattern, parts[0])
		}
		return false
	}

	// Non-rooted patterns can match at any level
	// Try matching against full path
	if ip.matchGlob(pattern, relPath) {
		return true
	}

	// Try matching against just the filename
	base := filepath.Base(relPath)
	if !strings.Contains(pattern, "/") {
		return ip.matchGlob(pattern, base)
	}

	// Try matching against path suffixes
	parts := strings.Split(relPath, "/")
	for i := range parts {
		suffix := strings.Join(parts[i:], "/")
		if ip.matchGlob(pattern, suffix) {
			return true
		}
	}

	return false
}

// matchGlob matches a pattern against a path using glob syntax.
func (ip *IgnorePatterns) matchGlob(pattern, path string) bool {
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Try matching with basename for patterns without /
	if !strings.Contains(pattern, "/") {
		matched, _ = filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
	}

	return false
}

// matchDoubleGlob handles ** patterns that match any number of path components.
func (ip *IgnorePatterns) matchDoubleGlob(pattern, path string) bool {
	// Handle patterns like **/name/** which means "name" can appear anywhere
	// Also handle **/name/* or **/name (any path containing "name")

	pathParts := strings.Split(path, "/")

	// Pattern: **/middle/** or **/middle/*
	// This should match any path containing "middle" as a component
	if strings.HasPrefix(pattern, "**/") {
		// Remove leading **/
		rest := strings.TrimPrefix(pattern, "**/")

		// Check if rest ends with /** or /*
		if strings.HasSuffix(rest, "/**") {
			// Pattern like **/node_modules/**
			middle := strings.TrimSuffix(rest, "/**")
			// Check if middle appears as a component anywhere in path
			for _, part := range pathParts {
				if ip.matchGlob(middle, part) {
					return true
				}
			}
			return false
		}

		if strings.HasSuffix(rest, "/*") {
			// Pattern like **/test/*
			middle := strings.TrimSuffix(rest, "/*")
			// Check if middle appears as a non-final component
			for i, part := range pathParts {
				if i < len(pathParts)-1 && ip.matchGlob(middle, part) {
					return true
				}
			}
			return false
		}

		// Pattern like **/name - matches if name is anywhere in path
		// Try matching rest against path suffixes
		for i := range pathParts {
			candidate := strings.Join(pathParts[i:], "/")
			if ip.matchGlob(rest, candidate) {
				return true
			}
			// Also try matching just the component
			if !strings.Contains(rest, "/") && ip.matchGlob(rest, pathParts[i]) {
				return true
			}
		}
		return false
	}

	// Handle pattern like foo/**/bar
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		// More complex patterns, fall back to simple matching
		return ip.matchGlob(pattern, path)
	}

	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	// Check if path starts with prefix
	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}

	// Check if path ends with suffix or contains it
	if suffix != "" {
		if strings.HasSuffix(path, suffix) {
			return true
		}
		// Try matching suffix against path suffixes
		for i := range pathParts {
			candidate := strings.Join(pathParts[i:], "/")
			if ip.matchGlob(suffix, candidate) {
				return true
			}
		}
		return false
	}

	return true
}

// Clear removes all patterns.
func (ip *IgnorePatterns) Clear() {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	ip.patterns = ip.patterns[:0]
}

// Count returns the number of patterns.
func (ip *IgnorePatterns) Count() int {
	ip.mu.RLock()
	defer ip.mu.RUnlock()
	return len(ip.patterns)
}

// Patterns returns a copy of all patterns.
func (ip *IgnorePatterns) Patterns() []string {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	patterns := make([]string, len(ip.patterns))
	for i, p := range ip.patterns {
		patterns[i] = p.original
	}
	return patterns
}

// DefaultIgnorePatterns are common patterns to ignore in most projects.
var DefaultIgnorePatterns = []string{
	// Version control
	".git/",
	".svn/",
	".hg/",

	// Dependencies
	"node_modules/",
	"vendor/",
	".venv/",
	"venv/",
	"__pycache__/",
	"*.pyc",

	// Build outputs
	"dist/",
	"build/",
	"out/",
	"target/",
	"bin/",
	"obj/",

	// IDE/Editor
	".idea/",
	".vscode/",
	".vs/",
	"*.swp",
	"*.swo",
	"*~",

	// OS
	".DS_Store",
	"Thumbs.db",

	// Logs and temp
	"*.log",
	"tmp/",
	"temp/",
}

// NewDefaultIgnorePatterns creates an IgnorePatterns with default patterns.
func NewDefaultIgnorePatterns() *IgnorePatterns {
	ip := NewIgnorePatterns()
	_ = ip.AddPatterns(DefaultIgnorePatterns)
	return ip
}
