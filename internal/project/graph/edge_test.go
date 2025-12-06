package graph

import "testing"

func TestEdgeType_String(t *testing.T) {
	tests := []struct {
		edgeType EdgeType
		want     string
	}{
		{EdgeTypeImports, "imports"},
		{EdgeTypeExports, "exports"},
		{EdgeTypeCalls, "calls"},
		{EdgeTypeExtends, "extends"},
		{EdgeTypeImplements, "implements"},
		{EdgeTypeTests, "tests"},
		{EdgeTypeContains, "contains"},
		{EdgeTypeDependsOn, "depends_on"},
		{EdgeTypeReferences, "references"},
		{EdgeType(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.edgeType.String(); got != tt.want {
			t.Errorf("EdgeType(%d).String() = %q, want %q", tt.edgeType, got, tt.want)
		}
	}
}

func TestNewImportEdge(t *testing.T) {
	edge := NewImportEdge("file1", "file2", "path/to/pkg", []string{"Foo", "Bar"})

	if edge.Type != EdgeTypeImports {
		t.Errorf("Type = %v, want EdgeTypeImports", edge.Type)
	}
	if edge.From != "file1" {
		t.Errorf("From = %q, want file1", edge.From)
	}
	if edge.To != "file2" {
		t.Errorf("To = %q, want file2", edge.To)
	}
	if edge.Metadata.ImportPath != "path/to/pkg" {
		t.Errorf("ImportPath = %q, want path/to/pkg", edge.Metadata.ImportPath)
	}
	if len(edge.Metadata.Symbols) != 2 {
		t.Errorf("Symbols len = %d, want 2", len(edge.Metadata.Symbols))
	}
}

func TestNewCallsEdge(t *testing.T) {
	callSites := []Location{
		{Path: "/file.go", Line: 10, Column: 5},
		{Path: "/file.go", Line: 20, Column: 5},
	}
	edge := NewCallsEdge("func1", "func2", callSites)

	if edge.Type != EdgeTypeCalls {
		t.Errorf("Type = %v, want EdgeTypeCalls", edge.Type)
	}
	if len(edge.Metadata.CallSites) != 2 {
		t.Errorf("CallSites len = %d, want 2", len(edge.Metadata.CallSites))
	}
	if edge.Weight != 2.0 {
		t.Errorf("Weight = %f, want 2.0 (len of call sites)", edge.Weight)
	}
}

func TestNewExtendsEdge(t *testing.T) {
	edge := NewExtendsEdge("childClass", "parentClass")

	if edge.Type != EdgeTypeExtends {
		t.Errorf("Type = %v, want EdgeTypeExtends", edge.Type)
	}
	if edge.From != "childClass" {
		t.Errorf("From = %q, want childClass", edge.From)
	}
	if edge.To != "parentClass" {
		t.Errorf("To = %q, want parentClass", edge.To)
	}
}

func TestNewImplementsEdge(t *testing.T) {
	edge := NewImplementsEdge("concreteType", "interfaceType")

	if edge.Type != EdgeTypeImplements {
		t.Errorf("Type = %v, want EdgeTypeImplements", edge.Type)
	}
	if edge.From != "concreteType" {
		t.Errorf("From = %q, want concreteType", edge.From)
	}
	if edge.To != "interfaceType" {
		t.Errorf("To = %q, want interfaceType", edge.To)
	}
}

func TestNewTestsEdge(t *testing.T) {
	edge := NewTestsEdge("testFile", "implFile")

	if edge.Type != EdgeTypeTests {
		t.Errorf("Type = %v, want EdgeTypeTests", edge.Type)
	}
	if edge.From != "testFile" {
		t.Errorf("From = %q, want testFile", edge.From)
	}
	if edge.To != "implFile" {
		t.Errorf("To = %q, want implFile", edge.To)
	}
}

func TestNewContainsEdge(t *testing.T) {
	edge := NewContainsEdge("parent", "child")

	if edge.Type != EdgeTypeContains {
		t.Errorf("Type = %v, want EdgeTypeContains", edge.Type)
	}
	if edge.From != "parent" {
		t.Errorf("From = %q, want parent", edge.From)
	}
	if edge.To != "child" {
		t.Errorf("To = %q, want child", edge.To)
	}
}

func TestNewDependsOnEdge(t *testing.T) {
	edge := NewDependsOnEdge("module1", "module2")

	if edge.Type != EdgeTypeDependsOn {
		t.Errorf("Type = %v, want EdgeTypeDependsOn", edge.Type)
	}
	if edge.From != "module1" {
		t.Errorf("From = %q, want module1", edge.From)
	}
	if edge.To != "module2" {
		t.Errorf("To = %q, want module2", edge.To)
	}
}

func TestNewReferencesEdge(t *testing.T) {
	edge := NewReferencesEdge("file1", "file2")

	if edge.Type != EdgeTypeReferences {
		t.Errorf("Type = %v, want EdgeTypeReferences", edge.Type)
	}
	if edge.From != "file1" {
		t.Errorf("From = %q, want file1", edge.From)
	}
	if edge.To != "file2" {
		t.Errorf("To = %q, want file2", edge.To)
	}
}

func TestEdge_IsImportEdge(t *testing.T) {
	importEdge := NewImportEdge("a", "b", "path", nil)
	containsEdge := NewContainsEdge("a", "b")

	if !importEdge.IsImportEdge() {
		t.Error("import edge should return true for IsImportEdge")
	}
	if containsEdge.IsImportEdge() {
		t.Error("contains edge should return false for IsImportEdge")
	}
}

func TestEdge_IsContainsEdge(t *testing.T) {
	importEdge := NewImportEdge("a", "b", "path", nil)
	containsEdge := NewContainsEdge("a", "b")

	if importEdge.IsContainsEdge() {
		t.Error("import edge should return false for IsContainsEdge")
	}
	if !containsEdge.IsContainsEdge() {
		t.Error("contains edge should return true for IsContainsEdge")
	}
}

func TestEdge_IsTestEdge(t *testing.T) {
	testEdge := NewTestsEdge("a", "b")
	importEdge := NewImportEdge("a", "b", "path", nil)

	if !testEdge.IsTestEdge() {
		t.Error("test edge should return true for IsTestEdge")
	}
	if importEdge.IsTestEdge() {
		t.Error("import edge should return false for IsTestEdge")
	}
}

func TestLocation(t *testing.T) {
	loc := Location{
		Path:   "/path/to/file.go",
		Line:   42,
		Column: 10,
	}

	if loc.Path != "/path/to/file.go" {
		t.Errorf("Path = %q, want /path/to/file.go", loc.Path)
	}
	if loc.Line != 42 {
		t.Errorf("Line = %d, want 42", loc.Line)
	}
	if loc.Column != 10 {
		t.Errorf("Column = %d, want 10", loc.Column)
	}
}

func TestEdge_IsValid(t *testing.T) {
	tests := []struct {
		name string
		edge Edge
		want bool
	}{
		{"valid edge", NewImportEdge("a", "b", "path", nil), true},
		{"empty from", Edge{From: "", To: "b", Type: EdgeTypeImports}, false},
		{"empty to", Edge{From: "a", To: "", Type: EdgeTypeImports}, false},
		{"both empty", Edge{From: "", To: "", Type: EdgeTypeImports}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.edge.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEdgeMeta(t *testing.T) {
	meta := EdgeMeta{
		ImportPath: "github.com/user/pkg",
		Symbols:    []string{"Foo", "Bar"},
		CallSites: []Location{
			{Path: "/file.go", Line: 10, Column: 5},
		},
		Extra: map[string]any{
			"custom": "value",
		},
	}

	if meta.ImportPath != "github.com/user/pkg" {
		t.Errorf("ImportPath = %q, want github.com/user/pkg", meta.ImportPath)
	}
	if len(meta.Symbols) != 2 {
		t.Errorf("Symbols len = %d, want 2", len(meta.Symbols))
	}
	if len(meta.CallSites) != 1 {
		t.Errorf("CallSites len = %d, want 1", len(meta.CallSites))
	}
	if meta.Extra["custom"] != "value" {
		t.Errorf("Extra[custom] = %v, want value", meta.Extra["custom"])
	}
}
