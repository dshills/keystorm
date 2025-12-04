// Package operator provides handlers for Vim-style operator commands.
//
// Operators are commands that require a motion or text object to define
// the range of text they operate on. This enables powerful composability
// where operators can be combined with any motion (e.g., "dw" deletes a word,
// "ci(" changes inside parentheses).
//
// # Operator Commands
//
// The OperatorHandler type provides Vim-style operators:
//   - operator.delete (d): Delete text in range
//   - operator.change (c): Delete text and enter insert mode
//   - operator.yank (y): Copy text to register
//   - operator.indent (>): Increase indentation
//   - operator.outdent (<): Decrease indentation
//   - operator.lowercase (gu): Convert to lowercase
//   - operator.uppercase (gU): Convert to uppercase
//   - operator.toggleCase (g~): Toggle case
//   - operator.format (gq): Format text
//
// # Motions
//
// Operators accept motions to define the range:
//   - word (w): To next word start
//   - WORD (W): To next WORD start (whitespace-delimited)
//   - wordEnd (e): To word end
//   - wordBack (b): To previous word start
//   - line (l): Entire line(s)
//   - lineEnd ($): To end of line
//   - lineStart (0): To start of line
//   - firstNonBlank (^): To first non-blank character
//   - paragraph (}): To next paragraph
//   - documentEnd (G): To end of document
//   - documentStart (gg): To start of document
//
// # Text Objects
//
// Operators accept text objects for more precise ranges:
//   - word (iw/aw): Inner/around word
//   - WORD (iW/aW): Inner/around WORD
//   - sentence (is/as): Inner/around sentence
//   - paragraph (ip/ap): Inner/around paragraph
//   - quote (i"/a"): Inner/around quotes
//   - paren (i(/a(): Inner/around parentheses
//   - bracket (i[/a[): Inner/around brackets
//   - brace (i{/a{): Inner/around braces
//   - angle (i</a<): Inner/around angle brackets
//   - tag (it/at): Inner/around XML/HTML tags
//
// # Visual Selection
//
// When in visual mode, operators use the visual selection as the range.
// This allows selecting arbitrary text before applying an operator.
//
// # Register Support
//
// Delete, change, and yank operators support registers:
//   - Default register (")
//   - Named registers (a-z)
//   - System clipboard (+, *)
//
// # Usage
//
// Register the handler with the dispatcher:
//
//	dispatcher.RegisterNamespace("operator", operator.NewOperatorHandler())
//
// Dispatch operator actions with motions:
//
//	result := dispatcher.Dispatch(input.Action{
//	    Name: operator.ActionDelete,
//	    Args: input.ActionArgs{
//	        Motion: &input.Motion{
//	            Name:      "word",
//	            Direction: input.DirForward,
//	            Count:     1,
//	        },
//	    },
//	})
//
// Or with text objects:
//
//	result := dispatcher.Dispatch(input.Action{
//	    Name: operator.ActionChange,
//	    Args: input.ActionArgs{
//	        TextObject: &input.TextObject{
//	            Name:      "paren",
//	            Inner:     true,
//	            Delimiter: '(',
//	        },
//	    },
//	})
package operator
