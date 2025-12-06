# Keystorm Language Server Protocol (LSP) - Implementation Plan

## Comprehensive Design Document for `internal/lsp`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Server Management](#6-server-management)
7. [Protocol Implementation](#7-protocol-implementation)
8. [Document Synchronization](#8-document-synchronization)
9. [Feature Handlers](#9-feature-handlers)
10. [Diagnostics System](#10-diagnostics-system)
11. [Integration with Editor](#11-integration-with-editor)
12. [Implementation Phases](#12-implementation-phases)
13. [Testing Strategy](#13-testing-strategy)
14. [Performance Considerations](#14-performance-considerations)

---

## 1. Executive Summary

The Language Server Protocol (LSP) layer provides intelligent code features by communicating with external language servers. It abstracts the complexity of JSON-RPC communication, server lifecycle management, and protocol negotiation while exposing a clean interface to the rest of Keystorm.

### Role in the Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   Dispatcher    │─────▶│   LSP Client    │◀────▶│  Language       │
│  (completion,   │      │  (internal/lsp) │      │  Server         │
│   go-to-def)    │      └────────┬────────┘      │  (gopls, etc.)  │
└─────────────────┘               │               └─────────────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            ┌───────────┐  ┌───────────┐  ┌───────────┐
            │  Plugin   │  │  Renderer │  │   Event   │
            │   API     │  │ (diags)   │  │    Bus    │
            └───────────┘  └───────────┘  └───────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| go.lsp.dev/protocol | Official Go LSP types, well-maintained, auto-generated from spec |
| go.lsp.dev/jsonrpc2 | LSP-compatible JSON-RPC with content-length headers |
| Multi-server support | Different languages need different servers (gopls, rust-analyzer, etc.) |
| Lazy server start | Only start servers when files of that language are opened |
| Document versioning | Track versions for incremental sync and race condition prevention |
| Async operations | Non-blocking LSP calls with callbacks/channels for results |
| Graceful degradation | Editor works normally when LSP unavailable |

### Integration Points

The LSP layer connects to:
- **Dispatcher**: Receives LSP-related actions (completion.trigger, lsp.definition)
- **Plugin API**: Implements `api.LSPProvider` interface for plugin access
- **Engine**: Listens for buffer changes to sync documents
- **Renderer**: Provides diagnostics for display (squiggles, error markers)
- **Event Bus**: Publishes LSP events (diagnostics, server status)
- **Config System**: Reads server configurations per language
- **Project**: Workspace folders, file watchers

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "Modern editors offload intelligence to language servers:
> - autocompletion
> - go-to-definition
> - diagnostics
> - formatting
> - refactoring
> - semantic highlighting"

> "AI orchestration uses LSP as a structured source of truth"

### 2.2 Functional Requirements

1. **Server Lifecycle**
   - Auto-start servers when relevant files opened
   - Graceful shutdown on exit
   - Restart on crash with backoff
   - Support multiple concurrent servers

2. **Document Synchronization**
   - Open/close document notifications
   - Incremental text change sync
   - Version tracking for all documents

3. **Code Intelligence**
   - Completions with filtering and sorting
   - Hover information
   - Go-to-definition/declaration/type-definition
   - Find references
   - Document/workspace symbols

4. **Code Quality**
   - Real-time diagnostics (errors, warnings, hints)
   - Code actions (quick fixes, refactors)
   - Document/range formatting

5. **Refactoring**
   - Rename symbol (across files)
   - Workspace edits application

### 2.3 Non-Functional Requirements

1. **Performance**
   - Sub-100ms response for completions
   - Background diagnostic updates (no UI blocking)
   - Efficient incremental sync

2. **Reliability**
   - Server crash recovery
   - Timeout handling
   - Graceful degradation

3. **Extensibility**
   - Easy server configuration
   - Plugin access to LSP features
   - Custom server support

### 2.4 Existing Interfaces

From `internal/plugin/api/lsp.go`, the `LSPProvider` interface:

```go
type LSPProvider interface {
    Completions(bufferPath string, offset int) ([]CompletionItem, error)
    Diagnostics(bufferPath string) ([]Diagnostic, error)
    Definition(bufferPath string, offset int) (*Location, error)
    References(bufferPath string, offset int, includeDeclaration bool) ([]Location, error)
    Hover(bufferPath string, offset int) (*HoverInfo, error)
    SignatureHelp(bufferPath string, offset int) (*SignatureInfo, error)
    Format(bufferPath string, startOffset, endOffset int) ([]TextEdit, error)
    CodeActions(bufferPath string, startOffset, endOffset int, diagnostics []Diagnostic) ([]CodeAction, error)
    Rename(bufferPath string, offset int, newName string) ([]TextEdit, error)
    IsAvailable(bufferPath string) bool
}
```

---

## 3. Architecture Overview

### 3.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        LSP Client                                │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                      Manager                                 │ │
│  │  - Server lifecycle                                          │ │
│  │  - Language → Server mapping                                 │ │
│  │  - Configuration                                             │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                              │                                   │
│           ┌──────────────────┼──────────────────┐               │
│           ▼                  ▼                  ▼               │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │   Server    │    │   Server    │    │   Server    │         │
│  │   (gopls)   │    │ (rust-ana)  │    │  (tsserver) │         │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘         │
│         │                  │                  │                 │
│  ┌──────┴──────────────────┴──────────────────┴──────┐         │
│  │                  Transport Layer                   │         │
│  │  - JSON-RPC 2.0 over stdio                        │         │
│  │  - Request/Response correlation                    │         │
│  │  - Notification handling                           │         │
│  └────────────────────────────────────────────────────┘         │
│                              │                                   │
│  ┌───────────────────────────┴───────────────────────┐         │
│  │                  Document Manager                  │         │
│  │  - Open document tracking                          │         │
│  │  - Version management                              │         │
│  │  - Change buffering                                │         │
│  └────────────────────────────────────────────────────┘         │
│                              │                                   │
│  ┌───────────────────────────┴───────────────────────┐         │
│  │                 Diagnostics Store                  │         │
│  │  - Per-file diagnostic cache                       │         │
│  │  - Change notifications                            │         │
│  └────────────────────────────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 Data Flow

**Completion Request Flow:**
```
1. User types → Engine.Insert()
2. Engine emits buffer.change event
3. Dispatcher receives completion.trigger action
4. LSP Client receives request
5. Client finds server for file's language
6. Client sends textDocument/completion via JSON-RPC
7. Server responds with CompletionList
8. Client transforms to internal CompletionItem[]
9. Dispatcher receives items, updates completion state
10. Renderer shows completion popup
```

**Diagnostic Flow:**
```
1. User edits file → Engine.Insert()
2. LSP Client sends textDocument/didChange
3. Server analyzes and sends publishDiagnostics notification
4. Client receives notification via JSON-RPC handler
5. Client transforms to internal Diagnostic[]
6. Client stores in DiagnosticsStore
7. Client emits diagnostics.updated event
8. Renderer updates error markers
```

---

## 4. Package Structure

```
internal/lsp/
├── doc.go                  # Package documentation
├── client.go               # Main LSP client interface
├── manager.go              # Server lifecycle management
├── server.go               # Single server connection
├── transport.go            # JSON-RPC transport layer
├── document.go             # Document synchronization
├── diagnostics.go          # Diagnostics management
├── position.go             # Position/range conversion utilities
├── errors.go               # Error types
│
├── protocol/               # LSP protocol types (or use go.lsp.dev/protocol)
│   ├── types.go            # Core LSP types
│   ├── methods.go          # Method constants
│   └── capabilities.go     # Capability definitions
│
├── handlers/               # Feature-specific handlers
│   ├── completion.go       # Completion handling
│   ├── hover.go            # Hover information
│   ├── definition.go       # Go-to-definition
│   ├── references.go       # Find references
│   ├── symbols.go          # Document/workspace symbols
│   ├── formatting.go       # Code formatting
│   ├── codeaction.go       # Code actions
│   ├── rename.go           # Rename refactoring
│   └── signature.go        # Signature help
│
├── config/                 # Server configuration
│   ├── config.go           # Configuration types
│   └── languages.go        # Language → server mapping
│
└── testing/                # Test utilities
    ├── mock_server.go      # Mock LSP server
    └── fixtures.go         # Test fixtures
```

---

## 5. Core Types and Interfaces

### 5.1 Client Interface

```go
// Client is the main LSP client interface exposed to the rest of Keystorm.
// It provides a high-level API for LSP operations without exposing
// JSON-RPC details or server management complexity.
type Client interface {
    // Lifecycle
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error

    // Document management
    OpenDocument(ctx context.Context, path string, languageID string, content string) error
    CloseDocument(ctx context.Context, path string) error
    ChangeDocument(ctx context.Context, path string, changes []TextChange, version int) error

    // Code intelligence
    Completion(ctx context.Context, path string, pos Position) (*CompletionResult, error)
    Hover(ctx context.Context, path string, pos Position) (*HoverResult, error)
    Definition(ctx context.Context, path string, pos Position) ([]Location, error)
    TypeDefinition(ctx context.Context, path string, pos Position) ([]Location, error)
    References(ctx context.Context, path string, pos Position, includeDecl bool) ([]Location, error)
    DocumentSymbols(ctx context.Context, path string) ([]DocumentSymbol, error)
    WorkspaceSymbols(ctx context.Context, query string) ([]WorkspaceSymbol, error)

    // Code quality
    Diagnostics(path string) []Diagnostic
    CodeActions(ctx context.Context, path string, rng Range, diags []Diagnostic) ([]CodeAction, error)
    Format(ctx context.Context, path string, options FormattingOptions) ([]TextEdit, error)
    FormatRange(ctx context.Context, path string, rng Range, options FormattingOptions) ([]TextEdit, error)

    // Refactoring
    Rename(ctx context.Context, path string, pos Position, newName string) (*WorkspaceEdit, error)
    PrepareRename(ctx context.Context, path string, pos Position) (*PrepareRenameResult, error)

    // Signature help
    SignatureHelp(ctx context.Context, path string, pos Position) (*SignatureHelp, error)

    // Status
    IsAvailable(path string) bool
    ServerStatus(languageID string) ServerStatus

    // Events
    OnDiagnostics(handler func(path string, diagnostics []Diagnostic))
    OnServerStatus(handler func(languageID string, status ServerStatus))
}
```

### 5.2 Manager Types

```go
// Manager manages multiple language servers.
type Manager struct {
    mu          sync.RWMutex
    servers     map[string]*Server  // languageID -> server
    config      *Config
    workspaces  []string            // workspace folders

    // Event handlers
    diagHandler   func(path string, diagnostics []Diagnostic)
    statusHandler func(languageID string, status ServerStatus)
}

// ServerConfig defines how to start a language server.
type ServerConfig struct {
    // Command is the executable to run.
    Command string
    // Args are command-line arguments.
    Args []string
    // Environment variables (merged with current env).
    Env map[string]string
    // Working directory (defaults to workspace root).
    WorkDir string
    // InitializationOptions sent during initialize.
    InitializationOptions any
    // Settings sent via workspace/didChangeConfiguration.
    Settings any
    // FilePatterns that this server handles.
    FilePatterns []string
    // LanguageIDs that this server handles.
    LanguageIDs []string
}

// Config holds all LSP configuration.
type Config struct {
    // Servers maps language IDs to server configurations.
    Servers map[string]ServerConfig
    // DefaultTimeout for LSP requests.
    DefaultTimeout time.Duration
    // MaxRestarts before giving up on a server.
    MaxRestarts int
    // RestartDelay between restart attempts.
    RestartDelay time.Duration
}
```

### 5.3 Server Types

```go
// Server represents a connection to a single language server.
type Server struct {
    mu            sync.Mutex
    config        ServerConfig
    languageID    string

    // Process management
    cmd           *exec.Cmd
    stdin         io.WriteCloser
    stdout        io.ReadCloser

    // JSON-RPC connection
    conn          jsonrpc2.Conn

    // State
    status        ServerStatus
    capabilities  ServerCapabilities

    // Document tracking
    documents     map[string]*Document

    // Diagnostics
    diagnostics   map[string][]Diagnostic
    diagHandler   func(path string, diagnostics []Diagnostic)

    // Request tracking
    pendingReqs   map[jsonrpc2.ID]chan any
    nextID        int64
}

// ServerStatus indicates the current state of a server.
type ServerStatus int

const (
    ServerStatusStopped ServerStatus = iota
    ServerStatusStarting
    ServerStatusInitializing
    ServerStatusReady
    ServerStatusShuttingDown
    ServerStatusError
)

// ServerCapabilities tracks what features a server supports.
type ServerCapabilities struct {
    CompletionProvider      *CompletionOptions
    HoverProvider           bool
    DefinitionProvider      bool
    TypeDefinitionProvider  bool
    ReferencesProvider      bool
    DocumentSymbolProvider  bool
    WorkspaceSymbolProvider bool
    CodeActionProvider      *CodeActionOptions
    DocumentFormattingProvider bool
    RenameProvider          *RenameOptions
    SignatureHelpProvider   *SignatureHelpOptions
    TextDocumentSync        TextDocumentSyncOptions
}
```

### 5.4 Document Types

```go
// Document represents an open document tracked by the LSP client.
type Document struct {
    mu         sync.RWMutex
    Path       string
    URI        DocumentURI
    LanguageID string
    Version    int
    Content    string

    // Sync state
    dirty      bool
    lastSync   time.Time
}

// TextChange represents a change to a document.
type TextChange struct {
    Range   *Range  // nil means full document replacement
    Text    string
}

// DocumentURI is a file:// URI.
type DocumentURI string
```

### 5.5 Position Types

```go
// Position in a document (0-indexed line and character).
type Position struct {
    Line      int
    Character int  // UTF-16 code units, per LSP spec
}

// Range in a document.
type Range struct {
    Start Position
    End   Position
}

// Location is a position in a specific document.
type Location struct {
    URI   DocumentURI
    Range Range
}
```

### 5.6 Result Types

```go
// CompletionResult holds completion response data.
type CompletionResult struct {
    IsIncomplete bool
    Items        []CompletionItem
}

// CompletionItem represents a single completion suggestion.
type CompletionItem struct {
    Label               string
    Kind                CompletionItemKind
    Detail              string
    Documentation       string
    Deprecated          bool
    Preselect           bool
    SortText            string
    FilterText          string
    InsertText          string
    InsertTextFormat    InsertTextFormat
    TextEdit            *TextEdit
    AdditionalTextEdits []TextEdit
    CommitCharacters    []string
    Command             *Command
    Data                any
}

// HoverResult holds hover response data.
type HoverResult struct {
    Contents MarkupContent
    Range    *Range
}

// Diagnostic represents an error, warning, or hint.
type Diagnostic struct {
    Range              Range
    Severity           DiagnosticSeverity
    Code               any // string or number
    CodeDescription    *CodeDescription
    Source             string
    Message            string
    Tags               []DiagnosticTag
    RelatedInformation []DiagnosticRelatedInformation
    Data               any
}

// TextEdit represents a change to apply to a document.
type TextEdit struct {
    Range   Range
    NewText string
}

// WorkspaceEdit represents changes across multiple files.
type WorkspaceEdit struct {
    Changes         map[DocumentURI][]TextEdit
    DocumentChanges []TextDocumentEdit
}
```

---

## 6. Server Management

### 6.1 Server Lifecycle

```
┌──────────┐  start()   ┌───────────┐  initialize()  ┌──────────────┐
│ Stopped  │───────────▶│ Starting  │───────────────▶│ Initializing │
└──────────┘            └───────────┘                └──────┬───────┘
     ▲                                                      │
     │                                              initialized
     │                                                      │
     │                                                      ▼
     │  exit()          ┌──────────────┐   shutdown()  ┌───────┐
     └──────────────────│ ShuttingDown │◀──────────────│ Ready │
                        └──────────────┘               └───────┘
```

### 6.2 Server Start Sequence

```go
// startServer starts a language server for the given language.
func (m *Manager) startServer(ctx context.Context, languageID string) (*Server, error) {
    config, ok := m.config.Servers[languageID]
    if !ok {
        return nil, fmt.Errorf("no server configured for language: %s", languageID)
    }

    server := &Server{
        config:      config,
        languageID:  languageID,
        status:      ServerStatusStarting,
        documents:   make(map[string]*Document),
        diagnostics: make(map[string][]Diagnostic),
        pendingReqs: make(map[jsonrpc2.ID]chan any),
    }

    // Start process
    cmd := exec.CommandContext(ctx, config.Command, config.Args...)
    cmd.Env = append(os.Environ(), mapToEnv(config.Env)...)
    if config.WorkDir != "" {
        cmd.Dir = config.WorkDir
    }

    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, fmt.Errorf("stdin pipe: %w", err)
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("stdout pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("start server: %w", err)
    }

    server.cmd = cmd
    server.stdin = stdin
    server.stdout = stdout

    // Create JSON-RPC connection
    stream := jsonrpc2.NewStream(NewReadWriteCloser(stdout, stdin))
    server.conn = jsonrpc2.NewConn(stream)

    // Start handling notifications
    go server.handleNotifications(ctx)

    // Initialize the server
    server.status = ServerStatusInitializing
    if err := server.initialize(ctx, m.workspaces); err != nil {
        server.shutdown(ctx)
        return nil, fmt.Errorf("initialize: %w", err)
    }

    server.status = ServerStatusReady
    return server, nil
}
```

### 6.3 Initialization

```go
// initialize performs the LSP initialize handshake.
func (s *Server) initialize(ctx context.Context, workspaces []string) error {
    // Build workspace folders
    folders := make([]WorkspaceFolder, len(workspaces))
    for i, ws := range workspaces {
        folders[i] = WorkspaceFolder{
            URI:  filePathToURI(ws),
            Name: filepath.Base(ws),
        }
    }

    params := InitializeParams{
        ProcessID: os.Getpid(),
        RootURI:   folders[0].URI,
        Capabilities: ClientCapabilities{
            Workspace: &WorkspaceClientCapabilities{
                WorkspaceFolders: true,
                ApplyEdit:        true,
                Symbol:           &WorkspaceSymbolClientCapabilities{},
                Configuration:    true,
            },
            TextDocument: &TextDocumentClientCapabilities{
                Synchronization: &TextDocumentSyncClientCapabilities{
                    DidSave:           true,
                    WillSave:          true,
                    WillSaveWaitUntil: true,
                },
                Completion: &CompletionClientCapabilities{
                    CompletionItem: &CompletionItemCapabilities{
                        SnippetSupport:         true,
                        DocumentationFormat:    []MarkupKind{MarkupKindMarkdown, MarkupKindPlainText},
                        DeprecatedSupport:      true,
                        PreselectSupport:       true,
                        InsertReplaceSupport:   true,
                        ResolveSupport:         &CompletionItemResolveSupport{},
                    },
                    CompletionItemKind: &CompletionItemKindCapabilities{},
                    ContextSupport:     true,
                },
                Hover: &HoverClientCapabilities{
                    ContentFormat: []MarkupKind{MarkupKindMarkdown, MarkupKindPlainText},
                },
                SignatureHelp:    &SignatureHelpClientCapabilities{},
                Definition:       &DefinitionClientCapabilities{},
                References:       &ReferenceClientCapabilities{},
                DocumentHighlight: &DocumentHighlightClientCapabilities{},
                DocumentSymbol:   &DocumentSymbolClientCapabilities{},
                CodeAction:       &CodeActionClientCapabilities{},
                Formatting:       &DocumentFormattingClientCapabilities{},
                RangeFormatting:  &DocumentRangeFormattingClientCapabilities{},
                Rename:           &RenameClientCapabilities{PrepareSupport: true},
                PublishDiagnostics: &PublishDiagnosticsClientCapabilities{
                    RelatedInformation: true,
                    TagSupport:         &DiagnosticTagSupport{ValueSet: []DiagnosticTag{DiagnosticTagUnnecessary, DiagnosticTagDeprecated}},
                    VersionSupport:     true,
                    CodeDescriptionSupport: true,
                    DataSupport:        true,
                },
            },
        },
        WorkspaceFolders: folders,
        InitializationOptions: s.config.InitializationOptions,
    }

    var result InitializeResult
    if err := s.call(ctx, "initialize", params, &result); err != nil {
        return err
    }

    // Store server capabilities
    s.capabilities = convertCapabilities(result.Capabilities)

    // Send initialized notification
    return s.notify(ctx, "initialized", InitializedParams{})
}
```

### 6.4 Crash Recovery

```go
// monitorServer watches a server and restarts on crash.
func (m *Manager) monitorServer(ctx context.Context, s *Server) {
    restarts := 0

    for {
        select {
        case <-ctx.Done():
            return
        case err := <-s.waitExit():
            if err == nil {
                // Clean shutdown
                return
            }

            // Server crashed
            s.status = ServerStatusError
            m.notifyStatus(s.languageID, ServerStatusError)

            restarts++
            if restarts > m.config.MaxRestarts {
                log.Printf("LSP server %s crashed too many times, giving up", s.languageID)
                return
            }

            // Exponential backoff
            delay := m.config.RestartDelay * time.Duration(1<<(restarts-1))
            log.Printf("LSP server %s crashed, restarting in %v", s.languageID, delay)

            select {
            case <-ctx.Done():
                return
            case <-time.After(delay):
            }

            // Restart
            newServer, err := m.startServer(ctx, s.languageID)
            if err != nil {
                log.Printf("Failed to restart LSP server %s: %v", s.languageID, err)
                continue
            }

            // Re-open all documents
            for _, doc := range s.documents {
                newServer.openDocument(ctx, doc.Path, doc.LanguageID, doc.Content)
            }

            m.mu.Lock()
            m.servers[s.languageID] = newServer
            m.mu.Unlock()

            s = newServer
            m.notifyStatus(s.languageID, ServerStatusReady)
        }
    }
}
```

---

## 7. Protocol Implementation

### 7.1 JSON-RPC Transport

```go
// Transport handles JSON-RPC 2.0 communication.
type Transport struct {
    conn   io.ReadWriteCloser
    codec  *json.Encoder
    reader *bufio.Reader

    mu       sync.Mutex
    nextID   int64
    pending  map[int64]chan *Response
    handlers map[string]NotificationHandler
}

// NewTransport creates a transport over the given connection.
func NewTransport(conn io.ReadWriteCloser) *Transport {
    return &Transport{
        conn:     conn,
        codec:    json.NewEncoder(conn),
        reader:   bufio.NewReader(conn),
        pending:  make(map[int64]chan *Response),
        handlers: make(map[string]NotificationHandler),
    }
}

// Call sends a request and waits for a response.
func (t *Transport) Call(ctx context.Context, method string, params any, result any) error {
    t.mu.Lock()
    id := t.nextID
    t.nextID++
    ch := make(chan *Response, 1)
    t.pending[id] = ch
    t.mu.Unlock()

    defer func() {
        t.mu.Lock()
        delete(t.pending, id)
        t.mu.Unlock()
    }()

    // Send request
    req := &Request{
        JSONRPC: "2.0",
        ID:      id,
        Method:  method,
        Params:  params,
    }

    if err := t.send(req); err != nil {
        return fmt.Errorf("send request: %w", err)
    }

    // Wait for response
    select {
    case <-ctx.Done():
        return ctx.Err()
    case resp := <-ch:
        if resp.Error != nil {
            return &RPCError{
                Code:    resp.Error.Code,
                Message: resp.Error.Message,
                Data:    resp.Error.Data,
            }
        }
        if result != nil && resp.Result != nil {
            return json.Unmarshal(resp.Result, result)
        }
        return nil
    }
}

// Notify sends a notification (no response expected).
func (t *Transport) Notify(ctx context.Context, method string, params any) error {
    notif := &Request{
        JSONRPC: "2.0",
        Method:  method,
        Params:  params,
    }
    return t.send(notif)
}

// send writes a message with LSP content-length header.
func (t *Transport) send(msg any) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

    t.mu.Lock()
    defer t.mu.Unlock()

    if _, err := io.WriteString(t.conn, header); err != nil {
        return err
    }
    _, err = t.conn.Write(data)
    return err
}

// readLoop reads messages from the connection.
func (t *Transport) readLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        msg, err := t.readMessage()
        if err != nil {
            if err == io.EOF {
                return
            }
            log.Printf("LSP read error: %v", err)
            continue
        }

        t.dispatch(msg)
    }
}

// readMessage reads a single LSP message.
func (t *Transport) readMessage() (json.RawMessage, error) {
    // Read headers
    var contentLength int
    for {
        line, err := t.reader.ReadString('\n')
        if err != nil {
            return nil, err
        }
        line = strings.TrimSpace(line)
        if line == "" {
            break // End of headers
        }
        if strings.HasPrefix(line, "Content-Length:") {
            fmt.Sscanf(line, "Content-Length: %d", &contentLength)
        }
    }

    if contentLength == 0 {
        return nil, fmt.Errorf("missing Content-Length header")
    }

    // Read body
    body := make([]byte, contentLength)
    if _, err := io.ReadFull(t.reader, body); err != nil {
        return nil, err
    }

    return body, nil
}
```

### 7.2 Request/Response Correlation

```go
// dispatch routes a message to the appropriate handler.
func (t *Transport) dispatch(data json.RawMessage) {
    // Try to parse as response first
    var resp Response
    if err := json.Unmarshal(data, &resp); err == nil && resp.ID != 0 {
        t.mu.Lock()
        ch, ok := t.pending[resp.ID]
        t.mu.Unlock()

        if ok {
            ch <- &resp
        }
        return
    }

    // Try to parse as request/notification
    var req Request
    if err := json.Unmarshal(data, &req); err == nil {
        if handler, ok := t.handlers[req.Method]; ok {
            go handler(req.Params)
        }
    }
}

// OnNotification registers a handler for a notification method.
func (t *Transport) OnNotification(method string, handler NotificationHandler) {
    t.mu.Lock()
    t.handlers[method] = handler
    t.mu.Unlock()
}
```

---

## 8. Document Synchronization

### 8.1 Document Manager

```go
// DocumentManager tracks open documents and their versions.
type DocumentManager struct {
    mu        sync.RWMutex
    documents map[DocumentURI]*Document
    server    *Server
}

// Open notifies the server that a document was opened.
func (dm *DocumentManager) Open(ctx context.Context, path, languageID, content string) error {
    uri := filePathToURI(path)

    dm.mu.Lock()
    doc := &Document{
        Path:       path,
        URI:        uri,
        LanguageID: languageID,
        Version:    1,
        Content:    content,
    }
    dm.documents[uri] = doc
    dm.mu.Unlock()

    return dm.server.notify(ctx, "textDocument/didOpen", DidOpenTextDocumentParams{
        TextDocument: TextDocumentItem{
            URI:        uri,
            LanguageID: languageID,
            Version:    1,
            Text:       content,
        },
    })
}

// Change notifies the server of document changes.
func (dm *DocumentManager) Change(ctx context.Context, path string, changes []TextChange) error {
    uri := filePathToURI(path)

    dm.mu.Lock()
    doc, ok := dm.documents[uri]
    if !ok {
        dm.mu.Unlock()
        return fmt.Errorf("document not open: %s", path)
    }

    doc.Version++
    version := doc.Version

    // Apply changes to cached content
    for _, change := range changes {
        if change.Range == nil {
            doc.Content = change.Text
        } else {
            doc.Content = applyChange(doc.Content, *change.Range, change.Text)
        }
    }
    dm.mu.Unlock()

    // Convert to LSP format
    contentChanges := make([]TextDocumentContentChangeEvent, len(changes))
    for i, change := range changes {
        contentChanges[i] = TextDocumentContentChangeEvent{
            Range: change.Range,
            Text:  change.Text,
        }
    }

    return dm.server.notify(ctx, "textDocument/didChange", DidChangeTextDocumentParams{
        TextDocument: VersionedTextDocumentIdentifier{
            TextDocumentIdentifier: TextDocumentIdentifier{URI: uri},
            Version:                version,
        },
        ContentChanges: contentChanges,
    })
}

// Close notifies the server that a document was closed.
func (dm *DocumentManager) Close(ctx context.Context, path string) error {
    uri := filePathToURI(path)

    dm.mu.Lock()
    delete(dm.documents, uri)
    dm.mu.Unlock()

    return dm.server.notify(ctx, "textDocument/didClose", DidCloseTextDocumentParams{
        TextDocument: TextDocumentIdentifier{URI: uri},
    })
}
```

### 8.2 Incremental Sync

```go
// TextDocumentSyncKind defines how document changes are synced.
type TextDocumentSyncKind int

const (
    TextDocumentSyncKindNone        TextDocumentSyncKind = 0
    TextDocumentSyncKindFull        TextDocumentSyncKind = 1
    TextDocumentSyncKindIncremental TextDocumentSyncKind = 2
)

// SyncDocument sends document changes based on server capabilities.
func (dm *DocumentManager) SyncDocument(ctx context.Context, path string, fullContent string, changes []TextChange) error {
    syncKind := dm.server.capabilities.TextDocumentSync.Change

    switch syncKind {
    case TextDocumentSyncKindNone:
        return nil // Server doesn't want sync

    case TextDocumentSyncKindFull:
        // Send full document
        return dm.Change(ctx, path, []TextChange{{Text: fullContent}})

    case TextDocumentSyncKindIncremental:
        // Send incremental changes
        return dm.Change(ctx, path, changes)

    default:
        // Default to full
        return dm.Change(ctx, path, []TextChange{{Text: fullContent}})
    }
}
```

---

## 9. Feature Handlers

### 9.1 Completion Handler

```go
// Completion requests completion items at a position.
func (s *Server) Completion(ctx context.Context, path string, pos Position) (*CompletionResult, error) {
    if s.capabilities.CompletionProvider == nil {
        return nil, ErrNotSupported
    }

    uri := filePathToURI(path)

    params := CompletionParams{
        TextDocumentPositionParams: TextDocumentPositionParams{
            TextDocument: TextDocumentIdentifier{URI: uri},
            Position:     pos,
        },
        Context: &CompletionContext{
            TriggerKind: CompletionTriggerKindInvoked,
        },
    }

    var result CompletionResult
    if err := s.call(ctx, "textDocument/completion", params, &result); err != nil {
        return nil, err
    }

    return &result, nil
}

// ResolveCompletion gets additional details for a completion item.
func (s *Server) ResolveCompletion(ctx context.Context, item CompletionItem) (*CompletionItem, error) {
    if s.capabilities.CompletionProvider == nil || !s.capabilities.CompletionProvider.ResolveProvider {
        return &item, nil
    }

    var result CompletionItem
    if err := s.call(ctx, "completionItem/resolve", item, &result); err != nil {
        return nil, err
    }

    return &result, nil
}
```

### 9.2 Navigation Handlers

```go
// Definition returns the definition location(s) for a symbol.
func (s *Server) Definition(ctx context.Context, path string, pos Position) ([]Location, error) {
    if !s.capabilities.DefinitionProvider {
        return nil, ErrNotSupported
    }

    params := TextDocumentPositionParams{
        TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
        Position:     pos,
    }

    var result any
    if err := s.call(ctx, "textDocument/definition", params, &result); err != nil {
        return nil, err
    }

    return parseLocationResult(result)
}

// References finds all references to a symbol.
func (s *Server) References(ctx context.Context, path string, pos Position, includeDecl bool) ([]Location, error) {
    if !s.capabilities.ReferencesProvider {
        return nil, ErrNotSupported
    }

    params := ReferenceParams{
        TextDocumentPositionParams: TextDocumentPositionParams{
            TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
            Position:     pos,
        },
        Context: ReferenceContext{
            IncludeDeclaration: includeDecl,
        },
    }

    var result []Location
    if err := s.call(ctx, "textDocument/references", params, &result); err != nil {
        return nil, err
    }

    return result, nil
}

// Hover returns hover information for a position.
func (s *Server) Hover(ctx context.Context, path string, pos Position) (*HoverResult, error) {
    if !s.capabilities.HoverProvider {
        return nil, ErrNotSupported
    }

    params := TextDocumentPositionParams{
        TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
        Position:     pos,
    }

    var result HoverResult
    if err := s.call(ctx, "textDocument/hover", params, &result); err != nil {
        return nil, err
    }

    return &result, nil
}
```

### 9.3 Code Actions Handler

```go
// CodeActions returns available code actions for a range.
func (s *Server) CodeActions(ctx context.Context, path string, rng Range, diags []Diagnostic) ([]CodeAction, error) {
    if s.capabilities.CodeActionProvider == nil {
        return nil, ErrNotSupported
    }

    params := CodeActionParams{
        TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
        Range:        rng,
        Context: CodeActionContext{
            Diagnostics: diags,
        },
    }

    var result []CodeActionOrCommand
    if err := s.call(ctx, "textDocument/codeAction", params, &result); err != nil {
        return nil, err
    }

    // Convert to CodeAction slice
    actions := make([]CodeAction, 0, len(result))
    for _, item := range result {
        if item.CodeAction != nil {
            actions = append(actions, *item.CodeAction)
        }
        // TODO: Handle Command items
    }

    return actions, nil
}

// ResolveCodeAction gets the edit for a code action.
func (s *Server) ResolveCodeAction(ctx context.Context, action CodeAction) (*CodeAction, error) {
    if s.capabilities.CodeActionProvider == nil || !s.capabilities.CodeActionProvider.ResolveProvider {
        return &action, nil
    }

    var result CodeAction
    if err := s.call(ctx, "codeAction/resolve", action, &result); err != nil {
        return nil, err
    }

    return &result, nil
}
```

### 9.4 Formatting Handler

```go
// Format formats an entire document.
func (s *Server) Format(ctx context.Context, path string, opts FormattingOptions) ([]TextEdit, error) {
    if !s.capabilities.DocumentFormattingProvider {
        return nil, ErrNotSupported
    }

    params := DocumentFormattingParams{
        TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
        Options:      opts,
    }

    var result []TextEdit
    if err := s.call(ctx, "textDocument/formatting", params, &result); err != nil {
        return nil, err
    }

    return result, nil
}

// FormatRange formats a range within a document.
func (s *Server) FormatRange(ctx context.Context, path string, rng Range, opts FormattingOptions) ([]TextEdit, error) {
    if !s.capabilities.DocumentRangeFormattingProvider {
        return nil, ErrNotSupported
    }

    params := DocumentRangeFormattingParams{
        TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
        Range:        rng,
        Options:      opts,
    }

    var result []TextEdit
    if err := s.call(ctx, "textDocument/rangeFormatting", params, &result); err != nil {
        return nil, err
    }

    return result, nil
}
```

### 9.5 Rename Handler

```go
// PrepareRename checks if rename is valid at a position.
func (s *Server) PrepareRename(ctx context.Context, path string, pos Position) (*PrepareRenameResult, error) {
    if s.capabilities.RenameProvider == nil || !s.capabilities.RenameProvider.PrepareProvider {
        return nil, nil // No prepare support, proceed with rename
    }

    params := TextDocumentPositionParams{
        TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
        Position:     pos,
    }

    var result PrepareRenameResult
    if err := s.call(ctx, "textDocument/prepareRename", params, &result); err != nil {
        return nil, err
    }

    return &result, nil
}

// Rename renames a symbol across the workspace.
func (s *Server) Rename(ctx context.Context, path string, pos Position, newName string) (*WorkspaceEdit, error) {
    if s.capabilities.RenameProvider == nil {
        return nil, ErrNotSupported
    }

    params := RenameParams{
        TextDocumentPositionParams: TextDocumentPositionParams{
            TextDocument: TextDocumentIdentifier{URI: filePathToURI(path)},
            Position:     pos,
        },
        NewName: newName,
    }

    var result WorkspaceEdit
    if err := s.call(ctx, "textDocument/rename", params, &result); err != nil {
        return nil, err
    }

    return &result, nil
}
```

---

## 10. Diagnostics System

### 10.1 Diagnostics Store

```go
// DiagnosticsStore manages diagnostics for all open documents.
type DiagnosticsStore struct {
    mu          sync.RWMutex
    diagnostics map[DocumentURI][]Diagnostic
    handlers    []DiagnosticsHandler
}

// DiagnosticsHandler is called when diagnostics change.
type DiagnosticsHandler func(path string, diagnostics []Diagnostic)

// NewDiagnosticsStore creates a new diagnostics store.
func NewDiagnosticsStore() *DiagnosticsStore {
    return &DiagnosticsStore{
        diagnostics: make(map[DocumentURI][]Diagnostic),
    }
}

// Update replaces diagnostics for a document.
func (ds *DiagnosticsStore) Update(uri DocumentURI, diags []Diagnostic) {
    ds.mu.Lock()
    if len(diags) == 0 {
        delete(ds.diagnostics, uri)
    } else {
        ds.diagnostics[uri] = diags
    }
    handlers := ds.handlers
    ds.mu.Unlock()

    // Notify handlers
    path := uriToFilePath(uri)
    for _, handler := range handlers {
        handler(path, diags)
    }
}

// Get returns diagnostics for a document.
func (ds *DiagnosticsStore) Get(path string) []Diagnostic {
    uri := filePathToURI(path)

    ds.mu.RLock()
    defer ds.mu.RUnlock()

    return ds.diagnostics[uri]
}

// GetAll returns diagnostics for all documents.
func (ds *DiagnosticsStore) GetAll() map[string][]Diagnostic {
    ds.mu.RLock()
    defer ds.mu.RUnlock()

    result := make(map[string][]Diagnostic, len(ds.diagnostics))
    for uri, diags := range ds.diagnostics {
        result[uriToFilePath(uri)] = diags
    }
    return result
}

// OnChange registers a handler for diagnostic changes.
func (ds *DiagnosticsStore) OnChange(handler DiagnosticsHandler) {
    ds.mu.Lock()
    ds.handlers = append(ds.handlers, handler)
    ds.mu.Unlock()
}
```

### 10.2 Notification Handler

```go
// handlePublishDiagnostics processes publishDiagnostics notifications.
func (s *Server) handlePublishDiagnostics(params json.RawMessage) {
    var p PublishDiagnosticsParams
    if err := json.Unmarshal(params, &p); err != nil {
        log.Printf("LSP: failed to parse diagnostics: %v", err)
        return
    }

    s.mu.Lock()
    s.diagnostics[p.URI] = p.Diagnostics
    handler := s.diagHandler
    s.mu.Unlock()

    if handler != nil {
        path := uriToFilePath(p.URI)
        handler(path, p.Diagnostics)
    }
}
```

---

## 11. Integration with Editor

### 11.1 LSPProvider Implementation

```go
// Provider implements api.LSPProvider for the plugin system.
type Provider struct {
    client *Client
}

// NewProvider creates an LSPProvider backed by the LSP client.
func NewProvider(client *Client) *Provider {
    return &Provider{client: client}
}

// Completions implements api.LSPProvider.
func (p *Provider) Completions(bufferPath string, offset int) ([]api.CompletionItem, error) {
    pos, err := p.offsetToPosition(bufferPath, offset)
    if err != nil {
        return nil, err
    }

    result, err := p.client.Completion(context.Background(), bufferPath, pos)
    if err != nil {
        return nil, err
    }

    items := make([]api.CompletionItem, len(result.Items))
    for i, item := range result.Items {
        items[i] = api.CompletionItem{
            Label:         item.Label,
            Kind:          api.CompletionKind(item.Kind),
            Detail:        item.Detail,
            Documentation: extractDocumentation(item.Documentation),
            InsertText:    item.InsertText,
            SortText:      item.SortText,
        }
    }

    return items, nil
}

// Diagnostics implements api.LSPProvider.
func (p *Provider) Diagnostics(bufferPath string) ([]api.Diagnostic, error) {
    diags := p.client.Diagnostics(bufferPath)

    result := make([]api.Diagnostic, len(diags))
    for i, d := range diags {
        result[i] = api.Diagnostic{
            Range:    convertRange(d.Range),
            Severity: api.DiagnosticSeverity(d.Severity),
            Code:     fmt.Sprint(d.Code),
            Source:   d.Source,
            Message:  d.Message,
        }
    }

    return result, nil
}

// Definition implements api.LSPProvider.
func (p *Provider) Definition(bufferPath string, offset int) (*api.Location, error) {
    pos, err := p.offsetToPosition(bufferPath, offset)
    if err != nil {
        return nil, err
    }

    locs, err := p.client.Definition(context.Background(), bufferPath, pos)
    if err != nil {
        return nil, err
    }

    if len(locs) == 0 {
        return nil, nil
    }

    // Return first location
    loc := locs[0]
    return &api.Location{
        Path:  uriToFilePath(loc.URI),
        Range: convertRange(loc.Range),
    }, nil
}

// ... other api.LSPProvider methods
```

### 11.2 Engine Integration

```go
// BufferWatcher watches buffer changes and syncs to LSP.
type BufferWatcher struct {
    client     *Client
    engine     *engine.Engine
    unsubscribe func()
}

// NewBufferWatcher creates a watcher that syncs buffer changes to LSP.
func NewBufferWatcher(client *Client, engine *engine.Engine, eventBus *event.Bus) *BufferWatcher {
    w := &BufferWatcher{
        client: client,
        engine: engine,
    }

    // Subscribe to buffer events
    w.unsubscribe = eventBus.Subscribe(event.BufferChange, w.handleChange)

    return w
}

// handleChange processes buffer change events.
func (w *BufferWatcher) handleChange(e event.Event) {
    change, ok := e.Data.(*event.BufferChangeData)
    if !ok {
        return
    }

    path := change.Path

    // Check if document is open in LSP
    if !w.client.IsDocumentOpen(path) {
        // Open the document
        languageID := detectLanguage(path)
        content := w.engine.Text()
        w.client.OpenDocument(context.Background(), path, languageID, content)
        return
    }

    // Send incremental change
    lspChanges := []TextChange{{
        Range: &Range{
            Start: Position{Line: change.StartLine, Character: change.StartCol},
            End:   Position{Line: change.EndLine, Character: change.EndCol},
        },
        Text: change.Text,
    }}

    w.client.ChangeDocument(context.Background(), path, lspChanges, change.Version)
}

// Stop stops watching buffer changes.
func (w *BufferWatcher) Stop() {
    if w.unsubscribe != nil {
        w.unsubscribe()
    }
}
```

### 11.3 Dispatcher Integration

```go
// RegisterLSPHandlers registers LSP-related action handlers.
func RegisterLSPHandlers(d *dispatcher.Dispatcher, client *Client) {
    // Go to definition
    d.Register("lsp.definition", func(ctx *execctx.ExecutionContext) handler.Result {
        path := ctx.Engine.Path()
        offset := int(ctx.Cursors.Primary().Head)

        pos, _ := offsetToPosition(ctx.Engine, offset)
        locs, err := client.Definition(ctx.Context, path, pos)
        if err != nil {
            return handler.Error(err)
        }

        if len(locs) == 0 {
            return handler.NoOpWithMessage("No definition found")
        }

        loc := locs[0]
        return handler.Success().
            WithData("location", loc).
            WithNavigate(uriToFilePath(loc.URI), loc.Range.Start.Line, loc.Range.Start.Character)
    })

    // Format document
    d.Register("lsp.format", func(ctx *execctx.ExecutionContext) handler.Result {
        path := ctx.Engine.Path()

        edits, err := client.Format(ctx.Context, path, defaultFormattingOptions())
        if err != nil {
            return handler.Error(err)
        }

        // Apply edits in reverse order to maintain positions
        sortEditsReverse(edits)
        for _, edit := range edits {
            startOffset := positionToOffset(ctx.Engine, edit.Range.Start)
            endOffset := positionToOffset(ctx.Engine, edit.Range.End)
            ctx.Engine.Replace(startOffset, endOffset, edit.NewText)
        }

        return handler.Success().WithRedraw()
    })

    // ... other LSP action handlers
}
```

---

## 12. Implementation Phases

### Phase 1: Core Infrastructure (Foundation)

**Goals:** Basic server management and JSON-RPC transport.

**Tasks:**
1. Create package structure and documentation
2. Implement JSON-RPC transport (`transport.go`)
   - Content-length header parsing
   - Request/response correlation
   - Notification handling
3. Implement server lifecycle (`server.go`)
   - Process spawning
   - Initialize/shutdown handshake
   - Basic error handling
4. Implement manager (`manager.go`)
   - Server registry
   - Language → server mapping
   - Basic configuration

**Deliverables:**
- `doc.go`, `errors.go`
- `transport.go`, `transport_test.go`
- `server.go`, `server_test.go`
- `manager.go`, `manager_test.go`

**Test Criteria:**
- Can start/stop a mock server
- Can send initialize and receive capabilities
- Proper cleanup on shutdown

### Phase 2: Document Synchronization

**Goals:** Track open documents and sync changes to servers.

**Tasks:**
1. Implement document management (`document.go`)
   - Open/close tracking
   - Version management
   - Content caching
2. Implement change synchronization
   - Full sync mode
   - Incremental sync mode
   - Debouncing for rapid changes
3. Implement position utilities (`position.go`)
   - Byte offset ↔ line/column conversion
   - UTF-16 handling per LSP spec
   - Range operations

**Deliverables:**
- `document.go`, `document_test.go`
- `position.go`, `position_test.go`

**Test Criteria:**
- Documents tracked correctly
- Changes synced with proper versions
- Position conversions accurate

### Phase 3: Code Intelligence Features

**Goals:** Implement core code intelligence features.

**Tasks:**
1. Implement completion handler (`handlers/completion.go`)
   - Basic completions
   - Trigger characters
   - Item resolution
2. Implement hover handler (`handlers/hover.go`)
3. Implement definition handler (`handlers/definition.go`)
   - Go-to-definition
   - Go-to-type-definition
4. Implement references handler (`handlers/references.go`)
5. Implement symbols handler (`handlers/symbols.go`)
   - Document symbols
   - Workspace symbols

**Deliverables:**
- `handlers/completion.go`, `handlers/completion_test.go`
- `handlers/hover.go`, `handlers/hover_test.go`
- `handlers/definition.go`, `handlers/definition_test.go`
- `handlers/references.go`, `handlers/references_test.go`
- `handlers/symbols.go`, `handlers/symbols_test.go`

**Test Criteria:**
- Completions returned and formatted
- Hover shows documentation
- Definition navigation works
- References found across files

### Phase 4: Diagnostics System

**Goals:** Real-time error/warning reporting.

**Tasks:**
1. Implement diagnostics store (`diagnostics.go`)
   - Per-file storage
   - Change notifications
2. Handle publishDiagnostics notifications
3. Integrate with renderer for display
4. Implement diagnostic navigation (next/prev error)

**Deliverables:**
- `diagnostics.go`, `diagnostics_test.go`

**Test Criteria:**
- Diagnostics received and stored
- Updates propagated to UI
- Old diagnostics cleared

### Phase 5: Code Actions and Formatting

**Goals:** Quick fixes, refactorings, and formatting.

**Tasks:**
1. Implement code action handler (`handlers/codeaction.go`)
   - Code action request
   - Action resolution
   - Workspace edit application
2. Implement formatting handler (`handlers/formatting.go`)
   - Document formatting
   - Range formatting
   - Format on save

**Deliverables:**
- `handlers/codeaction.go`, `handlers/codeaction_test.go`
- `handlers/formatting.go`, `handlers/formatting_test.go`

**Test Criteria:**
- Quick fixes applied correctly
- Formatting changes document
- Multi-file edits work

### Phase 6: Rename and Signature Help

**Goals:** Refactoring and parameter hints.

**Tasks:**
1. Implement rename handler (`handlers/rename.go`)
   - Prepare rename
   - Execute rename
   - Multi-file edits
2. Implement signature help handler (`handlers/signature.go`)
   - Trigger on open paren
   - Parameter highlighting

**Deliverables:**
- `handlers/rename.go`, `handlers/rename_test.go`
- `handlers/signature.go`, `handlers/signature_test.go`

**Test Criteria:**
- Rename works across files
- Signature help shows parameters

### Phase 7: Client Interface and Provider

**Goals:** Unified client interface and plugin integration.

**Tasks:**
1. Implement client interface (`client.go`)
   - High-level API
   - Error handling
   - Timeout management
2. Implement LSPProvider (`provider.go`)
   - Implement api.LSPProvider interface
   - Position conversion
   - Type mapping

**Deliverables:**
- `client.go`, `client_test.go`
- `provider.go`, `provider_test.go`

**Test Criteria:**
- Client API works end-to-end
- Provider satisfies plugin interface

### Phase 8: Configuration and Server Management

**Goals:** Server configuration and reliability.

**Tasks:**
1. Implement configuration system (`config/config.go`)
   - Server definitions
   - Per-language settings
   - Default configurations
2. Implement crash recovery
   - Server monitoring
   - Automatic restart
   - Exponential backoff
3. Implement multi-server support
   - Concurrent servers
   - Server selection by file

**Deliverables:**
- `config/config.go`, `config/config_test.go`
- `config/languages.go`

**Test Criteria:**
- Configuration loaded correctly
- Servers restart on crash
- Multiple languages work

### Phase 9: Editor Integration

**Goals:** Full integration with Keystorm editor.

**Tasks:**
1. Implement buffer watcher
   - Event subscription
   - Change translation
2. Register dispatcher handlers
   - All LSP actions
   - Keybinding defaults
3. Implement UI integration
   - Diagnostic display
   - Completion popup
   - Hover tooltip

**Deliverables:**
- Integration code
- Default keybindings

**Test Criteria:**
- End-to-end workflows work
- UI updates correctly

### Phase 10: Testing and Polish

**Goals:** Comprehensive testing and documentation.

**Tasks:**
1. Implement mock server for testing
2. Add integration tests with real servers
3. Performance optimization
4. Documentation

**Deliverables:**
- `testing/mock_server.go`
- Integration tests
- Performance benchmarks
- README and examples

---

## 13. Testing Strategy

### 13.1 Unit Tests

Each component should have thorough unit tests:

```go
// transport_test.go
func TestTransport_Call(t *testing.T) {
    // Test request/response
}

func TestTransport_Notify(t *testing.T) {
    // Test notifications
}

func TestTransport_Timeout(t *testing.T) {
    // Test request timeout
}
```

### 13.2 Mock Server

```go
// testing/mock_server.go
type MockServer struct {
    stdin  io.WriteCloser
    stdout io.ReadCloser

    capabilities ServerCapabilities
    handlers     map[string]MockHandler
}

func NewMockServer() *MockServer {
    // Create pipes
    // Start goroutine to handle requests
}

func (m *MockServer) OnRequest(method string, handler MockHandler) {
    m.handlers[method] = handler
}
```

### 13.3 Integration Tests

```go
// integration_test.go
func TestIntegration_GoServer(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Requires gopls installed
    client := NewClient(Config{
        Servers: map[string]ServerConfig{
            "go": {Command: "gopls"},
        },
    })

    // Test actual LSP operations
}
```

### 13.4 Test Matrix

| Test Type | Coverage Target |
|-----------|----------------|
| Unit | 80%+ line coverage |
| Integration | Core workflows |
| Mock | All protocol messages |
| Benchmark | Performance regression |

---

## 14. Performance Considerations

### 14.1 Latency Targets

| Operation | Target | Approach |
|-----------|--------|----------|
| Completion | <100ms | Cache, prefetch |
| Hover | <50ms | Cache results |
| Definition | <100ms | Index caching |
| Diagnostics | <500ms | Background, debounce |

### 14.2 Optimization Strategies

1. **Request Debouncing**
   - Debounce rapid document changes
   - Coalesce completion requests

2. **Caching**
   - Cache hover/definition results
   - Cache document symbols
   - Invalidate on change

3. **Async Operations**
   - Non-blocking LSP calls
   - Background diagnostic updates
   - Parallel server requests

4. **Resource Management**
   - Limit concurrent requests
   - Connection pooling
   - Memory-mapped file access

### 14.3 Benchmarks

```go
func BenchmarkCompletion(b *testing.B) {
    client := setupClient(b)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        client.Completion(ctx, testFile, testPos)
    }
}

func BenchmarkDocumentSync(b *testing.B) {
    client := setupClient(b)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        client.ChangeDocument(ctx, testFile, smallChange, i)
    }
}
```

---

## Appendix A: Default Server Configurations

```go
var DefaultServers = map[string]ServerConfig{
    "go": {
        Command:     "gopls",
        Args:        []string{"serve"},
        LanguageIDs: []string{"go"},
        FilePatterns: []string{"*.go"},
    },
    "rust": {
        Command:     "rust-analyzer",
        LanguageIDs: []string{"rust"},
        FilePatterns: []string{"*.rs"},
    },
    "typescript": {
        Command:     "typescript-language-server",
        Args:        []string{"--stdio"},
        LanguageIDs: []string{"typescript", "typescriptreact", "javascript", "javascriptreact"},
        FilePatterns: []string{"*.ts", "*.tsx", "*.js", "*.jsx"},
    },
    "python": {
        Command:     "pylsp",
        LanguageIDs: []string{"python"},
        FilePatterns: []string{"*.py"},
    },
}
```

---

## Appendix B: LSP Method Reference

| Category | Method | Direction |
|----------|--------|-----------|
| Lifecycle | initialize | C→S |
| | initialized | C→S |
| | shutdown | C→S |
| | exit | C→S |
| Document Sync | textDocument/didOpen | C→S |
| | textDocument/didChange | C→S |
| | textDocument/didClose | C→S |
| | textDocument/didSave | C→S |
| Completion | textDocument/completion | C→S |
| | completionItem/resolve | C→S |
| Hover | textDocument/hover | C→S |
| Navigation | textDocument/definition | C→S |
| | textDocument/typeDefinition | C→S |
| | textDocument/references | C→S |
| Symbols | textDocument/documentSymbol | C→S |
| | workspace/symbol | C→S |
| Diagnostics | textDocument/publishDiagnostics | S→C |
| Code Actions | textDocument/codeAction | C→S |
| | codeAction/resolve | C→S |
| Formatting | textDocument/formatting | C→S |
| | textDocument/rangeFormatting | C→S |
| Rename | textDocument/prepareRename | C→S |
| | textDocument/rename | C→S |
| Signature | textDocument/signatureHelp | C→S |

---

## Appendix C: Dependencies

```go
// go.mod additions
require (
    go.lsp.dev/jsonrpc2 v0.10.0
    go.lsp.dev/protocol v0.12.0
    go.uber.org/zap v1.26.0  // Logging
)
```

---

## Summary

This implementation plan provides a comprehensive roadmap for building the LSP layer in Keystorm. The 10-phase approach ensures incremental delivery of value while maintaining code quality and testability. Key architectural decisions prioritize:

1. **Clean abstraction** - High-level Client interface hides protocol complexity
2. **Multi-server support** - Different languages, different servers
3. **Reliability** - Crash recovery, timeouts, graceful degradation
4. **Performance** - Async operations, caching, debouncing
5. **Integration** - Seamless connection with plugin system, dispatcher, and renderer

The estimated implementation effort is approximately 8-12 weeks for a single developer, with phases deliverable independently for early testing and feedback.
