// Package debug provides debugger integration through the Debug Adapter Protocol.
package debug

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/dshills/keystorm/internal/integration/debug/dap"
)

// SessionState represents the current state of a debug session.
type SessionState int

const (
	// StateInitializing is the initial state before connection.
	StateInitializing SessionState = iota
	// StateConnected is after transport is established.
	StateConnected
	// StateConfiguring is after initialize but before configurationDone.
	StateConfiguring
	// StateRunning is when the debuggee is running.
	StateRunning
	// StateStopped is when the debuggee is stopped (breakpoint, exception, etc).
	StateStopped
	// StateTerminated is when the debuggee has exited.
	StateTerminated
	// StateDisconnected is when the debug adapter has disconnected.
	StateDisconnected
)

// String returns a string representation of the state.
func (s SessionState) String() string {
	switch s {
	case StateInitializing:
		return "initializing"
	case StateConnected:
		return "connected"
	case StateConfiguring:
		return "configuring"
	case StateRunning:
		return "running"
	case StateStopped:
		return "stopped"
	case StateTerminated:
		return "terminated"
	case StateDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// Session represents a debug session with a debug adapter.
type Session struct {
	client       *dap.Client
	capabilities *dap.Capabilities
	state        SessionState
	stateMu      sync.RWMutex

	// Current thread ID (when stopped)
	currentThread int

	// All threads
	threads   []dap.Thread
	threadsMu sync.RWMutex

	// Breakpoints by source path
	breakpoints   map[string][]dap.Breakpoint
	breakpointsMu sync.RWMutex

	// Event handlers
	handlers   SessionHandlers
	handlersMu sync.RWMutex

	// Adapter command (for stdio transport)
	cmd *exec.Cmd
}

// SessionHandlers contains callbacks for session events.
type SessionHandlers struct {
	// OnStateChanged is called when the session state changes.
	OnStateChanged func(old, new SessionState)

	// OnStopped is called when the debuggee stops.
	OnStopped func(reason string, threadID int, allStopped bool)

	// OnOutput is called when the debuggee produces output.
	OnOutput func(category, output string)

	// OnBreakpointChanged is called when breakpoints change.
	OnBreakpointChanged func(reason string, breakpoint dap.Breakpoint)

	// OnThreadChanged is called when threads start or exit.
	OnThreadChanged func(reason string, threadID int)

	// OnTerminated is called when the debuggee terminates.
	OnTerminated func()
}

// SessionConfig configures a debug session.
type SessionConfig struct {
	// AdapterID is the debug adapter identifier.
	AdapterID string

	// ClientID is this client's identifier.
	ClientID string

	// ClientName is this client's name.
	ClientName string

	// LinesStartAt1 indicates if line numbers start at 1.
	LinesStartAt1 bool

	// ColumnsStartAt1 indicates if column numbers start at 1.
	ColumnsStartAt1 bool

	// PathFormat is the path format ("path" or "uri").
	PathFormat string
}

// DefaultSessionConfig returns a default session configuration.
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		AdapterID:       "generic",
		ClientID:        "keystorm",
		ClientName:      "Keystorm Editor",
		LinesStartAt1:   true,
		ColumnsStartAt1: true,
		PathFormat:      "path",
	}
}

// NewSession creates a new debug session with the given client.
func NewSession(client *dap.Client) *Session {
	s := &Session{
		client:      client,
		state:       StateConnected,
		breakpoints: make(map[string][]dap.Breakpoint),
	}

	// Set up event handlers
	client.OnInitialized(s.onInitialized)
	client.OnStopped(s.onStopped)
	client.OnContinued(s.onContinued)
	client.OnExited(s.onExited)
	client.OnTerminated(s.onTerminated)
	client.OnThread(s.onThread)
	client.OnOutput(s.onOutput)
	client.OnBreakpoint(s.onBreakpoint)

	return s
}

// NewStdioSession creates a debug session using stdio transport with a subprocess.
func NewStdioSession(command string, args ...string) (*Session, error) {
	cmd := exec.Command(command, args...)
	transport, err := dap.NewStdioTransport(cmd)
	if err != nil {
		return nil, fmt.Errorf("create stdio transport: %w", err)
	}

	client := dap.NewClient(transport)
	session := NewSession(client)
	session.cmd = cmd

	return session, nil
}

// NewSocketSession creates a debug session using socket transport.
func NewSocketSession(address string) (*Session, error) {
	transport, err := dap.NewSocketTransport(address)
	if err != nil {
		return nil, fmt.Errorf("create socket transport: %w", err)
	}

	client := dap.NewClient(transport)
	return NewSession(client), nil
}

// SetHandlers sets the session event handlers.
func (s *Session) SetHandlers(handlers SessionHandlers) {
	s.handlersMu.Lock()
	s.handlers = handlers
	s.handlersMu.Unlock()
}

// State returns the current session state.
func (s *Session) State() SessionState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

// setState updates the session state.
func (s *Session) setState(state SessionState) {
	s.stateMu.Lock()
	old := s.state
	s.state = state
	s.stateMu.Unlock()

	s.handlersMu.RLock()
	handler := s.handlers.OnStateChanged
	s.handlersMu.RUnlock()

	if handler != nil {
		handler(old, state)
	}
}

