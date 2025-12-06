package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/config/notify"
)

func TestConfigSystem_New(t *testing.T) {
	tmpDir := t.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	if sys.Config() == nil {
		t.Error("Config() returned nil")
	}

	if sys.LoadTime() == 0 {
		t.Error("LoadTime() returned 0")
	}

	if sys.LastReloadAt().IsZero() {
		t.Error("LastReloadAt() returned zero time")
	}
}

func TestConfigSystem_TypedAccessors(t *testing.T) {
	tmpDir := t.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	// Test all section accessors
	editor := sys.Editor()
	if editor.TabSize != 4 {
		t.Errorf("Editor().TabSize = %d, want 4", editor.TabSize)
	}

	ui := sys.UI()
	if ui.Theme != "dark" {
		t.Errorf("UI().Theme = %q, want 'dark'", ui.Theme)
	}

	vim := sys.Vim()
	if !vim.Enabled {
		t.Error("Vim().Enabled = false, want true")
	}

	input := sys.Input()
	if input.LeaderKey != "<Space>" {
		t.Errorf("Input().LeaderKey = %q, want '<Space>'", input.LeaderKey)
	}

	files := sys.Files()
	if files.Encoding != "utf-8" {
		t.Errorf("Files().Encoding = %q, want 'utf-8'", files.Encoding)
	}

	search := sys.Search()
	if search.MaxResults != 1000 {
		t.Errorf("Search().MaxResults = %d, want 1000", search.MaxResults)
	}

	ai := sys.AI()
	if !ai.Enabled {
		t.Error("AI().Enabled = false, want true")
	}

	logging := sys.Logging()
	if logging.Level != "info" {
		t.Errorf("Logging().Level = %q, want 'info'", logging.Level)
	}

	terminal := sys.Terminal()
	if terminal.FontSize != 14 {
		t.Errorf("Terminal().FontSize = %d, want 14", terminal.FontSize)
	}

	lsp := sys.LSP()
	if !lsp.Enabled {
		t.Error("LSP().Enabled = false, want true")
	}

	paths := sys.Paths()
	// Paths are empty by default
	if paths.ConfigDir != "" {
		t.Errorf("Paths().ConfigDir = %q, want empty", paths.ConfigDir)
	}
}

func TestConfigSystem_Health(t *testing.T) {
	tmpDir := t.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	health := sys.Health()

	if health.Status != HealthOK {
		t.Errorf("Health().Status = %v, want HealthOK", health.Status)
	}

	if health.ErrorCount != 0 {
		t.Errorf("Health().ErrorCount = %d, want 0", health.ErrorCount)
	}
}

func TestConfigSystem_Reload(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial settings
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	// Check initial value
	tabSize, _ := sys.GetInt("editor.tabSize")
	if tabSize != 2 {
		t.Errorf("initial editor.tabSize = %d, want 2", tabSize)
	}

	// Modify file
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 8\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Reload
	if err := sys.Reload(context.Background()); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	// Check new value
	tabSize, _ = sys.GetInt("editor.tabSize")
	if tabSize != 8 {
		t.Errorf("after reload editor.tabSize = %d, want 8", tabSize)
	}
}

