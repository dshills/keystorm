package dispatcher

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/macro"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Mock implementations that satisfy the execctx interfaces

// mockEngine implements execctx.EngineInterface
type mockEngine struct {
	text     string
	modified bool
}

func newMockEngine(text string) *mockEngine {
	return &mockEngine{text: text}
}

func (e *mockEngine) Text() string { return e.text }
func (e *mockEngine) TextRange(start, end buffer.ByteOffset) string {
	if start < 0 {
		start = 0
	}
	if end > buffer.ByteOffset(len(e.text)) {
		end = buffer.ByteOffset(len(e.text))
	}
	if start >= end {
		return ""
	}
	return e.text[start:end]
}
func (e *mockEngine) LineText(line uint32) string { return e.text }
func (e *mockEngine) Len() buffer.ByteOffset      { return buffer.ByteOffset(len(e.text)) }
func (e *mockEngine) LineCount() uint32           { return 1 }

func (e *mockEngine) Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error) {
	e.modified = true
	return buffer.EditResult{
		OldRange: buffer.Range{Start: offset, End: offset},
		NewRange: buffer.Range{Start: offset, End: offset + buffer.ByteOffset(len(text))},
		OldText:  "",
		Delta:    int64(len(text)),
	}, nil
}

func (e *mockEngine) Delete(start, end buffer.ByteOffset) (buffer.EditResult, error) {
	e.modified = true
	oldText := ""
	if start >= 0 && end <= buffer.ByteOffset(len(e.text)) && start < end {
		oldText = e.text[start:end]
	}
	return buffer.EditResult{
		OldRange: buffer.Range{Start: start, End: end},
		NewRange: buffer.Range{Start: start, End: start},
		OldText:  oldText,
		Delta:    int64(start - end),
	}, nil
}

func (e *mockEngine) Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error) {
	e.modified = true
	oldText := ""
	if start >= 0 && end <= buffer.ByteOffset(len(e.text)) && start < end {
		oldText = e.text[start:end]
	}
	return buffer.EditResult{
		OldRange: buffer.Range{Start: start, End: end},
		NewRange: buffer.Range{Start: start, End: start + buffer.ByteOffset(len(text))},
		OldText:  oldText,
		Delta:    int64(len(text)) - int64(end-start),
	}, nil
}

func (e *mockEngine) LineStartOffset(line uint32) buffer.ByteOffset { return 0 }
func (e *mockEngine) LineEndOffset(line uint32) buffer.ByteOffset {
	return buffer.ByteOffset(len(e.text))
}
func (e *mockEngine) LineLen(line uint32) uint32 { return uint32(len(e.text)) }
func (e *mockEngine) OffsetToPoint(offset buffer.ByteOffset) buffer.Point {
	return buffer.Point{Line: 0, Column: uint32(offset)}
}
func (e *mockEngine) PointToOffset(point buffer.Point) buffer.ByteOffset {
	return buffer.ByteOffset(point.Column)
}
func (e *mockEngine) Snapshot() execctx.EngineReader { return e }
func (e *mockEngine) RevisionID() buffer.RevisionID  { return 1 }

// mockCursorManager implements execctx.CursorManagerInterface
type mockCursorManager struct {
	cursorSet *cursor.CursorSet
	modified  bool
}

func newMockCursorManager(pos int) *mockCursorManager {
	return &mockCursorManager{
		cursorSet: cursor.NewCursorSetAt(buffer.ByteOffset(pos)),
	}
}

func (m *mockCursorManager) Primary() cursor.Selection {
	return m.cursorSet.Primary()
}

func (m *mockCursorManager) SetPrimary(sel cursor.Selection) {
	m.cursorSet.SetPrimary(sel)
	m.modified = true
}

func (m *mockCursorManager) All() []cursor.Selection {
	return m.cursorSet.All()
}

func (m *mockCursorManager) Add(sel cursor.Selection) {
	m.cursorSet.Add(sel)
	m.modified = true
}

func (m *mockCursorManager) Clear() {
	m.cursorSet.Clear()
}

func (m *mockCursorManager) Count() int {
	return m.cursorSet.Count()
}

func (m *mockCursorManager) IsMulti() bool {
	return m.cursorSet.IsMulti()
}

func (m *mockCursorManager) HasSelection() bool {
	return m.cursorSet.HasSelection()
}

func (m *mockCursorManager) SetAll(sels []cursor.Selection) {
	m.cursorSet.SetAll(sels)
	m.modified = true
}

