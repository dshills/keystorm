package sources

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/keystorm/internal/integration/task"
)

func TestKeystormSource_Name(t *testing.T) {
	s := NewKeystormSource()
	if s.Name() != "keystorm" {
		t.Errorf("Name() = %q, want %q", s.Name(), "keystorm")
	}
}

func TestKeystormSource_Patterns(t *testing.T) {
	s := NewKeystormSource()
	patterns := s.Patterns()

	if len(patterns) != 1 || patterns[0] != "tasks.json" {
		t.Errorf("Patterns() = %v, want [tasks.json]", patterns)
	}
}

func TestKeystormSource_Priority(t *testing.T) {
	s := NewKeystormSource()
	if s.Priority() != 200 {
		t.Errorf("Priority() = %d, want 200", s.Priority())
	}
}

func TestKeystormSource_Discover(t *testing.T) {
	tmpDir := t.TempDir()
	keystormDir := filepath.Join(tmpDir, ".keystorm")
	if err := os.MkdirAll(keystormDir, 0755); err != nil {
		t.Fatalf("failed to create .keystorm dir: %v", err)
	}

	tasksPath := filepath.Join(keystormDir, "tasks.json")

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
				Detail:         "Build the Go project",
			},
			{
				Label:   "Test",
				Type:    "shell",
				Command: "go",
				Args:    []string{"test", "./..."},
				Group: KeystormGroupRef{
					Kind: "test",
				},
				DependsOn: []string{"Build"},
				Options: KeystormOptions{
					Cwd: "./src",
					Env: map[string]string{"GO_TEST_TIMEOUT": "30s"},
				},
			},
			{
				Label:   "Lint",
				Type:    "process",
				Command: "golangci-lint",
				Args:    []string{"run"},
				Group: KeystormGroupRef{
					Kind: "lint",
				},
				RunOptions: KeystormRunOpts{
					InstanceLimit:     1,
					RunOn:             "save",
					ReevaluateOnRerun: true,
				},
			},
		},
	}

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal tasks: %v", err)
	}

	if err := os.WriteFile(tasksPath, data, 0644); err != nil {
		t.Fatalf("failed to write tasks.json: %v", err)
	}

	s := NewKeystormSource()
	tasks, err := s.Discover(context.Background(), tasksPath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("got %d tasks, want 3", len(tasks))
	}

	taskMap := make(map[string]*task.Task)
	for _, tsk := range tasks {
		taskMap[tsk.Name] = tsk
	}

	// Verify Build task
	if build, ok := taskMap["Build"]; ok {
		if build.Description != "Build the Go project" {
			t.Errorf("build description = %q, want %q", build.Description, "Build the Go project")
		}
		if build.Type != task.TaskTypeShell {
			t.Errorf("build type = %q, want %q", build.Type, task.TaskTypeShell)
		}
		if build.Group != task.TaskGroupBuild {
			t.Errorf("build group = %q, want %q", build.Group, task.TaskGroupBuild)
		}
		if !build.IsDefault {
			t.Error("build should be marked as default")
		}
		if build.ProblemMatcher != "$go" {
			t.Errorf("build problemMatcher = %q, want %q", build.ProblemMatcher, "$go")
		}
	} else {
		t.Error("Build task not found")
	}

	// Verify Test task
	if test, ok := taskMap["Test"]; ok {
		if test.Cwd != "./src" {
			t.Errorf("test cwd = %q, want %q", test.Cwd, "./src")
		}
		if test.Env == nil || test.Env["GO_TEST_TIMEOUT"] != "30s" {
			t.Error("test should have env GO_TEST_TIMEOUT=30s")
		}
		if len(test.DependsOn) != 1 || test.DependsOn[0] != "Build" {
			t.Errorf("test dependencies = %v, want [Build]", test.DependsOn)
		}
	} else {
		t.Error("Test task not found")
	}

	// Verify Lint task
	if lint, ok := taskMap["Lint"]; ok {
		if lint.Type != task.TaskTypeProcess {
			t.Errorf("lint type = %q, want %q", lint.Type, task.TaskTypeProcess)
		}
		if lint.RunOptions == nil {
			t.Error("lint should have RunOptions")
		} else {
			if lint.RunOptions.InstanceLimit != 1 {
				t.Errorf("lint instanceLimit = %d, want 1", lint.RunOptions.InstanceLimit)
			}
			if lint.RunOptions.RunOn != "save" {
				t.Errorf("lint runOn = %q, want %q", lint.RunOptions.RunOn, "save")
			}
			if !lint.RunOptions.ReevaluateOnRerun {
				t.Error("lint should have reevaluateOnRerun = true")
			}
		}
	} else {
		t.Error("Lint task not found")
	}
}

