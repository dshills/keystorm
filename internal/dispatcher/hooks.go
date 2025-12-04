package dispatcher

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/dispatcher/hook"
	"github.com/dshills/keystorm/internal/input"
)

// PreDispatchHook is called before an action is dispatched.
// Returning false cancels the dispatch.
//
// For hooks with priority ordering, use the hook.PreDispatchHook interface
// from the hook package instead.
type PreDispatchHook interface {
	// PreDispatch is called before dispatch.
	// It may modify the action or context.
	// Returns false to cancel the dispatch.
	PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool
}

// PostDispatchHook is called after an action is dispatched.
//
// For hooks with priority ordering, use the hook.PostDispatchHook interface
// from the hook package instead.
type PostDispatchHook interface {
	// PostDispatch is called after dispatch completes.
	// It may inspect or modify the result.
	PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result)
}

// PreDispatchFunc is a function adapter for PreDispatchHook.
type PreDispatchFunc func(action *input.Action, ctx *execctx.ExecutionContext) bool

// PreDispatch implements PreDispatchHook.
func (f PreDispatchFunc) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	return f(action, ctx)
}

// PostDispatchFunc is a function adapter for PostDispatchHook.
type PostDispatchFunc func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result)

// PostDispatch implements PostDispatchHook.
func (f PostDispatchFunc) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	f(action, ctx, result)
}

// simplePreHookAdapter wraps a simple PreDispatchHook for the hook.Manager.
type simplePreHookAdapter struct {
	name string
	hook PreDispatchHook
}

func (a *simplePreHookAdapter) Name() string  { return a.name }
func (a *simplePreHookAdapter) Priority() int { return 0 } // Default priority

func (a *simplePreHookAdapter) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	return a.hook.PreDispatch(action, ctx)
}

// simplePostHookAdapter wraps a simple PostDispatchHook for the hook.Manager.
type simplePostHookAdapter struct {
	name string
	hook PostDispatchHook
}

func (a *simplePostHookAdapter) Name() string  { return a.name }
func (a *simplePostHookAdapter) Priority() int { return 0 } // Default priority

func (a *simplePostHookAdapter) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	a.hook.PostDispatch(action, ctx, result)
}

// LoggingHook provides basic logging for dispatch operations.
// Deprecated: Use hook.NewLoggingHook from the hook package instead.
type LoggingHook struct {
	// LogFunc is called with log messages.
	LogFunc func(format string, args ...interface{})
}

// NewLoggingHook creates a new logging hook.
// Deprecated: Use hook.NewLoggingHook from the hook package instead.
func NewLoggingHook(logFunc func(format string, args ...interface{})) *LoggingHook {
	return &LoggingHook{LogFunc: logFunc}
}

// PreDispatch logs the action being dispatched.
func (h *LoggingHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.LogFunc != nil {
		h.LogFunc("dispatching action: %s (count=%d, mode=%s)", action.Name, ctx.Count, ctx.Mode())
	}
	return true
}

// PostDispatch logs the dispatch result.
func (h *LoggingHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	if h.LogFunc != nil {
		h.LogFunc("dispatch complete: %s -> %s", action.Name, result.Status)
	}
}

// ValidationHook validates actions before dispatch.
// Deprecated: Use hook.NewValidationHook from the hook package instead.
type ValidationHook struct {
	// ValidateFunc is called to validate an action.
	// Returns true if the action is valid.
	ValidateFunc func(action *input.Action, ctx *execctx.ExecutionContext) bool
}

// PreDispatch validates the action.
func (h *ValidationHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.ValidateFunc != nil {
		return h.ValidateFunc(action, ctx)
	}
	return true
}

// CountLimitHook enforces a maximum repeat count.
// Deprecated: Use hook.NewCountLimitHook from the hook package instead.
type CountLimitHook struct {
	MaxCount int
}

// NewCountLimitHook creates a new count limit hook.
// Deprecated: Use hook.NewCountLimitHook from the hook package instead.
func NewCountLimitHook(maxCount int) *CountLimitHook {
	return &CountLimitHook{MaxCount: maxCount}
}

// PreDispatch limits the repeat count.
func (h *CountLimitHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if h.MaxCount > 0 && ctx.Count > h.MaxCount {
		ctx.Count = h.MaxCount
	}
	return true
}

// Re-export hook package types for convenience.
type (
	// Hook is the base interface for named, prioritized hooks.
	Hook = hook.Hook

	// HookManager manages hooks with priority ordering.
	HookManager = hook.Manager
)

// NewHookManager creates a new hook manager.
func NewHookManager() *hook.Manager {
	return hook.NewManager()
}
