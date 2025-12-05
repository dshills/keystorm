package lsp

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.Status() != ClientStatusStopped {
		t.Errorf("Initial status: got %v, want %v", client.Status(), ClientStatusStopped)
	}

	if client.IsReady() {
		t.Error("New client should not be ready")
	}
}

func TestClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	if config.RequestTimeout != 10*time.Second {
		t.Errorf("RequestTimeout: got %v, want 10s", config.RequestTimeout)
	}

	if config.CompletionMaxResults != 100 {
		t.Errorf("CompletionMaxResults: got %d, want 100", config.CompletionMaxResults)
	}

	if config.FormatOnSave {
		t.Error("FormatOnSave should default to false")
	}

	if !config.AutoDetectServers {
		t.Error("AutoDetectServers should default to true")
	}

	if !config.RenameConfirmation {
		t.Error("RenameConfirmation should default to true")
	}
}

func TestClientWithOptions(t *testing.T) {
	servers := map[string]ServerConfig{
		"go": {Command: "gopls", Args: []string{"serve"}},
	}

	folders := []WorkspaceFolder{
		{URI: "file:///test/project", Name: "project"},
	}

	formatOpts := FormattingOptions{
		TabSize:      2,
		InsertSpaces: true,
	}

	client := NewClient(
		WithServers(servers),
		WithWorkspaceFolders(folders),
		WithClientRequestTimeout(5*time.Second),
		WithClientFormatOnSave(true),
		WithClientFormatOptions(formatOpts),
		WithAutoDetectServers(false),
	)

	config := client.Config()

	if len(config.Servers) != 1 {
		t.Errorf("Servers: got %d, want 1", len(config.Servers))
	}

	if len(config.WorkspaceFolders) != 1 {
		t.Errorf("WorkspaceFolders: got %d, want 1", len(config.WorkspaceFolders))
	}

	if config.RequestTimeout != 5*time.Second {
		t.Errorf("RequestTimeout: got %v, want 5s", config.RequestTimeout)
	}

	if !config.FormatOnSave {
		t.Error("FormatOnSave should be true")
	}

	if config.FormatOptions.TabSize != 2 {
		t.Errorf("TabSize: got %d, want 2", config.FormatOptions.TabSize)
	}

	if config.AutoDetectServers {
		t.Error("AutoDetectServers should be false")
	}
}

func TestClientWithWorkspaceRoot(t *testing.T) {
	client := NewClient(WithWorkspaceRoot("/test/project"))

	config := client.Config()
	if len(config.WorkspaceFolders) != 1 {
		t.Fatalf("WorkspaceFolders: got %d, want 1", len(config.WorkspaceFolders))
	}

	if config.WorkspaceFolders[0].Name != "project" {
		t.Errorf("Workspace name: got %q, want %q", config.WorkspaceFolders[0].Name, "project")
	}
}

