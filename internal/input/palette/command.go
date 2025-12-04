package palette

import (
	"fmt"
	"strings"
)

// ArgType defines the type of a command argument.
type ArgType uint8

const (
	// ArgString is a string argument.
	ArgString ArgType = iota

	// ArgNumber is a numeric argument (int or float).
	ArgNumber

	// ArgBoolean is a boolean argument.
	ArgBoolean

	// ArgFile is a file path argument.
	ArgFile

	// ArgEnum is an enumeration argument with predefined options.
	ArgEnum
)

// String returns a string representation of the argument type.
func (t ArgType) String() string {
	switch t {
	case ArgString:
		return "string"
	case ArgNumber:
		return "number"
	case ArgBoolean:
		return "boolean"
	case ArgFile:
		return "file"
	case ArgEnum:
		return "enum"
	default:
		return "unknown"
	}
}

// CommandArg defines a command argument.
type CommandArg struct {
	// Name is the argument identifier.
	Name string

	// Type is the argument type.
	Type ArgType

	// Required indicates if the argument must be provided.
	Required bool

	// Default is the default value if not provided.
	Default any

	// Description explains the argument.
	Description string

	// Options lists valid values for enum types.
	Options []string
}

// Validate checks if a value is valid for this argument.
func (a *CommandArg) Validate(value any) error {
	if value == nil {
		if a.Required {
			return fmt.Errorf("argument %q is required", a.Name)
		}
		return nil
	}

	switch a.Type {
	case ArgString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("argument %q must be a string", a.Name)
		}
	case ArgNumber:
		switch value.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
			// Valid
		default:
			return fmt.Errorf("argument %q must be a number", a.Name)
		}
	case ArgBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("argument %q must be a boolean", a.Name)
		}
	case ArgFile:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("argument %q must be a file path string", a.Name)
		}
	case ArgEnum:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("argument %q must be a string", a.Name)
		}
		valid := false
		for _, opt := range a.Options {
			if opt == str {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("argument %q must be one of: %v", a.Name, a.Options)
		}
	}

	return nil
}

// CommandHandler is a function that executes a command.
type CommandHandler func(args map[string]any) error

// Command represents a registered command in the palette.
type Command struct {
	// ID is the unique command identifier (e.g., "editor.save").
	ID string

	// Title is the display name shown in the palette.
	Title string

	// Description provides additional context about the command.
	Description string

	// Category groups related commands (e.g., "File", "Edit", "View").
	Category string

	// Keybinding shows the keyboard shortcut (for display only).
	Keybinding string

	// Handler executes the command.
	Handler CommandHandler

	// Args defines the command's arguments.
	Args []CommandArg

	// When is a condition expression for availability.
	// Empty means always available.
	When string

	// Source indicates where the command was registered.
	// e.g., "core", "plugin:git", "user"
	Source string
}

// ValidateArgs validates the provided arguments against the command's definition.
func (c *Command) ValidateArgs(args map[string]any) error {
	if args == nil {
		args = make(map[string]any)
	}

	for i := range c.Args {
		arg := &c.Args[i]
		value, exists := args[arg.Name]
		if !exists {
			value = arg.Default
		}
		if err := arg.Validate(value); err != nil {
			return err
		}
	}

	return nil
}

// Execute runs the command with the given arguments.
// The args map is cloned before modification, so the caller's map is not modified.
func (c *Command) Execute(args map[string]any) error {
	if err := c.ValidateArgs(args); err != nil {
		return fmt.Errorf("command %q: %w", c.ID, err)
	}

	if c.Handler == nil {
		return fmt.Errorf("command %q has no handler", c.ID)
	}

	// Clone args map to avoid modifying caller's map
	execArgs := make(map[string]any, len(args))
	for k, v := range args {
		execArgs[k] = v
	}

	// Fill in defaults for missing args
	for i := range c.Args {
		arg := &c.Args[i]
		if _, exists := execArgs[arg.Name]; !exists && arg.Default != nil {
			execArgs[arg.Name] = arg.Default
		}
	}

	return c.Handler(execArgs)
}

// SearchText returns the text to use for fuzzy searching.
// This combines title and description for better matching.
func (c *Command) SearchText() string {
	desc := strings.TrimSpace(c.Description)
	if desc == "" {
		return c.Title
	}
	return c.Title + " " + desc
}
