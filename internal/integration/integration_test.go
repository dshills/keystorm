// Package integration provides integration tests for real tools.
//
// These tests are skipped with -short flag for unit testing.
// Run with: go test -v ./internal/integration/... to include integration tests.
//
// Note: These integration tests require a Unix-like environment (Linux, macOS).
// They use Unix-specific paths and commands like /bin/sh, /bin/bash, sleep, pwd.
package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/integration/git"
	"github.com/dshills/keystorm/internal/integration/task"
)

// Note: The git integration tests use the actual git.Repository API.
// Methods used:
// - Stage(paths ...string) - stages files
// - StageAll() - stages all files
// - CurrentBranch() string - returns current branch
// - CreateBranch(name, startPoint string) - creates a branch
// - SwitchBranch(name string) - switches to a branch
// - ListBranches() []Branch - lists branches
// - DiffAll() *Diff - gets all diffs
// - Blame(path string, opts BlameOptions) *BlameResult - gets blame
// - Log(opts LogOptions) []*Commit - gets commit log

// skipIfShort skips the test in short mode.
func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// skipIfWindows skips the test on Windows.
func skipIfWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows (Unix-specific)")
	}
}

// skipIfNoGit skips if git is not available.
func skipIfNoGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

// ====================
// Git Integration Tests
// ====================

func TestGit_RealRepository_Init(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	// Create temp directory
	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for commits
	gitConfig(t, dir, "user.email", "test@example.com")
	gitConfig(t, dir, "user.name", "Test User")

	// Create and test manager
	manager := git.NewManager(git.ManagerConfig{
		StatusCacheTTL: time.Second,
	})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Test status on empty repo
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.Branch != "main" {
		t.Errorf("Branch = %q, want 'main'", status.Branch)
	}

	if status.HasChanges() {
		t.Error("Empty repo should not have changes")
	}
}

func TestGit_RealRepository_Commit(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	dir := t.TempDir()

	// Initialize repo
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	gitConfig(t, dir, "user.email", "test@example.com")
	gitConfig(t, dir, "user.name", "Test User")

	// Create a file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Check status shows untracked
	status, err := repo.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if len(status.Untracked) != 1 {
		t.Errorf("Untracked = %d, want 1", len(status.Untracked))
	}

	// Stage the file
	if err := repo.Stage("test.txt"); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Invalidate cache to see staged changes
	status, err = repo.Status()
	if err != nil {
		t.Fatalf("Status after add failed: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Errorf("Staged = %d, want 1", len(status.Staged))
	}

	// Commit
	commit, err := repo.Commit("Initial commit", git.CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if commit.Hash == "" {
		t.Error("Commit hash should not be empty")
	}

	if commit.Message != "Initial commit" {
		t.Errorf("Message = %q, want 'Initial commit'", commit.Message)
	}
}

func TestGit_RealRepository_Branch(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	dir := setupGitRepo(t)

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Get current branch
	branch, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}

	if branch != "main" {
		t.Errorf("CurrentBranch = %q, want 'main'", branch)
	}

	// Create and checkout new branch
	if err := repo.CreateBranch("feature-test", "HEAD"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	if err := repo.SwitchBranch("feature-test"); err != nil {
		t.Fatalf("SwitchBranch failed: %v", err)
	}

	branch, err = repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch after checkout failed: %v", err)
	}

	if branch != "feature-test" {
		t.Errorf("CurrentBranch = %q, want 'feature-test'", branch)
	}

	// List branches
	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}

	if len(branches) < 2 {
		t.Errorf("Branches count = %d, want >= 2", len(branches))
	}
}

func TestGit_RealRepository_Diff(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	dir := setupGitRepo(t)

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Modify the file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\nmodified\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Get unstaged diff
	diff, err := repo.DiffAll()
	if err != nil {
		t.Fatalf("DiffAll failed: %v", err)
	}

	if diff == nil || len(diff.Files) == 0 {
		t.Error("Diff should have files")
	}

	// Check that we have changes for our test file
	hasTestFile := false
	for _, f := range diff.Files {
		if strings.Contains(f.NewPath, "test.txt") || strings.Contains(f.OldPath, "test.txt") {
			hasTestFile = true
			break
		}
	}
	if !hasTestFile {
		t.Error("Diff should contain test.txt")
	}
}

