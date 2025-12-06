package search

import (
	"testing"
	"time"
)

func TestDefaultRankingWeights(t *testing.T) {
	weights := DefaultRankingWeights()

	if weights.MatchScore != 0.5 {
		t.Errorf("MatchScore = %f, want 0.5", weights.MatchScore)
	}
	if weights.Recency != 0.2 {
		t.Errorf("Recency = %f, want 0.2", weights.Recency)
	}
	if weights.Frequency != 0.15 {
		t.Errorf("Frequency = %f, want 0.15", weights.Frequency)
	}
	if weights.PathDepth != 0.1 {
		t.Errorf("PathDepth = %f, want 0.1", weights.PathDepth)
	}
	if weights.NameMatch != 0.05 {
		t.Errorf("NameMatch = %f, want 0.05", weights.NameMatch)
	}
}

func TestRanker_RankFileMatches_Empty(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())
	opts := DefaultFileSearchOptions()

	results := ranker.RankFileMatches(nil, opts)
	if results != nil {
		t.Error("Expected nil for empty input")
	}

	results = ranker.RankFileMatches([]FileMatch{}, opts)
	if len(results) != 0 {
		t.Error("Expected empty slice for empty input")
	}
}

func TestRanker_RankFileMatches_SortsByScore(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())
	opts := DefaultFileSearchOptions()
	opts.BoostRecent = false
	opts.BoostFrequent = false

	matches := []FileMatch{
		{Path: "/a/b/c/file1.go", Name: "file1.go", Score: 0.5},
		{Path: "/a/b/c/file2.go", Name: "file2.go", Score: 0.9},
		{Path: "/a/b/c/file3.go", Name: "file3.go", Score: 0.7},
	}

	ranked := ranker.RankFileMatches(matches, opts)

	// Should be sorted by score descending
	for i := 1; i < len(ranked); i++ {
		if ranked[i].Score > ranked[i-1].Score {
			t.Errorf("Results not sorted: %f > %f", ranked[i].Score, ranked[i-1].Score)
		}
	}
}

func TestRanker_RankFileMatches_RecencyBoost(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())
	opts := DefaultFileSearchOptions()
	opts.BoostRecent = true
	opts.BoostFrequent = false

	// Record mod times
	ranker.RecordModTime("/recent.go", time.Now().Add(-1*time.Minute))
	ranker.RecordModTime("/old.go", time.Now().Add(-365*24*time.Hour))

	matches := []FileMatch{
		{Path: "/old.go", Name: "old.go", Score: 0.9},
		{Path: "/recent.go", Name: "recent.go", Score: 0.7},
	}

	ranked := ranker.RankFileMatches(matches, opts)

	// Recent file should be boosted
	if ranked[0].Path != "/recent.go" {
		t.Errorf("Expected recent file first, got %s", ranked[0].Path)
	}
}

func TestRanker_RankFileMatches_FrequencyBoost(t *testing.T) {
	// Use weights that heavily favor frequency
	weights := RankingWeights{
		MatchScore: 0.3,
		Recency:    0.0,
		Frequency:  0.6, // High frequency weight
		PathDepth:  0.05,
		NameMatch:  0.05,
	}
	ranker := NewRanker(weights)
	opts := DefaultFileSearchOptions()
	opts.BoostRecent = false
	opts.BoostFrequent = true

	// Record opens
	for i := 0; i < 100; i++ {
		ranker.RecordOpen("/frequent.go")
	}
	ranker.RecordOpen("/rare.go")

	matches := []FileMatch{
		{Path: "/rare.go", Name: "rare.go", Score: 0.8},
		{Path: "/frequent.go", Name: "frequent.go", Score: 0.7},
	}

	ranked := ranker.RankFileMatches(matches, opts)

	// Frequent file should be boosted
	if ranked[0].Path != "/frequent.go" {
		t.Errorf("Expected frequent file first, got %s", ranked[0].Path)
	}
}

func TestRanker_RankContentMatches_Empty(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())

	results := ranker.RankContentMatches(nil)
	if results != nil {
		t.Error("Expected nil for empty input")
	}

	results = ranker.RankContentMatches([]ContentMatch{})
	if len(results) != 0 {
		t.Error("Expected empty slice for empty input")
	}
}

func TestRanker_RankContentMatches_GroupsByFile(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())

	matches := []ContentMatch{
		{Path: "/file1.go", Line: 10, Text: "match1"},
		{Path: "/file2.go", Line: 5, Text: "match2"},
		{Path: "/file1.go", Line: 20, Text: "match3"},
		{Path: "/file1.go", Line: 30, Text: "match4"},
	}

	ranked := ranker.RankContentMatches(matches)

	// file1.go has more matches, should come first
	if ranked[0].Path != "/file1.go" {
		t.Errorf("Expected file1.go first, got %s", ranked[0].Path)
	}

	// Matches within same file should be sorted by line
	lastLine := 0
	lastPath := ""
	for _, m := range ranked {
		if m.Path == lastPath && m.Line < lastLine {
			t.Errorf("Matches not sorted by line within file: %d < %d", m.Line, lastLine)
		}
		if m.Path != lastPath {
			lastLine = 0
		}
		lastLine = m.Line
		lastPath = m.Path
	}
}

