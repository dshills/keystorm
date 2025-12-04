// Package macro provides keyboard macro recording and playback for Keystorm.
//
// This package implements Vim-style macro functionality where key sequences
// can be recorded to named registers and played back with optional repeat counts.
//
// # Concepts
//
// A macro is a recorded sequence of key events that can be replayed.
// Macros are stored in registers, identified by lowercase letters (a-z)
// or digits (0-9), following Vim conventions.
//
// # Recording
//
// Recording is started by calling StartRecording with a register name.
// While recording, key events are captured via the Record method.
// Recording ends by calling StopRecording, which saves the events to the register.
//
// Example:
//
//	recorder := macro.NewRecorder()
//	recorder.StartRecording('a')  // Start recording to register 'a'
//	// ... user types keys, each passed to Record() ...
//	recorder.StopRecording()      // Save macro to register 'a'
//
// # Playback
//
// Macros are played back using the Player, which sends recorded events
// through a callback function. Playback supports a repeat count.
//
// Example:
//
//	player := macro.NewPlayer(recorder)
//	player.Play('a', 5, func(event key.Event) {
//	    // Handle replayed event
//	})
//
// # Registers
//
// Valid registers are:
//   - a-z: Named registers (26 total)
//   - 0-9: Numbered registers (10 total)
//
// Special registers:
//   - Register 0 is typically used for the last yank operation
//   - Register 1-9 may be used for recent deletions (not enforced)
//
// # Persistence
//
// Macros can be saved to and loaded from disk using JSON format.
// This allows macros to persist across editor sessions.
//
// # Thread Safety
//
// All types in this package are safe for concurrent use.
// Recording and playback can occur from different goroutines.
package macro
