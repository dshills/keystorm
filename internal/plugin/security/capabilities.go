// Package security provides security primitives for the plugin system.
package security

import (
	"fmt"
	"strings"
)

// Capability represents a permission that a plugin can request.
// Capabilities are hierarchical - granting a parent capability
// implicitly grants all child capabilities.
type Capability string

// Core capabilities that plugins can request.
const (
	// CapabilityFileRead allows reading files from the filesystem.
	CapabilityFileRead Capability = "filesystem.read"

	// CapabilityFileWrite allows writing files to the filesystem.
	CapabilityFileWrite Capability = "filesystem.write"

	// CapabilityNetwork allows network access (HTTP, sockets, etc.).
	CapabilityNetwork Capability = "network"

	// CapabilityShell allows executing shell commands.
	CapabilityShell Capability = "shell"

	// CapabilityClipboard allows clipboard access.
	CapabilityClipboard Capability = "clipboard"

	// CapabilityProcess allows spawning child processes.
	CapabilityProcess Capability = "process.spawn"

	// CapabilityUnsafe grants full Lua stdlib access (debug, io, os).
	// This is a dangerous capability and should be granted sparingly.
	CapabilityUnsafe Capability = "unsafe"

	// CapabilityEditor grants access to editor internals.
	CapabilityEditor Capability = "editor"

	// CapabilityBuffer grants buffer manipulation access.
	CapabilityBuffer Capability = "editor.buffer"

	// CapabilityCursor grants cursor manipulation access.
	CapabilityCursor Capability = "editor.cursor"

	// CapabilityKeymap grants keymap modification access.
	CapabilityKeymap Capability = "editor.keymap"

	// CapabilityCommand grants command registration access.
	CapabilityCommand Capability = "editor.command"

	// CapabilityUI grants UI access (notifications, statusline, etc.).
	CapabilityUI Capability = "editor.ui"

	// CapabilityConfig grants configuration access.
	CapabilityConfig Capability = "editor.config"

	// CapabilityEvent grants event subscription access.
	CapabilityEvent Capability = "editor.event"

	// CapabilityLSP grants LSP client access.
	CapabilityLSP Capability = "editor.lsp"
)

// CapabilityInfo provides metadata about a capability.
type CapabilityInfo struct {
	// Name is the capability identifier.
	Name Capability

	// DisplayName is a human-readable name.
	DisplayName string

	// Description explains what the capability allows.
	Description string

	// Parent is the parent capability (for hierarchical capabilities).
	Parent Capability

	// RiskLevel indicates how dangerous this capability is.
	RiskLevel RiskLevel

	// RequiresUserApproval indicates if the user must explicitly approve.
	RequiresUserApproval bool
}

// RiskLevel indicates the security risk of a capability.
type RiskLevel int

const (
	// RiskLow indicates minimal security risk.
	RiskLow RiskLevel = iota

	// RiskMedium indicates moderate security risk.
	RiskMedium

	// RiskHigh indicates significant security risk.
	RiskHigh

	// RiskCritical indicates maximum security risk.
	RiskCritical
)

