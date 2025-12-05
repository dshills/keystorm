package lsp

import (
	"testing"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		text    string
		pattern string
		want    bool
	}{
		// Empty pattern matches everything
		{"hello", "", true},
		{"", "", true},

		// Exact match
		{"hello", "hello", true},
		{"Hello", "hello", true},

		// Prefix match
		{"hello", "hel", true},
		{"HelloWorld", "Hel", true},

		// Contains
		{"hello", "ell", true},
		{"GetDocument", "doc", true},

		// Fuzzy match
		{"GetDocument", "gd", true},
		{"GetDocument", "gdoc", true},
		{"getUserName", "gun", true},
		{"my_variable_name", "mvn", true},

		// No match
		{"hello", "xyz", false},
	}

	for _, tt := range tests {
		got := FuzzyMatch(tt.text, tt.pattern)
		if got != tt.want {
			t.Errorf("FuzzyMatch(%q, %q) = %v, want %v", tt.text, tt.pattern, got, tt.want)
		}
	}
}

func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		text    string
		pattern string
	}{
		// Exact match should have highest score
		{"hello", "hello"},
		// Prefix should score higher than substring
		{"hello", "hel"},
		{"foobar", "oo"},
	}

	// Exact match should score higher than prefix
	exactScore := FuzzyScore("hello", "hello")
	prefixScore := FuzzyScore("hello", "hel")
	if exactScore <= prefixScore {
		t.Errorf("Exact match score (%d) should be higher than prefix score (%d)", exactScore, prefixScore)
	}

	// Prefix match should score higher than substring
	substringScore := FuzzyScore("foobar", "oo")
	if prefixScore <= substringScore {
		t.Errorf("Prefix score (%d) should be higher than substring score (%d)", prefixScore, substringScore)
	}

	// Boundary matching
	boundaryScore := FuzzyScore("GetDocument", "gd")
	if boundaryScore <= 0 {
		t.Errorf("Boundary match should have positive score, got %d", boundaryScore)
	}

	_ = tests // Silence unused variable
}

func TestMatchesBoundaries(t *testing.T) {
	tests := []struct {
		text    string
		pattern string
		want    bool
	}{
		// CamelCase
		{"GetDocument", "gd", true},
		{"GetDocument", "GD", true},
		{"getUserName", "gun", true},

		// snake_case
		{"get_document", "gd", true},
		{"my_long_variable", "mlv", true},

		// Mixed
		{"getUser_name", "gun", true},

		// No match
		{"GetDocument", "xyz", false},
		{"hello", "hel", false}, // Not a boundary match, just prefix
	}

	for _, tt := range tests {
		got := matchesBoundaries(tt.text, tt.pattern)
		if got != tt.want {
			t.Errorf("matchesBoundaries(%q, %q) = %v, want %v", tt.text, tt.pattern, got, tt.want)
		}
	}
}

