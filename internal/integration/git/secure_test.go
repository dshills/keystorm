package git

import (
	"runtime"
	"testing"
)

func TestSecureBytes_Basic(t *testing.T) {
	data := []byte("secret password")
	sb := NewSecureBytes(data)

	// Verify data is stored correctly
	if string(sb.Bytes()) != "secret password" {
		t.Errorf("expected 'secret password', got %s", string(sb.Bytes()))
	}

	// Verify string conversion
	if sb.String() != "secret password" {
		t.Errorf("expected 'secret password', got %s", sb.String())
	}

	// Verify length
	if sb.Len() != 15 {
		t.Errorf("expected length 15, got %d", sb.Len())
	}

	// Clear and verify
	sb.Clear()
	if !sb.IsEmpty() {
		t.Error("expected empty after clear")
	}
}

func TestSecureBytes_Clear(t *testing.T) {
	data := []byte("sensitive data")
	sb := NewSecureBytes(data)

	// Get a reference to the internal data
	internal := sb.Bytes()

	// Clear the secure bytes
	sb.Clear()

	// The bytes should now be nil
	if sb.Bytes() != nil {
		t.Error("expected nil bytes after clear")
	}

	// Original data should still be intact (we made a copy)
	if string(data) != "sensitive data" {
		t.Error("original data should not be modified")
	}

	// The internal slice we captured should be zeroed
	allZero := true
	for _, b := range internal {
		if b != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("expected internal data to be zeroed")
	}
}

func TestSecureBytes_Clone(t *testing.T) {
	original := NewSecureBytesFromString("original secret")
	clone := original.Clone()

	// Verify clone has the same data
	if clone.String() != "original secret" {
		t.Errorf("expected 'original secret', got %s", clone.String())
	}

	// Clear original and verify clone is unaffected
	original.Clear()
	if clone.String() != "original secret" {
		t.Error("clone should not be affected by clearing original")
	}

	// Clear clone
	clone.Clear()
}

func TestSecureBytes_Nil(t *testing.T) {
	sb := NewSecureBytes(nil)
	if !sb.IsEmpty() {
		t.Error("nil data should create empty SecureBytes")
	}

	// Clear should be safe on nil data
	sb.Clear()
}

func TestSecureBytes_FromString(t *testing.T) {
	sb := NewSecureBytesFromString("test string")
	if sb.String() != "test string" {
		t.Errorf("expected 'test string', got %s", sb.String())
	}
	sb.Clear()
}

func TestSecureCredential_Basic(t *testing.T) {
	cred := &Credential{
		Protocol: "https",
		Host:     "github.com",
		Path:     "owner/repo",
		Username: "testuser",
		Password: "supersecret",
	}

	sc := NewSecureCredential(cred)

	// Verify fields
	if sc.Protocol != "https" {
		t.Errorf("expected 'https', got %s", sc.Protocol)
	}
	if sc.Host != "github.com" {
		t.Errorf("expected 'github.com', got %s", sc.Host)
	}
	if sc.Username != "testuser" {
		t.Errorf("expected 'testuser', got %s", sc.Username)
	}
	if sc.Password() != "supersecret" {
		t.Errorf("expected 'supersecret', got %s", sc.Password())
	}

	// Original credential password should be cleared
	if cred.Password != "" {
		t.Error("original credential password should be cleared")
	}

	// Test Clear
	sc.Clear()
	if sc.Password() != "" {
		t.Error("password should be empty after clear")
	}
}

func TestSecureCredential_SetPassword(t *testing.T) {
	sc := &SecureCredential{
		Protocol: "https",
		Host:     "github.com",
	}

	sc.SetPassword("password1")
	if sc.Password() != "password1" {
		t.Errorf("expected 'password1', got %s", sc.Password())
	}

	// Change password
	sc.SetPassword("password2")
	if sc.Password() != "password2" {
		t.Errorf("expected 'password2', got %s", sc.Password())
	}

	sc.Clear()
}

func TestSecureCredential_ToCredential(t *testing.T) {
	sc := &SecureCredential{
		Protocol: "ssh",
		Host:     "gitlab.com",
		Path:     "user/project",
		Username: "gituser",
	}
	sc.SetPassword("mypassword")

	cred := sc.ToCredential()

	if cred.Protocol != "ssh" {
		t.Errorf("expected 'ssh', got %s", cred.Protocol)
	}
	if cred.Host != "gitlab.com" {
		t.Errorf("expected 'gitlab.com', got %s", cred.Host)
	}
	if cred.Username != "gituser" {
		t.Errorf("expected 'gituser', got %s", cred.Username)
	}
	if cred.Password != "mypassword" {
		t.Errorf("expected 'mypassword', got %s", cred.Password)
	}

	// Clear the credential
	ClearCredential(cred)
	if cred.Password != "" {
		t.Error("credential password should be cleared")
	}

	sc.Clear()
}

func TestSecureCredential_Nil(t *testing.T) {
	sc := NewSecureCredential(nil)
	if sc != nil {
		t.Error("nil credential should return nil SecureCredential")
	}
}

func TestClearCredential(t *testing.T) {
	cred := &Credential{
		Protocol: "https",
		Host:     "github.com",
		Username: "user",
		Password: "secret",
	}

	ClearCredential(cred)

	if cred.Password != "" {
		t.Error("password should be cleared")
	}
	// Other fields should remain
	if cred.Protocol != "https" {
		t.Error("non-sensitive fields should remain")
	}
}

func TestClearCredential_Nil(t *testing.T) {
	// Should not panic
	ClearCredential(nil)
}

func TestSecureBytes_Finalizer(t *testing.T) {
	// Create secure bytes
	dataCopy := make([]byte, 10)
	copy(dataCopy, []byte("testdata00"))

	func() {
		sb := NewSecureBytes(dataCopy)
		_ = sb.String() // Use it
		// sb goes out of scope here
	}()

	// Force GC to run the finalizer
	runtime.GC()
	runtime.GC()

	// The finalizer should have zeroed the internal data
	// We can't easily verify this, but at least ensure no panic
}

func TestZeroBytes(t *testing.T) {
	data := []byte("sensitive")
	zeroBytes(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("byte at index %d was not zeroed: %d", i, b)
		}
	}
}

func TestZeroBytes_Empty(t *testing.T) {
	// Should not panic
	zeroBytes(nil)
	zeroBytes([]byte{})
}
