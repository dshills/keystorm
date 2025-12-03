package cursor

import (
	"sort"
	"sync"
)

// MultiCursorManager manages multiple cursors for multi-cursor editing.
type MultiCursorManager struct {
	mu sync.RWMutex

	// All cursors, with the primary cursor first
	cursors []Cursor

	// Index of the primary cursor in the cursors slice
	primaryIndex int
}

// NewMultiCursorManager creates a new multi-cursor manager.
func NewMultiCursorManager() *MultiCursorManager {
	return &MultiCursorManager{
		cursors:      make([]Cursor, 0),
		primaryIndex: 0,
	}
}

// SetPrimary sets the primary cursor position, clearing all secondary cursors.
func (m *MultiCursorManager) SetPrimary(line, col uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cursors = []Cursor{{
		Position:  Position{Line: line, Column: col},
		IsPrimary: true,
		Visible:   true,
	}}
	m.primaryIndex = 0
}

// AddCursor adds a new secondary cursor at the given position.
// Returns the index of the added cursor.
func (m *MultiCursorManager) AddCursor(line, col uint32) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicates
	pos := Position{Line: line, Column: col}
	for _, c := range m.cursors {
		if c.Position.Line == pos.Line && c.Position.Column == pos.Column {
			return -1 // Cursor already exists at this position
		}
	}

	cursor := Cursor{
		Position:  pos,
		IsPrimary: false,
		Visible:   true,
	}

	m.cursors = append(m.cursors, cursor)
	return len(m.cursors) - 1
}

// AddCursorAbove adds a cursor on the line above the primary cursor.
func (m *MultiCursorManager) AddCursorAbove() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cursors) == 0 {
		return false
	}

	primary := m.cursors[m.primaryIndex]
	if primary.Position.Line == 0 {
		return false // Already at top
	}

	newPos := Position{
		Line:   primary.Position.Line - 1,
		Column: primary.Position.Column,
	}

	// Check for duplicates
	for _, c := range m.cursors {
		if c.Position.Line == newPos.Line && c.Position.Column == newPos.Column {
			return false
		}
	}

	m.cursors = append(m.cursors, Cursor{
		Position:  newPos,
		IsPrimary: false,
		Visible:   true,
	})

	return true
}

// AddCursorBelow adds a cursor on the line below the primary cursor.
func (m *MultiCursorManager) AddCursorBelow() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cursors) == 0 {
		return false
	}

	primary := m.cursors[m.primaryIndex]

	newPos := Position{
		Line:   primary.Position.Line + 1,
		Column: primary.Position.Column,
	}

	// Check for duplicates
	for _, c := range m.cursors {
		if c.Position.Line == newPos.Line && c.Position.Column == newPos.Column {
			return false
		}
	}

	m.cursors = append(m.cursors, Cursor{
		Position:  newPos,
		IsPrimary: false,
		Visible:   true,
	})

	return true
}

// RemoveCursor removes the cursor at the given index.
// Cannot remove the primary cursor if it's the only one.
func (m *MultiCursorManager) RemoveCursor(index int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cursors) {
		return false
	}

	// Don't remove the last cursor
	if len(m.cursors) == 1 {
		return false
	}

	// If removing primary, promote another cursor
	if index == m.primaryIndex {
		// Find a new primary (preferably the next cursor, or previous)
		newPrimary := index + 1
		if newPrimary >= len(m.cursors) {
			newPrimary = index - 1
		}
		m.cursors[newPrimary].IsPrimary = true
		if newPrimary > index {
			m.primaryIndex = newPrimary - 1 // Will shift after removal
		} else {
			m.primaryIndex = newPrimary
		}
	} else if index < m.primaryIndex {
		m.primaryIndex--
	}

	m.cursors = append(m.cursors[:index], m.cursors[index+1:]...)
	return true
}

// RemoveSecondary removes all secondary cursors, keeping only the primary.
func (m *MultiCursorManager) RemoveSecondary() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cursors) <= 1 {
		return
	}

	primary := m.cursors[m.primaryIndex]
	m.cursors = []Cursor{primary}
	m.primaryIndex = 0
}

// Cursors returns all cursors.
func (m *MultiCursorManager) Cursors() []Cursor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Cursor, len(m.cursors))
	copy(result, m.cursors)
	return result
}

// CursorCount returns the number of cursors.
func (m *MultiCursorManager) CursorCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cursors)
}

// HasMultiple returns true if there are multiple cursors.
func (m *MultiCursorManager) HasMultiple() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cursors) > 1
}

// Primary returns the primary cursor.
func (m *MultiCursorManager) Primary() (Cursor, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.cursors) == 0 {
		return Cursor{}, false
	}
	return m.cursors[m.primaryIndex], true
}

// MovePrimary moves the primary cursor to a new position.
func (m *MultiCursorManager) MovePrimary(line, col uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cursors) == 0 {
		m.cursors = []Cursor{{
			Position:  Position{Line: line, Column: col},
			IsPrimary: true,
			Visible:   true,
		}}
		m.primaryIndex = 0
		return
	}

	m.cursors[m.primaryIndex].Position = Position{Line: line, Column: col}
}

