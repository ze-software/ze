// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE SA table
// Related: table.go -- SATable.lookupInbound, the mapping both receive loops use

package engine

import "testing"

// spiidFixture is a table holding two established SAs that share one initiator SPI
// (the responder chose different SPIs) and a half-open initiator SA whose responder SPI
// is not yet known, with the SPIs used.
type spiidFixture struct {
	tbl                             *SATable
	shared, first, second, halfOpen [8]byte
	a, b, c                         *SA
}

// spiidTable builds the fixture.
func spiidTable(t *testing.T) *spiidFixture {
	t.Helper()
	f := &spiidFixture{
		tbl:      NewSATable(),
		shared:   [8]byte{0x11, 1, 1, 1, 1, 1, 1, 1},
		first:    [8]byte{0x22, 2, 2, 2, 2, 2, 2, 2},
		second:   [8]byte{0x33, 3, 3, 3, 3, 3, 3, 3},
		halfOpen: [8]byte{0x44, 4, 4, 4, 4, 4, 4, 4},
	}
	f.a = &SA{InitiatorSPI: f.shared, ResponderSPI: f.first, PeerName: "a"}
	f.b = &SA{InitiatorSPI: f.shared, ResponderSPI: f.second, PeerName: "b"}
	f.c = &SA{InitiatorSPI: f.halfOpen, PeerName: "half-open"}
	for _, sa := range []*SA{f.a, f.b, f.c} {
		if !f.tbl.Insert(sa) {
			t.Fatalf("inserting SA %q: duplicate SPI pair", sa.PeerName)
		}
	}
	return f
}

// VALIDATES: RFC 7296 Section 2.6, the two IKE SPIs in the header are the connection
// identifier: an incoming packet reaches the IKE SA its SPI pair names.
// PREVENTS: a packet reaching an IKE SA its pair does not name.
//
// METHOD: SATable.lookupInbound, which both receive loops call, is asked for each pair.
//
// RFC requirement: RFC7296-2.6-1 positive -- two IKE SAs sharing the initiator SPI are
// each reached by their own full SPI pair; an IKE_SA_INIT response, which carries the
// responder's newly chosen SPI, reaches the half-open SA whose responder SPI is still zero;
// and a retransmitted IKE_SA_INIT request, whose responder SPI is zero ("the remote SPI
// value is not yet known by the sender"), reaches the SA of its initiator SPI.
func TestRFC7296IKESPIPairIdentifiesTheIKESA(t *testing.T) {
	f := spiidTable(t)
	tbl, shared, first, second, halfOpen, a, b, c := f.tbl, f.shared, f.first, f.second, f.halfOpen, f.a, f.b, f.c
	if got := tbl.lookupInbound(shared, first); got != a {
		t.Errorf("pair (shared, first) reached %v, want SA a", got)
	}
	if got := tbl.lookupInbound(shared, second); got != b {
		t.Errorf("pair (shared, second) reached %v, want SA b", got)
	}
	response := [8]byte{0x55, 5, 5, 5, 5, 5, 5, 5}
	if got := tbl.lookupInbound(halfOpen, response); got != c {
		t.Errorf("an IKE_SA_INIT response for the half-open SA reached %v, want SA c", got)
	}
	responder := &SA{InitiatorSPI: [8]byte{0x77, 7, 7, 7, 7, 7, 7, 7}, ResponderSPI: response, PeerName: "responder"}
	if !tbl.Insert(responder) {
		t.Fatal("inserting the responder SA: duplicate SPI pair")
	}
	if got := tbl.lookupInbound(responder.InitiatorSPI, [8]byte{}); got != responder {
		t.Errorf("a retransmitted IKE_SA_INIT request (zero responder SPI) reached %v, want the responder SA", got)
	}
}

// RFC requirement: RFC7296-2.6-1 negative -- a packet whose initiator SPI names established
// IKE SAs but whose non-zero responder SPI is neither of theirs maps to no IKE SA, and
// neither does a known responder SPI under an unknown initiator SPI: the pair identifies
// the SA, never one of its halves.
func TestRFC7296UnknownIKESPIPairMapsToNoSA(t *testing.T) {
	f := spiidTable(t)
	tbl, shared, first := f.tbl, f.shared, f.first
	stranger := [8]byte{0x66, 6, 6, 6, 6, 6, 6, 6}
	if got := tbl.lookupInbound(shared, stranger); got != nil {
		t.Errorf("pair (shared, stranger) reached SA %q, want none", got.PeerName)
	}
	if got := tbl.lookupInbound(stranger, first); got != nil {
		t.Errorf("pair (stranger, first) reached SA %q, want none", got.PeerName)
	}
}
