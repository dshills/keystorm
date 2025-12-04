// Package window provides handlers for window and split operations.
//
// This package implements Vim-style window management including:
//   - Horizontal splits (Ctrl+W s, :sp)
//   - Vertical splits (Ctrl+W v, :vsp)
//   - Window navigation (Ctrl+W h/j/k/l)
//   - Window closing (Ctrl+W c, :close)
//   - Window resizing (Ctrl+W +/-, Ctrl+W </>)
//   - Window equalization (Ctrl+W =)
//
// The window handler coordinates with the renderer and layout
// system to manage multiple view windows.
package window
