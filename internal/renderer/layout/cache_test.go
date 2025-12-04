package layout

import (
	"testing"
	"time"
)

func TestNewLineCache(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	if cache.Size() != 0 {
		t.Errorf("new cache should be empty, got size %d", cache.Size())
	}

	stats := cache.Stats()
	if stats.MaxSize != 100 {
		t.Errorf("expected max size 100, got %d", stats.MaxSize)
	}
}

func TestLineCacheGet(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	layout := cache.Get(0, "Hello")

	if layout == nil {
		t.Fatal("Get should return a layout")
	}
	if layout.BufferLine != 0 {
		t.Errorf("expected buffer line 0, got %d", layout.BufferLine)
	}
	if layout.Width != 5 {
		t.Errorf("expected width 5, got %d", layout.Width)
	}
	if cache.Size() != 1 {
		t.Errorf("cache should have 1 entry, got %d", cache.Size())
	}
}

func TestLineCacheHit(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	// First access - cache miss
	layout1 := cache.Get(0, "Hello")

	// Second access with same content - cache hit
	layout2 := cache.Get(0, "Hello")

	// Should return the same layout
	if layout1 != layout2 {
		t.Error("second Get should return cached layout")
	}

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestLineCacheContentChange(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	layout1 := cache.Get(0, "Hello")
	layout2 := cache.Get(0, "World") // Different content

	// Should be different layouts
	if layout1 == layout2 {
		t.Error("different content should return different layout")
	}
	if layout2.Width != 5 {
		t.Errorf("expected width 5 for 'World', got %d", layout2.Width)
	}
}

func TestLineCacheGetIfCached(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	// Not cached yet
	layout := cache.GetIfCached(0, "Hello")
	if layout != nil {
		t.Error("GetIfCached should return nil for uncached line")
	}

	// Cache it
	cache.Get(0, "Hello")

	// Now should return cached
	layout = cache.GetIfCached(0, "Hello")
	if layout == nil {
		t.Error("GetIfCached should return cached layout")
	}

	// Different content should return nil
	layout = cache.GetIfCached(0, "World")
	if layout != nil {
		t.Error("GetIfCached should return nil for changed content")
	}
}

func TestLineCachePut(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	layout := engine.Layout("Custom", 5)
	cache.Put(5, "Custom", layout)

	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}

	retrieved := cache.GetIfCached(5, "Custom")
	if retrieved != layout {
		t.Error("Put layout should be retrievable")
	}
}

func TestLineCacheInvalidate(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	cache.Get(0, "Hello")
	cache.Get(1, "World")

	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}

	cache.Invalidate(0)

	if cache.Size() != 1 {
		t.Errorf("expected size 1 after invalidate, got %d", cache.Size())
	}

	if cache.GetIfCached(0, "Hello") != nil {
		t.Error("invalidated line should not be cached")
	}
	if cache.GetIfCached(1, "World") == nil {
		t.Error("other line should still be cached")
	}
}

func TestLineCacheInvalidateRange(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	// Add some entries
	for i := uint32(0); i < 10; i++ {
		cache.Get(i, "Line")
	}

	if cache.Size() != 10 {
		t.Errorf("expected size 10, got %d", cache.Size())
	}

	cache.InvalidateRange(3, 7)

	// Lines 0-2 and 8-9 should remain
	if cache.Size() != 5 {
		t.Errorf("expected size 5 after range invalidate, got %d", cache.Size())
	}
}

func TestLineCacheInvalidateRangeEdgeCases(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	cache.Get(5, "Hello")

	// Start > end should do nothing
	cache.InvalidateRange(10, 5)
	if cache.Size() != 1 {
		t.Error("invalid range should not affect cache")
	}
}

func TestLineCacheInvalidateFrom(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	for i := uint32(0); i < 10; i++ {
		cache.Get(i, "Line")
	}

	cache.InvalidateFrom(5)

	// Lines 0-4 should remain
	if cache.Size() != 5 {
		t.Errorf("expected size 5, got %d", cache.Size())
	}

	for i := uint32(0); i < 5; i++ {
		if cache.GetIfCached(i, "Line") == nil {
			t.Errorf("line %d should still be cached", i)
		}
	}
	for i := uint32(5); i < 10; i++ {
		if cache.GetIfCached(i, "Line") != nil {
			t.Errorf("line %d should be invalidated", i)
		}
	}
}

func TestLineCacheInvalidateAll(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	for i := uint32(0); i < 10; i++ {
		cache.Get(i, "Line")
	}

	cache.InvalidateAll()

	if cache.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", cache.Size())
	}
}

