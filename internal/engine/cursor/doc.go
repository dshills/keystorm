// Package cursor provides cursor and selection management for text editing.
//
// The cursor package handles:
//
//   - Single cursor positioning with Cursor type
//   - Text selections with anchor/head model via Selection type
//   - Multi-cursor support with CursorSet
//   - Cursor transformation after buffer edits
//
// Selection Model:
//
// Selections use an anchor/head model where:
//   - Anchor: The position where the selection started
//   - Head: The current cursor position (where typing would occur)
//
// When Anchor == Head, the selection represents just a cursor with no
// selected text. The selection can extend forward (head > anchor) or
// backward (head < anchor), preserving the user's selection direction.
//
// Multi-Cursor Support:
//
// CursorSet manages multiple selections that are:
//   - Kept sorted by position
//   - Automatically merged when overlapping
//   - Transformed together after edits
//
// Basic usage:
//
//	// Create a selection
//	sel := cursor.NewCursorSelection(10)  // Cursor at offset 10
//
//	// Extend selection
//	sel = sel.Extend(20)  // Select from 10 to 20
//
//	// Multi-cursor
//	cs := cursor.NewCursorSet(sel)
//	cs.Add(cursor.NewCursorSelection(50))  // Add another cursor
//
//	// Transform after edit
//	edit := buffer.Edit{Range: buffer.Range{Start: 0, End: 5}, NewText: "Hello"}
//	cursor.TransformCursorSet(cs, edit)
//
// Thread Safety:
//
// Cursor and Selection types are immutable value types and safe for
// concurrent use. CursorSet is not thread-safe and should be protected
// by external synchronization if accessed concurrently.
package cursor
