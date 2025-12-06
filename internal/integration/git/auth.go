package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Credential represents git credentials.
type Credential struct {
	// Protocol is the protocol (e.g., "https", "ssh").
	Protocol string

	// Host is the remote host.
	Host string

	// Path is the repository path on the host.
	Path string

	// Username is the username.
	Username string

	// Password is the password or token.
	Password string
}

// CredentialHelper manages git credentials.
type CredentialHelper struct {
	// Helper is the credential helper name or path.
	Helper string
}

// NewCredentialHelper creates a credential helper wrapper.
func NewCredentialHelper(helper string) *CredentialHelper {
	return &CredentialHelper{Helper: helper}
}

// Get retrieves credentials for a host.
func (h *CredentialHelper) Get(protocol, host, path string) (*Credential, error) {
	input := fmt.Sprintf("protocol=%s\nhost=%s\n", protocol, host)
	if path != "" {
		input += fmt.Sprintf("path=%s\n", path)
	}
	input += "\n"

	args := []string{"credential"}
	if h.Helper != "" {
		args = append(args, h.Helper)
	}
	args = append(args, "fill")

	cmd := exec.Command("git", args...)
	cmd.Stdin = strings.NewReader(input)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("credential fill: %w", err)
	}

	return parseCredentialOutput(string(output)), nil
}

// Store stores credentials.
func (h *CredentialHelper) Store(cred *Credential) error {
	input := h.formatCredential(cred)

	args := []string{"credential"}
	if h.Helper != "" {
		args = append(args, h.Helper)
	}
	args = append(args, "approve")

	cmd := exec.Command("git", args...)
	cmd.Stdin = strings.NewReader(input)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("credential approve: %w", err)
	}

	return nil
}

// Erase removes stored credentials.
func (h *CredentialHelper) Erase(cred *Credential) error {
	input := h.formatCredential(cred)

	args := []string{"credential"}
	if h.Helper != "" {
		args = append(args, h.Helper)
	}
	args = append(args, "reject")

	cmd := exec.Command("git", args...)
	cmd.Stdin = strings.NewReader(input)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("credential reject: %w", err)
	}

	return nil
}

func (h *CredentialHelper) formatCredential(cred *Credential) string {
	var parts []string
	if cred.Protocol != "" {
		parts = append(parts, fmt.Sprintf("protocol=%s", cred.Protocol))
	}
	if cred.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", cred.Host))
	}
	if cred.Path != "" {
		parts = append(parts, fmt.Sprintf("path=%s", cred.Path))
	}
	if cred.Username != "" {
		parts = append(parts, fmt.Sprintf("username=%s", cred.Username))
	}
	if cred.Password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", cred.Password))
	}
	parts = append(parts, "")
	return strings.Join(parts, "\n")
}

func parseCredentialOutput(output string) *Credential {
	cred := &Credential{}
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "protocol":
			cred.Protocol = parts[1]
		case "host":
			cred.Host = parts[1]
		case "path":
			cred.Path = parts[1]
		case "username":
			cred.Username = parts[1]
		case "password":
			cred.Password = parts[1]
		}
	}
	return cred
}

// SSHKeyInfo represents SSH key information.
type SSHKeyInfo struct {
	// Path is the key file path.
	Path string

	// Type is the key type (e.g., "rsa", "ed25519").
	Type string

	// Fingerprint is the key fingerprint.
	Fingerprint string

	// Comment is the key comment.
	Comment string

	// HasPassphrase indicates if the key has a passphrase.
	HasPassphrase bool
}

// ListSSHKeys returns available SSH keys.
func ListSSHKeys() ([]SSHKeyInfo, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read .ssh dir: %w", err)
	}

	var keys []SSHKeyInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip public keys and known_hosts
		if strings.HasSuffix(name, ".pub") ||
			name == "known_hosts" ||
			name == "known_hosts.old" ||
			name == "authorized_keys" ||
			name == "config" {
			continue
		}

		keyPath := filepath.Join(sshDir, name)
		info, err := getSSHKeyInfo(keyPath)
		if err != nil {
			// Not a valid key, skip
			continue
		}
		keys = append(keys, *info)
	}

	return keys, nil
}

