package terminal

import (
	"strconv"
	"strings"
)

// Parser parses ANSI escape sequences and updates the screen.
type Parser struct {
	screen *Screen

	// Parser state
	state  parserState
	params []int
	inter  []byte // intermediate bytes
	osc    []byte // OSC data

	// UTF-8 decoding state
	utf8Buf   [4]byte // buffer for UTF-8 sequence
	utf8Len   int     // expected length of current UTF-8 sequence
	utf8Count int     // bytes collected so far

	// Callbacks
	onTitle   func(string)
	onOSC     func(cmd int, data string)
	onUnknown func(seq string)
}

type parserState int

const (
	stateGround parserState = iota
	stateEscape
	stateEscapeInter
	stateCSI
	stateCSIParam
	stateCSIInter
	stateOSC
	stateDCS
)

// NewParser creates a new ANSI parser for the given screen.
func NewParser(screen *Screen) *Parser {
	return &Parser{
		screen: screen,
		state:  stateGround,
		params: make([]int, 0, 16),
		inter:  make([]byte, 0, 4),
		osc:    make([]byte, 0, 256),
	}
}

// SetTitleCallback sets the callback for title changes.
func (p *Parser) SetTitleCallback(fn func(string)) {
	p.onTitle = fn
}

// SetOSCCallback sets the callback for OSC sequences.
func (p *Parser) SetOSCCallback(fn func(cmd int, data string)) {
	p.onOSC = fn
}

// SetUnknownCallback sets the callback for unknown sequences.
func (p *Parser) SetUnknownCallback(fn func(seq string)) {
	p.onUnknown = fn
}

// Parse parses the given data and updates the screen.
func (p *Parser) Parse(data []byte) {
	for _, b := range data {
		p.processByte(b)
	}
}

// ParseString parses the given string and updates the screen.
func (p *Parser) ParseString(s string) {
	p.Parse([]byte(s))
}

func (p *Parser) processByte(b byte) {
	switch p.state {
	case stateGround:
		p.processGround(b)
	case stateEscape:
		p.processEscape(b)
	case stateEscapeInter:
		p.processEscapeInter(b)
	case stateCSI:
		p.processCSI(b)
	case stateCSIParam:
		p.processCSIParam(b)
	case stateCSIInter:
		p.processCSIInter(b)
	case stateOSC:
		p.processOSC(b)
	case stateDCS:
		p.processDCS(b)
	}
}

func (p *Parser) processGround(b byte) {
	// If we're in the middle of a UTF-8 sequence, continue collecting
	if p.utf8Len > 0 {
		p.processUTF8Continuation(b)
		return
	}

	switch {
	case b == 0x1B: // ESC
		// Reset UTF-8 state if incomplete sequence
		if p.utf8Len > 0 {
			p.screen.WriteRune('\uFFFD') // Output replacement for incomplete sequence
			p.utf8Len = 0
			p.utf8Count = 0
		}
		p.state = stateEscape
		p.params = p.params[:0]
		p.inter = p.inter[:0]
	case b == 0x07: // BEL
		// Bell - ignore
	case b == 0x08: // BS - Backspace
		p.screen.MoveCursorRelative(-1, 0)
	case b == 0x09: // HT - Tab
		p.handleTab()
	case b == 0x0A, b == 0x0B, b == 0x0C: // LF, VT, FF
		p.screen.LineFeed()
	case b == 0x0D: // CR
		p.screen.CarriageReturn()
	case b >= 0x20 && b < 0x7F: // Printable ASCII
		p.screen.WriteRune(rune(b))
	case b >= 0xC0 && b < 0xE0: // 2-byte UTF-8 start
		p.utf8Buf[0] = b
		p.utf8Len = 2
		p.utf8Count = 1
	case b >= 0xE0 && b < 0xF0: // 3-byte UTF-8 start
		p.utf8Buf[0] = b
		p.utf8Len = 3
		p.utf8Count = 1
	case b >= 0xF0 && b < 0xF8: // 4-byte UTF-8 start
		p.utf8Buf[0] = b
		p.utf8Len = 4
		p.utf8Count = 1
	case b >= 0x80 && b < 0xC0: // Unexpected continuation byte
		// Invalid UTF-8, output replacement character
		p.screen.WriteRune('\uFFFD')
	default:
		// Control characters in 0x00-0x1F (except handled above) or invalid bytes
		// Ignore them
	}
}

