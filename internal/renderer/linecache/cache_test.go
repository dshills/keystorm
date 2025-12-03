package linecache

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
	"github.com/dshills/keystorm/internal/renderer/dirty"
	"github.com/dshills/keystorm/internal/renderer/layout"
	"github.com/dshills/keystorm/internal/renderer/style"
)

// mockHighlightSource provides test highlighting.
type mockHighlightSource struct {
	spans map[uint32][]style.Span
}

func (m *mockHighlightSource) HighlightsForLine(line uint32) []style.Span {
	if m.spans == nil {
		return nil
	}
	return m.spans[line]
}

// mockOverlaySource provides test overlays.
type mockOverlaySource struct {
	spans map[uint32][]style.Span
}

func (m *mockOverlaySource) OverlaysForLine(line uint32) []style.Span {
	if m.spans == nil {
		return nil
	}
	return m.spans[line]
}

// mockSelectionSource provides test selections.
type mockSelectionSource struct {
	spans map[uint32][]style.Span
}

func (m *mockSelectionSource) SelectionSpansForLine(line uint32) []style.Span {
	if m.spans == nil {
		return nil
	}
	return m.spans[line]
}

func newTestCache() *Cache {
	layoutEngine := layout.NewLayoutEngine(4)
	layoutCache := layout.NewLineCache(layoutEngine, 100)
	return New(layoutCache, DefaultConfig())
}

