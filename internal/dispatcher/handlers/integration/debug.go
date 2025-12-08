package integration

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/integration/debug"
)

// Debug action names.
const (
	ActionDebugStart         = "debug.start"             // Start debug session
	ActionDebugStop          = "debug.stop"              // Stop debug session
	ActionDebugSessions      = "debug.sessions"          // List active sessions
	ActionDebugContinue      = "debug.continue"          // Continue execution
	ActionDebugStepOver      = "debug.stepOver"          // Step over
	ActionDebugStepInto      = "debug.stepInto"          // Step into
	ActionDebugStepOut       = "debug.stepOut"           // Step out
	ActionDebugPause         = "debug.pause"             // Pause execution
	ActionDebugBreakpointSet = "debug.breakpoint.set"    // Set breakpoint
	ActionDebugBreakpointDel = "debug.breakpoint.remove" // Remove breakpoint
	ActionDebugBreakpoints   = "debug.breakpoints"       // List breakpoints
	ActionDebugVariables     = "debug.variables"         // Get variables
	ActionDebugStack         = "debug.stack"             // Get stack trace
	ActionDebugEvaluate      = "debug.evaluate"          // Evaluate expression
)

// DebugConfig represents debug session configuration.
type DebugConfig struct {
	Adapter     string            // Debug adapter (e.g., "delve", "node")
	Program     string            // Program to debug
	Args        []string          // Program arguments
	Cwd         string            // Working directory
	Env         map[string]string // Environment variables
	StopOnEntry bool              // Stop at program entry
}

// DebugSession represents a debug session.
type DebugSession interface {
	// ID returns the session ID.
	ID() string

	// State returns the current session state.
	State() debug.SessionState

	// Continue resumes execution.
	Continue() error

	// StepOver steps over the current line.
	StepOver() error

	// StepInto steps into the current call.
	StepInto() error

	// StepOut steps out of the current function.
	StepOut() error

	// Pause pauses execution.
	Pause() error

	// Variables returns variables in the current scope.
	Variables() ([]debug.Variable, error)

	// StackTrace returns the current stack trace.
	StackTrace() ([]debug.StackFrame, error)

	// Evaluate evaluates an expression.
	Evaluate(expression string) (string, error)
}

// DebugManager manages debug sessions.
type DebugManager interface {
	// StartSession starts a new debug session.
	StartSession(config DebugConfig) (DebugSession, error)

	// StopSession stops a debug session.
	StopSession(sessionID string) error

	// GetSession returns a session by ID.
	GetSession(sessionID string) (DebugSession, bool)

	// ListSessions returns all active sessions.
	ListSessions() []DebugSession

	// SetBreakpoint sets a breakpoint.
	SetBreakpoint(file string, line int) (string, error)

	// RemoveBreakpoint removes a breakpoint.
	RemoveBreakpoint(id string) error

	// ListBreakpoints returns all breakpoints.
	ListBreakpoints() []debug.Breakpoint
}

const debugManagerKey = "_debug_manager"

// DebugHandler handles debug-related actions.
type DebugHandler struct {
	manager DebugManager
}

// NewDebugHandler creates a new debug handler.
func NewDebugHandler() *DebugHandler {
	return &DebugHandler{}
}

// NewDebugHandlerWithManager creates a handler with a debug manager.
func NewDebugHandlerWithManager(manager DebugManager) *DebugHandler {
	return &DebugHandler{manager: manager}
}

// SetManager updates the debug manager.
// This allows in-place configuration updates without replacing the handler.
func (h *DebugHandler) SetManager(manager DebugManager) {
	h.manager = manager
}

// Namespace returns the debug namespace.
func (h *DebugHandler) Namespace() string {
	return "debug"
}

// CanHandle returns true if this handler can process the action.
func (h *DebugHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionDebugStart, ActionDebugStop, ActionDebugSessions,
		ActionDebugContinue, ActionDebugStepOver, ActionDebugStepInto,
		ActionDebugStepOut, ActionDebugPause,
		ActionDebugBreakpointSet, ActionDebugBreakpointDel, ActionDebugBreakpoints,
		ActionDebugVariables, ActionDebugStack, ActionDebugEvaluate:
		return true
	}
	return false
}

// HandleAction processes a debug action.
func (h *DebugHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	switch action.Name {
	case ActionDebugStart:
		return h.start(action, ctx)
	case ActionDebugStop:
		return h.stop(action, ctx)
	case ActionDebugSessions:
		return h.sessions(ctx)
	case ActionDebugContinue:
		return h.continueExec(action, ctx)
	case ActionDebugStepOver:
		return h.stepOver(action, ctx)
	case ActionDebugStepInto:
		return h.stepInto(action, ctx)
	case ActionDebugStepOut:
		return h.stepOut(action, ctx)
	case ActionDebugPause:
		return h.pause(action, ctx)
	case ActionDebugBreakpointSet:
		return h.setBreakpoint(action, ctx)
	case ActionDebugBreakpointDel:
		return h.removeBreakpoint(action, ctx)
	case ActionDebugBreakpoints:
		return h.listBreakpoints(ctx)
	case ActionDebugVariables:
		return h.variables(action, ctx)
	case ActionDebugStack:
		return h.stack(action, ctx)
	case ActionDebugEvaluate:
		return h.evaluate(action, ctx)
	default:
		return handler.Errorf("unknown debug action: %s", action.Name)
	}
}

