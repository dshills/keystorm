package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPermissionChecker(t *testing.T) {
	pc := NewPermissionChecker("test-plugin")
	if pc == nil {
		t.Fatal("NewPermissionChecker returned nil")
	}
	if pc.pluginName != "test-plugin" {
		t.Errorf("pluginName = %q, want %q", pc.pluginName, "test-plugin")
	}
}

func TestPermissionCheckerGrant(t *testing.T) {
	pc := NewPermissionChecker("test")

	pc.Grant(CapabilityFileRead)
	if !pc.HasCapability(CapabilityFileRead) {
		t.Error("HasCapability(FileRead) = false after Grant")
	}
}

func TestPermissionCheckerRevoke(t *testing.T) {
	pc := NewPermissionChecker("test")

	pc.Grant(CapabilityFileRead)
	pc.Revoke(CapabilityFileRead)
	if pc.HasCapability(CapabilityFileRead) {
		t.Error("HasCapability(FileRead) = true after Revoke")
	}
}

func TestPermissionCheckerGrantAll(t *testing.T) {
	pc := NewPermissionChecker("test")

	caps := []Capability{CapabilityFileRead, CapabilityNetwork, CapabilityClipboard}
	pc.GrantAll(caps)

	for _, cap := range caps {
		if !pc.HasCapability(cap) {
			t.Errorf("HasCapability(%q) = false", cap)
		}
	}
}

func TestPermissionCheckerHasCapabilityHierarchy(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Grant parent capability
	pc.Grant(CapabilityEditor)

	// Should imply child capabilities
	if !pc.HasCapability(CapabilityBuffer) {
		t.Error("HasCapability(Buffer) = false (should be implied by Editor)")
	}
	if !pc.HasCapability(CapabilityCursor) {
		t.Error("HasCapability(Cursor) = false (should be implied by Editor)")
	}
}

func TestPermissionCheckerCheckCapability(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Without capability
	err := pc.CheckCapability(CapabilityFileRead)
	if err == nil {
		t.Error("CheckCapability should fail without capability")
	}

	// With capability
	pc.Grant(CapabilityFileRead)
	err = pc.CheckCapability(CapabilityFileRead)
	if err != nil {
		t.Errorf("CheckCapability with capability error = %v", err)
	}
}

func TestPermissionCheckerCapabilities(t *testing.T) {
	pc := NewPermissionChecker("test")

	pc.Grant(CapabilityFileRead)
	pc.Grant(CapabilityNetwork)

	caps := pc.Capabilities()
	if len(caps) != 2 {
		t.Errorf("Capabilities() returned %d items, want 2", len(caps))
	}
}

func TestPermissionCheckerCheckFileRead(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Without capability
	err := pc.CheckFileRead("/some/path")
	if err == nil {
		t.Error("CheckFileRead should fail without capability")
	}

	// With capability
	pc.Grant(CapabilityFileRead)
	err = pc.CheckFileRead("/some/path")
	if err != nil {
		t.Errorf("CheckFileRead with capability error = %v", err)
	}
}

func TestPermissionCheckerCheckFileWrite(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Without capability
	err := pc.CheckFileWrite("/some/path")
	if err == nil {
		t.Error("CheckFileWrite should fail without capability")
	}

	// With capability
	pc.Grant(CapabilityFileWrite)
	err = pc.CheckFileWrite("/some/path")
	if err != nil {
		t.Errorf("CheckFileWrite with capability error = %v", err)
	}
}

func TestPermissionCheckerWorkspacePath(t *testing.T) {
	tmpDir := t.TempDir()
	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityFileRead)
	pc.SetWorkspacePath(tmpDir)

	// Path within workspace
	inWorkspace := filepath.Join(tmpDir, "file.txt")
	err := pc.CheckFileRead(inWorkspace)
	if err != nil {
		t.Errorf("CheckFileRead within workspace error = %v", err)
	}

	// Path outside workspace
	outsideWorkspace := "/tmp/outside"
	err = pc.CheckFileRead(outsideWorkspace)
	if err == nil {
		t.Error("CheckFileRead outside workspace should fail")
	}
}

func TestPermissionCheckerAllowedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	os.MkdirAll(allowedDir, 0755)

	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityFileRead)
	pc.AllowPath(allowedDir)

	// Allowed path
	allowedFile := filepath.Join(allowedDir, "file.txt")
	err := pc.CheckFileRead(allowedFile)
	if err != nil {
		t.Errorf("CheckFileRead on allowed path error = %v", err)
	}

	// Not allowed path
	notAllowed := filepath.Join(tmpDir, "other", "file.txt")
	err = pc.CheckFileRead(notAllowed)
	if err == nil {
		t.Error("CheckFileRead on non-allowed path should fail")
	}
}

func TestPermissionCheckerBlockedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	blockedDir := filepath.Join(tmpDir, "blocked")
	os.MkdirAll(blockedDir, 0755)

	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityFileRead)
	pc.BlockPath(blockedDir)

	// Blocked path
	blockedFile := filepath.Join(blockedDir, "file.txt")
	err := pc.CheckFileRead(blockedFile)
	if err == nil {
		t.Error("CheckFileRead on blocked path should fail")
	}

	// Non-blocked path
	allowedFile := filepath.Join(tmpDir, "allowed.txt")
	err = pc.CheckFileRead(allowedFile)
	if err != nil {
		t.Errorf("CheckFileRead on non-blocked path error = %v", err)
	}
}

func TestPermissionCheckerBlockedTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()

	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityFileRead)
	pc.AllowPath(tmpDir)
	pc.BlockPath(filepath.Join(tmpDir, "secret"))

	// Allowed
	allowed := filepath.Join(tmpDir, "public.txt")
	err := pc.CheckFileRead(allowed)
	if err != nil {
		t.Errorf("CheckFileRead on allowed path error = %v", err)
	}

	// Blocked (even though parent is allowed)
	blocked := filepath.Join(tmpDir, "secret", "data.txt")
	err = pc.CheckFileRead(blocked)
	if err == nil {
		t.Error("CheckFileRead on blocked path should fail (even if parent allowed)")
	}
}

func TestPermissionCheckerCheckNetwork(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Without capability
	err := pc.CheckNetwork("example.com")
	if err == nil {
		t.Error("CheckNetwork should fail without capability")
	}

	// With capability
	pc.Grant(CapabilityNetwork)
	err = pc.CheckNetwork("example.com")
	if err != nil {
		t.Errorf("CheckNetwork with capability error = %v", err)
	}
}

func TestPermissionCheckerAllowedHosts(t *testing.T) {
	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityNetwork)
	pc.AllowHost("api.example.com")

	// Allowed host
	err := pc.CheckNetwork("api.example.com")
	if err != nil {
		t.Errorf("CheckNetwork on allowed host error = %v", err)
	}

	// Not allowed host
	err = pc.CheckNetwork("other.com")
	if err == nil {
		t.Error("CheckNetwork on non-allowed host should fail")
	}
}

func TestPermissionCheckerBlockedHosts(t *testing.T) {
	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityNetwork)
	pc.BlockHost("malware.com")

	// Blocked host
	err := pc.CheckNetwork("malware.com")
	if err == nil {
		t.Error("CheckNetwork on blocked host should fail")
	}

	// Non-blocked host
	err = pc.CheckNetwork("safe.com")
	if err != nil {
		t.Errorf("CheckNetwork on non-blocked host error = %v", err)
	}
}

func TestPermissionCheckerWildcardHosts(t *testing.T) {
	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityNetwork)
	pc.AllowHost("*.example.com")

	// Subdomain match
	err := pc.CheckNetwork("api.example.com")
	if err != nil {
		t.Errorf("CheckNetwork on subdomain error = %v", err)
	}

	// Deep subdomain match
	err = pc.CheckNetwork("deep.api.example.com")
	if err != nil {
		t.Errorf("CheckNetwork on deep subdomain error = %v", err)
	}

	// No match
	err = pc.CheckNetwork("other.com")
	if err == nil {
		t.Error("CheckNetwork on non-matching host should fail")
	}
}

func TestPermissionCheckerNetworkWithPort(t *testing.T) {
	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityNetwork)
	pc.AllowHost("api.example.com")

	// Host with port
	err := pc.CheckNetwork("api.example.com:443")
	if err != nil {
		t.Errorf("CheckNetwork with port error = %v", err)
	}
}

func TestPermissionCheckerCheckShell(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Without capability
	err := pc.CheckShell("ls -la")
	if err == nil {
		t.Error("CheckShell should fail without capability")
	}

	// With capability
	pc.Grant(CapabilityShell)
	err = pc.CheckShell("ls -la")
	if err != nil {
		t.Errorf("CheckShell with capability error = %v", err)
	}
}

func TestPermissionCheckerCheckProcess(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Without capability
	err := pc.CheckProcess("/bin/bash")
	if err == nil {
		t.Error("CheckProcess should fail without capability")
	}

	// With capability
	pc.Grant(CapabilityProcess)
	err = pc.CheckProcess("/bin/bash")
	if err != nil {
		t.Errorf("CheckProcess with capability error = %v", err)
	}
}

func TestPermissionCheckerCheckClipboard(t *testing.T) {
	pc := NewPermissionChecker("test")

	// Without capability
	err := pc.CheckClipboard("read")
	if err == nil {
		t.Error("CheckClipboard should fail without capability")
	}

	// With capability
	pc.Grant(CapabilityClipboard)
	err = pc.CheckClipboard("read")
	if err != nil {
		t.Errorf("CheckClipboard with capability error = %v", err)
	}
}

