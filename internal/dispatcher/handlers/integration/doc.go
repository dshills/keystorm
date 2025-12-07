// Package integration provides handlers for integration layer actions.
//
// This package implements dispatcher handlers for:
//   - Git operations (git.status, git.commit, git.checkout, etc.)
//   - Task runner (task.run, task.stop, task.list)
//   - Debug operations (debug.start, debug.stop, debug.step*, etc.)
//   - Terminal operations (terminal.create, terminal.send, etc.)
//
// Handlers are namespace-based, following the same pattern as other
// dispatcher handlers (cursor, editor, mode, etc.).
//
// # Handler Registration
//
// Handlers can be registered with the dispatcher System:
//
//	sys := dispatcher.NewSystemWithDefaults()
//	gitHandler := integration.NewGitHandler(gitManager)
//	sys.RegisterNamespace("git", gitHandler)
//
// # Actions
//
// Git actions:
//   - git.status: Get repository status
//   - git.branch: Get current branch
//   - git.branches: List all branches
//   - git.checkout: Switch branches
//   - git.commit: Create a commit
//   - git.add: Stage files
//   - git.diff: Get diff output
//
// Task actions:
//   - task.list: List available tasks
//   - task.run: Run a task
//   - task.stop: Stop a running task
//   - task.status: Get task status
//
// Debug actions:
//   - debug.start: Start debug session
//   - debug.stop: Stop debug session
//   - debug.continue: Continue execution
//   - debug.stepOver: Step over
//   - debug.stepInto: Step into
//   - debug.stepOut: Step out
//   - debug.breakpoint.set: Set breakpoint
//   - debug.breakpoint.remove: Remove breakpoint
//
// # Integration Manager Interface
//
// Handlers require an IntegrationProvider interface to access
// the integration layer components. This is typically provided
// by the integration.Manager.
package integration
