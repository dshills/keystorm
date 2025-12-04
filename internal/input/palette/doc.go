// Package palette provides a searchable command interface for editor commands.
//
// The palette implements a VS Code-style command palette that provides
// discoverable access to all registered editor commands. Key features include:
//
//   - Command registration with ID, title, description, and handler
//   - Fuzzy search across command titles and descriptions
//   - Recent command history for quick access
//   - Command arguments with type validation
//   - Conditional command availability
//
// # Architecture
//
// The palette consists of several components:
//
//   - Command: Represents a registered command with metadata
//   - Palette: Main type managing commands and search
//   - History: Tracks recently executed commands
//   - Filter: Handles search/filter logic with fuzzy matching
//
// # Usage
//
// Create a palette and register commands:
//
//	p := palette.New()
//
//	// Register a command
//	p.Register(&palette.Command{
//	    ID:          "editor.save",
//	    Title:       "Save File",
//	    Description: "Save the current file to disk",
//	    Category:    "File",
//	    Handler:     func(args map[string]any) error { return saveFile() },
//	})
//
//	// Search commands
//	results := p.Search("save", 10)
//
//	// Execute a command
//	err := p.Execute("editor.save", nil)
//
// # Command Arguments
//
// Commands can declare required or optional arguments:
//
//	p.Register(&palette.Command{
//	    ID:    "editor.goToLine",
//	    Title: "Go to Line",
//	    Args: []palette.CommandArg{
//	        {Name: "line", Type: palette.ArgNumber, Required: true},
//	    },
//	    Handler: func(args map[string]any) error {
//	        line := args["line"].(int)
//	        return goToLine(line)
//	    },
//	})
//
// # History
//
// The palette tracks command execution history to prioritize
// recently used commands in search results:
//
//	// Recent commands appear first with empty query
//	results := p.Search("", 10)
//
// # Thread Safety
//
// All palette operations are safe for concurrent use.
package palette