func TestConfigSystem_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user settings
	settingsContent := `
[editor]
tabSize = 2
insertSpaces = false
formatOnSave = true

[ui]
theme = "light"
fontSize = 16

[vim]
enabled = true
relativeLineNumbers = true

[ai]
provider = "openai"
model = "gpt-4"
temperature = 0.5
`
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create keymaps
	keymapsContent := `
[[keymaps]]
name = "user-custom"
mode = "normal"
priority = 100

[[keymaps.bindings]]
keys = "<Space> f"
action = "file.find"
description = "Find files"
`
	keymapsPath := filepath.Join(tmpDir, "keymaps.toml")
	if err := os.WriteFile(keymapsPath, []byte(keymapsContent), 0644); err != nil {
		t.Fatal(err)
	}

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	// Verify editor settings
	editor := sys.Editor()
	if editor.TabSize != 2 {
		t.Errorf("Editor().TabSize = %d, want 2", editor.TabSize)
	}
	if editor.InsertSpaces {
		t.Error("Editor().InsertSpaces = true, want false")
	}
	if !editor.FormatOnSave {
		t.Error("Editor().FormatOnSave = false, want true")
	}

	// Verify UI settings
	ui := sys.UI()
	if ui.Theme != "light" {
		t.Errorf("UI().Theme = %q, want 'light'", ui.Theme)
	}
	if ui.FontSize != 16 {
		t.Errorf("UI().FontSize = %d, want 16", ui.FontSize)
	}

	// Verify vim settings
	vim := sys.Vim()
	if !vim.Enabled {
		t.Error("Vim().Enabled = false, want true")
	}
	if !vim.RelativeLineNumbers {
		t.Error("Vim().RelativeLineNumbers = false, want true")
	}

	// Verify AI settings
	ai := sys.AI()
	if ai.Provider != "openai" {
		t.Errorf("AI().Provider = %q, want 'openai'", ai.Provider)
	}
	if ai.Model != "gpt-4" {
		t.Errorf("AI().Model = %q, want 'gpt-4'", ai.Model)
	}
	if ai.Temperature != 0.5 {
		t.Errorf("AI().Temperature = %f, want 0.5", ai.Temperature)
	}

	// Verify keymaps loaded
	km := sys.Keymaps()
	if km == nil {
		t.Fatal("Keymaps() returned nil")
	}

	reg := km.Registry()
	if reg == nil {
		t.Fatal("Registry() returned nil")
	}

	userKm := reg.Get("user-custom")
	if userKm == nil {
		t.Error("user-custom keymap not found")
	}
}

func TestConfigSystem_Subscription(t *testing.T) {
	tmpDir := t.TempDir()

	// Create settings file so we can modify it
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
		WithSystemSchemaValidation(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	var changes []notify.Change
	var mu sync.Mutex

	sub := sys.Subscribe(func(change notify.Change) {
		mu.Lock()
		changes = append(changes, change)
		mu.Unlock()
	})
	defer sub.Unsubscribe()

	// Make changes
	_ = sys.Set("editor.tabSize", 2)
	_ = sys.Set("ui.theme", "light")

	mu.Lock()
	count := len(changes)
	mu.Unlock()

	if count != 2 {
		t.Errorf("received %d changes, want 2", count)
	}
}

func TestConfigSystem_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create settings
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
		WithSystemSchemaValidation(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sys.Editor()
			_ = sys.UI()
			_ = sys.Vim()
			_, _ = sys.GetInt("editor.tabSize")
			_ = sys.Merged()
		}()
	}

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = sys.Set("editor.tabSize", i%10+1)
		}(i)
	}

	wg.Wait()
}

func TestMigrator_Basic(t *testing.T) {
	m := NewMigrator(Version{1, 0, 0})

	// Add a simple migration
	m.Register(Migration{
		FromVersion: Version{0, 0, 0},
		ToVersion:   Version{1, 0, 0},
		Description: "Initial migration",
		Migrate: func(data map[string]any) (map[string]any, error) {
			// Rename old.setting to new.setting
			if old, ok := data["old"]; ok {
				data["new"] = old
				delete(data, "old")
			}
			return data, nil
		},
	})

	// Test migration
	data := map[string]any{
		"old": map[string]any{
			"value": 42,
		},
	}

	migrated, results, err := m.Migrate(data)
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("got %d migration results, want 1", len(results))
	}

	if !results[0].Success {
		t.Error("migration should have succeeded")
	}

	if _, ok := migrated["old"]; ok {
		t.Error("old key should have been removed")
	}

	newVal, ok := migrated["new"]
	if !ok {
		t.Error("new key should exist")
	}
	if newMap, ok := newVal.(map[string]any); !ok || newMap["value"] != 42 {
		t.Error("new value should be migrated correctly")
	}
}

func TestMigrator_NeedsMigration(t *testing.T) {
	m := NewMigrator(Version{1, 0, 0})

	// No version = needs migration
	data := map[string]any{}
	if !m.NeedsMigration(data) {
		t.Error("data without version should need migration")
	}

	// Old version = needs migration
	data = map[string]any{"_version": "0.9.0"}
	if !m.NeedsMigration(data) {
		t.Error("old version should need migration")
	}

	// Current version = no migration needed
	data = map[string]any{"_version": "1.0.0"}
	if m.NeedsMigration(data) {
		t.Error("current version should not need migration")
	}
}

