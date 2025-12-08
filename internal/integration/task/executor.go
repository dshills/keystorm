package task

import (
	"context"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ExecutorConfig configures the task executor.
type ExecutorConfig struct {
	// DefaultShell is the shell to use for shell tasks.
	DefaultShell string

	// DefaultShellArgs are the default arguments for the shell.
	DefaultShellArgs []string

	// DefaultEnv are environment variables to add to all tasks.
	DefaultEnv map[string]string

	// WorkingDir is the default working directory.
	WorkingDir string

	// OutputBufferSize is the size of output buffers.
	OutputBufferSize int

	// MaxConcurrent is the maximum concurrent task executions (0 = unlimited).
	MaxConcurrent int
}

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	return ExecutorConfig{
		DefaultShell:     shell,
		DefaultShellArgs: []string{"-c"},
		OutputBufferSize: 64 * 1024, // 64KB
		MaxConcurrent:    4,
	}
}

// ExecutionState represents the state of a task execution.
type ExecutionState string

const (
	// ExecutionStatePending indicates the task is waiting to run.
	ExecutionStatePending ExecutionState = "pending"
	// ExecutionStateRunning indicates the task is currently running.
	ExecutionStateRunning ExecutionState = "running"
	// ExecutionStateSucceeded indicates the task completed successfully.
	ExecutionStateSucceeded ExecutionState = "succeeded"
	// ExecutionStateFailed indicates the task failed.
	ExecutionStateFailed ExecutionState = "failed"
	// ExecutionStateCanceled indicates the task was canceled.
	ExecutionStateCanceled ExecutionState = "canceled"
)

// Execution represents a running or completed task execution.
type Execution struct {
	// ID is a unique identifier for this execution.
	ID string

	// Task is the task being executed.
	Task *Task

	// State is the current execution state.
	State ExecutionState

	// StartTime is when execution started.
	StartTime time.Time

	// EndTime is when execution ended.
	EndTime time.Time

	// ExitCode is the process exit code (-1 if not yet finished).
	ExitCode int

	// Error is any error that occurred.
	Error error

	// Problems are problems found in the output.
	Problems []Problem

	// cmd is the underlying command.
	cmd *osexec.Cmd

	// cancel cancels the execution context.
	cancel context.CancelFunc

	// output handles output processing.
	output *OutputProcessor

	// done is closed when execution completes.
	done chan struct{}

	// doneOnce ensures done is closed exactly once.
	doneOnce sync.Once

	// notifiedComplete ensures completion is notified once.
	notifiedComplete bool

	// mu protects state changes.
	mu sync.RWMutex
}

// Executor manages task execution.
type Executor struct {
	config ExecutorConfig

	// executions tracks active executions.
	executions   map[string]*Execution
	executionsMu sync.RWMutex

	// sem limits concurrent executions.
	sem chan struct{}

	// variables handles variable substitution.
	variables *VariableResolver

	// problems handles problem matching.
	problems *ProblemMatcher

	// security validates tasks before execution.
	security *SecurityValidator

	// listeners receive execution events.
	listeners   []ExecutionListener
	listenersMu sync.RWMutex

	// idCounter generates unique execution IDs.
	idCounter   int64
	idCounterMu sync.Mutex
}

// ExecutionListener receives execution events.
type ExecutionListener interface {
	// OnExecutionStarted is called when execution starts.
	OnExecutionStarted(exec *Execution)

	// OnExecutionOutput is called for each output line.
	OnExecutionOutput(exec *Execution, line OutputLine)

	// OnExecutionProblem is called when a problem is detected.
	OnExecutionProblem(exec *Execution, problem Problem)

	// OnExecutionCompleted is called when execution completes.
	OnExecutionCompleted(exec *Execution)
}

// NewExecutor creates a new task executor.
func NewExecutor(config ExecutorConfig) *Executor {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 4
	}

	// Initialize security validator with default config
	secConfig := DefaultSecurityConfig()
	secConfig.WorkspaceRoot = config.WorkingDir

	return &Executor{
		config:     config,
		executions: make(map[string]*Execution),
		sem:        make(chan struct{}, config.MaxConcurrent),
		variables:  NewVariableResolver(),
		problems:   NewProblemMatcher(),
		security:   NewSecurityValidator(secConfig),
	}
}

