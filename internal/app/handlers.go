// Package app provides handler registration for the dispatcher.
package app

import (
	"github.com/dshills/keystorm/internal/dispatcher"
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	cursorhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/cursor"
	editorhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/editor"
	modehandler "github.com/dshills/keystorm/internal/dispatcher/handlers/mode"
	operatorhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/operator"
	searchhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/search"
	viewhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/view"
	windowhandler "github.com/dshills/keystorm/internal/dispatcher/handlers/window"
	"github.com/dshills/keystorm/internal/input"
)

// RegisterHandlers registers all standard handlers with the dispatcher.
// This should be called during application bootstrap after the dispatcher is created.
func RegisterHandlers(d *dispatcher.Dispatcher) {
	// Core cursor handler
	d.RegisterNamespace("cursor", cursorhandler.NewHandler())

	// Editor handlers (multiple handlers for different operations)
	d.RegisterNamespace("editor", editorhandler.NewInsertHandler())
	// Note: The router handles multiple handlers per namespace by using
	// a composite or fallback pattern. For now we register insert as primary.
	// Additional editor handlers can be registered individually:
	// - editorhandler.NewDeleteHandler()
	// - editorhandler.NewYankHandler()
	// - editorhandler.NewIndentHandler()

	// Mode handler
	d.RegisterNamespace("mode", modehandler.NewModeHandler())

	// Operator handler
	d.RegisterNamespace("operator", operatorhandler.NewOperatorHandler())

	// Navigation handlers
	d.RegisterNamespace("search", searchhandler.NewHandler())
	d.RegisterNamespace("view", viewhandler.NewHandler())
	d.RegisterNamespace("window", windowhandler.NewHandler())
}

// BuildExecutionContext creates an execctx.ExecutionContext from the application state.
// This bridges the app layer with the dispatcher's handler system.
func (app *Application) BuildExecutionContext() *execctx.ExecutionContext {
	doc := app.documents.Active()
	if doc == nil {
		return execctx.New()
	}

	ctx := execctx.New()

	// Wire engine adapter
	if doc.Engine != nil {
		ctx.Engine = NewEngineExecAdapter(doc.Engine)

		// Wire cursor adapter
		cursors := doc.Engine.Cursors()
		if cursors != nil {
			ctx.Cursors = NewCursorManagerAdapter(cursors)
		}

		// Wire history adapter
		ctx.History = NewHistoryAdapter(doc.Engine)
	}

	// Wire mode manager adapter
	if app.modeManager != nil {
		ctx.ModeManager = NewModeExecAdapter(app.modeManager)
	}

	// Set file info
	ctx.FilePath = doc.Path
	ctx.FileType = doc.LanguageID

	return ctx
}

// ExecuteAction dispatches an action with the current execution context.
// Returns the handler result.
func (app *Application) ExecuteAction(actionName string, count int) error {
	if app.dispatcher == nil {
		return ErrComponentNotAvailable
	}

	doc := app.documents.Active()
	if doc == nil {
		return ErrNoActiveDocument
	}

	// Wire up the dispatcher with current document's adapters
	app.wireDispatcherContext(doc)

	// Build the action
	action := input.Action{
		Name:  actionName,
		Count: count,
	}

	// Dispatch the action
	result := app.dispatcher.Dispatch(action)
	if result.Error != nil {
		return result.Error
	}

	// Mark document as modified if the action made changes (edits were applied)
	if len(result.Edits) > 0 {
		doc.SetModified(true)
	}

	return nil
}

// wireDispatcherContext sets up the dispatcher with the current document's context.
func (app *Application) wireDispatcherContext(doc *Document) {
	if doc == nil || doc.Engine == nil {
		return
	}

	// Wire engine adapter
	app.dispatcher.SetEngine(NewEngineExecAdapter(doc.Engine))

	// Wire cursor adapter
	cursors := doc.Engine.Cursors()
	if cursors != nil {
		app.dispatcher.SetCursors(NewCursorManagerAdapter(cursors))
	}

	// Wire history adapter (engine exposes history operations directly)
	app.dispatcher.SetHistory(NewHistoryAdapter(doc.Engine))

	// Wire mode manager adapter
	if app.modeManager != nil {
		app.dispatcher.SetModeManager(NewModeExecAdapter(app.modeManager))
	}
}

// HandlerInfo provides information about a registered handler.
type HandlerInfo struct {
	Namespace string
}

// ListHandlers returns information about all registered namespaces.
func (app *Application) ListHandlers() []HandlerInfo {
	if app.dispatcher == nil {
		return nil
	}

	router := app.dispatcher.Router()
	if router == nil {
		return nil
	}

	// Get handler namespaces from router
	namespaces := router.Namespaces()
	infos := make([]HandlerInfo, 0, len(namespaces))

	for _, ns := range namespaces {
		infos = append(infos, HandlerInfo{Namespace: ns})
	}

	return infos
}
