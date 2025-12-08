package task

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// SecurityConfig configures security settings for task execution.
type SecurityConfig struct {
	// TrustedSources are task sources that don't require user confirmation.
	// Default trusted sources: "user" (manually created tasks).
	TrustedSources []string

	// AllowedCommands is an optional whitelist of allowed command executables.
	// If empty, all commands are allowed (but may require confirmation).
	AllowedCommands []string

	// BlockedCommands is a blacklist of dangerous commands that are never allowed.
	// These commands are blocked regardless of source or user confirmation.
	BlockedCommands []string

	// BlockedPatterns are regex patterns for commands that should be blocked.
	BlockedPatterns []string

	// RequireConfirmation requires user confirmation for untrusted tasks.
	// This is enforced at the handler level, not the executor.
	RequireConfirmation bool

	// AllowNetworkAccess allows tasks that may access the network.
	AllowNetworkAccess bool

	// MaxCommandLength limits command string length to prevent DoS.
	MaxCommandLength int

	// RestrictWorkingDir limits task working directories to workspace.
	RestrictWorkingDir bool

	// WorkspaceRoot is the workspace root for directory restriction.
	WorkspaceRoot string
}

// DefaultSecurityConfig returns sensible security defaults.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		TrustedSources: []string{"user"},
		BlockedCommands: []string{
			// Dangerous system commands
			"rm", "rmdir", "del",
			"format", "fdisk", "mkfs",
			"dd",
			"shutdown", "reboot", "halt", "poweroff",
			// Privilege escalation
			"sudo", "su", "doas", "pkexec",
			// Reverse shells / network exfiltration
			"nc", "netcat", "ncat",
			"curl", "wget", // Can be used for data exfiltration
			// Credential theft
			"passwd", "shadow",
		},
		BlockedPatterns: []string{
			// Prevent recursive deletion
			`rm\s+(-[rRf]+\s+)*[/~]`,
			`rm\s+-[rRf]*\s+\*`,
			// Prevent shell injection via backticks or $()
			"`[^`]+`",
			`\$\([^)]+\)`,
			// Prevent output redirection to sensitive files
			`>\s*/etc/`,
			`>\s*/dev/`,
			// Prevent base64 decode (common obfuscation)
			`base64\s+-d`,
			`base64\s+--decode`,
		},
		RequireConfirmation: true,
		AllowNetworkAccess:  false,
		MaxCommandLength:    4096,
		RestrictWorkingDir:  true,
	}
}

// SecurityValidator validates tasks before execution.
type SecurityValidator struct {
	config SecurityConfig

	// Compiled patterns for efficiency
	blockedPatterns []*regexp.Regexp
	mu              sync.RWMutex
}

// NewSecurityValidator creates a new security validator.
func NewSecurityValidator(config SecurityConfig) *SecurityValidator {
	sv := &SecurityValidator{
		config: config,
	}
	sv.compilePatterns()
	return sv
}

// compilePatterns pre-compiles blocked patterns.
func (sv *SecurityValidator) compilePatterns() {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	sv.blockedPatterns = make([]*regexp.Regexp, 0, len(sv.config.BlockedPatterns))
	for _, pattern := range sv.config.BlockedPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			sv.blockedPatterns = append(sv.blockedPatterns, re)
		}
	}
}

// UpdateConfig updates the security configuration.
func (sv *SecurityValidator) UpdateConfig(config SecurityConfig) {
	sv.mu.Lock()
	sv.config = config
	sv.mu.Unlock()
	sv.compilePatterns()
}

// SecurityViolation represents a security check failure.
type SecurityViolation struct {
	// Code is the violation type.
	Code SecurityViolationCode

	// Message describes the violation.
	Message string

	// Details provides additional context.
	Details string

	// Task is the task that violated security policy.
	Task *Task
}

func (v *SecurityViolation) Error() string {
	if v.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", v.Code, v.Message, v.Details)
	}
	return fmt.Sprintf("%s: %s", v.Code, v.Message)
}

// SecurityViolationCode identifies the type of security violation.
type SecurityViolationCode string

const (
	ViolationBlockedCommand    SecurityViolationCode = "blocked_command"
	ViolationBlockedPattern    SecurityViolationCode = "blocked_pattern"
	ViolationCommandTooLong    SecurityViolationCode = "command_too_long"
	ViolationPathTraversal     SecurityViolationCode = "path_traversal"
	ViolationUntrustedSource   SecurityViolationCode = "untrusted_source"
	ViolationEmptyCommand      SecurityViolationCode = "empty_command"
	ViolationNotWhitelisted    SecurityViolationCode = "not_whitelisted"
	ViolationRestrictedWorkDir SecurityViolationCode = "restricted_workdir"
)

// ValidationResult contains the result of security validation.
type ValidationResult struct {
	// Valid indicates if the task passed security validation.
	Valid bool

	// RequiresConfirmation indicates the task needs user confirmation.
	RequiresConfirmation bool

	// Violations are the security violations found.
	Violations []*SecurityViolation

	// Warnings are non-blocking security concerns.
	Warnings []string
}

