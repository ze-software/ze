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

// VALIDATES: JSON redacts a secret-bearing value at any depth while leaving
// every other value intact.
// PREVENTS: a captured config payload shipping an operator's TCP-MD5 key.
func TestJSONRedactsNestedSecrets(t *testing.T) {
	in := []byte(`{"a":{"md5":"s3cret","ip":"192.0.2.1"},"b":[{"auth-key":"k1"},{"name":"plain"}],"passphrase":"pp"}`)
	out, err := JSON(in)
	assert.NoError(t, err)
	got := string(out)
	for _, secret := range []string{"s3cret", "k1", "pp"} {
		assert.NotContains(t, got, secret, "secret must not survive")
	}
	assert.Contains(t, got, "192.0.2.1", "non-secret values are preserved")
	assert.Contains(t, got, "plain", "non-secret values are preserved")
	assert.Equal(t, 3, strings.Count(got, Placeholder))
	assert.NotContains(t, got, `\u003c`, "the placeholder must not be HTML-escaped")
	assert.Contains(t, got, Placeholder, "the placeholder appears literally")
}

// VALIDATES: a secret spelled as an object or a list is replaced whole, so it
// cannot survive by changing shape.
// PREVENTS: `"md5": {"value": "s3cret"}` walking straight through.
func TestJSONRedactsSecretSubtree(t *testing.T) {
	out, err := JSON([]byte(`{"md5":{"value":"s3cret"},"pre-shared-key":["a","b"]}`))
	assert.NoError(t, err)
	assert.NotContains(t, string(out), "s3cret")
	assert.NotContains(t, string(out), `"a"`)
	assert.Equal(t, 2, strings.Count(string(out), Placeholder))
}

// VALIDATES: a bcrypt-shaped value is redacted wherever it appears, even under a
// key that does not name a secret.
func TestJSONRedactsBcryptAnywhere(t *testing.T) {
	hash := "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
	out, err := JSON([]byte(`{"comment":"` + hash + `"}`))
	assert.NoError(t, err)
	assert.NotContains(t, string(out), hash)
}

// VALIDATES: JSON fails closed -- unparseable input yields the placeholder and
// an error, never the input.
// PREVENTS: a malformed payload smuggling a secret past redaction.
func TestJSONFailsClosed(t *testing.T) {
	out, err := JSON([]byte(`{"md5":"s3cret"`))
	assert.Error(t, err)
	assert.NotContains(t, string(out), "s3cret")
	assert.Equal(t, `"`+Placeholder+`"`, string(out))
}

// VALIDATES: the bare word "key" is NOT treated as a secret name, so a key-chain
// entry id and a YANG list key survive.
// PREVENTS: over-redaction that destroys the readable half of a capture.
func TestIsSecretConfigKeyExcludesBareKey(t *testing.T) {
	assert.False(t, isSecretConfigKey("key"))
	assert.False(t, isSecretConfigKey("key-chain"))
	assert.True(t, isSecretConfigKey("md5"))
	assert.True(t, isSecretConfigKey("auth-key"))
	assert.True(t, isSecretConfigKey("PASSWORD"))
	assert.True(t, isSecretConfigKey("bgp-secret"))
	assert.False(t, isSecretConfigKey("community"))
}

// VALIDATES: multiple credential tokens in one command are all redacted.
func TestCommandRedactsMultiple(t *testing.T) {
	hash := "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
	out := Command("user a password p1 user b password " + hash)
	assert.NotContains(t, out, "p1")
	assert.NotContains(t, out, hash)
	assert.Equal(t, 2, strings.Count(out, Placeholder))
}
