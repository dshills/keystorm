package hook

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// Standard hook priorities.
const (
	PriorityAudit      = 1000 // Runs first (pre) / last (post)
	PriorityCountLimit = 900  // Enforce count limits early
	PriorityValidation = 800  // Validate before processing
	PriorityRepeat     = 500  // Capture for repeat command
	PriorityAIContext  = 100  // Build AI context
)

// Logger is the interface for logging hooks.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// AuditHook logs all dispatched actions for debugging and audit trails.
type AuditHook struct {
	logger Logger
}

// NewAuditHook creates an audit hook with the given logger.
func NewAuditHook(logger Logger) *AuditHook {
	return &AuditHook{logger: logger}
}

// Name implements Hook.
func (h *AuditHook) Name() string { return "audit" }

// Priority implements Hook.
func (h *AuditHook) Priority() int { return PriorityAudit }

// PreDispatch logs the action being dispatched.
func (h *AuditHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.logger != nil {
		h.logger.Debug("dispatch start",
			"action", action.Name,
			"count", ctx.Count,
			"mode", ctx.Mode(),
		)
	}
	return true
}

// PostDispatch logs the dispatch result.
func (h *AuditHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	if h.logger == nil {
		return
	}

	if result.Status == handler.StatusError {
		h.logger.Error("dispatch failed",
			"action", action.Name,
			"error", result.Error,
		)
	} else {
		h.logger.Debug("dispatch complete",
			"action", action.Name,
			"status", result.Status.String(),
			"message", result.Message,
		)
	}
}

// RepeatHook captures the last repeatable action for the Vim "." command.
type RepeatHook struct {
	mu         sync.RWMutex
	lastAction *input.Action
	lastCount  int
}

// NewRepeatHook creates a new repeat hook.
func NewRepeatHook() *RepeatHook {
	return &RepeatHook{}
}

// Name implements Hook.
func (h *RepeatHook) Name() string { return "repeat" }

// Priority implements Hook.
func (h *RepeatHook) Priority() int { return PriorityRepeat }

// PostDispatch captures successful editing actions.
func (h *RepeatHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	// Only capture successful actions
	if result.Status != handler.StatusOK {
		return
	}

	// Only capture repeatable actions
	if !isRepeatable(action.Name) {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Deep copy the action including nested maps
	h.lastAction = copyAction(action)
	h.lastCount = ctx.Count
}

// LastAction returns a copy of the last captured action and count.
// Returns nil if no action has been captured.
func (h *RepeatHook) LastAction() (*input.Action, int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.lastAction == nil {
		return nil, 0
	}

	// Return a copy to prevent mutation of internal state
	return copyAction(h.lastAction), h.lastCount
}

// copyAction creates a deep copy of an action including nested maps.
func copyAction(action *input.Action) *input.Action {
	if action == nil {
		return nil
	}

	actionCopy := *action

	// Deep copy Args.Extra map if present
	if action.Args.Extra != nil {
		actionCopy.Args.Extra = make(map[string]interface{}, len(action.Args.Extra))
		for k, v := range action.Args.Extra {
			actionCopy.Args.Extra[k] = v
		}
	}

	// Deep copy Motion if present
	if action.Args.Motion != nil {
		motionCopy := *action.Args.Motion
		actionCopy.Args.Motion = &motionCopy
	}

	// Deep copy TextObject if present
	if action.Args.TextObject != nil {
		textObjCopy := *action.Args.TextObject
		actionCopy.Args.TextObject = &textObjCopy
	}

	return &actionCopy
}

// Clear clears the last captured action.
func (h *RepeatHook) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastAction = nil
	h.lastCount = 0
}

// isRepeatable returns true if the action should be captured for repeat.
func isRepeatable(actionName string) bool {
	// Editor actions are typically repeatable
	if strings.HasPrefix(actionName, "editor.") {
		return true
	}
	// Operator actions are repeatable
	if strings.HasPrefix(actionName, "operator.") {
		return true
	}
	// Some mode transitions that involve editing
	switch actionName {
	case "mode.openBelow", "mode.openAbove":
		return true
	}
	return false
}

// ChangeRecord represents a recorded edit for AI context.
type ChangeRecord struct {
	Timestamp   time.Time
	Action      string
	FilePath    string
	StartOffset int64
	EndOffset   int64
	OldText     string
	NewText     string
}

// AIContextHook builds context for AI integration by tracking edits.
type AIContextHook struct {
	mu       sync.RWMutex
	changes  []ChangeRecord
	maxSize  int
	callback func(record ChangeRecord)
}

// NewAIContextHook creates an AI context tracking hook.
// maxSize limits the number of changes retained (0 = unlimited).
func NewAIContextHook(maxSize int) *AIContextHook {
	return &AIContextHook{
		changes: make([]ChangeRecord, 0),
		maxSize: maxSize,
	}
}