func TestRanker_recencyScore(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())

	tests := []struct {
		age  time.Duration
		want float64
	}{
		{30 * time.Minute, 1.0},     // Modified in last hour
		{12 * time.Hour, 0.9},       // Modified today
		{3 * 24 * time.Hour, 0.7},   // Modified this week
		{15 * 24 * time.Hour, 0.5},  // Modified this month
		{100 * 24 * time.Hour, 0.3}, // Modified long ago
	}

	for _, tt := range tests {
		ranker.modTimes["/test.go"] = time.Now().Add(-tt.age)
		score := ranker.recencyScore("/test.go")
		if score != tt.want {
			t.Errorf("recencyScore(age=%v) = %f, want %f", tt.age, score, tt.want)
		}
	}
}

func TestRanker_recencyScore_Unknown(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())

	score := ranker.recencyScore("/unknown.go")
	if score != 0.5 {
		t.Errorf("recencyScore for unknown file = %f, want 0.5", score)
	}
}

func TestRanker_frequencyScore(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())

	tests := []struct {
		count int
		want  float64
	}{
		{0, 0.0},
		{1, 0.306}, // ~0.3 + 0.6 * 1/100
		{50, 0.6},  // 0.3 + 0.6 * 50/100
		{100, 0.9}, // 0.3 + 0.6 * 100/100
		{200, 0.9}, // Capped at 100
	}

	for _, tt := range tests {
		ranker.openCounts["/test.go"] = tt.count
		score := ranker.frequencyScore("/test.go")
		// Allow small floating point differences
		diff := score - tt.want
		if diff < -0.01 || diff > 0.01 {
			t.Errorf("frequencyScore(count=%d) = %f, want ~%f", tt.count, score, tt.want)
		}
	}
}

func TestRanker_pathDepthScore(t *testing.T) {
	ranker := NewRanker(DefaultRankingWeights())

	tests := []struct {
		path string
		want float64
	}{
		{"/a.go", 1.0},                 // depth 1
		{"/a/b.go", 0.9},               // depth 2
		{"/a/b/c.go", 0.8},             // depth 3
		{"/a/b/c/d.go", 0.7},           // depth 4
		{"/a/b/c/d/e/f/g/h/i.go", 0.3}, // depth 9
	}

	for _, tt := range tests {
		score := ranker.pathDepthScore(tt.path)
		if score != tt.want {
			t.Errorf("pathDepthScore(%q) = %f, want %f", tt.path, score, tt.want)
		}
	}
}

func TestGroupContentMatchesByFile(t *testing.T) {
	matches := []ContentMatch{
		{Path: "/file1.go", Line: 10},
		{Path: "/file2.go", Line: 5},
		{Path: "/file1.go", Line: 20},
	}

	grouped := GroupContentMatchesByFile(matches)

	if len(grouped) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(grouped))
	}
	if len(grouped["/file1.go"]) != 2 {
		t.Errorf("Expected 2 matches for file1.go, got %d", len(grouped["/file1.go"]))
	}
	if len(grouped["/file2.go"]) != 1 {
		t.Errorf("Expected 1 match for file2.go, got %d", len(grouped["/file2.go"]))
	}
}

func TestLimitResults(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	// Limit less than length
	limited := LimitResults(items, 3)
	if len(limited) != 3 {
		t.Errorf("LimitResults(5 items, 3) = %d items, want 3", len(limited))
	}

	// Limit greater than length
	limited = LimitResults(items, 10)
	if len(limited) != 5 {
		t.Errorf("LimitResults(5 items, 10) = %d items, want 5", len(limited))
	}

	// Limit 0 (unlimited)
	limited = LimitResults(items, 0)
	if len(limited) != 5 {
		t.Errorf("LimitResults(5 items, 0) = %d items, want 5", len(limited))
	}
}

func TestDeduplicateFileMatches(t *testing.T) {
	matches := []FileMatch{
		{Path: "/file1.go", Score: 0.9},
		{Path: "/file2.go", Score: 0.8},
		{Path: "/file1.go", Score: 0.7}, // Duplicate
	}

	deduped := DeduplicateFileMatches(matches)

	if len(deduped) != 2 {
		t.Errorf("Expected 2 unique matches, got %d", len(deduped))
	}

	// First occurrence should be kept
	for _, m := range deduped {
		if m.Path == "/file1.go" && m.Score != 0.9 {
			t.Errorf("Expected first occurrence (score 0.9), got %f", m.Score)
		}
	}
}

func TestMergeFileMatches(t *testing.T) {
	set1 := []FileMatch{
		{Path: "/file1.go", Score: 0.9},
		{Path: "/file2.go", Score: 0.8},
	}
	set2 := []FileMatch{
		{Path: "/file1.go", Score: 0.5}, // Lower score, should be ignored
		{Path: "/file3.go", Score: 0.7},
	}
	set3 := []FileMatch{
		{Path: "/file2.go", Score: 0.95}, // Higher score, should replace
	}

	merged := MergeFileMatches(set1, set2, set3)

	if len(merged) != 3 {
		t.Errorf("Expected 3 merged matches, got %d", len(merged))
	}

	// Check scores
	for _, m := range merged {
		switch m.Path {
		case "/file1.go":
			if m.Score != 0.9 {
				t.Errorf("file1.go score = %f, want 0.9 (highest)", m.Score)
			}
		case "/file2.go":
			if m.Score != 0.95 {
				t.Errorf("file2.go score = %f, want 0.95 (highest)", m.Score)
			}
		case "/file3.go":
			if m.Score != 0.7 {
				t.Errorf("file3.go score = %f, want 0.7", m.Score)
			}
		}
	}

	// Should be sorted by score
	if merged[0].Score < merged[1].Score || merged[1].Score < merged[2].Score {
		t.Error("Merged results should be sorted by score descending")
	}
}
