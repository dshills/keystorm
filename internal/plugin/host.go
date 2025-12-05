package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	plua "github.com/dshills/keystorm/internal/plugin/lua"
	lua "github.com/yuin/gopher-lua"
)

// Host manages a single plugin's Lua state and lifecycle.
type Host struct {
	mu sync.RWMutex

	// Identity
	name     string
	manifest *Manifest

	// Lua runtime
	state  *plua.State
	bridge *plua.Bridge

	// State
	pluginState State
	err         error

	// Configuration
	config map[string]interface{}

	// Resource tracking
	commands      []string
	keymaps       []string
	subscriptions []string

	// Options
	memoryLimit      int64
	executionTimeout time.Duration
}

// HostOption configures a Host.
type HostOption func(*Host)

// WithHostMemoryLimit sets the memory limit for the plugin.
func WithHostMemoryLimit(bytes int64) HostOption {
	return func(h *Host) {
		h.memoryLimit = bytes
	}
}

// WithHostExecutionTimeout sets the execution timeout for plugin calls.
func WithHostExecutionTimeout(d time.Duration) HostOption {
	return func(h *Host) {
		h.executionTimeout = d
	}
}

// WithHostConfig sets the initial configuration for the plugin.
func WithHostConfig(config map[string]interface{}) HostOption {
	return func(h *Host) {
		h.config = config
	}
}

// NewHost creates a new plugin host for the given manifest.
func NewHost(manifest *Manifest, opts ...HostOption) (*Host, error) {
	if manifest == nil {
		return nil, ErrNilManifest
	}

	h := &Host{
		name:             manifest.Name,
		manifest:         manifest,
		pluginState:      StateUnloaded,
		config:           make(map[string]interface{}),
		memoryLimit:      plua.DefaultMemoryLimit,
		executionTimeout: plua.DefaultExecutionTimeout,
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	// Apply manifest config defaults
	for key, prop := range manifest.ConfigSchema {
		if prop.Default != nil {
			h.config[key] = prop.Default
		}
	}

	return h, nil
}

// Name returns the plugin name.
func (h *Host) Name() string {
	return h.name
}

// Manifest returns the plugin manifest.
func (h *Host) Manifest() *Manifest {
	return h.manifest
}

// State returns the current plugin state.
func (h *Host) State() State {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.pluginState
}

// Error returns any error that occurred.
func (h *Host) Error() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.err
}

// Config returns the plugin configuration.
func (h *Host) Config() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return a copy
	config := make(map[string]interface{}, len(h.config))
	for k, v := range h.config {
		config[k] = v
	}
	return config
}

// SetConfig sets a configuration value.
func (h *Host) SetConfig(key string, value interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.config[key] = value
}

// Load initializes the Lua state and loads the plugin code.
func (h *Host) Load(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pluginState != StateUnloaded {
		return ErrAlreadyLoaded
	}

	// Create Lua state
	state, err := plua.NewState(
		plua.WithMemoryLimit(h.memoryLimit),
		plua.WithExecutionTimeout(h.executionTimeout),
	)
	if err != nil {
		h.pluginState = StateError
		h.err = err
		return err
	}

	h.state = state
	h.bridge = plua.NewBridge(state.LuaState())

	// Grant capabilities
	for _, cap := range h.manifest.Capabilities {
		h.state.Sandbox().Grant(cap)
	}

	// Load the main file
	mainPath := h.manifest.MainPath()
	if err := h.state.DoFile(mainPath); err != nil {
		h.state.Close()
		h.state = nil
		h.pluginState = StateError
		h.err = fmt.Errorf("failed to load plugin: %w", err)
		return h.err
	}

	h.pluginState = StateLoaded
	h.err = nil
	return nil
}

// Activate calls the plugin's setup and activate functions.
func (h *Host) Activate(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pluginState != StateLoaded {
		return ErrNotLoaded
	}

	h.pluginState = StateActivating

	// Call setup(config) if it exists
	if err := h.callSetup(); err != nil {
		h.pluginState = StateError
		h.err = err
		return err
	}

	// Call activate() if it exists
	if err := h.callActivate(); err != nil {
		h.pluginState = StateError
		h.err = err
		return err
	}

	h.pluginState = StateActive
	h.err = nil
	return nil
}

// callSetup calls the plugin's setup function with configuration.
func (h *Host) callSetup() error {
	L := h.state.LuaState()
	setup := L.GetGlobal("setup")
	if setup == lua.LNil {
		return nil // setup is optional
	}

	if setup.Type() != lua.LTFunction {
		return nil // Not a function, skip
	}

	// Convert config to Lua table
	configTable := h.bridge.ToLuaValue(h.config)

	// Call setup(config)
	_, err := h.state.Call("setup", configTable)
	return err
}

// callActivate calls the plugin's activate function.
func (h *Host) callActivate() error {
	L := h.state.LuaState()
	activate := L.GetGlobal("activate")
	if activate == lua.LNil {
		return nil // activate is optional
	}

	if activate.Type() != lua.LTFunction {
		return nil // Not a function, skip
	}

	_, err := h.state.Call("activate")
	return err
}

// Deactivate calls the plugin's deactivate function and cleans up.
func (h *Host) Deactivate(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pluginState != StateActive {
		return nil // Nothing to deactivate
	}

	h.pluginState = StateDeactivating

	// Call deactivate() if it exists
	if err := h.callDeactivate(); err != nil {
		// Log but continue with cleanup
		h.err = err
	}

	h.pluginState = StateLoaded
	return nil
}

