package overlay

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
}

func TestManagerConfig(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	if m.Config() != config {
		t.Error("Config() should return initial config")
	}

	newConfig := Config{ShowGhostText: false}
	m.SetConfig(newConfig)

	if m.Config().ShowGhostText != false {
		t.Error("SetConfig should update config")
	}
}

func TestManagerAddRemove(t *testing.T) {
	m := NewManager(DefaultConfig())

	// Create a simple overlay
	o := NewBaseOverlay("test-1", TypeGhostText, PriorityNormal, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})

	// Wrap in a simple overlay implementation
	gt := &simpleOverlay{BaseOverlay: o}

	m.Add(gt)

	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1", m.Count())
	}

	got, ok := m.Get("test-1")
	if !ok {
		t.Error("Get should find added overlay")
	}
	if got.ID() != "test-1" {
		t.Errorf("ID() = %q, want %q", got.ID(), "test-1")
	}

	// Remove
	if !m.Remove("test-1") {
		t.Error("Remove should return true for existing overlay")
	}
	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after remove", m.Count())
	}

	// Remove non-existent
	if m.Remove("non-existent") {
		t.Error("Remove should return false for non-existent overlay")
	}
}

// simpleOverlay is a minimal Overlay implementation for testing
type simpleOverlay struct {
	*BaseOverlay
}

func (s *simpleOverlay) SpansForLine(line uint32) []Span {
	if !s.IsVisible() || !s.Range().ContainsLine(line) {
		return nil
	}
	return []Span{{StartCol: 0, EndCol: 10, Style: core.DefaultStyle()}}
}

func TestManagerClear(t *testing.T) {
	m := NewManager(DefaultConfig())

	for i := 0; i < 5; i++ {
		o := &simpleOverlay{BaseOverlay: NewBaseOverlay(
			"test-"+string(rune('0'+i)),
			TypeGhostText,
			PriorityNormal,
			Range{},
		)}
		m.Add(o)
	}

	m.Clear()

	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after Clear", m.Count())
	}
}

func TestManagerClearType(t *testing.T) {
	m := NewManager(DefaultConfig())

	// Add different types
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("ghost-1", TypeGhostText, PriorityNormal, Range{})})
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("ghost-2", TypeGhostText, PriorityNormal, Range{})})
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("diff-1", TypeDiffAdd, PriorityNormal, Range{})})

	m.ClearType(TypeGhostText)

	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1 after ClearType", m.Count())
	}

	_, ok := m.Get("diff-1")
	if !ok {
		t.Error("Non-matching type should remain after ClearType")
	}
}

func TestManagerOverlaysOnLine(t *testing.T) {
	m := NewManager(DefaultConfig())

	// Add overlays on different lines
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("o1", TypeGhostText, PriorityLow, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})})
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("o2", TypeGhostText, PriorityHigh, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})})
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("o3", TypeGhostText, PriorityNormal, Range{
		Start: Position{Line: 10, Col: 0},
		End:   Position{Line: 10, Col: 10},
	})})

	overlays := m.OverlaysOnLine(5)
	if len(overlays) != 2 {
		t.Errorf("len(overlays) = %d, want 2", len(overlays))
	}

	// Should be sorted by priority (low to high)
	if overlays[0].Priority() > overlays[1].Priority() {
		t.Error("Overlays should be sorted by priority ascending")
	}

	overlays = m.OverlaysOnLine(10)
	if len(overlays) != 1 {
		t.Errorf("len(overlays) on line 10 = %d, want 1", len(overlays))
	}

	overlays = m.OverlaysOnLine(20)
	if len(overlays) != 0 {
		t.Errorf("len(overlays) on line 20 = %d, want 0", len(overlays))
	}
}

func TestManagerSpansForLine(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	// Add overlays
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("o1", TypeGhostText, PriorityNormal, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})})

	spans := m.SpansForLine(5)
	if len(spans) == 0 {
		t.Error("Should have spans for line with overlay")
	}

	spans = m.SpansForLine(10)
	if len(spans) != 0 {
		t.Error("Should have no spans for line without overlay")
	}
}

