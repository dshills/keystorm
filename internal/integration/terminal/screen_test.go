package terminal

import (
	"testing"
)

func TestNewScreen(t *testing.T) {
	s := NewScreen(80, 24)

	if s.Width() != 80 {
		t.Errorf("expected width 80, got %d", s.Width())
	}
	if s.Height() != 24 {
		t.Errorf("expected height 24, got %d", s.Height())
	}

	x, y := s.CursorPos()
	if x != 0 || y != 0 {
		t.Errorf("expected cursor at (0,0), got (%d,%d)", x, y)
	}
}

func TestScreenSetCell(t *testing.T) {
	s := NewScreen(80, 24)

	cell := Cell{Rune: 'A', Width: 1, Foreground: DefaultForeground, Background: DefaultBackground}
	s.SetCell(5, 10, cell)
	got := s.Cell(5, 10)

	if got.Rune != 'A' {
		t.Errorf("expected 'A', got %c", got.Rune)
	}
}

func TestScreenSetCellOutOfBounds(t *testing.T) {
	s := NewScreen(80, 24)

	cell := Cell{Rune: 'A'}
	// These should not panic
	s.SetCell(-1, 0, cell)
	s.SetCell(0, -1, cell)
	s.SetCell(80, 0, cell)
	s.SetCell(0, 24, cell)
}

func TestScreenCursorMovement(t *testing.T) {
	s := NewScreen(80, 24)

	s.MoveCursor(10, 5)
	x, y := s.CursorPos()
	if x != 10 || y != 5 {
		t.Errorf("expected cursor at (10,5), got (%d,%d)", x, y)
	}

	// Test cursor relative movement up
	s.MoveCursorRelative(0, -2)
	_, y = s.CursorPos()
	if y != 3 {
		t.Errorf("expected cursor y=3, got %d", y)
	}

	// Test cursor down
	s.MoveCursorRelative(0, 1)
	_, y = s.CursorPos()
	if y != 4 {
		t.Errorf("expected cursor y=4, got %d", y)
	}

	// Test cursor right
	s.MoveCursorRelative(5, 0)
	x, _ = s.CursorPos()
	if x != 15 {
		t.Errorf("expected cursor x=15, got %d", x)
	}

	// Test cursor left
	s.MoveCursorRelative(-3, 0)
	x, _ = s.CursorPos()
	if x != 12 {
		t.Errorf("expected cursor x=12, got %d", x)
	}
}

func TestScreenCursorBounds(t *testing.T) {
	s := NewScreen(80, 24)

	// Try to move cursor out of bounds - should clamp
	s.MoveCursor(100, 50)
	x, y := s.CursorPos()
	if x != 79 || y != 23 {
		t.Errorf("expected cursor at (79,23), got (%d,%d)", x, y)
	}

	s.MoveCursor(-5, -5)
	x, y = s.CursorPos()
	if x != 0 || y != 0 {
		t.Errorf("expected cursor at (0,0), got (%d,%d)", x, y)
	}
}

func TestScreenCursorUpBeyondTop(t *testing.T) {
	s := NewScreen(80, 24)
	s.MoveCursor(0, 2)

	s.MoveCursorRelative(0, -10) // Try to go up more than current y
	_, y := s.CursorPos()
	if y != 0 {
		t.Errorf("expected cursor y=0, got %d", y)
	}
}

func TestScreenWriteRune(t *testing.T) {
	s := NewScreen(80, 24)

	s.WriteRune('H')
	s.WriteRune('i')

	cell := s.Cell(0, 0)
	if cell.Rune != 'H' {
		t.Errorf("expected 'H', got %c", cell.Rune)
	}

	cell = s.Cell(1, 0)
	if cell.Rune != 'i' {
		t.Errorf("expected 'i', got %c", cell.Rune)
	}

	x, _ := s.CursorPos()
	if x != 2 {
		t.Errorf("expected cursor x=2, got %d", x)
	}
}

