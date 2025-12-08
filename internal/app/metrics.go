// Package app provides the main application structure and coordination.
package app

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks application performance metrics.
type Metrics struct {
	mu sync.RWMutex

	// Frame timing
	frameCount    atomic.Uint64
	frameTotalNs  atomic.Int64
	frameMinNs    atomic.Int64
	frameMaxNs    atomic.Int64
	lastFrameNs   atomic.Int64
	droppedFrames atomic.Uint64

	// Input handling
	inputCount   atomic.Uint64
	inputTotalNs atomic.Int64
	inputDropped atomic.Uint64

	// Render timing
	renderCount   atomic.Uint64
	renderTotalNs atomic.Int64

	// Event processing
	eventCount   atomic.Uint64
	eventTotalNs atomic.Int64

	// Memory (sampled periodically)
	lastHeapBytes atomic.Uint64
	lastGCPauseNs atomic.Int64

	// Start time for uptime calculation
	startTime time.Time
}

// NewMetrics creates a new metrics tracker.
func NewMetrics() *Metrics {
	m := &Metrics{
		startTime: time.Now(),
	}
	// Initialize min to max int64 so first frame will be smaller
	m.frameMinNs.Store(1<<63 - 1)
	return m
}

// RecordFrame records frame timing.
func (m *Metrics) RecordFrame(duration time.Duration) {
	ns := duration.Nanoseconds()

	m.frameCount.Add(1)
	m.frameTotalNs.Add(ns)
	m.lastFrameNs.Store(ns)

	// Update min (atomic compare-and-swap loop)
	for {
		old := m.frameMinNs.Load()
		if ns >= old {
			break
		}
		if m.frameMinNs.CompareAndSwap(old, ns) {
			break
		}
	}

	// Update max (atomic compare-and-swap loop)
	for {
		old := m.frameMaxNs.Load()
		if ns <= old {
			break
		}
		if m.frameMaxNs.CompareAndSwap(old, ns) {
			break
		}
	}
}

// RecordDroppedFrame records a dropped frame.
func (m *Metrics) RecordDroppedFrame() {
	m.droppedFrames.Add(1)
}

// RecordInput records input processing timing.
func (m *Metrics) RecordInput(duration time.Duration) {
	m.inputCount.Add(1)
	m.inputTotalNs.Add(duration.Nanoseconds())
}

// RecordInputDropped records a dropped input event.
func (m *Metrics) RecordInputDropped() {
	m.inputDropped.Add(1)
}

// RecordRender records render timing.
func (m *Metrics) RecordRender(duration time.Duration) {
	m.renderCount.Add(1)
	m.renderTotalNs.Add(duration.Nanoseconds())
}

// RecordEvent records event processing timing.
func (m *Metrics) RecordEvent(duration time.Duration) {
	m.eventCount.Add(1)
	m.eventTotalNs.Add(duration.Nanoseconds())
}

// UpdateMemory updates memory statistics.
func (m *Metrics) UpdateMemory(heapBytes uint64, gcPauseNs int64) {
	m.lastHeapBytes.Store(heapBytes)
	m.lastGCPauseNs.Store(gcPauseNs)
}

// Snapshot returns a snapshot of current metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	frameCount := m.frameCount.Load()
	inputCount := m.inputCount.Load()
	renderCount := m.renderCount.Load()
	eventCount := m.eventCount.Load()

	var avgFrameNs int64
	if frameCount > 0 {
		avgFrameNs = m.frameTotalNs.Load() / int64(frameCount)
	}

	var avgInputNs int64
	if inputCount > 0 {
		avgInputNs = m.inputTotalNs.Load() / int64(inputCount)
	}

	var avgRenderNs int64
	if renderCount > 0 {
		avgRenderNs = m.renderTotalNs.Load() / int64(renderCount)
	}

	var avgEventNs int64
	if eventCount > 0 {
		avgEventNs = m.eventTotalNs.Load() / int64(eventCount)
	}

	minFrameNs := m.frameMinNs.Load()
	if minFrameNs == 1<<63-1 {
		minFrameNs = 0
	}

	return MetricsSnapshot{
		Uptime:         time.Since(m.startTime),
		FrameCount:     frameCount,
		AvgFrameTimeNs: avgFrameNs,
		MinFrameTimeNs: minFrameNs,
		MaxFrameTimeNs: m.frameMaxNs.Load(),
		LastFrameNs:    m.lastFrameNs.Load(),
		DroppedFrames:  m.droppedFrames.Load(),
		InputCount:     inputCount,
		AvgInputTimeNs: avgInputNs,
		InputDropped:   m.inputDropped.Load(),
		RenderCount:    renderCount,
		AvgRenderNs:    avgRenderNs,
		EventCount:     eventCount,
		AvgEventNs:     avgEventNs,
		HeapBytes:      m.lastHeapBytes.Load(),
		LastGCPauseNs:  m.lastGCPauseNs.Load(),
	}
}

