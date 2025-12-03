package style

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestLayerString(t *testing.T) {
	tests := []struct {
		layer    Layer
		expected string
	}{
		{LayerBase, "base"},
		{LayerSyntax, "syntax"},
		{LayerDiagnostic, "diagnostic"},
		{LayerSearch, "search"},
		{LayerDiff, "diff"},
		{LayerSelection, "selection"},
		{LayerGhostText, "ghost-text"},
		{LayerCursor, "cursor"},
		{Layer(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.layer.String(); got != tt.expected {
			t.Errorf("%d.String() = %q, want %q", tt.layer, got, tt.expected)
		}
	}
}

func TestNewResolver(t *testing.T) {
	r := NewResolver()

	if r == nil {
		t.Fatal("NewResolver returned nil")
	}

	// All layers should be enabled by default
	for layer := LayerBase; layer < LayerCount; layer++ {
		if !r.IsLayerEnabled(layer) {
			t.Errorf("Layer %s should be enabled by default", layer)
		}
	}
}

func TestResolverSetBaseStyle(t *testing.T) {
	r := NewResolver()
	style := core.NewStyle(core.ColorFromRGB(255, 0, 0))

	r.SetBaseStyle(style)

	// Resolve with no spans should return base style
	result := r.Resolve(0, nil)
	if result.Foreground != style.Foreground {
		t.Error("Base style not applied correctly")
	}
}

func TestResolverSetLayerEnabled(t *testing.T) {
	r := NewResolver()

	r.SetLayerEnabled(LayerSyntax, false)

	if r.IsLayerEnabled(LayerSyntax) {
		t.Error("LayerSyntax should be disabled")
	}

	r.SetLayerEnabled(LayerSyntax, true)

	if !r.IsLayerEnabled(LayerSyntax) {
		t.Error("LayerSyntax should be enabled")
	}

	// Invalid layer should return false
	if r.IsLayerEnabled(LayerCount + 1) {
		t.Error("Invalid layer should return false")
	}
}

func TestResolverResolve(t *testing.T) {
	t.Run("no spans returns base style", func(t *testing.T) {
		r := NewResolver()
		baseStyle := core.NewStyle(core.ColorFromRGB(200, 200, 200))
		r.SetBaseStyle(baseStyle)

		result := r.Resolve(5, nil)

		if result.Foreground != baseStyle.Foreground {
			t.Error("Should return base style with no spans")
		}
	})

	t.Run("single span applies", func(t *testing.T) {
		r := NewResolver()
		spanStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{{
			StartCol: 0,
			EndCol:   10,
			Style:    spanStyle,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		}}

		result := r.Resolve(5, spans)

		if result.Foreground != spanStyle.Foreground {
			t.Error("Span style should be applied")
		}
	})

	t.Run("column outside span range", func(t *testing.T) {
		r := NewResolver()
		baseStyle := core.NewStyle(core.ColorFromRGB(200, 200, 200))
		r.SetBaseStyle(baseStyle)

		spanStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{{
			StartCol: 10,
			EndCol:   20,
			Style:    spanStyle,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		}}

		result := r.Resolve(5, spans)

		if result.Foreground != baseStyle.Foreground {
			t.Error("Column outside span should use base style")
		}
	})

	t.Run("higher priority layer wins", func(t *testing.T) {
		r := NewResolver()
		syntaxStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		selectionStyle := core.NewStyle(core.ColorFromRGB(0, 0, 255))

		spans := []Span{
			{StartCol: 0, EndCol: 10, Style: syntaxStyle, Layer: LayerSyntax, Merge: MergeReplace},
			{StartCol: 0, EndCol: 10, Style: selectionStyle, Layer: LayerSelection, Merge: MergeReplace},
		}

		result := r.Resolve(5, spans)

		// Selection has higher priority than syntax
		if result.Foreground != selectionStyle.Foreground {
			t.Error("Higher priority layer should win")
		}
	})

	t.Run("disabled layer skipped", func(t *testing.T) {
		r := NewResolver()
		r.SetLayerEnabled(LayerSyntax, false)

		baseStyle := core.NewStyle(core.ColorFromRGB(200, 200, 200))
		r.SetBaseStyle(baseStyle)

		syntaxStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{{
			StartCol: 0,
			EndCol:   10,
			Style:    syntaxStyle,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		}}

		result := r.Resolve(5, spans)

		if result.Foreground != baseStyle.Foreground {
			t.Error("Disabled layer should be skipped")
		}
	})
}

func TestResolverMergeModes(t *testing.T) {
	t.Run("MergeReplace", func(t *testing.T) {
		r := NewResolver()
		baseStyle := core.NewStyle(core.ColorFromRGB(100, 100, 100)).
			WithBackground(core.ColorFromRGB(50, 50, 50)).Bold()
		r.SetBaseStyle(baseStyle)

		overlayStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{{
			StartCol: 0,
			EndCol:   10,
			Style:    overlayStyle,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		}}

		result := r.Resolve(5, spans)

		if result.Foreground != overlayStyle.Foreground {
			t.Error("MergeReplace should completely replace style")
		}
		// Background should be from overlay (which is default)
		if result.Background == baseStyle.Background {
			t.Error("MergeReplace should replace background too")
		}
	})

	t.Run("MergeOverlay", func(t *testing.T) {
		r := NewResolver()
		baseStyle := core.NewStyle(core.ColorFromRGB(100, 100, 100)).
			WithBackground(core.ColorFromRGB(50, 50, 50))
		r.SetBaseStyle(baseStyle)

		overlayStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{{
			StartCol: 0,
			EndCol:   10,
			Style:    overlayStyle,
			Layer:    LayerSyntax,
			Merge:    MergeOverlay,
		}}

		result := r.Resolve(5, spans)

		if result.Foreground != overlayStyle.Foreground {
			t.Error("MergeOverlay should apply overlay foreground")
		}
		if result.Background != baseStyle.Background {
			t.Error("MergeOverlay should preserve base background when overlay bg is default")
		}
	})

	t.Run("MergeAttributes", func(t *testing.T) {
		r := NewResolver()
		baseStyle := core.NewStyle(core.ColorFromRGB(100, 100, 100)).Bold()
		r.SetBaseStyle(baseStyle)

		overlayStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0)).Italic()
		spans := []Span{{
			StartCol: 0,
			EndCol:   10,
			Style:    overlayStyle,
			Layer:    LayerSyntax,
			Merge:    MergeAttributes,
		}}

		result := r.Resolve(5, spans)

		// Should preserve base foreground
		if result.Foreground != baseStyle.Foreground {
			t.Error("MergeAttributes should preserve base foreground")
		}
		// Should have both bold and italic
		if !result.Attributes.Has(core.AttrBold) {
			t.Error("MergeAttributes should preserve bold")
		}
		if !result.Attributes.Has(core.AttrItalic) {
			t.Error("MergeAttributes should add italic")
		}
	})

	t.Run("MergeForeground", func(t *testing.T) {
		r := NewResolver()
		baseStyle := core.NewStyle(core.ColorFromRGB(100, 100, 100)).
			WithBackground(core.ColorFromRGB(50, 50, 50))
		r.SetBaseStyle(baseStyle)

		overlayStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0)).
			WithBackground(core.ColorFromRGB(0, 255, 0))
		spans := []Span{{
			StartCol: 0,
			EndCol:   10,
			Style:    overlayStyle,
			Layer:    LayerSyntax,
			Merge:    MergeForeground,
		}}

		result := r.Resolve(5, spans)

		if result.Foreground != overlayStyle.Foreground {
			t.Error("MergeForeground should apply overlay foreground")
		}
		if result.Background != baseStyle.Background {
			t.Error("MergeForeground should preserve base background")
		}
	})

	t.Run("MergeBackground", func(t *testing.T) {
		r := NewResolver()
		baseStyle := core.NewStyle(core.ColorFromRGB(100, 100, 100)).
			WithBackground(core.ColorFromRGB(50, 50, 50))
		r.SetBaseStyle(baseStyle)

		overlayStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0)).
			WithBackground(core.ColorFromRGB(0, 255, 0))
		spans := []Span{{
			StartCol: 0,
			EndCol:   10,
			Style:    overlayStyle,
			Layer:    LayerSyntax,
			Merge:    MergeBackground,
		}}

		result := r.Resolve(5, spans)

		if result.Foreground != baseStyle.Foreground {
			t.Error("MergeBackground should preserve base foreground")
		}
		if result.Background != overlayStyle.Background {
			t.Error("MergeBackground should apply overlay background")
		}
	})
}

