package app

import (
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}

	snapshot := m.Snapshot()
	if snapshot.FrameCount != 0 {
		t.Errorf("expected 0 frame count, got %d", snapshot.FrameCount)
	}
	if snapshot.MinFrameTimeNs != 0 {
		t.Errorf("expected 0 min frame time (sentinel handled), got %d", snapshot.MinFrameTimeNs)
	}
}

func TestMetrics_RecordFrame(t *testing.T) {
	m := NewMetrics()

	m.RecordFrame(10 * time.Millisecond)
	m.RecordFrame(20 * time.Millisecond)
	m.RecordFrame(5 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.FrameCount != 3 {
		t.Errorf("expected 3 frames, got %d", snapshot.FrameCount)
	}
	if snapshot.MinFrameTimeNs != int64(5*time.Millisecond) {
		t.Errorf("expected min 5ms, got %d ns", snapshot.MinFrameTimeNs)
	}
	if snapshot.MaxFrameTimeNs != int64(20*time.Millisecond) {
		t.Errorf("expected max 20ms, got %d ns", snapshot.MaxFrameTimeNs)
	}
	if snapshot.LastFrameNs != int64(5*time.Millisecond) {
		t.Errorf("expected last 5ms, got %d ns", snapshot.LastFrameNs)
	}
}

func TestMetrics_RecordDroppedFrame(t *testing.T) {
	m := NewMetrics()

	m.RecordDroppedFrame()
	m.RecordDroppedFrame()

	snapshot := m.Snapshot()
	if snapshot.DroppedFrames != 2 {
		t.Errorf("expected 2 dropped frames, got %d", snapshot.DroppedFrames)
	}
}

func TestMetrics_RecordInput(t *testing.T) {
	m := NewMetrics()

	m.RecordInput(1 * time.Millisecond)
	m.RecordInput(2 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.InputCount != 2 {
		t.Errorf("expected 2 inputs, got %d", snapshot.InputCount)
	}
	expectedAvg := int64(1500000) // 1.5ms in nanoseconds
	if snapshot.AvgInputTimeNs != expectedAvg {
		t.Errorf("expected avg input time %d ns, got %d ns", expectedAvg, snapshot.AvgInputTimeNs)
	}
}

func TestMetrics_RecordInputDropped(t *testing.T) {
	m := NewMetrics()

	m.RecordInputDropped()
	m.RecordInputDropped()
	m.RecordInputDropped()

	snapshot := m.Snapshot()
	if snapshot.InputDropped != 3 {
		t.Errorf("expected 3 dropped inputs, got %d", snapshot.InputDropped)
	}
}

func TestMetrics_RecordRender(t *testing.T) {
	m := NewMetrics()

	m.RecordRender(5 * time.Millisecond)
	m.RecordRender(10 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.RenderCount != 2 {
		t.Errorf("expected 2 renders, got %d", snapshot.RenderCount)
	}
}

func TestMetrics_RecordEvent(t *testing.T) {
	m := NewMetrics()

	m.RecordEvent(100 * time.Microsecond)
	m.RecordEvent(200 * time.Microsecond)

	snapshot := m.Snapshot()
	if snapshot.EventCount != 2 {
		t.Errorf("expected 2 events, got %d", snapshot.EventCount)
	}
}

func TestMetrics_UpdateMemory(t *testing.T) {
	m := NewMetrics()

	m.UpdateMemory(1024*1024, 500000) // 1MB heap, 500us GC pause

	snapshot := m.Snapshot()
	if snapshot.HeapBytes != 1024*1024 {
		t.Errorf("expected 1MB heap, got %d", snapshot.HeapBytes)
	}
	if snapshot.LastGCPauseNs != 500000 {
		t.Errorf("expected 500us GC pause, got %d ns", snapshot.LastGCPauseNs)
	}
}

func TestMetrics_Snapshot_Uptime(t *testing.T) {
	m := NewMetrics()

	time.Sleep(10 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.Uptime < 10*time.Millisecond {
		t.Errorf("expected uptime >= 10ms, got %v", snapshot.Uptime)
	}
}

func TestMetrics_Reset(t *testing.T) {
	m := NewMetrics()

	m.RecordFrame(10 * time.Millisecond)
	m.RecordInput(1 * time.Millisecond)
	m.RecordDroppedFrame()

	m.Reset()

	snapshot := m.Snapshot()
	if snapshot.FrameCount != 0 {
		t.Errorf("expected 0 frames after reset, got %d", snapshot.FrameCount)
	}
	if snapshot.InputCount != 0 {
		t.Errorf("expected 0 inputs after reset, got %d", snapshot.InputCount)
	}
	if snapshot.DroppedFrames != 0 {
		t.Errorf("expected 0 dropped frames after reset, got %d", snapshot.DroppedFrames)
	}
}

func TestMetricsSnapshot_AvgFPS(t *testing.T) {
	tests := []struct {
		avgFrameTimeNs int64
		expectedFPS    float64
	}{
		{0, 0},                    // Zero protection
		{16666666, 60.0},          // ~60 FPS
		{33333333, 30.0},          // ~30 FPS
		{1000000000 / 120, 120.0}, // 120 FPS
	}

	for _, tt := range tests {
		snapshot := MetricsSnapshot{AvgFrameTimeNs: tt.avgFrameTimeNs}
		fps := snapshot.AvgFPS()
		// Allow small floating point variance
		if tt.expectedFPS == 0 && fps != 0 {
			t.Errorf("AvgFPS() for 0 ns = %f, expected 0", fps)
		} else if tt.expectedFPS > 0 {
			diff := fps - tt.expectedFPS
			if diff < -1 || diff > 1 {
				t.Errorf("AvgFPS() for %d ns = %f, expected ~%f", tt.avgFrameTimeNs, fps, tt.expectedFPS)
			}
		}
	}
}

func TestMetricsSnapshot_CurrentFPS(t *testing.T) {
	tests := []struct {
		lastFrameNs int64
		expectedFPS float64
	}{
		{0, 0},           // Zero protection
		{16666666, 60.0}, // ~60 FPS
		{33333333, 30.0}, // ~30 FPS
	}

	for _, tt := range tests {
		snapshot := MetricsSnapshot{LastFrameNs: tt.lastFrameNs}
		fps := snapshot.CurrentFPS()
		if tt.expectedFPS == 0 && fps != 0 {
			t.Errorf("CurrentFPS() for 0 ns = %f, expected 0", fps)
		} else if tt.expectedFPS > 0 {
			diff := fps - tt.expectedFPS
			if diff < -1 || diff > 1 {
				t.Errorf("CurrentFPS() for %d ns = %f, expected ~%f", tt.lastFrameNs, fps, tt.expectedFPS)
			}
		}
	}
}

func TestMetricsSnapshot_DropRate(t *testing.T) {
	tests := []struct {
		frameCount    uint64
		droppedFrames uint64
		expectedRate  float64
	}{
		{0, 0, 0},      // Zero protection
		{100, 0, 0},    // No drops
		{90, 10, 10.0}, // 10% drop rate
		{50, 50, 50.0}, // 50% drop rate
		{0, 10, 100.0}, // All dropped
	}

	for _, tt := range tests {
		snapshot := MetricsSnapshot{
			FrameCount:    tt.frameCount,
			DroppedFrames: tt.droppedFrames,
		}
		rate := snapshot.DropRate()
		if rate != tt.expectedRate {
			t.Errorf("DropRate() for %d/%d = %f, expected %f",
				tt.droppedFrames, tt.frameCount+tt.droppedFrames, rate, tt.expectedRate)
		}
	}
}

func TestMetricsSnapshot_HeapMB(t *testing.T) {
	snapshot := MetricsSnapshot{HeapBytes: 10 * 1024 * 1024} // 10MB
	heapMB := snapshot.HeapMB()
	if heapMB != 10.0 {
		t.Errorf("HeapMB() = %f, expected 10.0", heapMB)
	}
}

func TestTimer(t *testing.T) {
	timer := StartTimer()
	if timer == nil {
		t.Fatal("StartTimer() returned nil")
	}

	time.Sleep(10 * time.Millisecond)

	elapsed := timer.Elapsed()
	if elapsed < 10*time.Millisecond {
		t.Errorf("Elapsed() = %v, expected >= 10ms", elapsed)
	}
}

func TestTimer_ElapsedMs(t *testing.T) {
	timer := StartTimer()

	time.Sleep(10 * time.Millisecond)

	ms := timer.ElapsedMs()
	if ms < 10.0 {
		t.Errorf("ElapsedMs() = %f, expected >= 10.0", ms)
	}
}

func TestTimer_Stop(t *testing.T) {
	timer := StartTimer()

	time.Sleep(10 * time.Millisecond)

	elapsed := timer.Stop()
	if elapsed < 10*time.Millisecond {
		t.Errorf("Stop() returned %v, expected >= 10ms", elapsed)
	}

	// After stop, timer should be reset
	time.Sleep(5 * time.Millisecond)
	elapsed2 := timer.Elapsed()
	if elapsed2 > 10*time.Millisecond {
		t.Errorf("expected timer to be reset after Stop(), got %v", elapsed2)
	}
}

func TestGetMetrics(t *testing.T) {
	m := GetMetrics()
	if m == nil {
		t.Fatal("GetMetrics() returned nil")
	}

	// Should return same instance
	m2 := GetMetrics()
	if m != m2 {
		t.Error("expected GetMetrics() to return same instance")
	}
}

func TestSetMetrics(t *testing.T) {
	original := appMetrics

	m := NewMetrics()
	SetMetrics(m)

	// Note: Can't easily test this fully due to sync.Once
	// but we can verify the function doesn't panic
	_ = original
}

func BenchmarkMetrics_RecordFrame(b *testing.B) {
	m := NewMetrics()
	duration := 16 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordFrame(duration)
	}
}

func BenchmarkMetrics_Snapshot(b *testing.B) {
	m := NewMetrics()
	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		m.RecordFrame(16 * time.Millisecond)
		m.RecordInput(1 * time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Snapshot()
	}
}
