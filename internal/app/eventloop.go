// Package app provides the main application structure and coordination.
package app

import (
	"strings"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/input/key"
	"github.com/dshills/keystorm/internal/input/mode"
	"github.com/dshills/keystorm/internal/renderer/backend"
)

// editingActionPrefixes contains action name prefixes that modify document content.
var editingActionPrefixes = []string{
	"editor.insert",
	"editor.delete",
	"editor.backspace",
	"editor.newline",
	"editor.indent",
	"editor.unindent",
	"editor.yank",
	"editor.paste",
	"editor.change",
	"editor.substitute",
	"editor.replace",
	"editor.join",
	"editor.toggle",
}

// handleBackendEvent processes a backend event and routes it appropriately.
// Returns ErrQuit if the application should exit.
func (app *Application) handleBackendEvent(ev backend.Event) error {
	switch ev.Type {
	case backend.EventResize:
		return app.handleResize(ev)
	case backend.EventKey:
		return app.handleKeyEvent(ev)
	case backend.EventMouse:
		return app.handleMouseEvent(ev)
	case backend.EventPaste:
		return app.handlePasteEvent(ev)
	case backend.EventFocus:
		return app.handleFocusEvent(ev)
	default:
		return nil
	}
}

// handleResize processes terminal resize events.
func (app *Application) handleResize(ev backend.Event) error {
	if app.renderer != nil {
		app.renderer.Resize(ev.Width, ev.Height)
	}
	return nil
}

// handleKeyEvent processes keyboard input events.
func (app *Application) handleKeyEvent(ev backend.Event) error {
	// Convert backend event to key.Event
	keyEv := app.convertToKeyEvent(ev)

	// Let mode manager handle the key
	if app.modeManager == nil {
		return nil
	}

	currentMode := app.modeManager.Current()
	if currentMode == nil {
		return nil
	}

	// Try to handle unmapped key
	modeCtx := app.buildModeContext()
	result := currentMode.HandleUnmapped(keyEv, modeCtx)
	if result == nil {
		return nil
	}

	// Process the result
	return app.processModeResult(result, keyEv)
}

// handleMouseEvent processes mouse input events.
func (app *Application) handleMouseEvent(_ backend.Event) error {
	// Mouse handling will be implemented in a future phase
	// For now, we just ignore mouse events
	return nil
}

// handlePasteEvent processes paste events.
func (app *Application) handlePasteEvent(ev backend.Event) error {
	if ev.PasteText == "" {
		return nil
	}

	// Get active document
	doc := app.documents.Active()
	if doc == nil || doc.ReadOnly {
		return nil
	}

	// Insert pasted text at cursor position
	// This is a simplified implementation - a full implementation would
	// handle this through the dispatcher with proper undo grouping
	if doc.Engine != nil {
		// Get cursor position
		cursors := doc.Engine.Cursors()
		if cursors != nil && cursors.Count() > 0 {
			primary := cursors.Primary()
			offset := primary.Head // Use Head as the cursor position

			_, err := doc.Engine.Insert(offset, ev.PasteText)
			if err == nil {
				doc.SetModified(true)
				doc.IncrementVersion()
			}
		}
	}

	return nil
}

// handleFocusEvent processes focus change events.
func (app *Application) handleFocusEvent(_ backend.Event) error {
	// Focus handling will be implemented in a future phase
	// Could be used to pause/resume certain operations
	return nil
}

// convertToKeyEvent converts a backend.Event to a key.Event.
func (app *Application) convertToKeyEvent(ev backend.Event) key.Event {
	// Map backend key to key.Key
	k := mapBackendKey(ev.Key, ev.Rune)

	// Map modifiers
	mods := key.ModNone
	if ev.Mod.Has(backend.ModCtrl) {
		mods = mods.With(key.ModCtrl)
	}
	if ev.Mod.Has(backend.ModAlt) {
		mods = mods.With(key.ModAlt)
	}
	if ev.Mod.Has(backend.ModShift) {
		mods = mods.With(key.ModShift)
	}
	if ev.Mod.Has(backend.ModMeta) {
		mods = mods.With(key.ModMeta)
	}

	return key.NewEvent(k, ev.Rune, mods)
}

