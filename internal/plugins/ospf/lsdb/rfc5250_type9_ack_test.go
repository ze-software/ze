// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- link-local opaque scope.
// Related: flooding.go -- ackForReceive and FlushDelayedAcks, the acknowledgment path.
//
// VALIDATES: RFC 5250 section 3.1: a Type 9 Opaque LSA received on eth0 is discarded and not
// acknowledged toward the neighbor on eth1, the other interface of the same area: no
// LSAck and no LS Update carrying it leaves eth1. It is acknowledged on eth0, where it
// arrived.
// PREVENTS: a link-local LSA acknowledged, or flooded, toward a neighbor on another link.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc5250Type9Sends receives one Type 9 Opaque LSA from 2.2.2.2 on eth0, flushes the
// delayed acknowledgments of eth0 and eth1, and reports, per interface, whether an LSAck
// and whether an LS Update carrying that LSA was sent there.
func rfc5250Type9Sends(t *testing.T) (acked, flooded map[string]bool) {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTopology)
	lsa9 := opaqueLSA(t, types.LSTypeOpaqueLink, 1, 0x30, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"),
		Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa9}}})
	db.FlushDelayedAcks("eth0")
	db.FlushDelayedAcks("eth1")
	key := lsa9.Header.Key()
	acked, flooded = make(map[string]bool), make(map[string]bool)
	for i := range tx.sends {
		s := &tx.sends[i]
		if s.pkt.LSAck != nil {
			for _, h := range s.pkt.LSAck.Headers {
				if h.Key() == key {
					acked[s.iface] = true
				}
			}
		}
		if s.pkt.LSUpdate != nil {
			for _, l := range s.pkt.LSUpdate.LSAs {
				if l.Header.Key() == key {
					flooded[s.iface] = true
				}
			}
		}
	}
	return acked, flooded
}

// RFC requirement: RFC5250-3.1-1 positive -- a Type 9 Opaque LSA received on eth0 is kept
// for eth0 and acknowledged there: after the delayed acknowledgments are flushed, an LSAck
// carrying its header leaves eth0.
func TestRFC5250Type9AcknowledgedOnArrivalInterface(t *testing.T) {
	// Goal: the link-local LSA is acknowledged on its own link. Method: one receive on eth0,
	// both interfaces' delayed acknowledgments flushed, the sent packets read.
	acked, _ := rfc5250Type9Sends(t)
	if !acked["eth0"] {
		t.Fatal("the Type 9 Opaque LSA received on eth0 was not acknowledged on eth0")
	}
}

// RFC requirement: RFC5250-3.1-1 negative -- toward eth1, which is not the interface the Type
// 9 Opaque LSA was received on, the LSA is discarded and not acknowledged: no LSAck carrying
// its header and no LS Update carrying it leaves eth1.
func TestRFC5250Type9NotAcknowledgedOnOtherInterface(t *testing.T) {
	// Goal: neither the acknowledgment nor the LSA crosses to another link. Method: the
	// positive's receive and flushes, the packets sent out eth1 read.
	acked, flooded := rfc5250Type9Sends(t)
	if acked["eth1"] {
		t.Fatal("the Type 9 Opaque LSA received on eth0 was acknowledged out eth1")
	}
	if flooded["eth1"] {
		t.Fatal("the Type 9 Opaque LSA received on eth0 was flooded out eth1")
	}
}
