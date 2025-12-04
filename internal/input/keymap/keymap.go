package keymap

import (
	"fmt"

	"github.com/dshills/keystorm/internal/input/key"
)

// Keymap holds key bindings for a mode or context.
type Keymap struct {
	// Name is the keymap identifier.
	Name string

	// Mode is the mode this keymap applies to.
	// Empty string means global (all modes).
	Mode string

	// FileType restricts this keymap to a specific file type.
	// Empty string means all file types.
	FileType string

	// Bindings are the key-to-action mappings.
	Bindings []Binding

	// Priority determines precedence when multiple keymaps match.
	// Higher priority wins. Default is 0.
	Priority int

	// Source indicates where this keymap was defined.
	// Examples: "default", "user", "plugin:vim-surround"
	Source string
}

// NewKeymap creates a new keymap with the given name.
func NewKeymap(name string) *Keymap {
	return &Keymap{
		Name:     name,
		Bindings: make([]Binding, 0),
	}
}

// ForMode sets the mode for this keymap.
func (k *Keymap) ForMode(mode string) *Keymap {
	k.Mode = mode
	return k
}

// ForFileType sets the file type for this keymap.
func (k *Keymap) ForFileType(fileType string) *Keymap {
	k.FileType = fileType
	return k
}

// WithPriority sets the priority for this keymap.
func (k *Keymap) WithPriority(priority int) *Keymap {
	k.Priority = priority
	return k
}

// WithSource sets the source for this keymap.
func (k *Keymap) WithSource(source string) *Keymap {
	k.Source = source
	return k
}

// Add adds a binding to this keymap.
func (k *Keymap) Add(keys, action string) *Keymap {
	k.Bindings = append(k.Bindings, Binding{
		Keys:   keys,
		Action: action,
	})
	return k
}

// AddBinding adds a fully configured binding to this keymap.
func (k *Keymap) AddBinding(binding Binding) *Keymap {
	k.Bindings = append(k.Bindings, binding)
	return k
}

// Validate checks that all bindings in the keymap are valid.
func (k *Keymap) Validate() error {
	for i, b := range k.Bindings {
		if b.Keys == "" {
			return fmt.Errorf("binding %d: empty keys", i)
		}
		if b.Action == "" {
			return fmt.Errorf("binding %d (%s): empty action", i, b.Keys)
		}
		// Try to parse the key sequence
		if _, err := key.ParseSequence(b.Keys); err != nil {
			return fmt.Errorf("binding %d (%s): %w", i, b.Keys, err)
		}
	}
	return nil
}

// ParsedKeymap is a keymap with pre-parsed key sequences.
type ParsedKeymap struct {
	*Keymap
	ParsedBindings []ParsedBinding
}

// Parse parses all bindings in the keymap.
func (k *Keymap) Parse() (*ParsedKeymap, error) {
	parsed := &ParsedKeymap{
		Keymap:         k,
		ParsedBindings: make([]ParsedBinding, 0, len(k.Bindings)),
	}

	for _, b := range k.Bindings {
		seq, err := key.ParseSequence(b.Keys)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", b.Keys, err)
		}
		// seq is already *key.Sequence from ParseSequence
		parsed.ParsedBindings = append(parsed.ParsedBindings, ParsedBinding{
			Binding:  b,
			Sequence: seq,
		})
	}

	return parsed, nil
}

// Clone creates a deep copy of the keymap.
func (k *Keymap) Clone() *Keymap {
	clone := &Keymap{
		Name:     k.Name,
		Mode:     k.Mode,
		FileType: k.FileType,
		Priority: k.Priority,
		Source:   k.Source,
		Bindings: make([]Binding, len(k.Bindings)),
	}
	for i, b := range k.Bindings {
		clone.Bindings[i] = Binding{
			Keys:        b.Keys,
			Action:      b.Action,
			When:        b.When,
			Description: b.Description,
			Priority:    b.Priority,
			Category:    b.Category,
		}
		// Deep copy Args map if present
		if b.Args != nil {
			clone.Bindings[i].Args = make(map[string]any, len(b.Args))
			for k, v := range b.Args {
				clone.Bindings[i].Args[k] = v
			}
		}
	}
	return clone
}
