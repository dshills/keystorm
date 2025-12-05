package lsp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DiagnosticsService provides high-level diagnostic management with
// aggregation, filtering, severity tracking, and change notifications.
type DiagnosticsService struct {
	mu      sync.RWMutex
	manager *Manager

	// Per-file diagnostics
	diagnostics map[DocumentURI]*FileDiagnostics

	// Configuration
	minSeverity      DiagnosticSeverity
	debounceDelay    time.Duration
	maxPerFile       int
	enabledSources   map[string]bool // nil means all enabled
	codeActionsCache map[diagnosticKey][]CodeAction

	// Callbacks
	onChange func(uri DocumentURI, diagnostics []Diagnostic)

	// Debouncing with version tracking to avoid stale callbacks
	pendingNotify     map[DocumentURI]*time.Timer
	notifyVersions    map[DocumentURI]int64 // Version per URI for callback validation
	nextNotifyVersion int64
}

// FileDiagnostics holds diagnostics for a single file with metadata.
type FileDiagnostics struct {
	URI         DocumentURI
	Path        string
	Diagnostics []Diagnostic
	UpdatedAt   time.Time
	Version     int

	// Aggregated counts by severity
	ErrorCount   int
	WarningCount int
	InfoCount    int
	HintCount    int
}

// diagnosticKey uniquely identifies a diagnostic for caching.
type diagnosticKey struct {
	uri     DocumentURI
	line    int
	char    int
	message string
}

// DiagnosticsServiceOption configures the diagnostics service.
type DiagnosticsServiceOption func(*DiagnosticsService)

// WithMinSeverity sets the minimum severity to include.
func WithMinSeverity(severity DiagnosticSeverity) DiagnosticsServiceOption {
	return func(ds *DiagnosticsService) {
		ds.minSeverity = severity
	}
}

// WithDiagnosticsDebounce sets the debounce delay for change notifications.
func WithDiagnosticsDebounce(d time.Duration) DiagnosticsServiceOption {
	return func(ds *DiagnosticsService) {
		ds.debounceDelay = d
	}
}

// WithMaxDiagnosticsPerFile limits diagnostics per file.
func WithMaxDiagnosticsPerFile(max int) DiagnosticsServiceOption {
	return func(ds *DiagnosticsService) {
		ds.maxPerFile = max
	}
}

// WithDiagnosticsChangeHandler sets a callback for diagnostic changes.
func WithDiagnosticsChangeHandler(handler func(uri DocumentURI, diagnostics []Diagnostic)) DiagnosticsServiceOption {
	return func(ds *DiagnosticsService) {
		ds.onChange = handler
	}
}

// WithEnabledSources limits diagnostics to specific sources.
func WithEnabledSources(sources []string) DiagnosticsServiceOption {
	return func(ds *DiagnosticsService) {
		ds.enabledSources = make(map[string]bool, len(sources))
		for _, s := range sources {
			ds.enabledSources[s] = true
		}
	}
}

// NewDiagnosticsService creates a new diagnostics service.
func NewDiagnosticsService(mgr *Manager, opts ...DiagnosticsServiceOption) *DiagnosticsService {
	ds := &DiagnosticsService{
		manager:          mgr,
		diagnostics:      make(map[DocumentURI]*FileDiagnostics),
		minSeverity:      DiagnosticSeverityHint, // Include all by default
		debounceDelay:    100 * time.Millisecond,
		maxPerFile:       1000,
		codeActionsCache: make(map[diagnosticKey][]CodeAction),
		pendingNotify:    make(map[DocumentURI]*time.Timer),
		notifyVersions:   make(map[DocumentURI]int64),
	}

	for _, opt := range opts {
		opt(ds)
	}

	// Register with manager for diagnostic updates
	if mgr != nil {
		mgr.mu.Lock()
		mgr.diagnosticsCb = ds.handleDiagnostics
		mgr.mu.Unlock()
	}

	return ds
}

