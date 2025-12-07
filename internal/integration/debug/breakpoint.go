package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dshills/keystorm/internal/integration/debug/dap"
)

// BreakpointType represents the type of breakpoint.
type BreakpointType int

const (
	// BreakpointTypeLine is a standard line breakpoint.
	BreakpointTypeLine BreakpointType = iota
	// BreakpointTypeConditional is a breakpoint with a condition.
	BreakpointTypeConditional
	// BreakpointTypeLogPoint is a log point (prints message without stopping).
	BreakpointTypeLogPoint
	// BreakpointTypeFunction is a function breakpoint.
	BreakpointTypeFunction
	// BreakpointTypeData is a data/watchpoint breakpoint.
	BreakpointTypeData
)

// String returns a string representation of the breakpoint type.
func (t BreakpointType) String() string {
	switch t {
	case BreakpointTypeLine:
		return "line"
	case BreakpointTypeConditional:
		return "conditional"
	case BreakpointTypeLogPoint:
		return "logpoint"
	case BreakpointTypeFunction:
		return "function"
	case BreakpointTypeData:
		return "data"
	default:
		return "unknown"
	}
}

// Breakpoint represents a user-defined breakpoint.
type Breakpoint struct {
	// ID is a unique identifier for this breakpoint.
	ID int `json:"id"`

	// Type is the breakpoint type.
	Type BreakpointType `json:"type"`

	// Path is the source file path (for line breakpoints).
	Path string `json:"path,omitempty"`

	// Line is the line number (1-based).
	Line int `json:"line,omitempty"`

	// Column is the column number (1-based, optional).
	Column int `json:"column,omitempty"`

	// Condition is the condition expression (for conditional breakpoints).
	Condition string `json:"condition,omitempty"`

	// HitCondition is the hit count condition.
	HitCondition string `json:"hitCondition,omitempty"`

	// LogMessage is the message to log (for log points).
	LogMessage string `json:"logMessage,omitempty"`

	// FunctionName is the function name (for function breakpoints).
	FunctionName string `json:"functionName,omitempty"`

	// DataID is the data expression (for data breakpoints).
	DataID string `json:"dataId,omitempty"`

	// AccessType is "read", "write", or "readWrite" (for data breakpoints).
	AccessType string `json:"accessType,omitempty"`

	// Enabled indicates if the breakpoint is enabled.
	Enabled bool `json:"enabled"`

	// Verified indicates if the adapter confirmed the breakpoint.
	Verified bool `json:"verified"`

	// Message contains any message from the adapter.
	Message string `json:"message,omitempty"`

	// ActualLine is the actual line where the breakpoint was set.
	ActualLine int `json:"actualLine,omitempty"`

	// HitCount is the number of times this breakpoint has been hit.
	HitCount int `json:"hitCount"`
}

// BreakpointManager manages breakpoints for a debug session.
type BreakpointManager struct {
	session *Session
	mu      sync.RWMutex

	// All breakpoints by ID
	breakpoints map[int]*Breakpoint

	// Breakpoints grouped by file path
	byPath map[string][]*Breakpoint

	// Function breakpoints
	functionBreakpoints []*Breakpoint

	// Data breakpoints
	dataBreakpoints []*Breakpoint

	// Next breakpoint ID
	nextID int

	// Persistence file path
	persistPath string
}

// NewBreakpointManager creates a new breakpoint manager.
func NewBreakpointManager(session *Session) *BreakpointManager {
	return &BreakpointManager{
		session:     session,
		breakpoints: make(map[int]*Breakpoint),
		byPath:      make(map[string][]*Breakpoint),
		nextID:      1,
	}
}

// SetPersistPath sets the file path for breakpoint persistence.
func (m *BreakpointManager) SetPersistPath(path string) {
	m.mu.Lock()
	m.persistPath = path
	m.mu.Unlock()
}

// allocateID allocates a new breakpoint ID.
func (m *BreakpointManager) allocateID() int {
	id := m.nextID
	m.nextID++
	return id
}

// AddLineBreakpoint adds a line breakpoint.
func (m *BreakpointManager) AddLineBreakpoint(path string, line int) (*Breakpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp := &Breakpoint{
		ID:      m.allocateID(),
		Type:    BreakpointTypeLine,
		Path:    path,
		Line:    line,
		Enabled: true,
	}

	m.breakpoints[bp.ID] = bp
	m.byPath[path] = append(m.byPath[path], bp)

	return bp, nil
}

