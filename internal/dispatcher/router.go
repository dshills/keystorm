package dispatcher

import (
	"strings"
	"sync"

	"github.com/dshills/keystorm/internal/dispatcher/handler"
)

// Router routes actions to handlers using namespace prefixes.
// It provides O(1) lookup for namespaced actions like "cursor.moveDown".
type Router struct {
	mu sync.RWMutex

	// Namespace handlers (e.g., "cursor" handles "cursor.*")
	namespaces map[string]handler.NamespaceHandler

	// Fallback handler for unmatched actions
	fallback handler.Handler
}

// NewRouter creates a new action router.
func NewRouter() *Router {
	return &Router{
		namespaces: make(map[string]handler.NamespaceHandler),
	}
}

// RegisterNamespace registers a handler for all actions in a namespace.
// The namespace is the prefix before the first dot (e.g., "cursor" in "cursor.moveDown").
func (r *Router) RegisterNamespace(namespace string, h handler.NamespaceHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.namespaces[namespace] = h
}

// UnregisterNamespace removes a namespace handler.
func (r *Router) UnregisterNamespace(namespace string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.namespaces, namespace)
}

// SetFallback sets the fallback handler for unmatched actions.
func (r *Router) SetFallback(h handler.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = h
}

// Route finds the appropriate handler for an action.
// Returns nil if no handler is found.
func (r *Router) Route(actionName string) handler.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Extract namespace prefix
	namespace := extractNamespace(actionName)
	if namespace != "" {
		if h, ok := r.namespaces[namespace]; ok {
			if h.CanHandle(actionName) {
				return handler.NewNamespaceAdapter(h)
			}
		}
	}

	// Fallback
	return r.fallback
}

// GetNamespaceHandler returns the handler for a namespace.
// Returns nil if no handler is registered.
func (r *Router) GetNamespaceHandler(namespace string) handler.NamespaceHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.namespaces[namespace]
}

// HasNamespace returns true if a handler is registered for the namespace.
func (r *Router) HasNamespace(namespace string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.namespaces[namespace]
	return ok
}

// Namespaces returns all registered namespace names.
func (r *Router) Namespaces() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.namespaces))
	for name := range r.namespaces {
		names = append(names, name)
	}
	return names
}

// CanRoute returns true if the router can handle the action.
func (r *Router) CanRoute(actionName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	namespace := extractNamespace(actionName)
	if namespace != "" {
		if h, ok := r.namespaces[namespace]; ok {
			return h.CanHandle(actionName)
		}
	}

	return r.fallback != nil
}

// extractNamespace extracts the namespace from "namespace.action" format.
// Returns empty string if no namespace separator is found.
func extractNamespace(actionName string) string {
	idx := strings.Index(actionName, ".")
	if idx < 0 {
		return ""
	}
	return actionName[:idx]
}

// ExtractActionName extracts the action name without namespace.
// For "cursor.moveDown", returns "moveDown".
// For actions without namespace, returns the full name.
func ExtractActionName(fullName string) string {
	idx := strings.Index(fullName, ".")
	if idx < 0 {
		return fullName
	}
	return fullName[idx+1:]
}

// BuildActionName builds a full action name from namespace and action.
// For "cursor" and "moveDown", returns "cursor.moveDown".
func BuildActionName(namespace, action string) string {
	if namespace == "" {
		return action
	}
	return namespace + "." + action
}
