package keymap

import (
	"fmt"
	"sort"
	"sync"

	"github.com/dshills/keystorm/internal/input/key"
)

// Registry manages all keymaps and provides binding lookup.
type Registry struct {
	mu sync.RWMutex

	// keymaps holds all registered keymaps by name.
	keymaps map[string]*ParsedKeymap

	// prefixTree provides efficient prefix-based lookup.
	prefixTree *PrefixTree

	// conditionEvaluator evaluates "when" conditions.
	conditionEvaluator ConditionEvaluator
}

// ConditionEvaluator evaluates binding conditions.
type ConditionEvaluator interface {
	// Evaluate evaluates a condition expression against the current context.
	Evaluate(condition string, ctx *LookupContext) bool
}

// LookupContext provides context for binding lookup.
type LookupContext struct {
	// Mode is the current mode.
	Mode string

	// FileType is the current file type (e.g., "go", "python").
	FileType string

	// Conditions holds current condition values.
	// Keys: "editorTextFocus", "editorReadonly", etc.
	Conditions map[string]bool

	// Variables holds context variables.
	// Keys: "resourceLangId", "activeEditor", etc.
	Variables map[string]string
}

// NewLookupContext creates a new lookup context.
func NewLookupContext() *LookupContext {
	return &LookupContext{
		Conditions: make(map[string]bool),
		Variables:  make(map[string]string),
	}
}

// NewRegistry creates a new keymap registry.
func NewRegistry() *Registry {
	return &Registry{
		keymaps:            make(map[string]*ParsedKeymap),
		prefixTree:         NewPrefixTree(),
		conditionEvaluator: &DefaultConditionEvaluator{},
	}
}

// SetConditionEvaluator sets the condition evaluator.
func (r *Registry) SetConditionEvaluator(eval ConditionEvaluator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conditionEvaluator = eval
}

