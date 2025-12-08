package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Security errors for auth operations.
var (
	// ErrInvalidCredentialHelper indicates the helper name contains invalid characters.
	ErrInvalidCredentialHelper = errors.New("invalid credential helper: contains disallowed characters")

	// ErrInvalidHostname indicates the hostname contains invalid characters.
	ErrInvalidHostname = errors.New("invalid hostname: contains disallowed characters")

	// ErrInvalidProtocol indicates the protocol contains invalid characters.
	ErrInvalidProtocol = errors.New("invalid protocol: must be https, http, or ssh")

	// ErrInvalidPath indicates the path contains invalid characters.
	ErrInvalidPath = errors.New("invalid path: contains disallowed characters")

	// ErrInvalidKeyPath indicates the SSH key path is invalid.
	ErrInvalidKeyPath = errors.New("invalid SSH key path")
)

// allowedCredentialHelpers is a whitelist of known-safe credential helpers.
// Additional helpers can be validated if they match a safe path pattern.
var allowedCredentialHelpers = map[string]bool{
	"":                       true, // Empty means use system default
	"cache":                  true,
	"store":                  true,
	"osxkeychain":            true,
	"manager":                true,
	"manager-core":           true,
	"wincred":                true,
	"gnome-keyring":          true,
	"libsecret":              true,
	"credential-osxkeychain": true,
	"credential-wincred":     true,
}

// validProtocols are the allowed Git protocols.
var validProtocols = map[string]bool{
	"https": true,
	"http":  true,
	"ssh":   true,
	"git":   true,
}

// hostnameRegex validates hostnames (RFC 1123 with optional port).
var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?(:[0-9]+)?$`)

// pathRegex validates repository paths (alphanumeric, dashes, underscores, slashes, dots).
var pathRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_./]*$`)

// credentialHelperPathRegex validates credential helper paths.
// Allows absolute paths to executables.
var credentialHelperPathRegex = regexp.MustCompile(`^(/[a-zA-Z0-9\-_./]+|[a-zA-Z]:\\[a-zA-Z0-9\-_./\\]+)$`)

// validateCredentialHelper validates a credential helper name or path.
// Returns an error if the helper name could be used for command injection.
func validateCredentialHelper(helper string) error {
	// Empty helper uses system default
	if helper == "" {
		return nil
	}

	// Check whitelist first
	if allowedCredentialHelpers[helper] {
		return nil
	}

	// Check if it's a valid path to an executable
	if credentialHelperPathRegex.MatchString(helper) {
		// Verify the path exists and is executable
		info, err := os.Stat(helper)
		if err != nil {
			return ErrInvalidCredentialHelper
		}
		// Check it's not a directory
		if info.IsDir() {
			return ErrInvalidCredentialHelper
		}
		return nil
	}

	// Disallow shell metacharacters that could enable injection
	// These characters have special meaning in shells
	dangerousChars := []string{
		";", "&", "|", "$", "`", "(", ")", "{", "}",
		"<", ">", "!", "\n", "\r", "\"", "'", "\\",
	}
	for _, char := range dangerousChars {
		if strings.Contains(helper, char) {
			return ErrInvalidCredentialHelper
		}
	}

	// Helper name should be simple alphanumeric with optional dashes
	simpleNameRegex := regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
	if !simpleNameRegex.MatchString(helper) {
		return ErrInvalidCredentialHelper
	}

	return nil
}

// validateProtocol validates a protocol string.
func validateProtocol(protocol string) error {
	if !validProtocols[strings.ToLower(protocol)] {
		return ErrInvalidProtocol
	}
	return nil
}

// validateHostname validates a hostname string.
func validateHostname(host string) error {
	if host == "" {
		return nil
	}
	// Check for newlines (credential injection)
	if strings.ContainsAny(host, "\n\r") {
		return ErrInvalidHostname
	}
	if !hostnameRegex.MatchString(host) {
		return ErrInvalidHostname
	}
	return nil
}

// validatePath validates a repository path string.
func validatePath(path string) error {
	if path == "" {
		return nil
	}
	// Check for newlines (credential injection)
	if strings.ContainsAny(path, "\n\r") {
		return ErrInvalidPath
	}
	if !pathRegex.MatchString(path) {
		return ErrInvalidPath
	}
	// Prevent path traversal
	if strings.Contains(path, "..") {
		return ErrInvalidPath
	}
	return nil
}

