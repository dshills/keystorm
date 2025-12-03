package cursor

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestStyleString(t *testing.T) {
	tests := []struct {
		style Style
		want  string
	}{
		{StyleBlock, "block"},
		{StyleBar, "bar"},
		{StyleUnderline, "underline"},
		{StyleHollow, "hollow"},
		{Style(99), "block"}, // Unknown style
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.style.String(); got != tt.want {
				t.Errorf("Style.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStyleFromString(t *testing.T) {
	tests := []struct {
		input string
		want  Style
	}{
		{"block", StyleBlock},
		{"bar", StyleBar},
		{"line", StyleBar},
		{"underline", StyleUnderline},
		{"underscore", StyleUnderline},
		{"hollow", StyleHollow},
		{"unknown", StyleBlock},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := StyleFromString(tt.input); got != tt.want {
				t.Errorf("StyleFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Style != StyleBlock {
		t.Errorf("DefaultConfig().Style = %v, want StyleBlock", cfg.Style)
	}
	if !cfg.BlinkEnabled {
		t.Error("DefaultConfig().BlinkEnabled = false, want true")
	}
	if cfg.BlinkRate != 500*time.Millisecond {
		t.Errorf("DefaultConfig().BlinkRate = %v, want 500ms", cfg.BlinkRate)
	}
	if !cfg.BlinkOnType {
		t.Error("DefaultConfig().BlinkOnType = false, want true")
	}
}

func TestRendererNew(t *testing.T) {
	cfg := DefaultConfig()
	r := New(cfg)

	if r == nil {
		t.Fatal("New() returned nil")
	}

	gotCfg := r.Config()
	if gotCfg.Style != cfg.Style {
		t.Errorf("Config().Style = %v, want %v", gotCfg.Style, cfg.Style)
	}
	if !r.IsVisible() {
		t.Error("New renderer should start visible")
	}
}

func TestRendererSetConfig(t *testing.T) {
	r := New(DefaultConfig())

	newCfg := Config{
		Style:        StyleBar,
		BlinkEnabled: false,
		BlinkRate:    250 * time.Millisecond,
	}
	r.SetConfig(newCfg)

	got := r.Config()
	if got.Style != StyleBar {
		t.Errorf("Config().Style = %v, want StyleBar", got.Style)
	}
	if got.BlinkEnabled {
		t.Error("Config().BlinkEnabled = true, want false")
	}
}

func TestRendererSetStyle(t *testing.T) {
	r := New(DefaultConfig())

	r.SetStyle(StyleUnderline)
	if r.Style() != StyleUnderline {
		t.Errorf("Style() = %v, want StyleUnderline", r.Style())
	}
}

func TestRendererSetCursors(t *testing.T) {
	r := New(DefaultConfig())

	cursors := []Cursor{
		{Position: Position{Line: 1, Column: 5}, IsPrimary: true, Visible: true},
		{Position: Position{Line: 2, Column: 10}, IsPrimary: false, Visible: true},
	}
	r.SetCursors(cursors)

	got := r.Cursors()
	if len(got) != 2 {
		t.Fatalf("Cursors() returned %d cursors, want 2", len(got))
	}
	if got[0].Position.Line != 1 || got[0].Position.Column != 5 {
		t.Errorf("Cursor[0] position = (%d, %d), want (1, 5)", got[0].Position.Line, got[0].Position.Column)
	}
}

func TestRendererSetPrimaryCursor(t *testing.T) {
	r := New(DefaultConfig())

	r.SetPrimaryCursor(10, 20)

	cursor, ok := r.PrimaryCursor()
	if !ok {
		t.Fatal("PrimaryCursor() returned false")
	}
	if cursor.Position.Line != 10 || cursor.Position.Column != 20 {
		t.Errorf("PrimaryCursor position = (%d, %d), want (10, 20)", cursor.Position.Line, cursor.Position.Column)
	}
	if !cursor.IsPrimary {
		t.Error("Primary cursor should have IsPrimary = true")
	}
}

func TestRendererPrimaryCursorNotFound(t *testing.T) {
	r := New(DefaultConfig())

	// Set cursors without primary flag
	r.SetCursors([]Cursor{
		{Position: Position{Line: 1, Column: 5}, IsPrimary: false, Visible: true},
	})

	cursor, ok := r.PrimaryCursor()
	if !ok {
		t.Fatal("PrimaryCursor() should return first cursor when no primary")
	}
	if cursor.Position.Line != 1 {
		t.Errorf("PrimaryCursor returned wrong cursor")
	}
}

func TestRendererPrimaryCursorEmpty(t *testing.T) {
	r := New(DefaultConfig())

	_, ok := r.PrimaryCursor()
	if ok {
		t.Error("PrimaryCursor() should return false for empty cursors")
	}
}

func TestRendererBlinkUpdate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BlinkRate = 10 * time.Millisecond
	r := New(cfg)

	// Initially visible
	if !r.IsVisible() {
		t.Error("Should start visible")
	}

	// Wait for blink
	time.Sleep(15 * time.Millisecond)
	changed := r.Update(time.Now())

	if !changed {
		t.Error("Update should return true when visibility changes")
	}
	if r.IsVisible() {
		t.Error("Should be invisible after blink")
	}

	// Wait for another blink
	time.Sleep(15 * time.Millisecond)
	r.Update(time.Now())

	if !r.IsVisible() {
		t.Error("Should be visible after second blink")
	}
}

func TestRendererBlinkDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BlinkEnabled = false
	r := New(cfg)

	// Force visibility to false
	r.mu.Lock()
	r.blinkVisible = false
	r.mu.Unlock()

	// Update should reset to visible when blink disabled
	changed := r.Update(time.Now())
	if !changed {
		t.Error("Update should return true when visibility restored")
	}
	if !r.IsVisible() {
		t.Error("Should always be visible when blink disabled")
	}
}

func TestRendererResetBlink(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BlinkRate = 10 * time.Millisecond
	r := New(cfg)

	// Wait for blink
	time.Sleep(15 * time.Millisecond)
	r.Update(time.Now())

	// Reset blink
	r.ResetBlink()

	if !r.IsVisible() {
		t.Error("Should be visible after ResetBlink")
	}
}

func TestRendererPauseBlink(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BlinkRate = 10 * time.Millisecond
	r := New(cfg)

	// Pause blink
	r.PauseBlink(100 * time.Millisecond)

	// Wait for what would be a blink
	time.Sleep(15 * time.Millisecond)
	r.Update(time.Now())

	if !r.IsVisible() {
		t.Error("Should stay visible during pause")
	}
}

func TestRendererGetRenderStates(t *testing.T) {
	cfg := DefaultConfig()
	r := New(cfg)

	r.SetCursors([]Cursor{
		{Position: Position{Line: 0, Column: 0}, IsPrimary: true, Visible: true},
		{Position: Position{Line: 1, Column: 5}, IsPrimary: false, Visible: true},
	})

	// Converter that makes all cursors visible
	converter := func(line, col uint32) (int, int, bool) {
		return int(col), int(line), true
	}

	states := r.GetRenderStates(converter)
	if len(states) != 2 {
		t.Fatalf("GetRenderStates returned %d states, want 2", len(states))
	}

	if !states[0].IsPrimary {
		t.Error("First cursor should be primary")
	}
	if states[1].IsPrimary {
		t.Error("Second cursor should not be primary")
	}
}

func TestRendererGetRenderStatesFiltered(t *testing.T) {
	cfg := DefaultConfig()
	r := New(cfg)

	r.SetCursors([]Cursor{
		{Position: Position{Line: 0, Column: 0}, IsPrimary: true, Visible: true},
		{Position: Position{Line: 100, Column: 5}, IsPrimary: false, Visible: true}, // Off screen
	})

	// Converter that only shows line 0
	converter := func(line, col uint32) (int, int, bool) {
		return int(col), int(line), line < 10
	}

	states := r.GetRenderStates(converter)
	if len(states) != 1 {
		t.Fatalf("GetRenderStates should filter off-screen cursors, got %d", len(states))
	}
}

func TestRendererCursorCellBlock(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Style = StyleBlock
	r := New(cfg)

	baseCell := core.Cell{
		Rune:  'A',
		Width: 1,
		Style: core.Style{
			Foreground: core.ColorWhite,
			Background: core.ColorBlack,
		},
	}

	state := RenderState{
		Style:   StyleBlock,
		Color:   core.ColorDefault,
		Visible: true,
	}

	result := r.CursorCell(baseCell, state)

	// Block cursor inverts colors
	if result.Style.Background == baseCell.Style.Background {
		t.Error("Block cursor should invert background color")
	}
}

func TestRendererCursorCellUnderline(t *testing.T) {
	cfg := DefaultConfig()
	r := New(cfg)

	baseCell := core.Cell{
		Rune:  'A',
		Width: 1,
		Style: core.DefaultStyle(),
	}

	state := RenderState{
		Style:   StyleUnderline,
		Visible: true,
	}

	result := r.CursorCell(baseCell, state)

	// Underline cursor adds underline attribute
	if result.Style.Attributes&core.AttrUnderline == 0 {
		t.Error("Underline cursor should add underline attribute")
	}
}

func TestRendererCursorCellNotVisible(t *testing.T) {
	cfg := DefaultConfig()
	r := New(cfg)

	baseCell := core.Cell{Rune: 'A', Width: 1}

	state := RenderState{
		Visible: false,
	}

	result := r.CursorCell(baseCell, state)

	if result.Rune != baseCell.Rune {
		t.Error("Invisible cursor should return base cell unchanged")
	}
}

func TestRendererCursorCellBar(t *testing.T) {
	cfg := DefaultConfig()
	r := New(cfg)

	baseCell := core.Cell{Rune: 'A', Width: 1}

	state := RenderState{
		Style:   StyleBar,
		Visible: true,
	}

	result := r.CursorCell(baseCell, state)

	// Bar cursor returns base cell unchanged (terminal handles it)
	if result.Rune != baseCell.Rune {
		t.Error("Bar cursor should return base cell unchanged")
	}
}

func TestRendererCursorCellHollow(t *testing.T) {
	cfg := DefaultConfig()
	r := New(cfg)

	baseCell := core.Cell{
		Rune:  'A',
		Width: 1,
		Style: core.DefaultStyle(),
	}

	state := RenderState{
		Style:   StyleHollow,
		Visible: true,
	}

	result := r.CursorCell(baseCell, state)

	// Hollow cursor uses reverse video
	if result.Style.Attributes&core.AttrReverse == 0 {
		t.Error("Hollow cursor should use reverse video")
	}
}
