// Package debug provides debugger integration for Keystorm.
//
// This package implements a Debug Adapter Protocol (DAP) client that enables
// Keystorm to debug programs written in any language that has a DAP-compatible
// debug adapter (Go/Delve, Python/debugpy, JavaScript/node-debug, etc.).
//
// # Architecture
//
// The debug system is organized around sessions and adapters:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                      Debug Session                               │
//	│  - Manages connection to debug adapter                          │
//	│  - Tracks breakpoints, stack frames, variables                  │
//	│  - Provides execution control (step, continue, pause)           │
//	└─────────────────────────────────────────────────────────────────┘
//	                              │
//	                              ▼
//	┌─────────────────────────────────────────────────────────────────┐
//	│                      Debug Adapter                               │
//	│  - DAP protocol implementation                                  │
//	│  - Language-specific adapter process                            │
//	│  - Delve, debugpy, node-debug, etc.                             │
//	└─────────────────────────────────────────────────────────────────┘
//
// # Session States
//
// Debug sessions transition through the following states:
//
//   - Initializing: Adapter is being started
//   - Running: Program is executing
//   - Stopped: Program is paused (breakpoint, step, exception)
//   - Terminated: Debug session has ended
//
// # Breakpoints
//
// The package supports several breakpoint types:
//
//   - Line breakpoints: Stop at a specific line
//   - Conditional breakpoints: Stop when condition is true
//   - Logpoints: Log message without stopping
//   - Function breakpoints: Stop when entering a function
//
// # Variables and Evaluation
//
// When stopped, you can inspect:
//
//   - Local variables in current scope
//   - Global variables
//   - Watch expressions
//   - Arbitrary expression evaluation
//
// # Usage
//
// Create and start a debug session:
//
//	session := debug.NewSession(debug.SessionConfig{
//	    Adapter:     "delve",
//	    Program:     "./cmd/myapp",
//	    Args:        []string{"-v"},
//	    StopOnEntry: true,
//	})
//
//	if err := session.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer session.Stop()
//
//	// Set a breakpoint
//	bp, err := session.SetBreakpoint("main.go", 42)
//
//	// Continue execution
//	session.Continue()
//
//	// When stopped, inspect variables
//	vars, _ := session.Variables()
//	stack, _ := session.StackTrace()
//
// # Subpackages
//
//   - adapters: Debug adapter implementations (Delve, generic DAP)
//   - dap: Debug Adapter Protocol types and client
package debug
