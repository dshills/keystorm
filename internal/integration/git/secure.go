package git

import (
	"runtime"
	"sync"
)

// SecureBytes holds sensitive byte data that is zeroed when no longer needed.
// It provides methods to safely handle passwords, tokens, and other secrets.
//
// SECURITY NOTE: While this implementation zeros memory on cleanup, Go's
// garbage collector may have already copied the data during normal operations.
// This provides defense-in-depth but is not a complete solution. For maximum
// security, use OS-level protected memory or hardware security modules.
//
// IMPORTANT: Go strings are immutable and stored in read-only memory. We cannot
// zero original string passwords. The secure approach is to:
// 1. Use SecureBytes to store sensitive data internally
// 2. Zero the byte slice copy when done
// 3. Avoid storing passwords as plain strings in long-lived structures
type SecureBytes struct {
	data []byte
	mu   sync.RWMutex
}

// NewSecureBytes creates a new SecureBytes from the given data.
// The original data is copied and should be zeroed by the caller if sensitive.
func NewSecureBytes(data []byte) *SecureBytes {
	if data == nil {
		return &SecureBytes{}
	}
	// Make a copy so we own the data
	copied := make([]byte, len(data))
	copy(copied, data)
	sb := &SecureBytes{data: copied}
	// Register a finalizer to zero the data when garbage collected
	runtime.SetFinalizer(sb, func(s *SecureBytes) {
		s.Clear()
	})
	return sb
}

// NewSecureBytesFromString creates a new SecureBytes from a string.
// NOTE: The original string cannot be zeroed due to Go's immutable strings.
// This creates a byte slice copy that can be securely cleared.
func NewSecureBytesFromString(s string) *SecureBytes {
	return NewSecureBytes([]byte(s))
}

// Bytes returns the underlying bytes.
// The returned slice should not be stored or modified.
func (sb *SecureBytes) Bytes() []byte {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.data
}

// String returns the data as a string.
// The returned string should not be stored long-term.
func (sb *SecureBytes) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return string(sb.data)
}

// Len returns the length of the secure data.
func (sb *SecureBytes) Len() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return len(sb.data)
}

// IsEmpty returns true if the secure data is empty.
func (sb *SecureBytes) IsEmpty() bool {
	return sb.Len() == 0
}

// Clear zeros out the secure data.
// This should be called when the data is no longer needed.
func (sb *SecureBytes) Clear() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	zeroBytes(sb.data)
	sb.data = nil
}

// Clone creates a copy of the secure bytes.
func (sb *SecureBytes) Clone() *SecureBytes {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return NewSecureBytes(sb.data)
}

// zeroBytes overwrites a byte slice with zeros.
// This uses a memory barrier to prevent the compiler from optimizing it away.
func zeroBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	// Zero each byte
	for i := range b {
		b[i] = 0
	}
	// Memory barrier to prevent optimization
	runtime.KeepAlive(b)
}

// SecureCredential wraps a Credential with secure password handling.
// Use this instead of Credential when you need to store passwords securely.
type SecureCredential struct {
	Protocol string
	Host     string
	Path     string
	Username string
	password *SecureBytes
}

// NewSecureCredential creates a new SecureCredential from a Credential.
// NOTE: Due to Go's immutable strings, the original credential's password
// cannot be zeroed in memory. The caller should avoid storing the original
// Credential in long-lived structures.
func NewSecureCredential(cred *Credential) *SecureCredential {
	if cred == nil {
		return nil
	}
	sc := &SecureCredential{
		Protocol: cred.Protocol,
		Host:     cred.Host,
		Path:     cred.Path,
		Username: cred.Username,
		password: NewSecureBytesFromString(cred.Password),
	}
	// We cannot zero the original password string due to Go's immutable strings
	// The best we can do is set it to empty to allow the original to be GC'd
	cred.Password = ""
	return sc
}

// Password returns the password.
// The returned string should not be stored long-term.
func (sc *SecureCredential) Password() string {
	if sc.password == nil {
		return ""
	}
	return sc.password.String()
}

// SetPassword sets the password securely.
func (sc *SecureCredential) SetPassword(password string) {
	if sc.password != nil {
		sc.password.Clear()
	}
	sc.password = NewSecureBytesFromString(password)
}

// Clear zeros out the password.
func (sc *SecureCredential) Clear() {
	if sc.password != nil {
		sc.password.Clear()
		sc.password = nil
	}
}

// ToCredential converts back to a regular Credential.
// WARNING: The returned Credential contains a plain string password.
// Clear it when done.
func (sc *SecureCredential) ToCredential() *Credential {
	return &Credential{
		Protocol: sc.Protocol,
		Host:     sc.Host,
		Path:     sc.Path,
		Username: sc.Username,
		Password: sc.Password(),
	}
}

// ClearCredential clears sensitive fields in a Credential.
// NOTE: Due to Go's immutable strings, the original password data
// cannot be zeroed in memory. This function sets the password to empty
// to allow the original string to be garbage collected.
func ClearCredential(cred *Credential) {
	if cred == nil {
		return
	}
	// We cannot zero the password string memory due to Go's immutability
	// Setting to empty allows the original to be GC'd
	cred.Password = ""
}
