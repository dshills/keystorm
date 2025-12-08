package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialHelperFormat(t *testing.T) {
	helper, err := NewCredentialHelper("")
	if err != nil {
		t.Fatalf("NewCredentialHelper: %v", err)
	}

	cred := &Credential{
		Protocol: "https",
		Host:     "github.com",
		Path:     "user/repo",
		Username: "testuser",
		Password: "testpass",
	}

	formatted := helper.formatCredential(cred)

	// Should contain all fields
	if !contains(formatted, "protocol=https") {
		t.Error("expected protocol=https")
	}
	if !contains(formatted, "host=github.com") {
		t.Error("expected host=github.com")
	}
	if !contains(formatted, "path=user/repo") {
		t.Error("expected path=user/repo")
	}
	if !contains(formatted, "username=testuser") {
		t.Error("expected username=testuser")
	}
	if !contains(formatted, "password=testpass") {
		t.Error("expected password=testpass")
	}
}

func TestCredentialHelperFormatPartial(t *testing.T) {
	helper, err := NewCredentialHelper("")
	if err != nil {
		t.Fatalf("NewCredentialHelper: %v", err)
	}

	cred := &Credential{
		Protocol: "https",
		Host:     "github.com",
	}

	formatted := helper.formatCredential(cred)

	if !contains(formatted, "protocol=https") {
		t.Error("expected protocol=https")
	}
	if !contains(formatted, "host=github.com") {
		t.Error("expected host=github.com")
	}
	// Should not contain empty fields
	if contains(formatted, "path=") {
		t.Error("should not contain empty path")
	}
}

func TestParseCredentialOutput(t *testing.T) {
	output := `protocol=https
host=github.com
username=testuser
password=testtoken
`

	cred := parseCredentialOutput(output)

	if cred.Protocol != "https" {
		t.Errorf("expected https, got %s", cred.Protocol)
	}
	if cred.Host != "github.com" {
		t.Errorf("expected github.com, got %s", cred.Host)
	}
	if cred.Username != "testuser" {
		t.Errorf("expected testuser, got %s", cred.Username)
	}
	if cred.Password != "testtoken" {
		t.Errorf("expected testtoken, got %s", cred.Password)
	}
}

func TestParseCredentialOutputEmpty(t *testing.T) {
	cred := parseCredentialOutput("")

	if cred.Protocol != "" {
		t.Errorf("expected empty protocol, got %s", cred.Protocol)
	}
}

func TestSSHAgentIsRunning(t *testing.T) {
	agent := NewSSHAgent()

	// Just verify the method doesn't panic
	// Result depends on whether SSH agent is running
	_ = agent.IsRunning()
}

func TestListSSHKeysNoSSHDir(t *testing.T) {
	// Save and restore HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Set HOME to temp dir without .ssh
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	keys, err := ListSSHKeys()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestListSSHKeysSkipsPublicKeys(t *testing.T) {
	// Save and restore HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Set HOME to temp dir
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create .ssh directory
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}

	// Create test files that should be skipped
	skipFiles := []string{"id_rsa.pub", "known_hosts", "known_hosts.old", "authorized_keys", "config"}
	for _, name := range skipFiles {
		path := filepath.Join(sshDir, name)
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	keys, err := ListSSHKeys()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All files should be skipped
	if len(keys) != 0 {
		t.Errorf("expected 0 keys (all skipped), got %d", len(keys))
	}
}

func TestConfigureGitSSH(t *testing.T) {
	// Save and restore env
	origEnv := os.Getenv("GIT_SSH_COMMAND")
	defer os.Setenv("GIT_SSH_COMMAND", origEnv)

	// Get a valid SSH key path in ~/.ssh
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	keyPath := filepath.Join(homeDir, ".ssh", "id_rsa")

	if err := ConfigureGitSSH(keyPath); err != nil {
		t.Fatalf("configure git ssh: %v", err)
	}

	sshCmd := os.Getenv("GIT_SSH_COMMAND")
	if !contains(sshCmd, keyPath) {
		t.Errorf("expected GIT_SSH_COMMAND to contain key path")
	}
	if !contains(sshCmd, "-o IdentitiesOnly=yes") {
		t.Errorf("expected GIT_SSH_COMMAND to contain IdentitiesOnly")
	}
}

func TestTestConnection(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, dir, "file.txt", "content")
	gitCmd(t, dir, "add", "file.txt")
	gitCmd(t, dir, "commit", "-m", "initial")

	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	repo, err := mgr.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Test with non-existent remote (should fail)
	err = repo.TestConnection("origin")
	if err == nil {
		t.Error("expected error for non-existent remote")
	}
}

func TestNewCredentialHelper(t *testing.T) {
	helper, err := NewCredentialHelper("store")
	if err != nil {
		t.Fatalf("NewCredentialHelper(store): %v", err)
	}
	if helper.Helper != "store" {
		t.Errorf("expected store, got %s", helper.Helper)
	}

	helper2, err := NewCredentialHelper("")
	if err != nil {
		t.Fatalf("NewCredentialHelper(''): %v", err)
	}
	if helper2.Helper != "" {
		t.Errorf("expected empty, got %s", helper2.Helper)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