// NewExecutorWithSecurity creates a new task executor with custom security config.
func NewExecutorWithSecurity(config ExecutorConfig, secConfig SecurityConfig) *Executor {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 4
	}

	if secConfig.WorkspaceRoot == "" {
		secConfig.WorkspaceRoot = config.WorkingDir
	}

	return &Executor{
		config:     config,
		executions: make(map[string]*Execution),
		sem:        make(chan struct{}, config.MaxConcurrent),
		variables:  NewVariableResolver(),
		problems:   NewProblemMatcher(),
		security:   NewSecurityValidator(secConfig),
	}
}

// Security returns the security validator for configuration.
func (e *Executor) Security() *SecurityValidator {
	return e.security
}

// AddListener adds an execution listener.
func (e *Executor) AddListener(listener ExecutionListener) {
	e.listenersMu.Lock()
	defer e.listenersMu.Unlock()
	e.listeners = append(e.listeners, listener)
}

// RemoveListener removes an execution listener.
func (e *Executor) RemoveListener(listener ExecutionListener) {
	e.listenersMu.Lock()
	defer e.listenersMu.Unlock()

	for i, l := range e.listeners {
		if l == listener {
			e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
			return
		}
	}
}

// ValidateTask validates a task against security policies without executing it.
// Use this to check if a task requires user confirmation before execution.
func (e *Executor) ValidateTask(task *Task) *ValidationResult {
	return e.security.Validate(task)
}

// Execute runs a task and returns the execution handle.
// The task is validated against security policies before execution.
// If validation fails, an error is returned with details about the violations.
func (e *Executor) Execute(ctx context.Context, task *Task) (*Execution, error) {
	return e.ExecuteWithEnv(ctx, task, nil)
}

