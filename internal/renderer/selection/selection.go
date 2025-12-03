// Package selection provides selection rendering and management.
package selection

import (
	"sort"
	"sync"

	"github.com/dshills/keystorm/internal/renderer/core"
)

// Type represents the type of selection.
type Type uint8

const (
	// TypeNormal is a regular character-based selection.
	TypeNormal Type = iota
	// TypeLine is a line-based selection (selects entire lines).
	TypeLine
	// TypeBlock is a block/column selection (rectangular).
	TypeBlock
)

// Range represents a selection range in buffer coordinates.
type Range struct {
	// Start is the beginning of the selection (anchor point).
	Start Position
	// End is the end of the selection (active cursor).
	End Position
	// Type is the selection type.
	Type Type
}

// Position represents a position in buffer coordinates.
type Position struct {
	Line   uint32
	Column uint32
}

// IsEmpty returns true if the range selects nothing.
func (r Range) IsEmpty() bool {
	return r.Start.Line == r.End.Line && r.Start.Column == r.End.Column
}

// Normalize returns a range where Start is always before End.
func (r Range) Normalize() Range {
	if r.Start.Line > r.End.Line ||
		(r.Start.Line == r.End.Line && r.Start.Column > r.End.Column) {
		return Range{
			Start: r.End,
			End:   r.Start,
			Type:  r.Type,
		}
	}
	return r
}

// Contains returns true if the given position is within the selection.
func (r Range) Contains(line, col uint32) bool {
	if r.IsEmpty() {
		return false
	}

	norm := r.Normalize()

	switch r.Type {
	case TypeNormal:
		return r.containsNormal(norm, line, col)
	case TypeLine:
		return r.containsLine(norm, line)
	case TypeBlock:
		return r.containsBlock(norm, line, col)
	default:
		return r.containsNormal(norm, line, col)
	}
}

// containsNormal checks if position is in a normal (stream) selection.
func (r Range) containsNormal(norm Range, line, col uint32) bool {
	// Before start line
	if line < norm.Start.Line {
		return false
	}
	// After end line
	if line > norm.End.Line {
		return false
	}
	// On start line, check column
	if line == norm.Start.Line && col < norm.Start.Column {
		return false
	}
	// On end line, check column
	if line == norm.End.Line && col >= norm.End.Column {
		return false
	}
	return true
}

// containsLine checks if position is in a line selection.
func (r Range) containsLine(norm Range, line uint32) bool {
	return line >= norm.Start.Line && line <= norm.End.Line
}

// containsBlock checks if position is in a block (rectangular) selection.
func (r Range) containsBlock(norm Range, line, col uint32) bool {
	if line < norm.Start.Line || line > norm.End.Line {
		return false
	}
	// For block selection, columns are independent of lines
	minCol := norm.Start.Column
	maxCol := norm.End.Column
	if minCol > maxCol {
		minCol, maxCol = maxCol, minCol
	}
	return col >= minCol && col < maxCol
}

// LineRange returns the line range of the selection.
func (r Range) LineRange() (startLine, endLine uint32) {
	norm := r.Normalize()
	return norm.Start.Line, norm.End.Line
}

// Manager manages selections for a buffer.
type Manager struct {
	mu sync.RWMutex

	// Primary selection
	primary Range

	// Additional selections (for multi-cursor support)
	secondary []Range

	// Selection active state
	active bool
}

// NewManager creates a new selection manager.
func NewManager() *Manager {
	return &Manager{
		secondary: make([]Range, 0),
	}
}

// SetPrimary sets the primary selection.
func (m *Manager) SetPrimary(sel Range) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.primary = sel
	m.active = !sel.IsEmpty()
}

// Primary returns the primary selection.
func (m *Manager) Primary() Range {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.primary
}

// StartSelection begins a new selection at the given position.
func (m *Manager) StartSelection(line, col uint32, selType Type) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.primary = Range{
		Start: Position{Line: line, Column: col},
		End:   Position{Line: line, Column: col},
		Type:  selType,
	}
	m.active = true
}

// ExtendSelection extends the current selection to the given position.
func (m *Manager) ExtendSelection(line, col uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return
	}

	m.primary.End = Position{Line: line, Column: col}
}

// Clear clears all selections.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.primary = Range{}
	m.secondary = make([]Range, 0)
	m.active = false
}

