package linecache

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
	"github.com/dshills/keystorm/internal/renderer/dirty"
	"github.com/dshills/keystorm/internal/renderer/layout"
)

func newTestLineRenderer() *LineRenderer {
	layoutEngine := layout.NewLayoutEngine(4)
	layoutCache := layout.NewLineCache(layoutEngine, 100)
	cache := New(layoutCache, DefaultConfig())
	return NewLineRenderer(cache)
}

func TestNewLineRenderer(t *testing.T) {
	lr := newTestLineRenderer()

	if lr == nil {
		t.Fatal("NewLineRenderer returned nil")
	}
	if lr.cache == nil {
		t.Error("cache should be initialized")
	}
	if lr.styleResolver == nil {
		t.Error("styleResolver should be initialized")
	}
	if lr.screenWidth != 80 {
		t.Errorf("screenWidth = %d, want 80", lr.screenWidth)
	}
	if lr.screenHeight != 24 {
		t.Errorf("screenHeight = %d, want 24", lr.screenHeight)
	}
}

func TestLineRendererSetScreenSize(t *testing.T) {
	lr := newTestLineRenderer()

	lr.SetScreenSize(120, 40)

	if lr.screenWidth != 120 {
		t.Errorf("screenWidth = %d, want 120", lr.screenWidth)
	}
	if lr.screenHeight != 40 {
		t.Errorf("screenHeight = %d, want 40", lr.screenHeight)
	}
}

func TestLineRendererSetGutterWidth(t *testing.T) {
	lr := newTestLineRenderer()

	lr.SetGutterWidth(6)

	if lr.gutterWidth != 6 {
		t.Errorf("gutterWidth = %d, want 6", lr.gutterWidth)
	}
}

func TestLineRendererSetViewport(t *testing.T) {
	lr := newTestLineRenderer()

	lr.SetViewport(100, 10)

	if lr.topLine != 100 {
		t.Errorf("topLine = %d, want 100", lr.topLine)
	}
	if lr.leftColumn != 10 {
		t.Errorf("leftColumn = %d, want 10", lr.leftColumn)
	}
}

func TestLineRendererSetStyles(t *testing.T) {
	lr := newTestLineRenderer()

	baseStyle := core.NewStyle(core.ColorFromRGB(200, 200, 200))
	gutterStyle := core.NewStyle(core.ColorFromRGB(100, 100, 100))
	selStyle := core.NewStyle(core.ColorDefault).WithBackground(core.ColorFromRGB(50, 50, 100))

	lr.SetBaseStyle(baseStyle)
	lr.SetGutterStyle(gutterStyle)
	lr.SetSelectionStyle(selStyle)

	if lr.baseStyle != baseStyle {
		t.Error("baseStyle not set correctly")
	}
	if lr.gutterStyle != gutterStyle {
		t.Error("gutterStyle not set correctly")
	}
	if lr.selectionStyle != selStyle {
		t.Error("selectionStyle not set correctly")
	}
}

func TestLineRendererRenderVisibleLines(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(80, 5)
	lr.SetGutterWidth(4)
	lr.SetViewport(0, 0)

	getText := func(line uint32) string {
		lines := []string{"line 0", "line 1", "line 2", "line 3", "line 4"}
		if int(line) < len(lines) {
			return lines[line]
		}
		return ""
	}

	result := lr.RenderVisibleLines(getText)

	if len(result) != 5 {
		t.Fatalf("len(result) = %d, want 5", len(result))
	}

	for i, rl := range result {
		if rl.Line != uint32(i) {
			t.Errorf("result[%d].Line = %d, want %d", i, rl.Line, i)
		}
		if rl.ScreenRow != i {
			t.Errorf("result[%d].ScreenRow = %d, want %d", i, rl.ScreenRow, i)
		}
		if len(rl.Cells) == 0 {
			t.Errorf("result[%d].Cells is empty", i)
		}
		if len(rl.GutterCells) == 0 {
			t.Errorf("result[%d].GutterCells is empty", i)
		}
	}
}

func TestLineRendererRenderDirtyLines(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(80, 10)
	lr.SetViewport(0, 0)

	tracker := dirty.NewTracker(80, 10)
	lr.SetDirtyTracker(tracker)
	lr.cache.SetDirtyTracker(tracker)

	// Clear and mark specific lines dirty
	tracker.Clear()
	tracker.MarkLine(2)
	tracker.MarkLine(5)

	getText := func(line uint32) string {
		return "test line"
	}

	result := lr.RenderDirtyLines(getText)

	// Should only render dirty lines
	if len(result) != 2 {
		t.Errorf("len(result) = %d, want 2", len(result))
	}

	// Verify the correct lines were rendered
	foundLine2 := false
	foundLine5 := false
	for _, rl := range result {
		if rl.Line == 2 {
			foundLine2 = true
		}
		if rl.Line == 5 {
			foundLine5 = true
		}
	}

	if !foundLine2 {
		t.Error("Line 2 should be in results")
	}
	if !foundLine5 {
		t.Error("Line 5 should be in results")
	}
}

