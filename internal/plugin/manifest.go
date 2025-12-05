package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	plua "github.com/dshills/keystorm/internal/plugin/lua"
)

// Manifest describes a plugin's metadata and requirements.
type Manifest struct {
	// Identity
	Name        string `json:"name"`        // Unique identifier (e.g., "vim-surround")
	Version     string `json:"version"`     // Semver (e.g., "1.2.0")
	DisplayName string `json:"displayName"` // Human-readable name
	Description string `json:"description"` // Short description
	Author      string `json:"author"`      // Author name or org
	License     string `json:"license"`     // SPDX license identifier
	Homepage    string `json:"homepage"`    // URL to plugin homepage
	Repository  string `json:"repository"`  // Git repository URL

	// Entry point
	Main string `json:"main"` // Relative path to main Lua file (default: "init.lua")

	// Requirements
	MinEditorVersion string   `json:"minEditorVersion"` // Minimum Keystorm version
	Dependencies     []string `json:"dependencies"`     // Required plugins

	// Capabilities requested
	Capabilities []plua.Capability `json:"capabilities"`

	// Contributions
	Commands    []CommandContribution    `json:"commands"`
	Keybindings []KeybindingContribution `json:"keybindings"`
	Menus       []MenuContribution       `json:"menus"`

	// Configuration schema
	ConfigSchema map[string]ConfigProperty `json:"configSchema"`

	// Internal: path to the plugin directory
	path string
}

// CommandContribution declares a command the plugin provides.
type CommandContribution struct {
	ID          string `json:"id"`          // Command ID (e.g., "myplugin.doThing")
	Title       string `json:"title"`       // Display title
	Description string `json:"description"` // Long description
	Category    string `json:"category"`    // Command category
}

// KeybindingContribution declares default keybindings.
type KeybindingContribution struct {
	Keys    string `json:"keys"`    // Key sequence (e.g., "ctrl+shift+p")
	Command string `json:"command"` // Command to invoke
	When    string `json:"when"`    // Condition expression
	Mode    string `json:"mode"`    // Vim mode (normal, insert, visual)
}

// MenuContribution declares menu items.
type MenuContribution struct {
	ID      string `json:"id"`      // Menu ID
	Command string `json:"command"` // Command to invoke
	Title   string `json:"title"`   // Display title
	Group   string `json:"group"`   // Menu group
	When    string `json:"when"`    // Condition expression
}

// ConfigProperty describes a configuration option.
type ConfigProperty struct {
	Type        string      `json:"type"`        // string, number, boolean, array, object
	Default     interface{} `json:"default"`     // Default value
	Description string      `json:"description"` // Property description
	Enum        []string    `json:"enum"`        // Allowed values for enum types
	Minimum     *float64    `json:"minimum"`     // Minimum value for numbers
	Maximum     *float64    `json:"maximum"`     // Maximum value for numbers
	MinLength   *int        `json:"minLength"`   // Minimum length for strings/arrays
	MaxLength   *int        `json:"maxLength"`   // Maximum length for strings/arrays
}

// Validation errors.
var (
	ErrMissingName        = errors.New("manifest: name is required")
	ErrInvalidName        = errors.New("manifest: name must be alphanumeric with hyphens")
	ErrMissingVersion     = errors.New("manifest: version is required")
	ErrInvalidVersion     = errors.New("manifest: version must be valid semver")
	ErrInvalidMain        = errors.New("manifest: main must be a .lua file")
	ErrInvalidCapability  = errors.New("manifest: invalid capability")
	ErrInvalidConfigType  = errors.New("manifest: invalid config property type")
	ErrMissingCommandID   = errors.New("manifest: command id is required")
	ErrMissingCommandName = errors.New("manifest: command title is required")
)

// namePattern validates plugin names.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)

// semverPattern validates version strings (simplified semver).
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)

// validConfigTypes are the allowed configuration property types.
var validConfigTypes = map[string]bool{
	"string":  true,
	"number":  true,
	"boolean": true,
	"array":   true,
	"object":  true,
}

// validCapabilities are the known capability values.
var validCapabilities = map[plua.Capability]bool{
	plua.CapabilityFileRead:  true,
	plua.CapabilityFileWrite: true,
	plua.CapabilityNetwork:   true,
	plua.CapabilityShell:     true,
	plua.CapabilityClipboard: true,
	plua.CapabilityProcess:   true,
	plua.CapabilityUnsafe:    true,
}

// LoadManifest loads and validates a plugin manifest from a file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Set the path to the plugin directory
	m.path = filepath.Dir(path)

	// Apply defaults
	m.applyDefaults()

	// Validate
	if err := m.Validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

// LoadManifestFromDir loads a manifest from a plugin directory.
// Looks for plugin.json in the directory.
func LoadManifestFromDir(dir string) (*Manifest, error) {
	manifestPath := filepath.Join(dir, "plugin.json")
	return LoadManifest(manifestPath)
}

