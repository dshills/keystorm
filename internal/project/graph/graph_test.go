package graph

import (
	"bytes"
	"testing"
)

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Fatal("New() returned nil")
	}
	if g.NodeCount() != 0 {
		t.Errorf("NodeCount() = %d, want 0", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0", g.EdgeCount())
	}
}

func TestMemGraph_AddNode(t *testing.T) {
	g := New()
	node := NewFileNode("/path/to/file.go")

	err := g.AddNode(node)
	if err != nil {
		t.Fatalf("AddNode() error = %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("NodeCount() = %d, want 1", g.NodeCount())
	}

	// Adding duplicate should fail
	err = g.AddNode(node)
	if err != ErrNodeExists {
		t.Errorf("AddNode() duplicate error = %v, want ErrNodeExists", err)
	}
}

func TestMemGraph_AddNode_InvalidID(t *testing.T) {
	g := New()
	node := Node{ID: ""}

	err := g.AddNode(node)
	if err != ErrInvalidNodeID {
		t.Errorf("AddNode() empty ID error = %v, want ErrInvalidNodeID", err)
	}
}

func TestMemGraph_GetNode(t *testing.T) {
	g := New()
	node := NewFileNode("/path/to/file.go")
	_ = g.AddNode(node)

	got, ok := g.GetNode(node.ID)
	if !ok {
		t.Fatal("GetNode() returned false")
	}
	if got.ID != node.ID {
		t.Errorf("GetNode().ID = %q, want %q", got.ID, node.ID)
	}

	// Non-existent node
	_, ok = g.GetNode("nonexistent")
	if ok {
		t.Error("GetNode() for nonexistent should return false")
	}
}

func TestMemGraph_UpdateNode(t *testing.T) {
	g := New()
	node := NewFileNode("/path/to/file.go")
	_ = g.AddNode(node)

	// Update the node
	node.Name = "updated.go"
	err := g.UpdateNode(node)
	if err != nil {
		t.Fatalf("UpdateNode() error = %v", err)
	}

	got, _ := g.GetNode(node.ID)
	if got.Name != "updated.go" {
		t.Errorf("UpdateNode().Name = %q, want updated.go", got.Name)
	}
}

func TestMemGraph_RemoveNode(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path/to/file2", nil))

	err := g.RemoveNode(node1.ID)
	if err != nil {
		t.Fatalf("RemoveNode() error = %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("NodeCount() after remove = %d, want 1", g.NodeCount())
	}

	// Edge should also be removed
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() after remove = %d, want 0", g.EdgeCount())
	}

	// Remove non-existent node
	err = g.RemoveNode("nonexistent")
	if err != ErrNodeNotFound {
		t.Errorf("RemoveNode() nonexistent error = %v, want ErrNodeNotFound", err)
	}
}

func TestMemGraph_AddEdge(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)

	edge := NewImportEdge(node1.ID, node2.ID, "path/to/file2", nil)
	err := g.AddEdge(edge)
	if err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() = %d, want 1", g.EdgeCount())
	}
}

func TestMemGraph_AddEdge_Errors(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	_ = g.AddNode(node1)

	// Missing target node
	edge := NewImportEdge(node1.ID, "nonexistent", "path", nil)
	err := g.AddEdge(edge)
	if err != ErrNodeNotFound {
		t.Errorf("AddEdge() missing target error = %v, want ErrNodeNotFound", err)
	}

	// Invalid edge
	invalidEdge := Edge{From: "", To: node1.ID}
	err = g.AddEdge(invalidEdge)
	if err != ErrInvalidEdge {
		t.Errorf("AddEdge() invalid error = %v, want ErrInvalidEdge", err)
	}
}

func TestMemGraph_AddEdge_Duplicate(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)

	edge := NewImportEdge(node1.ID, node2.ID, "path", nil)
	_ = g.AddEdge(edge)

	err := g.AddEdge(edge)
	if err != ErrEdgeExists {
		t.Errorf("AddEdge() duplicate error = %v, want ErrEdgeExists", err)
	}
}

func TestMemGraph_RemoveEdge(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path", nil))

	err := g.RemoveEdge(node1.ID, node2.ID, EdgeTypeImports)
	if err != nil {
		t.Fatalf("RemoveEdge() error = %v", err)
	}

	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() after remove = %d, want 0", g.EdgeCount())
	}

	// Remove non-existent edge
	err = g.RemoveEdge(node1.ID, node2.ID, EdgeTypeImports)
	if err != ErrEdgeNotFound {
		t.Errorf("RemoveEdge() nonexistent error = %v, want ErrEdgeNotFound", err)
	}
}

func TestMemGraph_GetEdges(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	node3 := NewFileNode("/path/to/file3.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddNode(node3)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path2", nil))
	_ = g.AddEdge(NewImportEdge(node1.ID, node3.ID, "path3", nil))

	edges := g.GetEdges(node1.ID)
	if len(edges) != 2 {
		t.Errorf("GetEdges() len = %d, want 2", len(edges))
	}
}

