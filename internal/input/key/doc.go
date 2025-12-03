// Package key provides key event types and parsing for the input system.
//
// This package defines the fundamental types for representing keyboard input:
//
//   - Key: Identifies a keyboard key (special keys, function keys, or runes)
//   - Modifier: Represents modifier keys (Ctrl, Alt, Shift, Meta)
//   - KeyEvent: A single key press with modifiers and timestamp
//   - KeySequence: A series of key events forming a command
//
// # Key Specifications
//
// Key specifications can be written in multiple formats:
//
//   - Simple keys: "a", "A", "1", "Enter", "Escape"
//   - With modifiers: "Ctrl+S", "Alt+F4", "Ctrl+Shift+P"
//   - Vim-style: "<C-s>", "<A-f>", "<C-S-p>", "<CR>", "<Esc>"
//
// # Key Sequences
//
// Multi-key sequences like Vim's "g g" or "d i w" are represented as
// KeySequence values. The sequence parser handles timeout logic and
// prefix matching for incomplete sequences.
package key
