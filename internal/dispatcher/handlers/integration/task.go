package integration

import (
	"context"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/integration/task"
)

// Task action names.
const (
	ActionTaskList   = "task.list"   // List available tasks
	ActionTaskRun    = "task.run"    // Run a task
	ActionTaskStop   = "task.stop"   // Stop a running task
	ActionTaskStatus = "task.status" // Get task execution status
	ActionTaskOutput = "task.output" // Get task output
)

// TaskDiscoverer provides task discovery.
type TaskDiscoverer interface {
	// DiscoverTasks finds all tasks in the workspace.
	DiscoverTasks(ctx context.Context, workspaceRoot string) ([]*task.Task, error)
}

// TaskExecutor provides task execution.
type TaskExecutor interface {
	// Execute runs a task.
	Execute(ctx context.Context, task *task.Task) (*task.Execution, error)

	// GetExecution returns an execution by ID.
	GetExecution(id string) (*task.Execution, bool)

	// ListExecutions returns all active executions.
	ListExecutions() []*task.Execution

	// CancelExecution cancels an execution.
	CancelExecution(id string) error
}

// TaskManager combines task discovery and execution.
type TaskManager interface {
	TaskDiscoverer
	TaskExecutor
}

const taskManagerKey = "_task_manager"

// TaskHandler handles task-related actions.
type TaskHandler struct {
	discoverer TaskDiscoverer
	executor   TaskExecutor
	workspace  string
}

// NewTaskHandler creates a new task handler.
func NewTaskHandler() *TaskHandler {
	return &TaskHandler{}
}

// NewTaskHandlerWithManager creates a handler with task manager.
func NewTaskHandlerWithManager(manager TaskManager, workspace string) *TaskHandler {
	return &TaskHandler{
		discoverer: manager,
		executor:   manager,
		workspace:  workspace,
	}
}

// NewTaskHandlerWithComponents creates a handler with separate components.
func NewTaskHandlerWithComponents(discoverer TaskDiscoverer, executor TaskExecutor, workspace string) *TaskHandler {
	return &TaskHandler{
		discoverer: discoverer,
		executor:   executor,
		workspace:  workspace,
	}
}

// Namespace returns the task namespace.
func (h *TaskHandler) Namespace() string {
	return "task"
}

// CanHandle returns true if this handler can process the action.
func (h *TaskHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionTaskList, ActionTaskRun, ActionTaskStop, ActionTaskStatus, ActionTaskOutput:
		return true
	}
	return false
}

// HandleAction processes a task action.
func (h *TaskHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	switch action.Name {
	case ActionTaskList:
		return h.list(ctx)
	case ActionTaskRun:
		return h.run(action, ctx)
	case ActionTaskStop:
		return h.stop(action, ctx)
	case ActionTaskStatus:
		return h.status(action, ctx)
	case ActionTaskOutput:
		return h.output(action, ctx)
	default:
		return handler.Errorf("unknown task action: %s", action.Name)
	}
}

// getDiscoverer returns the task discoverer from handler or context.
func (h *TaskHandler) getDiscoverer(ctx *execctx.ExecutionContext) TaskDiscoverer {
	if h.discoverer != nil {
		return h.discoverer
	}
	if v, ok := ctx.GetData(taskManagerKey); ok {
		if tm, ok := v.(TaskDiscoverer); ok {
			return tm
		}
	}
	return nil
}

// getExecutor returns the task executor from handler or context.
func (h *TaskHandler) getExecutor(ctx *execctx.ExecutionContext) TaskExecutor {
	if h.executor != nil {
		return h.executor
	}
	if v, ok := ctx.GetData(taskManagerKey); ok {
		if tm, ok := v.(TaskExecutor); ok {
			return tm
		}
	}
	return nil
}

// getWorkspace returns the workspace root.
func (h *TaskHandler) getWorkspace(ctx *execctx.ExecutionContext) string {
	if h.workspace != "" {
		return h.workspace
	}
	if v, ok := ctx.GetData("_workspace_root"); ok {
		if ws, ok := v.(string); ok {
			return ws
		}
	}
	return ""
}

func (h *TaskHandler) list(ctx *execctx.ExecutionContext) handler.Result {
	discoverer := h.getDiscoverer(ctx)
	if discoverer == nil {
		return handler.Errorf("task.list: no task discoverer available")
	}

	workspace := h.getWorkspace(ctx)
	if workspace == "" {
		return handler.Errorf("task.list: no workspace root configured")
	}

	// Use a reasonable timeout for discovery
	discoverCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasks, err := discoverer.DiscoverTasks(discoverCtx, workspace)
	if err != nil {
		return handler.Error(err)
	}

	// Convert to result data
	taskInfos := make([]map[string]string, len(tasks))
	for i, t := range tasks {
		taskInfos[i] = map[string]string{
			"name":        t.Name,
			"source":      string(t.Source),
			"description": t.Description,
			"command":     t.Command,
		}
	}

	return handler.Success().
		WithData("tasks", taskInfos).
		WithData("count", len(tasks)).
		WithMessage(formatTaskList(tasks))
}

