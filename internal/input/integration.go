package input

import (
	"context"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/input/key"
	"github.com/dshills/keystorm/internal/input/macro"
	"github.com/dshills/keystorm/internal/input/palette"
)

// ActionDispatcher is the interface for dispatching actions to the editor.
type ActionDispatcher interface {
	// Dispatch sends an action to be executed by the editor.
	Dispatch(action Action) error

	// DispatchAsync sends an action asynchronously.
	DispatchAsync(action Action)
}

// StatusProvider provides status information for the UI.
type StatusProvider interface {
	// Mode returns the current mode name for status display.
	Mode() string

	// PendingKeys returns pending key sequence for status display.
	PendingKeys() string

	// IsRecording returns true if macro recording is active.
	IsRecording() bool

	// RecordingRegister returns the register being recorded to.
	RecordingRegister() rune
}

// MouseEventHandler processes mouse events and returns actions.
// This interface allows the InputSystem to work with mouse handling
// without creating an import cycle.
type MouseEventHandler interface {
	// HandleMouseEvent processes a mouse event and returns an action (or nil).
	HandleMouseEvent(x, y int, button, action uint8, modifiers key.Modifier) *Action

	// Reset clears all mouse handler state.
	Reset()
}

// InputSystem is the unified input subsystem that integrates all input components.
// It provides a high-level interface for the editor to process input events
// and receive actions.
type InputSystem struct {
	mu sync.RWMutex

	// Core components
	handler *Handler
	mouse   MouseEventHandler
	macro   *macro.Recorder
	macroP  *macro.Player
	palette *palette.Palette
	hooks   *HookManager
	metrics *Metrics

	// Configuration
	config SystemConfig

	// State
	closed bool

	// Dispatcher for output
	dispatcher ActionDispatcher
}

// SystemConfig configures the input system.
type SystemConfig struct {
	// Handler configuration
	Handler Config

	// Metrics enabled
	EnableMetrics bool

	// Hooks enabled
	EnableHooks bool
}

// DefaultSystemConfig returns sensible defaults for the input system.
func DefaultSystemConfig() SystemConfig {
	return SystemConfig{
		Handler:       DefaultConfig(),
		EnableMetrics: true,
		EnableHooks:   true,
	}
}

// NewInputSystem creates a new integrated input system.
// Note: Mouse handling must be set up separately via SetMouseHandler to avoid import cycles.
func NewInputSystem(config SystemConfig) *InputSystem {
	sys := &InputSystem{
		config:  config,
		handler: NewHandler(config.Handler),
		macro:   macro.NewRecorder(),
		hooks:   NewHookManager(),
		metrics: NewMetrics(),
	}

	// Create macro player
	sys.macroP = macro.NewPlayer(sys.macro)

	// Create command palette
	sys.palette = palette.New()

	// Configure metrics
	sys.metrics.SetEnabled(config.EnableMetrics)

	// Configure hooks
	sys.hooks.SetEnabled(config.EnableHooks)

	return sys
}

// SetMouseHandler sets the mouse event handler.
// This is separate from NewInputSystem to avoid import cycles with the mouse package.
func (s *InputSystem) SetMouseHandler(handler MouseEventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mouse = handler
}

// HandleKeyEvent processes a key event through the input system.
func (s *InputSystem) HandleKeyEvent(event key.Event) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	metrics := s.metrics
	hooks := s.hooks
	macro := s.macro
	handler := s.handler
	s.mu.RUnlock()

	// Start timing
	var timer *Timer
	if metrics.IsEnabled() {
		timer = metrics.StartKeyEventTimer()
	}

	// Record for macro if recording
	if macro.IsRecording() {
		macro.Record(event)
	}

	// Run pre-hooks
	s.mu.RLock()
	ctx := handler.Context()
	s.mu.RUnlock()

	if hooks.RunPreKeyEvent(&event, ctx) {
		if metrics.IsEnabled() {
			metrics.RecordHookConsumption()
		}
		if timer != nil {
			timer.Stop()
		}
		return
	}

	// Process through handler
	handler.HandleKeyEvent(event)

	// Stop timing
	if timer != nil {
		timer.Stop()
	}
}

// RecordMouseEvent records a mouse event in metrics (call after handling).
// The actual mouse event handling is done through MouseHandler().
func (s *InputSystem) RecordMouseEvent() {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	metrics := s.metrics
	s.mu.RUnlock()

	if metrics.IsEnabled() {
		metrics.RecordMouseEvent()
	}
}

// dispatchAction sends an action through the system.
func (s *InputSystem) dispatchAction(action Action) {
	s.mu.RLock()
	dispatcher := s.dispatcher
	metrics := s.metrics
	s.mu.RUnlock()

	if dispatcher != nil {
		if metrics.IsEnabled() {
			timer := metrics.StartActionTimer()
			dispatcher.DispatchAsync(action)
			timer.StopAction()
		} else {
			dispatcher.DispatchAsync(action)
		}
	}
}

// Actions returns the action channel for receiving dispatched actions.
func (s *InputSystem) Actions() <-chan Action {
	return s.handler.Actions()
}

// SetDispatcher sets the action dispatcher.
func (s *InputSystem) SetDispatcher(dispatcher ActionDispatcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher = dispatcher
}

// Handler returns the underlying input handler.
func (s *InputSystem) Handler() *Handler {
	return s.handler
}

