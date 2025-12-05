package plugin

import (
	"os"
	"path/filepath"
	"testing"

	plua "github.com/dshills/keystorm/internal/plugin/lua"
)

func TestLoadManifest(t *testing.T) {
	// Create a temporary manifest file
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "plugin.json")

	content := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"displayName": "Test Plugin",
		"description": "A test plugin",
		"main": "init.lua",
		"capabilities": ["filesystem.read"],
		"commands": [
			{"id": "test.command", "title": "Test Command"}
		]
	}`

	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	m, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if m.Name != "test-plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "test-plugin")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.DisplayName != "Test Plugin" {
		t.Errorf("DisplayName = %q, want %q", m.DisplayName, "Test Plugin")
	}
	if m.Main != "init.lua" {
		t.Errorf("Main = %q, want %q", m.Main, "init.lua")
	}
	if len(m.Capabilities) != 1 || m.Capabilities[0] != plua.CapabilityFileRead {
		t.Errorf("Capabilities = %v, want [%v]", m.Capabilities, plua.CapabilityFileRead)
	}
	if len(m.Commands) != 1 || m.Commands[0].ID != "test.command" {
		t.Errorf("Commands = %v", m.Commands)
	}
}

func TestLoadManifestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "plugin.json")

	if err := os.WriteFile(manifestPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	_, err := LoadManifest(manifestPath)
	if err == nil {
		t.Error("LoadManifest() with invalid JSON should return error")
	}
}

func TestLoadManifestNotFound(t *testing.T) {
	_, err := LoadManifest("/nonexistent/path/plugin.json")
	if err == nil {
		t.Error("LoadManifest() with nonexistent file should return error")
	}
}

func TestLoadManifestFromDir(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "plugin.json")

	content := `{
		"name": "test-plugin",
		"version": "1.0.0"
	}`

	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	m, err := LoadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("LoadManifestFromDir() error = %v", err)
	}

	if m.Name != "test-plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "test-plugin")
	}
}

func TestNewManifestMinimal(t *testing.T) {
	m := NewManifestMinimal("my-plugin", "/path/to/plugin")

	if m.Name != "my-plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "my-plugin")
	}
	if m.Version != "0.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "0.0.0")
	}
	if m.Main != "init.lua" {
		t.Errorf("Main = %q, want %q", m.Main, "init.lua")
	}
	if m.Path() != "/path/to/plugin" {
		t.Errorf("Path() = %q, want %q", m.Path(), "/path/to/plugin")
	}
}

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		wantErr bool
	}{
		{
			name:    "valid",
			m:       Manifest{Name: "test-plugin", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "missing name",
			m:       Manifest{Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "invalid name - uppercase",
			m:       Manifest{Name: "Test-Plugin", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "invalid name - starts with number",
			m:       Manifest{Name: "1plugin", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "missing version",
			m:       Manifest{Name: "test-plugin", Version: ""},
			wantErr: true,
		},
		{
			name:    "invalid version",
			m:       Manifest{Name: "test-plugin", Version: "invalid"},
			wantErr: true,
		},
		{
			name:    "invalid main file",
			m:       Manifest{Name: "test-plugin", Version: "1.0.0", Main: "init.js"},
			wantErr: true,
		},
		{
			name:    "invalid capability",
			m:       Manifest{Name: "test-plugin", Version: "1.0.0", Capabilities: []plua.Capability{"invalid"}},
			wantErr: true,
		},
		{
			name:    "command missing id",
			m:       Manifest{Name: "test-plugin", Version: "1.0.0", Commands: []CommandContribution{{Title: "Test"}}},
			wantErr: true,
		},
		{
			name:    "command missing title",
			m:       Manifest{Name: "test-plugin", Version: "1.0.0", Commands: []CommandContribution{{ID: "test.cmd"}}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManifestValidNamePatterns(t *testing.T) {
	validNames := []string{
		"a",
		"ab",
		"my-plugin",
		"vim-surround",
		"lsp-client",
		"plugin123",
		"a1b2c3",
	}

	for _, name := range validNames {
		m := Manifest{Name: name, Version: "1.0.0"}
		if err := m.Validate(); err != nil {
			t.Errorf("Name %q should be valid, got error: %v", name, err)
		}
	}
}

func TestManifestInvalidNamePatterns(t *testing.T) {
	invalidNames := []string{
		"",
		"-plugin",   // starts with hyphen
		"plugin-",   // ends with hyphen
		"Plugin",    // uppercase
		"PLUGIN",    // all uppercase
		"my_plugin", // underscore
		"my plugin", // space
		"my.plugin", // dot
		"123plugin", // starts with number
		"a-",        // single char then hyphen
	}

	for _, name := range invalidNames {
		m := Manifest{Name: name, Version: "1.0.0"}
		if err := m.Validate(); err == nil {
			t.Errorf("Name %q should be invalid", name)
		}
	}
}

func TestManifestValidVersionPatterns(t *testing.T) {
	validVersions := []string{
		"0.0.0",
		"1.0.0",
		"1.2.3",
		"10.20.30",
		"1.0.0-alpha",
		"1.0.0-beta.1",
		"1.0.0+build.123",
		"1.0.0-rc.1+build.456",
	}

	for _, version := range validVersions {
		m := Manifest{Name: "test", Version: version}
		if err := m.Validate(); err != nil {
			t.Errorf("Version %q should be valid, got error: %v", version, err)
		}
	}
}

func TestManifestInvalidVersionPatterns(t *testing.T) {
	invalidVersions := []string{
		"",
		"1",
		"1.0",
		"v1.0.0",
		"1.0.0.0",
		"a.b.c",
	}

	for _, version := range invalidVersions {
		m := Manifest{Name: "test", Version: version}
		if err := m.Validate(); err == nil {
			t.Errorf("Version %q should be invalid", version)
		}
	}
}

func TestManifestPath(t *testing.T) {
	m := NewManifestMinimal("test", "/path/to/plugin")
	if m.Path() != "/path/to/plugin" {
		t.Errorf("Path() = %q, want %q", m.Path(), "/path/to/plugin")
	}
}

func TestManifestMainPath(t *testing.T) {
	m := NewManifestMinimal("test", "/path/to/plugin")
	expected := filepath.Join("/path/to/plugin", "init.lua")
	if m.MainPath() != expected {
		t.Errorf("MainPath() = %q, want %q", m.MainPath(), expected)
	}
}

func TestManifestHasCapability(t *testing.T) {
	m := &Manifest{
		Name:         "test",
		Version:      "1.0.0",
		Capabilities: []plua.Capability{plua.CapabilityFileRead, plua.CapabilityNetwork},
	}

	if !m.HasCapability(plua.CapabilityFileRead) {
		t.Error("HasCapability(FileRead) = false, want true")
	}
	if !m.HasCapability(plua.CapabilityNetwork) {
		t.Error("HasCapability(Network) = false, want true")
	}
	if m.HasCapability(plua.CapabilityShell) {
		t.Error("HasCapability(Shell) = true, want false")
	}
}

func TestManifestGetConfigDefault(t *testing.T) {
	m := &Manifest{
		Name:    "test",
		Version: "1.0.0",
		ConfigSchema: map[string]ConfigProperty{
			"enabled":   {Type: "boolean", Default: true},
			"count":     {Type: "number", Default: 42.0},
			"nodefault": {Type: "string"},
		},
	}

	// Has default
	val, ok := m.GetConfigDefault("enabled")
	if !ok {
		t.Error("GetConfigDefault(enabled) ok = false")
	}
	if val != true {
		t.Errorf("GetConfigDefault(enabled) = %v, want true", val)
	}

	// No default
	_, ok = m.GetConfigDefault("nodefault")
	if ok {
		t.Error("GetConfigDefault(nodefault) ok = true, want false")
	}

	// Non-existent
	_, ok = m.GetConfigDefault("nonexistent")
	if ok {
		t.Error("GetConfigDefault(nonexistent) ok = true, want false")
	}
}

func TestManifestGetAllConfigDefaults(t *testing.T) {
	m := &Manifest{
		Name:    "test",
		Version: "1.0.0",
		ConfigSchema: map[string]ConfigProperty{
			"enabled":   {Type: "boolean", Default: true},
			"count":     {Type: "number", Default: 42.0},
			"nodefault": {Type: "string"},
		},
	}

	defaults := m.GetAllConfigDefaults()
	if len(defaults) != 2 {
		t.Errorf("GetAllConfigDefaults() len = %d, want 2", len(defaults))
	}
	if defaults["enabled"] != true {
		t.Errorf("defaults[enabled] = %v, want true", defaults["enabled"])
	}
	if defaults["count"] != 42.0 {
		t.Errorf("defaults[count] = %v, want 42.0", defaults["count"])
	}
}

func TestManifestString(t *testing.T) {
	m := &Manifest{Name: "test", Version: "1.0.0", DisplayName: "Test Plugin"}
	expected := "Test Plugin v1.0.0"
	if m.String() != expected {
		t.Errorf("String() = %q, want %q", m.String(), expected)
	}

	// Without display name
	m2 := &Manifest{Name: "test", Version: "1.0.0"}
	expected2 := "test v1.0.0"
	if m2.String() != expected2 {
		t.Errorf("String() = %q, want %q", m2.String(), expected2)
	}
}

func TestManifestClone(t *testing.T) {
	original := &Manifest{
		Name:         "test",
		Version:      "1.0.0",
		Dependencies: []string{"dep1", "dep2"},
		Capabilities: []plua.Capability{plua.CapabilityFileRead},
		Commands:     []CommandContribution{{ID: "cmd1", Title: "Cmd 1"}},
		ConfigSchema: map[string]ConfigProperty{
			"key": {Type: "string", Default: "value"},
		},
	}

	clone := original.Clone()

	// Verify values are equal
	if clone.Name != original.Name {
		t.Errorf("Clone Name = %q, want %q", clone.Name, original.Name)
	}

	// Verify it's a deep copy - modifying clone shouldn't affect original
	clone.Name = "modified"
	if original.Name == "modified" {
		t.Error("Clone is not a deep copy - Name was modified")
	}

	clone.Dependencies[0] = "modified"
	if original.Dependencies[0] == "modified" {
		t.Error("Clone is not a deep copy - Dependencies was modified")
	}
}

func TestManifestApplyDefaults(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "plugin.json")

	// Minimal manifest without Main
	content := `{
		"name": "test-plugin",
		"version": "1.0.0"
	}`

	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	m, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	// Main should default to init.lua
	if m.Main != "init.lua" {
		t.Errorf("Main default = %q, want %q", m.Main, "init.lua")
	}
}
