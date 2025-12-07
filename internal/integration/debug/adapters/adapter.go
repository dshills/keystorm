// Package adapters provides debug adapter configurations for various languages.
package adapters

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"
)

// AdapterType identifies a debug adapter.
type AdapterType string

const (
	// AdapterDelve is the Go debugger (delve).
	AdapterDelve AdapterType = "delve"
	// AdapterNodeJS is the Node.js debugger.
	AdapterNodeJS AdapterType = "nodejs"
	// AdapterPython is the Python debugger (debugpy).
	AdapterPython AdapterType = "python"
	// AdapterLLDB is the LLDB debugger for C/C++/Rust.
	AdapterLLDB AdapterType = "lldb"
	// AdapterGeneric is a generic DAP adapter.
	AdapterGeneric AdapterType = "generic"
)

// Config is the base configuration for a debug adapter.
type Config struct {
	// Type is the adapter type.
	Type AdapterType `json:"type"`

	// Name is a human-readable name for this configuration.
	Name string `json:"name"`

	// Request is the request type: "launch" or "attach".
	Request string `json:"request"`

	// Program is the program to debug.
	Program string `json:"program,omitempty"`

	// Module is the module to run (for Python, Node.js).
	Module string `json:"module,omitempty"`

	// Args are the program arguments.
	Args []string `json:"args,omitempty"`

	// Cwd is the working directory.
	Cwd string `json:"cwd,omitempty"`

	// Env are additional environment variables.
	Env map[string]string `json:"env,omitempty"`

	// StopOnEntry stops at the program entry point.
	StopOnEntry bool `json:"stopOnEntry,omitempty"`

	// Port is the port to connect to (for attach or socket adapter).
	Port int `json:"port,omitempty"`

	// Host is the host to connect to.
	Host string `json:"host,omitempty"`

	// ProcessID is the process ID to attach to.
	ProcessID int `json:"processId,omitempty"`

	// AdapterPath is the path to the debug adapter executable.
	AdapterPath string `json:"adapterPath,omitempty"`

	// AdapterArgs are arguments for the adapter executable.
	AdapterArgs []string `json:"adapterArgs,omitempty"`
}

// Adapter provides configuration and launch capabilities for a debug adapter.
type Adapter interface {
	// Type returns the adapter type.
	Type() AdapterType

	// Name returns a human-readable adapter name.
	Name() string

	// Validate validates the configuration.
	Validate() error

	// GetCommand returns the command to start the adapter.
	GetCommand() (*exec.Cmd, error)

	// GetLaunchArgs returns the arguments for the launch request.
	GetLaunchArgs() (interface{}, error)

	// GetAttachArgs returns the arguments for the attach request.
	GetAttachArgs() (interface{}, error)

	// GetConnectionType returns whether to use "stdio" or "socket".
	GetConnectionType() string

	// GetAddress returns the socket address (for socket connection).
	GetAddress() string
}

// Registry manages available debug adapters.
type Registry struct {
	adapters map[AdapterType]func(Config) (Adapter, error)
}

// NewRegistry creates a new adapter registry with default adapters.
func NewRegistry() *Registry {
	r := &Registry{
		adapters: make(map[AdapterType]func(Config) (Adapter, error)),
	}

	// Register built-in adapters
	r.Register(AdapterDelve, NewDelveAdapter)
	r.Register(AdapterNodeJS, NewNodeJSAdapter)
	r.Register(AdapterPython, NewPythonAdapter)

	return r
}

// Register registers an adapter factory.
func (r *Registry) Register(adapterType AdapterType, factory func(Config) (Adapter, error)) {
	r.adapters[adapterType] = factory
}

// Create creates an adapter from configuration.
func (r *Registry) Create(config Config) (Adapter, error) {
	factory, ok := r.adapters[config.Type]
	if !ok {
		return nil, fmt.Errorf("unknown adapter type: %s", config.Type)
	}
	return factory(config)
}

// AvailableAdapters returns the list of registered adapter types.
func (r *Registry) AvailableAdapters() []AdapterType {
	result := make([]AdapterType, 0, len(r.adapters))
	for t := range r.adapters {
		result = append(result, t)
	}
	return result
}

// FindExecutable searches for an executable in PATH.
func FindExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH: %w", name, err)
	}
	return path, nil
}

// DetectAdapterType attempts to detect the appropriate adapter type for a file.
func DetectAdapterType(filename string) AdapterType {
	// Simple extension-based detection
	switch {
	case hasExtension(filename, ".go"):
		return AdapterDelve
	case hasExtension(filename, ".js", ".ts", ".mjs", ".cjs"):
		return AdapterNodeJS
	case hasExtension(filename, ".py"):
		return AdapterPython
	case hasExtension(filename, ".c", ".cpp", ".cc", ".rs"):
		return AdapterLLDB
	default:
		return AdapterGeneric
	}
}

func hasExtension(filename string, extensions ...string) bool {
	for _, ext := range extensions {
		if len(filename) > len(ext) && filename[len(filename)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// WaitForPort waits for a port to become available by polling with connection attempts.
// It returns nil when the port is accepting connections, or an error if the context
// is cancelled or times out.
func WaitForPort(ctx context.Context, host string, port int) error {
	address := fmt.Sprintf("%s:%d", host, port)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for port %d: %w", port, ctx.Err())
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
			if err == nil {
				conn.Close()
				return nil
			}
			// Port not ready yet, continue polling
		}
	}
}