func TestExtractBoundaries(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"GetDocument", "GD"},
		{"getUserName", "gUN"},
		{"HTTPServer", "H"}, // All caps - only first char is a boundary
		{"get_document", "gd"},
		{"my_long_variable", "mlv"},
		{"simple", "s"},
		{"", ""},
	}

	for _, tt := range tests {
		got := string(extractBoundaries(tt.text))
		if got != tt.want {
			t.Errorf("extractBoundaries(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestFilterCompletions(t *testing.T) {
	items := []CompletionItem{
		{Label: "GetDocument", Kind: CompletionItemKindFunction},
		{Label: "GetUser", Kind: CompletionItemKindFunction},
		{Label: "SetDocument", Kind: CompletionItemKindFunction},
		{Label: "document", Kind: CompletionItemKindVariable},
	}

	// Filter by "get"
	filtered := FilterCompletions(items, "get")
	if len(filtered) != 2 {
		t.Errorf("FilterCompletions with 'get': got %d items, want 2", len(filtered))
	}

	// Filter by "doc" - matches GetDocument, SetDocument, and document via fuzzy match
	filtered = FilterCompletions(items, "doc")
	if len(filtered) != 3 {
		t.Errorf("FilterCompletions with 'doc': got %d items, want 3", len(filtered))
	}

	// Filter with FilterText
	itemsWithFilter := []CompletionItem{
		{Label: "Display Label", FilterText: "actual_filter"},
		{Label: "Another", FilterText: "different"},
	}

	filtered = FilterCompletions(itemsWithFilter, "actual")
	if len(filtered) != 1 {
		t.Errorf("FilterCompletions with FilterText: got %d items, want 1", len(filtered))
	}

	// Empty prefix returns all
	filtered = FilterCompletions(items, "")
	if len(filtered) != len(items) {
		t.Errorf("FilterCompletions with empty prefix: got %d items, want %d", len(filtered), len(items))
	}
}

func TestSortCompletions(t *testing.T) {
	items := []CompletionItem{
		{Label: "zzz", Kind: CompletionItemKindText},
		{Label: "aaa", Kind: CompletionItemKindFunction},
		{Label: "bbb", Kind: CompletionItemKindFunction, Preselect: true},
		{Label: "ccc", Kind: CompletionItemKindKeyword},
	}

	sorted := SortCompletions(items, "")

	// Preselected should be first
	if sorted[0].Label != "bbb" {
		t.Errorf("First item should be preselected 'bbb', got %q", sorted[0].Label)
	}

	// Functions should come before keywords and text
	functionIdx := -1
	keywordIdx := -1
	textIdx := -1
	for i, item := range sorted {
		switch item.Kind {
		case CompletionItemKindFunction:
			if functionIdx == -1 || i < functionIdx {
				functionIdx = i
			}
		case CompletionItemKindKeyword:
			keywordIdx = i
		case CompletionItemKindText:
			textIdx = i
		}
	}

	if keywordIdx < functionIdx {
		t.Error("Functions should come before keywords")
	}
	if textIdx < keywordIdx {
		t.Error("Keywords should come before text")
	}
}

func TestSortCompletions_PrefixMatch(t *testing.T) {
	items := []CompletionItem{
		{Label: "getString", Kind: CompletionItemKindFunction},
		{Label: "getNumber", Kind: CompletionItemKindFunction},
		{Label: "ageGetter", Kind: CompletionItemKindFunction},
	}

	sorted := SortCompletions(items, "get")

	// Items starting with "get" should come first
	if !hasPrefix(sorted[0].Label, "get") {
		t.Errorf("First item should start with 'get', got %q", sorted[0].Label)
	}
	if !hasPrefix(sorted[1].Label, "get") {
		t.Errorf("Second item should start with 'get', got %q", sorted[1].Label)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestCompletionItemKindString(t *testing.T) {
	tests := []struct {
		kind CompletionItemKind
		want string
	}{
		{CompletionItemKindFunction, "Function"},
		{CompletionItemKindMethod, "Method"},
		{CompletionItemKindVariable, "Variable"},
		{CompletionItemKindClass, "Class"},
		{CompletionItemKindKeyword, "Keyword"},
		{CompletionItemKind(999), "Unknown"},
	}

	for _, tt := range tests {
		got := CompletionItemKindString(tt.kind)
		if got != tt.want {
			t.Errorf("CompletionItemKindString(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestCompletionItemKindIcon(t *testing.T) {
	tests := []struct {
		kind CompletionItemKind
		want string
	}{
		{CompletionItemKindFunction, "f"},
		{CompletionItemKindMethod, "m"},
		{CompletionItemKindVariable, "v"},
		{CompletionItemKindClass, "C"},
		{CompletionItemKindKeyword, "k"},
	}

	for _, tt := range tests {
		got := CompletionItemKindIcon(tt.kind)
		if got != tt.want {
			t.Errorf("CompletionItemKindIcon(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestGetInsertText(t *testing.T) {
	// With TextEdit
	item1 := CompletionItem{
		Label:      "label",
		InsertText: "insertText",
		TextEdit:   &TextEdit{NewText: "textEditText"},
	}
	if got := GetInsertText(item1); got != "textEditText" {
		t.Errorf("GetInsertText with TextEdit: got %q, want %q", got, "textEditText")
	}

	// With InsertText only
	item2 := CompletionItem{
		Label:      "label",
		InsertText: "insertText",
	}
	if got := GetInsertText(item2); got != "insertText" {
		t.Errorf("GetInsertText with InsertText: got %q, want %q", got, "insertText")
	}

	// With Label only
	item3 := CompletionItem{
		Label: "label",
	}
	if got := GetInsertText(item3); got != "label" {
		t.Errorf("GetInsertText with Label only: got %q, want %q", got, "label")
	}
}

func TestIsSnippet(t *testing.T) {
	item1 := CompletionItem{InsertTextFormat: InsertTextFormatSnippet}
	if !IsSnippet(item1) {
		t.Error("IsSnippet should return true for snippet format")
	}

	item2 := CompletionItem{InsertTextFormat: InsertTextFormatPlainText}
	if IsSnippet(item2) {
		t.Error("IsSnippet should return false for plain text format")
	}

	item3 := CompletionItem{} // Default (0)
	if IsSnippet(item3) {
		t.Error("IsSnippet should return false for default format")
	}
}

func TestExpandSnippet(t *testing.T) {
	tests := []struct {
		snippet string
		want    string
	}{
		// Simple tabstops
		{"func $1() {}", "func () {}"},
		{"$1 + $2", " + "},

		// Tabstops with defaults
		{"func ${1:name}() {}", "func name() {}"},
		{"${1:Hello} ${2:World}", "Hello World"},

		// Nested/complex
		{"if ${1:condition} { $2 }", "if condition {  }"},

		// No placeholders
		{"plain text", "plain text"},

		// Mixed
		{"for ${1:i} := 0; $1 < ${2:n}; $1++ {}", "for i := 0;  < n; ++ {}"},
	}

	for _, tt := range tests {
		got := ExpandSnippet(tt.snippet)
		if got != tt.want {
			t.Errorf("ExpandSnippet(%q) = %q, want %q", tt.snippet, got, tt.want)
		}
	}
}

func TestNewCompletionService(t *testing.T) {
	cs := NewCompletionService(nil)
	if cs == nil {
		t.Fatal("NewCompletionService returned nil")
	}

	if cs.maxResults != 100 {
		t.Errorf("Default maxResults: got %d, want 100", cs.maxResults)
	}

	// With options
	cs = NewCompletionService(nil,
		WithMaxResults(50),
		WithCacheTimeout(10),
		WithPrefetchOnType(false),
	)

	if cs.maxResults != 50 {
		t.Errorf("Custom maxResults: got %d, want 50", cs.maxResults)
	}

	if cs.prefetchOnType {
		t.Error("prefetchOnType should be false")
	}
}

func TestCompletionService_InvalidateCache(t *testing.T) {
	cs := NewCompletionService(nil)

	// Manually add cache entries
	cs.cache[cacheKey{path: "/test/a.go", line: 1}] = &cachedCompletion{}
	cs.cache[cacheKey{path: "/test/a.go", line: 2}] = &cachedCompletion{}
	cs.cache[cacheKey{path: "/test/b.go", line: 1}] = &cachedCompletion{}

	cs.InvalidateCache("/test/a.go")

	if len(cs.cache) != 1 {
		t.Errorf("After InvalidateCache: got %d entries, want 1", len(cs.cache))
	}

	// The remaining entry should be for b.go
	for key := range cs.cache {
		if key.path != "/test/b.go" {
			t.Errorf("Remaining cache entry path: got %q, want %q", key.path, "/test/b.go")
		}
	}
}

func TestCompletionService_ClearCache(t *testing.T) {
	cs := NewCompletionService(nil)

	cs.cache[cacheKey{path: "/test/a.go"}] = &cachedCompletion{}
	cs.cache[cacheKey{path: "/test/b.go"}] = &cachedCompletion{}

	cs.ClearCache()

	if len(cs.cache) != 0 {
		t.Errorf("After ClearCache: got %d entries, want 0", len(cs.cache))
	}
}

func TestCompletionResult_ProcessResults(t *testing.T) {
	cs := NewCompletionService(nil, WithMaxResults(2))

	list := &CompletionList{
		Items: []CompletionItem{
			{Label: "aaa"},
			{Label: "bbb"},
			{Label: "ccc"},
		},
		IsIncomplete: false,
	}

	result := cs.processResults(list, "")

	if len(result.Items) != 2 {
		t.Errorf("processResults should limit to maxResults: got %d, want 2", len(result.Items))
	}

	if !result.IsIncomplete {
		t.Error("IsIncomplete should be true when results were truncated")
	}

	if result.ServerTotalCount != 3 {
		t.Errorf("ServerTotalCount: got %d, want 3", result.ServerTotalCount)
	}
}

func TestCompletionResult_ProcessResultsWithFilter(t *testing.T) {
	cs := NewCompletionService(nil)

	list := &CompletionList{
		Items: []CompletionItem{
			{Label: "GetDocument"},
			{Label: "GetUser"},
			{Label: "SetDocument"},
		},
	}

	result := cs.processResults(list, "get")

	if len(result.Items) != 2 {
		t.Errorf("processResults with filter: got %d items, want 2", len(result.Items))
	}
}

func TestCompletionResult_Empty(t *testing.T) {
	cs := NewCompletionService(nil)

	// Nil list
	result := cs.processResults(nil, "")
	if result.Items != nil {
		t.Error("processResults(nil) should return nil items")
	}

	// Empty list
	result = cs.processResults(&CompletionList{}, "")
	if result.Items != nil {
		t.Error("processResults(empty) should return nil items")
	}
}
