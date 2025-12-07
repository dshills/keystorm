package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewVariableResolver(t *testing.T) {
	vr := NewVariableResolver()
	if vr == nil {
		t.Fatal("NewVariableResolver returned nil")
	}

	if vr.custom == nil {
		t.Error("custom map is nil")
	}
	if vr.providers == nil {
		t.Error("providers map is nil")
	}
}

func TestVariableResolver_SetGetDelete(t *testing.T) {
	vr := NewVariableResolver()

	// Set
	vr.Set("foo", "bar")

	// Get
	val, ok := vr.Get("foo")
	if !ok {
		t.Error("Get returned false for existing key")
	}
	if val != "bar" {
		t.Errorf("Get = %q, want %q", val, "bar")
	}

	// Get non-existent
	_, ok = vr.Get("nonexistent")
	if ok {
		t.Error("Get returned true for non-existent key")
	}

	// Delete
	vr.Delete("foo")
	_, ok = vr.Get("foo")
	if ok {
		t.Error("key still exists after Delete")
	}
}

func TestVariableResolver_RegisterProvider(t *testing.T) {
	vr := NewVariableResolver()

	vr.RegisterProvider("custom", func(ctx *VariableContext) string {
		return "custom_value"
	})

	result := vr.ResolveWithContext("${custom}", &VariableContext{})
	if result != "custom_value" {
		t.Errorf("Resolve = %q, want %q", result, "custom_value")
	}

	// Unregister
	vr.UnregisterProvider("custom")
	result = vr.ResolveWithContext("${custom}", &VariableContext{})
	if result != "${custom}" {
		t.Errorf("after unregister, Resolve = %q, want ${custom}", result)
	}
}