// processUTF8Continuation handles continuation bytes of a multi-byte UTF-8 sequence.
func (p *Parser) processUTF8Continuation(b byte) {
	// Check if this is a valid continuation byte
	if b < 0x80 || b >= 0xC0 {
		// Invalid continuation - reset and process byte normally
		p.utf8Len = 0
		p.utf8Count = 0
		p.screen.WriteRune('\uFFFD') // Output replacement for incomplete sequence
		p.processGround(b)           // Re-process the byte
		return
	}

	// Add continuation byte
	p.utf8Buf[p.utf8Count] = b
	p.utf8Count++

	// Check if sequence is complete
	if p.utf8Count == p.utf8Len {
		// Decode the UTF-8 sequence
		r := p.decodeUTF8()
		p.utf8Len = 0
		p.utf8Count = 0
		p.screen.WriteRune(r)
	}
}

// decodeUTF8 decodes the collected UTF-8 bytes into a rune.
func (p *Parser) decodeUTF8() rune {
	switch p.utf8Len {
	case 2:
		r := rune(p.utf8Buf[0]&0x1F)<<6 |
			rune(p.utf8Buf[1]&0x3F)
		// Validate: must be >= 0x80 (overlong check)
		if r < 0x80 {
			return '\uFFFD'
		}
		return r
	case 3:
		r := rune(p.utf8Buf[0]&0x0F)<<12 |
			rune(p.utf8Buf[1]&0x3F)<<6 |
			rune(p.utf8Buf[2]&0x3F)
		// Validate: must be >= 0x800 and not surrogate
		if r < 0x800 || (r >= 0xD800 && r <= 0xDFFF) {
			return '\uFFFD'
		}
		return r
	case 4:
		r := rune(p.utf8Buf[0]&0x07)<<18 |
			rune(p.utf8Buf[1]&0x3F)<<12 |
			rune(p.utf8Buf[2]&0x3F)<<6 |
			rune(p.utf8Buf[3]&0x3F)
		// Validate: must be >= 0x10000 and <= 0x10FFFF
		if r < 0x10000 || r > 0x10FFFF {
			return '\uFFFD'
		}
		return r
	default:
		return '\uFFFD'
	}
}

