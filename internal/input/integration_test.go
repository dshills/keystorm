package input

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/input/key"
)

// ==================== Hook Manager Tests ====================

func TestHookManagerRegister(t *testing.T) {
	m := NewHookManager()

	hook := BaseHook{}
	id := m.Register(hook)

	if id == 0 {
		t.Error("expected non-zero hook ID")
	}

	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1", m.Count())
	}
}

func TestHookManagerPriority(t *testing.T) {
	m := NewHookManager()

	// Register hooks in reverse priority order
	m.RegisterWithPriority(BaseHook{}, HookPriorityLow)
	m.RegisterWithPriority(BaseHook{}, HookPriorityHigh)
	m.RegisterWithPriority(BaseHook{}, HookPriorityNormal)

	// Verify they get sorted
	hooks := m.List()
	if len(hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(hooks))
	}

	// Force sort by running hooks
	ctx := NewContext()
	event := key.NewRuneEvent('a', key.ModNone)
	m.RunPreKeyEvent(&event, ctx)

	// Check order
	hooks = m.List()
	if hooks[0].Priority != HookPriorityHigh {
		t.Errorf("first hook priority = %d, want %d", hooks[0].Priority, HookPriorityHigh)
	}
	if hooks[1].Priority != HookPriorityNormal {
		t.Errorf("second hook priority = %d, want %d", hooks[1].Priority, HookPriorityNormal)
	}
	if hooks[2].Priority != HookPriorityLow {
		t.Errorf("third hook priority = %d, want %d", hooks[2].Priority, HookPriorityLow)
	}
}

func TestHookManagerNamed(t *testing.T) {
	m := NewHookManager()

	id := m.RegisterNamed(BaseHook{}, "myHook")

	reg := m.GetByName("myHook")
	if reg == nil {
		t.Fatal("expected to find hook by name")
	}

	if reg.ID != id {
		t.Errorf("ID mismatch: got %d, want %d", reg.ID, id)
	}

	// Unregister by name
	if !m.UnregisterByName("myHook") {
		t.Error("expected UnregisterByName to return true")
	}

	if m.Count() != 0 {
		t.Errorf("expected 0 hooks after unregister, got %d", m.Count())
	}
}

func TestHookManagerEnable(t *testing.T) {
	m := NewHookManager()

	consumed := false
	hook := FuncHook{
		PreKeyEventFunc: func(*key.Event, *Context) bool {
			consumed = true
			return true
		},
	}
	m.Register(hook)

	// Disable hooks
	m.SetEnabled(false)

	ctx := NewContext()
	event := key.NewRuneEvent('a', key.ModNone)
	m.RunPreKeyEvent(&event, ctx)

	if consumed {
		t.Error("hook should not run when disabled")
	}

	// Re-enable
	m.SetEnabled(true)
	m.RunPreKeyEvent(&event, ctx)

	if !consumed {
		t.Error("hook should run when enabled")
	}
}

func TestFuncHook(t *testing.T) {
	preKeyCalled := false
	postKeyCalled := false
	preActionCalled := false

	hook := FuncHook{
		PreKeyEventFunc: func(*key.Event, *Context) bool {
			preKeyCalled = true
			return false
		},
		PostKeyEventFunc: func(*key.Event, *Action, *Context) {
			postKeyCalled = true
		},
		PreActionFunc: func(*Action, *Context) bool {
			preActionCalled = true
			return false
		},
	}

	ctx := NewContext()
	event := key.NewRuneEvent('a', key.ModNone)
	action := Action{Name: "test"}

	hook.PreKeyEvent(&event, ctx)
	hook.PostKeyEvent(&event, &action, ctx)
	hook.PreAction(&action, ctx)

	if !preKeyCalled {
		t.Error("PreKeyEventFunc not called")
	}
	if !postKeyCalled {
		t.Error("PostKeyEventFunc not called")
	}
	if !preActionCalled {
		t.Error("PreActionFunc not called")
	}
}

func TestFilterHook(t *testing.T) {
	// Block 'a' key
	hook := FilterHook{
		KeyEventFilter: func(e *key.Event, _ *Context) bool {
			return e.Rune == 'a'
		},
	}

	ctx := NewContext()

	eventA := key.NewRuneEvent('a', key.ModNone)
	if !hook.PreKeyEvent(&eventA, ctx) {
		t.Error("filter should block 'a'")
	}

	eventB := key.NewRuneEvent('b', key.ModNone)
	if hook.PreKeyEvent(&eventB, ctx) {
		t.Error("filter should not block 'b'")
	}
}

