package sources

import (
	"context"
	"os"

	"github.com/dshills/keystorm/internal/integration/task"
	"gopkg.in/yaml.v3"
)

// TaskfileSource discovers tasks from Taskfile.yml (go-task).
type TaskfileSource struct{}

// NewTaskfileSource creates a new Taskfile source.
func NewTaskfileSource() *TaskfileSource {
	return &TaskfileSource{}
}

// Name returns the source name.
func (s *TaskfileSource) Name() string {
	return "taskfile"
}

// Patterns returns the file patterns this source handles.
func (s *TaskfileSource) Patterns() []string {
	return []string{
		"Taskfile.yml",
		"Taskfile.yaml",
		"taskfile.yml",
		"taskfile.yaml",
	}
}

// Priority returns the source priority.
func (s *TaskfileSource) Priority() int {
	return 95
}

// Taskfile represents the structure of a Taskfile.
type Taskfile struct {
	Version string                 `yaml:"version"`
	Tasks   map[string]TaskfileDef `yaml:"tasks"`
	Vars    map[string]interface{} `yaml:"vars"`
	Env     map[string]string      `yaml:"env"`
	Dotenv  []string               `yaml:"dotenv"`
}

// TaskfileDef represents a task definition in a Taskfile.
type TaskfileDef struct {
	Desc          string                 `yaml:"desc"`
	Summary       string                 `yaml:"summary"`
	Cmds          []interface{}          `yaml:"cmds"`
	Deps          []interface{}          `yaml:"deps"`
	Dir           string                 `yaml:"dir"`
	Env           map[string]string      `yaml:"env"`
	Vars          map[string]interface{} `yaml:"vars"`
	Sources       []string               `yaml:"sources"`
	Generates     []string               `yaml:"generates"`
	Status        []string               `yaml:"status"`
	Preconditions []interface{}          `yaml:"preconditions"`
	Method        string                 `yaml:"method"`
	Silent        bool                   `yaml:"silent"`
	Interactive   bool                   `yaml:"interactive"`
	Internal      bool                   `yaml:"internal"`
	Platforms     []string               `yaml:"platforms"`
}

// Discover finds tasks in a Taskfile.
func (s *TaskfileSource) Discover(ctx context.Context, path string) ([]*task.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf Taskfile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, err
	}

	if len(tf.Tasks) == 0 {
		return nil, nil
	}

	var tasks []*task.Task
	for name, def := range tf.Tasks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Skip internal tasks
		if def.Internal {
			continue
		}

		t := &task.Task{
			Name:        name,
			Description: s.getDescription(def),
			Type:        task.TaskTypeShell,
			Group:       task.InferGroup(name),
			Command:     "task",
			Args:        []string{name},
			Env:         s.mergeEnv(tf.Env, def.Env),
		}

		// Set working directory if specified
		if def.Dir != "" {
			t.Cwd = def.Dir
		}

		// Set dependencies
		t.DependsOn = s.extractDeps(def.Deps)

		// Mark default task
		if name == "default" {
			t.IsDefault = true
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

// getDescription returns the description for a task.
func (s *TaskfileSource) getDescription(def TaskfileDef) string {
	if def.Desc != "" {
		return def.Desc
	}
	if def.Summary != "" {
		// Truncate long summaries
		if len(def.Summary) > 80 {
			return def.Summary[:77] + "..."
		}
		return def.Summary
	}
	return ""
}

// mergeEnv merges global and task-level environment variables.
func (s *TaskfileSource) mergeEnv(global, local map[string]string) map[string]string {
	if len(global) == 0 && len(local) == 0 {
		return nil
	}

	result := make(map[string]string)
	for k, v := range global {
		result[k] = v
	}
	for k, v := range local {
		result[k] = v
	}
	return result
}

// extractDeps extracts dependency task names.
func (s *TaskfileSource) extractDeps(deps []interface{}) []string {
	if len(deps) == 0 {
		return nil
	}

	var result []string
	for _, dep := range deps {
		switch d := dep.(type) {
		case string:
			result = append(result, d)
		case map[string]interface{}:
			// Handle complex dependency with task key
			if taskName, ok := d["task"].(string); ok {
				result = append(result, taskName)
			}
		}
	}
	return result
}

// ParseTaskfileTasks extracts just the task names from a Taskfile.
func ParseTaskfileTasks(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf Taskfile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, err
	}

	tasks := make([]string, 0, len(tf.Tasks))
	for name, def := range tf.Tasks {
		if !def.Internal {
			tasks = append(tasks, name)
		}
	}

	return tasks, nil
}

// GetTaskfileTask returns details for a specific task.
func GetTaskfileTask(path, taskName string) (*TaskfileDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf Taskfile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, err
	}

	if def, ok := tf.Tasks[taskName]; ok {
		return &def, nil
	}

	return nil, nil
}
