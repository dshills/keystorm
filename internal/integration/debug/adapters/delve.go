package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// DelveConfig extends Config with Delve-specific options.
type DelveConfig struct {
	Config

	// Mode is the debug mode: "debug", "test", "exec", "core", "replay".
	Mode string `json:"mode,omitempty"`

	// BuildFlags are the flags to pass to go build.
	BuildFlags string `json:"buildFlags,omitempty"`

	// ShowGlobalVariables shows global package variables.
	ShowGlobalVariables bool `json:"showGlobalVariables,omitempty"`

	// ShowRegisters shows CPU registers.
	ShowRegisters bool `json:"showRegisters,omitempty"`

	// ShowPprofLabels shows pprof labels.
	ShowPprofLabels bool `json:"showPprofLabels,omitempty"`

	// HideSystemGoroutines hides system goroutines.
	HideSystemGoroutines bool `json:"hideSystemGoroutines,omitempty"`

	// StackTraceDepth is the maximum stack trace depth.
	StackTraceDepth int `json:"stackTraceDepth,omitempty"`

	// GoroutineFilters are filters for goroutines.
	GoroutineFilters string `json:"goroutineFilters,omitempty"`

	// Dlv is the path to the dlv executable.
	DlvPath string `json:"dlvPath,omitempty"`

	// UseAPI2 uses DAP v2 protocol.
	UseAPI2 bool `json:"useApi2,omitempty"`

	// Substitutions are path substitutions for remote debugging.
	Substitutions map[string]string `json:"substitutePath,omitempty"`

	// DebugAdapter is the type of debug adapter: "legacy" or "dlv-dap".
	DebugAdapter string `json:"debugAdapter,omitempty"`

	// Backend is the backend to use: "default", "native", "lldb", "rr".
	Backend string `json:"backend,omitempty"`

	// Output is the compiled binary output path.
	Output string `json:"output,omitempty"`

	// CoreFilePath is the path to the core dump file (for core mode).
	CoreFilePath string `json:"coreFilePath,omitempty"`

	// TraceDirPath is the path to the rr trace (for replay mode).
	TraceDirPath string `json:"traceDirPath,omitempty"`
}

// DelveAdapter implements the Adapter interface for Go debugging with Delve.
type DelveAdapter struct {
	config DelveConfig
}

// NewDelveAdapter creates a new Delve adapter.
func NewDelveAdapter(baseConfig Config) (Adapter, error) {
	config := DelveConfig{
		Config:          baseConfig,
		Mode:            "debug",
		StackTraceDepth: 50,
		DebugAdapter:    "dlv-dap",
	}

	return &DelveAdapter{config: config}, nil
}

// NewDelveAdapterWithConfig creates a Delve adapter with full configuration.
func NewDelveAdapterWithConfig(config DelveConfig) (*DelveAdapter, error) {
	// Set defaults
	if config.Mode == "" {
		config.Mode = "debug"
	}
	if config.StackTraceDepth == 0 {
		config.StackTraceDepth = 50
	}
	if config.DebugAdapter == "" {
		config.DebugAdapter = "dlv-dap"
	}

	return &DelveAdapter{config: config}, nil
}

// Type returns the adapter type.
func (a *DelveAdapter) Type() AdapterType {
	return AdapterDelve
}

// Name returns a human-readable adapter name.
func (a *DelveAdapter) Name() string {
	return "Delve (Go Debugger)"
}

// Validate validates the configuration.
func (a *DelveAdapter) Validate() error {
	if a.config.Request == "launch" {
		if a.config.Program == "" {
			return fmt.Errorf("program is required for launch request")
		}
	} else if a.config.Request == "attach" {
		if a.config.ProcessID == 0 && a.config.Port == 0 {
			return fmt.Errorf("processId or port is required for attach request")
		}
	} else if a.config.Request != "" {
		return fmt.Errorf("invalid request type: %s", a.config.Request)
	}

	return nil
}

