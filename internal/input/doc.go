// Package input handles all user input processing for the Keystorm editor.
//
// The input package transforms raw user input (keystrokes, mouse actions, commands)
// into structured editor actions. It supports Vim-style modal editing with
// configurable keymaps and extensible command handling.
//
// # Architecture
//
// The input system consists of several cooperating components:
//
//   - Key Event Processing: Parses raw terminal events into normalized KeyEvents
//   - Mode System: Manages editor modes (Normal, Insert, Visual, etc.)
//   - Keymap Registry: Maps key sequences to actions based on mode and context
//   - Command Palette: Provides searchable access to all editor commands
//   - Fuzzy Matcher: Enables quick navigation via fuzzy search
//   - Mouse Handler: Processes mouse clicks, drags, and scrolls
//   - Macro System: Records and replays key sequences
//
// # Key Sequences
//
// The input system supports multi-key sequences like Vim's "g g" (go to top)
// or "d i w" (delete inner word). Sequences are accumulated until they match
// a binding or timeout.
//
// # Modal Editing
//
// By default, Keystorm uses Vim-style modal editing:
//
//   - Normal mode: Navigation and commands
//   - Insert mode: Text entry
//   - Visual mode: Selection (character, line, or block)
//   - Command-line mode: Ex commands
//   - Operator-pending mode: Awaiting motion/text object
//
// Modal editing can be disabled for a more traditional editing experience.
//
// # Usage
//
//	handler := input.NewHandler(input.DefaultConfig())
//
//	// Process key events from the backend
//	for event := range keyEvents {
//	    handler.HandleKeyEvent(event)
//	}
//
//	// Receive actions
//	for action := range handler.Actions() {
//	    dispatcher.Execute(action)
//	}
package input
