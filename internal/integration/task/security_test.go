package task

import (
	"testing"
)

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	if len(config.TrustedSources) == 0 {
		t.Error("default config should have trusted sources")
	}
	if len(config.BlockedCommands) == 0 {
		t.Error("default config should have blocked commands")
	}
	if len(config.BlockedPatterns) == 0 {
		t.Error("default config should have blocked patterns")
	}
	if config.MaxCommandLength <= 0 {
		t.Error("default config should have max command length")
	}
}

func TestSecurityValidator_BlockedCommands(t *testing.T) {
	config := DefaultSecurityConfig()
	validator := NewSecurityValidator(config)

	tests := []struct {
		name    string
		command string
		blocked bool
	}{
		{"rm command", "rm", true},
		{"sudo command", "sudo", true},
		{"shutdown", "shutdown", true},
		{"curl", "curl", true},
		{"wget", "wget", true},
		{"go command", "go", false},
		{"make command", "make", false},
		{"npm command", "npm", false},
		{"python command", "python", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Name:    "test",
				Command: tt.command,
				Source:  "test",
			}

			result := validator.Validate(task)
			hasBlockedViolation := false
			for _, v := range result.Violations {
				if v.Code == ViolationBlockedCommand {
					hasBlockedViolation = true
					break
				}
			}

			if tt.blocked && !hasBlockedViolation {
				t.Errorf("expected command %q to be blocked", tt.command)
			}
			if !tt.blocked && hasBlockedViolation {
				t.Errorf("expected command %q to be allowed", tt.command)
			}
		})
	}
}

func TestSecurityValidator_BlockedPatterns(t *testing.T) {
	config := DefaultSecurityConfig()
	validator := NewSecurityValidator(config)

	tests := []struct {
		name    string
		command string
		args    []string
		blocked bool
	}{
		{
			name:    "rm -rf /",
			command: "rm",
			args:    []string{"-rf", "/"},
			blocked: true,
		},
		{
			name:    "rm recursive wildcard",
			command: "rm",
			args:    []string{"-rf", "*"},
			blocked: true,
		},
		{
			name:    "command substitution backticks",
			command: "echo",
			args:    []string{"`whoami`"},
			blocked: true,
		},
		{
			name:    "command substitution $()",
			command: "echo",
			args:    []string{"$(whoami)"},
			blocked: true,
		},
		{
			name:    "redirect to /etc",
			command: "echo",
			args:    []string{"foo", ">", "/etc/passwd"},
			blocked: true,
		},
		{
			name:    "base64 decode",
			command: "base64",
			args:    []string{"-d"},
			blocked: true,
		},
		{
			name:    "normal echo",
			command: "echo",
			args:    []string{"hello", "world"},
			blocked: false,
		},
		{
			name:    "go build",
			command: "go",
			args:    []string{"build", "./..."},
			blocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Name:    "test",
				Command: tt.command,
				Args:    tt.args,
				Source:  "test",
			}

			result := validator.Validate(task)
			hasPatternViolation := false
			for _, v := range result.Violations {
				if v.Code == ViolationBlockedPattern {
					hasPatternViolation = true
					break
				}
			}

			if tt.blocked && !hasPatternViolation {
				t.Errorf("expected command to be blocked by pattern")
			}
			if !tt.blocked && hasPatternViolation {
				t.Errorf("expected command to be allowed")
			}
		})
	}
}

func TestSecurityValidator_CommandLength(t *testing.T) {
	config := DefaultSecurityConfig()
	config.MaxCommandLength = 100
	validator := NewSecurityValidator(config)

	// Short command should pass
	shortTask := &Task{
		Name:    "test",
		Command: "echo",
		Args:    []string{"hello"},
		Source:  "user",
	}
	result := validator.Validate(shortTask)
	if !result.Valid {
		t.Error("short command should be valid")
	}

	// Long command should fail
	longTask := &Task{
		Name:    "test",
		Command: "echo " + string(make([]byte, 200)),
		Source:  "user",
	}
	result = validator.Validate(longTask)
	hasLengthViolation := false
	for _, v := range result.Violations {
		if v.Code == ViolationCommandTooLong {
			hasLengthViolation = true
			break
		}
	}
	if !hasLengthViolation {
		t.Error("long command should have length violation")
	}
}

