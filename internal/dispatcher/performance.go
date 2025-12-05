package dispatcher

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// PerformanceMonitor tracks dispatch performance and identifies bottlenecks.
type PerformanceMonitor struct {
	mu sync.RWMutex

	// Latency tracking
	latencies     map[string]*LatencyTracker
	globalTracker *LatencyTracker

	// Threshold alerting
	slowThreshold time.Duration
	alertCallback func(action string, duration time.Duration)

	// Sampling
	sampleRate  float64 // 0.0-1.0, 1.0 = sample all
	sampleCount uint64

	// Enabled state
	enabled atomic.Bool
}

// LatencyTracker tracks latency statistics for an action or globally.
// Uses Welford's online algorithm for numerically stable variance calculation.
type LatencyTracker struct {
	mu sync.RWMutex

	count    uint64
	minNanos uint64
	maxNanos uint64

	// Welford's algorithm state (using float64 to avoid overflow)
	mean float64 // Running mean in nanoseconds
	m2   float64 // Sum of squared differences from the mean

	// Histogram buckets (in microseconds)
	// <10us, <50us, <100us, <500us, <1ms, <5ms, <10ms, <50ms, <100ms, >=100ms
	buckets [10]uint64
}

// Bucket boundaries in microseconds for percentile estimation.
var bucketBoundaries = []int64{10, 50, 100, 500, 1000, 5000, 10000, 50000, 100000}

// NewLatencyTracker creates a new latency tracker.
func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{
		minNanos: ^uint64(0), // Max uint64
	}
}

// Record records a latency measurement.
func (lt *LatencyTracker) Record(d time.Duration) {
	nanos := d.Nanoseconds()
	if nanos < 0 {
		nanos = 0
	}
	uNanos := uint64(nanos)
	fNanos := float64(nanos)

	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.count++

	// Welford's online algorithm for mean and variance
	// https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Welford's_online_algorithm
	delta := fNanos - lt.mean
	lt.mean += delta / float64(lt.count)
	delta2 := fNanos - lt.mean
	lt.m2 += delta * delta2

	if uNanos < lt.minNanos {
		lt.minNanos = uNanos
	}
	if uNanos > lt.maxNanos {
		lt.maxNanos = uNanos
	}

	// Update histogram
	micros := uNanos / 1000
	switch {
	case micros < 10:
		lt.buckets[0]++
	case micros < 50:
		lt.buckets[1]++
	case micros < 100:
		lt.buckets[2]++
	case micros < 500:
		lt.buckets[3]++
	case micros < 1000:
		lt.buckets[4]++
	case micros < 5000:
		lt.buckets[5]++
	case micros < 10000:
		lt.buckets[6]++
	case micros < 50000:
		lt.buckets[7]++
	case micros < 100000:
		lt.buckets[8]++
	default:
		lt.buckets[9]++
	}
}

// LatencyStats holds computed latency statistics.
type LatencyStats struct {
	Count        uint64
	TotalTime    time.Duration
	MinTime      time.Duration
	MaxTime      time.Duration
	AvgTime      time.Duration
	StdDev       time.Duration
	Percentile50 time.Duration // Estimated from histogram
	Percentile95 time.Duration
	Percentile99 time.Duration
	Histogram    [10]uint64
}

// Stats returns computed statistics.
// The returned LatencyStats is a snapshot and will not be updated.
func (lt *LatencyTracker) Stats() LatencyStats {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	stats := LatencyStats{
		Count:     lt.count,
		Histogram: lt.buckets,
	}

	if lt.count == 0 {
		return stats
	}

	stats.MinTime = time.Duration(lt.minNanos)
	stats.MaxTime = time.Duration(lt.maxNanos)
	stats.AvgTime = time.Duration(lt.mean)
	stats.TotalTime = time.Duration(lt.mean * float64(lt.count))

	// Calculate standard deviation using Welford's algorithm
	if lt.count > 1 {
		variance := lt.m2 / float64(lt.count-1) // Sample variance
		if variance > 0 {
			stats.StdDev = time.Duration(math.Sqrt(variance))
		}
	}

	// Estimate percentiles from histogram
	stats.Percentile50 = lt.estimatePercentileLocked(50)
	stats.Percentile95 = lt.estimatePercentileLocked(95)
	stats.Percentile99 = lt.estimatePercentileLocked(99)

	return stats
}