func TestResolverResolveCell(t *testing.T) {
	r := NewResolver()
	spanStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
	spans := []Span{{
		StartCol: 0,
		EndCol:   10,
		Style:    spanStyle,
		Layer:    LayerSyntax,
		Merge:    MergeReplace,
	}}

	cell := core.Cell{Rune: 'A', Width: 1}
	result := r.ResolveCell(cell, 5, spans)

	if result.Rune != 'A' {
		t.Error("ResolveCell should preserve rune")
	}
	if result.Style.Foreground != spanStyle.Foreground {
		t.Error("ResolveCell should apply style")
	}
}

func TestResolverResolveLine(t *testing.T) {
	t.Run("no spans returns copy", func(t *testing.T) {
		r := NewResolver()
		cells := []core.Cell{
			{Rune: 'A', Width: 1},
			{Rune: 'B', Width: 1},
		}

		result := r.ResolveLine(cells, nil)

		// Verify it's a copy by checking modification
		if len(result) != len(cells) {
			t.Error("Should return same length")
		}
	})

	t.Run("applies spans to each cell", func(t *testing.T) {
		r := NewResolver()
		spanStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{{
			StartCol: 1,
			EndCol:   3,
			Style:    spanStyle,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		}}

		cells := make([]core.Cell, 5)
		for i := range cells {
			cells[i] = core.Cell{Rune: rune('A' + i), Width: 1}
		}

		result := r.ResolveLine(cells, spans)

		// Cell 0 should not be styled
		if result[0].Style.Foreground == spanStyle.Foreground {
			t.Error("Cell 0 should not be styled")
		}
		// Cells 1-2 should be styled
		if result[1].Style.Foreground != spanStyle.Foreground {
			t.Error("Cell 1 should be styled")
		}
		if result[2].Style.Foreground != spanStyle.Foreground {
			t.Error("Cell 2 should be styled")
		}
		// Cell 3 should not be styled (EndCol is exclusive)
		if result[3].Style.Foreground == spanStyle.Foreground {
			t.Error("Cell 3 should not be styled")
		}
	})
}