func TestKeystormSource_DiscoverNotInKeystormDir(t *testing.T) {
	tmpDir := t.TempDir()
	// tasks.json NOT in .keystorm directory
	tasksPath := filepath.Join(tmpDir, "tasks.json")

	tf := KeystormTasksFile{
		Version: "1.0.0",
		Tasks: []KeystormTask{
			{Label: "Build", Type: "shell", Command: "go", Args: []string{"build"}},
		},
	}

	data, _ := json.Marshal(tf)
	if err := os.WriteFile(tasksPath, data, 0644); err != nil {
		t.Fatalf("failed to write tasks.json: %v", err)
	}

	s := NewKeystormSource()
	tasks, err := s.Discover(context.Background(), tasksPath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Should return nil because it's not in .keystorm directory
	if tasks != nil {
		t.Errorf("expected nil tasks for file not in .keystorm dir, got %d", len(tasks))
	}
}

func TestKeystormSource_MapTaskType(t *testing.T) {
	s := &KeystormSource{}

	tests := []struct {
		input string
		want  task.TaskType
	}{
		{"shell", task.TaskTypeShell},
		{"process", task.TaskTypeProcess},
		{"npm", task.TaskTypeNPM},
		{"unknown", task.TaskTypeShell}, // defaults to shell
		{"", task.TaskTypeShell},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := s.mapTaskType(tt.input)
			if got != tt.want {
				t.Errorf("mapTaskType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestKeystormSource_MapGroup(t *testing.T) {
	s := &KeystormSource{}

	tests := []struct {
		input string
		want  task.TaskGroup
	}{
		{"build", task.TaskGroupBuild},
		{"test", task.TaskGroupTest},
		{"run", task.TaskGroupRun},
		{"clean", task.TaskGroupClean},
		{"lint", task.TaskGroupLint},
		{"unknown", task.TaskGroupOther},
		{"", task.TaskGroupOther},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := s.mapGroup(tt.input)
			if got != tt.want {
				t.Errorf("mapGroup(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestKeystormSource_ExtractProblemMatcher(t *testing.T) {
	s := &KeystormSource{}

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"string", "$go", "$go"},
		{"array with string", []interface{}{"$go", "$gcc"}, "$go"},
		{"empty array", []interface{}{}, ""},
		{"nil", nil, ""},
		{"number", 42, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.extractProblemMatcher(tt.input)
			if got != tt.want {
				t.Errorf("extractProblemMatcher(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCreateKeystormTasksFile(t *testing.T) {
	tmpDir := t.TempDir()

	err := CreateKeystormTasksFile(tmpDir)
	if err != nil {
		t.Fatalf("CreateKeystormTasksFile() error = %v", err)
	}

	// Verify file was created
	tasksPath := filepath.Join(tmpDir, ".keystorm", "tasks.json")
	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		t.Error("tasks.json was not created")
	}

	// Verify content
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks.json: %v", err)
	}

	var tf KeystormTasksFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("failed to parse tasks.json: %v", err)
	}

	if tf.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", tf.Version, "1.0.0")
	}

	if len(tf.Tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tf.Tasks))
	}
}

func TestLoadKeystormTasks(t *testing.T) {
	tmpDir := t.TempDir()

	// First create the file
	if err := CreateKeystormTasksFile(tmpDir); err != nil {
		t.Fatalf("CreateKeystormTasksFile() error = %v", err)
	}

	// Then load it
	tf, err := LoadKeystormTasks(tmpDir)
	if err != nil {
		t.Fatalf("LoadKeystormTasks() error = %v", err)
	}

	if tf == nil {
		t.Fatal("LoadKeystormTasks() returned nil")
	}

	if tf.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", tf.Version, "1.0.0")
	}
}

func TestLoadKeystormTasks_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadKeystormTasks(tmpDir)
	if err == nil {
		t.Error("LoadKeystormTasks() should return error for missing file")
	}
}

func TestSaveKeystormTasks(t *testing.T) {
	tmpDir := t.TempDir()

	tf := &KeystormTasksFile{
		Version: "2.0.0",
		Tasks: []KeystormTask{
			{Label: "Custom", Type: "shell", Command: "echo", Args: []string{"hello"}},
		},
	}

	err := SaveKeystormTasks(tmpDir, tf)
	if err != nil {
		t.Fatalf("SaveKeystormTasks() error = %v", err)
	}

	// Verify content
	loaded, err := LoadKeystormTasks(tmpDir)
	if err != nil {
		t.Fatalf("LoadKeystormTasks() error = %v", err)
	}

	if loaded.Version != "2.0.0" {
		t.Errorf("version = %q, want %q", loaded.Version, "2.0.0")
	}

	if len(loaded.Tasks) != 1 || loaded.Tasks[0].Label != "Custom" {
		t.Errorf("tasks not saved correctly")
	}
}

func TestKeystormSource_DiscoverContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	keystormDir := filepath.Join(tmpDir, ".keystorm")
	if err := os.MkdirAll(keystormDir, 0755); err != nil {
		t.Fatalf("failed to create .keystorm dir: %v", err)
	}

	tasksPath := filepath.Join(keystormDir, "tasks.json")

	// Create a file with many tasks
	tf := KeystormTasksFile{
		Version: "1.0.0",
		Tasks:   make([]KeystormTask, 100),
	}
	for i := 0; i < 100; i++ {
		tf.Tasks[i] = KeystormTask{
			Label:   "Task" + string(rune('0'+i%10)),
			Type:    "shell",
			Command: "echo",
		}
	}

	data, _ := json.Marshal(tf)
	if err := os.WriteFile(tasksPath, data, 0644); err != nil {
		t.Fatalf("failed to write tasks.json: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	s := NewKeystormSource()
	_, err := s.Discover(ctx, tasksPath)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestKeystormSource_DiscoverEmptyTasks(t *testing.T) {
	tmpDir := t.TempDir()
	keystormDir := filepath.Join(tmpDir, ".keystorm")
	if err := os.MkdirAll(keystormDir, 0755); err != nil {
		t.Fatalf("failed to create .keystorm dir: %v", err)
	}

	tasksPath := filepath.Join(keystormDir, "tasks.json")

	tf := KeystormTasksFile{
		Version: "1.0.0",
		Tasks:   []KeystormTask{},
	}

	data, _ := json.Marshal(tf)
	if err := os.WriteFile(tasksPath, data, 0644); err != nil {
		t.Fatalf("failed to write tasks.json: %v", err)
	}

	s := NewKeystormSource()
	tasks, err := s.Discover(context.Background(), tasksPath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if tasks != nil {
		t.Errorf("expected nil tasks for empty tasks array, got %d", len(tasks))
	}
}
