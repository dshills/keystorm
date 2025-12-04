// Package keymap provides key binding management for the Keystorm editor.
//
// The keymap system manages the mapping between key sequences and actions.
// It supports multiple modes, conditional bindings, and layered precedence
// (user > filetype > mode > default).
//
// # Key Concepts
//
// Keymap: A named collection of bindings for a specific mode or context.
//
// Binding: Maps a key sequence to an action with optional conditions.
//
// Registry: Central registry that manages all keymaps and provides lookup.
//
// # Binding Precedence
//
// When multiple bindings match a key sequence, precedence is determined by:
//  1. Priority field (higher wins)
//  2. Specificity (mode-specific > global)
//  3. Registration order (later wins)
//
// # Key Sequence Parsing
//
// Key sequences can be specified in multiple formats:
//
//	"j"        - Single character
//	"g g"      - Multi-key sequence
//	"C-s"      - Ctrl+S (Vim notation)
//	"<C-s>"    - Ctrl+S (angle bracket notation)
//	"Ctrl+S"   - Ctrl+S (readable notation)
//	"<C-S-a>"  - Ctrl+Shift+A
//
// # Conditional Bindings
//
// Bindings can have conditions that must be met:
//
//	binding := Binding{
//	    Keys:   "C-s",
//	    Action: "file.save",
//	    When:   "editorTextFocus && !editorReadonly",
//	}
//
// # Usage
//
//	registry := keymap.NewRegistry()
//	registry.LoadDefaults()
//
//	// Lookup a binding
//	seq := key.ParseSequence("g g")
//	binding := registry.Lookup(seq, "normal", ctx)
//	if binding != nil {
//	    // Execute binding.Action
//	}
//
//	// Check if more keys might complete a binding
//	if registry.HasPrefix(partialSeq, "normal", ctx) {
//	    // Wait for more keys
//	}
package keymap
