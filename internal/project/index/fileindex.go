package index

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// FileIndex implements Index using a map with path-based lookups.
// It provides fast exact lookups and supports fuzzy, glob, and regex matching.
type FileIndex struct {
	mu     sync.RWMutex
	config Config

	// Primary storage: path -> FileInfo
	entries map[string]FileInfo

	// Name index: lowercase name -> list of paths (for fast name-based lookups)
	nameIndex map[string][]string

	// Directory index: directory path -> list of child paths
	dirIndex map[string][]string

	closed bool
}

// NewFileIndex creates a new file index.
func NewFileIndex(opts ...Option) *FileIndex {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}

	return &FileIndex{
		config:    config,
		entries:   make(map[string]FileInfo, config.InitialCapacity),
		nameIndex: make(map[string][]string),
		dirIndex:  make(map[string][]string),
	}
}

// Add adds a file to the index.
func (fi *FileIndex) Add(path string, info FileInfo) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.closed {
		return ErrIndexClosed
	}

	// Normalize path
	path = filepath.Clean(path)

	if _, exists := fi.entries[path]; exists {
		return ErrAlreadyExists
	}

	// Store entry
	info.Path = path
	if info.Name == "" {
		info.Name = filepath.Base(path)
	}
	fi.entries[path] = info

	// Update name index
	nameLower := strings.ToLower(info.Name)
	fi.nameIndex[nameLower] = append(fi.nameIndex[nameLower], path)

	// Update directory index
	dir := filepath.Dir(path)
	fi.dirIndex[dir] = append(fi.dirIndex[dir], path)

	return nil
}

// Remove removes a file from the index.
func (fi *FileIndex) Remove(path string) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.closed {
		return ErrIndexClosed
	}

	path = filepath.Clean(path)

	info, exists := fi.entries[path]
	if !exists {
		return ErrNotFound
	}

	// Remove from entries
	delete(fi.entries, path)

	// Remove from name index
	nameLower := strings.ToLower(info.Name)
	fi.nameIndex[nameLower] = removeFromSlice(fi.nameIndex[nameLower], path)
	if len(fi.nameIndex[nameLower]) == 0 {
		delete(fi.nameIndex, nameLower)
	}

	// Remove from directory index
	dir := filepath.Dir(path)
	fi.dirIndex[dir] = removeFromSlice(fi.dirIndex[dir], path)
	if len(fi.dirIndex[dir]) == 0 {
		delete(fi.dirIndex, dir)
	}

	return nil
}

// Update updates an existing file in the index.
func (fi *FileIndex) Update(path string, info FileInfo) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.closed {
		return ErrIndexClosed
	}

	path = filepath.Clean(path)

	oldInfo, exists := fi.entries[path]
	if !exists {
		return ErrNotFound
	}

	// Update entry
	info.Path = path
	if info.Name == "" {
		info.Name = filepath.Base(path)
	}
	fi.entries[path] = info

	// If name changed, update name index
	oldNameLower := strings.ToLower(oldInfo.Name)
	newNameLower := strings.ToLower(info.Name)
	if oldNameLower != newNameLower {
		fi.nameIndex[oldNameLower] = removeFromSlice(fi.nameIndex[oldNameLower], path)
		if len(fi.nameIndex[oldNameLower]) == 0 {
			delete(fi.nameIndex, oldNameLower)
		}
		fi.nameIndex[newNameLower] = append(fi.nameIndex[newNameLower], path)
	}

	return nil
}

// Get retrieves file info by path.
func (fi *FileIndex) Get(path string) (FileInfo, bool) {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	path = filepath.Clean(path)
	info, ok := fi.entries[path]
	return info, ok
}

// Has checks if a path exists in the index.
func (fi *FileIndex) Has(path string) bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	path = filepath.Clean(path)
	_, ok := fi.entries[path]
	return ok
}

// Count returns the number of indexed files.
func (fi *FileIndex) Count() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	return len(fi.entries)
}

// All returns all indexed paths.
func (fi *FileIndex) All() []string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	paths := make([]string, 0, len(fi.entries))
	for path := range fi.entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// Query searches the index.
func (fi *FileIndex) Query(q Query) ([]Result, error) {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if fi.closed {
		return nil, ErrIndexClosed
	}

	if q.Pattern == "" && q.MatchType != MatchExact {
		// Empty pattern with non-exact match returns all
		return fi.allAsResults(q), nil
	}

	var results []Result

	var err error
	switch q.MatchType {
	case MatchExact:
		results = fi.queryExact(q)
	case MatchPrefix:
		results = fi.queryPrefix(q)
	case MatchSuffix:
		results = fi.querySuffix(q)
	case MatchContains:
		results = fi.queryContains(q)
	case MatchFuzzy:
		results = fi.queryFuzzy(q)
	case MatchGlob:
		results, err = fi.queryGlob(q)
		if err != nil {
			return nil, err
		}
	case MatchRegex:
		results, err = fi.queryRegex(q)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidQuery
	}

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply max results limit
	if q.MaxResults > 0 && len(results) > q.MaxResults {
		results = results[:q.MaxResults]
	}

	return results, nil
}

