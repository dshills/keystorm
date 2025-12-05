package api

import (
	"errors"
	"sync"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockUIProvider implements UIProvider for testing.
type mockUIProvider struct {
	mu sync.Mutex

	// Track notifications
	notifications []notificationRecord

	// Track inputs
	inputPrompts  []inputRecord
	inputResponse string
	inputErr      error

	// Track selects
	selectCalls    []selectRecord
	selectResponse int
	selectErr      error

	// Track confirms
	confirmCalls    []string
	confirmResponse bool
	confirmErr      error

	// Track statusline
	statusline map[string]string

	// Track overlays
	overlays      map[string]OverlayOptions
	nextOverlayID int
	overlayErr    error
}

type notificationRecord struct {
	message string
	level   NotificationLevel
}

type inputRecord struct {
	prompt       string
	defaultValue string
}

type selectRecord struct {
	items []string
	opts  SelectOptions
}

func newMockUIProvider() *mockUIProvider {
	return &mockUIProvider{
		statusline:     make(map[string]string),
		overlays:       make(map[string]OverlayOptions),
		selectResponse: -1,
	}
}

func (m *mockUIProvider) Notify(message string, level NotificationLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, notificationRecord{message, level})
	return nil
}

func (m *mockUIProvider) Input(prompt string, defaultValue string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputPrompts = append(m.inputPrompts, inputRecord{prompt, defaultValue})
	return m.inputResponse, m.inputErr
}

func (m *mockUIProvider) Select(items []string, opts SelectOptions) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selectCalls = append(m.selectCalls, selectRecord{items, opts})
	return m.selectResponse, m.selectErr
}

func (m *mockUIProvider) Confirm(message string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.confirmCalls = append(m.confirmCalls, message)
	return m.confirmResponse, m.confirmErr
}

func (m *mockUIProvider) SetStatusline(position StatuslinePosition, segment string, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(position) + ":" + segment
	m.statusline[key] = content
	return nil
}

func (m *mockUIProvider) ClearStatusline(position StatuslinePosition, segment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(position) + ":" + segment
	delete(m.statusline, key)
	return nil
}

func (m *mockUIProvider) CreateOverlay(opts OverlayOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.overlayErr != nil {
		return "", m.overlayErr
	}
	m.nextOverlayID++
	id := string(rune('A' + m.nextOverlayID - 1))
	m.overlays[id] = opts
	return id, nil
}

func (m *mockUIProvider) UpdateOverlay(id string, opts OverlayOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.overlays[id]; !exists {
		return errors.New("overlay not found")
	}
	m.overlays[id] = opts
	return nil
}

func (m *mockUIProvider) CloseOverlay(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.overlays[id]; !exists {
		return errors.New("overlay not found")
	}
	delete(m.overlays, id)
	return nil
}

func (m *mockUIProvider) GetNotifications() []notificationRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]notificationRecord, len(m.notifications))
	copy(result, m.notifications)
	return result
}

func (m *mockUIProvider) GetStatusline(position StatuslinePosition, segment string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(position) + ":" + segment
	content, ok := m.statusline[key]
	return content, ok
}

func (m *mockUIProvider) GetOverlay(id string) (OverlayOptions, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	opts, ok := m.overlays[id]
	return opts, ok
}

func (m *mockUIProvider) OverlayCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.overlays)
}

func setupUITest(t *testing.T, up *mockUIProvider) (*lua.LState, *UIModule) {
	t.Helper()

	ctx := &Context{UI: up}
	mod := NewUIModule(ctx, "testplugin")

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestUIModuleName(t *testing.T) {
	ctx := &Context{}
	mod := NewUIModule(ctx, "test")
	if mod.Name() != "ui" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "ui")
	}
}

func TestUIModuleCapability(t *testing.T) {
	ctx := &Context{}
	mod := NewUIModule(ctx, "test")
	if mod.RequiredCapability() != security.CapabilityUI {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityUI)
	}
}

func TestUINotify(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		_ks_ui.notify("Hello, World!")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	notifications := up.GetNotifications()
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}

	if notifications[0].message != "Hello, World!" {
		t.Errorf("message = %q, want %q", notifications[0].message, "Hello, World!")
	}
	if notifications[0].level != NotificationInfo {
		t.Errorf("level = %q, want %q", notifications[0].level, NotificationInfo)
	}
}

func TestUINotifyWithLevel(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	tests := []struct {
		level    string
		expected NotificationLevel
	}{
		{"info", NotificationInfo},
		{"warning", NotificationWarning},
		{"error", NotificationError},
		{"success", NotificationSuccess},
	}

	for _, tt := range tests {
		err := L.DoString(`
			_ks_ui.notify("Test message", "` + tt.level + `")
		`)
		if err != nil {
			t.Fatalf("DoString error for level %q: %v", tt.level, err)
		}
	}

	notifications := up.GetNotifications()
	if len(notifications) != 4 {
		t.Fatalf("notification count = %d, want 4", len(notifications))
	}

	for i, tt := range tests {
		if notifications[i].level != tt.expected {
			t.Errorf("notifications[%d].level = %q, want %q", i, notifications[i].level, tt.expected)
		}
	}
}

