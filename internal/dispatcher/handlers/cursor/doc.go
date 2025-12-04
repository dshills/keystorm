// Package cursor provides handlers for cursor movement operations.
//
// This package implements Vim-style cursor movements as action handlers
// for the dispatcher. All handlers support multi-cursor operations and
// properly handle visual mode selection extension.
//
// # Basic Movements
//
// The Handler type provides basic cursor movements:
//   - cursor.moveLeft (h): Move cursor left by [count] characters
//   - cursor.moveRight (l): Move cursor right by [count] characters
//   - cursor.moveUp (k): Move cursor up by [count] lines
//   - cursor.moveDown (j): Move cursor down by [count] lines
//   - cursor.moveLineStart (0): Move to start of line
//   - cursor.moveLineEnd ($): Move to end of line
//   - cursor.moveFirstLine (gg): Move to first line
//   - cursor.moveLastLine (G): Move to last line
//
// # Word Motions
//
// The MotionHandler type provides word-based movements:
//   - cursor.wordForward (w): Move to next word start
//   - cursor.wordBackward (b): Move to previous word start
//   - cursor.wordEndForward (e): Move to word end
//   - cursor.bigWordForward (W): Move to next WORD start (whitespace-delimited)
//   - cursor.bigWordBackward (B): Move to previous WORD start
//   - cursor.bigWordEndForward (E): Move to WORD end
//
// # Line Motions
//
//   - cursor.firstNonBlank (^): Move to first non-blank character
//   - cursor.gotoLine (:[n]): Go to line n
//   - cursor.gotoColumn (|): Go to column [count]
//   - cursor.matchingBracket (%): Go to matching bracket
//   - cursor.gotoPercent ([n]%): Go to percentage position in file
//
// # Paragraph and Sentence Motions
//
//   - cursor.paragraphForward (}): Move to next paragraph
//   - cursor.paragraphBackward ({): Move to previous paragraph
//   - cursor.sentenceForward ()): Move to next sentence
//   - cursor.sentenceBackward ((): Move to previous sentence
//
// # Screen Motions
//
//   - cursor.screenTop (H): Move to top of visible screen
//   - cursor.screenMiddle (M): Move to middle of visible screen
//   - cursor.screenBottom (L): Move to bottom of visible screen
//
// # Multi-cursor Support
//
// All movements operate on all cursors simultaneously using the
// CursorManagerInterface.MapInPlace method. This ensures consistent
// behavior whether in single-cursor or multi-cursor mode.
//
// # Selection Handling
//
// In visual mode (when ctx.HasSelection() returns true), movements
// extend the selection instead of collapsing it. This is determined
// by checking the cursor manager's HasSelection() method.
//
// # Usage
//
// Register handlers with the dispatcher:
//
//	dispatcher.RegisterNamespace("cursor", cursor.NewHandler())
//	dispatcher.RegisterNamespace("cursor", cursor.NewMotionHandler())
//
// Dispatch cursor actions:
//
//	result := dispatcher.Dispatch(input.Action{
//	    Name:  cursor.ActionMoveDown,
//	    Count: 5,
//	})
package cursor