// callDeactivate calls the plugin's deactivate function.
func (h *Host) callDeactivate() error {
	L := h.state.LuaState()
	deactivate := L.GetGlobal("deactivate")
	if deactivate == lua.LNil {
		return nil // deactivate is optional
	}

	if deactivate.Type() != lua.LTFunction {
		return nil // Not a function, skip
	}

	_, err := h.state.Call("deactivate")
	return err
}

// Unload closes the Lua state and releases resources.
func (h *Host) Unload(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pluginState == StateUnloaded {
		return nil
	}

	// Deactivate first if active
	if h.pluginState == StateActive {
		h.pluginState = StateDeactivating
		_ = h.callDeactivate()
	}

	// Close Lua state
	if h.state != nil {
		h.state.Close()
		h.state = nil
	}

	h.bridge = nil
	h.pluginState = StateUnloaded
	h.err = nil

	// Clear tracked resources
	h.commands = nil
	h.keymaps = nil
	h.subscriptions = nil

	return nil
}

// Reload unloads and reloads the plugin.
func (h *Host) Reload(ctx context.Context) error {
	wasActive := h.State() == StateActive

	if err := h.Unload(ctx); err != nil {
		return err
	}

	if err := h.Load(ctx); err != nil {
		return err
	}

	if wasActive {
		return h.Activate(ctx)
	}

	return nil
}

// Call calls a global Lua function in the plugin.
func (h *Host) Call(fn string, args ...interface{}) ([]interface{}, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.state == nil {
		return nil, ErrNotLoaded
	}

	// Convert Go args to Lua values
	luaArgs := make([]lua.LValue, len(args))
	for i, arg := range args {
		luaArgs[i] = h.bridge.ToLuaValue(arg)
	}

	// Call the function
	results, err := h.state.Call(fn, luaArgs...)
	if err != nil {
		return nil, err
	}

	// Convert Lua results to Go values
	goResults := make([]interface{}, len(results))
	for i, result := range results {
		goResults[i] = h.bridge.ToGoValue(result)
	}

	return goResults, nil
}

// HasFunction returns true if the plugin has the named global function.
func (h *Host) HasFunction(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.state == nil {
		return false
	}

	v := h.state.GetGlobal(name)
	return v != nil && v.Type() == lua.LTFunction
}

// GetGlobal returns a global variable value.
func (h *Host) GetGlobal(name string) interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.state == nil {
		return nil
	}

	v := h.state.GetGlobal(name)
	return h.bridge.ToGoValue(v)
}

// SetGlobal sets a global variable.
func (h *Host) SetGlobal(name string, value interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == nil {
		return
	}

	h.state.SetGlobal(name, h.bridge.ToLuaValue(value))
}

// RegisterFunc registers a Go function as a global Lua function.
func (h *Host) RegisterFunc(name string, fn lua.LGFunction) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == nil {
		return
	}

	h.state.RegisterFunc(name, fn)
}

// RegisterModule registers a module with functions.
func (h *Host) RegisterModule(name string, funcs map[string]lua.LGFunction) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == nil {
		return
	}

	h.state.RegisterModule(name, funcs)
}

// LuaState returns the underlying Lua state.
// Use with caution - direct access bypasses safety measures.
func (h *Host) LuaState() *lua.LState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.state == nil {
		return nil
	}
	return h.state.LuaState()
}

// Bridge returns the Go-Lua bridge.
func (h *Host) Bridge() *plua.Bridge {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.bridge
}

// TrackCommand records a command registered by this plugin.
func (h *Host) TrackCommand(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.commands = append(h.commands, id)
}

// TrackKeymap records a keymap registered by this plugin.
func (h *Host) TrackKeymap(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.keymaps = append(h.keymaps, id)
}

// TrackSubscription records an event subscription by this plugin.
func (h *Host) TrackSubscription(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscriptions = append(h.subscriptions, id)
}

// TrackedCommands returns commands registered by this plugin.
func (h *Host) TrackedCommands() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string{}, h.commands...)
}

// TrackedKeymaps returns keymaps registered by this plugin.
func (h *Host) TrackedKeymaps() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string{}, h.keymaps...)
}

// TrackedSubscriptions returns event subscriptions by this plugin.
func (h *Host) TrackedSubscriptions() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string{}, h.subscriptions...)
}

// DoString executes Lua code in the plugin context.
func (h *Host) DoString(code string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == nil {
		return ErrNotLoaded
	}

	return h.state.DoString(code)
}

// DoFile executes a Lua file in the plugin context.
func (h *Host) DoFile(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == nil {
		return ErrNotLoaded
	}

	// Make path relative to plugin directory if not absolute
	if !filepath.IsAbs(path) {
		path = filepath.Join(h.manifest.Path(), path)
	}

	return h.state.DoFile(path)
}

// Stats returns runtime statistics for the plugin.
func (h *Host) Stats() HostStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return HostStats{
		Name:          h.name,
		State:         h.pluginState,
		Commands:      len(h.commands),
		Keymaps:       len(h.keymaps),
		Subscriptions: len(h.subscriptions),
		HasError:      h.err != nil,
	}
}

// HostStats contains runtime statistics for a plugin host.
type HostStats struct {
	Name          string
	State         State
	Commands      int
	Keymaps       int
	Subscriptions int
	HasError      bool
}
