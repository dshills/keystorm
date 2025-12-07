package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// NodeJSConfig extends Config with Node.js-specific options.
type NodeJSConfig struct {
	Config

	// RuntimeExecutable is the path to the Node.js executable.
	RuntimeExecutable string `json:"runtimeExecutable,omitempty"`

	// RuntimeArgs are arguments passed to the runtime.
	RuntimeArgs []string `json:"runtimeArgs,omitempty"`

	// Console is where to launch the debug target: "internalConsole", "integratedTerminal", "externalTerminal".
	Console string `json:"console,omitempty"`

	// SourceMaps enables source map support.
	SourceMaps bool `json:"sourceMaps,omitempty"`

	// OutFiles are glob patterns for generated JavaScript files.
	OutFiles []string `json:"outFiles,omitempty"`

	// SkipFiles are glob patterns for files to skip when stepping.
	SkipFiles []string `json:"skipFiles,omitempty"`

	// Trace enables diagnostic tracing.
	Trace bool `json:"trace,omitempty"`

	// SmartStep automatically steps through generated code.
	SmartStep bool `json:"smartStep,omitempty"`

	// Restart automatically restarts after the program terminates.
	Restart bool `json:"restart,omitempty"`

	// LocalRoot is the local source root for remote debugging.
	LocalRoot string `json:"localRoot,omitempty"`

	// RemoteRoot is the remote source root for remote debugging.
	RemoteRoot string `json:"remoteRoot,omitempty"`

	// Protocol is the debug protocol: "auto", "inspector", "legacy".
	Protocol string `json:"protocol,omitempty"`

	// Timeout is the timeout for connecting to the debuggee (ms).
	Timeout int `json:"timeout,omitempty"`

	// ResolveSourceMapLocations are patterns for resolving source maps.
	ResolveSourceMapLocations []string `json:"resolveSourceMapLocations,omitempty"`

	// AutoAttachChildProcesses attaches to child processes.
	AutoAttachChildProcesses bool `json:"autoAttachChildProcesses,omitempty"`

	// ShowAsyncStacks shows async stack traces.
	ShowAsyncStacks bool `json:"showAsyncStacks,omitempty"`
}

// NodeJSAdapter implements the Adapter interface for Node.js debugging.
type NodeJSAdapter struct {
	config NodeJSConfig
}

// NewNodeJSAdapter creates a new Node.js adapter.
func NewNodeJSAdapter(baseConfig Config) (Adapter, error) {
	config := NodeJSConfig{
		Config:     baseConfig,
		Console:    "internalConsole",
		SourceMaps: true,
		SmartStep:  true,
		Protocol:   "inspector",
		Timeout:    10000,
	}

	return &NodeJSAdapter{config: config}, nil
}

// NewNodeJSAdapterWithConfig creates a Node.js adapter with full configuration.
func NewNodeJSAdapterWithConfig(config NodeJSConfig) (*NodeJSAdapter, error) {
	// Set defaults
	if config.Console == "" {
		config.Console = "internalConsole"
	}
	if config.Protocol == "" {
		config.Protocol = "inspector"
	}
	if config.Timeout == 0 {
		config.Timeout = 10000
	}

	return &NodeJSAdapter{config: config}, nil
}

// Type returns the adapter type.
func (a *NodeJSAdapter) Type() AdapterType {
	return AdapterNodeJS
}

// Name returns a human-readable adapter name.
func (a *NodeJSAdapter) Name() string {
	return "Node.js Debugger"
}

// Validate validates the configuration.
func (a *NodeJSAdapter) Validate() error {
	if a.config.Request == "launch" {
		if a.config.Program == "" {
			return fmt.Errorf("program is required for launch request")
		}
	} else if a.config.Request == "attach" {
		if a.config.Port == 0 {
			return fmt.Errorf("port is required for attach request")
		}
	} else if a.config.Request != "" {
		return fmt.Errorf("invalid request type: %s", a.config.Request)
	}

	return nil
}

// GetCommand returns the command to start the adapter.
func (a *NodeJSAdapter) GetCommand() (*exec.Cmd, error) {
	// Node.js debugging uses the built-in inspector protocol
	// We start Node.js with --inspect or --inspect-brk
	runtime := a.config.RuntimeExecutable
	if runtime == "" {
		var err error
		runtime, err = FindExecutable("node")
		if err != nil {
			return nil, fmt.Errorf("node.js runtime not found: %w (install from https://nodejs.org/)", err)
		}
	}

	args := make([]string, 0)

	// Add runtime arguments
	args = append(args, a.config.RuntimeArgs...)

	// Add inspect flag
	if a.config.StopOnEntry {
		args = append(args, fmt.Sprintf("--inspect-brk=%d", a.getPort()))
	} else {
		args = append(args, fmt.Sprintf("--inspect=%d", a.getPort()))
	}

	// Add program
	args = append(args, a.config.Program)

	// Add program arguments
	args = append(args, a.config.Args...)

	cmd := exec.Command(runtime, args...)

	// Set working directory
	if a.config.Cwd != "" {
		cmd.Dir = a.config.Cwd
	}

	// Inherit parent environment and add/override with config values
	cmd.Env = os.Environ()
	for k, v := range a.config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	return cmd, nil
}

