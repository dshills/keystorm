package adapters

import (
	"testing"
)

func TestNewPythonAdapter(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Name:    "Test Python",
		Request: "launch",
		Program: "/path/to/script.py",
	}

	adapter, err := NewPythonAdapter(config)
	if err != nil {
		t.Fatalf("NewPythonAdapter failed: %v", err)
	}

	if adapter.Type() != AdapterPython {
		t.Errorf("expected type Python, got %v", adapter.Type())
	}

	if adapter.Name() != "Python Debugger (debugpy)" {
		t.Errorf("unexpected adapter name: %s", adapter.Name())
	}
}

func TestNewPythonAdapterWithConfig(t *testing.T) {
	config := PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Request: "launch",
			Program: "/path/to/script.py",
		},
		Console:    "integratedTerminal",
		JustMyCode: false,
		Django:     true,
	}

	adapter, err := NewPythonAdapterWithConfig(config)
	if err != nil {
		t.Fatalf("NewPythonAdapterWithConfig failed: %v", err)
	}

	if adapter.config.Console != "integratedTerminal" {
		t.Errorf("expected console 'integratedTerminal', got %s", adapter.config.Console)
	}

	if adapter.config.JustMyCode {
		t.Error("JustMyCode should be false")
	}

	if !adapter.config.Django {
		t.Error("Django should be true")
	}
}

func TestPythonAdapter_Validate_Launch(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
		Program: "/path/to/script.py",
	}

	adapter, _ := NewPythonAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid launch config: %v", err)
	}
}

func TestPythonAdapter_Validate_Launch_WithModule(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
		Module:  "flask",
	}

	adapter, _ := NewPythonAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid launch config with module: %v", err)
	}
}

func TestPythonAdapter_Validate_Launch_NoProgram(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
	}

	adapter, _ := NewPythonAdapter(config)
	err := adapter.Validate()
	if err == nil {
		t.Error("expected error for launch without program or module")
	}
}

func TestPythonAdapter_Validate_Attach(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "attach",
		Port:    5678,
	}

	adapter, _ := NewPythonAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid attach config: %v", err)
	}
}

func TestPythonAdapter_Validate_Attach_WithProcessID(t *testing.T) {
	config := Config{
		Type:      AdapterPython,
		Request:   "attach",
		ProcessID: 12345,
	}

	adapter, _ := NewPythonAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid attach config with processId: %v", err)
	}
}

func TestPythonAdapter_Validate_Attach_NoPortOrProcess(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "attach",
	}

	adapter, _ := NewPythonAdapter(config)
	err := adapter.Validate()
	if err == nil {
		t.Error("expected error for attach without port or processId")
	}
}

func TestPythonAdapter_GetConnectionType(t *testing.T) {
	// With port - socket
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
		Program: "/path/to/script.py",
		Port:    5678,
	}
	adapter, _ := NewPythonAdapter(config)
	if adapter.GetConnectionType() != "socket" {
		t.Error("expected socket connection type with port")
	}

	// Without port - stdio
	config.Port = 0
	adapter, _ = NewPythonAdapter(config)
	if adapter.GetConnectionType() != "stdio" {
		t.Error("expected stdio connection type without port")
	}
}

func TestPythonAdapter_GetAddress(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
		Program: "/path/to/script.py",
		Port:    5678,
	}

	adapter, _ := NewPythonAdapter(config)
	addr := adapter.GetAddress()
	if addr != "127.0.0.1:5678" {
		t.Errorf("expected '127.0.0.1:5678', got %s", addr)
	}
}

func TestPythonAdapter_GetAddress_NoPort(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
		Program: "/path/to/script.py",
	}

	adapter, _ := NewPythonAdapter(config)
	addr := adapter.GetAddress()
	if addr != "" {
		t.Errorf("expected empty address, got %s", addr)
	}
}