// MouseHandler returns the mouse handler.
func (s *InputSystem) MouseHandler() MouseEventHandler {
	return s.mouse
}

// MacroRecorder returns the macro recorder.
func (s *InputSystem) MacroRecorder() *macro.Recorder {
	return s.macro
}

// MacroPlayer returns the macro player.
func (s *InputSystem) MacroPlayer() *macro.Player {
	return s.macroP
}

// Palette returns the command palette.
func (s *InputSystem) Palette() *palette.Palette {
	return s.palette
}

// Hooks returns the hook manager.
func (s *InputSystem) Hooks() *HookManager {
	return s.hooks
}

// Metrics returns the metrics tracker.
func (s *InputSystem) Metrics() *Metrics {
	return s.metrics
}

// StartMacroRecording starts recording to the specified register.
func (s *InputSystem) StartMacroRecording(register rune) error {
	return s.macro.StartRecording(register)
}

// StopMacroRecording stops the current macro recording.
func (s *InputSystem) StopMacroRecording() []key.Event {
	return s.macro.StopRecording()
}

// PlayMacro plays a macro from the specified register.
func (s *InputSystem) PlayMacro(register rune, count int) error {
	return s.macroP.Play(register, count, func(event key.Event) {
		s.HandleKeyEvent(event)
	})
}

// PlayMacroAsync plays a macro asynchronously.
func (s *InputSystem) PlayMacroAsync(register rune, count int, done chan<- struct{}) error {
	return s.macroP.PlayAsync(register, count, func(event key.Event) {
		s.HandleKeyEvent(event)
	}, done)
}

// CurrentMode returns the current input mode.
func (s *InputSystem) CurrentMode() string {
	return s.handler.CurrentMode()
}

// SwitchMode changes the input mode.
func (s *InputSystem) SwitchMode(mode string) error {
	return s.handler.SwitchMode(mode)
}

// PendingKeys returns the pending key sequence string.
func (s *InputSystem) PendingKeys() string {
	return s.handler.PendingKeys()
}

// IsRecording returns true if macro recording is active.
func (s *InputSystem) IsRecording() bool {
	return s.macro.IsRecording()
}

// RecordingRegister returns the register being recorded to.
func (s *InputSystem) RecordingRegister() rune {
	return s.macro.CurrentRegister()
}

// Mode implements StatusProvider.
func (s *InputSystem) Mode() string {
	return s.CurrentMode()
}

// UpdateContext updates the input context from editor state.
func (s *InputSystem) UpdateContext(editor EditorStateProvider) {
	s.handler.UpdateContext(editor)
}

// HealthCheck returns the current health status of the input system.
func (s *InputSystem) HealthCheck() HealthStatus {
	return s.metrics.HealthCheck(5 * time.Millisecond) // 5ms threshold
}

// Close shuts down the input system.
func (s *InputSystem) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	s.handler.Close()
	if s.mouse != nil {
		s.mouse.Reset()
	}
}

// IsClosed returns true if the system has been closed.
func (s *InputSystem) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// ActionConsumer processes actions from the input system.
// This is a convenience function for setting up the action processing loop.
func (s *InputSystem) ActionConsumer(ctx context.Context, handler func(Action)) {
	actions := s.Actions()
	for {
		select {
		case <-ctx.Done():
			return
		case action, ok := <-actions:
			if !ok {
				return
			}
			handler(action)
		}
	}
}

// ActionBridge bridges the input system to an ActionDispatcher.
// It consumes actions from the input system and dispatches them.
type ActionBridge struct {
	system     *InputSystem
	dispatcher ActionDispatcher
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewActionBridge creates a bridge between the input system and a dispatcher.
func NewActionBridge(system *InputSystem, dispatcher ActionDispatcher) *ActionBridge {
	return &ActionBridge{
		system:     system,
		dispatcher: dispatcher,
	}
}

// Start begins processing actions from the input system.
func (b *ActionBridge) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.system.ActionConsumer(ctx, func(action Action) {
			b.dispatcher.DispatchAsync(action)
		})
	}()
}

// Stop stops processing actions.
func (b *ActionBridge) Stop() {
	if b.cancel != nil {
		b.cancel()
		b.wg.Wait()
	}
}

// SimpleDispatcher is a basic dispatcher implementation for testing.
type SimpleDispatcher struct {
	mu       sync.Mutex
	actions  []Action
	handlers map[string]func(Action)
}

// NewSimpleDispatcher creates a new simple dispatcher.
func NewSimpleDispatcher() *SimpleDispatcher {
	return &SimpleDispatcher{
		actions:  make([]Action, 0),
		handlers: make(map[string]func(Action)),
	}
}

// Dispatch sends an action synchronously.
func (d *SimpleDispatcher) Dispatch(action Action) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.actions = append(d.actions, action)

	if handler, ok := d.handlers[action.Name]; ok {
		handler(action)
	}

	return nil
}

// DispatchAsync sends an action asynchronously.
func (d *SimpleDispatcher) DispatchAsync(action Action) {
	go d.Dispatch(action)
}

// RegisterHandler registers a handler for a specific action.
func (d *SimpleDispatcher) RegisterHandler(actionName string, handler func(Action)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[actionName] = handler
}

// Actions returns all dispatched actions.
func (d *SimpleDispatcher) Actions() []Action {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]Action, len(d.actions))
	copy(result, d.actions)
	return result
}

// Clear removes all recorded actions.
func (d *SimpleDispatcher) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.actions = make([]Action, 0)
}