func (m *mockCursorManager) MapInPlace(f func(sel cursor.Selection) cursor.Selection) {
	m.cursorSet.MapInPlace(f)
	m.modified = true
}

func (m *mockCursorManager) Clone() *cursor.CursorSet {
	return m.cursorSet.Clone()
}

func (m *mockCursorManager) Clamp(maxOffset cursor.ByteOffset) {
	m.cursorSet.Clamp(maxOffset)
}

// mockMode implements execctx.ModeInterface
type mockMode struct {
	name        string
	displayName string
}

func (m *mockMode) Name() string        { return m.name }
func (m *mockMode) DisplayName() string { return m.displayName }

// mockModeManager implements execctx.ModeManagerInterface
type mockModeManager struct {
	currentMode *mockMode
	switched    bool
}

func newMockModeManager(mode string) *mockModeManager {
	return &mockModeManager{
		currentMode: &mockMode{name: mode, displayName: mode},
	}
}

func (m *mockModeManager) Current() execctx.ModeInterface {
	return m.currentMode
}

func (m *mockModeManager) CurrentName() string {
	return m.currentMode.name
}

func (m *mockModeManager) Switch(mode string) error {
	m.currentMode = &mockMode{name: mode, displayName: mode}
	m.switched = true
	return nil
}

func (m *mockModeManager) Push(name string) error {
	m.currentMode = &mockMode{name: name, displayName: name}
	return nil
}

func (m *mockModeManager) Pop() error {
	return nil
}

func (m *mockModeManager) IsMode(name string) bool {
	return m.currentMode.name == name
}

func (m *mockModeManager) IsAnyMode(names ...string) bool {
	for _, name := range names {
		if m.currentMode.name == name {
			return true
		}
	}
	return false
}

// mockHistory implements execctx.HistoryInterface
type mockHistory struct {
	undoCount int
	redoCount int
	grouping  bool
}

func newMockHistory() *mockHistory {
	return &mockHistory{}
}

func (h *mockHistory) BeginGroup(name string) { h.grouping = true }
func (h *mockHistory) EndGroup()              { h.grouping = false }
func (h *mockHistory) CancelGroup()           { h.grouping = false }
func (h *mockHistory) IsGrouping() bool       { return h.grouping }
func (h *mockHistory) CanUndo() bool          { return h.undoCount > 0 }
func (h *mockHistory) CanRedo() bool          { return h.redoCount > 0 }
func (h *mockHistory) UndoCount() int         { return h.undoCount }
func (h *mockHistory) RedoCount() int         { return h.redoCount }

// mockRenderer implements execctx.RendererInterface
type mockRenderer struct {
	redrawCalled  bool
	scrollToCalls int
	centerCalls   int
	firstLine     uint32
	lastLine      uint32
}

func newMockRenderer() *mockRenderer {
	return &mockRenderer{
		firstLine: 0,
		lastLine:  24,
	}
}

func (r *mockRenderer) Redraw()                    { r.redrawCalled = true }
func (r *mockRenderer) RedrawLines(lines []uint32) { r.redrawCalled = true }
func (r *mockRenderer) ScrollTo(line, col uint32)  { r.scrollToCalls++ }
func (r *mockRenderer) CenterOnLine(line uint32)   { r.centerCalls++ }
func (r *mockRenderer) VisibleLineRange() (uint32, uint32) {
	return r.firstLine, r.lastLine
}

// Integration Tests

func TestSystem_NewWithDefaults(t *testing.T) {
	sys := NewSystemWithDefaults()
	if sys == nil {
		t.Fatal("expected system to be created")
	}

	if sys.Dispatcher() == nil {
		t.Error("expected dispatcher to be initialized")
	}

	if sys.HookManager() == nil {
		t.Error("expected hook manager to be initialized")
	}

	if sys.MacroRecorder() == nil {
		t.Error("expected macro recorder to be initialized")
	}
}

func TestSystem_SetSubsystems(t *testing.T) {
	sys := NewSystemWithDefaults()

	engine := newMockEngine("test content")
	cursors := newMockCursorManager(0)
	modeManager := newMockModeManager("normal")
	history := newMockHistory()
	renderer := newMockRenderer()

	sys.SetSubsystems(
		engine,
		cursors,
		modeManager,
		history,
		renderer,
	)

	// Verify subsystems are set
	if sys.Dispatcher().Engine() == nil {
		t.Error("expected engine to be set")
	}
	if sys.Dispatcher().Cursors() == nil {
		t.Error("expected cursors to be set")
	}
	if sys.Dispatcher().ModeManager() == nil {
		t.Error("expected mode manager to be set")
	}
}

