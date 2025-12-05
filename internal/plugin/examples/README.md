# Example Plugins

This directory contains example plugins demonstrating the Keystorm plugin API.

## hello-world

A minimal plugin that demonstrates:
- Plugin manifest structure
- The `setup`, `activate`, and `deactivate` lifecycle functions
- Registering commands with `ks.command.register`
- Showing notifications with `ks.ui.notify`
- Using plugin configuration

## word-count

A more complete plugin that demonstrates:
- Buffer access with `ks.buf.text()`
- Event subscriptions with `ks.event.on`
- Statusline updates with `ks.ui.statusline`
- Multiple commands
- State management within a plugin

## Using These Examples

To use these plugins in Keystorm:

1. Copy the plugin directory to your plugins folder:
   ```bash
   cp -r hello-world ~/.config/keystorm/plugins/
   ```

2. The plugin will be automatically discovered and loaded on startup.

3. Run plugin commands from the command palette.

## Creating Your Own Plugin

1. Create a directory in `~/.config/keystorm/plugins/your-plugin/`

2. Create a `plugin.json` manifest:
   ```json
   {
     "name": "your-plugin",
     "version": "1.0.0",
     "main": "init.lua",
     "capabilities": ["command"]
   }
   ```

3. Create `init.lua` with your plugin code:
   ```lua
   local ks = require("ks")

   function activate()
       -- Your plugin code here
   end

   function deactivate()
       -- Cleanup code here
   end
   ```

4. Declare any capabilities your plugin needs in the manifest.

## Available Capabilities

- `command` - Register commands in the command palette
- `keymap` - Register custom keybindings
- `event` - Subscribe to and emit editor events
- `config` - Read and write configuration
- `ui` - Show notifications, update statusline
- `lsp` - Access language server features
- `filesystem.read` - Read files
- `filesystem.write` - Write files
