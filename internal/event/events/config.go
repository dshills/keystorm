package events

import "github.com/dshills/keystorm/internal/event/topic"

// Config event topics.
const (
	// TopicConfigChanged is published when a setting changes.
	TopicConfigChanged topic.Topic = "config.changed"

	// TopicConfigSectionReloaded is published when a config section is reloaded.
	TopicConfigSectionReloaded topic.Topic = "config.section.reloaded"

	// TopicConfigKeymapUpdated is published when keymaps change.
	TopicConfigKeymapUpdated topic.Topic = "config.keymap.updated"

	// TopicConfigFileWatched is published when config file watching starts.
	TopicConfigFileWatched topic.Topic = "config.file.watched"

	// TopicConfigFileModified is published when a watched config file changes.
	TopicConfigFileModified topic.Topic = "config.file.modified"

	// TopicConfigValidationError is published when config validation fails.
	TopicConfigValidationError topic.Topic = "config.validation.error"
)

// ConfigSource indicates where a configuration value came from.
type ConfigSource string

// Configuration sources in order of precedence.
const (
	ConfigSourceDefault   ConfigSource = "default"
	ConfigSourceUser      ConfigSource = "user"
	ConfigSourceWorkspace ConfigSource = "workspace"
	ConfigSourceLanguage  ConfigSource = "language"
	ConfigSourceOverride  ConfigSource = "override"
)

// ConfigChanged is published when a setting changes.
type ConfigChanged struct {
	// Path is the dot-notation path to the setting (e.g., "editor.tabSize").
	Path string

	// OldValue is the previous value.
	OldValue any

	// NewValue is the new value.
	NewValue any

	// Source indicates where the new value came from.
	Source ConfigSource

	// Scope is the scope of the change (e.g., "global", "workspace", "buffer").
	Scope string
}

// ConfigSectionReloaded is published when a config section is reloaded.
type ConfigSectionReloaded struct {
	// Section is the section that was reloaded (e.g., "editor", "keybindings").
	Section string

	// Source is where the section was loaded from.
	Source ConfigSource

	// Path is the file path if loaded from a file.
	Path string

	// ChangeCount is the number of values that changed.
	ChangeCount int
}

// KeymapChange represents a single keymap modification.
type KeymapChange struct {
	// Keys is the key sequence (e.g., "ctrl+s", "jj").
	Keys string

	// Action is the action name.
	Action string

	// Mode is the mode for this binding.
	Mode string

	// Added is true if the binding was added, false if removed.
	Added bool

	// When is an optional condition for the binding.
	When string
}

// ConfigKeymapUpdated is published when keymaps change.
type ConfigKeymapUpdated struct {
	// Mode is the mode that was updated, or empty for all modes.
	Mode string

	// Changes lists all keymap modifications.
	Changes []KeymapChange

	// Source is where the keymap came from.
	Source ConfigSource
}

// ConfigFileWatched is published when config file watching starts.
type ConfigFileWatched struct {
	// Path is the path to the config file.
	Path string

	// Type is the config file type (e.g., "settings", "keybindings").
	Type string
}

// ConfigFileModified is published when a watched config file changes.
type ConfigFileModified struct {
	// Path is the path to the modified file.
	Path string

	// Type is the config file type.
	Type string

	// Action is the type of modification (e.g., "modified", "created", "deleted").
	Action string
}

// ConfigValidationError is published when config validation fails.
type ConfigValidationError struct {
	// Path is the dot-notation path to the invalid setting.
	Path string

	// Value is the invalid value.
	Value any

	// ExpectedType is what type was expected.
	ExpectedType string

	// Message describes the validation error.
	Message string

	// Source is where the invalid config came from.
	Source ConfigSource

	// FilePath is the file containing the error, if applicable.
	FilePath string

	// Line is the line number in the file, if known.
	Line int
}