func TestSystem_Dispatch(t *testing.T) {
	sys := NewSystemWithDefaults()

	// Register a test handler
	var called bool
	sys.RegisterHandlerFunc("test.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		called = true
		return handler.Success().WithMessage("test passed")
	})

	result := sys.Dispatch(input.Action{Name: "test.action"})

	if !called {
		t.Error("expected handler to be called")
	}
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if result.Message != "test passed" {
		t.Errorf("expected message 'test passed', got '%s'", result.Message)
	}
}

func TestSystem_DispatchBatch(t *testing.T) {
	sys := NewSystemWithDefaults()

	var callCount int
	sys.RegisterHandlerFunc("batch.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		callCount++
		return handler.Success()
	})

	actions := []input.Action{
		{Name: "batch.test"},
		{Name: "batch.test"},
		{Name: "batch.test"},
	}

	results := sys.DispatchBatch(actions, false)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestSystem_DispatchBatch_StopOnError(t *testing.T) {
	sys := NewSystemWithDefaults()

	var callCount int
	sys.RegisterHandlerFunc("batch.error", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		callCount++
		if callCount == 2 {
			return handler.Errorf("error on call 2")
		}
		return handler.Success()
	})

	actions := []input.Action{
		{Name: "batch.error"},
		{Name: "batch.error"},
		{Name: "batch.error"},
	}

	results := sys.DispatchBatch(actions, true)

	// Should stop after the error
	if len(results) != 2 {
		t.Errorf("expected 2 results (stopped on error), got %d", len(results))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestSystem_RepeatHook(t *testing.T) {
	config := DefaultSystemConfig()
	config.EnableRepeatHook = true
	sys := NewSystem(config)

	// Register a repeatable action
	sys.RegisterHandlerFunc("editor.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	// Dispatch the action
	sys.Dispatch(input.Action{Name: "editor.test"})

	// Check that it was captured
	action, count := sys.LastRepeatableAction()
	if action == nil {
		t.Error("expected action to be captured")
	}
	if action != nil && action.Name != "editor.test" {
		t.Errorf("expected action name 'editor.test', got '%s'", action.Name)
	}
	_ = count
}

func TestSystem_AIContextHook(t *testing.T) {
	config := DefaultSystemConfig()
	config.EnableAIContext = true
	config.AIContextMaxChanges = 10
	sys := NewSystem(config)

	// Register a handler that produces edits
	sys.RegisterHandlerFunc("editor.edit", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithEdits([]handler.Edit{
			{Range: buffer.Range{Start: 0, End: 5}, OldText: "hello", NewText: "world"},
		})
	})

	sys.Dispatch(input.Action{Name: "editor.edit"})

	changes := sys.RecentChanges(10)
	if len(changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(changes))
	}
}

