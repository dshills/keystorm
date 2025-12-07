package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// PythonConfig extends Config with Python-specific options.
type PythonConfig struct {
	Config

	// PythonPath is the path to the Python interpreter.
	PythonPath string `json:"pythonPath,omitempty"`

	// Console is where to launch the debug target: "internalConsole", "integratedTerminal", "externalTerminal".
	Console string `json:"console,omitempty"`

	// JustMyCode enables "just my code" debugging.
	JustMyCode bool `json:"justMyCode,omitempty"`

	// Django enables Django debugging.
	Django bool `json:"django,omitempty"`

	// Jinja enables Jinja template debugging.
	Jinja bool `json:"jinja,omitempty"`

	// Flask enables Flask debugging.
	Flask bool `json:"flask,omitempty"`

	// Pyramid enables Pyramid debugging.
	Pyramid bool `json:"pyramid,omitempty"`

	// GeventSupport enables gevent debugging.
	GeventSupport bool `json:"gevent,omitempty"`

	// Sudo runs the program under sudo.
	Sudo bool `json:"sudo,omitempty"`

	// RedirectOutput redirects output to the debug console.
	RedirectOutput bool `json:"redirectOutput,omitempty"`

	// ShowReturnValue shows function return values.
	ShowReturnValue bool `json:"showReturnValue,omitempty"`

	// SubProcess enables debugging of child processes.
	SubProcess bool `json:"subProcess,omitempty"`

	// DebugpyPath is the path to debugpy.
	DebugpyPath string `json:"debugpyPath,omitempty"`

	// PathMappings are mappings for remote debugging.
	PathMappings []PathMapping `json:"pathMappings,omitempty"`

	// LogToFile enables logging to a file.
	LogToFile bool `json:"logToFile,omitempty"`
}

// PathMapping represents a path mapping for remote debugging.
type PathMapping struct {
	LocalRoot  string `json:"localRoot"`
	RemoteRoot string `json:"remoteRoot"`
}

// PythonAdapter implements the Adapter interface for Python debugging.
type PythonAdapter struct {
	config PythonConfig
}

// NewPythonAdapter creates a new Python adapter.
func NewPythonAdapter(baseConfig Config) (Adapter, error) {
	config := PythonConfig{
		Config:          baseConfig,
		Console:         "internalConsole",
		JustMyCode:      true,
		RedirectOutput:  true,
		ShowReturnValue: true,
	}

	return &PythonAdapter{config: config}, nil
}

// NewPythonAdapterWithConfig creates a Python adapter with full configuration.
func NewPythonAdapterWithConfig(config PythonConfig) (*PythonAdapter, error) {
	// Set defaults
	if config.Console == "" {
		config.Console = "internalConsole"
	}

	return &PythonAdapter{config: config}, nil
}

// Type returns the adapter type.
func (a *PythonAdapter) Type() AdapterType {
	return AdapterPython
}

// Name returns a human-readable adapter name.
func (a *PythonAdapter) Name() string {
	return "Python Debugger (debugpy)"
}

// Validate validates the configuration.
func (a *PythonAdapter) Validate() error {
	if a.config.Request == "launch" {
		if a.config.Program == "" && a.config.Module == "" {
			return fmt.Errorf("program or module is required for launch request")
		}
	} else if a.config.Request == "attach" {
		if a.config.Port == 0 && a.config.ProcessID == 0 {
			return fmt.Errorf("port or processId is required for attach request")
		}
	} else if a.config.Request != "" {
		return fmt.Errorf("invalid request type: %s", a.config.Request)
	}

	return nil
}

