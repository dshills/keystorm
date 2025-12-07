package task

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestDefaultExecutorConfig(t *testing.T) {
	config := DefaultExecutorConfig()

	if config.DefaultShell == "" {
		t.Error("DefaultShell is empty")
	}

	if len(config.DefaultShellArgs) == 0 {
		t.Error("DefaultShellArgs is empty")
	}

	if config.OutputBufferSize != 64*1024 {
		t.Errorf("OutputBufferSize = %d, want %d", config.OutputBufferSize, 64*1024)
	}

	if config.MaxConcurrent != 4 {
		t.Errorf("MaxConcurrent = %d, want 4", config.MaxConcurrent)
	}
}

func TestNewExecutor(t *testing.T) {
	config := DefaultExecutorConfig()
	e := NewExecutor(config)

	if e == nil {
		t.Fatal("NewExecutor returned nil")
	}

	if e.executions == nil {
		t.Error("executions map is nil")
	}

	if e.sem == nil {
		t.Error("semaphore channel is nil")
	}

	if cap(e.sem) != config.MaxConcurrent {
		t.Errorf("semaphore capacity = %d, want %d", cap(e.sem), config.MaxConcurrent)
	}

	if e.variables == nil {
		t.Error("variables resolver is nil")
	}

	if e.problems == nil {
		t.Error("problems matcher is nil")
	}
}

func TestNewExecutor_ZeroMaxConcurrent(t *testing.T) {
	config := ExecutorConfig{
		MaxConcurrent: 0,
	}
	e := NewExecutor(config)

	// Should default to 4
	if cap(e.sem) != 4 {
		t.Errorf("semaphore capacity = %d, want 4 (default)", cap(e.sem))
	}
}

func TestExecutionState_Values(t *testing.T) {
	states := []ExecutionState{
		ExecutionStatePending,
		ExecutionStateRunning,
		ExecutionStateSucceeded,
		ExecutionStateFailed,
		ExecutionStateCanceled,
	}

	expected := []string{
		"pending",
		"running",
		"succeeded",
		"failed",
		"canceled",
	}

	for i, state := range states {
		if string(state) != expected[i] {
			t.Errorf("state = %q, want %q", state, expected[i])
		}
	}
}

// MockExecutionListener records execution events
type MockExecutionListener struct {
	started   []*Execution
	outputs   []OutputLine
	problems  []Problem
	completed []*Execution
	mu        sync.Mutex
}

func (m *MockExecutionListener) OnExecutionStarted(exec *Execution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = append(m.started, exec)
}

func (m *MockExecutionListener) OnExecutionOutput(exec *Execution, line OutputLine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputs = append(m.outputs, line)
}

func (m *MockExecutionListener) OnExecutionProblem(exec *Execution, problem Problem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.problems = append(m.problems, problem)
}

func (m *MockExecutionListener) OnExecutionCompleted(exec *Execution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = append(m.completed, exec)
}

func TestExecutor_AddRemoveListener(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())
	listener := &MockExecutionListener{}

	e.AddListener(listener)

	if len(e.listeners) != 1 {
		t.Errorf("listener count = %d, want 1", len(e.listeners))
	}

	e.RemoveListener(listener)

	if len(e.listeners) != 0 {
		t.Errorf("listener count after remove = %d, want 0", len(e.listeners))
	}
}

func TestExecutor_Execute(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())
	listener := &MockExecutionListener{}
	e.AddListener(listener)

	task := &Task{
		Name:    "echo-test",
		Type:    TaskTypeShell,
		Command: "echo",
		Args:    []string{"hello"},
	}

	ctx := context.Background()
	exec, err := e.Execute(ctx, task)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec == nil {
		t.Fatal("Execute returned nil execution")
	}

	if exec.ID == "" {
		t.Error("execution ID is empty")
	}

	if exec.Task != task {
		t.Error("execution task mismatch")
	}

	// Wait for completion
	select {
	case <-exec.Done():
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("execution timed out")
	}

	if exec.State != ExecutionStateSucceeded {
		t.Errorf("State = %q, want succeeded", exec.State)
	}

	if exec.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", exec.ExitCode)
	}

	// Check listener was notified
	listener.mu.Lock()
	defer listener.mu.Unlock()
	if len(listener.started) != 1 {
		t.Errorf("started events = %d, want 1", len(listener.started))
	}
	if len(listener.completed) != 1 {
		t.Errorf("completed events = %d, want 1", len(listener.completed))
	}
}

