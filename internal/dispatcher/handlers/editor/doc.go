// Package editor provides handlers for text editing operations.
//
// This package implements Vim-style text manipulation operations as action
// handlers for the dispatcher. All handlers support multi-cursor operations
// and properly track changes for undo/redo support.
//
// # Insert Operations
//
// The InsertHandler type provides text insertion:
//   - editor.insertChar: Insert a single character at cursor
//   - editor.insertText: Insert arbitrary text at cursor
//   - editor.insertNewline: Insert a newline (Enter key)
//   - editor.insertLineAbove (O): Insert new line above cursor
//   - editor.insertLineBelow (o): Insert new line below cursor
//   - editor.insertTab: Insert tab or spaces based on settings
//
// # Delete Operations
//
// The DeleteHandler type provides text deletion:
//   - editor.deleteChar (x): Delete character under cursor
//   - editor.deleteCharBack (X): Delete character before cursor
//   - editor.deleteLine (dd): Delete entire line(s)
//   - editor.deleteToEnd (D): Delete from cursor to end of line
//   - editor.deleteSelection: Delete selected text
//   - editor.deleteWord (dw): Delete word forward
//   - editor.deleteWordBack (db): Delete word backward
//
// # Yank/Paste Operations
//
// The YankHandler type provides copy/paste functionality:
//   - editor.yankSelection (y): Yank (copy) selected text
//   - editor.yankLine (yy): Yank entire line(s)
//   - editor.yankToEnd (Y): Yank from cursor to end of line
//   - editor.yankWord (yw): Yank word forward
//   - editor.pasteAfter (p): Paste after cursor
//   - editor.pasteBefore (P): Paste before cursor
//
// # Indent Operations
//
// The IndentHandler type provides indentation control:
//   - editor.indent (>>): Increase indentation
//   - editor.outdent (<<): Decrease indentation
//   - editor.autoIndent (=): Auto-indent based on context
//   - editor.indentBlock (>}): Indent paragraph/block
//   - editor.outdentBlock (<{): Outdent paragraph/block
//
// # Multi-cursor Support
//
// All operations support multiple cursors:
//   - Selections are processed in reverse order to maintain offsets
//   - Undo groups are created for multi-cursor edits
//   - Register content captures the last deleted/yanked text
//
// # Undo/Redo Support
//
// Operations that modify text:
//   - Begin undo groups for multi-cursor edits
//   - Track affected lines for efficient redraw
//   - Store deleted text in registers for yank/delete
//
// # Usage
//
// Register handlers with the dispatcher:
//
//	dispatcher.RegisterNamespace("editor", editor.NewInsertHandler())
//	dispatcher.RegisterNamespace("editor", editor.NewDeleteHandler())
//	dispatcher.RegisterNamespace("editor", editor.NewYankHandler())
//	dispatcher.RegisterNamespace("editor", editor.NewIndentHandler())
//
// Or with custom indent settings:
//
//	handler := editor.NewIndentHandlerWithConfig(tabWidth, indentSize, useTabs)
//	dispatcher.RegisterNamespace("editor", handler)
//
// Dispatch editor actions:
//
//	result := dispatcher.Dispatch(input.Action{
//	    Name: editor.ActionInsertText,
//	    Args: input.ActionArgs{Text: "hello"},
//	})
package editor