func TestMemGraph_GetReverseEdges(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	node3 := NewFileNode("/path/to/file3.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddNode(node3)
	_ = g.AddEdge(NewImportEdge(node1.ID, node3.ID, "path3", nil))
	_ = g.AddEdge(NewImportEdge(node2.ID, node3.ID, "path3", nil))

	edges := g.GetReverseEdges(node3.ID)
	if len(edges) != 2 {
		t.Errorf("GetReverseEdges() len = %d, want 2", len(edges))
	}
}

func TestMemGraph_Dependencies(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	node3 := NewFileNode("/path/to/file3.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddNode(node3)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path2", nil))
	_ = g.AddEdge(NewImportEdge(node1.ID, node3.ID, "path3", nil))

	deps := g.Dependencies(node1.ID)
	if len(deps) != 2 {
		t.Errorf("Dependencies() len = %d, want 2", len(deps))
	}
}

func TestMemGraph_Dependents(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	node3 := NewFileNode("/path/to/file3.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddNode(node3)
	_ = g.AddEdge(NewImportEdge(node1.ID, node3.ID, "path3", nil))
	_ = g.AddEdge(NewImportEdge(node2.ID, node3.ID, "path3", nil))

	deps := g.Dependents(node3.ID)
	if len(deps) != 2 {
		t.Errorf("Dependents() len = %d, want 2", len(deps))
	}
}

func TestMemGraph_RelatedNodes(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	node3 := NewFileNode("/path/to/file3.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddNode(node3)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path2", nil))
	_ = g.AddEdge(NewImportEdge(node2.ID, node3.ID, "path3", nil))

	// Degree 1: node1 -> node2
	related := g.RelatedNodes(node1.ID, 1)
	if len(related) != 1 {
		t.Errorf("RelatedNodes(1) len = %d, want 1", len(related))
	}

	// Degree 2: node1 -> node2 -> node3
	related = g.RelatedNodes(node1.ID, 2)
	if len(related) != 2 {
		t.Errorf("RelatedNodes(2) len = %d, want 2", len(related))
	}
}

func TestMemGraph_FindPath(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	node3 := NewFileNode("/path/to/file3.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddNode(node3)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path2", nil))
	_ = g.AddEdge(NewImportEdge(node2.ID, node3.ID, "path3", nil))

	path := g.FindPath(node1.ID, node3.ID)
	if len(path) != 3 {
		t.Errorf("FindPath() len = %d, want 3", len(path))
	}

	// No path
	path = g.FindPath(node3.ID, node1.ID)
	if path != nil {
		t.Errorf("FindPath() reverse should be nil, got %d nodes", len(path))
	}
}

func TestMemGraph_FindNodesByType(t *testing.T) {
	g := New()
	_ = g.AddNode(NewFileNode("/path/to/file1.go"))
	_ = g.AddNode(NewFileNode("/path/to/file2.go"))
	_ = g.AddNode(NewDirectoryNode("/path/to"))

	files := g.FindNodesByType(NodeTypeFile)
	if len(files) != 2 {
		t.Errorf("FindNodesByType(File) len = %d, want 2", len(files))
	}

	dirs := g.FindNodesByType(NodeTypeDirectory)
	if len(dirs) != 1 {
		t.Errorf("FindNodesByType(Directory) len = %d, want 1", len(dirs))
	}
}

func TestMemGraph_FindNodeByPath(t *testing.T) {
	g := New()
	node := NewFileNode("/path/to/file.go")
	_ = g.AddNode(node)

	found, ok := g.FindNodeByPath("/path/to/file.go")
	if !ok {
		t.Fatal("FindNodeByPath() should find node")
	}
	if found.ID != node.ID {
		t.Errorf("FindNodeByPath().ID = %q, want %q", found.ID, node.ID)
	}

	_, ok = g.FindNodeByPath("/nonexistent.go")
	if ok {
		t.Error("FindNodeByPath() should not find nonexistent")
	}
}

func TestMemGraph_Clear(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path", nil))

	g.Clear()

	if g.NodeCount() != 0 {
		t.Errorf("NodeCount() after Clear = %d, want 0", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() after Clear = %d, want 0", g.EdgeCount())
	}
}

func TestMemGraph_SaveLoad(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path", nil))

	// Save
	var buf bytes.Buffer
	err := g.Save(&buf)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load into new graph
	g2 := New()
	err = g2.Load(&buf)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if g2.NodeCount() != 2 {
		t.Errorf("Loaded NodeCount() = %d, want 2", g2.NodeCount())
	}
	if g2.EdgeCount() != 1 {
		t.Errorf("Loaded EdgeCount() = %d, want 1", g2.EdgeCount())
	}
}

func TestMemGraph_AllNodes(t *testing.T) {
	g := New()
	_ = g.AddNode(NewFileNode("/path/to/file1.go"))
	_ = g.AddNode(NewFileNode("/path/to/file2.go"))

	nodes := g.AllNodes()
	if len(nodes) != 2 {
		t.Errorf("AllNodes() len = %d, want 2", len(nodes))
	}
}

func TestMemGraph_AllEdges(t *testing.T) {
	g := New()
	node1 := NewFileNode("/path/to/file1.go")
	node2 := NewFileNode("/path/to/file2.go")
	_ = g.AddNode(node1)
	_ = g.AddNode(node2)
	_ = g.AddEdge(NewImportEdge(node1.ID, node2.ID, "path", nil))

	edges := g.AllEdges()
	if len(edges) != 1 {
		t.Errorf("AllEdges() len = %d, want 1", len(edges))
	}
}