func TestManagerSpansForLineDisabled(t *testing.T) {
	config := DefaultConfig()
	config.ShowGhostText = false
	m := NewManager(config)

	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("ghost", TypeGhostText, PriorityNormal, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})})

	spans := m.SpansForLine(5)
	if len(spans) != 0 {
		t.Error("Should have no spans when ghost text disabled")
	}
}

func TestManagerGhostText(t *testing.T) {
	m := NewManager(DefaultConfig())
	style := core.DefaultStyle()

	gt := NewGhostText("ghost-1", Position{Line: 5, Col: 10}, "completion", style)
	m.SetGhostText(gt)

	if m.ActiveGhostText() != gt {
		t.Error("ActiveGhostText should return set ghost text")
	}

	// Set another ghost text (should replace)
	gt2 := NewGhostText("ghost-2", Position{Line: 10, Col: 0}, "another", style)
	m.SetGhostText(gt2)

	if m.ActiveGhostText() != gt2 {
		t.Error("SetGhostText should replace existing")
	}
	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1 (old should be removed)", m.Count())
	}

	// Clear ghost text
	m.ClearGhostText()
	if m.ActiveGhostText() != nil {
		t.Error("ActiveGhostText should be nil after ClearGhostText")
	}
}

func TestManagerAcceptGhostText(t *testing.T) {
	m := NewManager(DefaultConfig())
	style := core.DefaultStyle()

	gt := NewGhostText("ghost", Position{Line: 5, Col: 0}, "hello world", style)
	m.SetGhostText(gt)

	text := m.AcceptGhostText()
	if text != "hello world" {
		t.Errorf("AcceptGhostText() = %q, want %q", text, "hello world")
	}
	if m.ActiveGhostText() != nil {
		t.Error("Ghost text should be removed after accept")
	}

	// Accept with no ghost text
	text = m.AcceptGhostText()
	if text != "" {
		t.Errorf("AcceptGhostText() with no ghost = %q, want empty", text)
	}
}

func TestManagerAcceptGhostTextPartial(t *testing.T) {
	m := NewManager(DefaultConfig())
	style := core.DefaultStyle()

	gt := NewGhostText("ghost", Position{Line: 5, Col: 0}, "hello", style)
	m.SetGhostText(gt)

	text := m.AcceptGhostTextPartial()
	if text != "hello" {
		t.Errorf("First partial = %q, want %q", text, "hello")
	}

	// Need another call to finalize when at end of content
	m.AcceptGhostTextPartial()

	// Ghost text should be removed after full acceptance
	if m.ActiveGhostText() != nil {
		t.Error("Ghost text should be removed after full partial acceptance")
	}
}

func TestManagerRejectGhostText(t *testing.T) {
	m := NewManager(DefaultConfig())
	style := core.DefaultStyle()

	gt := NewGhostText("ghost", Position{Line: 5, Col: 0}, "hello", style)
	m.SetGhostText(gt)

	m.RejectGhostText()

	if m.ActiveGhostText() != nil {
		t.Error("Ghost text should be removed after reject")
	}
}

func TestManagerDiffPreview(t *testing.T) {
	m := NewManager(DefaultConfig())

	dp := NewDiffPreview("diff-1", nil, DefaultConfig())
	m.SetDiffPreview(dp)

	if m.ActiveDiff() != dp {
		t.Error("ActiveDiff should return set diff preview")
	}

	// Set another diff (should replace)
	dp2 := NewDiffPreview("diff-2", nil, DefaultConfig())
	m.SetDiffPreview(dp2)

	if m.ActiveDiff() != dp2 {
		t.Error("SetDiffPreview should replace existing")
	}

	// Clear diff preview
	m.ClearDiffPreview()
	if m.ActiveDiff() != nil {
		t.Error("ActiveDiff should be nil after ClearDiffPreview")
	}
}