func TestSecurityValidator_EmptyCommand(t *testing.T) {
	validator := NewSecurityValidator(DefaultSecurityConfig())

	task := &Task{
		Name:    "test",
		Command: "",
		Source:  "user",
	}

	result := validator.Validate(task)
	if result.Valid {
		t.Error("empty command should not be valid")
	}

	hasEmptyViolation := false
	for _, v := range result.Violations {
		if v.Code == ViolationEmptyCommand {
			hasEmptyViolation = true
			break
		}
	}
	if !hasEmptyViolation {
		t.Error("empty command should have empty command violation")
	}
}

func TestSecurityValidator_TrustedSources(t *testing.T) {
	config := DefaultSecurityConfig()
	config.TrustedSources = []string{"user", "builtin"}
	validator := NewSecurityValidator(config)

	// Trusted source should not require confirmation
	trustedTask := &Task{
		Name:    "test",
		Command: "echo",
		Args:    []string{"hello"},
		Source:  "user",
	}
	result := validator.Validate(trustedTask)
	if result.RequiresConfirmation {
		t.Error("trusted source should not require confirmation")
	}

	// Untrusted source should require confirmation
	untrustedTask := &Task{
		Name:    "test",
		Command: "echo",
		Args:    []string{"hello"},
		Source:  "npm",
	}
	result = validator.Validate(untrustedTask)
	if !result.RequiresConfirmation {
		t.Error("untrusted source should require confirmation")
	}
}

func TestSecurityValidator_IsTrustedSource(t *testing.T) {
	config := DefaultSecurityConfig()
	config.TrustedSources = []string{"user", "builtin"}
	validator := NewSecurityValidator(config)

	if !validator.IsTrustedSource("user") {
		t.Error("user should be trusted")
	}
	if !validator.IsTrustedSource("builtin") {
		t.Error("builtin should be trusted")
	}
	if validator.IsTrustedSource("npm") {
		t.Error("npm should not be trusted by default")
	}
}

func TestSecurityValidator_AddRemoveTrustedSource(t *testing.T) {
	validator := NewSecurityValidator(DefaultSecurityConfig())

	// Add trusted source
	validator.AddTrustedSource("makefile")
	if !validator.IsTrustedSource("makefile") {
		t.Error("makefile should be trusted after adding")
	}

	// Adding again should not duplicate
	validator.AddTrustedSource("makefile")

	// Remove trusted source
	validator.RemoveTrustedSource("makefile")
	if validator.IsTrustedSource("makefile") {
		t.Error("makefile should not be trusted after removal")
	}
}

func TestSecurityValidator_AllowedCommands(t *testing.T) {
	config := DefaultSecurityConfig()
	config.AllowedCommands = []string{"go", "make", "npm"}
	validator := NewSecurityValidator(config)

	// Allowed command should pass
	allowedTask := &Task{
		Name:    "test",
		Command: "go",
		Args:    []string{"build"},
		Source:  "user",
	}
	result := validator.Validate(allowedTask)
	hasNotWhitelisted := false
	for _, v := range result.Violations {
		if v.Code == ViolationNotWhitelisted {
			hasNotWhitelisted = true
			break
		}
	}
	if hasNotWhitelisted {
		t.Error("allowed command should not have whitelist violation")
	}

	// Not allowed command should fail
	notAllowedTask := &Task{
		Name:    "test",
		Command: "python",
		Args:    []string{"script.py"},
		Source:  "user",
	}
	result = validator.Validate(notAllowedTask)
	hasNotWhitelisted = false
	for _, v := range result.Violations {
		if v.Code == ViolationNotWhitelisted {
			hasNotWhitelisted = true
			break
		}
	}
	if !hasNotWhitelisted {
		t.Error("not allowed command should have whitelist violation")
	}
}