func (p *Parser) processEscape(b byte) {
	switch {
	case b == '[': // CSI
		p.state = stateCSI
	case b == ']': // OSC
		p.state = stateOSC
		p.osc = p.osc[:0]
	case b == 'P': // DCS
		p.state = stateDCS
	case b == '7': // DECSC - Save cursor
		p.screen.SaveCursor()
		p.state = stateGround
	case b == '8': // DECRC - Restore cursor
		p.screen.RestoreCursor()
		p.state = stateGround
	case b == 'D': // IND - Index (line feed)
		p.screen.LineFeed()
		p.state = stateGround
	case b == 'E': // NEL - Next line
		p.screen.CarriageReturn()
		p.screen.LineFeed()
		p.state = stateGround
	case b == 'M': // RI - Reverse index
		p.screen.ReverseLineFeed()
		p.state = stateGround
	case b == 'c': // RIS - Reset
		p.screen.Reset()
		p.state = stateGround
	case b == '\\': // ST - String terminator
		p.state = stateGround
	case b >= 0x20 && b <= 0x2F: // Intermediate
		p.inter = append(p.inter, b)
		p.state = stateEscapeInter
	case b >= 0x30 && b <= 0x7E: // Final
		p.handleEscapeSequence(b)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) processEscapeInter(b byte) {
	switch {
	case b >= 0x20 && b <= 0x2F: // More intermediate
		p.inter = append(p.inter, b)
	case b >= 0x30 && b <= 0x7E: // Final
		p.handleEscapeSequence(b)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) processCSI(b byte) {
	switch {
	case b >= '0' && b <= '9':
		p.params = append(p.params, int(b-'0'))
		p.state = stateCSIParam
	case b == ';':
		p.params = append(p.params, 0)
		p.state = stateCSIParam
	case b == '?', b == '>', b == '!': // Private mode prefix
		p.inter = append(p.inter, b)
	case b >= 0x20 && b <= 0x2F: // Intermediate
		p.inter = append(p.inter, b)
		p.state = stateCSIInter
	case b >= 0x40 && b <= 0x7E: // Final
		p.handleCSI(b)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) processCSIParam(b byte) {
	switch {
	case b >= '0' && b <= '9':
		if len(p.params) == 0 {
			p.params = append(p.params, 0)
		}
		p.params[len(p.params)-1] = p.params[len(p.params)-1]*10 + int(b-'0')
	case b == ';':
		p.params = append(p.params, 0)
	case b >= 0x20 && b <= 0x2F: // Intermediate
		p.inter = append(p.inter, b)
		p.state = stateCSIInter
	case b >= 0x40 && b <= 0x7E: // Final
		p.handleCSI(b)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) processCSIInter(b byte) {
	switch {
	case b >= 0x20 && b <= 0x2F: // More intermediate
		p.inter = append(p.inter, b)
	case b >= 0x40 && b <= 0x7E: // Final
		p.handleCSI(b)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) processOSC(b byte) {
	switch {
	case b == 0x07: // BEL terminates OSC
		p.handleOSC()
		p.state = stateGround
	case b == 0x1B: // ESC might start ST
		// Check next byte
		p.handleOSC()
		p.state = stateEscape
	case b == 0x9C: // ST (single byte)
		p.handleOSC()
		p.state = stateGround
	default:
		p.osc = append(p.osc, b)
	}
}

func (p *Parser) processDCS(b byte) {
	// For now, just consume until ST
	switch {
	case b == 0x1B:
		p.state = stateEscape
	case b == 0x9C:
		p.state = stateGround
	}
}

func (p *Parser) handleTab() {
	x, _ := p.screen.CursorPos()
	// Move to next tab stop (every 8 columns)
	nextTab := ((x / 8) + 1) * 8
	if nextTab >= p.screen.Width() {
		nextTab = p.screen.Width() - 1
	}
	p.screen.MoveCursor(nextTab, -1)
}

func (p *Parser) handleEscapeSequence(final byte) {
	// Handle escape sequences like ESC ( A, ESC ) 0, etc.
	// These are mostly charset selection, which we ignore for now
	if p.onUnknown != nil {
		p.onUnknown("ESC " + string(p.inter) + string(final))
	}
}

func (p *Parser) handleCSI(final byte) {
	// Check for private mode sequences
	private := len(p.inter) > 0 && p.inter[0] == '?'

	switch final {
	case 'A': // CUU - Cursor Up
		n := p.param(0, 1)
		p.screen.MoveCursorRelative(0, -n)

	case 'B': // CUD - Cursor Down
		n := p.param(0, 1)
		p.screen.MoveCursorRelative(0, n)

	case 'C': // CUF - Cursor Forward
		n := p.param(0, 1)
		p.screen.MoveCursorRelative(n, 0)

	case 'D': // CUB - Cursor Back
		n := p.param(0, 1)
		p.screen.MoveCursorRelative(-n, 0)

	case 'E': // CNL - Cursor Next Line
		n := p.param(0, 1)
		p.screen.CarriageReturn()
		for i := 0; i < n; i++ {
			p.screen.LineFeed()
		}

	case 'F': // CPL - Cursor Previous Line
		n := p.param(0, 1)
		p.screen.CarriageReturn()
		for i := 0; i < n; i++ {
			p.screen.ReverseLineFeed()
		}

	case 'G': // CHA - Cursor Horizontal Absolute
		n := p.param(0, 1)
		_, y := p.screen.CursorPos()
		p.screen.MoveCursor(n-1, y)

	case 'H', 'f': // CUP/HVP - Cursor Position
		row := p.param(0, 1)
		col := p.param(1, 1)
		p.screen.MoveCursor(col-1, row-1)

	case 'J': // ED - Erase Display
		n := p.param(0, 0)
		switch n {
		case 0:
			p.screen.ClearScreenBelow()
		case 1:
			p.screen.ClearScreenAbove()
		case 2, 3:
			p.screen.ClearScreen()
		}

	case 'K': // EL - Erase Line
		n := p.param(0, 0)
		switch n {
		case 0:
			p.screen.ClearLineRight()
		case 1:
			p.screen.ClearLineLeft()
		case 2:
			p.screen.ClearLine()
		}

	case 'L': // IL - Insert Lines
		n := p.param(0, 1)
		p.screen.InsertLines(n)

	case 'M': // DL - Delete Lines
		n := p.param(0, 1)
		p.screen.DeleteLines(n)

	case 'P': // DCH - Delete Characters
		n := p.param(0, 1)
		p.screen.DeleteChars(n)

	case 'S': // SU - Scroll Up
		n := p.param(0, 1)
		p.screen.ScrollUp(n)

	case 'T': // SD - Scroll Down
		n := p.param(0, 1)
		p.screen.ScrollDown(n)

	case 'X': // ECH - Erase Characters
		n := p.param(0, 1)
		p.screen.EraseChars(n)

	case '@': // ICH - Insert Characters
		n := p.param(0, 1)
		p.screen.InsertChars(n)

	case 'd': // VPA - Vertical Position Absolute
		n := p.param(0, 1)
		x, _ := p.screen.CursorPos()
		p.screen.MoveCursor(x, n-1)

	case 'h': // SM - Set Mode
		if private {
			p.handlePrivateMode(true)
		}

	case 'l': // RM - Reset Mode
		if private {
			p.handlePrivateMode(false)
		}

	case 'm': // SGR - Select Graphic Rendition
		p.handleSGR()

	case 'r': // DECSTBM - Set Scrolling Region
		top := p.param(0, 1)
		bottom := p.param(1, p.screen.Height())
		p.screen.SetScrollRegion(top-1, bottom-1)

	case 's': // SCP - Save Cursor Position
		p.screen.SaveCursor()

	case 'u': // RCP - Restore Cursor Position
		p.screen.RestoreCursor()

	case 'n': // DSR - Device Status Report
		// Ignore for now

	case 'c': // DA - Device Attributes
		// Ignore for now

	case 'q': // DECSCUSR - Set Cursor Style
		if len(p.inter) > 0 && p.inter[0] == ' ' {
			n := p.param(0, 1)
			switch n {
			case 0, 1, 2:
				p.screen.SetCursorStyle(CursorBlock)
			case 3, 4:
				p.screen.SetCursorStyle(CursorUnderline)
			case 5, 6:
				p.screen.SetCursorStyle(CursorBar)
			}
		}

	default:
		if p.onUnknown != nil {
			p.onUnknown("CSI " + string(p.inter) + formatParams(p.params) + string(final))
		}
	}
}

func (p *Parser) handlePrivateMode(set bool) {
	for _, mode := range p.params {
		switch mode {
		case 1: // DECCKM - Cursor Keys Mode (application/normal)
			// Application mode - affects arrow key sequences
		case 6: // DECOM - Origin Mode
			p.screen.SetOriginMode(set)
		case 7: // DECAWM - Auto Wrap Mode
			p.screen.SetAutoWrap(set)
		case 12: // Cursor blinking
			// Ignore
		case 25: // DECTCEM - Text Cursor Enable Mode
			p.screen.SetCursorVisible(set)
		case 47, 1047: // Alternate screen buffer
			// TODO: Implement alternate buffer
		case 1049: // Alternate screen buffer with save/restore cursor
			// TODO: Implement alternate buffer
		case 2004: // Bracketed paste mode
			// Ignore
		}
	}
}

func (p *Parser) handleSGR() {
	if len(p.params) == 0 {
		p.screen.ResetAttributes()
		return
	}

	i := 0
	for i < len(p.params) {
		param := p.params[i]
		switch param {
		case 0: // Reset
			p.screen.ResetAttributes()
		case 1: // Bold
			p.screen.AddAttribute(AttrBold)
		case 2: // Dim
			p.screen.AddAttribute(AttrDim)
		case 3: // Italic
			p.screen.AddAttribute(AttrItalic)
		case 4: // Underline
			p.screen.AddAttribute(AttrUnderline)
		case 5: // Blink
			p.screen.AddAttribute(AttrBlink)
		case 7: // Reverse
			p.screen.AddAttribute(AttrReverse)
		case 8: // Hidden
			p.screen.AddAttribute(AttrHidden)
		case 9: // Strikethrough
			p.screen.AddAttribute(AttrStrike)
		case 21: // Double underline (treat as underline)
			p.screen.AddAttribute(AttrUnderline)
		case 22: // Normal intensity (not bold, not dim)
			p.screen.RemoveAttribute(AttrBold | AttrDim)
		case 23: // Not italic
			p.screen.RemoveAttribute(AttrItalic)
		case 24: // Not underline
			p.screen.RemoveAttribute(AttrUnderline)
		case 25: // Not blink
			p.screen.RemoveAttribute(AttrBlink)
		case 27: // Not reverse
			p.screen.RemoveAttribute(AttrReverse)
		case 28: // Not hidden
			p.screen.RemoveAttribute(AttrHidden)
		case 29: // Not strikethrough
			p.screen.RemoveAttribute(AttrStrike)

		// Foreground colors
		case 30:
			p.screen.SetForeground(ColorBlack)
		case 31:
			p.screen.SetForeground(ColorRed)
		case 32:
			p.screen.SetForeground(ColorGreen)
		case 33:
			p.screen.SetForeground(ColorYellow)
		case 34:
			p.screen.SetForeground(ColorBlue)
		case 35:
			p.screen.SetForeground(ColorMagenta)
		case 36:
			p.screen.SetForeground(ColorCyan)
		case 37:
			p.screen.SetForeground(ColorWhite)
		case 38: // Extended foreground
			i = p.parseExtendedColor(i, true)
		case 39: // Default foreground
			p.screen.SetForeground(DefaultForeground)

		// Background colors
		case 40:
			p.screen.SetBackground(ColorBlack)
		case 41:
			p.screen.SetBackground(ColorRed)
		case 42:
			p.screen.SetBackground(ColorGreen)
		case 43:
			p.screen.SetBackground(ColorYellow)
		case 44:
			p.screen.SetBackground(ColorBlue)
		case 45:
			p.screen.SetBackground(ColorMagenta)
		case 46:
			p.screen.SetBackground(ColorCyan)
		case 47:
			p.screen.SetBackground(ColorWhite)
		case 48: // Extended background
			i = p.parseExtendedColor(i, false)
		case 49: // Default background
			p.screen.SetBackground(DefaultBackground)

		// Bright foreground colors
		case 90:
			p.screen.SetForeground(ColorBrightBlack)
		case 91:
			p.screen.SetForeground(ColorBrightRed)
		case 92:
			p.screen.SetForeground(ColorBrightGreen)
		case 93:
			p.screen.SetForeground(ColorBrightYellow)
		case 94:
			p.screen.SetForeground(ColorBrightBlue)
		case 95:
			p.screen.SetForeground(ColorBrightMagenta)
		case 96:
			p.screen.SetForeground(ColorBrightCyan)
		case 97:
			p.screen.SetForeground(ColorBrightWhite)

		// Bright background colors
		case 100:
			p.screen.SetBackground(ColorBrightBlack)
		case 101:
			p.screen.SetBackground(ColorBrightRed)
		case 102:
			p.screen.SetBackground(ColorBrightGreen)
		case 103:
			p.screen.SetBackground(ColorBrightYellow)
		case 104:
			p.screen.SetBackground(ColorBrightBlue)
		case 105:
			p.screen.SetBackground(ColorBrightMagenta)
		case 106:
			p.screen.SetBackground(ColorBrightCyan)
		case 107:
			p.screen.SetBackground(ColorBrightWhite)
		}
		i++
	}
}

func (p *Parser) parseExtendedColor(i int, foreground bool) int {
	if i+1 >= len(p.params) {
		return i
	}

	switch p.params[i+1] {
	case 5: // 256-color
		if i+2 < len(p.params) {
			idx := p.params[i+2]
			// Validate index is in 0-255 range
			if idx < 0 {
				idx = 0
			} else if idx > 255 {
				idx = 255
			}
			color := ColorFromIndex(idx)
			if foreground {
				p.screen.SetForeground(color)
			} else {
				p.screen.SetBackground(color)
			}
			return i + 2
		}
	case 2: // RGB
		if i+4 < len(p.params) {
			// Clamp RGB values to 0-255 range
			r := clampColorValue(p.params[i+2])
			g := clampColorValue(p.params[i+3])
			b := clampColorValue(p.params[i+4])
			color := ColorFromRGB(r, g, b)
			if foreground {
				p.screen.SetForeground(color)
			} else {
				p.screen.SetBackground(color)
			}
			return i + 4
		}
	}
	return i
}

// clampColorValue clamps an integer to valid RGB range (0-255).
func clampColorValue(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func (p *Parser) handleOSC() {
	data := string(p.osc)

	// Parse OSC command number
	parts := strings.SplitN(data, ";", 2)
	if len(parts) == 0 {
		return
	}

	cmd, err := strconv.Atoi(parts[0])
	if err != nil {
		return
	}

	value := ""
	if len(parts) > 1 {
		value = parts[1]
	}

	switch cmd {
	case 0: // Set icon name and window title
		if p.onTitle != nil {
			p.onTitle(value)
		}
	case 1: // Set icon name
		// Ignore
	case 2: // Set window title
		if p.onTitle != nil {
			p.onTitle(value)
		}
	default:
		if p.onOSC != nil {
			p.onOSC(cmd, value)
		}
	}
}

func (p *Parser) param(index, defaultValue int) int {
	if index < len(p.params) && p.params[index] > 0 {
		return p.params[index]
	}
	return defaultValue
}

func formatParams(params []int) string {
	if len(params) == 0 {
		return ""
	}
	var parts []string
	for _, p := range params {
		parts = append(parts, strconv.Itoa(p))
	}
	return strings.Join(parts, ";")
}
