package search

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Ranker provides result ranking and sorting functionality.
// It is safe for concurrent use.
type Ranker struct {
	mu sync.RWMutex

	// Weights for different ranking factors
	weights RankingWeights

	// Recency tracking
	modTimes map[string]time.Time

	// Frequency tracking
	openCounts map[string]int
}

// RankingWeights configures the relative importance of ranking factors.
type RankingWeights struct {
	// MatchScore weight for the base match score
	MatchScore float64

	// Recency weight for recently modified files
	Recency float64

	// Frequency weight for frequently opened files
	Frequency float64

	// PathDepth weight (shorter paths preferred)
	PathDepth float64

	// NameMatch weight (name matches preferred over path matches)
	NameMatch float64
}

// DefaultRankingWeights returns sensible default weights.
func DefaultRankingWeights() RankingWeights {
	return RankingWeights{
		MatchScore: 0.5,
		Recency:    0.2,
		Frequency:  0.15,
		PathDepth:  0.1,
		NameMatch:  0.05,
	}
}

// NewRanker creates a new result ranker.
func NewRanker(weights RankingWeights) *Ranker {
	return &Ranker{
		weights:    weights,
		modTimes:   make(map[string]time.Time),
		openCounts: make(map[string]int),
	}
}

// RankFileMatches ranks file search results by multiple factors.
func (r *Ranker) RankFileMatches(matches []FileMatch, opts FileSearchOptions) []FileMatch {
	if len(matches) == 0 {
		return matches
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Calculate composite scores
	for i := range matches {
		matches[i].Score = r.calculateFileScoreLocked(matches[i], opts)
	}

	// Sort by score
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

// RankContentMatches ranks content search results.
func (r *Ranker) RankContentMatches(matches []ContentMatch) []ContentMatch {
	if len(matches) == 0 {
		return matches
	}

	// Group matches by file for better ranking
	fileMatches := make(map[string][]ContentMatch)
	for _, m := range matches {
		fileMatches[m.Path] = append(fileMatches[m.Path], m)
	}

	// Calculate file-level scores
	fileScores := make(map[string]float64)
	for path, pathMatches := range fileMatches {
		// More matches in a file = higher score
		matchCount := float64(len(pathMatches))
		// Diminishing returns for many matches
		fileScores[path] = 1.0 + (matchCount-1.0)*0.1
	}

	// Sort by file score, then by line number within file
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return fileScores[matches[i].Path] > fileScores[matches[j].Path]
		}
		return matches[i].Line < matches[j].Line
	})

	return matches
}

// RecordModTime records the modification time of a file.
func (r *Ranker) RecordModTime(path string, modTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modTimes[path] = modTime
}

// RecordOpen records that a file was opened.
func (r *Ranker) RecordOpen(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.openCounts[path]++
}

// calculateFileScoreLocked calculates the composite score for a file match.
// Caller must hold r.mu.RLock().
func (r *Ranker) calculateFileScoreLocked(match FileMatch, opts FileSearchOptions) float64 {
	score := 0.0

	// Base match score (already 0-1)
	score += match.Score * r.weights.MatchScore

	// Recency bonus
	if opts.BoostRecent {
		score += r.recencyScore(match.Path) * r.weights.Recency
	}

	// Frequency bonus
	if opts.BoostFrequent {
		score += r.frequencyScore(match.Path) * r.weights.Frequency
	}

	// Path depth bonus (shorter paths preferred)
	score += r.pathDepthScore(match.Path) * r.weights.PathDepth

	// Name match bonus
	score += r.nameMatchScore(match) * r.weights.NameMatch

	return score
}

// recencyScore returns a 0-1 score based on file recency.
func (r *Ranker) recencyScore(path string) float64 {
	modTime, ok := r.modTimes[path]
	if !ok {
		return 0.5 // Default for unknown files
	}

	age := time.Since(modTime)

	// Files modified in the last hour get highest score
	if age < time.Hour {
		return 1.0
	}
	// Files modified today
	if age < 24*time.Hour {
		return 0.9
	}
	// Files modified this week
	if age < 7*24*time.Hour {
		return 0.7
	}
	// Files modified this month
	if age < 30*24*time.Hour {
		return 0.5
	}
	// Older files
	return 0.3
}

// frequencyScore returns a 0-1 score based on open frequency.
func (r *Ranker) frequencyScore(path string) float64 {
	count := r.openCounts[path]
	if count == 0 {
		return 0.0
	}

	// Use logarithmic scaling for frequency
	// 1 open = 0.3, 10 opens = 0.6, 100 opens = 0.9
	maxCount := 100.0
	normalizedCount := float64(count)
	if normalizedCount > maxCount {
		normalizedCount = maxCount
	}

	return 0.3 + 0.6*(normalizedCount/maxCount)
}

// pathDepthScore returns a 0-1 score based on path depth.
func (r *Ranker) pathDepthScore(path string) float64 {
	depth := strings.Count(path, string(filepath.Separator))

	// Normalize: depth 1 = 1.0, decreasing by 0.1 per level, min 0.3
	if depth <= 1 {
		return 1.0
	}

	score := 1.0 - float64(depth-1)*0.1
	if score < 0.3 {
		return 0.3
	}
	return score
}

// nameMatchScore returns a 0-1 score based on match location.
func (r *Ranker) nameMatchScore(match FileMatch) float64 {
	// If we have match positions, check if they're in the name portion
	if len(match.MatchPositions) == 0 {
		return 0.5
	}

	// Check if first match is early in the name
	if match.MatchPositions[0] == 0 {
		return 1.0 // Match at start of name
	}

	return 0.5
}

// GroupContentMatchesByFile groups content matches by file path.
func GroupContentMatchesByFile(matches []ContentMatch) map[string][]ContentMatch {
	result := make(map[string][]ContentMatch)
	for _, m := range matches {
		result[m.Path] = append(result[m.Path], m)
	}
	return result
}

// LimitResults limits results to the specified count.
func LimitResults[T any](results []T, limit int) []T {
	if limit <= 0 || len(results) <= limit {
		return results
	}
	return results[:limit]
}

// DeduplicateFileMatches removes duplicate file matches by path.
func DeduplicateFileMatches(matches []FileMatch) []FileMatch {
	seen := make(map[string]bool)
	result := make([]FileMatch, 0, len(matches))

	for _, m := range matches {
		if !seen[m.Path] {
			seen[m.Path] = true
			result = append(result, m)
		}
	}

	return result
}

// MergeFileMatches merges multiple file match slices, keeping highest scores.
func MergeFileMatches(matchSets ...[]FileMatch) []FileMatch {
	best := make(map[string]FileMatch)

	for _, matches := range matchSets {
		for _, m := range matches {
			if existing, ok := best[m.Path]; !ok || m.Score > existing.Score {
				best[m.Path] = m
			}
		}
	}

	result := make([]FileMatch, 0, len(best))
	for _, m := range best {
		result = append(result, m)
	}

	// Sort by score
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}
