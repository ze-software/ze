// Design: plan/learned/956-ospf-2-wire.md -- LSA header and body tests

package packet

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: spec-ospf-ext-7 A-1 -- the OSPFv2 Router-LSA round-trips the V-bit and a
// Type-4 virtual link record (Link ID = neighbor Router ID, Link Data = local address,
// Metric = transit cost) byte-for-byte (RFC 2328 App A.4.2).
func TestRouterLSAVirtualLinkRoundTrip(t *testing.T) {
	want := RouterLSA{
		Flags: RouterFlagV | RouterFlagB,
		Links: []RouterLink{
			{LinkID: mustLSID(t, "9.9.9.9"), LinkData: [4]byte{172, 16, 0, 1}, Type: RouterLinkTypeVirtual, Metric: mustMetric(t, 15)},
		},
	}
	buf := make([]byte, want.EncodedLen())
	want.WriteTo(buf, 0)
	got, err := DecodeRouterLSA(buf)
	if err != nil {
		t.Fatalf("DecodeRouterLSA: %v", err)
	}
	if got.Flags&RouterFlagV == 0 {
		t.Fatalf("V-bit lost: flags = %#x", got.Flags)
	}
	if len(got.Links) != 1 || got.Links[0] != want.Links[0] {
		t.Fatalf("virtual link record = %+v, want %+v", got.Links, want.Links)
	}
}

// VALIDATES: AC-5 - LSA header fields and LSDB key round-trip.
// PREVENTS: sequence/checksum/age becoming part of LSA identity.
func TestOSPFLSAHeaderRoundTrip(t *testing.T) {
	h := sampleLSAHeader(t, types.LSTypeRouter, "10.0.0.1")
	h.Checksum = 0x1234
	h.Length = types.LSAHeaderLen
	buf := make([]byte, types.LSAHeaderLen)
	writeLSAHeader(h, buf, 0)
	got, err := DecodeLSAHeader(buf)
	if err != nil {
		t.Fatalf("DecodeLSAHeader: %v", err)
	}
	if got != h || got.Key() != h.Key() {
		t.Fatalf("DecodeLSAHeader = %+v key %+v, want %+v key %+v", got, got.Key(), h, h.Key())
	}
}

// VALIDATES: AC-7 - Router-LSA flags and 12-byte link records round-trip.
// PREVENTS: SPF graph edges being decoded with wrong type, ID, data, or metric.
func TestOSPFRouterLSARoundTrip(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))
	lsa, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA router: %v", err)
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Flags != RouterFlagB|RouterFlagE || len(body.Links) != 4 {
		t.Fatalf("router body wrong: %+v", body)
	}
	if body.Links[1].Metric != mustMetric(t, 65535) || body.Links[3].Type != RouterLinkTypeVirtual {
		t.Fatalf("router links wrong: %+v", body.Links)
	}
}

// VALIDATES: AC-8 - Network-LSA mask and attached-router list round-trip; LS ID preserved verbatim.
// PREVENTS: treating the Type 2 LS ID as a network prefix instead of the DR interface address.
func TestOSPFNetworkLSARoundTrip(t *testing.T) {
	want := sampleNetworkLSA(t)
	wire := encodeLSA(t, want)
	lsa, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA network: %v", err)
	}
	body, err := lsa.DecodeNetwork()
	if err != nil {
		t.Fatalf("DecodeNetwork: %v", err)
	}
	if lsa.Header.LinkStateID != want.Header.LinkStateID || body.NetworkMask != [4]byte{255, 255, 255, 0} || len(body.AttachedRouters) != 2 {
		t.Fatalf("network LSA wrong: header=%+v body=%+v", lsa.Header, body)
	}
}

// VALIDATES: AC-9 - Summary-LSA type 3 and 4 use a 24-bit metric.
// PREVENTS: truncating summary metrics to the Router-LSA 16-bit width.
func TestOSPFSummaryLSARoundTrip(t *testing.T) {
	for _, typ := range []types.LSType{types.LSTypeSummaryNetwork, types.LSTypeSummaryASBR} {
		wire := encodeLSA(t, sampleSummaryLSA(t, typ))
		lsa, err := DecodeLSA(wire)
		if err != nil {
			t.Fatalf("DecodeLSA summary %v: %v", typ, err)
		}
		body, err := lsa.DecodeSummary()
		if err != nil {
			t.Fatalf("DecodeSummary %v: %v", typ, err)
		}
		if body.Metric != SummaryMetricMax || body.NetworkMask != [4]byte{255, 255, 255, 0} {
			t.Fatalf("summary body wrong: %+v", body)
		}
	}
}

