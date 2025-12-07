package config

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Editor(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	editor := c.Editor()

	if editor.TabSize != 4 {
		t.Errorf("TabSize = %d, want 4", editor.TabSize)
	}
	if !editor.InsertSpaces {
		t.Error("InsertSpaces = false, want true")
	}
	if editor.WordWrap != "off" {
		t.Errorf("WordWrap = %q, want 'off'", editor.WordWrap)
	}
	if !editor.ScrollBeyondLastLine {
		t.Error("ScrollBeyondLastLine = false, want true")
	}
}

func TestConfig_EditorWithOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user settings file with overrides
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	settingsContent := `
[editor]
tabSize = 2
insertSpaces = false
wordWrap = "on"
`
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
	)
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	editor := c.Editor()

	if editor.TabSize != 2 {
		t.Errorf("TabSize = %d, want 2", editor.TabSize)
	}
	if editor.InsertSpaces {
		t.Error("InsertSpaces = true, want false")
	}
	if editor.WordWrap != "on" {
		t.Errorf("WordWrap = %q, want 'on'", editor.WordWrap)
	}
}

func TestConfig_UI(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ui := c.UI()

	if ui.Theme != "dark" {
		t.Errorf("Theme = %q, want 'dark'", ui.Theme)
	}
	if ui.FontSize != 14 {
		t.Errorf("FontSize = %d, want 14", ui.FontSize)
	}
	if ui.FontFamily != "monospace" {
		t.Errorf("FontFamily = %q, want 'monospace'", ui.FontFamily)
	}
	if !ui.ShowMinimap {
		t.Error("ShowMinimap = false, want true")
	}
}

func TestConfig_UIWithOverride(t *testing.T) {
	tmpDir := t.TempDir()

	settingsPath := filepath.Join(tmpDir, "settings.toml")
	settingsContent := `
[ui]
theme = "light"
fontSize = 16
showMinimap = false
`
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
	)
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ui := c.UI()

	if ui.Theme != "light" {
		t.Errorf("Theme = %q, want 'light'", ui.Theme)
	}
	if ui.FontSize != 16 {
		t.Errorf("FontSize = %d, want 16", ui.FontSize)
	}
	if ui.ShowMinimap {
		t.Error("ShowMinimap = true, want false")
	}
}

func TestConfig_Vim(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	vim := c.Vim()

	if !vim.Enabled {
		t.Error("Enabled = false, want true")
	}
	if vim.StartInInsertMode {
		t.Error("StartInInsertMode = true, want false")
	}
	if vim.RelativeLineNumbers {
		t.Error("RelativeLineNumbers = true, want false")
	}
}

func TestConfig_VimWithOverride(t *testing.T) {
	tmpDir := t.TempDir()

	settingsPath := filepath.Join(tmpDir, "settings.toml")
	settingsContent := `
[vim]
enabled = false
startInInsertMode = true
relativeLineNumbers = true
`
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
	)
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	vim := c.Vim()

	if vim.Enabled {
		t.Error("Enabled = true, want false")
	}
	if !vim.StartInInsertMode {
		t.Error("StartInInsertMode = false, want true")
	}
	if !vim.RelativeLineNumbers {
		t.Error("RelativeLineNumbers = false, want true")
	}
}

func TestConfig_Files(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	files := c.Files()

	if files.Encoding != "utf-8" {
		t.Errorf("Encoding = %q, want 'utf-8'", files.Encoding)
	}
	if files.EOL != "lf" {
		t.Errorf("EOL = %q, want 'lf'", files.EOL)
	}
	if len(files.Exclude) == 0 {
		t.Error("Exclude is empty, want non-empty")
	}
}

func TestConfig_Search(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	search := c.Search()

	if search.CaseSensitive {
		t.Error("CaseSensitive = true, want false")
	}
	if search.WholeWord {
		t.Error("WholeWord = true, want false")
	}
	if search.Regex {
		t.Error("Regex = true, want false")
	}
	if search.MaxResults != 1000 {
		t.Errorf("MaxResults = %d, want 1000", search.MaxResults)
	}
}

