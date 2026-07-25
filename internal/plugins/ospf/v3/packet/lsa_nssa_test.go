// VALIDATES: spec-ospfv3-2-wire AC-11 -- the NSSA-LSA body is byte-identical to
// the AS-External body, decodes through the same path, has area scope, and
// carries the P-bit in PrefixOptions (not a header Options byte).
// PREVENTS: NSSA body drift from AS-External, or looking for the P-bit in the
// wrong field.

package packet

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3NSSARoundTrip(t *testing.T) {
	const uBitMask = 0x8000
	// LSTypeRouter (0x2001) is the canonical area-scoped type (RFC 5340 S2/S1=01).
	areaScope := types.LSTypeRouter.Scope()
	body := ExternalLSA{
		ExternalType2:    true,
		Metric:           0x00abcd,
		Prefix:           makePrefix(t, 64, types.OptPrefixP, 0), // P-bit propagate
		HasRouteTag:      true,
		ExternalRouteTag: 0x99887766,
	}
	lsa := LSA{Header: sampleLSAHeader(t, types.LSTypeNSSA, "0.0.0.1"), External: &body}
	wire := encodeLSA(t, lsa)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA nssa: %v", err)
	}
	// Area scope, U-bit clear (the NSSA type 0x2007).
	if decoded.Header.Type.Scope() != areaScope {
		t.Fatalf("NSSA scope = %v, want area", decoded.Header.Type.Scope())
	}
	if uint16(decoded.Header.Type)&uBitMask != 0 {
		t.Fatalf("NSSA U-bit set, want clear")
	}

	// Decoding through the LSA method and the shared external body path must agree.
	viaExternal, err := decoded.DecodeExternal()
	if err != nil {
		t.Fatalf("DecodeExternal: %v", err)
	}
	viaNSSA, err := DecodeExternalLSA(decoded.Body)
	if err != nil {
		t.Fatalf("DecodeExternalLSA: %v", err)
	}
	if viaExternal.Metric != viaNSSA.Metric || viaExternal.ExternalRouteTag != viaNSSA.ExternalRouteTag {
		t.Fatalf("NSSA external decode diverges")
	}

	// The P-bit lives in PrefixOptions, not the header.
	if !NSSAPropagate(viaNSSA) {
		t.Fatalf("NSSA P-bit not preserved in PrefixOptions")
	}
	if viaNSSA.Prefix.Options&types.OptPrefixP == 0 {
		t.Fatalf("PrefixOptions P-bit cleared: %#x", viaNSSA.Prefix.Options)
	}

	encodedNSSA := make([]byte, body.EncodedLen())
	body.WriteTo(encodedNSSA, 0)
	if !bytes.Equal(decoded.Body, encodedNSSA) {
		t.Fatalf("ExternalLSA.WriteTo body differs from decoded NSSA body:\n ext  % x\n wire % x", encodedNSSA, decoded.Body)
	}

	// Build an AS-External with the same body bytes and confirm the bodies match.
	extLSA := LSA{Header: sampleLSAHeader(t, types.LSTypeASExternal, "0.0.0.1"), External: &body}
	extWire := encodeLSA(t, extLSA)
	if !bytes.Equal(decoded.Body, extWire[LSAHeaderLen:]) {
		t.Fatalf("NSSA body differs from AS-External body:\n nssa % x\n ext  % x", decoded.Body, extWire[LSAHeaderLen:])
	}
}
