package dispatcher

import (
	"sync"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/completion"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/cursor"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/editor"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/file"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/macro"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/mode"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/operator"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/search"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/view"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/window"
	"github.com/dshills/keystorm/internal/dispatcher/hook"
	"github.com/dshills/keystorm/internal/input"
)

// System provides a unified facade for the dispatcher subsystem.
// It coordinates the dispatcher, handlers, hooks, and subsystems into
// a cohesive system that can be easily integrated with the editor.
type System struct {
	mu sync.RWMutex

	// Core dispatcher
	dispatcher *Dispatcher

	// Hook management
	hookManager   *hook.Manager
	repeatHook    *hook.RepeatHook
	aiContextHook *hook.AIContextHook

	// Handler instances (for direct access/configuration)
	cursorHandler     *cursor.Handler
	motionHandler     *cursor.MotionHandler
	insertHandler     *editor.InsertHandler
	deleteHandler     *editor.DeleteHandler
	yankHandler       *editor.YankHandler
	indentHandler     *editor.IndentHandler
	modeHandler       *mode.ModeHandler
	operatorHandler   *operator.OperatorHandler
	searchHandler     *search.Handler
	viewHandler       *view.Handler
	fileHandler       *file.Handler
	windowHandler     *window.Handler
	completionHandler *completion.Handler
	macroHandler      *macro.Handler

	// Macro recorder (shared between handler and system)
	macroRecorder *macro.DefaultMacroRecorder

	// Context pool for performance
	contextPool *sync.Pool

	// Configuration
	config SystemConfig

	// State
	started bool
}

// SystemConfig holds configuration for the dispatcher system.
type SystemConfig struct {
	// DispatcherConfig is the underlying dispatcher configuration.
	DispatcherConfig Config

	// EnableRepeatHook enables the Vim "." repeat command hook.
	EnableRepeatHook bool

	// EnableAIContext enables AI context tracking hook.
	EnableAIContext bool

	// AIContextMaxChanges limits stored AI context changes.
	AIContextMaxChanges int

	// EnableContextPool uses sync.Pool for ExecutionContext reuse.
	EnableContextPool bool

	// IndentConfig for editor indent handler.
	TabWidth   int
	IndentSize int
	UseTabs    bool
}

// DefaultSystemConfig returns a configuration with sensible defaults.
func DefaultSystemConfig() SystemConfig {
	return SystemConfig{
		DispatcherConfig:    DefaultConfig().WithMetrics(),
		EnableRepeatHook:    true,
		EnableAIContext:     true,
		AIContextMaxChanges: 1000,
		EnableContextPool:   true,
	}
}

// NewSystem creates a new dispatcher system with the given configuration.
func NewSystem(config SystemConfig) *System {
	s := &System{
		config:     config,
		dispatcher: New(config.DispatcherConfig),
	}

	// Initialize context pool
	if config.EnableContextPool {
		s.contextPool = &sync.Pool{
			New: func() interface{} {
				return execctx.New()
			},
		}
	}

	// Create macro recorder
	s.macroRecorder = macro.NewDefaultMacroRecorder()

	// Initialize handlers
	s.initializeHandlers(config)

	// Register all handlers
	s.registerHandlers()

	// Initialize hooks
	s.initializeHooks(config)

	return s
}

// NewSystemWithDefaults creates a system with default configuration.
func NewSystemWithDefaults() *System {
	return NewSystem(DefaultSystemConfig())
}

// initializeHandlers creates all handler instances.
func (s *System) initializeHandlers(config SystemConfig) {
	// Cursor handlers
	s.cursorHandler = cursor.NewHandler()
	s.motionHandler = cursor.NewMotionHandler()

	// Editor handlers
	s.insertHandler = editor.NewInsertHandler()
	s.deleteHandler = editor.NewDeleteHandler()
	s.yankHandler = editor.NewYankHandler()
	if config.TabWidth > 0 || config.IndentSize > 0 {
		tabWidth := config.TabWidth
		if tabWidth == 0 {
			tabWidth = 4
		}
		indentSize := config.IndentSize
		if indentSize == 0 {
			indentSize = 4
		}
		s.indentHandler = editor.NewIndentHandlerWithConfig(tabWidth, indentSize, config.UseTabs)
	} else {
		s.indentHandler = editor.NewIndentHandler()
	}

	// Mode and operator handlers
	s.modeHandler = mode.NewModeHandler()
	s.operatorHandler = operator.NewOperatorHandler()

	// Additional handlers
	s.searchHandler = search.NewHandler()
	s.viewHandler = view.NewHandler()
	s.fileHandler = file.NewHandler()
	s.windowHandler = window.NewHandler()
	s.completionHandler = completion.NewHandler()
	s.macroHandler = macro.NewHandlerWithRecorder(s.macroRecorder)
}

