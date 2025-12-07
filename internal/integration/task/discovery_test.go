package task

import (
	"context"
	"testing"
	"time"
)

// MockSource is a test source that returns predefined tasks.
type MockSource struct {
	name     string
	patterns []string
	priority int
	tasks    []*Task
	err      error
}

func (s *MockSource) Name() string       { return s.name }
func (s *MockSource) Patterns() []string { return s.patterns }
func (s *MockSource) Priority() int      { return s.priority }
func (s *MockSource) Discover(ctx context.Context, path string) ([]*Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tasks, nil
}

func TestNewDiscovery(t *testing.T) {
	d := NewDiscovery()
	if d == nil {
		t.Fatal("NewDiscovery returned nil")
	}

	if d.sources == nil {
		t.Error("sources map is nil")
	}

	if d.cache == nil {
		t.Error("cache map is nil")
	}
}

func TestDiscovery_RegisterSource(t *testing.T) {
	d := NewDiscovery()

	source := &MockSource{
		name:     "test",
		patterns: []string{"*.test"},
		priority: 100,
	}

	d.RegisterSource(source)

	got, ok := d.GetSource("test")
	if !ok {
		t.Fatal("source not found after registration")
	}

	if got.Name() != "test" {
		t.Errorf("got source name %q, want %q", got.Name(), "test")
	}
}

func TestDiscovery_UnregisterSource(t *testing.T) {
	d := NewDiscovery()

	source := &MockSource{name: "test"}
	d.RegisterSource(source)

	d.UnregisterSource("test")

	_, ok := d.GetSource("test")
	if ok {
		t.Error("source found after unregistration")
	}
}

func TestDiscovery_Sources(t *testing.T) {
	d := NewDiscovery()

	sources := []Source{
		&MockSource{name: "alpha"},
		&MockSource{name: "beta"},
		&MockSource{name: "gamma"},
	}

	for _, s := range sources {
		d.RegisterSource(s)
	}

	names := d.Sources()
	if len(names) != 3 {
		t.Errorf("got %d sources, want 3", len(names))
	}

	// Should be sorted alphabetically
	expected := []string{"alpha", "beta", "gamma"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want)
		}
	}
}

func TestDiscovery_ClearCache(t *testing.T) {
	d := NewDiscovery()

	// Add something to cache manually
	d.cache["/test"] = &DiscoveryResult{}
	d.lastUpdate["/test"] = time.Now()

	d.ClearCache()

	if len(d.cache) != 0 {
		t.Error("cache not cleared")
	}

	if len(d.lastUpdate) != 0 {
		t.Error("lastUpdate not cleared")
	}
}

func TestDiscovery_ClearCacheFor(t *testing.T) {
	d := NewDiscovery()

	// Add entries to cache
	d.cache["/test1"] = &DiscoveryResult{}
	d.cache["/test2"] = &DiscoveryResult{}
	d.lastUpdate["/test1"] = time.Now()
	d.lastUpdate["/test2"] = time.Now()

	d.ClearCacheFor("/test1")

	if _, ok := d.cache["/test1"]; ok {
		t.Error("/test1 not removed from cache")
	}

	if _, ok := d.cache["/test2"]; !ok {
		t.Error("/test2 should still be in cache")
	}
}

func TestDiscovery_SetCacheTime(t *testing.T) {
	d := NewDiscovery()

	d.SetCacheTime(10 * time.Minute)

	if d.cacheTime != 10*time.Minute {
		t.Errorf("cacheTime = %v, want %v", d.cacheTime, 10*time.Minute)
	}
}

func TestDefaultDiscoveryOptions(t *testing.T) {
	opts := DefaultDiscoveryOptions("/test/root")

	if opts.RootDir != "/test/root" {
		t.Errorf("RootDir = %q, want %q", opts.RootDir, "/test/root")
	}

	if opts.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d, want 3", opts.MaxDepth)
	}

	if opts.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", opts.Timeout, 30*time.Second)
	}

	// Check excluded directories
	excludedDirs := map[string]bool{
		"node_modules": true,
		".git":         true,
		"vendor":       true,
	}

	for dir := range excludedDirs {
		found := false
		for _, excluded := range opts.ExcludeDirs {
			if excluded == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q to be in ExcludeDirs", dir)
		}
	}
}

func TestInferGroup(t *testing.T) {
	tests := []struct {
		name string
		want TaskGroup
	}{
		{"build", TaskGroupBuild},
		{"build:prod", TaskGroupBuild},
		{"compile", TaskGroupBuild},
		{"test", TaskGroupTest},
		{"test:unit", TaskGroupTest},
		{"run", TaskGroupRun},
		{"start", TaskGroupRun},
		{"dev", TaskGroupRun},
		{"serve", TaskGroupRun},
		{"clean", TaskGroupClean},
		{"lint", TaskGroupLint},
		{"format", TaskGroupLint},
		{"fmt", TaskGroupLint},
		{"random", TaskGroupOther},
		{"deploy", TaskGroupOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferGroup(tt.name)
			if got != tt.want {
				t.Errorf("InferGroup(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestGenerateTaskID(t *testing.T) {
	tests := []struct {
		rootDir string
		source  string
		file    string
		name    string
		want    string
	}{
		{"/project", "makefile", "/project/Makefile", "build", "makefile:Makefile:build"},
		{"/project", "npm", "/project/package.json", "test", "npm:package.json:test"},
		{"/project", "taskfile", "/project/Taskfile.yml", "lint", "taskfile:Taskfile.yml:lint"},
		{"/project", "npm", "/project/subdir/package.json", "test", "npm:subdir/package.json:test"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := generateTaskID(tt.rootDir, tt.source, tt.file, tt.name)
			if got != tt.want {
				t.Errorf("generateTaskID(%q, %q, %q, %q) = %q, want %q",
					tt.rootDir, tt.source, tt.file, tt.name, got, tt.want)
			}
		})
	}
}

func TestDiscoveryError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  DiscoveryError
		want string
	}{
		{
			name: "with file",
			err: DiscoveryError{
				Source: "makefile",
				File:   "/project/Makefile",
				Err:    context.DeadlineExceeded,
			},
			want: "makefile: /project/Makefile: context deadline exceeded",
		},
		{
			name: "without file",
			err: DiscoveryError{
				Source: "npm",
				Err:    context.Canceled,
			},
			want: "npm: context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiscovery_DiscoverNoSources(t *testing.T) {
	d := NewDiscovery()
	ctx := context.Background()

	result, err := d.Discover(ctx, DiscoveryOptions{
		RootDir:  "/nonexistent",
		MaxDepth: 1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(result.Tasks))
	}
}
