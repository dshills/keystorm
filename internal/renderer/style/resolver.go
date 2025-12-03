// Package style provides style resolution for combining styles from multiple sources.
// The StyleResolver handles priority-based style merging from syntax highlighting,
// selections, overlays, diagnostics, and other visual layers.
package style

import (
	"github.com/dshills/keystorm/internal/renderer/core"
)

// Layer represents a style layer with priority.
type Layer uint8

const (
	// LayerBase is the base/default style layer.
	LayerBase Layer = iota

	// LayerSyntax is the syntax highlighting layer.
	LayerSyntax

	// LayerDiagnostic is the diagnostic (errors, warnings) layer.
	LayerDiagnostic

	// LayerSearch is the search highlight layer.
	LayerSearch

	// LayerDiff is the diff preview layer.
	LayerDiff

	// LayerSelection is the selection highlight layer.
	LayerSelection

	// LayerGhostText is the AI ghost text layer.
	LayerGhostText

	// LayerCursor is the cursor highlight layer (highest priority).
	LayerCursor

	// LayerCount is the number of layers.
	LayerCount
)

// String returns the string representation of the layer.
func (l Layer) String() string {
	switch l {
	case LayerBase:
		return "base"
	case LayerSyntax:
		return "syntax"
	case LayerDiagnostic:
		return "diagnostic"
	case LayerSearch:
		return "search"
	case LayerDiff:
		return "diff"
	case LayerSelection:
		return "selection"
	case LayerGhostText:
		return "ghost-text"
	case LayerCursor:
		return "cursor"
	default:
		return "unknown"
	}
}

// Span represents a styled span at a specific layer.
type Span struct {
	// StartCol is the starting column (inclusive).
	StartCol uint32

	// EndCol is the ending column (exclusive).
	EndCol uint32

	// Style is the style to apply.
	Style core.Style

	// Layer is the priority layer.
	Layer Layer

	// Merge indicates how to merge with lower layers.
	Merge MergeMode
}

// MergeMode determines how styles are merged.
type MergeMode uint8

const (
	// MergeReplace replaces all lower layer styles.
	MergeReplace MergeMode = iota

	// MergeOverlay overlays onto lower layers (preserves base background).
	MergeOverlay

	// MergeAttributes only adds attributes, preserves colors.
	MergeAttributes

	// MergeForeground only changes foreground color.
	MergeForeground

	// MergeBackground only changes background color.
	MergeBackground
)

// Resolver resolves styles by combining multiple layers.
type Resolver struct {
	// baseStyle is the default style when no layers apply.
	baseStyle core.Style

	// layerEnabled tracks which layers are enabled.
	layerEnabled [LayerCount]bool
}

// NewResolver creates a new style resolver.
func NewResolver() *Resolver {
	r := &Resolver{
		baseStyle: core.DefaultStyle(),
	}

	// Enable all layers by default
	for i := 0; i < int(LayerCount); i++ {
		r.layerEnabled[i] = true
	}

	return r
}

// SetBaseStyle sets the base style.
func (r *Resolver) SetBaseStyle(style core.Style) {
	r.baseStyle = style
}

// SetLayerEnabled enables or disables a layer.
func (r *Resolver) SetLayerEnabled(layer Layer, enabled bool) {
	if layer < LayerCount {
		r.layerEnabled[layer] = enabled
	}
}

// IsLayerEnabled returns true if a layer is enabled.
func (r *Resolver) IsLayerEnabled(layer Layer) bool {
	if layer >= LayerCount {
		return false
	}
	return r.layerEnabled[layer]
}

// Resolve combines styles from multiple spans at a specific column.
func (r *Resolver) Resolve(col uint32, spans []Span) core.Style {
	result := r.baseStyle

	// Process spans in layer order (lower layers first)
	for layer := LayerBase; layer < LayerCount; layer++ {
		if !r.layerEnabled[layer] {
			continue
		}

		// Find spans for this layer that cover this column
		for _, span := range spans {
			if span.Layer != layer {
				continue
			}
			if col < span.StartCol || col >= span.EndCol {
				continue
			}

			result = r.mergeStyle(result, span.Style, span.Merge)
		}
	}

	return result
}

