package integration

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_Basic(t *testing.T) {
	var callCount atomic.Int32

	d := NewDebouncer(50*time.Millisecond, func() {
		callCount.Add(1)
	})

	// Call multiple times rapidly
	for i := 0; i < 10; i++ {
		d.Call()
	}

	// Wait for debounce
	time.Sleep(100 * time.Millisecond)

	// Should only have called once
	if callCount.Load() != 1 {
		t.Errorf("callCount = %d, want 1", callCount.Load())
	}
}

func TestDebouncer_SpacedCalls(t *testing.T) {
	var callCount atomic.Int32

	d := NewDebouncer(50*time.Millisecond, func() {
		callCount.Add(1)
	})

	// Call with enough time between for each to fire
	d.Call()
	time.Sleep(100 * time.Millisecond)

	d.Call()
	time.Sleep(100 * time.Millisecond)

	d.Call()
	time.Sleep(100 * time.Millisecond)

	// Should have called 3 times
	if callCount.Load() != 3 {
		t.Errorf("callCount = %d, want 3", callCount.Load())
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	var callCount atomic.Int32

	d := NewDebouncer(50*time.Millisecond, func() {
		callCount.Add(1)
	})

	d.Call()
	d.Cancel()

	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Errorf("callCount = %d, want 0 (canceled)", callCount.Load())
	}
}

func TestDebouncer_CallImmediate(t *testing.T) {
	var callCount atomic.Int32

	d := NewDebouncer(100*time.Millisecond, func() {
		callCount.Add(1)
	})

	d.Call()

	// Call immediate before debounce fires
	d.CallImmediate()

	if callCount.Load() != 1 {
		t.Errorf("callCount = %d, want 1", callCount.Load())
	}

	// Wait past debounce time
	time.Sleep(150 * time.Millisecond)

	// Should still be 1 (debounced call was cleared)
	if callCount.Load() != 1 {
		t.Errorf("callCount after wait = %d, want 1", callCount.Load())
	}
}

func TestDebouncer_IsPending(t *testing.T) {
	d := NewDebouncer(100*time.Millisecond, func() {})

	if d.IsPending() {
		t.Error("should not be pending initially")
	}

	d.Call()

	if !d.IsPending() {
		t.Error("should be pending after Call")
	}

	time.Sleep(150 * time.Millisecond)

	if d.IsPending() {
		t.Error("should not be pending after debounce")
	}
}

func TestThrottler_Basic(t *testing.T) {
	var callCount atomic.Int32

	th := NewThrottler(50*time.Millisecond, func() {
		callCount.Add(1)
	})

	// Call multiple times rapidly
	for i := 0; i < 10; i++ {
		th.Call()
	}

	// Wait for trailing call
	time.Sleep(100 * time.Millisecond)

	// Should have called twice (leading + trailing)
	count := callCount.Load()
	if count < 1 || count > 2 {
		t.Errorf("callCount = %d, want 1-2", count)
	}
}

func TestThrottler_LeadingOnly(t *testing.T) {
	var callCount atomic.Int32

	th := NewThrottler(50*time.Millisecond, func() {
		callCount.Add(1)
	}, WithLeadingEdge(true), WithTrailingEdge(false))

	// Call multiple times rapidly
	th.Call()
	th.Call()
	th.Call()

	// Small wait to ensure async call completes
	time.Sleep(20 * time.Millisecond)

	// Should have called once (leading only)
	if callCount.Load() != 1 {
		t.Errorf("callCount = %d, want 1", callCount.Load())
	}

	// Wait for interval to pass
	time.Sleep(100 * time.Millisecond)

	// Still 1 (no trailing)
	if callCount.Load() != 1 {
		t.Errorf("callCount after wait = %d, want 1", callCount.Load())
	}
}

func TestThrottler_Cancel(t *testing.T) {
	var callCount atomic.Int32

	th := NewThrottler(100*time.Millisecond, func() {
		callCount.Add(1)
	}, WithLeadingEdge(false), WithTrailingEdge(true))

	th.Call()
	th.Cancel()

	time.Sleep(150 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Errorf("callCount = %d, want 0", callCount.Load())
	}
}

