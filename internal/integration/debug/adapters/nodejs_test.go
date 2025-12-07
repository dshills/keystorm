package adapters

import (
	"testing"
)

func TestNewNodeJSAdapter(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Name:    "Test Node.js",
		Request: "launch",
		Program: "/path/to/app.js",
	}

	adapter, err := NewNodeJSAdapter(config)
	if err != nil {
		t.Fatalf("NewNodeJSAdapter failed: %v", err)
	}

	if adapter.Type() != AdapterNodeJS {
		t.Errorf("expected type NodeJS, got %v", adapter.Type())
	}

	if adapter.Name() != "Node.js Debugger" {
		t.Errorf("unexpected adapter name: %s", adapter.Name())
	}
}

func TestNewNodeJSAdapterWithConfig(t *testing.T) {
	config := NodeJSConfig{
		Config: Config{
			Type:    AdapterNodeJS,
			Request: "launch",
			Program: "/path/to/app.js",
		},
		Console:    "integratedTerminal",
		SourceMaps: true,
		SmartStep:  true,
	}

	adapter, err := NewNodeJSAdapterWithConfig(config)
	if err != nil {
		t.Fatalf("NewNodeJSAdapterWithConfig failed: %v", err)
	}

	if adapter.config.Console != "integratedTerminal" {
		t.Errorf("expected console 'integratedTerminal', got %s", adapter.config.Console)
	}
}

func TestNodeJSAdapter_Validate_Launch(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "launch",
		Program: "/path/to/app.js",
	}

	adapter, _ := NewNodeJSAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid launch config: %v", err)
	}
}

func TestNodeJSAdapter_Validate_Launch_NoProgram(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "launch",
	}

	adapter, _ := NewNodeJSAdapter(config)
	err := adapter.Validate()
	if err == nil {
		t.Error("expected error for launch without program")
	}
}

func TestNodeJSAdapter_Validate_Attach(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "attach",
		Port:    9229,
	}

	adapter, _ := NewNodeJSAdapter(config)
	err := adapter.Validate()
	if err != nil {
		t.Errorf("Validate failed for valid attach config: %v", err)
	}
}

func TestNodeJSAdapter_Validate_Attach_NoPort(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "attach",
	}

	adapter, _ := NewNodeJSAdapter(config)
	err := adapter.Validate()
	if err == nil {
		t.Error("expected error for attach without port")
	}
}

func TestNodeJSAdapter_GetConnectionType(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "launch",
		Program: "/path/to/app.js",
	}

	adapter, _ := NewNodeJSAdapter(config)
	if adapter.GetConnectionType() != "socket" {
		t.Error("expected socket connection type for Node.js")
	}
}

func TestNodeJSAdapter_GetAddress(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "launch",
		Program: "/path/to/app.js",
		Port:    9229,
	}

	adapter, _ := NewNodeJSAdapter(config)
	addr := adapter.GetAddress()
	if addr != "127.0.0.1:9229" {
		t.Errorf("expected '127.0.0.1:9229', got %s", addr)
	}
}

func TestNodeJSAdapter_GetAddress_DefaultPort(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "launch",
		Program: "/path/to/app.js",
	}

	adapter, _ := NewNodeJSAdapter(config)
	addr := adapter.GetAddress()
	// Should use default port 9229
	if addr != "127.0.0.1:9229" {
		t.Errorf("expected '127.0.0.1:9229', got %s", addr)
	}
}

func TestNodeJSAdapter_GetLaunchArgs(t *testing.T) {
	config := Config{
		Type:        AdapterNodeJS,
		Request:     "launch",
		Program:     "/path/to/app.js",
		Args:        []string{"--arg1", "--arg2"},
		Cwd:         "/working/dir",
		Env:         map[string]string{"NODE_ENV": "development"},
		StopOnEntry: true,
	}

	adapter, _ := NewNodeJSAdapter(config)
	args, err := adapter.GetLaunchArgs()
	if err != nil {
		t.Fatalf("GetLaunchArgs failed: %v", err)
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		t.Fatal("args should be a map")
	}

	if argsMap["type"] != "node" {
		t.Error("type should be 'node'")
	}
	if argsMap["request"] != "launch" {
		t.Error("request should be 'launch'")
	}
	if argsMap["program"] != "/path/to/app.js" {
		t.Error("program mismatch")
	}
	if argsMap["stopOnEntry"] != true {
		t.Error("stopOnEntry mismatch")
	}
	if argsMap["sourceMaps"] != true {
		t.Error("sourceMaps should be true by default")
	}
	if argsMap["smartStep"] != true {
		t.Error("smartStep should be true by default")
	}
}

