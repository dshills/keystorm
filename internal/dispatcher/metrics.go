package dispatcher

import (
	"sort"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/handler"
)

// Metrics collects dispatch statistics.
type Metrics struct {
	mu sync.RWMutex

	// Per-action metrics
	actionMetrics map[string]*ActionMetrics

	// Global counters
	totalDispatches uint64
	totalErrors     uint64
	totalPanics     uint64

	// Timing
	totalDuration time.Duration
}

// ActionMetrics holds metrics for a specific action.
type ActionMetrics struct {
	Name          string
	DispatchCount uint64
	ErrorCount    uint64
	TotalDuration time.Duration
	MinDuration   time.Duration
	MaxDuration   time.Duration
	LastStatus    handler.ResultStatus
	LastDispatch  time.Time
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		actionMetrics: make(map[string]*ActionMetrics),
	}
}

// RecordDispatch records a dispatch event.
func (m *Metrics) RecordDispatch(actionName string, duration time.Duration, status handler.ResultStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalDispatches++
	m.totalDuration += duration

	if status == handler.StatusError {
		m.totalErrors++
	}

	am := m.actionMetrics[actionName]
	if am == nil {
		am = &ActionMetrics{
			Name:        actionName,
			MinDuration: duration,
			MaxDuration: duration,
		}
		m.actionMetrics[actionName] = am
	}

	am.DispatchCount++
	am.TotalDuration += duration
	am.LastStatus = status
	am.LastDispatch = time.Now()

	if duration < am.MinDuration {
		am.MinDuration = duration
	}
	if duration > am.MaxDuration {
		am.MaxDuration = duration
	}

	if status == handler.StatusError {
		am.ErrorCount++
	}
}

// RecordPanic records a panic recovery.
func (m *Metrics) RecordPanic(actionName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalPanics++

	am := m.actionMetrics[actionName]
	if am != nil {
		am.ErrorCount++
	}
}

// TotalDispatches returns the total number of dispatches.
func (m *Metrics) TotalDispatches() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalDispatches
}

// TotalErrors returns the total number of errors.
func (m *Metrics) TotalErrors() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalErrors
}

// TotalPanics returns the total number of panics recovered.
func (m *Metrics) TotalPanics() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalPanics
}

// TotalDuration returns the total duration of all dispatches.
func (m *Metrics) TotalDuration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalDuration
}

// AverageDuration returns the average dispatch duration.
func (m *Metrics) AverageDuration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalDispatches == 0 {
		return 0
	}
	return m.totalDuration / time.Duration(m.totalDispatches)
}

// ActionStats returns metrics for a specific action.
func (m *Metrics) ActionStats(actionName string) *ActionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	am := m.actionMetrics[actionName]
	if am == nil {
		return nil
	}

	// Return a copy
	copy := *am
	return &copy
}

// TopActions returns the top N most dispatched actions.
func (m *Metrics) TopActions(n int) []*ActionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	actions := make([]*ActionMetrics, 0, len(m.actionMetrics))
	for _, am := range m.actionMetrics {
		copy := *am
		actions = append(actions, &copy)
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].DispatchCount > actions[j].DispatchCount
	})

	if n > len(actions) {
		n = len(actions)
	}
	return actions[:n]
}

// SlowestActions returns the top N slowest actions by average duration.
func (m *Metrics) SlowestActions(n int) []*ActionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	actions := make([]*ActionMetrics, 0, len(m.actionMetrics))
	for _, am := range m.actionMetrics {
		if am.DispatchCount > 0 {
			copy := *am
			actions = append(actions, &copy)
		}
	}

	sort.Slice(actions, func(i, j int) bool {
		avgI := actions[i].TotalDuration / time.Duration(actions[i].DispatchCount)
		avgJ := actions[j].TotalDuration / time.Duration(actions[j].DispatchCount)
		return avgI > avgJ
	})

	if n > len(actions) {
		n = len(actions)
	}
	return actions[:n]
}

// Reset clears all metrics.
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.actionMetrics = make(map[string]*ActionMetrics)
	m.totalDispatches = 0
	m.totalErrors = 0
	m.totalPanics = 0
	m.totalDuration = 0
}

// Snapshot returns a point-in-time snapshot of all metrics.
type MetricsSnapshot struct {
	TotalDispatches uint64
	TotalErrors     uint64
	TotalPanics     uint64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	ActionCount     int
	Timestamp       time.Time
}

// Snapshot returns a snapshot of current metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := MetricsSnapshot{
		TotalDispatches: m.totalDispatches,
		TotalErrors:     m.totalErrors,
		TotalPanics:     m.totalPanics,
		TotalDuration:   m.totalDuration,
		ActionCount:     len(m.actionMetrics),
		Timestamp:       time.Now(),
	}

	if m.totalDispatches > 0 {
		snapshot.AverageDuration = m.totalDuration / time.Duration(m.totalDispatches)
	}

	return snapshot
}

// AverageActionDuration returns the average duration for a specific action.
func (am *ActionMetrics) AverageActionDuration() time.Duration {
	if am.DispatchCount == 0 {
		return 0
	}
	return am.TotalDuration / time.Duration(am.DispatchCount)
}

// ErrorRate returns the error rate as a percentage.
func (am *ActionMetrics) ErrorRate() float64 {
	if am.DispatchCount == 0 {
		return 0
	}
	return float64(am.ErrorCount) / float64(am.DispatchCount) * 100
}
