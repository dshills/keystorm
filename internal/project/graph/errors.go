package graph

import "errors"

// Graph errors.
var (
	// ErrInvalidNodeID indicates an invalid or empty node ID.
	ErrInvalidNodeID = errors.New("invalid node ID")
	// ErrNodeNotFound indicates a node was not found in the graph.
	ErrNodeNotFound = errors.New("node not found")
	// ErrNodeExists indicates a node already exists in the graph.
	ErrNodeExists = errors.New("node already exists")
	// ErrInvalidEdge indicates an invalid edge (missing from/to).
	ErrInvalidEdge = errors.New("invalid edge")
	// ErrEdgeNotFound indicates an edge was not found in the graph.
	ErrEdgeNotFound = errors.New("edge not found")
	// ErrEdgeExists indicates an edge already exists.
	ErrEdgeExists = errors.New("edge already exists")
	// ErrCycleDetected indicates a cycle was detected in the graph.
	ErrCycleDetected = errors.New("cycle detected")
)
