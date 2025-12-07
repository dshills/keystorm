package integration

import (
	"context"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/integration/debug"
	"github.com/dshills/keystorm/internal/integration/git"
	"github.com/dshills/keystorm/internal/integration/task"
)

// Mock implementations

type mockGitManager struct {
	status   *git.Status
	branch   string
	branches []*git.Reference
	commit   *git.Commit
	diff     string
	commits  []*git.Commit
	blame    []git.BlameLine
	err      error
}

func (m *mockGitManager) Status() (*git.Status, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.status, nil
}

func (m *mockGitManager) CurrentBranch() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.branch, nil
}

func (m *mockGitManager) Branches() ([]*git.Reference, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.branches, nil
}

func (m *mockGitManager) Checkout(branch string) error {
	return m.err
}

func (m *mockGitManager) Commit(message string, opts git.CommitOptions) (*git.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commit, nil
}

func (m *mockGitManager) Add(paths ...string) error {
	return m.err
}

func (m *mockGitManager) AddAll() error {
	return m.err
}

func (m *mockGitManager) Diff(staged bool) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.diff, nil
}

func (m *mockGitManager) DiffFile(path string, staged bool) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.diff, nil
}

func (m *mockGitManager) Log(n int) ([]*git.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commits, nil
}

func (m *mockGitManager) Pull() error {
	return m.err
}

func (m *mockGitManager) Push() error {
	return m.err
}

func (m *mockGitManager) Stash(message string) error {
	return m.err
}

func (m *mockGitManager) StashPop() error {
	return m.err
}

func (m *mockGitManager) Blame(path string) ([]git.BlameLine, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.blame, nil
}

// Task mocks

type mockTaskManager struct {
	tasks      []*task.Task
	execution  *task.Execution
	executions []*task.Execution
	err        error
}

func (m *mockTaskManager) DiscoverTasks(ctx context.Context, workspaceRoot string) ([]*task.Task, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tasks, nil
}

func (m *mockTaskManager) Execute(ctx context.Context, t *task.Task) (*task.Execution, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.execution, nil
}

func (m *mockTaskManager) GetExecution(id string) (*task.Execution, bool) {
	for _, e := range m.executions {
		if e.ID == id {
			return e, true
		}
	}
	return nil, false
}

func (m *mockTaskManager) ListExecutions() []*task.Execution {
	return m.executions
}

func (m *mockTaskManager) CancelExecution(id string) error {
	return m.err
}

// Debug mocks

type mockDebugSession struct {
	id         string
	state      debug.SessionState
	variables  []debug.Variable
	frames     []debug.StackFrame
	evalResult string
	err        error
}

func (m *mockDebugSession) ID() string {
	return m.id
}

func (m *mockDebugSession) State() debug.SessionState {
	return m.state
}

func (m *mockDebugSession) Continue() error {
	return m.err
}

func (m *mockDebugSession) StepOver() error {
	return m.err
}

func (m *mockDebugSession) StepInto() error {
	return m.err
}

func (m *mockDebugSession) StepOut() error {
	return m.err
}

func (m *mockDebugSession) Pause() error {
	return m.err
}

func (m *mockDebugSession) Variables() ([]debug.Variable, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.variables, nil
}

func (m *mockDebugSession) StackTrace() ([]debug.StackFrame, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.frames, nil
}

func (m *mockDebugSession) Evaluate(expression string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.evalResult, nil
}

type mockDebugManager struct {
	sessions    []DebugSession
	breakpoints []debug.Breakpoint
	newSession  DebugSession
	newBpID     string
	err         error
}

func (m *mockDebugManager) StartSession(config DebugConfig) (DebugSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.newSession, nil
}

func (m *mockDebugManager) StopSession(sessionID string) error {
	return m.err
}

func (m *mockDebugManager) GetSession(sessionID string) (DebugSession, bool) {
	for _, s := range m.sessions {
		if s.ID() == sessionID {
			return s, true
		}
	}
	return nil, false
}

func (m *mockDebugManager) ListSessions() []DebugSession {
	return m.sessions
}

func (m *mockDebugManager) SetBreakpoint(file string, line int) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.newBpID, nil
}

func (m *mockDebugManager) RemoveBreakpoint(id string) error {
	return m.err
}

func (m *mockDebugManager) ListBreakpoints() []debug.Breakpoint {
	return m.breakpoints
}