func TestConfig_AI(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ai := c.AI()

	if !ai.Enabled {
		t.Error("Enabled = false, want true")
	}
	if ai.Provider != "anthropic" {
		t.Errorf("Provider = %q, want 'anthropic'", ai.Provider)
	}
	if ai.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", ai.MaxTokens)
	}
	if math.Abs(ai.Temperature-0.7) > 1e-6 {
		t.Errorf("Temperature = %f, want 0.7", ai.Temperature)
	}
}

func TestConfig_Logging(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	logging := c.Logging()

	if logging.Level != "info" {
		t.Errorf("Level = %q, want 'info'", logging.Level)
	}
	if logging.Format != "text" {
		t.Errorf("Format = %q, want 'text'", logging.Format)
	}
}

func TestConfig_Input(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	input := c.Input()

	if input.LeaderKey != "<Space>" {
		t.Errorf("LeaderKey = %q, want '<Space>'", input.LeaderKey)
	}
	if input.DefaultMode != "normal" {
		t.Errorf("DefaultMode = %q, want 'normal'", input.DefaultMode)
	}
}

func TestConfig_SectionsWithNoConfig(t *testing.T) {
	// Test that sections return defaults when no config is loaded
	tmpDir := t.TempDir()

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
	)
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// All section accessors should return default values
	editor := c.Editor()
	if editor.TabSize != 4 {
		t.Errorf("Editor.TabSize = %d, want 4 (default)", editor.TabSize)
	}

	ui := c.UI()
	if ui.Theme != "dark" {
		t.Errorf("UI.Theme = %q, want 'dark' (default)", ui.Theme)
	}

	vim := c.Vim()
	if !vim.Enabled {
		t.Error("Vim.Enabled = false, want true (default)")
	}
}

// TestConfig_SnapshotContract tests that returned section structs are snapshots
// and do not affect the underlying configuration when mutated.
func TestConfig_SnapshotContract(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	t.Run("slice mutation does not affect config", func(t *testing.T) {
		// Get files config
		files1 := c.Files()
		originalLen := len(files1.Exclude)

		// Mutate the returned slice
		files1.Exclude = append(files1.Exclude, "mutated-value")
		files1.Exclude[0] = "mutated-first"

		// Get files config again
		files2 := c.Files()

		// The mutation should not have affected the underlying config
		if len(files2.Exclude) != originalLen {
			t.Errorf("Slice mutation affected config: got len %d, want %d", len(files2.Exclude), originalLen)
		}
		if files2.Exclude[0] == "mutated-first" {
			t.Error("Slice element mutation affected config")
		}

		// Verify via direct Get call
		excludeSlice, err := c.GetStringSlice("files.exclude")
		if err != nil {
			t.Fatalf("GetStringSlice error: %v", err)
		}
		if len(excludeSlice) != originalLen {
			t.Errorf("GetStringSlice shows mutation: got len %d, want %d", len(excludeSlice), originalLen)
		}
	})

	t.Run("struct field mutation does not affect config", func(t *testing.T) {
		// Get editor config
		editor1 := c.Editor()
		originalTabSize := editor1.TabSize

		// Mutate the returned struct
		editor1.TabSize = 999

		// Get editor config again
		editor2 := c.Editor()

		// The mutation should not have affected the underlying config
		if editor2.TabSize != originalTabSize {
			t.Errorf("Struct mutation affected config: got TabSize %d, want %d", editor2.TabSize, originalTabSize)
		}

		// Verify via direct Get call
		tabSize, err := c.GetInt("editor.tabSize")
		if err != nil {
			t.Fatalf("GetInt error: %v", err)
		}
		if tabSize != originalTabSize {
			t.Errorf("GetInt shows mutation: got %d, want %d", tabSize, originalTabSize)
		}
	})

	t.Run("multiple calls return independent copies", func(t *testing.T) {
		files1 := c.Files()
		files2 := c.Files()

		// Both should have the same initial values
		if len(files1.Exclude) != len(files2.Exclude) {
			t.Errorf("Initial lengths differ: %d vs %d", len(files1.Exclude), len(files2.Exclude))
		}

		// Mutate files1 by changing an existing element (more robust than append)
		if len(files1.Exclude) > 0 {
			originalValue := files2.Exclude[0]
			files1.Exclude[0] = "mutated-in-files1"

			// files2 should be unaffected - the element should still have original value
			if files2.Exclude[0] != originalValue {
				t.Errorf("Slice element mutation affected other copy: got %q, want %q",
					files2.Exclude[0], originalValue)
			}
		}
	})
}

