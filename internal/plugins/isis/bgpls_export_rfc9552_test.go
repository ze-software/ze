// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS router identifiers.
// RFC: rfc/short/rfc9552.md

package isis

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// rfc9552RouterIDSnapshot stores two L1 LSPs, nodes 0000.0000.0002 and
// 0000.0000.0003, each with a TE Router ID (TLV 134) and an extended IS
// reachability (TLV 22) towards the other, then builds the native snapshot.
func rfc9552RouterIDSnapshot(t *testing.T) *linkstateevents.Snapshot {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	for _, n := range []struct{ self, peer byte }{{2, 3}, {3, 2}} {
		id := types.LSPID{0, 0, 0, 0, 0, n.self, 0, 0}
		bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
			{Type: 134, Value: []byte{192, 0, 2, n.self}},
			{Type: 22, Value: []byte{0, 0, 0, 0, 0, n.peer, 0, 0, 0, 10, 0}},
		})
	}
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	return &builder.snapshot
}

func rfc9552HasAttribute(attrs []linkstateevents.TLV, typ uint16, value []byte) bool {
	for _, attr := range attrs {
		if attr.Type == typ && bytes.Equal(attr.Value, value) {
			return true
		}
	}
	return false
}

// VALIDATES: each IS-IS node's TE Router ID reaches its BGP-LS Node attribute
// as TLV 1028.
// PREVENTS: a node advertised without the Router-ID its IGP carries.
func TestRFC9552ISISNodeRouterID(t *testing.T) {
	// RFC requirement: RFC9552-5.2.1-1 positive -- the IPv4 TE Router ID of each IS-IS node is carried as TLV 1028 in that node's attributes.
	snapshot := rfc9552RouterIDSnapshot(t)
	if len(snapshot.Nodes) != 2 {
		t.Fatalf("nodes = %+v", snapshot.Nodes)
	}
	for _, node := range snapshot.Nodes {
		self := node.ID.RouterID[5]
		if !rfc9552HasAttribute(node.Attributes, 1028, []byte{192, 0, 2, self}) {
			t.Fatalf("node %d attributes %+v lack TLV 1028 192.0.2.%d", self, node.Attributes, self)
		}
	}
}

// VALIDATES: a link's attributes carry the local node's Router-ID as TLV 1028
// and the remote node's as TLV 1030, and never the other way round.
// PREVENTS: a link without both Router-IDs, or with the two sides swapped.
func TestRFC9552ISISLinkRouterIDs(t *testing.T) {
	snapshot := rfc9552RouterIDSnapshot(t)
	if len(snapshot.Links) == 0 {
		t.Fatalf("links = %+v", snapshot.Links)
	}
	for _, link := range snapshot.Links {
		local, remote := link.Local.RouterID[5], link.Remote.RouterID[5]
		// RFC requirement: RFC9552-5.3.2.1-1 positive -- every IS-IS link carries the local node's Router-ID as TLV 1028 and the remote node's as TLV 1030.
		if !rfc9552HasAttribute(link.Attributes, 1028, []byte{192, 0, 2, local}) ||
			!rfc9552HasAttribute(link.Attributes, 1030, []byte{192, 0, 2, remote}) {
			t.Fatalf("link %d->%d attributes %+v lack its Router-IDs", local, remote, link.Attributes)
		}
		// RFC requirement: RFC9552-5.3.2.1-1 negative -- no IS-IS link carries the remote Router-ID as its local TLV 1028 or the local Router-ID as its remote TLV 1030.
		if rfc9552HasAttribute(link.Attributes, 1028, []byte{192, 0, 2, remote}) ||
			rfc9552HasAttribute(link.Attributes, 1030, []byte{192, 0, 2, local}) {
			t.Fatalf("link %d->%d attributes %+v swap its Router-IDs", local, remote, link.Attributes)
		}
	}
}