// registerHandlers registers all handlers with the dispatcher.
func (s *System) registerHandlers() {
	router := s.dispatcher.Router()

	// Register namespace handlers
	router.RegisterNamespace("cursor", s.cursorHandler)
	router.RegisterNamespace("motion", s.motionHandler)
	router.RegisterNamespace("editor", s.insertHandler) // insert handles editor.insert*
	router.RegisterNamespace("mode", s.modeHandler)
	router.RegisterNamespace("operator", s.operatorHandler)
	router.RegisterNamespace("search", s.searchHandler)
	router.RegisterNamespace("view", s.viewHandler)
	router.RegisterNamespace("file", s.fileHandler)
	router.RegisterNamespace("window", s.windowHandler)
	router.RegisterNamespace("completion", s.completionHandler)
	router.RegisterNamespace("macro", s.macroHandler)

	// Register additional editor handlers for specific actions
	// Delete, yank, indent share the "editor" namespace so we register by action
	s.registerEditorActions()
}

// registerEditorActions registers individual editor actions.
func (s *System) registerEditorActions() {
	registry := s.dispatcher.Registry()

	// Delete actions
	for _, action := range []string{
		editor.ActionDeleteChar, editor.ActionDeleteCharBack, editor.ActionDeleteLine,
		editor.ActionDeleteToEnd, editor.ActionDeleteSelection, editor.ActionDeleteWord,
		editor.ActionDeleteWordBack,
	} {
		registry.Register(action, handler.NewNamespaceAdapter(s.deleteHandler))
	}

	// Yank actions
	for _, action := range []string{
		editor.ActionYankSelection, editor.ActionYankLine, editor.ActionYankToEnd,
		editor.ActionYankWord, editor.ActionPasteAfter, editor.ActionPasteBefore,
	} {
		registry.Register(action, handler.NewNamespaceAdapter(s.yankHandler))
	}

	// Indent actions
	for _, action := range []string{
		editor.ActionIndent, editor.ActionOutdent, editor.ActionAutoIndent,
	} {
		registry.Register(action, handler.NewNamespaceAdapter(s.indentHandler))
	}
}

// initializeHooks sets up the hook system.
func (s *System) initializeHooks(config SystemConfig) {
	s.hookManager = hook.NewManager()
	s.dispatcher.SetHookManager(s.hookManager)

	// Count limit hook (always enabled)
	if config.DispatcherConfig.MaxRepeatCount > 0 {
		s.hookManager.RegisterPre(hook.NewCountLimitHook(config.DispatcherConfig.MaxRepeatCount))
	}

	// Repeat hook
	if config.EnableRepeatHook {
		s.repeatHook = hook.NewRepeatHook()
		s.hookManager.RegisterPost(s.repeatHook)
	}

	// AI context hook
	if config.EnableAIContext {
		s.aiContextHook = hook.NewAIContextHook(config.AIContextMaxChanges)
		s.hookManager.RegisterPost(s.aiContextHook)
	}
}

// SetEngine sets the text engine for all operations.
func (s *System) SetEngine(engine execctx.EngineInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher.SetEngine(engine)
}

// SetCursors sets the cursor manager.
func (s *System) SetCursors(cursors execctx.CursorManagerInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher.SetCursors(cursors)
}

// SetModeManager sets the mode manager.
func (s *System) SetModeManager(modeManager execctx.ModeManagerInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher.SetModeManager(modeManager)
}

// SetHistory sets the history/undo manager.
func (s *System) SetHistory(history execctx.HistoryInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher.SetHistory(history)
}