// Capabilities returns the debug adapter capabilities.
func (s *Session) Capabilities() *dap.Capabilities {
	return s.capabilities
}

// CurrentThread returns the current thread ID.
func (s *Session) CurrentThread() int {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.currentThread
}

// Threads returns the current list of threads.
func (s *Session) Threads() []dap.Thread {
	s.threadsMu.RLock()
	defer s.threadsMu.RUnlock()
	return append([]dap.Thread{}, s.threads...)
}

// Initialize initializes the debug session.
func (s *Session) Initialize(ctx context.Context, config SessionConfig) error {
	args := dap.InitializeRequestArguments{
		ClientID:        config.ClientID,
		ClientName:      config.ClientName,
		AdapterID:       config.AdapterID,
		LinesStartAt1:   config.LinesStartAt1,
		ColumnsStartAt1: config.ColumnsStartAt1,
		PathFormat:      config.PathFormat,
	}

	caps, err := s.client.Initialize(ctx, args)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	s.capabilities = caps
	s.setState(StateConfiguring)

	return nil
}

// ConfigurationDone signals that configuration is complete.
func (s *Session) ConfigurationDone(ctx context.Context) error {
	if err := s.client.ConfigurationDone(ctx); err != nil {
		return fmt.Errorf("configurationDone: %w", err)
	}

	s.setState(StateRunning)
	return nil
}

// Launch launches the debuggee with the given arguments.
func (s *Session) Launch(ctx context.Context, launchArgs interface{}) error {
	if err := s.client.Launch(ctx, launchArgs); err != nil {
		return fmt.Errorf("launch: %w", err)
	}

	return nil
}

// Attach attaches to a running process.
func (s *Session) Attach(ctx context.Context, attachArgs interface{}) error {
	if err := s.client.Attach(ctx, attachArgs); err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	return nil
}

// Disconnect disconnects from the debug adapter.
func (s *Session) Disconnect(ctx context.Context, terminate bool) error {
	args := dap.DisconnectArguments{
		TerminateDebuggee: terminate,
	}

	if err := s.client.Disconnect(ctx, args); err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}

	s.setState(StateDisconnected)
	return nil
}

// Close closes the session and underlying client.
func (s *Session) Close() error {
	s.setState(StateDisconnected)
	return s.client.Close()
}

// SetBreakpoints sets breakpoints in a source file.
func (s *Session) SetBreakpoints(ctx context.Context, path string, lines []int) ([]dap.Breakpoint, error) {
	source := dap.Source{
		Path: path,
	}

	breakpoints := make([]dap.SourceBreakpoint, len(lines))
	for i, line := range lines {
		breakpoints[i] = dap.SourceBreakpoint{Line: line}
	}

	args := dap.SetBreakpointsArguments{
		Source:      source,
		Breakpoints: breakpoints,
	}

	result, err := s.client.SetBreakpoints(ctx, args)
	if err != nil {
		return nil, err
	}

	s.breakpointsMu.Lock()
	s.breakpoints[path] = result
	s.breakpointsMu.Unlock()

	return result, nil
}

// SetBreakpointsWithConditions sets breakpoints with conditions.
func (s *Session) SetBreakpointsWithConditions(ctx context.Context, path string, bps []dap.SourceBreakpoint) ([]dap.Breakpoint, error) {
	source := dap.Source{
		Path: path,
	}

	args := dap.SetBreakpointsArguments{
		Source:      source,
		Breakpoints: bps,
	}

	result, err := s.client.SetBreakpoints(ctx, args)
	if err != nil {
		return nil, err
	}

	s.breakpointsMu.Lock()
	s.breakpoints[path] = result
	s.breakpointsMu.Unlock()

	return result, nil
}

// ClearBreakpoints clears all breakpoints in a source file.
func (s *Session) ClearBreakpoints(ctx context.Context, path string) error {
	source := dap.Source{
		Path: path,
	}

	args := dap.SetBreakpointsArguments{
		Source:      source,
		Breakpoints: []dap.SourceBreakpoint{},
	}

	_, err := s.client.SetBreakpoints(ctx, args)
	if err != nil {
		return err
	}

	s.breakpointsMu.Lock()
	delete(s.breakpoints, path)
	s.breakpointsMu.Unlock()

	return nil
}

// GetBreakpoints returns breakpoints for a source file.
func (s *Session) GetBreakpoints(path string) []dap.Breakpoint {
	s.breakpointsMu.RLock()
	defer s.breakpointsMu.RUnlock()
	return append([]dap.Breakpoint{}, s.breakpoints[path]...)
}

// Continue resumes execution.
func (s *Session) Continue(ctx context.Context, threadID int) error {
	args := dap.ContinueArguments{
		ThreadID: threadID,
	}

	if _, err := s.client.Continue(ctx, args); err != nil {
		return err
	}

	s.setState(StateRunning)
	return nil
}

