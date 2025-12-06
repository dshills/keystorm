// Package graph provides a project graph implementation for tracking file relationships.
// The graph models the project as interconnected nodes (files, modules, packages)
// with edges representing relationships (imports, calls, tests).
package graph

import (
	"context"
	"encoding/json"
	"io"
	"sync"
)

// Graph represents the project's structural relationships.
// It is thread-safe for concurrent access.
type Graph interface {
	// Node operations
	AddNode(node Node) error
	RemoveNode(id NodeID) error
	GetNode(id NodeID) (Node, bool)
	UpdateNode(node Node) error
	NodeCount() int
	AllNodes() []Node

	// Edge operations
	AddEdge(edge Edge) error
	RemoveEdge(from, to NodeID, edgeType EdgeType) error
	GetEdges(from NodeID) []Edge
	GetReverseEdges(to NodeID) []Edge
	EdgeCount() int
	AllEdges() []Edge

	// Queries
	Dependencies(id NodeID) []Node
	Dependents(id NodeID) []Node
	RelatedNodes(id NodeID, maxDegree int) []Node
	FindPath(from, to NodeID) []Node
	FindNodesByType(nodeType NodeType) []Node
	FindNodeByPath(path string) (Node, bool)

	// Batch operations
	Clear()
	Rebuild(ctx context.Context) error

	// Serialization
	Save(w io.Writer) error
	Load(r io.Reader) error
}

// MemGraph is an in-memory implementation of Graph.
type MemGraph struct {
	mu sync.RWMutex

	// Node storage
	nodes map[NodeID]Node
	// Path to NodeID index for fast path lookups
	pathIndex map[string]NodeID

	// Edge storage - adjacency list
	outEdges map[NodeID][]Edge // from -> [edges]
	inEdges  map[NodeID][]Edge // to -> [edges]
}

// New creates a new in-memory graph.
func New() *MemGraph {
	return &MemGraph{
		nodes:     make(map[NodeID]Node),
		pathIndex: make(map[string]NodeID),
		outEdges:  make(map[NodeID][]Edge),
		inEdges:   make(map[NodeID][]Edge),
	}
}

// AddNode adds a node to the graph.
func (g *MemGraph) AddNode(node Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node.ID == "" {
		return ErrInvalidNodeID
	}

	if _, exists := g.nodes[node.ID]; exists {
		return ErrNodeExists
	}

	g.nodes[node.ID] = node

	// Index by path if available
	if node.Path != "" {
		g.pathIndex[node.Path] = node.ID
	}

	return nil
}

// RemoveNode removes a node and all its edges from the graph.
func (g *MemGraph) RemoveNode(id NodeID) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[id]
	if !exists {
		return ErrNodeNotFound
	}

	// Remove from path index
	if node.Path != "" {
		delete(g.pathIndex, node.Path)
	}

	// Remove all outgoing edges
	delete(g.outEdges, id)

	// Remove all incoming edges
	delete(g.inEdges, id)

	// Remove edges referencing this node from other nodes
	for fromID, edges := range g.outEdges {
		filtered := make([]Edge, 0, len(edges))
		for _, e := range edges {
			if e.To != id {
				filtered = append(filtered, e)
			}
		}
		g.outEdges[fromID] = filtered
	}

	for toID, edges := range g.inEdges {
		filtered := make([]Edge, 0, len(edges))
		for _, e := range edges {
			if e.From != id {
				filtered = append(filtered, e)
			}
		}
		g.inEdges[toID] = filtered
	}

	// Remove the node
	delete(g.nodes, id)

	return nil
}

// GetNode returns a node by ID.
func (g *MemGraph) GetNode(id NodeID) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	return node, exists
}

// UpdateNode updates an existing node.
func (g *MemGraph) UpdateNode(node Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node.ID == "" {
		return ErrInvalidNodeID
	}

	oldNode, exists := g.nodes[node.ID]
	if !exists {
		return ErrNodeNotFound
	}

	// Update path index if path changed
	if oldNode.Path != node.Path {
		if oldNode.Path != "" {
			delete(g.pathIndex, oldNode.Path)
		}
		if node.Path != "" {
			g.pathIndex[node.Path] = node.ID
		}
	}

	g.nodes[node.ID] = node
	return nil
}

