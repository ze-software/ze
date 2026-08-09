// Design: docs/architecture/isis/isis-10-auth.md -- tests for the auth type/algorithm helpers.
//
// These pin the pure per-algorithm helpers in auth_types.go: the on-wire
// digest length (RFC 5304 / RFC 5310), the Key-ID width per family, and the
// hash-constructor availability. They complement the sign/verify round-trip
// coverage in auth_sign_test.go / auth_verify_test.go.

package packet

import (
	"crypto/md5" //nolint:gosec // test asserts the RFC 5304 HMAC-MD5 digest length
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"testing"
)

// VALIDATES: digestLen returns the RFC-mandated authentication-value length for
// every supported HMAC algorithm (RFC 5304 sec 2 for MD5; RFC 5310 sec 3.1 for
// the SHA family), and 0 for non-HMAC algorithms.
// PREVENTS: a wrong digest length that would mis-frame TLV 10 and break interop.
func TestISISAuthDigestLen(t *testing.T) {
	cases := []struct {
		algo AuthAlgorithm
		want int
	}{
		{AuthAlgoNone, 0},
		{AuthAlgoCleartext, 0}, // password length is variable, not a fixed digest
		{AuthAlgoHMACMD5, md5.Size},
		{AuthAlgoHMACSHA1, sha1.Size},
		{AuthAlgoHMACSHA224, sha256.Size224},
		{AuthAlgoHMACSHA256, sha256.Size},
		{AuthAlgoHMACSHA384, sha512.Size384},
		{AuthAlgoHMACSHA512, sha512.Size},
	}
	for _, tc := range cases {
		if got := digestLen(tc.algo); got != tc.want {
			t.Errorf("digestLen(%d) = %d, want %d", tc.algo, got, tc.want)
		}
	}
}

// VALIDATES: keyIDOctets is 2 only for the generic-crypto family (type 3,
// RFC 5310 sec 3.1, where TLV 10 carries a 2-octet Key ID) and 0 for cleartext
// and HMAC-MD5 (which carry no Key ID).
// PREVENTS: a Key-ID width mismatch that would shift the digest offset.
func TestISISAuthKeyIDOctets(t *testing.T) {
	two := []AuthAlgorithm{
		AuthAlgoHMACSHA1, AuthAlgoHMACSHA224, AuthAlgoHMACSHA256,
		AuthAlgoHMACSHA384, AuthAlgoHMACSHA512,
	}
	for _, a := range two {
		if got := keyIDOctets(a); got != 2 {
			t.Errorf("keyIDOctets(%d) = %d, want 2 (generic crypto)", a, got)
		}
	}
	zero := []AuthAlgorithm{AuthAlgoNone, AuthAlgoCleartext, AuthAlgoHMACMD5}
	for _, a := range zero {
		if got := keyIDOctets(a); got != 0 {
			t.Errorf("keyIDOctets(%d) = %d, want 0", a, got)
		}
	}
}

// VALIDATES: newHash returns a constructor for every HMAC algorithm whose digest
// length equals digestLen, and nil for non-HMAC algorithms (None/Cleartext).
// PREVENTS: an algorithm wired into the enum but missing a hash constructor,
// which would silently fall through to ErrAuthUnsupported at sign time.
func TestISISAuthNewHash(t *testing.T) {
	hmacAlgos := []AuthAlgorithm{
		AuthAlgoHMACMD5, AuthAlgoHMACSHA1, AuthAlgoHMACSHA224,
		AuthAlgoHMACSHA256, AuthAlgoHMACSHA384, AuthAlgoHMACSHA512,
	}
	for _, a := range hmacAlgos {
		ctor := newHash(a)
		if ctor == nil {
			t.Errorf("newHash(%d) = nil, want a constructor", a)
			continue
		}
		if got := ctor().Size(); got != digestLen(a) {
			t.Errorf("newHash(%d) size = %d, want %d", a, got, digestLen(a))
		}
	}
	for _, a := range []AuthAlgorithm{AuthAlgoNone, AuthAlgoCleartext} {
		if newHash(a) != nil {
			t.Errorf("newHash(%d) != nil, want nil (no HMAC)", a)
		}
	}
}

// VALIDATES: the auth type codes are pinned to the RFC values (cleartext 1,
// HMAC-MD5 54, generic crypto 3). The research guide has 54/3 swapped; this test
// is the regression guard (spec Design Insight, A-1).
func TestISISAuthTypeCodes(t *testing.T) {
	cases := []struct {
		algo AuthAlgorithm
		want uint8
	}{
		{AuthAlgoCleartext, 1},
		{AuthAlgoHMACMD5, 54},
		{AuthAlgoHMACSHA1, 3},
		{AuthAlgoHMACSHA256, 3},
		{AuthAlgoHMACSHA512, 3},
	}
	for _, tc := range cases {
		got, ok := authTypeFor(tc.algo)
		if !ok || got != tc.want {
			t.Errorf("authTypeFor(%d) = %d (ok=%v), want %d", tc.algo, got, ok, tc.want)
		}
	}
}
