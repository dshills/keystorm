package graph

import "testing"

func setupTestGraph() *MemGraph {
	g := New()

	// Create file nodes
	file1 := NewFileNode("/project/main.go")
	file2 := NewFileNode("/project/util.go")
	file3 := NewFileNode("/project/util_test.go")
	file4 := NewFileNode("/project/helper.go")

	// Create package node
	pkg := NewPackageNode("/project", "main", "go")

	// Add nodes
	_ = g.AddNode(file1)
	_ = g.AddNode(file2)
	_ = g.AddNode(file3)
	_ = g.AddNode(file4)
	_ = g.AddNode(pkg)

	// main.go imports util.go
	_ = g.AddEdge(NewImportEdge(file1.ID, file2.ID, "util", nil))

	// util.go imports helper.go
	_ = g.AddEdge(NewImportEdge(file2.ID, file4.ID, "helper", nil))

	// util_test.go tests util.go
	_ = g.AddEdge(NewTestsEdge(file3.ID, file2.ID))

	// Package contains all files
	_ = g.AddEdge(NewContainsEdge(pkg.ID, file1.ID))
	_ = g.AddEdge(NewContainsEdge(pkg.ID, file2.ID))
	_ = g.AddEdge(NewContainsEdge(pkg.ID, file3.ID))
	_ = g.AddEdge(NewContainsEdge(pkg.ID, file4.ID))

	return g
}

func TestFindRelatedFiles(t *testing.T) {
	g := setupTestGraph()

	related := FindRelatedFiles(g, "/project/main.go", 10)
	if len(related) == 0 {
		t.Error("FindRelatedFiles() should find related files")
	}

	// Should find util.go as a dependency
	foundUtil := false
	for _, r := range related {
		if r.Node.Path == "/project/util.go" {
			foundUtil = true
			if r.Relation != EdgeTypeImports {
				t.Errorf("util.go relation = %v, want EdgeTypeImports", r.Relation)
			}
			break
		}
	}
	if !foundUtil {
		t.Error("Should find util.go as related file")
	}
}

func TestFindRelatedFiles_NotFound(t *testing.T) {
	g := setupTestGraph()

	related := FindRelatedFiles(g, "/nonexistent.go", 10)
	if related != nil {
		t.Error("FindRelatedFiles() for nonexistent should return nil")
	}
}

func TestFindRelatedFiles_MaxResults(t *testing.T) {
	g := setupTestGraph()

	related := FindRelatedFiles(g, "/project/main.go", 1)
	if len(related) > 1 {
		t.Errorf("FindRelatedFiles() with maxResults=1 returned %d results", len(related))
	}
}

func TestFindTestsFor(t *testing.T) {
	g := setupTestGraph()

	tests := FindTestsFor(g, "/project/util.go")
	if len(tests) != 1 {
		t.Errorf("FindTestsFor() len = %d, want 1", len(tests))
	}

	if tests[0].Path != "/project/util_test.go" {
		t.Errorf("FindTestsFor()[0].Path = %q, want /project/util_test.go", tests[0].Path)
	}
}

func TestFindTestsFor_NotFound(t *testing.T) {
	g := setupTestGraph()

	tests := FindTestsFor(g, "/nonexistent.go")
	if len(tests) != 0 {
		t.Errorf("FindTestsFor() for nonexistent len = %d, want 0", len(tests))
	}
}

func TestFindTestsFor_ByConvention(t *testing.T) {
	g := New()

	// Create impl and test files without explicit edge
	impl := NewFileNode("/project/handler.go")
	test := NewFileNode("/project/handler_test.go")

	_ = g.AddNode(impl)
	_ = g.AddNode(test)

	tests := FindTestsFor(g, "/project/handler.go")
	if len(tests) != 1 {
		t.Errorf("FindTestsFor() by convention len = %d, want 1", len(tests))
	}
}

func TestFindImplementationFor(t *testing.T) {
	g := setupTestGraph()

	impls := FindImplementationFor(g, "/project/util_test.go")
	if len(impls) != 1 {
		t.Errorf("FindImplementationFor() len = %d, want 1", len(impls))
	}

	if impls[0].Path != "/project/util.go" {
		t.Errorf("FindImplementationFor()[0].Path = %q, want /project/util.go", impls[0].Path)
	}
}

func TestFindImplementationFor_ByConvention(t *testing.T) {
	g := New()

	// Create impl and test files without explicit edge
	impl := NewFileNode("/project/handler.go")
	test := NewFileNode("/project/handler_test.go")

	_ = g.AddNode(impl)
	_ = g.AddNode(test)

	impls := FindImplementationFor(g, "/project/handler_test.go")
	if len(impls) != 1 {
		t.Errorf("FindImplementationFor() by convention len = %d, want 1", len(impls))
	}
}

func TestFindFilesInSamePackage(t *testing.T) {
	g := setupTestGraph()

	files := FindFilesInSamePackage(g, "/project/main.go")
	// Should find util.go, util_test.go, helper.go
	if len(files) != 3 {
		t.Errorf("FindFilesInSamePackage() len = %d, want 3", len(files))
	}
}

func TestFindFilesInSamePackage_NotFound(t *testing.T) {
	g := setupTestGraph()

	files := FindFilesInSamePackage(g, "/nonexistent.go")
	if len(files) != 0 {
		t.Errorf("FindFilesInSamePackage() for nonexistent len = %d, want 0", len(files))
	}
}

