// Design: docs/architecture/isis/isis-10-auth.md -- tests for the sign-side helpers.
//
// These pin the lower-level sign machinery in auth_sign.go (prependTLV ordering
// and tlvRegionStart per PDU type) that the round-trip tests in
// auth_verify_test.go exercise only indirectly. The end-to-end SignPDU/VerifyPDU
// coverage lives in auth_verify_test.go.

package packet

import "testing"

// VALIDATES: prependTLV places its argument as the FIRST element and preserves
// the order of the remaining TLVs (RFC 5304 sec 1: the auth TLV is the first
// TLV). It must not mutate the caller's slice.
// PREVENTS: a regression that appends instead of prepends, which would put TLV 10
// last and make every verifier reject the PDU as ErrAuthNotFirst.
func TestISISAuthPrependTLV(t *testing.T) {
	orig := []TLV{
		{Type: TLVAreaAddresses, Value: []byte{0x01, 0x49}},
		{Type: TLVProtocolsSupported, Value: []byte{NLPIDIPv4}},
	}
	auth := TLV{Type: TLVAuthentication, Value: []byte{AuthTypeHMACMD5}}
	out := prependTLV(orig, auth)
	if len(out) != len(orig)+1 {
		t.Fatalf("len = %d, want %d", len(out), len(orig)+1)
	}
	if out[0].Type != TLVAuthentication {
		t.Errorf("out[0].Type = %d, want %d (auth first)", out[0].Type, TLVAuthentication)
	}
	if out[1].Type != TLVAreaAddresses || out[2].Type != TLVProtocolsSupported {
		t.Errorf("tail order changed: %d, %d", out[1].Type, out[2].Type)
	}
	// The original slice header is untouched.
	if len(orig) != 2 || orig[0].Type != TLVAreaAddresses {
		t.Error("prependTLV mutated the caller's slice")
	}
}

// VALIDATES: tlvRegionStart reports the byte offset where the TLV region begins
// for each PDU type, and rejects an unknown type / a too-short buffer. It is the
// locator the digest layout relies on; an off-by-one here would mis-place the
// auth value.
func TestISISAuthTLVRegionStart(t *testing.T) {
	builders := pduBuilders()
	for name, build := range builders {
		t.Run(name, func(t *testing.T) {
			pdu := build(t)
			off, ok := tlvRegionStart(pdu)
			if !ok {
				t.Fatalf("tlvRegionStart(%s) = !ok", name)
			}
			if off < CommonHeaderLen || off > len(pdu) {
				t.Errorf("tlvRegionStart(%s) = %d, out of range [%d,%d]", name, off, CommonHeaderLen, len(pdu))
			}
		})
	}
	// Too short: a buffer shorter than the common header is rejected.
	if _, ok := tlvRegionStart([]byte{0x83}); ok {
		t.Error("tlvRegionStart on a too-short buffer = ok, want !ok")
	}
}