// Register adds a keymap to the registry.
// If a keymap with the same name already exists, it is replaced.
func (r *Registry) Register(km *Keymap) error {
	if km == nil {
		return fmt.Errorf("cannot register nil keymap")
	}

	parsed, err := km.Parse()
	if err != nil {
		return fmt.Errorf("parsing keymap %q: %w", km.Name, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove existing keymap with same name if present
	r.unregisterLocked(km.Name)

	r.keymaps[km.Name] = parsed

	// Index all bindings in the prefix tree
	for i := range parsed.ParsedBindings {
		pb := &parsed.ParsedBindings[i]
		r.prefixTree.Insert(pb.Sequence, km.Mode, pb, km)
	}

	return nil
}

// Unregister removes a keymap from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.unregisterLocked(name)
}

// unregisterLocked removes a keymap without acquiring the lock.
// Caller must hold the write lock.
func (r *Registry) unregisterLocked(name string) {
	km, ok := r.keymaps[name]
	if !ok {
		return
	}

	// Remove from prefix tree
	for i := range km.ParsedBindings {
		pb := &km.ParsedBindings[i]
		r.prefixTree.Remove(pb.Sequence, km.Mode, km.Keymap)
	}

	delete(r.keymaps, name)
}

// Get returns a keymap by name.
func (r *Registry) Get(name string) *ParsedKeymap {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.keymaps[name]
}

// Lookup finds the best matching binding for a key sequence.
// If ctx is nil, a default empty context is used.
func (r *Registry) Lookup(seq *key.Sequence, ctx *LookupContext) *Binding {
	if seq == nil {
		return nil
	}
	if ctx == nil {
		ctx = NewLookupContext()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	matches := r.findMatches(seq, ctx)
	if len(matches) == 0 {
		return nil
	}

	// Return highest priority match
	return &matches[0].Binding
}

// LookupAll finds all matching bindings for a key sequence.
// If ctx is nil, a default empty context is used.
func (r *Registry) LookupAll(seq *key.Sequence, ctx *LookupContext) []BindingMatch {
	if seq == nil {
		return nil
	}
	if ctx == nil {
		ctx = NewLookupContext()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.findMatches(seq, ctx)
}

// HasPrefix checks if any binding starts with the given sequence.
// If ctx is nil, a default empty context is used.
func (r *Registry) HasPrefix(seq *key.Sequence, ctx *LookupContext) bool {
	if seq == nil {
		return false
	}
	if ctx == nil {
		ctx = NewLookupContext()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check mode-specific and global bindings
	modes := []string{ctx.Mode, ""}
	for _, mode := range modes {
		if r.prefixTree.HasPrefix(seq, mode) {
			return true
		}
	}
	return false
}

// findMatches finds all matches and sorts by priority.
func (r *Registry) findMatches(seq *key.Sequence, ctx *LookupContext) []BindingMatch {
	matches := make([]BindingMatch, 0)

	// Check mode-specific bindings first, then global
	modes := []string{ctx.Mode, ""}
	for _, mode := range modes {
		entries := r.prefixTree.Lookup(seq, mode)
		for _, entry := range entries {
			// Check filetype match
			if entry.Keymap.FileType != "" && entry.Keymap.FileType != ctx.FileType {
				continue
			}

			// Check condition
			if entry.Binding.When != "" {
				if !r.conditionEvaluator.Evaluate(entry.Binding.When, ctx) {
					continue
				}
			}

			match := BindingMatch{
				ParsedBinding: entry.Binding,
				Keymap:        entry.Keymap,
			}
			match.CalculateScore()
			matches = append(matches, match)
		}
	}

	// Sort by priority (descending)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Less(matches[j])
	})

	return matches
}

// Keymaps returns all registered keymaps.
func (r *Registry) Keymaps() []*ParsedKeymap {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ParsedKeymap, 0, len(r.keymaps))
	for _, km := range r.keymaps {
		result = append(result, km)
	}
	return result
}

// AllBindings returns all bindings for a mode.
func (r *Registry) AllBindings(mode string) []BindingMatch {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matches := make([]BindingMatch, 0)
	for _, km := range r.keymaps {
		if km.Mode != "" && km.Mode != mode {
			continue
		}
		for i := range km.ParsedBindings {
			match := BindingMatch{
				ParsedBinding: &km.ParsedBindings[i],
				Keymap:        km.Keymap,
			}
			match.CalculateScore()
			matches = append(matches, match)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Less(matches[j])
	})

	return matches
}

// PrefixTree provides efficient prefix-based binding lookup.
type PrefixTree struct {
	root *prefixNode
}

type prefixNode struct {
	children map[string]*prefixNode
	entries  []prefixEntry
}

type prefixEntry struct {
	Mode    string
	Binding *ParsedBinding
	Keymap  *Keymap
}

// NewPrefixTree creates a new prefix tree.
func NewPrefixTree() *PrefixTree {
	return &PrefixTree{
		root: &prefixNode{
			children: make(map[string]*prefixNode),
		},
	}
}

// Insert adds a binding to the prefix tree.
func (t *PrefixTree) Insert(seq *key.Sequence, mode string, binding *ParsedBinding, km *Keymap) {
	node := t.root

	// Navigate/create path for each key in sequence
	for _, event := range seq.Events {
		keyStr := event.String()
		child, ok := node.children[keyStr]
		if !ok {
			child = &prefixNode{
				children: make(map[string]*prefixNode),
			}
			node.children[keyStr] = child
		}
		node = child
	}

	// Add entry at final node
	node.entries = append(node.entries, prefixEntry{
		Mode:    mode,
		Binding: binding,
		Keymap:  km,
	})
}

// Remove removes a binding from the prefix tree for a specific keymap.
func (t *PrefixTree) Remove(seq *key.Sequence, mode string, km *Keymap) {
	if seq == nil || len(seq.Events) == 0 {
		return
	}

	// Track path for pruning
	path := make([]*prefixNode, 0, len(seq.Events)+1)
	path = append(path, t.root)

	node := t.root

	// Navigate to the node
	for _, event := range seq.Events {
		keyStr := event.String()
		child, ok := node.children[keyStr]
		if !ok {
			return
		}
		path = append(path, child)
		node = child
	}

	// Remove matching entries (must match both mode and keymap)
	filtered := node.entries[:0]
	for _, entry := range node.entries {
		if !(entry.Mode == mode && entry.Keymap == km) {
			filtered = append(filtered, entry)
		}
	}
	node.entries = filtered

	// Prune empty nodes from leaf to root
	for i := len(path) - 1; i > 0; i-- {
		current := path[i]
		if len(current.entries) == 0 && len(current.children) == 0 {
			parent := path[i-1]
			// Find and remove the child key
			for k, child := range parent.children {
				if child == current {
					delete(parent.children, k)
					break
				}
			}
		} else {
			break // Stop pruning if node is not empty
		}
	}
}

// Lookup finds exact matches for a key sequence.
func (t *PrefixTree) Lookup(seq *key.Sequence, mode string) []prefixEntry {
	node := t.root

	// Navigate to the node
	for _, event := range seq.Events {
		keyStr := event.String()
		child, ok := node.children[keyStr]
		if !ok {
			return nil
		}
		node = child
	}

	// Filter by mode
	result := make([]prefixEntry, 0)
	for _, entry := range node.entries {
		if entry.Mode == mode || entry.Mode == "" {
			result = append(result, entry)
		}
	}
	return result
}

// HasPrefix checks if any binding starts with the given sequence.
func (t *PrefixTree) HasPrefix(seq *key.Sequence, mode string) bool {
	node := t.root

	// Navigate to the node
	for _, event := range seq.Events {
		keyStr := event.String()
		child, ok := node.children[keyStr]
		if !ok {
			return false
		}
		node = child
	}

	// Check if there are children or matching entries
	return len(node.children) > 0 || t.hasMatchingEntry(node, mode)
}

func (t *PrefixTree) hasMatchingEntry(node *prefixNode, mode string) bool {
	for _, entry := range node.entries {
		if entry.Mode == mode || entry.Mode == "" {
			return true
		}
	}
	return false
}

// DefaultConditionEvaluator provides basic condition evaluation.
type DefaultConditionEvaluator struct{}

// Evaluate evaluates a condition expression.
// Supports: condition, !condition, condition1 && condition2, condition1 || condition2
func (e *DefaultConditionEvaluator) Evaluate(condition string, ctx *LookupContext) bool {
	if condition == "" {
		return true
	}
	return e.evaluateExpr(condition, ctx)
}

func (e *DefaultConditionEvaluator) evaluateExpr(expr string, ctx *LookupContext) bool {
	// Simple expression parser
	// This is a basic implementation - a full one would use proper parsing

	// Check for OR
	for i := 0; i < len(expr)-1; i++ {
		if expr[i] == '|' && expr[i+1] == '|' {
			left := e.evaluateExpr(trimSpace(expr[:i]), ctx)
			right := e.evaluateExpr(trimSpace(expr[i+2:]), ctx)
			return left || right
		}
	}

	// Check for AND
	for i := 0; i < len(expr)-1; i++ {
		if expr[i] == '&' && expr[i+1] == '&' {
			left := e.evaluateExpr(trimSpace(expr[:i]), ctx)
			right := e.evaluateExpr(trimSpace(expr[i+2:]), ctx)
			return left && right
		}
	}

	// Check for NOT
	expr = trimSpace(expr)
	if len(expr) > 0 && expr[0] == '!' {
		return !e.evaluateExpr(trimSpace(expr[1:]), ctx)
	}

	// Check for equality comparison
	for i := 0; i < len(expr)-1; i++ {
		if expr[i] == '=' && expr[i+1] == '=' {
			left := trimSpace(expr[:i])
			right := trimSpace(expr[i+2:])
			// Check variables
			if val, ok := ctx.Variables[left]; ok {
				return val == right
			}
			return false
		}
	}

	// Simple condition check
	return ctx.Conditions[expr]
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