// Clear removes all entries.
func (fi *FileIndex) Clear() {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.entries = make(map[string]FileInfo, fi.config.InitialCapacity)
	fi.nameIndex = make(map[string][]string)
	fi.dirIndex = make(map[string][]string)
}

// Close releases resources.
func (fi *FileIndex) Close() error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.closed = true
	fi.entries = nil
	fi.nameIndex = nil
	fi.dirIndex = nil
	return nil
}

// GetByDirectory returns all files in a directory.
func (fi *FileIndex) GetByDirectory(dir string) []FileInfo {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	dir = filepath.Clean(dir)
	paths := fi.dirIndex[dir]
	infos := make([]FileInfo, 0, len(paths))
	for _, path := range paths {
		if info, ok := fi.entries[path]; ok {
			infos = append(infos, info)
		}
	}
	return infos
}

// GetByName returns all files with the given name.
func (fi *FileIndex) GetByName(name string) []FileInfo {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	nameLower := strings.ToLower(name)
	paths := fi.nameIndex[nameLower]
	infos := make([]FileInfo, 0, len(paths))
	for _, path := range paths {
		if info, ok := fi.entries[path]; ok {
			infos = append(infos, info)
		}
	}
	return infos
}

// Query implementations

func (fi *FileIndex) allAsResults(q Query) []Result {
	results := make([]Result, 0, len(fi.entries))
	for _, info := range fi.entries {
		if fi.matchesFilters(info, q) {
			results = append(results, Result{
				Path:  info.Path,
				Info:  info,
				Score: 1.0,
			})
		}
	}
	return results
}

func (fi *FileIndex) queryExact(q Query) []Result {
	var results []Result
	pattern := q.Pattern
	if !q.CaseSensitive {
		pattern = strings.ToLower(pattern)
	}

	for path, info := range fi.entries {
		if !fi.matchesFilters(info, q) {
			continue
		}

		target := info.Name
		if !q.CaseSensitive {
			target = strings.ToLower(target)
		}

		if target == pattern {
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: 1.0,
			})
		}
	}
	return results
}

func (fi *FileIndex) queryPrefix(q Query) []Result {
	var results []Result
	pattern := q.Pattern
	if !q.CaseSensitive {
		pattern = strings.ToLower(pattern)
	}

	for path, info := range fi.entries {
		if !fi.matchesFilters(info, q) {
			continue
		}

		target := info.Name
		if !q.CaseSensitive {
			target = strings.ToLower(target)
		}

		if strings.HasPrefix(target, pattern) {
			// Score based on how much of the name is matched
			score := float64(len(pattern)) / float64(len(target))
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: score,
			})
		}
	}
	return results
}

func (fi *FileIndex) querySuffix(q Query) []Result {
	var results []Result
	pattern := q.Pattern
	if !q.CaseSensitive {
		pattern = strings.ToLower(pattern)
	}

	for path, info := range fi.entries {
		if !fi.matchesFilters(info, q) {
			continue
		}

		target := info.Name
		if !q.CaseSensitive {
			target = strings.ToLower(target)
		}

		if strings.HasSuffix(target, pattern) {
			score := float64(len(pattern)) / float64(len(target))
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: score,
			})
		}
	}
	return results
}

func (fi *FileIndex) queryContains(q Query) []Result {
	var results []Result
	pattern := q.Pattern
	if !q.CaseSensitive {
		pattern = strings.ToLower(pattern)
	}

	for path, info := range fi.entries {
		if !fi.matchesFilters(info, q) {
			continue
		}

		target := info.Name
		if !q.CaseSensitive {
			target = strings.ToLower(target)
		}

		if strings.Contains(target, pattern) {
			// Score higher if pattern is at the start
			idx := strings.Index(target, pattern)
			positionBonus := 1.0 - (float64(idx) / float64(len(target)))
			score := (float64(len(pattern)) / float64(len(target))) * (0.5 + 0.5*positionBonus)
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: score,
			})
		}
	}
	return results
}

