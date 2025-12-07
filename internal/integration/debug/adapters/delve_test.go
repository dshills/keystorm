package adapters

import (
	"testing"
)

func TestNewDelveAdapter(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Name:    "Test Go",
		Request: "launch",
		Program: "/path/to/main.go",
	}

	adapter, err := NewDelveAdapter(config)
	if err != nil {
		t.Fatalf("NewDelveAdapter failed: %v", err)
	}

	if adapter.Type() != AdapterDelve {
		t.Errorf("expected type Delve, got %v", adapter.Type())
	}

	if adapter.Name() != "Delve (Go Debugger)" {
		t.Errorf("unexpected adapter name: %s", adapter.Name())
	}
}

func TestNewDelveAdapterWithConfig(t *testing.T) {
	config := DelveConfig{
		Config: Config{
			Type:    AdapterDelve,
			Request: "launch",
			Program: "/path/to/main.go",
		},
		Mode:                "test",
		BuildFlags:          "-race",
		ShowGlobalVariables: true,
		StackTraceDepth:     100,
	}

	adapter, err := NewDelveAdapterWithConfig(config)
	if err != nil {
		t.Fatalf("NewDelveAdapterWithConfig failed: %v", err)
	}

	if adapter.config.Mode != "test" {
		t.Errorf("expected mode 'test', got %s", adapter.config.Mode)
	}

	if adapter.config.BuildFlags != "-race" {
		t.Errorf("expected buildFlags '-race', got %s", adapter.config.BuildFlags)
	}
}

func TestDelveAdapter_Validate_Launch(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "launch",
		Program: "/path/to/main.go",
	}

	adapter, _ := NewDelveAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid launch config: %v", err)
	}
}

func TestDelveAdapter_Validate_Launch_NoProgram(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "launch",
	}

	adapter, _ := NewDelveAdapter(config)
	err := adapter.Validate()
	if err == nil {
		t.Error("expected error for launch without program")
	}
}

func TestDelveAdapter_Validate_Attach(t *testing.T) {
	config := Config{
		Type:      AdapterDelve,
		Request:   "attach",
		ProcessID: 12345,
	}

	adapter, _ := NewDelveAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid attach config: %v", err)
	}
}

func TestDelveAdapter_Validate_Attach_NoProcessID(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "attach",
	}

	adapter, _ := NewDelveAdapter(config)
	err := adapter.Validate()
	if err == nil {
		t.Error("expected error for attach without processId")
	}
}

func TestDelveAdapter_Validate_InvalidRequest(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "invalid",
	}

	adapter, _ := NewDelveAdapter(config)
	err := adapter.Validate()
	if err == nil {
		t.Error("expected error for invalid request type")
	}
}

func TestDelveAdapter_GetConnectionType(t *testing.T) {
	// Without port - stdio
	config := Config{
		Type:    AdapterDelve,
		Request: "launch",
		Program: "/path/to/main.go",
	}
	adapter, _ := NewDelveAdapter(config)
	if adapter.GetConnectionType() != "stdio" {
		t.Error("expected stdio connection type")
	}

	// With port - socket
	config.Port = 8080
	adapter, _ = NewDelveAdapter(config)
	if adapter.GetConnectionType() != "socket" {
		t.Error("expected socket connection type")
	}
}

func TestDelveAdapter_GetAddress(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "launch",
		Program: "/path/to/main.go",
		Port:    8080,
	}

	adapter, _ := NewDelveAdapter(config)
	addr := adapter.GetAddress()
	if addr != "127.0.0.1:8080" {
		t.Errorf("expected '127.0.0.1:8080', got %s", addr)
	}
}

func TestDelveAdapter_GetAddress_CustomHost(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "launch",
		Program: "/path/to/main.go",
		Port:    8080,
		Host:    "192.168.1.1",
	}

	adapter, _ := NewDelveAdapter(config)
	addr := adapter.GetAddress()
	if addr != "192.168.1.1:8080" {
		t.Errorf("expected '192.168.1.1:8080', got %s", addr)
	}
}

func TestDelveAdapter_GetAddress_NoPort(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "launch",
		Program: "/path/to/main.go",
	}

	adapter, _ := NewDelveAdapter(config)
	addr := adapter.GetAddress()
	if addr != "" {
		t.Errorf("expected empty address, got %s", addr)
	}
}

