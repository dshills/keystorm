// Package plugin provides the plugin system for Keystorm.
//
// The plugin system allows extending the editor with Lua scripts that can:
//   - Define custom commands
//   - Create keybindings
//   - Subscribe to editor events
//   - Integrate with the buffer, cursor, and mode systems
//
// # Plugin Structure
//
// Plugins can be either single-file or directory-based:
//
// Single-file plugin:
//
//	~/.config/keystorm/plugins/myplugin.lua
//
// Directory plugin:
//
//	~/.config/keystorm/plugins/myplugin/
//	├── plugin.json      # Manifest (optional but recommended)
//	├── init.lua         # Entry point
//	└── lib/             # Additional modules
//	    └── helper.lua
//
// # Manifest
//
// The plugin.json manifest describes the plugin:
//
//	{
//	  "name": "my-plugin",
//	  "version": "1.0.0",
//	  "displayName": "My Plugin",
//	  "description": "A helpful plugin",
//	  "main": "init.lua",
//	  "capabilities": ["filesystem.read"],
//	  "commands": [
//	    {"id": "my-plugin.doThing", "title": "Do Thing"}
//	  ]
//	}
//
// # Capabilities
//
// Plugins must declare required capabilities in their manifest:
//   - filesystem.read: Read files
//   - filesystem.write: Write files
//   - network: Make network requests
//   - shell: Execute shell commands
//   - clipboard: Access clipboard
//   - process.spawn: Spawn processes
//   - unsafe: Disable sandbox restrictions
//
// # Plugin Lifecycle
//
// Plugins go through these states:
//
//	StateUnloaded -> Load() -> StateLoaded
//	StateLoaded -> Activate() -> StateActive
//	StateActive -> Deactivate() -> StateLoaded
//	StateLoaded -> Unload() -> StateUnloaded
//
// The Host type manages a single plugin's lifecycle and Lua state.
// The Manager type (to be implemented) coordinates multiple plugins.
//
// # Security
//
// Plugins run in a sandboxed Lua environment with:
//   - Dangerous functions removed (dofile, loadfile, load, os.execute, etc.)
//   - Instruction counting to prevent infinite loops
//   - Capability-based access control
//   - Execution timeouts
//
// # Example Plugin
//
//	-- init.lua
//	local ks = require("ks")
//
//	function setup(config)
//	    -- Initialize with config
//	end
//
//	function activate()
//	    -- Register commands and keybindings
//	    ks.command.register("my-plugin.hello", function()
//	        ks.ui.notify("Hello from plugin!")
//	    end)
//	end
//
//	function deactivate()
//	    -- Cleanup
//	end
package plugin
