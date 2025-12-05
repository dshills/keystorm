// Package lsp provides Language Server Protocol (LSP) client integration for Keystorm.
//
// The LSP layer enables intelligent code features by communicating with external
// language servers (gopls, rust-analyzer, typescript-language-server, etc.).
// It abstracts the complexity of JSON-RPC communication, server lifecycle management,
// and protocol negotiation while exposing a clean interface to the rest of Keystorm.
//
// # Architecture
//
// The package is organized around these core components:
//
//   - Client: High-level interface for LSP operations
//   - Manager: Manages multiple language server lifecycles
//   - Server: Single server connection and communication
//   - Transport: JSON-RPC 2.0 protocol implementation
//
// # Quick Start
//
// Create and initialize the LSP client:
//
//	config := lsp.DefaultConfig()
//	client := lsp.NewClient(config)
//
//	if err := client.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Shutdown(ctx)
//
//	// Open a document
//	client.OpenDocument(ctx, "/path/to/file.go", "go", content)
//
//	// Request completions
//	result, err := client.Completion(ctx, "/path/to/file.go", lsp.Position{Line: 10, Character: 5})
//
// # Server Configuration
//
// Servers are configured per-language:
//
//	config := lsp.Config{
//	    Servers: map[string]lsp.ServerConfig{
//	        "go": {
//	            Command:     "gopls",
//	            Args:        []string{"serve"},
//	            LanguageIDs: []string{"go"},
//	        },
//	        "rust": {
//	            Command:     "rust-analyzer",
//	            LanguageIDs: []string{"rust"},
//	        },
//	    },
//	}
//
// # Features
//
// The LSP client supports:
//   - Code completion with filtering and sorting
//   - Hover information
//   - Go-to-definition/type-definition
//   - Find references
//   - Document and workspace symbols
//   - Real-time diagnostics (errors, warnings)
//   - Code actions (quick fixes, refactorings)
//   - Document formatting
//   - Symbol renaming
//   - Signature help
//
// # Multi-Server Support
//
// The Manager handles multiple concurrent language servers. Servers are started
// lazily when files of that language are opened, and shut down gracefully when
// no longer needed.
//
// # Crash Recovery
//
// Servers are monitored and automatically restarted on crash with exponential
// backoff. Open documents are re-synced to the new server instance.
//
// # Thread Safety
//
// The Client and Manager are safe for concurrent use. Individual Server instances
// use internal locking for thread safety.
//
// # Integration
//
// The LSP client integrates with:
//   - Plugin system: Implements api.LSPProvider interface
//   - Dispatcher: Registers action handlers for LSP operations
//   - Engine: Syncs buffer changes to language servers
//   - Event Bus: Publishes diagnostic updates
package lsp