func TestExecutor_ExecuteSync(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "true-test",
		Type:    TaskTypeProcess,
		Command: "true",
	}

	ctx := context.Background()
	exec, err := e.ExecuteSync(ctx, task)
	if err != nil {
		t.Fatalf("ExecuteSync failed: %v", err)
	}

	if exec.State != ExecutionStateSucceeded {
		t.Errorf("State = %q, want succeeded", exec.State)
	}
}

func TestExecutor_ExecuteWithEnv(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "env-test",
		Type:    TaskTypeShell,
		Command: "echo",
		Args:    []string{"$TEST_VAR"},
	}

	ctx := context.Background()
	exec, err := e.ExecuteWithEnv(ctx, task, map[string]string{
		"TEST_VAR": "test_value",
	})
	if err != nil {
		t.Fatalf("ExecuteWithEnv failed: %v", err)
	}

	<-exec.Done()

	if exec.State != ExecutionStateSucceeded {
		t.Errorf("State = %q, want succeeded", exec.State)
	}
}

func TestExecutor_ExecuteFailing(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "fail-test",
		Type:    TaskTypeProcess,
		Command: "false",
	}

	ctx := context.Background()
	exec, err := e.ExecuteSync(ctx, task)
	if err != nil {
		t.Fatalf("ExecuteSync failed: %v", err)
	}

	if exec.State != ExecutionStateFailed {
		t.Errorf("State = %q, want failed", exec.State)
	}

	if exec.ExitCode == 0 {
		t.Error("ExitCode should not be 0 for failing command")
	}
}

func TestExecutor_GetExecution(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "test",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"0.1"},
	}

	ctx := context.Background()
	exec, _ := e.Execute(ctx, task)

	got, ok := e.GetExecution(exec.ID)
	if !ok {
		t.Error("GetExecution returned false")
	}
	if got != exec {
		t.Error("GetExecution returned different execution")
	}

	_, ok = e.GetExecution("nonexistent")
	if ok {
		t.Error("GetExecution returned true for nonexistent ID")
	}

	<-exec.Done()
}

func TestExecutor_ListExecutions(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "test",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"0.1"},
	}

	ctx := context.Background()
	exec1, _ := e.Execute(ctx, task)
	exec2, _ := e.Execute(ctx, task)

	executions := e.ListExecutions()
	if len(executions) != 2 {
		t.Errorf("ListExecutions returned %d, want 2", len(executions))
	}

	<-exec1.Done()
	<-exec2.Done()
}

func TestExecutor_CancelExecution(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "long-running",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"10"},
	}

	ctx := context.Background()
	exec, _ := e.Execute(ctx, task)

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	err := e.CancelExecution(exec.ID)
	if err != nil {
		t.Fatalf("CancelExecution failed: %v", err)
	}

	<-exec.Done()

	if exec.State != ExecutionStateCanceled && exec.State != ExecutionStateFailed {
		t.Errorf("State = %q, want canceled or failed", exec.State)
	}
}

func TestExecutor_CancelExecutionNotFound(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	err := e.CancelExecution("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent execution")
	}
}

