package renderer

import (
	"fmt"
	"strconv"
	"strings"
)

// Color represents a color value.
// Supports true color (RGB) and terminal palette colors.
type Color struct {
	R, G, B uint8
	// If Indexed is true, R contains the palette index (0-255).
	// G and B are ignored in indexed mode.
	Indexed bool
	// Default indicates this is the terminal's default color.
	Default bool
}

// ColorDefault represents the terminal's default color.
// Use this for transparent/inherited colors.
var ColorDefault = Color{Default: true}

// Common colors.
var (
	ColorBlack   = Color{R: 0, G: 0, B: 0}
	ColorWhite   = Color{R: 255, G: 255, B: 255}
	ColorRed     = Color{R: 255, G: 0, B: 0}
	ColorGreen   = Color{R: 0, G: 255, B: 0}
	ColorBlue    = Color{R: 0, G: 0, B: 255}
	ColorYellow  = Color{R: 255, G: 255, B: 0}
	ColorCyan    = Color{R: 0, G: 255, B: 255}
	ColorMagenta = Color{R: 255, G: 0, B: 255}
	ColorGray    = Color{R: 128, G: 128, B: 128}
)

// ColorFromRGB creates a true color from RGB components.
func ColorFromRGB(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, Indexed: false}
}

// ColorFromIndex creates an indexed palette color.
// Index should be 0-255 for standard terminal palettes.
func ColorFromIndex(index uint8) Color {
	return Color{R: index, Indexed: true}
}

// ColorFromHex creates a color from a hex string.
// Supports formats: "#RGB", "#RRGGBB", "RGB", "RRGGBB".
func ColorFromHex(hex string) (Color, error) {
	// Remove leading #
	hex = strings.TrimPrefix(hex, "#")

	var r, g, b uint64
	var err error

	switch len(hex) {
	case 3:
		// Short form: RGB -> RRGGBB
		r, err = strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %s", hex)
		}
		g, err = strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %s", hex)
		}
		b, err = strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %s", hex)
		}

	case 6:
		// Full form: RRGGBB
		r, err = strconv.ParseUint(hex[0:2], 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %s", hex)
		}
		g, err = strconv.ParseUint(hex[2:4], 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %s", hex)
		}
		b, err = strconv.ParseUint(hex[4:6], 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %s", hex)
		}

	default:
		return Color{}, fmt.Errorf("invalid hex color length: %s", hex)
	}

	return Color{R: uint8(r), G: uint8(g), B: uint8(b), Indexed: false}, nil
}

// IsDefault returns true if this is the default/transparent color.
func (c Color) IsDefault() bool {
	return c.Default
}

// Equals returns true if two colors are equal.
func (c Color) Equals(other Color) bool {
	if c.Default != other.Default {
		return false
	}
	if c.Default {
		return true
	}
	if c.Indexed != other.Indexed {
		return false
	}
	if c.Indexed {
		return c.R == other.R
	}
	return c.R == other.R && c.G == other.G && c.B == other.B
}

// String returns a string representation of the color.
func (c Color) String() string {
	if c.IsDefault() {
		return "default"
	}
	if c.Indexed {
		return fmt.Sprintf("idx(%d)", c.R)
	}
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// ToHex returns the hex representation of a true color.
// Returns empty string for indexed colors.
func (c Color) ToHex() string {
	if c.Indexed {
		return ""
	}
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// Lighten returns a lighter version of the color.
// Amount should be 0.0 to 1.0.
func (c Color) Lighten(amount float64) Color {
	if c.Indexed {
		return c
	}
	return Color{
		R:       uint8(min(255, float64(c.R)+float64(255-c.R)*amount)),
		G:       uint8(min(255, float64(c.G)+float64(255-c.G)*amount)),
		B:       uint8(min(255, float64(c.B)+float64(255-c.B)*amount)),
		Indexed: false,
	}
}

// Darken returns a darker version of the color.
// Amount should be 0.0 to 1.0.
func (c Color) Darken(amount float64) Color {
	if c.Indexed {
		return c
	}
	return Color{
		R:       uint8(float64(c.R) * (1 - amount)),
		G:       uint8(float64(c.G) * (1 - amount)),
		B:       uint8(float64(c.B) * (1 - amount)),
		Indexed: false,
	}
}

// Blend blends two colors together.
// Amount 0.0 = c, 1.0 = other.
func (c Color) Blend(other Color, amount float64) Color {
	if c.Indexed || other.Indexed {
		if amount < 0.5 {
			return c
		}
		return other
	}
	return Color{
		R:       uint8(float64(c.R)*(1-amount) + float64(other.R)*amount),
		G:       uint8(float64(c.G)*(1-amount) + float64(other.G)*amount),
		B:       uint8(float64(c.B)*(1-amount) + float64(other.B)*amount),
		Indexed: false,
	}
}
