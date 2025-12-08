package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCredentialHelper(t *testing.T) {
	tests := []struct {
		name    string
		helper  string
		wantErr bool
	}{
		// Valid helpers (whitelisted)
		{"empty helper", "", false},
		{"cache helper", "cache", false},
		{"store helper", "store", false},
		{"osxkeychain", "osxkeychain", false},
		{"manager", "manager", false},
		{"manager-core", "manager-core", false},
		{"wincred", "wincred", false},
		{"gnome-keyring", "gnome-keyring", false},
		{"libsecret", "libsecret", false},

		// Valid simple names (not whitelisted but safe)
		{"custom-helper", "custom-helper", false},
		{"my_helper", "my_helper", false},
		{"helper123", "helper123", false},

		// INJECTION ATTEMPTS - these must all fail
		{"semicolon injection", "cache; rm -rf /", true},
		{"ampersand injection", "cache && whoami", true},
		{"pipe injection", "cache | cat /etc/passwd", true},
		{"subcommand injection", "cache$(whoami)", true},
		{"backtick injection", "cache`whoami`", true},
		{"newline injection", "cache\nwhoami", true},
		{"carriage return injection", "cache\rwhoami", true},
		{"redirect injection", "cache > /tmp/output", true},
		{"input redirect", "cache < /etc/passwd", true},
		{"exclamation injection", "cache!whoami", true},
		{"parenthesis injection", "cache(whoami)", true},
		{"brace injection", "cache{whoami}", true},
		{"single quote injection", "cache'whoami'", true},
		{"double quote injection", "cache\"whoami\"", true},
		{"backslash injection", "cache\\whoami", true},
		{"dollar sign injection", "cache$HOME", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredentialHelper(tt.helper)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCredentialHelper(%q) error = %v, wantErr %v", tt.helper, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProtocol(t *testing.T) {
	tests := []struct {
		protocol string
		wantErr  bool
	}{
		{"https", false},
		{"http", false},
		{"ssh", false},
		{"git", false},
		{"HTTPS", false}, // Case insensitive
		{"HTTP", false},

		// Invalid protocols
		{"ftp", true},
		{"file", true},
		{"https; rm -rf /", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			err := validateProtocol(tt.protocol)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProtocol(%q) error = %v, wantErr %v", tt.protocol, err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{"valid github", "github.com", false},
		{"valid gitlab", "gitlab.com", false},
		{"valid with subdomain", "api.github.com", false},
		{"valid with port", "github.com:443", false},
		{"empty host", "", false}, // Empty is allowed (optional)
		{"localhost", "localhost", false},
		{"ip address", "192.168.1.1", false},

		// INJECTION ATTEMPTS
		{"newline injection", "github.com\nusername=evil", true},
		{"carriage return injection", "github.com\rusername=evil", true},
		{"space in hostname", "github.com foo", true},
		{"semicolon in hostname", "github.com;evil.com", true},
		{"special chars", "github.com<script>", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHostname(tt.host)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateHostname(%q) error = %v, wantErr %v", tt.host, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid repo path", "owner/repo", false},
		{"valid repo with dots", "owner/repo.git", false},
		{"empty path", "", false}, // Empty is allowed (optional)
		{"valid underscore", "my_org/my_repo", false},
		{"valid dash", "my-org/my-repo", false},

		// INJECTION ATTEMPTS
		{"newline injection", "owner/repo\npassword=evil", true},
		{"carriage return injection", "owner/repo\rpassword=evil", true},
		{"path traversal", "../../../etc/passwd", true},
		{"special chars", "owner/repo<script>", true},
		{"space in path", "owner/repo name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSSHKeyPath(t *testing.T) {
	// Get home directory for testing
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	sshDir := filepath.Join(homeDir, ".ssh")

	tests := []struct {
		name    string
		keyPath string
		wantErr bool
	}{
		// Valid paths (relative to .ssh)
		{"valid ssh key", filepath.Join(sshDir, "id_rsa"), false},
		{"valid ed25519 key", filepath.Join(sshDir, "id_ed25519"), false},
		{"valid custom key", filepath.Join(sshDir, "my_key"), false},

		// Valid system paths
		{"etc ssh key", "/etc/ssh/ssh_host_rsa_key", false},

		// Invalid paths
		{"empty path", "", true},
		{"relative path", "id_rsa", true},
		{"outside ssh dir", "/tmp/id_rsa", true},
		{"path traversal", filepath.Join(sshDir, "..", "..", "etc", "passwd"), true},

		// INJECTION ATTEMPTS
		{"semicolon injection", filepath.Join(sshDir, "id_rsa; rm -rf /"), true},
		{"newline injection", filepath.Join(sshDir, "id_rsa\nmalicious"), true},
		{"backtick injection", filepath.Join(sshDir, "id_rsa`whoami`"), true},
		{"dollar injection", filepath.Join(sshDir, "id_rsa$HOME"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSSHKeyPath(tt.keyPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSSHKeyPath(%q) error = %v, wantErr %v", tt.keyPath, err, tt.wantErr)
			}
		})
	}
}

func TestNewCredentialHelper_Validation(t *testing.T) {
	// Valid helpers should succeed
	helper, err := NewCredentialHelper("cache")
	if err != nil {
		t.Errorf("NewCredentialHelper(cache) failed: %v", err)
	}
	if helper == nil {
		t.Error("NewCredentialHelper(cache) returned nil helper")
	}

	// Invalid helpers should fail
	_, err = NewCredentialHelper("cache; rm -rf /")
	if err == nil {
		t.Error("NewCredentialHelper with injection should fail")
	}
}

func TestCredentialHelper_Get_Validation(t *testing.T) {
	helper, _ := NewCredentialHelper("")

	// Test validation directly - don't call helper.Get with valid inputs
	// as that will actually run git credential and wait forever.

	// Invalid protocol should fail validation immediately
	_, err := helper.Get("ftp", "github.com", "owner/repo")
	if err != ErrInvalidProtocol {
		t.Errorf("invalid protocol should fail with ErrInvalidProtocol, got: %v", err)
	}

	// Invalid hostname should fail validation immediately
	_, err = helper.Get("https", "github.com\nusername=evil", "owner/repo")
	if err != ErrInvalidHostname {
		t.Errorf("invalid hostname should fail with ErrInvalidHostname, got: %v", err)
	}

	// Invalid path should fail validation immediately
	_, err = helper.Get("https", "github.com", "../../../etc/passwd")
	if err != ErrInvalidPath {
		t.Errorf("invalid path should fail with ErrInvalidPath, got: %v", err)
	}
}

func TestCredentialHelper_Store_Validation(t *testing.T) {
	helper, _ := NewCredentialHelper("")

	// Credential with newline in username should fail
	cred := &Credential{
		Protocol: "https",
		Host:     "github.com",
		Username: "user\npassword=evil",
		Password: "secret",
	}
	err := helper.Store(cred)
	if err == nil {
		t.Error("credential with newline in username should fail")
	}

	// Credential with newline in password should fail
	cred = &Credential{
		Protocol: "https",
		Host:     "github.com",
		Username: "user",
		Password: "secret\nusername=evil",
	}
	err = helper.Store(cred)
	if err == nil {
		t.Error("credential with newline in password should fail")
	}
}

func TestSetGitCredentialHelper_Validation(t *testing.T) {
	// Don't actually set the helper, just test validation

	// Valid helper - would succeed if we let it run
	err := validateCredentialHelper("cache")
	if err != nil {
		t.Errorf("cache should be valid: %v", err)
	}

	// Invalid helper with injection should fail
	err = validateCredentialHelper("cache; rm -rf /")
	if err == nil {
		t.Error("injection attempt should fail")
	}
}