// handleDiagnostics processes incoming diagnostics from the LSP server.
func (ds *DiagnosticsService) handleDiagnostics(uri DocumentURI, diagnostics []Diagnostic) {
	ds.mu.Lock()

	// Filter by severity and source
	filtered := ds.filterDiagnostics(diagnostics)

	// Sort by position
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Range.Start.Line != filtered[j].Range.Start.Line {
			return filtered[i].Range.Start.Line < filtered[j].Range.Start.Line
		}
		return filtered[i].Range.Start.Character < filtered[j].Range.Start.Character
	})

	// Limit per file
	if ds.maxPerFile > 0 && len(filtered) > ds.maxPerFile {
		filtered = filtered[:ds.maxPerFile]
	}

	// Update storage
	path := URIToFilePath(uri)
	fd := &FileDiagnostics{
		URI:         uri,
		Path:        path,
		Diagnostics: filtered,
		UpdatedAt:   time.Now(),
	}

	// Count by severity
	for _, d := range filtered {
		switch d.Severity {
		case DiagnosticSeverityError:
			fd.ErrorCount++
		case DiagnosticSeverityWarning:
			fd.WarningCount++
		case DiagnosticSeverityInformation:
			fd.InfoCount++
		case DiagnosticSeverityHint:
			fd.HintCount++
		}
	}

	if len(filtered) == 0 {
		delete(ds.diagnostics, uri)
	} else {
		if existing, ok := ds.diagnostics[uri]; ok {
			fd.Version = existing.Version + 1
		}
		ds.diagnostics[uri] = fd
	}

	// Invalidate code actions cache for this file
	for key := range ds.codeActionsCache {
		if key.uri == uri {
			delete(ds.codeActionsCache, key)
		}
	}

	// Schedule debounced notification with version tracking
	if ds.onChange != nil {
		if timer, ok := ds.pendingNotify[uri]; ok {
			timer.Stop()
		}

		// Increment version for this URI to invalidate stale callbacks
		ds.nextNotifyVersion++
		version := ds.nextNotifyVersion
		ds.notifyVersions[uri] = version

		// Capture filtered for callback
		filteredCopy := make([]Diagnostic, len(filtered))
		copy(filteredCopy, filtered)

		ds.pendingNotify[uri] = time.AfterFunc(ds.debounceDelay, func() {
			ds.mu.Lock()
			// Check if this callback is still valid (version matches)
			currentVersion := ds.notifyVersions[uri]
			if currentVersion != version {
				ds.mu.Unlock()
				return // Stale callback, newer update superseded this one
			}
			delete(ds.pendingNotify, uri)
			handler := ds.onChange
			ds.mu.Unlock()

			if handler != nil {
				handler(uri, filteredCopy)
			}
		})
	}

	ds.mu.Unlock()
}

// filterDiagnostics filters diagnostics by severity and source.
func (ds *DiagnosticsService) filterDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	if len(diagnostics) == 0 {
		return nil
	}

	var filtered []Diagnostic
	for _, d := range diagnostics {
		// Filter by severity (lower number = higher severity)
		if d.Severity > ds.minSeverity {
			continue
		}

		// Filter by source if enabled
		if ds.enabledSources != nil && d.Source != "" {
			if !ds.enabledSources[d.Source] {
				continue
			}
		}

		filtered = append(filtered, d)
	}

	return filtered
}

// GetDiagnostics returns diagnostics for a file.
func (ds *DiagnosticsService) GetDiagnostics(path string) []Diagnostic {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok {
		return nil
	}

	// Return a copy
	result := make([]Diagnostic, len(fd.Diagnostics))
	copy(result, fd.Diagnostics)
	return result
}

// GetFileDiagnostics returns full diagnostic info for a file.
func (ds *DiagnosticsService) GetFileDiagnostics(path string) (*FileDiagnostics, bool) {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok {
		return nil, false
	}

	// Return a copy
	result := &FileDiagnostics{
		URI:          fd.URI,
		Path:         fd.Path,
		Diagnostics:  make([]Diagnostic, len(fd.Diagnostics)),
		UpdatedAt:    fd.UpdatedAt,
		Version:      fd.Version,
		ErrorCount:   fd.ErrorCount,
		WarningCount: fd.WarningCount,
		InfoCount:    fd.InfoCount,
		HintCount:    fd.HintCount,
	}
	copy(result.Diagnostics, fd.Diagnostics)
	return result, true
}

// GetDiagnosticsAtLine returns diagnostics at a specific line.
func (ds *DiagnosticsService) GetDiagnosticsAtLine(path string, line int) []Diagnostic {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok {
		return nil
	}

	var result []Diagnostic
	for _, d := range fd.Diagnostics {
		if d.Range.Start.Line <= line && d.Range.End.Line >= line {
			result = append(result, d)
		}
	}
	return result
}

