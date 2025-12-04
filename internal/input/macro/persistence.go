package macro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dshills/keystorm/internal/input/key"
)

// persistedEvent is the JSON-serializable form of key.Event.
type persistedEvent struct {
	Key       uint16 `json:"key"`
	Rune      rune   `json:"rune,omitempty"`
	Modifiers uint8  `json:"modifiers,omitempty"`
}

// persistedMacro represents a single macro for persistence.
type persistedMacro struct {
	Register rune             `json:"register"`
	Events   []persistedEvent `json:"events"`
}

// persistedData is the root structure for macro persistence.
type persistedData struct {
	Version    int              `json:"version"`
	SavedAt    time.Time        `json:"saved_at"`
	LastPlayed rune             `json:"last_played,omitempty"`
	Macros     []persistedMacro `json:"macros"`
}

const currentVersion = 1

// toPersistedEvent converts a key.Event to a persistedEvent.
func toPersistedEvent(e key.Event) persistedEvent {
	return persistedEvent{
		Key:       uint16(e.Key),
		Rune:      e.Rune,
		Modifiers: uint8(e.Modifiers),
	}
}

// toKeyEvent converts a persistedEvent back to a key.Event.
// Note: Timestamps are reset to the current time on load since the original
// timestamps are not persisted. This is intentional as macro playback timing
// is determined by the player, not the original recording times.
func toKeyEvent(p persistedEvent) key.Event {
	return key.Event{
		Key:       key.Key(p.Key),
		Rune:      p.Rune,
		Modifiers: key.Modifier(p.Modifiers),
		Timestamp: time.Now(),
	}
}

// Save writes all macros from the recorder to the specified file.
// The file is written atomically using a temporary file and rename.
func Save(recorder *Recorder, path string) error {
	registers := recorder.GetAllRegisters()

	data := persistedData{
		Version:    currentVersion,
		SavedAt:    time.Now(),
		LastPlayed: recorder.LastPlayed(),
		Macros:     make([]persistedMacro, 0, len(registers)),
	}

	for reg, events := range registers {
		if len(events) == 0 {
			continue
		}

		persisted := persistedMacro{
			Register: reg,
			Events:   make([]persistedEvent, len(events)),
		}
		for i, e := range events {
			persisted.Events[i] = toPersistedEvent(e)
		}
		data.Macros = append(data.Macros, persisted)
	}

	// Marshal to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal macros: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write atomically using temp file + rename
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, jsonData, 0o644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		// Clean up temp file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Load reads macros from the specified file into the recorder.
// Existing macros in the recorder are replaced.
func Load(recorder *Recorder, path string) error {
	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, nothing to load
			return nil
		}
		return fmt.Errorf("failed to read macros file: %w", err)
	}

	var data persistedData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal macros: %w", err)
	}

	// Check version
	if data.Version > currentVersion {
		return fmt.Errorf("unsupported macros file version: %d (max supported: %d)",
			data.Version, currentVersion)
	}

	// Convert persisted data to registers
	registers := make(map[rune][]key.Event, len(data.Macros))
	for _, macro := range data.Macros {
		if !IsValidRegister(macro.Register) {
			continue // Skip invalid registers
		}

		events := make([]key.Event, len(macro.Events))
		for i, p := range macro.Events {
			events[i] = toKeyEvent(p)
		}
		registers[macro.Register] = events
	}

	// Update recorder
	recorder.SetAllRegisters(registers)
	if data.LastPlayed != 0 && IsValidRegister(data.LastPlayed) {
		recorder.SetLastPlayed(data.LastPlayed)
	}

	return nil
}

// LoadOrCreate loads macros from the specified file, creating an empty file if it doesn't exist.
func LoadOrCreate(recorder *Recorder, path string) error {
	err := Load(recorder, path)
	if err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create empty file
		return Save(recorder, path)
	}

	return nil
}

// DefaultMacrosPath returns the default path for storing macros.
// On Unix-like systems: ~/.config/keystorm/macros.json
// On Windows: %APPDATA%/keystorm/macros.json
func DefaultMacrosPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}
	return filepath.Join(configDir, "keystorm", "macros.json"), nil
}

// Export exports macros to a portable format (for sharing/backup).
// Returns the JSON data that can be written to any location.
func Export(recorder *Recorder) ([]byte, error) {
	registers := recorder.GetAllRegisters()

	data := persistedData{
		Version:    currentVersion,
		SavedAt:    time.Now(),
		LastPlayed: recorder.LastPlayed(),
		Macros:     make([]persistedMacro, 0, len(registers)),
	}

	for reg, events := range registers {
		if len(events) == 0 {
			continue
		}

		persisted := persistedMacro{
			Register: reg,
			Events:   make([]persistedEvent, len(events)),
		}
		for i, e := range events {
			persisted.Events[i] = toPersistedEvent(e)
		}
		data.Macros = append(data.Macros, persisted)
	}

	return json.MarshalIndent(data, "", "  ")
}

// Import imports macros from JSON data.
// Merge determines whether to merge with existing macros (true) or replace (false).
func Import(recorder *Recorder, jsonData []byte, merge bool) error {
	var data persistedData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal macros: %w", err)
	}

	if data.Version > currentVersion {
		return fmt.Errorf("unsupported macros version: %d (max supported: %d)",
			data.Version, currentVersion)
	}

	for _, macro := range data.Macros {
		if !IsValidRegister(macro.Register) {
			continue
		}

		events := make([]key.Event, len(macro.Events))
		for i, p := range macro.Events {
			events[i] = toKeyEvent(p)
		}

		if merge && recorder.HasMacro(macro.Register) {
			// Skip existing registers when merging
			continue
		}

		if err := recorder.Set(macro.Register, events); err != nil {
			return fmt.Errorf("failed to set register %c: %w", macro.Register, err)
		}
	}

	return nil
}
