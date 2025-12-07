package debug

import (
	"testing"

	"github.com/dshills/keystorm/internal/integration/debug/dap"
)

func TestVariable_HasChildren(t *testing.T) {
	tests := []struct {
		name     string
		varRef   int
		expected bool
	}{
		{"no children", 0, false},
		{"has children", 5, true},
		{"negative ref", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Variable{VariablesReference: tt.varRef}
			if v.HasChildren() != tt.expected {
				t.Errorf("HasChildren() = %v, expected %v", v.HasChildren(), tt.expected)
			}
		})
	}
}

func TestVariable_TotalChildren(t *testing.T) {
	v := &Variable{
		NamedVariables:   5,
		IndexedVariables: 10,
	}

	if v.TotalChildren() != 15 {
		t.Errorf("TotalChildren() = %d, expected 15", v.TotalChildren())
	}
}

func TestVariableInspector_NewVariableInspector(t *testing.T) {
	vi := NewVariableInspector(nil)
	if vi == nil {
		t.Fatal("NewVariableInspector returned nil")
	}
	if vi.cache == nil {
		t.Error("cache should be initialized")
	}
}

func TestVariableInspector_Watches(t *testing.T) {
	vi := NewVariableInspector(nil)

	// Add watches
	vi.AddWatch("x")
	vi.AddWatch("y + z")
	vi.AddWatch("obj.field")

	watches := vi.GetWatches()
	if len(watches) != 3 {
		t.Errorf("expected 3 watches, got %d", len(watches))
	}

	if watches[0] != "x" {
		t.Errorf("expected first watch 'x', got %s", watches[0])
	}

	// Remove watch
	err := vi.RemoveWatch(1)
	if err != nil {
		t.Fatalf("RemoveWatch failed: %v", err)
	}

	watches = vi.GetWatches()
	if len(watches) != 2 {
		t.Errorf("expected 2 watches after removal, got %d", len(watches))
	}

	if watches[1] != "obj.field" {
		t.Errorf("expected second watch 'obj.field' after removal, got %s", watches[1])
	}
}

func TestVariableInspector_RemoveWatchOutOfRange(t *testing.T) {
	vi := NewVariableInspector(nil)
	vi.AddWatch("x")

	err := vi.RemoveWatch(5)
	if err == nil {
		t.Error("expected error for out of range index")
	}

	err = vi.RemoveWatch(-1)
	if err == nil {
		t.Error("expected error for negative index")
	}
}

func TestVariableInspector_ClearWatches(t *testing.T) {
	vi := NewVariableInspector(nil)

	vi.AddWatch("x")
	vi.AddWatch("y")
	vi.AddWatch("z")

	vi.ClearWatches()

	watches := vi.GetWatches()
	if len(watches) != 0 {
		t.Errorf("expected 0 watches after clear, got %d", len(watches))
	}
}

func TestVariableInspector_ClearCache(t *testing.T) {
	vi := NewVariableInspector(nil)

	// Manually populate cache
	vi.cache[1] = []*Variable{{Name: "x"}}
	vi.cache[2] = []*Variable{{Name: "y"}}

	vi.ClearCache()

	if len(vi.cache) != 0 {
		t.Errorf("expected empty cache after clear, got %d entries", len(vi.cache))
	}
}

func TestVariableInspector_CollapseVariable(t *testing.T) {
	vi := NewVariableInspector(nil)

	v := &Variable{
		Name:     "obj",
		Expanded: true,
		Children: []*Variable{
			{Name: "field1"},
			{Name: "field2"},
		},
	}

	vi.CollapseVariable(v)

	if v.Expanded {
		t.Error("variable should not be expanded after collapse")
	}
	if v.Children != nil {
		t.Error("children should be nil after collapse")
	}
}