// GetCommand returns the command to start the adapter.
func (a *DelveAdapter) GetCommand() (*exec.Cmd, error) {
	dlvPath := a.config.DlvPath
	if dlvPath == "" {
		var err error
		dlvPath, err = FindExecutable("dlv")
		if err != nil {
			return nil, fmt.Errorf("delve debugger not found: %w (install with: go install github.com/go-delve/delve/cmd/dlv@latest)", err)
		}
	}

	args := []string{"dap"}

	// Add listen address if using socket mode
	if a.config.Port > 0 {
		args = append(args, "--listen", fmt.Sprintf("%s:%d", a.getHost(), a.config.Port))
	}

	cmd := exec.Command(dlvPath, args...)

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
func (a *DelveAdapter) GetLaunchArgs() (interface{}, error) {
	args := map[string]interface{}{
		"mode":        a.config.Mode,
		"program":     a.config.Program,
		"stopOnEntry": a.config.StopOnEntry,
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

	if a.config.BuildFlags != "" {
		args["buildFlags"] = a.config.BuildFlags
	}

	if a.config.Output != "" {
		args["output"] = a.config.Output
	}

	if a.config.Backend != "" {
		args["backend"] = a.config.Backend
	}

	if a.config.ShowGlobalVariables {
		args["showGlobalVariables"] = true
	}

	if a.config.ShowRegisters {
		args["showRegisters"] = true
	}

	if a.config.ShowPprofLabels {
		args["showPprofLabels"] = true
	}

	if a.config.HideSystemGoroutines {
		args["hideSystemGoroutines"] = true
	}

	if a.config.StackTraceDepth > 0 {
		args["stackTraceDepth"] = a.config.StackTraceDepth
	}

	if a.config.GoroutineFilters != "" {
		args["goroutineFilters"] = a.config.GoroutineFilters
	}

	if len(a.config.Substitutions) > 0 {
		subs := make([]map[string]string, 0, len(a.config.Substitutions))
		for from, to := range a.config.Substitutions {
			subs = append(subs, map[string]string{"from": from, "to": to})
		}
		args["substitutePath"] = subs
	}

	// Mode-specific settings
	switch a.config.Mode {
	case "core":
		if a.config.CoreFilePath != "" {
			args["coreFilePath"] = a.config.CoreFilePath
		}
	case "replay":
		if a.config.TraceDirPath != "" {
			args["traceDirPath"] = a.config.TraceDirPath
		}
	}

	return args, nil
}

// GetAttachArgs returns the arguments for the attach request.
func (a *DelveAdapter) GetAttachArgs() (interface{}, error) {
	args := map[string]interface{}{
		"mode":        "local",
		"stopOnEntry": a.config.StopOnEntry,
	}

	if a.config.ProcessID > 0 {
		args["processId"] = a.config.ProcessID
	}

	if a.config.Cwd != "" {
		args["cwd"] = a.config.Cwd
	}

	if a.config.ShowGlobalVariables {
		args["showGlobalVariables"] = true
	}

	if a.config.ShowRegisters {
		args["showRegisters"] = true
	}

	if a.config.StackTraceDepth > 0 {
		args["stackTraceDepth"] = a.config.StackTraceDepth
	}

	if len(a.config.Substitutions) > 0 {
		subs := make([]map[string]string, 0, len(a.config.Substitutions))
		for from, to := range a.config.Substitutions {
			subs = append(subs, map[string]string{"from": from, "to": to})
		}
		args["substitutePath"] = subs
	}

	return args, nil
}

// GetConnectionType returns whether to use "stdio" or "socket".
func (a *DelveAdapter) GetConnectionType() string {
	if a.config.Port > 0 {
		return "socket"
	}
	return "stdio"
}

// GetAddress returns the socket address (for socket connection).
func (a *DelveAdapter) GetAddress() string {
	if a.config.Port > 0 {
		return a.getHost() + ":" + strconv.Itoa(a.config.Port)
	}
	return ""
}

func (a *DelveAdapter) getHost() string {
	if a.config.Host != "" {
		return a.config.Host
	}
	return "127.0.0.1"
}

// SetMode sets the debug mode.
func (a *DelveAdapter) SetMode(mode string) {
	a.config.Mode = mode
}

// SetBuildFlags sets the build flags.
func (a *DelveAdapter) SetBuildFlags(flags string) {
	a.config.BuildFlags = flags
}

// SetProgram sets the program to debug.
func (a *DelveAdapter) SetProgram(program string) {
	a.config.Program = program
}

// SetArgs sets the program arguments.
func (a *DelveAdapter) SetArgs(args []string) {
	a.config.Args = args
}

// CreateDefaultLaunchConfig creates a default launch configuration for Go.
func CreateDefaultLaunchConfig(program string) DelveConfig {
	return DelveConfig{
		Config: Config{
			Type:    AdapterDelve,
			Name:    "Launch Go Program",
			Request: "launch",
			Program: program,
		},
		Mode:            "debug",
		StackTraceDepth: 50,
		DebugAdapter:    "dlv-dap",
	}
}

// CreateDefaultTestConfig creates a configuration for debugging tests.
func CreateDefaultTestConfig(testDir string, testName string) DelveConfig {
	config := DelveConfig{
		Config: Config{
			Type:    AdapterDelve,
			Name:    "Debug Go Test",
			Request: "launch",
			Program: testDir,
		},
		Mode:            "test",
		StackTraceDepth: 50,
		DebugAdapter:    "dlv-dap",
	}

	if testName != "" {
		config.Args = []string{"-test.run", testName}
	}

	return config
}

// CreateDefaultAttachConfig creates a configuration for attaching to a process.
func CreateDefaultAttachConfig(processID int) DelveConfig {
	return DelveConfig{
		Config: Config{
			Type:      AdapterDelve,
			Name:      "Attach to Process",
			Request:   "attach",
			ProcessID: processID,
		},
		Mode:            "local",
		StackTraceDepth: 50,
		DebugAdapter:    "dlv-dap",
	}
}
