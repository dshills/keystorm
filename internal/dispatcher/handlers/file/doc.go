// Package file provides handlers for file operations.
//
// This package implements file management functionality including:
//   - Save current buffer (:w)
//   - Save as (:saveas)
//   - Open file (:e)
//   - Reload from disk (:e!)
//   - Close buffer (:bd)
//   - Buffer switching (:bn, :bp)
//   - New file creation
//
// File handlers coordinate with the engine for buffer content
// and the project system for file management.
package file
