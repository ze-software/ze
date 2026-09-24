// Design: docs/architecture/aaa-tacacs.md
// Related: authen.go, author.go, acct.go -- body codecs using these rules.
// RFC 8907 Section 3.7 -- see rfc/short/rfc8907.md.
// PRECIS: golang.org/x/text v0.41.0, https://go.googlesource.com/text/+/refs/tags/v0.41.0/secure/precis/.
package tacacs

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"

	"golang.org/x/text/secure/precis"
)

// prepareUsername preserves absent optional fields; callers requiring a user
// MUST reject the empty value before emitting their request.
func prepareUsername(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	// RFC 8907 Section 3.7: "Usernames MUST be encoded and handled using
	// the UsernameCasePreserved Profile specified in [RFC8265]."
	username, err := precis.UsernameCasePreserved.String(value)
	if err != nil {
		return "", fmt.Errorf("invalid TACACS+ username: %w", err)
	}
	if len(username) == 0 {
		return "", fmt.Errorf("empty TACACS+ username after preparation")
	}
	if len(username) > 255 {
		return "", fmt.Errorf("TACACS+ username exceeds 255 bytes")
	}
	return username, nil
}

const wireIdentityPrefix = "~ze~"

// prepareWireUsername encodes trusted synthetic identities only at the TACACS+
// boundary. The local NUL namespace remains unchanged. Its raw bytes use an
// injective base64url representation, never normalization, hashing or truncation.
// Human usernames are prepared first; names entering this wire namespace use
// the disjoint "u:" branch, including during PAP, so they cannot impersonate a
// synthetic "r:" identity through Unicode width mapping or a printable lookalike.
func prepareWireUsername(value string, allowSynthetic bool) (string, error) {
	if allowSynthetic && (strings.HasPrefix(value, aaa.ReservedInternalPrefix) || value == aaa.ReservedSharedAPIUsername) {
		return encodeWireUsername("r:", value)
	}
	username, err := prepareUsername(value)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(username, wireIdentityPrefix) {
		return encodeWireUsername("u:", username)
	}
	return username, nil
}

func encodeWireUsername(kind, value string) (string, error) {
	if len(wireIdentityPrefix)+len(kind)+base64.RawURLEncoding.EncodedLen(len(value)) > 255 {
		return "", fmt.Errorf("encoded TACACS+ username exceeds 255 bytes")
	}
	// This alphabet is printable ASCII and already satisfies the unchanged
	// UsernameCasePreserved profile. Decode the payload to recover the exact
	// local identity; the r:/u: discriminator is part of the wire identity.
	return wireIdentityPrefix + kind + base64.RawURLEncoding.EncodeToString([]byte(value)), nil
}

func validateText(value string) error {
	// RFC 8907 Section 3.7: "All other text fields in TACACS+ MUST be
	// treated as printable byte arrays of US-ASCII as defined by [RFC0020]."
	// The field length bounds the scan, and errors never include credentials.
	for i := range len(value) {
		if value[i] < 0x20 {
			return fmt.Errorf("TACACS+ text contains a control character at byte %d", i)
		}
		if value[i] > 0x7e {
			return fmt.Errorf("TACACS+ text contains a non-printable ASCII byte at byte %d", i)
		}
	}
	return nil
}
