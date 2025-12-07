package integration

import (
	"sync"
	"time"
)

// Debouncer provides event debouncing to prevent excessive calls.
//
// It groups rapid successive calls into a single call after a quiet period.
// This is useful for operations like git status queries or file change events.
//
// Thread-safety: All methods are safe for concurrent use. The callback is
// guaranteed to not be called concurrently with itself from the debouncer.
type Debouncer struct {
	mu       sync.Mutex
	delay    time.Duration
	timer    *time.Timer
	pending  bool
	seq      uint64 // sequence number to detect stale callbacks
	callback func()
}

// NewDebouncer creates a new debouncer with the specified delay.
//
// The callback will be invoked after no new calls have been made
// for at least 'delay' duration.
func NewDebouncer(delay time.Duration, callback func()) *Debouncer {
	return &Debouncer{
		delay:    delay,
		callback: callback,
	}
}

// Call schedules the callback to run after the debounce delay.
//
// If called multiple times within the delay period, only the last
// call's timing is used - the callback fires once after the final
// quiet period.
func (d *Debouncer) Call() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pending = true
	d.seq++
	currentSeq := d.seq

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		// Only execute if this is still the current scheduled callback
		// and we're still pending
		if d.pending && d.seq == currentSeq && d.callback != nil {
			d.pending = false
			d.mu.Unlock()
			d.callback()
		} else {
			d.mu.Unlock()
		}
	})
}

// CallImmediate runs the callback immediately if there's a pending call,
// canceling any scheduled debounced call.
func (d *Debouncer) CallImmediate() {
	d.mu.Lock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	// Increment seq to invalidate any running timer callback
	d.seq++

	if d.pending && d.callback != nil {
		d.pending = false
		d.mu.Unlock()
		d.callback()
	} else {
		d.mu.Unlock()
	}
}

// Cancel cancels any pending debounced call.
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	// Increment seq to invalidate any running timer callback
	d.seq++
	d.pending = false
}

// IsPending returns true if there's a pending debounced call.
func (d *Debouncer) IsPending() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pending
}

// Reset resets the debouncer, canceling any pending call.
func (d *Debouncer) Reset() {
	d.Cancel()
}

// Throttler provides rate limiting that allows at most one call per interval.
//
// Unlike Debouncer which waits for a quiet period, Throttler ensures
// the callback runs at most once per interval, executing immediately
// on the first call after the interval expires.
//
// Thread-safety: All methods are safe for concurrent use.
type Throttler struct {
	mu       sync.Mutex
	interval time.Duration
	lastCall time.Time
	pending  bool
	seq      uint64 // sequence number to detect stale callbacks
	timer    *time.Timer
	callback func()
	leading  bool // If true, execute on leading edge
	trailing bool // If true, execute on trailing edge
}

// ThrottlerOption configures a Throttler.
type ThrottlerOption func(*Throttler)

// WithLeadingEdge configures the throttler to execute on the leading edge.
func WithLeadingEdge(leading bool) ThrottlerOption {
	return func(t *Throttler) {
		t.leading = leading
	}
}

// WithTrailingEdge configures the throttler to execute on the trailing edge.
func WithTrailingEdge(trailing bool) ThrottlerOption {
	return func(t *Throttler) {
		t.trailing = trailing
	}
}

// NewThrottler creates a new throttler with the specified interval.
//
// By default, it executes on both leading and trailing edges.
func NewThrottler(interval time.Duration, callback func(), opts ...ThrottlerOption) *Throttler {
	t := &Throttler{
		interval: interval,
		callback: callback,
		leading:  true,
		trailing: true,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Call attempts to run the callback, respecting the throttle interval.
func (t *Throttler) Call() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.lastCall)

	if elapsed >= t.interval {
		// Interval has passed, can execute
		if t.leading {
			t.lastCall = now
			go t.callback()
		} else {
			t.pending = true
			t.scheduleTrailingLocked()
		}
	} else {
		// Within interval, schedule trailing if enabled
		t.pending = true
		if t.trailing && t.timer == nil {
			remaining := t.interval - elapsed
			t.seq++
			currentSeq := t.seq
			t.timer = time.AfterFunc(remaining, func() {
				t.mu.Lock()
				if t.pending && t.seq == currentSeq {
					t.pending = false
					t.lastCall = time.Now()
					t.timer = nil
					t.mu.Unlock()
					t.callback()
				} else {
					t.timer = nil
					t.mu.Unlock()
				}
			})
		}
	}
}