func TestExecutor_CancelAll(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "long-running",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"10"},
	}

	ctx := context.Background()
	exec1, _ := e.Execute(ctx, task)
	exec2, _ := e.Execute(ctx, task)

	// Give them time to start
	time.Sleep(100 * time.Millisecond)

	e.CancelAll()

	<-exec1.Done()
	<-exec2.Done()

	for _, exec := range []*Execution{exec1, exec2} {
		if exec.State != ExecutionStateCanceled && exec.State != ExecutionStateFailed {
			t.Errorf("State = %q, want canceled or failed", exec.State)
		}
	}
}

func TestExecutor_CleanupCompleted(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "quick",
		Type:    TaskTypeProcess,
		Command: "true",
	}

	ctx := context.Background()
	exec, _ := e.ExecuteSync(ctx, task)

	if exec.State != ExecutionStateSucceeded {
		t.Fatalf("expected succeeded state")
	}

	count := e.CleanupCompleted()
	if count != 1 {
		t.Errorf("CleanupCompleted returned %d, want 1", count)
	}

	_, ok := e.GetExecution(exec.ID)
	if ok {
		t.Error("execution still exists after cleanup")
	}
}

func TestExecutor_WorkingDirectory(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	tmpDir := t.TempDir()

	task := &Task{
		Name:    "pwd-test",
		Type:    TaskTypeShell,
		Command: "pwd",
		Cwd:     tmpDir,
	}

	ctx := context.Background()
	exec, _ := e.ExecuteSync(ctx, task)

	if exec.State != ExecutionStateSucceeded {
		t.Fatalf("State = %q, want succeeded", exec.State)
	}

	output := exec.StdoutLines()
	if len(output) == 0 {
		t.Fatal("no output captured")
	}

	// Output should contain the temp directory
	// On macOS, /var is symlinked to /private/var, so resolve both
	got := output[0].Content
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(tmpDir)
	if gotResolved != wantResolved {
		t.Errorf("pwd output = %q (resolved: %q), want %q (resolved: %q)", got, gotResolved, tmpDir, wantResolved)
	}
}

func TestExecution_Duration(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "sleep-test",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"0.1"},
	}

	ctx := context.Background()
	exec, _ := e.ExecuteSync(ctx, task)

	dur := exec.Duration()
	if dur < 100*time.Millisecond {
		t.Errorf("Duration = %v, want >= 100ms", dur)
	}
}

func TestExecution_IsRunning(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "sleep-test",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"0.2"},
	}

	ctx := context.Background()
	exec, _ := e.Execute(ctx, task)

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	if !exec.IsRunning() {
		t.Error("IsRunning() = false, want true")
	}

	<-exec.Done()

	if exec.IsRunning() {
		t.Error("IsRunning() = true after completion, want false")
	}
}

func TestExecution_Output(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "output-test",
		Type:    TaskTypeShell,
		Command: "echo",
		Args:    []string{"line1; echo line2"},
	}

	ctx := context.Background()
	exec, _ := e.ExecuteSync(ctx, task)

	output := exec.Output()
	if len(output) == 0 {
		t.Error("no output captured")
	}
}

func TestExecution_Cancel(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "cancel-test",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"10"},
	}

	ctx := context.Background()
	exec, _ := e.Execute(ctx, task)

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	exec.Cancel()

	<-exec.Done()

	if exec.State != ExecutionStateCanceled && exec.State != ExecutionStateFailed {
		t.Errorf("State = %q, want canceled or failed", exec.State)
	}
}

