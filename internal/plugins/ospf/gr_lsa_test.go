// VALIDATES: the IPv4 (RFC 3623 sec A) Grace-LSA body glue builds the mandatory Grace Period
// (type 1) + Restart Reason (type 2) TLVs on every interface and adds the type-3 IP Interface
// Address TLV only on shared media (broadcast/NBMA/P2MP), and that the body round-trips
// through the ext-1 opaque TLV codec.
// PREVENTS: a malformed Grace-LSA (missing mandatory TLV, or a stray/absent interface-address
// TLV) that a helper or FRR would reject.
package ospf

import (
	"testing"

	ospfpacket "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
)

// TestGraceLSAv4BodyBuild (AC-3): the body always carries the type-1 Grace Period and type-2
// Reason; the type-3 IP Interface Address TLV appears only on shared media.
func TestGraceLSAv4BodyBuild(t *testing.T) {
	// RFC requirement: RFC3623-A-4 positive -- a shared-media (broadcast/NBMA/P2MP) grace-LSA
	// carries the type-3 IP Interface Address TLV (RFC 3623 sec A, sec 3.1): grV4Body with
	// sharedMedia=true sets HasInterfaceAddr, so EncodeGraceLSA appends the type-3 TLV
	// (gr_lsa.go:19-27, packet/grace_lsa.go:51-53) and the address round-trips.
	// RFC requirement: RFC3623-A-4 negative -- a non-shared (point-to-point / virtual-link)
	// grace-LSA OMITS the type-3 TLV: grV4Body with sharedMedia=false leaves HasInterfaceAddr
	// clear, so no type-3 TLV is emitted (the p2p case below asserts !g2.HasInterfaceAddr).
	addr := [4]byte{192, 0, 2, 5}

	shared := grV4Body(120, 2, addr, true)
	g, err := grV4Parse(shared)
	if err != nil {
		t.Fatalf("parse shared-media body: %v", err)
	}
	if g.GracePeriod != 120 || g.Reason != 2 {
		t.Fatalf("mandatory TLVs wrong: %+v", g)
	}
	if !g.HasInterfaceAddr || g.InterfaceAddr != addr {
		t.Fatalf("shared media must carry the type-3 IP address TLV: %+v", g)
	}

	p2p := grV4Body(60, 1, addr, false)
	g2, err := grV4Parse(p2p)
	if err != nil {
		t.Fatalf("parse p2p body: %v", err)
	}
	if g2.GracePeriod != 60 || g2.Reason != 1 {
		t.Fatalf("mandatory TLVs wrong on p2p: %+v", g2)
	}
	if g2.HasInterfaceAddr {
		t.Fatalf("non-shared media must NOT carry the type-3 IP address TLV: %+v", g2)
	}
}

// TestGraceLSAv4TLVRoundTrip (A-5): a built body decodes back to the same field values via
// the shared ext-1 opaque TLV iterator/builder (4-octet aligned).
func TestGraceLSAv4TLVRoundTrip(t *testing.T) {
	want := ospfpacket.GraceLSA{GracePeriod: 1800, Reason: 3, HasInterfaceAddr: true, InterfaceAddr: [4]byte{10, 1, 2, 3}}
	body := ospfpacket.EncodeGraceLSA(want)
	if len(body)%4 != 0 {
		t.Fatalf("body not 4-octet aligned: len=%d", len(body))
	}
	got, err := grV4Parse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
}
