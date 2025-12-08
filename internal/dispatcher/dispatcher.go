// Package dispatcher routes actions to handlers and coordinates execution.
package dispatcher

import (
	"runtime"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/dispatcher/hook"
	"github.com/dshills/keystorm/internal/input"
)

// Dispatcher routes actions to handlers and coordinates execution.
type Dispatcher struct {
	mu sync.RWMutex

	// Core components
	registry *Registry
	router   *Router

	// Editor subsystems
	engine      execctx.EngineInterface
	cursors     execctx.CursorManagerInterface
	modeManager execctx.ModeManagerInterface
	history     execctx.HistoryInterface
	renderer    execctx.RendererInterface

	// Configuration
	config Config

	// Metrics
	metrics *Metrics

	// Hooks (legacy simple hooks)
	preHooks  []PreDispatchHook
	postHooks []PostDispatchHook

	// Hook manager for priority-based hooks
	hookManager *hook.Manager

	// Async dispatch
	actionChan chan input.Action
	resultChan chan handler.Result
	done       chan struct{}
}

// New creates a new dispatcher with the given configuration.
func New(config Config) *Dispatcher {
	d := &Dispatcher{
		registry: NewRegistry(),
		router:   NewRouter(),
		config:   config,
		done:     make(chan struct{}),
	}

	if config.AsyncDispatch {
		bufSize := config.ActionBufferSize
		if bufSize <= 0 {
			bufSize = 100
		}
		d.actionChan = make(chan input.Action, bufSize)
		d.resultChan = make(chan handler.Result, bufSize)
	}

	if config.EnableMetrics {
		d.metrics = NewMetrics()
	}

	return d
}

// NewWithDefaults creates a new dispatcher with default configuration.
func NewWithDefaults() *Dispatcher {
	return New(DefaultConfig())
}

// SetEngine sets the text engine.
func (d *Dispatcher) SetEngine(engine execctx.EngineInterface) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.engine = engine
}

// SetCursors sets the cursor manager.
func (d *Dispatcher) SetCursors(cursors execctx.CursorManagerInterface) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cursors = cursors
}

// SetModeManager sets the mode manager.
func (d *Dispatcher) SetModeManager(modeManager execctx.ModeManagerInterface) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.modeManager = modeManager
}

// SetHistory sets the history/undo manager.
func (d *Dispatcher) SetHistory(history execctx.HistoryInterface) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.history = history
}

// SetRenderer sets the renderer.
func (d *Dispatcher) SetRenderer(renderer execctx.RendererInterface) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.renderer = renderer
}

// Engine returns the text engine.
func (d *Dispatcher) Engine() execctx.EngineInterface {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.engine
}

// Cursors returns the cursor manager.
func (d *Dispatcher) Cursors() execctx.CursorManagerInterface {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cursors
}

// ModeManager returns the mode manager.
func (d *Dispatcher) ModeManager() execctx.ModeManagerInterface {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.modeManager
}

// History returns the history manager.
func (d *Dispatcher) History() execctx.HistoryInterface {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.history
}

// Renderer returns the renderer.
func (d *Dispatcher) Renderer() execctx.RendererInterface {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.renderer
}

// Dispatch executes an action synchronously.
func (d *Dispatcher) Dispatch(action input.Action) handler.Result {
	return d.dispatchInternal(action, nil)
}

// DispatchWithContext executes an action with explicit input context.
func (d *Dispatcher) DispatchWithContext(action input.Action, inputCtx *input.Context) handler.Result {
	return d.dispatchInternal(action, inputCtx)
}