func TestSystem_Metrics(t *testing.T) {
	config := DefaultSystemConfig()
	config.DispatcherConfig = DefaultConfig().WithMetrics()
	sys := NewSystem(config)

	sys.RegisterHandlerFunc("metrics.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	// Dispatch some actions
	for i := 0; i < 10; i++ {
		sys.Dispatch(input.Action{Name: "metrics.test"})
	}

	metrics := sys.Metrics()
	if metrics == nil {
		t.Fatal("expected metrics to be enabled")
	}

	if metrics.TotalDispatches() != 10 {
		t.Errorf("expected 10 dispatches, got %d", metrics.TotalDispatches())
	}
}

func TestSystem_MacroRecording(t *testing.T) {
	sys := NewSystemWithDefaults()
	sys.EnableMacroDispatch()

	// Set up mocks
	engine := newMockEngine("test")
	cursors := newMockCursorManager(0)
	sys.SetSubsystems(engine, cursors, nil, nil, nil)

	recorder := sys.MacroRecorder()

	// Start recording
	err := recorder.StartRecording('a')
	if err != nil {
		t.Fatalf("failed to start recording: %v", err)
	}

	// Record some actions
	recorder.RecordAction(macro.RecordedAction{Name: "test.action1"})
	recorder.RecordAction(macro.RecordedAction{Name: "test.action2"})

	// Stop recording
	m, err := recorder.StopRecording()
	if err != nil {
		t.Fatalf("failed to stop recording: %v", err)
	}

	if len(m.Actions) != 2 {
		t.Errorf("expected 2 recorded actions, got %d", len(m.Actions))
	}

	// Check macro is stored
	stored := recorder.GetMacro('a')
	if stored == nil {
		t.Error("expected macro to be stored")
	}
}

func TestSystem_RegisterHook(t *testing.T) {
	sys := NewSystemWithDefaults()

	var preHookCalled, postHookCalled bool

	// Register custom hooks
	sys.RegisterPreHook(&testPreHook{
		name:     "test-pre",
		priority: 100,
		callback: func() { preHookCalled = true },
	})

	sys.RegisterPostHook(&testPostHook{
		name:     "test-post",
		priority: 100,
		callback: func() { postHookCalled = true },
	})

	sys.RegisterHandlerFunc("hook.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	sys.Dispatch(input.Action{Name: "hook.test"})

	if !preHookCalled {
		t.Error("expected pre hook to be called")
	}
	if !postHookCalled {
		t.Error("expected post hook to be called")
	}
}

type testPreHook struct {
	name     string
	priority int
	callback func()
}

func (h *testPreHook) Name() string  { return h.name }
func (h *testPreHook) Priority() int { return h.priority }
func (h *testPreHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.callback != nil {
		h.callback()
	}
	return true
}

type testPostHook struct {
	name     string
	priority int
	callback func()
}

func (h *testPostHook) Name() string  { return h.name }
func (h *testPostHook) Priority() int { return h.priority }
func (h *testPostHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	if h.callback != nil {
		h.callback()
	}
}

func TestSystem_CanHandle(t *testing.T) {
	sys := NewSystemWithDefaults()

	sys.RegisterHandlerFunc("custom.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	if !sys.CanHandle("custom.action") {
		t.Error("expected to handle custom.action")
	}

	// Built-in namespaces should be handled
	// Note: These may not work without full handler registration
}

func TestSystem_Stats(t *testing.T) {
	sys := NewSystemWithDefaults()

	stats := sys.Stats()

	if stats.NamespaceCount == 0 {
		t.Error("expected namespaces to be registered")
	}
	if stats.PreHookCount == 0 {
		t.Error("expected pre hooks to be registered")
	}
}

func TestSystem_Reset(t *testing.T) {
	config := DefaultSystemConfig()
	config.DispatcherConfig = DefaultConfig().WithMetrics()
	sys := NewSystem(config)

	sys.RegisterHandlerFunc("reset.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	sys.Dispatch(input.Action{Name: "reset.test"})

	// Verify metrics before reset
	if sys.Metrics().TotalDispatches() != 1 {
		t.Error("expected 1 dispatch before reset")
	}

	sys.Reset()

	// Verify reset
	if sys.Metrics().TotalDispatches() != 0 {
		t.Error("expected 0 dispatches after reset")
	}
}

func TestSystem_AsyncDispatch(t *testing.T) {
	config := DefaultSystemConfig()
	config.DispatcherConfig = DefaultConfig().WithAsyncDispatch(10)
	sys := NewSystem(config)

	var called atomic.Bool
	sys.RegisterHandlerFunc("async.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		called.Store(true)
		return handler.Success()
	})

	sys.Start()
	defer sys.Stop()

	// Send action via channel
	actions := sys.Actions()
	if actions == nil {
		t.Fatal("expected action channel")
	}

	actions <- input.Action{Name: "async.test"}

	// Wait for result
	results := sys.Results()
	select {
	case result := <-results:
		if result.Status != handler.StatusOK {
			t.Errorf("expected StatusOK, got %v", result.Status)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for result")
	}

	if !called.Load() {
		t.Error("expected handler to be called")
	}
}

func TestSystem_ListNamespaces(t *testing.T) {
	sys := NewSystemWithDefaults()

	namespaces := sys.ListNamespaces()

	// Should have registered namespaces
	if len(namespaces) == 0 {
		t.Error("expected namespaces to be listed")
	}

	// Check for expected namespaces
	found := make(map[string]bool)
	for _, ns := range namespaces {
		found[ns] = true
	}

	expectedNamespaces := []string{"cursor", "mode", "macro", "search", "view", "file", "window", "completion"}
	for _, expected := range expectedNamespaces {
		if !found[expected] {
			t.Errorf("expected namespace '%s' to be registered", expected)
		}
	}
}

// Performance Tests

func TestPerformanceMonitor_Basic(t *testing.T) {
	pm := NewPerformanceMonitor()

	pm.Record("test.action", time.Millisecond)
	pm.Record("test.action", 2*time.Millisecond)
	pm.Record("test.action", 3*time.Millisecond)

	stats := pm.ActionStats("test.action")
	if stats == nil {
		t.Fatal("expected stats for test.action")
	}

	if stats.Count != 3 {
		t.Errorf("expected count 3, got %d", stats.Count)
	}

	if stats.MinTime != time.Millisecond {
		t.Errorf("expected min 1ms, got %v", stats.MinTime)
	}

	if stats.MaxTime != 3*time.Millisecond {
		t.Errorf("expected max 3ms, got %v", stats.MaxTime)
	}
}

func TestPerformanceMonitor_SlowThreshold(t *testing.T) {
	pm := NewPerformanceMonitor()

	var alertCount int
	pm.SetSlowThreshold(time.Millisecond)
	pm.SetAlertCallback(func(action string, duration time.Duration) {
		alertCount++
	})

	pm.Record("fast.action", 500*time.Microsecond)
	pm.Record("slow.action", 2*time.Millisecond)

	if alertCount != 1 {
		t.Errorf("expected 1 alert, got %d", alertCount)
	}
}

func TestPerformanceMonitor_SlowestActions(t *testing.T) {
	pm := NewPerformanceMonitor()

	pm.Record("fast.action", 100*time.Microsecond)
	pm.Record("medium.action", time.Millisecond)
	pm.Record("slow.action", 10*time.Millisecond)

	slowest := pm.SlowestActions(2)

	if len(slowest) != 2 {
		t.Errorf("expected 2 slowest actions, got %d", len(slowest))
	}

	if slowest[0].Action != "slow.action" {
		t.Errorf("expected slow.action first, got %s", slowest[0].Action)
	}
}

func TestPerformanceMonitor_Disable(t *testing.T) {
	pm := NewPerformanceMonitor()
	pm.Enable(false)

	pm.Record("test.action", time.Millisecond)

	stats := pm.GlobalStats()
	if stats.Count != 0 {
		t.Error("expected no recordings when disabled")
	}
}

func TestBenchmark_RunAction(t *testing.T) {
	d := NewWithDefaults()
	d.RegisterHandlerFunc("bench.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	b := NewBenchmark(d)
	result := b.RunAction("bench.test", 100)

	if result.Iterations != 100 {
		t.Errorf("expected 100 iterations, got %d", result.Iterations)
	}

	if result.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", result.ErrorCount)
	}

	if result.SuccessRate != 100 {
		t.Errorf("expected 100%% success rate, got %.2f%%", result.SuccessRate)
	}

	if result.Throughput <= 0 {
		t.Error("expected positive throughput")
	}
}

func TestActionBatcher_Basic(t *testing.T) {
	var dispatched []input.Action
	batcher := NewActionBatcher(3, 0, func(actions []input.Action) {
		dispatched = actions
	})

	batcher.Add(input.Action{Name: "action1"})
	batcher.Add(input.Action{Name: "action2"})

	if batcher.Pending() != 2 {
		t.Errorf("expected 2 pending, got %d", batcher.Pending())
	}

	// Third action should trigger flush
	batcher.Add(input.Action{Name: "action3"})

	if len(dispatched) != 3 {
		t.Errorf("expected 3 dispatched actions, got %d", len(dispatched))
	}

	if batcher.Pending() != 0 {
		t.Errorf("expected 0 pending after flush, got %d", batcher.Pending())
	}
}

func TestActionBatcher_ManualFlush(t *testing.T) {
	var dispatched []input.Action
	batcher := NewActionBatcher(10, 0, func(actions []input.Action) {
		dispatched = actions
	})

	batcher.Add(input.Action{Name: "action1"})
	batcher.Add(input.Action{Name: "action2"})

	batcher.Flush()

	if len(dispatched) != 2 {
		t.Errorf("expected 2 dispatched actions, got %d", len(dispatched))
	}
}

func TestDispatchOptimizer_HotPaths(t *testing.T) {
	opt := NewDispatchOptimizer()

	opt.MarkHotPath("cursor.moveRight")

	if !opt.IsHotPath("cursor.moveRight") {
		t.Error("expected cursor.moveRight to be hot path")
	}

	if opt.IsHotPath("cursor.moveLeft") {
		t.Error("expected cursor.moveLeft not to be hot path")
	}
}

func TestDefaultHotPaths(t *testing.T) {
	paths := DefaultHotPaths()

	if len(paths) == 0 {
		t.Error("expected default hot paths")
	}

	// Check some expected paths
	found := make(map[string]bool)
	for _, p := range paths {
		found[p] = true
	}

	expected := []string{"cursor.moveLeft", "cursor.moveRight", "editor.insertChar"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("expected %s in default hot paths", e)
		}
	}
}
