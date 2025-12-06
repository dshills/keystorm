package process

import (
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// Supervisor manages child processes with lifecycle tracking and cleanup.
//
// The Supervisor provides:
//   - Process start and tracking
//   - Signal forwarding
//   - Graceful shutdown with timeout
//   - Resource cleanup
//
// Supervisor is safe for concurrent use.
type Supervisor struct {
	mu        sync.RWMutex
	processes map[string]*Process

	// shutdown signals that the supervisor is shutting down
	shutdown chan struct{}

	// closed indicates the supervisor has been shut down
	closed atomic.Bool

	// maxProcesses limits the number of concurrent processes (0 = unlimited)
	maxProcesses int

	// onProcessExit is called when a process exits
	onProcessExit func(p *Process)
}

// SupervisorOption configures a Supervisor instance.
type SupervisorOption func(*Supervisor)

// WithMaxProcesses sets the maximum number of concurrent processes.
// A value of 0 (default) means unlimited.
func WithMaxProcesses(max int) SupervisorOption {
	return func(s *Supervisor) {
		s.maxProcesses = max
	}
}

// WithProcessExitCallback sets a callback for when processes exit.
func WithProcessExitCallback(fn func(p *Process)) SupervisorOption {
	return func(s *Supervisor) {
		s.onProcessExit = fn
	}
}

// NewSupervisor creates a new process supervisor.
func NewSupervisor(opts ...SupervisorOption) *Supervisor {
	s := &Supervisor{
		processes: make(map[string]*Process),
		shutdown:  make(chan struct{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts a new managed process.
//
// The command's stdin, stdout, and stderr are automatically piped
// unless already configured. The process is tracked and will be
// cleaned up on shutdown.
//
// Returns ErrSupervisorShutdown if the supervisor is shutting down.
func (s *Supervisor) Start(name string, cmd *exec.Cmd) (*Process, error) {
	return s.StartWithID(uuid.New().String(), name, cmd)
}

// StartWithID starts a new managed process with a specific ID.
//
// This is useful when you need to control the process ID, for example
// when restoring state or for deterministic testing.
func (s *Supervisor) StartWithID(id, name string, cmd *exec.Cmd) (*Process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check shutdown state under lock to prevent race
	if s.closed.Load() {
		return nil, ErrSupervisorShutdown
	}

	// Check process limit
	if s.maxProcesses > 0 && len(s.processes) >= s.maxProcesses {
		return nil, fmt.Errorf("process limit reached: %d", s.maxProcesses)
	}

	// Check for duplicate ID
	if _, exists := s.processes[id]; exists {
		return nil, fmt.Errorf("process ID already exists: %s", id)
	}

	// Create process wrapper
	proc := NewProcess(id, name, cmd)

	// Setup I/O pipes if not already configured
	// Track created pipes for cleanup on error
	var createdPipes []interface{ Close() error }
	cleanupPipes := func() {
		for _, p := range createdPipes {
			_ = p.Close()
		}
	}

	if cmd.Stdin == nil {
		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			cleanupPipes()
			return nil, fmt.Errorf("create stdin pipe: %w", err)
		}
		proc.Stdin = stdinPipe
		createdPipes = append(createdPipes, stdinPipe)
	}

	if cmd.Stdout == nil {
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			cleanupPipes()
			return nil, fmt.Errorf("create stdout pipe: %w", err)
		}
		proc.Stdout = stdoutPipe
		createdPipes = append(createdPipes, stdoutPipe)
	}

	if cmd.Stderr == nil {
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			cleanupPipes()
			return nil, fmt.Errorf("create stderr pipe: %w", err)
		}
		proc.Stderr = stderrPipe
		createdPipes = append(createdPipes, stderrPipe)
	}

	// Start the process before tracking (so we don't track failed starts)
	if err := proc.start(); err != nil {
		cleanupPipes()
		return nil, err
	}

	// Track process after successful start
	s.processes[id] = proc

	// Monitor for exit
	go s.monitorProcess(proc)

	return proc, nil
}

// monitorProcess watches for process exit and cleans up.
func (s *Supervisor) monitorProcess(proc *Process) {
	<-proc.Done()

	// Call exit callback if set, with panic recovery
	if s.onProcessExit != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't crash - callback errors shouldn't affect supervisor
				}
			}()
			s.onProcessExit(proc)
		}()
	}

	// Remove from tracking
	s.mu.Lock()
	delete(s.processes, proc.ID)
	s.mu.Unlock()
}

// Get returns a process by ID.
// Returns nil if the process is not found.
func (s *Supervisor) Get(id string) *Process {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.processes[id]
}

// GetByName returns processes matching the given name.
// Multiple processes can have the same name.
func (s *Supervisor) GetByName(name string) []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Process
	for _, p := range s.processes {
		if p.Name == name {
			result = append(result, p)
		}
	}
	return result
}

