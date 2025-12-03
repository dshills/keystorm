// Package cursor provides cursor rendering with blink animation and multi-cursor support.
package cursor

import (
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/renderer/core"
)

// Style represents the visual appearance of the cursor.
type Style uint8

const (
	// StyleBlock is a filled block cursor (like vim normal mode).
	StyleBlock Style = iota
	// StyleBar is a vertical line cursor (like vim insert mode).
	StyleBar
	// StyleUnderline is an underscore cursor.
	StyleUnderline
	// StyleHollow is an unfilled block cursor.
	StyleHollow
)

// Position represents a cursor position in buffer coordinates.
type Position struct {
	Line   uint32
	Column uint32
}

// Cursor represents a single cursor with its visual state.
type Cursor struct {
	// Position in buffer coordinates
	Position Position

	// Whether this is the primary cursor
	IsPrimary bool

	// Visual state
	Visible bool // Whether cursor should be drawn (for blink)
}

// Config holds cursor configuration.
type Config struct {
	// Style is the visual appearance of the cursor.
	Style Style

	// BlinkEnabled enables cursor blinking.
	BlinkEnabled bool

	// BlinkRate is the blink interval (cursor toggles on/off at this rate).
	BlinkRate time.Duration

	// PrimaryColor is the color for the primary cursor.
	PrimaryColor core.Color

	// SecondaryColor is the color for secondary cursors (multi-cursor).
	SecondaryColor core.Color

	// BlinkOnType determines if the cursor resets blink on typing.
	BlinkOnType bool
}

// DefaultConfig returns sensible default cursor configuration.
func DefaultConfig() Config {
	return Config{
		Style:          StyleBlock,
		BlinkEnabled:   true,
		BlinkRate:      500 * time.Millisecond,
		PrimaryColor:   core.ColorDefault,
		SecondaryColor: core.ColorGray,
		BlinkOnType:    true,
	}
}

// Renderer handles cursor rendering and blink animation.
type Renderer struct {
	mu sync.RWMutex

	config Config

	// Cursors to render
	cursors []Cursor

	// Blink state
	blinkVisible bool
	lastBlink    time.Time
	blinkPaused  bool // Temporarily pause blink (e.g., during typing)
	pauseUntil   time.Time
}

// New creates a new cursor renderer with the given configuration.
func New(config Config) *Renderer {
	return &Renderer{
		config:       config,
		blinkVisible: true,
		lastBlink:    time.Now(),
	}
}

// Config returns the current configuration.
func (r *Renderer) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig updates the cursor configuration.
func (r *Renderer) SetConfig(config Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// SetStyle updates the cursor style.
func (r *Renderer) SetStyle(style Style) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.Style = style
}

// Style returns the current cursor style.
func (r *Renderer) Style() Style {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.Style
}

// SetCursors updates the cursors to render.
func (r *Renderer) SetCursors(cursors []Cursor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cursors = make([]Cursor, len(cursors))
	copy(r.cursors, cursors)
}

// SetPrimaryCursor sets a single primary cursor.
func (r *Renderer) SetPrimaryCursor(line, col uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cursors = []Cursor{{
		Position:  Position{Line: line, Column: col},
		IsPrimary: true,
		Visible:   true,
	}}
}

// Cursors returns all cursors.
func (r *Renderer) Cursors() []Cursor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Cursor, len(r.cursors))
	copy(result, r.cursors)
	return result
}

// PrimaryCursor returns the primary cursor, if any.
func (r *Renderer) PrimaryCursor() (Cursor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.cursors {
		if c.IsPrimary {
			return c, true
		}
	}
	if len(r.cursors) > 0 {
		return r.cursors[0], true
	}
	return Cursor{}, false
}

// Update advances blink animation.
// Returns true if the cursor visibility changed.
func (r *Renderer) Update(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.config.BlinkEnabled {
		if !r.blinkVisible {
			r.blinkVisible = true
			return true
		}
		return false
	}

	// Check if blink is paused
	if r.blinkPaused && now.Before(r.pauseUntil) {
		if !r.blinkVisible {
			r.blinkVisible = true
			return true
		}
		return false
	}
	r.blinkPaused = false

	// Check if it's time to toggle
	elapsed := now.Sub(r.lastBlink)
	if elapsed >= r.config.BlinkRate {
		r.blinkVisible = !r.blinkVisible
		r.lastBlink = now
		return true
	}

	return false
}

