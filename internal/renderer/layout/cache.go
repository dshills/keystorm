package layout

import (
	"hash/fnv"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// LineCache caches computed line layouts with LRU eviction.
type LineCache struct {
	mu        sync.RWMutex
	entries   map[uint32]*cacheEntry
	engine    *LayoutEngine
	maxSize   int
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

type cacheEntry struct {
	layout     *LineLayout
	lineHash   uint64    // Hash of line content for validation
	lastAccess time.Time // For LRU eviction
}

// NewLineCache creates a new line cache.
// maxSize is the maximum number of lines to cache (0 = unlimited, not recommended).
func NewLineCache(engine *LayoutEngine, maxSize int) *LineCache {
	if maxSize < 0 {
		maxSize = 0
	}
	return &LineCache{
		entries: make(map[uint32]*cacheEntry),
		engine:  engine,
		maxSize: maxSize,
	}
}

// Get retrieves or computes the layout for a line.
// The text parameter is the current line content used for validation.
func (c *LineCache) Get(line uint32, text string) *LineLayout {
	hash := hashLine(text)

	// Fast path: check if cached with read lock
	c.mu.RLock()
	entry, ok := c.entries[line]
	if ok && entry.lineHash == hash {
		c.mu.RUnlock()
		// Update access time with write lock and re-verify entry
		c.mu.Lock()
		if e, ok := c.entries[line]; ok && e.lineHash == hash {
			e.lastAccess = time.Now()
			layout := e.layout
			c.mu.Unlock()
			c.hits.Add(1)
			return layout
		}
		c.mu.Unlock()
		// Entry was removed or changed between locks, treat as miss
	} else {
		c.mu.RUnlock()
	}

	c.misses.Add(1)

	// Compute layout
	layout := c.engine.Layout(text, line)

	// Store in cache
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[line] = &cacheEntry{
		layout:     layout,
		lineHash:   hash,
		lastAccess: time.Now(),
	}

	// Evict if too large
	if c.maxSize > 0 && len(c.entries) > c.maxSize {
		c.evict()
	}

	return layout
}

// GetIfCached returns the cached layout if available and valid.
// Returns nil if not cached or content has changed.
func (c *LineCache) GetIfCached(line uint32, text string) *LineLayout {
	hash := hashLine(text)

	c.mu.RLock()
	entry, ok := c.entries[line]
	if ok && entry.lineHash == hash {
		c.mu.RUnlock()
		// Update access time with write lock and re-verify entry
		c.mu.Lock()
		if e, ok := c.entries[line]; ok && e.lineHash == hash {
			e.lastAccess = time.Now()
			layout := e.layout
			c.mu.Unlock()
			return layout
		}
		c.mu.Unlock()
		// Entry was removed or changed between locks
		return nil
	}
	c.mu.RUnlock()
	return nil
}

// Put stores a pre-computed layout in the cache.
func (c *LineCache) Put(line uint32, text string, layout *LineLayout) {
	hash := hashLine(text)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[line] = &cacheEntry{
		layout:     layout,
		lineHash:   hash,
		lastAccess: time.Now(),
	}

	if c.maxSize > 0 && len(c.entries) > c.maxSize {
		c.evict()
	}
}

// Invalidate marks a line as needing re-layout.
func (c *LineCache) Invalidate(line uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, line)
}

// InvalidateRange marks a range of lines as needing re-layout.
// Both startLine and endLine are inclusive.
func (c *LineCache) InvalidateRange(startLine, endLine uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Protect against overflow when startLine > endLine
	if startLine > endLine {
		return
	}

	for line := startLine; line <= endLine; line++ {
		delete(c.entries, line)
		// Prevent overflow on last iteration
		if line == ^uint32(0) {
			break
		}
	}
}

// InvalidateFrom marks all lines from startLine onwards as invalid.
// Useful when lines are inserted/deleted and all following lines shift.
func (c *LineCache) InvalidateFrom(startLine uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for line := range c.entries {
		if line >= startLine {
			delete(c.entries, line)
		}
	}
}

// InvalidateAll clears the entire cache.
func (c *LineCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[uint32]*cacheEntry)
}

// ShiftLines adjusts line numbers when lines are inserted or deleted.
// fromLine is the first affected line.
// delta is positive for insertion, negative for deletion.
func (c *LineCache) ShiftLines(fromLine uint32, delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if delta == 0 {
		return
	}

	// Collect entries that need to move
	toMove := make(map[uint32]*cacheEntry)
	for line, entry := range c.entries {
		if line >= fromLine {
			delete(c.entries, line)
			newLine := int64(line) + int64(delta)
			if newLine >= 0 && newLine <= int64(math.MaxUint32) {
				entry.layout.BufferLine = uint32(newLine)
				toMove[uint32(newLine)] = entry
			}
		}
	}

	// Re-insert moved entries
	for line, entry := range toMove {
		c.entries[line] = entry
	}
}

// evict removes the least recently used entries until under maxSize.
// Must be called with write lock held.
func (c *LineCache) evict() {
	if c.maxSize <= 0 || len(c.entries) <= c.maxSize {
		return
	}

	// Find the oldest entries
	type lineTime struct {
		line uint32
		time time.Time
	}

	entries := make([]lineTime, 0, len(c.entries))
	for line, entry := range c.entries {
		entries = append(entries, lineTime{line, entry.lastAccess})
	}

	// Simple sort by time (insertion sort is fine for small N)
	for i := 1; i < len(entries); i++ {
		j := i
		for j > 0 && entries[j].time.Before(entries[j-1].time) {
			entries[j], entries[j-1] = entries[j-1], entries[j]
			j--
		}
	}

	// Remove oldest entries until we're at maxSize
	toRemove := len(entries) - c.maxSize
	if toRemove > 0 {
		for i := 0; i < toRemove; i++ {
			delete(c.entries, entries[i].line)
		}
		c.evictions.Add(uint64(toRemove))
	}
}

// Size returns the number of cached entries.
func (c *LineCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stats returns cache statistics.
func (c *LineCache) Stats() CacheStats {
	c.mu.RLock()
	size := len(c.entries)
	c.mu.RUnlock()

	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return CacheStats{
		Size:      size,
		MaxSize:   c.maxSize,
		Hits:      hits,
		Misses:    misses,
		Evictions: c.evictions.Load(),
		HitRate:   hitRate,
	}
}

// ResetStats resets the cache statistics counters.
func (c *LineCache) ResetStats() {
	c.hits.Store(0)
	c.misses.Store(0)
	c.evictions.Store(0)
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Size      int     // Current number of entries
	MaxSize   int     // Maximum entries allowed
	Hits      uint64  // Number of cache hits
	Misses    uint64  // Number of cache misses
	Evictions uint64  // Number of evicted entries
	HitRate   float64 // Hit rate (0.0 - 1.0)
}

// hashLine computes a hash of line content using FNV-1a.
// Includes string length to reduce collision probability.
func hashLine(s string) uint64 {
	h := fnv.New64a()
	// Include length as first 8 bytes to reduce collisions
	length := uint64(len(s))
	h.Write([]byte{
		byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24),
		byte(length >> 32), byte(length >> 40), byte(length >> 48), byte(length >> 56),
	})
	h.Write([]byte(s))
	return h.Sum64()
}

// Engine returns the layout engine used by this cache.
func (c *LineCache) Engine() *LayoutEngine {
	return c.engine
}

// SetEngine replaces the layout engine and invalidates the cache.
func (c *LineCache) SetEngine(engine *LayoutEngine) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.engine = engine
	c.entries = make(map[uint32]*cacheEntry)
}