func (fi *FileIndex) queryFuzzy(q Query) []Result {
	var results []Result
	pattern := q.Pattern
	if !q.CaseSensitive {
		pattern = strings.ToLower(pattern)
	}

	for path, info := range fi.entries {
		if !fi.matchesFilters(info, q) {
			continue
		}

		target := info.Name
		targetPath := path
		if !q.CaseSensitive {
			target = strings.ToLower(target)
			targetPath = strings.ToLower(path)
		}

		// Try matching against name first (higher score)
		if score := fuzzyMatch(pattern, target); score > 0 {
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: score * 1.2, // Boost name matches
			})
			continue
		}

		// Try matching against full path
		if score := fuzzyMatch(pattern, targetPath); score > 0 {
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: score,
			})
		}
	}
	return results
}

func (fi *FileIndex) queryGlob(q Query) ([]Result, error) {
	var results []Result
	pattern := q.Pattern
	if !q.CaseSensitive {
		pattern = strings.ToLower(pattern)
	}

	// Validate pattern first
	if _, err := filepath.Match(pattern, ""); err != nil {
		return nil, ErrInvalidQuery
	}

	for path, info := range fi.entries {
		if !fi.matchesFilters(info, q) {
			continue
		}

		target := info.Name
		if !q.CaseSensitive {
			target = strings.ToLower(target)
		}

		matched, _ := filepath.Match(pattern, target)
		if matched {
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: 1.0,
			})
		}
	}
	return results, nil
}

func (fi *FileIndex) queryRegex(q Query) ([]Result, error) {
	flags := ""
	if !q.CaseSensitive {
		flags = "(?i)"
	}

	re, err := regexp.Compile(flags + q.Pattern)
	if err != nil {
		return nil, ErrInvalidQuery
	}

	var results []Result
	for path, info := range fi.entries {
		if !fi.matchesFilters(info, q) {
			continue
		}

		if re.MatchString(info.Name) {
			results = append(results, Result{
				Path:  path,
				Info:  info,
				Score: 1.0,
			})
		}
	}
	return results, nil
}

func (fi *FileIndex) matchesFilters(info FileInfo, q Query) bool {
	// Filter directories
	if info.IsDir && !q.IncludeDirs {
		return false
	}

	// Filter by path prefix
	if q.PathPrefix != "" && !strings.HasPrefix(info.Path, q.PathPrefix) {
		return false
	}

	// Filter by file types
	if len(q.FileTypes) > 0 && !info.IsDir {
		ext := filepath.Ext(info.Name)
		found := false
		for _, ft := range q.FileTypes {
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

// fuzzyMatch returns a score (0-1) for how well pattern matches target.
// 0 means no match, higher values mean better matches.
func fuzzyMatch(pattern, target string) float64 {
	if pattern == "" {
		return 1.0
	}
	if target == "" {
		return 0
	}

	pLen := len(pattern)
	tLen := len(target)

	// Quick check: pattern longer than target
	if pLen > tLen {
		return 0
	}

	// Exact match gets highest score
	if pattern == target {
		return 1.0
	}

	// Find all pattern characters in order
	pIdx := 0
	matches := make([]int, 0, pLen)

	for tIdx := 0; tIdx < tLen && pIdx < pLen; tIdx++ {
		if pattern[pIdx] == target[tIdx] {
			matches = append(matches, tIdx)
			pIdx++
		}
	}

	// All pattern characters must be found
	if pIdx != pLen {
		return 0
	}

	// Calculate score based on:
	// 1. Length of pattern vs target (longer pattern = higher score)
	// 2. Consecutive matches (bonus)
	// 3. Start of word matches (bonus)
	// 4. Position of first match (earlier = better)

	baseScore := float64(pLen) / float64(tLen) * 0.5

	// Consecutive character bonus
	consecutiveBonus := 0.0
	consecutiveCount := 0
	for i := 1; i < len(matches); i++ {
		if matches[i] == matches[i-1]+1 {
			consecutiveCount++
		}
	}
	if pLen > 1 {
		consecutiveBonus = float64(consecutiveCount) / float64(pLen-1) * 0.3
	}

	// Word boundary bonus
	wordBoundaryBonus := 0.0
	wordBoundaryCount := 0
	for _, idx := range matches {
		if idx == 0 || !isAlphaNum(target[idx-1]) {
			wordBoundaryCount++
		}
	}
	wordBoundaryBonus = float64(wordBoundaryCount) / float64(pLen) * 0.15

	// Position bonus (earlier matches are better)
	positionBonus := 0.0
	if len(matches) > 0 {
		positionBonus = (1.0 - float64(matches[0])/float64(tLen)) * 0.05
	}

	score := baseScore + consecutiveBonus + wordBoundaryBonus + positionBonus

	// Cap at just under 1.0 (exact match is 1.0)
	if score > 0.95 {
		score = 0.95
	}

	return score
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func removeFromSlice(slice []string, item string) []string {
	for i, s := range slice {
		if s == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// Ensure FileIndex implements Index.
var _ Index = (*FileIndex)(nil)
