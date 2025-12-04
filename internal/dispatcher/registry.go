package dispatcher

import (
	"sort"
	"sync"

	"github.com/dshills/keystorm/internal/dispatcher/handler"
)

// Registry manages handler registration by exact action name.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string][]handler.Handler // action name -> handlers (sorted by priority)
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string][]handler.Handler),
	}
}

// Register adds a handler for an action name.
// Multiple handlers can be registered for the same action; they are sorted by priority.
func (r *Registry) Register(actionName string, h handler.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	handlers := r.handlers[actionName]
	handlers = append(handlers, h)

	// Sort by priority (descending)
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].Priority() > handlers[j].Priority()
	})

	r.handlers[actionName] = handlers
}

// Unregister removes all handlers for an action name.
func (r *Registry) Unregister(actionName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, actionName)
}

// UnregisterHandler removes a specific handler for an action name.
func (r *Registry) UnregisterHandler(actionName string, h handler.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	handlers := r.handlers[actionName]
	for i, existing := range handlers {
		if existing == h {
			r.handlers[actionName] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

// Get returns the highest priority handler for an action.
// Returns nil if no handler is registered.
func (r *Registry) Get(actionName string) handler.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := r.handlers[actionName]
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

// GetAll returns all handlers for an action.
func (r *Registry) GetAll(actionName string) []handler.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := r.handlers[actionName]
	result := make([]handler.Handler, len(handlers))
	copy(result, handlers)
	return result
}

// Has returns true if a handler is registered for the action.
func (r *Registry) Has(actionName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers[actionName]) > 0
}

// List returns all registered action names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count returns the number of registered actions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

// Clear removes all registered handlers.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = make(map[string][]handler.Handler)
}