// GetDiagnosticsAtPosition returns diagnostics at a specific position.
func (ds *DiagnosticsService) GetDiagnosticsAtPosition(path string, pos Position) []Diagnostic {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok {
		return nil
	}

	var result []Diagnostic
	for _, d := range fd.Diagnostics {
		if positionInRange(pos, d.Range) {
			result = append(result, d)
		}
	}
	return result
}

// GetDiagnosticsBySeverity returns diagnostics filtered by severity.
func (ds *DiagnosticsService) GetDiagnosticsBySeverity(path string, severity DiagnosticSeverity) []Diagnostic {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok {
		return nil
	}

	var result []Diagnostic
	for _, d := range fd.Diagnostics {
		if d.Severity == severity {
			result = append(result, d)
		}
	}
	return result
}

// GetErrors returns only error-level diagnostics for a file.
func (ds *DiagnosticsService) GetErrors(path string) []Diagnostic {
	return ds.GetDiagnosticsBySeverity(path, DiagnosticSeverityError)
}

// GetWarnings returns only warning-level diagnostics for a file.
func (ds *DiagnosticsService) GetWarnings(path string) []Diagnostic {
	return ds.GetDiagnosticsBySeverity(path, DiagnosticSeverityWarning)
}

// AllDiagnostics returns diagnostics for all files.
func (ds *DiagnosticsService) AllDiagnostics() map[string][]Diagnostic {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	result := make(map[string][]Diagnostic, len(ds.diagnostics))
	for _, fd := range ds.diagnostics {
		diags := make([]Diagnostic, len(fd.Diagnostics))
		copy(diags, fd.Diagnostics)
		result[fd.Path] = diags
	}
	return result
}

// AllFileDiagnostics returns full diagnostic info for all files.
func (ds *DiagnosticsService) AllFileDiagnostics() []*FileDiagnostics {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	result := make([]*FileDiagnostics, 0, len(ds.diagnostics))
	for _, fd := range ds.diagnostics {
		fdCopy := &FileDiagnostics{
			URI:          fd.URI,
			Path:         fd.Path,
			Diagnostics:  make([]Diagnostic, len(fd.Diagnostics)),
			UpdatedAt:    fd.UpdatedAt,
			Version:      fd.Version,
			ErrorCount:   fd.ErrorCount,
			WarningCount: fd.WarningCount,
			InfoCount:    fd.InfoCount,
			HintCount:    fd.HintCount,
		}
		copy(fdCopy.Diagnostics, fd.Diagnostics)
		result = append(result, fdCopy)
	}
	return result
}

// FilesWithDiagnostics returns paths of files that have diagnostics.
func (ds *DiagnosticsService) FilesWithDiagnostics() []string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	paths := make([]string, 0, len(ds.diagnostics))
	for _, fd := range ds.diagnostics {
		paths = append(paths, fd.Path)
	}
	return paths
}

// FilesWithErrors returns paths of files that have errors.
func (ds *DiagnosticsService) FilesWithErrors() []string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var paths []string
	for _, fd := range ds.diagnostics {
		if fd.ErrorCount > 0 {
			paths = append(paths, fd.Path)
		}
	}
	return paths
}

// HasDiagnostics returns true if a file has any diagnostics.
func (ds *DiagnosticsService) HasDiagnostics(path string) bool {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	_, ok := ds.diagnostics[uri]
	return ok
}

// HasErrors returns true if a file has any errors.
func (ds *DiagnosticsService) HasErrors(path string) bool {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	return ok && fd.ErrorCount > 0
}

// DiagnosticSummary provides an overview of all diagnostics.
type DiagnosticSummary struct {
	TotalFiles   int
	TotalErrors  int
	TotalWarns   int
	TotalInfos   int
	TotalHints   int
	FilesWithErr int
}

// Summary returns an overall diagnostic summary.
func (ds *DiagnosticsService) Summary() DiagnosticSummary {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	summary := DiagnosticSummary{
		TotalFiles: len(ds.diagnostics),
	}

	for _, fd := range ds.diagnostics {
		summary.TotalErrors += fd.ErrorCount
		summary.TotalWarns += fd.WarningCount
		summary.TotalInfos += fd.InfoCount
		summary.TotalHints += fd.HintCount
		if fd.ErrorCount > 0 {
			summary.FilesWithErr++
		}
	}

	return summary
}

