package renderer

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
// The other style takes precedence for non-default values.
// Attributes are OR'd together.
func (s Style) Merge(other Style) Style {
	result := s

	// Foreground: use other if not default
	if !other.Foreground.IsDefault() {
		result.Foreground = other.Foreground
	}

	// Background: use other if not default
	if !other.Background.IsDefault() {
		result.Background = other.Background
	}

	// Attributes: combine both
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
// Returns an empty span if they don't overlap.
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
