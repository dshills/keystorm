// Package process provides child process management for the integration layer.
//
// The process package implements a supervisor pattern for managing child
// processes spawned by terminal, debugger, and task runner components.
//
// # Features
//
//   - Process lifecycle management (start, stop, kill)
//   - Signal forwarding to child processes
//   - Graceful shutdown with configurable timeout
//   - Resource tracking and cleanup
//   - Exit code and status tracking
//
// # Supervisor
//
// The Supervisor manages multiple child processes:
//
//	supervisor := process.NewSupervisor()
//	defer supervisor.Shutdown(5 * time.Second)
//
//	// Start a process
//	cmd := exec.Command("bash", "-c", "sleep 10")
//	proc, err := supervisor.Start("my-process", cmd)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Wait for completion
//	<-proc.Done()
//	fmt.Printf("Exit code: %d\n", proc.ExitCode())
//
// # Process
//
// Each Process wraps an exec.Cmd with additional tracking:
//
//   - Unique ID for identification
//   - Start time tracking
//   - Exit code retrieval
//   - Done channel for completion notification
//   - Standard I/O access
//
// # Graceful Shutdown
//
// The supervisor supports graceful shutdown:
//
//	// Send SIGTERM, wait up to 5 seconds, then SIGKILL
//	supervisor.Shutdown(5 * time.Second)
//
// # Thread Safety
//
// Both Supervisor and Process are safe for concurrent use.
package process