// scheduleTrailingLocked schedules a trailing edge call (must hold lock).
func (t *Throttler) scheduleTrailingLocked() {
	if t.trailing && t.timer == nil {
		t.seq++
		currentSeq := t.seq
		t.timer = time.AfterFunc(t.interval, func() {
			t.mu.Lock()
			if t.pending && t.seq == currentSeq {
				t.pending = false
				t.lastCall = time.Now()
				t.timer = nil
				t.mu.Unlock()
				t.callback()
			} else {
				t.timer = nil
				t.mu.Unlock()
			}
		})
	}
}

// Cancel cancels any pending throttled call.
// Note: Cancel does not guarantee no callbacks are in-flight; it prevents
// future scheduled callbacks from executing.
func (t *Throttler) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	// Increment seq to invalidate any running timer callback
	t.seq++
	t.pending = false
}

// Reset resets the throttler state.
func (t *Throttler) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	// Increment seq to invalidate any running timer callback
	t.seq++
	t.pending = false
	t.lastCall = time.Time{}
}

// Cache provides a generic TTL-based cache.
type Cache[K comparable, V any] struct {
	mu      sync.RWMutex
	items   map[K]*cacheItem[V]
	ttl     time.Duration
	maxSize int
}

type cacheItem[V any] struct {
	value     V
	expiresAt time.Time
}

// CacheOption configures a Cache.
type CacheOption[K comparable, V any] func(*Cache[K, V])

// WithMaxSize sets the maximum cache size.
func WithMaxSize[K comparable, V any](size int) CacheOption[K, V] {
	return func(c *Cache[K, V]) {
		c.maxSize = size
	}
}

// NewCache creates a new cache with the specified TTL.
func NewCache[K comparable, V any](ttl time.Duration, opts ...CacheOption[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		items: make(map[K]*cacheItem[V]),
		ttl:   ttl,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, otherwise zero value and false.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if time.Now().After(item.expiresAt) {
		// Expired, remove it
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		var zero V
		return zero, false
	}

	return item.value, true
}

// Set stores a value in the cache.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at max size
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = &cacheItem[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// SetWithTTL stores a value with a custom TTL.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at max size
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = &cacheItem[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Clear removes all values from the cache.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	c.items = make(map[K]*cacheItem[V])
	c.mu.Unlock()
}

// Size returns the number of items in the cache (including expired).
func (c *Cache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Cleanup removes expired items from the cache.
func (c *Cache[K, V]) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	count := 0

	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
			count++
		}
	}

	return count
}

// evictOldest removes the item with the soonest expiration time (must hold lock).
// Note: This is expiration-based eviction, not LRU (least recently used).
func (c *Cache[K, V]) evictOldest() {
	var oldestKey K
	var oldestTime time.Time
	first := true

	for key, item := range c.items {
		if first || item.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiresAt
			first = false
		}
	}

	if !first {
		delete(c.items, oldestKey)
	}
}

// GetOrSet returns the cached value if present, otherwise calls fn to compute it.
// Note: If multiple goroutines call GetOrSet concurrently for the same key,
// fn may be called multiple times (stampede). Use singleflight if this is undesirable.
func (c *Cache[K, V]) GetOrSet(key K, fn func() (V, error)) (V, error) {
	// Try to get from cache first
	if value, ok := c.Get(key); ok {
		return value, nil
	}

	// Compute the value
	value, err := fn()
	if err != nil {
		var zero V
		return zero, err
	}

	// Store in cache
	c.Set(key, value)
	return value, nil
}