// NodeCount returns the number of nodes in the graph.
func (g *MemGraph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// AllNodes returns all nodes in the graph.
func (g *MemGraph) AllNodes() []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// AddEdge adds an edge to the graph.
func (g *MemGraph) AddEdge(edge Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if edge.From == "" || edge.To == "" {
		return ErrInvalidEdge
	}

	// Check that both nodes exist
	if _, exists := g.nodes[edge.From]; !exists {
		return ErrNodeNotFound
	}
	if _, exists := g.nodes[edge.To]; !exists {
		return ErrNodeNotFound
	}

	// Check for duplicate edge
	for _, e := range g.outEdges[edge.From] {
		if e.To == edge.To && e.Type == edge.Type {
			return ErrEdgeExists
		}
	}

	// Add to adjacency lists
	g.outEdges[edge.From] = append(g.outEdges[edge.From], edge)
	g.inEdges[edge.To] = append(g.inEdges[edge.To], edge)

	return nil
}

// RemoveEdge removes an edge from the graph.
func (g *MemGraph) RemoveEdge(from, to NodeID, edgeType EdgeType) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Remove from outEdges
	edges := g.outEdges[from]
	found := false
	filtered := make([]Edge, 0, len(edges))
	for _, e := range edges {
		if e.To == to && e.Type == edgeType {
			found = true
		} else {
			filtered = append(filtered, e)
		}
	}
	if !found {
		return ErrEdgeNotFound
	}
	g.outEdges[from] = filtered

	// Remove from inEdges
	edges = g.inEdges[to]
	filtered = make([]Edge, 0, len(edges))
	for _, e := range edges {
		if !(e.From == from && e.Type == edgeType) {
			filtered = append(filtered, e)
		}
	}
	g.inEdges[to] = filtered

	return nil
}

// GetEdges returns all outgoing edges from a node.
func (g *MemGraph) GetEdges(from NodeID) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := g.outEdges[from]
	result := make([]Edge, len(edges))
	copy(result, edges)
	return result
}

// GetReverseEdges returns all incoming edges to a node.
func (g *MemGraph) GetReverseEdges(to NodeID) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := g.inEdges[to]
	result := make([]Edge, len(edges))
	copy(result, edges)
	return result
}

// EdgeCount returns the total number of edges in the graph.
func (g *MemGraph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	count := 0
	for _, edges := range g.outEdges {
		count += len(edges)
	}
	return count
}

// AllEdges returns all edges in the graph.
func (g *MemGraph) AllEdges() []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var edges []Edge
	for _, nodeEdges := range g.outEdges {
		edges = append(edges, nodeEdges...)
	}
	return edges
}

// Dependencies returns all nodes that this node depends on (outgoing edges).
func (g *MemGraph) Dependencies(id NodeID) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var deps []Node
	seen := make(map[NodeID]bool)

	for _, edge := range g.outEdges[id] {
		if !seen[edge.To] {
			seen[edge.To] = true
			if node, exists := g.nodes[edge.To]; exists {
				deps = append(deps, node)
			}
		}
	}

	return deps
}

// Dependents returns all nodes that depend on this node (incoming edges).
func (g *MemGraph) Dependents(id NodeID) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var deps []Node
	seen := make(map[NodeID]bool)

	for _, edge := range g.inEdges[id] {
		if !seen[edge.From] {
			seen[edge.From] = true
			if node, exists := g.nodes[edge.From]; exists {
				deps = append(deps, node)
			}
		}
	}

	return deps
}