// validateSSHKeyPath validates an SSH key path.
func validateSSHKeyPath(keyPath string) error {
	if keyPath == "" {
		return ErrInvalidKeyPath
	}

	// Must be an absolute path
	if !filepath.IsAbs(keyPath) {
		return ErrInvalidKeyPath
	}

	// Check for path traversal
	cleanPath := filepath.Clean(keyPath)
	if cleanPath != keyPath && cleanPath+"/" != keyPath {
		// Path was modified by Clean, might contain traversal
		// Allow trailing slash difference
		if strings.Contains(keyPath, "..") {
			return ErrInvalidKeyPath
		}
	}

	// Must be in user's home directory or /etc/ssh
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ErrInvalidKeyPath
	}
	sshDir := filepath.Join(homeDir, ".ssh")

	// Check if path is within allowed directories
	allowedPaths := []string{sshDir, "/etc/ssh"}
	inAllowed := false
	for _, allowed := range allowedPaths {
		if strings.HasPrefix(cleanPath, allowed) {
			inAllowed = true
			break
		}
	}
	if !inAllowed {
		return ErrInvalidKeyPath
	}

	// Check for shell metacharacters
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">", "!", "\n", "\r", "\"", "'"}
	for _, char := range dangerousChars {
		if strings.Contains(keyPath, char) {
			return ErrInvalidKeyPath
		}
	}

	return nil
}

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
// Returns an error if the helper name is invalid or contains dangerous characters.
func NewCredentialHelper(helper string) (*CredentialHelper, error) {
	if err := validateCredentialHelper(helper); err != nil {
		return nil, err
	}
	return &CredentialHelper{Helper: helper}, nil
}

// Get retrieves credentials for a host.
// All input parameters are validated to prevent credential injection attacks.
func (h *CredentialHelper) Get(protocol, host, path string) (*Credential, error) {
	// SECURITY: Validate all inputs to prevent injection
	if err := validateProtocol(protocol); err != nil {
		return nil, err
	}
	if err := validateHostname(host); err != nil {
		return nil, err
	}
	if err := validatePath(path); err != nil {
		return nil, err
	}

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
// The credential is validated to prevent injection attacks.
func (h *CredentialHelper) Store(cred *Credential) error {
	// SECURITY: Validate credential before storing
	if err := h.validateCredential(cred); err != nil {
		return err
	}

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
// The credential is validated to prevent injection attacks.
func (h *CredentialHelper) Erase(cred *Credential) error {
	// SECURITY: Validate credential before erasing
	if err := h.validateCredential(cred); err != nil {
		return err
	}

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

// validateCredential validates a credential to prevent injection.
func (h *CredentialHelper) validateCredential(cred *Credential) error {
	if cred == nil {
		return errors.New("credential is nil")
	}
	if cred.Protocol != "" {
		if err := validateProtocol(cred.Protocol); err != nil {
			return err
		}
	}
	if cred.Host != "" {
		if err := validateHostname(cred.Host); err != nil {
			return err
		}
	}
	if cred.Path != "" {
		if err := validatePath(cred.Path); err != nil {
			return err
		}
	}
	// Username and password can contain special chars, but no newlines
	if strings.ContainsAny(cred.Username, "\n\r") {
		return errors.New("username contains invalid characters")
	}
	if strings.ContainsAny(cred.Password, "\n\r") {
		return errors.New("password contains invalid characters")
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Exit code 1 with "no identities" means no keys loaded (not an error)
		outputStr := string(output)
		if strings.Contains(outputStr, "no identities") ||
			strings.Contains(outputStr, "The agent has no identities") {
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
// The key path is validated to prevent command injection.
func (a *SSHAgent) AddKey(keyPath string) error {
	// SECURITY: Validate key path
	if err := validateSSHKeyPath(keyPath); err != nil {
		return err
	}

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
// The key path is validated to prevent command injection.
func (a *SSHAgent) RemoveKey(keyPath string) error {
	// SECURITY: Validate key path
	if err := validateSSHKeyPath(keyPath); err != nil {
		return err
	}

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
// The key path is validated to prevent command injection.
func ConfigureGitSSH(keyPath string) error {
	// SECURITY: Validate key path
	if err := validateSSHKeyPath(keyPath); err != nil {
		return err
	}

	// Set GIT_SSH_COMMAND environment variable
	// Quote the path to handle spaces and special characters
	sshCmd := fmt.Sprintf("ssh -i %q -o IdentitiesOnly=yes", keyPath)
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
// The helper name is validated to prevent command injection.
func SetGitCredentialHelper(helper string, global bool) error {
	// SECURITY: Validate credential helper name
	if err := validateCredentialHelper(helper); err != nil {
		return err
	}

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
