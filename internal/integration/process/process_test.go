package process

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestNewProcess(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	proc := NewProcess("test-id", "test-process", cmd)

	if proc.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %q", proc.ID)
	}

	if proc.Name != "test-process" {
		t.Errorf("expected Name 'test-process', got %q", proc.Name)
	}

	if proc.State() != StateCreated {
		t.Errorf("expected state StateCreated, got %v", proc.State())
	}

	if proc.ExitCode() != -1 {
		t.Errorf("expected exit code -1, got %d", proc.ExitCode())
	}

	if proc.PID() != -1 {
		t.Errorf("expected PID -1 before start, got %d", proc.PID())
	}

	if proc.IsRunning() {
		t.Error("expected IsRunning() to be false before start")
	}

	if proc.HasExited() {
		t.Error("expected HasExited() to be false before start")
	}
}

func TestProcess_Start(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	proc := NewProcess("test-id", "test-process", cmd)

	err := proc.start()
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	if proc.State() != StateRunning {
		t.Errorf("expected state StateRunning, got %v", proc.State())
	}

	if proc.PID() <= 0 {
		t.Errorf("expected positive PID, got %d", proc.PID())
	}

	if proc.Started.IsZero() {
		t.Error("expected Started time to be set")
	}

	// Wait for process to complete
	<-proc.Done()

	if proc.State() != StateExited {
		t.Errorf("expected state StateExited, got %v", proc.State())
	}

	if proc.ExitCode() != 0 {
		t.Errorf("expected exit code 0, got %d", proc.ExitCode())
	}

	if !proc.HasExited() {
		t.Error("expected HasExited() to be true after exit")
	}
}

func TestProcess_StartTwice(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	proc := NewProcess("test-id", "test-process", cmd)

	err := proc.start()
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Try to start again
	err = proc.start()
	if err != ErrProcessAlreadyStarted {
		t.Errorf("expected ErrProcessAlreadyStarted, got %v", err)
	}
}

func TestProcess_ExitCode(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *exec.Cmd
		wantCode int
	}{
		{
			name:     "success",
			cmd:      exec.Command("true"),
			wantCode: 0,
		},
		{
			name:     "failure",
			cmd:      exec.Command("false"),
			wantCode: 1,
		},
		{
			name:     "exit 42",
			cmd:      exec.Command("sh", "-c", "exit 42"),
			wantCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := NewProcess("test-id", tt.name, tt.cmd)

			err := proc.start()
			if err != nil {
				t.Fatalf("failed to start process: %v", err)
			}

			<-proc.Done()

			if proc.ExitCode() != tt.wantCode {
				t.Errorf("expected exit code %d, got %d", tt.wantCode, proc.ExitCode())
			}
		})
	}
}

func TestProcess_Signal(t *testing.T) {
	// Start a long-running process
	cmd := exec.Command("sleep", "10")
	proc := NewProcess("test-id", "sleep", cmd)

	err := proc.start()
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGTERM
	err = proc.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatalf("failed to signal process: %v", err)
	}

	// Wait for exit
	select {
	case <-proc.Done():
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after SIGTERM")
	}

	if proc.State() != StateKilled {
		t.Errorf("expected state StateKilled, got %v", proc.State())
	}
}

func TestProcess_Kill(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	proc := NewProcess("test-id", "sleep", cmd)

	err := proc.start()
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = proc.Kill()
	if err != nil {
		t.Fatalf("failed to kill process: %v", err)
	}

	select {
	case <-proc.Done():
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after SIGKILL")
	}
}

func TestProcess_Terminate(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	proc := NewProcess("test-id", "sleep", cmd)

	err := proc.start()
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = proc.Terminate()
	if err != nil {
		t.Fatalf("failed to terminate process: %v", err)
	}

	select {
	case <-proc.Done():
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after SIGTERM")
	}
}

func TestProcess_SignalBeforeStart(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	proc := NewProcess("test-id", "test", cmd)

	err := proc.Signal(syscall.SIGTERM)
	if err == nil {
		t.Error("expected error when signaling non-started process")
	}
}

func TestProcess_Runtime(t *testing.T) {
	cmd := exec.Command("sleep", "0.1")
	proc := NewProcess("test-id", "sleep", cmd)

	// Runtime before start should be 0
	if proc.Runtime() != 0 {
		t.Errorf("expected runtime 0 before start, got %v", proc.Runtime())
	}

	err := proc.start()
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Runtime during execution should be positive
	time.Sleep(50 * time.Millisecond)
	runtime := proc.Runtime()
	if runtime < 50*time.Millisecond {
		t.Errorf("expected runtime >= 50ms, got %v", runtime)
	}

	<-proc.Done()
}

func TestProcess_State_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateCreated, "created"},
		{StateRunning, "running"},
		{StateExited, "exited"},
		{StateKilled, "killed"},
		{State(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcess_Done_ClosedAfterExit(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	proc := NewProcess("test-id", "test", cmd)

	err := proc.start()
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Wait for done
	<-proc.Done()

	// Done channel should remain closed (not block on second receive)
	select {
	case <-proc.Done():
		// Expected - channel is closed
	default:
		t.Error("Done() channel should remain closed after exit")
	}
}
