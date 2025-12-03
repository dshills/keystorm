// Package backend provides terminal backend abstraction for the renderer.
package backend

import "github.com/dshills/keystorm/internal/renderer/core"

// CursorStyle defines how the cursor appears.
type CursorStyle int

const (
	CursorBlock CursorStyle = iota
	CursorUnderline
	CursorBar
	CursorHidden
)

// EventType identifies the type of terminal event.
type EventType int

const (
	EventNone EventType = iota
	EventKey
	EventMouse
	EventResize
	EventPaste
	EventFocus
)

// Event represents a terminal event.
type Event struct {
	Type EventType

	// Key event fields
	Key  Key
	Rune rune
	Mod  ModMask

	// Mouse event fields
	MouseX, MouseY int
	MouseButton    MouseButton

	// Resize event fields
	Width, Height int

	// Focus event fields
	Focused bool

	// Paste event fields
	PasteText string
}

// Key represents a keyboard key.
type Key int

// Key constants for special keys.
const (
	KeyNone Key = iota
	KeyRune     // Regular character (use Rune field)
	KeyEscape
	KeyEnter
	KeyTab
	KeyBackspace
	KeyDelete
	KeyInsert
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyCtrlSpace
	KeyCtrlA
	KeyCtrlB
	KeyCtrlC
	KeyCtrlD
	KeyCtrlE
	KeyCtrlF
	KeyCtrlG
	KeyCtrlH
	KeyCtrlI
	KeyCtrlJ
	KeyCtrlK
	KeyCtrlL
	KeyCtrlM
	KeyCtrlN
	KeyCtrlO
	KeyCtrlP
	KeyCtrlQ
	KeyCtrlR
	KeyCtrlS
	KeyCtrlT
	KeyCtrlU
	KeyCtrlV
	KeyCtrlW
	KeyCtrlX
	KeyCtrlY
	KeyCtrlZ
)

// ModMask represents modifier key state.
type ModMask int

const (
	ModNone  ModMask = 0
	ModShift ModMask = 1 << iota
	ModCtrl
	ModAlt
	ModMeta
)

// Has returns true if the mask contains the given modifier.
func (m ModMask) Has(mod ModMask) bool {
	return m&mod != 0
}

// MouseButton represents mouse button state.
type MouseButton int

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
	MouseWheelLeft
	MouseWheelRight
)

// Backend defines the interface for terminal/display backends.
// Implementations handle actual drawing to the terminal or other display surfaces.
type Backend interface {
	// Init initializes the backend for use.
	// Must be called before any other methods.
	Init() error

	// Shutdown releases backend resources and restores terminal state.
	// Must be called when done with the backend.
	Shutdown()

	// Size returns the current terminal dimensions.
	Size() (width, height int)

	// OnResize registers a callback for terminal resize events.
	OnResize(callback func(width, height int))

	// SetCell sets a single cell at the given position.
	// Positions outside the terminal are silently ignored.
	SetCell(x, y int, cell core.Cell)

	// GetCell returns the cell at the given position.
	// Returns an empty cell for positions outside the terminal.
	GetCell(x, y int) core.Cell

	// Fill fills a rectangular region with the given cell.
	Fill(rect core.ScreenRect, cell core.Cell)

	// Clear clears the entire screen with the default style.
	Clear()

	// Show synchronizes the internal buffer with the actual display.
	// Call this after making changes to flush them to the screen.
	Show()

	// ShowCursor positions and displays the cursor.
	ShowCursor(x, y int)

	// HideCursor hides the cursor.
	HideCursor()

	// SetCursorStyle changes the cursor appearance.
	SetCursorStyle(style CursorStyle)

	// PollEvent waits for and returns the next terminal event.
	// This is a blocking call.
	PollEvent() Event

	// PostEvent posts a synthetic event to the event queue.
	PostEvent(event Event)

	// HasTrueColor returns true if the backend supports 24-bit color.
	HasTrueColor() bool

	// Beep produces an audible or visual bell.
	Beep()

	// EnableMouse enables mouse event reporting.
	EnableMouse()

	// DisableMouse disables mouse event reporting.
	DisableMouse()

	// EnablePaste enables bracketed paste mode.
	EnablePaste()

	// DisablePaste disables bracketed paste mode.
	DisablePaste()

	// Suspend suspends the terminal (for shell escape).
	Suspend() error

	// Resume resumes from suspension.
	Resume() error
}

