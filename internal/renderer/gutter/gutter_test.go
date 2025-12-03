package gutter

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.ShowLineNumbers {
		t.Error("ShowLineNumbers should be true by default")
	}
	if cfg.ShowSigns {
		t.Error("ShowSigns should be false by default")
	}
	if cfg.ShowFoldMarkers {
		t.Error("ShowFoldMarkers should be false by default")
	}
	if cfg.RelativeLineNumbers {
		t.Error("RelativeLineNumbers should be false by default")
	}
	if cfg.MinLineNumberWidth != 3 {
		t.Errorf("expected MinLineNumberWidth 3, got %d", cfg.MinLineNumberWidth)
	}
	if cfg.SignColumnWidth != 2 {
		t.Errorf("expected SignColumnWidth 2, got %d", cfg.SignColumnWidth)
	}
}

func TestNewGutter(t *testing.T) {
	g := New(DefaultConfig())

	if g == nil {
		t.Fatal("New returned nil")
	}

	width := g.Width()
	// Default: 3 line number digits + 1 separator = 4
	if width != 4 {
		t.Errorf("expected initial width 4, got %d", width)
	}
}

func TestGutterSetLineCount(t *testing.T) {
	g := New(DefaultConfig())

	// Small line count
	g.SetLineCount(10)
	width := g.Width()
	// Min 3 digits + separator = 4
	if width != 4 {
		t.Errorf("expected width 4 for 10 lines, got %d", width)
	}

	// Medium line count
	g.SetLineCount(1000)
	width = g.Width()
	// 4 digits + separator = 5
	if width != 5 {
		t.Errorf("expected width 5 for 1000 lines, got %d", width)
	}

	// Large line count
	g.SetLineCount(100000)
	width = g.Width()
	// 6 digits + separator = 7
	if width != 7 {
		t.Errorf("expected width 7 for 100000 lines, got %d", width)
	}
}

func TestGutterSetLineCountFixed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LineNumberWidth = 5 // Fixed width
	g := New(cfg)

	g.SetLineCount(1000000)
	width := g.Width()
	// Fixed 5 digits + separator = 6
	if width != 6 {
		t.Errorf("expected width 6 with fixed LineNumberWidth, got %d", width)
	}
}

func TestGutterSetCurrentLine(t *testing.T) {
	g := New(DefaultConfig())
	g.SetLineCount(100)

	g.SetCurrentLine(50)
	// This sets internal state for current line highlighting
	// Can't directly verify without rendering
}

func TestGutterRenderLine(t *testing.T) {
	g := New(DefaultConfig())
	g.SetLineCount(100)
	g.SetCurrentLine(5)

	// Render a normal line
	cells := g.RenderLine(10, true)
	if len(cells) == 0 {
		t.Error("RenderLine should return cells")
	}

	// Verify line number content
	// For line 10 (0-indexed) displayed as 11 (1-indexed)
	// With min width 3, should be " 11"
	numStr := ""
	for _, c := range cells[:g.Width()-1] {
		numStr += string(c.Rune)
	}
	// Line 10 (0-indexed) = 11 in 1-indexed display
	if numStr != " 11" {
		t.Errorf("expected ' 11', got %q", numStr)
	}
}

func TestGutterRenderCurrentLine(t *testing.T) {
	g := New(DefaultConfig())
	g.SetLineCount(100)
	g.SetCurrentLine(10)

	// Render the current line
	cells := g.RenderLine(10, true)
	if len(cells) == 0 {
		t.Error("RenderLine should return cells")
	}

	// Current line should have StyleCurrentLine
	if cells[0].Style != StyleCurrentLine {
		t.Errorf("expected StyleCurrentLine for current line, got %v", cells[0].Style)
	}
}

func TestGutterRenderNonVisibleLine(t *testing.T) {
	g := New(DefaultConfig())
	g.SetLineCount(100)

	// Render a non-visible line (like beyond buffer)
	cells := g.RenderLine(100, false)
	if len(cells) == 0 {
		t.Error("RenderLine should return cells for non-visible line")
	}

	// Non-visible line should show "~"
	hasPlaceholder := false
	for _, c := range cells {
		if c.Rune == '~' {
			hasPlaceholder = true
			break
		}
	}
	if !hasPlaceholder {
		t.Error("non-visible line should show placeholder")
	}
}

