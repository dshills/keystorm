// Package completion provides handlers for completion operations.
//
// This package implements completion functionality including:
//   - Trigger completion (Ctrl+Space)
//   - Accept completion (Tab, Enter)
//   - Navigate completions (Ctrl+N, Ctrl+P)
//   - Cancel completion (Escape)
//   - Word completion (Ctrl+N in insert mode)
//   - Line completion (Ctrl+X Ctrl+L)
//   - Path completion (Ctrl+X Ctrl+F)
//
// The completion handler coordinates with the LSP integration
// and local completion sources to provide intelligent suggestions.
package completion
