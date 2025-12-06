package graph

import (
	"sort"
	"strings"
)

// QueryResult represents a graph query result.
type QueryResult struct {
	Nodes []Node
	Edges []Edge
}

// RelatedFile represents a file related to a given file.
type RelatedFile struct {
	Node      Node
	Relation  EdgeType
	Distance  int     // Graph distance from source
	Relevance float64 // Relevance score (higher is more relevant)
}

// FindRelatedFiles finds files related to the given path.
// It considers imports, tests, and other relationships.
func FindRelatedFiles(g Graph, path string, maxResults int) []RelatedFile {
	node, ok := g.FindNodeByPath(path)
	if !ok {
		return nil
	}

	var related []RelatedFile
	seen := make(map[NodeID]bool)
	seen[node.ID] = true

	// Direct dependencies (imports)
	for _, dep := range g.Dependencies(node.ID) {
		if dep.IsFileNode() && !seen[dep.ID] {
			seen[dep.ID] = true
			related = append(related, RelatedFile{
				Node:      dep,
				Relation:  EdgeTypeImports,
				Distance:  1,
				Relevance: 0.9,
			})
		}
	}

	// Direct dependents (files that import this)
	for _, dep := range g.Dependents(node.ID) {
		if dep.IsFileNode() && !seen[dep.ID] {
			seen[dep.ID] = true
			related = append(related, RelatedFile{
				Node:      dep,
				Relation:  EdgeTypeImports,
				Distance:  1,
				Relevance: 0.8,
			})
		}
	}

	// Find test files
	testFiles := FindTestsFor(g, path)
	for _, testNode := range testFiles {
		if !seen[testNode.ID] {
			seen[testNode.ID] = true
			related = append(related, RelatedFile{
				Node:      testNode,
				Relation:  EdgeTypeTests,
				Distance:  1,
				Relevance: 0.95,
			})
		}
	}

	// Find implementation if this is a test file
	if strings.HasSuffix(path, "_test.go") {
		implFiles := FindImplementationFor(g, path)
		for _, implNode := range implFiles {
			if !seen[implNode.ID] {
				seen[implNode.ID] = true
				related = append(related, RelatedFile{
					Node:      implNode,
					Relation:  EdgeTypeTests,
					Distance:  1,
					Relevance: 0.95,
				})
			}
		}
	}

	// Files in the same package/directory
	sameDir := FindFilesInSamePackage(g, path)
	for _, sameNode := range sameDir {
		if !seen[sameNode.ID] {
			seen[sameNode.ID] = true
			related = append(related, RelatedFile{
				Node:      sameNode,
				Relation:  EdgeTypeContains,
				Distance:  1,
				Relevance: 0.7,
			})
		}
	}

	// Sort by relevance
	sort.Slice(related, func(i, j int) bool {
		return related[i].Relevance > related[j].Relevance
	})

	// Limit results
	if maxResults > 0 && len(related) > maxResults {
		related = related[:maxResults]
	}

	return related
}

// FindTestsFor finds test files for a given implementation file.
func FindTestsFor(g Graph, implPath string) []Node {
	var tests []Node

	// Look for test edges pointing to this file
	implNode, ok := g.FindNodeByPath(implPath)
	if !ok {
		return tests
	}

	// Check incoming test edges
	for _, edge := range g.GetReverseEdges(implNode.ID) {
		if edge.Type == EdgeTypeTests {
			if testNode, ok := g.GetNode(edge.From); ok {
				tests = append(tests, testNode)
			}
		}
	}

	// Also check by naming convention
	if strings.HasSuffix(implPath, ".go") && !strings.HasSuffix(implPath, "_test.go") {
		testPath := strings.TrimSuffix(implPath, ".go") + "_test.go"
		if testNode, ok := g.FindNodeByPath(testPath); ok {
			// Check if already added
			found := false
			for _, t := range tests {
				if t.ID == testNode.ID {
					found = true
					break
				}
			}
			if !found {
				tests = append(tests, testNode)
			}
		}
	}

	return tests
}

// FindImplementationFor finds implementation files for a given test file.
func FindImplementationFor(g Graph, testPath string) []Node {
	var impls []Node

	// Look for test edges from this file
	testNode, ok := g.FindNodeByPath(testPath)
	if !ok {
		return impls
	}

	// Check outgoing test edges
	for _, edge := range g.GetEdges(testNode.ID) {
		if edge.Type == EdgeTypeTests {
			if implNode, ok := g.GetNode(edge.To); ok {
				impls = append(impls, implNode)
			}
		}
	}

	// Also check by naming convention
	if strings.HasSuffix(testPath, "_test.go") {
		implPath := strings.TrimSuffix(testPath, "_test.go") + ".go"
		if implNode, ok := g.FindNodeByPath(implPath); ok {
			// Check if already added
			found := false
			for _, i := range impls {
				if i.ID == implNode.ID {
					found = true
					break
				}
			}
			if !found {
				impls = append(impls, implNode)
			}
		}
	}

	return impls
}

