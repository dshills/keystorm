package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceFile represents a .code-workspace file format.
// This is compatible with VS Code workspace files.
type WorkspaceFile struct {
	// Folders is the list of workspace folders.
	Folders []WorkspaceFolderEntry `json:"folders"`

	// Settings contains workspace-level settings.
	Settings map[string]any `json:"settings,omitempty"`

	// Extensions contains extension recommendations.
	Extensions *ExtensionRecommendations `json:"extensions,omitempty"`

	// Launch contains debug launch configurations.
	Launch map[string]any `json:"launch,omitempty"`

	// Tasks contains task configurations.
	Tasks map[string]any `json:"tasks,omitempty"`
}

// WorkspaceFolderEntry represents a folder entry in a workspace file.
type WorkspaceFolderEntry struct {
	// Path is the folder path (relative or absolute).
	Path string `json:"path"`

	// Name is an optional display name for the folder.
	Name string `json:"name,omitempty"`
}

// ExtensionRecommendations holds extension recommendations.
type ExtensionRecommendations struct {
	// Recommendations are extension IDs to recommend.
	Recommendations []string `json:"recommendations,omitempty"`

	// UnwantedRecommendations are extension IDs to not recommend.
	UnwantedRecommendations []string `json:"unwantedRecommendations,omitempty"`
}

// LoadWorkspaceFile loads a .code-workspace file.
func LoadWorkspaceFile(path string) (*WorkspaceFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wsFile WorkspaceFile
	if err := json.Unmarshal(data, &wsFile); err != nil {
		return nil, err
	}

	return &wsFile, nil
}

