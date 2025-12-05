package lsp

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.requestTimeout != 10*time.Second {
		t.Errorf("Expected default timeout 10s, got %v", m.requestTimeout)
	}
}

func TestManager_WithOptions(t *testing.T) {
	cb := func(uri DocumentURI, diagnostics []Diagnostic) {
		// Callback for test
	}

	m := NewManager(
		WithRequestTimeout(5*time.Second),
		WithDiagnosticsCallback(cb),
	)

	if m.requestTimeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", m.requestTimeout)
	}

	if m.diagnosticsCb == nil {
		t.Error("Expected diagnostics callback to be set")
	}
}

func TestManager_RegisterServer(t *testing.T) {
	m := NewManager()

	config := ServerConfig{
		Command: "gopls",
		Args:    []string{"serve"},
	}

	m.RegisterServer("go", config)

	langs := m.RegisteredLanguages()
	if len(langs) != 1 || langs[0] != "go" {
		t.Errorf("Expected ['go'], got %v", langs)
	}
}

func TestManager_SetWorkspaceFolders(t *testing.T) {
	m := NewManager()

	folders := []WorkspaceFolder{
		{URI: "file:///test/project", Name: "project"},
	}

	m.SetWorkspaceFolders(folders)

	// Can't directly test m.workspaceFolders, but it should be set
}

func TestManager_ServerStatus_NotRegistered(t *testing.T) {
	m := NewManager()

	status := m.ServerStatus("unknown")
	if status != ServerStatusStopped {
		t.Errorf("Expected ServerStatusStopped, got %v", status)
	}
}

func TestManager_IsAvailable_NoConfig(t *testing.T) {
	m := NewManager()

	// File with unknown extension
	if m.IsAvailable("/test/file.xyz") {
		t.Error("Expected false for unknown file type")
	}

	// Go file with no config
	if m.IsAvailable("/test/file.go") {
		t.Error("Expected false when no server is configured")
	}
}

func TestManager_IsAvailable_WithConfig(t *testing.T) {
	m := NewManager()

	config := ServerConfig{
		Command: "gopls",
		Args:    []string{"serve"},
	}

	m.RegisterServer("go", config)

	if !m.IsAvailable("/test/file.go") {
		t.Error("Expected true when server is configured")
	}
}

func TestManager_ServerForFile_NoConfig(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	_, err := m.ServerForFile(ctx, "/test/file.go")
	if err == nil {
		t.Error("Expected error for unconfigured language")
	}
}

func TestManager_Shutdown_Empty(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	// Shutting down with no servers should not error
	if err := m.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

func TestDefaultServerConfigs(t *testing.T) {
	configs := DefaultServerConfigs()

	// Check for common languages
	expectedLangs := []string{"go", "rust", "typescript", "javascript", "python", "c", "cpp"}
	for _, lang := range expectedLangs {
		if _, ok := configs[lang]; !ok {
			t.Errorf("Expected config for %s", lang)
		}
	}

	// Check Go config
	goConfig := configs["go"]
	if goConfig.Command != "gopls" {
		t.Errorf("Expected gopls for Go, got %s", goConfig.Command)
	}
}

func TestDetectLanguageID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.go", "go"},
		{"/path/to/file.rs", "rust"},
		{"/path/to/file.ts", "typescript"},
		{"/path/to/file.tsx", "typescriptreact"},
		{"/path/to/file.js", "javascript"},
		{"/path/to/file.jsx", "javascriptreact"},
		{"/path/to/file.py", "python"},
		{"/path/to/file.c", "c"},
		{"/path/to/file.cpp", "cpp"},
		{"/path/to/file.unknown", "plaintext"},
	}

	for _, tt := range tests {
		result := DetectLanguageID(tt.path)
		if result != tt.expected {
			t.Errorf("DetectLanguageID(%s) = %s, expected %s", tt.path, result, tt.expected)
		}
	}
}

func TestLanguageIDForExtension(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{"go", "go"},
		{".go", "go"},
		{"rs", "rust"},
		{"ts", "typescript"},
		{"tsx", "typescriptreact"},
		{"py", "python"},
		{"java", "java"},
		{"rb", "ruby"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := LanguageIDForExtension(tt.ext)
		if result != tt.expected {
			t.Errorf("LanguageIDForExtension(%s) = %s, expected %s", tt.ext, result, tt.expected)
		}
	}
}

func TestWorkspaceFolderFromPath(t *testing.T) {
	folder := WorkspaceFolderFromPath("/test/project")

	if folder.Name != "project" {
		t.Errorf("Expected name 'project', got %s", folder.Name)
	}

	// URI should be a file:// URI
	if folder.URI[:7] != "file://" {
		t.Errorf("Expected file:// URI, got %s", folder.URI)
	}
}

func TestFilePathToURI(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.go", "file:///path/to/file.go"},
		{"/path with spaces/file.go", "file:///path%20with%20spaces/file.go"},
	}

	for _, tt := range tests {
		result := FilePathToURI(tt.path)
		if string(result) != tt.expected {
			t.Errorf("FilePathToURI(%s) = %s, expected %s", tt.path, result, tt.expected)
		}
	}
}

func TestURIToFilePath(t *testing.T) {
	tests := []struct {
		uri      DocumentURI
		expected string
	}{
		{"file:///path/to/file.go", "/path/to/file.go"},
		{"file:///path%20with%20spaces/file.go", "/path with spaces/file.go"},
	}

	for _, tt := range tests {
		result := URIToFilePath(tt.uri)
		if result != tt.expected {
			t.Errorf("URIToFilePath(%s) = %s, expected %s", tt.uri, result, tt.expected)
		}
	}
}

func TestServerInfos_Empty(t *testing.T) {
	m := NewManager()

	infos := m.ServerInfos()
	if len(infos) != 0 {
		t.Errorf("Expected empty infos, got %v", infos)
	}
}