func TestScreenCarriageReturn(t *testing.T) {
	s := NewScreen(80, 24)
	s.MoveCursor(10, 5)

	s.CarriageReturn()
	x, y := s.CursorPos()
	if x != 0 || y != 5 {
		t.Errorf("expected cursor at (0,5), got (%d,%d)", x, y)
	}
}

func TestScreenLineFeed(t *testing.T) {
	s := NewScreen(80, 24)
	s.MoveCursor(10, 5)

	s.LineFeed()
	x, y := s.CursorPos()
	if x != 10 || y != 6 {
		t.Errorf("expected cursor at (10,6), got (%d,%d)", x, y)
	}
}

func TestScreenClearScreen(t *testing.T) {
	s := NewScreen(80, 24)

	// Fill screen with 'X'
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			s.SetCell(x, y, Cell{Rune: 'X'})
		}
	}

	s.ClearScreen()

	// Check cell
	cell := s.Cell(40, 12)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' at cursor, got %c", cell.Rune)
	}
}

func TestScreenClearScreenBelow(t *testing.T) {
	s := NewScreen(80, 24)

	// Fill screen with 'X'
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			s.SetCell(x, y, Cell{Rune: 'X'})
		}
	}

	s.MoveCursor(40, 12)
	s.ClearScreenBelow()

	// Check cell before cursor (should still be 'X')
	cell := s.Cell(39, 12)
	if cell.Rune != 'X' {
		t.Errorf("expected 'X' before cursor, got %c", cell.Rune)
	}

	// Check cell at cursor (should be cleared)
	cell = s.Cell(40, 12)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' at cursor, got %c", cell.Rune)
	}
}

func TestScreenClearScreenAbove(t *testing.T) {
	s := NewScreen(80, 24)

	// Fill screen with 'X'
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			s.SetCell(x, y, Cell{Rune: 'X'})
		}
	}

	s.MoveCursor(40, 12)
	s.ClearScreenAbove()

	// Check cell before cursor (should be cleared)
	cell := s.Cell(39, 12)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' before cursor, got %c", cell.Rune)
	}

	// Check cell after cursor (should still be 'X')
	cell = s.Cell(41, 12)
	if cell.Rune != 'X' {
		t.Errorf("expected 'X' after cursor, got %c", cell.Rune)
	}
}

func TestScreenClearLine(t *testing.T) {
	s := NewScreen(80, 24)

	// Fill a line with 'X'
	for x := 0; x < 80; x++ {
		s.SetCell(x, 10, Cell{Rune: 'X'})
	}

	s.MoveCursor(40, 10)
	s.ClearLine()

	// Check entire line is cleared
	cell := s.Cell(0, 10)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' at start of line, got %c", cell.Rune)
	}

	cell = s.Cell(79, 10)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' at end of line, got %c", cell.Rune)
	}
}

func TestScreenClearLineRight(t *testing.T) {
	s := NewScreen(80, 24)

	// Fill a line with 'X'
	for x := 0; x < 80; x++ {
		s.SetCell(x, 10, Cell{Rune: 'X'})
	}

	s.MoveCursor(40, 10)
	s.ClearLineRight()

	// Check cell before cursor
	cell := s.Cell(39, 10)
	if cell.Rune != 'X' {
		t.Errorf("expected 'X' before cursor, got %c", cell.Rune)
	}

	// Check cell at cursor
	cell = s.Cell(40, 10)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' at cursor, got %c", cell.Rune)
	}
}

func TestScreenScrollUp(t *testing.T) {
	s := NewScreen(80, 24)

	// Put content on first few lines
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')
	s.MoveCursor(0, 2)
	s.WriteRune('C')

	s.ScrollUp(1)

	// Line 0 should now have 'B'
	cell := s.Cell(0, 0)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' on line 0 after scroll, got %c", cell.Rune)
	}

	// Line 1 should now have 'C'
	cell = s.Cell(0, 1)
	if cell.Rune != 'C' {
		t.Errorf("expected 'C' on line 1 after scroll, got %c", cell.Rune)
	}

	// Last line should be empty
	cell = s.Cell(0, 23)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' on last line after scroll, got %c", cell.Rune)
	}
}