func TestExecutor_ConcurrencyLimit(t *testing.T) {
	config := DefaultExecutorConfig()
	config.MaxConcurrent = 2
	e := NewExecutor(config)

	task := &Task{
		Name:    "concurrent-test",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"0.2"},
	}

	ctx := context.Background()
	start := time.Now()

	// Launch 4 tasks with concurrency limit of 2
	var executions []*Execution
	for i := 0; i < 4; i++ {
		exec, _ := e.Execute(ctx, task)
		executions = append(executions, exec)
	}

	// Wait for all
	for _, exec := range executions {
		<-exec.Done()
	}

	elapsed := time.Since(start)

	// With concurrency 2 and 0.2s tasks, 4 tasks should take ~0.4s
	// Allow some margin for scheduling
	if elapsed < 350*time.Millisecond {
		t.Errorf("elapsed = %v, expected >= 350ms (concurrency should limit)", elapsed)
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	task := &Task{
		Name:    "ctx-cancel-test",
		Type:    TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"10"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	exec, _ := e.Execute(ctx, task)

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	cancel()

	<-exec.Done()

	if exec.State != ExecutionStateCanceled && exec.State != ExecutionStateFailed {
		t.Errorf("State = %q, want canceled or failed", exec.State)
	}
}

func TestExecutor_MakeTask(t *testing.T) {
	e := NewExecutor(DefaultExecutorConfig())

	// Create a temp Makefile
	tmpDir := t.TempDir()
	makefile := filepath.Join(tmpDir, "Makefile")

	content := `.PHONY: test
test:
	@echo "test passed"
`
	if err := os.WriteFile(makefile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}

	task := &Task{
		Name:    "make-test",
		Type:    TaskTypeMake,
		Command: "make",
		Args:    []string{"test"},
		Cwd:     tmpDir,
	}

	ctx := context.Background()
	exec, _ := e.ExecuteSync(ctx, task)

	if exec.State != ExecutionStateSucceeded {
		t.Errorf("State = %q, want succeeded; error: %v", exec.State, exec.Error)
	}
}

func TestExecutor_VariableSubstitution(t *testing.T) {
	config := DefaultExecutorConfig()
	config.WorkingDir = t.TempDir()
	e := NewExecutor(config)

	e.variables.Set("CUSTOM_VAR", "custom_value")

	task := &Task{
		Name:    "var-test",
		Type:    TaskTypeShell,
		Command: "echo",
		Args:    []string{"${CUSTOM_VAR}"},
	}

	ctx := context.Background()
	exec, _ := e.ExecuteSync(ctx, task)

	if exec.State != ExecutionStateSucceeded {
		t.Fatalf("State = %q, want succeeded", exec.State)
	}

	output := exec.StdoutLines()
	if len(output) == 0 {
		t.Fatal("no output")
	}

	if output[0].Content != "custom_value" {
		t.Errorf("output = %q, want %q", output[0].Content, "custom_value")
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "''"},
		{"simple", "hello", "hello"},
		{"with spaces", "hello world", "'hello world'"},
		{"single quote", "it's", "'it'\\''s'"},
		{"double quotes", `say "hello"`, `'say "hello"'`},
		{"backticks", "echo `whoami`", "'echo `whoami`'"},
		{"dollar sign", "$HOME", "'$HOME'"},
		{"semicolon", "a;b", "'a;b'"},
		{"pipe", "a|b", "'a|b'"},
		{"ampersand", "a&b", "'a&b'"},
		{"newline", "a\nb", "'a\nb'"},
		{"tab", "a\tb", "'a\tb'"},
		{"backslash", "a\\b", "'a\\b'"},
		{"multiple single quotes", "it's John's", "'it'\\''s John'\\''s'"},
		{"path with slash", "/path/to/file", "/path/to/file"},
		{"path with colon", "/usr/bin:.", "'/usr/bin:.'"}, // colon requires escaping
		{"complex", "echo 'hello' && rm -rf /", "'echo '\\''hello'\\'' && rm -rf /'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellEscape(tt.input)
			if got != tt.want {
				t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShellEscape_SafeCharacters(t *testing.T) {
	// These should not need escaping
	safeInputs := []string{
		"hello",
		"hello123",
		"hello_world",
		"hello-world",
		"hello.txt",
		"/path/to/file",
		"./relative/path",
		"file.txt",
		"CamelCase",
		"UPPERCASE",
		"a",
		"A",
		"1",
	}

	for _, input := range safeInputs {
		got := shellEscape(input)
		if got != input {
			t.Errorf("shellEscape(%q) = %q, expected no change", input, got)
		}
	}
}
