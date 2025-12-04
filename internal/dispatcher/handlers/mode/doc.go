// Package mode provides handlers for mode switching operations.
//
// This package implements Vim-style mode transitions as action handlers
// for the dispatcher. Modes control how user input is interpreted.
//
// # Mode Operations
//
// The ModeHandler type provides mode transitions:
//   - mode.normal (Escape): Switch to normal mode
//   - mode.insert (i): Insert before cursor
//   - mode.insertLineStart (I): Insert at first non-blank
//   - mode.append (a): Append after cursor
//   - mode.appendLineEnd (A): Append at end of line
//   - mode.openBelow (o): Open line below
//   - mode.openAbove (O): Open line above
//   - mode.visual (v): Visual character mode
//   - mode.visualLine (V): Visual line mode
//   - mode.visualBlock (Ctrl-V): Visual block mode
//   - mode.command (:): Command line mode
//   - mode.replace (R): Replace mode
//   - mode.replaceChar (r): Replace single character
//
// # Cursor Behavior
//
// Different modes affect cursor positioning:
//   - Normal mode collapses selections to cursor position
//   - Insert mode keeps cursor at current position
//   - Visual modes create/extend selections
//
// # Multi-cursor Support
//
// All mode operations support multiple cursors:
//   - Mode changes apply to all cursors
//   - Open above/below creates new lines for each cursor
//   - Replace char operates on each cursor position
//
// # Usage
//
// Register the handler with the dispatcher:
//
//	dispatcher.RegisterNamespace("mode", mode.NewModeHandler())
//
// Dispatch mode actions:
//
//	result := dispatcher.Dispatch(input.Action{
//	    Name: mode.ActionInsert,
//	})
package mode