// String returns a string representation of the risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// capabilityRegistry holds metadata about all known capabilities.
var capabilityRegistry = map[Capability]CapabilityInfo{
	CapabilityFileRead: {
		Name:                 CapabilityFileRead,
		DisplayName:          "File Read",
		Description:          "Read files from the filesystem",
		RiskLevel:            RiskMedium,
		RequiresUserApproval: false,
	},
	CapabilityFileWrite: {
		Name:                 CapabilityFileWrite,
		DisplayName:          "File Write",
		Description:          "Write files to the filesystem",
		RiskLevel:            RiskHigh,
		RequiresUserApproval: true,
	},
	CapabilityNetwork: {
		Name:                 CapabilityNetwork,
		DisplayName:          "Network Access",
		Description:          "Make network requests",
		RiskLevel:            RiskHigh,
		RequiresUserApproval: true,
	},
	CapabilityShell: {
		Name:                 CapabilityShell,
		DisplayName:          "Shell Access",
		Description:          "Execute shell commands",
		RiskLevel:            RiskCritical,
		RequiresUserApproval: true,
	},
	CapabilityClipboard: {
		Name:                 CapabilityClipboard,
		DisplayName:          "Clipboard Access",
		Description:          "Read and write clipboard",
		RiskLevel:            RiskMedium,
		RequiresUserApproval: false,
	},
	CapabilityProcess: {
		Name:                 CapabilityProcess,
		DisplayName:          "Process Spawn",
		Description:          "Spawn child processes",
		RiskLevel:            RiskCritical,
		RequiresUserApproval: true,
	},
	CapabilityUnsafe: {
		Name:                 CapabilityUnsafe,
		DisplayName:          "Unsafe Mode",
		Description:          "Full Lua stdlib access (dangerous)",
		RiskLevel:            RiskCritical,
		RequiresUserApproval: true,
	},
	CapabilityEditor: {
		Name:                 CapabilityEditor,
		DisplayName:          "Editor Access",
		Description:          "Access editor internals",
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityBuffer: {
		Name:                 CapabilityBuffer,
		DisplayName:          "Buffer Access",
		Description:          "Read and modify buffers",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityCursor: {
		Name:                 CapabilityCursor,
		DisplayName:          "Cursor Access",
		Description:          "Control cursor position",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityKeymap: {
		Name:                 CapabilityKeymap,
		DisplayName:          "Keymap Access",
		Description:          "Register keybindings",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityCommand: {
		Name:                 CapabilityCommand,
		DisplayName:          "Command Access",
		Description:          "Register commands",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityUI: {
		Name:                 CapabilityUI,
		DisplayName:          "UI Access",
		Description:          "Show notifications and UI elements",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityConfig: {
		Name:                 CapabilityConfig,
		DisplayName:          "Config Access",
		Description:          "Read and write configuration",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityEvent: {
		Name:                 CapabilityEvent,
		DisplayName:          "Event Access",
		Description:          "Subscribe to editor events",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
	CapabilityLSP: {
		Name:                 CapabilityLSP,
		DisplayName:          "LSP Access",
		Description:          "Access LSP client",
		Parent:               CapabilityEditor,
		RiskLevel:            RiskLow,
		RequiresUserApproval: false,
	},
}

// GetCapabilityInfo returns information about a capability.
func GetCapabilityInfo(cap Capability) (CapabilityInfo, bool) {
	info, ok := capabilityRegistry[cap]
	return info, ok
}

// IsValidCapability returns true if the capability is known.
func IsValidCapability(cap Capability) bool {
	_, ok := capabilityRegistry[cap]
	return ok
}

// AllCapabilities returns all known capabilities.
func AllCapabilities() []Capability {
	caps := make([]Capability, 0, len(capabilityRegistry))
	for cap := range capabilityRegistry {
		caps = append(caps, cap)
	}
	return caps
}

// HighRiskCapabilities returns capabilities that require user approval.
func HighRiskCapabilities() []Capability {
	var caps []Capability
	for cap, info := range capabilityRegistry {
		if info.RequiresUserApproval {
			caps = append(caps, cap)
		}
	}
	return caps
}

// IsChildOf returns true if child is a child of parent.
func IsChildOf(child, parent Capability) bool {
	// Direct string prefix check for hierarchical capabilities
	return strings.HasPrefix(string(child), string(parent)+".")
}

// ImpliesCapability returns true if having 'granted' implies having 'required'.
func ImpliesCapability(granted, required Capability) bool {
	// Same capability
	if granted == required {
		return true
	}

	// Check if granted is a parent of required
	return IsChildOf(required, granted)
}

// CapabilityError represents a capability-related error.
type CapabilityError struct {
	Capability Capability
	Operation  string
	Message    string
}

// Error implements the error interface.
func (e *CapabilityError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("capability %q required for %s: %s", e.Capability, e.Operation, e.Message)
	}
	return fmt.Sprintf("capability %q: %s", e.Capability, e.Message)
}

// NewCapabilityError creates a new capability error.
func NewCapabilityError(cap Capability, operation, message string) *CapabilityError {
	return &CapabilityError{
		Capability: cap,
		Operation:  operation,
		Message:    message,
	}
}