// GetCommand returns the command to start the adapter.
func (a *PythonAdapter) GetCommand() (*exec.Cmd, error) {
	python := a.config.PythonPath
	if python == "" {
		var err error
		python, err = FindExecutable("python3")
		if err != nil {
			python, err = FindExecutable("python")
			if err != nil {
				return nil, fmt.Errorf("python interpreter not found in PATH (install Python 3 and debugpy: pip install debugpy)")
			}
		}
	}

	// Start debugpy as a DAP server
	args := []string{
		"-m", "debugpy.adapter",
	}

	// Add host and port for socket mode
	if a.config.Port > 0 {
		args = append(args, "--host", a.getHost(), "--port", strconv.Itoa(a.config.Port))
	}

	cmd := exec.Command(python, args...)

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
func (a *PythonAdapter) GetLaunchArgs() (interface{}, error) {
	args := map[string]interface{}{
		"type":           "python",
		"request":        "launch",
		"stopOnEntry":    a.config.StopOnEntry,
		"justMyCode":     a.config.JustMyCode,
		"console":        a.config.Console,
		"redirectOutput": a.config.RedirectOutput,
	}

	if a.config.Program != "" {
		args["program"] = a.config.Program
	}

	if a.config.Module != "" {
		args["module"] = a.config.Module
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

	if a.config.PythonPath != "" {
		args["pythonPath"] = a.config.PythonPath
	}

	if a.config.Django {
		args["django"] = true
	}

	if a.config.Jinja {
		args["jinja"] = true
	}

	if a.config.Flask {
		args["flask"] = true
	}

	if a.config.Pyramid {
		args["pyramid"] = true
	}

	if a.config.GeventSupport {
		args["gevent"] = true
	}

	if a.config.Sudo {
		args["sudo"] = true
	}

	if a.config.ShowReturnValue {
		args["showReturnValue"] = true
	}

	if a.config.SubProcess {
		args["subProcess"] = true
	}

	if a.config.LogToFile {
		args["logToFile"] = true
	}

	return args, nil
}

// GetAttachArgs returns the arguments for the attach request.
func (a *PythonAdapter) GetAttachArgs() (interface{}, error) {
	args := map[string]interface{}{
		"type":           "python",
		"request":        "attach",
		"justMyCode":     a.config.JustMyCode,
		"redirectOutput": a.config.RedirectOutput,
	}

	if a.config.Port > 0 {
		args["connect"] = map[string]interface{}{
			"host": a.getHost(),
			"port": a.config.Port,
		}
	}

	if a.config.ProcessID > 0 {
		args["processId"] = a.config.ProcessID
	}

	if len(a.config.PathMappings) > 0 {
		mappings := make([]map[string]string, len(a.config.PathMappings))
		for i, m := range a.config.PathMappings {
			mappings[i] = map[string]string{
				"localRoot":  m.LocalRoot,
				"remoteRoot": m.RemoteRoot,
			}
		}
		args["pathMappings"] = mappings
	}

	if a.config.Django {
		args["django"] = true
	}

	if a.config.Jinja {
		args["jinja"] = true
	}

	if a.config.ShowReturnValue {
		args["showReturnValue"] = true
	}

	if a.config.SubProcess {
		args["subProcess"] = true
	}

	return args, nil
}

// GetConnectionType returns whether to use "stdio" or "socket".
func (a *PythonAdapter) GetConnectionType() string {
	if a.config.Port > 0 {
		return "socket"
	}
	return "stdio"
}

// GetAddress returns the socket address (for socket connection).
func (a *PythonAdapter) GetAddress() string {
	if a.config.Port > 0 {
		return a.getHost() + ":" + strconv.Itoa(a.config.Port)
	}
	return ""
}

func (a *PythonAdapter) getHost() string {
	if a.config.Host != "" {
		return a.config.Host
	}
	return "127.0.0.1"
}

// SetProgram sets the program to debug.
func (a *PythonAdapter) SetProgram(program string) {
	a.config.Program = program
}

// SetModule sets the module to run.
func (a *PythonAdapter) SetModule(module string) {
	a.config.Module = module
}

// SetArgs sets the program arguments.
func (a *PythonAdapter) SetArgs(args []string) {
	a.config.Args = args
}

// SetJustMyCode enables or disables "just my code" debugging.
func (a *PythonAdapter) SetJustMyCode(enabled bool) {
	a.config.JustMyCode = enabled
}

// CreateDefaultPythonLaunchConfig creates a default launch configuration for Python.
func CreateDefaultPythonLaunchConfig(program string) PythonConfig {
	return PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Name:    "Launch Python",
			Request: "launch",
			Program: program,
		},
		Console:         "internalConsole",
		JustMyCode:      true,
		RedirectOutput:  true,
		ShowReturnValue: true,
	}
}

// CreateDefaultPythonAttachConfig creates a default attach configuration for Python.
func CreateDefaultPythonAttachConfig(port int) PythonConfig {
	return PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Name:    "Attach to Python",
			Request: "attach",
			Port:    port,
		},
		JustMyCode:      true,
		RedirectOutput:  true,
		ShowReturnValue: true,
	}
}

// CreateDjangoLaunchConfig creates a configuration for debugging Django.
func CreateDjangoLaunchConfig(managePy string) PythonConfig {
	return PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Name:    "Launch Django",
			Request: "launch",
			Program: managePy,
			Args:    []string{"runserver", "--noreload"},
		},
		Console:         "integratedTerminal",
		Django:          true,
		JustMyCode:      true,
		RedirectOutput:  true,
		ShowReturnValue: true,
	}
}

// CreateFlaskLaunchConfig creates a configuration for debugging Flask.
func CreateFlaskLaunchConfig(appFile string) PythonConfig {
	return PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Name:    "Launch Flask",
			Request: "launch",
			Module:  "flask",
			Args:    []string{"run", "--no-debugger", "--no-reload"},
			Env: map[string]string{
				"FLASK_APP": appFile,
			},
		},
		Console:         "integratedTerminal",
		Flask:           true,
		Jinja:           true,
		JustMyCode:      true,
		RedirectOutput:  true,
		ShowReturnValue: true,
	}
}

// CreatePytestLaunchConfig creates a configuration for debugging pytest.
func CreatePytestLaunchConfig(testPath string) PythonConfig {
	return PythonConfig{
		Config: Config{
			Type:    AdapterPython,
			Name:    "Debug pytest",
			Request: "launch",
			Module:  "pytest",
			Args:    []string{testPath, "-v"},
		},
		Console:         "integratedTerminal",
		JustMyCode:      false, // Include test framework code
		RedirectOutput:  true,
		ShowReturnValue: true,
	}
}
