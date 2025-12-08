// Package linecache provides an enhanced line cache that integrates dirty tracking
// and style resolution for efficient incremental rendering.
package linecache

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/keystorm/internal/renderer/core"
	"github.com/dshills/keystorm/internal/renderer/dirty"
	"github.com/dshills/keystorm/internal/renderer/layout"
	"github.com/dshills/keystorm/internal/renderer/style"
)

// HighlightSource provides syntax highlighting spans for lines.
type HighlightSource interface {
	// HighlightsForLine returns style spans for the given line.
	HighlightsForLine(line uint32) []style.Span
}

// OverlaySource provides overlay spans (ghost text, diagnostics, etc.) for lines.
type OverlaySource interface {
	// OverlaysForLine returns overlay spans for the given line.
	OverlaysForLine(line uint32) []style.Span
}

// SelectionSource provides selection information.
type SelectionSource interface {
	// SelectionSpansForLine returns selection spans for the given line.
	SelectionSpansForLine(line uint32) []style.Span
}

// CachedLine represents a fully rendered line with styled cells.
type CachedLine struct {
	// BufferLine is the buffer line number.
	BufferLine uint32

	// Cells contains the styled cells ready for rendering.
	Cells []core.Cell

	// Layout contains the layout information.
	Layout *layout.LineLayout

	// Version tracks the cache version for invalidation.
	Version uint64

	// LastAccess tracks when this entry was last used.
	LastAccess time.Time

	// ContentHash is the hash of the source text.
	ContentHash uint64

	// LayoutOnly indicates if only layout is cached (no styling).
	LayoutOnly bool
}

// Config configures the line cache behavior.
type Config struct {
	// MaxCachedLines is the maximum number of lines to cache.
	MaxCachedLines int

	// CacheStyledCells enables caching of styled cells (more memory, faster render).
	CacheStyledCells bool

	// PrefetchLines is the number of lines to prefetch around visible region.
	PrefetchLines int

	// EvictionBatchSize is the number of entries to evict at once.
	EvictionBatchSize int
}

// DefaultConfig returns the default cache configuration.
func DefaultConfig() Config {
	return Config{
		MaxCachedLines:    2000,
		CacheStyledCells:  true,
		PrefetchLines:     20,
		EvictionBatchSize: 50,
	}
}

