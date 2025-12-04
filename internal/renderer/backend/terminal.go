package backend

import (
	"sync"

	"github.com/gdamore/tcell/v2"

	"github.com/dshills/keystorm/internal/renderer/core"
)

// Terminal implements Backend using tcell for terminal output.
type Terminal struct {
	screen        tcell.Screen
	resizeHandler func(width, height int)
	mu            sync.Mutex
}

// NewTerminal creates a new terminal backend.
func NewTerminal() (*Terminal, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	return &Terminal{screen: screen}, nil
}

func (t *Terminal) Init() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.screen.Init(); err != nil {
		return err
	}

	// Enable mouse support by default
	t.screen.EnableMouse()

	// Enable bracketed paste
	t.screen.EnablePaste()

	return nil
}

func (t *Terminal) Shutdown() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.Fini()
}

func (t *Terminal) Size() (int, int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.screen.Size()
}

func (t *Terminal) OnResize(callback func(width, height int)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resizeHandler = callback
}

func (t *Terminal) SetCell(x, y int, cell core.Cell) {
	t.mu.Lock()
	defer t.mu.Unlock()

	style := convertStyle(cell.Style)
	t.screen.SetContent(x, y, cell.Rune, nil, style)
}

func (t *Terminal) GetCell(x, y int) core.Cell {
	t.mu.Lock()
	defer t.mu.Unlock()

	mainc, _, style, _ := t.screen.GetContent(x, y) //nolint:staticcheck // GetContent is the correct API
	return core.Cell{
		Rune:  mainc,
		Width: core.RuneWidth(mainc),
		Style: convertTcellStyle(style),
	}
}

func (t *Terminal) Fill(rect core.ScreenRect, cell core.Cell) {
	t.mu.Lock()
	defer t.mu.Unlock()

	style := convertStyle(cell.Style)
	width, height := t.screen.Size()

	for y := rect.Top; y < rect.Bottom && y < height; y++ {
		for x := rect.Left; x < rect.Right && x < width; x++ {
			if x >= 0 && y >= 0 {
				t.screen.SetContent(x, y, cell.Rune, nil, style)
			}
		}
	}
}

func (t *Terminal) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.Clear()
}

func (t *Terminal) Show() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.Show()
}

func (t *Terminal) ShowCursor(x, y int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.ShowCursor(x, y)
}

func (t *Terminal) HideCursor() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.HideCursor()
}

func (t *Terminal) SetCursorStyle(style CursorStyle) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var tcellStyle tcell.CursorStyle
	switch style {
	case CursorBlock:
		tcellStyle = tcell.CursorStyleSteadyBlock
	case CursorUnderline:
		tcellStyle = tcell.CursorStyleSteadyUnderline
	case CursorBar:
		tcellStyle = tcell.CursorStyleSteadyBar
	case CursorHidden:
		t.screen.HideCursor()
		return
	}
	t.screen.SetCursorStyle(tcellStyle)
}

func (t *Terminal) PollEvent() Event {
	ev := t.screen.PollEvent()
	return convertEvent(ev, t)
}

func (t *Terminal) PostEvent(event Event) {
	// Convert our event to tcell event and post it
	// For now, we only support posting key events
	if event.Type == EventKey {
		tcellEv := tcell.NewEventKey(convertToTcellKey(event.Key), event.Rune, convertToTcellMod(event.Mod))
		_ = t.screen.PostEvent(tcellEv) // best-effort; event queue may be full
	}
}

func (t *Terminal) HasTrueColor() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.screen.Colors() > 256
}

func (t *Terminal) Beep() {
	t.mu.Lock()
	defer t.mu.Unlock()

	_ = t.screen.Beep() // best-effort; terminal may not support beep
}

func (t *Terminal) EnableMouse() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.EnableMouse()
}

func (t *Terminal) DisableMouse() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.DisableMouse()
}

func (t *Terminal) EnablePaste() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.EnablePaste()
}

func (t *Terminal) DisablePaste() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.screen.DisablePaste()
}

func (t *Terminal) Suspend() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.screen.Suspend()
}

func (t *Terminal) Resume() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.screen.Resume()
}

