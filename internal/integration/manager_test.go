package integration

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// mockEventBus implements EventPublisher for testing.
type mockEventBus struct {
	mu     sync.Mutex
	events []mockEvent
}

type mockEvent struct {
	Type string
	Data map[string]any
}

func (m *mockEventBus) Publish(eventType string, data map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, mockEvent{Type: eventType, Data: data})
}

func (m *mockEventBus) Events() []mockEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockEvent, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockEventBus) HasEvent(eventType string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.events {
		if e.Type == eventType {
			return true
		}
	}
	return false
}

func TestNewManager(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	if m.IsClosed() {
		t.Error("expected IsClosed() to be false")
	}

	if m.Supervisor() == nil {
		t.Error("expected non-nil supervisor")
	}
}

func TestNewManager_WithOptions(t *testing.T) {
	eb := &mockEventBus{}

	m, err := NewManager(
		WithWorkspaceRoot("/test/workspace"),
		WithEventBus(eb),
		WithMaxProcesses(10),
		WithShutdownTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	if m.WorkspaceRoot() != "/test/workspace" {
		t.Errorf("expected workspace '/test/workspace', got %q", m.WorkspaceRoot())
	}

	if m.EventBus() != eb {
		t.Error("expected event bus to be set")
	}

	// Check started event was published
	if !eb.HasEvent("integration.started") {
		t.Error("expected 'integration.started' event")
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	eb := &mockEventBus{}

	m, err := NewManagerWithConfig(ManagerConfig{
		WorkspaceRoot:   "/test/workspace",
		EventBus:        eb,
		MaxProcesses:    5,
		ShutdownTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	if m.WorkspaceRoot() != "/test/workspace" {
		t.Errorf("expected workspace '/test/workspace', got %q", m.WorkspaceRoot())
	}
}

func TestManager_Close(t *testing.T) {
	eb := &mockEventBus{}

	m, err := NewManager(WithEventBus(eb))
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Start a process to test cleanup
	_, err = m.Supervisor().Start("test", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	err = m.Close()
	if err != nil {
		t.Fatalf("failed to close manager: %v", err)
	}

	if !m.IsClosed() {
		t.Error("expected IsClosed() to be true after close")
	}

	// Process should be cleaned up
	if m.Supervisor().Count() != 0 {
		t.Errorf("expected 0 processes after close, got %d", m.Supervisor().Count())
	}

	// Check events
	if !eb.HasEvent("integration.stopping") {
		t.Error("expected 'integration.stopping' event")
	}
	if !eb.HasEvent("integration.stopped") {
		t.Error("expected 'integration.stopped' event")
	}
}

func TestManager_Close_Idempotent(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Multiple closes should not panic
	m.Close()
	m.Close()
	m.Close()
}

func TestManager_CloseWithTimeout(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Start a stubborn process
	m.Supervisor().Start("stubborn", exec.Command("sh", "-c", "trap '' TERM; sleep 60"))

	// Give process time to start and set up trap
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	err = m.CloseWithTimeout(500 * time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("CloseWithTimeout returned error: %v", err)
	}

	// Should have taken roughly the timeout duration
	if elapsed < 400*time.Millisecond {
		t.Errorf("close was too fast: %v", elapsed)
	}
}

func TestManager_WorkspaceRoot(t *testing.T) {
	m, err := NewManager(WithWorkspaceRoot("/initial/path"))
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	if m.WorkspaceRoot() != "/initial/path" {
		t.Errorf("expected '/initial/path', got %q", m.WorkspaceRoot())
	}

	m.SetWorkspaceRoot("/new/path")

	if m.WorkspaceRoot() != "/new/path" {
		t.Errorf("expected '/new/path', got %q", m.WorkspaceRoot())
	}
}

func TestManager_SetWorkspaceRoot_Event(t *testing.T) {
	eb := &mockEventBus{}
	m, err := NewManager(WithEventBus(eb))
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	m.SetWorkspaceRoot("/new/workspace")

	if !eb.HasEvent("integration.workspace.changed") {
		t.Error("expected 'integration.workspace.changed' event")
	}
}

func TestManager_Config(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	// Initially nil
	if m.Config() != nil {
		t.Error("expected nil config initially")
	}
}

func TestManager_EventBus(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	// Initially nil
	if m.EventBus() != nil {
		t.Error("expected nil event bus initially")
	}

	eb := &mockEventBus{}
	m.SetEventBus(eb)

	if m.EventBus() != eb {
		t.Error("expected event bus to be set")
	}
}

func TestManager_Uptime(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	time.Sleep(100 * time.Millisecond)

	uptime := m.Uptime()
	if uptime < 100*time.Millisecond {
		t.Errorf("expected uptime >= 100ms, got %v", uptime)
	}
}

func TestManager_Health(t *testing.T) {
	eb := &mockEventBus{}
	m, err := NewManager(
		WithWorkspaceRoot("/test/workspace"),
		WithEventBus(eb),
	)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	health := m.Health()

	if health.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy, got %v", health.Status)
	}

	if health.WorkspaceRoot != "/test/workspace" {
		t.Errorf("expected workspace '/test/workspace', got %q", health.WorkspaceRoot)
	}

	if !health.EventsEnabled {
		t.Error("expected EventsEnabled to be true")
	}

	if _, ok := health.Components["supervisor"]; !ok {
		t.Error("expected supervisor component in health")
	}
}

func TestManager_Health_WithProcesses(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	// Start some processes
	m.Supervisor().Start("proc1", exec.Command("sleep", "10"))
	m.Supervisor().Start("proc2", exec.Command("sleep", "10"))

	health := m.Health()

	if health.ProcessCount != 2 {
		t.Errorf("expected 2 processes, got %d", health.ProcessCount)
	}
}

func TestManager_ShutdownChan(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Channel should not be closed yet
	select {
	case <-m.ShutdownChan():
		t.Error("shutdown channel should not be closed yet")
	default:
		// Expected
	}

	m.Close()

	// Channel should be closed
	select {
	case <-m.ShutdownChan():
		// Expected
	default:
		t.Error("shutdown channel should be closed after close")
	}
}

func TestManager_WaitForShutdown(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	done := make(chan struct{})
	go func() {
		m.WaitForShutdown()
		close(done)
	}()

	// Should not complete yet
	select {
	case <-done:
		t.Fatal("WaitForShutdown returned before Close")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}

	m.Close()

	// Should complete now
	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForShutdown did not return after Close")
	}
}

func TestManager_WaitForShutdownContext(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = m.WaitForShutdownContext(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}

	// Now close and wait with cancelled context
	m.Close()

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	err = m.WaitForShutdownContext(ctx2)
	if err != nil {
		t.Errorf("expected nil after close, got %v", err)
	}
}

func TestManager_Supervisor_Integration(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	// Start a process through the supervisor
	proc, err := m.Supervisor().Start("echo", exec.Command("echo", "hello"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Wait for completion
	<-proc.Done()

	if proc.ExitCode() != 0 {
		t.Errorf("expected exit code 0, got %d", proc.ExitCode())
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
		{Status(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("Status.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestManager_Concurrent(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer m.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent operations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Read workspace root
			_ = m.WorkspaceRoot()

			// Get health
			_ = m.Health()

			// Check supervisor
			_ = m.Supervisor().Count()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent error: %v", err)
	}
}