// dispatchInternal is the core dispatch logic.
func (d *Dispatcher) dispatchInternal(action input.Action, inputCtx *input.Context) handler.Result {
	startTime := time.Now()

	// Build execution context
	ctx := d.buildContext(inputCtx)

	// Apply repeat count from action if specified
	if action.Count > 0 {
		ctx.Count = action.Count
	}

	// Run pre-dispatch hooks
	if !d.runPreHooks(&action, ctx) {
		return handler.CancelledWithMessage("cancelled by hook")
	}

	// Find handler
	h := d.router.Route(action.Name)
	if h == nil {
		h = d.registry.Get(action.Name)
	}
	if h == nil {
		return handler.Errorf("no handler for action: %s", action.Name)
	}

	// Execute handler
	var result handler.Result
	if d.config.RecoverFromPanic {
		result = d.executeWithRecovery(h, action, ctx)
	} else {
		result = h.Handle(action, ctx)
	}

	// Process result (mode changes, view updates, etc.)
	d.processResult(action, result, ctx)

	// Run post-dispatch hooks
	d.runPostHooks(&action, ctx, &result)

	// Record metrics
	if d.metrics != nil {
		d.metrics.RecordDispatch(action.Name, time.Since(startTime), result.Status)
	}

	return result
}

// executeWithRecovery executes a handler with panic recovery.
func (d *Dispatcher) executeWithRecovery(h handler.Handler, action input.Action, ctx *execctx.ExecutionContext) (result handler.Result) {
	defer func() {
		if r := recover(); r != nil {
			stack := make([]byte, 4096)
			n := runtime.Stack(stack, false)

			result = handler.Errorf("handler panic for %s: %v\n%s", action.Name, r, string(stack[:n]))

			if d.metrics != nil {
				d.metrics.RecordPanic(action.Name)
			}
		}
	}()

	return h.Handle(action, ctx)
}

// buildContext builds an execution context from current state.
func (d *Dispatcher) buildContext(inputCtx *input.Context) *execctx.ExecutionContext {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ctx := execctx.NewWithInputContext(inputCtx)
	ctx.Engine = d.engine
	ctx.Cursors = d.cursors
	ctx.ModeManager = d.modeManager
	ctx.History = d.history
	ctx.Renderer = d.renderer

	return ctx
}

// processResult processes a handler result.
func (d *Dispatcher) processResult(action input.Action, result handler.Result, ctx *execctx.ExecutionContext) {
	// Handle mode change
	if result.ModeChange != "" && ctx.ModeManager != nil {
		_ = ctx.ModeManager.Switch(result.ModeChange)
	}

	// Handle view updates
	if ctx.Renderer != nil {
		if result.ViewUpdate.Redraw {
			ctx.Renderer.Redraw()
		} else if len(result.ViewUpdate.RedrawLines) > 0 {
			ctx.Renderer.RedrawLines(result.ViewUpdate.RedrawLines)
		}

		if result.ViewUpdate.ScrollTo != nil {
			st := result.ViewUpdate.ScrollTo
			if st.Center {
				ctx.Renderer.CenterOnLine(st.Line)
			} else {
				ctx.Renderer.ScrollTo(st.Line, st.Column)
			}
		} else if result.ViewUpdate.CenterLine != nil {
			ctx.Renderer.CenterOnLine(*result.ViewUpdate.CenterLine)
		} else {
			// Auto-scroll to keep cursor visible after any action
			d.ensureCursorVisible(ctx)
		}
	}
}

// ensureCursorVisible scrolls the viewport to keep the primary cursor visible.
func (d *Dispatcher) ensureCursorVisible(ctx *execctx.ExecutionContext) {
	if ctx.Cursors == nil || ctx.Engine == nil || ctx.Renderer == nil {
		return
	}

	// Get primary cursor position
	primary := ctx.Cursors.Primary()
	point := ctx.Engine.OffsetToPoint(primary.Cursor())

	// Check if cursor is visible
	if !ctx.Renderer.IsLineVisible(point.Line) {
		// Scroll to reveal cursor with some context
		ctx.Renderer.ScrollToReveal(point.Line, point.Column)
	}
}

// RegisterHandler registers a handler for an exact action name.
func (d *Dispatcher) RegisterHandler(actionName string, h handler.Handler) {
	d.registry.Register(actionName, h)
}

// RegisterHandlerFunc registers a handler function for an action name.
func (d *Dispatcher) RegisterHandlerFunc(actionName string, fn func(input.Action, *execctx.ExecutionContext) handler.Result) {
	d.registry.Register(actionName, handler.NewHandlerFunc(fn))
}