// AddConditionalBreakpoint adds a conditional breakpoint.
func (m *BreakpointManager) AddConditionalBreakpoint(path string, line int, condition string) (*Breakpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp := &Breakpoint{
		ID:        m.allocateID(),
		Type:      BreakpointTypeConditional,
		Path:      path,
		Line:      line,
		Condition: condition,
		Enabled:   true,
	}

	m.breakpoints[bp.ID] = bp
	m.byPath[path] = append(m.byPath[path], bp)

	return bp, nil
}

// AddLogPoint adds a log point.
func (m *BreakpointManager) AddLogPoint(path string, line int, logMessage string) (*Breakpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp := &Breakpoint{
		ID:         m.allocateID(),
		Type:       BreakpointTypeLogPoint,
		Path:       path,
		Line:       line,
		LogMessage: logMessage,
		Enabled:    true,
	}

	m.breakpoints[bp.ID] = bp
	m.byPath[path] = append(m.byPath[path], bp)

	return bp, nil
}

// AddFunctionBreakpoint adds a function breakpoint.
func (m *BreakpointManager) AddFunctionBreakpoint(functionName string, condition string) (*Breakpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp := &Breakpoint{
		ID:           m.allocateID(),
		Type:         BreakpointTypeFunction,
		FunctionName: functionName,
		Condition:    condition,
		Enabled:      true,
	}

	m.breakpoints[bp.ID] = bp
	m.functionBreakpoints = append(m.functionBreakpoints, bp)

	return bp, nil
}

// AddDataBreakpoint adds a data breakpoint.
func (m *BreakpointManager) AddDataBreakpoint(dataID, accessType, condition string) (*Breakpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp := &Breakpoint{
		ID:         m.allocateID(),
		Type:       BreakpointTypeData,
		DataID:     dataID,
		AccessType: accessType,
		Condition:  condition,
		Enabled:    true,
	}

	m.breakpoints[bp.ID] = bp
	m.dataBreakpoints = append(m.dataBreakpoints, bp)

	return bp, nil
}

// RemoveBreakpoint removes a breakpoint by ID.
func (m *BreakpointManager) RemoveBreakpoint(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.breakpoints[id]
	if !ok {
		return fmt.Errorf("breakpoint %d not found", id)
	}

	delete(m.breakpoints, id)

	// Remove from appropriate collection
	switch bp.Type {
	case BreakpointTypeLine, BreakpointTypeConditional, BreakpointTypeLogPoint:
		m.removeFromPath(bp.Path, id)
	case BreakpointTypeFunction:
		m.functionBreakpoints = removeBreakpointFromSlice(m.functionBreakpoints, id)
	case BreakpointTypeData:
		m.dataBreakpoints = removeBreakpointFromSlice(m.dataBreakpoints, id)
	}

	return nil
}

// removeFromPath removes a breakpoint from the path collection.
func (m *BreakpointManager) removeFromPath(path string, id int) {
	bps := m.byPath[path]
	m.byPath[path] = removeBreakpointFromSlice(bps, id)
	if len(m.byPath[path]) == 0 {
		delete(m.byPath, path)
	}
}

// removeBreakpointFromSlice removes a breakpoint from a slice by ID.
func removeBreakpointFromSlice(slice []*Breakpoint, id int) []*Breakpoint {
	for i, bp := range slice {
		if bp.ID == id {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// GetBreakpoint returns a breakpoint by ID.
func (m *BreakpointManager) GetBreakpoint(id int) (*Breakpoint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bp, ok := m.breakpoints[id]
	return bp, ok
}

// GetBreakpointsForPath returns all breakpoints for a file path.
func (m *BreakpointManager) GetBreakpointsForPath(path string) []*Breakpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Breakpoint, len(m.byPath[path]))
	copy(result, m.byPath[path])
	return result
}

// GetAllBreakpoints returns all breakpoints.
func (m *BreakpointManager) GetAllBreakpoints() []*Breakpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Breakpoint, 0, len(m.breakpoints))
	for _, bp := range m.breakpoints {
		result = append(result, bp)
	}
	return result
}

// GetFunctionBreakpoints returns all function breakpoints.
func (m *BreakpointManager) GetFunctionBreakpoints() []*Breakpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Breakpoint, len(m.functionBreakpoints))
	copy(result, m.functionBreakpoints)
	return result
}

// GetDataBreakpoints returns all data breakpoints.
func (m *BreakpointManager) GetDataBreakpoints() []*Breakpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Breakpoint, len(m.dataBreakpoints))
	copy(result, m.dataBreakpoints)
	return result
}

// SetEnabled enables or disables a breakpoint.
func (m *BreakpointManager) SetEnabled(id int, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.breakpoints[id]
	if !ok {
		return fmt.Errorf("breakpoint %d not found", id)
	}

	bp.Enabled = enabled
	return nil
}

