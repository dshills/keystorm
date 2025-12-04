// Package dispatcher routes input actions to handlers and coordinates execution.
//
// The dispatcher is the central hub that connects user input to editor functionality.
// It receives actions from the input system and routes them to appropriate handlers
// based on action names and namespace prefixes.
//
// # Architecture
//
// The dispatcher uses a two-tier routing system:
//
//  1. Namespace Router: Routes actions by namespace prefix (e.g., "cursor.moveDown"
//     is routed to the "cursor" namespace handler). This provides O(1) lookup
//     for namespaced actions.
//
//  2. Handler Registry: Maps exact action names to handlers. Multiple handlers
//     can be registered for the same action, sorted by priority.
//
// # Handler Execution
//
// When an action is dispatched:
//
//  1. Pre-dispatch hooks are called (can modify or cancel the action)
//  2. The router finds the appropriate handler
//  3. An ExecutionContext is built with references to editor subsystems
//  4. The handler is executed (with optional panic recovery)
//  5. The result is processed (mode changes, view updates)
//  6. Post-dispatch hooks are called
//  7. Metrics are recorded (if enabled)
//
// # Handlers
//
// Handlers implement the Handler interface:
//
//	type Handler interface {
//	    Handle(action input.Action, ctx *execctx.ExecutionContext) Result
//	    CanHandle(actionName string) bool
//	    Priority() int
//	}
//
// For namespace-based handlers, implement NamespaceHandler:
//
//	type NamespaceHandler interface {
//	    HandleAction(action input.Action, ctx *execctx.ExecutionContext) Result
//	    CanHandle(actionName string) bool
//	    Namespace() string
//	}
//
// # Execution Context
//
// The ExecutionContext provides handlers with access to:
//   - Engine: Text buffer operations (insert, delete, read)
//   - Cursors: Cursor/selection state management
//   - ModeManager: Editor mode state (normal, insert, visual)
//   - History: Undo/redo grouping
//   - Renderer: View operations (scroll, redraw)
//   - Input: Input context (pending state, counts)
//
// # Usage
//
// Basic setup:
//
//	dispatcher := dispatcher.NewWithDefaults()
//	dispatcher.SetEngine(engine)
//	dispatcher.SetCursors(cursors)
//	dispatcher.SetModeManager(modeManager)
//
//	// Register handlers
//	dispatcher.RegisterNamespace("cursor", cursorHandler)
//	dispatcher.RegisterHandler("editor.save", saveHandler)
//
//	// Dispatch actions
//	result := dispatcher.Dispatch(action)
//
// With async dispatch:
//
//	config := dispatcher.DefaultConfig().WithAsyncDispatch(100)
//	dispatcher := dispatcher.New(config)
//	dispatcher.Start()
//
//	dispatcher.Actions() <- action
//	result := <-dispatcher.Results()
//
//	dispatcher.Stop()
//
// # Hooks
//
// Pre-dispatch hooks can modify or cancel actions:
//
//	dispatcher.RegisterPreHook(PreDispatchFunc(func(action *input.Action, ctx *execctx.ExecutionContext) bool {
//	    // Return false to cancel
//	    return true
//	}))
//
// Post-dispatch hooks can observe or modify results:
//
//	dispatcher.RegisterPostHook(PostDispatchFunc(func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
//	    // Log, audit, etc.
//	}))
package dispatcher
