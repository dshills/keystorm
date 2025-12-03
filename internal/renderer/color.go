package renderer

import "github.com/dshills/keystorm/internal/renderer/core"

// Color represents a color value.
// Re-exported from core package.
type Color = core.Color

// ColorDefault represents the terminal's default color.
var ColorDefault = core.ColorDefault

// Common colors.
var (
	ColorBlack   = core.ColorBlack
	ColorWhite   = core.ColorWhite
	ColorRed     = core.ColorRed
	ColorGreen   = core.ColorGreen
	ColorBlue    = core.ColorBlue
	ColorYellow  = core.ColorYellow
	ColorCyan    = core.ColorCyan
	ColorMagenta = core.ColorMagenta
	ColorGray    = core.ColorGray
)

// ColorFromRGB creates a true color from RGB components.
func ColorFromRGB(r, g, b uint8) Color {
	return core.ColorFromRGB(r, g, b)
}

// ColorFromIndex creates an indexed palette color.
func ColorFromIndex(index uint8) Color {
	return core.ColorFromIndex(index)
}

// ColorFromHex creates a color from a hex string.
func ColorFromHex(hex string) (Color, error) {
	return core.ColorFromHex(hex)
}
