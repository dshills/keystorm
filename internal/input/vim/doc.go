// Package vim provides Vim-style input parsing for key sequences.
//
// This package implements the grammar for parsing Vim commands, including:
//   - Count prefixes: Numbers like "5" in "5j" (move down 5 lines)
//   - Registers: Register selection like `"a` in `"ayw` (yank to register a)
//   - Operators: Commands like d, c, y that require a motion or text object
//   - Motions: Cursor movements like w, e, b, j, k
//   - Text objects: Object selections like iw (inner word), a" (around quotes)
//
// # Vim Grammar
//
// The grammar for Vim normal mode commands is:
//
//	[count][register][operator][count][motion|text-object]
//	[count][register][operator][operator]  (line-wise: dd, yy, cc)
//	[count][motion]
//	[count][register][simple-command]
//
// Examples:
//   - "5j": count=5, motion=j (move down 5 lines)
//   - "d3w": operator=d, count=3, motion=w (delete 3 words)
//   - "diw": operator=d, text-object=iw (delete inner word)
//   - `"ayw`: register=a, operator=y, motion=w (yank word to register a)
//   - "dd": operator=d, line-wise (delete line)
//   - "5dd": count=5, operator=d, line-wise (delete 5 lines)
//
// # Parser States
//
// The parser operates as a state machine that accumulates input:
//
//  1. Initial: Waiting for count, register, operator, or motion
//  2. Count: Accumulating digit characters
//  3. Register: After ", waiting for register name
//  4. Operator: After operator key, waiting for motion/text-object
//  5. Motion/TextObject: Accumulating multi-key motion or text object
//
// # Usage
//
//	parser := vim.NewParser()
//	result := parser.Parse(keyEvent)
//	switch result.Status {
//	case vim.StatusComplete:
//	    // Execute result.Action
//	case vim.StatusPending:
//	    // Wait for more input
//	case vim.StatusInvalid:
//	    // Invalid sequence, clear and try again
//	}
package vim
