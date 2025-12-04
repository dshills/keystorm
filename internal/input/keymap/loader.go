package keymap

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Loader loads keymaps from configuration files.
type Loader struct {
	// searchPaths are directories to search for keymap files.
	searchPaths []string
}

// NewLoader creates a new keymap loader.
func NewLoader() *Loader {
	return &Loader{
		searchPaths: make([]string, 0),
	}
}

// AddSearchPath adds a directory to search for keymap files.
func (l *Loader) AddSearchPath(path string) {
	l.searchPaths = append(l.searchPaths, path)
}

// LoadFile loads a keymap from a JSON file.
func (l *Loader) LoadFile(path string) (*Keymap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening keymap file: %w", err)
	}
	defer f.Close()

	return l.LoadReader(f)
}

// LoadReader loads a keymap from a reader.
func (l *Loader) LoadReader(r io.Reader) (*Keymap, error) {
	var config keymapConfig
	if err := json.NewDecoder(r).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding keymap: %w", err)
	}

	km := &Keymap{
		Name:     config.Name,
		Mode:     config.Mode,
		FileType: config.FileType,
		Priority: config.Priority,
		Source:   config.Source,
		Bindings: make([]Binding, 0, len(config.Bindings)),
	}

	for _, bc := range config.Bindings {
		km.Bindings = append(km.Bindings, Binding(bc))
	}

	return km, nil
}

// LoadAll loads all keymaps from the search paths.
func (l *Loader) LoadAll() ([]*Keymap, error) {
	keymaps := make([]*Keymap, 0)

	for _, dir := range l.searchPaths {
		matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			continue
		}

		for _, path := range matches {
			km, err := l.LoadFile(path)
			if err != nil {
				// Log warning but continue
				continue
			}
			keymaps = append(keymaps, km)
		}
	}

	return keymaps, nil
}

// LoadAndRegister loads all keymaps and registers them.
func (l *Loader) LoadAndRegister(registry *Registry) error {
	keymaps, err := l.LoadAll()
	if err != nil {
		return err
	}

	for _, km := range keymaps {
		if err := registry.Register(km); err != nil {
			return fmt.Errorf("registering keymap %q: %w", km.Name, err)
		}
	}

	return nil
}

// keymapConfig is the JSON structure for keymap files.
type keymapConfig struct {
	Name     string          `json:"name"`
	Mode     string          `json:"mode,omitempty"`
	FileType string          `json:"fileType,omitempty"`
	Priority int             `json:"priority,omitempty"`
	Source   string          `json:"source,omitempty"`
	Bindings []bindingConfig `json:"bindings"`
}

type bindingConfig struct {
	Keys        string         `json:"keys"`
	Action      string         `json:"action"`
	Args        map[string]any `json:"args,omitempty"`
	When        string         `json:"when,omitempty"`
	Description string         `json:"description,omitempty"`
	Priority    int            `json:"priority,omitempty"`
	Category    string         `json:"category,omitempty"`
}

// MarshalJSON converts a keymap to JSON.
func (k *Keymap) MarshalJSON() ([]byte, error) {
	config := keymapConfig{
		Name:     k.Name,
		Mode:     k.Mode,
		FileType: k.FileType,
		Priority: k.Priority,
		Source:   k.Source,
		Bindings: make([]bindingConfig, 0, len(k.Bindings)),
	}

	for _, b := range k.Bindings {
		config.Bindings = append(config.Bindings, bindingConfig(b))
	}

	return json.MarshalIndent(config, "", "  ")
}

// UnmarshalJSON parses a keymap from JSON.
func (k *Keymap) UnmarshalJSON(data []byte) error {
	var config keymapConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	k.Name = config.Name
	k.Mode = config.Mode
	k.FileType = config.FileType
	k.Priority = config.Priority
	k.Source = config.Source
	k.Bindings = make([]Binding, 0, len(config.Bindings))

	for _, bc := range config.Bindings {
		k.Bindings = append(k.Bindings, Binding(bc))
	}

	return nil
}

// SaveFile saves a keymap to a JSON file.
func (k *Keymap) SaveFile(path string) error {
	data, err := k.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshaling keymap: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing keymap file: %w", err)
	}

	return nil
}
