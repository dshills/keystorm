// Package cursor provides handlers for cursor movement operations.
package cursor

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// CombinedHandler handles all cursor operations by delegating to specialized handlers.
type CombinedHandler struct {
	basic  *Handler
	motion *MotionHandler
}

// NewCombinedHandler creates a handler that combines basic cursor and motion handlers.
func NewCombinedHandler() *CombinedHandler {
	return &CombinedHandler{
		basic:  NewHandler(),
		motion: NewMotionHandler(),
	}
}

// Namespace returns the cursor namespace.
func (h *CombinedHandler) Namespace() string {
	return "cursor"
}

// CanHandle returns true if this handler can process the action.
func (h *CombinedHandler) CanHandle(actionName string) bool {
	return h.basic.CanHandle(actionName) || h.motion.CanHandle(actionName)
}

// HandleAction processes a cursor action by delegating to the appropriate handler.
func (h *CombinedHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Try basic handler first
	if h.basic.CanHandle(action.Name) {
		return h.basic.HandleAction(action, ctx)
	}

	// Then try motion handler
	if h.motion.CanHandle(action.Name) {
		return h.motion.HandleAction(action, ctx)
	}

	return handler.Errorf("unknown cursor action: %s", action.Name)
}