// Next performs step over.
func (s *Session) Next(ctx context.Context, threadID int) error {
	args := dap.NextArguments{
		ThreadID: threadID,
	}

	if err := s.client.Next(ctx, args); err != nil {
		return err
	}

	s.setState(StateRunning)
	return nil
}

// StepIn performs step into.
func (s *Session) StepIn(ctx context.Context, threadID int) error {
	args := dap.StepInArguments{
		ThreadID: threadID,
	}

	if err := s.client.StepIn(ctx, args); err != nil {
		return err
	}

	s.setState(StateRunning)
	return nil
}

// StepOut performs step out.
func (s *Session) StepOut(ctx context.Context, threadID int) error {
	args := dap.StepOutArguments{
		ThreadID: threadID,
	}

	if err := s.client.StepOut(ctx, args); err != nil {
		return err
	}

	s.setState(StateRunning)
	return nil
}

// Pause pauses execution.
func (s *Session) Pause(ctx context.Context, threadID int) error {
	args := dap.PauseArguments{
		ThreadID: threadID,
	}

	return s.client.Pause(ctx, args)
}

// GetThreads retrieves the current threads.
func (s *Session) GetThreads(ctx context.Context) ([]dap.Thread, error) {
	threads, err := s.client.Threads(ctx)
	if err != nil {
		return nil, err
	}

	s.threadsMu.Lock()
	s.threads = threads
	s.threadsMu.Unlock()

	return threads, nil
}

// GetStackTrace retrieves the stack trace for a thread.
func (s *Session) GetStackTrace(ctx context.Context, threadID int, startFrame, levels int) ([]dap.StackFrame, int, error) {
	args := dap.StackTraceArguments{
		ThreadID:   threadID,
		StartFrame: startFrame,
		Levels:     levels,
	}

	result, err := s.client.StackTrace(ctx, args)
	if err != nil {
		return nil, 0, err
	}

	return result.StackFrames, result.TotalFrames, nil
}

// GetScopes retrieves the scopes for a stack frame.
func (s *Session) GetScopes(ctx context.Context, frameID int) ([]dap.Scope, error) {
	args := dap.ScopesArguments{
		FrameID: frameID,
	}

	return s.client.Scopes(ctx, args)
}

// GetVariables retrieves variables from a scope or variable reference.
func (s *Session) GetVariables(ctx context.Context, variablesRef int) ([]dap.Variable, error) {
	args := dap.VariablesArguments{
		VariablesReference: variablesRef,
	}

	return s.client.Variables(ctx, args)
}

// SetVariable sets a variable value.
func (s *Session) SetVariable(ctx context.Context, variablesRef int, name, value string) (string, error) {
	args := dap.SetVariableArguments{
		VariablesReference: variablesRef,
		Name:               name,
		Value:              value,
	}

	result, err := s.client.SetVariable(ctx, args)
	if err != nil {
		return "", err
	}

	return result.Value, nil
}

// Evaluate evaluates an expression.
func (s *Session) Evaluate(ctx context.Context, expression string, frameID int, context string) (*dap.EvaluateResponseBody, error) {
	args := dap.EvaluateArguments{
		Expression: expression,
		FrameID:    frameID,
		Context:    context,
	}

	return s.client.Evaluate(ctx, args)
}

// Event handlers

func (s *Session) onInitialized() {
	s.setState(StateConfiguring)
}

func (s *Session) onStopped(body dap.StoppedEventBody) {
	// Update current thread while holding state lock
	s.stateMu.Lock()
	s.currentThread = body.ThreadID
	s.stateMu.Unlock()

	// Use setState to properly notify state change handlers
	s.setState(StateStopped)

	s.handlersMu.RLock()
	handler := s.handlers.OnStopped
	s.handlersMu.RUnlock()

	if handler != nil {
		handler(body.Reason, body.ThreadID, body.AllThreadsStopped)
	}
}

func (s *Session) onContinued(body dap.ContinuedEventBody) {
	s.setState(StateRunning)
}

func (s *Session) onExited(body dap.ExitedEventBody) {
	s.setState(StateTerminated)
}

func (s *Session) onTerminated(body dap.TerminatedEventBody) {
	s.setState(StateTerminated)

	s.handlersMu.RLock()
	handler := s.handlers.OnTerminated
	s.handlersMu.RUnlock()

	if handler != nil {
		handler()
	}
}

func (s *Session) onThread(body dap.ThreadEventBody) {
	s.handlersMu.RLock()
	handler := s.handlers.OnThreadChanged
	s.handlersMu.RUnlock()

	if handler != nil {
		handler(body.Reason, body.ThreadID)
	}
}

func (s *Session) onOutput(body dap.OutputEventBody) {
	s.handlersMu.RLock()
	handler := s.handlers.OnOutput
	s.handlersMu.RUnlock()

	if handler != nil {
		handler(body.Category, body.Output)
	}
}

func (s *Session) onBreakpoint(body dap.BreakpointEventBody) {
	s.handlersMu.RLock()
	handler := s.handlers.OnBreakpointChanged
	s.handlersMu.RUnlock()

	if handler != nil {
		handler(body.Reason, body.Breakpoint)
	}
}