// SetCondition sets the condition for a breakpoint.
func (m *BreakpointManager) SetCondition(id int, condition string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.breakpoints[id]
	if !ok {
		return fmt.Errorf("breakpoint %d not found", id)
	}

	bp.Condition = condition
	if condition != "" && bp.Type == BreakpointTypeLine {
		bp.Type = BreakpointTypeConditional
	}
	return nil
}

// SetHitCondition sets the hit condition for a breakpoint.
func (m *BreakpointManager) SetHitCondition(id int, hitCondition string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.breakpoints[id]
	if !ok {
		return fmt.Errorf("breakpoint %d not found", id)
	}

	bp.HitCondition = hitCondition
	return nil
}

// SetLogMessage sets the log message for a breakpoint.
func (m *BreakpointManager) SetLogMessage(id int, logMessage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.breakpoints[id]
	if !ok {
		return fmt.Errorf("breakpoint %d not found", id)
	}

	bp.LogMessage = logMessage
	if logMessage != "" {
		bp.Type = BreakpointTypeLogPoint
	}
	return nil
}

// ToggleBreakpoint toggles a breakpoint at the given location.
// If a breakpoint exists at the location, it is removed.
// Otherwise, a new line breakpoint is added.
func (m *BreakpointManager) ToggleBreakpoint(path string, line int) (*Breakpoint, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if breakpoint exists at this location
	for _, bp := range m.byPath[path] {
		if bp.Line == line {
			// Remove existing breakpoint
			delete(m.breakpoints, bp.ID)
			m.byPath[path] = removeBreakpointFromSlice(m.byPath[path], bp.ID)
			if len(m.byPath[path]) == 0 {
				delete(m.byPath, path)
			}
			return bp, false, nil // false = removed
		}
	}

	// Add new breakpoint
	bp := &Breakpoint{
		ID:      m.allocateID(),
		Type:    BreakpointTypeLine,
		Path:    path,
		Line:    line,
		Enabled: true,
	}

	m.breakpoints[bp.ID] = bp
	m.byPath[path] = append(m.byPath[path], bp)

	return bp, true, nil // true = added
}

// IncrementHitCount increments the hit count for a breakpoint.
func (m *BreakpointManager) IncrementHitCount(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if bp, ok := m.breakpoints[id]; ok {
		bp.HitCount++
	}
}

// ResetHitCounts resets all breakpoint hit counts.
func (m *BreakpointManager) ResetHitCounts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, bp := range m.breakpoints {
		bp.HitCount = 0
	}
}

// ClearAll removes all breakpoints.
func (m *BreakpointManager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.breakpoints = make(map[int]*Breakpoint)
	m.byPath = make(map[string][]*Breakpoint)
	m.functionBreakpoints = nil
	m.dataBreakpoints = nil
}

// ClearForPath removes all breakpoints for a file path.
func (m *BreakpointManager) ClearForPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, bp := range m.byPath[path] {
		delete(m.breakpoints, bp.ID)
	}
	delete(m.byPath, path)
}

// SyncToSession sends all breakpoints to the debug session.
func (m *BreakpointManager) SyncToSession(ctx context.Context) error {
	if m.session == nil {
		return fmt.Errorf("no session attached")
	}

	m.mu.RLock()
	paths := make([]string, 0, len(m.byPath))
	for path := range m.byPath {
		paths = append(paths, path)
	}
	functionBPs := make([]*Breakpoint, len(m.functionBreakpoints))
	copy(functionBPs, m.functionBreakpoints)
	m.mu.RUnlock()

	// Sync source breakpoints per file
	for _, path := range paths {
		if err := m.syncPathToSession(ctx, path); err != nil {
			return fmt.Errorf("sync breakpoints for %s: %w", path, err)
		}
	}

	// Sync function breakpoints
	if len(functionBPs) > 0 {
		if err := m.syncFunctionBreakpointsToSession(ctx, functionBPs); err != nil {
			return fmt.Errorf("sync function breakpoints: %w", err)
		}
	}

	return nil
}