// Cache provides efficient line caching with dirty tracking and style resolution.
type Cache struct {
	mu sync.RWMutex

	// Configuration
	config Config

	// Line cache entries
	entries map[uint32]*CachedLine

	// Components
	layoutCache   *layout.LineCache
	styleResolver *style.Resolver
	dirtyTracker  *dirty.Tracker

	// Sources
	highlightSrc HighlightSource
	overlaySrc   OverlaySource
	selectionSrc SelectionSource

	// Cache versioning
	version uint64

	// Stats (atomic for thread-safe access without holding locks)
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

// New creates a new line cache.
func New(layoutCache *layout.LineCache, config Config) *Cache {
	if config.MaxCachedLines <= 0 {
		config.MaxCachedLines = 2000
	}
	if config.EvictionBatchSize <= 0 {
		config.EvictionBatchSize = 50
	}

	return &Cache{
		config:        config,
		entries:       make(map[uint32]*CachedLine),
		layoutCache:   layoutCache,
		styleResolver: style.NewResolver(),
	}
}

// SetDirtyTracker sets the dirty tracker for change notification.
func (c *Cache) SetDirtyTracker(tracker *dirty.Tracker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirtyTracker = tracker
}

// SetHighlightSource sets the syntax highlighting source.
func (c *Cache) SetHighlightSource(src HighlightSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.highlightSrc = src
	c.version++
}

// SetOverlaySource sets the overlay source.
func (c *Cache) SetOverlaySource(src OverlaySource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.overlaySrc = src
	c.version++
}

// SetSelectionSource sets the selection source.
func (c *Cache) SetSelectionSource(src SelectionSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.selectionSrc = src
	// Don't increment version - selections are volatile
}

// StyleResolver returns the style resolver for configuration.
// Note: The style.Resolver is thread-safe and can be used concurrently.
func (c *Cache) StyleResolver() *style.Resolver {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.styleResolver
}

// GetLine retrieves or computes the styled cells for a line.
func (c *Cache) GetLine(line uint32, text string) *CachedLine {
	contentHash := hashContent(text)

	// Check cache under read lock first (read-only check)
	c.mu.RLock()
	if entry, ok := c.entries[line]; ok {
		if entry.ContentHash == contentHash && entry.Version == c.version && !entry.LayoutOnly {
			// Found valid cache entry - upgrade to write lock to update LastAccess
			c.mu.RUnlock()
			c.mu.Lock()
			// Re-check entry still exists and is valid after acquiring write lock
			if entry, ok := c.entries[line]; ok {
				if entry.ContentHash == contentHash && entry.Version == c.version && !entry.LayoutOnly {
					entry.LastAccess = time.Now()
					c.hits.Add(1)
					c.mu.Unlock()
					return entry
				}
			}
			// Entry was invalidated between locks, continue to compute
			c.mu.Unlock()
		}
	}
	// Capture state needed for computation
	version := c.version
	highlightSrc := c.highlightSrc
	overlaySrc := c.overlaySrc
	styleResolver := c.styleResolver
	c.mu.RUnlock()

	// Get layout outside the lock (layoutCache has its own synchronization)
	lineLayout := c.layoutCache.Get(line, text)

	// Handle nil layout
	if lineLayout == nil {
		lineLayout = &layout.LineLayout{
			BufferLine: line,
			Cells:      nil,
		}
	}

	// Build styled cells outside the lock
	cells := buildStyledCellsUnlocked(line, lineLayout, highlightSrc, overlaySrc, styleResolver)

	// Acquire write lock to update cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: entry may have been added by another goroutine
	if entry, ok := c.entries[line]; ok {
		if entry.ContentHash == contentHash && entry.Version == version && !entry.LayoutOnly {
			entry.LastAccess = time.Now()
			c.hits.Add(1)
			return entry
		}
	}

	c.misses.Add(1)

	// Create cache entry
	entry := &CachedLine{
		BufferLine:  line,
		Cells:       cells,
		Layout:      lineLayout,
		Version:     version,
		LastAccess:  time.Now(),
		ContentHash: contentHash,
		LayoutOnly:  false,
	}

	// Store in cache
	c.entries[line] = entry
	c.evictIfNeeded()

	return entry
}

// GetLineLayout retrieves just the layout without styling.
func (c *Cache) GetLineLayout(line uint32, text string) *layout.LineLayout {
	return c.layoutCache.Get(line, text)
}

// GetStyledCells retrieves styled cells for a line, using cache if available.
// Unlike GetLine, this also applies volatile styles like selection.
func (c *Cache) GetStyledCells(line uint32, text string) []core.Cell {
	cached := c.GetLine(line, text)

	// Capture selection source and resolver under lock, then release
	c.mu.RLock()
	selectionSrc := c.selectionSrc
	styleResolver := c.styleResolver
	c.mu.RUnlock()

	// Apply volatile selection styling outside the lock
	if selectionSrc != nil {
		selSpans := selectionSrc.SelectionSpansForLine(line)
		if len(selSpans) > 0 {
			// Make a copy to avoid modifying cached cells
			cells := make([]core.Cell, len(cached.Cells))
			copy(cells, cached.Cells)
			// ResolveLine is called outside the lock (style.Resolver is thread-safe)
			return styleResolver.ResolveLine(cells, selSpans)
		}
	}

	return cached.Cells
}

// buildStyledCellsUnlocked builds styled cells from a layout without holding the cache lock.
// This is a package-level function to avoid accidental lock access.
func buildStyledCellsUnlocked(line uint32, lineLayout *layout.LineLayout, highlightSrc HighlightSource, overlaySrc OverlaySource, styleResolver *style.Resolver) []core.Cell {
	// Handle nil or empty layout
	if lineLayout == nil || len(lineLayout.Cells) == 0 {
		return nil
	}

	// Start with layout cells
	cells := make([]core.Cell, len(lineLayout.Cells))
	for i, lc := range lineLayout.Cells {
		cells[i] = core.Cell{
			Rune:  lc.Rune,
			Width: lc.Width,
			Style: core.DefaultStyle(),
		}
	}

	// Collect all style spans
	var spans []style.Span

	// Add syntax highlighting spans
	if highlightSrc != nil {
		hlSpans := highlightSrc.HighlightsForLine(line)
		spans = append(spans, hlSpans...)
	}

	// Add overlay spans (ghost text, diagnostics, etc.)
	if overlaySrc != nil {
		overlaySpans := overlaySrc.OverlaysForLine(line)
		spans = append(spans, overlaySpans...)
	}

	// Apply all spans using style resolver
	if len(spans) > 0 && styleResolver != nil {
		cells = styleResolver.ResolveLine(cells, spans)
	}

	return cells
}

// Invalidate invalidates a specific line.
func (c *Cache) Invalidate(line uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, line)
	c.layoutCache.Invalidate(line)

	if c.dirtyTracker != nil {
		c.dirtyTracker.MarkLine(line)
	}
}