// getSSHKeyInfo gets information about an SSH key.
func getSSHKeyInfo(keyPath string) (*SSHKeyInfo, error) {
	// Check if file exists and is readable
	if _, err := os.Stat(keyPath); err != nil {
		return nil, err
	}

	// Get fingerprint using ssh-keygen
	pubPath := keyPath + ".pub"
	cmd := exec.Command("ssh-keygen", "-l", "-f", pubPath)
	output, err := cmd.Output()
	if err != nil {
		// Try the private key
		cmd = exec.Command("ssh-keygen", "-l", "-f", keyPath)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("get key fingerprint: %w", err)
		}
	}

	info := &SSHKeyInfo{
		Path: keyPath,
	}

	// Parse output: "2048 SHA256:... comment (RSA)"
	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) >= 2 {
		info.Fingerprint = parts[1]
	}
	if len(parts) >= 3 {
		// Last part is (TYPE)
		lastPart := parts[len(parts)-1]
		info.Type = strings.ToLower(strings.Trim(lastPart, "()"))

		// Comment is everything between fingerprint and type
		if len(parts) > 3 {
			info.Comment = strings.Join(parts[2:len(parts)-1], " ")
		}
	}

	// Check if key has passphrase
	info.HasPassphrase = checkKeyHasPassphrase(keyPath)

	return info, nil
}

// checkKeyHasPassphrase checks if an SSH key has a passphrase.
func checkKeyHasPassphrase(keyPath string) bool {
	// Try to read the key without passphrase
	cmd := exec.Command("ssh-keygen", "-y", "-P", "", "-f", keyPath)
	err := cmd.Run()
	// If it fails, the key likely has a passphrase
	return err != nil
}

// SSHAgent represents the SSH agent.
type SSHAgent struct{}

// NewSSHAgent creates an SSH agent wrapper.
func NewSSHAgent() *SSHAgent {
	return &SSHAgent{}
}

// IsRunning checks if the SSH agent is running.
func (a *SSHAgent) IsRunning() bool {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return false
	}
	_, err := os.Stat(socket)
	return err == nil
}

// ListKeys lists keys loaded in the agent.
func (a *SSHAgent) ListKeys() ([]string, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("SSH agent not running")
	}

	cmd := exec.Command("ssh-add", "-l")
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no keys
		if strings.Contains(string(output), "no identities") {
			return nil, nil
		}
		return nil, fmt.Errorf("list agent keys: %w", err)
	}

	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			keys = append(keys, line)
		}
	}
	return keys, nil
}

// AddKey adds a key to the agent.
func (a *SSHAgent) AddKey(keyPath string) error {
	if !a.IsRunning() {
		return fmt.Errorf("SSH agent not running")
	}

	cmd := exec.Command("ssh-add", keyPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add key to agent: %w", err)
	}

	return nil
}

// RemoveKey removes a key from the agent.
func (a *SSHAgent) RemoveKey(keyPath string) error {
	if !a.IsRunning() {
		return fmt.Errorf("SSH agent not running")
	}

	cmd := exec.Command("ssh-add", "-d", keyPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove key from agent: %w", err)
	}

	return nil
}

// RemoveAllKeys removes all keys from the agent.
func (a *SSHAgent) RemoveAllKeys() error {
	if !a.IsRunning() {
		return fmt.Errorf("SSH agent not running")
	}

	cmd := exec.Command("ssh-add", "-D")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove all keys from agent: %w", err)
	}

	return nil
}

// ConfigureGitSSH configures git to use a specific SSH key.
func ConfigureGitSSH(keyPath string) error {
	// Set GIT_SSH_COMMAND environment variable
	sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", keyPath)
	return os.Setenv("GIT_SSH_COMMAND", sshCmd)
}

// GetGitCredentialHelper returns the configured credential helper.
func GetGitCredentialHelper() (string, error) {
	cmd := exec.Command("git", "config", "--get", "credential.helper")
	output, err := cmd.Output()
	if err != nil {
		// No helper configured is not an error
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// SetGitCredentialHelper sets the credential helper.
func SetGitCredentialHelper(helper string, global bool) error {
	args := []string{"config"}
	if global {
		args = append(args, "--global")
	}
	args = append(args, "credential.helper", helper)

	cmd := exec.Command("git", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set credential helper: %w", err)
	}
	return nil
}

// TestConnection tests connectivity to a remote.
func (r *Repository) TestConnection(remote string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, err := r.git("ls-remote", "--exit-code", remote)
	if err != nil {
		if strings.Contains(err.Error(), "Authentication failed") ||
			strings.Contains(err.Error(), "Permission denied") {
			return ErrAuthenticationFailed
		}
		return fmt.Errorf("test connection: %w", err)
	}
	return nil
}