// Tests

// newArgs is a helper to create ActionArgs with Extra map.
func newArgs() input.ActionArgs {
	return input.ActionArgs{Extra: make(map[string]interface{})}
}

// withString returns ActionArgs with a string value set.
func withString(args input.ActionArgs, key, value string) input.ActionArgs {
	if args.Extra == nil {
		args.Extra = make(map[string]interface{})
	}
	args.Extra[key] = value
	return args
}

// withInt returns ActionArgs with an int value set.
func withInt(args input.ActionArgs, key string, value int) input.ActionArgs {
	if args.Extra == nil {
		args.Extra = make(map[string]interface{})
	}
	args.Extra[key] = value
	return args
}

func TestGitHandler_Namespace(t *testing.T) {
	h := NewGitHandler()
	if h.Namespace() != "git" {
		t.Errorf("expected namespace 'git', got %q", h.Namespace())
	}
}

func TestGitHandler_CanHandle(t *testing.T) {
	h := NewGitHandler()

	tests := []struct {
		action string
		want   bool
	}{
		{ActionGitStatus, true},
		{ActionGitBranch, true},
		{ActionGitCommit, true},
		{ActionGitDiff, true},
		{"git.invalid", false},
		{"other.action", false},
	}

	for _, tt := range tests {
		got := h.CanHandle(tt.action)
		if got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.action, got, tt.want)
		}
	}
}

func TestGitHandler_Status(t *testing.T) {
	mock := &mockGitManager{
		status: &git.Status{
			Branch:    "main",
			Ahead:     2,
			Behind:    1,
			Staged:    []git.FileStatus{{Path: "file.go", Status: git.StatusModified}},
			Untracked: []string{"new.txt"},
		},
	}

	h := NewGitHandlerWithManager(mock)
	ctx := execctx.New()

	action := input.Action{Name: ActionGitStatus}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v: %s", result.Status, result.Error)
	}

	if branch, _ := result.Data["branch"].(string); branch != "main" {
		t.Errorf("expected branch 'main', got %q", branch)
	}
}

func TestGitHandler_Branch(t *testing.T) {
	mock := &mockGitManager{branch: "feature-x"}
	h := NewGitHandlerWithManager(mock)
	ctx := execctx.New()

	action := input.Action{Name: ActionGitBranch}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v", result.Status)
	}

	if branch, _ := result.Data["branch"].(string); branch != "feature-x" {
		t.Errorf("expected branch 'feature-x', got %q", branch)
	}
}

func TestGitHandler_Commit(t *testing.T) {
	mock := &mockGitManager{
		commit: &git.Commit{
			Hash:      "abc123def456",
			ShortHash: "abc123d",
			Message:   "test commit",
		},
	}

	h := NewGitHandlerWithManager(mock)
	ctx := execctx.New()

	action := input.Action{
		Name: ActionGitCommit,
		Args: withString(newArgs(), "message", "test commit"),
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v: %s", result.Status, result.Error)
	}

	if hash, _ := result.Data["shortHash"].(string); hash != "abc123d" {
		t.Errorf("expected shortHash 'abc123d', got %q", hash)
	}
}

func TestGitHandler_NoManager(t *testing.T) {
	h := NewGitHandler()
	ctx := execctx.New()

	action := input.Action{Name: ActionGitStatus}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected error, got %v", result.Status)
	}
}

func TestTaskHandler_Namespace(t *testing.T) {
	h := NewTaskHandler()
	if h.Namespace() != "task" {
		t.Errorf("expected namespace 'task', got %q", h.Namespace())
	}
}

func TestTaskHandler_CanHandle(t *testing.T) {
	h := NewTaskHandler()

	tests := []struct {
		action string
		want   bool
	}{
		{ActionTaskList, true},
		{ActionTaskRun, true},
		{ActionTaskStop, true},
		{ActionTaskStatus, true},
		{"task.invalid", false},
	}

	for _, tt := range tests {
		got := h.CanHandle(tt.action)
		if got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.action, got, tt.want)
		}
	}
}

