package index

import (
	"os"
	"testing"
	"time"
)

func TestNewFileIndex(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	if idx.Count() != 0 {
		t.Errorf("Count() = %d, want 0", idx.Count())
	}
}

func TestNewFileIndex_WithOptions(t *testing.T) {
	idx := NewFileIndex(
		WithInitialCapacity(100),
		WithCaseSensitive(true),
	)
	defer idx.Close()

	if idx.config.InitialCapacity != 100 {
		t.Errorf("InitialCapacity = %d, want 100", idx.config.InitialCapacity)
	}
	if !idx.config.CaseSensitive {
		t.Error("CaseSensitive should be true")
	}
}

func TestFileIndex_AddGetRemove(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	info := FileInfo{
		Name:    "test.go",
		Size:    1024,
		ModTime: time.Now(),
		IsDir:   false,
		Mode:    0644,
	}

	// Add
	err := idx.Add("/project/src/test.go", info)
	if err != nil {
		t.Fatalf("Add error = %v", err)
	}

	if idx.Count() != 1 {
		t.Errorf("Count() = %d, want 1", idx.Count())
	}

	// Get
	got, ok := idx.Get("/project/src/test.go")
	if !ok {
		t.Fatal("Get returned false")
	}
	if got.Name != "test.go" {
		t.Errorf("Name = %q, want %q", got.Name, "test.go")
	}
	if got.Size != 1024 {
		t.Errorf("Size = %d, want 1024", got.Size)
	}

	// Has
	if !idx.Has("/project/src/test.go") {
		t.Error("Has should return true")
	}
	if idx.Has("/nonexistent") {
		t.Error("Has should return false for nonexistent path")
	}

	// Remove
	err = idx.Remove("/project/src/test.go")
	if err != nil {
		t.Fatalf("Remove error = %v", err)
	}

	if idx.Count() != 0 {
		t.Errorf("Count() = %d after remove, want 0", idx.Count())
	}

	_, ok = idx.Get("/project/src/test.go")
	if ok {
		t.Error("Get should return false after remove")
	}
}

func TestFileIndex_Add_Duplicate(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	info := FileInfo{Name: "test.go"}
	_ = idx.Add("/test.go", info)

	err := idx.Add("/test.go", info)
	if err != ErrAlreadyExists {
		t.Errorf("Add duplicate error = %v, want ErrAlreadyExists", err)
	}
}

func TestFileIndex_Remove_NotFound(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	err := idx.Remove("/nonexistent")
	if err != ErrNotFound {
		t.Errorf("Remove error = %v, want ErrNotFound", err)
	}
}

func TestFileIndex_Update(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	info := FileInfo{Name: "test.go", Size: 1024}
	_ = idx.Add("/test.go", info)

	// Update
	newInfo := FileInfo{Name: "test.go", Size: 2048}
	err := idx.Update("/test.go", newInfo)
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}

	got, _ := idx.Get("/test.go")
	if got.Size != 2048 {
		t.Errorf("Size = %d after update, want 2048", got.Size)
	}
}

func TestFileIndex_Update_NotFound(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	err := idx.Update("/nonexistent", FileInfo{})
	if err != ErrNotFound {
		t.Errorf("Update error = %v, want ErrNotFound", err)
	}
}

func TestFileIndex_All(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/a.go", FileInfo{Name: "a.go"})
	_ = idx.Add("/b.go", FileInfo{Name: "b.go"})
	_ = idx.Add("/c.go", FileInfo{Name: "c.go"})

	all := idx.All()
	if len(all) != 3 {
		t.Fatalf("All() length = %d, want 3", len(all))
	}

	// Should be sorted
	if all[0] != "/a.go" || all[1] != "/b.go" || all[2] != "/c.go" {
		t.Errorf("All() = %v, want sorted", all)
	}
}

func TestFileIndex_Clear(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/a.go", FileInfo{Name: "a.go"})
	_ = idx.Add("/b.go", FileInfo{Name: "b.go"})

	idx.Clear()

	if idx.Count() != 0 {
		t.Errorf("Count() = %d after clear, want 0", idx.Count())
	}
}

func TestFileIndex_Close(t *testing.T) {
	idx := NewFileIndex()
	_ = idx.Add("/test.go", FileInfo{Name: "test.go"})

	err := idx.Close()
	if err != nil {
		t.Fatalf("Close error = %v", err)
	}

	// Operations after close should fail
	err = idx.Add("/new.go", FileInfo{})
	if err != ErrIndexClosed {
		t.Errorf("Add after close error = %v, want ErrIndexClosed", err)
	}

	_, err = idx.Query(DefaultQuery("test"))
	if err != ErrIndexClosed {
		t.Errorf("Query after close error = %v, want ErrIndexClosed", err)
	}
}

