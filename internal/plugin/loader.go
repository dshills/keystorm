package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Loader discovers and loads plugins from the filesystem.
type Loader struct {
	// Search paths for plugins (checked in order)
	paths []string

	// Discovered plugins cache
	discovered map[string]*PluginInfo
}

// PluginInfo contains discovery information about a plugin.
type PluginInfo struct {
	Name     string
	Path     string
	Manifest *Manifest
	State    State
	Error    error
}

// LoaderOption configures a Loader.
type LoaderOption func(*Loader)

// WithPaths sets the plugin search paths.
func WithPaths(paths ...string) LoaderOption {
	return func(l *Loader) {
		l.paths = paths
	}
}

// NewLoader creates a new plugin loader.
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{
		paths:      DefaultPluginPaths(),
		discovered: make(map[string]*PluginInfo),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// DefaultPluginPaths returns the default plugin search paths.
func DefaultPluginPaths() []string {
	paths := make([]string, 0, 3)

	// User plugins: ~/.config/keystorm/plugins/
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "keystorm", "plugins"))
	}

	// User data plugins: ~/.local/share/keystorm/plugins/
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".local", "share", "keystorm", "plugins"))
	}

	// Project plugins: .keystorm/plugins/
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".keystorm", "plugins"))
	}

	return paths
}

// Paths returns the configured search paths.
func (l *Loader) Paths() []string {
	return l.paths
}

// AddPath adds a search path.
func (l *Loader) AddPath(path string) {
	l.paths = append(l.paths, path)
}

// Discover finds all plugins in the search paths.
// Returns plugins sorted by name.
func (l *Loader) Discover() ([]*PluginInfo, error) {
	l.discovered = make(map[string]*PluginInfo)

	for _, basePath := range l.paths {
		if err := l.discoverInPath(basePath); err != nil {
			// Log but continue - missing paths are not errors
			continue
		}
	}

	// Convert to sorted slice
	plugins := make([]*PluginInfo, 0, len(l.discovered))
	for _, info := range l.discovered {
		plugins = append(plugins, info)
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})

	return plugins, nil
}

// discoverInPath finds plugins in a single directory.
func (l *Loader) discoverInPath(basePath string) error {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error if path doesn't exist
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			// Check for single-file plugins (name.lua)
			if filepath.Ext(entry.Name()) == ".lua" {
				name := strings.TrimSuffix(entry.Name(), ".lua")
				l.addSingleFilePlugin(name, filepath.Join(basePath, entry.Name()))
			}
			continue
		}

		pluginPath := filepath.Join(basePath, entry.Name())
		info := l.inspectPlugin(entry.Name(), pluginPath)

		// Don't override earlier discoveries (first path wins)
		if _, exists := l.discovered[info.Name]; !exists {
			l.discovered[info.Name] = info
		}
	}

	return nil
}

// addSingleFilePlugin adds a single-file plugin.
func (l *Loader) addSingleFilePlugin(name, luaPath string) {
	if _, exists := l.discovered[name]; exists {
		return
	}

	// Create minimal manifest
	manifest := NewManifestMinimal(name, filepath.Dir(luaPath))
	manifest.Main = filepath.Base(luaPath)

	l.discovered[name] = &PluginInfo{
		Name:     name,
		Path:     filepath.Dir(luaPath),
		Manifest: manifest,
		State:    StateUnloaded,
	}
}

// inspectPlugin examines a plugin directory and returns its info.
func (l *Loader) inspectPlugin(name, path string) *PluginInfo {
	info := &PluginInfo{
		Name:  name,
		Path:  path,
		State: StateUnloaded,
	}

	// Try to load manifest
	manifestPath := filepath.Join(path, "plugin.json")
	if _, err := os.Stat(manifestPath); err == nil {
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			info.Error = fmt.Errorf("invalid manifest: %w", err)
			info.State = StateError
			return info
		}
		info.Manifest = manifest
		info.Name = manifest.Name // Use name from manifest
		return info
	}

	// No manifest - check for init.lua
	initPath := filepath.Join(path, "init.lua")
	if _, err := os.Stat(initPath); err == nil {
		info.Manifest = NewManifestMinimal(name, path)
		return info
	}

	// Check for plugin.lua (alternative entry point)
	pluginPath := filepath.Join(path, "plugin.lua")
	if _, err := os.Stat(pluginPath); err == nil {
		manifest := NewManifestMinimal(name, path)
		manifest.Main = "plugin.lua"
		info.Manifest = manifest
		return info
	}

	// No valid entry point found
	info.Error = ErrNoEntryPoint
	info.State = StateError
	return info
}

// Get returns info for a specific plugin by name.
func (l *Loader) Get(name string) (*PluginInfo, bool) {
	info, ok := l.discovered[name]
	return info, ok
}

// Refresh re-discovers plugins.
func (l *Loader) Refresh() ([]*PluginInfo, error) {
	return l.Discover()
}

// FindPlugin searches for a plugin by name across all paths.
// Returns the first match found.
func (l *Loader) FindPlugin(name string) (*PluginInfo, error) {
	// Check cache first
	if info, ok := l.discovered[name]; ok {
		return info, nil
	}

	// Search each path
	for _, basePath := range l.paths {
		// Check directory plugin
		pluginPath := filepath.Join(basePath, name)
		if stat, err := os.Stat(pluginPath); err == nil && stat.IsDir() {
			info := l.inspectPlugin(name, pluginPath)
			if info.Error == nil {
				l.discovered[name] = info
				return info, nil
			}
		}

		// Check single-file plugin
		luaPath := filepath.Join(basePath, name+".lua")
		if _, err := os.Stat(luaPath); err == nil {
			manifest := NewManifestMinimal(name, basePath)
			manifest.Main = name + ".lua"
			info := &PluginInfo{
				Name:     name,
				Path:     basePath,
				Manifest: manifest,
				State:    StateUnloaded,
			}
			l.discovered[name] = info
			return info, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, name)
}

// ValidatePlugin checks if a plugin at the given path is valid.
func (l *Loader) ValidatePlugin(path string) error {
	info := l.inspectPlugin(filepath.Base(path), path)
	if info.Error != nil {
		return info.Error
	}
	if info.Manifest == nil {
		return ErrNoEntryPoint
	}
	return info.Manifest.Validate()
}

// LoadManifestOnly loads just the manifest without full plugin setup.
func (l *Loader) LoadManifestOnly(name string) (*Manifest, error) {
	info, err := l.FindPlugin(name)
	if err != nil {
		return nil, err
	}
	return info.Manifest, nil
}

// ListNames returns the names of all discovered plugins.
func (l *Loader) ListNames() []string {
	names := make([]string, 0, len(l.discovered))
	for name := range l.discovered {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count returns the number of discovered plugins.
func (l *Loader) Count() int {
	return len(l.discovered)
}

// HasErrors returns true if any discovered plugins have errors.
func (l *Loader) HasErrors() bool {
	for _, info := range l.discovered {
		if info.Error != nil {
			return true
		}
	}
	return false
}

// Errors returns all plugins that have errors.
func (l *Loader) Errors() []*PluginInfo {
	var errored []*PluginInfo
	for _, info := range l.discovered {
		if info.Error != nil {
			errored = append(errored, info)
		}
	}
	return errored
}

// PluginsByState returns plugins filtered by state.
func (l *Loader) PluginsByState(state State) []*PluginInfo {
	var filtered []*PluginInfo
	for _, info := range l.discovered {
		if info.State == state {
			filtered = append(filtered, info)
		}
	}
	return filtered
}