// NewManifestMinimal creates a minimal manifest for single-file plugins.
func NewManifestMinimal(name, path string) *Manifest {
	return &Manifest{
		Name:    name,
		Version: "0.0.0",
		Main:    "init.lua",
		path:    path,
	}
}

// applyDefaults sets default values for optional fields.
func (m *Manifest) applyDefaults() {
	if m.Main == "" {
		m.Main = "init.lua"
	}
	if m.Version == "" {
		m.Version = "0.0.0"
	}
}

// Validate checks that the manifest is valid.
func (m *Manifest) Validate() error {
	// Required fields
	if m.Name == "" {
		return ErrMissingName
	}
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("%w: %s", ErrInvalidName, m.Name)
	}

	if m.Version == "" {
		return ErrMissingVersion
	}
	if !semverPattern.MatchString(m.Version) {
		return fmt.Errorf("%w: %s", ErrInvalidVersion, m.Version)
	}

	// Main file
	if m.Main != "" && filepath.Ext(m.Main) != ".lua" {
		return fmt.Errorf("%w: %s", ErrInvalidMain, m.Main)
	}

	// Capabilities
	for _, cap := range m.Capabilities {
		if !validCapabilities[cap] {
			return fmt.Errorf("%w: %s", ErrInvalidCapability, cap)
		}
	}

	// Commands
	for i, cmd := range m.Commands {
		if cmd.ID == "" {
			return fmt.Errorf("%w at index %d", ErrMissingCommandID, i)
		}
		if cmd.Title == "" {
			return fmt.Errorf("%w at index %d (id: %s)", ErrMissingCommandName, i, cmd.ID)
		}
	}

	// Config schema
	for name, prop := range m.ConfigSchema {
		if prop.Type != "" && !validConfigTypes[prop.Type] {
			return fmt.Errorf("%w: %s.%s has type %q", ErrInvalidConfigType, m.Name, name, prop.Type)
		}
	}

	return nil
}

// Path returns the path to the plugin directory.
func (m *Manifest) Path() string {
	return m.path
}

// MainPath returns the full path to the main Lua file.
func (m *Manifest) MainPath() string {
	return filepath.Join(m.path, m.Main)
}

// HasCapability returns true if the plugin requests the capability.
func (m *Manifest) HasCapability(cap plua.Capability) bool {
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// GetConfigDefault returns the default value for a config property.
// Returns the default value and true if the property exists and has a default.
// Returns nil and false if the property doesn't exist or has no default.
func (m *Manifest) GetConfigDefault(key string) (interface{}, bool) {
	if prop, ok := m.ConfigSchema[key]; ok && prop.Default != nil {
		return prop.Default, true
	}
	return nil, false
}

// GetAllConfigDefaults returns all default config values.
func (m *Manifest) GetAllConfigDefaults() map[string]interface{} {
	defaults := make(map[string]interface{})
	for key, prop := range m.ConfigSchema {
		if prop.Default != nil {
			defaults[key] = prop.Default
		}
	}
	return defaults
}

// String returns a string representation of the manifest.
func (m *Manifest) String() string {
	display := m.DisplayName
	if display == "" {
		display = m.Name
	}
	return fmt.Sprintf("%s v%s", display, m.Version)
}

// MarshalJSON implements json.Marshaler.
func (m *Manifest) MarshalJSON() ([]byte, error) {
	// Create an alias to avoid infinite recursion
	type ManifestAlias Manifest
	return json.Marshal((*ManifestAlias)(m))
}

// Clone creates a deep copy of the manifest.
func (m *Manifest) Clone() *Manifest {
	clone := *m

	// Deep copy slices
	if m.Dependencies != nil {
		clone.Dependencies = make([]string, len(m.Dependencies))
		copy(clone.Dependencies, m.Dependencies)
	}

	if m.Capabilities != nil {
		clone.Capabilities = make([]plua.Capability, len(m.Capabilities))
		copy(clone.Capabilities, m.Capabilities)
	}

	if m.Commands != nil {
		clone.Commands = make([]CommandContribution, len(m.Commands))
		copy(clone.Commands, m.Commands)
	}

	if m.Keybindings != nil {
		clone.Keybindings = make([]KeybindingContribution, len(m.Keybindings))
		copy(clone.Keybindings, m.Keybindings)
	}

	if m.Menus != nil {
		clone.Menus = make([]MenuContribution, len(m.Menus))
		copy(clone.Menus, m.Menus)
	}

	if m.ConfigSchema != nil {
		clone.ConfigSchema = make(map[string]ConfigProperty, len(m.ConfigSchema))
		for k, v := range m.ConfigSchema {
			clone.ConfigSchema[k] = v
		}
	}

	return &clone
}
