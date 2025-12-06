package search

import (
	"context"
	"io"
	"testing"

	"github.com/dshills/keystorm/internal/project/index"
)

// mockIndex implements a simple index for testing.
type mockIndex struct {
	entries map[string]index.FileInfo
}

func newMockIndex() *mockIndex {
	return &mockIndex{
		entries: make(map[string]index.FileInfo),
	}
}

func (m *mockIndex) Add(path string, info index.FileInfo) error {
	info.Path = path
	m.entries[path] = info
	return nil
}

func (m *mockIndex) Remove(path string) error {
	delete(m.entries, path)
	return nil
}

func (m *mockIndex) Update(path string, info index.FileInfo) error {
	m.entries[path] = info
	return nil
}

func (m *mockIndex) Get(path string) (index.FileInfo, bool) {
	info, ok := m.entries[path]
	return info, ok
}

func (m *mockIndex) Has(path string) bool {
	_, ok := m.entries[path]
	return ok
}

func (m *mockIndex) Count() int {
	return len(m.entries)
}

func (m *mockIndex) All() []string {
	paths := make([]string, 0, len(m.entries))
	for path := range m.entries {
		paths = append(paths, path)
	}
	return paths
}

func (m *mockIndex) Query(q index.Query) ([]index.Result, error) {
	return nil, nil
}

func (m *mockIndex) Clear() {
	m.entries = make(map[string]index.FileInfo)
}

func (m *mockIndex) Save(w io.Writer) error { return nil }
func (m *mockIndex) Load(r io.Reader) error { return nil }
func (m *mockIndex) Close() error           { return nil }

func TestFuzzySearcher_Search_EmptyQuery(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})

	fs := NewFuzzySearcher(idx)
	results, err := fs.Search(context.Background(), "", DefaultFileSearchOptions())
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty query, got %d", len(results))
	}
}