func TestNew(t *testing.T) {
	cache := newTestCache()

	if cache == nil {
		t.Fatal("New returned nil")
	}
	if cache.styleResolver == nil {
		t.Error("styleResolver should be initialized")
	}
	if cache.entries == nil {
		t.Error("entries map should be initialized")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxCachedLines <= 0 {
		t.Error("MaxCachedLines should be positive")
	}
	if config.EvictionBatchSize <= 0 {
		t.Error("EvictionBatchSize should be positive")
	}
}

func TestCacheGetLine(t *testing.T) {
	cache := newTestCache()

	// First access - miss
	line1 := cache.GetLine(0, "hello world")
	if line1 == nil {
		t.Fatal("GetLine returned nil")
	}
	if line1.BufferLine != 0 {
		t.Errorf("BufferLine = %d, want 0", line1.BufferLine)
	}
	if len(line1.Cells) == 0 {
		t.Error("Cells should not be empty")
	}

	// Second access - hit
	line2 := cache.GetLine(0, "hello world")
	if line2 == nil {
		t.Fatal("Second GetLine returned nil")
	}

	stats := cache.Stats()
	if stats.Hits == 0 {
		t.Error("Should have cache hit on second access")
	}
}

func TestCacheGetLineContentChange(t *testing.T) {
	cache := newTestCache()

	// First access
	line1 := cache.GetLine(0, "hello")
	version1 := line1.ContentHash

	// Same content - should use cache
	line2 := cache.GetLine(0, "hello")
	if line2.ContentHash != version1 {
		t.Error("Same content should have same hash")
	}

	// Different content - should recompute
	line3 := cache.GetLine(0, "world")
	if line3.ContentHash == version1 {
		t.Error("Different content should have different hash")
	}
}

func TestCacheWithHighlighting(t *testing.T) {
	cache := newTestCache()

	// Add highlight source
	hlSrc := &mockHighlightSource{
		spans: map[uint32][]style.Span{
			0: {
				{
					StartCol: 0,
					EndCol:   5,
					Style:    core.NewStyle(core.ColorFromRGB(255, 0, 0)),
					Layer:    style.LayerSyntax,
					Merge:    style.MergeOverlay,
				},
			},
		},
	}
	cache.SetHighlightSource(hlSrc)

	line := cache.GetLine(0, "hello world")
	if line == nil {
		t.Fatal("GetLine returned nil")
	}

	// Verify styling was applied to first 5 characters
	if len(line.Cells) < 5 {
		t.Fatal("Not enough cells")
	}

	// First cell should have the highlight color
	if line.Cells[0].Style.Foreground == core.ColorDefault {
		t.Error("Highlight should have been applied")
	}
}

func TestCacheWithOverlay(t *testing.T) {
	cache := newTestCache()

	overlaySrc := &mockOverlaySource{
		spans: map[uint32][]style.Span{
			0: {
				{
					StartCol: 6,
					EndCol:   11,
					Style:    core.NewStyle(core.ColorFromRGB(0, 255, 0)),
					Layer:    style.LayerDiagnostic,
					Merge:    style.MergeOverlay,
				},
			},
		},
	}
	cache.SetOverlaySource(overlaySrc)

	line := cache.GetLine(0, "hello world")
	if line == nil {
		t.Fatal("GetLine returned nil")
	}
}

func TestCacheWithSelection(t *testing.T) {
	cache := newTestCache()

	selSrc := &mockSelectionSource{
		spans: map[uint32][]style.Span{
			0: {
				{
					StartCol: 0,
					EndCol:   5,
					Style:    core.NewStyle(core.ColorDefault).WithBackground(core.ColorFromRGB(60, 90, 130)),
					Layer:    style.LayerSelection,
					Merge:    style.MergeOverlay,
				},
			},
		},
	}
	cache.SetSelectionSource(selSrc)

	// GetStyledCells applies selection
	cells := cache.GetStyledCells(0, "hello world")
	if cells == nil {
		t.Fatal("GetStyledCells returned nil")
	}

	// First cell should have selection background
	if len(cells) > 0 && cells[0].Style.Background == core.ColorDefault {
		t.Error("Selection should have been applied")
	}
}

func TestCacheInvalidate(t *testing.T) {
	cache := newTestCache()

	// Cache a line
	cache.GetLine(0, "hello")

	stats1 := cache.Stats()
	if stats1.Size != 1 {
		t.Errorf("Size = %d, want 1", stats1.Size)
	}

	// Invalidate it
	cache.Invalidate(0)

	stats2 := cache.Stats()
	if stats2.Size != 0 {
		t.Errorf("Size = %d, want 0 after invalidate", stats2.Size)
	}
}

func TestCacheInvalidateRange(t *testing.T) {
	cache := newTestCache()

	// Cache multiple lines
	for i := uint32(0); i < 10; i++ {
		cache.GetLine(i, "line")
	}

	// Invalidate range
	cache.InvalidateRange(3, 7)

	stats := cache.Stats()
	if stats.Size != 5 { // 0-2 and 8-9 remain
		t.Errorf("Size = %d, want 5", stats.Size)
	}
}

func TestCacheInvalidateFrom(t *testing.T) {
	cache := newTestCache()

	// Cache multiple lines
	for i := uint32(0); i < 10; i++ {
		cache.GetLine(i, "line")
	}

	// Invalidate from line 5
	cache.InvalidateFrom(5)

	stats := cache.Stats()
	if stats.Size != 5 { // 0-4 remain
		t.Errorf("Size = %d, want 5", stats.Size)
	}
}

func TestCacheInvalidateStyles(t *testing.T) {
	cache := newTestCache()

	// Cache a line
	line1 := cache.GetLine(0, "hello")
	version1 := line1.Version

	// Invalidate styles
	cache.InvalidateStyles()

	// Line should be re-computed
	line2 := cache.GetLine(0, "hello")
	if line2.Version == version1 {
		t.Error("Version should have changed after InvalidateStyles")
	}
}

func TestCacheInvalidateAll(t *testing.T) {
	cache := newTestCache()

	// Cache multiple lines
	for i := uint32(0); i < 10; i++ {
		cache.GetLine(i, "line")
	}

	cache.InvalidateAll()

	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Size = %d, want 0 after InvalidateAll", stats.Size)
	}
}

func TestCacheShiftLines(t *testing.T) {
	cache := newTestCache()

	// Cache lines 0, 1, 2
	cache.GetLine(0, "line0")
	cache.GetLine(1, "line1")
	cache.GetLine(2, "line2")

	// Insert a line at position 1 (shift lines 1+ down by 1)
	cache.ShiftLines(1, 1)

	// Line 0 should still be there
	line0 := cache.GetLine(0, "line0")
	if line0.BufferLine != 0 {
		t.Errorf("Line 0 BufferLine = %d, want 0", line0.BufferLine)
	}

	// Old line 1 should now be at line 2
	// (cache entry was moved)
	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("Size = %d, want 3", stats.Size)
	}
}

