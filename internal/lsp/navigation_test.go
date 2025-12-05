package lsp

import (
	"context"
	"testing"
)

func TestNewNavigationService(t *testing.T) {
	ns := NewNavigationService(nil)
	if ns == nil {
		t.Fatal("NewNavigationService returned nil")
	}

	if ns.maxResults != 100 {
		t.Errorf("Default maxResults: got %d, want 100", ns.maxResults)
	}

	if ns.maxHistory != 100 {
		t.Errorf("Default maxHistory: got %d, want 100", ns.maxHistory)
	}

	if !ns.includeDeclaration {
		t.Error("Default includeDeclaration should be true")
	}

	if !ns.enableSymbolCaching {
		t.Error("Default enableSymbolCaching should be true")
	}

	if !ns.enableLocationCache {
		t.Error("Default enableLocationCache should be true")
	}
}

func TestNavigationServiceOptions(t *testing.T) {
	ns := NewNavigationService(nil,
		WithMaxHistory(50),
		WithMaxNavigationResults(25),
		WithIncludeDeclaration(false),
		WithSymbolCaching(false),
		WithLocationCaching(false),
		WithSymbolCacheMaxAge(120),
		WithLocationCacheMaxAge(60),
	)

	if ns.maxHistory != 50 {
		t.Errorf("maxHistory: got %d, want 50", ns.maxHistory)
	}

	if ns.maxResults != 25 {
		t.Errorf("maxResults: got %d, want 25", ns.maxResults)
	}

	if ns.includeDeclaration {
		t.Error("includeDeclaration should be false")
	}

	if ns.enableSymbolCaching {
		t.Error("enableSymbolCaching should be false")
	}

	if ns.enableLocationCache {
		t.Error("enableLocationCache should be false")
	}

	if ns.symbolCacheMaxAge != 120 {
		t.Errorf("symbolCacheMaxAge: got %d, want 120", ns.symbolCacheMaxAge)
	}

	if ns.locationCacheMaxAge != 60 {
		t.Errorf("locationCacheMaxAge: got %d, want 60", ns.locationCacheMaxAge)
	}
}

func TestNavigationHistory(t *testing.T) {
	ns := NewNavigationService(nil, WithMaxHistory(5))

	// Initially empty
	if ns.CanGoBack() {
		t.Error("CanGoBack should be false when history is empty")
	}
	if ns.CanGoForward() {
		t.Error("CanGoForward should be false when history is empty")
	}

	// Add entries
	ns.PushLocation(Location{
		URI:   "file:///a.go",
		Range: Range{Start: Position{Line: 1, Character: 0}},
	}, "Location A")

	ns.PushLocation(Location{
		URI:   "file:///b.go",
		Range: Range{Start: Position{Line: 2, Character: 0}},
	}, "Location B")

	ns.PushLocation(Location{
		URI:   "file:///c.go",
		Range: Range{Start: Position{Line: 3, Character: 0}},
	}, "Location C")

	// Should be able to go back
	if !ns.CanGoBack() {
		t.Error("CanGoBack should be true")
	}
	if ns.CanGoForward() {
		t.Error("CanGoForward should be false at end of history")
	}

	// Go back
	entry, ok := ns.GoBack()
	if !ok {
		t.Error("GoBack should succeed")
	}
	if entry.Description != "Location B" {
		t.Errorf("GoBack: got %q, want %q", entry.Description, "Location B")
	}

	// Now can go forward
	if !ns.CanGoForward() {
		t.Error("CanGoForward should be true after GoBack")
	}

	// Go forward
	entry, ok = ns.GoForward()
	if !ok {
		t.Error("GoForward should succeed")
	}
	if entry.Description != "Location C" {
		t.Errorf("GoForward: got %q, want %q", entry.Description, "Location C")
	}

	// Can't go forward anymore
	if ns.CanGoForward() {
		t.Error("CanGoForward should be false at end")
	}
}

