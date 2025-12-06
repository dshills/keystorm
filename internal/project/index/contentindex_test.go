package index

import (
	"bytes"
	"context"
	"testing"
)

func TestNewContentIndex(t *testing.T) {
	config := DefaultContentIndexConfig()
	ci := NewContentIndex(config)

	if ci == nil {
		t.Fatal("NewContentIndex returned nil")
	}
	if ci.DocumentCount() != 0 {
		t.Errorf("DocumentCount() = %d, want 0", ci.DocumentCount())
	}
	if ci.TermCount() != 0 {
		t.Errorf("TermCount() = %d, want 0", ci.TermCount())
	}
}

func TestContentIndex_IndexDocument(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	content := []byte("func main() {\n\tfmt.Println(\"Hello World\")\n}")
	err := ci.IndexDocument("/path/to/file.go", content)
	if err != nil {
		t.Fatalf("IndexDocument() error = %v", err)
	}

	if ci.DocumentCount() != 1 {
		t.Errorf("DocumentCount() = %d, want 1", ci.DocumentCount())
	}

	// Check document metadata
	meta, ok := ci.GetDocument("/path/to/file.go")
	if !ok {
		t.Fatal("GetDocument() returned false")
	}
	if meta.Path != "/path/to/file.go" {
		t.Errorf("Path = %q, want /path/to/file.go", meta.Path)
	}
	if meta.LineCount != 3 {
		t.Errorf("LineCount = %d, want 3", meta.LineCount)
	}
}

func TestContentIndex_RemoveDocument(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	_ = ci.IndexDocument("/path/to/file.go", []byte("func main() {}"))

	err := ci.RemoveDocument("/path/to/file.go")
	if err != nil {
		t.Fatalf("RemoveDocument() error = %v", err)
	}

	if ci.DocumentCount() != 0 {
		t.Errorf("DocumentCount() after remove = %d, want 0", ci.DocumentCount())
	}

	if ci.HasDocument("/path/to/file.go") {
		t.Error("HasDocument() should return false after remove")
	}
}

func TestContentIndex_Search(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	// Index some files
	_ = ci.IndexDocument("/path/to/main.go", []byte("func main() {\n\tfmt.Println(\"Hello\")\n}"))
	_ = ci.IndexDocument("/path/to/util.go", []byte("func helper() {\n\treturn nil\n}"))
	_ = ci.IndexDocument("/path/to/test.go", []byte("func TestMain(t *testing.T) {\n\tmain()\n}"))

	tests := []struct {
		name      string
		query     string
		matchAll  bool
		wantPaths int
	}{
		{"single term", "main", false, 2},         // main.go and test.go
		{"match all terms", "func main", true, 2}, // Files with both func and main
		{"unique term", "helper", false, 1},       // Only util.go
		{"no match", "nonexistent", false, 0},     // No matches
		{"common term", "func", false, 3},         // All files
		{"case insensitive", "MAIN", false, 2},    // Should match main
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := ci.Search(context.Background(), tt.query, ContentSearchOptions{
				MatchAll: tt.matchAll,
			})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(results) != tt.wantPaths {
				t.Errorf("Search() returned %d results, want %d", len(results), tt.wantPaths)
			}
		})
	}
}