// Validate checks a task against security policies.
// Returns a ValidationResult with any violations or warnings.
func (sv *SecurityValidator) Validate(task *Task) *ValidationResult {
	sv.mu.RLock()
	config := sv.config
	patterns := sv.blockedPatterns
	sv.mu.RUnlock()

	result := &ValidationResult{
		Valid:      true,
		Violations: make([]*SecurityViolation, 0),
		Warnings:   make([]string, 0),
	}

	// Check empty command
	if task.Command == "" {
		result.Valid = false
		result.Violations = append(result.Violations, &SecurityViolation{
			Code:    ViolationEmptyCommand,
			Message: "task has no command",
			Task:    task,
		})
		return result
	}

	// Check command length
	fullCommand := task.Command
	for _, arg := range task.Args {
		fullCommand += " " + arg
	}
	if config.MaxCommandLength > 0 && len(fullCommand) > config.MaxCommandLength {
		result.Valid = false
		result.Violations = append(result.Violations, &SecurityViolation{
			Code:    ViolationCommandTooLong,
			Message: "command exceeds maximum length",
			Details: fmt.Sprintf("length=%d, max=%d", len(fullCommand), config.MaxCommandLength),
			Task:    task,
		})
	}

	// Extract base command (without path)
	baseCommand := filepath.Base(task.Command)
	// Handle cases where command includes arguments
	if idx := strings.Index(baseCommand, " "); idx > 0 {
		baseCommand = baseCommand[:idx]
	}

	// Check blocked commands
	for _, blocked := range config.BlockedCommands {
		if strings.EqualFold(baseCommand, blocked) {
			result.Valid = false
			result.Violations = append(result.Violations, &SecurityViolation{
				Code:    ViolationBlockedCommand,
				Message: fmt.Sprintf("command %q is blocked", blocked),
				Details: "this command is on the security blocklist",
				Task:    task,
			})
		}
	}

	// Check blocked patterns
	for _, re := range patterns {
		if re.MatchString(fullCommand) {
			result.Valid = false
			result.Violations = append(result.Violations, &SecurityViolation{
				Code:    ViolationBlockedPattern,
				Message: "command matches blocked pattern",
				Details: re.String(),
				Task:    task,
			})
		}
	}

	// Check command whitelist (if configured)
	if len(config.AllowedCommands) > 0 {
		allowed := false
		for _, cmd := range config.AllowedCommands {
			if strings.EqualFold(baseCommand, cmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			result.Valid = false
			result.Violations = append(result.Violations, &SecurityViolation{
				Code:    ViolationNotWhitelisted,
				Message: fmt.Sprintf("command %q is not in the allowed list", baseCommand),
				Task:    task,
			})
		}
	}

	// Check working directory restriction
	if config.RestrictWorkingDir && config.WorkspaceRoot != "" && task.Cwd != "" {
		if !isPathWithin(task.Cwd, config.WorkspaceRoot) {
			result.Valid = false
			result.Violations = append(result.Violations, &SecurityViolation{
				Code:    ViolationRestrictedWorkDir,
				Message: "working directory outside workspace",
				Details: fmt.Sprintf("cwd=%q, workspace=%q", task.Cwd, config.WorkspaceRoot),
				Task:    task,
			})
		}
	}

	// Check for path traversal in arguments
	for _, arg := range task.Args {
		if containsPathTraversal(arg) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("argument contains path traversal: %s", truncate(arg, 50)))
		}
	}

	// Check if source is trusted
	isTrusted := false
	for _, trusted := range config.TrustedSources {
		if task.Source == trusted {
			isTrusted = true
			break
		}
	}

	// Determine if confirmation is required
	if !isTrusted && config.RequireConfirmation {
		result.RequiresConfirmation = true
	}

	// Add warning for shell tasks (higher risk)
	if task.Type == TaskTypeShell && !isTrusted {
		result.Warnings = append(result.Warnings,
			"shell tasks can execute arbitrary commands")
	}

	return result
}

// IsTrustedSource checks if a task source is trusted.
func (sv *SecurityValidator) IsTrustedSource(source string) bool {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	for _, trusted := range sv.config.TrustedSources {
		if source == trusted {
			return true
		}
	}
	return false
}

// AddTrustedSource adds a source to the trusted list.
func (sv *SecurityValidator) AddTrustedSource(source string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	for _, existing := range sv.config.TrustedSources {
		if existing == source {
			return
		}
	}
	sv.config.TrustedSources = append(sv.config.TrustedSources, source)
}

// RemoveTrustedSource removes a source from the trusted list.
func (sv *SecurityValidator) RemoveTrustedSource(source string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	for i, existing := range sv.config.TrustedSources {
		if existing == source {
			sv.config.TrustedSources = append(
				sv.config.TrustedSources[:i],
				sv.config.TrustedSources[i+1:]...,
			)
			return
		}
	}
}

// SanitizeCommand sanitizes a command string for safer execution.
// This removes or escapes potentially dangerous constructs.
func SanitizeCommand(cmd string) string {
	// Remove null bytes
	cmd = strings.ReplaceAll(cmd, "\x00", "")

	// Escape backticks (command substitution)
	cmd = strings.ReplaceAll(cmd, "`", "\\`")

	// Remove ANSI escape sequences
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	cmd = ansiPattern.ReplaceAllString(cmd, "")

	return cmd
}

// isPathWithin checks if path is within the base directory.
func isPathWithin(path, base string) bool {
	// Clean and resolve paths
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return false
	}

	// Check if path starts with base
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}

	// If relative path starts with .., it's outside the base
	return !strings.HasPrefix(rel, "..")
}

// containsPathTraversal checks if a string contains path traversal attempts.
func containsPathTraversal(s string) bool {
	// Check for various path traversal patterns
	patterns := []string{
		"..",
		"/etc/",
		"/dev/",
		"/proc/",
		"/sys/",
		"~/.ssh",
		"~/.gnupg",
		"~/.aws",
	}

	lower := strings.ToLower(s)
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// truncate truncates a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