func TestManagerAcceptDiff(t *testing.T) {
	m := NewManager(DefaultConfig())

	dp := NewDiffPreview("diff", nil, DefaultConfig())
	m.SetDiffPreview(dp)

	if !m.AcceptDiff() {
		t.Error("AcceptDiff should return true")
	}
	if m.ActiveDiff() != nil {
		t.Error("Diff should be removed after accept")
	}

	// Accept with no diff
	if m.AcceptDiff() {
		t.Error("AcceptDiff with no diff should return false")
	}
}

func TestManagerRejectDiff(t *testing.T) {
	m := NewManager(DefaultConfig())

	dp := NewDiffPreview("diff", nil, DefaultConfig())
	m.SetDiffPreview(dp)

	m.RejectDiff()

	if m.ActiveDiff() != nil {
		t.Error("Diff should be removed after reject")
	}
}

func TestManagerPrioritySorting(t *testing.T) {
	m := NewManager(DefaultConfig())

	// Add overlays in random priority order
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("high", TypeInlineHint, PriorityHigh, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})})
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("low", TypeInlineHint, PriorityLow, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})})
	m.Add(&simpleOverlay{BaseOverlay: NewBaseOverlay("normal", TypeInlineHint, PriorityNormal, Range{
		Start: Position{Line: 5, Col: 0},
		End:   Position{Line: 5, Col: 10},
	})})

	// Get overlays - should be sorted by priority
	overlays := m.OverlaysOnLine(5)
	if len(overlays) != 3 {
		t.Fatalf("len(overlays) = %d, want 3", len(overlays))
	}

	if overlays[0].Priority() != PriorityLow {
		t.Error("First overlay should have lowest priority")
	}
	if overlays[1].Priority() != PriorityNormal {
		t.Error("Second overlay should have normal priority")
	}
	if overlays[2].Priority() != PriorityHigh {
		t.Error("Third overlay should have highest priority")
	}
}

func TestNewCompositor(t *testing.T) {
	config := DefaultConfig()
	c := NewCompositor(config)

	if c == nil {
		t.Fatal("NewCompositor returned nil")
	}
}

func TestCompositorSetConfig(t *testing.T) {
	c := NewCompositor(DefaultConfig())

	newConfig := Config{ShowGhostText: false}
	c.SetConfig(newConfig)

	if c.config.ShowGhostText != false {
		t.Error("SetConfig should update config")
	}
}

func TestCompositorCompositeCell(t *testing.T) {
	c := NewCompositor(DefaultConfig())

	base := core.Cell{
		Rune:  'a',
		Width: 1,
		Style: core.DefaultStyle(),
	}

	overlayStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0)).Bold()
	span := Span{
		StartCol: 0,
		EndCol:   10,
		Style:    overlayStyle,
	}

	t.Run("within span", func(t *testing.T) {
		result := c.CompositeCell(base, span, 5)
		if result.Style.Foreground != overlayStyle.Foreground {
			t.Error("Should apply overlay style")
		}
	})

	t.Run("before span", func(t *testing.T) {
		span := Span{StartCol: 10, EndCol: 20, Style: overlayStyle}
		result := c.CompositeCell(base, span, 5)
		if result.Style.Foreground == overlayStyle.Foreground {
			t.Error("Should not apply style before span")
		}
	})

	t.Run("after span", func(t *testing.T) {
		result := c.CompositeCell(base, span, 15)
		if result.Style.Foreground == overlayStyle.Foreground {
			t.Error("Should not apply style after span")
		}
	})

	t.Run("replace content", func(t *testing.T) {
		span := Span{
			StartCol:       0,
			EndCol:         10,
			Text:           "replacement",
			Style:          overlayStyle,
			ReplaceContent: true,
		}
		result := c.CompositeCell(base, span, 0)
		if result.Rune != 'r' {
			t.Errorf("Rune = %q, want 'r'", result.Rune)
		}
	})
}

