-- Hello World Plugin for Keystorm
-- Demonstrates basic plugin features: commands, UI notifications, and configuration

local ks = require("ks")

-- Plugin configuration (set during setup)
local config = {
    greeting = "Hello",
    name = "World"
}

-- Called when the plugin is loaded with configuration
function setup(cfg)
    if cfg then
        config.greeting = cfg.greeting or config.greeting
        config.name = cfg.name or config.name
    end
end

-- Called when the plugin is activated
function activate()
    -- Register the greet command
    ks.command.register("hello-world.greet", function()
        local message = config.greeting .. ", " .. config.name .. "!"
        ks.ui.notify(message, "info")
    end, {
        title = "Hello World: Greet",
        description = "Display a greeting message"
    })

    -- Register the info command
    ks.command.register("hello-world.info", function()
        local info = string.format(
            "Hello World Plugin v1.0.0\nGreeting: %s\nName: %s",
            config.greeting,
            config.name
        )
        ks.ui.notify(info, "info")
    end, {
        title = "Hello World: Show Info",
        description = "Display plugin information"
    })

    -- Show activation message
    ks.ui.notify("Hello World plugin activated!", "info")
end

-- Called when the plugin is deactivated
function deactivate()
    -- Commands are automatically unregistered
    -- Any cleanup would go here
end
