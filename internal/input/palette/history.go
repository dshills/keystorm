package palette

import "sync"

// History tracks recently executed commands.
// Commands are stored in most-recently-used order.
type History struct {
	mu       sync.Mutex
	items    []string
	maxItems int
}

// NewHistory creates a command history with the given capacity.
func NewHistory(maxItems int) *History {
	if maxItems <= 0 {
		maxItems = 100
	}
	return &History{
		items:    make([]string, 0, maxItems),
		maxItems: maxItems,
	}
}

// Add records a command execution.
// If the command was already in history, it is moved to the front.
func (h *History) Add(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove if already present
	for i, item := range h.items {
		if item == id {
			h.items = append(h.items[:i], h.items[i+1:]...)
			break
		}
	}

	// Add to front
	h.items = append([]string{id}, h.items...)

	// Trim to max size
	if len(h.items) > h.maxItems {
		h.items = h.items[:h.maxItems]
	}
}

// Recent returns the most recently used command IDs.
func (h *History) Recent(limit int) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if limit <= 0 || limit > len(h.items) {
		limit = len(h.items)
	}

	result := make([]string, limit)
	copy(result, h.items[:limit])
	return result
}

// Contains checks if a command ID is in history.
func (h *History) Contains(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, item := range h.items {
		if item == id {
			return true
		}
	}
	return false
}

// Position returns the position of a command in history (0 = most recent).
// Returns -1 if not found.
func (h *History) Position(id string) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, item := range h.items {
		if item == id {
			return i
		}
	}
	return -1
}

// Clear removes all history entries.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = h.items[:0]
}

// Len returns the number of items in history.
func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.items)
}

// Remove removes a specific command from history.
func (h *History) Remove(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, item := range h.items {
		if item == id {
			h.items = append(h.items[:i], h.items[i+1:]...)
			return true
		}
	}
	return false
}
