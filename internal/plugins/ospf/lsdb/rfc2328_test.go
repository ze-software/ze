// VALIDATES: RFC 2328 Section 13 receive-side obligations that had no direct coverage:
// the LS-checksum discard (step 1), the "database copy is more recent" reply (step 8), and
// the confinement of the MaxAge+MaxSequenceNumber silent discard to that one case.
// PREVENTS: a corrupted LSA entering the LSDB, and a stale neighbor never being corrected
// because the newer database copy is not sent back to it.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13-1 negative -- an LSA whose covered region is corrupted fails the
// Fletcher check and is discarded by the flooding receive procedure: ReceiveUpdate returns
// "bad-lsa-checksum" and the LSA never reaches the database (ReceiveUpdate, flooding.go:161-164).
func TestRFC2328BadLSChecksumDiscarded(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	// Flip an octet inside the Fletcher-covered region (the Options octet at offset 2); the
	// decoded header is kept, so only the raw bytes the checksum covers are wrong.
	corrupt := make([]byte, len(lsa.RawBytes))
	copy(corrupt, lsa.RawBytes)
	corrupt[2] ^= 0xff
	bad := packet.LSA{Header: lsa.Header, Body: corrupt[types.LSAHeaderLen:], RawBytes: corrupt}
	if bad.VerifyChecksum() {
		t.Fatalf("precondition: the corrupted LSA must fail the Fletcher check")
	}

	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{bad}}})

	if reason != "bad-lsa-checksum" {
		t.Fatalf("ReceiveUpdate reason = %q, want bad-lsa-checksum", reason)
	}
	if _, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key()); ok {
		t.Fatalf("an LSA with an invalid LS checksum was installed")
	}
	if len(tx.sends) != 0 {
		t.Fatalf("a discarded LSA must not be flooded or acknowledged: %+v", tx.sends)
	}
}

// RFC requirement: RFC2328-13.1-1 negative -- an instance with a LOWER LS sequence number than
// the database copy is classified Older and never replaces it (CompareHeaders, entry.go:115-121;
// installLocked keeps the existing entry, lsdb.go:388-392).
// RFC requirement: RFC2328-13-4 negative -- the silent discard is confined to the wrapping case:
// with an ordinary (non-MaxAge, non-MaxSequenceNumber) database copy, the older instance is NOT
// swallowed -- the more recent database copy is sent straight back to the sender
// (ReceiveUpdate Older branch, flooding.go:229-240).
func TestRFC2328OlderInstanceGetsDatabaseCopyBack(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a0 := area("0.0.0.0")
	current := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 10)
	if !db.Install(a0, current) {
		t.Fatalf("install of the database copy was rejected")
	}
	tx.sends = nil

	older := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{older}}})

	got, ok := db.Lookup(a0, current.Header.Key())
	if !ok || got.Sequence != current.Header.Sequence {
		t.Fatalf("older instance replaced the database copy: %+v", got)
	}
	if len(tx.sends) != 1 || tx.sends[0].pkt.LSUpdate == nil {
		t.Fatalf("the database copy was not sent back to the sender: %+v", tx.sends)
	}
	if dst := tx.sends[0].dst; dst != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("database copy sent to %s, want the sender 10.0.0.2", dst)
	}
	if seq := tx.sends[0].pkt.LSUpdate.LSAs[0].Header.Sequence; seq != current.Header.Sequence {
		t.Fatalf("sent-back sequence = %v, want the database copy %v", seq, current.Header.Sequence)
	}
}