func TestUINotifyEmptyMessage(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		_ks_ui.notify("")
	`)
	if err == nil {
		t.Error("notify with empty message should error")
	}
}

func TestUIInput(t *testing.T) {
	up := newMockUIProvider()
	up.inputResponse = "user input"
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		result = _ks_ui.input("Enter value:", "default")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.(lua.LString) != "user input" {
		t.Errorf("result = %v, want 'user input'", result)
	}
}

func TestUIInputCancelled(t *testing.T) {
	up := newMockUIProvider()
	up.inputErr = errors.New("cancelled")
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		result = _ks_ui.input("Enter value:")
		is_nil = result == nil
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	isNil := L.GetGlobal("is_nil")
	if isNil != lua.LTrue {
		t.Error("cancelled input should return nil")
	}
}

func TestUISelect(t *testing.T) {
	up := newMockUIProvider()
	up.selectResponse = 1 // Second item (0-indexed)
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		items = {"Option 1", "Option 2", "Option 3"}
		result = _ks_ui.select(items, { title = "Choose:" })
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	// Should be 2 (1-indexed for Lua)
	if result.(lua.LNumber) != 2 {
		t.Errorf("result = %v, want 2 (1-indexed)", result)
	}
}

func TestUISelectCancelled(t *testing.T) {
	up := newMockUIProvider()
	up.selectResponse = -1 // Cancelled
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		items = {"Option 1", "Option 2"}
		result = _ks_ui.select(items)
		is_nil = result == nil
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	isNil := L.GetGlobal("is_nil")
	if isNil != lua.LTrue {
		t.Error("cancelled select should return nil")
	}
}

func TestUISelectEmptyItems(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		items = {}
		result = _ks_ui.select(items)
		is_nil = result == nil
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	isNil := L.GetGlobal("is_nil")
	if isNil != lua.LTrue {
		t.Error("select with empty items should return nil")
	}
}

func TestUIConfirm(t *testing.T) {
	up := newMockUIProvider()
	up.confirmResponse = true
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		result = _ks_ui.confirm("Are you sure?")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Error("confirm should return true when user confirms")
	}
}

func TestUIConfirmDeclined(t *testing.T) {
	up := newMockUIProvider()
	up.confirmResponse = false
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		result = _ks_ui.confirm("Are you sure?")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LFalse {
		t.Error("confirm should return false when user declines")
	}
}

func TestUIStatuslineSet(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		_ks_ui.statusline.set("left", "Custom Status")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	content, exists := up.GetStatusline(StatuslineLeft, "plugin:testplugin")
	if !exists {
		t.Fatal("statusline segment should exist")
	}
	if content != "Custom Status" {
		t.Errorf("content = %q, want %q", content, "Custom Status")
	}
}

func TestUIStatuslineSetAllPositions(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	positions := []string{"left", "center", "right"}
	for _, pos := range positions {
		err := L.DoString(`
			_ks_ui.statusline.set("` + pos + `", "` + pos + ` content")
		`)
		if err != nil {
			t.Fatalf("DoString error for position %q: %v", pos, err)
		}
	}

	// Verify all positions
	testCases := []struct {
		position StatuslinePosition
		expected string
	}{
		{StatuslineLeft, "left content"},
		{StatuslineCenter, "center content"},
		{StatuslineRight, "right content"},
	}

	for _, tc := range testCases {
		content, exists := up.GetStatusline(tc.position, "plugin:testplugin")
		if !exists {
			t.Errorf("statusline segment for %q should exist", tc.position)
		}
		if content != tc.expected {
			t.Errorf("content for %q = %q, want %q", tc.position, content, tc.expected)
		}
	}
}

func TestUIStatuslineSetInvalidPosition(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		_ks_ui.statusline.set("invalid", "content")
	`)
	if err == nil {
		t.Error("statusline.set with invalid position should error")
	}
}

