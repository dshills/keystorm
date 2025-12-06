package terminal

import (
	"testing"
)

func TestParserPlainText(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("Hello"))

	text := s.GetTextRange(0, 0, 4, 0)
	if text != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", text)
	}
}

func TestParserNewline(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// LF (0x0A) only moves cursor down, not to column 0
	// To simulate typical shell behavior (CR+LF), use \r\n
	p.Parse([]byte("A\r\nB"))

	cell := s.Cell(0, 0)
	if cell.Rune != 'A' {
		t.Errorf("expected 'A' on line 0, got %c", cell.Rune)
	}

	cell = s.Cell(0, 1)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' on line 1, got %c", cell.Rune)
	}
}

func TestParserCarriageReturn(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("ABC\rX"))

	cell := s.Cell(0, 0)
	if cell.Rune != 'X' {
		t.Errorf("expected 'X' at position 0 (overwritten), got %c", cell.Rune)
	}

	cell = s.Cell(1, 0)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' at position 1, got %c", cell.Rune)
	}
}

func TestParserTab(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("A\tB"))

	cell := s.Cell(0, 0)
	if cell.Rune != 'A' {
		t.Errorf("expected 'A' at position 0, got %c", cell.Rune)
	}

	cell = s.Cell(8, 0)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' at position 8 (after tab), got %c", cell.Rune)
	}
}

func TestParserBackspace(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("AB\bC"))

	text := s.GetTextRange(0, 0, 1, 0)
	if text != "AC" {
		t.Errorf("expected 'AC', got '%s'", text)
	}
}

func TestParserCSICursorUp(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(0, 10)
	p.Parse([]byte("\x1b[3A")) // Cursor up 3

	_, y := s.CursorPos()
	if y != 7 {
		t.Errorf("expected cursor y=7, got %d", y)
	}
}

func TestParserCSICursorDown(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(0, 5)
	p.Parse([]byte("\x1b[2B")) // Cursor down 2

	_, y := s.CursorPos()
	if y != 7 {
		t.Errorf("expected cursor y=7, got %d", y)
	}
}

func TestParserCSICursorForward(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(5, 0)
	p.Parse([]byte("\x1b[4C")) // Cursor forward 4

	x, _ := s.CursorPos()
	if x != 9 {
		t.Errorf("expected cursor x=9, got %d", x)
	}
}

func TestParserCSICursorBackward(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(10, 0)
	p.Parse([]byte("\x1b[3D")) // Cursor backward 3

	x, _ := s.CursorPos()
	if x != 7 {
		t.Errorf("expected cursor x=7, got %d", x)
	}
}

func TestParserCSICursorPosition(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[5;10H")) // Move to row 5, column 10

	x, y := s.CursorPos()
	if x != 9 || y != 4 { // 1-indexed in CSI, 0-indexed in screen
		t.Errorf("expected cursor at (9,4), got (%d,%d)", x, y)
	}
}

func TestParserCSICursorPositionDefault(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(10, 10)
	p.Parse([]byte("\x1b[H")) // Move to home (1,1)

	x, y := s.CursorPos()
	if x != 0 || y != 0 {
		t.Errorf("expected cursor at (0,0), got (%d,%d)", x, y)
	}
}

func TestParserCSIEraseDisplay(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Fill screen
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			s.SetCell(x, y, Cell{Rune: 'X'})
		}
	}

	s.MoveCursor(0, 0)
	p.Parse([]byte("\x1b[2J")) // Clear entire screen

	cell := s.Cell(40, 12)
	if cell.Rune != ' ' {
		t.Errorf("expected cleared cell, got %c", cell.Rune)
	}
}

func TestParserCSIEraseLine(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Fill a line
	for x := 0; x < 80; x++ {
		s.SetCell(x, 0, Cell{Rune: 'X'})
	}

	s.MoveCursor(40, 0)
	p.Parse([]byte("\x1b[K")) // Clear from cursor to end of line

	// Before cursor should be 'X'
	cell := s.Cell(39, 0)
	if cell.Rune != 'X' {
		t.Errorf("expected 'X' before cursor, got %c", cell.Rune)
	}

	// At cursor and after should be cleared
	cell = s.Cell(40, 0)
	if cell.Rune != ' ' {
		t.Errorf("expected cleared cell at cursor, got %c", cell.Rune)
	}
}

