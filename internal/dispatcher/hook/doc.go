// Package hook provides extensible pre/post dispatch hooks for the dispatcher.
//
// Hooks allow interception of action dispatch for logging, validation,
// transformation, and other cross-cutting concerns. They are organized
// by priority to control execution order.
//
// # Hook Types
//
// There are two main hook interfaces:
//
//   - PreDispatchHook: Called before an action is dispatched. Can cancel the action.
//   - PostDispatchHook: Called after dispatch completes. Can inspect/modify results.
//
// Hooks implement the base Hook interface with Name() and Priority() methods
// for identification and ordering.
//
// # Priority System
//
// Hooks are ordered by priority:
//
//   - Pre-hooks: Higher priority runs first (1000+ = system, 500-999 = framework)
//   - Post-hooks: Lower priority runs first, higher runs last (to see final results)
//
// Standard priority constants are provided:
//
//	PriorityAudit     = 1000  // System/audit hooks
//	PriorityCountLimit = 900  // Enforce limits early
//	PriorityValidation = 800  // Validation before processing
//	PriorityRepeat    = 500  // Capture for repeat command
//	PriorityAIContext = 100  // Build AI context
//
// # Built-in Hooks
//
// The package provides several built-in hooks:
//
//   - AuditHook: Logs all dispatched actions for debugging/audit
//   - RepeatHook: Captures last action for Vim "." command
//   - AIContextHook: Tracks edits for AI integration
//   - CountLimitHook: Enforces maximum repeat count
//   - ValidationHook: Custom validation before dispatch
//   - ReadOnlyHook: Prevents modifications to read-only buffers
//   - TimingHook: Measures action execution time
//   - LoggingHook: Simple printf-style logging
//
// # Hook Manager
//
// The Manager type handles hook registration and execution:
//
//	manager := hook.NewManager()
//	manager.RegisterPre(hook.NewCountLimitHook(1000))
//	manager.RegisterPost(hook.NewRepeatHook())
//
//	// Run hooks
//	if manager.RunPreDispatch(&action, ctx) {
//	    // Dispatch action...
//	    manager.RunPostDispatch(&action, ctx, &result)
//	}
//
// # Function Adapters
//
// For simple hooks, function adapters are provided:
//
//	hook := hook.NewPreDispatchFunc("my-hook", 500, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
//	    // Pre-dispatch logic
//	    return true // Continue dispatch
//	})
//
// # Usage Example
//
// Typical setup with built-in hooks:
//
//	manager := hook.NewManager()
//
//	// Add audit logging (highest priority)
//	manager.Register(hook.NewAuditHook(logger))
//
//	// Enforce repeat count limit
//	manager.RegisterPre(hook.NewCountLimitHook(10000))
//
//	// Track changes for AI
//	aiHook := hook.NewAIContextHook(100)
//	manager.RegisterPost(aiHook)
//
//	// Capture for repeat command
//	repeatHook := hook.NewRepeatHook()
//	manager.RegisterPost(repeatHook)
//
// # Thread Safety
//
// All hook types are thread-safe. The Manager uses read-write locks
// to allow concurrent hook execution while protecting registration.
package hook