func TestVariableResolver_ResolveBasic(t *testing.T) {
	vr := NewVariableResolver()

	vr.Set("name", "world")

	tests := []struct {
		input string
		want  string
	}{
		{"hello ${name}", "hello world"},
		{"${name}", "world"},
		{"$name", "world"},
		{"${name} ${name}", "world world"},
		{"no vars here", "no vars here"},
	}

	for _, tt := range tests {
		got := vr.Resolve(tt.input, nil)
		if got != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestVariableResolver_ResolveWithDefault(t *testing.T) {
	vr := NewVariableResolver()

	tests := []struct {
		input string
		want  string
	}{
		{"${undefined:default}", "default"},
		{"${undefined:}", ""},
		{"${undefined:with spaces}", "with spaces"},
	}

	for _, tt := range tests {
		got := vr.Resolve(tt.input, nil)
		if got != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}

	// With defined variable, should use actual value
	vr.Set("defined", "actual")
	got := vr.Resolve("${defined:default}", nil)
	if got != "actual" {
		t.Errorf("Resolve with defined = %q, want %q", got, "actual")
	}
}

func TestVariableResolver_ResolveEnv(t *testing.T) {
	vr := NewVariableResolver()

	// Set up env var
	os.Setenv("TEST_VAR_12345", "test_value")
	defer os.Unsetenv("TEST_VAR_12345")

	result := vr.ResolveEnv("${env:TEST_VAR_12345}")
	if result != "test_value" {
		t.Errorf("ResolveEnv = %q, want %q", result, "test_value")
	}
}

func TestVariableResolver_BuiltinProviders(t *testing.T) {
	vr := NewVariableResolver()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "test.go")

	ctx := &VariableContext{
		WorkingDir: tmpDir,
		File:       testFile,
		Line:       42,
		Column:     10,
		Selection:  "selected text",
	}

	tests := []struct {
		input string
		want  string
	}{
		{"${workspaceFolder}", tmpDir},
		{"${workspaceFolderBasename}", filepath.Base(tmpDir)},
		{"${file}", testFile},
		{"${fileBasename}", "test.go"},
		{"${fileBasenameNoExtension}", "test"},
		{"${fileDirname}", filepath.Join(tmpDir, "subdir")},
		{"${fileExtname}", ".go"},
		{"${lineNumber}", "42"},
		{"${selectedText}", "selected text"},
		{"${pathSeparator}", string(filepath.Separator)},
	}

	for _, tt := range tests {
		got := vr.ResolveWithContext(tt.input, ctx)
		if got != tt.want {
			t.Errorf("ResolveWithContext(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestVariableResolver_RelativeFile(t *testing.T) {
	vr := NewVariableResolver()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "src", "main.go")

	ctx := &VariableContext{
		WorkingDir: tmpDir,
		File:       testFile,
	}

	result := vr.ResolveWithContext("${relativeFile}", ctx)
	want := filepath.Join("src", "main.go")
	if result != want {
		t.Errorf("relativeFile = %q, want %q", result, want)
	}

	result = vr.ResolveWithContext("${relativeFileDirname}", ctx)
	if result != "src" {
		t.Errorf("relativeFileDirname = %q, want src", result)
	}
}

func TestVariableResolver_CwdProvider(t *testing.T) {
	vr := NewVariableResolver()

	cwd, _ := os.Getwd()
	result := vr.ResolveWithContext("${cwd}", &VariableContext{})
	if result != cwd {
		t.Errorf("cwd = %q, want %q", result, cwd)
	}
}

func TestVariableResolver_ExecPath(t *testing.T) {
	vr := NewVariableResolver()

	result := vr.ResolveWithContext("${execPath}", &VariableContext{})
	// Should return something (the executable path)
	if result == "" {
		t.Error("execPath is empty")
	}
}

func TestVariableResolver_EnvFallback(t *testing.T) {
	vr := NewVariableResolver()

	// Set environment variable
	os.Setenv("TEST_ENV_VAR_XYZ", "env_value")
	defer os.Unsetenv("TEST_ENV_VAR_XYZ")

	// Should resolve to env var value
	result := vr.Resolve("${TEST_ENV_VAR_XYZ}", nil)
	if result != "env_value" {
		t.Errorf("env fallback = %q, want %q", result, "env_value")
	}
}

func TestVariableResolver_ResolveAll(t *testing.T) {
	vr := NewVariableResolver()

	os.Setenv("TEST_ALL_VAR", "test_value")
	defer os.Unsetenv("TEST_ALL_VAR")

	vr.Set("custom", "custom_value")

	ctx := &VariableContext{
		File: "/path/to/file.go",
	}

	result := vr.ResolveAll("${env:TEST_ALL_VAR} ${custom} ${fileBasename}", ctx)
	want := "test_value custom_value file.go"
	if result != want {
		t.Errorf("ResolveAll = %q, want %q", result, want)
	}
}

func TestVariableResolver_WithTask(t *testing.T) {
	vr := NewVariableResolver()

	task := &Task{
		Name: "test-task",
		Cwd:  "/project/dir",
	}

	ctx := &VariableContext{
		Task: task,
	}

	// workspaceFolder should use task.Cwd if WorkingDir not set
	result := vr.ResolveWithContext("${workspaceFolder}", ctx)
	if result != "/project/dir" {
		t.Errorf("workspaceFolder with task = %q, want /project/dir", result)
	}
}

func TestVariableResolver_EmptyFile(t *testing.T) {
	vr := NewVariableResolver()

	ctx := &VariableContext{
		File: "",
	}

	// When File is empty, file-related variables are not resolvable
	// so they should remain as-is (this matches VS Code behavior)
	tests := []struct {
		input string
		want  string
	}{
		{"${file}", "${file}"},
		{"${fileBasename}", "${fileBasename}"},
		{"${fileBasenameNoExtension}", "${fileBasenameNoExtension}"},
		{"${fileDirname}", "${fileDirname}"},
		{"${fileExtname}", "${fileExtname}"},
		{"${relativeFile}", "${relativeFile}"},
		{"${relativeFileDirname}", "${relativeFileDirname}"},
	}

	for _, tt := range tests {
		got := vr.ResolveWithContext(tt.input, ctx)
		if got != tt.want {
			t.Errorf("ResolveWithContext(%q) with empty file = %q, want %q", tt.input, got, tt.want)
		}
	}

	// But with defaults, the default should be used
	got := vr.ResolveWithContext("${file:default.txt}", ctx)
	if got != "default.txt" {
		t.Errorf("${file:default.txt} with empty file = %q, want default.txt", got)
	}
}

func TestVariableResolver_ZeroLine(t *testing.T) {
	vr := NewVariableResolver()

	ctx := &VariableContext{
		Line: 0,
	}

	// When Line is 0, lineNumber is not resolvable, so it remains as-is
	result := vr.ResolveWithContext("${lineNumber}", ctx)
	if result != "${lineNumber}" {
		t.Errorf("lineNumber with 0 = %q, want ${lineNumber}", result)
	}

	// But with a default, the default should be used
	result = vr.ResolveWithContext("${lineNumber:1}", ctx)
	if result != "1" {
		t.Errorf("${lineNumber:1} with 0 = %q, want 1", result)
	}
}

func TestVariableContext_Fields(t *testing.T) {
	task := &Task{Name: "test"}
	ctx := &VariableContext{
		Task:       task,
		WorkingDir: "/work",
		File:       "/work/file.go",
		Selection:  "selected",
		Line:       10,
		Column:     5,
	}

	if ctx.Task != task {
		t.Error("Task mismatch")
	}
	if ctx.WorkingDir != "/work" {
		t.Errorf("WorkingDir = %q", ctx.WorkingDir)
	}
	if ctx.File != "/work/file.go" {
		t.Errorf("File = %q", ctx.File)
	}
	if ctx.Selection != "selected" {
		t.Errorf("Selection = %q", ctx.Selection)
	}
	if ctx.Line != 10 {
		t.Errorf("Line = %d", ctx.Line)
	}
	if ctx.Column != 5 {
		t.Errorf("Column = %d", ctx.Column)
	}
}

func TestVariableResolver_EnvWithDefault(t *testing.T) {
	vr := NewVariableResolver()

	// Set up env var
	os.Setenv("TEST_ENV_DEFAULT_VAR", "actual_value")
	defer os.Unsetenv("TEST_ENV_DEFAULT_VAR")

	tests := []struct {
		input string
		want  string
	}{
		// Env var set - should use actual value
		{"${env:TEST_ENV_DEFAULT_VAR}", "actual_value"},
		{"${env:TEST_ENV_DEFAULT_VAR:default}", "actual_value"},
		// Env var not set - should use default
		{"${env:UNSET_VAR_12345}", ""},
		{"${env:UNSET_VAR_12345:fallback}", "fallback"},
		{"${env:UNSET_VAR_12345:}", ""},
		{"${env:UNSET_VAR_12345:with spaces}", "with spaces"},
	}

	for _, tt := range tests {
		got := vr.ResolveWithContext(tt.input, &VariableContext{})
		if got != tt.want {
			t.Errorf("ResolveWithContext(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
