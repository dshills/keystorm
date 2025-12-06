package config

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/config/notify"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	defer c.Close()
}

func TestNew_WithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	c := New(
		WithUserConfigDir(tmpDir),
		WithProjectConfigDir(tmpDir),
		WithWatcher(false),
		WithSchemaValidation(false),
	)
	defer c.Close()

	if c.userConfigDir != tmpDir {
		t.Errorf("userConfigDir = %q, want %q", c.userConfigDir, tmpDir)
	}
	if c.projectConfigDir != tmpDir {
		t.Errorf("projectConfigDir = %q, want %q", c.projectConfigDir, tmpDir)
	}
	if c.enableWatcher {
		t.Error("enableWatcher = true, want false")
	}
	if c.enableSchema {
		t.Error("enableSchema = true, want false")
	}
}

func TestConfig_Load(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user settings file
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	settingsContent := `
[editor]
tabSize = 2
insertSpaces = false

[ui]
theme = "light"
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

	// Check that user settings override defaults
	tabSize, err := c.GetInt("editor.tabSize")
	if err != nil {
		t.Errorf("GetInt('editor.tabSize') error = %v", err)
	}
	if tabSize != 2 {
		t.Errorf("editor.tabSize = %d, want 2", tabSize)
	}

	insertSpaces, err := c.GetBool("editor.insertSpaces")
	if err != nil {
		t.Errorf("GetBool('editor.insertSpaces') error = %v", err)
	}
	if insertSpaces {
		t.Error("editor.insertSpaces = true, want false")
	}

	theme, err := c.GetString("ui.theme")
	if err != nil {
		t.Errorf("GetString('ui.theme') error = %v", err)
	}
	if theme != "light" {
		t.Errorf("ui.theme = %q, want 'light'", theme)
	}
}

func TestConfig_Get(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	_ = c.Load(context.Background())

	// Get existing value
	v, ok := c.Get("editor.tabSize")
	if !ok {
		t.Error("Get('editor.tabSize') not found")
	}
	if v != 4 {
		t.Errorf("editor.tabSize = %v, want 4", v)
	}

	// Get non-existent value
	_, ok = c.Get("nonexistent.path")
	if ok {
		t.Error("Get('nonexistent.path') should not be found")
	}
}

func TestConfig_GetString(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	_ = c.Load(context.Background())

	s, err := c.GetString("ui.theme")
	if err != nil {
		t.Errorf("GetString('ui.theme') error = %v", err)
	}
	if s != "dark" {
		t.Errorf("ui.theme = %q, want 'dark'", s)
	}

	// Wrong type
	_, err = c.GetString("editor.tabSize")
	if err == nil {
		t.Error("GetString('editor.tabSize') should return error for int")
	}

	// Not found
	_, err = c.GetString("nonexistent")
	if err != ErrSettingNotFound {
		t.Errorf("GetString('nonexistent') error = %v, want ErrSettingNotFound", err)
	}
}

func TestConfig_GetInt(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	_ = c.Load(context.Background())

	i, err := c.GetInt("editor.tabSize")
	if err != nil {
		t.Errorf("GetInt('editor.tabSize') error = %v", err)
	}
	if i != 4 {
		t.Errorf("editor.tabSize = %d, want 4", i)
	}

	// Wrong type
	_, err = c.GetInt("ui.theme")
	if err == nil {
		t.Error("GetInt('ui.theme') should return error for string")
	}
}

func TestConfig_GetBool(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	_ = c.Load(context.Background())

	b, err := c.GetBool("editor.insertSpaces")
	if err != nil {
		t.Errorf("GetBool('editor.insertSpaces') error = %v", err)
	}
	if !b {
		t.Error("editor.insertSpaces = false, want true")
	}

	// Wrong type
	_, err = c.GetBool("editor.tabSize")
	if err == nil {
		t.Error("GetBool('editor.tabSize') should return error for int")
	}
}

func TestConfig_GetFloat(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	_ = c.Load(context.Background())

	f, err := c.GetFloat("ai.temperature")
	if err != nil {
		t.Errorf("GetFloat('ai.temperature') error = %v", err)
	}
	if f != 0.7 {
		t.Errorf("ai.temperature = %f, want 0.7", f)
	}
}

func TestConfig_GetStringSlice(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	_ = c.Load(context.Background())

	s, err := c.GetStringSlice("files.exclude")
	if err != nil {
		t.Errorf("GetStringSlice('files.exclude') error = %v", err)
	}
	if len(s) != 3 {
		t.Errorf("files.exclude length = %d, want 3", len(s))
	}

	// Wrong type
	_, err = c.GetStringSlice("editor.tabSize")
	if err == nil {
		t.Error("GetStringSlice('editor.tabSize') should return error for int")
	}
}

func TestConfig_Set(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user settings file so the layer exists
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
		WithSchemaValidation(false), // Disable for simpler testing
	)
	defer c.Close()
	_ = c.Load(context.Background())

	// Set a value
	if err := c.Set("editor.tabSize", 8); err != nil {
		t.Errorf("Set() error = %v", err)
	}

	// Verify the change
	v, err := c.GetInt("editor.tabSize")
	if err != nil {
		t.Errorf("GetInt() error = %v", err)
	}
	if v != 8 {
		t.Errorf("editor.tabSize = %d, want 8", v)
	}
}

func TestConfig_Subscribe(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user settings file
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
		WithSchemaValidation(false),
	)
	defer c.Close()
	_ = c.Load(context.Background())

	var received atomic.Bool
	var receivedChange notify.Change

	sub := c.Subscribe(func(change notify.Change) {
		receivedChange = change
		received.Store(true)
	})
	defer sub.Unsubscribe()

	_ = c.Set("editor.tabSize", 2)

	if !received.Load() {
		t.Error("observer did not receive notification")
	}
	if receivedChange.Path != "editor.tabSize" {
		t.Errorf("change.Path = %q, want 'editor.tabSize'", receivedChange.Path)
	}
	if receivedChange.NewValue != 2 {
		t.Errorf("change.NewValue = %v, want 2", receivedChange.NewValue)
	}
}

func TestConfig_SubscribePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user settings file
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 4\n[ui]\ntheme = \"dark\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
		WithSchemaValidation(false),
	)
	defer c.Close()
	_ = c.Load(context.Background())

	var editorCount, uiCount atomic.Int32

	editorSub := c.SubscribePath("editor", func(change notify.Change) {
		editorCount.Add(1)
	})
	defer editorSub.Unsubscribe()

	uiSub := c.SubscribePath("ui", func(change notify.Change) {
		uiCount.Add(1)
	})
	defer uiSub.Unsubscribe()

	_ = c.Set("editor.tabSize", 2)
	_ = c.Set("ui.theme", "light")

	if editorCount.Load() != 1 {
		t.Errorf("editor observer received %d changes, want 1", editorCount.Load())
	}
	if uiCount.Load() != 1 {
		t.Errorf("ui observer received %d changes, want 1", uiCount.Load())
	}
}

func TestConfig_Merged(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()
	_ = c.Load(context.Background())

	merged := c.Merged()
	if merged == nil {
		t.Fatal("Merged() returned nil")
	}

	editor, ok := merged["editor"].(map[string]any)
	if !ok {
		t.Fatal("merged[editor] is not a map")
	}
	if editor["tabSize"] != 4 {
		t.Errorf("merged[editor][tabSize] = %v, want 4", editor["tabSize"])
	}
}

func TestConfig_FileWatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user settings file
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(true),
		WithSchemaValidation(false),
	)
	defer c.Close()
	_ = c.Load(context.Background())

	var reloadReceived atomic.Bool

	c.Subscribe(func(change notify.Change) {
		if change.Type == notify.ChangeReload {
			reloadReceived.Store(true)
		}
	})

	// Modify the file
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 8\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for the watcher to detect the change
	deadline := time.Now().Add(2 * time.Second)
	for !reloadReceived.Load() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	if !reloadReceived.Load() {
		t.Error("did not receive reload notification")
	}

	// Check that the new value is loaded
	tabSize, _ := c.GetInt("editor.tabSize")
	if tabSize != 8 {
		t.Errorf("editor.tabSize = %d, want 8 after reload", tabSize)
	}
}

func TestGetPath(t *testing.T) {
	m := map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
			"font": map[string]any{
				"family": "monospace",
			},
		},
	}

	// Test scalar values
	got, ok := getPath(m, "editor.tabSize")
	if !ok || got != 4 {
		t.Errorf("getPath('editor.tabSize') = %v, %v, want 4, true", got, ok)
	}

	got, ok = getPath(m, "editor.font.family")
	if !ok || got != "monospace" {
		t.Errorf("getPath('editor.font.family') = %v, %v, want 'monospace', true", got, ok)
	}

	// Test map value (just check it's found and is a map)
	got, ok = getPath(m, "editor")
	if !ok {
		t.Error("getPath('editor') should be found")
	}
	if _, isMap := got.(map[string]any); !isMap {
		t.Error("getPath('editor') should return a map")
	}

	// Test not found
	_, ok = getPath(m, "nonexistent")
	if ok {
		t.Error("getPath('nonexistent') should not be found")
	}

	_, ok = getPath(m, "editor.nonexistent")
	if ok {
		t.Error("getPath('editor.nonexistent') should not be found")
	}

	_, ok = getPath(m, "")
	if ok {
		t.Error("getPath('') should not be found")
	}
}

func TestSetPath(t *testing.T) {
	m := map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
		},
	}

	// Set existing path
	if err := setPath(m, "editor.tabSize", 8); err != nil {
		t.Errorf("setPath() error = %v", err)
	}
	if m["editor"].(map[string]any)["tabSize"] != 8 {
		t.Error("setPath() did not update value")
	}

	// Set new path with nesting
	if err := setPath(m, "ui.theme.color", "dark"); err != nil {
		t.Errorf("setPath() error = %v", err)
	}
	ui := m["ui"].(map[string]any)
	theme := ui["theme"].(map[string]any)
	if theme["color"] != "dark" {
		t.Error("setPath() did not create nested path")
	}

	// Invalid path
	if err := setPath(m, "", "value"); err != ErrInvalidPath {
		t.Errorf("setPath('') error = %v, want ErrInvalidPath", err)
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"editor.tabSize", []string{"editor", "tabSize"}},
		{"a.b.c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"", nil},
		{"a..b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		got := splitPath(tt.path)
		if len(got) != len(tt.want) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.path, got, tt.want)
			continue
		}
		for i, v := range got {
			if v != tt.want[i] {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.path, i, v, tt.want[i])
			}
		}
	}
}

func TestTypeName(t *testing.T) {
	tests := []struct {
		v    any
		want string
	}{
		{nil, "nil"},
		{"hello", "string"},
		{42, "int"},
		{int64(42), "int"},
		{3.14, "float64"},
		{true, "bool"},
		{[]string{"a"}, "[]string"},
		{[]any{1, 2}, "[]any"},
		{map[string]any{}, "map"},
		{struct{}{}, "unknown"},
	}

	for _, tt := range tests {
		got := typeName(tt.v)
		if got != tt.want {
			t.Errorf("typeName(%T) = %q, want %q", tt.v, got, tt.want)
		}
	}
}