func TestMigrationHelpers(t *testing.T) {
	// Test MigrationRename
	rename := MigrationRename(
		Version{0, 0, 0},
		Version{1, 0, 0},
		"old.path",
		"new.path",
		"Rename old.path to new.path",
	)

	data := map[string]any{
		"old": map[string]any{
			"path": "value",
		},
	}

	migrated, err := rename.Migrate(data)
	if err != nil {
		t.Fatalf("MigrationRename.Migrate() error = %v", err)
	}

	if _, ok := migrated["old"].(map[string]any)["path"]; ok {
		t.Error("old.path should have been removed")
	}

	newPath, ok := migrated["new"].(map[string]any)["path"]
	if !ok || newPath != "value" {
		t.Error("new.path should have the value")
	}

	// Test MigrationTransform
	transform := MigrationTransform(
		Version{0, 0, 0},
		Version{1, 0, 0},
		"value",
		"Double the value",
		func(v any) (any, error) {
			if i, ok := v.(int); ok {
				return i * 2, nil
			}
			return v, nil
		},
	)

	data = map[string]any{"value": 21}
	migrated, err = transform.Migrate(data)
	if err != nil {
		t.Fatalf("MigrationTransform.Migrate() error = %v", err)
	}

	if migrated["value"] != 42 {
		t.Errorf("value = %v, want 42", migrated["value"])
	}

	// Test MigrationDelete
	del := MigrationDelete(
		Version{0, 0, 0},
		Version{1, 0, 0},
		"deprecated",
		"Remove deprecated setting",
	)

	data = map[string]any{"deprecated": "old", "keep": "this"}
	migrated, err = del.Migrate(data)
	if err != nil {
		t.Fatalf("MigrationDelete.Migrate() error = %v", err)
	}

	if _, ok := migrated["deprecated"]; ok {
		t.Error("deprecated should have been deleted")
	}
	if migrated["keep"] != "this" {
		t.Error("keep should still exist")
	}
}

func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		status HealthStatus
		want   string
	}{
		{HealthOK, "ok"},
		{HealthDegraded, "degraded"},
		{HealthUnhealthy, "unhealthy"},
		{HealthStatus(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("HealthStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		a, b Version
		want int
	}{
		{Version{1, 0, 0}, Version{1, 0, 0}, 0},
		{Version{1, 0, 0}, Version{2, 0, 0}, -1},
		{Version{2, 0, 0}, Version{1, 0, 0}, 1},
		{Version{1, 1, 0}, Version{1, 0, 0}, 1},
		{Version{1, 0, 1}, Version{1, 0, 0}, 1},
		{Version{0, 9, 9}, Version{1, 0, 0}, -1},
	}

	for _, tt := range tests {
		got := tt.a.Compare(tt.b)
		if got != tt.want {
			t.Errorf("%s.Compare(%s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// Benchmark tests for performance optimization
func BenchmarkConfigSystem_Get(b *testing.B) {
	tmpDir := b.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		b.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sys.Get("editor.tabSize")
	}
}

func BenchmarkConfigSystem_GetInt(b *testing.B) {
	tmpDir := b.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		b.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sys.GetInt("editor.tabSize")
	}
}

func BenchmarkConfigSystem_Editor(b *testing.B) {
	tmpDir := b.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		b.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sys.Editor()
	}
}

func BenchmarkConfigSystem_Merged(b *testing.B) {
	tmpDir := b.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		b.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sys.Merged()
	}
}

func BenchmarkConfigSystem_ConcurrentReads(b *testing.B) {
	tmpDir := b.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		b.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = sys.Editor()
			_, _ = sys.GetInt("editor.tabSize")
		}
	})
}

func TestConfigSystem_LoadTimePerformance(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a moderately sized config
	settingsContent := `
[editor]
tabSize = 4
insertSpaces = true
wordWrap = "off"
lineNumbers = "on"
formatOnSave = false

[ui]
theme = "dark"
fontSize = 14
fontFamily = "monospace"
showMinimap = true

[vim]
enabled = true
startInInsertMode = false
relativeLineNumbers = false

[files]
encoding = "utf-8"
eol = "lf"
autoSave = "off"

[search]
caseSensitive = false
wholeWord = false
maxResults = 1000

[ai]
enabled = true
provider = "anthropic"
model = "claude-sonnet-4-20250514"
temperature = 0.7
maxTokens = 4096

[logging]
level = "info"
format = "text"
`
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	loadTime := time.Since(start)

	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	// Load time should be under 50ms (success criteria from plan)
	if loadTime > 50*time.Millisecond {
		t.Errorf("Load time = %v, want < 50ms", loadTime)
	}

	t.Logf("Config load time: %v", loadTime)
}

