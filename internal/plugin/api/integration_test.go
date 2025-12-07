package api

import (
	"errors"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// mockIntegrationProvider implements IntegrationProvider for testing.
type mockIntegrationProvider struct {
	workspaceRoot string
	health        IntegrationHealth
	git           GitProvider
	debug         DebugProvider
	task          TaskProvider
}

func (m *mockIntegrationProvider) WorkspaceRoot() string     { return m.workspaceRoot }
func (m *mockIntegrationProvider) Health() IntegrationHealth { return m.health }
func (m *mockIntegrationProvider) Git() GitProvider          { return m.git }
func (m *mockIntegrationProvider) Debug() DebugProvider      { return m.debug }
func (m *mockIntegrationProvider) Task() TaskProvider        { return m.task }

// mockGitProvider implements GitProvider for testing.
type mockGitProvider struct {
	status    GitStatus
	branch    string
	branches  []string
	diff      string
	addErr    error
	commitErr error
}

func (m *mockGitProvider) Status() (GitStatus, error)       { return m.status, nil }
func (m *mockGitProvider) Branch() (string, error)          { return m.branch, nil }
func (m *mockGitProvider) Branches() ([]string, error)      { return m.branches, nil }
func (m *mockGitProvider) Commit(message string) error      { return m.commitErr }
func (m *mockGitProvider) Add(paths []string) error         { return m.addErr }
func (m *mockGitProvider) Diff(staged bool) (string, error) { return m.diff, nil }

// mockTaskProvider implements TaskProvider for testing.
type mockTaskProvider struct {
	tasks  []TaskInfo
	status TaskStatus
	output string
	runID  string
	runErr error
}

func (m *mockTaskProvider) List() ([]TaskInfo, error)                { return m.tasks, nil }
func (m *mockTaskProvider) Run(name string) (string, error)          { return m.runID, m.runErr }
func (m *mockTaskProvider) Stop(taskID string) error                 { return nil }
func (m *mockTaskProvider) Status(taskID string) (TaskStatus, error) { return m.status, nil }
func (m *mockTaskProvider) Output(taskID string) (string, error)     { return m.output, nil }

// mockDebugProvider implements DebugProvider for testing.
type mockDebugProvider struct {
	sessions     []DebugSession
	variables    []DebugVariable
	startID      string
	breakpointID string
}

func (m *mockDebugProvider) Start(config DebugConfig) (string, error) { return m.startID, nil }
func (m *mockDebugProvider) Stop(sessionID string) error              { return nil }
func (m *mockDebugProvider) Sessions() []DebugSession                 { return m.sessions }
func (m *mockDebugProvider) SetBreakpoint(file string, line int) (string, error) {
	return m.breakpointID, nil
}
func (m *mockDebugProvider) RemoveBreakpoint(id string) error { return nil }
func (m *mockDebugProvider) Continue(sessionID string) error  { return nil }
func (m *mockDebugProvider) StepOver(sessionID string) error  { return nil }
func (m *mockDebugProvider) StepInto(sessionID string) error  { return nil }
func (m *mockDebugProvider) StepOut(sessionID string) error   { return nil }
func (m *mockDebugProvider) Variables(sessionID string) ([]DebugVariable, error) {
	return m.variables, nil
}

func TestIntegrationModule_Name(t *testing.T) {
	mod := NewIntegrationModule(nil)
	if mod.Name() != "integration" {
		t.Errorf("Name() = %q, want 'integration'", mod.Name())
	}
}

func TestIntegrationModule_WorkspaceRoot(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	provider := &mockIntegrationProvider{
		workspaceRoot: "/test/workspace",
	}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`result = _ks_integration.workspace_root()`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if s, ok := result.(lua.LString); !ok || string(s) != "/test/workspace" {
		t.Errorf("workspace_root() = %v, want '/test/workspace'", result)
	}
}

func TestIntegrationModule_Health(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	provider := &mockIntegrationProvider{
		workspaceRoot: "/test",
		health: IntegrationHealth{
			Status:        "healthy",
			Uptime:        60000,
			ProcessCount:  3,
			WorkspaceRoot: "/test",
			Components:    map[string]string{"git": "ok", "task": "ok"},
		},
	}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		h = _ks_integration.health()
		status = h.status
		uptime = h.uptime
		process_count = h.process_count
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("status"); s.String() != "healthy" {
		t.Errorf("health().status = %v, want 'healthy'", s)
	}
	if n := L.GetGlobal("uptime"); n.(lua.LNumber) != 60000 {
		t.Errorf("health().uptime = %v, want 60000", n)
	}
	if n := L.GetGlobal("process_count"); n.(lua.LNumber) != 3 {
		t.Errorf("health().process_count = %v, want 3", n)
	}
}

func TestIntegrationModule_GitStatus(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	gitProvider := &mockGitProvider{
		status: GitStatus{
			Branch:       "main",
			Ahead:        2,
			Behind:       1,
			Staged:       []string{"file1.go"},
			Modified:     []string{"file2.go"},
			Untracked:    []string{"file3.go"},
			HasConflicts: false,
			IsClean:      false,
		},
	}
	provider := &mockIntegrationProvider{git: gitProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		s = _ks_integration.git.status()
		branch = s.branch
		ahead = s.ahead
		behind = s.behind
		is_clean = s.is_clean
		staged_count = #s.staged
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("branch"); s.String() != "main" {
		t.Errorf("git.status().branch = %v, want 'main'", s)
	}
	if n := L.GetGlobal("ahead"); n.(lua.LNumber) != 2 {
		t.Errorf("git.status().ahead = %v, want 2", n)
	}
	if n := L.GetGlobal("staged_count"); n.(lua.LNumber) != 1 {
		t.Errorf("git.status() staged count = %v, want 1", n)
	}
}

func TestIntegrationModule_GitBranch(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	gitProvider := &mockGitProvider{branch: "feature/test"}
	provider := &mockIntegrationProvider{git: gitProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`result = _ks_integration.git.branch()`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("result"); s.String() != "feature/test" {
		t.Errorf("git.branch() = %v, want 'feature/test'", s)
	}
}

func TestIntegrationModule_GitBranches(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	gitProvider := &mockGitProvider{branches: []string{"main", "develop", "feature/test"}}
	provider := &mockIntegrationProvider{git: gitProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		branches = _ks_integration.git.branches()
		count = #branches
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if n := L.GetGlobal("count"); n.(lua.LNumber) != 3 {
		t.Errorf("git.branches() count = %v, want 3", n)
	}
}

func TestIntegrationModule_GitDiff(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	gitProvider := &mockGitProvider{diff: "diff --git a/file.go b/file.go\n..."}
	provider := &mockIntegrationProvider{git: gitProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`result = _ks_integration.git.diff(true)`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if s, ok := result.(lua.LString); !ok || string(s) != "diff --git a/file.go b/file.go\n..." {
		t.Errorf("git.diff() = %v, want diff content", result)
	}
}

func TestIntegrationModule_TaskList(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	taskProvider := &mockTaskProvider{
		tasks: []TaskInfo{
			{Name: "build", Source: "Makefile", Description: "Build the project", Command: "make build"},
			{Name: "test", Source: "Makefile", Description: "Run tests", Command: "make test"},
		},
	}
	provider := &mockIntegrationProvider{task: taskProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		tasks = _ks_integration.task.list()
		count = #tasks
		first_name = tasks[1].name
		first_source = tasks[1].source
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if n := L.GetGlobal("count"); n.(lua.LNumber) != 2 {
		t.Errorf("task.list() count = %v, want 2", n)
	}
	if s := L.GetGlobal("first_name"); s.String() != "build" {
		t.Errorf("task.list()[1].name = %v, want 'build'", s)
	}
	if s := L.GetGlobal("first_source"); s.String() != "Makefile" {
		t.Errorf("task.list()[1].source = %v, want 'Makefile'", s)
	}
}

func TestIntegrationModule_TaskRun(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	taskProvider := &mockTaskProvider{runID: "task-123"}
	provider := &mockIntegrationProvider{task: taskProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`result = _ks_integration.task.run("build")`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("result"); s.String() != "task-123" {
		t.Errorf("task.run() = %v, want 'task-123'", s)
	}
}

func TestIntegrationModule_TaskStatus(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	taskProvider := &mockTaskProvider{
		status: TaskStatus{
			ID:        "task-123",
			Name:      "build",
			State:     "completed",
			ExitCode:  0,
			StartTime: 1700000000000,
			EndTime:   1700000060000,
		},
	}
	provider := &mockIntegrationProvider{task: taskProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		s = _ks_integration.task.status("task-123")
		state = s.state
		exit_code = s.exit_code
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("state"); s.String() != "completed" {
		t.Errorf("task.status().state = %v, want 'completed'", s)
	}
	if n := L.GetGlobal("exit_code"); n.(lua.LNumber) != 0 {
		t.Errorf("task.status().exit_code = %v, want 0", n)
	}
}

func TestIntegrationModule_TaskOutput(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	taskProvider := &mockTaskProvider{output: "Building...\nDone!"}
	provider := &mockIntegrationProvider{task: taskProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`result = _ks_integration.task.output("task-123")`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("result"); s.String() != "Building...\nDone!" {
		t.Errorf("task.output() = %v, want 'Building...\\nDone!'", s)
	}
}

func TestIntegrationModule_DebugSessions(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	debugProvider := &mockDebugProvider{
		sessions: []DebugSession{
			{ID: "session-1", Adapter: "delve", Program: "./main", State: "running"},
		},
	}
	provider := &mockIntegrationProvider{debug: debugProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		sessions = _ks_integration.debug.sessions()
		count = #sessions
		first_id = sessions[1].id
		first_state = sessions[1].state
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if n := L.GetGlobal("count"); n.(lua.LNumber) != 1 {
		t.Errorf("debug.sessions() count = %v, want 1", n)
	}
	if s := L.GetGlobal("first_id"); s.String() != "session-1" {
		t.Errorf("debug.sessions()[1].id = %v, want 'session-1'", s)
	}
	if s := L.GetGlobal("first_state"); s.String() != "running" {
		t.Errorf("debug.sessions()[1].state = %v, want 'running'", s)
	}
}

func TestIntegrationModule_DebugStart(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	debugProvider := &mockDebugProvider{startID: "session-new"}
	provider := &mockIntegrationProvider{debug: debugProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		result = _ks_integration.debug.start({
			adapter = "delve",
			program = "./main",
			stop_on_entry = true
		})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("result"); s.String() != "session-new" {
		t.Errorf("debug.start() = %v, want 'session-new'", s)
	}
}

func TestIntegrationModule_DebugSetBreakpoint(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	debugProvider := &mockDebugProvider{breakpointID: "bp-123"}
	provider := &mockIntegrationProvider{debug: debugProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`result = _ks_integration.debug.set_breakpoint("main.go", 42)`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("result"); s.String() != "bp-123" {
		t.Errorf("debug.set_breakpoint() = %v, want 'bp-123'", s)
	}
}

func TestIntegrationModule_DebugVariables(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	debugProvider := &mockDebugProvider{
		variables: []DebugVariable{
			{Name: "x", Value: "42", Type: "int"},
			{Name: "name", Value: "test", Type: "string"},
		},
	}
	provider := &mockIntegrationProvider{debug: debugProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`
		vars = _ks_integration.debug.variables("session-1")
		count = #vars
		first_name = vars[1].name
		first_value = vars[1].value
		first_type = vars[1].type
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if n := L.GetGlobal("count"); n.(lua.LNumber) != 2 {
		t.Errorf("debug.variables() count = %v, want 2", n)
	}
	if s := L.GetGlobal("first_name"); s.String() != "x" {
		t.Errorf("debug.variables()[1].name = %v, want 'x'", s)
	}
	if s := L.GetGlobal("first_value"); s.String() != "42" {
		t.Errorf("debug.variables()[1].value = %v, want '42'", s)
	}
}

func TestIntegrationModule_NoProvider(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	ctx := &Context{Integration: nil}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// workspace_root should return empty string without error
	err := L.DoString(`result = _ks_integration.workspace_root()`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if s := L.GetGlobal("result"); s.String() != "" {
		t.Errorf("workspace_root() without provider = %v, want ''", s)
	}
}

func TestIntegrationModule_GitCommitError(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	gitProvider := &mockGitProvider{commitErr: errors.New("nothing to commit")}
	provider := &mockIntegrationProvider{git: gitProvider}
	ctx := &Context{Integration: provider}
	mod := NewIntegrationModule(ctx)

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := L.DoString(`_ks_integration.git.commit("test message")`)
	if err == nil {
		t.Error("expected error from git.commit with commitErr")
	}
}