// convertStyle converts our Style to tcell.Style.
func convertStyle(s core.Style) tcell.Style {
	style := tcell.StyleDefault

	// Convert foreground
	if !s.Foreground.IsDefault() {
		if s.Foreground.Indexed {
			style = style.Foreground(tcell.PaletteColor(int(s.Foreground.R)))
		} else {
			style = style.Foreground(tcell.NewRGBColor(int32(s.Foreground.R), int32(s.Foreground.G), int32(s.Foreground.B)))
		}
	}

	// Convert background
	if !s.Background.IsDefault() {
		if s.Background.Indexed {
			style = style.Background(tcell.PaletteColor(int(s.Background.R)))
		} else {
			style = style.Background(tcell.NewRGBColor(int32(s.Background.R), int32(s.Background.G), int32(s.Background.B)))
		}
	}

	// Convert attributes
	if s.Attributes.Has(core.AttrBold) {
		style = style.Bold(true)
	}
	if s.Attributes.Has(core.AttrDim) {
		style = style.Dim(true)
	}
	if s.Attributes.Has(core.AttrItalic) {
		style = style.Italic(true)
	}
	if s.Attributes.Has(core.AttrUnderline) {
		style = style.Underline(true)
	}
	if s.Attributes.Has(core.AttrBlink) {
		style = style.Blink(true)
	}
	if s.Attributes.Has(core.AttrReverse) {
		style = style.Reverse(true)
	}
	if s.Attributes.Has(core.AttrStrikethrough) {
		style = style.StrikeThrough(true)
	}

	return style
}

// convertTcellStyle converts tcell.Style back to our Style.
func convertTcellStyle(ts tcell.Style) core.Style {
	fg, bg, attrs := ts.Decompose()

	s := core.Style{
		Foreground: convertTcellColor(fg),
		Background: convertTcellColor(bg),
		Attributes: core.AttrNone,
	}

	if attrs&tcell.AttrBold != 0 {
		s.Attributes |= core.AttrBold
	}
	if attrs&tcell.AttrDim != 0 {
		s.Attributes |= core.AttrDim
	}
	if attrs&tcell.AttrItalic != 0 {
		s.Attributes |= core.AttrItalic
	}
	if attrs&tcell.AttrUnderline != 0 {
		s.Attributes |= core.AttrUnderline
	}
	if attrs&tcell.AttrBlink != 0 {
		s.Attributes |= core.AttrBlink
	}
	if attrs&tcell.AttrReverse != 0 {
		s.Attributes |= core.AttrReverse
	}
	if attrs&tcell.AttrStrikeThrough != 0 {
		s.Attributes |= core.AttrStrikethrough
	}

	return s
}

