// Package sources provides task discovery sources for various build systems.
package sources

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/dshills/keystorm/internal/integration/task"
)

// MakefileSource discovers tasks from Makefiles.
type MakefileSource struct{}

// NewMakefileSource creates a new Makefile source.
func NewMakefileSource() *MakefileSource {
	return &MakefileSource{}
}

// Name returns the source name.
func (s *MakefileSource) Name() string {
	return "makefile"
}

// Patterns returns the file patterns this source handles.
func (s *MakefileSource) Patterns() []string {
	return []string{
		"Makefile",
		"makefile",
		"GNUmakefile",
		"*.mk",
	}
}

// Priority returns the source priority.
func (s *MakefileSource) Priority() int {
	return 100
}

// Discover finds tasks in a Makefile.
func (s *MakefileSource) Discover(ctx context.Context, path string) ([]*task.Task, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tasks []*task.Task
	var currentComment string

	scanner := bufio.NewScanner(file)

	// Regex patterns
	targetPattern := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*)\s*:(?:[^=]|$)`)
	phonyCapturePattern := regexp.MustCompile(`^\.PHONY\s*:\s*(.+)$`)
	commentPattern := regexp.MustCompile(`^##\s*(.*)$`)

	// Track phony targets (these are typically the "runnable" tasks)
	phonyTargets := make(map[string]bool)

	// First pass: collect phony targets
	for scanner.Scan() {
		line := scanner.Text()
		if matches := phonyCapturePattern.FindStringSubmatch(line); matches != nil {
			for _, target := range strings.Fields(matches[1]) {
				phonyTargets[target] = true
			}
		}
	}

	// Reset file for second pass
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	scanner = bufio.NewScanner(file)

	lineNum := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		lineNum++
		line := scanner.Text()

		// Check for documentation comment (## prefix)
		if matches := commentPattern.FindStringSubmatch(line); matches != nil {
			currentComment = matches[1]
			continue
		}

		// Check for target definition
		if matches := targetPattern.FindStringSubmatch(line); matches != nil {
			targetName := matches[1]

			// Skip internal targets (starting with . or _)
			if strings.HasPrefix(targetName, ".") || strings.HasPrefix(targetName, "_") {
				currentComment = ""
				continue
			}

			// Skip pattern rules (contain %)
			if strings.Contains(targetName, "%") {
				currentComment = ""
				continue
			}

			t := &task.Task{
				Name:        targetName,
				Description: currentComment,
				Type:        task.TaskTypeMake,
				Group:       task.InferGroup(targetName),
				Command:     "make",
				Args:        []string{targetName},
			}

			// Check if this is a default target
			if targetName == "all" || targetName == "default" {
				t.IsDefault = true
			}

			// Set problem matcher for common make errors
			t.ProblemMatcher = "$gcc"

			// Only include phony targets (runnable tasks) if we found any,
			// otherwise include all targets for projects without .PHONY
			if len(phonyTargets) == 0 || phonyTargets[targetName] {
				tasks = append(tasks, t)
			}

			// Clear comment after use
			currentComment = ""
		} else if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
			// Clear comment if line is not a comment and not empty
			currentComment = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// ParseMakefileTargets extracts just the target names from a Makefile.
// This is a simpler version for quick listing.
func ParseMakefileTargets(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []string
	targetPattern := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*)\s*:(?:[^=]|$)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := targetPattern.FindStringSubmatch(line); matches != nil {
			target := matches[1]
			if !strings.HasPrefix(target, ".") && !strings.HasPrefix(target, "_") {
				targets = append(targets, target)
			}
		}
	}

	return targets, scanner.Err()
}

// GetMakefileDefault returns the default target (usually 'all' or first target).
func GetMakefileDefault(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	targetPattern := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*)\s*:(?:[^=]|$)`)
	defaultPattern := regexp.MustCompile(`^\.DEFAULT_GOAL\s*[:?]?=\s*(\S+)`)

	var firstTarget string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for explicit default goal
		if matches := defaultPattern.FindStringSubmatch(line); matches != nil {
			return matches[1], nil
		}

		// Track first target
		if firstTarget == "" {
			if matches := targetPattern.FindStringSubmatch(line); matches != nil {
				target := matches[1]
				if !strings.HasPrefix(target, ".") {
					firstTarget = target
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return firstTarget, nil
}