func TestSpanBuilder(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		b := NewSpanBuilder()
		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))

		b.Add(0, 10, style, LayerSyntax)

		spans := b.Build()
		if len(spans) != 1 {
			t.Fatalf("Expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 0 || spans[0].EndCol != 10 {
			t.Error("Span columns incorrect")
		}
		if spans[0].Layer != LayerSyntax {
			t.Error("Span layer incorrect")
		}
		if spans[0].Merge != MergeOverlay {
			t.Error("Default merge mode should be MergeOverlay")
		}
	})

	t.Run("AddWithMerge", func(t *testing.T) {
		b := NewSpanBuilder()
		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))

		b.AddWithMerge(0, 10, style, LayerSyntax, MergeReplace)

		spans := b.Build()
		if spans[0].Merge != MergeReplace {
			t.Error("Merge mode should be MergeReplace")
		}
	})

	t.Run("convenience methods", func(t *testing.T) {
		b := NewSpanBuilder()
		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))

		b.AddSyntax(0, 10, style).
			AddSelection(10, 20, style).
			AddDiagnostic(20, 30, style).
			AddSearch(30, 40, style).
			AddDiff(40, 50, style).
			AddGhostText(50, 60, style)

		spans := b.Build()
		if len(spans) != 6 {
			t.Fatalf("Expected 6 spans, got %d", len(spans))
		}
		if spans[0].Layer != LayerSyntax {
			t.Error("First span should be syntax layer")
		}
		if spans[1].Layer != LayerSelection {
			t.Error("Second span should be selection layer")
		}
	})

	t.Run("chaining", func(t *testing.T) {
		b := NewSpanBuilder()
		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))

		result := b.Add(0, 10, style, LayerSyntax)

		if result != b {
			t.Error("Add should return builder for chaining")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		b := NewSpanBuilder()
		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))

		b.Add(0, 10, style, LayerSyntax)
		b.Clear()

		spans := b.Build()
		if len(spans) != 0 {
			t.Error("Clear should remove all spans")
		}
	})
}