func TestContentIndex_SearchWithFilters(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	_ = ci.IndexDocument("/src/main.go", []byte("func main() {}"))
	_ = ci.IndexDocument("/src/util.go", []byte("func main() {}"))
	_ = ci.IndexDocument("/test/test.go", []byte("func main() {}"))

	// Include filter
	results, err := ci.Search(context.Background(), "main", ContentSearchOptions{
		IncludePaths: []string{"/src/"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search() with include filter returned %d results, want 2", len(results))
	}

	// Exclude filter
	results, err = ci.Search(context.Background(), "main", ContentSearchOptions{
		ExcludePaths: []string{"/test/"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search() with exclude filter returned %d results, want 2", len(results))
	}

	// File type filter
	results, err = ci.Search(context.Background(), "main", ContentSearchOptions{
		FileTypes: []string{".go"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 3 {
		t.Errorf("Search() with file type filter returned %d results, want 3", len(results))
	}
}

func TestContentIndex_SearchMaxResults(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	// Index many files with the same term
	for i := 0; i < 10; i++ {
		path := "/path/to/file" + string(rune('0'+i)) + ".go"
		_ = ci.IndexDocument(path, []byte("func common() {}"))
	}

	results, err := ci.Search(context.Background(), "common", ContentSearchOptions{
		MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Search() with MaxResults=5 returned %d results, want 5", len(results))
	}
}

func TestContentIndex_SearchRegex(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	_ = ci.IndexDocument("/path/to/file1.go", []byte("funcOne funcTwo"))
	_ = ci.IndexDocument("/path/to/file2.go", []byte("funcThree funcFour"))
	_ = ci.IndexDocument("/path/to/file3.go", []byte("something else"))

	results, err := ci.SearchRegex(context.Background(), "func.*", ContentSearchOptions{})
	if err != nil {
		t.Fatalf("SearchRegex() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("SearchRegex() returned %d results, want 2", len(results))
	}
}

func TestContentIndex_Clear(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	_ = ci.IndexDocument("/path/to/file1.go", []byte("content"))
	_ = ci.IndexDocument("/path/to/file2.go", []byte("content"))

	ci.Clear()

	if ci.DocumentCount() != 0 {
		t.Errorf("DocumentCount() after Clear = %d, want 0", ci.DocumentCount())
	}
	if ci.TermCount() != 0 {
		t.Errorf("TermCount() after Clear = %d, want 0", ci.TermCount())
	}
}

func TestContentIndex_SaveLoad(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	_ = ci.IndexDocument("/path/to/file1.go", []byte("func main() {}"))
	_ = ci.IndexDocument("/path/to/file2.go", []byte("func helper() {}"))

	// Save
	var buf bytes.Buffer
	err := ci.Save(&buf)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load into new index
	ci2 := NewContentIndex(DefaultContentIndexConfig())
	err = ci2.Load(&buf)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if ci2.DocumentCount() != 2 {
		t.Errorf("Loaded DocumentCount() = %d, want 2", ci2.DocumentCount())
	}

	// Verify search works on loaded index
	results, err := ci2.Search(context.Background(), "main", ContentSearchOptions{})
	if err != nil {
		t.Fatalf("Search() after load error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search() after load returned %d results, want 1", len(results))
	}
}

func TestContentIndex_HasDocument(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	_ = ci.IndexDocument("/path/to/file.go", []byte("content"))

	if !ci.HasDocument("/path/to/file.go") {
		t.Error("HasDocument() should return true for indexed file")
	}
	if ci.HasDocument("/path/to/other.go") {
		t.Error("HasDocument() should return false for non-indexed file")
	}
}

func TestContentIndex_Tokenize(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	tests := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"func_name", []string{"func_name"}},
		{"hello123world", []string{"hello123world"}},
		{"hello-world", []string{"hello", "world"}},
		{"  spaced  out  ", []string{"spaced", "out"}},
		{"CamelCase", []string{"CamelCase"}},
		{"with.dots.here", []string{"with", "dots", "here"}},
	}

	for _, tt := range tests {
		tokens := ci.tokenize(tt.input)
		if len(tokens) != len(tt.want) {
			t.Errorf("tokenize(%q) = %v, want %v", tt.input, tokens, tt.want)
			continue
		}
		for i, tok := range tokens {
			if tok != tt.want[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, tok, tt.want[i])
			}
		}
	}
}

func TestContentIndex_StopWords(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	content := []byte("the quick brown fox jumps over the lazy dog")
	_ = ci.IndexDocument("/path/to/file.txt", content)

	// "the" is a stop word, should not be indexed
	results, err := ci.Search(context.Background(), "the", ContentSearchOptions{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search('the') returned %d results, want 0 (stop word)", len(results))
	}

	// "quick" is not a stop word
	results, err = ci.Search(context.Background(), "quick", ContentSearchOptions{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search('quick') returned %d results, want 1", len(results))
	}
}

func TestContentIndex_CaseSensitive(t *testing.T) {
	config := DefaultContentIndexConfig()
	config.CaseSensitive = true
	ci := NewContentIndex(config)

	_ = ci.IndexDocument("/path/to/file.go", []byte("HelloWorld"))

	// Exact case should match
	results, err := ci.Search(context.Background(), "HelloWorld", ContentSearchOptions{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Case-sensitive search for exact match returned %d results, want 1", len(results))
	}

	// Different case should not match
	results, err = ci.Search(context.Background(), "helloworld", ContentSearchOptions{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Case-sensitive search for wrong case returned %d results, want 0", len(results))
	}
}

func TestContentIndex_ReindexDocument(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	// Initial index
	_ = ci.IndexDocument("/path/to/file.go", []byte("func oldFunction() {}"))

	// Verify old content is indexed
	results, _ := ci.Search(context.Background(), "oldFunction", ContentSearchOptions{})
	if len(results) != 1 {
		t.Errorf("Initial search returned %d results, want 1", len(results))
	}

	// Re-index with new content
	_ = ci.IndexDocument("/path/to/file.go", []byte("func newFunction() {}"))

	// Old content should be gone
	results, _ = ci.Search(context.Background(), "oldFunction", ContentSearchOptions{})
	if len(results) != 0 {
		t.Errorf("Search for old content returned %d results, want 0", len(results))
	}

	// New content should be indexed
	results, _ = ci.Search(context.Background(), "newFunction", ContentSearchOptions{})
	if len(results) != 1 {
		t.Errorf("Search for new content returned %d results, want 1", len(results))
	}

	// Document count should still be 1
	if ci.DocumentCount() != 1 {
		t.Errorf("DocumentCount() = %d, want 1", ci.DocumentCount())
	}
}

func TestContentIndex_SearchContextCancellation(t *testing.T) {
	ci := NewContentIndex(DefaultContentIndexConfig())

	// Index some files
	for i := 0; i < 100; i++ {
		path := "/path/to/file" + string(rune('0'+i%10)) + ".go"
		_ = ci.IndexDocument(path, []byte("content"))
	}

	// Cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ci.Search(ctx, "content", ContentSearchOptions{})
	if err != context.Canceled {
		t.Errorf("Search() with canceled context error = %v, want context.Canceled", err)
	}
}

func TestContentSearchResult(t *testing.T) {
	result := ContentSearchResult{
		Path:        "/path/to/file.go",
		LineNumbers: []int{1, 5, 10},
		Score:       0.95,
		DocumentMeta: DocumentMeta{
			Path:      "/path/to/file.go",
			Size:      1024,
			LineCount: 50,
			WordCount: 200,
		},
	}

	if result.Path != "/path/to/file.go" {
		t.Errorf("Path = %q, want /path/to/file.go", result.Path)
	}
	if len(result.LineNumbers) != 3 {
		t.Errorf("LineNumbers len = %d, want 3", len(result.LineNumbers))
	}
	if result.Score != 0.95 {
		t.Errorf("Score = %f, want 0.95", result.Score)
	}
}
