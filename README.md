# Keystorm

An AI-native programming editor written in Go.

Keystorm combines the modal editing power of Vim with modern IDE capabilities and deep AI integration. Built from the ground up to be extensible, performant, and AI-aware.

## Features

### Editor Capabilities
- **Modal Editing** - Vim-style modes (Normal, Insert, Visual, Command)
- **Multi-cursor Support** - Edit multiple locations simultaneously
- **Syntax Highlighting** - Fast, tree-sitter-ready highlighting
- **60+ FPS Rendering** - Smooth terminal rendering with dirty region updates
- **Undo/Redo** - Full command pattern with branching history
- **Macro Recording** - Record and playback keystroke sequences

### Project Intelligence
- **Workspace Model** - Project-aware file management
- **File Watching** - Automatic detection of external changes
- **Fuzzy Search** - Fast file and content search
- **Project Graph** - Understand relationships between files, modules, and tests

### Language Support
- **LSP Integration** - Completions, diagnostics, go-to-definition, formatting
- **Multi-language** - Per-language configuration and settings
- **Semantic Highlighting** - Enhanced token-based coloring

### Extensibility
- **Lua Plugins** - Lightweight, sandboxed plugin system
- **Capability-based Security** - Fine-grained plugin permissions
- **Event System** - Pub/sub architecture for loose coupling
- **Custom Keymaps** - Full keybinding customization

### AI Integration (Planned)
- **Context Engine** - Curated context from project graph for AI prompts
- **Intent Router** - Maps user actions to AI workflows
- **Agent Runtime** - Coder, Reviewer, Planner, and Verifier agents
- **Diff-based Proposals** - AI suggestions as reviewable diffs

## Requirements

- Go 1.25 or later
- A terminal with true color support (recommended)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/dshills/keystorm.git
cd keystorm

# Build
make build

# Install to GOPATH/bin
make install
```

### Binary

The built binary will be at `./bin/keystorm`.

## Usage

```bash
# Open with empty buffer
keystorm

# Open a file
keystorm file.go

# Open multiple files
keystorm main.go utils.go config.go

# Open a workspace
keystorm -w ./my-project

# Open in read-only mode
keystorm -R important-file.go
```

### Command Line Options

| Flag | Description |
|------|-------------|
| `-c, --config <path>` | Path to configuration file |
| `-w, --workspace <path>` | Workspace/project directory |
| `-d, --debug` | Enable debug mode |
| `--log-level <level>` | Log level: debug, info, warn, error |
| `-R, --readonly` | Open files in read-only mode |
| `-v, --version` | Show version information |
| `-h, --help` | Show help message |

## Configuration

Keystorm uses a layered configuration system (highest to lowest priority):

1. Command-line arguments
2. Environment variables
3. Plugin settings
4. Project config (`.keystorm/config.toml`)
5. User keymaps (`~/.config/keystorm/keymaps.toml`)
6. User settings (`~/.config/keystorm/settings.toml`)
7. Built-in defaults

### Example Configuration

```toml
# ~/.config/keystorm/settings.toml

[editor]
tab_size = 4
insert_spaces = true
line_numbers = true
relative_numbers = false
wrap_lines = false
cursor_style = "block"

[editor.theme]
name = "default"

[languages.go]
tab_size = 4
insert_spaces = false

[languages.python]
tab_size = 4
insert_spaces = true
```

## Architecture

Keystorm is built with a modular architecture consisting of 10 core components:

```
internal/
├── engine/       # Text buffer with B+ tree rope, undo/redo, multi-cursor
├── renderer/     # Terminal display, syntax highlighting, viewport
├── input/        # Keystroke handling, mode management, command palette
├── dispatcher/   # Input → action mapping with context awareness
├── event/        # Pub/sub event bus for component communication
├── config/       # 7-layer configuration system with live reload
├── plugin/       # Lua plugin system with sandboxing
├── lsp/          # Language Server Protocol client
├── project/      # Workspace model with file graph
├── integration/  # Terminal, Git, debugger, task runner
└── app/          # Application coordinator and lifecycle
```

### Design Principles

- **AI-Agnostic Core** - Editor layer is solid and independent
- **Event-Driven** - Loose coupling via pub/sub messaging
- **Thread-Safe** - Concurrent access with proper synchronization
- **Minimal Dependencies** - Lean on Go's standard library

## Development

### Prerequisites

```bash
# Install golangci-lint for linting
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Building

