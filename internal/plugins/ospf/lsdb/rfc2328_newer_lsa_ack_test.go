// VALIDATES: RFC 2328 Section 13.5 through ReceiveUpdate: a newly received LSA that is
// installed and not flooded back is acknowledged with an LS Ack sent on the receiving
// interface, and one flooded back is acknowledged implicitly by that LS Update.
// PREVENTS: a ReceiveUpdate that skips or mis-flags the acknowledgment of an ordinary
// newer LSA; TestOSPFAckDecisionTable reaches ackForReceive with hand-set flags only.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13.5-1 positive -- a new router-LSA received through ReceiveUpdate is acknowledged on the receiving interface in every state: on a point-to-point interface, a DROther and a Backup (each from the DR) by an LS Ack carrying its header once the delayed acknowledgments are flushed, and on the DR receiving from a DROther implicitly, by an LS Update carrying the LSA back out the receiving interface (ReceiveUpdate and ackForReceive, flooding.go).
func TestRFC2328NewerLSAAcknowledgedThroughReceiveUpdate(t *testing.T) {
	// Goal: the ordinary Table 19 row reached the way a packet reaches it.
	// Method: eth0 is the receiving interface in the state under test, eth1 a second
	// broadcast interface with a Full neighbor so the LSA is flooded somewhere.
	cases := []struct {
		name, network, state string
		dr, sender           string
		implicit             bool
	}{
		{"point-to-point", types.NetworkPointToPoint, "point-to-point", "0.0.0.0", "2.2.2.2", false},
		{"dr-other from the DR", types.NetworkBroadcast, "dr-other", "2.2.2.2", "2.2.2.2", false},
		{"backup from the DR", types.NetworkBroadcast, InterfaceStateBackup, "2.2.2.2", "2.2.2.2", false},
		{"DR from a DROther", types.NetworkBroadcast, InterfaceStateDR, "1.1.1.1", "2.2.2.2", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := &fakeClock{now: time.Unix(0, 0)}
			db := newTestDB(clock)
			tx := &txRecorder{}
			db.SetTx(tx.Send)
			db.SetTopology(func() []InterfaceInfo {
				ifs := floodTopology()
				ifs[0].NetworkType = tc.network
				ifs[0].State = tc.state
				ifs[0].DR = rid(tc.dr)
				ifs[0].BDR = types.RouterID{}
				// A second Full DROther, so the DR has someone to flood back to.
				ifs[0].Neighbors = append(ifs[0].Neighbors, NeighborInfo{RouterID: rid("5.5.5.5"), Address: naddr4("10.0.0.5"), State: NeighborStateFull})
				return ifs
			})
			lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
			reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid(tc.sender), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
			if reason != "" {
				t.Fatalf("ReceiveUpdate reason = %q", reason)
			}
			if _, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key()); !ok {
				t.Fatalf("setup: the new LSA was not installed")
			}
			db.FlushDelayedAcks("eth0")
			acked := false
			for _, a := range ackPackets(tx) {
				if a.iface != "eth0" {
					continue
				}
				for _, h := range a.pkt.LSAck.Headers {
					if h.Key() == lsa.Header.Key() && h.Sequence == lsa.Header.Sequence {
						acked = true
					}
				}
			}
			floodedBack := lsaFloodedOn(tx, "eth0", lsa.Header.Key())
			if tc.implicit {
				if !floodedBack {
					t.Fatalf("the DR did not flood the LSA back out eth0 (the implicit acknowledgment): %+v", tx.sends)
				}
				return
			}
			if !acked {
				t.Fatalf("no LS Ack on eth0 carries the new LSA's header: %+v", tx.sends)
			}
		})
	}
}
