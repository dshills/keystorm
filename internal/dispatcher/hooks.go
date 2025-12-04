package dispatcher

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// PreDispatchHook is called before an action is dispatched.
// Returning false cancels the dispatch.
type PreDispatchHook interface {
	// PreDispatch is called before dispatch.
	// It may modify the action or context.
	// Returns false to cancel the dispatch.
	PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool
}

// PostDispatchHook is called after an action is dispatched.
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

// LoggingHook provides basic logging for dispatch operations.
type LoggingHook struct {
	// LogFunc is called with log messages.
	LogFunc func(format string, args ...interface{})
}

// NewLoggingHook creates a new logging hook.
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
type CountLimitHook struct {
	MaxCount int
}

// NewCountLimitHook creates a new count limit hook.
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