// List returns all managed processes.
func (s *Supervisor) List() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		result = append(result, p)
	}
	return result
}

// Count returns the number of managed processes.
func (s *Supervisor) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.processes)
}

// Kill kills a process by ID.
// Returns ErrProcessNotFound if the process doesn't exist.
func (s *Supervisor) Kill(id string) error {
	proc := s.Get(id)
	if proc == nil {
		return ErrProcessNotFound
	}

	if !proc.IsRunning() {
		return nil // Already exited
	}

	return proc.Kill()
}

// Terminate sends SIGTERM to a process by ID.
// Returns ErrProcessNotFound if the process doesn't exist.
func (s *Supervisor) Terminate(id string) error {
	proc := s.Get(id)
	if proc == nil {
		return ErrProcessNotFound
	}

	if !proc.IsRunning() {
		return nil // Already exited
	}

	return proc.Terminate()
}

// Signal sends a signal to a process by ID.
func (s *Supervisor) Signal(id string, sig syscall.Signal) error {
	proc := s.Get(id)
	if proc == nil {
		return ErrProcessNotFound
	}

	if !proc.IsRunning() {
		return nil // Already exited
	}

	return proc.Signal(sig)
}

// KillAll kills all managed processes immediately.
func (s *Supervisor) KillAll() {
	s.mu.RLock()
	procs := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		procs = append(procs, p)
	}
	s.mu.RUnlock()

	for _, p := range procs {
		if p.IsRunning() {
			_ = p.Kill()
		}
	}
}

// TerminateAll sends SIGTERM to all managed processes.
func (s *Supervisor) TerminateAll() {
	s.mu.RLock()
	procs := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		procs = append(procs, p)
	}
	s.mu.RUnlock()

	for _, p := range procs {
		if p.IsRunning() {
			_ = p.Terminate()
		}
	}
}

// Shutdown gracefully shuts down all processes.
//
// It first sends SIGTERM to all processes and waits up to timeout
// for them to exit. Any processes still running after the timeout
// are killed with SIGKILL.
//
// Shutdown blocks until all processes have exited and been removed.
func (s *Supervisor) Shutdown(timeout time.Duration) {
	if s.closed.Swap(true) {
		return // Already shutting down
	}

	close(s.shutdown)

	// Get all processes
	s.mu.RLock()
	procs := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		procs = append(procs, p)
	}
	s.mu.RUnlock()

	if len(procs) == 0 {
		return
	}

	// Send SIGTERM to all
	for _, p := range procs {
		if p.IsRunning() {
			_ = p.Terminate()
		}
	}

	// Wait for processes to exit with timeout
	done := make(chan struct{})
	go func() {
		for _, p := range procs {
			<-p.Done()
		}
		close(done)
	}()

	select {
	case <-done:
		// All processes exited gracefully
	case <-time.After(timeout):
		// Timeout - kill remaining processes
		for _, p := range procs {
			if p.IsRunning() {
				_ = p.Kill()
			}
		}
		// Wait for kill to complete
		<-done
	}

	// Wait for monitor goroutines to finish cleanup (remove from map)
	// This ensures Count() returns 0 after Shutdown completes
	s.waitForCleanup()
}

// waitForCleanup waits for all processes to be removed from the map.
func (s *Supervisor) waitForCleanup() {
	for {
		s.mu.RLock()
		count := len(s.processes)
		s.mu.RUnlock()
		if count == 0 {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// IsShuttingDown returns true if the supervisor is shutting down.
func (s *Supervisor) IsShuttingDown() bool {
	return s.closed.Load()
}

// ShutdownChan returns a channel that is closed when shutdown begins.
func (s *Supervisor) ShutdownChan() <-chan struct{} {
	return s.shutdown
}

// Wait blocks until all processes have exited.
func (s *Supervisor) Wait() {
	for {
		s.mu.RLock()
		count := len(s.processes)
		procs := make([]*Process, 0, count)
		for _, p := range s.processes {
			procs = append(procs, p)
		}
		s.mu.RUnlock()

		if count == 0 {
			return
		}

		// Wait for any process to exit
		for _, p := range procs {
			select {
			case <-p.Done():
				// Process exited, loop again to check remaining
			default:
				continue
			}
			break
		}

		// Brief sleep to avoid tight loop
		time.Sleep(10 * time.Millisecond)
	}
}

// Sentinel errors.
var (
	// ErrProcessNotFound is returned when a process ID is not found.
	ErrProcessNotFound = fmt.Errorf("process not found")

	// ErrSupervisorShutdown is returned when the supervisor is shutting down.
	ErrSupervisorShutdown = fmt.Errorf("supervisor is shutting down")
)
