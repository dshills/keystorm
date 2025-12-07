package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/keystorm/internal/integration/task"
)

func TestPackageJSONSource_Name(t *testing.T) {
	s := NewPackageJSONSource()
	if s.Name() != "npm" {
		t.Errorf("Name() = %q, want %q", s.Name(), "npm")
	}
}

func TestPackageJSONSource_Patterns(t *testing.T) {
	s := NewPackageJSONSource()
	patterns := s.Patterns()

	if len(patterns) != 1 || patterns[0] != "package.json" {
		t.Errorf("Patterns() = %v, want [package.json]", patterns)
	}
}

func TestPackageJSONSource_Priority(t *testing.T) {
	s := NewPackageJSONSource()
	if s.Priority() != 90 {
		t.Errorf("Priority() = %d, want 90", s.Priority())
	}
}

func TestPackageJSONSource_Discover(t *testing.T) {
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")

	content := `{
  "name": "test-project",
  "scripts": {
    "build": "tsc",
    "test": "jest",
    "lint": "eslint src/",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "jest": "^29.0.0"
  }
}`

	if err := os.WriteFile(pkgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	s := NewPackageJSONSource()
	tasks, err := s.Discover(context.Background(), pkgPath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(tasks) != 5 {
		t.Errorf("got %d tasks, want 5", len(tasks))
	}

	taskMap := make(map[string]*task.Task)
	for _, tsk := range tasks {
		taskMap[tsk.Name] = tsk
	}

	// Verify build task
	if build, ok := taskMap["build"]; ok {
		if build.Type != task.TaskTypeNPM {
			t.Errorf("build type = %q, want %q", build.Type, task.TaskTypeNPM)
		}
		if build.Group != task.TaskGroupBuild {
			t.Errorf("build group = %q, want %q", build.Group, task.TaskGroupBuild)
		}
		if !build.IsDefault {
			t.Error("build should be marked as default")
		}
		if build.ProblemMatcher != "$tsc" {
			t.Errorf("build problemMatcher = %q, want %q", build.ProblemMatcher, "$tsc")
		}
	} else {
		t.Error("build task not found")
	}

	// Verify test task
	if test, ok := taskMap["test"]; ok {
		if test.Group != task.TaskGroupTest {
			t.Errorf("test group = %q, want %q", test.Group, task.TaskGroupTest)
		}
		if test.ProblemMatcher != "$jest" {
			t.Errorf("test problemMatcher = %q, want %q", test.ProblemMatcher, "$jest")
		}
	} else {
		t.Error("test task not found")
	}

	// Verify start task
	if start, ok := taskMap["start"]; ok {
		if start.Group != task.TaskGroupRun {
			t.Errorf("start group = %q, want %q", start.Group, task.TaskGroupRun)
		}
	} else {
		t.Error("start task not found")
	}

	// Verify lint task
	if lint, ok := taskMap["lint"]; ok {
		if lint.ProblemMatcher != "$eslint-compact" {
			t.Errorf("lint problemMatcher = %q, want %q", lint.ProblemMatcher, "$eslint-compact")
		}
	} else {
		t.Error("lint task not found")
	}
}

func TestPackageJSONSource_DetectPackageManager(t *testing.T) {
	tests := []struct {
		name     string
		lockFile string
		want     string
	}{
		{"npm", "package-lock.json", "npm"},
		{"yarn", "yarn.lock", "yarn"},
		{"pnpm", "pnpm-lock.yaml", "pnpm"},
		{"bun", "bun.lockb", "bun"},
		{"default", "", "npm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.lockFile != "" {
				lockPath := filepath.Join(tmpDir, tt.lockFile)
				if err := os.WriteFile(lockPath, []byte{}, 0644); err != nil {
					t.Fatalf("failed to create lock file: %v", err)
				}
			}

			s := &PackageJSONSource{}
			got := s.detectPackageManager(tmpDir)

			if got != tt.want {
				t.Errorf("detectPackageManager() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPackageJSONSource_GenerateDescription(t *testing.T) {
	s := &PackageJSONSource{}

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"short script", "tsc", "tsc"},
		{"medium script", "npm run build && npm run test", "npm run build && npm run test"},
		{
			"long script",
			"webpack --config webpack.config.js --mode production --optimization-minimize --output-path dist",
			"webpack --config webpack.config.js --mode production --optimization-minimize ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.generateDescription(tt.name, tt.script)
			if got != tt.want {
				t.Errorf("generateDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPackageJSONSource_InferProblemMatcher(t *testing.T) {
	s := &PackageJSONSource{}

	tests := []struct {
		name   string
		script string
		pkg    packageJSON
		want   string
	}{
		{"tsc", "tsc", packageJSON{}, "$tsc"},
		{"eslint", "eslint src/", packageJSON{}, "$eslint-compact"},
		{"jest", "jest --coverage", packageJSON{}, "$jest"},
		{"mocha", "mocha tests/", packageJSON{}, "$mocha"},
		{
			"typescript dep build",
			"node build.js",
			packageJSON{DevDependencies: map[string]string{"typescript": "^5.0.0"}},
			"$tsc",
		},
		{"no match", "node index.js", packageJSON{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.name
			if name == "typescript dep build" {
				name = "build"
			}
			got := s.inferProblemMatcher(name, tt.script, tt.pkg)
			if got != tt.want {
				t.Errorf("inferProblemMatcher() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseNPMScripts(t *testing.T) {
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")

	content := `{
  "name": "test",
  "scripts": {
    "build": "tsc",
    "test": "jest"
  }
}`

	if err := os.WriteFile(pkgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	scripts, err := ParseNPMScripts(pkgPath)
	if err != nil {
		t.Fatalf("ParseNPMScripts() error = %v", err)
	}

	if len(scripts) != 2 {
		t.Errorf("got %d scripts, want 2", len(scripts))
	}
}

func TestGetNPMScript(t *testing.T) {
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")

	content := `{
  "name": "test",
  "scripts": {
    "build": "tsc --build",
    "test": "jest"
  }
}`

	if err := os.WriteFile(pkgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	script, err := GetNPMScript(pkgPath, "build")
	if err != nil {
		t.Fatalf("GetNPMScript() error = %v", err)
	}

	if script != "tsc --build" {
		t.Errorf("GetNPMScript() = %q, want %q", script, "tsc --build")
	}

	// Test non-existent script
	script, err = GetNPMScript(pkgPath, "nonexistent")
	if err != nil {
		t.Fatalf("GetNPMScript() error = %v", err)
	}
	if script != "" {
		t.Errorf("GetNPMScript(nonexistent) = %q, want empty", script)
	}
}

func TestPackageJSONSource_DiscoverEmptyScripts(t *testing.T) {
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")

	content := `{
  "name": "test-project"
}`

	if err := os.WriteFile(pkgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	s := NewPackageJSONSource()
	tasks, err := s.Discover(context.Background(), pkgPath)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if tasks != nil {
		t.Errorf("expected nil tasks for empty scripts, got %d tasks", len(tasks))
	}
}