// TestConfig_TypeErrorLogging tests that type errors are captured for debugging.
func TestConfig_TypeErrorLogging(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with wrong type for tabSize (string instead of int)
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	settingsContent := `
[editor]
tabSize = "not-a-number"
`
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
		WithSchemaValidation(false), // Disable schema validation to test type error handling
	)
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Clear any errors from loading
	c.ClearConfigErrors()

	// Access editor - this should use default for tabSize but log an error
	editor := c.Editor()

	// Should get default value
	if editor.TabSize != 4 {
		t.Errorf("TabSize = %d, want 4 (default due to type error)", editor.TabSize)
	}

	// Should have logged the error
	errors := c.ConfigErrors()
	if errors == nil {
		t.Error("ConfigErrors() returned nil, expected error for editor.tabSize")
	} else if _, ok := errors["editor.tabSize"]; !ok {
		t.Error("ConfigErrors() missing error for editor.tabSize")
	}
}

// TestConfig_ConfigErrorsCopy tests that ConfigErrors returns a copy.
func TestConfig_ConfigErrorsCopy(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Simulate an error being recorded
	c.recordConfigError("test.path", ErrTypeMismatch)

	errors1 := c.ConfigErrors()
	errors2 := c.ConfigErrors()

	// Mutate errors1
	errors1["mutated"] = ErrSettingNotFound

	// errors2 should be unaffected
	if _, ok := errors2["mutated"]; ok {
		t.Error("ConfigErrors() returned shared map, mutation affected other calls")
	}
}

