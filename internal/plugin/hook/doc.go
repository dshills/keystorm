// Package hook provides dispatcher integration for the plugin system.
//
// This package bridges the plugin system with the dispatcher, allowing plugins
// to register handlers for actions and receive action callbacks.
//
// The main components are:
//   - PluginNamespaceHandler: Routes plugin.* actions to the appropriate plugin
//   - ActionBridge: Allows plugins to register action handlers with the dispatcher
//
// Example usage:
//
//	// Create namespace handler
//	nsHandler := hook.NewPluginNamespaceHandler(pluginManager)
//	dispatcher.RegisterNamespace("plugin", nsHandler)
//
//	// Now actions like "plugin.myplugin.goToDefinition" will be routed to the plugin
package hook
