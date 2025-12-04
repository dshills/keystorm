// Package mouse provides mouse input handling for the Keystorm editor.
//
// The mouse package handles all mouse-related input including clicks,
// double/triple clicks, drag operations, and scroll wheel events. It
// translates raw mouse events into editor actions.
//
// # Core Types
//
// MouseEvent represents a raw mouse input event with position, button,
// modifiers, and action type:
//
//	event := mouse.Event{
//	    X:         100,
//	    Y:         50,
//	    Button:    mouse.ButtonLeft,
//	    Modifiers: key.ModNone,
//	    Action:    mouse.ActionPress,
//	    Timestamp: time.Now(),
//	}
//
// # Handler
//
// Handler processes mouse events and generates editor actions:
//
//	handler := mouse.NewHandler(mouse.DefaultConfig())
//	action := handler.Handle(event, context)
//	if action != nil {
//	    dispatch(action)
//	}
//
// # Click Detection
//
// The handler automatically detects single, double, and triple clicks
// based on timing and position thresholds:
//
//   - Single click: Positions cursor
//   - Double click: Selects word
//   - Triple click: Selects line
//
// # Drag Handling
//
// When the mouse is moved while a button is held, the handler tracks
// drag state and generates selection extension actions:
//
//	// Drag events extend selection from initial click position
//	// to current mouse position
//
// # Scroll Handling
//
// Scroll wheel events are translated to scroll actions with configurable
// line counts and modifier support:
//
//   - Normal scroll: Scroll by configured line count
//   - Shift+scroll: Scroll by single line
//   - Ctrl+scroll: Zoom in/out
//
// # Thread Safety
//
// Handler is safe for concurrent use. All state mutations are properly
// synchronized with mutex protection.
package mouse