// RelatedNodes returns nodes within maxDegree edges of the given node.
func (g *MemGraph) RelatedNodes(id NodeID, maxDegree int) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if maxDegree <= 0 {
		return nil
	}

	visited := make(map[NodeID]bool)
	visited[id] = true

	// BFS to find related nodes
	current := []NodeID{id}

	for degree := 0; degree < maxDegree && len(current) > 0; degree++ {
		var next []NodeID

		for _, nodeID := range current {
			// Outgoing edges
			for _, edge := range g.outEdges[nodeID] {
				if !visited[edge.To] {
					visited[edge.To] = true
					next = append(next, edge.To)
				}
			}

			// Incoming edges
			for _, edge := range g.inEdges[nodeID] {
				if !visited[edge.From] {
					visited[edge.From] = true
					next = append(next, edge.From)
				}
			}
		}

		current = next
	}

	// Collect nodes (excluding the source node)
	var nodes []Node
	for nodeID := range visited {
		if nodeID != id {
			if node, exists := g.nodes[nodeID]; exists {
				nodes = append(nodes, node)
			}
		}
	}

	return nodes
}

// FindPath finds the shortest path between two nodes using BFS.
// Returns nil if no path exists.
func (g *MemGraph) FindPath(from, to NodeID) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if from == to {
		if node, exists := g.nodes[from]; exists {
			return []Node{node}
		}
		return nil
	}

	// BFS
	visited := make(map[NodeID]bool)
	parent := make(map[NodeID]NodeID)
	queue := []NodeID{from}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range g.outEdges[current] {
			if !visited[edge.To] {
				visited[edge.To] = true
				parent[edge.To] = current
				queue = append(queue, edge.To)

				if edge.To == to {
					// Found path - reconstruct it
					var path []Node
					for nodeID := to; nodeID != ""; {
						if node, exists := g.nodes[nodeID]; exists {
							path = append([]Node{node}, path...)
						}
						if nodeID == from {
							break
						}
						nodeID = parent[nodeID]
					}
					return path
				}
			}
		}
	}

	return nil
}

// FindNodesByType returns all nodes of a given type.
func (g *MemGraph) FindNodesByType(nodeType NodeType) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []Node
	for _, node := range g.nodes {
		if node.Type == nodeType {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// FindNodeByPath returns the node with the given file path.
func (g *MemGraph) FindNodeByPath(path string) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if id, exists := g.pathIndex[path]; exists {
		if node, ok := g.nodes[id]; ok {
			return node, true
		}
	}
	return Node{}, false
}

// Clear removes all nodes and edges from the graph.
func (g *MemGraph) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes = make(map[NodeID]Node)
	g.pathIndex = make(map[string]NodeID)
	g.outEdges = make(map[NodeID][]Edge)
	g.inEdges = make(map[NodeID][]Edge)
}

// Rebuild rebuilds the graph from scratch.
// This is a no-op for MemGraph; subclasses can override to reload from source.
func (g *MemGraph) Rebuild(ctx context.Context) error {
	return nil
}

// graphData is the serialization format for the graph.
type graphData struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Save serializes the graph to a writer.
func (g *MemGraph) Save(w io.Writer) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	data := graphData{
		Nodes: make([]Node, 0, len(g.nodes)),
		Edges: make([]Edge, 0),
	}

	for _, node := range g.nodes {
		data.Nodes = append(data.Nodes, node)
	}

	for _, edges := range g.outEdges {
		data.Edges = append(data.Edges, edges...)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Load deserializes the graph from a reader.
func (g *MemGraph) Load(r io.Reader) error {
	var data graphData
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Clear existing data
	g.nodes = make(map[NodeID]Node)
	g.pathIndex = make(map[string]NodeID)
	g.outEdges = make(map[NodeID][]Edge)
	g.inEdges = make(map[NodeID][]Edge)

	// Load nodes
	for _, node := range data.Nodes {
		g.nodes[node.ID] = node
		if node.Path != "" {
			g.pathIndex[node.Path] = node.ID
		}
	}

	// Load edges
	for _, edge := range data.Edges {
		g.outEdges[edge.From] = append(g.outEdges[edge.From], edge)
		g.inEdges[edge.To] = append(g.inEdges[edge.To], edge)
	}

	return nil
}