// ExecuteWithEnv runs a task with additional environment variables.
// The task is validated against security policies before execution.
func (e *Executor) ExecuteWithEnv(ctx context.Context, task *Task, env map[string]string) (*Execution, error) {
	// SECURITY: Validate task before execution
	validation := e.security.Validate(task)
	if !validation.Valid {
		// Build error message from violations
		var errMsg strings.Builder
		errMsg.WriteString("task security validation failed:")
		for _, v := range validation.Violations {
			errMsg.WriteString("\n  - ")
			errMsg.WriteString(v.Error())
		}
		return nil, fmt.Errorf("%s", errMsg.String())
	}

	// Generate execution ID
	execID := e.generateID()

	// Create execution context
	execCtx, cancel := context.WithCancel(ctx)

	// Create execution
	exec := &Execution{
		ID:       execID,
		Task:     task,
		State:    ExecutionStatePending,
		ExitCode: -1,
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	// Register execution
	e.executionsMu.Lock()
	e.executions[execID] = exec
	e.executionsMu.Unlock()

	// Start execution in background
	go e.runExecution(execCtx, exec, env)

	return exec, nil
}

// ExecuteConfirmed runs a task that has already been validated and confirmed.
// Use this after getting user confirmation for tasks that require it.
// This bypasses security validation - caller is responsible for validation.
func (e *Executor) ExecuteConfirmed(ctx context.Context, task *Task, env map[string]string) (*Execution, error) {
	// Generate execution ID
	execID := e.generateID()

	// Create execution context
	execCtx, cancel := context.WithCancel(ctx)

	// Create execution
	exec := &Execution{
		ID:       execID,
		Task:     task,
		State:    ExecutionStatePending,
		ExitCode: -1,
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	// Register execution
	e.executionsMu.Lock()
	e.executions[execID] = exec
	e.executionsMu.Unlock()

	// Start execution in background
	go e.runExecution(execCtx, exec, env)

	return exec, nil
}

// ExecuteSync runs a task synchronously and waits for completion.
func (e *Executor) ExecuteSync(ctx context.Context, task *Task) (*Execution, error) {
	exec, err := e.Execute(ctx, task)
	if err != nil {
		return nil, err
	}

	// Wait for completion
	<-exec.Done()

	return exec, nil
}

// GetExecution returns an execution by ID.
func (e *Executor) GetExecution(id string) (*Execution, bool) {
	e.executionsMu.RLock()
	defer e.executionsMu.RUnlock()
	exec, ok := e.executions[id]
	return exec, ok
}

// ListExecutions returns all active executions.
func (e *Executor) ListExecutions() []*Execution {
	e.executionsMu.RLock()
	defer e.executionsMu.RUnlock()

	result := make([]*Execution, 0, len(e.executions))
	for _, exec := range e.executions {
		result = append(result, exec)
	}
	return result
}

// CancelExecution cancels an execution by ID.
func (e *Executor) CancelExecution(id string) error {
	e.executionsMu.RLock()
	exec, ok := e.executions[id]
	e.executionsMu.RUnlock()

	if !ok {
		return fmt.Errorf("execution not found: %s", id)
	}

	exec.Cancel()
	return nil
}

// CancelAll cancels all active executions.
func (e *Executor) CancelAll() {
	e.executionsMu.RLock()
	executions := make([]*Execution, 0, len(e.executions))
	for _, exec := range e.executions {
		executions = append(executions, exec)
	}
	e.executionsMu.RUnlock()

	for _, exec := range executions {
		exec.Cancel()
	}
}

// CleanupCompleted removes completed executions from tracking.
func (e *Executor) CleanupCompleted() int {
	e.executionsMu.Lock()
	defer e.executionsMu.Unlock()

	count := 0
	for id, exec := range e.executions {
		exec.mu.RLock()
		state := exec.State
		exec.mu.RUnlock()

		if state == ExecutionStateSucceeded || state == ExecutionStateFailed || state == ExecutionStateCanceled {
			delete(e.executions, id)
			count++
		}
	}
	return count
}

// runExecution handles the actual task execution.
func (e *Executor) runExecution(ctx context.Context, exec *Execution, extraEnv map[string]string) {
	// Acquire semaphore
	select {
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	case <-ctx.Done():
		e.setExecutionState(exec, ExecutionStateCanceled, ctx.Err())
		return
	}

	// Build command
	cmd, err := e.buildCommand(ctx, exec.Task, extraEnv)
	if err != nil {
		e.setExecutionState(exec, ExecutionStateFailed, err)
		return
	}

	exec.mu.Lock()
	exec.cmd = cmd
	exec.mu.Unlock()

	// Set up output processing
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		e.setExecutionState(exec, ExecutionStateFailed, fmt.Errorf("stdout pipe: %w", err))
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		e.setExecutionState(exec, ExecutionStateFailed, fmt.Errorf("stderr pipe: %w", err))
		return
	}

	// Create output processor
	outputProc := NewOutputProcessor(e.config.OutputBufferSize)
	exec.mu.Lock()
	exec.output = outputProc
	exec.mu.Unlock()

	// Get problem matcher for this task
	var matcher *CompiledMatcher
	if exec.Task.ProblemMatcher != "" {
		matcher = e.problems.GetMatcher(exec.Task.ProblemMatcher)
	}

	// Start output processing
	var outputWg sync.WaitGroup
	outputWg.Add(2)

	go func() {
		defer outputWg.Done()
		e.processOutput(exec, stdout, OutputStreamStdout, matcher)
	}()

	go func() {
		defer outputWg.Done()
		e.processOutput(exec, stderr, OutputStreamStderr, matcher)
	}()

	// Start execution
	exec.mu.Lock()
	exec.StartTime = time.Now()
	exec.State = ExecutionStateRunning
	exec.mu.Unlock()

	e.notifyStarted(exec)

	if err := cmd.Start(); err != nil {
		e.setExecutionState(exec, ExecutionStateFailed, fmt.Errorf("start: %w", err))
		return
	}

	// Wait for output processing to complete
	outputWg.Wait()

	// Wait for command to complete
	err = cmd.Wait()

	exec.mu.Lock()
	exec.EndTime = time.Now()

	if ctx.Err() != nil {
		exec.State = ExecutionStateCanceled
		exec.Error = ctx.Err()
	} else if err != nil {
		exec.State = ExecutionStateFailed
		exec.Error = err
		if exitErr, ok := err.(*osexec.ExitError); ok {
			exec.ExitCode = exitErr.ExitCode()
		}
	} else {
		exec.State = ExecutionStateSucceeded
		exec.ExitCode = 0
	}
	exec.mu.Unlock()

	e.notifyCompleted(exec)
}

// buildCommand creates the osexec.Cmd for a task.
func (e *Executor) buildCommand(ctx context.Context, task *Task, extraEnv map[string]string) (*osexec.Cmd, error) {
	// Resolve variables in command and args
	command := e.variables.Resolve(task.Command, task)
	args := make([]string, len(task.Args))
	for i, arg := range task.Args {
		args[i] = e.variables.Resolve(arg, task)
	}

	// Validate command is not empty
	if command == "" {
		return nil, fmt.Errorf("empty command")
	}

	var cmd *osexec.Cmd

	switch task.Type {
	case TaskTypeShell:
		// For shell tasks, combine command and args with proper escaping
		shellCmd := shellEscape(command)
		for _, arg := range args {
			shellCmd += " " + shellEscape(arg)
		}
		cmd = osexec.CommandContext(ctx, e.config.DefaultShell, append(e.config.DefaultShellArgs, shellCmd)...)

	case TaskTypeProcess, TaskTypeMake, TaskTypeGo:
		// For process tasks, run the command directly
		cmd = osexec.CommandContext(ctx, command, args...)

	case TaskTypeNPM:
		// For npm tasks, use the detected package manager
		cmd = osexec.CommandContext(ctx, command, args...)

	default:
		// Default to shell execution with proper escaping
		shellCmd := shellEscape(command)
		for _, arg := range args {
			shellCmd += " " + shellEscape(arg)
		}
		cmd = osexec.CommandContext(ctx, e.config.DefaultShell, append(e.config.DefaultShellArgs, shellCmd)...)
	}

	// Set working directory
	cwd := task.Cwd
	if cwd == "" {
		cwd = e.config.WorkingDir
	}
	if cwd != "" {
		cwd = e.variables.Resolve(cwd, task)
		cmd.Dir = cwd
	}

	// Build environment
	cmd.Env = e.buildEnvironment(task, extraEnv)

	// Set process group for proper cleanup
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return cmd, nil
}

// buildEnvironment creates the environment for a task.
// Precedence (highest to lowest): extraEnv > task.Env > defaultEnv > os.Environ()
func (e *Executor) buildEnvironment(task *Task, extraEnv map[string]string) []string {
	// Use a map to ensure unique keys with proper precedence
	envMap := make(map[string]string)

	// Start with current environment (lowest precedence)
	for _, kv := range os.Environ() {
		if idx := strings.Index(kv, "="); idx > 0 {
			envMap[kv[:idx]] = kv[idx+1:]
		}
	}

	// Add default environment (overrides os.Environ)
	for k, v := range e.config.DefaultEnv {
		envMap[k] = e.variables.Resolve(v, task)
	}

	// Add task environment (overrides default)
	for k, v := range task.Env {
		envMap[k] = e.variables.Resolve(v, task)
	}

	// Add extra environment (highest precedence)
	for k, v := range extraEnv {
		envMap[k] = e.variables.Resolve(v, task)
	}

	// Convert map back to slice with deterministic ordering
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(envMap))
	for _, k := range keys {
		env = append(env, k+"="+envMap[k])
	}

	return env
}