// mapBackendKey maps a backend.Key to a key.Key.
func mapBackendKey(bk backend.Key, r rune) key.Key {
	switch bk {
	case backend.KeyRune:
		return key.KeyRune
	case backend.KeyEscape:
		return key.KeyEscape
	case backend.KeyEnter:
		return key.KeyEnter
	case backend.KeyTab:
		return key.KeyTab
	case backend.KeyBackspace:
		return key.KeyBackspace
	case backend.KeyDelete:
		return key.KeyDelete
	case backend.KeyInsert:
		return key.KeyInsert
	case backend.KeyHome:
		return key.KeyHome
	case backend.KeyEnd:
		return key.KeyEnd
	case backend.KeyPageUp:
		return key.KeyPageUp
	case backend.KeyPageDown:
		return key.KeyPageDown
	case backend.KeyUp:
		return key.KeyUp
	case backend.KeyDown:
		return key.KeyDown
	case backend.KeyLeft:
		return key.KeyLeft
	case backend.KeyRight:
		return key.KeyRight
	case backend.KeyF1:
		return key.KeyF1
	case backend.KeyF2:
		return key.KeyF2
	case backend.KeyF3:
		return key.KeyF3
	case backend.KeyF4:
		return key.KeyF4
	case backend.KeyF5:
		return key.KeyF5
	case backend.KeyF6:
		return key.KeyF6
	case backend.KeyF7:
		return key.KeyF7
	case backend.KeyF8:
		return key.KeyF8
	case backend.KeyF9:
		return key.KeyF9
	case backend.KeyF10:
		return key.KeyF10
	case backend.KeyF11:
		return key.KeyF11
	case backend.KeyF12:
		return key.KeyF12
	case backend.KeyCtrlA:
		return key.KeyRune // Will be handled via modifier
	case backend.KeyCtrlB:
		return key.KeyRune
	case backend.KeyCtrlC:
		return key.KeyRune
	case backend.KeyCtrlD:
		return key.KeyRune
	case backend.KeyCtrlE:
		return key.KeyRune
	case backend.KeyCtrlF:
		return key.KeyRune
	case backend.KeyCtrlG:
		return key.KeyRune
	case backend.KeyCtrlH:
		return key.KeyBackspace // Ctrl+H is often backspace
	case backend.KeyCtrlI:
		return key.KeyTab // Ctrl+I is tab
	case backend.KeyCtrlJ:
		return key.KeyEnter // Ctrl+J is often enter
	case backend.KeyCtrlK:
		return key.KeyRune
	case backend.KeyCtrlL:
		return key.KeyRune
	case backend.KeyCtrlM:
		return key.KeyEnter // Ctrl+M is carriage return
	case backend.KeyCtrlN:
		return key.KeyRune
	case backend.KeyCtrlO:
		return key.KeyRune
	case backend.KeyCtrlP:
		return key.KeyRune
	case backend.KeyCtrlQ:
		return key.KeyRune
	case backend.KeyCtrlR:
		return key.KeyRune
	case backend.KeyCtrlS:
		return key.KeyRune
	case backend.KeyCtrlT:
		return key.KeyRune
	case backend.KeyCtrlU:
		return key.KeyRune
	case backend.KeyCtrlV:
		return key.KeyRune
	case backend.KeyCtrlW:
		return key.KeyRune
	case backend.KeyCtrlX:
		return key.KeyRune
	case backend.KeyCtrlY:
		return key.KeyRune
	case backend.KeyCtrlZ:
		return key.KeyRune
	default:
		if r != 0 {
			return key.KeyRune
		}
		return key.KeyNone
	}
}

// processModeResult handles the result of an unmapped key press.
func (app *Application) processModeResult(result *mode.UnmappedResult, _ key.Event) error {
	if result == nil {
		return nil
	}

	// Handle action dispatch
	if result.Action != nil {
		action := &input.Action{
			Name: result.Action.Name,
			Args: convertModeArgs(result.Action.Args),
		}

		// Check for mode change action
		if action.Name == "mode.normal" || action.Name == "mode.insert" ||
			action.Name == "mode.visual" || action.Name == "mode.command" ||
			action.Name == "mode.replace" {
			modeName := action.Name[5:] // Remove "mode." prefix
			if err := app.modeManager.SetInitialMode(modeName); err != nil {
				_ = err // Log but don't fail
			}
			return nil
		}

		return app.dispatchAction(action)
	}

	// Handle text insertion in insert mode
	if result.InsertText != "" {
		return app.insertText(result.InsertText)
	}

	return nil
}

// convertModeArgs converts mode.Action.Args to input.ActionArgs.
func convertModeArgs(args map[string]any) input.ActionArgs {
	result := input.ActionArgs{}
	if args != nil {
		result.Extra = make(map[string]interface{})
		for k, v := range args {
			result.Extra[k] = v
		}
	}
	return result
}

// insertText inserts text at the cursor position.
func (app *Application) insertText(text string) error {
	if text == "" {
		return nil
	}
	doc := app.documents.Active()
	if doc == nil || doc.ReadOnly || doc.Engine == nil {
		return nil
	}

	cursors := doc.Engine.Cursors()
	if cursors == nil || cursors.Count() == 0 {
		return nil
	}

	// Insert at primary cursor
	primary := cursors.Primary()
	_, err := doc.Engine.Insert(primary.Head, text)
	if err != nil {
		return err
	}

	doc.SetModified(true)
	doc.IncrementVersion()

	return nil
}