// IsActive returns true if there's an active selection.
func (m *Manager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active && !m.primary.IsEmpty()
}

// AddSecondary adds a secondary selection.
func (m *Manager) AddSecondary(sel Range) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secondary = append(m.secondary, sel)
}

// Secondary returns all secondary selections.
func (m *Manager) Secondary() []Range {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Range, len(m.secondary))
	copy(result, m.secondary)
	return result
}

// AllSelections returns all selections (primary + secondary).
func (m *Manager) AllSelections() []Range {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.primary.IsEmpty() && len(m.secondary) == 0 {
		return nil
	}

	result := make([]Range, 0, 1+len(m.secondary))
	if !m.primary.IsEmpty() {
		result = append(result, m.primary)
	}
	result = append(result, m.secondary...)
	return result
}

// ClearSecondary removes all secondary selections.
func (m *Manager) ClearSecondary() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secondary = make([]Range, 0)
}

// Contains returns true if any selection contains the given position.
func (m *Manager) Contains(line, col uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.primary.Contains(line, col) {
		return true
	}

	for _, sel := range m.secondary {
		if sel.Contains(line, col) {
			return true
		}
	}

	return false
}

// SelectionsOnLine returns all selection ranges that affect the given line.
func (m *Manager) SelectionsOnLine(line uint32) []LineSelection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]LineSelection, 0)

	// Check primary
	if ls := m.selectionOnLine(m.primary, line, true); ls != nil {
		result = append(result, *ls)
	}

	// Check secondary
	for _, sel := range m.secondary {
		if ls := m.selectionOnLine(sel, line, false); ls != nil {
			result = append(result, *ls)
		}
	}

	return result
}

// LineSelection represents a selection's intersection with a line.
type LineSelection struct {
	// StartCol is the starting column of the selection on this line.
	StartCol uint32
	// EndCol is the ending column (exclusive) of the selection on this line.
	// If EndCol is 0 and SelectToEnd is true, selection extends to end of line.
	EndCol uint32
	// SelectToEnd indicates if the selection extends to end of line.
	SelectToEnd bool
	// IsPrimary indicates if this is from the primary selection.
	IsPrimary bool
}

// selectionOnLine returns the selection's intersection with the given line.
func (m *Manager) selectionOnLine(sel Range, line uint32, isPrimary bool) *LineSelection {
	if sel.IsEmpty() {
		return nil
	}

	norm := sel.Normalize()

	switch sel.Type {
	case TypeNormal:
		return m.normalSelectionOnLine(norm, line, isPrimary)
	case TypeLine:
		return m.lineSelectionOnLine(norm, line, isPrimary)
	case TypeBlock:
		return m.blockSelectionOnLine(norm, line, isPrimary)
	default:
		return m.normalSelectionOnLine(norm, line, isPrimary)
	}
}

func (m *Manager) normalSelectionOnLine(norm Range, line uint32, isPrimary bool) *LineSelection {
	startLine, endLine := norm.Start.Line, norm.End.Line

	if line < startLine || line > endLine {
		return nil
	}

	var startCol, endCol uint32
	var selectToEnd bool

	if line == startLine && line == endLine {
		// Single line selection
		startCol = norm.Start.Column
		endCol = norm.End.Column
	} else if line == startLine {
		// First line of multi-line selection
		startCol = norm.Start.Column
		selectToEnd = true
	} else if line == endLine {
		// Last line of multi-line selection
		startCol = 0
		endCol = norm.End.Column
	} else {
		// Middle line - entire line selected
		startCol = 0
		selectToEnd = true
	}

	return &LineSelection{
		StartCol:    startCol,
		EndCol:      endCol,
		SelectToEnd: selectToEnd,
		IsPrimary:   isPrimary,
	}
}

func (m *Manager) lineSelectionOnLine(norm Range, line uint32, isPrimary bool) *LineSelection {
	if line < norm.Start.Line || line > norm.End.Line {
		return nil
	}

	return &LineSelection{
		StartCol:    0,
		SelectToEnd: true,
		IsPrimary:   isPrimary,
	}
}

