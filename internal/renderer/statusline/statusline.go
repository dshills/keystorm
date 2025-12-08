// Package statusline provides the status line and command line UI components.
package statusline

import (
	"github.com/dshills/keystorm/internal/renderer"
	"github.com/dshills/keystorm/internal/renderer/backend"
)

// StatusLine renders the bottom status line including mode display and command input.
type StatusLine struct {
	// Display state
	mode          string // Current mode name (e.g., "NORMAL", "INSERT")
	filename      string // Current filename (empty for scratch)
	modified      bool   // Buffer has unsaved changes
	line          uint32 // Current line (1-indexed for display)
	col           uint32 // Current column (1-indexed for display)
	totalLines    uint32 // Total lines in buffer
	percentScroll int    // Scroll percentage (0-100)

	// Command line state
	commandActive bool   // In command mode
	commandPrompt rune   // Prompt character (usually ':')
	commandBuffer string // Command being typed
	commandCursor int    // Cursor position in command

	// Message display
	message     string // Status message to display
	messageType MessageType

	// Style configuration
	modeStyles map[string]renderer.Style // Mode-specific styles

	// Dimensions
	width  int
	height int // Usually 1, but can be 2 for command line
}

// MessageType indicates the type of status message.
type MessageType int

const (
	MessageNone MessageType = iota
	MessageInfo
	MessageWarning
	MessageError
)

// New creates a new status line.
func New() *StatusLine {
	return &StatusLine{
		mode:          "NORMAL",
		commandPrompt: ':',
		modeStyles:    defaultModeStyles(),
		height:        1,
	}
}

// defaultModeStyles returns default styles for each mode.
func defaultModeStyles() map[string]renderer.Style {
	return map[string]renderer.Style{
		"NORMAL":   renderer.DefaultStyle().Bold().WithBackground(renderer.ColorBlue).WithForeground(renderer.ColorWhite),
		"INSERT":   renderer.DefaultStyle().Bold().WithBackground(renderer.ColorGreen).WithForeground(renderer.ColorBlack),
		"VISUAL":   renderer.DefaultStyle().Bold().WithBackground(renderer.ColorMagenta).WithForeground(renderer.ColorWhite),
		"V-LINE":   renderer.DefaultStyle().Bold().WithBackground(renderer.ColorMagenta).WithForeground(renderer.ColorWhite),
		"V-BLOCK":  renderer.DefaultStyle().Bold().WithBackground(renderer.ColorMagenta).WithForeground(renderer.ColorWhite),
		"COMMAND":  renderer.DefaultStyle().Bold().WithBackground(renderer.ColorYellow).WithForeground(renderer.ColorBlack),
		"REPLACE":  renderer.DefaultStyle().Bold().WithBackground(renderer.ColorRed).WithForeground(renderer.ColorWhite),
		"OPERATOR": renderer.DefaultStyle().Bold().WithBackground(renderer.ColorCyan).WithForeground(renderer.ColorBlack),
	}
}

// SetMode updates the displayed mode.
func (s *StatusLine) SetMode(mode string) {
	s.mode = mode
}

// SetFilename updates the displayed filename.
func (s *StatusLine) SetFilename(filename string) {
	s.filename = filename
}

// SetModified updates the modified indicator.
func (s *StatusLine) SetModified(modified bool) {
	s.modified = modified
}

// SetPosition updates the cursor position (1-indexed).
func (s *StatusLine) SetPosition(line, col uint32) {
	s.line = line
	s.col = col
}

// SetTotalLines updates the total line count.
func (s *StatusLine) SetTotalLines(total uint32) {
	s.totalLines = total
}

// SetScrollPercent updates the scroll percentage.
func (s *StatusLine) SetScrollPercent(percent int) {
	s.percentScroll = percent
}

// SetCommandMode activates command line display.
func (s *StatusLine) SetCommandMode(active bool, prompt rune) {
	s.commandActive = active
	s.commandPrompt = prompt
	if !active {
		s.commandBuffer = ""
		s.commandCursor = 0
	}
}

// SetCommandBuffer updates the command being typed.
func (s *StatusLine) SetCommandBuffer(buffer string, cursor int) {
	s.commandBuffer = buffer
	s.commandCursor = cursor
}

// SetMessage displays a status message.
func (s *StatusLine) SetMessage(msg string, msgType MessageType) {
	s.message = msg
	s.messageType = msgType
}

// ClearMessage clears the status message.
func (s *StatusLine) ClearMessage() {
	s.message = ""
	s.messageType = MessageNone
}

// Resize updates the status line dimensions.
func (s *StatusLine) Resize(width, height int) {
	s.width = width
	// Status line is always at the bottom, but we need screen height for positioning
}

// Height returns the number of rows the status line uses.
func (s *StatusLine) Height() int {
	if s.commandActive {
		return 2 // Mode line + command line
	}
	return 1
}

