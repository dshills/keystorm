package api

import (
	"fmt"
	"sync"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// NotificationLevel represents the severity of a notification.
type NotificationLevel string

const (
	// NotificationInfo is an informational notification.
	NotificationInfo NotificationLevel = "info"
	// NotificationWarning is a warning notification.
	NotificationWarning NotificationLevel = "warning"
	// NotificationError is an error notification.
	NotificationError NotificationLevel = "error"
	// NotificationSuccess is a success notification.
	NotificationSuccess NotificationLevel = "success"
)

// StatuslinePosition represents where in the statusline to place content.
type StatuslinePosition string

const (
	// StatuslineLeft is the left section of the statusline.
	StatuslineLeft StatuslinePosition = "left"
	// StatuslineCenter is the center section of the statusline.
	StatuslineCenter StatuslinePosition = "center"
	// StatuslineRight is the right section of the statusline.
	StatuslineRight StatuslinePosition = "right"
)

// UIProvider defines the interface for UI operations.
type UIProvider interface {
	// Notify shows a notification to the user.
	Notify(message string, level NotificationLevel) error

	// Input prompts the user for text input.
	// Returns the input text, or empty string if cancelled.
	Input(prompt string, defaultValue string) (string, error)

	// Select shows a selection menu to the user.
	// Returns the selected index (0-based), or -1 if cancelled.
	Select(items []string, opts SelectOptions) (int, error)

	// Confirm shows a yes/no confirmation dialog.
	Confirm(message string) (bool, error)

	// SetStatusline sets content in a statusline segment.
	SetStatusline(position StatuslinePosition, segment string, content string) error

	// ClearStatusline clears a statusline segment.
	ClearStatusline(position StatuslinePosition, segment string) error

	// CreateOverlay creates an overlay window.
	// Returns the overlay ID.
	CreateOverlay(opts OverlayOptions) (string, error)

	// UpdateOverlay updates an existing overlay.
	UpdateOverlay(id string, opts OverlayOptions) error

	// CloseOverlay closes an overlay.
	CloseOverlay(id string) error
}

// SelectOptions configures a selection menu.
type SelectOptions struct {
	Title       string
	Placeholder string
	MultiSelect bool
}

// OverlayOptions configures an overlay window.
type OverlayOptions struct {
	Title   string
	Content string
	X       int
	Y       int
	Width   int
	Height  int
	Border  bool
}

// UIModule implements the ks.ui API module.
type UIModule struct {
	ctx        *Context
	pluginName string
	L          *lua.LState

	// Track overlays for cleanup
	mu       sync.Mutex
	overlays map[string]bool
}

// NewUIModule creates a new UI module.
func NewUIModule(ctx *Context, pluginName string) *UIModule {
	return &UIModule{
		ctx:        ctx,
		pluginName: pluginName,
		overlays:   make(map[string]bool),
	}
}

// Name returns the module name.
func (m *UIModule) Name() string {
	return "ui"
}

// RequiredCapability returns the capability required for this module.
func (m *UIModule) RequiredCapability() security.Capability {
	return security.CapabilityUI
}

// Register registers the module into the Lua state.
func (m *UIModule) Register(L *lua.LState) error {
	m.L = L

	mod := L.NewTable()

	// Register main UI functions
	L.SetField(mod, "notify", L.NewFunction(m.notify))
	L.SetField(mod, "input", L.NewFunction(m.input))
	L.SetField(mod, "select", L.NewFunction(m.selectMenu))
	L.SetField(mod, "confirm", L.NewFunction(m.confirm))

	// Create statusline sub-module
	statusline := L.NewTable()
	L.SetField(statusline, "set", L.NewFunction(m.statuslineSet))
	L.SetField(statusline, "clear", L.NewFunction(m.statuslineClear))
	L.SetField(mod, "statusline", statusline)

	// Create overlay sub-module
	overlay := L.NewTable()
	L.SetField(overlay, "create", L.NewFunction(m.overlayCreate))
	L.SetField(overlay, "update", L.NewFunction(m.overlayUpdate))
	L.SetField(overlay, "close", L.NewFunction(m.overlayClose))
	L.SetField(mod, "overlay", overlay)

	// Add notification level constants
	L.SetField(mod, "INFO", lua.LString(NotificationInfo))
	L.SetField(mod, "WARNING", lua.LString(NotificationWarning))
	L.SetField(mod, "ERROR", lua.LString(NotificationError))
	L.SetField(mod, "SUCCESS", lua.LString(NotificationSuccess))

	// Add statusline position constants
	L.SetField(mod, "LEFT", lua.LString(StatuslineLeft))
	L.SetField(mod, "CENTER", lua.LString(StatuslineCenter))
	L.SetField(mod, "RIGHT", lua.LString(StatuslineRight))

	L.SetGlobal("_ks_ui", mod)
	return nil
}

// Cleanup closes all overlays created by this plugin.
func (m *UIModule) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctx.UI == nil {
		return
	}

	// Close all overlays
	for id := range m.overlays {
		_ = m.ctx.UI.CloseOverlay(id)
	}
	m.overlays = make(map[string]bool)

	// Clear statusline segments for this plugin
	segmentPrefix := "plugin:" + m.pluginName
	_ = m.ctx.UI.ClearStatusline(StatuslineLeft, segmentPrefix)
	_ = m.ctx.UI.ClearStatusline(StatuslineCenter, segmentPrefix)
	_ = m.ctx.UI.ClearStatusline(StatuslineRight, segmentPrefix)
}

