package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Renderer event topics.
const (
	// TopicRendererFrameRendered is published after a frame is rendered.
	TopicRendererFrameRendered topic.Topic = "renderer.frame.rendered"

	// TopicRendererRedrawNeeded is published when display needs update.
	TopicRendererRedrawNeeded topic.Topic = "renderer.redraw.needed"

	// TopicRendererResizeHandled is published when window is resized.
	TopicRendererResizeHandled topic.Topic = "renderer.resize.handled"

	// TopicRendererScrollChanged is published when viewport scrolls.
	TopicRendererScrollChanged topic.Topic = "renderer.scroll.changed"

	// TopicRendererHighlightInvalidated is published when syntax highlighting is stale.
	TopicRendererHighlightInvalidated topic.Topic = "renderer.highlight.invalidated"

	// TopicRendererCursorBlink is published on cursor blink state change.
	TopicRendererCursorBlink topic.Topic = "renderer.cursor.blink"

	// TopicRendererThemeChanged is published when the color theme changes.
	TopicRendererThemeChanged topic.Topic = "renderer.theme.changed"

	// TopicRendererFontChanged is published when font settings change.
	TopicRendererFontChanged topic.Topic = "renderer.font.changed"

	// TopicRendererViewportChanged is published when visible content changes.
	TopicRendererViewportChanged topic.Topic = "renderer.viewport.changed"

	// TopicRendererSelectionRendered is published when selection is rendered.
	TopicRendererSelectionRendered topic.Topic = "renderer.selection.rendered"

	// TopicRendererDiagnosticsRendered is published when diagnostics are rendered.
	TopicRendererDiagnosticsRendered topic.Topic = "renderer.diagnostics.rendered"

	// TopicRendererGutterUpdated is published when gutter content changes.
	TopicRendererGutterUpdated topic.Topic = "renderer.gutter.updated"

	// TopicRendererStatusLineUpdated is published when status line changes.
	TopicRendererStatusLineUpdated topic.Topic = "renderer.statusline.updated"

	// TopicRendererPanelToggled is published when a panel is shown/hidden.
	TopicRendererPanelToggled topic.Topic = "renderer.panel.toggled"
)

// LineRange represents a range of lines.
type LineRange struct {
	// Start is the first line (0-based).
	Start int

	// End is the last line (exclusive).
	End int
}

// RendererFrameRendered is published after a frame is rendered.
type RendererFrameRendered struct {
	// FrameCount is the total frame count since start.
	FrameCount uint64

	// FPS is the current frames per second.
	FPS float64

	// DeltaMs is the time since last frame in milliseconds.
	DeltaMs float64

	// RenderTimeUs is the render time in microseconds.
	RenderTimeUs int64

	// LinesRendered is the number of lines rendered.
	LinesRendered int

	// DirtyRegions is the number of dirty regions updated.
	DirtyRegions int

	// Timestamp is when the frame was rendered.
	Timestamp time.Time
}

// RendererRedrawNeeded is published when display needs update.
type RendererRedrawNeeded struct {
	// BufferID is the buffer that needs redraw.
	BufferID string

	// FullRedraw indicates if a full redraw is needed.
	FullRedraw bool

	// LineRanges are the specific line ranges to redraw.
	LineRanges []LineRange

	// Reason describes why redraw is needed.
	Reason string

	// Priority indicates redraw priority.
	Priority int
}

// RendererResizeHandled is published when window is resized.
type RendererResizeHandled struct {
	// OldWidth was the previous width in characters.
	OldWidth int

	// OldHeight was the previous height in characters.
	OldHeight int

	// Width is the new width in characters.
	Width int

	// Height is the new height in characters.
	Height int

	// GutterWidth is the current gutter width.
	GutterWidth int

	// PixelWidth is the window width in pixels, if applicable.
	PixelWidth int

	// PixelHeight is the window height in pixels, if applicable.
	PixelHeight int
}

// RendererScrollChanged is published when viewport scrolls.
type RendererScrollChanged struct {
	// BufferID is the buffer that scrolled.
	BufferID string

	// OldTopLine was the previous top line.
	OldTopLine int

	// NewTopLine is the new top line.
	NewTopLine int

	// OldLeftColumn was the previous left column.
	OldLeftColumn int

	// NewLeftColumn is the new left column.
	NewLeftColumn int

	// Smooth indicates if this is a smooth scroll animation.
	Smooth bool

	// Trigger describes what caused the scroll.
	Trigger string
}