// VALIDATES: AC-10, AC-11 - External and NSSA bodies share layout, preserving E bit and P/N option.
// PREVENTS: Type 7 NSSA metric/body drift from Type 5 AS-external encoding.
func TestOSPFExternalLSARoundTrip(t *testing.T) {
	for _, typ := range []types.LSType{types.LSTypeASExternal, types.LSTypeNSSA} {
		wire := encodeLSA(t, sampleExternalLSA(t, typ))
		lsa, err := DecodeLSA(wire)
		if err != nil {
			t.Fatalf("DecodeLSA external %v: %v", typ, err)
		}
		body, err := lsa.DecodeExternal()
		if err != nil {
			t.Fatalf("DecodeExternal %v: %v", typ, err)
		}
		if !body.ExternalType2 || body.Metric != ExternalMetricMax || body.ExternalRouteTag != 0xfeedcafe {
			t.Fatalf("external body wrong: %+v", body)
		}
		if typ == types.LSTypeNSSA && !lsa.Header.Options.Has(types.OptionNP) {
			t.Fatalf("NSSA P/N option not preserved: %+v", lsa.Header.Options)
		}
	}
}

// VALIDATES: AC-16 - opaque LSA types are retained as raw spans and re-encoded byte-identical.
// PREVENTS: re-flooding unknown opaque LSAs with corrupted bodies.
func TestOSPFUnknownLSAPassthrough(t *testing.T) {
	h := sampleLSAHeader(t, types.LSTypeOpaqueArea, "1.2.3.4")
	lsa := LSA{Header: h, Opaque: &OpaqueLSA{Type: h.Type, Data: []byte{0xde, 0xad, 0xbe, 0xef}}}
	wire := encodeLSA(t, lsa)
	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA opaque: %v", err)
	}
	if decoded.Opaque != nil {
		t.Fatalf("decoded opaque LSA should keep RawBytes authoritative for passthrough")
	}
	out := make([]byte, decoded.EncodedLen())
	decoded.WriteTo(out, 0)
	if !bytes.Equal(out, wire) {
		t.Fatalf("opaque passthrough changed bytes:\n got % x\nwant % x", out, wire)
	}
}

// VALIDATES: spec-ospf-af-unify -- RefreshLSAInPlace re-stamps an already-encoded LSA's LS Age
// and LS Sequence Number in place and returns a checksum the decoder then accepts. The other
// header/body bytes are untouched, and a buffer shorter than a 20-byte LSA header returns
// (0, false) instead of panicking.
// PREVENTS: a MaxAge self-flush re-stamp that corrupts the LSA or leaves a stale checksum.
func TestRefreshLSAInPlace(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))

	newAge := types.LSAge(types.MaxAge)
	newSeq := types.InitialSequenceNumber.Next()
	cksum, ok := RefreshLSAInPlace(wire, newAge, newSeq)
	if !ok {
		t.Fatalf("RefreshLSAInPlace returned ok=false on a full LSA")
	}

	lsa, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA after refresh: %v", err)
	}
	if lsa.Header.Age != newAge {
		t.Errorf("re-stamped Age = %d, want %d", uint16(lsa.Header.Age), uint16(newAge))
	}
	if lsa.Header.Sequence != newSeq {
		t.Errorf("re-stamped Sequence = %#x, want %#x", uint32(lsa.Header.Sequence), uint32(newSeq))
	}
	if lsa.Header.Checksum != cksum {
		t.Errorf("decoded checksum %#x != returned checksum %#x", lsa.Header.Checksum, cksum)
	}
	if !lsa.VerifyChecksum() {
		t.Errorf("re-stamped LSA fails checksum verification")
	}
	// The Router-LSA body must still decode with the original links intact.
	body, err := lsa.DecodeRouter()
	if err != nil || len(body.Links) != 4 {
		t.Errorf("re-stamp corrupted body: err=%v links=%d, want 4 links", err, len(body.Links))
	}

	// A buffer shorter than a 20-byte LSA header is rejected, not a panic.
	if c, ok := RefreshLSAInPlace(make([]byte, types.LSAHeaderLen-1), newAge, newSeq); ok || c != 0 {
		t.Errorf("RefreshLSAInPlace(short) = (%#x, %v), want (0, false)", c, ok)
	}
}

// VALIDATES: AC-17 - LSA iterator terminates on bad Length without panic.
// PREVENTS: malformed LS Update input reading past the packet body.
func TestOSPFLSAIteratorTruncated(t *testing.T) {
	good := encodeLSA(t, sampleRouterLSA(t))
	bad := append([]byte(nil), good...)
	writeUint16(bad, lsaLengthOff, uint16(len(bad)+1))
	it := NewLSAIterator(bad)
	if it.Next() {
		t.Fatalf("iterator unexpectedly accepted truncated LSA")
	}
	if it.Err() == nil {
		t.Fatalf("iterator Err nil, want truncation")
	}
}
