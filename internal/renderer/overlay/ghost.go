package overlay

import (
	"strings"
	"time"

	"github.com/dshills/keystorm/internal/renderer/core"
)

// GhostText represents AI-generated completion suggestions shown as dimmed text.
// Ghost text appears inline after the cursor position and can span multiple lines.
type GhostText struct {
	*BaseOverlay

	// lines contains the ghost text content, one entry per line.
	lines []string

	// style is the rendering style for ghost text.
	style core.Style

	// showTime is when the ghost text was made visible.
	showTime time.Time

	// fadeInDuration is the fade-in animation duration.
	fadeInDuration time.Duration

	// animationEnabled controls whether fade-in is used.
	animationEnabled bool

	// accepted tracks if this ghost text has been accepted.
	accepted bool

	// partial tracks how much has been partially accepted.
	partialAccepted int
}

// NewGhostText creates a new ghost text overlay.
func NewGhostText(id string, position Position, text string, style core.Style) *GhostText {
	lines := strings.Split(text, "\n")

	// Calculate the range
	endLine := position.Line + uint32(len(lines)) - 1
	endCol := uint32(len(lines[len(lines)-1]))
	if len(lines) == 1 {
		endCol = position.Col + uint32(len(text))
	}

	rng := Range{
		Start: position,
		End:   Position{Line: endLine, Col: endCol},
	}

	return &GhostText{
		BaseOverlay:      NewBaseOverlay(id, TypeGhostText, PriorityNormal, rng),
		lines:            lines,
		style:            style,
		fadeInDuration:   200 * time.Millisecond,
		animationEnabled: true,
	}
}

// NewGhostTextMultiLine creates ghost text from multiple lines.
func NewGhostTextMultiLine(id string, position Position, lines []string, style core.Style) *GhostText {
	if len(lines) == 0 {
		lines = []string{""}
	}

	endLine := position.Line + uint32(len(lines)) - 1
	endCol := uint32(len(lines[len(lines)-1]))

	rng := Range{
		Start: position,
		End:   Position{Line: endLine, Col: endCol},
	}

	return &GhostText{
		BaseOverlay:      NewBaseOverlay(id, TypeGhostText, PriorityNormal, rng),
		lines:            lines,
		style:            style,
		fadeInDuration:   200 * time.Millisecond,
		animationEnabled: true,
	}
}

// Text returns the full ghost text content.
func (g *GhostText) Text() string {
	return strings.Join(g.lines, "\n")
}

// Lines returns the ghost text lines.
func (g *GhostText) Lines() []string {
	return g.lines
}

// LineCount returns the number of lines in the ghost text.
func (g *GhostText) LineCount() int {
	return len(g.lines)
}

// Show makes the ghost text visible and starts the fade-in animation.
func (g *GhostText) Show() {
	g.visible = true
	g.showTime = time.Now()
}

// Hide hides the ghost text.
func (g *GhostText) Hide() {
	g.visible = false
}

// SetAnimationEnabled enables or disables fade-in animation.
func (g *GhostText) SetAnimationEnabled(enabled bool) {
	g.animationEnabled = enabled
}

// SetFadeInDuration sets the fade-in animation duration.
func (g *GhostText) SetFadeInDuration(d time.Duration) {
	g.fadeInDuration = d
}

// Opacity returns the current opacity (0.0-1.0) based on animation.
func (g *GhostText) Opacity() float64 {
	if !g.animationEnabled || g.fadeInDuration == 0 {
		return 1.0
	}

	elapsed := time.Since(g.showTime)
	if elapsed >= g.fadeInDuration {
		return 1.0
	}

	return float64(elapsed) / float64(g.fadeInDuration)
}

// StyleWithOpacity returns the style adjusted for current opacity.
func (g *GhostText) StyleWithOpacity() core.Style {
	opacity := g.Opacity()
	if opacity >= 1.0 {
		return g.style
	}

	// Adjust the foreground color based on opacity
	// For now, we just use the base style at full opacity
	// A more sophisticated implementation would blend colors
	return g.style
}

// Accept marks the ghost text as fully accepted.
func (g *GhostText) Accept() {
	g.accepted = true
	g.visible = false
}