func TestConfig_Integration(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	integration := c.Integration()

	// Test top-level defaults
	if !integration.Enabled {
		t.Error("Enabled = false, want true")
	}
	if integration.MaxProcesses != 10 {
		t.Errorf("MaxProcesses = %d, want 10", integration.MaxProcesses)
	}
	if integration.ShutdownTimeoutSeconds != 30 {
		t.Errorf("ShutdownTimeoutSeconds = %d, want 30", integration.ShutdownTimeoutSeconds)
	}

	// Test git defaults
	if !integration.Git.Enabled {
		t.Error("Git.Enabled = false, want true")
	}
	if integration.Git.AutoFetch {
		t.Error("Git.AutoFetch = true, want false")
	}
	if integration.Git.AutoFetchInterval != 300 {
		t.Errorf("Git.AutoFetchInterval = %d, want 300", integration.Git.AutoFetchInterval)
	}
	if integration.Git.DefaultRemote != "origin" {
		t.Errorf("Git.DefaultRemote = %q, want 'origin'", integration.Git.DefaultRemote)
	}

	// Test debug defaults
	if !integration.Debug.Enabled {
		t.Error("Debug.Enabled = false, want true")
	}
	if !integration.Debug.AutoAttachBreakpoints {
		t.Error("Debug.AutoAttachBreakpoints = false, want true")
	}
	if !integration.Debug.ShowInlineValues {
		t.Error("Debug.ShowInlineValues = false, want true")
	}
	if integration.Debug.StopOnEntry {
		t.Error("Debug.StopOnEntry = true, want false")
	}
	if integration.Debug.Timeout != 30 {
		t.Errorf("Debug.Timeout = %d, want 30", integration.Debug.Timeout)
	}

	// Test debug adapters defaults
	if integration.Debug.Adapters.Delve.Path != "dlv" {
		t.Errorf("Debug.Adapters.Delve.Path = %q, want 'dlv'", integration.Debug.Adapters.Delve.Path)
	}
	if integration.Debug.Adapters.Node.InspectPort != 9229 {
		t.Errorf("Debug.Adapters.Node.InspectPort = %d, want 9229", integration.Debug.Adapters.Node.InspectPort)
	}
	if !integration.Debug.Adapters.Node.SourceMaps {
		t.Error("Debug.Adapters.Node.SourceMaps = false, want true")
	}
	if !integration.Debug.Adapters.Python.JustMyCode {
		t.Error("Debug.Adapters.Python.JustMyCode = false, want true")
	}

	// Test task defaults
	if !integration.Task.Enabled {
		t.Error("Task.Enabled = false, want true")
	}
	if !integration.Task.AutoDetect {
		t.Error("Task.AutoDetect = false, want true")
	}
	if integration.Task.MaxConcurrent != 5 {
		t.Errorf("Task.MaxConcurrent = %d, want 5", integration.Task.MaxConcurrent)
	}
	if integration.Task.OutputBufferSize != 65536 {
		t.Errorf("Task.OutputBufferSize = %d, want 65536", integration.Task.OutputBufferSize)
	}

	// Test task sources defaults
	if !integration.Task.Sources.Makefile {
		t.Error("Task.Sources.Makefile = false, want true")
	}
	if !integration.Task.Sources.PackageJSON {
		t.Error("Task.Sources.PackageJSON = false, want true")
	}
	if !integration.Task.Sources.TasksJSON {
		t.Error("Task.Sources.TasksJSON = false, want true")
	}
	if integration.Task.Sources.Custom {
		t.Error("Task.Sources.Custom = true, want false")
	}

	// Test terminal defaults
	if !integration.Terminal.Enabled {
		t.Error("Terminal.Enabled = false, want true")
	}
	if integration.Terminal.ScrollbackLines != 10000 {
		t.Errorf("Terminal.ScrollbackLines = %d, want 10000", integration.Terminal.ScrollbackLines)
	}
	if !integration.Terminal.CopyOnSelect {
		t.Error("Terminal.CopyOnSelect = false, want true")
	}
	if integration.Terminal.CursorStyle != "block" {
		t.Errorf("Terminal.CursorStyle = %q, want 'block'", integration.Terminal.CursorStyle)
	}
}

