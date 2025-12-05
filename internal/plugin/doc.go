// Package plugin provides the plugin system for Keystorm.
//
// The plugin system allows extending the editor with Lua scripts that can:
//   - Define custom commands
//   - Create keybindings
//   - Subscribe to editor events
//   - Integrate with the buffer, cursor, and mode systems
//   - Access LSP features (completions, diagnostics, etc.)
//   - Show UI notifications and statusline info
//   - Read and write configuration
//
// # Quick Start
//
// The easiest way to use the plugin system is through the System type:
//
//	// Create and initialize the plugin system
//	config := plugin.DefaultSystemConfig()
//	config.BufferProvider = myBufferProvider
//	config.CursorProvider = myCursorProvider
//	// ... set other providers
//
//	sys := plugin.NewSystem(config)
//	if err := sys.Initialize(); err != nil {
//	    log.Fatal(err)
//	}
//	defer sys.Shutdown(context.Background())
//
//	// Load all plugins
//	if err := sys.LoadAll(context.Background()); err != nil {
//	    log.Printf("some plugins failed to load: %v", err)
//	}
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
//	  "capabilities": ["filesystem.read", "keymap", "command"],
//	  "commands": [
//	    {"id": "my-plugin.doThing", "title": "Do Thing"}
//	  ]
//	}
//
// # Capabilities
//
// Plugins must declare required capabilities in their manifest:
//
// Filesystem capabilities:
//   - filesystem.read: Read files from the filesystem
//   - filesystem.write: Write files to the filesystem
//
// Editor capabilities:
//   - keymap: Register custom keybindings
//   - command: Register commands in the command palette
//   - event: Subscribe to and emit editor events
//   - config: Read and write configuration values
//   - ui: Show notifications, update statusline
//   - lsp: Access language server features
//
// System capabilities:
//   - network: Make network requests
//   - shell: Execute shell commands
//   - clipboard: Access system clipboard
//   - process.spawn: Spawn external processes
//   - unsafe: Full access (disables sandbox restrictions)
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
// # Architecture
//
// The plugin system consists of several components:
//
//   - System: High-level facade that coordinates all components
//   - Manager: Manages plugin lifecycle (discovery, loading, activation)
//   - Host: Per-plugin Lua state and lifecycle management
//   - Loader: Discovers plugins in the filesystem
//   - Registry: Manages API modules available to plugins
//
// # Security
//
// Plugins run in a sandboxed Lua environment with:
//   - Dangerous functions removed (dofile, loadfile, load, os.execute, etc.)
//   - Instruction counting to prevent infinite loops
//   - Capability-based access control
//   - Execution timeouts
//   - Namespace-restricted configuration access
//
// # Available API Modules
//
// Plugins access editor functionality through the ks module:
//
//	local ks = require("ks")
//
// Available submodules:
//   - ks.buf: Buffer operations (text, lines, insert, delete)
//   - ks.cursor: Cursor positioning and selection
//   - ks.mode: Mode switching (normal, insert, visual)
//   - ks.keymap: Register custom keybindings
//   - ks.command: Register commands
//   - ks.event: Subscribe to editor events
//   - ks.config: Read/write configuration
//   - ks.ui: Notifications, statusline, dialogs
//   - ks.lsp: Language server features
//   - ks.util: Utility functions
//
// # Example Plugin
//
//	-- init.lua
//	local ks = require("ks")
//
//	function setup(config)
//	    -- Initialize with config (config is optional)
//	end
//
//	function activate()
//	    -- Register a command
//	    ks.command.register("my-plugin.greet", function()
//	        ks.ui.notify("Hello from plugin!", "info")
//	    end, { title = "Greet User" })
//
//	    -- Register a keybinding
//	    ks.keymap.set("normal", "<leader>g", function()
//	        ks.command.execute("my-plugin.greet")
//	    end)
//
//	    -- Subscribe to buffer changes
//	    ks.event.on("buffer.change", function(data)
//	        -- Use ks.ui for user-visible messages
//	        ks.ui.notify("Buffer changed: " .. (data.file or ""), "info")
//	    end)
//	end
//
//	function deactivate()
//	    -- Cleanup (keymaps and commands are auto-cleaned)
//	end
package plugin
