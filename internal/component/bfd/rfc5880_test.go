// VALIDATES: RFC 5880 Section 6.7.2 / 6.7.3 / 6.7.4 key management -- the
// operator interface that configures a Simple Password, MD5, or SHA1
// authentication key accepts an ASCII string and carries it through to the
// session's auth settings.
// PREVENTS: a key-management surface that silently drops, truncates, or
// re-encodes the operator's key, one that accepts a session with no key at
// all, and one that accepts a password no Auth Len field can describe.
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
		{"simple password over 16 bytes", rfc5880AuthFields("simple-password", "1", "seventeen bytes!!")},
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

// RFC requirement: RFC5880-6.7.2-1 positive -- "For interoperability, the
// management interface by which the password is configured MUST accept ASCII
// strings." The `secret` leaf is a YANG string and parseAuthConfig stores
// []byte(secret) verbatim, so the operator's ASCII password reaches the signer
// unchanged at both ends of the permitted length range.
// RFC requirement: RFC5880-4.2-1 positive -- a one-byte and a sixteen-byte
// password are both accepted, which is the whole range RFC 5880 Section 6.7.2
// allows.
func TestRFC5880SimplePasswordManagementAcceptsASCIIStrings(t *testing.T) {
	cases := []string{"x", "correcthorseba", "sixteen bytes!!!"}
	for _, secret := range cases {
		t.Run(secret, func(t *testing.T) {
			ac, err := parseAuthConfig("p", rfc5880AuthFields("simple-password", "3", secret))
			if err != nil {
				t.Fatalf("parseAuthConfig: %v", err)
			}
			if ac.authType != packet.AuthTypeSimplePassword {
				t.Fatalf("auth type = %d, want %d", ac.authType, packet.AuthTypeSimplePassword)
			}
			if ac.meticulous {
				t.Fatal("simple-password resolved to a meticulous profile")
			}
			if ac.keyID != 3 {
				t.Fatalf("key id = %d, want 3", ac.keyID)
			}
			if string(ac.secret) != secret {
				t.Fatalf("secret = %q, want the configured ASCII string %q", string(ac.secret), secret)
			}
		})
	}
}

// RFC requirement: RFC5880-4.2-1 negative -- "The password is a binary string,
// and MUST be from 1 to 16 bytes in length." parseAuthConfig refuses an empty
// password and one of seventeen bytes or more, so no profile can produce an
// Auth Len outside the 4 to 19 the wire format allows.
// RFC requirement: RFC5880-6.7.2-1 negative -- the management interface is not
// a blanket accept: it takes the ASCII string only when the string is one the
// Authentication Section can carry.
func TestRFC5880SimplePasswordLengthOutOfRangeRefused(t *testing.T) {
	cases := []struct {
		name   string
		secret string
	}{
		{"empty", ""},
		{"seventeen bytes", "seventeen bytes!!"},
		{"thirty two bytes", "0123456789abcdef0123456789abcdef"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseAuthConfig("p", rfc5880AuthFields("simple-password", "1", tt.secret)); err == nil {
				t.Fatalf("parseAuthConfig accepted a %d-byte simple password", len(tt.secret))
			}
		})
	}
}