func TestNavigationHistoryTruncation(t *testing.T) {
	ns := NewNavigationService(nil, WithMaxHistory(3))

	// Add more than max
	for i := 0; i < 5; i++ {
		ns.PushLocation(Location{
			URI:   DocumentURI("file:///test.go"),
			Range: Range{Start: Position{Line: i, Character: 0}},
		}, "")
	}

	history := ns.GetHistory()
	if len(history) != 3 {
		t.Errorf("History length: got %d, want 3", len(history))
	}

	// Should have the last 3 entries (lines 2, 3, 4)
	if history[0].Location.Range.Start.Line != 2 {
		t.Errorf("First entry line: got %d, want 2", history[0].Location.Range.Start.Line)
	}
}

func TestNavigationHistoryBranchTruncation(t *testing.T) {
	ns := NewNavigationService(nil)

	// Add entries
	ns.PushLocation(Location{URI: "file:///a.go"}, "A")
	ns.PushLocation(Location{URI: "file:///b.go"}, "B")
	ns.PushLocation(Location{URI: "file:///c.go"}, "C")

	// Go back twice
	ns.GoBack()
	ns.GoBack()

	// Add new entry - should truncate forward history
	ns.PushLocation(Location{URI: "file:///d.go"}, "D")

	history := ns.GetHistory()
	if len(history) != 2 {
		t.Errorf("History length after branch: got %d, want 2", len(history))
	}

	// Should not be able to go forward
	if ns.CanGoForward() {
		t.Error("CanGoForward should be false after new entry")
	}

	// Last entry should be D
	if history[1].Description != "D" {
		t.Errorf("Last entry: got %q, want %q", history[1].Description, "D")
	}
}

func TestClearHistory(t *testing.T) {
	ns := NewNavigationService(nil)

	ns.PushLocation(Location{URI: "file:///a.go"}, "A")
	ns.PushLocation(Location{URI: "file:///b.go"}, "B")

	ns.ClearHistory()

	if len(ns.GetHistory()) != 0 {
		t.Error("History should be empty after clear")
	}

	if ns.GetCurrentHistoryIndex() != -1 {
		t.Errorf("History index: got %d, want -1", ns.GetCurrentHistoryIndex())
	}
}