func TestThrottler_Reset(t *testing.T) {
	var callCount atomic.Int32

	th := NewThrottler(50*time.Millisecond, func() {
		callCount.Add(1)
	})

	th.Call()
	time.Sleep(20 * time.Millisecond)

	th.Reset()

	// Can call again immediately after reset
	th.Call()
	time.Sleep(100 * time.Millisecond)

	// Should have at least 2 calls (one before reset, one after)
	if callCount.Load() < 2 {
		t.Errorf("callCount = %d, want >= 2", callCount.Load())
	}
}

func TestCache_Basic(t *testing.T) {
	c := NewCache[string, int](100 * time.Millisecond)

	// Get non-existent key
	_, ok := c.Get("key")
	if ok {
		t.Error("Get should return false for non-existent key")
	}

	// Set and get
	c.Set("key", 42)
	value, ok := c.Get("key")
	if !ok {
		t.Error("Get should return true for existing key")
	}
	if value != 42 {
		t.Errorf("value = %d, want 42", value)
	}
}

func TestCache_Expiration(t *testing.T) {
	c := NewCache[string, int](50 * time.Millisecond)

	c.Set("key", 42)

	// Should exist before expiration
	_, ok := c.Get("key")
	if !ok {
		t.Error("key should exist before expiration")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, ok = c.Get("key")
	if ok {
		t.Error("key should be expired")
	}
}

func TestCache_Delete(t *testing.T) {
	c := NewCache[string, int](time.Hour)

	c.Set("key", 42)
	c.Delete("key")

	_, ok := c.Get("key")
	if ok {
		t.Error("key should be deleted")
	}
}

func TestCache_Clear(t *testing.T) {
	c := NewCache[string, int](time.Hour)

	c.Set("key1", 1)
	c.Set("key2", 2)
	c.Set("key3", 3)

	c.Clear()

	if c.Size() != 0 {
		t.Errorf("Size = %d, want 0", c.Size())
	}
}

func TestCache_MaxSize(t *testing.T) {
	c := NewCache[string, int](time.Hour, WithMaxSize[string, int](3))

	c.Set("key1", 1)
	c.Set("key2", 2)
	c.Set("key3", 3)
	c.Set("key4", 4) // Should evict oldest

	if c.Size() != 3 {
		t.Errorf("Size = %d, want 3", c.Size())
	}

	// key4 should exist
	_, ok := c.Get("key4")
	if !ok {
		t.Error("key4 should exist")
	}
}

func TestCache_Cleanup(t *testing.T) {
	c := NewCache[string, int](50 * time.Millisecond)

	c.Set("key1", 1)
	c.Set("key2", 2)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Add one more (not expired)
	c.Set("key3", 3)

	// Cleanup should remove 2 expired items
	count := c.Cleanup()
	if count != 2 {
		t.Errorf("Cleanup removed %d, want 2", count)
	}

	if c.Size() != 1 {
		t.Errorf("Size = %d, want 1", c.Size())
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	c := NewCache[string, int](time.Hour)

	// Set with short TTL
	c.SetWithTTL("key", 42, 50*time.Millisecond)

	// Should exist before expiration
	_, ok := c.Get("key")
	if !ok {
		t.Error("key should exist before expiration")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, ok = c.Get("key")
	if ok {
		t.Error("key should be expired")
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c := NewCache[string, int](time.Hour)

	var computeCount int

	compute := func() (int, error) {
		computeCount++
		return 42, nil
	}

	// First call should compute
	value, err := c.GetOrSet("key", compute)
	if err != nil {
		t.Fatalf("GetOrSet error: %v", err)
	}
	if value != 42 {
		t.Errorf("value = %d, want 42", value)
	}
	if computeCount != 1 {
		t.Errorf("computeCount = %d, want 1", computeCount)
	}

	// Second call should use cache
	value, err = c.GetOrSet("key", compute)
	if err != nil {
		t.Fatalf("GetOrSet error: %v", err)
	}
	if value != 42 {
		t.Errorf("value = %d, want 42", value)
	}
	if computeCount != 1 {
		t.Errorf("computeCount = %d, want 1 (cached)", computeCount)
	}
}

func TestCache_Concurrent(t *testing.T) {
	c := NewCache[int, int](time.Hour)

	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				c.Set(id*100+j, j)
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				c.Get(id*100 + j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should not panic and have some items
	if c.Size() == 0 {
		t.Error("cache should have items")
	}
}
