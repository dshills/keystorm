package input

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks input processing performance.
type Metrics struct {
	// Event counters
	keyEventsTotal   atomic.Uint64
	mouseEventsTotal atomic.Uint64
	actionsTotal     atomic.Uint64
	droppedEvents    atomic.Uint64
	sequenceTimeouts atomic.Uint64
	hookConsumptions atomic.Uint64

	// Latency tracking
	mu                sync.RWMutex
	keyLatencies      []time.Duration
	actionLatencies   []time.Duration
	maxLatencySamples int
	latencyIdx        int
	actionLatencyIdx  int

	// Peak latency (all time)
	peakKeyLatency    atomic.Int64
	peakActionLatency atomic.Int64

	// Start time for uptime calculation
	startTime time.Time

	// Enable flag
	enabled atomic.Bool
}

// NewMetrics creates a new metrics tracker.
func NewMetrics() *Metrics {
	m := &Metrics{
		keyLatencies:      make([]time.Duration, 1000),
		actionLatencies:   make([]time.Duration, 1000),
		maxLatencySamples: 1000,
		startTime:         time.Now(),
	}
	m.enabled.Store(true)
	return m
}

// SetEnabled enables or disables metrics collection.
func (m *Metrics) SetEnabled(enabled bool) {
	m.enabled.Store(enabled)
}

// IsEnabled returns whether metrics collection is enabled.
func (m *Metrics) IsEnabled() bool {
	return m.enabled.Load()
}

// RecordKeyEvent records a key event with its processing time.
func (m *Metrics) RecordKeyEvent(latency time.Duration) {
	if !m.enabled.Load() {
		return
	}

	m.keyEventsTotal.Add(1)

	// Update peak latency
	latencyNs := latency.Nanoseconds()
	for {
		current := m.peakKeyLatency.Load()
		if latencyNs <= current {
			break
		}
		if m.peakKeyLatency.CompareAndSwap(current, latencyNs) {
			break
		}
	}

	// Store in circular buffer
	m.mu.Lock()
	m.keyLatencies[m.latencyIdx] = latency
	m.latencyIdx = (m.latencyIdx + 1) % m.maxLatencySamples
	m.mu.Unlock()
}

// RecordMouseEvent records a mouse event.
func (m *Metrics) RecordMouseEvent() {
	if !m.enabled.Load() {
		return
	}
	m.mouseEventsTotal.Add(1)
}

// RecordAction records an action dispatch with its processing time.
func (m *Metrics) RecordAction(latency time.Duration) {
	if !m.enabled.Load() {
		return
	}

	m.actionsTotal.Add(1)

	// Update peak latency
	latencyNs := latency.Nanoseconds()
	for {
		current := m.peakActionLatency.Load()
		if latencyNs <= current {
			break
		}
		if m.peakActionLatency.CompareAndSwap(current, latencyNs) {
			break
		}
	}

	// Store in circular buffer
	m.mu.Lock()
	m.actionLatencies[m.actionLatencyIdx] = latency
	m.actionLatencyIdx = (m.actionLatencyIdx + 1) % m.maxLatencySamples
	m.mu.Unlock()
}

// RecordDroppedEvent records a dropped event (channel full).
func (m *Metrics) RecordDroppedEvent() {
	if !m.enabled.Load() {
		return
	}
	m.droppedEvents.Add(1)
}

// RecordSequenceTimeout records a sequence timeout.
func (m *Metrics) RecordSequenceTimeout() {
	if !m.enabled.Load() {
		return
	}
	m.sequenceTimeouts.Add(1)
}

// RecordHookConsumption records when a hook consumes an event/action.
func (m *Metrics) RecordHookConsumption() {
	if !m.enabled.Load() {
		return
	}
	m.hookConsumptions.Add(1)
}

// MetricsSnapshot holds a point-in-time view of metrics.
type MetricsSnapshot struct {
	// Counters
	KeyEventsTotal   uint64
	MouseEventsTotal uint64
	ActionsTotal     uint64
	DroppedEvents    uint64
	SequenceTimeouts uint64
	HookConsumptions uint64

	// Latency stats
	AvgKeyLatency  time.Duration
	MaxKeyLatency  time.Duration
	P99KeyLatency  time.Duration
	PeakKeyLatency time.Duration

	AvgActionLatency  time.Duration
	MaxActionLatency  time.Duration
	P99ActionLatency  time.Duration
	PeakActionLatency time.Duration

	// Rates
	EventsPerSecond  float64
	ActionsPerSecond float64

	// Uptime
	Uptime time.Duration
}