func TestFuzzySearcher_Search_ExactMatch(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})
	_ = idx.Add("/project/util.go", index.FileInfo{Name: "util.go"})
	_ = idx.Add("/project/domain.go", index.FileInfo{Name: "domain.go"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchExact

	results, err := fs.Search(context.Background(), "main.go", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Name != "main.go" {
		t.Errorf("Expected main.go, got %s", results[0].Name)
	}
}

func TestFuzzySearcher_Search_FuzzyMatch(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})
	_ = idx.Add("/project/util.go", index.FileInfo{Name: "util.go"})
	_ = idx.Add("/project/domain.go", index.FileInfo{Name: "domain.go"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchFuzzy

	// "mgo" should match "main.go"
	results, err := fs.Search(context.Background(), "mgo", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	found := false
	for _, r := range results {
		if r.Name == "main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find main.go with fuzzy search for 'mgo'")
	}
}

func TestFuzzySearcher_Search_PrefixMatch(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})
	_ = idx.Add("/project/maintest.go", index.FileInfo{Name: "maintest.go"})
	_ = idx.Add("/project/util.go", index.FileInfo{Name: "util.go"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchPrefix

	results, err := fs.Search(context.Background(), "main", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestFuzzySearcher_Search_ContainsMatch(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})
	_ = idx.Add("/project/domain.go", index.FileInfo{Name: "domain.go"})
	_ = idx.Add("/project/util.go", index.FileInfo{Name: "util.go"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchContains

	// "ain" should match "main.go" and "domain.go"
	results, err := fs.Search(context.Background(), "ain", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestFuzzySearcher_Search_GlobMatch(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})
	_ = idx.Add("/project/util.go", index.FileInfo{Name: "util.go"})
	_ = idx.Add("/project/test.js", index.FileInfo{Name: "test.js"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchGlob

	results, err := fs.Search(context.Background(), "*.go", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for *.go, got %d", len(results))
	}
}

func TestFuzzySearcher_Search_FileTypeFilter(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})
	_ = idx.Add("/project/util.go", index.FileInfo{Name: "util.go"})
	_ = idx.Add("/project/test.js", index.FileInfo{Name: "test.js"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchFuzzy
	opts.FileTypes = []string{".go"}

	results, err := fs.Search(context.Background(), "main", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	for _, r := range results {
		if r.Name == "test.js" {
			t.Error("test.js should be filtered out")
		}
	}
}

func TestFuzzySearcher_Search_PathPrefixFilter(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/src/main.go", index.FileInfo{Name: "main.go"})
	_ = idx.Add("/project/test/main_test.go", index.FileInfo{Name: "main_test.go"})
	_ = idx.Add("/other/main.go", index.FileInfo{Name: "main.go"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchFuzzy
	opts.PathPrefix = "/project"

	results, err := fs.Search(context.Background(), "main", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	for _, r := range results {
		if r.Path == "/other/main.go" {
			t.Error("/other/main.go should be filtered out")
		}
	}
}

func TestFuzzySearcher_Search_IncludeDirs(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/src", index.FileInfo{Name: "src", IsDir: true})
	_ = idx.Add("/project/src/main.go", index.FileInfo{Name: "main.go"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchFuzzy
	opts.IncludeDirs = false

	results, err := fs.Search(context.Background(), "src", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	for _, r := range results {
		if r.IsDir {
			t.Error("Directories should be filtered out")
		}
	}

	// Now with dirs included
	opts.IncludeDirs = true
	results, err = fs.Search(context.Background(), "src", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	found := false
	for _, r := range results {
		if r.Name == "src" && r.IsDir {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find src directory with IncludeDirs=true")
	}
}

func TestFuzzySearcher_Search_MaxResults(t *testing.T) {
	idx := newMockIndex()
	for i := 0; i < 50; i++ {
		name := sprintf("file%d.go", i)
		_ = idx.Add("/project/"+name, index.FileInfo{Name: name})
	}

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchFuzzy
	opts.MaxResults = 10

	results, err := fs.Search(context.Background(), "file", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
	}
}

func TestFuzzySearcher_Search_CaseSensitive(t *testing.T) {
	idx := newMockIndex()
	_ = idx.Add("/project/Main.go", index.FileInfo{Name: "Main.go"})
	_ = idx.Add("/project/main.go", index.FileInfo{Name: "main.go"})

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()
	opts.MatchMode = MatchExact
	opts.CaseSensitive = true

	results, err := fs.Search(context.Background(), "Main.go", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result with case sensitivity, got %d", len(results))
	}
	if results[0].Name != "Main.go" {
		t.Errorf("Expected Main.go, got %s", results[0].Name)
	}
}

func TestFuzzySearcher_Search_Canceled(t *testing.T) {
	idx := newMockIndex()
	for i := 0; i < 100; i++ {
		name := sprintf("file%d.go", i)
		_ = idx.Add("/project/"+name, index.FileInfo{Name: name})
	}

	fs := NewFuzzySearcher(idx)
	opts := DefaultFileSearchOptions()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := fs.Search(ctx, "file", opts)
	if err != ErrSearchCanceled {
		t.Errorf("Expected ErrSearchCanceled, got %v", err)
	}
}

func TestFuzzyMatchWithPositions_ExactMatch(t *testing.T) {
	score, positions := fuzzyMatchWithPositions("main.go", "main.go", false)
	if score != 1.0 {
		t.Errorf("Exact match score = %f, want 1.0", score)
	}
	if len(positions) != 7 {
		t.Errorf("Expected 7 positions, got %d", len(positions))
	}
}

func TestFuzzyMatchWithPositions_SubsequenceMatch(t *testing.T) {
	score, positions := fuzzyMatchWithPositions("mgo", "main.go", false)
	if score == 0 {
		t.Error("Expected positive score for subsequence match")
	}
	if len(positions) != 3 {
		t.Errorf("Expected 3 positions, got %d", len(positions))
	}
	// Should match: m(0), g(5), o(6)
	if positions[0] != 0 {
		t.Errorf("First match should be at 0, got %d", positions[0])
	}
}

func TestFuzzyMatchWithPositions_NoMatch(t *testing.T) {
	score, positions := fuzzyMatchWithPositions("xyz", "main.go", false)
	if score != 0 {
		t.Errorf("No match score = %f, want 0", score)
	}
	if positions != nil {
		t.Error("No match should return nil positions")
	}
}

func TestFuzzyMatchWithPositions_CaseSensitive(t *testing.T) {
	// Case insensitive should match
	score, _ := fuzzyMatchWithPositions("MAIN", "main.go", false)
	if score == 0 {
		t.Error("Case insensitive should match 'MAIN' to 'main.go'")
	}

	// Case sensitive should not match
	score, _ = fuzzyMatchWithPositions("MAIN", "main.go", true)
	if score != 0 {
		t.Errorf("Case sensitive should not match 'MAIN' to 'main.go', score = %f", score)
	}
}

func TestFuzzyMatchWithPositions_ConsecutiveBonus(t *testing.T) {
	// Consecutive matches should score higher
	scoreConsecutive, _ := fuzzyMatchWithPositions("main", "main_test.go", false)
	scoreScattered, _ := fuzzyMatchWithPositions("main", "my_app_index_new.go", false)

	if scoreConsecutive <= scoreScattered {
		t.Errorf("Consecutive matches should score higher: consecutive=%f, scattered=%f",
			scoreConsecutive, scoreScattered)
	}
}

func TestFuzzyMatchWithPositions_WordBoundaryBonus(t *testing.T) {
	// Word boundary matches should score higher
	scoreWordBoundary, _ := fuzzyMatchWithPositions("mt", "main_test.go", false)
	scoreMiddle, _ := fuzzyMatchWithPositions("mt", "sometmiddlet.go", false)

	if scoreWordBoundary <= scoreMiddle {
		t.Errorf("Word boundary matches should score higher: boundary=%f, middle=%f",
			scoreWordBoundary, scoreMiddle)
	}
}

func TestFuzzyMatchWithPositions_EmptyPattern(t *testing.T) {
	score, _ := fuzzyMatchWithPositions("", "main.go", false)
	if score != 1.0 {
		t.Errorf("Empty pattern score = %f, want 1.0", score)
	}
}

func TestFuzzyMatchWithPositions_EmptyTarget(t *testing.T) {
	score, _ := fuzzyMatchWithPositions("main", "", false)
	if score != 0 {
		t.Errorf("Empty target score = %f, want 0", score)
	}
}

func TestFuzzyMatchWithPositions_PatternLongerThanTarget(t *testing.T) {
	score, _ := fuzzyMatchWithPositions("longpattern", "short", false)
	if score != 0 {
		t.Errorf("Pattern longer than target score = %f, want 0", score)
	}
}

// sprintf is a helper for formatting strings without importing fmt
func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	// Simple implementation for %d
	result := format
	for _, arg := range args {
		if n, ok := arg.(int); ok {
			result = replaceFirst(result, "%d", itoa(n))
		}
	}
	return result
}

func replaceFirst(s, old, new string) string {
	i := 0
	for j := 0; j < len(s)-len(old)+1; j++ {
		if s[j:j+len(old)] == old {
			i = j
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