// SaveWorkspaceFile saves a .code-workspace file.
func SaveWorkspaceFile(path string, wsFile *WorkspaceFile) error {
	data, err := json.MarshalIndent(wsFile, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// OpenFromWorkspaceFile creates a Workspace from a .code-workspace file.
func OpenFromWorkspaceFile(workspaceFilePath string) (*Workspace, error) {
	wsFile, err := LoadWorkspaceFile(workspaceFilePath)
	if err != nil {
		return nil, err
	}

	// Get the directory containing the workspace file for relative path resolution
	baseDir := filepath.Dir(workspaceFilePath)

	// Convert folder entries to absolute paths
	paths := make([]string, 0, len(wsFile.Folders))
	for _, entry := range wsFile.Folders {
		folderPath := entry.Path

		// Resolve relative paths
		if !filepath.IsAbs(folderPath) {
			folderPath = filepath.Join(baseDir, folderPath)
		}

		// Clean the path
		folderPath = filepath.Clean(folderPath)
		paths = append(paths, folderPath)
	}

	// Create workspace
	ws, err := NewFromPaths(paths...)
	if err != nil {
		return nil, err
	}

	// Apply folder names from workspace file
	for i, entry := range wsFile.Folders {
		if entry.Name != "" && i < len(ws.folders) {
			ws.folders[i].Name = entry.Name
		}
	}

	// Convert workspace settings to config if present
	if wsFile.Settings != nil {
		config := ws.Config()
		applySettingsToConfig(config, wsFile.Settings)
		ws.SetConfig(config)
	}

	return ws, nil
}

// SaveToWorkspaceFile saves a Workspace to a .code-workspace file.
func (w *Workspace) SaveToWorkspaceFile(workspaceFilePath string) error {
	// Copy data while holding lock, then release before I/O
	w.mu.RLock()
	folders := make([]Folder, len(w.folders))
	copy(folders, w.folders)
	config := w.config
	w.mu.RUnlock()

	baseDir := filepath.Dir(workspaceFilePath)

	// Convert folders to workspace file entries
	entries := make([]WorkspaceFolderEntry, len(folders))
	for i, folder := range folders {
		// Try to make path relative to workspace file
		relPath, err := filepath.Rel(baseDir, folder.Path)
		if err != nil {
			relPath = folder.Path
		}

		entries[i] = WorkspaceFolderEntry{
			Path: filepath.ToSlash(relPath), // Use forward slashes for cross-platform
			Name: folder.Name,
		}
	}

	wsFile := &WorkspaceFile{
		Folders:  entries,
		Settings: configToSettings(config),
	}

	return SaveWorkspaceFile(workspaceFilePath, wsFile)
}

// applySettingsToConfig applies workspace settings to a Config.
func applySettingsToConfig(config *Config, settings map[string]any) {
	if config == nil || settings == nil {
		return
	}

	// files.exclude
	if exclude, ok := settings["files.exclude"].(map[string]any); ok {
		for pattern, enabled := range exclude {
			if enabled == true {
				config.ExcludePatterns = appendIfNotExists(config.ExcludePatterns, pattern)
			}
		}
	}

	// search.exclude
	if exclude, ok := settings["search.exclude"].(map[string]any); ok {
		for pattern, enabled := range exclude {
			if enabled == true {
				config.SearchExcludePatterns = appendIfNotExists(config.SearchExcludePatterns, pattern)
			}
		}
	}

	// files.watcherExclude
	if exclude, ok := settings["files.watcherExclude"].(map[string]any); ok {
		for pattern, enabled := range exclude {
			if enabled == true {
				config.WatcherExcludePatterns = appendIfNotExists(config.WatcherExcludePatterns, pattern)
			}
		}
	}

	// editor.tabSize
	if tabSize, ok := settings["editor.tabSize"].(float64); ok {
		config.EditorSettings.TabSize = int(tabSize)
	}

	// editor.insertSpaces
	if insertSpaces, ok := settings["editor.insertSpaces"].(bool); ok {
		config.EditorSettings.InsertSpaces = insertSpaces
	}

	// files.trimTrailingWhitespace
	if trim, ok := settings["files.trimTrailingWhitespace"].(bool); ok {
		config.EditorSettings.TrimTrailingWhitespace = trim
	}

	// files.insertFinalNewline
	if insert, ok := settings["files.insertFinalNewline"].(bool); ok {
		config.EditorSettings.InsertFinalNewline = insert
	}

	// files.encoding
	if encoding, ok := settings["files.encoding"].(string); ok {
		config.EditorSettings.DefaultEncoding = encoding
	}

	// files.eol
	if eol, ok := settings["files.eol"].(string); ok {
		switch eol {
		case "\n":
			config.EditorSettings.DefaultLineEnding = "lf"
		case "\r\n":
			config.EditorSettings.DefaultLineEnding = "crlf"
		case "auto":
			config.EditorSettings.DefaultLineEnding = "auto"
		}
	}

	// files.associations
	if assoc, ok := settings["files.associations"].(map[string]any); ok {
		if config.FileAssociations == nil {
			config.FileAssociations = make(map[string]string)
		}
		for pattern, lang := range assoc {
			if langStr, ok := lang.(string); ok {
				config.FileAssociations[pattern] = langStr
			}
		}
	}
}

// configToSettings converts a Config to workspace settings.
func configToSettings(config *Config) map[string]any {
	if config == nil {
		return nil
	}

	settings := make(map[string]any)

	// files.exclude
	if len(config.ExcludePatterns) > 0 {
		exclude := make(map[string]any)
		for _, pattern := range config.ExcludePatterns {
			exclude[pattern] = true
		}
		settings["files.exclude"] = exclude
	}

	// search.exclude
	if len(config.SearchExcludePatterns) > 0 {
		exclude := make(map[string]any)
		for _, pattern := range config.SearchExcludePatterns {
			exclude[pattern] = true
		}
		settings["search.exclude"] = exclude
	}

	// files.watcherExclude
	if len(config.WatcherExcludePatterns) > 0 {
		exclude := make(map[string]any)
		for _, pattern := range config.WatcherExcludePatterns {
			exclude[pattern] = true
		}
		settings["files.watcherExclude"] = exclude
	}

	// Editor settings
	if config.EditorSettings.TabSize > 0 {
		settings["editor.tabSize"] = config.EditorSettings.TabSize
	}
	settings["editor.insertSpaces"] = config.EditorSettings.InsertSpaces
	settings["files.trimTrailingWhitespace"] = config.EditorSettings.TrimTrailingWhitespace
	settings["files.insertFinalNewline"] = config.EditorSettings.InsertFinalNewline

	if config.EditorSettings.DefaultEncoding != "" {
		settings["files.encoding"] = config.EditorSettings.DefaultEncoding
	}

	switch config.EditorSettings.DefaultLineEnding {
	case "lf":
		settings["files.eol"] = "\n"
	case "crlf":
		settings["files.eol"] = "\r\n"
	}

	// File associations
	if len(config.FileAssociations) > 0 {
		settings["files.associations"] = config.FileAssociations
	}

	return settings
}

// appendIfNotExists appends s to slice if not already present.
func appendIfNotExists(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// MultiRootManager provides utilities for managing multi-root workspaces.
type MultiRootManager struct {
	workspace *Workspace
}

// NewMultiRootManager creates a new multi-root manager for a workspace.
func NewMultiRootManager(ws *Workspace) *MultiRootManager {
	return &MultiRootManager{workspace: ws}
}

// FindCommonRoot finds the common parent directory of all workspace folders.
// Returns empty string if no common root exists.
func (m *MultiRootManager) FindCommonRoot() string {
	folders := m.workspace.Folders()
	if len(folders) == 0 {
		return ""
	}
	if len(folders) == 1 {
		return folders[0].Path
	}

	// Check if all paths are absolute (they should be)
	isAbsolute := filepath.IsAbs(folders[0].Path)

	// Split all paths into components
	var allParts [][]string
	for _, folder := range folders {
		parts := splitPathComponents(folder.Path)
		allParts = append(allParts, parts)
	}

	// Find common prefix
	var commonParts []string
	for i := 0; ; i++ {
		if i >= len(allParts[0]) {
			break
		}

		part := allParts[0][i]
		allMatch := true
		for _, parts := range allParts[1:] {
			if i >= len(parts) || parts[i] != part {
				allMatch = false
				break
			}
		}

		if !allMatch {
			break
		}
		commonParts = append(commonParts, part)
	}

	if len(commonParts) == 0 {
		return ""
	}

	// Join back into path, preserving absolute path prefix
	result := filepath.Join(commonParts...)
	if isAbsolute && !filepath.IsAbs(result) {
		result = string(filepath.Separator) + result
	}
	return result
}

// GetRelativePaths returns all folder paths relative to the common root.
func (m *MultiRootManager) GetRelativePaths() map[string]string {
	commonRoot := m.FindCommonRoot()
	if commonRoot == "" {
		return nil
	}

	result := make(map[string]string)
	for _, folder := range m.workspace.Folders() {
		relPath, err := filepath.Rel(commonRoot, folder.Path)
		if err != nil {
			relPath = folder.Path
		}
		result[folder.Path] = relPath
	}

	return result
}

// ReorderFolders reorders workspace folders by their paths.
func (m *MultiRootManager) ReorderFolders(paths []string) error {
	m.workspace.mu.Lock()
	defer m.workspace.mu.Unlock()

	// Build map of existing folders
	folderMap := make(map[string]Folder)
	for _, f := range m.workspace.folders {
		folderMap[f.Path] = f
	}

	// Reorder based on paths
	newFolders := make([]Folder, 0, len(paths))
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		folder, ok := folderMap[absPath]
		if !ok {
			return ErrFolderNotFound
		}
		newFolders = append(newFolders, folder)
	}

	m.workspace.folders = newFolders
	return nil
}

// SetPrimaryFolder sets a folder as the primary (first) folder.
func (m *MultiRootManager) SetPrimaryFolder(path string) error {
	m.workspace.mu.Lock()
	defer m.workspace.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Find the folder
	idx := -1
	for i, f := range m.workspace.folders {
		if f.Path == absPath {
			idx = i
			break
		}
	}

	if idx == -1 {
		return ErrFolderNotFound
	}

	if idx == 0 {
		return nil // Already primary
	}

	// Move to front
	folder := m.workspace.folders[idx]
	newFolders := make([]Folder, len(m.workspace.folders))
	newFolders[0] = folder
	copy(newFolders[1:idx+1], m.workspace.folders[:idx])
	copy(newFolders[idx+1:], m.workspace.folders[idx+1:])
	m.workspace.folders = newFolders

	return nil
}

// RenameFolder sets a custom display name for a folder.
func (m *MultiRootManager) RenameFolder(path, name string) error {
	m.workspace.mu.Lock()
	defer m.workspace.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	for i, f := range m.workspace.folders {
		if f.Path == absPath {
			m.workspace.folders[i].Name = name
			return nil
		}
	}

	return ErrFolderNotFound
}

// GetFoldersByName returns folders matching a name pattern.
func (m *MultiRootManager) GetFoldersByName(pattern string) []Folder {
	m.workspace.mu.RLock()
	defer m.workspace.mu.RUnlock()

	var matches []Folder
	for _, f := range m.workspace.folders {
		if matched, _ := filepath.Match(pattern, f.Name); matched {
			matches = append(matches, f)
		}
	}
	return matches
}

// splitPathComponents splits a path into its components (directory names only).
// For "/home/user/project" returns ["home", "user", "project"].
// Does not include root markers - callers should use filepath.IsAbs if needed.
func splitPathComponents(path string) []string {
	path = filepath.Clean(path)

	var parts []string
	for path != "" && path != "." && path != "/" {
		dir, base := filepath.Split(path)
		if base != "" {
			parts = append([]string{base}, parts...)
		}
		path = strings.TrimSuffix(dir, string(filepath.Separator))
	}

	return parts
}