// estimatePercentileLocked estimates a percentile from the histogram.
// Must be called with lock held.
func (lt *LatencyTracker) estimatePercentileLocked(p int) time.Duration {
	if lt.count == 0 {
		return 0
	}

	target := (uint64(p) * lt.count) / 100
	var cumulative uint64

	for i, count := range lt.buckets {
		cumulative += count
		if cumulative >= target {
			// Return midpoint of bucket
			if i == 0 {
				return time.Duration(bucketBoundaries[0]/2) * time.Microsecond
			}
			mid := (bucketBoundaries[i-1] + bucketBoundaries[i]) / 2
			return time.Duration(mid) * time.Microsecond
		}
	}

	// Last bucket (>=100ms), return 100ms as estimate
	return 100 * time.Millisecond
}

// Reset clears all tracked data.
func (lt *LatencyTracker) Reset() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.count = 0
	lt.minNanos = ^uint64(0)
	lt.maxNanos = 0
	lt.mean = 0
	lt.m2 = 0
	lt.buckets = [10]uint64{}
}

// NewPerformanceMonitor creates a new performance monitor.
func NewPerformanceMonitor() *PerformanceMonitor {
	pm := &PerformanceMonitor{
		latencies:     make(map[string]*LatencyTracker),
		globalTracker: NewLatencyTracker(),
		slowThreshold: time.Millisecond, // Default 1ms
		sampleRate:    1.0,              // Sample all by default
	}
	pm.enabled.Store(true)
	return pm
}

// SetSlowThreshold sets the threshold for slow action alerts.
func (pm *PerformanceMonitor) SetSlowThreshold(threshold time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.slowThreshold = threshold
}

// SetAlertCallback sets the callback for slow action alerts.
func (pm *PerformanceMonitor) SetAlertCallback(callback func(action string, duration time.Duration)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alertCallback = callback
}

// SetSampleRate sets the sampling rate (0.0-1.0).
func (pm *PerformanceMonitor) SetSampleRate(rate float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	pm.sampleRate = rate
}

// Enable enables or disables monitoring.
func (pm *PerformanceMonitor) Enable(enabled bool) {
	pm.enabled.Store(enabled)
}

// IsEnabled returns true if monitoring is enabled.
func (pm *PerformanceMonitor) IsEnabled() bool {
	return pm.enabled.Load()
}

// Record records a dispatch latency.
func (pm *PerformanceMonitor) Record(actionName string, duration time.Duration) {
	if !pm.enabled.Load() {
		return
	}

	pm.mu.Lock()
	rate := pm.sampleRate
	threshold := pm.slowThreshold
	callback := pm.alertCallback
	pm.sampleCount++
	count := pm.sampleCount
	pm.mu.Unlock()

	// Sampling
	if rate < 1.0 {
		// Simple deterministic sampling based on count
		if float64(count%100)/100 >= rate {
			return
		}
	}

	// Record globally
	pm.globalTracker.Record(duration)

	// Record per-action
	pm.mu.Lock()
	tracker := pm.latencies[actionName]
	if tracker == nil {
		tracker = NewLatencyTracker()
		pm.latencies[actionName] = tracker
	}
	pm.mu.Unlock()

	tracker.Record(duration)

	// Check slow threshold
	if duration > threshold && callback != nil {
		callback(actionName, duration)
	}
}

// GlobalStats returns global latency statistics.
func (pm *PerformanceMonitor) GlobalStats() LatencyStats {
	return pm.globalTracker.Stats()
}

