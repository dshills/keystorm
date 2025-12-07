package debug

import (
	"context"
	"fmt"
	"sync"

	"github.com/dshills/keystorm/internal/integration/debug/dap"
)

// ScopeType represents the type of a variable scope.
type ScopeType string

const (
	// ScopeLocals represents local variables.
	ScopeLocals ScopeType = "locals"
	// ScopeArguments represents function arguments.
	ScopeArguments ScopeType = "arguments"
	// ScopeGlobals represents global variables.
	ScopeGlobals ScopeType = "globals"
	// ScopeRegisters represents CPU registers.
	ScopeRegisters ScopeType = "registers"
)

// VariableScope represents a scope containing variables.
type VariableScope struct {
	// Name is the scope name.
	Name string

	// Type is the scope type.
	Type ScopeType

	// PresentationHint is a hint for how to present the scope.
	PresentationHint string

	// VariablesReference is the reference to retrieve variables.
	VariablesReference int

	// NamedVariables is the number of named variables in this scope.
	NamedVariables int

	// IndexedVariables is the number of indexed variables in this scope.
	IndexedVariables int

	// Expensive indicates if fetching variables is expensive.
	Expensive bool

	// Source is the source location for this scope.
	Source *dap.Source

	// Line is the start line of the scope.
	Line int

	// Column is the start column of the scope.
	Column int

	// EndLine is the end line of the scope.
	EndLine int

	// EndColumn is the end column of the scope.
	EndColumn int
}

// Variable represents a variable or expression result.
type Variable struct {
	// Name is the variable name.
	Name string

	// Value is the variable value as a string.
	Value string

	// Type is the variable type.
	Type string

	// VariablesReference is the reference for child variables.
	VariablesReference int

	// NamedVariables is the number of named children.
	NamedVariables int

	// IndexedVariables is the number of indexed children.
	IndexedVariables int

	// EvaluateName is the expression to evaluate this variable.
	EvaluateName string

	// MemoryReference is a memory reference for the variable.
	MemoryReference string

	// PresentationHint contains hints for presentation.
	PresentationHint *dap.VariablePresentationHint

	// Children are the expanded child variables.
	Children []*Variable

	// Expanded indicates if children have been fetched.
	Expanded bool

	// Parent is the parent variable (nil for top-level).
	Parent *Variable
}

// HasChildren returns true if this variable has child variables.
func (v *Variable) HasChildren() bool {
	return v.VariablesReference > 0
}

// TotalChildren returns the total number of children.
func (v *Variable) TotalChildren() int {
	return v.NamedVariables + v.IndexedVariables
}

// VariableInspector provides variable inspection capabilities.
type VariableInspector struct {
	session *Session
	mu      sync.RWMutex

	// Cache of expanded variables by reference
	cache map[int][]*Variable

	// Watch expressions
	watches []string

	// Last evaluated watch results
	watchResults []*Variable
}

// NewVariableInspector creates a new variable inspector.
func NewVariableInspector(session *Session) *VariableInspector {
	return &VariableInspector{
		session: session,
		cache:   make(map[int][]*Variable),
	}
}

// GetScopes returns the scopes for a stack frame.
func (v *VariableInspector) GetScopes(ctx context.Context, frameID int) ([]*VariableScope, error) {
	scopes, err := v.session.GetScopes(ctx, frameID)
	if err != nil {
		return nil, err
	}

	result := make([]*VariableScope, len(scopes))
	for i, s := range scopes {
		result[i] = &VariableScope{
			Name:               s.Name,
			Type:               mapScopeType(s.PresentationHint),
			PresentationHint:   s.PresentationHint,
			VariablesReference: s.VariablesReference,
			NamedVariables:     s.NamedVariables,
			IndexedVariables:   s.IndexedVariables,
			Expensive:          s.Expensive,
			Source:             s.Source,
			Line:               s.Line,
			Column:             s.Column,
			EndLine:            s.EndLine,
			EndColumn:          s.EndColumn,
		}
	}

	return result, nil
}

// mapScopeType maps a DAP presentation hint to a scope type.
func mapScopeType(hint string) ScopeType {
	switch hint {
	case "locals":
		return ScopeLocals
	case "arguments":
		return ScopeArguments
	case "globals":
		return ScopeGlobals
	case "registers":
		return ScopeRegisters
	default:
		return ScopeLocals
	}
}