func TestLineResolver(t *testing.T) {
	t.Run("basic usage", func(t *testing.T) {
		r := NewResolver()
		lr := NewLineResolver(r, 5)

		if lr.Line() != 5 {
			t.Errorf("Line() = %d, want 5", lr.Line())
		}
	})

	t.Run("AddSpan and resolve", func(t *testing.T) {
		r := NewResolver()
		lr := NewLineResolver(r, 5)

		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		lr.AddSpan(Span{
			StartCol: 0,
			EndCol:   10,
			Style:    style,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		})

		result := lr.Resolve(5)
		if result.Foreground != style.Foreground {
			t.Error("Style should be applied")
		}
	})

	t.Run("AddSpans", func(t *testing.T) {
		r := NewResolver()
		lr := NewLineResolver(r, 5)

		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{
			{StartCol: 0, EndCol: 10, Style: style, Layer: LayerSyntax, Merge: MergeReplace},
			{StartCol: 10, EndCol: 20, Style: style, Layer: LayerSyntax, Merge: MergeReplace},
		}
		lr.AddSpans(spans)

		result := lr.Resolve(15)
		if result.Foreground != style.Foreground {
			t.Error("Second span should be applied")
		}
	})

	t.Run("ResolveCell", func(t *testing.T) {
		r := NewResolver()
		lr := NewLineResolver(r, 5)

		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		lr.AddSpan(Span{
			StartCol: 0,
			EndCol:   10,
			Style:    style,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		})

		cell := core.Cell{Rune: 'X', Width: 1}
		result := lr.ResolveCell(cell, 5)

		if result.Rune != 'X' {
			t.Error("Should preserve rune")
		}
		if result.Style.Foreground != style.Foreground {
			t.Error("Should apply style")
		}
	})

	t.Run("ResolveCells", func(t *testing.T) {
		r := NewResolver()
		lr := NewLineResolver(r, 5)

		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		lr.AddSpan(Span{
			StartCol: 0,
			EndCol:   5,
			Style:    style,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		})

		cells := make([]core.Cell, 10)
		for i := range cells {
			cells[i] = core.Cell{Rune: rune('A' + i), Width: 1}
		}

		result := lr.ResolveCells(cells)

		if result[2].Style.Foreground != style.Foreground {
			t.Error("Cell in span should be styled")
		}
		if result[7].Style.Foreground == style.Foreground {
			t.Error("Cell outside span should not be styled")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		r := NewResolver()
		lr := NewLineResolver(r, 5)

		style := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		lr.AddSpan(Span{
			StartCol: 0,
			EndCol:   10,
			Style:    style,
			Layer:    LayerSyntax,
			Merge:    MergeReplace,
		})

		lr.Clear()

		result := lr.Resolve(5)
		if result.Foreground == style.Foreground {
			t.Error("Clear should remove spans")
		}
	})
}

func TestDefaultStyles(t *testing.T) {
	ds := NewDefaultStyles()

	// Just verify they're created without panic
	_ = ds.Selection
	_ = ds.SearchMatch
	_ = ds.CurrentMatch
	_ = ds.Error
	_ = ds.Warning
	_ = ds.Info
	_ = ds.Hint
	_ = ds.DiffAdd
	_ = ds.DiffDelete
	_ = ds.DiffModify
	_ = ds.GhostText
}
