package loader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// TOMLLoader loads configuration from TOML files.
type TOMLLoader struct {
	fs   FileSystem
	path string
}

// NewTOMLLoader creates a new TOML loader for the given path.
func NewTOMLLoader(path string) *TOMLLoader {
	return &TOMLLoader{
		fs:   DefaultFS(),
		path: path,
	}
}

// NewTOMLLoaderWithFS creates a TOML loader with a custom file system.
func NewTOMLLoaderWithFS(fs FileSystem, path string) *TOMLLoader {
	return &TOMLLoader{
		fs:   fs,
		path: path,
	}
}

// Load reads configuration from the configured path.
func (l *TOMLLoader) Load() (map[string]any, error) {
	return l.LoadFrom(l.path)
}

// LoadFrom reads configuration from a specific path.
func (l *TOMLLoader) LoadFrom(path string) (map[string]any, error) {
	data, err := l.fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist, not an error
		}
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	return l.parse(path, data)
}

// LoadFromReader reads configuration from an io.Reader.
func (l *TOMLLoader) LoadFromReader(r io.Reader) (map[string]any, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return l.parse("<reader>", data)
}

// parse parses TOML data into a map.
func (l *TOMLLoader) parse(source string, data []byte) (map[string]any, error) {
	var config map[string]any
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, &ParseError{
			Path:    source,
			Message: err.Error(),
			Err:     err,
		}
	}

	return config, nil
}

// LoadWithIncludes loads a TOML file and processes @include directives.
// The maxDepth parameter limits nested includes to prevent infinite loops.
func (l *TOMLLoader) LoadWithIncludes(path string, maxDepth int) (map[string]any, error) {
	if maxDepth <= 0 {
		return nil, fmt.Errorf("include depth exceeded for %s", path)
	}

	config, err := l.LoadFrom(path)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, nil
	}

	// Process @include directive
	includes, hasIncludes := config["@include"]
	if !hasIncludes {
		return config, nil
	}

	// Remove the @include key from result
	delete(config, "@include")

	// Handle includes
	baseDir := filepath.Dir(path)
	var includeList []string

	switch v := includes.(type) {
	case string:
		includeList = []string{v}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("@include must be string or array of strings")
			}
			includeList = append(includeList, s)
		}
	case []string:
		includeList = v
	default:
		return nil, fmt.Errorf("@include must be string or array of strings, got %T", includes)
	}

	// Load and merge includes (includes are lower priority than main file)
	for _, inc := range includeList {
		incPath := inc
		if !filepath.IsAbs(inc) {
			incPath = filepath.Join(baseDir, inc)
		}

		incConfig, err := l.LoadWithIncludes(incPath, maxDepth-1)
		if err != nil {
			return nil, fmt.Errorf("loading include %s: %w", incPath, err)
		}

		// Merge: main file values override include values
		config = DeepMerge(incConfig, config)
	}

	return config, nil
}

// ParseError represents an error while parsing a configuration file.
type ParseError struct {
	Path    string
	Line    int
	Column  int
	Message string
	Err     error
}

func (e *ParseError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("parse error in %s at line %d, column %d: %s", e.Path, e.Line, e.Column, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("parse error in %s at line %d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("parse error in %s: %s", e.Path, e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// DeepMerge recursively merges src into dst.
// Values in src override values in dst.
// Maps are merged recursively; other types are replaced.
func DeepMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}
	if src == nil {
		return dst
	}

	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			dst[key] = srcVal
			continue
		}

		// If both are maps, merge recursively
		srcMap, srcIsMap := srcVal.(map[string]any)
		dstMap, dstIsMap := dstVal.(map[string]any)
		if srcIsMap && dstIsMap {
			dst[key] = DeepMerge(dstMap, srcMap)
		} else {
			// Otherwise, src replaces dst
			dst[key] = srcVal
		}
	}

	return dst
}

// Clone creates a deep copy of a configuration map.
func Clone(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for key, val := range src {
		switch v := val.(type) {
		case map[string]any:
			dst[key] = Clone(v)
		case []any:
			dst[key] = cloneSlice(v)
		default:
			dst[key] = val
		}
	}

	return dst
}

// cloneSlice creates a deep copy of a slice.
func cloneSlice(src []any) []any {
	if src == nil {
		return nil
	}

	dst := make([]any, len(src))
	for i, val := range src {
		switch v := val.(type) {
		case map[string]any:
			dst[i] = Clone(v)
		case []any:
			dst[i] = cloneSlice(v)
		default:
			dst[i] = val
		}
	}

	return dst
}