// MoveAll moves all cursors by the given delta.
func (m *MultiCursorManager) MoveAll(deltaLine int32, deltaCol int32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.cursors {
		// Handle line delta
		if deltaLine < 0 && uint32(-deltaLine) > m.cursors[i].Position.Line {
			m.cursors[i].Position.Line = 0
		} else {
			m.cursors[i].Position.Line = uint32(int32(m.cursors[i].Position.Line) + deltaLine)
		}

		// Handle column delta
		if deltaCol < 0 && uint32(-deltaCol) > m.cursors[i].Position.Column {
			m.cursors[i].Position.Column = 0
		} else {
			m.cursors[i].Position.Column = uint32(int32(m.cursors[i].Position.Column) + deltaCol)
		}
	}

	// Remove any duplicate positions that may have resulted from movement
	m.deduplicateLocked()
}

// MoveAllToColumn moves all cursors to a specific column.
func (m *MultiCursorManager) MoveAllToColumn(col uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.cursors {
		m.cursors[i].Position.Column = col
	}
}

// SetPrimaryIndex sets which cursor is the primary.
func (m *MultiCursorManager) SetPrimaryIndex(index int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cursors) {
		return false
	}

	// Clear old primary
	m.cursors[m.primaryIndex].IsPrimary = false

	// Set new primary
	m.cursors[index].IsPrimary = true
	m.primaryIndex = index

	return true
}

// CyclePrimary cycles the primary cursor to the next one.
func (m *MultiCursorManager) CyclePrimary() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cursors) <= 1 {
		return
	}

	m.cursors[m.primaryIndex].IsPrimary = false
	m.primaryIndex = (m.primaryIndex + 1) % len(m.cursors)
	m.cursors[m.primaryIndex].IsPrimary = true
}

// SortByPosition sorts cursors by their position (line, then column).
// The primary cursor remains primary but may change index.
func (m *MultiCursorManager) SortByPosition() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cursors) <= 1 {
		return
	}

	// Find primary before sorting
	var primaryPos Position
	for _, c := range m.cursors {
		if c.IsPrimary {
			primaryPos = c.Position
			break
		}
	}

	sort.Slice(m.cursors, func(i, j int) bool {
		if m.cursors[i].Position.Line != m.cursors[j].Position.Line {
			return m.cursors[i].Position.Line < m.cursors[j].Position.Line
		}
		return m.cursors[i].Position.Column < m.cursors[j].Position.Column
	})

	// Find new primary index
	for i, c := range m.cursors {
		if c.Position.Line == primaryPos.Line && c.Position.Column == primaryPos.Column {
			m.primaryIndex = i
			break
		}
	}
}

// deduplicateLocked removes duplicate cursor positions (caller must hold lock).
func (m *MultiCursorManager) deduplicateLocked() {
	if len(m.cursors) <= 1 {
		return
	}

	seen := make(map[Position]int) // Position -> first index with this position
	toRemove := make([]int, 0)

	for i, c := range m.cursors {
		if firstIdx, exists := seen[c.Position]; exists {
			// Duplicate found
			// Keep the primary if one of them is primary
			if c.IsPrimary {
				toRemove = append(toRemove, firstIdx)
				seen[c.Position] = i
			} else {
				toRemove = append(toRemove, i)
			}
		} else {
			seen[c.Position] = i
		}
	}

	if len(toRemove) == 0 {
		return
	}

	// Sort in reverse order to remove from end first
	sort.Sort(sort.Reverse(sort.IntSlice(toRemove)))

	for _, idx := range toRemove {
		if idx == m.primaryIndex && len(m.cursors) > 1 {
			// Need to reassign primary
			newPrimary := 0
			if idx == 0 {
				newPrimary = 1
			}
			m.cursors[newPrimary].IsPrimary = true
			m.primaryIndex = newPrimary
		}
		m.cursors = append(m.cursors[:idx], m.cursors[idx+1:]...)
		if idx < m.primaryIndex {
			m.primaryIndex--
		}
	}
}

// Deduplicate removes any duplicate cursor positions.
func (m *MultiCursorManager) Deduplicate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deduplicateLocked()
}

// CursorsOnLine returns all cursors on the given line.
func (m *MultiCursorManager) CursorsOnLine(line uint32) []Cursor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Cursor, 0)
	for _, c := range m.cursors {
		if c.Position.Line == line {
			result = append(result, c)
		}
	}
	return result
}

// Clear removes all cursors.
func (m *MultiCursorManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cursors = make([]Cursor, 0)
	m.primaryIndex = 0
}

// Clone returns a deep copy of the manager.
func (m *MultiCursorManager) Clone() *MultiCursorManager {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &MultiCursorManager{
		cursors:      make([]Cursor, len(m.cursors)),
		primaryIndex: m.primaryIndex,
	}
	copy(clone.cursors, m.cursors)
	return clone
}
