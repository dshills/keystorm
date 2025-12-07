package task

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// VariableResolver handles variable substitution in task commands.
type VariableResolver struct {
	// custom holds custom variable values.
	custom map[string]string

	// providers holds dynamic variable providers.
	providers map[string]VariableProvider

	// mu protects custom and providers.
	mu sync.RWMutex
}

// VariableProvider provides dynamic variable values.
type VariableProvider func(ctx *VariableContext) string

// VariableContext provides context for variable resolution.
type VariableContext struct {
	// Task is the task being executed.
	Task *Task

	// WorkingDir is the current working directory.
	WorkingDir string

	// File is the current file (if applicable).
	File string

	// Selection is the current selection (if applicable).
	Selection string

	// Line is the current line number.
	Line int

	// Column is the current column number.
	Column int
}

// NewVariableResolver creates a new variable resolver.
func NewVariableResolver() *VariableResolver {
	vr := &VariableResolver{
		custom:    make(map[string]string),
		providers: make(map[string]VariableProvider),
	}

	// Register built-in providers
	vr.registerBuiltinProviders()

	return vr
}

// Set sets a custom variable value.
func (vr *VariableResolver) Set(name, value string) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.custom[name] = value
}

// Get returns a custom variable value.
func (vr *VariableResolver) Get(name string) (string, bool) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	v, ok := vr.custom[name]
	return v, ok
}

// Delete removes a custom variable.
func (vr *VariableResolver) Delete(name string) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	delete(vr.custom, name)
}

// RegisterProvider registers a dynamic variable provider.
func (vr *VariableResolver) RegisterProvider(name string, provider VariableProvider) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.providers[name] = provider
}

// UnregisterProvider removes a variable provider.
func (vr *VariableResolver) UnregisterProvider(name string) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	delete(vr.providers, name)
}

// Resolve replaces variables in a string.
// Supports ${var}, $var, and ${var:default} syntax.
func (vr *VariableResolver) Resolve(input string, task *Task) string {
	ctx := &VariableContext{
		Task: task,
	}
	return vr.ResolveWithContext(input, ctx)
}

// ResolveWithContext replaces variables using the provided context.
// Supports ${var}, ${var:default}, ${env:VAR}, and $var syntax.
func (vr *VariableResolver) ResolveWithContext(input string, ctx *VariableContext) string {
	// Pattern matches ${var}, ${var:default}, ${env:VAR}, and $var
	pattern := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

	return pattern.ReplaceAllStringFunc(input, func(match string) string {
		var name, defaultVal string
		hasDefault := false

		if strings.HasPrefix(match, "${") {
			// ${var} or ${var:default} or ${env:VAR} format
			inner := match[2 : len(match)-1]

			// Check for ${env:VAR} or ${env:VAR:default} syntax
			if strings.HasPrefix(inner, "env:") {
				envPart := inner[4:]
				// Split on first ':' to separate env var name from default
				var envName, envDefault string
				if idx := strings.Index(envPart, ":"); idx >= 0 {
					envName = envPart[:idx]
					envDefault = envPart[idx+1:]
				} else {
					envName = envPart
				}
				if val := os.Getenv(envName); val != "" {
					return val
				}
				return envDefault // Return default (empty if not provided)
			}

			if idx := strings.Index(inner, ":"); idx >= 0 {
				name = inner[:idx]
				defaultVal = inner[idx+1:]
				hasDefault = true
			} else {
				name = inner
			}
		} else {
			// $var format
			name = match[1:]
		}

		// Try to resolve the variable
		if value := vr.resolveVariable(name, ctx); value != "" {
			return value
		}

		// Use default if provided
		if hasDefault {
			return defaultVal
		}

		// Return original if not resolved
		return match
	})
}

func (vr *VariableResolver) resolveVariable(name string, ctx *VariableContext) string {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	// Check custom variables first
	if v, ok := vr.custom[name]; ok {
		return v
	}

	// Check providers
	if provider, ok := vr.providers[name]; ok {
		return provider(ctx)
	}

	// Check environment
	if v := os.Getenv(name); v != "" {
		return v
	}

	return ""
}