func (m *Manager) blockSelectionOnLine(norm Range, line uint32, isPrimary bool) *LineSelection {
	if line < norm.Start.Line || line > norm.End.Line {
		return nil
	}

	minCol := norm.Start.Column
	maxCol := norm.End.Column
	if minCol > maxCol {
		minCol, maxCol = maxCol, minCol
	}

	return &LineSelection{
		StartCol:  minCol,
		EndCol:    maxCol,
		IsPrimary: isPrimary,
	}
}

// SetType changes the selection type.
func (m *Manager) SetType(selType Type) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.primary.Type = selType
}

// Type returns the current selection type.
func (m *Manager) Type() Type {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.primary.Type
}

// Config holds selection rendering configuration.
type Config struct {
	// PrimaryColor is the background color for primary selections.
	PrimaryColor core.Color
	// SecondaryColor is the background color for secondary selections.
	SecondaryColor core.Color
}

// DefaultConfig returns sensible default selection configuration.
func DefaultConfig() Config {
	return Config{
		PrimaryColor:   core.ColorBlue,
		SecondaryColor: core.ColorCyan,
	}
}

// Renderer handles selection rendering.
type Renderer struct {
	mu     sync.RWMutex
	config Config
}

// NewRenderer creates a new selection renderer.
func NewRenderer(config Config) *Renderer {
	return &Renderer{
		config: config,
	}
}

// Config returns the current configuration.
func (r *Renderer) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig updates the configuration.
func (r *Renderer) SetConfig(config Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// ApplySelection modifies a cell to show selection highlighting.
func (r *Renderer) ApplySelection(cell core.Cell, isPrimary bool) core.Cell {
	r.mu.RLock()
	defer r.mu.RUnlock()

	color := r.config.SecondaryColor
	if isPrimary {
		color = r.config.PrimaryColor
	}

	return core.Cell{
		Rune:  cell.Rune,
		Width: cell.Width,
		Style: core.Style{
			Foreground: cell.Style.Foreground,
			Background: color,
			Attributes: cell.Style.Attributes,
		},
	}
}

// String returns the string representation of a selection type.
func (t Type) String() string {
	switch t {
	case TypeNormal:
		return "normal"
	case TypeLine:
		return "line"
	case TypeBlock:
		return "block"
	default:
		return "normal"
	}
}

// TypeFromString converts a string to a selection type.
func TypeFromString(s string) Type {
	switch s {
	case "normal", "character", "char":
		return TypeNormal
	case "line", "linewise":
		return TypeLine
	case "block", "column", "rectangular":
		return TypeBlock
	default:
		return TypeNormal
	}
}

// MergeOverlapping merges overlapping selections.
func MergeOverlapping(selections []Range) []Range {
	if len(selections) <= 1 {
		return selections
	}

	// Normalize all selections first
	normalized := make([]Range, len(selections))
	for i, sel := range selections {
		normalized[i] = sel.Normalize()
	}

	// Sort by start position
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Start.Line != normalized[j].Start.Line {
			return normalized[i].Start.Line < normalized[j].Start.Line
		}
		return normalized[i].Start.Column < normalized[j].Start.Column
	})

	result := make([]Range, 0, len(normalized))
	current := normalized[0]

	for i := 1; i < len(normalized); i++ {
		next := normalized[i]

		// Check if current and next overlap or are adjacent
		if overlapsOrAdjacent(current, next) {
			// Merge them
			current = mergeRanges(current, next)
		} else {
			result = append(result, current)
			current = next
		}
	}
	result = append(result, current)

	return result
}

// overlapsOrAdjacent returns true if two ranges overlap or are adjacent.
func overlapsOrAdjacent(a, b Range) bool {
	// b starts before or at the end of a
	if b.Start.Line < a.End.Line {
		return true
	}
	if b.Start.Line == a.End.Line && b.Start.Column <= a.End.Column {
		return true
	}
	return false
}

// mergeRanges merges two overlapping ranges.
func mergeRanges(a, b Range) Range {
	// Start is the minimum
	start := a.Start
	if b.Start.Line < start.Line || (b.Start.Line == start.Line && b.Start.Column < start.Column) {
		start = b.Start
	}

	// End is the maximum
	end := a.End
	if b.End.Line > end.Line || (b.End.Line == end.Line && b.End.Column > end.Column) {
		end = b.End
	}

	return Range{
		Start: start,
		End:   end,
		Type:  a.Type, // Keep the type of the first range
	}
}
