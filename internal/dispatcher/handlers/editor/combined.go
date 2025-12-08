// Package editor provides handlers for text editing operations.
package editor

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// CombinedHandler handles all editor operations by delegating to specialized handlers.
type CombinedHandler struct {
	insert *InsertHandler
	delete *DeleteHandler
	yank   *YankHandler
	indent *IndentHandler
}

// NewCombinedHandler creates a handler that combines all editor handlers.
func NewCombinedHandler() *CombinedHandler {
	return &CombinedHandler{
		insert: NewInsertHandler(),
		delete: NewDeleteHandler(),
		yank:   NewYankHandler(),
		indent: NewIndentHandler(),
	}
}

// Namespace returns the editor namespace.
func (h *CombinedHandler) Namespace() string {
	return "editor"
}

// CanHandle returns true if this handler can process the action.
func (h *CombinedHandler) CanHandle(actionName string) bool {
	return h.insert.CanHandle(actionName) ||
		h.delete.CanHandle(actionName) ||
		h.yank.CanHandle(actionName) ||
		h.indent.CanHandle(actionName)
}

// HandleAction processes an editor action by delegating to the appropriate handler.
func (h *CombinedHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if h.insert.CanHandle(action.Name) {
		return h.insert.HandleAction(action, ctx)
	}
	if h.delete.CanHandle(action.Name) {
		return h.delete.HandleAction(action, ctx)
	}
	if h.yank.CanHandle(action.Name) {
		return h.yank.HandleAction(action, ctx)
	}
	if h.indent.CanHandle(action.Name) {
		return h.indent.HandleAction(action, ctx)
	}

	return handler.Errorf("unknown editor action: %s", action.Name)
}