func (vr *VariableResolver) registerBuiltinProviders() {
	// workspaceFolder - the workspace root directory
	vr.providers["workspaceFolder"] = func(ctx *VariableContext) string {
		if ctx.WorkingDir != "" {
			return ctx.WorkingDir
		}
		if ctx.Task != nil && ctx.Task.Cwd != "" {
			return ctx.Task.Cwd
		}
		cwd, _ := os.Getwd()
		return cwd
	}

	// workspaceFolderBasename - the name of the workspace folder
	vr.providers["workspaceFolderBasename"] = func(ctx *VariableContext) string {
		folder := vr.providers["workspaceFolder"](ctx)
		return filepath.Base(folder)
	}

	// file - the current opened file
	vr.providers["file"] = func(ctx *VariableContext) string {
		return ctx.File
	}

	// fileWorkspaceFolder - the workspace folder of the current file
	vr.providers["fileWorkspaceFolder"] = func(ctx *VariableContext) string {
		if ctx.File == "" {
			return ""
		}
		return filepath.Dir(ctx.File)
	}

	// relativeFile - the current file relative to workspaceFolder
	vr.providers["relativeFile"] = func(ctx *VariableContext) string {
		if ctx.File == "" {
			return ""
		}
		folder := vr.providers["workspaceFolder"](ctx)
		rel, err := filepath.Rel(folder, ctx.File)
		if err != nil {
			return ctx.File
		}
		return rel
	}

	// relativeFileDirname - the directory of the current file relative to workspaceFolder
	vr.providers["relativeFileDirname"] = func(ctx *VariableContext) string {
		relFile := vr.providers["relativeFile"](ctx)
		if relFile == "" {
			return ""
		}
		return filepath.Dir(relFile)
	}

	// fileBasename - the basename of the current file
	vr.providers["fileBasename"] = func(ctx *VariableContext) string {
		if ctx.File == "" {
			return ""
		}
		return filepath.Base(ctx.File)
	}

	// fileBasenameNoExtension - the basename without extension
	vr.providers["fileBasenameNoExtension"] = func(ctx *VariableContext) string {
		base := vr.providers["fileBasename"](ctx)
		if base == "" {
			return ""
		}
		ext := filepath.Ext(base)
		return strings.TrimSuffix(base, ext)
	}

	// fileDirname - the directory of the current file
	vr.providers["fileDirname"] = func(ctx *VariableContext) string {
		if ctx.File == "" {
			return ""
		}
		return filepath.Dir(ctx.File)
	}

	// fileExtname - the extension of the current file
	vr.providers["fileExtname"] = func(ctx *VariableContext) string {
		if ctx.File == "" {
			return ""
		}
		return filepath.Ext(ctx.File)
	}

	// cwd - the current working directory
	vr.providers["cwd"] = func(ctx *VariableContext) string {
		cwd, _ := os.Getwd()
		return cwd
	}

	// lineNumber - the current line number
	vr.providers["lineNumber"] = func(ctx *VariableContext) string {
		if ctx.Line > 0 {
			return strconv.Itoa(ctx.Line)
		}
		return ""
	}

	// selectedText - the current selection
	vr.providers["selectedText"] = func(ctx *VariableContext) string {
		return ctx.Selection
	}

	// env - access environment variables via ${env:VAR_NAME}
	// This is handled specially in ResolveWithContext

	// pathSeparator - the OS path separator
	vr.providers["pathSeparator"] = func(_ *VariableContext) string {
		return string(filepath.Separator)
	}

	// execPath - the editor executable path (placeholder)
	vr.providers["execPath"] = func(_ *VariableContext) string {
		exe, _ := os.Executable()
		return exe
	}
}

// ResolveEnv resolves ${env:VAR_NAME} style variables.
func (vr *VariableResolver) ResolveEnv(input string) string {
	pattern := regexp.MustCompile(`\$\{env:([^}]+)\}`)
	return pattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name from ${env:VAR_NAME}
		name := match[6 : len(match)-1]
		return os.Getenv(name)
	})
}

// ResolveAll resolves all variable types in order.
func (vr *VariableResolver) ResolveAll(input string, ctx *VariableContext) string {
	// First resolve env variables
	result := vr.ResolveEnv(input)
	// Then resolve other variables
	return vr.ResolveWithContext(result, ctx)
}
