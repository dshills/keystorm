package search

import (
	"context"
	"strings"
	"testing"

	"github.com/dshills/keystorm/internal/project/vfs"
)

func TestContentSearch_Search_Simple(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	// Index a file
	content := []byte("hello world\nthis is a test\nhello again")
	_ = cs.IndexFile("/project/test.go", content)

	opts := DefaultContentSearchOptions()
	results, err := cs.Search(context.Background(), "hello", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check first result
	if results[0].Line != 1 {
		t.Errorf("First result line = %d, want 1", results[0].Line)
	}
	if !strings.Contains(results[0].Text, "hello world") {
		t.Errorf("First result text = %q, want 'hello world'", results[0].Text)
	}
}

func TestContentSearch_Search_CaseSensitive(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	content := []byte("Hello World\nhello world")
	_ = cs.IndexFile("/project/test.go", content)

	opts := DefaultContentSearchOptions()
	opts.CaseSensitive = true

	results, err := cs.Search(context.Background(), "Hello", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result with case sensitivity, got %d", len(results))
	}
}

func TestContentSearch_Search_WholeWord(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	content := []byte("hello world\nhelloworld\nworld hello")
	_ = cs.IndexFile("/project/test.go", content)

	opts := DefaultContentSearchOptions()
	opts.WholeWord = true

	results, err := cs.Search(context.Background(), "hello", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	// Should match "hello world" and "world hello" but not "helloworld"
	if len(results) != 2 {
		t.Errorf("Expected 2 results with whole word, got %d", len(results))
	}
}

func TestContentSearch_Search_Regex(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	content := []byte("hello123\nworld456\nhello789")
	_ = cs.IndexFile("/project/test.go", content)

	opts := DefaultContentSearchOptions()
	opts.UseRegex = true

	results, err := cs.Search(context.Background(), "hello\\d+", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results with regex, got %d", len(results))
	}
}

func TestContentSearch_Search_ContextLines(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	content := []byte("line1\nline2\nTARGET\nline4\nline5")
	_ = cs.IndexFile("/project/test.go", content)

	opts := DefaultContentSearchOptions()
	opts.ContextLines = 2

	results, err := cs.Search(context.Background(), "TARGET", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if len(result.ContextBefore) != 2 {
		t.Errorf("Expected 2 lines before, got %d", len(result.ContextBefore))
	}
	if len(result.ContextAfter) != 2 {
		t.Errorf("Expected 2 lines after, got %d", len(result.ContextAfter))
	}
	if result.ContextBefore[0] != "line1" {
		t.Errorf("ContextBefore[0] = %q, want 'line1'", result.ContextBefore[0])
	}
	if result.ContextAfter[1] != "line5" {
		t.Errorf("ContextAfter[1] = %q, want 'line5'", result.ContextAfter[1])
	}
}

func TestContentSearch_Search_FileTypeFilter(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	_ = cs.IndexFile("/project/main.go", []byte("hello from go"))
	_ = cs.IndexFile("/project/main.js", []byte("hello from js"))

	opts := DefaultContentSearchOptions()
	opts.FileTypes = []string{".go"}

	results, err := cs.Search(context.Background(), "hello", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].Path != "/project/main.go" {
		t.Errorf("Expected main.go, got %s", results[0].Path)
	}
}

func TestContentSearch_Search_ExcludePaths(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	_ = cs.IndexFile("/project/src/main.go", []byte("hello"))
	_ = cs.IndexFile("/project/vendor/lib.go", []byte("hello"))

	opts := DefaultContentSearchOptions()
	opts.ExcludePaths = []string{"**/vendor/**"}

	results, err := cs.Search(context.Background(), "hello", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].Path != "/project/src/main.go" {
		t.Errorf("Expected /project/src/main.go, got %s", results[0].Path)
	}
}

func TestContentSearch_Search_MaxResults(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	// Create content with many matches
	lines := make([]string, 50)
	for i := 0; i < 50; i++ {
		lines[i] = "hello world"
	}
	content := []byte(strings.Join(lines, "\n"))
	_ = cs.IndexFile("/project/test.go", content)

	opts := DefaultContentSearchOptions()
	opts.MaxResults = 10

	results, err := cs.Search(context.Background(), "hello", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) > 10 {
		t.Errorf("Expected at most 10 results, got %d", len(results))
	}
}

func TestContentSearch_Search_Highlights(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	content := []byte("hello world hello")
	_ = cs.IndexFile("/project/test.go", content)

	opts := DefaultContentSearchOptions()
	results, err := cs.Search(context.Background(), "hello", opts)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Should have 2 highlights (two occurrences of "hello")
	if len(results[0].Highlights) != 2 {
		t.Errorf("Expected 2 highlights, got %d", len(results[0].Highlights))
	}

	// First highlight at start
	if results[0].Highlights[0].Start != 0 {
		t.Errorf("First highlight start = %d, want 0", results[0].Highlights[0].Start)
	}
	if results[0].Highlights[0].End != 5 {
		t.Errorf("First highlight end = %d, want 5", results[0].Highlights[0].End)
	}
}

func TestContentSearch_Search_EmptyQuery(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	_ = cs.IndexFile("/project/test.go", []byte("hello world"))

	opts := DefaultContentSearchOptions()
	_, err := cs.Search(context.Background(), "", opts)
	if err != ErrInvalidQuery {
		t.Errorf("Expected ErrInvalidQuery for empty query, got %v", err)
	}
}

func TestContentSearch_Search_Canceled(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	// Index many files
	for i := 0; i < 100; i++ {
		name := sprintf("/project/file%d.go", i)
		_ = cs.IndexFile(name, []byte("hello world"))
	}

	opts := DefaultContentSearchOptions()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := cs.Search(ctx, "hello", opts)
	if err != ErrSearchCanceled {
		t.Errorf("Expected ErrSearchCanceled, got %v", err)
	}
}

func TestContentSearch_IndexFile(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	err := cs.IndexFile("/project/test.go", []byte("hello world"))
	if err != nil {
		t.Fatalf("IndexFile error = %v", err)
	}

	if cs.IndexedCount() != 1 {
		t.Errorf("IndexedCount = %d, want 1", cs.IndexedCount())
	}
}

func TestContentSearch_RemoveFile(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	_ = cs.IndexFile("/project/test.go", []byte("hello world"))
	_ = cs.RemoveFile("/project/test.go")

	if cs.IndexedCount() != 0 {
		t.Errorf("IndexedCount = %d, want 0", cs.IndexedCount())
	}
}

func TestContentSearch_Clear(t *testing.T) {
	fs := vfs.NewMemFS()
	cs := NewContentSearch(fs)

	_ = cs.IndexFile("/project/test1.go", []byte("hello"))
	_ = cs.IndexFile("/project/test2.go", []byte("world"))

	cs.Clear()

	if cs.IndexedCount() != 0 {
		t.Errorf("IndexedCount = %d after Clear, want 0", cs.IndexedCount())
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"", 0},
		{"hello", 1},
		{"hello\nworld", 2},
		{"hello\nworld\n", 2},
		{"line1\nline2\nline3", 3},
	}

	for _, tt := range tests {
		lines := splitLines([]byte(tt.content))
		if len(lines) != tt.want {
			t.Errorf("splitLines(%q) = %d lines, want %d", tt.content, len(lines), tt.want)
		}
	}
}

func TestGetContextBefore(t *testing.T) {
	lines := []string{"line0", "line1", "line2", "line3", "line4"}

	tests := []struct {
		lineNum int
		count   int
		want    int
	}{
		{2, 2, 2},  // line2 with 2 lines before
		{1, 2, 1},  // line1 with 2 lines before (only 1 available)
		{0, 2, 0},  // line0 with 2 lines before (none available)
		{3, 0, 0},  // 0 context lines
		{4, 10, 4}, // more context than available
	}

	for _, tt := range tests {
		before := getContextBefore(lines, tt.lineNum, tt.count)
		if len(before) != tt.want {
			t.Errorf("getContextBefore(lines, %d, %d) = %d lines, want %d",
				tt.lineNum, tt.count, len(before), tt.want)
		}
	}
}

func TestGetContextAfter(t *testing.T) {
	lines := []string{"line0", "line1", "line2", "line3", "line4"}

	tests := []struct {
		lineNum int
		count   int
		want    int
	}{
		{2, 2, 2},  // line2 with 2 lines after
		{3, 2, 1},  // line3 with 2 lines after (only 1 available)
		{4, 2, 0},  // line4 with 2 lines after (none available)
		{1, 0, 0},  // 0 context lines
		{0, 10, 4}, // more context than available
	}

	for _, tt := range tests {
		after := getContextAfter(lines, tt.lineNum, tt.count)
		if len(after) != tt.want {
			t.Errorf("getContextAfter(lines, %d, %d) = %d lines, want %d",
				tt.lineNum, tt.count, len(after), tt.want)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.go", "/project/main.go", true},
		{"*.go", "/project/main.js", false},
		{"**/vendor/**", "/project/vendor/lib.go", true},
		{"**/node_modules/**", "/project/src/main.go", false},
		{"test_*", "/project/test_main.go", true},
	}

	for _, tt := range tests {
		if got := matchGlob(tt.pattern, tt.path); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}