// GetLaunchArgs returns the arguments for the launch request.
func (a *NodeJSAdapter) GetLaunchArgs() (interface{}, error) {
	args := map[string]interface{}{
		"type":        "node",
		"request":     "launch",
		"program":     a.config.Program,
		"stopOnEntry": a.config.StopOnEntry,
		"sourceMaps":  a.config.SourceMaps,
		"smartStep":   a.config.SmartStep,
		"console":     a.config.Console,
		"protocol":    a.config.Protocol,
	}

	if len(a.config.Args) > 0 {
		args["args"] = a.config.Args
	}

	if a.config.Cwd != "" {
		args["cwd"] = a.config.Cwd
	}

	if len(a.config.Env) > 0 {
		args["env"] = a.config.Env
	}

	if a.config.RuntimeExecutable != "" {
		args["runtimeExecutable"] = a.config.RuntimeExecutable
	}

	if len(a.config.RuntimeArgs) > 0 {
		args["runtimeArgs"] = a.config.RuntimeArgs
	}

	if len(a.config.OutFiles) > 0 {
		args["outFiles"] = a.config.OutFiles
	}

	if len(a.config.SkipFiles) > 0 {
		args["skipFiles"] = a.config.SkipFiles
	}

	if a.config.Trace {
		args["trace"] = true
	}

	if a.config.Restart {
		args["restart"] = true
	}

	if len(a.config.ResolveSourceMapLocations) > 0 {
		args["resolveSourceMapLocations"] = a.config.ResolveSourceMapLocations
	}

	if a.config.AutoAttachChildProcesses {
		args["autoAttachChildProcesses"] = true
	}

	if a.config.ShowAsyncStacks {
		args["showAsyncStacks"] = true
	}

	if a.config.Timeout > 0 {
		args["timeout"] = a.config.Timeout
	}

	return args, nil
}

// GetAttachArgs returns the arguments for the attach request.
func (a *NodeJSAdapter) GetAttachArgs() (interface{}, error) {
	args := map[string]interface{}{
		"type":       "node",
		"request":    "attach",
		"port":       a.config.Port,
		"sourceMaps": a.config.SourceMaps,
		"smartStep":  a.config.SmartStep,
		"protocol":   a.config.Protocol,
	}

	if a.config.Host != "" {
		args["address"] = a.config.Host
	}

	if a.config.ProcessID > 0 {
		args["processId"] = a.config.ProcessID
	}

	if a.config.LocalRoot != "" {
		args["localRoot"] = a.config.LocalRoot
	}

	if a.config.RemoteRoot != "" {
		args["remoteRoot"] = a.config.RemoteRoot
	}

	if len(a.config.SkipFiles) > 0 {
		args["skipFiles"] = a.config.SkipFiles
	}

	if a.config.Trace {
		args["trace"] = true
	}

	if a.config.Timeout > 0 {
		args["timeout"] = a.config.Timeout
	}

	return args, nil
}

// GetConnectionType returns whether to use "stdio" or "socket".
func (a *NodeJSAdapter) GetConnectionType() string {
	// Node.js always uses socket connection to the inspector
	return "socket"
}

// GetAddress returns the socket address (for socket connection).
func (a *NodeJSAdapter) GetAddress() string {
	return a.getHost() + ":" + strconv.Itoa(a.getPort())
}

func (a *NodeJSAdapter) getHost() string {
	if a.config.Host != "" {
		return a.config.Host
	}
	return "127.0.0.1"
}

func (a *NodeJSAdapter) getPort() int {
	if a.config.Port > 0 {
		return a.config.Port
	}
	return 9229 // Default Node.js debug port
}

// SetProgram sets the program to debug.
func (a *NodeJSAdapter) SetProgram(program string) {
	a.config.Program = program
}

// SetArgs sets the program arguments.
func (a *NodeJSAdapter) SetArgs(args []string) {
	a.config.Args = args
}

// SetSourceMaps enables or disables source map support.
func (a *NodeJSAdapter) SetSourceMaps(enabled bool) {
	a.config.SourceMaps = enabled
}

// CreateDefaultNodeLaunchConfig creates a default launch configuration for Node.js.
func CreateDefaultNodeLaunchConfig(program string) NodeJSConfig {
	return NodeJSConfig{
		Config: Config{
			Type:    AdapterNodeJS,
			Name:    "Launch Node.js",
			Request: "launch",
			Program: program,
		},
		Console:    "internalConsole",
		SourceMaps: true,
		SmartStep:  true,
		Protocol:   "inspector",
		Timeout:    10000,
	}
}

// CreateDefaultNodeAttachConfig creates a default attach configuration for Node.js.
func CreateDefaultNodeAttachConfig(port int) NodeJSConfig {
	return NodeJSConfig{
		Config: Config{
			Type:    AdapterNodeJS,
			Name:    "Attach to Node.js",
			Request: "attach",
			Port:    port,
		},
		SourceMaps: true,
		SmartStep:  true,
		Protocol:   "inspector",
		Timeout:    10000,
	}
}

// CreateTypeScriptLaunchConfig creates a configuration for debugging TypeScript.
func CreateTypeScriptLaunchConfig(program string, outDir string) NodeJSConfig {
	return NodeJSConfig{
		Config: Config{
			Type:    AdapterNodeJS,
			Name:    "Launch TypeScript",
			Request: "launch",
			Program: program,
		},
		Console:    "internalConsole",
		SourceMaps: true,
		SmartStep:  true,
		Protocol:   "inspector",
		OutFiles:   []string{outDir + "/**/*.js"},
		SkipFiles:  []string{"<node_internals>/**"},
		Timeout:    10000,
	}
}