func TestSecurityValidator_RestrictedWorkDir(t *testing.T) {
	config := DefaultSecurityConfig()
	config.RestrictWorkingDir = true
	config.WorkspaceRoot = "/home/user/project"
	validator := NewSecurityValidator(config)

	// Working dir inside workspace should pass
	insideTask := &Task{
		Name:    "test",
		Command: "make",
		Cwd:     "/home/user/project/src",
		Source:  "user",
	}
	result := validator.Validate(insideTask)
	hasRestrictedViolation := false
	for _, v := range result.Violations {
		if v.Code == ViolationRestrictedWorkDir {
			hasRestrictedViolation = true
			break
		}
	}
	if hasRestrictedViolation {
		t.Error("working dir inside workspace should not have restriction violation")
	}

	// Working dir outside workspace should fail
	outsideTask := &Task{
		Name:    "test",
		Command: "make",
		Cwd:     "/etc",
		Source:  "user",
	}
	result = validator.Validate(outsideTask)
	hasRestrictedViolation = false
	for _, v := range result.Violations {
		if v.Code == ViolationRestrictedWorkDir {
			hasRestrictedViolation = true
			break
		}
	}
	if !hasRestrictedViolation {
		t.Error("working dir outside workspace should have restriction violation")
	}
}

func TestSecurityValidator_PathTraversalWarning(t *testing.T) {
	validator := NewSecurityValidator(DefaultSecurityConfig())

	task := &Task{
		Name:    "test",
		Command: "cat",
		Args:    []string{"../../etc/passwd"},
		Source:  "user",
	}

	result := validator.Validate(task)
	hasPathTraversalWarning := false
	for _, warning := range result.Warnings {
		if len(warning) > 0 {
			hasPathTraversalWarning = true
			break
		}
	}
	if !hasPathTraversalWarning {
		t.Error("path traversal should produce a warning")
	}
}

func TestSanitizeCommand(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "normal command",
			input:  "echo hello",
			expect: "echo hello",
		},
		{
			name:   "null bytes removed",
			input:  "echo\x00hello",
			expect: "echohello",
		},
		{
			name:   "backticks",
			input:  "echo `whoami`",
			expect: "echo \\`whoami\\`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeCommand(tt.input)
			if result != tt.expect {
				t.Errorf("SanitizeCommand(%q) = %q, want %q", tt.input, result, tt.expect)
			}
		})
	}
}

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		path   string
		base   string
		within bool
	}{
		{"/home/user/project/src", "/home/user/project", true},
		{"/home/user/project", "/home/user/project", true},
		{"/home/user/other", "/home/user/project", false},
		{"/etc/passwd", "/home/user/project", false},
		{"../../../etc", "/home/user/project", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isPathWithin(tt.path, tt.base)
			if result != tt.within {
				t.Errorf("isPathWithin(%q, %q) = %v, want %v", tt.path, tt.base, result, tt.within)
			}
		})
	}
}

func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		input    string
		contains bool
	}{
		{"hello.txt", false},
		{"../etc/passwd", true},
		{"/etc/shadow", true},
		{"~/.ssh/id_rsa", true},
		{"/dev/null", true},
		{"/proc/self/environ", true},
		{"./src/main.go", false},
		{"build/output", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsPathTraversal(tt.input)
			if result != tt.contains {
				t.Errorf("containsPathTraversal(%q) = %v, want %v", tt.input, result, tt.contains)
			}
		})
	}
}

func TestSecurityValidator_ShellTaskWarning(t *testing.T) {
	validator := NewSecurityValidator(DefaultSecurityConfig())

	// Shell task from untrusted source should have warning
	shellTask := &Task{
		Name:    "test",
		Command: "echo hello",
		Type:    TaskTypeShell,
		Source:  "npm", // untrusted
	}

	result := validator.Validate(shellTask)
	hasShellWarning := false
	for _, warning := range result.Warnings {
		if warning == "shell tasks can execute arbitrary commands" {
			hasShellWarning = true
			break
		}
	}
	if !hasShellWarning {
		t.Error("shell task from untrusted source should have warning")
	}

	// Shell task from trusted source should not have warning
	trustedShellTask := &Task{
		Name:    "test",
		Command: "echo hello",
		Type:    TaskTypeShell,
		Source:  "user", // trusted
	}

	result = validator.Validate(trustedShellTask)
	hasShellWarning = false
	for _, warning := range result.Warnings {
		if warning == "shell tasks can execute arbitrary commands" {
			hasShellWarning = true
			break
		}
	}
	if hasShellWarning {
		t.Error("shell task from trusted source should not have warning")
	}
}