// getManager returns the debug manager from handler or context.
func (h *DebugHandler) getManager(ctx *execctx.ExecutionContext) DebugManager {
	if h.manager != nil {
		return h.manager
	}
	if v, ok := ctx.GetData(debugManagerKey); ok {
		if dm, ok := v.(DebugManager); ok {
			return dm
		}
	}
	return nil
}

// getSession returns the session for an action (from id arg or active session).
func (h *DebugHandler) getSession(action input.Action, ctx *execctx.ExecutionContext) (DebugSession, handler.Result) {
	dm := h.getManager(ctx)
	if dm == nil {
		return nil, handler.Errorf("no debug manager available")
	}

	sessionID := action.Args.GetString("session")
	if sessionID == "" {
		// Try to use single active session
		sessions := dm.ListSessions()
		if len(sessions) == 0 {
			return nil, handler.Errorf("no active debug sessions")
		}
		if len(sessions) > 1 {
			return nil, handler.Errorf("multiple sessions active, specify session id")
		}
		return sessions[0], handler.Result{}
	}

	session, ok := dm.GetSession(sessionID)
	if !ok {
		return nil, handler.Errorf("session %q not found", sessionID)
	}
	return session, handler.Result{}
}

func (h *DebugHandler) start(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	dm := h.getManager(ctx)
	if dm == nil {
		return handler.Errorf("debug.start: no debug manager available")
	}

	// Get program args
	var args []string
	if argsVal, ok := action.Args.Get("args"); ok {
		if as, ok := argsVal.([]string); ok {
			args = as
		}
	}

	config := DebugConfig{
		Adapter:     action.Args.GetString("adapter"),
		Program:     action.Args.GetString("program"),
		Args:        args,
		Cwd:         action.Args.GetString("cwd"),
		StopOnEntry: action.Args.GetBool("stopOnEntry"),
	}

	if config.Adapter == "" {
		return handler.Errorf("debug.start: adapter required")
	}
	if config.Program == "" {
		return handler.Errorf("debug.start: program required")
	}

	session, err := dm.StartSession(config)
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("sessionId", session.ID()).
		WithData("state", session.State().String()).
		WithMessage("Started debug session: " + session.ID())
}

func (h *DebugHandler) stop(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	dm := h.getManager(ctx)
	if dm == nil {
		return handler.Errorf("debug.stop: no debug manager available")
	}

	sessionID := action.Args.GetString("session")
	if sessionID == "" {
		// Stop all sessions
		sessions := dm.ListSessions()
		for _, s := range sessions {
			_ = dm.StopSession(s.ID())
		}
		return handler.Success().WithMessage("Stopped all debug sessions")
	}

	if err := dm.StopSession(sessionID); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("sessionId", sessionID).
		WithMessage("Stopped session: " + sessionID)
}

func (h *DebugHandler) sessions(ctx *execctx.ExecutionContext) handler.Result {
	dm := h.getManager(ctx)
	if dm == nil {
		return handler.Errorf("debug.sessions: no debug manager available")
	}

	sessions := dm.ListSessions()

	sessionInfos := make([]map[string]string, len(sessions))
	for i, s := range sessions {
		sessionInfos[i] = map[string]string{
			"id":    s.ID(),
			"state": s.State().String(),
		}
	}

	return handler.Success().
		WithData("sessions", sessionInfos).
		WithData("count", len(sessions)).
		WithMessage(formatSessionList(sessions))
}

func (h *DebugHandler) continueExec(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	if err := session.Continue(); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("sessionId", session.ID()).
		WithMessage("Continuing execution")
}

func (h *DebugHandler) stepOver(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	if err := session.StepOver(); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("sessionId", session.ID()).
		WithMessage("Stepped over").
		WithRedraw()
}

func (h *DebugHandler) stepInto(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	if err := session.StepInto(); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("sessionId", session.ID()).
		WithMessage("Stepped into").
		WithRedraw()
}

func (h *DebugHandler) stepOut(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	if err := session.StepOut(); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("sessionId", session.ID()).
		WithMessage("Stepped out").
		WithRedraw()
}

func (h *DebugHandler) pause(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	if err := session.Pause(); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("sessionId", session.ID()).
		WithMessage("Paused execution")
}

