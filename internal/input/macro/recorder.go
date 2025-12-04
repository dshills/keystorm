package macro

import (
	"fmt"
	"sync"

	"github.com/dshills/keystorm/internal/input/key"
)

// Recorder records key sequences for macro playback.
// It maintains a set of registers, each capable of storing a sequence of key events.
type Recorder struct {
	mu         sync.Mutex
	recording  bool
	register   rune
	events     []key.Event
	registers  map[rune][]key.Event
	lastPlayed rune // Tracks last played register for @@ support
}

// NewRecorder creates a new macro recorder with empty registers.
func NewRecorder() *Recorder {
	return &Recorder{
		registers: make(map[rune][]key.Event),
	}
}

// StartRecording begins recording to the specified register.
// Returns an error if already recording or if the register is invalid.
func (r *Recorder) StartRecording(register rune) error {
	if !IsValidRegister(register) {
		return fmt.Errorf("invalid register: %c", register)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return fmt.Errorf("already recording to register %c", r.register)
	}

	r.recording = true
	r.register = register
	r.events = nil
	return nil
}

// StopRecording ends the current recording and saves it to the register.
// Returns the recorded events, or nil if not recording.
// Note: The returned slice is the original recording (not a copy), but modifying
// it will not affect the saved register since a copy is stored internally.
func (r *Recorder) StopRecording() []key.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return nil
	}

	r.recording = false
	// Only save if we have events
	if len(r.events) > 0 {
		// Make a copy for the register
		saved := make([]key.Event, len(r.events))
		copy(saved, r.events)
		r.registers[r.register] = saved
	}
	result := r.events
	r.events = nil
	return result
}

// IsRecording returns true if currently recording.
func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

// CurrentRegister returns the register being recorded to, or 0 if not recording.
func (r *Recorder) CurrentRegister() rune {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recording {
		return r.register
	}
	return 0
}

// Record adds a key event to the current recording.
// Does nothing if not recording.
func (r *Recorder) Record(event key.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		r.events = append(r.events, event)
	}
}

// Get retrieves the macro stored in a register.
// Returns an empty slice if the register is empty or invalid.
func (r *Recorder) Get(register rune) []key.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := r.registers[register]
	if len(events) == 0 {
		return []key.Event{}
	}
	// Return a copy to prevent external modification
	result := make([]key.Event, len(events))
	copy(result, events)
	return result
}

// Set stores a macro in a register, replacing any existing content.
// Returns an error if the register is invalid.
func (r *Recorder) Set(register rune, events []key.Event) error {
	if !IsValidRegister(register) {
		return fmt.Errorf("invalid register: %c", register)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(events) == 0 {
		delete(r.registers, register)
		return nil
	}

	// Make a copy to prevent external modification
	saved := make([]key.Event, len(events))
	copy(saved, events)
	r.registers[register] = saved
	return nil
}

// Append adds events to an existing macro in a register.
// If the register is empty, this creates a new macro.
// Returns an error if the register is invalid.
func (r *Recorder) Append(register rune, events []key.Event) error {
	if !IsValidRegister(register) {
		return fmt.Errorf("invalid register: %c", register)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Make a copy of the events to append
	toAppend := make([]key.Event, len(events))
	copy(toAppend, events)

	r.registers[register] = append(r.registers[register], toAppend...)
	return nil
}

// Clear removes all events from a register.
// Returns an error if the register is invalid.
func (r *Recorder) Clear(register rune) error {
	if !IsValidRegister(register) {
		return fmt.Errorf("invalid register: %c", register)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.registers, register)
	return nil
}

// ClearAll removes all macros from all registers.
func (r *Recorder) ClearAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.registers = make(map[rune][]key.Event)
}

// HasMacro returns true if the register contains a macro.
func (r *Recorder) HasMacro(register rune) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	events, ok := r.registers[register]
	return ok && len(events) > 0
}

// ListRegisters returns a list of registers that contain macros.
func (r *Recorder) ListRegisters() []rune {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]rune, 0, len(r.registers))
	for reg, events := range r.registers {
		if len(events) > 0 {
			result = append(result, reg)
		}
	}
	return result
}

// EventCount returns the number of events in a register's macro.
func (r *Recorder) EventCount(register rune) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.registers[register])
}

// SetLastPlayed sets the last played register (for @@ support).
func (r *Recorder) SetLastPlayed(register rune) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastPlayed = register
}

// LastPlayed returns the last played register (for @@ support).
// Returns 0 if no macro has been played.
func (r *Recorder) LastPlayed() rune {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastPlayed
}

// CurrentRecordingLength returns the number of events recorded so far.
// Returns 0 if not recording.
func (r *Recorder) CurrentRecordingLength() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.recording {
		return 0
	}
	return len(r.events)
}

// GetAllRegisters returns a map of all registers and their contents.
// Used for persistence operations.
func (r *Recorder) GetAllRegisters() map[rune][]key.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make(map[rune][]key.Event, len(r.registers))
	for reg, events := range r.registers {
		if len(events) > 0 {
			copied := make([]key.Event, len(events))
			copy(copied, events)
			result[reg] = copied
		}
	}
	return result
}

// SetAllRegisters replaces all registers with the provided map.
// Used for persistence operations.
func (r *Recorder) SetAllRegisters(registers map[rune][]key.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.registers = make(map[rune][]key.Event, len(registers))
	for reg, events := range registers {
		if IsValidRegister(reg) && len(events) > 0 {
			copied := make([]key.Event, len(events))
			copy(copied, events)
			r.registers[reg] = copied
		}
	}
}