func TestLineRendererRenderDirtyLinesFullRedraw(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(80, 5)
	lr.SetViewport(0, 0)

	tracker := dirty.NewTracker(80, 5)
	lr.SetDirtyTracker(tracker)

	// Mark full redraw
	tracker.MarkFullRedraw()

	getText := func(line uint32) string {
		return "test"
	}

	result := lr.RenderDirtyLines(getText)

	// Full redraw should render all visible lines
	if len(result) != 5 {
		t.Errorf("len(result) = %d, want 5", len(result))
	}
}

func TestLineRendererVisibleLineRange(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(80, 24)
	lr.SetViewport(100, 0)

	startLine, endLine := lr.VisibleLineRange()

	if startLine != 100 {
		t.Errorf("startLine = %d, want 100", startLine)
	}
	if endLine != 123 {
		t.Errorf("endLine = %d, want 123", endLine)
	}
}

func TestLineRendererLineToScreenRow(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(80, 24)
	lr.SetViewport(100, 0)

	tests := []struct {
		line uint32
		want int
	}{
		{100, 0},  // First visible line
		{110, 10}, // Middle
		{123, 23}, // Last visible line
		{99, -1},  // Before viewport
		{124, -1}, // After viewport
	}

	for _, tt := range tests {
		got := lr.LineToScreenRow(tt.line)
		if got != tt.want {
			t.Errorf("LineToScreenRow(%d) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestLineRendererScreenRowToLine(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(80, 24)
	lr.SetViewport(100, 0)

	tests := []struct {
		row  int
		want uint32
	}{
		{0, 100},
		{10, 110},
		{23, 123},
		{-1, 100}, // Negative row returns topLine
	}

	for _, tt := range tests {
		got := lr.ScreenRowToLine(tt.row)
		if got != tt.want {
			t.Errorf("ScreenRowToLine(%d) = %d, want %d", tt.row, got, tt.want)
		}
	}
}

func TestFormatLineNumber(t *testing.T) {
	tests := []struct {
		num   uint32
		width int
		want  string
	}{
		{1, 4, "   1"},
		{10, 4, "  10"},
		{100, 4, " 100"},
		{1000, 4, "1000"},
		{12345, 4, "12345"}, // Exceeds width
		{0, 4, ""},          // Zero returns empty
		{1, 0, ""},          // Zero width returns empty
	}

	for _, tt := range tests {
		got := formatLineNumber(tt.num, tt.width)
		if got != tt.want {
			t.Errorf("formatLineNumber(%d, %d) = %q, want %q", tt.num, tt.width, got, tt.want)
		}
	}
}

func TestLineRendererBuildGutterCells(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetGutterWidth(5)

	cells := lr.buildGutterCells(9) // Line 9 displays as "10"

	if len(cells) != 5 {
		t.Fatalf("len(cells) = %d, want 5", len(cells))
	}

	// Check content (should be "  10 " with separator)
	expected := "  10 "
	got := ""
	for _, c := range cells {
		got += string(c.Rune)
	}
	if got != expected {
		t.Errorf("gutter content = %q, want %q", got, expected)
	}
}

func TestLineRendererBuildGutterCellsZeroWidth(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetGutterWidth(0)

	cells := lr.buildGutterCells(5)

	if cells != nil {
		t.Errorf("cells should be nil for zero gutter width")
	}
}

func TestCompositorNew(t *testing.T) {
	c := NewCompositor()

	if c == nil {
		t.Fatal("NewCompositor returned nil")
	}
}

func TestCompositorSetGhostTextStyle(t *testing.T) {
	c := NewCompositor()
	style := core.NewStyle(core.ColorFromRGB(128, 128, 128)).Dim()

	c.SetGhostTextStyle(style)

	if c.ghostTextStyle != style {
		t.Error("ghostTextStyle not set correctly")
	}
}

func TestCompositorSetDiffStyles(t *testing.T) {
	c := NewCompositor()
	addStyle := core.NewStyle(core.ColorFromRGB(0, 255, 0))
	delStyle := core.NewStyle(core.ColorFromRGB(255, 0, 0))
	modStyle := core.NewStyle(core.ColorFromRGB(255, 255, 0))

	c.SetDiffStyles(addStyle, delStyle, modStyle)

	if c.diffAddStyle != addStyle {
		t.Error("diffAddStyle not set correctly")
	}
	if c.diffDelStyle != delStyle {
		t.Error("diffDelStyle not set correctly")
	}
	if c.diffModStyle != modStyle {
		t.Error("diffModStyle not set correctly")
	}
}

func TestCompositorApplyGhostText(t *testing.T) {
	c := NewCompositor()

	line := &RenderLine{
		Line:      0,
		ScreenRow: 0,
		Cells:     []core.Cell{},
	}

	c.ApplyGhostText(line, "suggestion")

	if !line.HasOverlay {
		t.Error("HasOverlay should be true")
	}
	if len(line.AfterContent) == 0 {
		t.Error("AfterContent should not be empty")
	}
	if len(line.AfterContent) != 10 { // "suggestion" = 10 chars
		t.Errorf("len(AfterContent) = %d, want 10", len(line.AfterContent))
	}
}

func TestCompositorApplyGhostTextEmpty(t *testing.T) {
	c := NewCompositor()

	line := &RenderLine{
		Line:      0,
		ScreenRow: 0,
		Cells:     []core.Cell{},
	}

	c.ApplyGhostText(line, "")

	if line.HasOverlay {
		t.Error("HasOverlay should be false for empty text")
	}
	if line.AfterContent != nil {
		t.Error("AfterContent should be nil for empty text")
	}
}

func TestCompositorApplyDiffAdd(t *testing.T) {
	c := NewCompositor()
	c.SetDiffStyles(
		core.NewStyle(core.ColorFromRGB(0, 255, 0)),
		core.DefaultStyle(),
		core.DefaultStyle(),
	)

	cells := []core.Cell{
		{Rune: 'a', Width: 1, Style: core.DefaultStyle()},
		{Rune: 'b', Width: 1, Style: core.DefaultStyle()},
	}

	result := c.ApplyDiffAdd(cells)

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}

	for i, cell := range result {
		if cell.Style != c.diffAddStyle {
			t.Errorf("result[%d].Style not set to diffAddStyle", i)
		}
		// Original content preserved
		if cell.Rune != cells[i].Rune {
			t.Errorf("result[%d].Rune = %c, want %c", i, cell.Rune, cells[i].Rune)
		}
	}
}

func TestCompositorApplyDiffDelete(t *testing.T) {
	c := NewCompositor()
	c.SetDiffStyles(
		core.DefaultStyle(),
		core.NewStyle(core.ColorFromRGB(255, 0, 0)),
		core.DefaultStyle(),
	)

	cells := []core.Cell{
		{Rune: 'x', Width: 1, Style: core.DefaultStyle()},
	}

	result := c.ApplyDiffDelete(cells)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Style != c.diffDelStyle {
		t.Error("result[0].Style not set to diffDelStyle")
	}
}

func TestCompositorApplyDiffModify(t *testing.T) {
	c := NewCompositor()
	c.SetDiffStyles(
		core.DefaultStyle(),
		core.DefaultStyle(),
		core.NewStyle(core.ColorFromRGB(255, 255, 0)),
	)

	cells := []core.Cell{
		{Rune: 'm', Width: 1, Style: core.DefaultStyle()},
	}

	result := c.ApplyDiffModify(cells)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Style != c.diffModStyle {
		t.Error("result[0].Style not set to diffModStyle")
	}
}

func TestLineRendererContentWidth(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(100, 24)
	lr.SetGutterWidth(6)

	// Render a line and check content width
	getText := func(line uint32) string {
		return "hello world"
	}

	result := lr.RenderVisibleLines(getText)

	if len(result) == 0 {
		t.Fatal("No lines rendered")
	}

	// Content width should be screenWidth - gutterWidth = 100 - 6 = 94
	expectedContentWidth := 94
	if len(result[0].Cells) != expectedContentWidth {
		t.Errorf("len(Cells) = %d, want %d", len(result[0].Cells), expectedContentWidth)
	}
}

func TestLineRendererHorizontalScroll(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(20, 1)
	lr.SetGutterWidth(4)
	lr.SetViewport(0, 5) // Scroll 5 columns right

	getText := func(line uint32) string {
		return "0123456789ABCDEFGHIJ"
	}

	result := lr.RenderVisibleLines(getText)

	if len(result) == 0 {
		t.Fatal("No lines rendered")
	}

	// First visible character should be '5' (offset by 5)
	if result[0].Cells[0].Rune != '5' {
		t.Errorf("First cell rune = %c, want '5'", result[0].Cells[0].Rune)
	}
}

func TestRenderLineDirtyFlag(t *testing.T) {
	lr := newTestLineRenderer()
	lr.SetScreenSize(80, 5)
	lr.SetViewport(0, 0)

	tracker := dirty.NewTracker(80, 5)
	lr.SetDirtyTracker(tracker)
	lr.cache.SetDirtyTracker(tracker)

	// Mark line 2 as dirty
	tracker.Clear()
	tracker.MarkLine(2)

	getText := func(line uint32) string {
		return "test"
	}

	result := lr.RenderVisibleLines(getText)

	for _, rl := range result {
		if rl.Line == 2 {
			if !rl.IsDirty {
				t.Error("Line 2 should be marked dirty")
			}
		} else {
			if rl.IsDirty {
				t.Errorf("Line %d should not be marked dirty", rl.Line)
			}
		}
	}
}