// GetVariables returns the variables for a scope or variable reference.
func (v *VariableInspector) GetVariables(ctx context.Context, variablesRef int) ([]*Variable, error) {
	// Check cache first
	v.mu.RLock()
	if cached, ok := v.cache[variablesRef]; ok {
		v.mu.RUnlock()
		return cached, nil
	}
	v.mu.RUnlock()

	vars, err := v.session.GetVariables(ctx, variablesRef)
	if err != nil {
		return nil, err
	}

	result := make([]*Variable, len(vars))
	for i, variable := range vars {
		result[i] = v.dapVariableToVariable(variable)
	}

	// Cache the result
	v.mu.Lock()
	v.cache[variablesRef] = result
	v.mu.Unlock()

	return result, nil
}

// dapVariableToVariable converts a DAP variable to our Variable type.
func (v *VariableInspector) dapVariableToVariable(dv dap.Variable) *Variable {
	return &Variable{
		Name:               dv.Name,
		Value:              dv.Value,
		Type:               dv.Type,
		VariablesReference: dv.VariablesReference,
		NamedVariables:     dv.NamedVariables,
		IndexedVariables:   dv.IndexedVariables,
		EvaluateName:       dv.EvaluateName,
		MemoryReference:    dv.MemoryReference,
		PresentationHint:   dv.PresentationHint,
	}
}

// ExpandVariable fetches and populates children for a variable.
func (v *VariableInspector) ExpandVariable(ctx context.Context, variable *Variable) error {
	if !variable.HasChildren() {
		return nil
	}

	if variable.Expanded {
		return nil // Already expanded
	}

	children, err := v.GetVariables(ctx, variable.VariablesReference)
	if err != nil {
		return err
	}

	// Set parent references
	for _, child := range children {
		child.Parent = variable
	}

	variable.Children = children
	variable.Expanded = true

	return nil
}

// CollapseVariable clears the children of a variable.
func (v *VariableInspector) CollapseVariable(variable *Variable) {
	variable.Children = nil
	variable.Expanded = false
}

// SetVariable sets the value of a variable.
func (v *VariableInspector) SetVariable(ctx context.Context, variablesRef int, name, value string) (string, error) {
	newValue, err := v.session.SetVariable(ctx, variablesRef, name, value)
	if err != nil {
		return "", err
	}

	// Invalidate cache for this reference
	v.mu.Lock()
	delete(v.cache, variablesRef)
	v.mu.Unlock()

	return newValue, nil
}

// Evaluate evaluates an expression in the given context.
func (v *VariableInspector) Evaluate(ctx context.Context, expression string, frameID int, evalContext string) (*Variable, error) {
	result, err := v.session.Evaluate(ctx, expression, frameID, evalContext)
	if err != nil {
		return nil, err
	}

	return &Variable{
		Name:               expression,
		Value:              result.Result,
		Type:               result.Type,
		VariablesReference: result.VariablesReference,
		NamedVariables:     result.NamedVariables,
		IndexedVariables:   result.IndexedVariables,
		MemoryReference:    result.MemoryReference,
		PresentationHint:   result.PresentationHint,
	}, nil
}

// EvaluateForHover evaluates an expression for hover display.
func (v *VariableInspector) EvaluateForHover(ctx context.Context, expression string, frameID int) (*Variable, error) {
	caps := v.session.Capabilities()
	if caps == nil || !caps.SupportsEvaluateForHovers {
		return nil, fmt.Errorf("hover evaluation not supported")
	}

	return v.Evaluate(ctx, expression, frameID, "hover")
}

// EvaluateForWatch evaluates an expression for watch display.
func (v *VariableInspector) EvaluateForWatch(ctx context.Context, expression string, frameID int) (*Variable, error) {
	return v.Evaluate(ctx, expression, frameID, "watch")
}

// EvaluateForRepl evaluates an expression in REPL context.
func (v *VariableInspector) EvaluateForRepl(ctx context.Context, expression string, frameID int) (*Variable, error) {
	return v.Evaluate(ctx, expression, frameID, "repl")
}