// IsVisible returns whether cursors should be visible (blink state).
func (r *Renderer) IsVisible() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.blinkVisible
}

// ResetBlink resets the blink animation to visible state.
// Useful when the user types or moves the cursor.
func (r *Renderer) ResetBlink() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blinkVisible = true
	r.lastBlink = time.Now()
}

// PauseBlink temporarily pauses blinking (keeps cursor visible).
// Useful during typing to keep cursor always visible.
func (r *Renderer) PauseBlink(duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blinkPaused = true
	r.pauseUntil = time.Now().Add(duration)
	r.blinkVisible = true
}

// RenderState contains the information needed to render a cursor.
type RenderState struct {
	// ScreenX is the X coordinate on screen.
	ScreenX int
	// ScreenY is the Y coordinate on screen.
	ScreenY int
	// Style is the cursor style.
	Style Style
	// Color is the cursor color.
	Color core.Color
	// Visible is whether the cursor should be drawn.
	Visible bool
	// IsPrimary indicates if this is the primary cursor.
	IsPrimary bool
	// CharUnder is the character under the cursor (for block cursors).
	CharUnder rune
	// CharStyle is the style of the character under the cursor.
	CharStyle core.Style
}

// GetRenderStates returns render information for all visible cursors.
// The converter function converts buffer position to screen position.
// Returns nil for the position if cursor is not visible on screen.
func (r *Renderer) GetRenderStates(converter func(line, col uint32) (screenX, screenY int, visible bool)) []RenderState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.cursors) == 0 {
		return nil
	}

	states := make([]RenderState, 0, len(r.cursors))

	for _, cursor := range r.cursors {
		screenX, screenY, visible := converter(cursor.Position.Line, cursor.Position.Column)
		if !visible {
			continue
		}

		color := r.config.SecondaryColor
		if cursor.IsPrimary {
			color = r.config.PrimaryColor
		}

		states = append(states, RenderState{
			ScreenX:   screenX,
			ScreenY:   screenY,
			Style:     r.config.Style,
			Color:     color,
			Visible:   r.blinkVisible,
			IsPrimary: cursor.IsPrimary,
		})
	}

	return states
}

// CursorCell returns a cell to render for a cursor at the given position.
// The baseCell is the cell that would normally be rendered at that position.
func (r *Renderer) CursorCell(baseCell core.Cell, state RenderState) core.Cell {
	if !state.Visible {
		return baseCell
	}

	switch state.Style {
	case StyleBlock:
		// Invert colors for block cursor
		return core.Cell{
			Rune:  baseCell.Rune,
			Width: baseCell.Width,
			Style: core.Style{
				Foreground: baseCell.Style.Background,
				Background: selectColor(state.Color, baseCell.Style.Foreground),
				Attributes: baseCell.Style.Attributes,
			},
		}

	case StyleBar:
		// Bar cursor is rendered by the terminal, not as a cell modification
		// Return the base cell; the terminal handles the cursor display
		return baseCell

	case StyleUnderline:
		// Add underline attribute
		return core.Cell{
			Rune:  baseCell.Rune,
			Width: baseCell.Width,
			Style: baseCell.Style.Underline(),
		}

	case StyleHollow:
		// Hollow cursor - draw a box around the character
		// For now, just use reverse video with lower intensity
		return core.Cell{
			Rune:  baseCell.Rune,
			Width: baseCell.Width,
			Style: baseCell.Style.Reverse(),
		}

	default:
		return baseCell
	}
}

// selectColor returns the cursor color if it's not default, otherwise falls back to fallback.
func selectColor(cursorColor, fallback core.Color) core.Color {
	if cursorColor.IsDefault() {
		return fallback
	}
	return cursorColor
}

// StyleFromString converts a string name to a cursor style.
func StyleFromString(s string) Style {
	switch s {
	case "block":
		return StyleBlock
	case "bar", "line":
		return StyleBar
	case "underline", "underscore":
		return StyleUnderline
	case "hollow":
		return StyleHollow
	default:
		return StyleBlock
	}
}

// String returns the string representation of a cursor style.
func (s Style) String() string {
	switch s {
	case StyleBlock:
		return "block"
	case StyleBar:
		return "bar"
	case StyleUnderline:
		return "underline"
	case StyleHollow:
		return "hollow"
	default:
		return "block"
	}
}
