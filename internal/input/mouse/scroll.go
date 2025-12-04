package mouse

import (
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/input/key"
)

// ScrollDirection represents the direction of a scroll event.
type ScrollDirection uint8

const (
	// ScrollNone indicates no scroll.
	ScrollNone ScrollDirection = iota
	// ScrollUp indicates scrolling up (content moves down).
	ScrollUp
	// ScrollDown indicates scrolling down (content moves up).
	ScrollDown
	// ScrollLeft indicates scrolling left.
	ScrollLeft
	// ScrollRight indicates scrolling right.
	ScrollRight
)

// String returns a string representation of the scroll direction.
func (d ScrollDirection) String() string {
	switch d {
	case ScrollUp:
		return "up"
	case ScrollDown:
		return "down"
	case ScrollLeft:
		return "left"
	case ScrollRight:
		return "right"
	default:
		return "none"
	}
}

// ButtonToScrollDirection converts a scroll button to a direction.
func ButtonToScrollDirection(b Button) ScrollDirection {
	switch b {
	case ButtonScrollUp:
		return ScrollUp
	case ButtonScrollDown:
		return ScrollDown
	case ButtonScrollLeft:
		return ScrollLeft
	case ButtonScrollRight:
		return ScrollRight
	default:
		return ScrollNone
	}
}

// ScrollEvent represents a parsed scroll event with computed values.
type ScrollEvent struct {
	// Direction is the scroll direction.
	Direction ScrollDirection

	// Lines is the number of lines to scroll.
	Lines int

	// Position is where the scroll occurred (for targeted scrolling).
	Position Position

	// Modifiers are the keyboard modifiers held during scroll.
	Modifiers key.Modifier

	// IsZoom indicates this is a zoom operation (Ctrl+scroll).
	IsZoom bool

	// ZoomIn indicates zoom direction (true = in, false = out).
	ZoomIn bool
}

// ParseScrollEvent parses a mouse event into a scroll event.
// Returns nil if the event is not a scroll event.
func ParseScrollEvent(event Event, config Config) *ScrollEvent {
	if !event.Button.IsScroll() {
		return nil
	}

	direction := ButtonToScrollDirection(event.Button)
	if direction == ScrollNone {
		return nil
	}

	// Check for zoom
	isZoom := config.EnableZoom && (event.Modifiers.HasCtrl() || event.Modifiers.HasMeta())
	if isZoom {
		return &ScrollEvent{
			Direction: direction,
			Position:  event.Position,
			Modifiers: event.Modifiers,
			IsZoom:    true,
			ZoomIn:    direction == ScrollUp,
		}
	}

	// Calculate scroll lines
	lines := config.ScrollLines
	if event.Modifiers.HasShift() {
		lines = config.ScrollLinesShift
	}

	return &ScrollEvent{
		Direction: direction,
		Lines:     lines,
		Position:  event.Position,
		Modifiers: event.Modifiers,
		IsZoom:    false,
	}
}

// ToAction converts a scroll event to an editor action.
func (e *ScrollEvent) ToAction() *input.Action {
	if e.IsZoom {
		if e.ZoomIn {
			return &input.Action{
				Name:   "view.zoomIn",
				Source: input.SourceMouse,
			}
		}
		return &input.Action{
			Name:   "view.zoomOut",
			Source: input.SourceMouse,
		}
	}

	var actionName string
	switch e.Direction {
	case ScrollUp:
		actionName = "scroll.up"
	case ScrollDown:
		actionName = "scroll.down"
	case ScrollLeft:
		actionName = "scroll.left"
	case ScrollRight:
		actionName = "scroll.right"
	default:
		return nil
	}

	return &input.Action{
		Name:   actionName,
		Source: input.SourceMouse,
		Count:  e.Lines,
	}
}

// IsHorizontal returns true if the scroll is horizontal.
func (e *ScrollEvent) IsHorizontal() bool {
	return e.Direction == ScrollLeft || e.Direction == ScrollRight
}

// IsVertical returns true if the scroll is vertical.
func (e *ScrollEvent) IsVertical() bool {
	return e.Direction == ScrollUp || e.Direction == ScrollDown
}