func TestVariableInspector_FormatVariable(t *testing.T) {
	vi := NewVariableInspector(nil)

	tests := []struct {
		name     string
		variable *Variable
		expected string
	}{
		{
			name:     "with type",
			variable: &Variable{Name: "x", Type: "int", Value: "42"},
			expected: "x: int = 42",
		},
		{
			name:     "without type",
			variable: &Variable{Name: "y", Value: "hello"},
			expected: "y = hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vi.FormatVariable(tt.variable)
			if result != tt.expected {
				t.Errorf("FormatVariable() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestVariableInspector_GetVariablePath(t *testing.T) {
	vi := NewVariableInspector(nil)

	root := &Variable{Name: "obj"}
	child := &Variable{Name: "field", Parent: root}
	grandchild := &Variable{Name: "value", Parent: child}

	path := vi.GetVariablePath(grandchild)

	if len(path) != 3 {
		t.Fatalf("expected path length 3, got %d", len(path))
	}
	if path[0] != "obj" {
		t.Errorf("expected path[0] = 'obj', got %s", path[0])
	}
	if path[1] != "field" {
		t.Errorf("expected path[1] = 'field', got %s", path[1])
	}
	if path[2] != "value" {
		t.Errorf("expected path[2] = 'value', got %s", path[2])
	}
}

func TestVariableInspector_GetWatchResults(t *testing.T) {
	vi := NewVariableInspector(nil)

	// Initially empty
	results := vi.GetWatchResults()
	if len(results) != 0 {
		t.Errorf("expected 0 results initially, got %d", len(results))
	}
}

func TestMapScopeType(t *testing.T) {
	tests := []struct {
		hint     string
		expected ScopeType
	}{
		{"locals", ScopeLocals},
		{"arguments", ScopeArguments},
		{"globals", ScopeGlobals},
		{"registers", ScopeRegisters},
		{"unknown", ScopeLocals},
		{"", ScopeLocals},
	}

	for _, tt := range tests {
		t.Run(tt.hint, func(t *testing.T) {
			result := mapScopeType(tt.hint)
			if result != tt.expected {
				t.Errorf("mapScopeType(%s) = %s, expected %s", tt.hint, result, tt.expected)
			}
		})
	}
}

func TestVariableScope(t *testing.T) {
	scope := &VariableScope{
		Name:               "Locals",
		Type:               ScopeLocals,
		PresentationHint:   "locals",
		VariablesReference: 100,
		NamedVariables:     5,
		IndexedVariables:   0,
		Expensive:          false,
	}

	if scope.Name != "Locals" {
		t.Errorf("expected name 'Locals', got %s", scope.Name)
	}
	if scope.Type != ScopeLocals {
		t.Errorf("expected type ScopeLocals, got %v", scope.Type)
	}
}

func TestDapVariableToVariable(t *testing.T) {
	vi := NewVariableInspector(nil)

	dapVar := dap.Variable{
		Name:               "myVar",
		Value:              "123",
		Type:               "int",
		VariablesReference: 0,
		NamedVariables:     0,
		IndexedVariables:   0,
		EvaluateName:       "myVar",
		MemoryReference:    "0x1234",
	}

	result := vi.dapVariableToVariable(dapVar)

	if result.Name != dapVar.Name {
		t.Errorf("Name mismatch: %s vs %s", result.Name, dapVar.Name)
	}
	if result.Value != dapVar.Value {
		t.Errorf("Value mismatch: %s vs %s", result.Value, dapVar.Value)
	}
	if result.Type != dapVar.Type {
		t.Errorf("Type mismatch: %s vs %s", result.Type, dapVar.Type)
	}
	if result.EvaluateName != dapVar.EvaluateName {
		t.Errorf("EvaluateName mismatch: %s vs %s", result.EvaluateName, dapVar.EvaluateName)
	}
	if result.MemoryReference != dapVar.MemoryReference {
		t.Errorf("MemoryReference mismatch: %s vs %s", result.MemoryReference, dapVar.MemoryReference)
	}
}

func TestScopeTypeConstants(t *testing.T) {
	if ScopeLocals != "locals" {
		t.Errorf("ScopeLocals should be 'locals'")
	}
	if ScopeArguments != "arguments" {
		t.Errorf("ScopeArguments should be 'arguments'")
	}
	if ScopeGlobals != "globals" {
		t.Errorf("ScopeGlobals should be 'globals'")
	}
	if ScopeRegisters != "registers" {
		t.Errorf("ScopeRegisters should be 'registers'")
	}
}