func TestCompositorCompositeLine(t *testing.T) {
	c := NewCompositor(DefaultConfig())

	baseCells := []core.Cell{
		{Rune: 'h', Width: 1, Style: core.DefaultStyle()},
		{Rune: 'e', Width: 1, Style: core.DefaultStyle()},
		{Rune: 'l', Width: 1, Style: core.DefaultStyle()},
		{Rune: 'l', Width: 1, Style: core.DefaultStyle()},
		{Rune: 'o', Width: 1, Style: core.DefaultStyle()},
	}

	t.Run("no spans", func(t *testing.T) {
		result := c.CompositeLine(baseCells, nil)
		if len(result) != len(baseCells) {
			t.Errorf("len(result) = %d, want %d", len(result), len(baseCells))
		}
	})

	t.Run("style overlay", func(t *testing.T) {
		overlayStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
		spans := []Span{
			{StartCol: 0, EndCol: 2, Style: overlayStyle},
		}

		result := c.CompositeLine(baseCells, spans)
		if result[0].Style.Foreground != overlayStyle.Foreground {
			t.Error("First cell should have overlay style")
		}
		if result[2].Style.Foreground == overlayStyle.Foreground {
			t.Error("Third cell should not have overlay style")
		}
	})

	t.Run("after content", func(t *testing.T) {
		spans := []Span{
			{StartCol: 0, Text: " world", Style: core.DefaultStyle(), AfterContent: true},
		}

		result := c.CompositeLine(baseCells, spans)
		if len(result) != len(baseCells)+6 {
			t.Errorf("len(result) = %d, want %d (with appended text)", len(result), len(baseCells)+6)
		}
	})

	t.Run("original unchanged", func(t *testing.T) {
		spans := []Span{
			{StartCol: 0, EndCol: 5, Style: core.NewStyle(core.ColorFromRGB(255, 0, 0))},
		}

		c.CompositeLine(baseCells, spans)

		// Original should be unchanged
		if baseCells[0].Style.Foreground != core.ColorDefault {
			t.Error("Original cells should not be modified")
		}
	})
}

func TestManagerIsTypeEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		typ     Type
		enabled bool
	}{
		{"ghost text enabled", Config{ShowGhostText: true}, TypeGhostText, true},
		{"ghost text disabled", Config{ShowGhostText: false}, TypeGhostText, false},
		{"diff add enabled", Config{ShowDiffPreview: true}, TypeDiffAdd, true},
		{"diff add disabled", Config{ShowDiffPreview: false}, TypeDiffAdd, false},
		{"diff delete enabled", Config{ShowDiffPreview: true}, TypeDiffDelete, true},
		{"diff modify enabled", Config{ShowDiffPreview: true}, TypeDiffModify, true},
		{"diagnostics enabled", Config{ShowDiagnostics: true}, TypeDiagnostic, true},
		{"diagnostics disabled", Config{ShowDiagnostics: false}, TypeDiagnostic, false},
		{"unknown type", Config{}, TypeInlineHint, true}, // Unknown types default to enabled
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.config)
			if got := m.isTypeEnabled(tt.typ); got != tt.enabled {
				t.Errorf("isTypeEnabled(%v) = %v, want %v", tt.typ, got, tt.enabled)
			}
		})
	}
}

func TestManagerAddTracksSpecialOverlays(t *testing.T) {
	m := NewManager(DefaultConfig())

	// Add ghost text via Add (not SetGhostText)
	gt := NewGhostText("ghost", Position{}, "test", core.DefaultStyle())
	m.Add(gt)

	if m.ActiveGhostText() != gt {
		t.Error("Add should track ghost text overlay")
	}

	// Add diff preview via Add
	dp := NewDiffPreview("diff", nil, DefaultConfig())
	m.Add(dp)

	if m.ActiveDiff() != dp {
		t.Error("Add should track diff preview overlay")
	}
}

func TestManagerRemoveClearsSpecialOverlays(t *testing.T) {
	m := NewManager(DefaultConfig())

	gt := NewGhostText("ghost", Position{}, "test", core.DefaultStyle())
	m.Add(gt)

	m.Remove("ghost")

	if m.ActiveGhostText() != nil {
		t.Error("Remove should clear ghost text reference")
	}

	dp := NewDiffPreview("diff", nil, DefaultConfig())
	m.Add(dp)

	m.Remove("diff")

	if m.ActiveDiff() != nil {
		t.Error("Remove should clear diff preview reference")
	}
}
