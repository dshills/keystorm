package task

import (
	"regexp"
	"strconv"
	"sync"
)

// ProblemSeverity indicates the severity of a problem.
type ProblemSeverity string

const (
	// ProblemSeverityError is an error.
	ProblemSeverityError ProblemSeverity = "error"
	// ProblemSeverityWarning is a warning.
	ProblemSeverityWarning ProblemSeverity = "warning"
	// ProblemSeverityInfo is informational.
	ProblemSeverityInfo ProblemSeverity = "info"
)

// Problem represents a detected problem from task output.
type Problem struct {
	// File is the file path where the problem occurred.
	File string

	// Line is the line number (1-based).
	Line int

	// Column is the column number (1-based, 0 if unknown).
	Column int

	// EndLine is the end line for multi-line problems (0 if single line).
	EndLine int

	// EndColumn is the end column for multi-line problems.
	EndColumn int

	// Severity indicates error, warning, or info.
	Severity ProblemSeverity

	// Code is an optional error code.
	Code string

	// Message is the problem description.
	Message string

	// Source is the tool that reported the problem.
	Source string
}

// ProblemPattern defines a regex pattern for matching problems.
type ProblemPattern struct {
	// Pattern is the regex pattern.
	Pattern string

	// File is the capture group for file path (1-based).
	File int

	// Line is the capture group for line number.
	Line int

	// Column is the capture group for column number (0 to skip).
	Column int

	// EndLine is the capture group for end line (0 to skip).
	EndLine int

	// EndColumn is the capture group for end column (0 to skip).
	EndColumn int

	// Severity is the capture group for severity (0 for default).
	Severity int

	// Code is the capture group for error code (0 to skip).
	Code int

	// Message is the capture group for message.
	Message int

	// DefaultSeverity is used when severity group is 0.
	DefaultSeverity ProblemSeverity
}

// ProblemMatcherDefinition defines a problem matcher.
type ProblemMatcherDefinition struct {
	// Name is the matcher name.
	Name string

	// Owner identifies the tool (e.g., "go", "gcc").
	Owner string

	// Patterns are the patterns to match.
	Patterns []ProblemPattern

	// FileLocation indicates how file paths are specified.
	// "relative" means relative to working directory.
	// "absolute" means absolute paths.
	FileLocation string
}

// CompiledMatcher is a compiled problem matcher ready for use.
type CompiledMatcher struct {
	def      ProblemMatcherDefinition
	patterns []*compiledPattern
}

type compiledPattern struct {
	regex   *regexp.Regexp
	pattern ProblemPattern
}

// Match attempts to match a line and extract a problem.
func (m *CompiledMatcher) Match(line string) (Problem, bool) {
	for _, p := range m.patterns {
		matches := p.regex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		problem := Problem{
			Source: m.def.Owner,
		}

		// Extract file
		if p.pattern.File > 0 && p.pattern.File < len(matches) {
			problem.File = matches[p.pattern.File]
		}

		// Extract line number
		if p.pattern.Line > 0 && p.pattern.Line < len(matches) {
			if n, err := strconv.Atoi(matches[p.pattern.Line]); err == nil {
				problem.Line = n
			}
		}

		// Extract column
		if p.pattern.Column > 0 && p.pattern.Column < len(matches) {
			if n, err := strconv.Atoi(matches[p.pattern.Column]); err == nil {
				problem.Column = n
			}
		}

		// Extract end line
		if p.pattern.EndLine > 0 && p.pattern.EndLine < len(matches) {
			if n, err := strconv.Atoi(matches[p.pattern.EndLine]); err == nil {
				problem.EndLine = n
			}
		}

		// Extract end column
		if p.pattern.EndColumn > 0 && p.pattern.EndColumn < len(matches) {
			if n, err := strconv.Atoi(matches[p.pattern.EndColumn]); err == nil {
				problem.EndColumn = n
			}
		}

		// Extract severity
		if p.pattern.Severity > 0 && p.pattern.Severity < len(matches) {
			problem.Severity = parseSeverity(matches[p.pattern.Severity])
		} else {
			problem.Severity = p.pattern.DefaultSeverity
			if problem.Severity == "" {
				problem.Severity = ProblemSeverityError
			}
		}

		// Extract code
		if p.pattern.Code > 0 && p.pattern.Code < len(matches) {
			problem.Code = matches[p.pattern.Code]
		}

		// Extract message
		if p.pattern.Message > 0 && p.pattern.Message < len(matches) {
			problem.Message = matches[p.pattern.Message]
		}

		return problem, true
	}

	return Problem{}, false
}

func parseSeverity(s string) ProblemSeverity {
	switch s {
	case "error", "Error", "ERROR", "fatal", "Fatal", "FATAL":
		return ProblemSeverityError
	case "warning", "Warning", "WARNING", "warn", "Warn", "WARN":
		return ProblemSeverityWarning
	case "info", "Info", "INFO", "note", "Note", "NOTE":
		return ProblemSeverityInfo
	default:
		return ProblemSeverityError
	}
}

// ProblemMatcher manages problem matchers.
type ProblemMatcher struct {
	matchers map[string]*CompiledMatcher
	mu       sync.RWMutex
}