// ActionStats returns statistics for a specific action.
func (pm *PerformanceMonitor) ActionStats(actionName string) *LatencyStats {
	pm.mu.RLock()
	tracker := pm.latencies[actionName]
	pm.mu.RUnlock()

	if tracker == nil {
		return nil
	}

	stats := tracker.Stats()
	return &stats
}

// AllActionStats returns statistics for all tracked actions.
func (pm *PerformanceMonitor) AllActionStats() map[string]LatencyStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]LatencyStats, len(pm.latencies))
	for name, tracker := range pm.latencies {
		result[name] = tracker.Stats()
	}
	return result
}

// SlowestActions returns the N slowest actions by average latency.
func (pm *PerformanceMonitor) SlowestActions(n int) []ActionLatency {
	stats := pm.AllActionStats()

	actions := make([]ActionLatency, 0, len(stats))
	for name, s := range stats {
		actions = append(actions, ActionLatency{
			Action:  name,
			AvgTime: s.AvgTime,
			MaxTime: s.MaxTime,
			Count:   s.Count,
		})
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].AvgTime > actions[j].AvgTime
	})

	if n > len(actions) {
		n = len(actions)
	}
	return actions[:n]
}

// ActionLatency holds latency info for an action.
type ActionLatency struct {
	Action  string
	AvgTime time.Duration
	MaxTime time.Duration
	Count   uint64
}

// Reset clears all tracked data.
func (pm *PerformanceMonitor) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.globalTracker.Reset()
	pm.latencies = make(map[string]*LatencyTracker)
	pm.sampleCount = 0
}

// PerformanceReport generates a performance report.
type PerformanceReport struct {
	Generated    time.Time
	GlobalStats  LatencyStats
	SlowestN     []ActionLatency
	ActionCount  int
	TotalSamples uint64
}

// Report generates a performance report.
func (pm *PerformanceMonitor) Report(topN int) PerformanceReport {
	pm.mu.RLock()
	sampleCount := pm.sampleCount
	actionCount := len(pm.latencies)
	pm.mu.RUnlock()

	return PerformanceReport{
		Generated:    time.Now(),
		GlobalStats:  pm.GlobalStats(),
		SlowestN:     pm.SlowestActions(topN),
		ActionCount:  actionCount,
		TotalSamples: sampleCount,
	}
}

// Benchmark runs a simple benchmark on the dispatcher.
type Benchmark struct {
	dispatcher *Dispatcher
	results    []BenchmarkResult
}

// BenchmarkResult holds results for a single benchmark run.
type BenchmarkResult struct {
	ActionName  string
	Iterations  int
	TotalTime   time.Duration
	AvgTime     time.Duration
	MinTime     time.Duration
	MaxTime     time.Duration
	Throughput  float64 // ops/sec
	ErrorCount  int
	SuccessRate float64
}

// NewBenchmark creates a benchmark runner.
func NewBenchmark(dispatcher *Dispatcher) *Benchmark {
	return &Benchmark{
		dispatcher: dispatcher,
		results:    make([]BenchmarkResult, 0),
	}
}

// RunAction benchmarks a single action.
func (b *Benchmark) RunAction(actionName string, iterations int) BenchmarkResult {
	result := BenchmarkResult{
		ActionName: actionName,
		Iterations: iterations,
		MinTime:    time.Hour, // Start with large value
	}

	action := input.Action{Name: actionName}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		iterStart := time.Now()
		res := b.dispatcher.Dispatch(action)
		iterDuration := time.Since(iterStart)

		if res.Status == handler.StatusError {
			result.ErrorCount++
		}

		if iterDuration < result.MinTime {
			result.MinTime = iterDuration
		}
		if iterDuration > result.MaxTime {
			result.MaxTime = iterDuration
		}
	}
	result.TotalTime = time.Since(start)

	if iterations > 0 {
		result.AvgTime = result.TotalTime / time.Duration(iterations)
		result.Throughput = float64(iterations) / result.TotalTime.Seconds()
		result.SuccessRate = float64(iterations-result.ErrorCount) / float64(iterations) * 100
	}

	b.results = append(b.results, result)
	return result
}