// shellEscape escapes a string for safe use in shell commands.
// It wraps arguments containing special characters in single quotes.
func shellEscape(s string) string {
	// If empty, return empty quoted string
	if s == "" {
		return "''"
	}

	// Check if escaping is needed
	needsEscape := false
	for _, c := range s {
		if !isShellSafe(c) {
			needsEscape = true
			break
		}
	}

	if !needsEscape {
		return s
	}

	// Use single quotes, escaping any existing single quotes
	// 'foo'\''bar' -> foo'bar
	var result strings.Builder
	result.WriteByte('\'')
	for _, c := range s {
		if c == '\'' {
			result.WriteString("'\\''")
		} else {
			result.WriteRune(c)
		}
	}
	result.WriteByte('\'')
	return result.String()
}

// isShellSafe returns true if the character doesn't need escaping.
// Note: We intentionally exclude ':' as it has special meaning in some shell contexts.
func isShellSafe(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '/'
}

// processOutput reads and processes output from a stream.
func (e *Executor) processOutput(exec *Execution, r io.Reader, stream OutputStream, matcher *CompiledMatcher) {
	// Process returns an error if scanning fails (e.g., token too long)
	// We ignore this error as there's no good way to surface it during execution
	// and the output is already partially captured
	_ = exec.output.Process(r, stream, func(line OutputLine) {
		// Notify listeners
		e.notifyOutput(exec, line)

		// Check for problems
		if matcher != nil {
			if problem, ok := matcher.Match(line.Content); ok {
				exec.mu.Lock()
				exec.Problems = append(exec.Problems, problem)
				exec.mu.Unlock()
				e.notifyProblem(exec, problem)
			}
		}
	})
}

