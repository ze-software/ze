// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS SRv6 BGP-LS origination.
// RFC: rfc/short/rfc9514.md
// Related: bgpls_export_rfc9514_test.go -- rfc9514Snapshot, rfc9514Attributes
//
// VALIDATES: the originate clause of the RFC 9514 sentences that state both
// halves, "MUST be set to 0 when originated and ignored on receipt". The
// receipt half is proven on the consumer (nlri/ls reserved_rfc9514_test.go);
// these tests prove the half Ze's IS-IS producer owes, on the TLVs it hands to
// the BGP-LS exporter.
// PREVENTS: a native IS-IS bit landing in an RFC 9514 Reserved field or an
// undefined flag of the originated TLV.
package isis

import (
	"bytes"
	"testing"
)

// requireEndXReserved originates one node whose End.X and LAN End.X SIDs carry
// the given native flags and weight, and checks the Reserved octet of the
// originated TLVs 1106 and 1107 (the octet after Weight) is 0.
func requireEndXReserved(t *testing.T, flags, weight byte) {
	t.Helper()
	snapshot := rfc9514Snapshot(t, rfc9514Inputs{capabilities: [][]byte{{0x40, 0}}, adjFlags: flags, adjWeight: weight})
	if len(snapshot.Links) != 1 {
		t.Fatalf("links = %+v", snapshot.Links)
	}
	for _, typ := range []uint16{1106, 1107} {
		got := rfc9514Attributes(snapshot.Links[0].Attributes, typ)
		if len(got) != 1 {
			t.Fatalf("TLV %d = %x, want one", typ, got)
		}
		if got[0][5] != 0 {
			t.Fatalf("native flags %x weight %x: TLV %d Reserved = %x, want 00", flags, weight, typ, got[0][5])
		}
	}
}

// TestRFC9514ISISEndXOriginatedReservedZero is the ordinary case: native
// values that set no bit near the Reserved octet.
//
// RFC requirement: RFC9514-4.1-2 positive -- the End.X SID TLV 1106 the IS-IS source originates from ordinary native flags and weight has Reserved octet 0 (§4.1).
// RFC requirement: RFC9514-4.2-2 positive -- the LAN End.X SID TLV 1107 the IS-IS source originates from ordinary native flags and weight has Reserved octet 0 (§4.2).
func TestRFC9514ISISEndXOriginatedReservedZero(t *testing.T) {
	requireEndXReserved(t, 0x20, 1)
}

// TestRFC9514ISISEndXAllOnesNeverReachReserved drives every native bit beside
// the Reserved octet to 1, the input that would set it if the producer copied
// one field too many.
//
// RFC requirement: RFC9514-4.1-2 negative -- native End.X flags and weight of 0xff never reach the Reserved octet of the originated TLV 1106: it is still 0 (§4.1).
// RFC requirement: RFC9514-4.2-2 negative -- native LAN End.X flags and weight of 0xff never reach the Reserved octet of the originated TLV 1107: it is still 0 (§4.2).
func TestRFC9514ISISEndXAllOnesNeverReachReserved(t *testing.T) {
	requireEndXReserved(t, 0xff, 0xff)
}

// requireEndpointBehaviorFlags originates one End SID with the given native
// flags and checks the originated SRv6 Endpoint Behavior TLV 1250 is exactly
// Behavior 0x0001, Flags 0, Algorithm 0.
func requireEndpointBehaviorFlags(t *testing.T, flags byte) {
	t.Helper()
	snapshot := rfc9514Snapshot(t, rfc9514Inputs{capabilities: [][]byte{{0x40, 0}}, endFlags: flags})
	if len(snapshot.SIDs) != 1 {
		t.Fatalf("SIDs = %+v", snapshot.SIDs)
	}
	got := rfc9514Attributes(snapshot.SIDs[0].Attributes, 1250)
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0, 1, 0, 0}) {
		t.Fatalf("native flags %x: Endpoint Behavior = %x, want 00010000", flags, got)
	}
}

// TestRFC9514ISISEndpointBehaviorOriginatedFlagsZero is the ordinary case: an
// End SID whose native flags are clear.
//
// RFC requirement: RFC9514-7.1-4 positive -- an End SID with native flags 0 originates the Endpoint Behavior TLV 1250 with every Flags bit, the undefined ones included, 0 (§7.1).
func TestRFC9514ISISEndpointBehaviorOriginatedFlagsZero(t *testing.T) {
	requireEndpointBehaviorFlags(t, 0)
}

// TestRFC9514ISISEndpointBehaviorNativeFlagsNeverOriginated sets every native
// End SID flag, the input that would set undefined flags if the producer copied
// the native octet.
//
// RFC requirement: RFC9514-7.1-4 negative -- an End SID with native flags 0xff still originates the Endpoint Behavior TLV 1250 with Flags 0: no undefined flag is set (§7.1).
func TestRFC9514ISISEndpointBehaviorNativeFlagsNeverOriginated(t *testing.T) {
	requireEndpointBehaviorFlags(t, 0xff)
}
