// VALIDATES: RFC 5880 Section 6.7.3 / 6.7.4 key management -- the operator
// interface that configures an MD5 or SHA1 authentication key accepts an
// ASCII string and carries it through to the session's auth settings.
// PREVENTS: a key-management surface that silently drops, truncates, or
// re-encodes the operator's key, and one that accepts a session with no key
// at all.
package bfd

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880AuthFields builds the `auth { ... }` field map the config parser
// consumes for a profile.
func rfc5880AuthFields(typeStr, keyID, secret string) map[string]any {
	return map[string]any{
		"type":   typeStr,
		"key-id": keyID,
		"secret": secret,
	}
}

// RFC requirement: RFC5880-6.7.3-7 positive -- the MD5 key management
// interface accepts ASCII strings. parseAuthConfig
// (internal/component/bfd/config.go:297-306) reads the `secret` leaf as a
// string and stores []byte(secret) verbatim, so the operator's ASCII key
// reaches the signer unchanged.
// RFC requirement: RFC5880-6.7.4-5 positive -- the same leaf and the same
// parser serve the SHA1 variants, so a SHA1 key is likewise configured as an
// ASCII string.
func TestRFC5880KeyManagementAcceptsASCIIStrings(t *testing.T) {
	cases := []struct {
		enum string
		want uint8
	}{
		{"keyed-md5", packet.AuthTypeKeyedMD5},
		{"meticulous-keyed-md5", packet.AuthTypeMeticulousKeyedMD5},
		{"keyed-sha1", packet.AuthTypeKeyedSHA1},
		{"meticulous-keyed-sha1", packet.AuthTypeMeticulousKeyedSHA1},
	}
	const secret = "Correct Horse Battery Staple 42!"
	for _, tt := range cases {
		t.Run(tt.enum, func(t *testing.T) {
			ac, err := parseAuthConfig("p", rfc5880AuthFields(tt.enum, "7", secret))
			if err != nil {
				t.Fatalf("parseAuthConfig: %v", err)
			}
			if ac.authType != tt.want {
				t.Fatalf("auth type = %d, want %d", ac.authType, tt.want)
			}
			if ac.keyID != 7 {
				t.Fatalf("key id = %d, want 7", ac.keyID)
			}
			if string(ac.secret) != secret {
				t.Fatalf("secret = %q, want the configured ASCII string %q", string(ac.secret), secret)
			}
		})
	}
}

// RFC requirement: RFC5880-6.7.3-7 negative -- the key management interface is
// not a blanket accept: parseAuthConfig rejects an auth block with no type
// (internal/component/bfd/config.go:279-281), an unknown type
// (config.go:285-288), a missing key id (config.go:289-292), a non-numeric key
// id (config.go:293-296), and a missing secret (config.go:297-300), so a
// session can never come up with an unusable key.
// RFC requirement: RFC5880-6.7.4-5 negative -- the same validation guards the
// SHA1 variants, which is what makes the accepted ASCII string above a real
// key rather than a default.
func TestRFC5880KeyManagementRejectsIncompleteConfig(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]any
	}{
		{"missing type", map[string]any{"key-id": "1", "secret": "k"}},
		{"unknown type", rfc5880AuthFields("keyed-sha512", "1", "k")},
		{"simple password", rfc5880AuthFields("simple-password", "1", "k")},
		{"missing key id", map[string]any{"type": "keyed-sha1", "secret": "k"}},
		{"non numeric key id", rfc5880AuthFields("keyed-sha1", "abc", "k")},
		{"key id out of range", rfc5880AuthFields("keyed-sha1", "256", "k")},
		{"missing secret", map[string]any{"type": "keyed-sha1", "key-id": "1"}},
		{"empty secret", rfc5880AuthFields("keyed-md5", "1", "")},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseAuthConfig("p", tt.fields); err == nil {
				t.Fatal("parseAuthConfig accepted an incomplete auth block")
			}
		})
	}
}
