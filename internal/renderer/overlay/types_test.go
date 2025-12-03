package overlay

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestTypeString(t *testing.T) {
	tests := []struct {
		typ  Type
		want string
	}{
		{TypeGhostText, "ghost-text"},
		{TypeDiffAdd, "diff-add"},
		{TypeDiffDelete, "diff-delete"},
		{TypeDiffModify, "diff-modify"},
		{TypeInlineHint, "inline-hint"},
		{TypeDiagnostic, "diagnostic"},
		{Type(255), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("Type.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRangeContains(t *testing.T) {
	rng := Range{
		Start: Position{Line: 5, Col: 10},
		End:   Position{Line: 10, Col: 20},
	}

	tests := []struct {
		name string
		line uint32
		col  uint32
		want bool
	}{
		{"before start line", 4, 15, false},
		{"after end line", 11, 15, false},
		{"on start line before col", 5, 5, false},
		{"on start line at col", 5, 10, true},
		{"on start line after col", 5, 15, true},
		{"middle line", 7, 0, true},
		{"on end line before col", 10, 15, true},
		{"on end line at col", 10, 20, false},
		{"on end line after col", 10, 25, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rng.Contains(tt.line, tt.col); got != tt.want {
				t.Errorf("Range.Contains(%d, %d) = %v, want %v", tt.line, tt.col, got, tt.want)
			}
		})
	}
}

func TestRangeContainsLine(t *testing.T) {
	rng := Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 10, Col: 0},
	}

	tests := []struct {
		line uint32
		want bool
	}{
		{4, false},
		{5, true},
		{7, true},
		{10, true},
		{11, false},
	}

	for _, tt := range tests {
		if got := rng.ContainsLine(tt.line); got != tt.want {
			t.Errorf("Range.ContainsLine(%d) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestRangeIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		rng   Range
		empty bool
	}{
		{
			"empty range",
			Range{Start: Position{Line: 5, Col: 10}, End: Position{Line: 5, Col: 10}},
			true,
		},
		{
			"same line different col",
			Range{Start: Position{Line: 5, Col: 10}, End: Position{Line: 5, Col: 20}},
			false,
		},
		{
			"different lines",
			Range{Start: Position{Line: 5, Col: 10}, End: Position{Line: 6, Col: 0}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rng.IsEmpty(); got != tt.empty {
				t.Errorf("Range.IsEmpty() = %v, want %v", got, tt.empty)
			}
		})
	}
}

func TestNewBaseOverlay(t *testing.T) {
	rng := Range{
		Start: Position{Line: 10, Col: 5},
		End:   Position{Line: 10, Col: 20},
	}

	o := NewBaseOverlay("test-id", TypeGhostText, PriorityHigh, rng)

	if o.ID() != "test-id" {
		t.Errorf("ID() = %q, want %q", o.ID(), "test-id")
	}
	if o.Type() != TypeGhostText {
		t.Errorf("Type() = %v, want %v", o.Type(), TypeGhostText)
	}
	if o.Priority() != PriorityHigh {
		t.Errorf("Priority() = %v, want %v", o.Priority(), PriorityHigh)
	}
	if o.Range() != rng {
		t.Errorf("Range() = %v, want %v", o.Range(), rng)
	}
	if !o.IsVisible() {
		t.Error("IsVisible() = false, want true (default)")
	}
}

func TestBaseOverlayVisibility(t *testing.T) {
	o := NewBaseOverlay("test", TypeGhostText, PriorityNormal, Range{})

	if !o.IsVisible() {
		t.Error("Should be visible by default")
	}

	o.SetVisible(false)
	if o.IsVisible() {
		t.Error("Should not be visible after SetVisible(false)")
	}

	o.SetVisible(true)
	if !o.IsVisible() {
		t.Error("Should be visible after SetVisible(true)")
	}
}

func TestBaseOverlaySetRange(t *testing.T) {
	o := NewBaseOverlay("test", TypeGhostText, PriorityNormal, Range{})

	newRange := Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 10, Col: 0},
	}

	o.SetRange(newRange)

	if o.Range() != newRange {
		t.Errorf("Range() = %v, want %v", o.Range(), newRange)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.ShowGhostText {
		t.Error("ShowGhostText should be true by default")
	}
	if !cfg.ShowDiffPreview {
		t.Error("ShowDiffPreview should be true by default")
	}
	if !cfg.ShowDiagnostics {
		t.Error("ShowDiagnostics should be true by default")
	}
	if !cfg.AnimateGhostText {
		t.Error("AnimateGhostText should be true by default")
	}
	if cfg.GhostTextDelay != 300 {
		t.Errorf("GhostTextDelay = %d, want 300", cfg.GhostTextDelay)
	}

	// Verify styles are set
	if cfg.GhostTextStyle.Foreground == core.ColorDefault {
		t.Error("GhostTextStyle should have a foreground color")
	}
	if cfg.DiffAddStyle.Foreground == core.ColorDefault {
		t.Error("DiffAddStyle should have a foreground color")
	}
	if cfg.DiffDeleteStyle.Foreground == core.ColorDefault {
		t.Error("DiffDeleteStyle should have a foreground color")
	}
}

func TestEmptyCell(t *testing.T) {
	cell := EmptyCell()

	if cell.Rune != ' ' {
		t.Errorf("Rune = %q, want ' '", cell.Rune)
	}
	if cell.Width != 1 {
		t.Errorf("Width = %d, want 1", cell.Width)
	}
	if cell.IsOverlay {
		t.Error("IsOverlay should be false for empty cell")
	}
}

func TestMergeStyles(t *testing.T) {
	base := core.NewStyle(core.ColorFromRGB(255, 255, 255)).
		WithBackground(core.ColorFromRGB(0, 0, 0))

	overlay := core.NewStyle(core.ColorFromRGB(128, 128, 128)).
		Bold().Italic()

	merged := MergeStyles(base, overlay)

	// Overlay foreground takes precedence
	if merged.Foreground != overlay.Foreground {
		t.Error("Overlay foreground should take precedence")
	}

	// Base background retained (overlay has default background)
	if merged.Background != base.Background {
		t.Error("Base background should be retained when overlay has default")
	}

	// Attributes are merged
	if !merged.Attributes.Has(core.AttrBold) {
		t.Error("Merged style should have bold from overlay")
	}
	if !merged.Attributes.Has(core.AttrItalic) {
		t.Error("Merged style should have italic from overlay")
	}
}

func TestMergeStylesWithOverlayBackground(t *testing.T) {
	base := core.NewStyle(core.ColorFromRGB(255, 255, 255)).
		WithBackground(core.ColorFromRGB(0, 0, 0))

	overlay := core.NewStyle(core.ColorFromRGB(128, 128, 128)).
		WithBackground(core.ColorFromRGB(50, 50, 50))

	merged := MergeStyles(base, overlay)

	// Overlay background takes precedence when set
	if merged.Background != overlay.Background {
		t.Error("Overlay background should take precedence when set")
	}
}

func TestPriorityConstants(t *testing.T) {
	if PriorityLow >= PriorityNormal {
		t.Error("PriorityLow should be less than PriorityNormal")
	}
	if PriorityNormal >= PriorityHigh {
		t.Error("PriorityNormal should be less than PriorityHigh")
	}
	if PriorityHigh >= PriorityCritical {
		t.Error("PriorityHigh should be less than PriorityCritical")
	}
}