// InvalidateRange invalidates a range of lines.
func (c *Cache) InvalidateRange(startLine, endLine uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if startLine > endLine {
		return
	}

	for line := startLine; line <= endLine; line++ {
		delete(c.entries, line)
		if line == ^uint32(0) {
			break
		}
	}
	c.layoutCache.InvalidateRange(startLine, endLine)

	if c.dirtyTracker != nil {
		c.dirtyTracker.MarkLines(startLine, endLine)
	}
}

// InvalidateFrom invalidates all lines from a given line onwards.
func (c *Cache) InvalidateFrom(line uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for l := range c.entries {
		if l >= line {
			delete(c.entries, l)
		}
	}
	c.layoutCache.InvalidateFrom(line)

	if c.dirtyTracker != nil {
		c.dirtyTracker.MarkChange(dirty.Change{
			Type:   dirty.ChangeDelete,
			Region: dirty.NewSingleLine(line),
		})
	}
}

// InvalidateStyles invalidates all styled cache entries without affecting layouts.
// Useful when syntax highlighting or themes change.
func (c *Cache) InvalidateStyles() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.version++

	if c.dirtyTracker != nil {
		c.dirtyTracker.MarkFullRedraw()
	}
}

// InvalidateAll clears the entire cache.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[uint32]*CachedLine)
	c.layoutCache.InvalidateAll()
	c.version++

	if c.dirtyTracker != nil {
		c.dirtyTracker.MarkFullRedraw()
	}
}

// ShiftLines adjusts line numbers when lines are inserted or deleted.
func (c *Cache) ShiftLines(fromLine uint32, delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if delta == 0 {
		return
	}

	// Collect entries that need to move
	toMove := make(map[uint32]*CachedLine)
	for line, entry := range c.entries {
		if line >= fromLine {
			delete(c.entries, line)
			newLine := int64(line) + int64(delta)
			if newLine >= 0 && newLine <= int64(^uint32(0)) {
				entry.BufferLine = uint32(newLine)
				toMove[uint32(newLine)] = entry
			}
		}
	}

	// Re-insert moved entries
	for line, entry := range toMove {
		c.entries[line] = entry
	}

	c.layoutCache.ShiftLines(fromLine, delta)

	if c.dirtyTracker != nil {
		if delta > 0 {
			c.dirtyTracker.MarkChange(dirty.Change{
				Type:   dirty.ChangeInsert,
				Region: dirty.NewSingleLine(fromLine),
			})
		} else {
			c.dirtyTracker.MarkChange(dirty.Change{
				Type:   dirty.ChangeDelete,
				Region: dirty.NewSingleLine(fromLine),
			})
		}
	}
}