// NullBackend is a no-op backend for testing.
type NullBackend struct {
	width, height int
	cells         [][]core.Cell
	cursorX       int
	cursorY       int
	cursorVisible bool
	cursorStyle   CursorStyle
	resizeHandler func(width, height int)
	events        chan Event
}

// NewNullBackend creates a null backend with the given dimensions.
func NewNullBackend(width, height int) *NullBackend {
	return &NullBackend{
		width:  width,
		height: height,
		events: make(chan Event, 100),
	}
}

func (b *NullBackend) Init() error {
	b.cells = make([][]core.Cell, b.height)
	for i := range b.cells {
		b.cells[i] = make([]core.Cell, b.width)
		for j := range b.cells[i] {
			b.cells[i][j] = core.EmptyCell()
		}
	}
	return nil
}

func (b *NullBackend) Shutdown() {}

func (b *NullBackend) Size() (int, int) {
	return b.width, b.height
}

func (b *NullBackend) OnResize(callback func(width, height int)) {
	b.resizeHandler = callback
}

func (b *NullBackend) SetCell(x, y int, cell core.Cell) {
	if x >= 0 && x < b.width && y >= 0 && y < b.height {
		b.cells[y][x] = cell
	}
}

func (b *NullBackend) GetCell(x, y int) core.Cell {
	if x >= 0 && x < b.width && y >= 0 && y < b.height {
		return b.cells[y][x]
	}
	return core.EmptyCell()
}

func (b *NullBackend) Fill(rect core.ScreenRect, cell core.Cell) {
	for y := rect.Top; y < rect.Bottom && y < b.height; y++ {
		for x := rect.Left; x < rect.Right && x < b.width; x++ {
			if x >= 0 && y >= 0 {
				b.cells[y][x] = cell
			}
		}
	}
}

func (b *NullBackend) Clear() {
	empty := core.EmptyCell()
	for y := range b.cells {
		for x := range b.cells[y] {
			b.cells[y][x] = empty
		}
	}
}

func (b *NullBackend) Show() {}

func (b *NullBackend) ShowCursor(x, y int) {
	b.cursorX = x
	b.cursorY = y
	b.cursorVisible = true
}

func (b *NullBackend) HideCursor() {
	b.cursorVisible = false
}

func (b *NullBackend) SetCursorStyle(style CursorStyle) {
	b.cursorStyle = style
}

func (b *NullBackend) PollEvent() Event {
	return <-b.events
}

func (b *NullBackend) PostEvent(event Event) {
	select {
	case b.events <- event:
	default:
		// Event dropped if queue is full (non-blocking for testing)
	}
}

func (b *NullBackend) HasTrueColor() bool { return true }
func (b *NullBackend) Beep()              {}
func (b *NullBackend) EnableMouse()       {}
func (b *NullBackend) DisableMouse()      {}
func (b *NullBackend) EnablePaste()       {}
func (b *NullBackend) DisablePaste()      {}
func (b *NullBackend) Suspend() error     { return nil }
func (b *NullBackend) Resume() error      { return nil }

// CursorPosition returns the current cursor position for testing.
func (b *NullBackend) CursorPosition() (x, y int, visible bool) {
	return b.cursorX, b.cursorY, b.cursorVisible
}

// CursorStyleValue returns the current cursor style for testing.
func (b *NullBackend) CursorStyleValue() CursorStyle {
	return b.cursorStyle
}

// Resize simulates a terminal resize for testing.
func (b *NullBackend) Resize(width, height int) {
	b.width = width
	b.height = height
	b.cells = make([][]core.Cell, height)
	for i := range b.cells {
		b.cells[i] = make([]core.Cell, width)
		for j := range b.cells[i] {
			b.cells[i][j] = core.EmptyCell()
		}
	}
	if b.resizeHandler != nil {
		b.resizeHandler(width, height)
	}
}