func TestNodeJSAdapter_GetAttachArgs(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "attach",
		Port:    9229,
		Host:    "localhost",
	}

	adapter, _ := NewNodeJSAdapter(config)
	args, err := adapter.GetAttachArgs()
	if err != nil {
		t.Fatalf("GetAttachArgs failed: %v", err)
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		t.Fatal("args should be a map")
	}

	if argsMap["type"] != "node" {
		t.Error("type should be 'node'")
	}
	if argsMap["request"] != "attach" {
		t.Error("request should be 'attach'")
	}
	if argsMap["port"] != 9229 {
		t.Error("port mismatch")
	}
	if argsMap["address"] != "localhost" {
		t.Error("address mismatch")
	}
}

func TestNodeJSAdapter_SetMethods(t *testing.T) {
	config := Config{
		Type:    AdapterNodeJS,
		Request: "launch",
		Program: "/path/to/app.js",
	}

	adapter, _ := NewNodeJSAdapter(config)
	node := adapter.(*NodeJSAdapter)

	node.SetProgram("/new/path.js")
	if node.config.Program != "/new/path.js" {
		t.Error("SetProgram failed")
	}

	node.SetArgs([]string{"a", "b"})
	if len(node.config.Args) != 2 {
		t.Error("SetArgs failed")
	}

	node.SetSourceMaps(false)
	if node.config.SourceMaps {
		t.Error("SetSourceMaps failed")
	}
}

func TestCreateDefaultNodeLaunchConfig(t *testing.T) {
	config := CreateDefaultNodeLaunchConfig("/path/to/app.js")

	if config.Type != AdapterNodeJS {
		t.Error("Type should be NodeJS")
	}
	if config.Request != "launch" {
		t.Error("Request should be 'launch'")
	}
	if config.Program != "/path/to/app.js" {
		t.Error("Program mismatch")
	}
	if !config.SourceMaps {
		t.Error("SourceMaps should be true")
	}
	if !config.SmartStep {
		t.Error("SmartStep should be true")
	}
}

func TestCreateDefaultNodeAttachConfig(t *testing.T) {
	config := CreateDefaultNodeAttachConfig(9229)

	if config.Request != "attach" {
		t.Error("Request should be 'attach'")
	}
	if config.Port != 9229 {
		t.Error("Port mismatch")
	}
	if !config.SourceMaps {
		t.Error("SourceMaps should be true")
	}
}

func TestCreateTypeScriptLaunchConfig(t *testing.T) {
	config := CreateTypeScriptLaunchConfig("/path/to/app.ts", "/dist")

	if config.Type != AdapterNodeJS {
		t.Error("Type should be NodeJS")
	}
	if !config.SourceMaps {
		t.Error("SourceMaps should be true for TypeScript")
	}
	if len(config.OutFiles) != 1 {
		t.Error("OutFiles should have one entry")
	}
	if len(config.SkipFiles) != 1 {
		t.Error("SkipFiles should have node_internals entry")
	}
}

func TestNodeJSConfig_AdvancedOptions(t *testing.T) {
	config := NodeJSConfig{
		Config: Config{
			Type:    AdapterNodeJS,
			Request: "launch",
			Program: "/path/to/app.js",
		},
		RuntimeExecutable:        "/usr/local/bin/node",
		RuntimeArgs:              []string{"--experimental-modules"},
		Console:                  "externalTerminal",
		SkipFiles:                []string{"<node_internals>/**", "**/node_modules/**"},
		Trace:                    true,
		Restart:                  true,
		AutoAttachChildProcesses: true,
		ShowAsyncStacks:          true,
		Timeout:                  30000,
	}

	adapter, _ := NewNodeJSAdapterWithConfig(config)
	args, _ := adapter.GetLaunchArgs()

	argsMap := args.(map[string]interface{})

	if argsMap["runtimeExecutable"] != "/usr/local/bin/node" {
		t.Error("runtimeExecutable mismatch")
	}
	if argsMap["console"] != "externalTerminal" {
		t.Error("console mismatch")
	}
	if argsMap["trace"] != true {
		t.Error("trace should be true")
	}
	if argsMap["restart"] != true {
		t.Error("restart should be true")
	}
	if argsMap["autoAttachChildProcesses"] != true {
		t.Error("autoAttachChildProcesses should be true")
	}
	if argsMap["showAsyncStacks"] != true {
		t.Error("showAsyncStacks should be true")
	}
	if argsMap["timeout"] != 30000 {
		t.Error("timeout mismatch")
	}
}

func TestNodeJSConfig_RemoteDebugging(t *testing.T) {
	config := NodeJSConfig{
		Config: Config{
			Type:    AdapterNodeJS,
			Request: "attach",
			Port:    9229,
			Host:    "remote-host",
		},
		LocalRoot:  "/local/project",
		RemoteRoot: "/app",
	}

	adapter, _ := NewNodeJSAdapterWithConfig(config)
	args, _ := adapter.GetAttachArgs()

	argsMap := args.(map[string]interface{})

	if argsMap["localRoot"] != "/local/project" {
		t.Error("localRoot mismatch")
	}
	if argsMap["remoteRoot"] != "/app" {
		t.Error("remoteRoot mismatch")
	}
}
