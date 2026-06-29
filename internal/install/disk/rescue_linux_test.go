// VALIDATES: AC-7 (sha256 auth), AC-7b (ungated ISO), AC-7c (30s reboot for network)
// PREVENTS: double-hashing ze.shell-auth; wrong fatal branch selection

//go:build linux && ze_installer

package disk

import (
	"crypto/sha256"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

func TestRecoveryAuthCorrectPassword(t *testing.T) {
	password := "admin-password"
	h := sha256.Sum256([]byte(password))
	shellAuth := textbuf.StringHex(h[:])

	if !checkPassword(password, shellAuth) {
		t.Fatal("correct password rejected")
	}
}

func TestRecoveryAuthWrongPassword(t *testing.T) {
	h := sha256.Sum256([]byte("correct"))
	shellAuth := textbuf.StringHex(h[:])

	if checkPassword("wrong", shellAuth) {
		t.Fatal("wrong password accepted")
	}
}

func TestRecoveryAuthEmptyPassword(t *testing.T) {
	h := sha256.Sum256([]byte("admin"))
	shellAuth := textbuf.StringHex(h[:])

	if checkPassword("", shellAuth) {
		t.Fatal("empty password accepted")
	}
}

func TestRecoveryAuthNoDoubleHash(t *testing.T) {
	// ze.shell-auth IS the sha256 hex digest. checkPassword must NOT
	// hash it again before comparing.
	password := "test"
	h := sha256.Sum256([]byte(password))
	shellAuth := textbuf.StringHex(h[:])

	// If checkPassword double-hashed, sha256(sha256("test")) != sha256("test")
	if !checkPassword(password, shellAuth) {
		t.Fatal("checkPassword appears to double-hash ze.shell-auth")
	}
}

func TestFatalPolicyBranch(t *testing.T) {
	tests := []struct {
		name      string
		shellAuth string
		source    string
		want      fatalBranch
	}{
		{"credential present HTTP", "abcd1234", sourceHTTP, branchGated},
		{"credential present ISO", "abcd1234", sourceISO, branchGated},
		{"no cred + ISO", "", sourceISO, branchUngated},
		{"no cred + HTTP", "", sourceHTTP, branchReboot},
	}

	for _, tt := range tests {
		got := selectFatalBranch(tt.shellAuth, tt.source)
		if got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}