func TestConfigSystem_ClosedBehavior(t *testing.T) {
	tmpDir := t.TempDir()

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(false),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}

	// Close the system
	sys.Close()

	// Verify Config() returns nil after close
	if sys.Config() != nil {
		t.Error("Config() should return nil after Close()")
	}

	// Verify Reload returns error after close
	if err := sys.Reload(context.Background()); err != ErrSystemClosed {
		t.Errorf("Reload() after close = %v, want ErrSystemClosed", err)
	}

	// Verify Set returns error after close
	if err := sys.Set("editor.tabSize", 4); err != ErrSystemClosed {
		t.Errorf("Set() after close = %v, want ErrSystemClosed", err)
	}

	// Verify Subscribe returns nil after close
	if sub := sys.Subscribe(func(notify.Change) {}); sub != nil {
		t.Error("Subscribe() should return nil after Close()")
	}

	// Verify SubscribePath returns nil after close
	if sub := sys.SubscribePath("editor", func(notify.Change) {}); sub != nil {
		t.Error("SubscribePath() should return nil after Close()")
	}

	// Verify Close is idempotent (no panic)
	sys.Close()
}

func TestMigrator_NewerVersionError(t *testing.T) {
	m := NewMigrator(Version{1, 0, 0})

	// Try to migrate data with a newer version
	data := map[string]any{"_version": "2.0.0"}
	_, _, err := m.Migrate(data)

	if err == nil {
		t.Error("Migrate() should return error for newer version")
	}
	if !errors.Is(err, ErrNewerVersion) {
		t.Errorf("Migrate() error = %v, want ErrNewerVersion", err)
	}
}

func TestMigrator_MigrationGapError(t *testing.T) {
	m := NewMigrator(Version{2, 0, 0})

	// Register migration from 1.0.0 to 2.0.0 but not from 0.0.0 to 1.0.0
	m.Register(Migration{
		FromVersion: Version{1, 0, 0},
		ToVersion:   Version{2, 0, 0},
		Description: "Skip migration",
		Migrate: func(data map[string]any) (map[string]any, error) {
			return data, nil
		},
	})

	// Try to migrate from 0.0.0 - should fail due to gap
	data := map[string]any{}
	_, _, err := m.Migrate(data)

	if err == nil {
		t.Error("Migrate() should return error for migration gap")
	}
	if !errors.Is(err, ErrMigrationGap) {
		t.Errorf("Migrate() error = %v, want ErrMigrationGap", err)
	}
}

func TestMigrator_AlreadyAtCurrentVersion(t *testing.T) {
	m := NewMigrator(Version{1, 0, 0})

	// Data is already at current version
	data := map[string]any{"_version": "1.0.0", "value": "unchanged"}
	migrated, results, err := m.Migrate(data)

	if err != nil {
		t.Errorf("Migrate() error = %v, want nil", err)
	}
	if len(results) != 0 {
		t.Errorf("Migrate() returned %d results, want 0", len(results))
	}
	if migrated["value"] != "unchanged" {
		t.Error("Data should be unchanged when already at current version")
	}
}

func TestConfigSystem_FileWatchReload(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial settings
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sys, err := NewConfigSystem(context.Background(),
		WithSystemUserConfigDir(tmpDir),
		WithSystemWatcher(true),
	)
	if err != nil {
		t.Fatalf("NewConfigSystem() error = %v", err)
	}
	defer sys.Close()

	var reloadReceived atomic.Bool

	sub := sys.Subscribe(func(change notify.Change) {
		if change.Type == notify.ChangeReload {
			reloadReceived.Store(true)
		}
	})
	defer sub.Unsubscribe()

	// Modify file
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(settingsPath, []byte("[editor]\ntabSize = 8\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for watcher
	deadline := time.Now().Add(2 * time.Second)
	for !reloadReceived.Load() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	if !reloadReceived.Load() {
		t.Error("did not receive reload notification")
	}
}
