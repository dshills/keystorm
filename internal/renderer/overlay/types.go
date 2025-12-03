// Package overlay provides AI overlay rendering for ghost text, diff previews,
// and inline suggestions in the editor.
package overlay

import (
	"github.com/dshills/keystorm/internal/renderer/core"
)

// Type represents the kind of overlay.
type Type uint8

const (
	// TypeGhostText is for AI completion suggestions shown as dim text.
	TypeGhostText Type = iota

	// TypeDiffAdd is for inline additions (green background).
	TypeDiffAdd

	// TypeDiffDelete is for inline deletions (red background, strikethrough).
	TypeDiffDelete

	// TypeDiffModify is for modifications (yellow background).
	TypeDiffModify

	// TypeInlineHint is for inline hints/annotations.
	TypeInlineHint

	// TypeDiagnostic is for diagnostic overlays (errors, warnings).
	TypeDiagnostic
)

// String returns the string representation of the overlay type.
func (t Type) String() string {
	switch t {
	case TypeGhostText:
		return "ghost-text"
	case TypeDiffAdd:
		return "diff-add"
	case TypeDiffDelete:
		return "diff-delete"
	case TypeDiffModify:
		return "diff-modify"
	case TypeInlineHint:
		return "inline-hint"
	case TypeDiagnostic:
		return "diagnostic"
	default:
		return "unknown"
	}
}

// Priority represents the rendering priority of overlays.
// Higher priority overlays are rendered on top.
type Priority uint8

const (
	PriorityLow      Priority = 50
	PriorityNormal   Priority = 100
	PriorityHigh     Priority = 150
	PriorityCritical Priority = 200
)

// Position represents a position in the buffer.
type Position struct {
	Line uint32
	Col  uint32
}

// Range represents a range in the buffer.
type Range struct {
	Start Position
	End   Position
}

// Contains returns true if the position is within the range.
func (r Range) Contains(line, col uint32) bool {
	if line < r.Start.Line || line > r.End.Line {
		return false
	}
	if line == r.Start.Line && col < r.Start.Col {
		return false
	}
	if line == r.End.Line && col >= r.End.Col {
		return false
	}
	return true
}

// ContainsLine returns true if the line is within the range.
func (r Range) ContainsLine(line uint32) bool {
	return line >= r.Start.Line && line <= r.End.Line
}

// IsEmpty returns true if the range is empty.
func (r Range) IsEmpty() bool {
	return r.Start.Line == r.End.Line && r.Start.Col == r.End.Col
}

// Overlay represents a visual overlay on the editor content.
type Overlay interface {
	// ID returns the unique identifier for this overlay.
	ID() string

	// Type returns the type of overlay.
	Type() Type

	// Priority returns the rendering priority.
	Priority() Priority

	// Range returns the affected buffer range.
	Range() Range

	// IsVisible returns true if the overlay should be rendered.
	IsVisible() bool

	// SpansForLine returns the overlay spans for a specific line.
	// Returns nil if this overlay doesn't affect the line.
	SpansForLine(line uint32) []Span
}

// Span represents a styled span of overlay content on a single line.
type Span struct {
	// StartCol is the starting column (0-indexed).
	StartCol uint32

	// EndCol is the ending column (exclusive).
	// If 0, the span extends to the end of inserted text.
	EndCol uint32

	// Text is the overlay text to display (for ghost text and insertions).
	// If empty, the span only applies styling to existing content.
	Text string

	// Style is the visual style for this span.
	Style core.Style

	// ReplaceContent indicates whether this span replaces existing content.
	// If false, the overlay is rendered on top of/alongside existing content.
	ReplaceContent bool

	// AfterContent indicates this span should appear after the line content.
	// Used for ghost text completions that appear at end of line.
	AfterContent bool
}

// BaseOverlay provides common functionality for overlay implementations.
type BaseOverlay struct {
	id       string
	typ      Type
	priority Priority
	rng      Range
	visible  bool
}

// NewBaseOverlay creates a new base overlay.
func NewBaseOverlay(id string, typ Type, priority Priority, rng Range) *BaseOverlay {
	return &BaseOverlay{
		id:       id,
		typ:      typ,
		priority: priority,
		rng:      rng,
		visible:  true,
	}
}

// ID returns the overlay ID.
func (o *BaseOverlay) ID() string {
	return o.id
}