func TestPythonAdapter_GetLaunchArgs(t *testing.T) {
	config := Config{
		Type:        AdapterPython,
		Request:     "launch",
		Program:     "/path/to/script.py",
		Args:        []string{"--arg1", "--arg2"},
		Cwd:         "/working/dir",
		Env:         map[string]string{"PYTHON_ENV": "development"},
		StopOnEntry: true,
	}

	adapter, _ := NewPythonAdapter(config)
	args, err := adapter.GetLaunchArgs()
	if err != nil {
		t.Fatalf("GetLaunchArgs failed: %v", err)
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		t.Fatal("args should be a map")
	}

	if argsMap["type"] != "python" {
		t.Error("type should be 'python'")
	}
	if argsMap["request"] != "launch" {
		t.Error("request should be 'launch'")
	}
	if argsMap["program"] != "/path/to/script.py" {
		t.Error("program mismatch")
	}
	if argsMap["stopOnEntry"] != true {
		t.Error("stopOnEntry mismatch")
	}
	if argsMap["justMyCode"] != true {
		t.Error("justMyCode should be true by default")
	}
	if argsMap["redirectOutput"] != true {
		t.Error("redirectOutput should be true by default")
	}
}

func TestPythonAdapter_GetLaunchArgs_WithModule(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
		Module:  "flask",
		Args:    []string{"run"},
	}

	adapter, _ := NewPythonAdapter(config)
	args, _ := adapter.GetLaunchArgs()

	argsMap := args.(map[string]interface{})

	if argsMap["module"] != "flask" {
		t.Error("module mismatch")
	}
}

func TestPythonAdapter_GetAttachArgs(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "attach",
		Port:    5678,
		Host:    "localhost",
	}

	adapter, _ := NewPythonAdapter(config)
	args, err := adapter.GetAttachArgs()
	if err != nil {
		t.Fatalf("GetAttachArgs failed: %v", err)
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		t.Fatal("args should be a map")
	}

	if argsMap["type"] != "python" {
		t.Error("type should be 'python'")
	}
	if argsMap["request"] != "attach" {
		t.Error("request should be 'attach'")
	}

	connect, ok := argsMap["connect"].(map[string]interface{})
	if !ok {
		t.Fatal("connect should be a map")
	}

	if connect["host"] != "localhost" {
		t.Error("host mismatch")
	}
	if connect["port"] != 5678 {
		t.Error("port mismatch")
	}
}

func TestPythonAdapter_GetAttachArgs_WithProcessID(t *testing.T) {
	config := Config{
		Type:      AdapterPython,
		Request:   "attach",
		ProcessID: 12345,
	}

	adapter, _ := NewPythonAdapter(config)
	args, _ := adapter.GetAttachArgs()

	argsMap := args.(map[string]interface{})

	if argsMap["processId"] != 12345 {
		t.Error("processId mismatch")
	}
}

func TestPythonAdapter_SetMethods(t *testing.T) {
	config := Config{
		Type:    AdapterPython,
		Request: "launch",
		Program: "/path/to/script.py",
	}

	adapter, _ := NewPythonAdapter(config)
	python := adapter.(*PythonAdapter)

	python.SetProgram("/new/path.py")
	if python.config.Program != "/new/path.py" {
		t.Error("SetProgram failed")
	}

	python.SetModule("pytest")
	if python.config.Module != "pytest" {
		t.Error("SetModule failed")
	}

	python.SetArgs([]string{"a", "b"})
	if len(python.config.Args) != 2 {
		t.Error("SetArgs failed")
	}

	python.SetJustMyCode(false)
	if python.config.JustMyCode {
		t.Error("SetJustMyCode failed")
	}
}

func TestCreateDefaultPythonLaunchConfig(t *testing.T) {
	config := CreateDefaultPythonLaunchConfig("/path/to/script.py")

	if config.Type != AdapterPython {
		t.Error("Type should be Python")
	}
	if config.Request != "launch" {
		t.Error("Request should be 'launch'")
	}
	if config.Program != "/path/to/script.py" {
		t.Error("Program mismatch")
	}
	if !config.JustMyCode {
		t.Error("JustMyCode should be true")
	}
	if !config.RedirectOutput {
		t.Error("RedirectOutput should be true")
	}
}