// ResolveCell resolves the style for a cell and returns an updated cell.
func (r *Resolver) ResolveCell(cell core.Cell, col uint32, spans []Span) core.Cell {
	cell.Style = r.Resolve(col, spans)
	return cell
}

// ResolveLine resolves styles for an entire line of cells.
func (r *Resolver) ResolveLine(cells []core.Cell, spans []Span) []core.Cell {
	if len(spans) == 0 {
		return cells
	}

	// Make a copy to avoid modifying original
	result := make([]core.Cell, len(cells))
	copy(result, cells)

	for i := range result {
		result[i].Style = r.Resolve(uint32(i), spans)
	}

	return result
}

// mergeStyle merges an overlay style onto a base style.
func (r *Resolver) mergeStyle(base, overlay core.Style, mode MergeMode) core.Style {
	switch mode {
	case MergeReplace:
		return overlay

	case MergeOverlay:
		result := base
		if !overlay.Foreground.IsDefault() {
			result.Foreground = overlay.Foreground
		}
		if !overlay.Background.IsDefault() {
			result.Background = overlay.Background
		}
		result.Attributes |= overlay.Attributes
		return result

	case MergeAttributes:
		result := base
		result.Attributes |= overlay.Attributes
		return result

	case MergeForeground:
		result := base
		if !overlay.Foreground.IsDefault() {
			result.Foreground = overlay.Foreground
		}
		return result

	case MergeBackground:
		result := base
		if !overlay.Background.IsDefault() {
			result.Background = overlay.Background
		}
		return result

	default:
		return overlay
	}
}

// SpanBuilder helps build spans for a line.
type SpanBuilder struct {
	spans []Span
}

// NewSpanBuilder creates a new span builder.
func NewSpanBuilder() *SpanBuilder {
	return &SpanBuilder{
		spans: make([]Span, 0, 8),
	}
}

// Add adds a span.
func (b *SpanBuilder) Add(startCol, endCol uint32, style core.Style, layer Layer) *SpanBuilder {
	b.spans = append(b.spans, Span{
		StartCol: startCol,
		EndCol:   endCol,
		Style:    style,
		Layer:    layer,
		Merge:    MergeOverlay,
	})
	return b
}

// AddWithMerge adds a span with a specific merge mode.
func (b *SpanBuilder) AddWithMerge(startCol, endCol uint32, style core.Style, layer Layer, merge MergeMode) *SpanBuilder {
	b.spans = append(b.spans, Span{
		StartCol: startCol,
		EndCol:   endCol,
		Style:    style,
		Layer:    layer,
		Merge:    merge,
	})
	return b
}

// AddSyntax adds a syntax highlighting span.
func (b *SpanBuilder) AddSyntax(startCol, endCol uint32, style core.Style) *SpanBuilder {
	return b.Add(startCol, endCol, style, LayerSyntax)
}

// AddSelection adds a selection span.
func (b *SpanBuilder) AddSelection(startCol, endCol uint32, style core.Style) *SpanBuilder {
	return b.Add(startCol, endCol, style, LayerSelection)
}

// AddDiagnostic adds a diagnostic span.
func (b *SpanBuilder) AddDiagnostic(startCol, endCol uint32, style core.Style) *SpanBuilder {
	return b.Add(startCol, endCol, style, LayerDiagnostic)
}

// AddSearch adds a search highlight span.
func (b *SpanBuilder) AddSearch(startCol, endCol uint32, style core.Style) *SpanBuilder {
	return b.Add(startCol, endCol, style, LayerSearch)
}

// AddDiff adds a diff preview span.
func (b *SpanBuilder) AddDiff(startCol, endCol uint32, style core.Style) *SpanBuilder {
	return b.Add(startCol, endCol, style, LayerDiff)
}

// AddGhostText adds a ghost text span.
func (b *SpanBuilder) AddGhostText(startCol, endCol uint32, style core.Style) *SpanBuilder {
	return b.Add(startCol, endCol, style, LayerGhostText)
}

// Build returns the built spans.
func (b *SpanBuilder) Build() []Span {
	return b.spans
}

