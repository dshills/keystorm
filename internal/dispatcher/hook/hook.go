// Package hook provides extensible pre/post dispatch hooks for the dispatcher.
package hook

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// Hook is the base interface for all dispatch hooks.
type Hook interface {
	// Name returns a unique identifier for this hook.
	Name() string

	// Priority returns the hook priority.
	// Higher values run first for pre-hooks, last for post-hooks.
	// Standard priorities:
	//   1000+ = system/critical hooks
	//   500-999 = framework hooks
	//   100-499 = plugin hooks
	//   0-99 = user hooks
	Priority() int
}

// PreDispatchHook is called before an action is dispatched.
type PreDispatchHook interface {
	Hook

	// PreDispatch is called before dispatch.
	// It may modify the action or context.
	// Returns false to cancel the dispatch.
	PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool
}

// PostDispatchHook is called after an action is dispatched.
type PostDispatchHook interface {
	Hook

	// PostDispatch is called after dispatch completes.
	// It may inspect or modify the result.
	PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result)
}

// PreDispatchFunc wraps a function as a PreDispatchHook.
type PreDispatchFunc struct {
	name     string
	priority int
	fn       func(action *input.Action, ctx *execctx.ExecutionContext) bool
}

// NewPreDispatchFunc creates a new PreDispatchFunc hook.
func NewPreDispatchFunc(name string, priority int, fn func(action *input.Action, ctx *execctx.ExecutionContext) bool) *PreDispatchFunc {
	return &PreDispatchFunc{
		name:     name,
		priority: priority,
		fn:       fn,
	}
}

// Name implements Hook.
func (f *PreDispatchFunc) Name() string { return f.name }

// Priority implements Hook.
func (f *PreDispatchFunc) Priority() int { return f.priority }

// PreDispatch implements PreDispatchHook.
func (f *PreDispatchFunc) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	if f.fn == nil {
		return true
	}
	return f.fn(action, ctx)
}

// PostDispatchFunc wraps a function as a PostDispatchHook.
type PostDispatchFunc struct {
	name     string
	priority int
	fn       func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result)
}

// NewPostDispatchFunc creates a new PostDispatchFunc hook.
func NewPostDispatchFunc(name string, priority int, fn func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result)) *PostDispatchFunc {
	return &PostDispatchFunc{
		name:     name,
		priority: priority,
		fn:       fn,
	}
}

// Name implements Hook.
func (f *PostDispatchFunc) Name() string { return f.name }

// Priority implements Hook.
func (f *PostDispatchFunc) Priority() int { return f.priority }

// PostDispatch implements PostDispatchHook.
func (f *PostDispatchFunc) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	if f.fn != nil {
		f.fn(action, ctx, result)
	}
}

// CombinedHook implements both PreDispatchHook and PostDispatchHook.
type CombinedHook interface {
	PreDispatchHook
	PostDispatchHook
}