func TestUIStatuslineClear(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	// Set then clear
	err := L.DoString(`
		_ks_ui.statusline.set("left", "Content")
		_ks_ui.statusline.clear("left")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	_, exists := up.GetStatusline(StatuslineLeft, "plugin:testplugin")
	if exists {
		t.Error("statusline segment should be cleared")
	}
}

func TestUIOverlayCreate(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	err := L.DoString(`
		overlay_id = _ks_ui.overlay.create({
			title = "My Overlay",
			content = "Hello!",
			x = 10,
			y = 20,
			width = 40,
			height = 10,
			border = true
		})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	overlayID := L.GetGlobal("overlay_id")
	if overlayID == lua.LNil {
		t.Fatal("overlay ID should not be nil")
	}

	if up.OverlayCount() != 1 {
		t.Errorf("overlay count = %d, want 1", up.OverlayCount())
	}
}

func TestUIOverlayUpdate(t *testing.T) {
	up := newMockUIProvider()
	L, mod := setupUITest(t, up)

	// Create overlay
	err := L.DoString(`
		overlay_id = _ks_ui.overlay.create({ content = "Initial" })
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	overlayID := string(L.GetGlobal("overlay_id").(lua.LString))

	// Track it in module (normally done by create)
	mod.mu.Lock()
	mod.overlays[overlayID] = true
	mod.mu.Unlock()

	// Update overlay
	err = L.DoString(`
		_ks_ui.overlay.update(overlay_id, { content = "Updated" })
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	opts, _ := up.GetOverlay(overlayID)
	if opts.Content != "Updated" {
		t.Errorf("overlay content = %q, want %q", opts.Content, "Updated")
	}
}

func TestUIOverlayClose(t *testing.T) {
	up := newMockUIProvider()
	L, mod := setupUITest(t, up)

	// Create overlay
	err := L.DoString(`
		overlay_id = _ks_ui.overlay.create({ content = "Test" })
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	overlayID := string(L.GetGlobal("overlay_id").(lua.LString))

	// Track it in module
	mod.mu.Lock()
	mod.overlays[overlayID] = true
	mod.mu.Unlock()

	if up.OverlayCount() != 1 {
		t.Fatalf("overlay count = %d, want 1", up.OverlayCount())
	}

	// Close overlay
	err = L.DoString(`
		_ks_ui.overlay.close(overlay_id)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if up.OverlayCount() != 0 {
		t.Errorf("overlay count after close = %d, want 0", up.OverlayCount())
	}
}

func TestUIOverlayUpdateNotOwned(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	// Create overlay directly (not through module)
	up.CreateOverlay(OverlayOptions{Content: "External"})

	// Try to update an overlay not owned by this plugin
	err := L.DoString(`
		_ks_ui.overlay.update("A", { content = "Hacked" })
	`)
	if err == nil {
		t.Error("overlay.update should error when plugin doesn't own the overlay")
	}
}

func TestUICleanup(t *testing.T) {
	up := newMockUIProvider()
	L, mod := setupUITest(t, up)

	// Create overlays
	err := L.DoString(`
		id1 = _ks_ui.overlay.create({ content = "1" })
		id2 = _ks_ui.overlay.create({ content = "2" })
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Track overlays
	id1 := string(L.GetGlobal("id1").(lua.LString))
	id2 := string(L.GetGlobal("id2").(lua.LString))
	mod.mu.Lock()
	mod.overlays[id1] = true
	mod.overlays[id2] = true
	mod.mu.Unlock()

	// Set statusline
	err = L.DoString(`
		_ks_ui.statusline.set("left", "Status")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if up.OverlayCount() != 2 {
		t.Fatalf("overlay count = %d, want 2", up.OverlayCount())
	}

	// Cleanup
	mod.Cleanup()

	if up.OverlayCount() != 0 {
		t.Errorf("overlay count after cleanup = %d, want 0", up.OverlayCount())
	}
}

func TestUIConstants(t *testing.T) {
	up := newMockUIProvider()
	L, _ := setupUITest(t, up)

	// Check notification level constants
	err := L.DoString(`
		info = _ks_ui.INFO
		warning = _ks_ui.WARNING
		error_level = _ks_ui.ERROR
		success = _ks_ui.SUCCESS
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if L.GetGlobal("info").(lua.LString) != lua.LString(NotificationInfo) {
		t.Error("INFO constant mismatch")
	}
	if L.GetGlobal("warning").(lua.LString) != lua.LString(NotificationWarning) {
		t.Error("WARNING constant mismatch")
	}
	if L.GetGlobal("error_level").(lua.LString) != lua.LString(NotificationError) {
		t.Error("ERROR constant mismatch")
	}
	if L.GetGlobal("success").(lua.LString) != lua.LString(NotificationSuccess) {
		t.Error("SUCCESS constant mismatch")
	}

	// Check position constants
	err = L.DoString(`
		left = _ks_ui.LEFT
		center = _ks_ui.CENTER
		right = _ks_ui.RIGHT
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if L.GetGlobal("left").(lua.LString) != lua.LString(StatuslineLeft) {
		t.Error("LEFT constant mismatch")
	}
	if L.GetGlobal("center").(lua.LString) != lua.LString(StatuslineCenter) {
		t.Error("CENTER constant mismatch")
	}
	if L.GetGlobal("right").(lua.LString) != lua.LString(StatuslineRight) {
		t.Error("RIGHT constant mismatch")
	}
}

func TestUINilProvider(t *testing.T) {
	ctx := &Context{UI: nil}
	mod := NewUIModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// notify should silently succeed (notifications are optional)
	err := L.DoString(`
		_ks_ui.notify("Test")
	`)
	if err != nil {
		t.Errorf("notify should succeed silently with nil provider: %v", err)
	}

	// input should return nil
	err = L.DoString(`
		result = _ks_ui.input("Prompt")
		is_nil = result == nil
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}
	if L.GetGlobal("is_nil") != lua.LTrue {
		t.Error("input should return nil with nil provider")
	}

	// confirm should return false
	err = L.DoString(`
		result = _ks_ui.confirm("Sure?")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}
	if L.GetGlobal("result") != lua.LFalse {
		t.Error("confirm should return false with nil provider")
	}

	// overlay.create should error
	err = L.DoString(`
		_ks_ui.overlay.create({})
	`)
	if err == nil {
		t.Error("overlay.create should error with nil provider")
	}
}
