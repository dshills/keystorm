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
	if math.Abs(ai.Temperature-0.7) > 1e-9 {
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
