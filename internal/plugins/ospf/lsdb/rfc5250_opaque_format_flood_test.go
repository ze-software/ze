// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- LS Update receive procedure
// Related: rfc5250_opaque_scope_test.go -- the LS-type and flooding-scope clauses of RFC5250-3-1
//
// VALIDATES: RFC 5250 Section 3, the two clauses rfc5250_opaque_scope_test.go leaves open: an
// Opaque LSA is a standard LSA header followed by a 32-bit aligned body, and Opaque LSAs
// are distributed by the standard link-state database flooding mechanisms.
// PREVENTS: storing and reflooding an Opaque LSA whose body breaks the RFC 5250 format,
// and a bespoke opaque path that refloods without the RFC 2328 Section 13 freshness test.
package lsdb

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// receiveOpaqueOnEth0 delivers one LS Update from 2.2.2.2 (10.0.0.2) on eth0 of the
// opaqueTopology: eth0 to 2.2.2.2, eth1 to 3.3.3.3, both opaque-capable, area 0.
func receiveOpaqueOnEth0(t *testing.T, db *LSDB, lsas ...packet.LSA) {
	t.Helper()
	reason := db.ReceiveUpdate(ReceiveInput{
		Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"),
		Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: lsas},
	})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
}

// updatesOnEth1Carrying counts the LS Updates sent out eth1 that carry an LSA with key.
func updatesOnEth1Carrying(tx *txRecorder, key types.LSAKey) int {
	count := 0
	for i := range tx.sends {
		sent := tx.sends[i]
		if sent.iface != "eth1" || sent.pkt.LSUpdate == nil {
			continue
		}
		for _, lsa := range sent.pkt.LSUpdate.LSAs {
			if lsa.Header.Key() == key {
				count++
			}
		}
	}
	return count
}

// acksOnEth0Naming counts the LS Acks sent out eth0 that name an LSA with key.
func acksOnEth0Naming(tx *txRecorder, key types.LSAKey) int {
	count := 0
	acks := ackPackets(tx)
	for i := range acks {
		if acks[i].iface != "eth0" {
			continue
		}
		for _, h := range acks[i].pkt.LSAck.Headers {
			if h.Key() == key {
				count++
			}
		}
	}
	return count
}

// RFC requirement: RFC5250-3-1 positive -- a Type 10 Opaque LSA whose body is 32-bit aligned (8 octets, LS length 28), received on eth0, is stored and flooded by the standard procedure: an LS Update carrying it, length 28 and body intact, leaves on eth1 and it is queued for retransmission to 3.3.3.3.
func TestRFC5250AlignedOpaqueStoredAndFlooded(t *testing.T) {
	// Method: a real LSDB with two point-to-point interfaces in area 0; the Opaque LSA
	// goes through ReceiveUpdate, the RFC 2328 Section 13 entry point.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTopology)
	body := []byte{0x00, 0x01, 0x00, 0x04, 0xc0, 0x00, 0x02, 0x01}
	aligned := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x11, rid("2.2.2.2"), types.InitialSequenceNumber, body)
	key := aligned.Header.Key()

	receiveOpaqueOnEth0(t, db, aligned)

	stored, ok := db.LookupLSA(area("0.0.0.0"), key)
	if !ok {
		t.Fatalf("aligned Type 10 Opaque LSA not stored")
	}
	if stored.Header.Length != 28 {
		t.Fatalf("stored LS length = %d, want 28", stored.Header.Length)
	}
	if got := updatesOnEth1Carrying(tx, key); got != 1 {
		t.Fatalf("LS Updates on eth1 carrying the Opaque LSA = %d, want 1", got)
	}
	for i := range tx.sends {
		sent := tx.sends[i]
		if sent.iface != "eth1" || sent.pkt.LSUpdate == nil {
			continue
		}
		for _, lsa := range sent.pkt.LSUpdate.LSAs {
			if lsa.Header.Key() != key {
				continue
			}
			if lsa.Header.Length != 28 || !bytes.Equal(lsa.Body, body) {
				t.Fatalf("flooded Opaque LSA length %d body % x, want 28 and % x", lsa.Header.Length, lsa.Body, body)
			}
		}
	}
	if db.retransmit[NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}][key] == nil {
		t.Fatalf("aligned Opaque LSA not on 3.3.3.3's retransmission list")
	}
}

// RFC requirement: RFC5250-3-1 negative -- a Type 10 Opaque LSA whose body is not 32-bit aligned (5 octets, LS length 25, valid LS checksum) is discarded: not stored, not acknowledged, not flooded out eth1; the aligned Opaque LSA after it in the same LS Update is still stored.
func TestRFC5250UnalignedOpaqueBodyDiscarded(t *testing.T) {
	// Method: the unaligned LSA carries a correct Fletcher checksum, so only the
	// RFC 5250 format refuses it; its aligned companion proves the update goes on.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTopology)
	unaligned := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x12, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{0x00, 0x01, 0x00, 0x01, 0xaa})
	if unaligned.Header.Length != 25 {
		t.Fatalf("test LSA length = %d, want 25", unaligned.Header.Length)
	}
	companion := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x13, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{0xde, 0xad, 0xbe, 0xef})
	key := unaligned.Header.Key()

	receiveOpaqueOnEth0(t, db, unaligned, companion)
	clock.Add(2 * time.Second)
	db.FlushDelayedAcks("eth0")

	if _, ok := db.LookupLSA(area("0.0.0.0"), key); ok {
		t.Fatalf("Opaque LSA with a 5-octet body was stored")
	}
	if got := updatesOnEth1Carrying(tx, key); got != 0 {
		t.Fatalf("Opaque LSA with a 5-octet body flooded out eth1 %d times", got)
	}
	if got := acksOnEth0Naming(tx, key); got != 0 {
		t.Fatalf("Opaque LSA with a 5-octet body acknowledged %d times", got)
	}
	if _, ok := db.LookupLSA(area("0.0.0.0"), companion.Header.Key()); !ok {
		t.Fatalf("aligned companion Opaque LSA in the same LS Update not stored")
	}
}

// RFC requirement: RFC5250-3-1 negative -- an Opaque LSA goes through the standard freshness test: a second copy of the same Type 10 instance, arriving after MinLSArrival, is not flooded out eth1 again (still one LS Update there) and is acknowledged to its sender on eth0.
func TestRFC5250DuplicateOpaqueNotReflooded(t *testing.T) {
	// Method: the same instance is received twice through ReceiveUpdate, 2 s apart,
	// past MinLSArrival (1 s), so only the Section 13 freshness comparison stops it.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTopology)
	opaque := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x14, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{0x00, 0x01, 0x00, 0x00})
	key := opaque.Header.Key()

	receiveOpaqueOnEth0(t, db, opaque)
	if got := updatesOnEth1Carrying(tx, key); got != 1 {
		t.Fatalf("first copy: LS Updates on eth1 = %d, want 1", got)
	}
	clock.Add(2 * time.Second)
	db.FlushDelayedAcks("eth0")
	acksBefore := acksOnEth0Naming(tx, key)

	receiveOpaqueOnEth0(t, db, opaque)
	clock.Add(2 * time.Second)
	db.FlushDelayedAcks("eth0")

	if got := updatesOnEth1Carrying(tx, key); got != 1 {
		t.Fatalf("duplicate copy reflooded: LS Updates on eth1 = %d, want 1", got)
	}
	if got := acksOnEth0Naming(tx, key); got != acksBefore+1 {
		t.Fatalf("duplicate copy acknowledgments on eth0 = %d, want %d", got, acksBefore+1)
	}
}