// Name implements Hook.
func (h *AIContextHook) Name() string { return "ai-context" }

// Priority implements Hook.
func (h *AIContextHook) Priority() int { return PriorityAIContext }

// PostDispatch tracks edits for AI context building.
func (h *AIContextHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	// Only track successful edits
	if result.Status != handler.StatusOK {
		return
	}

	// Check if there were any edits
	if len(result.Edits) == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, edit := range result.Edits {
		record := ChangeRecord{
			Timestamp:   time.Now(),
			Action:      action.Name,
			FilePath:    ctx.FilePath,
			StartOffset: int64(edit.Range.Start),
			EndOffset:   int64(edit.Range.End),
			OldText:     edit.OldText,
			NewText:     edit.NewText,
		}

		h.changes = append(h.changes, record)

		// Notify callback if registered
		if h.callback != nil {
			h.callback(record)
		}
	}

	// Trim to maxSize if needed
	if h.maxSize > 0 && len(h.changes) > h.maxSize {
		h.changes = h.changes[len(h.changes)-h.maxSize:]
	}
}

// Changes returns a copy of all recorded changes.
func (h *AIContextHook) Changes() []ChangeRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]ChangeRecord, len(h.changes))
	copy(result, h.changes)
	return result
}

// RecentChanges returns the most recent n changes.
func (h *AIContextHook) RecentChanges(n int) []ChangeRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n >= len(h.changes) {
		result := make([]ChangeRecord, len(h.changes))
		copy(result, h.changes)
		return result
	}

	result := make([]ChangeRecord, n)
	copy(result, h.changes[len(h.changes)-n:])
	return result
}

// SetCallback sets a callback to be called for each change.
func (h *AIContextHook) SetCallback(fn func(record ChangeRecord)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callback = fn
}

// Clear removes all recorded changes.
func (h *AIContextHook) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.changes = h.changes[:0]
}

// CountLimitHook enforces a maximum repeat count to prevent runaway commands.
type CountLimitHook struct {
	maxCount int
}

// NewCountLimitHook creates a count limit hook.
func NewCountLimitHook(maxCount int) *CountLimitHook {
	return &CountLimitHook{maxCount: maxCount}
}

// Name implements Hook.
func (h *CountLimitHook) Name() string { return "count-limit" }

// Priority implements Hook.
func (h *CountLimitHook) Priority() int { return PriorityCountLimit }

// PreDispatch limits the repeat count.
func (h *CountLimitHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.maxCount > 0 && ctx.Count > h.maxCount {
		ctx.Count = h.maxCount
	}
	return true
}

// ValidationHook validates actions before dispatch using a custom function.
type ValidationHook struct {
	name     string
	priority int
	validate func(action *input.Action, ctx *execctx.ExecutionContext) error
}

// NewValidationHook creates a validation hook.
func NewValidationHook(name string, priority int, validate func(*input.Action, *execctx.ExecutionContext) error) *ValidationHook {
	return &ValidationHook{
		name:     name,
		priority: priority,
		validate: validate,
	}
}

// Name implements Hook.
func (h *ValidationHook) Name() string { return h.name }

// Priority implements Hook.
func (h *ValidationHook) Priority() int { return h.priority }

// PreDispatch validates the action and cancels if invalid.
func (h *ValidationHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.validate == nil {
		return true
	}
	return h.validate(action, ctx) == nil
}

// LoggingHook provides simple logging without a structured logger.
type LoggingHook struct {
	name     string
	priority int
	logFunc  func(format string, args ...interface{})
}

// NewLoggingHook creates a logging hook with a printf-style function.
func NewLoggingHook(name string, priority int, logFunc func(format string, args ...interface{})) *LoggingHook {
	return &LoggingHook{
		name:     name,
		priority: priority,
		logFunc:  logFunc,
	}
}

// Name implements Hook.
func (h *LoggingHook) Name() string { return h.name }

// Priority implements Hook.
func (h *LoggingHook) Priority() int { return h.priority }

// PreDispatch logs the action.
func (h *LoggingHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.logFunc != nil {
		h.logFunc("dispatch: %s (count=%d, mode=%s)", action.Name, ctx.Count, ctx.Mode())
	}
	return true
}

// PostDispatch logs the result.
func (h *LoggingHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	if h.logFunc != nil {
		h.logFunc("complete: %s -> %s", action.Name, result.Status.String())
	}
}

// ReadOnlyHook prevents modifications to read-only buffers.
type ReadOnlyHook struct{}

// NewReadOnlyHook creates a read-only enforcement hook.
func NewReadOnlyHook() *ReadOnlyHook {
	return &ReadOnlyHook{}
}

// Name implements Hook.
func (h *ReadOnlyHook) Name() string { return "read-only" }

// Priority implements Hook.
func (h *ReadOnlyHook) Priority() int { return PriorityValidation }

