// Package core provides shared types for the renderer subsystem.
// This package breaks import cycles between renderer and backend.
package core

import (
	"fmt"
	"strconv"
	"strings"
)

// Attribute represents text attributes (bold, italic, etc.).
type Attribute uint16

// Text attribute flags.
const (
	AttrNone          Attribute = 0
	AttrBold          Attribute = 1 << iota
	AttrDim                     // Faint/dim text
	AttrItalic                  // Italic text
	AttrUnderline               // Underlined text
	AttrBlink                   // Blinking text (rarely supported)
	AttrReverse                 // Reverse video (swap fg/bg)
	AttrStrikethrough           // Strikethrough text
	AttrHidden                  // Hidden/invisible text
)

// Has returns true if the attribute set contains the given attribute.
func (a Attribute) Has(attr Attribute) bool {
	return a&attr != 0
}

// With returns a new attribute set with the given attribute added.
func (a Attribute) With(attr Attribute) Attribute {
	return a | attr
}

// Without returns a new attribute set with the given attribute removed.
func (a Attribute) Without(attr Attribute) Attribute {
	return a &^ attr
}

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
func ColorFromIndex(index uint8) Color {
	return Color{R: index, Indexed: true}
}

// ColorFromHex creates a color from a hex string.
func ColorFromHex(hex string) (Color, error) {
	hex = strings.TrimPrefix(hex, "#")

	var r, g, b uint64
	var err error

	switch len(hex) {
	case 3:
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
func (c Color) ToHex() string {
	if c.Indexed {
		return ""
	}
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// Lighten returns a lighter version of the color.
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

// Style represents the visual style of text.
type Style struct {
	Foreground Color
	Background Color
	Attributes Attribute
}

// DefaultStyle returns the default terminal style.
func DefaultStyle() Style {
	return Style{
		Foreground: ColorDefault,
		Background: ColorDefault,
		Attributes: AttrNone,
	}
}

// NewStyle creates a style with the given foreground color.
func NewStyle(fg Color) Style {
	return Style{
		Foreground: fg,
		Background: ColorDefault,
		Attributes: AttrNone,
	}
}

// WithForeground returns a new style with the given foreground color.
func (s Style) WithForeground(fg Color) Style {
	s.Foreground = fg
	return s
}

// WithBackground returns a new style with the given background color.
func (s Style) WithBackground(bg Color) Style {
	s.Background = bg
	return s
}

// WithAttributes returns a new style with the given attributes.
func (s Style) WithAttributes(attrs Attribute) Style {
	s.Attributes = attrs
	return s
}

// Bold returns a new style with bold attribute added.
func (s Style) Bold() Style {
	s.Attributes |= AttrBold
	return s
}

// Dim returns a new style with dim attribute added.
func (s Style) Dim() Style {
	s.Attributes |= AttrDim
	return s
}

// Italic returns a new style with italic attribute added.
func (s Style) Italic() Style {
	s.Attributes |= AttrItalic
	return s
}

// Underline returns a new style with underline attribute added.
func (s Style) Underline() Style {
	s.Attributes |= AttrUnderline
	return s
}

// Reverse returns a new style with reverse video attribute added.
func (s Style) Reverse() Style {
	s.Attributes |= AttrReverse
	return s
}

// Strikethrough returns a new style with strikethrough attribute added.
func (s Style) Strikethrough() Style {
	s.Attributes |= AttrStrikethrough
	return s
}

// Merge combines two styles.
func (s Style) Merge(other Style) Style {
	result := s

	if !other.Foreground.IsDefault() {
		result.Foreground = other.Foreground
	}
	if !other.Background.IsDefault() {
		result.Background = other.Background
	}
	result.Attributes |= other.Attributes

	return result
}

// Equals returns true if two styles are identical.
func (s Style) Equals(other Style) bool {
	return s.Foreground.Equals(other.Foreground) &&
		s.Background.Equals(other.Background) &&
		s.Attributes == other.Attributes
}

// IsDefault returns true if this is the default style.
func (s Style) IsDefault() bool {
	return s.Foreground.IsDefault() &&
		s.Background.IsDefault() &&
		s.Attributes == AttrNone
}

// Invert returns a style with foreground and background swapped.
func (s Style) Invert() Style {
	return Style{
		Foreground: s.Background,
		Background: s.Foreground,
		Attributes: s.Attributes,
	}
}

// Cell represents a single terminal cell.
type Cell struct {
	// Rune is the character to display.
	Rune rune

	// Width is the display width of this cell.
	Width int

	// Style is the visual style for this cell.
	Style Style
}

// EmptyCell returns an empty cell with default style.
func EmptyCell() Cell {
	return Cell{
		Rune:  ' ',
		Width: 1,
		Style: DefaultStyle(),
	}
}

// NewCell creates a cell with the given rune and default style.
func NewCell(r rune) Cell {
	return Cell{
		Rune:  r,
		Width: RuneWidth(r),
		Style: DefaultStyle(),
	}
}

// NewStyledCell creates a cell with the given rune and style.
func NewStyledCell(r rune, style Style) Cell {
	return Cell{
		Rune:  r,
		Width: RuneWidth(r),
		Style: style,
	}
}

// WithStyle returns a new cell with the given style.
func (c Cell) WithStyle(style Style) Cell {
	c.Style = style
	return c
}

// WithRune returns a new cell with the given rune.
func (c Cell) WithRune(r rune) Cell {
	c.Rune = r
	c.Width = RuneWidth(r)
	return c
}

// IsEmpty returns true if this is an empty (space) cell.
func (c Cell) IsEmpty() bool {
	return c.Rune == ' ' || c.Rune == 0
}

// IsContinuation returns true if this is a continuation cell.
func (c Cell) IsContinuation() bool {
	return c.Width == 0 && c.Rune == 0
}

// Equals returns true if two cells are identical.
func (c Cell) Equals(other Cell) bool {
	return c.Rune == other.Rune &&
		c.Width == other.Width &&
		c.Style.Equals(other.Style)
}

// ContinuationCell returns a continuation cell for wide characters.
func ContinuationCell() Cell {
	return Cell{
		Rune:  0,
		Width: 0,
		Style: DefaultStyle(),
	}
}

// RuneWidth returns the display width of a rune.
func RuneWidth(r rune) int {
	if r < 32 || r == 0x7F {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

// isWideRune checks if a rune is a wide (double-width) character.
func isWideRune(r rune) bool {
	if r >= 0x1100 && r <= 0x115F {
		return true
	}
	if r >= 0x3130 && r <= 0x318F {
		return true
	}
	if r >= 0x2E80 && r <= 0x9FFF {
		return true
	}
	if r >= 0xAC00 && r <= 0xD7A3 {
		return true
	}
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	if r >= 0xFE10 && r <= 0xFE1F {
		return true
	}
	if r >= 0xFE30 && r <= 0xFE6F {
		return true
	}
	if r >= 0xFF00 && r <= 0xFF60 {
		return true
	}
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return true
	}
	if r >= 0x20000 && r <= 0x2FFFF {
		return true
	}
	if r >= 0x2F800 && r <= 0x2FA1F {
		return true
	}
	return false
}

// CellsFromString creates cells from a string.
func CellsFromString(s string, style Style) []Cell {
	cells := make([]Cell, 0, len(s))
	for _, r := range s {
		width := RuneWidth(r)
		cells = append(cells, Cell{
			Rune:  r,
			Width: width,
			Style: style,
		})
		if width == 2 {
			cells = append(cells, ContinuationCell())
		}
	}
	return cells
}

// StringFromCells converts cells back to a string.
func StringFromCells(cells []Cell) string {
	runes := make([]rune, 0, len(cells))
	for _, c := range cells {
		if !c.IsContinuation() && c.Rune != 0 {
			runes = append(runes, c.Rune)
		}
	}
	return string(runes)
}

// ScreenPos represents a position on screen (0-indexed).
type ScreenPos struct {
	Row int
	Col int
}

// NewScreenPos creates a screen position.
func NewScreenPos(row, col int) ScreenPos {
	return ScreenPos{Row: row, Col: col}
}

// Add returns a new position offset by the given delta.
func (p ScreenPos) Add(dRow, dCol int) ScreenPos {
	return ScreenPos{Row: p.Row + dRow, Col: p.Col + dCol}
}

// Equals returns true if two positions are the same.
func (p ScreenPos) Equals(other ScreenPos) bool {
	return p.Row == other.Row && p.Col == other.Col
}

// Before returns true if p comes before other in reading order.
func (p ScreenPos) Before(other ScreenPos) bool {
	if p.Row != other.Row {
		return p.Row < other.Row
	}
	return p.Col < other.Col
}

// ScreenRect represents a rectangular region on screen.
type ScreenRect struct {
	Top    int // First row (inclusive)
	Left   int // First column (inclusive)
	Bottom int // Last row (exclusive)
	Right  int // Last column (exclusive)
}

// NewScreenRect creates a screen rectangle.
func NewScreenRect(top, left, bottom, right int) ScreenRect {
	return ScreenRect{Top: top, Left: left, Bottom: bottom, Right: right}
}

// RectFromSize creates a rectangle from position and size.
func RectFromSize(top, left, height, width int) ScreenRect {
	return ScreenRect{Top: top, Left: left, Bottom: top + height, Right: left + width}
}

// Width returns the width of the rectangle.
func (r ScreenRect) Width() int {
	if r.Right <= r.Left {
		return 0
	}
	return r.Right - r.Left
}

// Height returns the height of the rectangle.
func (r ScreenRect) Height() int {
	if r.Bottom <= r.Top {
		return 0
	}
	return r.Bottom - r.Top
}

// Size returns width and height.
func (r ScreenRect) Size() (width, height int) {
	return r.Width(), r.Height()
}

// IsEmpty returns true if the rectangle has no area.
func (r ScreenRect) IsEmpty() bool {
	return r.Width() <= 0 || r.Height() <= 0
}

// Contains returns true if pos is within the rectangle.
func (r ScreenRect) Contains(pos ScreenPos) bool {
	return pos.Row >= r.Top && pos.Row < r.Bottom &&
		pos.Col >= r.Left && pos.Col < r.Right
}

// ContainsRect returns true if other is entirely within r.
func (r ScreenRect) ContainsRect(other ScreenRect) bool {
	return other.Top >= r.Top && other.Bottom <= r.Bottom &&
		other.Left >= r.Left && other.Right <= r.Right
}

// Intersects returns true if two rectangles overlap.
func (r ScreenRect) Intersects(other ScreenRect) bool {
	return r.Left < other.Right && r.Right > other.Left &&
		r.Top < other.Bottom && r.Bottom > other.Top
}

// Intersection returns the overlapping region of two rectangles.
func (r ScreenRect) Intersection(other ScreenRect) ScreenRect {
	if !r.Intersects(other) {
		return ScreenRect{}
	}
	return ScreenRect{
		Top:    max(r.Top, other.Top),
		Left:   max(r.Left, other.Left),
		Bottom: min(r.Bottom, other.Bottom),
		Right:  min(r.Right, other.Right),
	}
}

// Union returns the smallest rectangle containing both rectangles.
func (r ScreenRect) Union(other ScreenRect) ScreenRect {
	if r.IsEmpty() {
		return other
	}
	if other.IsEmpty() {
		return r
	}
	return ScreenRect{
		Top:    min(r.Top, other.Top),
		Left:   min(r.Left, other.Left),
		Bottom: max(r.Bottom, other.Bottom),
		Right:  max(r.Right, other.Right),
	}
}

// Inset returns a rectangle inset by the given amounts.
func (r ScreenRect) Inset(top, right, bottom, left int) ScreenRect {
	return ScreenRect{
		Top:    r.Top + top,
		Left:   r.Left + left,
		Bottom: r.Bottom - bottom,
		Right:  r.Right - right,
	}
}

// Expand returns a rectangle expanded by the given amounts.
func (r ScreenRect) Expand(top, right, bottom, left int) ScreenRect {
	return r.Inset(-top, -right, -bottom, -left)
}

// TopLeft returns the top-left corner position.
func (r ScreenRect) TopLeft() ScreenPos {
	return ScreenPos{Row: r.Top, Col: r.Left}
}

// TopRight returns the top-right corner position (exclusive column).
func (r ScreenRect) TopRight() ScreenPos {
	return ScreenPos{Row: r.Top, Col: r.Right}
}

// BottomLeft returns the bottom-left corner position (exclusive row).
func (r ScreenRect) BottomLeft() ScreenPos {
	return ScreenPos{Row: r.Bottom, Col: r.Left}
}

// BottomRight returns the bottom-right corner position (both exclusive).
func (r ScreenRect) BottomRight() ScreenPos {
	return ScreenPos{Row: r.Bottom, Col: r.Right}
}

// Clamp returns a position clamped to be within the rectangle.
func (r ScreenRect) Clamp(pos ScreenPos) ScreenPos {
	result := pos
	if result.Row < r.Top {
		result.Row = r.Top
	}
	if result.Row >= r.Bottom {
		result.Row = r.Bottom - 1
	}
	if result.Col < r.Left {
		result.Col = r.Left
	}
	if result.Col >= r.Right {
		result.Col = r.Right - 1
	}
	return result
}

// Equals returns true if two rectangles are identical.
func (r ScreenRect) Equals(other ScreenRect) bool {
	return r.Top == other.Top && r.Left == other.Left &&
		r.Bottom == other.Bottom && r.Right == other.Right
}

// StyleSpan represents a styled range within a line.
type StyleSpan struct {
	StartCol uint32 // Starting column (0-indexed)
	EndCol   uint32 // Ending column (exclusive)
	Style    Style
}

// Len returns the length of the span in columns.
func (s StyleSpan) Len() uint32 {
	return s.EndCol - s.StartCol
}

// Contains returns true if the given column is within the span.
func (s StyleSpan) Contains(col uint32) bool {
	return col >= s.StartCol && col < s.EndCol
}

// Overlaps returns true if two spans overlap.
func (s StyleSpan) Overlaps(other StyleSpan) bool {
	return s.StartCol < other.EndCol && other.StartCol < s.EndCol
}

// Intersection returns the overlapping region of two spans.
func (s StyleSpan) Intersection(other StyleSpan) StyleSpan {
	if !s.Overlaps(other) {
		return StyleSpan{}
	}
	return StyleSpan{
		StartCol: max(s.StartCol, other.StartCol),
		EndCol:   min(s.EndCol, other.EndCol),
		Style:    s.Style.Merge(other.Style),
	}
}