// RendererHighlightInvalidated is published when syntax highlighting is stale.
type RendererHighlightInvalidated struct {
	// BufferID is the buffer with invalidated highlighting.
	BufferID string

	// LineRange is the range with stale highlighting.
	LineRange LineRange

	// Reason describes why highlighting was invalidated.
	Reason string
}

// RendererCursorBlink is published on cursor blink state change.
type RendererCursorBlink struct {
	// BufferID is the buffer with the cursor.
	BufferID string

	// IsVisible indicates if the cursor is currently visible.
	IsVisible bool

	// BlinkCount is the number of blinks since last edit.
	BlinkCount int
}

// RendererThemeChanged is published when the color theme changes.
type RendererThemeChanged struct {
	// OldTheme was the previous theme name.
	OldTheme string

	// NewTheme is the new theme name.
	NewTheme string

	// IsDark indicates if the new theme is dark.
	IsDark bool

	// Source indicates where the theme came from.
	Source string
}

// RendererFontChanged is published when font settings change.
type RendererFontChanged struct {
	// OldFontFamily was the previous font family.
	OldFontFamily string

	// NewFontFamily is the new font family.
	NewFontFamily string

	// OldFontSize was the previous font size.
	OldFontSize float64

	// NewFontSize is the new font size.
	NewFontSize float64

	// OldLineHeight was the previous line height.
	OldLineHeight float64

	// NewLineHeight is the new line height.
	NewLineHeight float64
}

// RendererViewportChanged is published when visible content changes.
type RendererViewportChanged struct {
	// BufferID is the buffer whose viewport changed.
	BufferID string

	// FirstVisibleLine is the first visible line.
	FirstVisibleLine int

	// LastVisibleLine is the last visible line.
	LastVisibleLine int

	// VisibleLineCount is the number of visible lines.
	VisibleLineCount int

	// FirstVisibleColumn is the first visible column.
	FirstVisibleColumn int

	// VisibleColumnCount is the number of visible columns.
	VisibleColumnCount int
}

// RendererSelectionRendered is published when selection is rendered.
type RendererSelectionRendered struct {
	// BufferID is the buffer with the selection.
	BufferID string

	// SelectionCount is the number of selections rendered.
	SelectionCount int

	// TotalLines is the total number of lines with selection.
	TotalLines int

	// Mode is the selection mode (char, line, block).
	Mode string
}

// RendererDiagnosticsRendered is published when diagnostics are rendered.
type RendererDiagnosticsRendered struct {
	// BufferID is the buffer with diagnostics.
	BufferID string

	// ErrorCount is the number of errors rendered.
	ErrorCount int

	// WarningCount is the number of warnings rendered.
	WarningCount int

	// InfoCount is the number of info diagnostics rendered.
	InfoCount int

	// HintCount is the number of hints rendered.
	HintCount int

	// VisibleCount is the number of diagnostics in the viewport.
	VisibleCount int
}

// RendererGutterUpdated is published when gutter content changes.
type RendererGutterUpdated struct {
	// BufferID is the buffer whose gutter was updated.
	BufferID string

	// Components lists the gutter components that changed.
	Components []string

	// Width is the new gutter width.
	Width int

	// LineRange is the range of lines updated.
	LineRange LineRange
}

// RendererStatusLineUpdated is published when status line changes.
type RendererStatusLineUpdated struct {
	// Mode is the current editor mode.
	Mode string

	// FilePath is the current file path.
	FilePath string

	// FileType is the file type/language.
	FileType string

	// Encoding is the file encoding.
	Encoding string

	// LineEnding is the line ending style.
	LineEnding string

	// CursorLine is the current line number.
	CursorLine int

	// CursorColumn is the current column number.
	CursorColumn int

	// SelectionInfo describes any active selection.
	SelectionInfo string

	// DiagnosticSummary summarizes diagnostics.
	DiagnosticSummary string

	// GitBranch is the current git branch.
	GitBranch string

	// IsDirty indicates if the buffer is dirty.
	IsDirty bool

	// IsReadOnly indicates if the buffer is read-only.
	IsReadOnly bool
}

// RendererPanelToggled is published when a panel is shown/hidden.
type RendererPanelToggled struct {
	// PanelID identifies the panel.
	PanelID string

	// PanelName is the panel name.
	PanelName string

	// IsVisible indicates if the panel is now visible.
	IsVisible bool

	// Position is the panel position (top, bottom, left, right).
	Position string

	// Size is the panel size.
	Size int

	// IsFocused indicates if the panel has focus.
	IsFocused bool
}