func TestCreateDefaultPythonAttachConfig(t *testing.T) {
	config := CreateDefaultPythonAttachConfig(5678)

	if config.Request != "attach" {
		t.Error("Request should be 'attach'")
	}
	if config.Port != 5678 {
		t.Error("Port mismatch")
	}
	if !config.JustMyCode {
		t.Error("JustMyCode should be true")
	}
}

func TestCreateDjangoLaunchConfig(t *testing.T) {
	config := CreateDjangoLaunchConfig("/path/to/manage.py")

	if config.Program != "/path/to/manage.py" {
		t.Error("Program mismatch")
	}
	if !config.Django {
		t.Error("Django should be true")
	}
	if len(config.Args) != 2 {
		t.Error("Args should have runserver --noreload")
	}
}

func TestCreateFlaskLaunchConfig(t *testing.T) {
	config := CreateFlaskLaunchConfig("app.py")

	if config.Module != "flask" {
		t.Error("Module should be 'flask'")
	}
	if !config.Flask {
		t.Error("Flask should be true")
	}
	if !config.Jinja {
		t.Error("Jinja should be true")
	}
	if config.Env["FLASK_APP"] != "app.py" {
		t.Error("FLASK_APP env var mismatch")
	}
}

func TestCreatePytestLaunchConfig(t *testing.T) {
	config := CreatePytestLaunchConfig("tests/")

	if config.Module != "pytest" {
		t.Error("Module should be 'pytest'")
	}
	if config.JustMyCode {
		t.Error("JustMyCode should be false for pytest")
	}
	if len(config.Args) != 2 {
		t.Error("Args should have test path and -v")
	}
}

func TestPythonConfig_PathMappings(t *testing.T) {
	config := PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Request: "attach",
			Port:    5678,
		},
		PathMappings: []PathMapping{
			{LocalRoot: "/local/path", RemoteRoot: "/remote/path"},
			{LocalRoot: "/local/path2", RemoteRoot: "/remote/path2"},
		},
	}

	adapter, _ := NewPythonAdapterWithConfig(config)
	args, _ := adapter.GetAttachArgs()

	argsMap := args.(map[string]interface{})

	mappings, ok := argsMap["pathMappings"].([]map[string]string)
	if !ok {
		t.Fatal("pathMappings should be a slice of maps")
	}

	if len(mappings) != 2 {
		t.Errorf("expected 2 path mappings, got %d", len(mappings))
	}

	if mappings[0]["localRoot"] != "/local/path" {
		t.Error("localRoot mismatch")
	}
	if mappings[0]["remoteRoot"] != "/remote/path" {
		t.Error("remoteRoot mismatch")
	}
}

func TestPythonConfig_AdvancedOptions(t *testing.T) {
	config := PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Request: "launch",
			Program: "/path/to/script.py",
		},
		PythonPath:      "/usr/bin/python3.10",
		Console:         "externalTerminal",
		Jinja:           true,
		GeventSupport:   true,
		Sudo:            true,
		ShowReturnValue: true,
		SubProcess:      true,
		LogToFile:       true,
	}

	adapter, _ := NewPythonAdapterWithConfig(config)
	args, _ := adapter.GetLaunchArgs()

	argsMap := args.(map[string]interface{})

	if argsMap["pythonPath"] != "/usr/bin/python3.10" {
		t.Error("pythonPath mismatch")
	}
	if argsMap["console"] != "externalTerminal" {
		t.Error("console mismatch")
	}
	if argsMap["jinja"] != true {
		t.Error("jinja should be true")
	}
	if argsMap["gevent"] != true {
		t.Error("gevent should be true")
	}
	if argsMap["sudo"] != true {
		t.Error("sudo should be true")
	}
	if argsMap["showReturnValue"] != true {
		t.Error("showReturnValue should be true")
	}
	if argsMap["subProcess"] != true {
		t.Error("subProcess should be true")
	}
	if argsMap["logToFile"] != true {
		t.Error("logToFile should be true")
	}
}
