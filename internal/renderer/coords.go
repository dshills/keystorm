package renderer

import "github.com/dshills/keystorm/internal/renderer/core"

// ScreenPos represents a position on screen (0-indexed).
// Re-exported from core package.
type ScreenPos = core.ScreenPos

// NewScreenPos creates a screen position.
func NewScreenPos(row, col int) ScreenPos {
	return core.NewScreenPos(row, col)
}

// ScreenRect represents a rectangular region on screen.
// Re-exported from core package.
type ScreenRect = core.ScreenRect

// NewScreenRect creates a screen rectangle.
func NewScreenRect(top, left, bottom, right int) ScreenRect {
	return core.NewScreenRect(top, left, bottom, right)
}

// RectFromSize creates a rectangle from position and size.
func RectFromSize(top, left, height, width int) ScreenRect {
	return core.RectFromSize(top, left, height, width)
}