// ==================== Metrics Tests ====================

func TestMetricsBasic(t *testing.T) {
	m := NewMetrics()

	// Record some events
	m.RecordKeyEvent(time.Millisecond)
	m.RecordKeyEvent(2 * time.Millisecond)
	m.RecordKeyEvent(3 * time.Millisecond)
	m.RecordMouseEvent()
	m.RecordAction(500 * time.Microsecond)

	if m.KeyEventsTotal() != 3 {
		t.Errorf("KeyEventsTotal() = %d, want 3", m.KeyEventsTotal())
	}

	if m.MouseEventsTotal() != 1 {
		t.Errorf("MouseEventsTotal() = %d, want 1", m.MouseEventsTotal())
	}

	if m.ActionsTotal() != 1 {
		t.Errorf("ActionsTotal() = %d, want 1", m.ActionsTotal())
	}
}

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics()

	// Record latencies
	for i := 0; i < 100; i++ {
		m.RecordKeyEvent(time.Duration(i+1) * time.Microsecond)
	}

	snap := m.Snapshot()

	if snap.KeyEventsTotal != 100 {
		t.Errorf("KeyEventsTotal = %d, want 100", snap.KeyEventsTotal)
	}

	if snap.AvgKeyLatency <= 0 {
		t.Error("AvgKeyLatency should be > 0")
	}

	if snap.MaxKeyLatency != 100*time.Microsecond {
		t.Errorf("MaxKeyLatency = %v, want 100us", snap.MaxKeyLatency)
	}
}

func TestMetricsDisabled(t *testing.T) {
	m := NewMetrics()
	m.SetEnabled(false)

	m.RecordKeyEvent(time.Millisecond)

	if m.KeyEventsTotal() != 0 {
		t.Error("metrics should not record when disabled")
	}
}

func TestMetricsHealthCheck(t *testing.T) {
	m := NewMetrics()

	// Should be healthy initially
	status := m.HealthCheck(5 * time.Millisecond)
	if !status.Healthy {
		t.Error("should be healthy initially")
	}

	// Record dropped event
	m.RecordDroppedEvent()
	status = m.HealthCheck(5 * time.Millisecond)
	if status.Healthy {
		t.Error("should be unhealthy after dropped event")
	}
}

func TestMetricsTimer(t *testing.T) {
	m := NewMetrics()

	timer := m.StartKeyEventTimer()
	time.Sleep(time.Millisecond)
	elapsed := timer.Stop()

	if elapsed < time.Millisecond {
		t.Errorf("elapsed time %v should be >= 1ms", elapsed)
	}

	if m.KeyEventsTotal() != 1 {
		t.Error("timer should record key event")
	}
}

func TestMetricsReset(t *testing.T) {
	m := NewMetrics()

	m.RecordKeyEvent(time.Millisecond)
	m.RecordDroppedEvent()

	m.Reset()

	if m.KeyEventsTotal() != 0 {
		t.Error("KeyEventsTotal should be 0 after reset")
	}

	if m.DroppedEvents() != 0 {
		t.Error("DroppedEvents should be 0 after reset")
	}
}

// ==================== Integration Tests ====================

func TestInputSystemBasic(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	if sys.Handler() == nil {
		t.Error("Handler should not be nil")
	}

	// MouseHandler is nil by default (set separately via SetMouseHandler)
	if sys.MouseHandler() != nil {
		t.Error("MouseHandler should be nil by default")
	}

	if sys.MacroRecorder() == nil {
		t.Error("MacroRecorder should not be nil")
	}

	if sys.Hooks() == nil {
		t.Error("Hooks should not be nil")
	}

	if sys.Metrics() == nil {
		t.Error("Metrics should not be nil")
	}
}

func TestInputSystemKeyEvent(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	// Process a key event
	event := key.NewRuneEvent('j', key.ModNone)
	sys.HandleKeyEvent(event)

	// Check metrics
	if sys.Metrics().KeyEventsTotal() != 1 {
		t.Errorf("KeyEventsTotal = %d, want 1", sys.Metrics().KeyEventsTotal())
	}
}

func TestInputSystemMouseMetrics(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	// Record a mouse event (actual handling is through MouseHandler)
	sys.RecordMouseEvent()

	// Check metrics
	if sys.Metrics().MouseEventsTotal() != 1 {
		t.Errorf("MouseEventsTotal = %d, want 1", sys.Metrics().MouseEventsTotal())
	}
}

