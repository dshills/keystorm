package sources

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/dshills/keystorm/internal/integration/task"
)

// PackageJSONSource discovers tasks from package.json files.
type PackageJSONSource struct{}

// NewPackageJSONSource creates a new package.json source.
func NewPackageJSONSource() *PackageJSONSource {
	return &PackageJSONSource{}
}

// Name returns the source name.
func (s *PackageJSONSource) Name() string {
	return "npm"
}

// Patterns returns the file patterns this source handles.
func (s *PackageJSONSource) Patterns() []string {
	return []string{
		"package.json",
	}
}

// Priority returns the source priority.
func (s *PackageJSONSource) Priority() int {
	return 90
}

// packageJSON represents the structure of a package.json file.
type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	DevDependencies map[string]string `json:"devDependencies"`
	Dependencies    map[string]string `json:"dependencies"`
}

// Discover finds tasks in a package.json file.
func (s *PackageJSONSource) Discover(ctx context.Context, path string) ([]*task.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	if len(pkg.Scripts) == 0 {
		return nil, nil
	}

	// Detect package manager
	packageManager := s.detectPackageManager(filepath.Dir(path))

	var tasks []*task.Task
	for name, script := range pkg.Scripts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		t := &task.Task{
			Name:        name,
			Description: s.generateDescription(name, script),
			Type:        task.TaskTypeNPM,
			Group:       task.InferGroup(name),
			Command:     packageManager,
			Args:        []string{"run", name},
		}

		// Set problem matcher based on script content
		t.ProblemMatcher = s.inferProblemMatcher(name, script, pkg)

		// Mark common default scripts
		if name == "start" || name == "dev" || name == "build" {
			if name == "start" || name == "dev" {
				t.Group = task.TaskGroupRun
			}
			if name == "build" {
				t.Group = task.TaskGroupBuild
				t.IsDefault = true
			}
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

// detectPackageManager determines which package manager to use.
func (s *PackageJSONSource) detectPackageManager(dir string) string {
	// Check for lock files in order of preference
	lockFiles := []struct {
		file    string
		manager string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"package-lock.json", "npm"},
	}

	for _, lf := range lockFiles {
		if _, err := os.Stat(filepath.Join(dir, lf.file)); err == nil {
			return lf.manager
		}
	}

	// Default to npm
	return "npm"
}

// generateDescription generates a description for a script.
func (s *PackageJSONSource) generateDescription(name, script string) string {
	// Truncate long scripts
	if len(script) > 80 {
		return script[:77] + "..."
	}
	return script
}

// inferProblemMatcher infers the appropriate problem matcher.
func (s *PackageJSONSource) inferProblemMatcher(name, script string, pkg packageJSON) string {
	// Check script content and dependencies for common tools
	scriptLower := strings.ToLower(script)

	// TypeScript
	if strings.Contains(scriptLower, "tsc") {
		return "$tsc"
	}

	// ESLint
	if strings.Contains(scriptLower, "eslint") {
		return "$eslint-compact"
	}

	// Jest
	if strings.Contains(scriptLower, "jest") {
		return "$jest"
	}

	// Mocha
	if strings.Contains(scriptLower, "mocha") {
		return "$mocha"
	}

	// Check for TypeScript in dependencies
	if _, hasTS := pkg.DevDependencies["typescript"]; hasTS {
		if name == "build" || name == "compile" {
			return "$tsc"
		}
	}

	return ""
}

// ParseNPMScripts extracts just the script names from package.json.
func ParseNPMScripts(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	scripts := make([]string, 0, len(pkg.Scripts))
	for name := range pkg.Scripts {
		scripts = append(scripts, name)
	}

	return scripts, nil
}

// GetNPMScript returns the command for a specific script.
func GetNPMScript(path, scriptName string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", err
	}

	return pkg.Scripts[scriptName], nil
}