// notify(message, level?) -> nil
// Shows a notification to the user.
func (m *UIModule) notify(L *lua.LState) int {
	message := L.CheckString(1)
	levelStr := L.OptString(2, string(NotificationInfo))

	if message == "" {
		L.ArgError(1, "message cannot be empty")
		return 0
	}

	if m.ctx.UI == nil {
		// If no UI provider, silently succeed (notification is optional)
		return 0
	}

	// Validate level
	level := NotificationLevel(levelStr)
	switch level {
	case NotificationInfo, NotificationWarning, NotificationError, NotificationSuccess:
		// Valid
	default:
		level = NotificationInfo
	}

	if err := m.ctx.UI.Notify(message, level); err != nil {
		L.RaiseError("notify: %v", err)
		return 0
	}

	return 0
}

// input(prompt, default?) -> string or nil
// Prompts the user for text input.
func (m *UIModule) input(L *lua.LState) int {
	prompt := L.CheckString(1)
	defaultValue := L.OptString(2, "")

	if m.ctx.UI == nil {
		L.Push(lua.LNil)
		return 1
	}

	result, err := m.ctx.UI.Input(prompt, defaultValue)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}

	L.Push(lua.LString(result))
	return 1
}

// select(items, opts?) -> index or nil
// Shows a selection menu. Returns 1-based index for Lua.
func (m *UIModule) selectMenu(L *lua.LState) int {
	itemsTable := L.CheckTable(1)

	if m.ctx.UI == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Convert items table to slice
	var items []string
	itemsTable.ForEach(func(_, value lua.LValue) {
		if str, ok := value.(lua.LString); ok {
			items = append(items, string(str))
		}
	})

	if len(items) == 0 {
		L.Push(lua.LNil)
		return 1
	}

	// Parse options
	opts := SelectOptions{}
	if L.GetTop() >= 2 {
		optsTable := L.OptTable(2, nil)
		if optsTable != nil {
			opts.Title = getTableString(L, optsTable, "title")
			opts.Placeholder = getTableString(L, optsTable, "placeholder")
			opts.MultiSelect = getTableBool(L, optsTable, "multi_select")
		}
	}

	idx, err := m.ctx.UI.Select(items, opts)
	if err != nil || idx < 0 {
		L.Push(lua.LNil)
		return 1
	}

	// Return 1-based index for Lua
	L.Push(lua.LNumber(idx + 1))
	return 1
}

// confirm(message) -> bool
// Shows a yes/no confirmation dialog.
func (m *UIModule) confirm(L *lua.LState) int {
	message := L.CheckString(1)

	if m.ctx.UI == nil {
		L.Push(lua.LFalse)
		return 1
	}

	result, err := m.ctx.UI.Confirm(message)
	if err != nil {
		L.Push(lua.LFalse)
		return 1
	}

	L.Push(lua.LBool(result))
	return 1
}

// statuslineSet(position, content) -> nil
// Sets content in a statusline segment for this plugin.
func (m *UIModule) statuslineSet(L *lua.LState) int {
	positionStr := L.CheckString(1)
	content := L.CheckString(2)

	if m.ctx.UI == nil {
		return 0
	}

	// Validate position
	position := StatuslinePosition(positionStr)
	switch position {
	case StatuslineLeft, StatuslineCenter, StatuslineRight:
		// Valid
	default:
		L.ArgError(1, "position must be 'left', 'center', or 'right'")
		return 0
	}

	// Use plugin name as segment identifier
	segment := "plugin:" + m.pluginName

	if err := m.ctx.UI.SetStatusline(position, segment, content); err != nil {
		L.RaiseError("statusline.set: %v", err)
		return 0
	}

	return 0
}