func TestGutterRelativeLineNumbers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RelativeLineNumbers = true
	g := New(cfg)
	g.SetLineCount(100)
	g.SetCurrentLine(50)

	// Render current line - should show absolute number
	cells := g.RenderLine(50, true)
	numStr := ""
	for _, c := range cells[:g.Width()-1] {
		numStr += string(c.Rune)
	}
	// Current line shows absolute (51)
	if numStr != " 51" {
		t.Errorf("current line should show '51', got %q", numStr)
	}

	// Render line above current - should show relative
	cells = g.RenderLine(48, true)
	numStr = ""
	for _, c := range cells[:g.Width()-1] {
		numStr += string(c.Rune)
	}
	// 2 lines above = relative 2
	if numStr != "  2" {
		t.Errorf("expected relative '2' for line 48, got %q", numStr)
	}

	// Render line below current - should show relative
	cells = g.RenderLine(52, true)
	numStr = ""
	for _, c := range cells[:g.Width()-1] {
		numStr += string(c.Rune)
	}
	// 2 lines below = relative 2
	if numStr != "  2" {
		t.Errorf("expected relative '2' for line 52, got %q", numStr)
	}
}

func TestGutterWithSigns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowSigns = true
	g := New(cfg)
	g.SetLineCount(100)

	// Should have sign column
	width := g.Width()
	expectedWidth := 3 + 1 + 2 // min digits + separator + sign column
	if width != expectedWidth {
		t.Errorf("expected width %d with signs, got %d", expectedWidth, width)
	}
}

func TestGutterConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowLineNumbers = false
	g := New(cfg)

	gotCfg := g.Config()
	if gotCfg.ShowLineNumbers {
		t.Error("Config should reflect constructor settings")
	}
}

func TestGutterSetConfig(t *testing.T) {
	g := New(DefaultConfig())

	newCfg := DefaultConfig()
	newCfg.ShowLineNumbers = false
	newCfg.ShowSigns = true
	g.SetConfig(newCfg)

	gotCfg := g.Config()
	if gotCfg.ShowLineNumbers {
		t.Error("Config should be updated")
	}
	if !gotCfg.ShowSigns {
		t.Error("ShowSigns should be updated")
	}
}

func TestGutterWidthNoLineNumbers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowLineNumbers = false
	g := New(cfg)

	width := g.Width()
	if width != 0 {
		t.Errorf("expected width 0 without line numbers, got %d", width)
	}
}

func TestGutterWidthOnlySigns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowLineNumbers = false
	cfg.ShowSigns = true
	g := New(cfg)

	width := g.Width()
	// Sign column + separator = 2 + 1 = 3
	if width != cfg.SignColumnWidth+1 {
		t.Errorf("expected width %d for signs only, got %d", cfg.SignColumnWidth+1, width)
	}
}

func TestSignType(t *testing.T) {
	// Verify sign type values
	if SignNone != 0 {
		t.Error("SignNone should be 0")
	}
	if SignError != 1 {
		t.Error("SignError should be 1")
	}
	if SignWarning != 2 {
		t.Error("SignWarning should be 2")
	}
	if SignInfo != 3 {
		t.Error("SignInfo should be 3")
	}
}

func TestCellStyle(t *testing.T) {
	// Verify cell style values
	if StyleNormal != 0 {
		t.Error("StyleNormal should be 0")
	}
	if StyleCurrentLine != 1 {
		t.Error("StyleCurrentLine should be 1")
	}
	if StyleDim != 2 {
		t.Error("StyleDim should be 2")
	}
}

func TestGutterConcurrency(t *testing.T) {
	g := New(DefaultConfig())
	g.SetLineCount(1000)

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = g.Width()
			_ = g.Config()
			_ = g.RenderLine(uint32(i%100), true)
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			g.SetCurrentLine(uint32(i % 100))
			g.SetLineCount(uint32(500 + i))
		}
		done <- true
	}()

	<-done
	<-done
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		n    uint32
		want string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{999, "999"},
		{1000, "1000"},
	}

	for _, tt := range tests {
		got := FormatNumber(tt.n)
		if got != tt.want {
			t.Errorf("FormatNumber(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestLineNumberWidth(t *testing.T) {
	g := New(DefaultConfig())

	// Check that LineNumberWidth method exists and returns correct value
	g.SetLineCount(100)
	width := g.LineNumberWidth()
	if width != 3 {
		t.Errorf("expected LineNumberWidth 3, got %d", width)
	}

	g.SetLineCount(10000)
	width = g.LineNumberWidth()
	if width != 5 {
		t.Errorf("expected LineNumberWidth 5, got %d", width)
	}
}