func TestClientStatusString(t *testing.T) {
	tests := []struct {
		status ClientStatus
		want   string
	}{
		{ClientStatusStopped, "stopped"},
		{ClientStatusStarting, "starting"},
		{ClientStatusReady, "ready"},
		{ClientStatusShuttingDown, "shutting down"},
		{ClientStatus(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("ClientStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestClientStartStop(t *testing.T) {
	client := NewClient(
		WithAutoDetectServers(false),
	)

	ctx := context.Background()

	// Start
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if client.Status() != ClientStatusReady {
		t.Errorf("Status after Start: got %v, want %v", client.Status(), ClientStatusReady)
	}

	if !client.IsReady() {
		t.Error("Client should be ready after Start")
	}

	// Double start should fail
	if err := client.Start(ctx); err != ErrAlreadyStarted {
		t.Errorf("Double Start: got %v, want ErrAlreadyStarted", err)
	}

	// Shutdown
	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	if client.Status() != ClientStatusStopped {
		t.Errorf("Status after Shutdown: got %v, want %v", client.Status(), ClientStatusStopped)
	}

	if client.IsReady() {
		t.Error("Client should not be ready after Shutdown")
	}

	// Double shutdown should succeed silently
	if err := client.Shutdown(ctx); err != nil {
		t.Errorf("Double Shutdown should not fail: %v", err)
	}
}

func TestClientNotStarted(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	// All operations should fail with ErrNotStarted
	if err := client.OpenDocument(ctx, "/test.go", "package main"); err != ErrNotStarted {
		t.Errorf("OpenDocument: got %v, want ErrNotStarted", err)
	}

	if err := client.CloseDocument(ctx, "/test.go"); err != ErrNotStarted {
		t.Errorf("CloseDocument: got %v, want ErrNotStarted", err)
	}

	if err := client.ChangeDocument(ctx, "/test.go", nil); err != ErrNotStarted {
		t.Errorf("ChangeDocument: got %v, want ErrNotStarted", err)
	}

	if _, err := client.Complete(ctx, "/test.go", Position{}, ""); err != ErrNotStarted {
		t.Errorf("Complete: got %v, want ErrNotStarted", err)
	}

	if _, err := client.Hover(ctx, "/test.go", Position{}); err != ErrNotStarted {
		t.Errorf("Hover: got %v, want ErrNotStarted", err)
	}

	if _, err := client.GoToDefinition(ctx, "/test.go", Position{}); err != ErrNotStarted {
		t.Errorf("GoToDefinition: got %v, want ErrNotStarted", err)
	}

	if _, err := client.Format(ctx, "/test.go"); err != ErrNotStarted {
		t.Errorf("Format: got %v, want ErrNotStarted", err)
	}

	if _, err := client.Rename(ctx, "/test.go", Position{}, "newName"); err != ErrNotStarted {
		t.Errorf("Rename: got %v, want ErrNotStarted", err)
	}
}

func TestClientDiagnosticsNotStarted(t *testing.T) {
	client := NewClient()

	// Diagnostics should return nil when not started
	if diags := client.Diagnostics("/test.go"); diags != nil {
		t.Errorf("Diagnostics: got %v, want nil", diags)
	}

	if all := client.AllDiagnostics(); all != nil {
		t.Errorf("AllDiagnostics: got %v, want nil", all)
	}

	if count := client.ErrorCount(); count != 0 {
		t.Errorf("ErrorCount: got %d, want 0", count)
	}

	if count := client.WarningCount(); count != 0 {
		t.Errorf("WarningCount: got %d, want 0", count)
	}
}

func TestClientServicesAccess(t *testing.T) {
	client := NewClient(WithAutoDetectServers(false))
	ctx := context.Background()

	// Before start, services should be nil
	if client.Manager() != nil {
		t.Error("Manager should be nil before Start")
	}
	if client.CompletionService() != nil {
		t.Error("CompletionService should be nil before Start")
	}
	if client.DiagnosticsService() != nil {
		t.Error("DiagnosticsService should be nil before Start")
	}
	if client.NavigationService() != nil {
		t.Error("NavigationService should be nil before Start")
	}
	if client.ActionsService() != nil {
		t.Error("ActionsService should be nil before Start")
	}

	// Start
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// After start, services should be available
	if client.Manager() == nil {
		t.Error("Manager should not be nil after Start")
	}
	if client.CompletionService() == nil {
		t.Error("CompletionService should not be nil after Start")
	}
	if client.DiagnosticsService() == nil {
		t.Error("DiagnosticsService should not be nil after Start")
	}
	if client.NavigationService() == nil {
		t.Error("NavigationService should not be nil after Start")
	}
	if client.ActionsService() == nil {
		t.Error("ActionsService should not be nil after Start")
	}

	client.Shutdown(ctx)
}

func TestClientRegisterServer(t *testing.T) {
	client := NewClient(WithAutoDetectServers(false))
	ctx := context.Background()

	// Register before start
	client.RegisterServer("go", ServerConfig{Command: "gopls"})

	langs := client.RegisteredLanguages()
	if len(langs) != 1 {
		t.Errorf("RegisteredLanguages before start: got %d, want 1", len(langs))
	}

	// Start
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Register after start
	client.RegisterServer("rust", ServerConfig{Command: "rust-analyzer"})

	langs = client.RegisteredLanguages()
	if len(langs) != 2 {
		t.Errorf("RegisteredLanguages after registration: got %d, want 2", len(langs))
	}

	client.Shutdown(ctx)
}

func TestClientServerStatus(t *testing.T) {
	client := NewClient(WithAutoDetectServers(false))
	ctx := context.Background()

	// Before start
	if status := client.ServerStatus("go"); status != ServerStatusStopped {
		t.Errorf("ServerStatus before start: got %v, want %v", status, ServerStatusStopped)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// After start, unregistered server should still be stopped
	if status := client.ServerStatus("unknown"); status != ServerStatusStopped {
		t.Errorf("ServerStatus for unknown: got %v, want %v", status, ServerStatusStopped)
	}

	client.Shutdown(ctx)
}

func TestClientConfigCopy(t *testing.T) {
	client := NewClient(
		WithServers(map[string]ServerConfig{
			"go": {Command: "gopls"},
		}),
	)

	// Get config
	config1 := client.Config()
	config1.Servers["rust"] = ServerConfig{Command: "rust-analyzer"}
	config1.FormatOnSave = true

	// Get config again - should not be affected by modifications to config1
	config2 := client.Config()

	if len(config2.Servers) != 1 {
		t.Errorf("Config copy leaked: got %d servers, want 1", len(config2.Servers))
	}

	if config2.FormatOnSave {
		t.Error("Config copy leaked: FormatOnSave should be false")
	}
}

func TestClientSetFormattingOptions(t *testing.T) {
	client := NewClient(WithAutoDetectServers(false))
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Default options
	opts := client.GetFormattingOptions()
	if opts.TabSize != 4 {
		t.Errorf("Default TabSize: got %d, want 4", opts.TabSize)
	}

	// Set new options
	newOpts := FormattingOptions{
		TabSize:      2,
		InsertSpaces: true,
	}
	client.SetFormattingOptions(newOpts)

	opts = client.GetFormattingOptions()
	if opts.TabSize != 2 {
		t.Errorf("Updated TabSize: got %d, want 2", opts.TabSize)
	}
	if !opts.InsertSpaces {
		t.Error("Updated InsertSpaces should be true")
	}

	client.Shutdown(ctx)
}

func TestClientSetFormatOnSave(t *testing.T) {
	client := NewClient(WithAutoDetectServers(false))
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Default is false
	config := client.Config()
	if config.FormatOnSave {
		t.Error("Default FormatOnSave should be false")
	}

	// Enable
	client.SetFormatOnSave(true)
	config = client.Config()
	if !config.FormatOnSave {
		t.Error("FormatOnSave should be true after enabling")
	}

	// Disable
	client.SetFormatOnSave(false)
	config = client.Config()
	if config.FormatOnSave {
		t.Error("FormatOnSave should be false after disabling")
	}

	client.Shutdown(ctx)
}

func TestClientIsAvailable(t *testing.T) {
	client := NewClient(
		WithServers(map[string]ServerConfig{
			"go": {Command: "gopls"},
		}),
		WithAutoDetectServers(false),
	)

	// Before start - not available
	if client.IsAvailable("/test.go") {
		t.Error("Should not be available before Start")
	}

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// After start with registered server
	if !client.IsAvailable("/test.go") {
		t.Error("Should be available for .go files")
	}

	// Unknown file type
	if client.IsAvailable("/test.xyz") {
		t.Error("Should not be available for unknown file types")
	}

	client.Shutdown(ctx)
}

func TestClientNeedsRenameConfirmation(t *testing.T) {
	// Default is true
	client := NewClient(WithAutoDetectServers(false))
	if !client.NeedsRenameConfirmation() {
		t.Error("Default should need rename confirmation")
	}

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !client.NeedsRenameConfirmation() {
		t.Error("Should need rename confirmation after start")
	}

	client.Shutdown(ctx)
}

func TestClientActiveSignature(t *testing.T) {
	client := NewClient(WithAutoDetectServers(false))

	// Before start
	if sig := client.ActiveSignature(); sig != nil {
		t.Error("ActiveSignature should be nil before Start")
	}

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// After start, should be nil until signature help is requested
	if sig := client.ActiveSignature(); sig != nil {
		t.Error("ActiveSignature should be nil initially")
	}

	// Clear should not panic
	client.ClearSignatureHelp()

	client.Shutdown(ctx)
}

func TestClientServerInfos(t *testing.T) {
	client := NewClient(WithAutoDetectServers(false))

	// Before start
	if infos := client.ServerInfos(); infos != nil {
		t.Error("ServerInfos should be nil before Start")
	}

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// After start with no running servers
	infos := client.ServerInfos()
	if len(infos) != 0 {
		t.Errorf("ServerInfos with no servers: got %d, want 0", len(infos))
	}

	client.Shutdown(ctx)
}

func TestClientDiagnosticsCallbacks(t *testing.T) {
	var receivedPath string
	var receivedDiags []Diagnostic

	client := NewClient(
		WithAutoDetectServers(false),
		WithClientDiagnosticsCallback(func(path string, diagnostics []Diagnostic) {
			receivedPath = path
			receivedDiags = diagnostics
		}),
	)

	// Callback should be stored
	if client.onDiagnostics == nil {
		t.Error("Diagnostics callback should be set")
	}

	_ = receivedPath
	_ = receivedDiags
}

func TestClientServerCallbacks(t *testing.T) {
	var startedLang string
	var stoppedLang string

	client := NewClient(
		WithAutoDetectServers(false),
		WithServerStartedCallback(func(languageID string) {
			startedLang = languageID
		}),
		WithServerStoppedCallback(func(languageID string, err error) {
			stoppedLang = languageID
		}),
	)

	// Callbacks should be stored
	if client.onServerStarted == nil {
		t.Error("Server started callback should be set")
	}
	if client.onServerStopped == nil {
		t.Error("Server stopped callback should be set")
	}

	_ = startedLang
	_ = stoppedLang
}

func TestClientStatusStringFunction(t *testing.T) {
	if ClientStatusString(ClientStatusReady) != "ready" {
		t.Error("ClientStatusString should return the status string")
	}
}

func TestClientWithClientConfig(t *testing.T) {
	config := ClientConfig{
		RequestTimeout:       20 * time.Second,
		CompletionMaxResults: 50,
		FormatOnSave:         true,
	}

	client := NewClient(WithClientConfig(config))

	got := client.Config()
	if got.RequestTimeout != 20*time.Second {
		t.Errorf("RequestTimeout: got %v, want 20s", got.RequestTimeout)
	}
	if got.CompletionMaxResults != 50 {
		t.Errorf("CompletionMaxResults: got %d, want 50", got.CompletionMaxResults)
	}
	if !got.FormatOnSave {
		t.Error("FormatOnSave should be true")
	}
}