// SetRenderer sets the renderer.
func (s *System) SetRenderer(renderer execctx.RendererInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher.SetRenderer(renderer)
}

// SetSubsystems sets all subsystems at once.
func (s *System) SetSubsystems(
	engine execctx.EngineInterface,
	cursors execctx.CursorManagerInterface,
	modeManager execctx.ModeManagerInterface,
	history execctx.HistoryInterface,
	renderer execctx.RendererInterface,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dispatcher.SetEngine(engine)
	s.dispatcher.SetCursors(cursors)
	s.dispatcher.SetModeManager(modeManager)
	s.dispatcher.SetHistory(history)
	s.dispatcher.SetRenderer(renderer)
}

// Dispatch executes an action synchronously.
func (s *System) Dispatch(action input.Action) handler.Result {
	return s.dispatcher.Dispatch(action)
}

// DispatchWithContext executes an action with explicit input context.
func (s *System) DispatchWithContext(action input.Action, inputCtx *input.Context) handler.Result {
	return s.dispatcher.DispatchWithContext(action, inputCtx)
}

// DispatchBatch executes multiple actions in sequence.
// Stops on first error if stopOnError is true.
func (s *System) DispatchBatch(actions []input.Action, stopOnError bool) []handler.Result {
	results := make([]handler.Result, 0, len(actions))

	for _, action := range actions {
		result := s.Dispatch(action)
		results = append(results, result)

		if stopOnError && result.Status == handler.StatusError {
			break
		}
	}

	return results
}

// Start starts the async dispatch loop if enabled.
func (s *System) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}

	s.dispatcher.Start()
	s.started = true
}

// Stop stops the async dispatch loop.
func (s *System) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	s.dispatcher.Stop()
	s.started = false
}

// IsStarted returns true if the system is running.
func (s *System) IsStarted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// Actions returns the action channel for async dispatch.
func (s *System) Actions() chan<- input.Action {
	return s.dispatcher.Actions()
}

// Results returns the result channel for async dispatch.
func (s *System) Results() <-chan handler.Result {
	return s.dispatcher.Results()
}

// Dispatcher returns the underlying dispatcher.
func (s *System) Dispatcher() *Dispatcher {
	return s.dispatcher
}

// HookManager returns the hook manager.
func (s *System) HookManager() *hook.Manager {
	return s.hookManager
}

// RepeatHook returns the repeat hook (may be nil if disabled).
func (s *System) RepeatHook() *hook.RepeatHook {
	return s.repeatHook
}

// AIContextHook returns the AI context hook (may be nil if disabled).
func (s *System) AIContextHook() *hook.AIContextHook {
	return s.aiContextHook
}

// MacroRecorder returns the macro recorder.
func (s *System) MacroRecorder() *macro.DefaultMacroRecorder {
	return s.macroRecorder
}

// Metrics returns the metrics collector (may be nil if disabled).
func (s *System) Metrics() *Metrics {
	return s.dispatcher.Metrics()
}

// LastRepeatableAction returns the last action that can be repeated with ".".
func (s *System) LastRepeatableAction() (*input.Action, int) {
	if s.repeatHook == nil {
		return nil, 0
	}
	return s.repeatHook.LastAction()
}

// RepeatLastAction dispatches the last repeatable action.
// Returns NoOp if no action is available.
func (s *System) RepeatLastAction() handler.Result {
	action, count := s.LastRepeatableAction()
	if action == nil {
		return handler.NoOpWithMessage("no action to repeat")
	}

	// Apply count
	if count > 0 {
		action.Count = count
	}

	return s.Dispatch(*action)
}

// RecentChanges returns recent edit changes for AI context.
func (s *System) RecentChanges(n int) []hook.ChangeRecord {
	if s.aiContextHook == nil {
		return nil
	}
	return s.aiContextHook.RecentChanges(n)
}

// RegisterHook registers a hook with the hook manager.
func (s *System) RegisterHook(h hook.Hook) {
	s.hookManager.Register(h)
}

// RegisterPreHook registers a pre-dispatch hook.
func (s *System) RegisterPreHook(h hook.PreDispatchHook) {
	s.hookManager.RegisterPre(h)
}