// Reset clears all metrics.
func (m *Metrics) Reset() {
	m.frameCount.Store(0)
	m.frameTotalNs.Store(0)
	m.frameMinNs.Store(1<<63 - 1)
	m.frameMaxNs.Store(0)
	m.lastFrameNs.Store(0)
	m.droppedFrames.Store(0)
	m.inputCount.Store(0)
	m.inputTotalNs.Store(0)
	m.inputDropped.Store(0)
	m.renderCount.Store(0)
	m.renderTotalNs.Store(0)
	m.eventCount.Store(0)
	m.eventTotalNs.Store(0)
	m.startTime = time.Now()
}

// MetricsSnapshot is a point-in-time view of metrics.
type MetricsSnapshot struct {
	Uptime         time.Duration
	FrameCount     uint64
	AvgFrameTimeNs int64
	MinFrameTimeNs int64
	MaxFrameTimeNs int64
	LastFrameNs    int64
	DroppedFrames  uint64
	InputCount     uint64
	AvgInputTimeNs int64
	InputDropped   uint64
	RenderCount    uint64
	AvgRenderNs    int64
	EventCount     uint64
	AvgEventNs     int64
	HeapBytes      uint64
	LastGCPauseNs  int64
}

// AvgFPS returns the average frames per second.
func (s MetricsSnapshot) AvgFPS() float64 {
	if s.AvgFrameTimeNs == 0 {
		return 0
	}
	return 1e9 / float64(s.AvgFrameTimeNs)
}

// CurrentFPS returns the FPS based on last frame time.
func (s MetricsSnapshot) CurrentFPS() float64 {
	if s.LastFrameNs == 0 {
		return 0
	}
	return 1e9 / float64(s.LastFrameNs)
}

// DropRate returns the percentage of dropped frames.
func (s MetricsSnapshot) DropRate() float64 {
	total := s.FrameCount + s.DroppedFrames
	if total == 0 {
		return 0
	}
	return float64(s.DroppedFrames) / float64(total) * 100
}

// HeapMB returns heap size in megabytes.
func (s MetricsSnapshot) HeapMB() float64 {
	return float64(s.HeapBytes) / (1024 * 1024)
}

// Timer provides a simple way to measure elapsed time.
type Timer struct {
	start time.Time
}

// StartTimer creates a new timer.
func StartTimer() *Timer {
	return &Timer{start: time.Now()}
}

// Elapsed returns the elapsed time since the timer started.
func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}

// ElapsedMs returns the elapsed time in milliseconds.
func (t *Timer) ElapsedMs() float64 {
	return float64(t.Elapsed().Nanoseconds()) / 1e6
}

// Stop returns the elapsed time and resets the timer.
func (t *Timer) Stop() time.Duration {
	elapsed := t.Elapsed()
	t.start = time.Now()
	return elapsed
}

// appMetrics is the application-wide metrics instance.
var (
	appMetrics     *Metrics
	appMetricsOnce sync.Once
)

// GetMetrics returns the application metrics.
func GetMetrics() *Metrics {
	appMetricsOnce.Do(func() {
		if appMetrics == nil {
			appMetrics = NewMetrics()
		}
	})
	return appMetrics
}

// SetMetrics sets the application-wide metrics.
func SetMetrics(m *Metrics) {
	appMetrics = m
}

// Metrics returns the application's metrics instance.
func (app *Application) Metrics() *Metrics {
	if app.metrics == nil {
		return GetMetrics()
	}
	return app.metrics
}

// SetMetrics sets the application's metrics.
func (app *Application) SetMetrics(m *Metrics) {
	app.metrics = m
}
