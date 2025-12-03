# Keystorm programming editor

## Editor modules

1. Core Text Engine
location: internal/engine
This is the beating heart. It stores the buffer, tracks edits, handles undo/redo, manages cursor positions, and maintains the model of “what is this file right now?” Vim uses a rope-like internal structure; VS Code uses a piece tree. Neither uses naïve strings unless they enjoy pain.

2. Rendering Layer
location: internal/renderer
Turning that internal buffer into glyphs on a screen isn’t trivial. Rendering handles:
• line layout
• syntax coloration
• drawing text at 60+ FPS
• scrolling
• dirty region updates

Vim relies on your terminal as the rendering surface. VS Code rolls its own in the browser engine using canvas/DOM trickery.

3. Input Engine
location: internal/input
Keystrokes, mouse actions, shortcuts, command palettes, fuzzy search—everything that turns human fidgeting into editor commands.
Vim is mode-based. VS Code is not. This difference cascades everywhere.

4. Command/Action Dispatcher
location: internal/dispatcher
Once you press a key or pick a command, someone must decide: “What does that mean right now?”
This dispatcher matches input → action, often with context: mode, selection, active file type, extension overrides, etc.

5. Extension/Plugin System
location: internal/plugin
The editor’s mutation chamber.
Both Vim and VS Code delegate huge swaths of capability to plugins: language support, themes, UI elements, macros, AI assistants, you name it.
VS Code runs extensions in separate processes using an RPC layer. Vim leans on its internal scripting languages plus external processes via jobstart.

6. Language Services Layer (LSP / Analysis Engines)
location: internal/lsp
Modern editors offload intelligence to language servers:
• autocompletion
• go-to-definition
• diagnostics
• formatting
• refactoring
• semantic highlighting
Vim (via coc.nvim, nvim-lsp, etc.) and VS Code both speak LSP, but VS Code was designed around it from day one.

7. File/Project Model
location: internal/project
Editors need to keep track of:
• open files
• file watchers
• folders/workspaces
• indexing
• search
VS Code builds a real workspace model. Vim treats files individually unless plugins add project intelligence.

8. Configuration System
location: internal/config
Where all chaos begins.
Config files, settings, overrides, per-language configs, keymaps, plugin settings.
Vim: .vimrc.
VS Code: JSON config plus UI settings.

9. Integration Layer (Terminal, Git, Debugger, Tasks)
location: internal/integration
This is where “just an editor” becomes “mini IDE.”
VS Code: integrated terminal, debugger protocol, Git interface, task runner.
Vim: can do all of this, but typically via plugins and duct tape.

10. Event & Messaging Bus
location: internal/event
The editor’s nervous system.
Keeps components informed about buffer changes, config changes, window events, extension events, etc.

## Module descriptions

1. Core Editor Engine (Same Old, Still Essential)

You don’t escape the basics:
	•	Text Buffer & Undo/Redo Engine
Piece tree/rope/whatever. Needs:
	•	Multi-cursor, multi-selection
	•	Snapshotting for AI diffs
	•	Cheap “what changed since X?” for context building
	•	Rendering & Input
	•	Syntax highlighting (but AI-enhanced semantic info layered on top)
	•	Fast rendering for huge files
	•	Keymaps, modes, command palette

This layer should be mostly AI-agnostic. Dumb but rock solid.

2. Workspace & Project Graph

Instead of “just files in a folder”, model the project as a graph:
	•	Nodes: files, modules, services, tests, APIs, DB schemas, configs
	•	Edges: imports, calls, ownership, “this test targets that code”, etc.
	•	Capabilities:
	•	Build a Code Graph (dependencies, call graph where possible)
	•	Maintain a Runtime Graph (services, endpoints, queues, DBs)
	•	Keep a Responsibility Map (what belongs to which bounded context)

This graph becomes the main prompt substrate for the AI layer.

3. AI Orchestration Layer (Brain + Nervous System)

This is where it stops being “just an editor” and becomes an AI-native environment.

3.1 Intent Router

Core job: “User did X. Which AI flow, if any, should run?”

Inputs:
	•	Editor state (cursor, selection, active file)
	•	Command invoked (e.g., “Refactor function”, “Explain this”, “Generate tests”)
	•	Context (project type, language, configured workflows)

Outputs:
	•	A selected workflow (pipeline/graph of tools + models)
	•	Parameters (temperature, models, strictness, etc.)

Think: a tiny decision engine that maps interactions → pipelines.

3.2 Workflow/Agent Runtime

You want something LangGraph-like:
	•	Nodes: tools, models, decision steps
	•	Edges: data passing / branching
	•	Built-in agents:
	•	Coder: writes/edits code
	•	Reviewer: critiques diffs, enforces style/constraints
	•	Planner: breaks down larger tasks into steps
	•	Verifier: runs tests, linters, compilers and feeds results back in