func TestTaskHandler_List(t *testing.T) {
	mock := &mockTaskManager{
		tasks: []*task.Task{
			{Name: "build", Description: "Build project", Source: "makefile"},
			{Name: "test", Description: "Run tests", Source: "makefile"},
		},
	}

	h := NewTaskHandlerWithManager(mock, "/workspace")
	ctx := execctx.New()

	action := input.Action{Name: ActionTaskList}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v: %s", result.Status, result.Error)
	}

	count, _ := result.Data["count"].(int)
	if count != 2 {
		t.Errorf("expected 2 tasks, got %d", count)
	}
}

func TestTaskHandler_Run(t *testing.T) {
	testTask := &task.Task{Name: "build", Command: "make build"}
	mock := &mockTaskManager{
		tasks: []*task.Task{testTask},
		execution: &task.Execution{
			ID:    "exec-1",
			Task:  testTask,
			State: task.ExecutionStateRunning,
		},
	}

	h := NewTaskHandlerWithManager(mock, "/workspace")
	ctx := execctx.New()

	action := input.Action{
		Name: ActionTaskRun,
		Args: withString(newArgs(), "name", "build"),
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v: %s", result.Status, result.Error)
	}

	if execID, _ := result.Data["executionId"].(string); execID != "exec-1" {
		t.Errorf("expected executionId 'exec-1', got %q", execID)
	}
}

func TestDebugHandler_Namespace(t *testing.T) {
	h := NewDebugHandler()
	if h.Namespace() != "debug" {
		t.Errorf("expected namespace 'debug', got %q", h.Namespace())
	}
}

func TestDebugHandler_CanHandle(t *testing.T) {
	h := NewDebugHandler()

	tests := []struct {
		action string
		want   bool
	}{
		{ActionDebugStart, true},
		{ActionDebugStop, true},
		{ActionDebugContinue, true},
		{ActionDebugStepOver, true},
		{ActionDebugBreakpointSet, true},
		{"debug.invalid", false},
	}

	for _, tt := range tests {
		got := h.CanHandle(tt.action)
		if got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.action, got, tt.want)
		}
	}
}

func TestDebugHandler_Start(t *testing.T) {
	session := &mockDebugSession{
		id:    "session-1",
		state: debug.StateStopped,
	}
	mock := &mockDebugManager{
		newSession: session,
	}

	h := NewDebugHandlerWithManager(mock)
	ctx := execctx.New()

	args := newArgs()
	args = withString(args, "adapter", "delve")
	args = withString(args, "program", "./main.go")
	action := input.Action{
		Name: ActionDebugStart,
		Args: args,
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v: %s", result.Status, result.Error)
	}

	if sessionID, _ := result.Data["sessionId"].(string); sessionID != "session-1" {
		t.Errorf("expected sessionId 'session-1', got %q", sessionID)
	}
}

func TestDebugHandler_SetBreakpoint(t *testing.T) {
	mock := &mockDebugManager{
		newBpID: "bp-1",
	}

	h := NewDebugHandlerWithManager(mock)
	ctx := execctx.New()
	ctx.FilePath = "/path/to/file.go"

	action := input.Action{
		Name: ActionDebugBreakpointSet,
		Args: withInt(newArgs(), "line", 42),
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v: %s", result.Status, result.Error)
	}

	if bpID, _ := result.Data["breakpointId"].(string); bpID != "bp-1" {
		t.Errorf("expected breakpointId 'bp-1', got %q", bpID)
	}
}

func TestDebugHandler_Variables(t *testing.T) {
	session := &mockDebugSession{
		id:    "session-1",
		state: debug.StateStopped,
		variables: []debug.Variable{
			{Name: "x", Value: "42", Type: "int"},
			{Name: "name", Value: `"test"`, Type: "string"},
		},
	}
	mock := &mockDebugManager{
		sessions: []DebugSession{session},
	}

	h := NewDebugHandlerWithManager(mock)
	ctx := execctx.New()

	action := input.Action{
		Name: ActionDebugVariables,
		Args: withString(newArgs(), "session", "session-1"),
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected success, got %v: %s", result.Status, result.Error)
	}

	vars, _ := result.Data["variables"].([]map[string]string)
	if len(vars) != 2 {
		t.Errorf("expected 2 variables, got %d", len(vars))
	}
}

func TestDebugHandler_NoManager(t *testing.T) {
	h := NewDebugHandler()
	ctx := execctx.New()

	action := input.Action{Name: ActionDebugSessions}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected error, got %v", result.Status)
	}
}
