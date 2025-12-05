// Package project provides workspace and file management for Keystorm.
//
// The project package handles file operations, workspace management, file watching,
// indexing, search, and project graph construction. Unlike simple editors that treat
// files individually, Keystorm models the project as a graph of interconnected nodes
// (files, modules, services, tests, APIs) with edges representing relationships
// (imports, calls, ownership).
//
// # Architecture
//
// The package is organized around these core components:
//
//   - Project: Main interface for workspace and file operations
//   - VFS: Virtual file system abstraction for file I/O
//   - Watcher: File system change detection
//   - Index: Fast file lookup and search
//   - Graph: Project structure and relationships
//
// # Quick Start
//
// Open a workspace and work with files:
//
//	proj := project.New()
//	if err := proj.Open(ctx, "/path/to/workspace"); err != nil {
//	    log.Fatal(err)
//	}
//	defer proj.Close(ctx)
//
//	// Open a file
//	doc, err := proj.OpenFile(ctx, "/path/to/workspace/main.go")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Search for files
//	matches, err := proj.FindFiles(ctx, "main", project.FindOptions{Limit: 10})
//
// # Virtual File System
//
// The VFS abstraction allows swapping the underlying file system:
//
//	// Use OS file system (default)
//	osfs := vfs.NewOSFS()
//
//	// Use in-memory file system (for testing)
//	memfs := vfs.NewMemFS()
//	memfs.WriteFile("/test.go", []byte("package main"), 0644)
//
// # File Watching
//
// The watcher detects external file changes:
//
//	proj.OnFileChange(func(event project.FileChangeEvent) {
//	    switch event.Type {
//	    case project.FileChangeModified:
//	        // Handle external modification
//	    case project.FileChangeDeleted:
//	        // Handle deletion
//	    }
//	})
//
// # Project Graph
//
// The graph tracks relationships between files:
//
//	graph := proj.Graph()
//	related, err := proj.RelatedFiles(ctx, "/path/to/file.go")
//
// # Integration Points
//
// The project package integrates with:
//   - Dispatcher: File/project actions (open, save, search)
//   - LSP: Workspace folders for language servers
//   - Context Engine: Project graph for AI prompts
//   - Event Bus: File change notifications
//   - Plugin API: ProjectProvider interface
//
// # Thread Safety
//
// The Project interface and its components are safe for concurrent use.
// Individual VFS implementations document their own concurrency guarantees.
package project
