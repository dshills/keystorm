package fuzzy

import (
	"container/list"
	"sync"
)

// Cache provides LRU caching for fuzzy match results.
// It is safe for concurrent use.
type Cache struct {
	mu      sync.RWMutex
	maxSize int
	items   map[string]*list.Element
	lru     *list.List
}

// cacheEntry holds a cached query result.
type cacheEntry struct {
	query   string
	results []Result
}

// NewCache creates a new LRU cache with the given maximum size.
func NewCache(maxSize int) *Cache {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &Cache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		lru:     list.New(),
	}
}

// Get retrieves cached results for a query.
// Returns nil if not found.
func (c *Cache) Get(query string) []Result {
	// First check with read lock for cache misses (common case)
	c.mu.RLock()
	_, ok := c.items[query]
	if !ok {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	// Cache hit - need write lock to update LRU order
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check in case entry was evicted between locks
	elem, ok := c.items[query]
	if !ok {
		return nil
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(elem)

	entry := elem.Value.(*cacheEntry) //nolint:errcheck // list only contains *cacheEntry

	// Return a copy to prevent external modification
	results := make([]Result, len(entry.results))
	copy(results, entry.results)
	return results
}

// Set stores results for a query in the cache.
func (c *Cache) Set(query string, results []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if elem, ok := c.items[query]; ok {
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry) //nolint:errcheck // list only contains *cacheEntry
		entry.results = c.copyResults(results)
		return
	}

	// Evict oldest if at capacity
	if c.lru.Len() >= c.maxSize {
		c.evictOldest()
	}

	// Add new entry
	entry := &cacheEntry{
		query:   query,
		results: c.copyResults(results),
	}
	elem := c.lru.PushFront(entry)
	c.items[query] = elem
}

// Delete removes a specific query from the cache.
func (c *Cache) Delete(query string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[query]; ok {
		c.removeElement(elem)
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lru.Init()
}

// Len returns the number of cached entries.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// evictOldest removes the least recently used entry.
// Must be called with lock held.
func (c *Cache) evictOldest() {
	elem := c.lru.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes an element from the cache.
// Must be called with lock held.
func (c *Cache) removeElement(elem *list.Element) {
	c.lru.Remove(elem)
	entry := elem.Value.(*cacheEntry) //nolint:errcheck // list only contains *cacheEntry
	delete(c.items, entry.query)
}

// copyResults creates a deep copy of results.
func (c *Cache) copyResults(results []Result) []Result {
	copied := make([]Result, len(results))
	for i, r := range results {
		copied[i] = Result{
			Item:  r.Item,
			Score: r.Score,
		}
		if r.Matches != nil {
			copied[i].Matches = make([]int, len(r.Matches))
			copy(copied[i].Matches, r.Matches)
		}
	}
	return copied
}

// PrefixCache extends Cache with prefix-aware invalidation.
// When a query is set, it also invalidates any cached queries
// that are prefixes of the new query (since results may differ).
type PrefixCache struct {
	*Cache
}

// NewPrefixCache creates a prefix-aware cache.
func NewPrefixCache(maxSize int) *PrefixCache {
	return &PrefixCache{
		Cache: NewCache(maxSize),
	}
}

// Set stores results and invalidates prefix queries.
func (c *PrefixCache) Set(query string, results []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Invalidate any cached queries that are prefixes of this query
	// This ensures incremental typing gets fresh results
	for cachedQuery, elem := range c.items {
		if len(cachedQuery) < len(query) && query[:len(cachedQuery)] == cachedQuery {
			c.lru.Remove(elem)
			delete(c.items, cachedQuery)
		}
	}

	// Check if already exists
	if elem, ok := c.items[query]; ok {
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry) //nolint:errcheck // list only contains *cacheEntry
		entry.results = c.copyResults(results)
		return
	}

	// Evict oldest if at capacity
	if c.lru.Len() >= c.maxSize {
		c.evictOldest()
	}

	// Add new entry
	entry := &cacheEntry{
		query:   query,
		results: c.copyResults(results),
	}
	elem := c.lru.PushFront(entry)
	c.items[query] = elem
}