// Snapshot returns a point-in-time view of all metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	keyLatencies := make([]time.Duration, len(m.keyLatencies))
	copy(keyLatencies, m.keyLatencies)
	actionLatencies := make([]time.Duration, len(m.actionLatencies))
	copy(actionLatencies, m.actionLatencies)
	m.mu.RUnlock()

	keyCount := m.keyEventsTotal.Load()
	actionCount := m.actionsTotal.Load()
	uptime := time.Since(m.startTime)

	snap := MetricsSnapshot{
		KeyEventsTotal:    keyCount,
		MouseEventsTotal:  m.mouseEventsTotal.Load(),
		ActionsTotal:      actionCount,
		DroppedEvents:     m.droppedEvents.Load(),
		SequenceTimeouts:  m.sequenceTimeouts.Load(),
		HookConsumptions:  m.hookConsumptions.Load(),
		PeakKeyLatency:    time.Duration(m.peakKeyLatency.Load()),
		PeakActionLatency: time.Duration(m.peakActionLatency.Load()),
		Uptime:            uptime,
	}

	// Calculate rates
	if uptime > 0 {
		snap.EventsPerSecond = float64(keyCount) / uptime.Seconds()
		snap.ActionsPerSecond = float64(actionCount) / uptime.Seconds()
	}

	// Calculate latency stats for key events
	snap.AvgKeyLatency, snap.MaxKeyLatency, snap.P99KeyLatency = calculateLatencyStats(keyLatencies)

	// Calculate latency stats for actions
	snap.AvgActionLatency, snap.MaxActionLatency, snap.P99ActionLatency = calculateLatencyStats(actionLatencies)

	return snap
}

// calculateLatencyStats computes average, max, and p99 from a slice of latencies.
func calculateLatencyStats(latencies []time.Duration) (avg, maxLat, p99 time.Duration) {
	// Filter non-zero latencies
	valid := make([]time.Duration, 0, len(latencies))
	for _, l := range latencies {
		if l > 0 {
			valid = append(valid, l)
		}
	}

	if len(valid) == 0 {
		return 0, 0, 0
	}

	// Calculate average
	var sum time.Duration
	for _, l := range valid {
		sum += l
		if l > maxLat {
			maxLat = l
		}
	}
	avg = sum / time.Duration(len(valid))

	// Sort for percentile calculation (simple approach)
	sorted := make([]time.Duration, len(valid))
	copy(sorted, valid)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// P99
	idx := int(float64(len(sorted)) * 0.99)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	p99 = sorted[idx]

	return avg, maxLat, p99
}

// Reset clears all metrics.
func (m *Metrics) Reset() {
	m.keyEventsTotal.Store(0)
	m.mouseEventsTotal.Store(0)
	m.actionsTotal.Store(0)
	m.droppedEvents.Store(0)
	m.sequenceTimeouts.Store(0)
	m.hookConsumptions.Store(0)
	m.peakKeyLatency.Store(0)
	m.peakActionLatency.Store(0)

	m.mu.Lock()
	m.keyLatencies = make([]time.Duration, m.maxLatencySamples)
	m.actionLatencies = make([]time.Duration, m.maxLatencySamples)
	m.latencyIdx = 0
	m.actionLatencyIdx = 0
	m.startTime = time.Now()
	m.mu.Unlock()
}

// KeyEventsTotal returns the total number of key events processed.
func (m *Metrics) KeyEventsTotal() uint64 {
	return m.keyEventsTotal.Load()
}

// MouseEventsTotal returns the total number of mouse events processed.
func (m *Metrics) MouseEventsTotal() uint64 {
	return m.mouseEventsTotal.Load()
}

// ActionsTotal returns the total number of actions dispatched.
func (m *Metrics) ActionsTotal() uint64 {
	return m.actionsTotal.Load()
}

// DroppedEvents returns the total number of dropped events.
func (m *Metrics) DroppedEvents() uint64 {
	return m.droppedEvents.Load()
}

// HealthStatus represents the current health status of input processing.
type HealthStatus struct {
	Healthy          bool
	DroppedEvents    uint64
	PeakLatency      time.Duration
	LatencyThreshold time.Duration
	Message          string
}

// HealthCheck returns the current health status.
func (m *Metrics) HealthCheck(latencyThreshold time.Duration) HealthStatus {
	status := HealthStatus{
		Healthy:          true,
		DroppedEvents:    m.droppedEvents.Load(),
		PeakLatency:      time.Duration(m.peakKeyLatency.Load()),
		LatencyThreshold: latencyThreshold,
	}

	if status.DroppedEvents > 0 {
		status.Healthy = false
		status.Message = "dropped events detected"
	} else if status.PeakLatency > latencyThreshold {
		status.Healthy = false
		status.Message = "latency threshold exceeded"
	} else {
		status.Message = "healthy"
	}

	return status
}

// Timer helps measure operation duration.
type Timer struct {
	start   time.Time
	metrics *Metrics
}

// StartKeyEventTimer starts a timer for measuring key event processing.
func (m *Metrics) StartKeyEventTimer() *Timer {
	return &Timer{
		start:   time.Now(),
		metrics: m,
	}
}

// Stop stops the timer and records the key event latency.
func (t *Timer) Stop() time.Duration {
	elapsed := time.Since(t.start)
	t.metrics.RecordKeyEvent(elapsed)
	return elapsed
}

// StartActionTimer starts a timer for measuring action processing.
func (m *Metrics) StartActionTimer() *Timer {
	return &Timer{
		start:   time.Now(),
		metrics: m,
	}
}

// StopAction stops the timer and records the action latency.
func (t *Timer) StopAction() time.Duration {
	elapsed := time.Since(t.start)
	t.metrics.RecordAction(elapsed)
	return elapsed
}
