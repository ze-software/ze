package peer

import (
	"net/netip"
	"testing"
)

// TestSortConnBatchRemoteIP verifies remote-ip mapping remains stable when
// router IDs change across reload batches.
//
// VALIDATES: conn_map remote-ip orders connections by TCP source address.
// PREVENTS: Router-id rotations making reload tests depend on accept order.
func TestSortConnBatchRemoteIP(t *testing.T) {
	conns := []connWithID{
		{routerID: 0x090A0B0C, remoteIP: netip.MustParseAddr("127.0.0.3")},
		{routerID: 0x01020304, remoteIP: netip.MustParseAddr("127.0.0.1")},
		{routerID: 0x05060708, remoteIP: netip.MustParseAddr("127.0.0.2")},
	}

	sortConnBatch(conns, connMapRemoteIP)

	want := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("127.0.0.2"),
		netip.MustParseAddr("127.0.0.3"),
	}
	for i := range want {
		if conns[i].remoteIP != want[i] {
			t.Fatalf("conns[%d].remoteIP = %s, want %s", i, conns[i].remoteIP, want[i])
		}
	}
}

// TestSortConnBatchRouterID verifies the existing router-id mapping remains
// available for concurrent sessions whose TCP source addresses are not stable.
//
// VALIDATES: conn_map router-id orders connections by OPEN router-id.
// PREVENTS: Remote-ip support changing existing router-id mapping semantics.
func TestSortConnBatchRouterID(t *testing.T) {
	conns := []connWithID{
		{routerID: 0x090A0B0C, remoteIP: netip.MustParseAddr("127.0.0.1")},
		{routerID: 0x01020304, remoteIP: netip.MustParseAddr("127.0.0.3")},
		{routerID: 0x05060708, remoteIP: netip.MustParseAddr("127.0.0.2")},
	}

	sortConnBatch(conns, connMapRouterID)

	want := []uint32{0x01020304, 0x05060708, 0x090A0B0C}
	for i := range want {
		if conns[i].routerID != want[i] {
			t.Fatalf("conns[%d].routerID = %#x, want %#x", i, conns[i].routerID, want[i])
		}
	}
}