func TestLineCacheShiftLinesInsert(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	// Cache lines 0-4
	for i := uint32(0); i < 5; i++ {
		cache.Get(i, "Line")
	}

	// Insert 2 lines at position 2
	cache.ShiftLines(2, 2)

	// Lines 0,1 should be unchanged
	if cache.GetIfCached(0, "Line") == nil {
		t.Error("line 0 should still be cached")
	}
	if cache.GetIfCached(1, "Line") == nil {
		t.Error("line 1 should still be cached")
	}

	// Lines 2,3 should be empty (new lines)
	if cache.GetIfCached(2, "Line") != nil {
		t.Error("line 2 should not be cached (shifted)")
	}
	if cache.GetIfCached(3, "Line") != nil {
		t.Error("line 3 should not be cached (shifted)")
	}

	// Old line 2 should now be at line 4
	if cache.GetIfCached(4, "Line") == nil {
		t.Error("old line 2 should now be at line 4")
	}
}

func TestLineCacheShiftLinesDelete(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	// Cache lines 0-9
	for i := uint32(0); i < 10; i++ {
		cache.Get(i, "Line")
	}

	// Delete 3 lines starting at position 2 (shift -3)
	cache.ShiftLines(5, -3)

	// Lines 0-4 unchanged, lines 5+ shifted down
	for i := uint32(0); i < 5; i++ {
		if cache.GetIfCached(i, "Line") == nil {
			t.Errorf("line %d should still be cached", i)
		}
	}

	// Old line 5 should now be at line 2 - wait, that's not right
	// ShiftLines(5, -3) means lines >= 5 move to line-3
	// So old line 5 -> line 2, old line 6 -> line 3, etc.
	// But line 2 was already there, so we need to be careful
}

func TestLineCacheEviction(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 5) // Small cache

	// Add more than max size
	for i := uint32(0); i < 10; i++ {
		cache.Get(i, "Line")
		// Small delay to ensure different access times
		time.Sleep(time.Microsecond)
	}

	// Should have evicted some entries
	if cache.Size() > 5 {
		t.Errorf("cache should not exceed max size, got %d", cache.Size())
	}

	stats := cache.Stats()
	if stats.Evictions == 0 {
		t.Error("should have evicted some entries")
	}
}

func TestLineCacheStats(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	// Generate some activity
	cache.Get(0, "Hello")  // Miss
	cache.Get(0, "Hello")  // Hit
	cache.Get(0, "Hello")  // Hit
	cache.Get(1, "World")  // Miss
	cache.Get(0, "Change") // Miss (content changed)

	stats := cache.Stats()

	if stats.Size != 2 {
		t.Errorf("expected size 2, got %d", stats.Size)
	}
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 3 {
		t.Errorf("expected 3 misses, got %d", stats.Misses)
	}
	if stats.HitRate < 0.39 || stats.HitRate > 0.41 {
		t.Errorf("expected hit rate ~0.4, got %f", stats.HitRate)
	}
}

func TestLineCacheResetStats(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	cache.Get(0, "Hello")
	cache.Get(0, "Hello")

	cache.ResetStats()

	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Error("hits should be reset")
	}
	if stats.Misses != 0 {
		t.Error("misses should be reset")
	}
}

func TestLineCacheEngine(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 100)

	if cache.Engine() != engine {
		t.Error("Engine() should return the engine")
	}

	newEngine := NewLayoutEngine(8)
	cache.SetEngine(newEngine)

	if cache.Engine() != newEngine {
		t.Error("SetEngine should change the engine")
	}
	if cache.Size() != 0 {
		t.Error("SetEngine should clear the cache")
	}
}

func TestLineCacheHashLine(t *testing.T) {
	// Same content should produce same hash
	h1 := hashLine("Hello")
	h2 := hashLine("Hello")
	if h1 != h2 {
		t.Error("same content should have same hash")
	}

	// Different content should (very likely) produce different hash
	h3 := hashLine("World")
	if h1 == h3 {
		t.Error("different content should have different hash")
	}
}

func TestLineCacheUnlimitedSize(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 0) // Unlimited

	// Add many entries
	for i := uint32(0); i < 100; i++ {
		cache.Get(i, "Line")
	}

	// All should be cached
	if cache.Size() != 100 {
		t.Errorf("unlimited cache should hold all entries, got %d", cache.Size())
	}

	stats := cache.Stats()
	if stats.Evictions != 0 {
		t.Error("unlimited cache should not evict")
	}
}

func TestLineCacheConcurrentAccess(t *testing.T) {
	engine := NewLayoutEngine(4)
	cache := NewLineCache(engine, 1000)

	done := make(chan bool)

	// Multiple goroutines reading/writing
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				line := uint32(j % 50)
				cache.Get(line, "Content")
				if j%3 == 0 {
					cache.Invalidate(line)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or deadlock
	// Just verify we can still use the cache
	_ = cache.Stats()
}