func TestScreenScrollDown(t *testing.T) {
	s := NewScreen(80, 24)

	// Put content on first few lines
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')
	s.MoveCursor(0, 2)
	s.WriteRune('C')

	s.ScrollDown(1)

	// Line 0 should be empty
	cell := s.Cell(0, 0)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' on line 0 after scroll down, got %c", cell.Rune)
	}

	// Line 1 should now have 'A'
	cell = s.Cell(0, 1)
	if cell.Rune != 'A' {
		t.Errorf("expected 'A' on line 1 after scroll down, got %c", cell.Rune)
	}
}

func TestScreenResize(t *testing.T) {
	s := NewScreen(80, 24)

	// Put some content
	s.MoveCursor(0, 0)
	s.WriteRune('A')

	s.Resize(100, 30)

	if s.Width() != 100 {
		t.Errorf("expected width 100, got %d", s.Width())
	}
	if s.Height() != 30 {
		t.Errorf("expected height 30, got %d", s.Height())
	}

	// Content should be preserved
	cell := s.Cell(0, 0)
	if cell.Rune != 'A' {
		t.Errorf("expected 'A' preserved after resize, got %c", cell.Rune)
	}
}

func TestScreenResizeSmaller(t *testing.T) {
	s := NewScreen(80, 24)

	// Put content at the edge
	s.MoveCursor(79, 23)
	s.WriteRune('Z')

	s.Resize(40, 12)

	if s.Width() != 40 {
		t.Errorf("expected width 40, got %d", s.Width())
	}
	if s.Height() != 12 {
		t.Errorf("expected height 12, got %d", s.Height())
	}

	// Cursor should be clamped
	x, y := s.CursorPos()
	if x >= 40 || y >= 12 {
		t.Errorf("cursor should be within new bounds, got (%d,%d)", x, y)
	}
}

func TestScreenSetForeground(t *testing.T) {
	s := NewScreen(80, 24)

	s.SetForeground(ColorRed)
	s.WriteRune('R')

	cell := s.Cell(0, 0)
	if cell.Foreground != ColorRed {
		t.Errorf("expected red foreground")
	}
}

func TestScreenSetBackground(t *testing.T) {
	s := NewScreen(80, 24)

	s.SetBackground(ColorBlue)
	s.WriteRune('B')

	cell := s.Cell(0, 0)
	if cell.Background != ColorBlue {
		t.Errorf("expected blue background")
	}
}

func TestScreenAddAttribute(t *testing.T) {
	s := NewScreen(80, 24)

	s.AddAttribute(AttrBold)
	s.AddAttribute(AttrUnderline)
	s.WriteRune('X')

	cell := s.Cell(0, 0)
	if !cell.Attributes.Has(AttrBold) {
		t.Error("expected bold attribute")
	}
	if !cell.Attributes.Has(AttrUnderline) {
		t.Error("expected underline attribute")
	}
}

func TestScreenResetAttributes(t *testing.T) {
	s := NewScreen(80, 24)

	s.AddAttribute(AttrBold)
	s.SetForeground(ColorRed)
	s.ResetAttributes()
	s.WriteRune('N')

	cell := s.Cell(0, 0)
	if cell.Attributes.Has(AttrBold) {
		t.Error("bold should be reset")
	}
	if !cell.Foreground.Default {
		t.Error("foreground should be default")
	}
}

func TestScreenSaveCursor(t *testing.T) {
	s := NewScreen(80, 24)

	s.MoveCursor(10, 5)
	s.AddAttribute(AttrBold)
	s.SaveCursor()

	s.MoveCursor(0, 0)
	s.ResetAttributes()

	s.RestoreCursor()

	x, y := s.CursorPos()
	if x != 10 || y != 5 {
		t.Errorf("expected cursor at (10,5), got (%d,%d)", x, y)
	}
}

func TestScreenCursorVisibility(t *testing.T) {
	s := NewScreen(80, 24)

	if !s.CursorVisible() {
		t.Error("cursor should be visible by default")
	}

	s.SetCursorVisible(false)
	if s.CursorVisible() {
		t.Error("cursor should be hidden")
	}

	s.SetCursorVisible(true)
	if !s.CursorVisible() {
		t.Error("cursor should be visible again")
	}
}

