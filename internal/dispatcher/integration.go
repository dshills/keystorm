package dispatcher

import (
	"sync"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/completion"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/cursor"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/editor"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/file"
	inthandlers "github.com/dshills/keystorm/internal/dispatcher/handlers/integration"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/macro"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/mode"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/operator"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/search"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/view"
	"github.com/dshills/keystorm/internal/dispatcher/handlers/window"
	"github.com/dshills/keystorm/internal/dispatcher/hook"
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/integration"
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

	// Integration handlers
	gitHandler   *inthandlers.GitHandler
	taskHandler  *inthandlers.TaskHandler
	debugHandler *inthandlers.DebugHandler

	// Macro recorder (shared between handler and system)
	macroRecorder *macro.DefaultMacroRecorder

	// Event publishing for integration events
	eventPublisher integration.EventPublisher

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

	// Integration handlers (managers can be set later via SetGitManager, etc.)
	s.gitHandler = inthandlers.NewGitHandler()
	s.taskHandler = inthandlers.NewTaskHandler()
	s.debugHandler = inthandlers.NewDebugHandler()
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

	// Register integration handlers
	router.RegisterNamespace("git", s.gitHandler)
	router.RegisterNamespace("task", s.taskHandler)
	router.RegisterNamespace("debug", s.debugHandler)

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

// GitHandler returns the git handler for direct configuration.
// The returned handler should not be retained across SetGitManager calls.
func (s *System) GitHandler() *inthandlers.GitHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gitHandler
}

// TaskHandler returns the task handler for direct configuration.
// The returned handler should not be retained across SetTaskManager/SetTaskComponents calls.
func (s *System) TaskHandler() *inthandlers.TaskHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.taskHandler
}

// DebugHandler returns the debug handler for direct configuration.
// The returned handler should not be retained across SetDebugManager calls.
func (s *System) DebugHandler() *inthandlers.DebugHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.debugHandler
}

// SetGitManager sets the git manager for the git handler.
// The handler is updated in-place to preserve router registration.
func (s *System) SetGitManager(manager inthandlers.GitManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gitHandler != nil {
		s.gitHandler.SetManager(manager)
	}
}

// SetTaskManager sets the task manager for the task handler.
// The handler is updated in-place to preserve router registration.
func (s *System) SetTaskManager(manager inthandlers.TaskManager, workspace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskHandler != nil {
		s.taskHandler.SetManager(manager, workspace)
	}
}

// SetTaskComponents sets separate task discoverer and executor.
// The handler is updated in-place to preserve router registration.
func (s *System) SetTaskComponents(discoverer inthandlers.TaskDiscoverer, executor inthandlers.TaskExecutor, workspace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskHandler != nil {
		s.taskHandler.SetComponents(discoverer, executor, workspace)
	}
}

// SetDebugManager sets the debug manager for the debug handler.
// The handler is updated in-place to preserve router registration.
func (s *System) SetDebugManager(manager inthandlers.DebugManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debugHandler != nil {
		s.debugHandler.SetManager(manager)
	}
}

// EventPublisher returns the event publisher for integration events.
// May return nil if no publisher was set.
func (s *System) EventPublisher() integration.EventPublisher {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventPublisher
}

// SetEventPublisher sets the event publisher for integration events.
// Events will be published when integration actions are dispatched.
func (s *System) SetEventPublisher(pub integration.EventPublisher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventPublisher = pub
}

// PublishEvent publishes an event if an event publisher is configured.
func (s *System) PublishEvent(eventType string, data map[string]any) {
	s.mu.RLock()
	pub := s.eventPublisher
	s.mu.RUnlock()

	if pub != nil {
		pub.Publish(eventType, data)
	}
}
