// Package security provides security primitives for the plugin system.
//
// The security package implements a capability-based security model
// for controlling plugin access to sensitive resources:
//
// # Capabilities
//
// Capabilities are permissions that plugins must request in their manifest.
// The capability system is hierarchical - granting a parent capability
// (e.g., "editor") implicitly grants all child capabilities (e.g.,
// "editor.buffer", "editor.cursor").
//
// Core capability categories:
//   - filesystem.read/write: File system access
//   - network: Network requests
//   - shell: Shell command execution
//   - clipboard: Clipboard access
//   - process.spawn: Process creation
//   - unsafe: Full Lua stdlib access
//   - editor.*: Editor-specific capabilities
//
// # Permissions
//
// The PermissionChecker validates plugin operations against granted
// capabilities and optional restrictions:
//
//   - File path allowlists/blocklists
//   - Network host allowlists/blocklists
//   - Workspace boundary enforcement
//
// # Resource Limits
//
// ResourceMonitor enforces resource limits to prevent plugins from
// consuming excessive resources:
//
//   - Memory limits (advisory)
//   - Execution timeouts
//   - Instruction limits (Lua VM)
//   - Rate limiting for file/network operations
//   - Output size limits
//
// Example usage:
//
//	// Create permission checker for a plugin
//	checker := security.NewPermissionChecker("my-plugin")
//	checker.Grant(security.CapabilityFileRead)
//	checker.SetWorkspacePath("/path/to/workspace")
//
//	// Check permission before file read
//	if err := checker.CheckFileRead("/path/to/file"); err != nil {
//	    // Access denied
//	}
//
//	// Create resource monitor
//	monitor := security.NewResourceMonitor(security.DefaultResourceLimits())
//
//	// Track resource usage
//	if monitor.IncrementInstructions(1000) {
//	    // Limit exceeded
//	}
package security