// dispatchAction sends an action through the dispatcher.
func (app *Application) dispatchAction(action *input.Action) error {
	if app.dispatcher == nil || action == nil {
		return nil
	}

	// Build input context
	inputCtx := app.buildInputContext()

	// Dispatch the action
	result := app.dispatcher.DispatchWithContext(*action, inputCtx)

	// Check for quit action
	if action.Name == "app.quit" || action.Name == "quit" {
		return ErrQuit
	}

	// Handle errors from dispatch
	if result.Error != nil {
		// Log error but don't fail the application
		// In a full implementation, this would show an error message
		_ = result.Error
	}

	// Mark document as modified if action changed content
	if result.Status == handler.StatusOK {
		doc := app.documents.Active()
		if doc != nil && !doc.ReadOnly {
			// Check if this was an editing action
			if isEditingAction(action.Name) {
				doc.SetModified(true)
				doc.IncrementVersion()
			}
		}
	}

	return nil
}

// insertCharacter inserts a character at the cursor position.
func (app *Application) insertCharacter(ch rune) error {
	doc := app.documents.Active()
	if doc == nil || doc.ReadOnly || doc.Engine == nil {
		return nil
	}

	cursors := doc.Engine.Cursors()
	if cursors == nil || cursors.Count() == 0 {
		return nil
	}

	// Insert at primary cursor
	primary := cursors.Primary()
	_, err := doc.Engine.Insert(primary.Head, string(ch))
	if err != nil {
		return err
	}

	doc.SetModified(true)
	doc.IncrementVersion()

	return nil
}

// buildInputContext creates an input.Context for dispatcher.
func (app *Application) buildInputContext() *input.Context {
	ctx := &input.Context{}

	// Set mode
	if app.modeManager != nil && app.modeManager.Current() != nil {
		ctx.Mode = app.modeManager.Current().Name()
	}

	// Set document info
	doc := app.documents.Active()
	if doc != nil {
		ctx.FilePath = doc.Path
		ctx.FileType = doc.LanguageID
		ctx.IsModified = doc.IsModified()
		ctx.IsReadOnly = doc.ReadOnly

		if doc.Engine != nil {
			cursors := doc.Engine.Cursors()
			if cursors != nil {
				ctx.HasSelection = cursors.HasSelection()
			}
		}
	}

	return ctx
}

// buildModeContext creates a mode.Context for mode handling.
func (app *Application) buildModeContext() *mode.Context {
	ctx := &mode.Context{}

	// Set previous mode if available
	if app.modeManager != nil && app.modeManager.Current() != nil {
		ctx.PreviousMode = app.modeManager.Current().Name()
	}

	return ctx
}

// buildExecutionContext creates an execution context for the dispatcher.
func (app *Application) buildExecutionContext() *execctx.ExecutionContext {
	ctx := execctx.New()

	doc := app.documents.Active()
	if doc != nil {
		ctx.FilePath = doc.Path
		ctx.FileType = doc.LanguageID

		// Note: Engine/Cursor wiring requires adapters (deferred to Phase 3 adapters)
		// For now, we set basic metadata
	}

	if app.modeManager != nil && app.modeManager.Current() != nil {
		// Mode name is available through the input context
	}

	return ctx
}

// isEditingAction returns true if the action modifies document content.
func isEditingAction(name string) bool {
	for _, prefix := range editingActionPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// startInputPolling starts a goroutine that polls for input events.
// Events are sent to the returned channel.
//
// Note: PollEvent is blocking, so this goroutine may not exit immediately
// on shutdown. The backend should be shutdown to unblock PollEvent.
// Callers should close the done channel and call backend.Shutdown() to
// ensure clean termination.
func (app *Application) startInputPolling() <-chan backend.Event {
	events := make(chan backend.Event, 100)

	go func() {
		defer close(events)

		for app.running.Load() {
			if app.backend == nil {
				return
			}

			// PollEvent is blocking. The backend.Shutdown() call in Run()
			// will unblock this by closing the underlying terminal.
			ev := app.backend.PollEvent()

			// Check if we should stop (may have been signaled during blocking poll)
			if !app.running.Load() {
				return
			}

			// Send event (non-blocking with buffer to avoid deadlock)
			select {
			case events <- ev:
			case <-app.done:
				return
			default:
				// Buffer full, drop event to prevent blocking.
				// This should be rare with buffer size 100.
				// In production, consider logging this at debug level.
			}
		}
	}()

	return events
}
