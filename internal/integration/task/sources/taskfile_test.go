package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/keystorm/internal/integration/task"
)

func TestTaskfileSource_Name(t *testing.T) {
	s := NewTaskfileSource()
	if s.Name() != "taskfile" {
		t.Errorf("Name() = %q, want %q", s.Name(), "taskfile")
	}
}

func TestTaskfileSource_Patterns(t *testing.T) {
	s := NewTaskfileSource()
	patterns := s.Patterns()

	expected := []string{"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml"}
	if len(patterns) != len(expected) {
		t.Errorf("got %d patterns, want %d", len(patterns), len(expected))
	}

	for i, want := range expected {
		if patterns[i] != want {
			t.Errorf("patterns[%d] = %q, want %q", i, patterns[i], want)
		}
	}
}

func TestTaskfileSource_Priority(t *testing.T) {
	s := NewTaskfileSource()
	if s.Priority() != 95 {
		t.Errorf("Priority() = %d, want 95", s.Priority())
	}
}

func TestTaskfileSource_Discover(t *testing.T) {
	tmpDir := t.TempDir()
	taskfilePath := filepath.Join(tmpDir, "Taskfile.yml")

	content := `version: '3'

env:
  GO111MODULE: "on"

tasks:
  default:
    deps: [build]

  build:
    desc: Build the project
    cmds:
      - go build ./...

  test:
    desc: Run tests
    dir: ./tests
    cmds:
      - go test ./...
    deps:
      - build

  lint:
    desc: Run linter
    cmds:
      - golangci-lint run
    env:
      LINT_MODE: strict

  internal:
    internal: true
    cmds:
      - echo "internal task"
`

	if err := os.WriteFile(taskfilePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Taskfile: %v", err)
	}

	s := NewTaskfileSource()
	tasks, err := s.Discover(context.Background(), taskfilePath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Should have 4 tasks (internal is skipped)
	if len(tasks) != 4 {
		t.Errorf("got %d tasks, want 4", len(tasks))
	}

	taskMap := make(map[string]*task.Task)
	for _, tsk := range tasks {
		taskMap[tsk.Name] = tsk
	}

	// Verify internal task was skipped
	if _, ok := taskMap["internal"]; ok {
		t.Error("internal task should have been skipped")
	}

	// Verify default task
	if def, ok := taskMap["default"]; ok {
		if !def.IsDefault {
			t.Error("default task should be marked as default")
		}
	} else {
		t.Error("default task not found")
	}

	// Verify build task
	if build, ok := taskMap["build"]; ok {
		if build.Description != "Build the project" {
			t.Errorf("build description = %q, want %q", build.Description, "Build the project")
		}
		if build.Command != "task" {
			t.Errorf("build command = %q, want %q", build.Command, "task")
		}
		if len(build.Args) != 1 || build.Args[0] != "build" {
			t.Errorf("build args = %v, want [build]", build.Args)
		}
		// Global env should be set
		if build.Env == nil || build.Env["GO111MODULE"] != "on" {
			t.Error("build should have global env GO111MODULE=on")
		}
	} else {
		t.Error("build task not found")
	}

	// Verify test task has directory set
	if test, ok := taskMap["test"]; ok {
		if test.Cwd != "./tests" {
			t.Errorf("test cwd = %q, want %q", test.Cwd, "./tests")
		}
		// Should have build as dependency
		if len(test.DependsOn) != 1 || test.DependsOn[0] != "build" {
			t.Errorf("test dependencies = %v, want [build]", test.DependsOn)
		}
	} else {
		t.Error("test task not found")
	}

	// Verify lint task has both global and local env
	if lint, ok := taskMap["lint"]; ok {
		if lint.Env == nil {
			t.Error("lint should have env")
		} else {
			if lint.Env["GO111MODULE"] != "on" {
				t.Error("lint should have global env GO111MODULE=on")
			}
			if lint.Env["LINT_MODE"] != "strict" {
				t.Error("lint should have local env LINT_MODE=strict")
			}
		}
	} else {
		t.Error("lint task not found")
	}
}

func TestTaskfileSource_DiscoverEmptyTasks(t *testing.T) {
	tmpDir := t.TempDir()
	taskfilePath := filepath.Join(tmpDir, "Taskfile.yml")

	content := `version: '3'

tasks: {}
`

	if err := os.WriteFile(taskfilePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Taskfile: %v", err)
	}

	s := NewTaskfileSource()
	tasks, err := s.Discover(context.Background(), taskfilePath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if tasks != nil {
		t.Errorf("expected nil tasks, got %d", len(tasks))
	}
}

func TestTaskfileSource_GetDescription(t *testing.T) {
	s := &TaskfileSource{}

	tests := []struct {
		name string
		def  TaskfileDef
		want string
	}{
		{
			name: "desc only",
			def:  TaskfileDef{Desc: "Short description"},
			want: "Short description",
		},
		{
			name: "summary only",
			def:  TaskfileDef{Summary: "Summary text"},
			want: "Summary text",
		},
		{
			name: "desc takes precedence",
			def:  TaskfileDef{Desc: "Description", Summary: "Summary"},
			want: "Description",
		},
		{
			name: "long summary truncated",
			def: TaskfileDef{
				Summary: "This is a very long summary that exceeds the maximum allowed length of 80 characters and should be truncated",
			},
			want: "This is a very long summary that exceeds the maximum allowed length of 80 cha...",
		},
		{
			name: "no description",
			def:  TaskfileDef{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.getDescription(tt.def)
			if got != tt.want {
				t.Errorf("getDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskfileSource_MergeEnv(t *testing.T) {
	s := &TaskfileSource{}

	tests := []struct {
		name   string
		global map[string]string
		local  map[string]string
		want   map[string]string
	}{
		{
			name:   "both nil",
			global: nil,
			local:  nil,
			want:   nil,
		},
		{
			name:   "global only",
			global: map[string]string{"A": "1"},
			local:  nil,
			want:   map[string]string{"A": "1"},
		},
		{
			name:   "local only",
			global: nil,
			local:  map[string]string{"B": "2"},
			want:   map[string]string{"B": "2"},
		},
		{
			name:   "local overrides global",
			global: map[string]string{"A": "1", "B": "global"},
			local:  map[string]string{"B": "local", "C": "3"},
			want:   map[string]string{"A": "1", "B": "local", "C": "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.mergeEnv(tt.global, tt.local)

			if tt.want == nil {
				if got != nil {
					t.Errorf("mergeEnv() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("mergeEnv() length = %d, want %d", len(got), len(tt.want))
			}

			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("mergeEnv()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestTaskfileSource_ExtractDeps(t *testing.T) {
	s := &TaskfileSource{}

	tests := []struct {
		name string
		deps []interface{}
		want []string
	}{
		{
			name: "nil",
			deps: nil,
			want: nil,
		},
		{
			name: "empty",
			deps: []interface{}{},
			want: nil,
		},
		{
			name: "string deps",
			deps: []interface{}{"build", "test"},
			want: []string{"build", "test"},
		},
		{
			name: "complex deps",
			deps: []interface{}{
				"simple",
				map[string]interface{}{"task": "complex"},
			},
			want: []string{"simple", "complex"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.extractDeps(tt.deps)

			if tt.want == nil {
				if got != nil {
					t.Errorf("extractDeps() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("extractDeps() length = %d, want %d", len(got), len(tt.want))
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("extractDeps()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestParseTaskfileTasks(t *testing.T) {
	tmpDir := t.TempDir()
	taskfilePath := filepath.Join(tmpDir, "Taskfile.yml")

	content := `version: '3'

tasks:
  build:
    cmds:
      - go build

  test:
    cmds:
      - go test

  internal:
    internal: true
    cmds:
      - echo internal
`

	if err := os.WriteFile(taskfilePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Taskfile: %v", err)
	}

	tasks, err := ParseTaskfileTasks(taskfilePath)
	if err != nil {
		t.Fatalf("ParseTaskfileTasks() error = %v", err)
	}

	// Should only return non-internal tasks
	if len(tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tasks))
	}
}

func TestGetTaskfileTask(t *testing.T) {
	tmpDir := t.TempDir()
	taskfilePath := filepath.Join(tmpDir, "Taskfile.yml")

	content := `version: '3'

tasks:
  build:
    desc: Build the project
    cmds:
      - go build
`

	if err := os.WriteFile(taskfilePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Taskfile: %v", err)
	}

	// Test existing task
	def, err := GetTaskfileTask(taskfilePath, "build")
	if err != nil {
		t.Fatalf("GetTaskfileTask() error = %v", err)
	}

	if def == nil {
		t.Fatal("GetTaskfileTask() returned nil for existing task")
	}

	if def.Desc != "Build the project" {
		t.Errorf("task desc = %q, want %q", def.Desc, "Build the project")
	}

	// Test non-existent task
	def, err = GetTaskfileTask(taskfilePath, "nonexistent")
	if err != nil {
		t.Fatalf("GetTaskfileTask() error = %v", err)
	}

	if def != nil {
		t.Error("GetTaskfileTask() should return nil for non-existent task")
	}
}
