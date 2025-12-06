package layer

// Standard priority levels for configuration layers.
// Higher values override lower values during merging.
const (
	// PriorityBuiltin is the lowest priority for built-in defaults.
	PriorityBuiltin = 0

	// PriorityUserGlobal is for user global settings (~/.config/keystorm/).
	PriorityUserGlobal = 100

	// PriorityUserKeymaps is for user keymap settings.
	PriorityUserKeymaps = 150

	// PriorityWorkspace is for workspace/project settings (.keystorm/).
	PriorityWorkspace = 200

	// PriorityLanguage is for language-specific overrides.
	PriorityLanguage = 300

	// PriorityPlugin is for plugin-provided settings.
	PriorityPlugin = 400

	// PriorityEnv is for environment variable overrides.
	PriorityEnv = 500

	// PriorityArgs is for command-line argument overrides.
	PriorityArgs = 600

	// PrioritySession is the highest priority for session overrides.
	PrioritySession = 1000
)

// DefaultPriority returns the default priority for a given source.
func DefaultPriority(source Source) int {
	switch source {
	case SourceBuiltin:
		return PriorityBuiltin
	case SourceUserGlobal:
		return PriorityUserGlobal
	case SourceWorkspace:
		return PriorityWorkspace
	case SourceLanguage:
		return PriorityLanguage
	case SourceEnv:
		return PriorityEnv
	case SourceArgs:
		return PriorityArgs
	case SourcePlugin:
		return PriorityPlugin
	case SourceSession:
		return PrioritySession
	default:
		return PriorityBuiltin
	}
}

// StandardLayerNames defines standard names for configuration layers.
var StandardLayerNames = map[Source]string{
	SourceBuiltin:    "defaults",
	SourceUserGlobal: "user",
	SourceWorkspace:  "workspace",
	SourceLanguage:   "language",
	SourceEnv:        "environment",
	SourceArgs:       "arguments",
	SourcePlugin:     "plugin",
	SourceSession:    "session",
}

// StandardLayerName returns the standard name for a source.
func StandardLayerName(source Source) string {
	if name, ok := StandardLayerNames[source]; ok {
		return name
	}
	return "unknown"
}