func (h *DebugHandler) setBreakpoint(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	dm := h.getManager(ctx)
	if dm == nil {
		return handler.Errorf("debug.breakpoint.set: no debug manager available")
	}

	file := action.Args.GetString("file")
	if file == "" {
		file = ctx.FilePath
	}
	if file == "" {
		return handler.Errorf("debug.breakpoint.set: file required")
	}

	line := action.Args.GetInt("line")
	if line <= 0 {
		return handler.Errorf("debug.breakpoint.set: line required")
	}

	bpID, err := dm.SetBreakpoint(file, line)
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("breakpointId", bpID).
		WithData("file", file).
		WithData("line", line).
		WithMessage("Breakpoint set at " + file + ":" + itoa(line))
}

func (h *DebugHandler) removeBreakpoint(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	dm := h.getManager(ctx)
	if dm == nil {
		return handler.Errorf("debug.breakpoint.remove: no debug manager available")
	}

	bpID := action.Args.GetString("id")
	if bpID == "" {
		return handler.Errorf("debug.breakpoint.remove: breakpoint id required")
	}

	if err := dm.RemoveBreakpoint(bpID); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("breakpointId", bpID).
		WithMessage("Removed breakpoint: " + bpID)
}

func (h *DebugHandler) listBreakpoints(ctx *execctx.ExecutionContext) handler.Result {
	dm := h.getManager(ctx)
	if dm == nil {
		return handler.Errorf("debug.breakpoints: no debug manager available")
	}

	breakpoints := dm.ListBreakpoints()

	bpInfos := make([]map[string]any, len(breakpoints))
	for i, bp := range breakpoints {
		bpInfos[i] = map[string]any{
			"id":      bp.ID,
			"file":    bp.Path,
			"line":    bp.Line,
			"enabled": bp.Enabled,
		}
	}

	return handler.Success().
		WithData("breakpoints", bpInfos).
		WithData("count", len(breakpoints)).
		WithMessage(formatBreakpointList(breakpoints))
}

func (h *DebugHandler) variables(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	vars, err := session.Variables()
	if err != nil {
		return handler.Error(err)
	}

	varInfos := make([]map[string]string, len(vars))
	for i, v := range vars {
		varInfos[i] = map[string]string{
			"name":  v.Name,
			"value": v.Value,
			"type":  v.Type,
		}
	}

	return handler.Success().
		WithData("variables", varInfos).
		WithMessage(formatVariableList(vars))
}

func (h *DebugHandler) stack(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	frames, err := session.StackTrace()
	if err != nil {
		return handler.Error(err)
	}

	frameInfos := make([]map[string]any, len(frames))
	for i, f := range frames {
		frameInfos[i] = map[string]any{
			"id":       f.ID,
			"name":     f.Name,
			"file":     f.SourcePath(),
			"line":     f.Line,
			"function": f.Name,
		}
	}

	return handler.Success().
		WithData("frames", frameInfos).
		WithMessage(formatStackTrace(frames))
}

func (h *DebugHandler) evaluate(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	session, errResult := h.getSession(action, ctx)
	if session == nil {
		return errResult
	}

	expr := action.Args.GetString("expression")
	if expr == "" {
		return handler.Errorf("debug.evaluate: expression required")
	}

	result, err := session.Evaluate(expr)
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("expression", expr).
		WithData("result", result).
		WithMessage(expr + " = " + result)
}

// Helper functions

func formatSessionList(sessions []DebugSession) string {
	if len(sessions) == 0 {
		return "No active debug sessions"
	}

	msg := "Debug sessions:\n"
	for _, s := range sessions {
		msg += "  " + s.ID() + " [" + s.State().String() + "]\n"
	}
	return msg
}

func formatBreakpointList(breakpoints []debug.Breakpoint) string {
	if len(breakpoints) == 0 {
		return "No breakpoints set"
	}

	msg := "Breakpoints:\n"
	for _, bp := range breakpoints {
		status := "enabled"
		if !bp.Enabled {
			status = "disabled"
		}
		msg += "  " + itoa(bp.ID) + " " + bp.Path + ":" + itoa(bp.Line) + " [" + status + "]\n"
	}
	return msg
}

func formatVariableList(vars []debug.Variable) string {
	if len(vars) == 0 {
		return "No variables in scope"
	}

	msg := "Variables:\n"
	for _, v := range vars {
		msg += "  " + v.Name + " (" + v.Type + ") = " + truncate(v.Value, 40) + "\n"
	}
	return msg
}

func formatStackTrace(frames []debug.StackFrame) string {
	if len(frames) == 0 {
		return "No stack frames"
	}

	msg := "Stack trace:\n"
	for i, f := range frames {
		msg += "  #" + itoa(i) + " " + f.Name + " at " + f.SourcePath() + ":" + itoa(f.Line) + "\n"
	}
	return msg
}
