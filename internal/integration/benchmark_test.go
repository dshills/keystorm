package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/integration/git"
	"github.com/dshills/keystorm/internal/integration/task"
)

// ====================
// Git Benchmarks
// ====================

func BenchmarkGit_Status(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git not found")
	}

	dir := setupBenchmarkRepo(b)

	manager := git.NewManager(git.ManagerConfig{
		StatusCacheTTL: 0, // Disable cache for accurate benchmark
	})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		b.Fatalf("Open failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.Status()
		if err != nil {
			b.Fatalf("Status failed: %v", err)
		}
	}
}

func BenchmarkGit_Status_Cached(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git not found")
	}

	dir := setupBenchmarkRepo(b)

	manager := git.NewManager(git.ManagerConfig{
		StatusCacheTTL: time.Hour, // Long cache for benchmark
	})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		b.Fatalf("Open failed: %v", err)
	}

	// Warm up cache
	_, _ = repo.Status()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.Status()
		if err != nil {
			b.Fatalf("Status failed: %v", err)
		}
	}
}

func BenchmarkGit_Head(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git not found")
	}

	dir := setupBenchmarkRepo(b)

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		b.Fatalf("Open failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.Head()
		if err != nil {
			b.Fatalf("Head failed: %v", err)
		}
	}
}

func BenchmarkGit_CurrentBranch(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git not found")
	}

	dir := setupBenchmarkRepo(b)

	manager := git.NewManager(git.ManagerConfig{})
	defer manager.Close()

	repo, err := manager.Open(dir)
	if err != nil {
		b.Fatalf("Open failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.CurrentBranch()
		if err != nil {
			b.Fatalf("CurrentBranch failed: %v", err)
		}
	}
}

// ====================
// Task Benchmarks
// ====================

func BenchmarkTask_Execute_Simple(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	executor := task.NewExecutor(task.DefaultExecutorConfig())

	testTask := &task.Task{
		Name:    "bench-task",
		Type:    task.TaskTypeShell,
		Command: "true",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exec, err := executor.ExecuteSync(ctx, testTask)
		if err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
		if exec.State != task.ExecutionStateSucceeded {
			b.Fatalf("Task failed: %v", exec.Error)
		}
	}
}

func BenchmarkTask_Execute_WithOutput(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	executor := task.NewExecutor(task.DefaultExecutorConfig())

	testTask := &task.Task{
		Name:    "bench-task",
		Type:    task.TaskTypeShell,
		Command: "echo hello world",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exec, err := executor.ExecuteSync(ctx, testTask)
		if err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
		if exec.State != task.ExecutionStateSucceeded {
			b.Fatalf("Task failed: %v", exec.Error)
		}
	}
}

// ====================
// Event Bus Benchmarks
// ====================

func BenchmarkEventBus_Emit_NoSubscribers(b *testing.B) {
	bus := NewEventBus()
	defer bus.Close()

	data := map[string]any{"key": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit("test.event", data)
	}
}

func BenchmarkEventBus_Emit_OneSubscriber(b *testing.B) {
	bus := NewEventBus()
	defer bus.Close()

	bus.Subscribe("test.event", func(data map[string]any) {
		// Empty handler
	})

	data := map[string]any{"key": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit("test.event", data)
	}
}

func BenchmarkEventBus_Emit_TenSubscribers(b *testing.B) {
	bus := NewEventBus()
	defer bus.Close()

	for i := 0; i < 10; i++ {
		bus.Subscribe("test.event", func(data map[string]any) {
			// Empty handler
		})
	}

	data := map[string]any{"key": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit("test.event", data)
	}
}

func BenchmarkEventBus_Emit_Wildcard(b *testing.B) {
	bus := NewEventBus()
	defer bus.Close()

	bus.Subscribe("test.*", func(data map[string]any) {
		// Empty handler
	})

	data := map[string]any{"key": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit("test.event", data)
	}
}

func BenchmarkEventBus_Subscribe_Unsubscribe(b *testing.B) {
	bus := NewEventBus()
	defer bus.Close()

	handler := func(data map[string]any) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := bus.Subscribe("test.event", handler)
		bus.Unsubscribe(id)
	}
}

func BenchmarkEventBus_EmitAsync(b *testing.B) {
	bus := NewEventBus()
	defer bus.Close()

	bus.Subscribe("test.event", func(data map[string]any) {
		time.Sleep(time.Microsecond) // Simulate work
	})

	data := map[string]any{"key": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.EmitAsync("test.event", data)
	}
}

// ====================
// Manager Benchmarks
// ====================

func BenchmarkManager_Health(b *testing.B) {
	manager, err := NewManager(
		WithWorkspaceRoot(b.TempDir()),
	)
	if err != nil {
		b.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Health()
	}
}

func BenchmarkManager_Uptime(b *testing.B) {
	manager, err := NewManager(
		WithWorkspaceRoot(b.TempDir()),
	)
	if err != nil {
		b.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Uptime()
	}
}

// ====================
// Helper Functions
// ====================

func setupBenchmarkRepo(b *testing.B) string {
	b.Helper()

	dir := b.TempDir()

	// Initialize repo
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if err := cmd.Run(); err != nil {
		b.Fatalf("git init failed: %v", err)
	}

	// Configure git
	for _, cfg := range [][]string{
		{"user.email", "test@example.com"},
		{"user.name", "Test User"},
	} {
		cmd := exec.Command("git", "config", cfg[0], cfg[1])
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			b.Fatalf("git config failed: %v", err)
		}
	}

	// Create initial commit with some files
	for i := 0; i < 10; i++ {
		filename := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filename, []byte("content\n"), 0644); err != nil {
			b.Fatalf("WriteFile failed: %v", err)
		}
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		b.Fatalf("git add failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		b.Fatalf("git commit failed: %v", err)
	}

	return dir
}
