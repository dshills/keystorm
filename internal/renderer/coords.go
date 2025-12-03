package renderer

// ScreenPos represents a position on screen (0-indexed).
type ScreenPos struct {
	Row int // Screen row (0 = top)
	Col int // Screen column (0 = left)
}

// NewScreenPos creates a screen position.
func NewScreenPos(row, col int) ScreenPos {
	return ScreenPos{Row: row, Col: col}
}

// Add returns a new position offset by the given delta.
func (p ScreenPos) Add(dRow, dCol int) ScreenPos {
	return ScreenPos{
		Row: p.Row + dRow,
		Col: p.Col + dCol,
	}
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
	return ScreenRect{
		Top:    top,
		Left:   left,
		Bottom: bottom,
		Right:  right,
	}
}

// RectFromSize creates a rectangle from position and size.
func RectFromSize(top, left, height, width int) ScreenRect {
	return ScreenRect{
		Top:    top,
		Left:   left,
		Bottom: top + height,
		Right:  left + width,
	}
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
// Returns an empty rectangle if they don't overlap.
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