// RegisterNamespace registers a namespace handler.
func (d *Dispatcher) RegisterNamespace(namespace string, h handler.NamespaceHandler) {
	d.router.RegisterNamespace(namespace, h)
}

// UnregisterHandler removes a handler for an action name.
func (d *Dispatcher) UnregisterHandler(actionName string) {
	d.registry.Unregister(actionName)
}

// RegisterPreHook registers a pre-dispatch hook.
func (d *Dispatcher) RegisterPreHook(hook PreDispatchHook) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.preHooks = append(d.preHooks, hook)
}

// RegisterPostHook registers a post-dispatch hook.
func (d *Dispatcher) RegisterPostHook(hook PostDispatchHook) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.postHooks = append(d.postHooks, hook)
}

// runPreHooks runs all pre-dispatch hooks.
// Returns false if any hook cancels the action.
func (d *Dispatcher) runPreHooks(action *input.Action, ctx *execctx.ExecutionContext) bool {
	// Run hook manager first (priority-based hooks)
	d.mu.RLock()
	manager := d.hookManager
	d.mu.RUnlock()

	if manager != nil {
		if !manager.RunPreDispatch(action, ctx) {
			return false
		}
	}

	// Then run legacy simple hooks
	d.mu.RLock()
	hooks := make([]PreDispatchHook, len(d.preHooks))
	copy(hooks, d.preHooks)
	d.mu.RUnlock()

	for _, h := range hooks {
		if !h.PreDispatch(action, ctx) {
			return false
		}
	}
	return true
}

// runPostHooks runs all post-dispatch hooks.
func (d *Dispatcher) runPostHooks(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	// Run legacy simple hooks first
	d.mu.RLock()
	hooks := make([]PostDispatchHook, len(d.postHooks))
	copy(hooks, d.postHooks)
	d.mu.RUnlock()

	for _, h := range hooks {
		h.PostDispatch(action, ctx, result)
	}

	// Then run hook manager (priority-based hooks)
	d.mu.RLock()
	manager := d.hookManager
	d.mu.RUnlock()

	if manager != nil {
		manager.RunPostDispatch(action, ctx, result)
	}
}

// Start starts the async dispatch loop (if enabled).
func (d *Dispatcher) Start() {
	if !d.config.AsyncDispatch {
		return
	}

	go d.dispatchLoop()
}

// Stop stops the async dispatch loop.
func (d *Dispatcher) Stop() {
	select {
	case <-d.done:
		// Already closed
	default:
		close(d.done)
	}
}

// dispatchLoop processes actions asynchronously.
func (d *Dispatcher) dispatchLoop() {
	for {
		select {
		case action := <-d.actionChan:
			result := d.Dispatch(action)
			select {
			case d.resultChan <- result:
			default:
				// Result channel full, drop result
			}
		case <-d.done:
			return
		}
	}
}

// Actions returns the action channel for async dispatch.
// Returns nil if async dispatch is not enabled.
func (d *Dispatcher) Actions() chan<- input.Action {
	return d.actionChan
}

// Results returns the result channel for async dispatch.
// Returns nil if async dispatch is not enabled.
func (d *Dispatcher) Results() <-chan handler.Result {
	return d.resultChan
}

// Registry returns the handler registry.
func (d *Dispatcher) Registry() *Registry {
	return d.registry
}

// Router returns the action router.
func (d *Dispatcher) Router() *Router {
	return d.router
}

// Metrics returns the metrics collector (may be nil if disabled).
func (d *Dispatcher) Metrics() *Metrics {
	return d.metrics
}

// Config returns the dispatcher configuration.
func (d *Dispatcher) Config() Config {
	return d.config
}

// HookManager returns the hook manager (may be nil).
func (d *Dispatcher) HookManager() *hook.Manager {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hookManager
}

// SetHookManager sets the hook manager.
func (d *Dispatcher) SetHookManager(manager *hook.Manager) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hookManager = manager
}

// EnableHookManager creates and sets a new hook manager if not already set.
// Returns the hook manager.
func (d *Dispatcher) EnableHookManager() *hook.Manager {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.hookManager == nil {
		d.hookManager = hook.NewManager()
	}
	return d.hookManager
}