func TestCacheEviction(t *testing.T) {
	layoutEngine := layout.NewLayoutEngine(4)
	layoutCache := layout.NewLineCache(layoutEngine, 100)

	config := Config{
		MaxCachedLines:    10,
		CacheStyledCells:  true,
		EvictionBatchSize: 5,
	}
	cache := New(layoutCache, config)

	// Cache more lines than max
	for i := uint32(0); i < 20; i++ {
		cache.GetLine(i, "line")
	}

	stats := cache.Stats()
	if stats.Size > 10 {
		t.Errorf("Size = %d, should be <= 10 after eviction", stats.Size)
	}
	if stats.Evictions == 0 {
		t.Error("Should have evicted entries")
	}
}

func TestCacheWithDirtyTracker(t *testing.T) {
	cache := newTestCache()
	tracker := dirty.NewTracker(80, 24)
	cache.SetDirtyTracker(tracker)

	// Initially not dirty
	tracker.Clear()
	if cache.IsDirty() {
		t.Error("Should not be dirty initially")
	}

	// Invalidate should mark dirty
	cache.Invalidate(5)
	if !cache.IsDirty() {
		t.Error("Should be dirty after invalidation")
	}

	dirtyLines := cache.DirtyLines()
	found := false
	for _, l := range dirtyLines {
		if l == 5 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Line 5 should be in dirty lines")
	}
}

func TestCacheDirtyOperations(t *testing.T) {
	cache := newTestCache()
	tracker := dirty.NewTracker(80, 24)
	cache.SetDirtyTracker(tracker)

	t.Run("ClearDirty", func(t *testing.T) {
		cache.Invalidate(5)
		cache.ClearDirty()
		if cache.IsDirty() {
			t.Error("Should not be dirty after ClearDirty")
		}
	})

	t.Run("NeedsFullRedraw", func(t *testing.T) {
		cache.InvalidateAll()
		if !cache.NeedsFullRedraw() {
			t.Error("Should need full redraw after InvalidateAll")
		}
	})
}

func TestCacheStyleResolver(t *testing.T) {
	cache := newTestCache()

	resolver := cache.StyleResolver()
	if resolver == nil {
		t.Fatal("StyleResolver returned nil")
	}

	// Should be able to configure it
	resolver.SetBaseStyle(core.NewStyle(core.ColorFromRGB(200, 200, 200)))
}

func TestCachePrefetchLines(t *testing.T) {
	cache := newTestCache()

	getText := func(line uint32) string {
		if line < 100 {
			return "test line"
		}
		return ""
	}

	cache.PrefetchLines(50, getText)

	stats := cache.Stats()
	if stats.Size == 0 {
		t.Error("Prefetch should have cached some lines")
	}
}

func TestCacheStats(t *testing.T) {
	cache := newTestCache()

	// Generate some activity
	cache.GetLine(0, "hello")
	cache.GetLine(0, "hello") // hit
	cache.GetLine(1, "world") // miss

	stats := cache.Stats()

	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Misses = %d, want 2", stats.Misses)
	}
	if stats.Size != 2 {
		t.Errorf("Size = %d, want 2", stats.Size)
	}
	if stats.HitRate < 0.3 || stats.HitRate > 0.4 {
		t.Errorf("HitRate = %f, want ~0.33", stats.HitRate)
	}
}

func TestHashContent(t *testing.T) {
	// Same content should have same hash
	h1 := hashContent("hello")
	h2 := hashContent("hello")
	if h1 != h2 {
		t.Error("Same content should have same hash")
	}

	// Different content should have different hash
	h3 := hashContent("world")
	if h1 == h3 {
		t.Error("Different content should have different hash")
	}

	// Empty string should have a hash
	h4 := hashContent("")
	if h4 == 0 {
		t.Error("Empty string should have non-zero hash")
	}
}

func TestCacheGetLineLayout(t *testing.T) {
	cache := newTestCache()

	layout := cache.GetLineLayout(0, "hello\tworld")
	if layout == nil {
		t.Fatal("GetLineLayout returned nil")
	}

	// Should have tab expansion
	if len(layout.Cells) < 12 {
		t.Errorf("Layout should have tab expansion, got %d cells", len(layout.Cells))
	}
}