func TestFindImportChain(t *testing.T) {
	g := setupTestGraph()

	// main.go -> util.go -> helper.go
	chain := FindImportChain(g, "/project/main.go", "/project/helper.go")
	if len(chain) != 3 {
		t.Errorf("FindImportChain() len = %d, want 3", len(chain))
	}
}

func TestFindImportChain_NotFound(t *testing.T) {
	g := setupTestGraph()

	chain := FindImportChain(g, "/project/helper.go", "/project/main.go")
	if chain != nil {
		t.Error("FindImportChain() reverse should be nil")
	}
}

func TestGetImports(t *testing.T) {
	g := setupTestGraph()

	imports := GetImports(g, "/project/main.go")
	if len(imports) != 1 {
		t.Errorf("GetImports() len = %d, want 1", len(imports))
	}
}

func TestGetImports_NotFound(t *testing.T) {
	g := setupTestGraph()

	imports := GetImports(g, "/nonexistent.go")
	if imports != nil {
		t.Error("GetImports() for nonexistent should return nil")
	}
}

func TestGetImportedBy(t *testing.T) {
	g := setupTestGraph()

	importers := GetImportedBy(g, "/project/util.go")
	if len(importers) != 1 {
		t.Errorf("GetImportedBy() len = %d, want 1", len(importers))
	}
}

func TestGetAllDependencies(t *testing.T) {
	g := setupTestGraph()

	// main.go -> util.go -> helper.go
	deps := GetAllDependencies(g, "/project/main.go", 10)
	if len(deps) != 2 {
		t.Errorf("GetAllDependencies() len = %d, want 2", len(deps))
	}
}

func TestGetAllDependencies_MaxDepth(t *testing.T) {
	g := setupTestGraph()

	// With depth 1, should only get util.go
	deps := GetAllDependencies(g, "/project/main.go", 1)
	if len(deps) != 1 {
		t.Errorf("GetAllDependencies(depth=1) len = %d, want 1", len(deps))
	}
}

func TestGetAllDependents(t *testing.T) {
	g := setupTestGraph()

	// helper.go <- util.go <- main.go
	deps := GetAllDependents(g, "/project/helper.go", 10)
	if len(deps) != 2 {
		t.Errorf("GetAllDependents() len = %d, want 2", len(deps))
	}
}

func TestFindCycles(t *testing.T) {
	g := New()

	// Create a cycle: a -> b -> c -> a
	nodeA := NewFileNode("/a.go")
	nodeB := NewFileNode("/b.go")
	nodeC := NewFileNode("/c.go")

	_ = g.AddNode(nodeA)
	_ = g.AddNode(nodeB)
	_ = g.AddNode(nodeC)

	_ = g.AddEdge(NewImportEdge(nodeA.ID, nodeB.ID, "b", nil))
	_ = g.AddEdge(NewImportEdge(nodeB.ID, nodeC.ID, "c", nil))
	_ = g.AddEdge(NewImportEdge(nodeC.ID, nodeA.ID, "a", nil))

	cycles := FindCycles(g)
	if len(cycles) == 0 {
		t.Error("FindCycles() should detect the cycle")
	}
}

func TestFindCycles_NoCycles(t *testing.T) {
	g := setupTestGraph()

	cycles := FindCycles(g)
	if len(cycles) != 0 {
		t.Errorf("FindCycles() found %d cycles, want 0", len(cycles))
	}
}

func TestComputeImpact(t *testing.T) {
	g := setupTestGraph()

	// helper.go is imported by util.go, which is imported by main.go
	// So helper.go impacts 2 out of 4 files = 0.5
	impact := ComputeImpact(g, "/project/helper.go")
	if impact < 0.4 || impact > 0.6 {
		t.Errorf("ComputeImpact() = %f, want around 0.5", impact)
	}

	// main.go has no dependents, impact should be 0
	impact = ComputeImpact(g, "/project/main.go")
	if impact != 0 {
		t.Errorf("ComputeImpact(main.go) = %f, want 0", impact)
	}
}

func TestComputeImpact_EmptyGraph(t *testing.T) {
	g := New()

	impact := ComputeImpact(g, "/nonexistent.go")
	if impact != 0 {
		t.Errorf("ComputeImpact() on empty graph = %f, want 0", impact)
	}
}

func TestQueryResult(t *testing.T) {
	// Ensure QueryResult can be constructed
	result := QueryResult{
		Nodes: []Node{NewFileNode("/test.go")},
		Edges: []Edge{NewImportEdge("a", "b", "path", nil)},
	}

	if len(result.Nodes) != 1 {
		t.Errorf("QueryResult.Nodes len = %d, want 1", len(result.Nodes))
	}
	if len(result.Edges) != 1 {
		t.Errorf("QueryResult.Edges len = %d, want 1", len(result.Edges))
	}
}

func TestRelatedFile(t *testing.T) {
	// Ensure RelatedFile can be constructed
	rf := RelatedFile{
		Node:      NewFileNode("/test.go"),
		Relation:  EdgeTypeImports,
		Distance:  1,
		Relevance: 0.9,
	}

	if rf.Distance != 1 {
		t.Errorf("RelatedFile.Distance = %d, want 1", rf.Distance)
	}
	if rf.Relevance != 0.9 {
		t.Errorf("RelatedFile.Relevance = %f, want 0.9", rf.Relevance)
	}
}
