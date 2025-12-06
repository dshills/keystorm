// Package terminal provides PTY-based terminal emulation for Keystorm.
//
// The terminal package implements full terminal emulation with:
//
//   - PTY management for Unix and ConPTY for Windows
//   - ANSI escape sequence parsing (CSI, SGR, OSC)
//   - Screen buffer with cell-based rendering
//   - Scrollback history
//   - Shell integration (working directory tracking)
//
// # Architecture
//
// The terminal package is organized around these core types:
//
//   - Terminal: Main interface for terminal instances
//   - PTY: Platform-specific pseudo-terminal implementation
//   - Parser: ANSI escape sequence parser
//   - Screen: Cell-based screen buffer
//   - Manager: Manages multiple terminal instances
//
// # Usage
//
// Create a terminal manager and spawn terminals:
//
//	manager := terminal.NewManager(terminal.ManagerConfig{
//	    EventBus:   eventPublisher,
//	    DefaultShell: "/bin/zsh",
//	})
//
//	// Create a new terminal
//	term, err := manager.Create(terminal.Options{
//	    Name:  "main",
//	    Shell: "/bin/zsh",
//	    Cols:  80,
//	    Rows:  24,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Write to terminal
//	term.Write([]byte("ls -la\n"))
//
//	// Read screen state
//	screen := term.Screen()
//	for y := 0; y < screen.Height; y++ {
//	    for x := 0; x < screen.Width; x++ {
//	        cell := screen.Cell(x, y)
//	        // Process cell...
//	    }
//	}
//
// # ANSI Support
//
// The parser supports common ANSI escape sequences:
//
//   - CSI sequences for cursor movement and screen control
//   - SGR sequences for colors and text attributes
//   - OSC sequences for title and shell integration
//   - DEC private modes
//
// # Thread Safety
//
// All types in this package are safe for concurrent use.
package terminal