// PreDispatch cancels modifications on read-only buffers.
func (h *ReadOnlyHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	// Check if context is read-only
	if !ctx.IsReadOnly() {
		return true
	}

	// Allow read-only safe actions
	if isReadOnlySafe(action.Name) {
		return true
	}

	// Block modifications
	return false
}

// isReadOnlySafe returns true if the action doesn't modify the buffer.
func isReadOnlySafe(actionName string) bool {
	// Cursor movements are safe
	if strings.HasPrefix(actionName, "cursor.") {
		return true
	}
	// View operations are safe
	if strings.HasPrefix(actionName, "view.") || strings.HasPrefix(actionName, "scroll.") {
		return true
	}
	// Search is safe
	if strings.HasPrefix(actionName, "search.") && !strings.Contains(actionName, "replace") {
		return true
	}
	// Mode switching to non-edit modes is safe
	switch actionName {
	case "mode.normal", "mode.visual", "mode.visualLine", "mode.visualBlock", "mode.command":
		return true
	}
	return false
}

// TimingHook measures action execution time.
// Start times are stored on the ExecutionContext to handle concurrent dispatches
// and avoid memory leaks when dispatches are cancelled.
type TimingHook struct {
	callback func(action string, duration time.Duration)
}

// timingStartKey is the context data key for timing start time.
const timingStartKey = "_timing_start"

// NewTimingHook creates a timing hook.
func NewTimingHook(callback func(action string, duration time.Duration)) *TimingHook {
	return &TimingHook{
		callback: callback,
	}
}

// Name implements Hook.
func (h *TimingHook) Name() string { return "timing" }

// Priority implements Hook.
func (h *TimingHook) Priority() int { return PriorityAudit }

// PreDispatch records the start time on the context.
func (h *TimingHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	ctx.SetData(timingStartKey, time.Now())
	return true
}

// PostDispatch calculates and reports the duration.
func (h *TimingHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	startVal, ok := ctx.GetData(timingStartKey)
	if !ok {
		return
	}

	start, ok := startVal.(time.Time)
	if ok && h.callback != nil {
		h.callback(action.Name, time.Since(start))
	}
}

// ActionFilterHook allows/blocks actions based on a filter function.
type ActionFilterHook struct {
	name     string
	priority int
	filter   func(action *input.Action, ctx *execctx.ExecutionContext) (allow bool, reason string)
}

// NewActionFilterHook creates an action filter hook.
func NewActionFilterHook(name string, priority int, filter func(*input.Action, *execctx.ExecutionContext) (bool, string)) *ActionFilterHook {
	return &ActionFilterHook{
		name:     name,
		priority: priority,
		filter:   filter,
	}
}

// Name implements Hook.
func (h *ActionFilterHook) Name() string { return h.name }

// Priority implements Hook.
func (h *ActionFilterHook) Priority() int { return h.priority }

// PreDispatch applies the filter.
func (h *ActionFilterHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.filter == nil {
		return true
	}
	allow, reason := h.filter(action, ctx)
	if !allow && reason != "" {
		// Store reason in action metadata for debugging
		if action.Args.Extra == nil {
			action.Args.Extra = make(map[string]interface{})
		}
		action.Args.Extra["filter_reason"] = reason
	}
	return allow
}

// ResultModifierHook modifies results after execution.
type ResultModifierHook struct {
	name     string
	priority int
	modify   func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result)
}

// NewResultModifierHook creates a result modifier hook.
func NewResultModifierHook(name string, priority int, modify func(*input.Action, *execctx.ExecutionContext, *handler.Result)) *ResultModifierHook {
	return &ResultModifierHook{
		name:     name,
		priority: priority,
		modify:   modify,
	}
}

// Name implements Hook.
func (h *ResultModifierHook) Name() string { return h.name }

// Priority implements Hook.
func (h *ResultModifierHook) Priority() int { return h.priority }

// PostDispatch modifies the result.
func (h *ResultModifierHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	if h.modify != nil {
		h.modify(action, ctx, result)
	}
}

// AddMessageHook adds a message prefix/suffix to results.
type AddMessageHook struct {
	prefix string
	suffix string
}

// NewAddMessageHook creates a message modifier hook.
func NewAddMessageHook(prefix, suffix string) *AddMessageHook {
	return &AddMessageHook{prefix: prefix, suffix: suffix}
}

// Name implements Hook.
func (h *AddMessageHook) Name() string { return "add-message" }

// Priority implements Hook.
func (h *AddMessageHook) Priority() int { return 0 }

// PostDispatch adds prefix/suffix to the message.
func (h *AddMessageHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	if result.Message != "" || result.Status == handler.StatusOK {
		result.Message = fmt.Sprintf("%s%s%s", h.prefix, result.Message, h.suffix)
	}
}