// Render draws the status line to the backend at the given row.
func (s *StatusLine) Render(b backend.Backend, row int) {
	if s.commandActive {
		// Render command line at row, status line at row-1
		s.renderStatusBar(b, row-1)
		s.renderCommandLine(b, row)
	} else if s.message != "" {
		// Show message instead of status bar
		s.renderMessage(b, row)
	} else {
		s.renderStatusBar(b, row)
	}
}

// renderStatusBar renders the mode and file info line.
func (s *StatusLine) renderStatusBar(b backend.Backend, row int) {
	// Get mode style
	modeStyle, ok := s.modeStyles[s.mode]
	if !ok {
		modeStyle = renderer.DefaultStyle().Bold().WithBackground(renderer.ColorGray)
	}

	// Status bar background
	barStyle := renderer.DefaultStyle().WithBackground(renderer.ColorGray).WithForeground(renderer.ColorWhite)

	// Clear the line first
	for x := 0; x < s.width; x++ {
		b.SetCell(x, row, renderer.Cell{Rune: ' ', Width: 1, Style: barStyle})
	}

	col := 0

	// Mode indicator with padding
	modeText := " " + s.mode + " "
	for _, r := range modeText {
		if col < s.width {
			b.SetCell(col, row, renderer.Cell{Rune: r, Width: 1, Style: modeStyle})
			col++
		}
	}

	// Separator space
	if col < s.width {
		b.SetCell(col, row, renderer.Cell{Rune: ' ', Width: 1, Style: barStyle})
		col++
	}

	// Filename (or [No Name])
	filename := s.filename
	if filename == "" {
		filename = "[No Name]"
	}
	if s.modified {
		filename += " [+]"
	}
	for _, r := range filename {
		if col < s.width-20 { // Leave room for position info
			b.SetCell(col, row, renderer.Cell{Rune: r, Width: 1, Style: barStyle})
			col++
		}
	}

	// Right side: position info
	posInfo := s.formatPosition()
	posStart := s.width - len(posInfo) - 1
	if posStart > col {
		for i, r := range posInfo {
			b.SetCell(posStart+i, row, renderer.Cell{Rune: r, Width: 1, Style: barStyle})
		}
	}
}

// renderCommandLine renders the command input line.
func (s *StatusLine) renderCommandLine(b backend.Backend, row int) {
	// Clear the line
	cmdStyle := renderer.DefaultStyle()
	for x := 0; x < s.width; x++ {
		b.SetCell(x, row, renderer.Cell{Rune: ' ', Width: 1, Style: cmdStyle})
	}

	// Draw prompt
	b.SetCell(0, row, renderer.Cell{Rune: s.commandPrompt, Width: 1, Style: cmdStyle})

	// Draw command buffer
	for i, r := range s.commandBuffer {
		if i+1 < s.width {
			b.SetCell(i+1, row, renderer.Cell{Rune: r, Width: 1, Style: cmdStyle})
		}
	}

	// Position cursor in command line
	b.ShowCursor(s.commandCursor+1, row)
}

// renderMessage renders a status message.
func (s *StatusLine) renderMessage(b backend.Backend, row int) {
	// Choose style based on message type
	var msgStyle renderer.Style
	switch s.messageType {
	case MessageError:
		msgStyle = renderer.DefaultStyle().WithForeground(renderer.ColorRed).Bold()
	case MessageWarning:
		msgStyle = renderer.DefaultStyle().WithForeground(renderer.ColorYellow)
	default:
		msgStyle = renderer.DefaultStyle()
	}

	// Clear the line
	for x := 0; x < s.width; x++ {
		b.SetCell(x, row, renderer.Cell{Rune: ' ', Width: 1, Style: msgStyle})
	}

	// Draw message
	for i, r := range s.message {
		if i < s.width {
			b.SetCell(i, row, renderer.Cell{Rune: r, Width: 1, Style: msgStyle})
		}
	}
}

// formatPosition formats the position info for the right side.
func (s *StatusLine) formatPosition() string {
	// Format: "Ln 123, Col 45 | 50%"
	line := s.line
	if line == 0 {
		line = 1
	}
	col := s.col
	if col == 0 {
		col = 1
	}

	// Simple number to string conversion
	result := "Ln " + uintToStr(line) + ", Col " + uintToStr(col)

	if s.totalLines > 0 {
		if s.line == 1 {
			result += " | Top"
		} else if s.line >= s.totalLines {
			result += " | Bot"
		} else {
			result += " | " + intToStr(s.percentScroll) + "%"
		}
	}

	return result
}

// uintToStr converts uint32 to string without fmt.
func uintToStr(n uint32) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// intToStr converts int to string without fmt.
func intToStr(n int) string {
	if n < 0 {
		return "-" + uintToStr(uint32(-n))
	}
	return uintToStr(uint32(n))
}
