// VALIDATES: a Link State Acknowledgment or Link State Update the database sends to
// one neighbor leaves for AllSPFRouters on a point-to-point interface, and for the
// neighbor's own address on every other network type (RFC 2328 Section 8.1).
// PREVENTS: an unnumbered point-to-point link losing its acknowledgments and
// retransmissions, because the neighbor's donor address is a unicast no route on
// that link reaches (interop scenario ospf-p2p-unnumbered-frr).
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestOSPFNeighborPacketDestinationByNetwork checks the rule itself over each
// network type, and that an OSPFv3 address passes through unchanged.
func TestOSPFNeighborPacketDestinationByNetwork(t *testing.T) {
	neighbor := netip.MustParseAddr("10.255.0.3")
	cases := []struct {
		network string
		want    netip.Addr
	}{
		{types.NetworkPointToPoint, transport.AllSPFRouters},
		{types.NetworkBroadcast, neighbor},
		{types.NetworkNBMA, neighbor},
		{types.NetworkPointToMultipoint, neighbor},
		{types.NetworkVirtual, neighbor},
	}
	for _, tc := range cases {
		if got := transport.PacketDestination(tc.network, neighbor); got != tc.want {
			t.Errorf("PacketDestination(%s, %s) = %s, want %s", tc.network, neighbor, got, tc.want)
		}
	}
	linkLocal := netip.MustParseAddr("fe80::2")
	if got := transport.PacketDestination(types.NetworkPointToPoint, linkLocal); got != linkLocal {
		t.Errorf("PacketDestination(point-to-point, %s) = %s, want it unchanged", linkLocal, got)
	}
}

// TestOSPFDirectAckDestinationFollowsNetwork sends a direct acknowledgment to the
// source of a received packet through the database, as Table 19 does, once on a
// point-to-point interface and once on a broadcast one, and reads the destination
// the transmit hook was handed.
func TestOSPFDirectAckDestinationFollowsNetwork(t *testing.T) {
	source := netip.MustParseAddr("10.255.0.3")
	cases := []struct {
		network string
		want    netip.Addr
	}{
		{types.NetworkPointToPoint, transport.AllSPFRouters},
		{types.NetworkBroadcast, source},
	}
	for _, tc := range cases {
		clock := &fakeClock{now: time.Unix(0, 0)}
		db := newTestDB(clock)
		tx := &txRecorder{}
		a := area("0.0.0.0")
		db.SetTx(tx.Send)
		db.SetTopology(func() []InterfaceInfo {
			return []InterfaceInfo{{
				Name: "eth0", AreaID: a, AreaType: types.AreaTypeNormal, NetworkType: tc.network,
				Address: ip4("10.255.0.2"), NetworkMask: ip4("255.255.255.255"), RouterID: rid("10.255.0.2"),
			}}
		})
		lsa := routerLSA(t, rid("10.255.0.3"), types.InitialSequenceNumber, 10)
		db.sendAck("eth0", source, a, []packet.LSAHeader{lsa.Header})
		if len(tx.sends) != 1 {
			t.Fatalf("%s: sent %d packets, want 1", tc.network, len(tx.sends))
		}
		if got := tx.sends[0].dst; got != tc.want {
			t.Errorf("%s: acknowledgment destination = %s, want %s", tc.network, got, tc.want)
		}
	}
}
