package security

import (
	"net"
	"path/filepath"
	"strings"
	"sync"
)

// PermissionChecker validates permissions for plugin operations.
type PermissionChecker struct {
	mu sync.RWMutex

	// Granted capabilities
	capabilities map[Capability]bool

	// File system restrictions (normalized absolute paths)
	allowedPaths  []string
	blockedPaths  []string
	workspacePath string

	// Network restrictions (lowercased)
	allowedHosts []string
	blockedHosts []string

	// Plugin identity
	pluginName string
}

// NewPermissionChecker creates a new permission checker.
func NewPermissionChecker(pluginName string) *PermissionChecker {
	return &PermissionChecker{
		capabilities: make(map[Capability]bool),
		pluginName:   pluginName,
	}
}

// Grant grants a capability to the plugin.
func (pc *PermissionChecker) Grant(cap Capability) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.capabilities[cap] = true
}

// Revoke revokes a capability from the plugin.
func (pc *PermissionChecker) Revoke(cap Capability) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.capabilities, cap)
}

// GrantAll grants multiple capabilities.
func (pc *PermissionChecker) GrantAll(caps []Capability) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	for _, cap := range caps {
		pc.capabilities[cap] = true
	}
}

// HasCapability returns true if the capability is granted.
func (pc *PermissionChecker) HasCapability(cap Capability) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// Direct check
	if pc.capabilities[cap] {
		return true
	}

	// Check if any granted capability implies this one
	for granted := range pc.capabilities {
		if ImpliesCapability(granted, cap) {
			return true
		}
	}

	return false
}

// CheckCapability returns an error if the capability is not granted.
func (pc *PermissionChecker) CheckCapability(cap Capability) error {
	if !pc.HasCapability(cap) {
		return NewCapabilityError(cap, "", "not granted")
	}
	return nil
}

// Capabilities returns all granted capabilities.
func (pc *PermissionChecker) Capabilities() []Capability {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	caps := make([]Capability, 0, len(pc.capabilities))
	for cap := range pc.capabilities {
		caps = append(caps, cap)
	}
	return caps
}

// SetWorkspacePath sets the workspace root path for file access checks.
// The path is normalized to an absolute path.
func (pc *PermissionChecker) SetWorkspacePath(path string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.workspacePath = normalizePath(path)
}

// AllowPath adds a path to the allowed list.
// The path is normalized to an absolute path.
func (pc *PermissionChecker) AllowPath(path string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.allowedPaths = append(pc.allowedPaths, normalizePath(path))
}

// BlockPath adds a path to the blocked list.
// The path is normalized to an absolute path.
func (pc *PermissionChecker) BlockPath(path string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.blockedPaths = append(pc.blockedPaths, normalizePath(path))
}

// normalizePath returns an absolute, clean path.
func normalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

// CheckFileRead checks if reading a file is permitted.
func (pc *PermissionChecker) CheckFileRead(path string) error {
	if !pc.HasCapability(CapabilityFileRead) {
		return NewCapabilityError(CapabilityFileRead, "read file", "not granted")
	}

	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.checkPathAccess(path, "read")
}

// CheckFileWrite checks if writing a file is permitted.
func (pc *PermissionChecker) CheckFileWrite(path string) error {
	if !pc.HasCapability(CapabilityFileWrite) {
		return NewCapabilityError(CapabilityFileWrite, "write file", "not granted")
	}

	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.checkPathAccess(path, "write")
}