func TestParserSGRBold(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[1mBold"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrBold) {
		t.Error("expected bold attribute")
	}
}

func TestParserSGRReset(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[1m\x1b[0mNormal"))

	cell := s.Cell(0, 0)
	if cell.Attributes.Has(AttrBold) {
		t.Error("expected bold to be reset")
	}
}

func TestParserSGRForeground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[31mRed")) // Red foreground

	cell := s.Cell(0, 0)
	if cell.Foreground != ColorRed {
		t.Errorf("expected red foreground, got %v", cell.Foreground)
	}
}

func TestParserSGRBackground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[44mBlue")) // Blue background

	cell := s.Cell(0, 0)
	if cell.Background != ColorBlue {
		t.Errorf("expected blue background, got %v", cell.Background)
	}
}

func TestParserSGR256Foreground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[38;5;196mRed256")) // 256-color red

	cell := s.Cell(0, 0)
	if cell.Foreground.Index != 196 {
		t.Errorf("expected 256-color 196, got %v", cell.Foreground)
	}
}

func TestParserSGR256Background(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[48;5;21mBlue256")) // 256-color blue

	cell := s.Cell(0, 0)
	if cell.Background.Index != 21 {
		t.Errorf("expected 256-color 21, got %v", cell.Background)
	}
}

func TestParserSGRRGBForeground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[38;2;255;128;64mRGB")) // RGB foreground

	cell := s.Cell(0, 0)
	if cell.Foreground.Index != -1 {
		t.Error("expected RGB foreground (index -1)")
	}

	if cell.Foreground.R != 255 || cell.Foreground.G != 128 || cell.Foreground.B != 64 {
		t.Errorf("expected (255,128,64), got (%d,%d,%d)", cell.Foreground.R, cell.Foreground.G, cell.Foreground.B)
	}
}

func TestParserSGRRGBBackground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[48;2;32;64;128mRGB")) // RGB background

	cell := s.Cell(0, 0)
	if cell.Background.Index != -1 {
		t.Error("expected RGB background (index -1)")
	}

	if cell.Background.R != 32 || cell.Background.G != 64 || cell.Background.B != 128 {
		t.Errorf("expected (32,64,128), got (%d,%d,%d)", cell.Background.R, cell.Background.G, cell.Background.B)
	}
}

func TestParserSGRUnderline(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[4mUnderlined"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrUnderline) {
		t.Error("expected underline attribute")
	}
}

func TestParserSGRItalic(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[3mItalic"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrItalic) {
		t.Error("expected italic attribute")
	}
}

func TestParserSGRBlink(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[5mBlinking"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrBlink) {
		t.Error("expected blink attribute")
	}
}

func TestParserSGRReverse(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[7mReverse"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrReverse) {
		t.Error("expected reverse attribute")
	}
}

func TestParserSGRHidden(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[8mHidden"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrHidden) {
		t.Error("expected hidden attribute")
	}
}

func TestParserSGRStrikethrough(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[9mStrikethrough"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrStrike) {
		t.Error("expected strikethrough attribute")
	}
}

func TestParserSGRMultiple(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[1;4;31mText")) // Bold, underline, red

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrBold) {
		t.Error("expected bold")
	}
	if !cell.Attributes.Has(AttrUnderline) {
		t.Error("expected underline")
	}
	if cell.Foreground != ColorRed {
		t.Error("expected red foreground")
	}
}

func TestParserOSCTitle(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	var title string
	p.SetTitleCallback(func(t string) {
		title = t
	})

	p.Parse([]byte("\x1b]0;My Title\x07"))

	if title != "My Title" {
		t.Errorf("expected 'My Title', got '%s'", title)
	}
}

func TestParserOSCTitleWithST(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	var title string
	p.SetTitleCallback(func(t string) {
		title = t
	})

	p.Parse([]byte("\x1b]2;Another Title\x1b\\"))

	if title != "Another Title" {
		t.Errorf("expected 'Another Title', got '%s'", title)
	}
}

func TestParserOSCCallback(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	var oscCmd int
	var oscData string
	p.SetOSCCallback(func(cmd int, data string) {
		oscCmd = cmd
		oscData = data
	})

	p.Parse([]byte("\x1b]7;/home/user\x07"))

	if oscCmd != 7 {
		t.Errorf("expected OSC 7, got %d", oscCmd)
	}
	if oscData != "/home/user" {
		t.Errorf("expected '/home/user', got '%s'", oscData)
	}
}

func TestParserCSIScrollUp(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Put content
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')

	p.Parse([]byte("\x1b[1S")) // Scroll up 1

	cell := s.Cell(0, 0)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' on line 0 after scroll, got %c", cell.Rune)
	}
}

func TestParserCSIScrollDown(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Put content
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')

	p.Parse([]byte("\x1b[1T")) // Scroll down 1

	cell := s.Cell(0, 0)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' on line 0 after scroll down, got %c", cell.Rune)
	}

	cell = s.Cell(0, 1)
	if cell.Rune != 'A' {
		t.Errorf("expected 'A' on line 1 after scroll down, got %c", cell.Rune)
	}
}

func TestParserCSIInsertLine(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')

	s.MoveCursor(0, 1)
	p.Parse([]byte("\x1b[1L")) // Insert 1 line

	cell := s.Cell(0, 1)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' on inserted line, got %c", cell.Rune)
	}

	cell = s.Cell(0, 2)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' pushed down, got %c", cell.Rune)
	}
}