// FindFilesInSamePackage finds other files in the same package/directory.
func FindFilesInSamePackage(g Graph, path string) []Node {
	var files []Node

	node, ok := g.FindNodeByPath(path)
	if !ok {
		return files
	}

	// Find parent package/directory
	for _, edge := range g.GetReverseEdges(node.ID) {
		if edge.Type == EdgeTypeContains {
			// Found parent - get all children
			for _, childEdge := range g.GetEdges(edge.From) {
				if childEdge.Type == EdgeTypeContains && childEdge.To != node.ID {
					if childNode, ok := g.GetNode(childEdge.To); ok {
						if childNode.IsFileNode() {
							files = append(files, childNode)
						}
					}
				}
			}
		}
	}

	return files
}

// FindImportChain finds the import chain from source to target.
// Returns nil if no path exists.
func FindImportChain(g Graph, sourcePath, targetPath string) []Node {
	sourceNode, ok := g.FindNodeByPath(sourcePath)
	if !ok {
		return nil
	}

	targetNode, ok := g.FindNodeByPath(targetPath)
	if !ok {
		return nil
	}

	return g.FindPath(sourceNode.ID, targetNode.ID)
}

// GetImports returns all files imported by the given file.
func GetImports(g Graph, path string) []Node {
	node, ok := g.FindNodeByPath(path)
	if !ok {
		return nil
	}

	var imports []Node
	for _, edge := range g.GetEdges(node.ID) {
		if edge.Type == EdgeTypeImports {
			if importNode, ok := g.GetNode(edge.To); ok {
				imports = append(imports, importNode)
			}
		}
	}

	return imports
}

// GetImportedBy returns all files that import the given file.
func GetImportedBy(g Graph, path string) []Node {
	node, ok := g.FindNodeByPath(path)
	if !ok {
		return nil
	}

	var importers []Node
	for _, edge := range g.GetReverseEdges(node.ID) {
		if edge.Type == EdgeTypeImports {
			if importerNode, ok := g.GetNode(edge.From); ok {
				importers = append(importers, importerNode)
			}
		}
	}

	return importers
}

// GetAllDependencies returns all transitive dependencies of a file.
func GetAllDependencies(g Graph, path string, maxDepth int) []Node {
	node, ok := g.FindNodeByPath(path)
	if !ok {
		return nil
	}

	visited := make(map[NodeID]bool)
	visited[node.ID] = true
	var deps []Node

	var collect func(id NodeID, depth int)
	collect = func(id NodeID, depth int) {
		if depth >= maxDepth {
			return
		}

		for _, edge := range g.GetEdges(id) {
			if edge.Type == EdgeTypeImports {
				if !visited[edge.To] {
					visited[edge.To] = true
					if depNode, ok := g.GetNode(edge.To); ok {
						deps = append(deps, depNode)
						collect(edge.To, depth+1)
					}
				}
			}
		}
	}

	collect(node.ID, 0)
	return deps
}

// GetAllDependents returns all files that transitively depend on this file.
func GetAllDependents(g Graph, path string, maxDepth int) []Node {
	node, ok := g.FindNodeByPath(path)
	if !ok {
		return nil
	}

	visited := make(map[NodeID]bool)
	visited[node.ID] = true
	var deps []Node

	var collect func(id NodeID, depth int)
	collect = func(id NodeID, depth int) {
		if depth >= maxDepth {
			return
		}

		for _, edge := range g.GetReverseEdges(id) {
			if edge.Type == EdgeTypeImports {
				if !visited[edge.From] {
					visited[edge.From] = true
					if depNode, ok := g.GetNode(edge.From); ok {
						deps = append(deps, depNode)
						collect(edge.From, depth+1)
					}
				}
			}
		}
	}

	collect(node.ID, 0)
	return deps
}

// FindCycles finds all cycles in the import graph.
func FindCycles(g Graph) [][]Node {
	var cycles [][]Node
	visited := make(map[NodeID]bool)
	inStack := make(map[NodeID]bool)
	stack := make([]NodeID, 0)

	var dfs func(id NodeID) bool
	dfs = func(id NodeID) bool {
		visited[id] = true
		inStack[id] = true
		stack = append(stack, id)

		for _, edge := range g.GetEdges(id) {
			if edge.Type != EdgeTypeImports {
				continue
			}

			if !visited[edge.To] {
				if dfs(edge.To) {
					return true
				}
			} else if inStack[edge.To] {
				// Found cycle - extract it
				var cycle []Node
				foundStart := false
				for _, nodeID := range stack {
					if nodeID == edge.To {
						foundStart = true
					}
					if foundStart {
						if node, ok := g.GetNode(nodeID); ok {
							cycle = append(cycle, node)
						}
					}
				}
				// Add the closing node
				if node, ok := g.GetNode(edge.To); ok {
					cycle = append(cycle, node)
				}
				cycles = append(cycles, cycle)
			}
		}

		stack = stack[:len(stack)-1]
		inStack[id] = false
		return false
	}

	// Run DFS from all file nodes
	for _, node := range g.FindNodesByType(NodeTypeFile) {
		if !visited[node.ID] {
			dfs(node.ID)
		}
	}

	return cycles
}

// ComputeImpact estimates the impact of changing a file.
// Returns a score from 0-1 where 1 means highest impact.
func ComputeImpact(g Graph, path string) float64 {
	dependents := GetAllDependents(g, path, 10)
	totalFiles := len(g.FindNodesByType(NodeTypeFile))

	if totalFiles == 0 {
		return 0
	}

	// Impact is the fraction of files that depend on this file
	return float64(len(dependents)) / float64(totalFiles)
}
