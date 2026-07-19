package redact

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBcryptHash(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", true},
		{"$2b$12$C6UzMDM.H6dfI/f/IKcEeO.9eSpDzD1z3z1z3z1z3z1z3z1z3z1z3", true},
		{"$2y$08$" + strings.Repeat("a", 53), true},
		{"", false},
		{"plaintext", false},
		{"$2a$10$tooShort", false},
		{"$1$abc$def", false}, // md5-crypt, not bcrypt
		{"$2c$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", false}, // invalid variant
	}
	for _, c := range cases {
		assert.Equal(t, c.want, IsBcryptHash(c.in), "IsBcryptHash(%q)", c.in)
	}
}

// VALIDATES: AC-7 — a bcrypt-shaped token anywhere in a command is redacted.
func TestCommandRedactsBcryptToken(t *testing.T) {
	hash := "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
	in := "config set system authentication user alice password " + hash
	out := Command(in)
	assert.NotContains(t, out, hash, "the bcrypt hash must not survive redaction")
	assert.Contains(t, out, Placeholder)
	// Non-credential tokens are preserved.
	assert.Contains(t, out, "config set system authentication user alice password")
}

// VALIDATES: AC-7 — the value after a password-family key is redacted even when
// it is a plaintext (non-bcrypt-shaped) password.
func TestCommandRedactsPasswordKeyValue(t *testing.T) {
	assert.Equal(t,
		"config set ... password "+Placeholder,
		Command("config set ... password hunter2"))
	assert.Equal(t,
		"config set ... plaintext-password "+Placeholder,
		Command("config set ... plaintext-password s3cr3t"))
	assert.Equal(t,
		"set web-password "+Placeholder,
		Command("set web-password letmein"))
}

// VALIDATES: non-credential commands pass through unchanged (no false positives).
func TestCommandPreservesNonCredential(t *testing.T) {
	cases := []string{
		"show bgp summary",
		"show config system",
		"commit",
		"",
	}
	for _, c := range cases {
		assert.Equal(t, c, Command(c), "non-credential command must be unchanged")
	}
}

// VALIDATES: a trailing password key with no following value does not panic and
// leaks nothing.
func TestCommandTrailingKey(t *testing.T) {
	assert.Equal(t, "config set password", Command("config set password"))
}

// VALIDATES: multiple credential tokens in one command are all redacted.
func TestCommandRedactsMultiple(t *testing.T) {
	hash := "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
	out := Command("user a password p1 user b password " + hash)
	assert.NotContains(t, out, "p1")
	assert.NotContains(t, out, hash)
	assert.Equal(t, 2, strings.Count(out, Placeholder))
}