// Type returns the overlay type.
func (o *BaseOverlay) Type() Type {
	return o.typ
}

// Priority returns the overlay priority.
func (o *BaseOverlay) Priority() Priority {
	return o.priority
}

// Range returns the overlay range.
func (o *BaseOverlay) Range() Range {
	return o.rng
}

// IsVisible returns true if the overlay is visible.
func (o *BaseOverlay) IsVisible() bool {
	return o.visible
}

// SetVisible sets the overlay visibility.
func (o *BaseOverlay) SetVisible(visible bool) {
	o.visible = visible
}

// SetRange updates the overlay range.
func (o *BaseOverlay) SetRange(rng Range) {
	o.rng = rng
}

// Config holds configuration for overlay rendering.
type Config struct {
	// GhostTextStyle is the style for AI completion ghost text.
	GhostTextStyle core.Style

	// DiffAddStyle is the style for diff additions.
	DiffAddStyle core.Style

	// DiffDeleteStyle is the style for diff deletions.
	DiffDeleteStyle core.Style

	// DiffModifyStyle is the style for diff modifications.
	DiffModifyStyle core.Style

	// HintStyle is the style for inline hints.
	HintStyle core.Style

	// ErrorStyle is the style for error diagnostics.
	ErrorStyle core.Style

	// WarningStyle is the style for warning diagnostics.
	WarningStyle core.Style

	// ShowGhostText enables ghost text rendering.
	ShowGhostText bool

	// ShowDiffPreview enables inline diff preview.
	ShowDiffPreview bool

	// ShowDiagnostics enables diagnostic overlays.
	ShowDiagnostics bool

	// AnimateGhostText enables fade-in animation for ghost text.
	AnimateGhostText bool

	// GhostTextDelay is the delay before showing ghost text (ms).
	GhostTextDelay int
}

// DefaultConfig returns the default overlay configuration.
func DefaultConfig() Config {
	return Config{
		GhostTextStyle: core.NewStyle(core.ColorFromRGB(128, 128, 128)).Italic(),
		DiffAddStyle: core.NewStyle(core.ColorFromRGB(80, 200, 80)).
			WithBackground(core.ColorFromRGB(30, 60, 30)),
		DiffDeleteStyle: core.NewStyle(core.ColorFromRGB(200, 80, 80)).
			WithBackground(core.ColorFromRGB(60, 30, 30)).
			Strikethrough(),
		DiffModifyStyle: core.NewStyle(core.ColorFromRGB(200, 200, 80)).
			WithBackground(core.ColorFromRGB(60, 60, 30)),
		HintStyle: core.NewStyle(core.ColorFromRGB(100, 149, 237)).Italic(), // Cornflower blue
		ErrorStyle: core.NewStyle(core.ColorFromRGB(255, 80, 80)).
			WithBackground(core.ColorFromRGB(60, 20, 20)),
		WarningStyle: core.NewStyle(core.ColorFromRGB(255, 200, 80)).
			WithBackground(core.ColorFromRGB(60, 50, 20)),
		ShowGhostText:    true,
		ShowDiffPreview:  true,
		ShowDiagnostics:  true,
		AnimateGhostText: true,
		GhostTextDelay:   300,
	}
}

// Cell represents a single cell in the overlay layer.
type Cell struct {
	// Rune is the character to display.
	Rune rune

	// Width is the display width of the cell.
	Width int

	// Style is the cell style.
	Style core.Style

	// IsOverlay indicates this is overlay content (not base content).
	IsOverlay bool

	// OverlayType is the type of overlay if IsOverlay is true.
	OverlayType Type
}

// EmptyCell returns an empty overlay cell.
func EmptyCell() Cell {
	return Cell{
		Rune:  ' ',
		Width: 1,
		Style: core.DefaultStyle(),
	}
}

// MergeStyles merges an overlay style onto a base style.
// The overlay style takes precedence for foreground and attributes.
// Background is blended if the overlay has transparency.
func MergeStyles(base, overlay core.Style) core.Style {
	result := base

	// Overlay foreground always takes precedence if set
	if overlay.Foreground != core.ColorDefault {
		result.Foreground = overlay.Foreground
	}

	// Overlay background takes precedence if set
	if overlay.Background != core.ColorDefault {
		result.Background = overlay.Background
	}

	// Merge attributes (add overlay attributes to base)
	result.Attributes |= overlay.Attributes

	return result
}
