// Package integration provides external tool integration for Keystorm.
//
// The integration package transforms Keystorm from a text editor into a
// development environment by providing seamless integration with:
//
//   - Terminal: PTY-based terminal emulation with shell integration
//   - Git: Repository operations, status, commits, branches, and diffs
//   - Debugger: Debug Adapter Protocol (DAP) client for multi-language debugging
//   - Task Runner: Task discovery and execution with problem matching
//
// # Architecture
//
// The integration layer is organized around a central Manager that coordinates
// all integration components:
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│                      Integration Manager                             │
//	│  - Component lifecycle management                                    │
//	│  - Unified API facade                                                │
//	│  - Configuration and event publishing                                │
//	└─────────────────────────────────────────────────────────────────────┘
//	                              │
//	           ┌──────────────────┼──────────────────┐
//	           ▼                  ▼                  ▼
//	  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
//	  │    Terminal     │ │      Git        │ │    Debugger     │
//	  └─────────────────┘ └─────────────────┘ └─────────────────┘
//	           │                  │                  │
//	           └──────────────────┼──────────────────┘
//	                              ▼
//	  ┌─────────────────────────────────────────────────────────────────┐
//	  │                        Task Runner                               │
//	  └─────────────────────────────────────────────────────────────────┘
//	                              │
//	  ┌───────────────────────────┴───────────────────────────┐
//	  │                    Process Supervisor                  │
//	  │  - Child process management                           │
//	  │  - Signal handling and forwarding                     │
//	  │  - Resource cleanup on shutdown                       │
//	  └────────────────────────────────────────────────────────┘
//
// # Process Supervisor
//
// The process supervisor (process subpackage) manages child processes for
// terminals, debug adapters, and task execution. It provides:
//
//   - Lifecycle management with proper cleanup
//   - Signal forwarding to child processes
//   - Graceful shutdown with configurable timeout
//   - Resource tracking and limits
//
// # Thread Safety
//
// The Manager and all integration components are safe for concurrent use.
// All public methods use appropriate synchronization.
//
// # Event Publishing
//
// Integration events are published through the EventPublisher interface:
//
//   - terminal.created, terminal.closed, terminal.output
//   - git.status.changed, git.branch.changed, git.commit.created
//   - debug.session.started, debug.session.stopped, debug.breakpoint.hit
//   - task.started, task.output, task.completed
//
// # Configuration
//
// Integration settings are managed through the config system:
//
//	config.Terminal()  // Terminal settings (shell, scrollback, etc.)
//	config.Git()       // Git settings (auto-fetch, decorations, etc.)
//	config.Debug()     // Debug settings (adapters, confirm on exit, etc.)
//	config.Task()      // Task settings (auto-detect, problem matchers, etc.)
//
// # Usage
//
// Create a Manager with configuration:
//
//	manager, err := integration.NewManager(integration.ManagerConfig{
//	    WorkspaceRoot: "/path/to/workspace",
//	    EventBus:      eventPublisher,
//	    ConfigSystem:  configSystem,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer manager.Close()
//
//	// Access integration components
//	term := manager.Terminal()
//	git := manager.Git()
//	debug := manager.Debug()
//	task := manager.Task()
package integration