func TestScreenScrollRegion(t *testing.T) {
	s := NewScreen(80, 24)

	s.SetScrollRegion(5, 15)

	// Put content in region
	for i := 5; i <= 15; i++ {
		s.MoveCursor(0, i)
		s.WriteRune(rune('A' + i - 5))
	}

	// Scroll within region
	s.ScrollUp(1)

	// Line 5 should now have what was on line 6
	cell := s.Cell(0, 5)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' on line 5 after scroll, got %c", cell.Rune)
	}
}

func TestScreenGetText(t *testing.T) {
	s := NewScreen(80, 24)

	s.WriteRune('H')
	s.WriteRune('e')
	s.WriteRune('l')
	s.WriteRune('l')
	s.WriteRune('o')

	text := s.GetTextRange(0, 0, 4, 0)
	if text != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", text)
	}
}

func TestColorFromRGB(t *testing.T) {
	c := ColorFromRGB(128, 64, 32)

	if c.R != 128 || c.G != 64 || c.B != 32 {
		t.Errorf("expected (128,64,32), got (%d,%d,%d)", c.R, c.G, c.B)
	}

	if c.Index != -1 {
		t.Error("should be RGB color (index -1)")
	}
}

func TestColorFromIndex(t *testing.T) {
	c := ColorFromIndex(196)

	if c.Index != 196 {
		t.Errorf("expected index 196, got %d", c.Index)
	}
}

func TestScreenWrapAtEnd(t *testing.T) {
	s := NewScreen(10, 5)
	s.SetAutoWrap(true)

	// Move to end of line
	s.MoveCursor(9, 0)
	s.WriteRune('A')

	// Next character should wrap
	s.WriteRune('B')

	x, y := s.CursorPos()
	if x != 1 || y != 1 {
		t.Errorf("expected cursor at (1,1) after wrap, got (%d,%d)", x, y)
	}

	cell := s.Cell(0, 1)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' at start of next line, got %c", cell.Rune)
	}
}

func TestScreenInsertLines(t *testing.T) {
	s := NewScreen(80, 24)

	// Put content
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')
	s.MoveCursor(0, 2)
	s.WriteRune('C')

	// Insert line at position 1
	s.MoveCursor(0, 1)
	s.InsertLines(1)

	// Line 0 should still be 'A'
	cell := s.Cell(0, 0)
	if cell.Rune != 'A' {
		t.Errorf("expected 'A' on line 0, got %c", cell.Rune)
	}

	// Line 1 should be empty (inserted)
	cell = s.Cell(0, 1)
	if cell.Rune != ' ' {
		t.Errorf("expected ' ' on line 1 (inserted), got %c", cell.Rune)
	}

	// Line 2 should now have 'B'
	cell = s.Cell(0, 2)
	if cell.Rune != 'B' {
		t.Errorf("expected 'B' on line 2, got %c", cell.Rune)
	}
}

func TestScreenDeleteLines(t *testing.T) {
	s := NewScreen(80, 24)

	// Put content
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.MoveCursor(0, 1)
	s.WriteRune('B')
	s.MoveCursor(0, 2)
	s.WriteRune('C')

	// Delete line at position 1
	s.MoveCursor(0, 1)
	s.DeleteLines(1)

	// Line 0 should still be 'A'
	cell := s.Cell(0, 0)
	if cell.Rune != 'A' {
		t.Errorf("expected 'A' on line 0, got %c", cell.Rune)
	}

	// Line 1 should now have 'C'
	cell = s.Cell(0, 1)
	if cell.Rune != 'C' {
		t.Errorf("expected 'C' on line 1, got %c", cell.Rune)
	}
}