// statuslineClear(position) -> nil
// Clears the plugin's statusline segment.
func (m *UIModule) statuslineClear(L *lua.LState) int {
	positionStr := L.CheckString(1)

	if m.ctx.UI == nil {
		return 0
	}

	// Validate position
	position := StatuslinePosition(positionStr)
	switch position {
	case StatuslineLeft, StatuslineCenter, StatuslineRight:
		// Valid
	default:
		L.ArgError(1, "position must be 'left', 'center', or 'right'")
		return 0
	}

	segment := "plugin:" + m.pluginName

	if err := m.ctx.UI.ClearStatusline(position, segment); err != nil {
		L.RaiseError("statusline.clear: %v", err)
		return 0
	}

	return 0
}

// overlayCreate(opts) -> overlayID
// Creates an overlay window.
func (m *UIModule) overlayCreate(L *lua.LState) int {
	opts := L.CheckTable(1)

	if m.ctx.UI == nil {
		L.RaiseError("overlay.create: no UI provider available")
		return 0
	}

	overlayOpts := OverlayOptions{
		Title:   getTableString(L, opts, "title"),
		Content: getTableString(L, opts, "content"),
		X:       int(getTableNumber(L, opts, "x")),
		Y:       int(getTableNumber(L, opts, "y")),
		Width:   int(getTableNumber(L, opts, "width")),
		Height:  int(getTableNumber(L, opts, "height")),
		Border:  getTableBool(L, opts, "border"),
	}

	// Prefix title with plugin name for identification
	if overlayOpts.Title != "" {
		overlayOpts.Title = fmt.Sprintf("[%s] %s", m.pluginName, overlayOpts.Title)
	}

	id, err := m.ctx.UI.CreateOverlay(overlayOpts)
	if err != nil {
		L.RaiseError("overlay.create: %v", err)
		return 0
	}

	// Track for cleanup
	m.mu.Lock()
	m.overlays[id] = true
	m.mu.Unlock()

	L.Push(lua.LString(id))
	return 1
}

// overlayUpdate(id, opts) -> nil
// Updates an existing overlay.
func (m *UIModule) overlayUpdate(L *lua.LState) int {
	id := L.CheckString(1)
	opts := L.CheckTable(2)

	if m.ctx.UI == nil {
		L.RaiseError("overlay.update: no UI provider available")
		return 0
	}

	// Verify this plugin owns the overlay
	m.mu.Lock()
	if !m.overlays[id] {
		m.mu.Unlock()
		L.RaiseError("overlay.update: overlay %q not found or not owned by this plugin", id)
		return 0
	}
	m.mu.Unlock()

	overlayOpts := OverlayOptions{
		Content: getTableString(L, opts, "content"),
	}

	// Only include non-zero values
	if title := getTableString(L, opts, "title"); title != "" {
		overlayOpts.Title = fmt.Sprintf("[%s] %s", m.pluginName, title)
	}
	if x := getTableNumber(L, opts, "x"); x != 0 {
		overlayOpts.X = int(x)
	}
	if y := getTableNumber(L, opts, "y"); y != 0 {
		overlayOpts.Y = int(y)
	}
	if width := getTableNumber(L, opts, "width"); width != 0 {
		overlayOpts.Width = int(width)
	}
	if height := getTableNumber(L, opts, "height"); height != 0 {
		overlayOpts.Height = int(height)
	}

	if err := m.ctx.UI.UpdateOverlay(id, overlayOpts); err != nil {
		L.RaiseError("overlay.update: %v", err)
		return 0
	}

	return 0
}

// overlayClose(id) -> nil
// Closes an overlay.
func (m *UIModule) overlayClose(L *lua.LState) int {
	id := L.CheckString(1)

	if m.ctx.UI == nil {
		return 0
	}

	// Verify and remove from tracking
	m.mu.Lock()
	if !m.overlays[id] {
		m.mu.Unlock()
		// Silently ignore if not found (may have been cleaned up already)
		return 0
	}
	delete(m.overlays, id)
	m.mu.Unlock()

	if err := m.ctx.UI.CloseOverlay(id); err != nil {
		L.RaiseError("overlay.close: %v", err)
		return 0
	}

	return 0
}

// getTableBool safely gets a boolean field from a Lua table.
func getTableBool(L *lua.LState, tbl *lua.LTable, key string) bool {
	val := L.GetField(tbl, key)
	if b, ok := val.(lua.LBool); ok {
		return bool(b)
	}
	return false
}

// getTableNumber safely gets a number field from a Lua table.
func getTableNumber(L *lua.LState, tbl *lua.LTable, key string) float64 {
	val := L.GetField(tbl, key)
	if n, ok := val.(lua.LNumber); ok {
		return float64(n)
	}
	return 0
}