```bash
make build          # Build optimized binary
make build-debug    # Build with debug symbols
make run            # Build and run
```

### Testing

```bash
make test           # Run all tests
make test-race      # Run with race detector
make test-short     # Run short tests only
make coverage       # Generate coverage report
make bench          # Run benchmarks
```

### Code Quality

```bash
make fmt            # Format code
make vet            # Run go vet
make lint           # Run golangci-lint
make check          # Run fmt, vet, and lint
```

### Dependencies

```bash
make tidy           # Tidy go.mod
make deps           # Download dependencies
make verify         # Verify dependencies
make update         # Update all dependencies
```

## Plugin Development

Keystorm supports Lua plugins with a capability-based security model.

### Example Plugin

```lua
-- ~/.config/keystorm/plugins/hello/init.lua

local ks = require("ks")

-- Plugin metadata
local M = {
    name = "hello-world",
    version = "1.0.0",
    description = "A simple hello world plugin"
}

function M.setup(config)
    -- Register a command
    ks.command.register("HelloWorld", function()
        ks.ui.message("Hello from Lua!")
    end)

    -- Add a keybinding
    ks.keymap.set("n", "<leader>h", ":HelloWorld<CR>")
end

return M
```

### Available Capabilities

| Capability | Description |
|------------|-------------|
| `filesystem.read` | Read files |
| `filesystem.write` | Write files |
| `network` | Network access |
| `shell` | Shell command execution |
| `clipboard` | Clipboard access |
| `process.spawn` | Spawn processes |
| `unsafe` | Full Lua stdlib (trusted plugins only) |

## Project Structure

```
keystorm/
├── cmd/keystorm/       # Entry point
├── internal/           # Core packages (not importable)
├── pkg/                # Public API (future)
├── design/             # Architecture and specs
│   ├── specs/          # Design documents
│   └── plans/          # Implementation plans
├── bin/                # Build output
├── coverage/           # Test coverage reports
├── Makefile            # Build automation
├── go.mod              # Go module definition
└── README.md           # This file
```

## Dependencies

| Package | Purpose |
|---------|---------|
| [tcell/v2](https://github.com/gdamore/tcell) | Terminal handling |
| [fsnotify](https://github.com/fsnotify/fsnotify) | File system notifications |
| [go-toml/v2](https://github.com/pelletier/go-toml) | TOML configuration |
| [gopher-lua](https://github.com/yuin/gopher-lua) | Lua scripting |

## Roadmap

### Completed
- [x] Core text engine with B+ tree rope
- [x] Terminal rendering with tcell
- [x] Modal input system
- [x] Action dispatcher
- [x] Event bus with pub/sub
- [x] Configuration system
- [x] Lua plugin system
- [x] LSP client
- [x] Project workspace model
- [x] Application integration

### In Progress
- [ ] Performance optimization
- [ ] Extended test coverage
- [ ] Documentation improvements

### Planned
- [ ] AI orchestration layer
- [ ] Context engine for AI prompts
- [ ] Agent runtime (Coder, Reviewer, Planner)
- [ ] Model registry with provider abstraction
- [ ] Inline AI completions
- [ ] Multi-file refactoring with AI

## Contributing

Contributions are welcome! Please ensure:

1. Code passes all checks: `make check`
2. Tests pass with race detector: `make test-race`
3. New features include tests
4. Commits follow conventional commit format

## License

[License information to be added]

## Acknowledgments

Keystorm draws inspiration from:
- [Vim](https://www.vim.org/) - Modal editing
- [Neovim](https://neovim.io/) - Lua extensibility
- [VS Code](https://code.visualstudio.com/) - LSP and extensions
- [Helix](https://helix-editor.com/) - Modern terminal editor design
