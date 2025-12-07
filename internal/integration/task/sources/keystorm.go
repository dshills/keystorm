package sources

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dshills/keystorm/internal/integration/task"
)

// KeystormSource discovers tasks from .keystorm/tasks.json files.
type KeystormSource struct{}

// NewKeystormSource creates a new Keystorm tasks source.
func NewKeystormSource() *KeystormSource {
	return &KeystormSource{}
}

// Name returns the source name.
func (s *KeystormSource) Name() string {
	return "keystorm"
}

// Patterns returns the file patterns this source handles.
func (s *KeystormSource) Patterns() []string {
	return []string{
		"tasks.json",
	}
}

// Priority returns the source priority (highest for keystorm tasks).
func (s *KeystormSource) Priority() int {
	return 200
}

// KeystormTasksFile represents the structure of a tasks.json file.
type KeystormTasksFile struct {
	Version string          `json:"version"`
	Tasks   []KeystormTask  `json:"tasks"`
	Groups  []KeystormGroup `json:"groups,omitempty"`
	Inputs  []KeystormInput `json:"inputs,omitempty"`
}

// KeystormTask represents a task definition in tasks.json.
type KeystormTask struct {
	Label          string           `json:"label"`
	Type           string           `json:"type"`
	Command        string           `json:"command"`
	Args           []string         `json:"args,omitempty"`
	Options        KeystormOptions  `json:"options,omitempty"`
	Group          KeystormGroupRef `json:"group,omitempty"`
	ProblemMatcher interface{}      `json:"problemMatcher,omitempty"`
	DependsOn      []string         `json:"dependsOn,omitempty"`
	DependsOrder   string           `json:"dependsOrder,omitempty"`
	Detail         string           `json:"detail,omitempty"`
	Presentation   KeystormPresent  `json:"presentation,omitempty"`
	RunOptions     KeystormRunOpts  `json:"runOptions,omitempty"`
	IsBackground   bool             `json:"isBackground,omitempty"`
}

// KeystormOptions contains task execution options.
type KeystormOptions struct {
	Cwd   string            `json:"cwd,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
	Shell KeystormShell     `json:"shell,omitempty"`
}

// KeystormShell configures the shell for task execution.
type KeystormShell struct {
	Executable string   `json:"executable,omitempty"`
	Args       []string `json:"args,omitempty"`
}

// KeystormGroupRef is a reference to a task group.
type KeystormGroupRef struct {
	Kind      string `json:"kind,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

// KeystormPresent configures task presentation.
type KeystormPresent struct {
	Reveal           string `json:"reveal,omitempty"`
	Echo             bool   `json:"echo,omitempty"`
	Focus            bool   `json:"focus,omitempty"`
	Panel            string `json:"panel,omitempty"`
	ShowReuseMessage bool   `json:"showReuseMessage,omitempty"`
	Clear            bool   `json:"clear,omitempty"`
}

// KeystormRunOpts configures run behavior.
type KeystormRunOpts struct {
	InstanceLimit     int    `json:"instanceLimit,omitempty"`
	RunOn             string `json:"runOn,omitempty"`
	ReevaluateOnRerun bool   `json:"reevaluateOnRerun,omitempty"`
}

// KeystormGroup defines a task group.
type KeystormGroup struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// KeystormInput defines an input variable.
type KeystormInput struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// Discover finds tasks in a tasks.json file.
func (s *KeystormSource) Discover(ctx context.Context, path string) ([]*task.Task, error) {
	// Only process files in .keystorm directories
	dir := filepath.Dir(path)
	if filepath.Base(dir) != ".keystorm" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf KeystormTasksFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, err
	}

	if len(tf.Tasks) == 0 {
		return nil, nil
	}

	var tasks []*task.Task
	for _, kt := range tf.Tasks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		t := &task.Task{
			Name:        kt.Label,
			Description: kt.Detail,
			Type:        s.mapTaskType(kt.Type),
			Group:       s.mapGroup(kt.Group.Kind),
			Command:     kt.Command,
			Args:        kt.Args,
			Cwd:         kt.Options.Cwd,
			Env:         kt.Options.Env,
			DependsOn:   kt.DependsOn,
			IsDefault:   kt.Group.IsDefault,
		}

		// Set problem matcher
		if pm := s.extractProblemMatcher(kt.ProblemMatcher); pm != "" {
			t.ProblemMatcher = pm
		}

		// Set run options
		if kt.RunOptions.InstanceLimit > 0 || kt.RunOptions.RunOn != "" {
			t.RunOptions = &task.RunOptions{
				InstanceLimit:     kt.RunOptions.InstanceLimit,
				RunOn:             kt.RunOptions.RunOn,
				ReevaluateOnRerun: kt.RunOptions.ReevaluateOnRerun,
			}
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

// mapTaskType maps a keystorm task type to our TaskType.
func (s *KeystormSource) mapTaskType(t string) task.TaskType {
	switch t {
	case "shell":
		return task.TaskTypeShell
	case "process":
		return task.TaskTypeProcess
	case "npm":
		return task.TaskTypeNPM
	default:
		return task.TaskTypeShell
	}
}

// mapGroup maps a keystorm group kind to our TaskGroup.
func (s *KeystormSource) mapGroup(kind string) task.TaskGroup {
	switch kind {
	case "build":
		return task.TaskGroupBuild
	case "test":
		return task.TaskGroupTest
	case "run":
		return task.TaskGroupRun
	case "clean":
		return task.TaskGroupClean
	case "lint":
		return task.TaskGroupLint
	default:
		return task.TaskGroupOther
	}
}

// extractProblemMatcher extracts the problem matcher name.
func (s *KeystormSource) extractProblemMatcher(pm interface{}) string {
	switch v := pm.(type) {
	case string:
		return v
	case []interface{}:
		if len(v) > 0 {
			if str, ok := v[0].(string); ok {
				return str
			}
		}
	}
	return ""
}

// CreateKeystormTasksFile creates a new tasks.json file with sample tasks.
func CreateKeystormTasksFile(dir string) error {
	tasksDir := filepath.Join(dir, ".keystorm")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return err
	}

	tf := KeystormTasksFile{
		Version: "1.0.0",
		Tasks: []KeystormTask{
			{
				Label:   "Build",
				Type:    "shell",
				Command: "go",
				Args:    []string{"build", "./..."},
				Group: KeystormGroupRef{
					Kind:      "build",
					IsDefault: true,
				},
				ProblemMatcher: "$go",
			},
			{
				Label:   "Test",
				Type:    "shell",
				Command: "go",
				Args:    []string{"test", "./..."},
				Group: KeystormGroupRef{
					Kind: "test",
				},
				ProblemMatcher: "$go",
			},
		},
	}

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(tasksDir, "tasks.json"), data, 0644)
}

// LoadKeystormTasks loads the tasks.json file from a directory.
func LoadKeystormTasks(dir string) (*KeystormTasksFile, error) {
	path := filepath.Join(dir, ".keystorm", "tasks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf KeystormTasksFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, err
	}

	return &tf, nil
}

// SaveKeystormTasks saves the tasks.json file to a directory.
func SaveKeystormTasks(dir string, tf *KeystormTasksFile) error {
	tasksDir := filepath.Join(dir, ".keystorm")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(tasksDir, "tasks.json"), data, 0644)
}