// Clear removes all diagnostics.
func (ds *DiagnosticsService) Clear() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.diagnostics = make(map[DocumentURI]*FileDiagnostics)
	ds.codeActionsCache = make(map[diagnosticKey][]CodeAction)

	// Cancel pending notifications and reset version tracking
	for _, timer := range ds.pendingNotify {
		timer.Stop()
	}
	ds.pendingNotify = make(map[DocumentURI]*time.Timer)
	ds.notifyVersions = make(map[DocumentURI]int64)
}

// ClearFile removes diagnostics for a specific file.
func (ds *DiagnosticsService) ClearFile(path string) {
	uri := FilePathToURI(path)

	ds.mu.Lock()
	defer ds.mu.Unlock()

	delete(ds.diagnostics, uri)

	// Clear cache entries for this file
	for key := range ds.codeActionsCache {
		if key.uri == uri {
			delete(ds.codeActionsCache, key)
		}
	}

	// Cancel pending notification
	if timer, ok := ds.pendingNotify[uri]; ok {
		timer.Stop()
		delete(ds.pendingNotify, uri)
	}
}

// SetMinSeverity updates the minimum severity filter.
func (ds *DiagnosticsService) SetMinSeverity(severity DiagnosticSeverity) {
	ds.mu.Lock()
	ds.minSeverity = severity
	ds.mu.Unlock()
}

// positionInRange checks if a position is within a range.
func positionInRange(pos Position, rng Range) bool {
	// Before range start
	if pos.Line < rng.Start.Line {
		return false
	}
	if pos.Line == rng.Start.Line && pos.Character < rng.Start.Character {
		return false
	}

	// After range end
	if pos.Line > rng.End.Line {
		return false
	}
	if pos.Line == rng.End.Line && pos.Character > rng.End.Character {
		return false
	}

	return true
}

// DiagnosticSeverityString returns a human-readable severity name.
func DiagnosticSeverityString(severity DiagnosticSeverity) string {
	switch severity {
	case DiagnosticSeverityError:
		return "Error"
	case DiagnosticSeverityWarning:
		return "Warning"
	case DiagnosticSeverityInformation:
		return "Information"
	case DiagnosticSeverityHint:
		return "Hint"
	default:
		return "Unknown"
	}
}

// DiagnosticSeverityIcon returns a single character icon for severity.
func DiagnosticSeverityIcon(severity DiagnosticSeverity) string {
	switch severity {
	case DiagnosticSeverityError:
		return "E"
	case DiagnosticSeverityWarning:
		return "W"
	case DiagnosticSeverityInformation:
		return "I"
	case DiagnosticSeverityHint:
		return "H"
	default:
		return "?"
	}
}

// FormatDiagnostic formats a diagnostic for display.
func FormatDiagnostic(d Diagnostic) string {
	var sb strings.Builder

	// Severity icon
	sb.WriteString(DiagnosticSeverityIcon(d.Severity))
	sb.WriteString(" ")

	if d.Source != "" {
		sb.WriteString("[")
		sb.WriteString(d.Source)
		sb.WriteString("] ")
	}

	sb.WriteString(d.Message)

	if d.Code != nil {
		sb.WriteString(" (")
		switch v := d.Code.(type) {
		case string:
			sb.WriteString(v)
		case float64:
			// Format without trailing zeros for whole numbers
			sb.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		case int:
			sb.WriteString(strconv.Itoa(v))
		default:
			sb.WriteString(fmt.Sprint(v))
		}
		sb.WriteString(")")
	}

	return sb.String()
}

// FormatDiagnosticWithLocation formats a diagnostic with file location.
func FormatDiagnosticWithLocation(path string, d Diagnostic) string {
	return fmt.Sprintf("%s:%d:%d: %s",
		path,
		d.Range.Start.Line+1,      // Convert to 1-based
		d.Range.Start.Character+1, // Convert to 1-based
		FormatDiagnostic(d),
	)
}