func TestConfig_IntegrationWithOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Create settings file with overrides
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	settingsContent := `
[integration]
enabled = false
maxProcesses = 20
shutdownTimeout = 60

[integration.git]
enabled = false
autoFetch = true
autoFetchInterval = 600
signCommits = true
defaultRemote = "upstream"

[integration.debug]
enabled = false
defaultAdapter = "delve"
stopOnEntry = true
timeout = 60

[integration.debug.adapters.delve]
path = "/usr/local/bin/dlv"
buildFlags = "-tags=debug"

[integration.debug.adapters.node]
inspectPort = 9999
sourceMaps = false

[integration.task]
enabled = false
autoDetect = false
maxConcurrent = 10
outputBufferSize = 131072

[integration.task.sources]
makefile = false
packageJson = false
custom = true
customPath = ".myproject/tasks.yaml"

[integration.terminal]
enabled = false
defaultShell = "/bin/zsh"
scrollbackLines = 20000
copyOnSelect = false
cursorStyle = "underline"
fontSize = 16
`
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
	)
	defer c.Close()
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	integration := c.Integration()

	// Test overridden top-level values
	if integration.Enabled {
		t.Error("Enabled = true, want false")
	}
	if integration.MaxProcesses != 20 {
		t.Errorf("MaxProcesses = %d, want 20", integration.MaxProcesses)
	}
	if integration.ShutdownTimeoutSeconds != 60 {
		t.Errorf("ShutdownTimeoutSeconds = %d, want 60", integration.ShutdownTimeoutSeconds)
	}

	// Test overridden git values
	if integration.Git.Enabled {
		t.Error("Git.Enabled = true, want false")
	}
	if !integration.Git.AutoFetch {
		t.Error("Git.AutoFetch = false, want true")
	}
	if integration.Git.AutoFetchInterval != 600 {
		t.Errorf("Git.AutoFetchInterval = %d, want 600", integration.Git.AutoFetchInterval)
	}
	if !integration.Git.SignCommits {
		t.Error("Git.SignCommits = false, want true")
	}
	if integration.Git.DefaultRemote != "upstream" {
		t.Errorf("Git.DefaultRemote = %q, want 'upstream'", integration.Git.DefaultRemote)
	}

	// Test overridden debug values
	if integration.Debug.Enabled {
		t.Error("Debug.Enabled = true, want false")
	}
	if integration.Debug.DefaultAdapter != "delve" {
		t.Errorf("Debug.DefaultAdapter = %q, want 'delve'", integration.Debug.DefaultAdapter)
	}
	if !integration.Debug.StopOnEntry {
		t.Error("Debug.StopOnEntry = false, want true")
	}
	if integration.Debug.Timeout != 60 {
		t.Errorf("Debug.Timeout = %d, want 60", integration.Debug.Timeout)
	}

	// Test overridden debug adapters
	if integration.Debug.Adapters.Delve.Path != "/usr/local/bin/dlv" {
		t.Errorf("Debug.Adapters.Delve.Path = %q, want '/usr/local/bin/dlv'", integration.Debug.Adapters.Delve.Path)
	}
	if integration.Debug.Adapters.Delve.BuildFlags != "-tags=debug" {
		t.Errorf("Debug.Adapters.Delve.BuildFlags = %q, want '-tags=debug'", integration.Debug.Adapters.Delve.BuildFlags)
	}
	if integration.Debug.Adapters.Node.InspectPort != 9999 {
		t.Errorf("Debug.Adapters.Node.InspectPort = %d, want 9999", integration.Debug.Adapters.Node.InspectPort)
	}
	if integration.Debug.Adapters.Node.SourceMaps {
		t.Error("Debug.Adapters.Node.SourceMaps = true, want false")
	}

	// Test overridden task values
	if integration.Task.Enabled {
		t.Error("Task.Enabled = true, want false")
	}
	if integration.Task.AutoDetect {
		t.Error("Task.AutoDetect = true, want false")
	}
	if integration.Task.MaxConcurrent != 10 {
		t.Errorf("Task.MaxConcurrent = %d, want 10", integration.Task.MaxConcurrent)
	}
	if integration.Task.OutputBufferSize != 131072 {
		t.Errorf("Task.OutputBufferSize = %d, want 131072", integration.Task.OutputBufferSize)
	}

	// Test overridden task sources
	if integration.Task.Sources.Makefile {
		t.Error("Task.Sources.Makefile = true, want false")
	}
	if integration.Task.Sources.PackageJSON {
		t.Error("Task.Sources.PackageJSON = true, want false")
	}
	if !integration.Task.Sources.Custom {
		t.Error("Task.Sources.Custom = false, want true")
	}
	if integration.Task.Sources.CustomPath != ".myproject/tasks.yaml" {
		t.Errorf("Task.Sources.CustomPath = %q, want '.myproject/tasks.yaml'", integration.Task.Sources.CustomPath)
	}

	// Test overridden terminal values
	if integration.Terminal.Enabled {
		t.Error("Terminal.Enabled = true, want false")
	}
	if integration.Terminal.DefaultShell != "/bin/zsh" {
		t.Errorf("Terminal.DefaultShell = %q, want '/bin/zsh'", integration.Terminal.DefaultShell)
	}
	if integration.Terminal.ScrollbackLines != 20000 {
		t.Errorf("Terminal.ScrollbackLines = %d, want 20000", integration.Terminal.ScrollbackLines)
	}
	if integration.Terminal.CopyOnSelect {
		t.Error("Terminal.CopyOnSelect = true, want false")
	}
	if integration.Terminal.CursorStyle != "underline" {
		t.Errorf("Terminal.CursorStyle = %q, want 'underline'", integration.Terminal.CursorStyle)
	}
	if integration.Terminal.FontSize != 16 {
		t.Errorf("Terminal.FontSize = %d, want 16", integration.Terminal.FontSize)
	}
}
