// Package view provides handlers for view and scroll operations.
//
// This package implements Vim-style view navigation including:
//   - Scrolling (Ctrl+F, Ctrl+B, Ctrl+D, Ctrl+U)
//   - Page movements (H, M, L - top, middle, bottom of screen)
//   - Centering view (zz, zt, zb)
//   - Line visibility operations
//
// The view handler coordinates with the renderer to control
// what portion of the buffer is visible.
package view