// GroupDiagnosticsByFile groups diagnostics by file path.
func GroupDiagnosticsByFile(diagnostics map[string][]Diagnostic) []struct {
	Path        string
	Diagnostics []Diagnostic
} {
	var result []struct {
		Path        string
		Diagnostics []Diagnostic
	}

	for path, diags := range diagnostics {
		result = append(result, struct {
			Path        string
			Diagnostics []Diagnostic
		}{
			Path:        path,
			Diagnostics: diags,
		})
	}

	// Sort by path
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result
}

// FilterDiagnosticsByPattern filters diagnostics by message or source pattern.
func FilterDiagnosticsByPattern(diagnostics []Diagnostic, pattern string) []Diagnostic {
	if pattern == "" {
		return diagnostics
	}

	patternLower := strings.ToLower(pattern)
	var result []Diagnostic
	for _, d := range diagnostics {
		// Check message or source (avoid duplicates by using single condition)
		messageMatch := strings.Contains(strings.ToLower(d.Message), patternLower)
		sourceMatch := d.Source != "" && strings.Contains(strings.ToLower(d.Source), patternLower)
		if messageMatch || sourceMatch {
			result = append(result, d)
		}
	}
	return result
}

// SortDiagnosticsBySeverity sorts diagnostics with errors first.
func SortDiagnosticsBySeverity(diagnostics []Diagnostic) []Diagnostic {
	sorted := make([]Diagnostic, len(diagnostics))
	copy(sorted, diagnostics)

	sort.SliceStable(sorted, func(i, j int) bool {
		// Lower severity number = higher priority (Error=1, Warning=2, etc.)
		if sorted[i].Severity != sorted[j].Severity {
			return sorted[i].Severity < sorted[j].Severity
		}
		// Then by line
		if sorted[i].Range.Start.Line != sorted[j].Range.Start.Line {
			return sorted[i].Range.Start.Line < sorted[j].Range.Start.Line
		}
		// Then by column
		return sorted[i].Range.Start.Character < sorted[j].Range.Start.Character
	})

	return sorted
}

// NextDiagnostic returns the next diagnostic after a position.
func (ds *DiagnosticsService) NextDiagnostic(path string, pos Position, wrapAround bool) *Diagnostic {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok || len(fd.Diagnostics) == 0 {
		return nil
	}

	// Find next diagnostic after position
	for i := range fd.Diagnostics {
		d := &fd.Diagnostics[i]
		if d.Range.Start.Line > pos.Line ||
			(d.Range.Start.Line == pos.Line && d.Range.Start.Character > pos.Character) {
			return d
		}
	}

	// Wrap around to first if enabled
	if wrapAround && len(fd.Diagnostics) > 0 {
		return &fd.Diagnostics[0]
	}

	return nil
}

// PrevDiagnostic returns the previous diagnostic before a position.
func (ds *DiagnosticsService) PrevDiagnostic(path string, pos Position, wrapAround bool) *Diagnostic {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok || len(fd.Diagnostics) == 0 {
		return nil
	}

	// Find previous diagnostic before position (iterate in reverse)
	for i := len(fd.Diagnostics) - 1; i >= 0; i-- {
		d := &fd.Diagnostics[i]
		if d.Range.Start.Line < pos.Line ||
			(d.Range.Start.Line == pos.Line && d.Range.Start.Character < pos.Character) {
			return d
		}
	}

	// Wrap around to last if enabled
	if wrapAround && len(fd.Diagnostics) > 0 {
		return &fd.Diagnostics[len(fd.Diagnostics)-1]
	}

	return nil
}

// NextError returns the next error after a position.
func (ds *DiagnosticsService) NextError(path string, pos Position, wrapAround bool) *Diagnostic {
	uri := FilePathToURI(path)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	fd, ok := ds.diagnostics[uri]
	if !ok {
		return nil
	}

	firstErrorIdx := -1

	for i := range fd.Diagnostics {
		d := &fd.Diagnostics[i]
		if d.Severity != DiagnosticSeverityError {
			continue
		}

		if firstErrorIdx < 0 {
			firstErrorIdx = i
		}

		if d.Range.Start.Line > pos.Line ||
			(d.Range.Start.Line == pos.Line && d.Range.Start.Character > pos.Character) {
			return d
		}
	}

	if wrapAround && firstErrorIdx >= 0 {
		return &fd.Diagnostics[firstErrorIdx]
	}

	return nil
}
