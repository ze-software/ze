// Design: docs/architecture/wire/isis.md -- TLV 10 (Authentication) codec test
package packet

import (
	"bytes"
	"testing"
)

// VALIDATES: AC-10 -- TLV 10 (Authentication) encodes with the auth-type octet
// and an opaque value, and decodes back to the same type + value. The codec is
// structural only; sign/verify is isis-10.
// PREVENTS: corrupting the auth-type octet or the opaque digest on the wire,
// which would make every authenticated peer reject the PDU.
func TestISISTLVAuthCodec(t *testing.T) {
	cases := []struct {
		name     string
		authType uint8
		value    []byte
	}{
		{"hmac-md5", AuthTypeHMACMD5, bytes.Repeat([]byte{0xAB}, 16)}, // 16-octet HMAC-MD5 digest
		{"cleartext", AuthTypeCleartext, []byte("s3cret")},
		{"generic-crypto", AuthTypeGenericCrypto, bytes.Repeat([]byte{0x11}, 20)}, // HMAC-SHA-ish
		{"empty-value", AuthTypeHMACMD5, nil},                                     // type octet only
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := AuthTLV{AuthType: tc.authType, Value: tc.value}
			buf := make([]byte, 64)
			n := writeAuthTLV(buf, 0, in)
			it := NewTLVIterator(buf[:n])
			typ, value, ok := it.Next()
			if !ok || typ != TLVAuthentication {
				t.Fatalf("framing: ok=%v typ=%d", ok, typ)
			}
			out, err := decodeAuthTLV(value)
			if err != nil {
				t.Fatalf("DecodeAuthTLV: %v", err)
			}
			if out.AuthType != tc.authType {
				t.Errorf("AuthType = %d, want %d", out.AuthType, tc.authType)
			}
			// bytes.Equal treats nil and empty as equal, covering the
			// type-octet-only case.
			if !bytes.Equal(out.Value, tc.value) {
				t.Errorf("Value = % x, want % x", out.Value, tc.value)
			}
		})
	}
}

// VALIDATES: the decoder reports TLV 10's index (via AuthTLVIndex) so isis-10
// can enforce the RFC 5304 first-TLV rule. A TLV 10 anywhere in the stream is
// found; this is the codec surfacing the position, not enforcing it.
func TestISISTLVAuthIndexReported(t *testing.T) {
	// Build a TLV region: TLV 10 first, then TLV 1.
	authBuf := make([]byte, 32)
	an := writeAuthTLV(authBuf, 0, AuthTLV{AuthType: AuthTypeHMACMD5, Value: []byte{1, 2, 3}})
	region := make([]byte, 0, an+4)
	region = append(region, authBuf[:an]...)
	region = append(region, 1, 1, 0x49) // TLV 1, len 1
	tlvs, err := DecodeTLVs(region)
	if err != nil {
		t.Fatalf("DecodeTLVs: %v", err)
	}
	if idx := AuthTLVIndex(tlvs); idx != 0 {
		t.Errorf("AuthTLVIndex = %d, want 0 (auth first)", idx)
	}
	if tlvs[0].Type != TLVAuthentication {
		t.Errorf("first TLV type = %d, want %d", tlvs[0].Type, TLVAuthentication)
	}
}

// VALIDATES: a zero-length TLV 10 value (not even the type octet) is rejected
// rather than silently accepted (security review: must not silently accept a
// malformed auth TLV structure).
func TestISISTLVAuthEmpty(t *testing.T) {
	if _, err := decodeAuthTLV(nil); err == nil {
		t.Fatal("expected ErrLength for empty TLV 10 value")
	}
}
