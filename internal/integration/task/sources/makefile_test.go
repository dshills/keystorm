package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/keystorm/internal/integration/task"
)

func TestMakefileSource_Name(t *testing.T) {
	s := NewMakefileSource()
	if s.Name() != "makefile" {
		t.Errorf("Name() = %q, want %q", s.Name(), "makefile")
	}
}

func TestMakefileSource_Patterns(t *testing.T) {
	s := NewMakefileSource()
	patterns := s.Patterns()

	expected := []string{"Makefile", "makefile", "GNUmakefile", "*.mk"}
	if len(patterns) != len(expected) {
		t.Errorf("got %d patterns, want %d", len(patterns), len(expected))
	}

	for i, want := range expected {
		if patterns[i] != want {
			t.Errorf("patterns[%d] = %q, want %q", i, patterns[i], want)
		}
	}
}

func TestMakefileSource_Priority(t *testing.T) {
	s := NewMakefileSource()
	if s.Priority() != 100 {
		t.Errorf("Priority() = %d, want 100", s.Priority())
	}
}

func TestMakefileSource_Discover(t *testing.T) {
	// Create a temporary directory with a Makefile
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	content := `.PHONY: build test clean

## Build the project
build:
	go build ./...

## Run tests
test:
	go test ./...

## Clean build artifacts
clean:
	rm -rf bin/

# Internal target (no doc comment)
_internal:
	echo "internal"

all: build test
`

	if err := os.WriteFile(makefilePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}

	s := NewMakefileSource()
	tasks, err := s.Discover(context.Background(), makefilePath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Should have 3 tasks: build, test, clean (phony targets only)
	// _internal is skipped (starts with _), all is not in .PHONY
	if len(tasks) != 3 {
		t.Errorf("got %d tasks, want 3", len(tasks))
	}

	// Check that we found expected tasks
	taskMap := make(map[string]*task.Task)
	for _, tsk := range tasks {
		taskMap[tsk.Name] = tsk
	}

	// Verify build task
	if build, ok := taskMap["build"]; ok {
		if build.Description != "Build the project" {
			t.Errorf("build description = %q, want %q", build.Description, "Build the project")
		}
		if build.Type != task.TaskTypeMake {
			t.Errorf("build type = %q, want %q", build.Type, task.TaskTypeMake)
		}
		if build.Command != "make" {
			t.Errorf("build command = %q, want %q", build.Command, "make")
		}
	} else {
		t.Error("build task not found")
	}

	// Verify all task was not included (not in .PHONY)
	if _, ok := taskMap["all"]; ok {
		t.Error("all task should have been excluded (not in .PHONY)")
	}

	// Verify internal task was skipped
	if _, ok := taskMap["_internal"]; ok {
		t.Error("_internal task should have been skipped")
	}
}

func TestMakefileSource_DiscoverContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	content := `build:
	go build ./...
test:
	go test ./...
`
	if err := os.WriteFile(makefilePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	s := NewMakefileSource()
	_, err := s.Discover(ctx, makefilePath)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestParseMakefileTargets(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	content := `build:
	go build

test:
	go test

.PHONY: build test
`
	if err := os.WriteFile(makefilePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}

	targets, err := ParseMakefileTargets(makefilePath)
	if err != nil {
		t.Fatalf("ParseMakefileTargets() error = %v", err)
	}

	if len(targets) != 2 {
		t.Errorf("got %d targets, want 2", len(targets))
	}
}

func TestGetMakefileDefault(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "explicit default goal",
			content: `.DEFAULT_GOAL := test

build:
	go build

test:
	go test
`,
			want: "test",
		},
		{
			name: "first target is default",
			content: `build:
	go build

test:
	go test
`,
			want: "build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			makefilePath := filepath.Join(tmpDir, "Makefile")

			if err := os.WriteFile(makefilePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write Makefile: %v", err)
			}

			got, err := GetMakefileDefault(makefilePath)
			if err != nil {
				t.Fatalf("GetMakefileDefault() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("GetMakefileDefault() = %q, want %q", got, tt.want)
			}
		})
	}
}