// RegisterPostHook registers a post-dispatch hook.
func (s *System) RegisterPostHook(h hook.PostDispatchHook) {
	s.hookManager.RegisterPost(h)
}

// UnregisterHook removes a hook by name.
func (s *System) UnregisterHook(name string) bool {
	return s.hookManager.Unregister(name)
}

// RegisterHandler registers a handler for a specific action.
func (s *System) RegisterHandler(actionName string, h handler.Handler) {
	s.dispatcher.RegisterHandler(actionName, h)
}

// RegisterHandlerFunc registers a handler function for an action.
func (s *System) RegisterHandlerFunc(actionName string, fn func(input.Action, *execctx.ExecutionContext) handler.Result) {
	s.dispatcher.RegisterHandlerFunc(actionName, fn)
}

// RegisterNamespace registers a namespace handler.
func (s *System) RegisterNamespace(namespace string, h handler.NamespaceHandler) {
	s.dispatcher.RegisterNamespace(namespace, h)
}

// CanHandle returns true if the system can handle the action.
func (s *System) CanHandle(actionName string) bool {
	return s.dispatcher.Router().CanRoute(actionName) || s.dispatcher.Registry().Has(actionName)
}

// ListNamespaces returns all registered namespace names.
func (s *System) ListNamespaces() []string {
	return s.dispatcher.Router().Namespaces()
}

// ListActions returns all registered action names.
func (s *System) ListActions() []string {
	return s.dispatcher.Registry().List()
}

// CursorHandler returns the cursor handler for direct configuration.
func (s *System) CursorHandler() *cursor.Handler {
	return s.cursorHandler
}

// SearchHandler returns the search handler for direct configuration.
func (s *System) SearchHandler() *search.Handler {
	return s.searchHandler
}

// MacroHandler returns the macro handler for direct configuration.
func (s *System) MacroHandler() *macro.Handler {
	return s.macroHandler
}

// CompletionHandler returns the completion handler for direct configuration.
func (s *System) CompletionHandler() *completion.Handler {
	return s.completionHandler
}

// SetMacroPlayCallback sets the callback for macro playback.
// This allows the macro handler to execute actions through the dispatcher.
func (s *System) SetMacroPlayCallback(callback func(action input.Action) handler.Result) {
	s.macroHandler.PlayCallback = callback
}

// EnableMacroDispatch sets up macro playback to use the dispatcher.
// This is the standard configuration for integrated macro support.
func (s *System) EnableMacroDispatch() {
	s.macroHandler.PlayCallback = func(action input.Action) handler.Result {
		return s.Dispatch(action)
	}
}

// Reset resets the system state (clears metrics, hooks state, etc.).
func (s *System) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reset metrics
	if s.dispatcher.Metrics() != nil {
		s.dispatcher.Metrics().Reset()
	}

	// Clear repeat hook
	if s.repeatHook != nil {
		s.repeatHook.Clear()
	}

	// Clear AI context
	if s.aiContextHook != nil {
		s.aiContextHook.Clear()
	}

	// Clear macros
	s.macroRecorder.ClearAll()
}

// SystemStats holds system statistics.
type SystemStats struct {
	// Metrics snapshot
	Metrics *MetricsSnapshot

	// Handler counts
	NamespaceCount int
	ActionCount    int

	// Hook counts
	PreHookCount  int
	PostHookCount int

	// Macro stats
	MacroCount int

	// State
	IsRunning bool
}

// Stats returns current system statistics.
func (s *System) Stats() SystemStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SystemStats{
		NamespaceCount: len(s.dispatcher.Router().Namespaces()),
		ActionCount:    s.dispatcher.Registry().Count(),
		PreHookCount:   s.hookManager.PreHookCount(),
		PostHookCount:  s.hookManager.PostHookCount(),
		MacroCount:     len(s.macroRecorder.ListMacros()),
		IsRunning:      s.started,
	}

	if s.dispatcher.Metrics() != nil {
		snapshot := s.dispatcher.Metrics().Snapshot()
		stats.Metrics = &snapshot
	}

	return stats
}