func TestContainsPosition(t *testing.T) {
	// Note: LSP ranges have inclusive start and exclusive end
	tests := []struct {
		name string
		r    Range
		pos  Position
		want bool
	}{
		{
			name: "position inside single line",
			r: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
			pos:  Position{Line: 5, Character: 5},
			want: true,
		},
		{
			name: "position at start (inclusive)",
			r: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
			pos:  Position{Line: 5, Character: 0},
			want: true,
		},
		{
			name: "position at end (exclusive - not contained)",
			r: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
			pos:  Position{Line: 5, Character: 10},
			want: false, // End is exclusive in LSP
		},
		{
			name: "position just before end",
			r: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
			pos:  Position{Line: 5, Character: 9},
			want: true,
		},
		{
			name: "position before range",
			r: Range{
				Start: Position{Line: 5, Character: 5},
				End:   Position{Line: 5, Character: 10},
			},
			pos:  Position{Line: 5, Character: 3},
			want: false,
		},
		{
			name: "position after range",
			r: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
			pos:  Position{Line: 5, Character: 15},
			want: false,
		},
		{
			name: "multi-line range, position in middle",
			r: Range{
				Start: Position{Line: 5, Character: 5},
				End:   Position{Line: 10, Character: 10},
			},
			pos:  Position{Line: 7, Character: 0},
			want: true,
		},
		{
			name: "position before start line",
			r: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 10, Character: 10},
			},
			pos:  Position{Line: 3, Character: 5},
			want: false,
		},
		{
			name: "position after end line",
			r: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 10, Character: 10},
			},
			pos:  Position{Line: 15, Character: 5},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsPosition(tt.r, tt.pos)
			if got != tt.want {
				t.Errorf("containsPosition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSymbolKindName(t *testing.T) {
	tests := []struct {
		kind SymbolKind
		want string
	}{
		{SymbolKindFile, "File"},
		{SymbolKindClass, "Class"},
		{SymbolKindFunction, "Function"},
		{SymbolKindMethod, "Method"},
		{SymbolKindVariable, "Variable"},
		{SymbolKindStruct, "Struct"},
		{SymbolKindInterface, "Interface"},
		{SymbolKind(999), "Unknown"},
	}

	for _, tt := range tests {
		got := SymbolKindName(tt.kind)
		if got != tt.want {
			t.Errorf("SymbolKindName(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestSymbolKindIcon(t *testing.T) {
	tests := []struct {
		kind SymbolKind
		want string
	}{
		{SymbolKindFile, "F"},
		{SymbolKindClass, "C"},
		{SymbolKindFunction, "F"},
		{SymbolKindMethod, "m"},
		{SymbolKindVariable, "v"},
		{SymbolKindInterface, "I"},
		{SymbolKind(999), "?"},
	}

	for _, tt := range tests {
		got := SymbolKindIcon(tt.kind)
		if got != tt.want {
			t.Errorf("SymbolKindIcon(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestFormatSymbol(t *testing.T) {
	sym := DocumentSymbol{
		Name: "MyFunction",
		Kind: SymbolKindFunction,
	}

	got := FormatSymbol(sym)
	want := "[F] MyFunction"
	if got != want {
		t.Errorf("FormatSymbol() = %q, want %q", got, want)
	}

	// With detail
	sym.Detail = "func() error"
	got = FormatSymbol(sym)
	want = "[F] MyFunction - func() error"
	if got != want {
		t.Errorf("FormatSymbol() with detail = %q, want %q", got, want)
	}
}

func TestFormatSymbolWithLocation(t *testing.T) {
	sym := DocumentSymbol{
		Name:  "MyFunction",
		Kind:  SymbolKindFunction,
		Range: Range{Start: Position{Line: 9, Character: 0}},
	}

	got := FormatSymbolWithLocation(sym, "/path/to/file.go")
	want := "[F] MyFunction (file.go:10)"
	if got != want {
		t.Errorf("FormatSymbolWithLocation() = %q, want %q", got, want)
	}
}

func TestFormatWorkspaceSymbol(t *testing.T) {
	sym := SymbolInformation{
		Name: "DoSomething",
		Kind: SymbolKindFunction,
		Location: Location{
			URI:   "file:///path/to/file.go",
			Range: Range{Start: Position{Line: 19, Character: 0}},
		},
	}

	got := FormatWorkspaceSymbol(sym)
	want := "[F] DoSomething (file.go:20)"
	if got != want {
		t.Errorf("FormatWorkspaceSymbol() = %q, want %q", got, want)
	}

	// With container
	sym.ContainerName = "MyPackage"
	got = FormatWorkspaceSymbol(sym)
	want = "[F] MyPackage.DoSomething (file.go:20)"
	if got != want {
		t.Errorf("FormatWorkspaceSymbol() with container = %q, want %q", got, want)
	}
}

func TestSortSymbolsByKind(t *testing.T) {
	symbols := []DocumentSymbol{
		{Name: "var1", Kind: SymbolKindVariable},
		{Name: "func1", Kind: SymbolKindFunction},
		{Name: "Class1", Kind: SymbolKindClass},
		{Name: "const1", Kind: SymbolKindConstant},
	}

	SortSymbolsByKind(symbols)

	// Class should come first
	if symbols[0].Kind != SymbolKindClass {
		t.Errorf("First should be Class, got %v", symbols[0].Kind)
	}

	// Function should come before Variable
	funcIdx := -1
	varIdx := -1
	for i, s := range symbols {
		if s.Kind == SymbolKindFunction {
			funcIdx = i
		}
		if s.Kind == SymbolKindVariable {
			varIdx = i
		}
	}

	if funcIdx > varIdx {
		t.Error("Function should come before Variable")
	}
}

func TestSortSymbolsByName(t *testing.T) {
	symbols := []DocumentSymbol{
		{Name: "Zebra"},
		{Name: "apple"},
		{Name: "Banana"},
	}

	SortSymbolsByName(symbols)

	if symbols[0].Name != "apple" {
		t.Errorf("First should be 'apple', got %q", symbols[0].Name)
	}
	if symbols[1].Name != "Banana" {
		t.Errorf("Second should be 'Banana', got %q", symbols[1].Name)
	}
	if symbols[2].Name != "Zebra" {
		t.Errorf("Third should be 'Zebra', got %q", symbols[2].Name)
	}
}

func TestSortSymbolsByPosition(t *testing.T) {
	symbols := []DocumentSymbol{
		{Name: "c", Range: Range{Start: Position{Line: 20, Character: 0}}},
		{Name: "a", Range: Range{Start: Position{Line: 5, Character: 0}}},
		{Name: "b", Range: Range{Start: Position{Line: 10, Character: 0}}},
	}

	SortSymbolsByPosition(symbols)

	if symbols[0].Name != "a" {
		t.Errorf("First should be 'a', got %q", symbols[0].Name)
	}
	if symbols[1].Name != "b" {
		t.Errorf("Second should be 'b', got %q", symbols[1].Name)
	}
	if symbols[2].Name != "c" {
		t.Errorf("Third should be 'c', got %q", symbols[2].Name)
	}
}

func TestFlattenSymbols(t *testing.T) {
	symbols := []DocumentSymbol{
		{
			Name: "Class1",
			Kind: SymbolKindClass,
			Children: []DocumentSymbol{
				{Name: "Method1", Kind: SymbolKindMethod},
				{
					Name: "Method2",
					Kind: SymbolKindMethod,
					Children: []DocumentSymbol{
						{Name: "LocalVar", Kind: SymbolKindVariable},
					},
				},
			},
		},
		{Name: "Func1", Kind: SymbolKindFunction},
	}

	flat := FlattenSymbols(symbols)

	if len(flat) != 5 {
		t.Errorf("Flattened length: got %d, want 5", len(flat))
	}

	// Check order (should be depth-first)
	names := make([]string, len(flat))
	for i, s := range flat {
		names[i] = s.Name
	}

	expected := []string{"Class1", "Method1", "Method2", "LocalVar", "Func1"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("Position %d: got %q, want %q", i, names[i], name)
		}
	}
}

func TestFilterSymbolsByKind(t *testing.T) {
	symbols := []DocumentSymbol{
		{Name: "class1", Kind: SymbolKindClass},
		{Name: "func1", Kind: SymbolKindFunction},
		{Name: "class2", Kind: SymbolKindClass},
		{Name: "var1", Kind: SymbolKindVariable},
	}

	filtered := FilterSymbolsByKind(symbols, SymbolKindClass, SymbolKindFunction)

	if len(filtered) != 3 {
		t.Errorf("Filtered length: got %d, want 3", len(filtered))
	}

	for _, s := range filtered {
		if s.Kind != SymbolKindClass && s.Kind != SymbolKindFunction {
			t.Errorf("Unexpected kind: %v", s.Kind)
		}
	}
}

func TestGetSymbolsOfKind(t *testing.T) {
	symbols := []DocumentSymbol{
		{
			Name: "Class1",
			Kind: SymbolKindClass,
			Children: []DocumentSymbol{
				{Name: "Method1", Kind: SymbolKindMethod},
				{Name: "Method2", Kind: SymbolKindMethod},
			},
		},
		{Name: "Method3", Kind: SymbolKindMethod},
	}

	methods := GetSymbolsOfKind(symbols, SymbolKindMethod)

	if len(methods) != 3 {
		t.Errorf("Methods count: got %d, want 3", len(methods))
	}
}

func TestGroupSymbolsByKind(t *testing.T) {
	symbols := []DocumentSymbol{
		{Name: "class1", Kind: SymbolKindClass},
		{Name: "func1", Kind: SymbolKindFunction},
		{Name: "class2", Kind: SymbolKindClass},
		{Name: "var1", Kind: SymbolKindVariable},
	}

	groups := GroupSymbolsByKind(symbols)

	if len(groups[SymbolKindClass]) != 2 {
		t.Errorf("Classes count: got %d, want 2", len(groups[SymbolKindClass]))
	}

	if len(groups[SymbolKindFunction]) != 1 {
		t.Errorf("Functions count: got %d, want 1", len(groups[SymbolKindFunction]))
	}

	if len(groups[SymbolKindVariable]) != 1 {
		t.Errorf("Variables count: got %d, want 1", len(groups[SymbolKindVariable]))
	}
}

func TestNavigationService_InvalidateCache(t *testing.T) {
	ns := NewNavigationService(nil)

	// Manually add cache entries
	uri1 := FilePathToURI("/test/a.go")
	uri2 := FilePathToURI("/test/b.go")

	ns.documentSymbols[uri1] = &symbolCache{}
	ns.documentSymbols[uri2] = &symbolCache{}
	ns.definitionCache[definitionKey{uri: uri1, line: 1, char: 0}] = &definitionCacheEntry{}
	ns.definitionCache[definitionKey{uri: uri1, line: 2, char: 0}] = &definitionCacheEntry{}
	ns.definitionCache[definitionKey{uri: uri2, line: 1, char: 0}] = &definitionCacheEntry{}

	ns.InvalidateCache("/test/a.go")

	// Check that a.go caches are removed
	if _, ok := ns.documentSymbols[uri1]; ok {
		t.Error("Symbol cache for a.go should be removed")
	}

	// Check that b.go caches remain
	if _, ok := ns.documentSymbols[uri2]; !ok {
		t.Error("Symbol cache for b.go should remain")
	}

	// Check definition cache
	for key := range ns.definitionCache {
		if key.uri == uri1 {
			t.Error("Definition cache for a.go should be removed")
		}
	}
}

func TestNavigationService_InvalidateAllCaches(t *testing.T) {
	ns := NewNavigationService(nil)

	// Add cache entries
	ns.documentSymbols[FilePathToURI("/test/a.go")] = &symbolCache{}
	ns.documentSymbols[FilePathToURI("/test/b.go")] = &symbolCache{}
	ns.definitionCache[definitionKey{}] = &definitionCacheEntry{}

	ns.InvalidateAllCaches()

	if len(ns.documentSymbols) != 0 {
		t.Errorf("Symbol cache should be empty, got %d entries", len(ns.documentSymbols))
	}

	if len(ns.definitionCache) != 0 {
		t.Errorf("Definition cache should be empty, got %d entries", len(ns.definitionCache))
	}
}

func TestNavigationService_BuildResult(t *testing.T) {
	ns := NewNavigationService(nil, WithMaxNavigationResults(2))

	locations := []Location{
		{URI: "file:///a.go", Range: Range{Start: Position{Line: 0}}},
		{URI: "file:///b.go", Range: Range{Start: Position{Line: 1}}},
		{URI: "file:///c.go", Range: Range{Start: Position{Line: 2}}},
	}

	result := ns.buildResult(locations)

	if result.TotalCount != 3 {
		t.Errorf("TotalCount: got %d, want 3", result.TotalCount)
	}

	if len(result.Locations) != 2 {
		t.Errorf("Locations length: got %d, want 2", len(result.Locations))
	}

	if !result.Truncated {
		t.Error("Truncated should be true")
	}

	if result.Primary == nil {
		t.Fatal("Primary should not be nil")
	}

	if result.Primary.URI != "file:///a.go" {
		t.Errorf("Primary URI: got %q, want %q", result.Primary.URI, "file:///a.go")
	}

	if len(result.FormattedLocations) != 2 {
		t.Errorf("FormattedLocations length: got %d, want 2", len(result.FormattedLocations))
	}
}

func TestNavigationService_BuildResultEmpty(t *testing.T) {
	ns := NewNavigationService(nil)

	result := ns.buildResult(nil)

	if result.TotalCount != 0 {
		t.Errorf("TotalCount: got %d, want 0", result.TotalCount)
	}

	if result.Primary != nil {
		t.Error("Primary should be nil for empty result")
	}

	if result.Truncated {
		t.Error("Truncated should be false for empty result")
	}
}

func TestNavigationService_FormatLocation(t *testing.T) {
	ns := NewNavigationService(nil)

	loc := Location{
		URI: "file:///path/to/file.go",
		Range: Range{
			Start: Position{Line: 9, Character: 4},
		},
	}

	formatted := ns.formatLocation(loc)

	if formatted.FilePath != "/path/to/file.go" {
		t.Errorf("FilePath: got %q, want %q", formatted.FilePath, "/path/to/file.go")
	}

	// Display should be in format path:line:char (1-indexed)
	expectedDisplay := "/path/to/file.go:10:5"
	if formatted.Display != expectedDisplay {
		t.Errorf("Display: got %q, want %q", formatted.Display, expectedDisplay)
	}
}

func TestFindSymbolAtPosition(t *testing.T) {
	ns := NewNavigationService(nil)

	symbols := []DocumentSymbol{
		{
			Name:  "Outer",
			Kind:  SymbolKindClass,
			Range: Range{Start: Position{Line: 0}, End: Position{Line: 20}},
			Children: []DocumentSymbol{
				{
					Name:  "Inner",
					Kind:  SymbolKindMethod,
					Range: Range{Start: Position{Line: 5}, End: Position{Line: 15}},
				},
			},
		},
		{
			Name:  "Standalone",
			Kind:  SymbolKindFunction,
			Range: Range{Start: Position{Line: 25}, End: Position{Line: 30}},
		},
	}

	// Position in Inner
	sym := ns.findSymbolAtPosition(symbols, Position{Line: 10, Character: 0})
	if sym == nil {
		t.Fatal("Should find Inner symbol")
	}
	if sym.Name != "Inner" {
		t.Errorf("Found: %q, want %q", sym.Name, "Inner")
	}

	// Position in Outer but not Inner
	sym = ns.findSymbolAtPosition(symbols, Position{Line: 3, Character: 0})
	if sym == nil {
		t.Fatal("Should find Outer symbol")
	}
	if sym.Name != "Outer" {
		t.Errorf("Found: %q, want %q", sym.Name, "Outer")
	}

	// Position in Standalone
	sym = ns.findSymbolAtPosition(symbols, Position{Line: 27, Character: 0})
	if sym == nil {
		t.Fatal("Should find Standalone symbol")
	}
	if sym.Name != "Standalone" {
		t.Errorf("Found: %q, want %q", sym.Name, "Standalone")
	}

	// Position outside all symbols
	sym = ns.findSymbolAtPosition(symbols, Position{Line: 50, Character: 0})
	if sym != nil {
		t.Error("Should not find any symbol")
	}
}

func TestFilterSymbols(t *testing.T) {
	ns := NewNavigationService(nil)

	symbols := []DocumentSymbol{
		{Name: "GetUser"},
		{Name: "SetUser"},
		{Name: "DeleteUser"},
		{Name: "ProcessData"},
	}

	// Simple contains
	filtered := ns.filterSymbols(symbols, "User")
	if len(filtered) != 3 {
		t.Errorf("Filtered by 'User': got %d, want 3", len(filtered))
	}

	// Case insensitive
	filtered = ns.filterSymbols(symbols, "user")
	if len(filtered) != 3 {
		t.Errorf("Filtered by 'user' (case insensitive): got %d, want 3", len(filtered))
	}

	// Empty pattern
	filtered = ns.filterSymbols(symbols, "")
	if len(filtered) != 4 {
		t.Errorf("Filtered by empty pattern: got %d, want 4", len(filtered))
	}

	// Regex pattern
	filtered = ns.filterSymbols(symbols, "^Get.*")
	if len(filtered) != 1 {
		t.Errorf("Filtered by regex: got %d, want 1", len(filtered))
	}
}

func TestSymbolTree(t *testing.T) {
	ns := NewNavigationService(nil)

	symbols := []DocumentSymbol{
		{
			Name: "Class1",
			Kind: SymbolKindClass,
			Children: []DocumentSymbol{
				{Name: "Method1", Kind: SymbolKindMethod},
				{Name: "Method2", Kind: SymbolKindMethod},
			},
		},
		{Name: "Func1", Kind: SymbolKindFunction},
	}

	tree := ns.buildSymbolTree("file:///test.go", "/test.go", symbols)

	if len(tree.Roots) != 2 {
		t.Errorf("Roots count: got %d, want 2", len(tree.Roots))
	}

	if len(tree.All) != 4 {
		t.Errorf("All count: got %d, want 4", len(tree.All))
	}

	// Check tree structure
	classNode := tree.Roots[0]
	if classNode.Symbol.Name != "Class1" {
		t.Errorf("First root name: got %q, want %q", classNode.Symbol.Name, "Class1")
	}

	if len(classNode.Children) != 2 {
		t.Errorf("Class1 children: got %d, want 2", len(classNode.Children))
	}

	// Check parent references
	for _, child := range classNode.Children {
		if child.Parent != classNode {
			t.Error("Child should have Class1 as parent")
		}
		if child.Depth != 1 {
			t.Errorf("Child depth: got %d, want 1", child.Depth)
		}
	}
}

func TestNavigationService_GoToDefinitionNoServer(t *testing.T) {
	ns := NewNavigationService(nil) // No manager

	_, err := ns.GoToDefinition(context.Background(), "/test.go", Position{})
	if err != ErrNoServerForFile {
		t.Errorf("Expected ErrNoServerForFile, got %v", err)
	}
}

func TestFindParentSymbol(t *testing.T) {
	symbols := []DocumentSymbol{
		{
			Name:  "Outer",
			Range: Range{Start: Position{Line: 0}, End: Position{Line: 20}},
			Children: []DocumentSymbol{
				{
					Name:  "Inner",
					Range: Range{Start: Position{Line: 5}, End: Position{Line: 15}},
				},
			},
		},
	}

	// Find innermost
	sym := FindParentSymbol(symbols, Position{Line: 10})
	if sym == nil || sym.Name != "Inner" {
		t.Error("Should find Inner as parent")
	}

	// Find outer
	sym = FindParentSymbol(symbols, Position{Line: 2})
	if sym == nil || sym.Name != "Outer" {
		t.Error("Should find Outer as parent")
	}

	// Outside all
	sym = FindParentSymbol(symbols, Position{Line: 50})
	if sym != nil {
		t.Error("Should not find parent outside range")
	}
}

func TestSymbolContains(t *testing.T) {
	sym := DocumentSymbol{
		Name:  "Test",
		Range: Range{Start: Position{Line: 5}, End: Position{Line: 10}},
	}

	if !SymbolContains(sym, Position{Line: 7}) {
		t.Error("Should contain position inside range")
	}

	if SymbolContains(sym, Position{Line: 3}) {
		t.Error("Should not contain position before range")
	}

	if SymbolContains(sym, Position{Line: 15}) {
		t.Error("Should not contain position after range")
	}
}
