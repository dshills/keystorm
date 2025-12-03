package renderer

import "github.com/dshills/keystorm/internal/renderer/core"

// Attribute represents text attributes (bold, italic, etc.).
// Re-exported from core package.
type Attribute = core.Attribute

// Text attribute flags.
const (
	AttrNone          = core.AttrNone
	AttrBold          = core.AttrBold
	AttrDim           = core.AttrDim
	AttrItalic        = core.AttrItalic
	AttrUnderline     = core.AttrUnderline
	AttrBlink         = core.AttrBlink
	AttrReverse       = core.AttrReverse
	AttrStrikethrough = core.AttrStrikethrough
	AttrHidden        = core.AttrHidden
)

// Style represents the visual style of text.
// Re-exported from core package.
type Style = core.Style

// DefaultStyle returns the default terminal style.
func DefaultStyle() Style {
	return core.DefaultStyle()
}

// NewStyle creates a style with the given foreground color.
func NewStyle(fg Color) Style {
	return core.NewStyle(fg)
}

// StyleSpan represents a styled range within a line.
// Re-exported from core package.
type StyleSpan = core.StyleSpan
