package events

import "github.com/dshills/keystorm/internal/event/topic"

// Plugin event topics.
const (
	// TopicPluginLoaded is published when a plugin is loaded from disk.
	TopicPluginLoaded topic.Topic = "plugin.loaded"

	// TopicPluginUnloaded is published when a plugin is unloaded.
	TopicPluginUnloaded topic.Topic = "plugin.unloaded"

	// TopicPluginActivated is published when a plugin is activated.
	TopicPluginActivated topic.Topic = "plugin.activated"

	// TopicPluginDeactivated is published when a plugin is deactivated.
	TopicPluginDeactivated topic.Topic = "plugin.deactivated"

	// TopicPluginReloaded is published when a plugin is hot-reloaded.
	TopicPluginReloaded topic.Topic = "plugin.reloaded"

	// TopicPluginError is published when a plugin encounters an error.
	TopicPluginError topic.Topic = "plugin.error"

	// TopicPluginActionRegistered is published when a plugin registers an action.
	TopicPluginActionRegistered topic.Topic = "plugin.action.registered"

	// TopicPluginActionUnregistered is published when a plugin unregisters an action.
	TopicPluginActionUnregistered topic.Topic = "plugin.action.unregistered"

	// TopicPluginSettingsChanged is published when plugin settings change.
	TopicPluginSettingsChanged topic.Topic = "plugin.settings.changed"

	// TopicPluginMessage is published when a plugin sends a message.
	TopicPluginMessage topic.Topic = "plugin.message"

	// TopicPluginAPICall is published when a plugin makes an API call.
	TopicPluginAPICall topic.Topic = "plugin.api.call"

	// TopicPluginDiscovered is published when a plugin is discovered.
	TopicPluginDiscovered topic.Topic = "plugin.discovered"

	// TopicPluginInstalled is published when a plugin is installed.
	TopicPluginInstalled topic.Topic = "plugin.installed"

	// TopicPluginUninstalled is published when a plugin is uninstalled.
	TopicPluginUninstalled topic.Topic = "plugin.uninstalled"
)

// PluginErrorSeverity indicates the severity of a plugin error.
type PluginErrorSeverity string

// Plugin error severities.
const (
	PluginErrorWarning PluginErrorSeverity = "warning"
	PluginErrorError   PluginErrorSeverity = "error"
	PluginErrorFatal   PluginErrorSeverity = "fatal"
)

// PluginLoaded is published when a plugin is loaded from disk.
type PluginLoaded struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Path is the path to the plugin.
	Path string

	// Version is the plugin version.
	Version string

	// Author is the plugin author.
	Author string

	// Description is the plugin description.
	Description string

	// Dependencies lists other plugins this plugin depends on.
	Dependencies []string
}

// PluginUnloaded is published when a plugin is unloaded.
type PluginUnloaded struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Reason explains why the plugin was unloaded.
	Reason string
}

// PluginActivated is published when a plugin is activated.
type PluginActivated struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Capabilities lists the plugin's capabilities.
	Capabilities []string

	// ActivationTrigger describes what caused activation.
	ActivationTrigger string
}

// PluginDeactivated is published when a plugin is deactivated.
type PluginDeactivated struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Reason explains why the plugin was deactivated.
	Reason string

	// WasGraceful indicates if deactivation was graceful.
	WasGraceful bool
}

// PluginReloaded is published when a plugin is hot-reloaded.
type PluginReloaded struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// OldVersion is the previous version.
	OldVersion string

	// NewVersion is the new version.
	NewVersion string

	// Changes describes what changed.
	Changes []string
}

// PluginError is published when a plugin encounters an error.
type PluginError struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// ErrorMessage describes the error.
	ErrorMessage string

	// Severity indicates the error severity.
	Severity PluginErrorSeverity

	// Context provides additional context.
	Context map[string]any

	// Stack is the error stack trace, if available.
	Stack string
}

// PluginActionRegistered is published when a plugin registers an action.
type PluginActionRegistered struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// ActionName is the registered action name.
	ActionName string

	// Description describes the action.
	Description string

	// DefaultBinding is the default key binding, if any.
	DefaultBinding string
}

// PluginActionUnregistered is published when a plugin unregisters an action.
type PluginActionUnregistered struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// ActionName is the unregistered action name.
	ActionName string
}

// PluginSettingsChanged is published when plugin settings change.
type PluginSettingsChanged struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Path is the setting path that changed.
	Path string

	// OldValue is the previous value.
	OldValue any

	// NewValue is the new value.
	NewValue any
}

// PluginMessage is published when a plugin sends a message.
type PluginMessage struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// MessageType categorizes the message.
	MessageType string

	// Message is the message content.
	Message string

	// Data contains additional data.
	Data map[string]any
}

// PluginAPICall is published when a plugin makes an API call.
type PluginAPICall struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Method is the API method called.
	Method string

	// Args are the call arguments.
	Args map[string]any

	// Success indicates if the call succeeded.
	Success bool

	// Error is the error message if the call failed.
	Error string
}

// PluginDiscovered is published when a plugin is discovered.
type PluginDiscovered struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Path is the path where the plugin was found.
	Path string

	// Version is the plugin version.
	Version string

	// Source indicates where the plugin came from.
	Source string
}

// PluginInstalled is published when a plugin is installed.
type PluginInstalled struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Version is the installed version.
	Version string

	// Path is the installation path.
	Path string

	// Source indicates where the plugin was installed from.
	Source string
}

// PluginUninstalled is published when a plugin is uninstalled.
type PluginUninstalled struct {
	// PluginName is the unique plugin identifier.
	PluginName string

	// Version was the installed version.
	Version string

	// Path was the installation path.
	Path string
}