func TestDelveAdapter_GetLaunchArgs(t *testing.T) {
	config := Config{
		Type:        AdapterDelve,
		Request:     "launch",
		Program:     "/path/to/main.go",
		Args:        []string{"arg1", "arg2"},
		Cwd:         "/working/dir",
		Env:         map[string]string{"KEY": "VALUE"},
		StopOnEntry: true,
	}

	adapter, _ := NewDelveAdapter(config)
	args, err := adapter.GetLaunchArgs()
	if err != nil {
		t.Fatalf("GetLaunchArgs failed: %v", err)
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		t.Fatal("args should be a map")
	}

	if argsMap["program"] != "/path/to/main.go" {
		t.Error("program mismatch")
	}
	if argsMap["mode"] != "debug" {
		t.Error("mode should default to 'debug'")
	}
	if argsMap["stopOnEntry"] != true {
		t.Error("stopOnEntry mismatch")
	}
	if argsMap["cwd"] != "/working/dir" {
		t.Error("cwd mismatch")
	}
}

func TestDelveAdapter_GetAttachArgs(t *testing.T) {
	config := Config{
		Type:        AdapterDelve,
		Request:     "attach",
		ProcessID:   12345,
		Cwd:         "/working/dir",
		StopOnEntry: true,
	}

	adapter, _ := NewDelveAdapter(config)
	args, err := adapter.GetAttachArgs()
	if err != nil {
		t.Fatalf("GetAttachArgs failed: %v", err)
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		t.Fatal("args should be a map")
	}

	if argsMap["mode"] != "local" {
		t.Error("mode should be 'local' for attach")
	}
	if argsMap["processId"] != 12345 {
		t.Error("processId mismatch")
	}
	if argsMap["stopOnEntry"] != true {
		t.Error("stopOnEntry mismatch")
	}
	if argsMap["cwd"] != "/working/dir" {
		t.Error("cwd mismatch")
	}
}

func TestDelveAdapter_SetMethods(t *testing.T) {
	config := Config{
		Type:    AdapterDelve,
		Request: "launch",
		Program: "/path/to/main.go",
	}

	adapter, _ := NewDelveAdapter(config)
	delve := adapter.(*DelveAdapter)

	delve.SetMode("test")
	if delve.config.Mode != "test" {
		t.Error("SetMode failed")
	}

	delve.SetBuildFlags("-race")
	if delve.config.BuildFlags != "-race" {
		t.Error("SetBuildFlags failed")
	}

	delve.SetProgram("/new/path.go")
	if delve.config.Program != "/new/path.go" {
		t.Error("SetProgram failed")
	}

	delve.SetArgs([]string{"a", "b"})
	if len(delve.config.Args) != 2 {
		t.Error("SetArgs failed")
	}
}

func TestCreateDefaultLaunchConfig(t *testing.T) {
	config := CreateDefaultLaunchConfig("/path/to/main.go")

	if config.Type != AdapterDelve {
		t.Error("Type should be Delve")
	}
	if config.Request != "launch" {
		t.Error("Request should be 'launch'")
	}
	if config.Program != "/path/to/main.go" {
		t.Error("Program mismatch")
	}
	if config.Mode != "debug" {
		t.Error("Mode should be 'debug'")
	}
}

func TestCreateDefaultTestConfig(t *testing.T) {
	config := CreateDefaultTestConfig("/path/to/tests", "TestFoo")

	if config.Mode != "test" {
		t.Error("Mode should be 'test'")
	}
	if config.Program != "/path/to/tests" {
		t.Error("Program mismatch")
	}
	if len(config.Args) != 2 || config.Args[0] != "-test.run" || config.Args[1] != "TestFoo" {
		t.Error("Args should contain test run filter")
	}
}

func TestCreateDefaultAttachConfig(t *testing.T) {
	config := CreateDefaultAttachConfig(12345)

	if config.Request != "attach" {
		t.Error("Request should be 'attach'")
	}
	if config.ProcessID != 12345 {
		t.Error("ProcessID mismatch")
	}
	if config.Mode != "local" {
		t.Error("Mode should be 'local'")
	}
}

func TestDelveConfig_Substitutions(t *testing.T) {
	config := DelveConfig{
		Config: Config{
			Type:    AdapterDelve,
			Request: "launch",
			Program: "/path/to/main.go",
		},
		Substitutions: map[string]string{
			"/local/path":  "/remote/path",
			"/local/path2": "/remote/path2",
		},
	}

	adapter, _ := NewDelveAdapterWithConfig(config)
	args, _ := adapter.GetLaunchArgs()

	argsMap := args.(map[string]interface{})
	subs, ok := argsMap["substitutePath"]
	if !ok {
		t.Fatal("substitutePath should be present")
	}

	subsSlice, ok := subs.([]map[string]string)
	if !ok {
		t.Fatal("substitutePath should be a slice of maps")
	}

	if len(subsSlice) != 2 {
		t.Errorf("expected 2 substitutions, got %d", len(subsSlice))
	}
}
