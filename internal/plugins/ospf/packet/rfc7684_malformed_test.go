package packet

import (
	"testing"
)

// extDecodePaths runs every decode path an OSPFv2 router applies to an Extended Prefix
// (opaque type 7) or Extended Link (opaque type 8) body, converting a panic into a returned
// failure so a crash is reported as a test failure rather than aborting the run.
func extDecodePaths(t *testing.T, opaqueType uint8, body []byte) (decodeErr, validateErr error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("decoding a %d-octet opaque type %d body panicked: %v", len(body), opaqueType, r)
		}
	}()
	switch opaqueType {
	case ExtPrefixOpaqueType:
		_, decodeErr = DecodeExtPrefixLSA(body)
	case ExtLinkOpaqueType:
		_, decodeErr = DecodeExtLinkLSA(body)
	}
	validateErr = ValidateExtLSABody(opaqueType, body)
	return decodeErr, validateErr
}

// RFC requirement: RFC7684-5-1 positive -- a well-formed Extended Prefix body (one TLV, one
// sub-TLV) and a well-formed Extended Link body (one TLV, one sub-TLV) are accepted by both
// the decoder and ValidateExtLSABody, so detection never refuses valid input.
// RFC requirement: RFC7684-5-1 negative -- each malformed TLV and sub-TLV permutation (a
// sub-TLV overrunning its TLV, a truncated sub-TLV header, a TLV shorter than its fixed
// fields, a top-level TLV overrunning the body, and a malformed duplicate Extended Link TLV)
// is detected: DecodeExtPrefixLSA or DecodeExtLinkLSA and ValidateExtLSABody each return an
// error, and none panics.
//
// Goal: prove both clauses of RFC 7684 Section 5 for both Extended opaque LSAs: a malformed
// permutation is detected (an error, never a silent accept) and cannot crash the process.
// Method: hand-built bodies, one malformation each, run through every decode path.
func TestRFC7684MalformedPermutationsDetected(t *testing.T) {
	prefixFixed := []byte{ExtRouteTypeIntraArea, 24, 0, 0, 10, 1, 2, 0}
	linkFixed := []byte{1, 0, 0, 0, 2, 2, 2, 2, 10, 0, 0, 1}
	join := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	valid := []struct {
		name       string
		opaqueType uint8
		body       []byte
	}{
		{"prefix with one sub-TLV", ExtPrefixOpaqueType, join([]byte{0, 1, 0, 16}, prefixFixed, []byte{0, 9, 0, 4, 1, 2, 3, 4})},
		{"link with one sub-TLV", ExtLinkOpaqueType, join([]byte{0, 1, 0, 20}, linkFixed, []byte{0, 9, 0, 4, 1, 2, 3, 4})},
	}
	for _, c := range valid {
		decodeErr, validateErr := extDecodePaths(t, c.opaqueType, c.body)
		if decodeErr != nil || validateErr != nil {
			t.Errorf("%s: well-formed body refused: decode %v, validate %v", c.name, decodeErr, validateErr)
		}
	}

	malformed := []struct {
		name       string
		opaqueType uint8
		body       []byte
	}{
		{"prefix sub-TLV overruns its TLV", ExtPrefixOpaqueType, join([]byte{0, 1, 0, 12}, prefixFixed, []byte{0, 9, 0, 8})},
		{"prefix sub-TLV header truncated", ExtPrefixOpaqueType, join([]byte{0, 1, 0, 10}, prefixFixed, []byte{0, 9, 0, 0})},
		{"prefix TLV shorter than its fixed fields", ExtPrefixOpaqueType, join([]byte{0, 1, 0, 4}, prefixFixed[:4])},
		{"prefix TLV overruns the body", ExtPrefixOpaqueType, join([]byte{0, 1, 0, 20}, prefixFixed)},
		{"link sub-TLV overruns its TLV", ExtLinkOpaqueType, join([]byte{0, 1, 0, 16}, linkFixed, []byte{0, 9, 0, 8})},
		{"link sub-TLV header truncated", ExtLinkOpaqueType, join([]byte{0, 1, 0, 14}, linkFixed, []byte{0, 9, 0, 0})},
		{"link TLV shorter than its fixed fields", ExtLinkOpaqueType, join([]byte{0, 1, 0, 8}, linkFixed[:8])},
		{"link TLV overruns the body", ExtLinkOpaqueType, join([]byte{0, 1, 0, 24}, linkFixed)},
		{"malformed duplicate link TLV", ExtLinkOpaqueType, join([]byte{0, 1, 0, 12}, linkFixed, []byte{0, 1, 0, 16}, linkFixed, []byte{0, 9, 0, 8})},
	}
	for _, c := range malformed {
		decodeErr, validateErr := extDecodePaths(t, c.opaqueType, c.body)
		if decodeErr == nil {
			t.Errorf("%s: decoder accepted the malformed body", c.name)
		}
		if validateErr == nil {
			t.Errorf("%s: ValidateExtLSABody accepted the malformed body", c.name)
		}
	}
}
