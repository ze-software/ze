// VALIDATES: spec-ospf-ext-9 A-3, AC-2, AC-4 -- the OSPFv3 Grace-LSA (LS Type 0x000B)
// round-trips byte-for-byte through the typed body and RawBytes passthrough, its body is
// exactly the two mandatory TLVs (Grace Period + Restart Reason, no IP-address tlv), and a
// missing mandatory tlv is rejected as malformed.
// PREVENTS: emitting the RFC 3623 type-3 tlv in OSPFv3, mis-padding the Reason tlv, and
// accepting a Grace-LSA missing a mandatory tlv.

package packet

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestGraceLSAv6BodyBuild(t *testing.T) {
	g := GraceLSA{GracePeriod: 120, Reason: 3}
	// Minimal body: Grace Period tlv (8) + Restart Reason tlv (8) = 16 octets.
	if got := g.EncodedLen(); got != 16 {
		t.Fatalf("GraceLSA.EncodedLen = %d, want 16", got)
	}
	buf := make([]byte, g.EncodedLen())
	if n := g.WriteTo(buf, 0); n != 16 {
		t.Fatalf("GraceLSA.WriteTo wrote %d, want 16", n)
	}
	it := newTLVIterator(buf)
	var types16 []uint16
	for it.Next() {
		types16 = append(types16, it.Type())
	}
	if it.Err() != nil {
		t.Fatalf("iterator error: %v", it.Err())
	}
	if len(types16) != 2 || types16[0] != GraceTLVPeriod || types16[1] != GraceTLVReason {
		t.Fatalf("Grace body tlv types = %v, want [1 2] (no type-3 IP-address tlv)", types16)
	}
}

func TestGraceLSAv6RoundTrip(t *testing.T) {
	want := LSA{
		Header: sampleLSAHeader(t, types.LSTypeGrace, "0.0.0.7"), // LS ID = Interface ID
		Grace:  &GraceLSA{GracePeriod: 120, Reason: 2},
	}
	wire := encodeLSA(t, want)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA grace: %v", err)
	}
	if decoded.Header.Type != types.LSTypeGrace {
		t.Fatalf("decoded LS Type = %#x, want %#x", decoded.Header.Type, types.LSTypeGrace)
	}
	if !decoded.VerifyChecksum() {
		t.Fatal("Grace-LSA Fletcher checksum failed to verify")
	}
	body, err := decoded.DecodeGrace()
	if err != nil {
		t.Fatalf("DecodeGrace: %v", err)
	}
	if body.GracePeriod != 120 || body.Reason != 2 {
		t.Fatalf("decoded Grace body = %+v, want {120 2}", body)
	}
	// RawBytes passthrough re-emits the received Grace-LSA byte-for-byte.
	reflood := LSA{Header: decoded.Header, Body: decoded.Body, RawBytes: decoded.RawBytes}
	out := make([]byte, reflood.EncodedLen())
	reflood.WriteTo(out, 0)
	if !bytes.Equal(out, wire) {
		t.Fatal("RawBytes re-emit is not byte-for-byte identical")
	}
}

func TestGraceLSAv6RejectsMissingMandatoryTLV(t *testing.T) {
	// A Grace-LSA carrying only the Grace Period tlv (no Restart Reason) is malformed.
	only := tlv{Type: GraceTLVPeriod, Value: []byte{0, 0, 0, 0x3c}}
	buf := make([]byte, only.EncodedLen())
	only.WriteTo(buf, 0)
	if _, err := decodeGraceLSA(buf); err == nil {
		t.Fatal("decodeGraceLSA accepted a body missing the mandatory Restart Reason tlv")
	}
	// An empty body (no TLVs) is malformed too.
	if _, err := decodeGraceLSA(nil); err == nil {
		t.Fatal("decodeGraceLSA accepted an empty body")
	}
}
