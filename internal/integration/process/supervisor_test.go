package process

import (
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestNewSupervisor(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	if s == nil {
		t.Fatal("expected non-nil supervisor")
	}

	if s.Count() != 0 {
		t.Errorf("expected 0 processes, got %d", s.Count())
	}

	if s.IsShuttingDown() {
		t.Error("expected IsShuttingDown() to be false")
	}
}

func TestSupervisor_WithMaxProcesses(t *testing.T) {
	s := NewSupervisor(WithMaxProcesses(2))
	defer s.Shutdown(time.Second)

	// Start first process
	proc1, err := s.Start("proc1", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start proc1: %v", err)
	}
	defer proc1.Kill()

	// Start second process
	proc2, err := s.Start("proc2", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start proc2: %v", err)
	}
	defer proc2.Kill()

	// Third process should fail
	_, err = s.Start("proc3", exec.Command("sleep", "10"))
	if err == nil {
		t.Error("expected error when exceeding max processes")
	}
}

func TestSupervisor_WithProcessExitCallback(t *testing.T) {
	var exitedProc *Process
	var callbackCalled atomic.Bool

	s := NewSupervisor(WithProcessExitCallback(func(p *Process) {
		exitedProc = p
		callbackCalled.Store(true)
	}))
	defer s.Shutdown(time.Second)

	proc, err := s.Start("test", exec.Command("echo", "hello"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Wait for process to exit
	<-proc.Done()

	// Give callback time to be called
	time.Sleep(50 * time.Millisecond)

	if !callbackCalled.Load() {
		t.Error("exit callback was not called")
	}

	if exitedProc == nil || exitedProc.ID != proc.ID {
		t.Error("callback received wrong process")
	}
}

func TestSupervisor_Start(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc, err := s.Start("test", exec.Command("echo", "hello"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	if proc.ID == "" {
		t.Error("expected non-empty process ID")
	}

	if proc.Name != "test" {
		t.Errorf("expected name 'test', got %q", proc.Name)
	}

	// Process should be tracked
	if s.Count() != 1 {
		t.Errorf("expected 1 process, got %d", s.Count())
	}

	// Wait for exit
	<-proc.Done()

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	// Process should be removed after exit
	if s.Count() != 0 {
		t.Errorf("expected 0 processes after exit, got %d", s.Count())
	}
}

func TestSupervisor_StartWithID(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc, err := s.StartWithID("my-custom-id", "test", exec.Command("echo", "hello"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	if proc.ID != "my-custom-id" {
		t.Errorf("expected ID 'my-custom-id', got %q", proc.ID)
	}

	<-proc.Done()
}

func TestSupervisor_StartWithID_Duplicate(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc1, err := s.StartWithID("same-id", "test1", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start proc1: %v", err)
	}
	defer proc1.Kill()

	// Try to start with same ID
	_, err = s.StartWithID("same-id", "test2", exec.Command("sleep", "10"))
	if err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestSupervisor_Get(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc, err := s.StartWithID("test-id", "test", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	defer proc.Kill()

	// Get existing process
	got := s.Get("test-id")
	if got == nil {
		t.Fatal("expected to find process")
	}
	if got.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %q", got.ID)
	}

	// Get non-existent process
	got = s.Get("non-existent")
	if got != nil {
		t.Error("expected nil for non-existent process")
	}
}

func TestSupervisor_GetByName(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	// Start multiple processes with same name
	proc1, _ := s.Start("workers", exec.Command("sleep", "10"))
	proc2, _ := s.Start("workers", exec.Command("sleep", "10"))
	proc3, _ := s.Start("other", exec.Command("sleep", "10"))
	defer proc1.Kill()
	defer proc2.Kill()
	defer proc3.Kill()

	workers := s.GetByName("workers")
	if len(workers) != 2 {
		t.Errorf("expected 2 workers, got %d", len(workers))
	}

	others := s.GetByName("other")
	if len(others) != 1 {
		t.Errorf("expected 1 other, got %d", len(others))
	}

	none := s.GetByName("non-existent")
	if len(none) != 0 {
		t.Errorf("expected 0 for non-existent name, got %d", len(none))
	}
}

func TestSupervisor_List(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc1, _ := s.Start("proc1", exec.Command("sleep", "10"))
	proc2, _ := s.Start("proc2", exec.Command("sleep", "10"))
	defer proc1.Kill()
	defer proc2.Kill()

	list := s.List()
	if len(list) != 2 {
		t.Errorf("expected 2 processes, got %d", len(list))
	}
}

func TestSupervisor_Kill(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc, err := s.StartWithID("test-id", "test", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = s.Kill("test-id")
	if err != nil {
		t.Fatalf("failed to kill process: %v", err)
	}

	select {
	case <-proc.Done():
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after Kill")
	}
}

func TestSupervisor_Kill_NotFound(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	err := s.Kill("non-existent")
	if err != ErrProcessNotFound {
		t.Errorf("expected ErrProcessNotFound, got %v", err)
	}
}

func TestSupervisor_Terminate(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc, err := s.StartWithID("test-id", "test", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = s.Terminate("test-id")
	if err != nil {
		t.Fatalf("failed to terminate process: %v", err)
	}

	select {
	case <-proc.Done():
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after Terminate")
	}
}

func TestSupervisor_Signal(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc, err := s.StartWithID("test-id", "test", exec.Command("sleep", "10"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = s.Signal("test-id", syscall.SIGINT)
	if err != nil {
		t.Fatalf("failed to signal process: %v", err)
	}

	select {
	case <-proc.Done():
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after Signal")
	}
}

func TestSupervisor_KillAll(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc1, _ := s.Start("proc1", exec.Command("sleep", "10"))
	proc2, _ := s.Start("proc2", exec.Command("sleep", "10"))

	time.Sleep(50 * time.Millisecond)

	s.KillAll()

	// Both processes should exit
	select {
	case <-proc1.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("proc1 did not exit")
	}

	select {
	case <-proc2.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("proc2 did not exit")
	}
}

func TestSupervisor_TerminateAll(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc1, _ := s.Start("proc1", exec.Command("sleep", "10"))
	proc2, _ := s.Start("proc2", exec.Command("sleep", "10"))

	time.Sleep(50 * time.Millisecond)

	s.TerminateAll()

	select {
	case <-proc1.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("proc1 did not exit")
	}

	select {
	case <-proc2.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("proc2 did not exit")
	}
}

func TestSupervisor_Shutdown(t *testing.T) {
	s := NewSupervisor()

	proc1, _ := s.Start("proc1", exec.Command("sleep", "10"))
	proc2, _ := s.Start("proc2", exec.Command("sleep", "10"))

	time.Sleep(50 * time.Millisecond)

	// Shutdown should terminate all processes
	s.Shutdown(2 * time.Second)

	// All processes should be done
	select {
	case <-proc1.Done():
	default:
		t.Error("proc1 should be done after shutdown")
	}

	select {
	case <-proc2.Done():
	default:
		t.Error("proc2 should be done after shutdown")
	}

	if !s.IsShuttingDown() {
		t.Error("expected IsShuttingDown() to be true")
	}
}

func TestSupervisor_Shutdown_Timeout(t *testing.T) {
	s := NewSupervisor()

	// Start a process that ignores SIGTERM (using trap)
	proc, _ := s.Start("stubborn", exec.Command("sh", "-c", "trap '' TERM; sleep 60"))

	time.Sleep(100 * time.Millisecond)

	// Shutdown with short timeout - should SIGKILL
	start := time.Now()
	s.Shutdown(500 * time.Millisecond)
	elapsed := time.Since(start)

	// Should have taken roughly the timeout duration
	if elapsed < 400*time.Millisecond {
		t.Errorf("shutdown was too fast: %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("shutdown took too long: %v", elapsed)
	}

	// Process should be killed
	select {
	case <-proc.Done():
	default:
		t.Error("process should be done after shutdown")
	}
}

func TestSupervisor_Shutdown_Idempotent(t *testing.T) {
	s := NewSupervisor()

	// Multiple shutdowns should not panic
	s.Shutdown(time.Second)
	s.Shutdown(time.Second)
	s.Shutdown(time.Second)
}

func TestSupervisor_StartAfterShutdown(t *testing.T) {
	s := NewSupervisor()
	s.Shutdown(time.Second)

	_, err := s.Start("test", exec.Command("echo", "hello"))
	if err != ErrSupervisorShutdown {
		t.Errorf("expected ErrSupervisorShutdown, got %v", err)
	}
}

func TestSupervisor_ShutdownChan(t *testing.T) {
	s := NewSupervisor()

	// Channel should not be closed yet
	select {
	case <-s.ShutdownChan():
		t.Error("shutdown channel should not be closed yet")
	default:
		// Expected
	}

	s.Shutdown(time.Second)

	// Channel should be closed
	select {
	case <-s.ShutdownChan():
		// Expected
	default:
		t.Error("shutdown channel should be closed after shutdown")
	}
}

func TestSupervisor_Concurrent(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Start many processes concurrently
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proc, err := s.Start("worker", exec.Command("echo", "hello"))
			if err != nil {
				errors <- err
				return
			}
			<-proc.Done()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestSupervisor_Wait(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	// Start some short-lived processes
	s.Start("proc1", exec.Command("echo", "1"))
	s.Start("proc2", exec.Command("echo", "2"))
	s.Start("proc3", exec.Command("echo", "3"))

	// Wait should return when all processes exit
	done := make(chan struct{})
	go func() {
		s.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() did not return after processes exited")
	}
}

func TestSupervisor_ProcessIO(t *testing.T) {
	s := NewSupervisor()
	defer s.Shutdown(time.Second)

	proc, err := s.Start("echo", exec.Command("echo", "hello world"))
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Should have stdout pipe
	if proc.Stdout == nil {
		t.Fatal("expected stdout pipe")
	}

	// Read output
	buf := make([]byte, 1024)
	n, err := proc.Stdout.Read(buf)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	output := string(buf[:n])
	if output != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", output)
	}

	<-proc.Done()
}