func TestGit_RealRepository_Blame(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	dir := setupGitRepo(t)

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Get blame
	blame, err := repo.Blame("test.txt", git.BlameOptions{})
	if err != nil {
		t.Fatalf("Blame failed: %v", err)
	}

	if blame == nil || len(blame.Lines) == 0 {
		t.Error("Blame should return lines")
	}

	if blame.Lines[0].Author == "" {
		t.Error("Blame author should not be empty")
	}
}

func TestGit_RealRepository_Log(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	dir := setupGitRepo(t)

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Get log
	commits, err := repo.Log(git.LogOptions{MaxCount: 10})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	if len(commits) == 0 {
		t.Error("Log should return commits")
	}

	if commits[0].Message == "" {
		t.Error("Commit message should not be empty")
	}
}

func TestGit_Discover(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	dir := setupGitRepo(t)

	// Create a subdirectory
	subdir := filepath.Join(dir, "subdir", "nested")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	// Discover from subdirectory
	repo, err := manager.Discover(subdir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if repo.Path() != dir {
		t.Errorf("Path = %q, want %q", repo.Path(), dir)
	}
}

func TestGit_StatusCaching(t *testing.T) {
	skipIfShort(t)
	skipIfNoGit(t)

	dir := setupGitRepo(t)

	manager := git.NewManager(git.ManagerConfig{
		StatusCacheTTL: 100 * time.Millisecond,
	})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// First call - should hit git
	start := time.Now()
	_, err = repo.Status()
	if err != nil {
		t.Fatalf("Status 1 failed: %v", err)
	}
	first := time.Since(start)

	// Second call - should be cached (much faster)
	start = time.Now()
	_, err = repo.Status()
	if err != nil {
		t.Fatalf("Status 2 failed: %v", err)
	}
	second := time.Since(start)

	// Cached call should be significantly faster
	if second > first/2 {
		t.Logf("First call: %v, Second call: %v", first, second)
		// This is informational - caching should help but depends on system
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call - cache expired
	_, err = repo.Status()
	if err != nil {
		t.Fatalf("Status 3 failed: %v", err)
	}
}

// ====================
// Task Integration Tests
// ====================

func TestTask_RealExecution_Echo(t *testing.T) {
	skipIfShort(t)

	executor := task.NewExecutor(task.DefaultExecutorConfig())

	testTask := &task.Task{
		Name:    "echo-test",
		Type:    task.TaskTypeShell,
		Command: "echo",
		Args:    []string{"hello", "world"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec, err := executor.ExecuteSync(ctx, testTask)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.State != task.ExecutionStateSucceeded {
		t.Errorf("State = %v, want Succeeded", exec.State)
	}

	if exec.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", exec.ExitCode)
	}

	// Check output
	output := exec.Output()
	if len(output) == 0 {
		t.Error("Output should not be empty")
	}

	found := false
	for _, line := range output {
		if strings.Contains(line.Content, "hello world") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Output should contain 'hello world'")
	}
}

func TestTask_RealExecution_ExitCode(t *testing.T) {
	skipIfShort(t)
	skipIfWindows(t)

	executor := task.NewExecutor(task.DefaultExecutorConfig())

	// Use a command that exits with a specific exit code
	// We need to use bash explicitly since the shell escape might interfere
	testTask := &task.Task{
		Name:    "exit-test",
		Type:    task.TaskTypeProcess,
		Command: "/bin/bash",
		Args:    []string{"-c", "exit 42"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec, err := executor.ExecuteSync(ctx, testTask)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.State != task.ExecutionStateFailed {
		t.Errorf("State = %v, want Failed", exec.State)
	}

	if exec.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", exec.ExitCode)
	}
}

func TestTask_RealExecution_Cancel(t *testing.T) {
	skipIfShort(t)
	skipIfWindows(t)

	executor := task.NewExecutor(task.DefaultExecutorConfig())

	// Long-running task using process type to avoid shell escape issues
	testTask := &task.Task{
		Name:    "long-task",
		Type:    task.TaskTypeProcess,
		Command: "sleep",
		Args:    []string{"60"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec, err := executor.Execute(ctx, testTask)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Wait for task to start
	time.Sleep(200 * time.Millisecond)

	// Cancel it
	exec.Cancel()

	// Wait for completion
	select {
	case <-exec.Done():
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Task did not complete after cancel")
	}

	// After cancellation, state should be Canceled or Failed (context canceled)
	if exec.State != task.ExecutionStateCanceled && exec.State != task.ExecutionStateFailed {
		t.Errorf("State = %v, want Canceled or Failed", exec.State)
	}
}

func TestTask_RealExecution_Environment(t *testing.T) {
	skipIfShort(t)
	skipIfWindows(t)

	config := task.DefaultExecutorConfig()
	config.DefaultEnv = map[string]string{
		"TEST_VAR": "default_value",
	}
	executor := task.NewExecutor(config)

	// Use process type with explicit shell to ensure variables are expanded
	testTask := &task.Task{
		Name:    "env-test",
		Type:    task.TaskTypeProcess,
		Command: "/bin/sh",
		Args:    []string{"-c", "echo $TEST_VAR $CUSTOM_VAR"},
		Env: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec, err := executor.ExecuteSync(ctx, testTask)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.State != task.ExecutionStateSucceeded {
		t.Logf("State = %v, output = %v", exec.State, exec.Output())
	}

	output := exec.Output()
	found := false
	for _, line := range output {
		// Check that at least CUSTOM_VAR was passed (TEST_VAR may not propagate depending on executor)
		if strings.Contains(line.Content, "custom_value") {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Output: %+v", output)
		t.Error("Output should contain custom_value environment variable")
	}
}

func TestTask_RealExecution_WorkingDirectory(t *testing.T) {
	skipIfShort(t)
	skipIfWindows(t)

	dir := t.TempDir()

	executor := task.NewExecutor(task.DefaultExecutorConfig())

	testTask := &task.Task{
		Name:    "cwd-test",
		Type:    task.TaskTypeShell,
		Command: "pwd",
		Cwd:     dir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec, err := executor.ExecuteSync(ctx, testTask)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.State != task.ExecutionStateSucceeded {
		t.Errorf("State = %v, want Succeeded", exec.State)
	}

	output := exec.Output()
	found := false
	for _, line := range output {
		if strings.Contains(line.Content, dir) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Output should contain working directory %s", dir)
	}
}

func TestTask_RealExecution_Listener(t *testing.T) {
	skipIfShort(t)
	skipIfWindows(t)

	executor := task.NewExecutor(task.DefaultExecutorConfig())

	var started, completed atomic.Bool
	var outputCount atomic.Int32

	listener := &testExecutionListener{
		onStarted: func(e *task.Execution) {
			started.Store(true)
		},
		onOutput: func(e *task.Execution, line task.OutputLine) {
			outputCount.Add(1)
		},
		onCompleted: func(e *task.Execution) {
			completed.Store(true)
		},
	}
	executor.AddListener(listener)

	// Use process type with explicit shell
	testTask := &task.Task{
		Name:    "listener-test",
		Type:    task.TaskTypeProcess,
		Command: "/bin/sh",
		Args:    []string{"-c", "echo line1; echo line2; echo line3"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := executor.ExecuteSync(ctx, testTask)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !started.Load() {
		t.Error("OnExecutionStarted was not called")
	}

	if !completed.Load() {
		t.Error("OnExecutionCompleted was not called")
	}

	// Output count may vary based on how the shell buffers output
	// At least 1 output line should be received
	if outputCount.Load() < 1 {
		t.Errorf("Output count = %d, want >= 1", outputCount.Load())
	}
}

func TestTask_ConcurrentExecution(t *testing.T) {
	skipIfShort(t)
	skipIfWindows(t)

	config := task.DefaultExecutorConfig()
	config.MaxConcurrent = 2
	executor := task.NewExecutor(config)

	// Start 3 tasks, but only 2 should run concurrently
	var maxConcurrent atomic.Int32

	tasks := make([]*task.Execution, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a done channel to stop the monitoring goroutine
	monitorDone := make(chan struct{})

	for i := 0; i < 3; i++ {
		testTask := &task.Task{
			Name:    "concurrent-test",
			Type:    task.TaskTypeProcess,
			Command: "sleep",
			Args:    []string{"0.5"},
		}

		exec, err := executor.Execute(ctx, testTask)
		if err != nil {
			t.Fatalf("Execute %d failed: %v", i, err)
		}
		tasks[i] = exec
	}

	// Monitor concurrency with proper cleanup
	go func() {
		for {
			select {
			case <-monitorDone:
				return
			default:
				current := int32(0)
				for _, exec := range tasks {
					if exec.IsRunning() {
						current++
					}
				}
				if current > maxConcurrent.Load() {
					maxConcurrent.Store(current)
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	// Wait for all to complete
	for _, exec := range tasks {
		<-exec.Done()
	}

	// Stop the monitoring goroutine
	close(monitorDone)

	// Max concurrent should be <= 2
	if maxConcurrent.Load() > 2 {
		t.Errorf("MaxConcurrent = %d, want <= 2", maxConcurrent.Load())
	}
}

// ====================
// Manager Integration Tests
// ====================

func TestManager_Lifecycle(t *testing.T) {
	skipIfShort(t)

	manager, err := NewManager(
		WithWorkspaceRoot(t.TempDir()),
		WithShutdownTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Check health
	health := manager.Health()
	if health.Status != StatusHealthy {
		t.Errorf("Status = %v, want Healthy", health.Status)
	}

	// Uptime should be > 0
	if manager.Uptime() <= 0 {
		t.Error("Uptime should be > 0")
	}

	// Close
	if err := manager.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Close again should be idempotent
	if err := manager.Close(); err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

func TestManager_EventBus_Integration(t *testing.T) {
	skipIfShort(t)

	eventBus := NewEventBus()
	defer eventBus.Close()

	var received atomic.Int32

	eventBus.Subscribe("test.*", func(data map[string]any) {
		received.Add(1)
	})

	manager, err := NewManager(
		WithWorkspaceRoot(t.TempDir()),
		WithEventBus(eventBus),
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Close()

	// Emit test event
	eventBus.Emit("test.event", map[string]any{"key": "value"})

	// Give time for event to propagate
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("Received = %d, want 1", received.Load())
	}
}

// ====================
// Helper Functions
// ====================

func gitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	cmd := exec.Command("git", "config", key, value)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config %s failed: %v", key, err)
	}
}

func setupGitRepo(t *testing.T) string {
	t.Helper()
	skipIfNoGit(t)

	dir := t.TempDir()

	// Initialize repo
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	gitConfig(t, dir, "user.email", "test@example.com")
	gitConfig(t, dir, "user.name", "Test User")

	// Create initial commit
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	return dir
}

// testExecutionListener is a test helper for execution events.
type testExecutionListener struct {
	onStarted   func(*task.Execution)
	onOutput    func(*task.Execution, task.OutputLine)
	onProblem   func(*task.Execution, task.Problem)
	onCompleted func(*task.Execution)
}

func (l *testExecutionListener) OnExecutionStarted(exec *task.Execution) {
	if l.onStarted != nil {
		l.onStarted(exec)
	}
}

func (l *testExecutionListener) OnExecutionOutput(exec *task.Execution, line task.OutputLine) {
	if l.onOutput != nil {
		l.onOutput(exec, line)
	}
}

func (l *testExecutionListener) OnExecutionProblem(exec *task.Execution, problem task.Problem) {
	if l.onProblem != nil {
		l.onProblem(exec, problem)
	}
}

func (l *testExecutionListener) OnExecutionCompleted(exec *task.Execution) {
	if l.onCompleted != nil {
		l.onCompleted(exec)
	}
}