// evictIfNeeded evicts entries if the cache is too large.
func (c *Cache) evictIfNeeded() {
	if len(c.entries) <= c.config.MaxCachedLines {
		return
	}

	// Find oldest entries
	type entryInfo struct {
		line   uint32
		access time.Time
	}

	entries := make([]entryInfo, 0, len(c.entries))
	for line, entry := range c.entries {
		entries = append(entries, entryInfo{line, entry.LastAccess})
	}

	// Sort by access time (oldest first) using standard library
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].access.Before(entries[j].access)
	})

	// Evict oldest entries
	toEvict := len(c.entries) - c.config.MaxCachedLines + c.config.EvictionBatchSize
	if toEvict > len(entries) {
		toEvict = len(entries)
	}

	for i := 0; i < toEvict; i++ {
		delete(c.entries, entries[i].line)
	}
	c.evictions.Add(uint64(toEvict))
}

// IsDirty returns true if any lines need redrawing.
// Note: Returns true if no dirty tracker is configured, assuming the conservative
// approach that everything needs redrawing when tracking is unavailable.
func (c *Cache) IsDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.dirtyTracker == nil {
		return true
	}
	return c.dirtyTracker.IsDirty()
}

// DirtyLines returns the list of lines that need redrawing.
func (c *Cache) DirtyLines() []uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.dirtyTracker == nil {
		return nil
	}
	return c.dirtyTracker.DirtyLines()
}

// ClearDirty clears the dirty tracking state.
func (c *Cache) ClearDirty() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dirtyTracker != nil {
		c.dirtyTracker.Clear()
	}
}

// NeedsFullRedraw returns true if a full screen redraw is needed.
// Note: Returns true if no dirty tracker is configured, assuming the conservative
// approach that a full redraw is needed when tracking is unavailable.
func (c *Cache) NeedsFullRedraw() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.dirtyTracker == nil {
		return true
	}
	return c.dirtyTracker.NeedsFullRedraw()
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
	// Load atomic counters first (no lock needed for atomics)
	hits := c.hits.Load()
	misses := c.misses.Load()
	evictions := c.evictions.Load()

	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	// Lock only for non-atomic fields
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:             len(c.entries),
		MaxSize:          c.config.MaxCachedLines,
		Hits:             hits,
		Misses:           misses,
		Evictions:        evictions,
		HitRate:          hitRate,
		Version:          c.version,
		LayoutCacheStats: c.layoutCache.Stats(),
	}
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Size             int
	MaxSize          int
	Hits             uint64
	Misses           uint64
	Evictions        uint64
	HitRate          float64
	Version          uint64
	LayoutCacheStats layout.CacheStats
}

// PrefetchLines pre-caches lines around the visible region.
func (c *Cache) PrefetchLines(centerLine uint32, text func(line uint32) string) {
	c.mu.RLock()
	prefetchCount := c.config.PrefetchLines
	c.mu.RUnlock()

	if prefetchCount <= 0 {
		return
	}

	// Prefetch lines before and after center
	startLine := uint32(0)
	if centerLine > uint32(prefetchCount) {
		startLine = centerLine - uint32(prefetchCount)
	}

	// Guard against uint32 overflow
	endLine := centerLine + uint32(prefetchCount)
	if endLine < centerLine {
		// Overflow occurred, clamp to max
		endLine = math.MaxUint32
	}

	for line := startLine; line <= endLine; line++ {
		lineText := text(line)
		if lineText != "" {
			c.GetLine(line, lineText)
		}
		if line == math.MaxUint32 {
			break
		}
	}
}

// hashContent computes a hash of the content.
func hashContent(s string) uint64 {
	// FNV-1a hash
	var hash uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= 1099511628211
	}
	return hash
}
