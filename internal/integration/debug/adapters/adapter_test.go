package adapters

import (
	"testing"
)

func TestAdapterTypeConstants(t *testing.T) {
	if AdapterDelve != "delve" {
		t.Errorf("AdapterDelve should be 'delve'")
	}
	if AdapterNodeJS != "nodejs" {
		t.Errorf("AdapterNodeJS should be 'nodejs'")
	}
	if AdapterPython != "python" {
		t.Errorf("AdapterPython should be 'python'")
	}
	if AdapterLLDB != "lldb" {
		t.Errorf("AdapterLLDB should be 'lldb'")
	}
	if AdapterGeneric != "generic" {
		t.Errorf("AdapterGeneric should be 'generic'")
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}

	adapters := r.AvailableAdapters()
	if len(adapters) != 3 {
		t.Errorf("expected 3 registered adapters, got %d", len(adapters))
	}
}

func TestRegistry_Create(t *testing.T) {
	r := NewRegistry()

	config := Config{
		Type:    AdapterDelve,
		Name:    "Test Config",
		Request: "launch",
		Program: "/path/to/main.go",
	}

	adapter, err := r.Create(config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if adapter.Type() != AdapterDelve {
		t.Errorf("expected Delve adapter, got %v", adapter.Type())
	}
}

func TestRegistry_Create_Unknown(t *testing.T) {
	r := NewRegistry()

	config := Config{
		Type: "unknown",
	}

	_, err := r.Create(config)
	if err == nil {
		t.Error("expected error for unknown adapter type")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	// Register a custom adapter
	r.Register("custom", func(config Config) (Adapter, error) {
		return &DelveAdapter{}, nil
	})

	adapters := r.AvailableAdapters()
	if len(adapters) != 4 {
		t.Errorf("expected 4 adapters after registration, got %d", len(adapters))
	}
}

func TestDetectAdapterType(t *testing.T) {
	tests := []struct {
		filename string
		expected AdapterType
	}{
		{"main.go", AdapterDelve},
		{"handler_test.go", AdapterDelve},
		{"app.js", AdapterNodeJS},
		{"server.ts", AdapterNodeJS},
		{"module.mjs", AdapterNodeJS},
		{"require.cjs", AdapterNodeJS},
		{"script.py", AdapterPython},
		{"main.c", AdapterLLDB},
		{"program.cpp", AdapterLLDB},
		{"source.cc", AdapterLLDB},
		{"lib.rs", AdapterLLDB},
		{"unknown.xyz", AdapterGeneric},
		{"", AdapterGeneric},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectAdapterType(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectAdapterType(%s) = %s, expected %s", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestHasExtension(t *testing.T) {
	tests := []struct {
		filename   string
		extensions []string
		expected   bool
	}{
		{"file.go", []string{".go"}, true},
		{"file.go", []string{".js", ".go"}, true},
		{"file.py", []string{".go"}, false},
		{"file", []string{".go"}, false},
		{".go", []string{".go"}, false},
	}

	for _, tt := range tests {
		result := hasExtension(tt.filename, tt.extensions...)
		if result != tt.expected {
			t.Errorf("hasExtension(%s, %v) = %v, expected %v", tt.filename, tt.extensions, result, tt.expected)
		}
	}
}

func TestConfig_Fields(t *testing.T) {
	config := Config{
		Type:        AdapterDelve,
		Name:        "Test",
		Request:     "launch",
		Program:     "/path/to/program",
		Module:      "mymodule",
		Args:        []string{"arg1", "arg2"},
		Cwd:         "/working/dir",
		Env:         map[string]string{"KEY": "VALUE"},
		StopOnEntry: true,
		Port:        8080,
		Host:        "localhost",
		ProcessID:   12345,
		AdapterPath: "/path/to/adapter",
		AdapterArgs: []string{"--debug"},
	}

	if config.Type != AdapterDelve {
		t.Error("Type mismatch")
	}
	if config.Name != "Test" {
		t.Error("Name mismatch")
	}
	if config.Request != "launch" {
		t.Error("Request mismatch")
	}
	if config.Program != "/path/to/program" {
		t.Error("Program mismatch")
	}
	if config.Module != "mymodule" {
		t.Error("Module mismatch")
	}
	if len(config.Args) != 2 {
		t.Error("Args length mismatch")
	}
	if config.Cwd != "/working/dir" {
		t.Error("Cwd mismatch")
	}
	if config.Env["KEY"] != "VALUE" {
		t.Error("Env mismatch")
	}
	if !config.StopOnEntry {
		t.Error("StopOnEntry should be true")
	}
	if config.Port != 8080 {
		t.Error("Port mismatch")
	}
	if config.Host != "localhost" {
		t.Error("Host mismatch")
	}
	if config.ProcessID != 12345 {
		t.Error("ProcessID mismatch")
	}
}