// AddWatch adds a watch expression.
func (v *VariableInspector) AddWatch(expression string) {
	v.mu.Lock()
	v.watches = append(v.watches, expression)
	v.mu.Unlock()
}

// RemoveWatch removes a watch expression by index.
func (v *VariableInspector) RemoveWatch(index int) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if index < 0 || index >= len(v.watches) {
		return fmt.Errorf("watch index %d out of range", index)
	}

	v.watches = append(v.watches[:index], v.watches[index+1:]...)
	if index < len(v.watchResults) {
		v.watchResults = append(v.watchResults[:index], v.watchResults[index+1:]...)
	}

	return nil
}

// ClearWatches removes all watch expressions.
func (v *VariableInspector) ClearWatches() {
	v.mu.Lock()
	v.watches = nil
	v.watchResults = nil
	v.mu.Unlock()
}

// GetWatches returns the current watch expressions.
func (v *VariableInspector) GetWatches() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]string, len(v.watches))
	copy(result, v.watches)
	return result
}

// GetWatchResults returns the last evaluated watch results.
func (v *VariableInspector) GetWatchResults() []*Variable {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]*Variable, len(v.watchResults))
	copy(result, v.watchResults)
	return result
}

// UpdateWatches evaluates all watch expressions.
func (v *VariableInspector) UpdateWatches(ctx context.Context, frameID int) error {
	v.mu.Lock()
	watches := make([]string, len(v.watches))
	copy(watches, v.watches)
	v.mu.Unlock()

	results := make([]*Variable, len(watches))
	for i, expr := range watches {
		result, err := v.EvaluateForWatch(ctx, expr, frameID)
		if err != nil {
			// Store error as value
			results[i] = &Variable{
				Name:  expr,
				Value: fmt.Sprintf("<error: %v>", err),
				Type:  "error",
			}
		} else {
			results[i] = result
		}
	}

	v.mu.Lock()
	v.watchResults = results
	v.mu.Unlock()

	return nil
}

// ClearCache clears the variable cache.
func (v *VariableInspector) ClearCache() {
	v.mu.Lock()
	v.cache = make(map[int][]*Variable)
	v.mu.Unlock()
}

// GetVariablesWithFilter returns variables matching a filter.
func (v *VariableInspector) GetVariablesWithFilter(ctx context.Context, variablesRef int, filter string) ([]*Variable, error) {
	args := dap.VariablesArguments{
		VariablesReference: variablesRef,
		Filter:             filter,
	}

	vars, err := v.session.client.Variables(ctx, args)
	if err != nil {
		return nil, err
	}

	result := make([]*Variable, len(vars))
	for i, variable := range vars {
		result[i] = v.dapVariableToVariable(variable)
	}

	return result, nil
}

// GetVariablesPaged returns a page of variables.
func (v *VariableInspector) GetVariablesPaged(ctx context.Context, variablesRef, start, count int) ([]*Variable, error) {
	args := dap.VariablesArguments{
		VariablesReference: variablesRef,
		Start:              start,
		Count:              count,
	}

	vars, err := v.session.client.Variables(ctx, args)
	if err != nil {
		return nil, err
	}

	result := make([]*Variable, len(vars))
	for i, variable := range vars {
		result[i] = v.dapVariableToVariable(variable)
	}

	return result, nil
}

// FindVariable searches for a variable by name in a scope.
func (v *VariableInspector) FindVariable(ctx context.Context, variablesRef int, name string) (*Variable, error) {
	vars, err := v.GetVariables(ctx, variablesRef)
	if err != nil {
		return nil, err
	}

	for _, variable := range vars {
		if variable.Name == name {
			return variable, nil
		}
	}

	return nil, fmt.Errorf("variable %s not found", name)
}

// GetVariablePath returns the path of a variable from root to the variable.
func (v *VariableInspector) GetVariablePath(variable *Variable) []string {
	var path []string
	current := variable
	for current != nil {
		path = append([]string{current.Name}, path...)
		current = current.Parent
	}
	return path
}

// FormatVariable returns a formatted string representation of a variable.
func (v *VariableInspector) FormatVariable(variable *Variable) string {
	if variable.Type != "" {
		return fmt.Sprintf("%s: %s = %s", variable.Name, variable.Type, variable.Value)
	}
	return fmt.Sprintf("%s = %s", variable.Name, variable.Value)
}