func TestFileIndex_GetByDirectory(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/project/src/a.go", FileInfo{Name: "a.go"})
	_ = idx.Add("/project/src/b.go", FileInfo{Name: "b.go"})
	_ = idx.Add("/project/test/c.go", FileInfo{Name: "c.go"})

	files := idx.GetByDirectory("/project/src")
	if len(files) != 2 {
		t.Errorf("GetByDirectory length = %d, want 2", len(files))
	}
}

func TestFileIndex_GetByName(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/project/src/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/project/cmd/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/project/test.go", FileInfo{Name: "test.go"})

	files := idx.GetByName("main.go")
	if len(files) != 2 {
		t.Errorf("GetByName length = %d, want 2", len(files))
	}

	// Case insensitive
	files = idx.GetByName("MAIN.GO")
	if len(files) != 2 {
		t.Errorf("GetByName (case insensitive) length = %d, want 2", len(files))
	}
}

func TestFileIndex_Query_Exact(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/main_test.go", FileInfo{Name: "main_test.go"})
	_ = idx.Add("/app.go", FileInfo{Name: "app.go"})

	results, err := idx.Query(Query{
		Pattern:   "main.go",
		MatchType: MatchExact,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("results length = %d, want 1", len(results))
	}
	if results[0].Info.Name != "main.go" {
		t.Errorf("result name = %q, want main.go", results[0].Info.Name)
	}
}

func TestFileIndex_Query_Prefix(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/main_test.go", FileInfo{Name: "main_test.go"})
	_ = idx.Add("/app.go", FileInfo{Name: "app.go"})

	results, err := idx.Query(Query{
		Pattern:   "main",
		MatchType: MatchPrefix,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFileIndex_Query_Suffix(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/main_test.go", FileInfo{Name: "main_test.go"})
	_ = idx.Add("/other_test.go", FileInfo{Name: "other_test.go"})

	results, err := idx.Query(Query{
		Pattern:   "_test.go",
		MatchType: MatchSuffix,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFileIndex_Query_Contains(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/handler.go", FileInfo{Name: "handler.go"})
	_ = idx.Add("/user_handler.go", FileInfo{Name: "user_handler.go"})
	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})

	results, err := idx.Query(Query{
		Pattern:   "handler",
		MatchType: MatchContains,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFileIndex_Query_Fuzzy(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/main_handler.go", FileInfo{Name: "main_handler.go"})
	_ = idx.Add("/user_service.go", FileInfo{Name: "user_service.go"})

	results, err := idx.Query(Query{
		Pattern:   "mh",
		MatchType: MatchFuzzy,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}

	// Should match main_handler.go (m...h)
	if len(results) != 1 {
		t.Errorf("results length = %d, want 1", len(results))
	}
}

func TestFileIndex_Query_Glob(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/main.ts", FileInfo{Name: "main.ts"})
	_ = idx.Add("/app.go", FileInfo{Name: "app.go"})

	results, err := idx.Query(Query{
		Pattern:   "*.go",
		MatchType: MatchGlob,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFileIndex_Query_Regex(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/main_test.go", FileInfo{Name: "main_test.go"})
	_ = idx.Add("/app.go", FileInfo{Name: "app.go"})

	results, err := idx.Query(Query{
		Pattern:   "^main.*\\.go$",
		MatchType: MatchRegex,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFileIndex_Query_Regex_Invalid(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})

	_, err := idx.Query(Query{
		Pattern:   "[invalid",
		MatchType: MatchRegex,
	})
	if err != ErrInvalidQuery {
		t.Errorf("Query error = %v, want ErrInvalidQuery", err)
	}
}

func TestFileIndex_Query_CaseSensitive(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/Main.go", FileInfo{Name: "Main.go"})
	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})

	// Case insensitive (default)
	results, _ := idx.Query(Query{
		Pattern:       "MAIN.GO",
		MatchType:     MatchExact,
		CaseSensitive: false,
	})
	if len(results) != 2 {
		t.Errorf("case insensitive results = %d, want 2", len(results))
	}

	// Case sensitive
	results, _ = idx.Query(Query{
		Pattern:       "Main.go",
		MatchType:     MatchExact,
		CaseSensitive: true,
	})
	if len(results) != 1 {
		t.Errorf("case sensitive results = %d, want 1", len(results))
	}
}

func TestFileIndex_Query_FileTypes(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/main.ts", FileInfo{Name: "main.ts"})
	_ = idx.Add("/main.py", FileInfo{Name: "main.py"})

	results, _ := idx.Query(Query{
		Pattern:   "main",
		MatchType: MatchPrefix,
		FileTypes: []string{".go", ".ts"},
	})

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFileIndex_Query_IncludeDirs(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/src", FileInfo{Name: "src", IsDir: true})
	_ = idx.Add("/src/main.go", FileInfo{Name: "main.go"})

	// Without IncludeDirs
	results, _ := idx.Query(Query{
		Pattern:     "src",
		MatchType:   MatchExact,
		IncludeDirs: false,
	})
	if len(results) != 0 {
		t.Errorf("without IncludeDirs, results = %d, want 0", len(results))
	}

	// With IncludeDirs
	results, _ = idx.Query(Query{
		Pattern:     "src",
		MatchType:   MatchExact,
		IncludeDirs: true,
	})
	if len(results) != 1 {
		t.Errorf("with IncludeDirs, results = %d, want 1", len(results))
	}
}

func TestFileIndex_Query_MaxResults(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	for i := 0; i < 100; i++ {
		_ = idx.Add(sprintf("/file%d.go", i), FileInfo{Name: sprintf("file%d.go", i)})
	}

	results, _ := idx.Query(Query{
		Pattern:    "file",
		MatchType:  MatchPrefix,
		MaxResults: 10,
	})

	if len(results) != 10 {
		t.Errorf("results length = %d, want 10", len(results))
	}
}

func TestFileIndex_Query_PathPrefix(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/project/src/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/project/test/main.go", FileInfo{Name: "main.go"})
	_ = idx.Add("/other/main.go", FileInfo{Name: "main.go"})

	results, _ := idx.Query(Query{
		Pattern:    "main.go",
		MatchType:  MatchExact,
		PathPrefix: "/project",
	})

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFileIndex_Query_EmptyPattern(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/a.go", FileInfo{Name: "a.go"})
	_ = idx.Add("/b.go", FileInfo{Name: "b.go"})

	// Empty pattern with fuzzy match returns all
	results, _ := idx.Query(Query{
		Pattern:   "",
		MatchType: MatchFuzzy,
	})

	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		pattern string
		target  string
		want    bool // whether match should be > 0
	}{
		{"", "anything", true},
		{"abc", "abc", true},
		{"abc", "aabbcc", true},
		{"abc", "abcdef", true},
		{"mh", "main_handler", true},
		{"fh", "file_handler.go", true},
		{"xyz", "abc", false},
		{"abcd", "abc", false}, // pattern longer than target
	}

	for _, tt := range tests {
		score := fuzzyMatch(tt.pattern, tt.target)
		got := score > 0
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v (score=%f), want match=%v",
				tt.pattern, tt.target, got, score, tt.want)
		}
	}
}

func TestFuzzyMatch_Scoring(t *testing.T) {
	// Exact match should score highest
	exact := fuzzyMatch("main", "main")
	partial := fuzzyMatch("main", "main_test")
	subsequence := fuzzyMatch("main", "my_app_index_name")

	if exact <= partial {
		t.Error("exact match should score higher than partial")
	}
	if partial <= subsequence {
		t.Error("partial match should score higher than subsequence")
	}
}

// Helper to format strings
func sprintf(format string, args ...interface{}) string {
	// Simple implementation for testing
	result := format
	for _, arg := range args {
		switch v := arg.(type) {
		case int:
			result = replaceFirst(result, "%d", itoa(v))
		case string:
			result = replaceFirst(result, "%s", v)
		}
	}
	return result
}

func replaceFirst(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func BenchmarkFileIndex_Add(b *testing.B) {
	idx := NewFileIndex()
	defer idx.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := sprintf("/project/src/file%d.go", i)
		_ = idx.Add(path, FileInfo{Name: sprintf("file%d.go", i)})
	}
}

func BenchmarkFileIndex_Query_Fuzzy(b *testing.B) {
	idx := NewFileIndex()
	defer idx.Close()

	// Add many files
	for i := 0; i < 10000; i++ {
		path := sprintf("/project/src/file%d.go", i)
		_ = idx.Add(path, FileInfo{Name: sprintf("file%d.go", i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Query(Query{
			Pattern:    "file50",
			MatchType:  MatchFuzzy,
			MaxResults: 100,
		})
	}
}

func BenchmarkFileIndex_Query_Contains(b *testing.B) {
	idx := NewFileIndex()
	defer idx.Close()

	// Add many files
	for i := 0; i < 10000; i++ {
		path := sprintf("/project/src/file%d.go", i)
		_ = idx.Add(path, FileInfo{Name: sprintf("file%d.go", i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Query(Query{
			Pattern:    "500",
			MatchType:  MatchContains,
			MaxResults: 100,
		})
	}
}

// Integration test
func TestFileIndex_Integration(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	// Simulate building an index
	files := []struct {
		path string
		info FileInfo
	}{
		{"/project/src/main.go", FileInfo{Name: "main.go", Size: 1024, Mode: 0644}},
		{"/project/src/handler/user.go", FileInfo{Name: "user.go", Size: 2048, Mode: 0644}},
		{"/project/src/handler/auth.go", FileInfo{Name: "auth.go", Size: 1536, Mode: 0644}},
		{"/project/src/model/user.go", FileInfo{Name: "user.go", Size: 512, Mode: 0644}},
		{"/project/test/main_test.go", FileInfo{Name: "main_test.go", Size: 768, Mode: 0644}},
		{"/project/README.md", FileInfo{Name: "README.md", Size: 256, Mode: 0644}},
		{"/project/go.mod", FileInfo{Name: "go.mod", Size: 128, Mode: 0644}},
	}

	for _, f := range files {
		f.info.ModTime = time.Now()
		if err := idx.Add(f.path, f.info); err != nil {
			t.Fatalf("Add(%q) error = %v", f.path, err)
		}
	}

	if idx.Count() != len(files) {
		t.Errorf("Count() = %d, want %d", idx.Count(), len(files))
	}

	// Find all Go files
	results, _ := idx.Query(Query{
		Pattern:   "*.go",
		MatchType: MatchGlob,
	})
	if len(results) != 5 {
		t.Errorf("*.go results = %d, want 5", len(results))
	}

	// Find user files
	results, _ = idx.Query(Query{
		Pattern:   "user",
		MatchType: MatchContains,
	})
	if len(results) != 2 {
		t.Errorf("user results = %d, want 2", len(results))
	}

	// Find test files
	results, _ = idx.Query(Query{
		Pattern:   "_test.go",
		MatchType: MatchSuffix,
	})
	if len(results) != 1 {
		t.Errorf("_test.go results = %d, want 1", len(results))
	}

	// Update a file
	if err := idx.Update("/project/src/main.go", FileInfo{
		Name:    "main.go",
		Size:    2000,
		ModTime: time.Now(),
		Mode:    0644,
	}); err != nil {
		t.Fatalf("Update error = %v", err)
	}

	info, _ := idx.Get("/project/src/main.go")
	if info.Size != 2000 {
		t.Errorf("After update, Size = %d, want 2000", info.Size)
	}

	// Remove a file
	if err := idx.Remove("/project/README.md"); err != nil {
		t.Fatalf("Remove error = %v", err)
	}

	if idx.Count() != len(files)-1 {
		t.Errorf("Count() after remove = %d, want %d", idx.Count(), len(files)-1)
	}
}

func TestFileIndex_NameIndex_AfterUpdate(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	_ = idx.Add("/test.go", FileInfo{Name: "test.go"})

	// Update with different name
	_ = idx.Update("/test.go", FileInfo{Name: "test_new.go"})

	// Old name should not find
	files := idx.GetByName("test.go")
	if len(files) != 0 {
		t.Error("old name should not be in index")
	}

	// New name should find
	files = idx.GetByName("test_new.go")
	if len(files) != 1 {
		t.Error("new name should be in index")
	}
}

func TestIsAlphaNum(t *testing.T) {
	tests := []struct {
		b    byte
		want bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'_', false},
		{'-', false},
		{' ', false},
		{'.', false},
	}

	for _, tt := range tests {
		if got := isAlphaNum(tt.b); got != tt.want {
			t.Errorf("isAlphaNum(%c) = %v, want %v", tt.b, got, tt.want)
		}
	}
}

// Verify FileInfo is properly populated on Add
func TestFileIndex_Add_PopulatesPath(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	// Add with empty Name
	_ = idx.Add("/project/src/main.go", FileInfo{})

	info, _ := idx.Get("/project/src/main.go")
	if info.Path != "/project/src/main.go" {
		t.Errorf("Path = %q, want /project/src/main.go", info.Path)
	}
	if info.Name != "main.go" {
		t.Errorf("Name = %q, want main.go", info.Name)
	}
}

// Test file mode preservation
func TestFileIndex_FileMode(t *testing.T) {
	idx := NewFileIndex()
	defer idx.Close()

	mode := os.FileMode(0755)
	_ = idx.Add("/script.sh", FileInfo{Name: "script.sh", Mode: mode})

	info, _ := idx.Get("/script.sh")
	if info.Mode != mode {
		t.Errorf("Mode = %o, want %o", info.Mode, mode)
	}
}
