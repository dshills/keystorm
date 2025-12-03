// Package history provides undo/redo functionality for the text editor engine.
//
// The history system uses the Command pattern to encapsulate edit operations,
// enabling them to be executed, undone, and redone. Key concepts:
//
// # Operations
//
// An Operation represents a single atomic edit with before/after state:
//   - The range that was modified
//   - The old and new text
//   - Cursor positions before and after
//
// # Commands
//
// Commands implement the Command interface with Execute and Undo methods.
// Built-in commands include:
//   - InsertCommand: Insert text at cursor positions
//   - DeleteCommand: Delete selected text or characters
//   - ReplaceCommand: Replace text in a range
//   - CompoundCommand: Group multiple commands as one undo unit
//
// # History Stack
//
// The History type manages undo/redo stacks and command grouping:
//
//	history := NewHistory(1000) // Max 1000 undo entries
//
//	// Execute commands
//	history.Execute(cmd, buffer, cursors)
//
//	// Undo/redo
//	history.Undo(buffer, cursors)
//	history.Redo(buffer, cursors)
//
// # Command Grouping
//
// Multiple commands can be grouped as a single undo unit:
//
//	history.BeginGroup("Find and Replace")
//	// ... multiple edits ...
//	history.EndGroup()
//
// Now all edits undo together with one Ctrl+Z.
//
// # Cursor Restoration
//
// Commands track cursor positions before and after execution,
// enabling proper cursor restoration on undo/redo.
package history