// Results returns all benchmark results.
func (b *Benchmark) Results() []BenchmarkResult {
	results := make([]BenchmarkResult, len(b.results))
	copy(results, b.results)
	return results
}

// Reset clears benchmark results.
func (b *Benchmark) Reset() {
	b.results = b.results[:0]
}

// DispatchOptimizer provides utilities for optimizing dispatch performance.
// It is safe for concurrent use.
type DispatchOptimizer struct {
	mu sync.RWMutex

	// Hot path actions that should be optimized
	hotPaths map[string]bool

	// Action batching
	batchSize     int
	batchInterval time.Duration
}

// NewDispatchOptimizer creates a new optimizer.
func NewDispatchOptimizer() *DispatchOptimizer {
	return &DispatchOptimizer{
		hotPaths:      make(map[string]bool),
		batchSize:     10,
		batchInterval: time.Millisecond,
	}
}

// MarkHotPath marks an action as a hot path for optimization.
func (o *DispatchOptimizer) MarkHotPath(actionName string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.hotPaths[actionName] = true
}

// IsHotPath returns true if the action is marked as a hot path.
func (o *DispatchOptimizer) IsHotPath(actionName string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.hotPaths[actionName]
}

// DefaultHotPaths returns the default set of hot path actions.
func DefaultHotPaths() []string {
	return []string{
		// Cursor movements - most frequent
		"cursor.moveLeft",
		"cursor.moveRight",
		"cursor.moveUp",
		"cursor.moveDown",
		"cursor.moveWordForward",
		"cursor.moveWordBackward",

		// Character insertion - very frequent in insert mode
		"editor.insertChar",
		"editor.insertText",

		// Character deletion - frequent
		"editor.deleteChar",
		"editor.deleteCharBefore",

		// Navigation
		"view.scrollDown",
		"view.scrollUp",
	}
}

// ActionBatcher batches multiple actions for efficient dispatch.
type ActionBatcher struct {
	mu       sync.Mutex
	actions  []input.Action
	maxSize  int
	interval time.Duration
	dispatch func([]input.Action)
	timer    *time.Timer
}

// NewActionBatcher creates a new action batcher.
// If maxSize <= 0, it defaults to 10 to prevent immediate/infinite flushing.
func NewActionBatcher(maxSize int, interval time.Duration, dispatch func([]input.Action)) *ActionBatcher {
	if maxSize <= 0 {
		maxSize = 10 // Sensible default
	}
	return &ActionBatcher{
		actions:  make([]input.Action, 0, maxSize),
		maxSize:  maxSize,
		interval: interval,
		dispatch: dispatch,
	}
}

// Add adds an action to the batch.
// Returns true if the batch was flushed.
func (b *ActionBatcher) Add(action input.Action) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.actions = append(b.actions, action)

	// Flush if max size reached
	if len(b.actions) >= b.maxSize {
		b.flushLocked()
		return true
	}

	// Start timer if first action
	if len(b.actions) == 1 && b.interval > 0 {
		b.timer = time.AfterFunc(b.interval, func() {
			b.Flush()
		})
	}

	return false
}

// Flush dispatches all batched actions.
func (b *ActionBatcher) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// flushLocked flushes without acquiring lock.
func (b *ActionBatcher) flushLocked() {
	if len(b.actions) == 0 {
		return
	}

	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}

	actions := make([]input.Action, len(b.actions))
	copy(actions, b.actions)
	b.actions = b.actions[:0]

	if b.dispatch != nil {
		b.dispatch(actions)
	}
}

// Pending returns the number of pending actions.
func (b *ActionBatcher) Pending() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.actions)
}
