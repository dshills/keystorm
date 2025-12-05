-- Word Count Plugin for Keystorm
-- Demonstrates buffer access, events, and statusline updates

local ks = require("ks")

-- Plugin state
local config = {
    showInStatusline = true,
    format = "W:%d L:%d C:%d"
}

local stats = {
    words = 0,
    lines = 0,
    chars = 0
}

-- Count words in text
local function countWords(text)
    if not text or text == "" then
        return 0
    end
    local count = 0
    for _ in string.gmatch(text, "%S+") do
        count = count + 1
    end
    return count
end

-- Count lines in text
local function countLines(text)
    if not text or text == "" then
        return 0
    end
    local count = 1
    for _ in string.gmatch(text, "\n") do
        count = count + 1
    end
    return count
end

-- Update statistics from buffer
local function updateStats()
    local text = ks.buf.text()
    if text then
        stats.chars = #text
        stats.words = countWords(text)
        stats.lines = countLines(text)
    else
        stats.chars = 0
        stats.words = 0
        stats.lines = 0
    end
end

-- Update statusline with current stats
local function updateStatusline()
    if config.showInStatusline then
        local status = string.format(config.format, stats.words, stats.lines, stats.chars)
        ks.ui.statusline("word_count", status, { priority = 50 })
    else
        ks.ui.statusline("word_count", nil) -- Clear statusline item
    end
end

-- Called when the plugin is loaded with configuration
function setup(cfg)
    if cfg then
        if cfg.showInStatusline ~= nil then
            config.showInStatusline = cfg.showInStatusline
        end
        if cfg.format then
            config.format = cfg.format
        end
    end
end

-- Called when the plugin is activated
function activate()
    -- Initial stats update
    updateStats()
    updateStatusline()

    -- Register command to show statistics
    ks.command.register("word-count.show", function()
        updateStats()
        local message = string.format(
            "Document Statistics:\n  Words: %d\n  Lines: %d\n  Characters: %d",
            stats.words, stats.lines, stats.chars
        )
        ks.ui.notify(message, "info")
    end, {
        title = "Word Count: Show Statistics",
        description = "Display word, line, and character counts"
    })

    -- Register command to toggle statusline display
    ks.command.register("word-count.toggle", function()
        config.showInStatusline = not config.showInStatusline
        updateStatusline()
        local status = config.showInStatusline and "enabled" or "disabled"
        ks.ui.notify("Word count statusline " .. status, "info")
    end, {
        title = "Word Count: Toggle Statusline",
        description = "Toggle word count display in statusline"
    })

    -- Subscribe to buffer changes
    ks.event.on("buffer.change", function(data)
        updateStats()
        updateStatusline()
    end)

    -- Subscribe to buffer switch
    ks.event.on("buffer.switch", function(data)
        updateStats()
        updateStatusline()
    end)
end

-- Called when the plugin is deactivated
function deactivate()
    -- Clear statusline
    ks.ui.statusline("word_count", nil)
end
