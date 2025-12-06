# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Keystorm is an AI-native programming editor written in Go 1.25+. It combines traditional editor capabilities (like Vim's modal editing and VS Code's extensibility) with deep AI integration for code generation, refactoring, and intelligent assistance.

## Build Commands

```bash
# Build the project
go build ./...

# Run tests
go test ./...

# Run a single test
go test -run TestName ./path/to/package

# Format code
go fmt ./...

# Vet code
go vet ./...
```

## Architecture

The editor follows a modular architecture with 10 core components in `internal/`:

### Core Editor Components
- **engine**: Text buffer with piece tree/rope structure, undo/redo, multi-cursor support, and change tracking for AI context
- **renderer**: Display layer handling line layout, syntax highlighting, 60+ FPS rendering, scrolling, and dirty region updates
- **input**: Keystroke and mouse handling, mode-based input (Vim-style), command palette, fuzzy search
- **dispatcher**: Maps input to actions with context awareness (mode, selection, file type, extension overrides)

### Project & Configuration
- **project**: Workspace model with file watchers, indexing, and search. Models project as a graph (files, modules, services, tests, APIs as nodes; imports/calls/ownership as edges)
- **config**: Configuration system for settings, keymaps, per-language configs, and plugin settings

### Extensibility
- **plugin**: Extension/plugin system running in separate processes via RPC
- **lsp**: Language Server Protocol integration for completions, diagnostics, go-to-definition, formatting, and semantic highlighting
- **integration**: Terminal, Git, debugger, and task runner integration

### Communication
- **event**: Internal event and messaging bus for buffer changes, config changes, window events, and extension events

## AI Integration Architecture

Keystorm is designed as an "AI-native" editor with these key AI components (to be implemented):

1. **Intent Router**: Maps user actions to AI workflows based on editor state, command, and context
2. **Workflow/Agent Runtime**: LangGraph-like pipeline system with agents (Coder, Reviewer, Planner, Verifier)
3. **Context Engine**: Assembles curated context from project graph, recent edits, errors, and semantic search
4. **Model Registry**: Abstracts model providers (OpenAI, Anthropic, local) with routing policies for different tasks

## Design Principles

- Core editor layer should be AI-agnostic ("dumb but rock solid")
- AI proposals presented as diffs for human review/partial acceptance
- Tool calls go through orchestrator, not direct LLM-to-tool communication
- Support both sync flows (inline completion) and async flows (multi-file refactors)
- Audit trail for AI actions linked to git commits
- Do not commit before mcp-pr review is run
- Do not cimmit before mcp-pr review