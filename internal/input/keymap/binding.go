package keymap

import (
	"github.com/dshills/keystorm/internal/input/key"
)

// Binding represents a single key-to-action mapping.
type Binding struct {
	// Keys is the key sequence that triggers this binding.
	// Formats: "j", "g g", "C-s", "<C-S-a>", "Ctrl+Shift+A"
	Keys string

	// Action is the command to execute.
	// Examples: "cursor.down", "editor.save", "mode.insert"
	Action string

	// Args are fixed arguments for the action.
	Args map[string]any

	// When is a condition expression that must be true for this binding.
	// Examples: "editorTextFocus", "!editorReadonly", "resourceLangId == go"
	When string

	// Description provides documentation for the binding.
	Description string

	// Priority determines precedence when multiple bindings match.
	// Higher priority wins. Default is 0.
	Priority int

	// Category groups bindings for display purposes.
	Category string
}

// NewBinding creates a new binding with the given keys and action.
func NewBinding(keys, action string) Binding {
	return Binding{
		Keys:   keys,
		Action: action,
	}
}

// WithArgs sets arguments for this binding.
func (b Binding) WithArgs(args map[string]any) Binding {
	b.Args = args
	return b
}

// WithWhen sets the condition for this binding.
func (b Binding) WithWhen(when string) Binding {
	b.When = when
	return b
}

// WithDescription sets the description for this binding.
func (b Binding) WithDescription(desc string) Binding {
	b.Description = desc
	return b
}

// WithPriority sets the priority for this binding.
func (b Binding) WithPriority(priority int) Binding {
	b.Priority = priority
	return b
}

// WithCategory sets the category for this binding.
func (b Binding) WithCategory(category string) Binding {
	b.Category = category
	return b
}

// ParsedBinding is a binding with a pre-parsed key sequence.
type ParsedBinding struct {
	Binding
	Sequence *key.Sequence
}

// Match checks if this binding's key sequence matches the given sequence.
func (pb *ParsedBinding) Match(seq *key.Sequence) bool {
	if pb == nil || pb.Sequence == nil || seq == nil {
		return false
	}
	return pb.Sequence.Equals(seq)
}

// IsPrefix checks if the given sequence is a prefix of this binding's sequence.
func (pb *ParsedBinding) IsPrefix(seq *key.Sequence) bool {
	if pb == nil || pb.Sequence == nil || seq == nil {
		return false
	}
	return pb.Sequence.HasPrefix(seq)
}

// BindingMatch represents a matched binding with its context.
type BindingMatch struct {
	// Binding is the matched binding.
	*ParsedBinding

	// Keymap is the keymap containing the binding.
	Keymap *Keymap

	// Score is used for sorting matches by priority.
	Score int
}

// Less returns true if this match should come before another.
// Higher scores come first.
func (bm BindingMatch) Less(other BindingMatch) bool {
	// Handle nil cases - nil keymaps sort last
	if bm.Keymap == nil && other.Keymap == nil {
		return false
	}
	if bm.Keymap == nil {
		return false
	}
	if other.Keymap == nil {
		return true
	}

	// First compare combined priority (keymap + binding)
	thisScore := bm.Score
	otherScore := other.Score

	if thisScore != otherScore {
		return thisScore > otherScore
	}

	// Then prefer more specific keymaps (mode-specific over global)
	thisModeSpecific := bm.Keymap.Mode != ""
	otherModeSpecific := other.Keymap.Mode != ""
	if thisModeSpecific != otherModeSpecific {
		return thisModeSpecific
	}

	// Then prefer filetype-specific
	thisFileSpecific := bm.Keymap.FileType != ""
	otherFileSpecific := other.Keymap.FileType != ""
	if thisFileSpecific != otherFileSpecific {
		return thisFileSpecific
	}

	return false
}

// CalculateScore calculates the priority score for this match.
func (bm *BindingMatch) CalculateScore() {
	if bm.Keymap == nil || bm.ParsedBinding == nil {
		bm.Score = 0
		return
	}

	// Base score from keymap priority
	bm.Score = bm.Keymap.Priority * 100

	// Add binding priority
	bm.Score += bm.ParsedBinding.Priority

	// Bonus for mode-specific bindings
	if bm.Keymap.Mode != "" {
		bm.Score += 50
	}

	// Bonus for filetype-specific bindings
	if bm.Keymap.FileType != "" {
		bm.Score += 25
	}
}

// BindingCategory represents a category of bindings for display.
type BindingCategory struct {
	Name     string
	Bindings []Binding
}

// GroupByCategory groups bindings by their category.
func GroupByCategory(bindings []Binding) []BindingCategory {
	categoryMap := make(map[string][]Binding)
	order := make([]string, 0)

	for _, b := range bindings {
		cat := b.Category
		if cat == "" {
			cat = "Other"
		}
		if _, exists := categoryMap[cat]; !exists {
			order = append(order, cat)
		}
		categoryMap[cat] = append(categoryMap[cat], b)
	}

	result := make([]BindingCategory, 0, len(order))
	for _, name := range order {
		result = append(result, BindingCategory{
			Name:     name,
			Bindings: categoryMap[name],
		})
	}
	return result
}
