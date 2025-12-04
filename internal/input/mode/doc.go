// Package mode provides the modal editing system for keystorm.
//
// The mode system implements Vim-style modal editing with support for:
//   - Normal mode: Navigation and commands
//   - Insert mode: Text input
//   - Visual mode: Character-wise selection
//   - Visual Line mode: Line-wise selection
//   - Visual Block mode: Block/column selection
//   - Command mode: Ex-style command line
//   - Operator-pending mode: Waiting for motion/text object
//
// # Architecture
//
// The mode system is built around the Mode interface, which defines the
// contract for all editor modes. The Manager coordinates mode transitions
// and maintains mode history.
//
// # Mode Lifecycle
//
//	┌─────────┐    Enter()    ┌─────────┐
//	│ Mode A  │ ───────────▶ │ Mode B  │
//	└─────────┘              └─────────┘
//	     │                        │
//	     │  Exit()                │
//	     ◀────────────────────────┘
//
// When switching modes:
// 1. Current mode's Exit() is called
// 2. New mode's Enter() is called
// 3. Mode change callbacks are notified
//
// # Custom Modes
//
// Plugins can register custom modes by implementing the Mode interface
// and registering with the Manager:
//
//	type MyMode struct{}
//	func (m *MyMode) Name() string { return "mymode" }
//	// ... implement other Mode methods
//
//	manager.Register(&MyMode{})
package mode