// setExecutionState sets the execution state and error.
func (e *Executor) setExecutionState(exec *Execution, state ExecutionState, err error) {
	exec.mu.Lock()
	exec.State = state
	exec.Error = err
	if exec.EndTime.IsZero() {
		exec.EndTime = time.Now()
	}
	exec.mu.Unlock()

	if state != ExecutionStatePending && state != ExecutionStateRunning {
		e.notifyCompleted(exec)
	}
}

// generateID generates a unique execution ID.
func (e *Executor) generateID() string {
	e.idCounterMu.Lock()
	e.idCounter++
	id := e.idCounter
	e.idCounterMu.Unlock()

	return fmt.Sprintf("exec-%d-%d", time.Now().UnixNano(), id)
}

// Notification helpers

func (e *Executor) notifyStarted(exec *Execution) {
	e.listenersMu.RLock()
	listeners := make([]ExecutionListener, len(e.listeners))
	copy(listeners, e.listeners)
	e.listenersMu.RUnlock()

	for _, l := range listeners {
		l.OnExecutionStarted(exec)
	}
}

func (e *Executor) notifyOutput(exec *Execution, line OutputLine) {
	e.listenersMu.RLock()
	listeners := make([]ExecutionListener, len(e.listeners))
	copy(listeners, e.listeners)
	e.listenersMu.RUnlock()

	for _, l := range listeners {
		l.OnExecutionOutput(exec, line)
	}
}

func (e *Executor) notifyProblem(exec *Execution, problem Problem) {
	e.listenersMu.RLock()
	listeners := make([]ExecutionListener, len(e.listeners))
	copy(listeners, e.listeners)
	e.listenersMu.RUnlock()

	for _, l := range listeners {
		l.OnExecutionProblem(exec, problem)
	}
}

func (e *Executor) notifyCompleted(exec *Execution) {
	// Ensure we only notify once
	exec.mu.Lock()
	if exec.notifiedComplete {
		exec.mu.Unlock()
		return
	}
	exec.notifiedComplete = true
	exec.mu.Unlock()

	// Mark execution as done (closes the done channel)
	exec.markDone()

	e.listenersMu.RLock()
	listeners := make([]ExecutionListener, len(e.listeners))
	copy(listeners, e.listeners)
	e.listenersMu.RUnlock()

	for _, l := range listeners {
		l.OnExecutionCompleted(exec)
	}
}

// Execution methods

// Cancel cancels the execution.
func (ex *Execution) Cancel() {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	if ex.cancel != nil {
		ex.cancel()
	}

	// Kill the process group if running
	if ex.cmd != nil && ex.cmd.Process != nil {
		// Kill the process group
		_ = syscall.Kill(-ex.cmd.Process.Pid, syscall.SIGKILL)
	}
}

// Done returns a channel that's closed when execution completes.
func (ex *Execution) Done() <-chan struct{} {
	return ex.done
}

// markDone closes the done channel exactly once.
func (ex *Execution) markDone() {
	ex.doneOnce.Do(func() {
		close(ex.done)
	})
}

// IsRunning returns true if the execution is still running.
func (ex *Execution) IsRunning() bool {
	ex.mu.RLock()
	defer ex.mu.RUnlock()
	return ex.State == ExecutionStateRunning
}

// Duration returns the execution duration.
func (ex *Execution) Duration() time.Duration {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	if ex.StartTime.IsZero() {
		return 0
	}

	end := ex.EndTime
	if end.IsZero() {
		end = time.Now()
	}

	return end.Sub(ex.StartTime)
}

// Output returns all captured output lines.
func (ex *Execution) Output() []OutputLine {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	if ex.output == nil {
		return nil
	}
	return ex.output.Lines()
}

// StdoutLines returns only stdout lines.
func (ex *Execution) StdoutLines() []OutputLine {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	if ex.output == nil {
		return nil
	}
	return ex.output.StdoutLines()
}

// StderrLines returns only stderr lines.
func (ex *Execution) StderrLines() []OutputLine {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	if ex.output == nil {
		return nil
	}
	return ex.output.StderrLines()
}
