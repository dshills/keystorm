// Package handler provides the handler interface and types for action dispatch.
package handler

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/input"
)

// Handler processes a specific action or set of actions.
type Handler interface {
	// Handle executes the action and returns a result.
	Handle(action input.Action, ctx *execctx.ExecutionContext) Result

	// CanHandle returns true if this handler can process the action.
	CanHandle(actionName string) bool

	// Priority returns the handler priority (higher = checked first).
	Priority() int
}

// HandlerFunc is a function adapter for Handler interface.
// It allows using a simple function as a Handler.
type HandlerFunc struct {
	fn   func(action input.Action, ctx *execctx.ExecutionContext) Result
	prio int
}

// NewHandlerFunc creates a HandlerFunc from a function.
func NewHandlerFunc(fn func(action input.Action, ctx *execctx.ExecutionContext) Result) *HandlerFunc {
	return &HandlerFunc{fn: fn, prio: 0}
}

// NewHandlerFuncWithPriority creates a HandlerFunc with a specified priority.
func NewHandlerFuncWithPriority(fn func(action input.Action, ctx *execctx.ExecutionContext) Result, priority int) *HandlerFunc {
	return &HandlerFunc{fn: fn, prio: priority}
}

// Handle implements Handler.Handle.
func (f *HandlerFunc) Handle(action input.Action, ctx *execctx.ExecutionContext) Result {
	if f.fn == nil {
		return Errorf("handler function is nil")
	}
	return f.fn(action, ctx)
}

// CanHandle implements Handler.CanHandle.
// HandlerFunc always returns true; caller must ensure correct routing.
func (f *HandlerFunc) CanHandle(actionName string) bool {
	return true
}

// Priority implements Handler.Priority.
func (f *HandlerFunc) Priority() int {
	return f.prio
}

// SimpleHandler wraps a function with an explicit action name.
type SimpleHandler struct {
	// ActionName is the name of the action this handler processes.
	ActionName string

	// Fn is the handler function.
	Fn func(action input.Action, ctx *execctx.ExecutionContext) Result

	// Prio is the handler priority.
	Prio int
}

// Handle implements Handler.Handle.
func (h *SimpleHandler) Handle(action input.Action, ctx *execctx.ExecutionContext) Result {
	if h.Fn == nil {
		return Errorf("handler function is nil")
	}
	return h.Fn(action, ctx)
}

// CanHandle implements Handler.CanHandle.
func (h *SimpleHandler) CanHandle(actionName string) bool {
	return actionName == h.ActionName
}

// Priority implements Handler.Priority.
func (h *SimpleHandler) Priority() int {
	return h.Prio
}

// NamespaceHandler handles all actions within a namespace.
// A namespace is the prefix before the first dot (e.g., "cursor" in "cursor.moveDown").
type NamespaceHandler interface {
	// HandleAction handles an action within this namespace.
	HandleAction(action input.Action, ctx *execctx.ExecutionContext) Result

	// CanHandle returns true if this handler can process the action.
	CanHandle(actionName string) bool

	// Namespace returns the namespace prefix (e.g., "cursor", "editor").
	Namespace() string
}

// namespaceAdapter adapts NamespaceHandler to Handler interface.
type namespaceAdapter struct {
	h NamespaceHandler
}

// NewNamespaceAdapter creates a Handler from a NamespaceHandler.
func NewNamespaceAdapter(h NamespaceHandler) Handler {
	return &namespaceAdapter{h: h}
}

func (a *namespaceAdapter) Handle(action input.Action, ctx *execctx.ExecutionContext) Result {
	return a.h.HandleAction(action, ctx)
}

func (a *namespaceAdapter) CanHandle(actionName string) bool {
	return a.h.CanHandle(actionName)
}

func (a *namespaceAdapter) Priority() int {
	return 0
}

// BaseNamespaceHandler provides a base implementation for namespace handlers.
type BaseNamespaceHandler struct {
	namespace string
	actions   map[string]func(action input.Action, ctx *execctx.ExecutionContext) Result
}

// NewBaseNamespaceHandler creates a new BaseNamespaceHandler.
func NewBaseNamespaceHandler(namespace string) *BaseNamespaceHandler {
	return &BaseNamespaceHandler{
		namespace: namespace,
		actions:   make(map[string]func(action input.Action, ctx *execctx.ExecutionContext) Result),
	}
}

// Register registers a handler function for an action name.
func (h *BaseNamespaceHandler) Register(actionName string, fn func(action input.Action, ctx *execctx.ExecutionContext) Result) {
	h.actions[actionName] = fn
}

// Namespace implements NamespaceHandler.Namespace.
func (h *BaseNamespaceHandler) Namespace() string {
	return h.namespace
}

// CanHandle implements NamespaceHandler.CanHandle.
func (h *BaseNamespaceHandler) CanHandle(actionName string) bool {
	_, ok := h.actions[actionName]
	return ok
}

// HandleAction implements NamespaceHandler.HandleAction.
func (h *BaseNamespaceHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) Result {
	fn, ok := h.actions[action.Name]
	if !ok {
		return Errorf("unknown action in namespace %s: %s", h.namespace, action.Name)
	}
	return fn(action, ctx)
}