// NewProblemMatcher creates a new problem matcher registry.
func NewProblemMatcher() *ProblemMatcher {
	pm := &ProblemMatcher{
		matchers: make(map[string]*CompiledMatcher),
	}

	// Register built-in matchers
	pm.registerBuiltinMatchers()

	return pm
}

// Register compiles and registers a problem matcher definition.
func (pm *ProblemMatcher) Register(def ProblemMatcherDefinition) error {
	compiled, err := pm.compile(def)
	if err != nil {
		return err
	}

	pm.mu.Lock()
	pm.matchers[def.Name] = compiled
	pm.mu.Unlock()

	return nil
}

// Unregister removes a problem matcher.
func (pm *ProblemMatcher) Unregister(name string) {
	pm.mu.Lock()
	delete(pm.matchers, name)
	pm.mu.Unlock()
}

// GetMatcher returns a compiled matcher by name.
func (pm *ProblemMatcher) GetMatcher(name string) *CompiledMatcher {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.matchers[name]
}

// ListMatchers returns all registered matcher names.
func (pm *ProblemMatcher) ListMatchers() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	names := make([]string, 0, len(pm.matchers))
	for name := range pm.matchers {
		names = append(names, name)
	}
	return names
}

func (pm *ProblemMatcher) compile(def ProblemMatcherDefinition) (*CompiledMatcher, error) {
	compiled := &CompiledMatcher{
		def:      def,
		patterns: make([]*compiledPattern, 0, len(def.Patterns)),
	}

	for _, p := range def.Patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, err
		}
		compiled.patterns = append(compiled.patterns, &compiledPattern{
			regex:   re,
			pattern: p,
		})
	}

	return compiled, nil
}

func (pm *ProblemMatcher) registerBuiltinMatchers() {
	// GCC/Clang style: file:line:column: severity: message
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$gcc",
		Owner: "gcc",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^(.+):(\d+):(\d+):\s*(error|warning|note):\s*(.+)$`,
				File:            1,
				Line:            2,
				Column:          3,
				Severity:        4,
				Message:         5,
				DefaultSeverity: ProblemSeverityError,
			},
			{
				Pattern:         `^(.+):(\d+):\s*(error|warning|note):\s*(.+)$`,
				File:            1,
				Line:            2,
				Severity:        3,
				Message:         4,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})

	// Go compiler: file:line:column: message
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$go",
		Owner: "go",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^(.+):(\d+):(\d+):\s*(.+)$`,
				File:            1,
				Line:            2,
				Column:          3,
				Message:         4,
				DefaultSeverity: ProblemSeverityError,
			},
			{
				Pattern:         `^(.+):(\d+):\s*(.+)$`,
				File:            1,
				Line:            2,
				Message:         3,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})

	// Go test failures: --- FAIL: TestName (0.00s)
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$go-test",
		Owner: "go-test",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^\s*(.+):(\d+):\s*(.+)$`,
				File:            1,
				Line:            2,
				Message:         3,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})

	// TypeScript/ESLint: file(line,col): severity code: message
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$tsc",
		Owner: "typescript",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^(.+)\((\d+),(\d+)\):\s*(error|warning)\s+(\w+):\s*(.+)$`,
				File:            1,
				Line:            2,
				Column:          3,
				Severity:        4,
				Code:            5,
				Message:         6,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})

	// ESLint default format: file:line:col: message severity
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$eslint-compact",
		Owner: "eslint",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^(.+):\s*line\s+(\d+),\s*col\s+(\d+),\s*(Error|Warning)\s*-\s*(.+)$`,
				File:            1,
				Line:            2,
				Column:          3,
				Severity:        4,
				Message:         5,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})

	// ESLint stylish (default): /path/file.js
	//   line:col  severity  message  rule-id
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$eslint-stylish",
		Owner: "eslint",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^\s+(\d+):(\d+)\s+(error|warning)\s+(.+?)\s+(\S+)$`,
				Line:            1,
				Column:          2,
				Severity:        3,
				Message:         4,
				Code:            5,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})

	// Python/pylint: file:line:column: code: message
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$pylint",
		Owner: "pylint",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^(.+):(\d+):(\d+):\s*([A-Z]\d+):\s*(.+)$`,
				File:            1,
				Line:            2,
				Column:          3,
				Code:            4,
				Message:         5,
				DefaultSeverity: ProblemSeverityWarning,
			},
		},
		FileLocation: "relative",
	})

	// Rust/cargo: error[E0001]: message
	//   --> file:line:col
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$rustc",
		Owner: "rustc",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^\s*-->\s*(.+):(\d+):(\d+)$`,
				File:            1,
				Line:            2,
				Column:          3,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})

	// Generic: file:line: message
	_ = pm.Register(ProblemMatcherDefinition{
		Name:  "$generic",
		Owner: "generic",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^(.+):(\d+):\s*(.+)$`,
				File:            1,
				Line:            2,
				Message:         3,
				DefaultSeverity: ProblemSeverityError,
			},
		},
		FileLocation: "relative",
	})
}

// MatchLine tries all registered matchers against a line.
func (pm *ProblemMatcher) MatchLine(line string) (Problem, string, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for name, matcher := range pm.matchers {
		if problem, ok := matcher.Match(line); ok {
			return problem, name, true
		}
	}

	return Problem{}, "", false
}
