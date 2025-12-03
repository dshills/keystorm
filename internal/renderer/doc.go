// Package renderer provides the display layer for the Keystorm editor.
//
// The renderer is responsible for:
//   - Converting buffer content to visual output
//   - Line layout with tab expansion and Unicode handling
//   - Syntax highlighting integration
//   - Cursor and selection rendering
//   - AI overlay rendering (ghost text, diff previews)
//   - Efficient dirty region tracking for incremental updates
//   - Backend abstraction for terminal/GUI output
//
// Architecture:
//
// The renderer follows a layered design:
//
//	┌─────────────────────────────────────────┐
//	│           Renderer (Facade)             │
//	├─────────────────────────────────────────┤
//	│  Viewport │ LineCache │ StyleResolver   │
//	│  Scrolling│ DirtyTrack│ CursorRenderer  │
//	├─────────────────────────────────────────┤
//	│           Backend Abstraction           │
//	├─────────────────────────────────────────┤
//	│  Terminal (tcell) │ Future: GUI         │
//	└─────────────────────────────────────────┘
//
// Usage:
//
//	backend, _ := backend.NewTerminal()
//	r := renderer.New(backend, renderer.DefaultOptions())
//	r.SetBuffer(engine)
//	r.Render()
package renderer
