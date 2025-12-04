// Package macro provides handlers for macro recording and playback.
//
// This package implements Vim-style macro functionality including:
//   - Record macro (q{register})
//   - Stop recording (q)
//   - Play macro (@{register})
//   - Repeat last macro (@@)
//   - Edit macro
//   - List macros
//
// Macros are stored in named registers (a-z) and can be played back
// multiple times with a count prefix.
package macro