func TestInputSystemMacro(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	// Start recording
	err := sys.StartMacroRecording('a')
	if err != nil {
		t.Fatalf("StartMacroRecording failed: %v", err)
	}

	if !sys.IsRecording() {
		t.Error("should be recording")
	}

	if sys.RecordingRegister() != 'a' {
		t.Errorf("RecordingRegister = %c, want 'a'", sys.RecordingRegister())
	}

	// Record some keys
	sys.HandleKeyEvent(key.NewRuneEvent('h', key.ModNone))
	sys.HandleKeyEvent(key.NewRuneEvent('i', key.ModNone))

	// Stop recording
	events := sys.StopMacroRecording()
	if len(events) != 2 {
		t.Errorf("recorded %d events, want 2", len(events))
	}

	if sys.IsRecording() {
		t.Error("should not be recording after stop")
	}
}

func TestInputSystemMode(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	// Check initial mode
	if sys.CurrentMode() != "normal" {
		t.Errorf("CurrentMode = %s, want normal", sys.CurrentMode())
	}

	// Switch mode
	err := sys.SwitchMode("insert")
	if err != nil {
		t.Fatalf("SwitchMode failed: %v", err)
	}

	if sys.CurrentMode() != "insert" {
		t.Errorf("CurrentMode = %s, want insert", sys.CurrentMode())
	}
}

func TestInputSystemHealthCheck(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	status := sys.HealthCheck()
	if !status.Healthy {
		t.Errorf("should be healthy: %s", status.Message)
	}
}

func TestInputSystemClose(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())

	sys.Close()

	if !sys.IsClosed() {
		t.Error("should be closed")
	}

	// Double close should not panic
	sys.Close()
}

// ==================== Simple Dispatcher Tests ====================

func TestSimpleDispatcher(t *testing.T) {
	d := NewSimpleDispatcher()

	action := Action{Name: "test.action", Count: 5}
	err := d.Dispatch(action)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	actions := d.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Name != "test.action" {
		t.Errorf("action name = %s, want test.action", actions[0].Name)
	}
}

func TestSimpleDispatcherHandler(t *testing.T) {
	d := NewSimpleDispatcher()

	handled := false
	d.RegisterHandler("test.action", func(a Action) {
		handled = true
	})

	d.Dispatch(Action{Name: "test.action"})

	if !handled {
		t.Error("handler should have been called")
	}
}

func TestSimpleDispatcherClear(t *testing.T) {
	d := NewSimpleDispatcher()

	d.Dispatch(Action{Name: "test"})
	d.Clear()

	if len(d.Actions()) != 0 {
		t.Error("actions should be cleared")
	}
}

// ==================== Action Bridge Tests ====================

func TestActionBridge(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	d := NewSimpleDispatcher()
	bridge := NewActionBridge(sys, d)

	bridge.Start()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	bridge.Stop()
}

// ==================== Concurrent Tests ====================

func TestHookManagerConcurrent(t *testing.T) {
	m := NewHookManager()

	var wg sync.WaitGroup

	// Concurrent registration
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				id := m.Register(BaseHook{})
				m.Unregister(id)
			}
		}()
	}

	// Concurrent hook execution
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := NewContext()
			event := key.NewRuneEvent('a', key.ModNone)
			for j := 0; j < 100; j++ {
				m.RunPreKeyEvent(&event, ctx)
			}
		}()
	}

	wg.Wait()
}

func TestMetricsConcurrent(t *testing.T) {
	m := NewMetrics()

	var wg sync.WaitGroup

	// Concurrent recording
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				m.RecordKeyEvent(time.Microsecond)
			}
		}()
	}

	// Concurrent snapshots
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.Snapshot()
			}
		}()
	}

	wg.Wait()

	if m.KeyEventsTotal() != 10000 {
		t.Errorf("KeyEventsTotal = %d, want 10000", m.KeyEventsTotal())
	}
}

func TestInputSystemConcurrent(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	var wg sync.WaitGroup

	// Concurrent key events
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				sys.HandleKeyEvent(key.NewRuneEvent('a', key.ModNone))
			}
		}()
	}

	// Concurrent mouse event metrics recording
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				sys.RecordMouseEvent()
			}
		}()
	}

	wg.Wait()
}

// ==================== Action Consumer Test ====================

func TestActionConsumer(t *testing.T) {
	sys := NewInputSystem(DefaultSystemConfig())
	defer sys.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received []Action
	var mu sync.Mutex

	go sys.ActionConsumer(ctx, func(action Action) {
		mu.Lock()
		received = append(received, action)
		mu.Unlock()
	})

	// Give consumer time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel and verify shutdown
	cancel()
	time.Sleep(10 * time.Millisecond)
}