This runtime must support:
	•	Sync flows (inline completion, quick refactor)
	•	Async flows (multi-file refactors, feature-level changes)
	•	Checkpointing of tool traces so you can inspect “WTF did the AI do?”


4. Context Engine (What the AI Actually Sees)

This is the secret sauce. The AI shouldn’t see “a random blob of text”; it should see curated, structured context.

Components:
	•	Context Builder
Given an intent, assemble:
	•	Current file / selection
	•	Related files from the project graph
	•	Relevant tests
	•	Recent edits in this session
	•	Error messages (compiler, linter, runtime)
	•	Possibly, external docs / tickets
	•	Vector Memory / Semantic Search
	•	Code chunks, docs, READMEs, ADRs, tickets
	•	Tagging: by feature, service, module, tech, owner
	•	Support “what’s relevant to this function?” queries
	•	Context Policies
	•	Hard limits per model (tokens)
	•	Priority rules (tests > docstrings > unrelated files)
	•	Mode-sensitive (explain vs refactor vs generate-test)

So the pipeline is: Intent → Context query → Packed prompt → Model.

5. Tool Layer (LSP + MCP + Shell + Cloud)

This is the “hands” of the system.
	•	Language Tools (LSP)
	•	Completions, diagnostics, go-to-def, etc.
	•	AI orchestration uses LSP as a structured source of truth
	•	MCP / Tool Protocol Adapter
	•	External tools: CI, ticketing, DB schema explorer, cloud APIs
	•	Each tool: typed inputs/outputs
	•	AI runtime calls them as part of workflows
	•	Local Runtime Tools
	•	Test runner
	•	Build tool
	•	Formatter, linter
	•	Script runner for custom commands (Make, Taskfile, NPM, etc.)

Think of this layer as a unified tool bus. LLMs don’t talk to tools directly; they go via the orchestrator.

6. Models & Provider Abstraction

You don’t want model calls littered throughout the codebase.
	•	Model Registry
	•	Named models: code-strict, code-creative, planner, reviewer
	•	Backed by: OpenAI, Anthropic, local models, etc.
	•	Routing Policies
	•	Per-task selection: generation vs refactor vs planning
	•	Fallback behavior
	•	Cost/latency aware choices
	•	Execution Modes
	•	Streaming for inline completions
	•	Non-streaming for bigger diffs/plans
	•	Batch requests for large-scale refactors

7. UX & Interaction Model

You don’t want an “AI sidebar that vomits text.” You want tight, controlled integration.

Core UI surfaces:
	•	Inline:
	•	Ghost text completions
	•	In-place refactor previews
	•	Quick-fix lightbulbs (like “apply this change” or “split into function”)
	•	Diff Views:
	•	AI proposals as patches
	•	Review comments from AI
	•	Ability to partially accept/reject
	•	Panel Views:
	•	“Task runs” – show a history of AI workflows, logs, and tools used
	•	“Context inspector” – why did it pick that file?
	•	“Agent chat” – conversational layer tied to the current project state

Goal: AI acts like a disciplined teammate, not a hallucinating firehose.

8. Policy, Control, and Safety

Developers (and orgs) will want guardrails.
	•	Policy Engine
	•	Where models can send data (no prod secrets to random SaaS)
	•	Which tools are allowed in which contexts
	•	Model usage limits and quotas
	•	Audit & Observability
	•	Log: prompts, tool calls, diffs applied
	•	Link AI changes to git commits and authorship
	•	Allow “explain this change” post-hoc
	•	Approval Flows
	•	For large refactors or multi-file changes:
	•	AI proposes → Human reviews → Editor applies → Tests run

9. Extensibility Model

You want others to extend this without turning it into plugin hell.
	•	Extension Types
	•	New tools (MCP, CLI, HTTP, etc.)
	•	New workflows (pipelines/graphs)
	•	New UI surfaces/panels
	•	Custom context collectors (e.g., “pull from Jira”, “query metrics”)
	•	Config Format
	•	Declarative pipeline definitions (YAML/JSON/TOML/whatever)
	•	Strongly typed interfaces where possible
	•	“Preset workflows” for common use cases (bugfix, feature scaffold, test generation)

10. Example Flow: “Refactor This Function”

Concrete example to tie it together:
	1.	User selects function → hits Refactor: Extract and Simplify.
	2.	Intent Router:
	•	Maps this to refactor_function workflow.
	3.	Context Engine:
	•	Grabs current file, related files, call sites, tests.
	4.	Workflow Runtime:
	•	Step 1: planner model decides strategy.
	•	Step 2: coder model proposes a patch.
	•	Step 3: reviewer model checks style, complexity, and correctness.
	•	Step 4: Tests run via tool layer.
	•	Step 5: If tests pass, show diff; if not, loop coder+reviewer with error output.
	5.	UI shows:
	•	Proposed diff
	•	Test results
	•	Explanation: “What I changed and why”

User accepts or modifies the patch like normal code.