// Clear clears all spans.
func (b *SpanBuilder) Clear() {
	b.spans = b.spans[:0]
}

// LineResolver resolves styles for a single line with caching.
type LineResolver struct {
	resolver *Resolver
	spans    []Span
	line     uint32
}

// NewLineResolver creates a line resolver for a specific line.
func NewLineResolver(resolver *Resolver, line uint32) *LineResolver {
	return &LineResolver{
		resolver: resolver,
		spans:    make([]Span, 0, 8),
		line:     line,
	}
}

// AddSpan adds a span for this line.
func (lr *LineResolver) AddSpan(span Span) {
	lr.spans = append(lr.spans, span)
}

// AddSpans adds multiple spans for this line.
func (lr *LineResolver) AddSpans(spans []Span) {
	lr.spans = append(lr.spans, spans...)
}

// Resolve resolves the style at a column.
func (lr *LineResolver) Resolve(col uint32) core.Style {
	return lr.resolver.Resolve(col, lr.spans)
}

// ResolveCell resolves and updates a cell's style.
func (lr *LineResolver) ResolveCell(cell core.Cell, col uint32) core.Cell {
	cell.Style = lr.Resolve(col)
	return cell
}

// ResolveCells resolves styles for a slice of cells.
func (lr *LineResolver) ResolveCells(cells []core.Cell) []core.Cell {
	return lr.resolver.ResolveLine(cells, lr.spans)
}

// Clear clears the spans.
func (lr *LineResolver) Clear() {
	lr.spans = lr.spans[:0]
}

// Line returns the line number.
func (lr *LineResolver) Line() uint32 {
	return lr.line
}

// DefaultStyles returns commonly used style presets.
type DefaultStyles struct {
	// Selection is the default selection style.
	Selection core.Style

	// SearchMatch is the default search match style.
	SearchMatch core.Style

	// CurrentMatch is the current search match style.
	CurrentMatch core.Style

	// Error is the error diagnostic style.
	Error core.Style

	// Warning is the warning diagnostic style.
	Warning core.Style

	// Info is the info diagnostic style.
	Info core.Style

	// Hint is the hint diagnostic style.
	Hint core.Style

	// DiffAdd is the diff addition style.
	DiffAdd core.Style

	// DiffDelete is the diff deletion style.
	DiffDelete core.Style

	// DiffModify is the diff modification style.
	DiffModify core.Style

	// GhostText is the ghost text style.
	GhostText core.Style
}

// NewDefaultStyles creates default style presets.
func NewDefaultStyles() DefaultStyles {
	return DefaultStyles{
		Selection: core.NewStyle(core.ColorDefault).
			WithBackground(core.ColorFromRGB(60, 90, 130)),

		SearchMatch: core.NewStyle(core.ColorDefault).
			WithBackground(core.ColorFromRGB(100, 100, 50)),

		CurrentMatch: core.NewStyle(core.ColorDefault).
			WithBackground(core.ColorFromRGB(150, 120, 50)),

		Error: core.NewStyle(core.ColorFromRGB(255, 80, 80)).
			WithBackground(core.ColorFromRGB(60, 20, 20)),

		Warning: core.NewStyle(core.ColorFromRGB(255, 200, 80)).
			WithBackground(core.ColorFromRGB(60, 50, 20)),

		Info: core.NewStyle(core.ColorFromRGB(80, 180, 255)),

		Hint: core.NewStyle(core.ColorFromRGB(128, 128, 128)).Italic(),

		DiffAdd: core.NewStyle(core.ColorFromRGB(80, 200, 80)).
			WithBackground(core.ColorFromRGB(30, 60, 30)),

		DiffDelete: core.NewStyle(core.ColorFromRGB(200, 80, 80)).
			WithBackground(core.ColorFromRGB(60, 30, 30)).
			Strikethrough(),

		DiffModify: core.NewStyle(core.ColorFromRGB(200, 200, 80)).
			WithBackground(core.ColorFromRGB(60, 60, 30)),

		GhostText: core.NewStyle(core.ColorFromRGB(128, 128, 128)).Italic(),
	}
}