// checkPathAccess validates path access against allowed/blocked lists.
// Uses filepath.Rel to properly check if target is within a base path.
func (pc *PermissionChecker) checkPathAccess(path, operation string) error {
	// Normalize the target path
	absPath := normalizePath(path)

	// Check blocked paths first (blocklist takes precedence)
	for _, blocked := range pc.blockedPaths {
		if isWithinPath(absPath, blocked) {
			return NewCapabilityError(CapabilityFileRead, operation, "path is blocked")
		}
	}

	// If allowed paths are specified, path must match one of them
	if len(pc.allowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range pc.allowedPaths {
			if isWithinPath(absPath, allowedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return NewCapabilityError(CapabilityFileRead, operation, "path not in allowed list")
		}
	}

	// If workspace is set, path must be within workspace (unless explicitly allowed)
	if pc.workspacePath != "" && len(pc.allowedPaths) == 0 {
		if !isWithinPath(absPath, pc.workspacePath) {
			return NewCapabilityError(CapabilityFileRead, operation, "path outside workspace")
		}
	}

	return nil
}

// isWithinPath checks if target is within or equal to base using filepath.Rel.
// This properly handles edge cases like "/tmp/blocked" not matching "/tmp/blockedfile".
func isWithinPath(target, base string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	// If the relative path doesn't start with "..", it's within or equal to base
	// Also check it's not absolute (shouldn't happen with valid inputs)
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// AllowHost adds a host to the allowed network list.
// The host is normalized to lowercase.
func (pc *PermissionChecker) AllowHost(host string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.allowedHosts = append(pc.allowedHosts, strings.ToLower(host))
}

// BlockHost adds a host to the blocked network list.
// The host is normalized to lowercase.
func (pc *PermissionChecker) BlockHost(host string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.blockedHosts = append(pc.blockedHosts, strings.ToLower(host))
}

// CheckNetwork checks if network access to a host is permitted.
func (pc *PermissionChecker) CheckNetwork(host string) error {
	if !pc.HasCapability(CapabilityNetwork) {
		return NewCapabilityError(CapabilityNetwork, "network request", "not granted")
	}

	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// Extract host from host:port, handling IPv6 addresses properly
	hostOnly := extractHost(host)
	hostOnly = strings.ToLower(hostOnly)

	// Check blocked hosts first
	for _, blocked := range pc.blockedHosts {
		if matchHost(hostOnly, blocked) {
			return NewCapabilityError(CapabilityNetwork, "network request", "host is blocked")
		}
	}

	// If allowed hosts are specified, host must match one
	if len(pc.allowedHosts) > 0 {
		allowed := false
		for _, allowedHost := range pc.allowedHosts {
			if matchHost(hostOnly, allowedHost) {
				allowed = true
				break
			}
		}
		if !allowed {
			return NewCapabilityError(CapabilityNetwork, "network request", "host not in allowed list")
		}
	}

	return nil
}

// extractHost extracts the host from a host:port string.
// Handles IPv6 addresses like [::1]:8080 and regular host:port.
func extractHost(hostPort string) string {
	// Try to split host:port using net.SplitHostPort
	host, _, err := net.SplitHostPort(hostPort)
	if err == nil {
		return host
	}

	// If SplitHostPort fails, the input might be just a host without port
	// Handle bracketed IPv6 addresses without ports: [::1]
	if strings.HasPrefix(hostPort, "[") && strings.HasSuffix(hostPort, "]") {
		return hostPort[1 : len(hostPort)-1]
	}

	// Return as-is (plain host without port)
	return hostPort
}

// CheckShell checks if shell command execution is permitted.
func (pc *PermissionChecker) CheckShell(command string) error {
	if !pc.HasCapability(CapabilityShell) {
		return NewCapabilityError(CapabilityShell, "shell command", "not granted")
	}
	return nil
}

// CheckProcess checks if process spawning is permitted.
func (pc *PermissionChecker) CheckProcess(executable string) error {
	if !pc.HasCapability(CapabilityProcess) {
		return NewCapabilityError(CapabilityProcess, "spawn process", "not granted")
	}
	return nil
}

// CheckClipboard checks if clipboard access is permitted.
func (pc *PermissionChecker) CheckClipboard(operation string) error {
	if !pc.HasCapability(CapabilityClipboard) {
		return NewCapabilityError(CapabilityClipboard, operation, "not granted")
	}
	return nil
}

// matchHost checks if a host matches a pattern (case-insensitive).
// Supports wildcard matching (e.g., "*.example.com").
func matchHost(host, pattern string) bool {
	// Both should already be lowercase, but ensure it
	host = strings.ToLower(host)
	pattern = strings.ToLower(pattern)

	// Exact match
	if host == pattern {
		return true
	}

	// Wildcard match
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // Remove *
		return strings.HasSuffix(host, suffix)
	}

	return false
}

// PermissionSet represents a collection of permissions for a plugin.
type PermissionSet struct {
	// Capabilities granted
	Capabilities []Capability

	// File system permissions
	AllowedPaths []string
	BlockedPaths []string

	// Network permissions
	AllowedHosts []string
	BlockedHosts []string
}

// ApplyPermissionSet applies a permission set to a checker.
func (pc *PermissionChecker) ApplyPermissionSet(set *PermissionSet) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Grant capabilities
	for _, cap := range set.Capabilities {
		pc.capabilities[cap] = true
	}

	// Apply file system restrictions (normalize paths)
	for _, path := range set.AllowedPaths {
		pc.allowedPaths = append(pc.allowedPaths, normalizePath(path))
	}
	for _, path := range set.BlockedPaths {
		pc.blockedPaths = append(pc.blockedPaths, normalizePath(path))
	}

	// Apply network restrictions (normalize hosts)
	for _, host := range set.AllowedHosts {
		pc.allowedHosts = append(pc.allowedHosts, strings.ToLower(host))
	}
	for _, host := range set.BlockedHosts {
		pc.blockedHosts = append(pc.blockedHosts, strings.ToLower(host))
	}
}

// Reset clears all permissions.
func (pc *PermissionChecker) Reset() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.capabilities = make(map[Capability]bool)
	pc.allowedPaths = nil
	pc.blockedPaths = nil
	pc.allowedHosts = nil
	pc.blockedHosts = nil
}
