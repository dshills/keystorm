package process

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// State represents the state of a process.
type State int

const (
	// StateCreated indicates the process has been created but not started.
	StateCreated State = iota
	// StateRunning indicates the process is currently running.
	StateRunning
	// StateExited indicates the process has exited normally or with an error.
	StateExited
	// StateKilled indicates the process was killed by a signal.
	StateKilled
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateRunning:
		return "running"
	case StateExited:
		return "exited"
	case StateKilled:
		return "killed"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// Process represents a managed child process.
//
// Process wraps an exec.Cmd with lifecycle management, exit tracking,
// and standard I/O access. It is safe for concurrent use.
type Process struct {
	// ID is the unique identifier for this process.
	ID string

	// Name is a human-readable name for the process.
	Name string

	// Cmd is the underlying exec.Cmd.
	Cmd *exec.Cmd

	// Stdin provides write access to the process's stdin.
	// May be nil if stdin was not piped.
	Stdin io.WriteCloser

	// Stdout provides read access to the process's stdout.
	// May be nil if stdout was not piped.
	Stdout io.ReadCloser

	// Stderr provides read access to the process's stderr.
	// May be nil if stderr was not piped.
	Stderr io.ReadCloser

	// Started is the time the process was started.
	Started time.Time

	// done is closed when the process exits.
	done chan struct{}

	// state tracks the current process state.
	state atomic.Int32

	// exitCode stores the exit code after the process exits.
	exitCode atomic.Int32

	// exitErr stores any error from Wait().
	exitErr error

	// mu protects exitErr.
	mu sync.RWMutex

	// waitOnce ensures Wait is only called once.
	waitOnce sync.Once
}

// NewProcess creates a new Process wrapping the given command.
//
// The command should not be started before calling NewProcess.
// Use Supervisor.Start() to start the process with proper tracking.
func NewProcess(id, name string, cmd *exec.Cmd) *Process {
	p := &Process{
		ID:   id,
		Name: name,
		Cmd:  cmd,
		done: make(chan struct{}),
	}
	p.state.Store(int32(StateCreated))
	p.exitCode.Store(-1) // -1 indicates not exited
	return p
}

// State returns the current process state.
func (p *Process) State() State {
	return State(p.state.Load())
}

// ExitCode returns the process exit code.
// Returns -1 if the process has not exited.
func (p *Process) ExitCode() int {
	return int(p.exitCode.Load())
}

// ExitError returns any error from waiting on the process.
// Returns nil if the process exited successfully or hasn't exited.
func (p *Process) ExitError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitErr
}

// Done returns a channel that is closed when the process exits.
func (p *Process) Done() <-chan struct{} {
	return p.done
}

// IsRunning returns true if the process is currently running.
func (p *Process) IsRunning() bool {
	return p.State() == StateRunning
}

// HasExited returns true if the process has exited (normally or killed).
func (p *Process) HasExited() bool {
	state := p.State()
	return state == StateExited || state == StateKilled
}

// PID returns the process ID, or -1 if not started.
func (p *Process) PID() int {
	if p.Cmd.Process == nil {
		return -1
	}
	return p.Cmd.Process.Pid
}

// Signal sends a signal to the process.
// Returns an error if the process is not running.
func (p *Process) Signal(sig os.Signal) error {
	if !p.IsRunning() {
		return fmt.Errorf("process not running: %w", ErrProcessNotStarted)
	}

	if p.Cmd.Process == nil {
		return ErrProcessNotStarted
	}

	return p.Cmd.Process.Signal(sig)
}

// Kill sends SIGKILL to the process.
// This is equivalent to Signal(syscall.SIGKILL).
func (p *Process) Kill() error {
	return p.Signal(syscall.SIGKILL)
}

// Interrupt sends SIGINT to the process.
// This is equivalent to Signal(syscall.SIGINT).
func (p *Process) Interrupt() error {
	return p.Signal(syscall.SIGINT)
}

// Terminate sends SIGTERM to the process.
// This is equivalent to Signal(syscall.SIGTERM).
func (p *Process) Terminate() error {
	return p.Signal(syscall.SIGTERM)
}

// start starts the process and begins tracking it.
// This is called by the Supervisor.
func (p *Process) start() error {
	if p.State() != StateCreated {
		return ErrProcessAlreadyStarted
	}

	if err := p.Cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	p.Started = time.Now()
	p.state.Store(int32(StateRunning))

	// Start wait goroutine
	go p.waitLoop()

	return nil
}

// waitLoop waits for the process to exit and updates state.
func (p *Process) waitLoop() {
	p.waitOnce.Do(func() {
		err := p.Cmd.Wait()

		p.mu.Lock()
		p.exitErr = err
		p.mu.Unlock()

		// Determine exit code and state
		exitCode := 0
		state := StateExited

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				// Check if killed by signal
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() {
						state = StateKilled
					}
				}
			} else {
				// Some other error, treat as exit code -1
				exitCode = -1
			}
		}

		p.exitCode.Store(int32(exitCode))
		p.state.Store(int32(state))
		close(p.done)
	})
}

// Close closes all I/O handles associated with the process.
// This does not kill the process.
func (p *Process) Close() error {
	var errs []error

	if p.Stdin != nil {
		if err := p.Stdin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close stdin: %w", err))
		}
	}

	if p.Stdout != nil {
		if err := p.Stdout.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close stdout: %w", err))
		}
	}

	if p.Stderr != nil {
		if err := p.Stderr.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close stderr: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close process I/O: %v", errs)
	}

	return nil
}

// Runtime returns the duration the process has been running.
// If the process has exited, returns the total runtime.
func (p *Process) Runtime() time.Duration {
	if p.Started.IsZero() {
		return 0
	}
	return time.Since(p.Started)
}

// Sentinel errors for process package.
var (
	// ErrProcessNotStarted is returned when operations require a started process.
	ErrProcessNotStarted = fmt.Errorf("process not started")

	// ErrProcessAlreadyStarted is returned when trying to start an already running process.
	ErrProcessAlreadyStarted = fmt.Errorf("process already started")
)