// syncPathToSession syncs breakpoints for a single path.
func (m *BreakpointManager) syncPathToSession(ctx context.Context, path string) error {
	m.mu.RLock()
	bps := m.byPath[path]
	sourceBPs := make([]dap.SourceBreakpoint, 0, len(bps))
	for _, bp := range bps {
		if !bp.Enabled {
			continue
		}
		sourceBPs = append(sourceBPs, dap.SourceBreakpoint{
			Line:         bp.Line,
			Column:       bp.Column,
			Condition:    bp.Condition,
			HitCondition: bp.HitCondition,
			LogMessage:   bp.LogMessage,
		})
	}
	m.mu.RUnlock()

	result, err := m.session.SetBreakpointsWithConditions(ctx, path, sourceBPs)
	if err != nil {
		return err
	}

	// Update breakpoint verification status
	m.mu.Lock()
	for i, bp := range m.byPath[path] {
		if i < len(result) {
			bp.Verified = result[i].Verified
			bp.Message = result[i].Message
			if result[i].Line > 0 {
				bp.ActualLine = result[i].Line
			}
		}
	}
	m.mu.Unlock()

	return nil
}

// syncFunctionBreakpointsToSession syncs function breakpoints to the session.
func (m *BreakpointManager) syncFunctionBreakpointsToSession(ctx context.Context, bps []*Breakpoint) error {
	caps := m.session.Capabilities()
	if caps == nil || !caps.SupportsFunctionBreakpoints {
		return nil // Adapter doesn't support function breakpoints
	}

	funcBPs := make([]dap.FunctionBreakpoint, 0, len(bps))
	for _, bp := range bps {
		if !bp.Enabled {
			continue
		}
		funcBPs = append(funcBPs, dap.FunctionBreakpoint{
			Name:         bp.FunctionName,
			Condition:    bp.Condition,
			HitCondition: bp.HitCondition,
		})
	}

	args := dap.SetFunctionBreakpointsArguments{
		Breakpoints: funcBPs,
	}

	result, err := m.session.client.SetFunctionBreakpoints(ctx, args)
	if err != nil {
		return err
	}

	// Update verification status
	m.mu.Lock()
	for i, bp := range m.functionBreakpoints {
		if i < len(result) {
			bp.Verified = result[i].Verified
			bp.Message = result[i].Message
		}
	}
	m.mu.Unlock()

	return nil
}

// persistedBreakpoints is the format for persisted breakpoints.
type persistedBreakpoints struct {
	Version     int           `json:"version"`
	Breakpoints []*Breakpoint `json:"breakpoints"`
}

// Save persists breakpoints to disk.
func (m *BreakpointManager) Save() error {
	m.mu.RLock()
	path := m.persistPath
	bps := make([]*Breakpoint, 0, len(m.breakpoints))
	for _, bp := range m.breakpoints {
		bps = append(bps, bp)
	}
	m.mu.RUnlock()

	if path == "" {
		return fmt.Errorf("persist path not set")
	}

	data := persistedBreakpoints{
		Version:     1,
		Breakpoints: bps,
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal breakpoints: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Load loads persisted breakpoints from disk.
func (m *BreakpointManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.persistPath == "" {
		return fmt.Errorf("persist path not set")
	}

	content, err := os.ReadFile(m.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No persisted breakpoints
		}
		return fmt.Errorf("read file: %w", err)
	}

	var data persistedBreakpoints
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("unmarshal breakpoints: %w", err)
	}

	// Clear existing breakpoints
	m.breakpoints = make(map[int]*Breakpoint)
	m.byPath = make(map[string][]*Breakpoint)
	m.functionBreakpoints = nil
	m.dataBreakpoints = nil

	// Load breakpoints
	maxID := 0
	for _, bp := range data.Breakpoints {
		m.breakpoints[bp.ID] = bp
		if bp.ID > maxID {
			maxID = bp.ID
		}

		switch bp.Type {
		case BreakpointTypeLine, BreakpointTypeConditional, BreakpointTypeLogPoint:
			m.byPath[bp.Path] = append(m.byPath[bp.Path], bp)
		case BreakpointTypeFunction:
			m.functionBreakpoints = append(m.functionBreakpoints, bp)
		case BreakpointTypeData:
			m.dataBreakpoints = append(m.dataBreakpoints, bp)
		}
	}

	m.nextID = maxID + 1

	return nil
}

// GetPathsWithBreakpoints returns all file paths that have breakpoints.
func (m *BreakpointManager) GetPathsWithBreakpoints() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	paths := make([]string, 0, len(m.byPath))
	for path := range m.byPath {
		paths = append(paths, path)
	}
	return paths
}

// HasBreakpointAt checks if there's a breakpoint at the given location.
func (m *BreakpointManager) HasBreakpointAt(path string, line int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, bp := range m.byPath[path] {
		if bp.Line == line {
			return true
		}
	}
	return false
}

// GetBreakpointAt returns the breakpoint at the given location, if any.
func (m *BreakpointManager) GetBreakpointAt(path string, line int) (*Breakpoint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, bp := range m.byPath[path] {
		if bp.Line == line {
			return bp, true
		}
	}
	return nil, false
}
