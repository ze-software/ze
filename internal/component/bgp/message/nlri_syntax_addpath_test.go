// RFC: rfc/short/rfc7606.md — revised UPDATE error handling
// RFC: rfc/short/rfc7911.md — ADD-PATH (4-octet Path Identifier per NLRI)
// Overview: rfc7606.go — ValidateNLRISyntaxAddPath (RFC 7911 path-id skip)
//
// ADD-PATH awareness of the §5.3 NLRI syntax walker. With ADD-PATH negotiated (RFC 7911
// Section 3) every NLRI on the wire is prefixed with a 4-octet Path Identifier that precedes
// the prefix-length octet. A walker that does not skip it misreads a path-id byte as a prefix
// length and spuriously session-resets a conforming UPDATE — the ADD-PATH-blind bug these
// tests pin the fix for.

package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateNLRISyntaxAddPathSkipsPathID proves the fix: an ADD-PATH NLRI whose path-id
// carries a byte that would be an out-of-range prefix length is accepted when addPath=true,
// and would be rejected by the ADD-PATH-blind walk that caused the bug.
//
// VALIDATES: with addPath=true the 4-octet path-id is skipped before the prefix length is
// read, so a valid prefix behind a large path-id is accepted.
// PREVENTS: the ADD-PATH-blind regression that read a path-id byte as a prefix length and
// session-reset a conforming RFC 7911 UPDATE.
func TestValidateNLRISyntaxAddPathSkipsPathID(t *testing.T) {
	// path-id = 0x00000021 (33). Its last octet, if misread as a prefix length, is 33 > 32 for
	// IPv4 — the exact §5.3 "greater than 32" trip. Then a valid 10.0.0.0/24 (3 octets).
	nlri := []byte{0x00, 0x00, 0x00, 0x21, 24, 10, 0, 0}

	require.Nil(t, ValidateNLRISyntaxAddPath(nlri, false, true),
		"with ADD-PATH the path-id must be skipped and the /24 behind it accepted")

	// The ADD-PATH-blind walk (addPath=false) is exactly the bug: it reads the 0x21 path-id
	// byte as a prefix length 33 > 32 and session-resets a conforming UPDATE.
	blind := ValidateNLRISyntaxAddPath(nlri, false, false)
	require.NotNil(t, blind,
		"the ADD-PATH-blind walk misreads the path-id — this is the regression the fix removes")
	require.Equal(t, RFC7606ActionSessionReset, blind.Action)
}

// TestValidateNLRISyntaxAddPathIPv6SkipsPathID is the IPv6 analog: a path-id byte that would
// exceed the 128 maximum is skipped when addPath=true.
//
// VALIDATES: path-id skipping works for IPv6 (max prefix length 128) too.
func TestValidateNLRISyntaxAddPathIPv6SkipsPathID(t *testing.T) {
	// path-id = 0x00000081 (129). Its last octet is 129 > 128 for IPv6 if misread. Then a
	// valid 2001:db8::/32 (4 prefix octets).
	nlri := []byte{0x00, 0x00, 0x00, 0x81, 32, 0x20, 0x01, 0x0d, 0xb8}

	require.Nil(t, ValidateNLRISyntaxAddPath(nlri, true, true),
		"with ADD-PATH the IPv6 path-id must be skipped and the /32 behind it accepted")
	require.NotNil(t, ValidateNLRISyntaxAddPath(nlri, true, false),
		"the ADD-PATH-blind walk misreads the 0x81 path-id byte as prefix length 129 > 128")
}

// TestValidateNLRISyntaxAddPathStillDetectsGenuineOverrun proves the skip does not blind the
// walker to real errors: a genuine overrun or over-long prefix AFTER the path-id is still
// session-reset.
//
// VALIDATES: with addPath=true, a real §5.3 violation behind the path-id is still caught.
// PREVENTS: an over-permissive fix that accepts any ADD-PATH NLRI.
func TestValidateNLRISyntaxAddPathStillDetectsGenuineOverrun(t *testing.T) {
	t.Run("prefix overruns after path-id", func(t *testing.T) {
		// path-id=1, then /24 claims 3 prefix octets but only 2 are present.
		nlri := []byte{0x00, 0x00, 0x00, 0x01, 24, 10, 0}
		result := ValidateNLRISyntaxAddPath(nlri, false, true)
		require.NotNil(t, result)
		require.Equal(t, RFC7606ActionSessionReset, result.Action)
	})

	t.Run("prefix length too long after path-id", func(t *testing.T) {
		// path-id=1, then a real prefix-length octet of 33 (> 32 for IPv4).
		nlri := []byte{0x00, 0x00, 0x00, 0x01, 33, 10, 0, 0, 0, 0}
		result := ValidateNLRISyntaxAddPath(nlri, false, true)
		require.NotNil(t, result)
		require.Equal(t, RFC7606ActionSessionReset, result.Action)
	})

	t.Run("path-id truncated", func(t *testing.T) {
		// Fewer than 4 bytes for the path-id itself: cannot be parsed.
		for _, nlri := range [][]byte{{0x00, 0x00, 0x00}, {0x00, 0x00, 0x00, 0x00}} {
			result := ValidateNLRISyntaxAddPath(nlri, false, true)
			require.NotNil(t, result, "a truncated ADD-PATH NLRI must not be parseable: %v", nlri)
			require.Equal(t, RFC7606ActionSessionReset, result.Action)
		}
	})
}