func TestParserCSIDeleteLine(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')
	s.MoveCursor(0, 2)
	s.WriteRune('C')

	s.MoveCursor(0, 1)
	p.Parse([]byte("\x1b[1M")) // Delete 1 line

	cell := s.Cell(0, 1)
	if cell.Rune != 'C' {
		t.Errorf("expected 'C' on line 1 after delete, got %c", cell.Rune)
	}
}

func TestParserCSIInsertChar(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.WriteRune('B')
	s.WriteRune('C')

	s.MoveCursor(1, 0)
	p.Parse([]byte("\x1b[1@")) // Insert 1 char

	text := s.GetTextRange(0, 0, 3, 0)
	if text != "A BC" {
		t.Errorf("expected 'A BC', got '%s'", text)
	}
}

func TestParserCSIDeleteChar(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.WriteRune('B')
	s.WriteRune('C')

	s.MoveCursor(1, 0)
	p.Parse([]byte("\x1b[1P")) // Delete 1 char

	text := s.GetTextRange(0, 0, 1, 0)
	if text != "AC" {
		t.Errorf("expected 'AC', got '%s'", text)
	}
}

func TestParserCSIHideCursor(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[?25l")) // Hide cursor

	if s.CursorVisible() {
		t.Error("cursor should be hidden")
	}
}

func TestParserCSIShowCursor(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.SetCursorVisible(false)
	p.Parse([]byte("\x1b[?25h")) // Show cursor

	if !s.CursorVisible() {
		t.Error("cursor should be visible")
	}
}

func TestParserCSISaveCursor(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(10, 5)
	p.Parse([]byte("\x1b[s")) // Save cursor

	s.MoveCursor(0, 0)
	p.Parse([]byte("\x1b[u")) // Restore cursor

	x, y := s.CursorPos()
	if x != 10 || y != 5 {
		t.Errorf("expected cursor at (10,5), got (%d,%d)", x, y)
	}
}

func TestParserCSISetScrollRegion(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[5;15r")) // Set scroll region rows 5-15

	// Move cursor to check region was set
	// The screen should limit scrolling to this region
	// This is a basic test - full scroll region tests are in screen_test.go
}

func TestParserSGRBrightForeground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[91mBrightRed")) // Bright red foreground

	cell := s.Cell(0, 0)
	if cell.Foreground != ColorBrightRed {
		t.Errorf("expected bright red foreground, got %v", cell.Foreground)
	}
}

func TestParserSGRBrightBackground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[104mBrightBlue")) // Bright blue background

	cell := s.Cell(0, 0)
	if cell.Background != ColorBrightBlue {
		t.Errorf("expected bright blue background, got %v", cell.Background)
	}
}

func TestParserSGRDefaultForeground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[31m\x1b[39mText")) // Red then default

	cell := s.Cell(0, 0)
	if !cell.Foreground.Default {
		t.Errorf("expected default foreground, got %v", cell.Foreground)
	}
}

func TestParserSGRDefaultBackground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[44m\x1b[49mText")) // Blue bg then default

	cell := s.Cell(0, 0)
	if !cell.Background.Default {
		t.Errorf("expected default background, got %v", cell.Background)
	}
}

func TestParserCSIEraseDisplayModes(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Fill screen
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			s.SetCell(x, y, Cell{Rune: 'X'})
		}
	}

	// Test mode 0 (cursor to end)
	s.MoveCursor(40, 12)
	p.Parse([]byte("\x1b[0J"))

	if s.Cell(39, 12).Rune != 'X' {
		t.Error("cell before cursor should not be cleared with mode 0")
	}
	if s.Cell(40, 12).Rune != ' ' {
		t.Error("cell at cursor should be cleared with mode 0")
	}
}

func TestParserCSIEraseLineModes(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Fill line
	for x := 0; x < 80; x++ {
		s.SetCell(x, 0, Cell{Rune: 'X'})
	}

	// Test mode 1 (start to cursor)
	s.MoveCursor(40, 0)
	p.Parse([]byte("\x1b[1K"))

	if s.Cell(39, 0).Rune != ' ' {
		t.Error("cell before cursor should be cleared with mode 1")
	}
	if s.Cell(41, 0).Rune != 'X' {
		t.Error("cell after cursor should not be cleared with mode 1")
	}
}

func TestParserCSIEraseEntireLine(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Fill line
	for x := 0; x < 80; x++ {
		s.SetCell(x, 0, Cell{Rune: 'X'})
	}

	s.MoveCursor(40, 0)
	p.Parse([]byte("\x1b[2K"))

	if s.Cell(0, 0).Rune != ' ' {
		t.Error("start of line should be cleared")
	}
	if s.Cell(79, 0).Rune != ' ' {
		t.Error("end of line should be cleared")
	}
}

func TestParserFragmentedSequence(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	// Send escape sequence in parts
	p.Parse([]byte("\x1b["))
	p.Parse([]byte("5"))
	p.Parse([]byte(";10H"))

	x, y := s.CursorPos()
	if x != 9 || y != 4 {
		t.Errorf("expected cursor at (9,4), got (%d,%d)", x, y)
	}
}

func TestParserUnknownSequenceCallback(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	var unknownSeq string
	p.SetUnknownCallback(func(seq string) {
		unknownSeq = seq
	})

	p.Parse([]byte("\x1b[999z")) // Unknown CSI sequence

	if unknownSeq == "" {
		t.Error("expected unknown sequence callback to be called")
	}
}

func TestParserCursorNextLine(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(10, 5)
	p.Parse([]byte("\x1b[2E")) // Cursor to start of 2nd next line

	x, y := s.CursorPos()
	if x != 0 || y != 7 {
		t.Errorf("expected cursor at (0,7), got (%d,%d)", x, y)
	}
}

func TestParserCursorPrevLine(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(10, 5)
	p.Parse([]byte("\x1b[2F")) // Cursor to start of 2nd previous line

	x, y := s.CursorPos()
	if x != 0 || y != 3 {
		t.Errorf("expected cursor at (0,3), got (%d,%d)", x, y)
	}
}

func TestParserCursorColumn(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(10, 5)
	p.Parse([]byte("\x1b[20G")) // Move to column 20

	x, y := s.CursorPos()
	if x != 19 || y != 5 { // 1-indexed in CSI
		t.Errorf("expected cursor at (19,5), got (%d,%d)", x, y)
	}
}

func TestParserDECSCRC(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	s.MoveCursor(10, 5)
	s.AddAttribute(AttrBold)
	p.Parse([]byte("\x1b7")) // DECSC - save cursor

	s.MoveCursor(0, 0)
	s.ResetAttributes()
	p.Parse([]byte("\x1b8")) // DECRC - restore cursor

	x, y := s.CursorPos()
	if x != 10 || y != 5 {
		t.Errorf("expected cursor at (10,5), got (%d,%d)", x, y)
	}
}

func TestParserSGRDim(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.Parse([]byte("\x1b[2mDim"))

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrDim) {
		t.Error("expected dim attribute")
	}
}

func TestParserString(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)

	p.ParseString("Hello")

	text := s.GetTextRange(0, 0, 4, 0)
	if text != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", text)
	}
}