// AcceptPartial accepts part of the ghost text (word by word).
func (g *GhostText) AcceptPartial() string {
	if g.accepted || len(g.lines) == 0 {
		return ""
	}

	firstLine := g.lines[0]
	if g.partialAccepted >= len(firstLine) {
		// Move to next line
		if len(g.lines) > 1 {
			accepted := g.lines[0][g.partialAccepted:]
			g.lines = g.lines[1:]
			g.partialAccepted = 0
			g.rng.Start.Line++
			g.rng.Start.Col = 0
			return accepted + "\n"
		}
		g.Accept()
		return ""
	}

	// Find next word boundary
	remaining := firstLine[g.partialAccepted:]
	wordEnd := findWordEnd(remaining)
	if wordEnd == 0 {
		wordEnd = len(remaining)
	}

	accepted := remaining[:wordEnd]
	g.partialAccepted += wordEnd
	g.rng.Start.Col += uint32(wordEnd)

	return accepted
}

// IsAccepted returns true if the ghost text has been accepted.
func (g *GhostText) IsAccepted() bool {
	return g.accepted
}

// Reject dismisses the ghost text.
func (g *GhostText) Reject() {
	g.visible = false
}

// UpdatePosition updates the ghost text position (e.g., after cursor movement).
func (g *GhostText) UpdatePosition(position Position) {
	// Use signed arithmetic to handle negative deltas correctly
	lineDelta := int64(position.Line) - int64(g.rng.Start.Line)
	colDelta := int64(position.Col) - int64(g.rng.Start.Col)

	g.rng.Start = position

	// Apply line delta to end position
	newEndLine := int64(g.rng.End.Line) + lineDelta
	if newEndLine < 0 {
		newEndLine = 0
	}
	g.rng.End.Line = uint32(newEndLine)

	// Apply column delta only if on the same line
	if lineDelta == 0 {
		newEndCol := int64(g.rng.End.Col) + colDelta
		if newEndCol < 0 {
			newEndCol = 0
		}
		g.rng.End.Col = uint32(newEndCol)
	}
}

// UpdateText updates the ghost text content.
func (g *GhostText) UpdateText(text string) {
	g.lines = strings.Split(text, "\n")
	g.partialAccepted = 0

	// Update range
	endLine := g.rng.Start.Line + uint32(len(g.lines)) - 1
	endCol := uint32(len(g.lines[len(g.lines)-1]))
	if len(g.lines) == 1 {
		endCol = g.rng.Start.Col + uint32(len(text))
	}

	g.rng.End = Position{Line: endLine, Col: endCol}
}

// SpansForLine returns the overlay spans for a specific line.
func (g *GhostText) SpansForLine(line uint32) []Span {
	if !g.visible || g.accepted {
		return nil
	}

	// Check if line is in range
	lineIdx := int(line - g.rng.Start.Line)
	if lineIdx < 0 || lineIdx >= len(g.lines) {
		return nil
	}

	text := g.lines[lineIdx]
	if lineIdx == 0 && g.partialAccepted > 0 {
		if g.partialAccepted >= len(text) {
			return nil
		}
		text = text[g.partialAccepted:]
	}

	if text == "" {
		return nil
	}

	span := Span{
		StartCol:     g.rng.Start.Col,
		Text:         text,
		Style:        g.StyleWithOpacity(),
		AfterContent: lineIdx == 0, // First line appears after cursor
	}

	// For subsequent lines, start at column 0
	if lineIdx > 0 {
		span.StartCol = 0
		span.AfterContent = false
	}

	return []Span{span}
}

// findWordEnd finds the end of the next word in the string.
func findWordEnd(s string) int {
	if len(s) == 0 {
		return 0
	}

	// Skip leading whitespace
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}

	if i > 0 {
		return i // Return whitespace as a "word"
	}

	// Find end of word
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		i++
	}

	return i
}

// GhostTextProvider provides ghost text for the renderer.
type GhostTextProvider interface {
	// GhostTextAt returns the ghost text at the given position, if any.
	GhostTextAt(line, col uint32) *GhostText

	// HasGhostText returns true if there's ghost text at the position.
	HasGhostText(line, col uint32) bool
}