// convertTcellColor converts tcell.Color to our Color.
func convertTcellColor(tc tcell.Color) core.Color {
	if tc == tcell.ColorDefault {
		return core.ColorDefault
	}

	// Check if it's a palette color
	if tc >= tcell.ColorValid && tc < tcell.ColorIsRGB {
		return core.ColorFromIndex(uint8(tc - tcell.ColorValid))
	}

	// True color
	r, g, b := tc.RGB()
	return core.ColorFromRGB(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// convertEvent converts tcell events to our Event type.
func convertEvent(ev tcell.Event, t *Terminal) Event {
	switch e := ev.(type) {
	case *tcell.EventKey:
		return Event{
			Type: EventKey,
			Key:  convertKey(e.Key()),
			Rune: e.Rune(),
			Mod:  convertMod(e.Modifiers()),
		}

	case *tcell.EventMouse:
		x, y := e.Position()
		return Event{
			Type:        EventMouse,
			MouseX:      x,
			MouseY:      y,
			MouseButton: convertMouseButton(e.Buttons()),
			Mod:         convertMod(e.Modifiers()),
		}

	case *tcell.EventResize:
		w, h := e.Size()
		if t.resizeHandler != nil {
			t.resizeHandler(w, h)
		}
		return Event{
			Type:   EventResize,
			Width:  w,
			Height: h,
		}

	case *tcell.EventPaste:
		// EventPaste marks start/end of bracketed paste
		// The actual content comes as key events between start and end
		return Event{
			Type:    EventPaste,
			Focused: e.Start(), // Repurpose Focused to indicate start vs end
		}

	case *tcell.EventFocus:
		return Event{
			Type:    EventFocus,
			Focused: e.Focused,
		}

	default:
		return Event{Type: EventNone}
	}
}

// convertKey converts tcell key to our Key type.
func convertKey(k tcell.Key) Key {
	switch k {
	case tcell.KeyRune:
		return KeyRune
	case tcell.KeyEscape:
		return KeyEscape
	case tcell.KeyEnter:
		return KeyEnter
	case tcell.KeyTab:
		return KeyTab
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return KeyBackspace
	case tcell.KeyDelete:
		return KeyDelete
	case tcell.KeyInsert:
		return KeyInsert
	case tcell.KeyHome:
		return KeyHome
	case tcell.KeyEnd:
		return KeyEnd
	case tcell.KeyPgUp:
		return KeyPageUp
	case tcell.KeyPgDn:
		return KeyPageDown
	case tcell.KeyUp:
		return KeyUp
	case tcell.KeyDown:
		return KeyDown
	case tcell.KeyLeft:
		return KeyLeft
	case tcell.KeyRight:
		return KeyRight
	case tcell.KeyF1:
		return KeyF1
	case tcell.KeyF2:
		return KeyF2
	case tcell.KeyF3:
		return KeyF3
	case tcell.KeyF4:
		return KeyF4
	case tcell.KeyF5:
		return KeyF5
	case tcell.KeyF6:
		return KeyF6
	case tcell.KeyF7:
		return KeyF7
	case tcell.KeyF8:
		return KeyF8
	case tcell.KeyF9:
		return KeyF9
	case tcell.KeyF10:
		return KeyF10
	case tcell.KeyF11:
		return KeyF11
	case tcell.KeyF12:
		return KeyF12
	case tcell.KeyCtrlSpace:
		return KeyCtrlSpace
	case tcell.KeyCtrlA:
		return KeyCtrlA
	case tcell.KeyCtrlB:
		return KeyCtrlB
	case tcell.KeyCtrlC:
		return KeyCtrlC
	case tcell.KeyCtrlD:
		return KeyCtrlD
	case tcell.KeyCtrlE:
		return KeyCtrlE
	case tcell.KeyCtrlF:
		return KeyCtrlF
	case tcell.KeyCtrlG:
		return KeyCtrlG
	case tcell.KeyCtrlH:
		return KeyCtrlH
	case tcell.KeyCtrlI:
		return KeyCtrlI
	case tcell.KeyCtrlJ:
		return KeyCtrlJ
	case tcell.KeyCtrlK:
		return KeyCtrlK
	case tcell.KeyCtrlL:
		return KeyCtrlL
	case tcell.KeyCtrlM:
		return KeyCtrlM
	case tcell.KeyCtrlN:
		return KeyCtrlN
	case tcell.KeyCtrlO:
		return KeyCtrlO
	case tcell.KeyCtrlP:
		return KeyCtrlP
	case tcell.KeyCtrlQ:
		return KeyCtrlQ
	case tcell.KeyCtrlR:
		return KeyCtrlR
	case tcell.KeyCtrlS:
		return KeyCtrlS
	case tcell.KeyCtrlT:
		return KeyCtrlT
	case tcell.KeyCtrlU:
		return KeyCtrlU
	case tcell.KeyCtrlV:
		return KeyCtrlV
	case tcell.KeyCtrlW:
		return KeyCtrlW
	case tcell.KeyCtrlX:
		return KeyCtrlX
	case tcell.KeyCtrlY:
		return KeyCtrlY
	case tcell.KeyCtrlZ:
		return KeyCtrlZ
	default:
		return KeyNone
	}
}

// convertToTcellKey converts our Key to tcell.Key.
func convertToTcellKey(k Key) tcell.Key {
	switch k {
	case KeyRune:
		return tcell.KeyRune
	case KeyEscape:
		return tcell.KeyEscape
	case KeyEnter:
		return tcell.KeyEnter
	case KeyTab:
		return tcell.KeyTab
	case KeyBackspace:
		return tcell.KeyBackspace2
	case KeyDelete:
		return tcell.KeyDelete
	case KeyInsert:
		return tcell.KeyInsert
	case KeyHome:
		return tcell.KeyHome
	case KeyEnd:
		return tcell.KeyEnd
	case KeyPageUp:
		return tcell.KeyPgUp
	case KeyPageDown:
		return tcell.KeyPgDn
	case KeyUp:
		return tcell.KeyUp
	case KeyDown:
		return tcell.KeyDown
	case KeyLeft:
		return tcell.KeyLeft
	case KeyRight:
		return tcell.KeyRight
	case KeyF1:
		return tcell.KeyF1
	case KeyF2:
		return tcell.KeyF2
	case KeyF3:
		return tcell.KeyF3
	case KeyF4:
		return tcell.KeyF4
	case KeyF5:
		return tcell.KeyF5
	case KeyF6:
		return tcell.KeyF6
	case KeyF7:
		return tcell.KeyF7
	case KeyF8:
		return tcell.KeyF8
	case KeyF9:
		return tcell.KeyF9
	case KeyF10:
		return tcell.KeyF10
	case KeyF11:
		return tcell.KeyF11
	case KeyF12:
		return tcell.KeyF12
	case KeyCtrlA:
		return tcell.KeyCtrlA
	case KeyCtrlB:
		return tcell.KeyCtrlB
	case KeyCtrlC:
		return tcell.KeyCtrlC
	case KeyCtrlD:
		return tcell.KeyCtrlD
	case KeyCtrlE:
		return tcell.KeyCtrlE
	case KeyCtrlF:
		return tcell.KeyCtrlF
	case KeyCtrlG:
		return tcell.KeyCtrlG
	case KeyCtrlH:
		return tcell.KeyCtrlH
	case KeyCtrlI:
		return tcell.KeyCtrlI
	case KeyCtrlJ:
		return tcell.KeyCtrlJ
	case KeyCtrlK:
		return tcell.KeyCtrlK
	case KeyCtrlL:
		return tcell.KeyCtrlL
	case KeyCtrlM:
		return tcell.KeyCtrlM
	case KeyCtrlN:
		return tcell.KeyCtrlN
	case KeyCtrlO:
		return tcell.KeyCtrlO
	case KeyCtrlP:
		return tcell.KeyCtrlP
	case KeyCtrlQ:
		return tcell.KeyCtrlQ
	case KeyCtrlR:
		return tcell.KeyCtrlR
	case KeyCtrlS:
		return tcell.KeyCtrlS
	case KeyCtrlT:
		return tcell.KeyCtrlT
	case KeyCtrlU:
		return tcell.KeyCtrlU
	case KeyCtrlV:
		return tcell.KeyCtrlV
	case KeyCtrlW:
		return tcell.KeyCtrlW
	case KeyCtrlX:
		return tcell.KeyCtrlX
	case KeyCtrlY:
		return tcell.KeyCtrlY
	case KeyCtrlZ:
		return tcell.KeyCtrlZ
	default:
		return tcell.KeyRune
	}
}

// convertMod converts tcell modifier mask to our ModMask.
func convertMod(m tcell.ModMask) ModMask {
	var result ModMask
	if m&tcell.ModShift != 0 {
		result |= ModShift
	}
	if m&tcell.ModCtrl != 0 {
		result |= ModCtrl
	}
	if m&tcell.ModAlt != 0 {
		result |= ModAlt
	}
	if m&tcell.ModMeta != 0 {
		result |= ModMeta
	}
	return result
}

// convertToTcellMod converts our ModMask to tcell.ModMask.
func convertToTcellMod(m ModMask) tcell.ModMask {
	var result tcell.ModMask
	if m&ModShift != 0 {
		result |= tcell.ModShift
	}
	if m&ModCtrl != 0 {
		result |= tcell.ModCtrl
	}
	if m&ModAlt != 0 {
		result |= tcell.ModAlt
	}
	if m&ModMeta != 0 {
		result |= tcell.ModMeta
	}
	return result
}

// convertMouseButton converts tcell button mask to our MouseButton.
func convertMouseButton(b tcell.ButtonMask) MouseButton {
	switch {
	case b&tcell.Button1 != 0:
		return MouseLeft
	case b&tcell.Button2 != 0:
		return MouseMiddle
	case b&tcell.Button3 != 0:
		return MouseRight
	case b&tcell.WheelUp != 0:
		return MouseWheelUp
	case b&tcell.WheelDown != 0:
		return MouseWheelDown
	case b&tcell.WheelLeft != 0:
		return MouseWheelLeft
	case b&tcell.WheelRight != 0:
		return MouseWheelRight
	default:
		return MouseNone
	}
}