func TestScreenInsertChars(t *testing.T) {
	s := NewScreen(80, 24)

	// Put content: "ABCD"
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.WriteRune('B')
	s.WriteRune('C')
	s.WriteRune('D')

	// Insert at position 1
	s.MoveCursor(1, 0)
	s.InsertChars(1)

	// Check: "A BCD"
	if s.Cell(0, 0).Rune != 'A' {
		t.Error("A should stay at position 0")
	}
	if s.Cell(1, 0).Rune != ' ' {
		t.Error("Space should be inserted at position 1")
	}
	if s.Cell(2, 0).Rune != 'B' {
		t.Error("B should move to position 2")
	}
}

func TestScreenDeleteChars(t *testing.T) {
	s := NewScreen(80, 24)

	// Put content: "ABCD"
	s.MoveCursor(0, 0)
	s.WriteRune('A')
	s.WriteRune('B')
	s.WriteRune('C')
	s.WriteRune('D')

	// Delete at position 1
	s.MoveCursor(1, 0)
	s.DeleteChars(1)

	// Check: "ACD "
	if s.Cell(0, 0).Rune != 'A' {
		t.Error("A should stay at position 0")
	}
	if s.Cell(1, 0).Rune != 'C' {
		t.Error("C should move to position 1")
	}
	if s.Cell(2, 0).Rune != 'D' {
		t.Error("D should move to position 2")
	}
}

func TestScreenReset(t *testing.T) {
	s := NewScreen(80, 24)

	s.MoveCursor(10, 10)
	s.WriteRune('X')
	s.SetCursorVisible(false)
	s.AddAttribute(AttrBold)

	s.Reset()

	x, y := s.CursorPos()
	if x != 0 || y != 0 {
		t.Errorf("cursor should be at origin after reset, got (%d,%d)", x, y)
	}

	if !s.CursorVisible() {
		t.Error("cursor should be visible after reset")
	}

	// Cell should be cleared
	cell := s.Cell(10, 10)
	if cell.Rune != ' ' {
		t.Errorf("cell should be cleared after reset, got %c", cell.Rune)
	}
}

func TestEmptyCell(t *testing.T) {
	cell := EmptyCell()

	if cell.Rune != ' ' {
		t.Errorf("expected space, got %c", cell.Rune)
	}
	if cell.Width != 1 {
		t.Errorf("expected width 1, got %d", cell.Width)
	}
	if !cell.Foreground.Default {
		t.Error("expected default foreground")
	}
	if !cell.Background.Default {
		t.Error("expected default background")
	}
}

func TestNewLine(t *testing.T) {
	line := NewLine(80)

	if len(line.Cells) != 80 {
		t.Errorf("expected 80 cells, got %d", len(line.Cells))
	}

	// All cells should be empty
	for i, cell := range line.Cells {
		if cell.Rune != ' ' {
			t.Errorf("cell %d should be space, got %c", i, cell.Rune)
		}
	}
}

func TestLineClear(t *testing.T) {
	line := NewLine(10)
	line.Cells[0].Rune = 'X'
	line.Cells[5].Rune = 'Y'
	line.Wrapped = true

	line.Clear()

	if line.Cells[0].Rune != ' ' || line.Cells[5].Rune != ' ' {
		t.Error("cells should be cleared")
	}
	if line.Wrapped {
		t.Error("wrapped should be reset")
	}
}

func TestLineClearRange(t *testing.T) {
	line := NewLine(10)
	for i := range line.Cells {
		line.Cells[i].Rune = 'X'
	}

	line.ClearRange(3, 7)

	if line.Cells[2].Rune != 'X' {
		t.Error("cell 2 should not be cleared")
	}
	if line.Cells[3].Rune != ' ' {
		t.Error("cell 3 should be cleared")
	}
	if line.Cells[6].Rune != ' ' {
		t.Error("cell 6 should be cleared")
	}
	if line.Cells[7].Rune != 'X' {
		t.Error("cell 7 should not be cleared")
	}
}

func TestCellAttributesHas(t *testing.T) {
	attrs := AttrBold | AttrUnderline

	if !attrs.Has(AttrBold) {
		t.Error("should have bold")
	}
	if !attrs.Has(AttrUnderline) {
		t.Error("should have underline")
	}
	if attrs.Has(AttrItalic) {
		t.Error("should not have italic")
	}
}
