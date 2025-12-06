package search

import (
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dshills/keystorm/internal/project/index"
)

// FuzzySearcher implements FileSearcher using fuzzy matching.
// Use Ranker for frequency boosting and result ranking.
type FuzzySearcher struct {
	mu    sync.RWMutex
	index index.Index
}

// NewFuzzySearcher creates a new fuzzy file searcher.
func NewFuzzySearcher(idx index.Index) *FuzzySearcher {
	return &FuzzySearcher{
		index: idx,
	}
}

// Search finds files matching the query using fuzzy matching.
func (fs *FuzzySearcher) Search(ctx context.Context, query string, opts FileSearchOptions) ([]FileMatch, error) {
	if query == "" {
		return nil, nil
	}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Get all files from the index
	paths := fs.index.All()

	var results []FileMatch

	for _, path := range paths {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil, ErrSearchCanceled
		default:
		}

		info, ok := fs.index.Get(path)
		if !ok {
			continue
		}

		// Apply filters
		if !fs.matchesFilters(info, opts) {
			continue
		}

		// Calculate match score
		var score float64
		var positions []int

		switch opts.MatchMode {
		case MatchFuzzy:
			score, positions = fuzzyMatchWithPositions(query, info.Name, opts.CaseSensitive)
			if score == 0 {
				// Try matching against path
				pathScore, pathPositions := fuzzyMatchWithPositions(query, path, opts.CaseSensitive)
				if pathScore > 0 {
					score = pathScore * 0.8 // Discount path matches
					positions = pathPositions
				}
			}
		case MatchExact:
			if fs.exactMatch(query, info.Name, opts.CaseSensitive) {
				score = 1.0
			}
		case MatchPrefix:
			if fs.prefixMatch(query, info.Name, opts.CaseSensitive) {
				score = float64(len(query)) / float64(len(info.Name))
			}
		case MatchContains:
			if fs.containsMatch(query, info.Name, opts.CaseSensitive) {
				score = float64(len(query)) / float64(len(info.Name))
			}
		case MatchGlob:
			matched, _ := filepath.Match(query, info.Name)
			if matched {
				score = 1.0
			}
		case MatchRegex:
			pattern := query
			if !opts.CaseSensitive {
				pattern = "(?i)" + pattern
			}
			if re, err := regexp.Compile(pattern); err == nil {
				if re.MatchString(info.Name) {
					score = 1.0
				} else if re.MatchString(path) {
					score = 0.8 // Discount path matches
				}
			}
		}

		if score > 0 {
			results = append(results, FileMatch{
				Path:           path,
				Name:           info.Name,
				Score:          score,
				IsDir:          info.IsDir,
				MatchPositions: positions,
			})
		}
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply max results limit
	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results, nil
}

// matchesFilters checks if a file matches the search filters.
func (fs *FuzzySearcher) matchesFilters(info index.FileInfo, opts FileSearchOptions) bool {
	// Filter directories
	if info.IsDir && !opts.IncludeDirs {
		return false
	}

	// Filter by path prefix
	if opts.PathPrefix != "" && !strings.HasPrefix(info.Path, opts.PathPrefix) {
		return false
	}

	// Filter by file types
	if len(opts.FileTypes) > 0 && !info.IsDir {
		ext := filepath.Ext(info.Name)
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

func (fs *FuzzySearcher) exactMatch(query, target string, caseSensitive bool) bool {
	if caseSensitive {
		return query == target
	}
	return strings.EqualFold(query, target)
}

func (fs *FuzzySearcher) prefixMatch(query, target string, caseSensitive bool) bool {
	if caseSensitive {
		return strings.HasPrefix(target, query)
	}
	return strings.HasPrefix(strings.ToLower(target), strings.ToLower(query))
}

func (fs *FuzzySearcher) containsMatch(query, target string, caseSensitive bool) bool {
	if caseSensitive {
		return strings.Contains(target, query)
	}
	return strings.Contains(strings.ToLower(target), strings.ToLower(query))
}

// fuzzyMatchWithPositions returns the match score and positions of matched characters.
func fuzzyMatchWithPositions(pattern, target string, caseSensitive bool) (float64, []int) {
	if pattern == "" {
		return 1.0, nil
	}
	if target == "" {
		return 0, nil
	}

	// Normalize case if not case sensitive
	p := pattern
	t := target
	if !caseSensitive {
		p = strings.ToLower(pattern)
		t = strings.ToLower(target)
	}

	pLen := len(p)
	tLen := len(t)

	// Quick check: pattern longer than target
	if pLen > tLen {
		return 0, nil
	}

	// Exact match gets highest score
	if p == t {
		positions := make([]int, pLen)
		for i := range positions {
			positions[i] = i
		}
		return 1.0, positions
	}

	// Find all pattern characters in order
	pIdx := 0
	matches := make([]int, 0, pLen)

	for tIdx := 0; tIdx < tLen && pIdx < pLen; tIdx++ {
		if p[pIdx] == t[tIdx] {
			matches = append(matches, tIdx)
			pIdx++
		}
	}

	// All pattern characters must be found
	if pIdx != pLen {
		return 0, nil
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
		if idx == 0 || !isAlphaNum(t[idx-1]) {
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

	return score, matches
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// Ensure FuzzySearcher implements FileSearcher.
var _ FileSearcher = (*FuzzySearcher)(nil)