func TestPermissionCheckerApplyPermissionSet(t *testing.T) {
	pc := NewPermissionChecker("test")

	set := &PermissionSet{
		Capabilities: []Capability{CapabilityFileRead, CapabilityNetwork},
		AllowedPaths: []string{"/allowed"},
		BlockedPaths: []string{"/blocked"},
		AllowedHosts: []string{"api.example.com"},
		BlockedHosts: []string{"blocked.com"},
	}

	pc.ApplyPermissionSet(set)

	if !pc.HasCapability(CapabilityFileRead) {
		t.Error("HasCapability(FileRead) = false after ApplyPermissionSet")
	}
	if !pc.HasCapability(CapabilityNetwork) {
		t.Error("HasCapability(Network) = false after ApplyPermissionSet")
	}
}

func TestPermissionCheckerReset(t *testing.T) {
	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityFileRead)
	pc.AllowPath("/allowed")
	pc.BlockPath("/blocked")
	pc.AllowHost("example.com")
	pc.BlockHost("blocked.com")

	pc.Reset()

	if pc.HasCapability(CapabilityFileRead) {
		t.Error("HasCapability should be false after Reset")
	}
	if len(pc.Capabilities()) != 0 {
		t.Error("Capabilities should be empty after Reset")
	}
}

func TestMatchHost(t *testing.T) {
	tests := []struct {
		host     string
		pattern  string
		expected bool
	}{
		{"example.com", "example.com", true},
		{"example.com", "other.com", false},
		{"api.example.com", "*.example.com", true},
		{"deep.api.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false}, // No subdomain
		{"notexample.com", "*.example.com", false},
		// Case insensitivity
		{"Example.Com", "example.com", true},
		{"API.Example.COM", "*.example.com", true},
	}

	for _, tt := range tests {
		got := matchHost(tt.host, tt.pattern)
		if got != tt.expected {
			t.Errorf("matchHost(%q, %q) = %v, want %v", tt.host, tt.pattern, got, tt.expected)
		}
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Regular host:port
		{"example.com:443", "example.com"},
		{"example.com:80", "example.com"},
		// IPv6 with port
		{"[::1]:8080", "::1"},
		{"[2001:db8::1]:443", "2001:db8::1"},
		// Plain host without port
		{"example.com", "example.com"},
		// IPv6 without port (bracketed)
		{"[::1]", "::1"},
		// IPv6 without brackets (no port)
		{"::1", "::1"},
	}

	for _, tt := range tests {
		got := extractHost(tt.input)
		if got != tt.expected {
			t.Errorf("extractHost(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestPermissionCheckerIPv6Network(t *testing.T) {
	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityNetwork)
	pc.AllowHost("::1")

	// IPv6 loopback with port
	err := pc.CheckNetwork("[::1]:8080")
	if err != nil {
		t.Errorf("CheckNetwork([::1]:8080) error = %v", err)
	}

	// IPv6 without port
	err = pc.CheckNetwork("::1")
	if err != nil {
		t.Errorf("CheckNetwork(::1) error = %v", err)
	}

	// Different IPv6 address should fail
	err = pc.CheckNetwork("[2001:db8::1]:443")
	if err == nil {
		t.Error("CheckNetwork([2001:db8::1]:443) should fail")
	}
}

func TestIsWithinPath(t *testing.T) {
	tests := []struct {
		target   string
		base     string
		expected bool
	}{
		// Basic containment
		{"/tmp/foo/bar", "/tmp/foo", true},
		{"/tmp/foo", "/tmp/foo", true}, // Equal paths
		{"/tmp/foo/bar/baz", "/tmp/foo", true},
		// Not contained
		{"/tmp/other", "/tmp/foo", false},
		{"/etc/passwd", "/tmp", false},
		// Edge case: similar prefix but not actually contained
		{"/tmp/foobar", "/tmp/foo", false},
		{"/tmp/foo-suffix", "/tmp/foo", false},
	}

	for _, tt := range tests {
		got := isWithinPath(tt.target, tt.base)
		if got != tt.expected {
			t.Errorf("isWithinPath(%q, %q) = %v, want %v", tt.target, tt.base, got, tt.expected)
		}
	}
}

func TestPermissionCheckerBlockedPathEdgeCase(t *testing.T) {
	tmpDir := t.TempDir()
	blockedDir := filepath.Join(tmpDir, "blocked")
	similarDir := filepath.Join(tmpDir, "blockedfiles") // Similar name but different
	os.MkdirAll(blockedDir, 0755)
	os.MkdirAll(similarDir, 0755)

	pc := NewPermissionChecker("test")
	pc.Grant(CapabilityFileRead)
	pc.BlockPath(blockedDir)

	// File in blocked directory should fail
	err := pc.CheckFileRead(filepath.Join(blockedDir, "secret.txt"))
	if err == nil {
		t.Error("CheckFileRead in blocked dir should fail")
	}

	// File in similarly-named directory should succeed
	err = pc.CheckFileRead(filepath.Join(similarDir, "file.txt"))
	if err != nil {
		t.Errorf("CheckFileRead in similar dir error = %v", err)
	}
}
