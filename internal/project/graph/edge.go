package graph

// EdgeType indicates the relationship type between nodes.
type EdgeType int

const (
	// EdgeTypeImports indicates file A imports file B.
	EdgeTypeImports EdgeType = iota
	// EdgeTypeExports indicates module exports a symbol.
	EdgeTypeExports
	// EdgeTypeCalls indicates function A calls function B.
	EdgeTypeCalls
	// EdgeTypeExtends indicates class A extends class B.
	EdgeTypeExtends
	// EdgeTypeImplements indicates class implements interface.
	EdgeTypeImplements
	// EdgeTypeTests indicates test file tests implementation.
	EdgeTypeTests
	// EdgeTypeContains indicates directory contains file.
	EdgeTypeContains
	// EdgeTypeDependsOn indicates module depends on module.
	EdgeTypeDependsOn
	// EdgeTypeReferences indicates a generic reference.
	EdgeTypeReferences
)

// String returns the string representation of an EdgeType.
func (t EdgeType) String() string {
	switch t {
	case EdgeTypeImports:
		return "imports"
	case EdgeTypeExports:
		return "exports"
	case EdgeTypeCalls:
		return "calls"
	case EdgeTypeExtends:
		return "extends"
	case EdgeTypeImplements:
		return "implements"
	case EdgeTypeTests:
		return "tests"
	case EdgeTypeContains:
		return "contains"
	case EdgeTypeDependsOn:
		return "depends_on"
	case EdgeTypeReferences:
		return "references"
	default:
		return "unknown"
	}
}

// Edge represents a relationship between nodes.
type Edge struct {
	// From is the source node ID.
	From NodeID `json:"from"`
	// To is the target node ID.
	To NodeID `json:"to"`
	// Type is the relationship type.
	Type EdgeType `json:"type"`
	// Weight is the relationship strength (for ranking).
	Weight float64 `json:"weight,omitempty"`
	// Metadata holds edge-specific metadata.
	Metadata EdgeMeta `json:"metadata,omitempty"`
}

// EdgeMeta holds edge-specific metadata.
type EdgeMeta struct {
	// For imports
	ImportPath string   `json:"import_path,omitempty"`
	Symbols    []string `json:"symbols,omitempty"` // Imported symbols

	// For calls
	CallSites []Location `json:"call_sites,omitempty"`

	// Custom metadata
	Extra map[string]any `json:"extra,omitempty"`
}

// Location represents a position in a file.
type Location struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// NewImportEdge creates an import relationship edge.
func NewImportEdge(from, to NodeID, importPath string, symbols []string) Edge {
	return Edge{
		From:   from,
		To:     to,
		Type:   EdgeTypeImports,
		Weight: 1.0,
		Metadata: EdgeMeta{
			ImportPath: importPath,
			Symbols:    symbols,
		},
	}
}

// NewContainsEdge creates a containment relationship edge.
func NewContainsEdge(parent, child NodeID) Edge {
	return Edge{
		From:   parent,
		To:     child,
		Type:   EdgeTypeContains,
		Weight: 1.0,
	}
}

// NewTestsEdge creates a test relationship edge.
func NewTestsEdge(testFile, targetFile NodeID) Edge {
	return Edge{
		From:   testFile,
		To:     targetFile,
		Type:   EdgeTypeTests,
		Weight: 1.0,
	}
}

// NewCallsEdge creates a function call relationship edge.
func NewCallsEdge(caller, callee NodeID, callSites []Location) Edge {
	return Edge{
		From:   caller,
		To:     callee,
		Type:   EdgeTypeCalls,
		Weight: float64(len(callSites)),
		Metadata: EdgeMeta{
			CallSites: callSites,
		},
	}
}

// NewDependsOnEdge creates a module dependency edge.
func NewDependsOnEdge(module, dependency NodeID) Edge {
	return Edge{
		From:   module,
		To:     dependency,
		Type:   EdgeTypeDependsOn,
		Weight: 1.0,
	}
}

// NewExtendsEdge creates a class extension edge.
func NewExtendsEdge(child, parent NodeID) Edge {
	return Edge{
		From:   child,
		To:     parent,
		Type:   EdgeTypeExtends,
		Weight: 1.0,
	}
}

// NewImplementsEdge creates an interface implementation edge.
func NewImplementsEdge(class, iface NodeID) Edge {
	return Edge{
		From:   class,
		To:     iface,
		Type:   EdgeTypeImplements,
		Weight: 1.0,
	}
}

// NewReferencesEdge creates a generic reference edge.
func NewReferencesEdge(from, to NodeID) Edge {
	return Edge{
		From:   from,
		To:     to,
		Type:   EdgeTypeReferences,
		Weight: 1.0,
	}
}

// IsImportEdge returns true if this is an import edge.
func (e Edge) IsImportEdge() bool {
	return e.Type == EdgeTypeImports
}

// IsContainsEdge returns true if this is a containment edge.
func (e Edge) IsContainsEdge() bool {
	return e.Type == EdgeTypeContains
}

// IsTestEdge returns true if this is a test edge.
func (e Edge) IsTestEdge() bool {
	return e.Type == EdgeTypeTests
}

// IsValid returns true if the edge has valid from and to IDs.
func (e Edge) IsValid() bool {
	return e.From != "" && e.To != ""
}