func (h *TaskHandler) run(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	discoverer := h.getDiscoverer(ctx)
	executor := h.getExecutor(ctx)
	if discoverer == nil || executor == nil {
		return handler.Errorf("task.run: no task manager available")
	}

	taskName := action.Args.GetString("name")
	if taskName == "" {
		return handler.Errorf("task.run: task name required")
	}

	workspace := h.getWorkspace(ctx)
	if workspace == "" {
		return handler.Errorf("task.run: no workspace root configured")
	}

	// Find the task
	discoverCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasks, err := discoverer.DiscoverTasks(discoverCtx, workspace)
	if err != nil {
		return handler.Error(err)
	}

	var targetTask *task.Task
	for _, t := range tasks {
		if t.Name == taskName {
			targetTask = t
			break
		}
	}

	if targetTask == nil {
		return handler.Errorf("task.run: task %q not found", taskName)
	}

	// Execute the task
	exec, err := executor.Execute(context.Background(), targetTask)
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("executionId", exec.ID).
		WithData("task", taskName).
		WithData("state", string(exec.State)).
		WithMessage("Started task: " + taskName + " (id: " + exec.ID + ")")
}

func (h *TaskHandler) stop(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	executor := h.getExecutor(ctx)
	if executor == nil {
		return handler.Errorf("task.stop: no task executor available")
	}

	execID := action.Args.GetString("id")
	if execID == "" {
		return handler.Errorf("task.stop: execution id required")
	}

	if err := executor.CancelExecution(execID); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("executionId", execID).
		WithMessage("Stopped execution: " + execID)
}

func (h *TaskHandler) status(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	executor := h.getExecutor(ctx)
	if executor == nil {
		return handler.Errorf("task.status: no task executor available")
	}

	execID := action.Args.GetString("id")
	if execID == "" {
		// List all executions
		executions := executor.ListExecutions()

		execInfos := make([]map[string]any, len(executions))
		for i, exec := range executions {
			execInfos[i] = map[string]any{
				"id":       exec.ID,
				"task":     exec.Task.Name,
				"state":    string(exec.State),
				"exitCode": exec.ExitCode,
			}
		}

		return handler.Success().
			WithData("executions", execInfos).
			WithData("count", len(executions)).
			WithMessage(formatExecutionList(executions))
	}

	// Get specific execution
	exec, ok := executor.GetExecution(execID)
	if !ok {
		return handler.Errorf("task.status: execution %q not found", execID)
	}

	return handler.Success().
		WithData("id", exec.ID).
		WithData("task", exec.Task.Name).
		WithData("state", string(exec.State)).
		WithData("exitCode", exec.ExitCode).
		WithData("duration", exec.Duration().String()).
		WithMessage(formatExecutionStatus(exec))
}

func (h *TaskHandler) output(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	executor := h.getExecutor(ctx)
	if executor == nil {
		return handler.Errorf("task.output: no task executor available")
	}

	execID := action.Args.GetString("id")
	if execID == "" {
		return handler.Errorf("task.output: execution id required")
	}

	exec, ok := executor.GetExecution(execID)
	if !ok {
		return handler.Errorf("task.output: execution %q not found", execID)
	}

	// Get output lines
	lines := exec.Output()
	output := ""
	for _, line := range lines {
		output += line.Content + "\n"
	}

	return handler.Success().
		WithData("output", output).
		WithData("lineCount", len(lines)).
		WithMessage(output)
}

// Helper functions

func formatTaskList(tasks []*task.Task) string {
	if len(tasks) == 0 {
		return "No tasks found"
	}

	msg := "Available tasks:\n"
	for _, t := range tasks {
		msg += "  " + t.Name
		if t.Description != "" {
			msg += " - " + t.Description
		}
		msg += " (" + string(t.Source) + ")\n"
	}
	return msg
}

func formatExecutionList(executions []*task.Execution) string {
	if len(executions) == 0 {
		return "No running tasks"
	}

	msg := "Task executions:\n"
	for _, exec := range executions {
		msg += "  " + exec.ID + " " + exec.Task.Name + " [" + string(exec.State) + "]\n"
	}
	return msg
}

func formatExecutionStatus(exec *task.Execution) string {
	msg := "Task: " + exec.Task.Name + "\n"
	msg += "State: " + string(exec.State) + "\n"
	msg += "Duration: " + exec.Duration().String()
	if exec.ExitCode >= 0 {
		msg += "\nExit code: " + itoa(exec.ExitCode)
	}
	return msg
}
