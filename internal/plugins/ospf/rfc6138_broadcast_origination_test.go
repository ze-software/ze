// Design: docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md -- LDP-IGP sync on broadcast links.
// Related: ldp_sync.go -- applyLDPSyncOverride, the producer these tests drive.
// Related: rfc5443_origination_test.go -- the point-to-point counterpart and shared helpers.
//
// VALIDATES: RFC 5443 section 3 and RFC 6138 section 4 on the ORIGINATED Router-LSA of a
// broadcast segment with two IGP/LDP peers. Each test builds a real engine whose eth1 is a
// broadcast interface with ldp-sync enabled, gives it a real SPF computer over a three-router
// LSDB, runs the production applyLDPSyncOverride (which asks the SPF computer whether the
// segment is a cut-edge), originates the Router-LSA into a fresh LSDB, and decodes it.
// PREVENTS: a cost-out applied to one peer on a shared segment (including a peer whose LDP
// session is already up), and a cut-edge segment whose transit link waits for LDP.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

var (
	rfc6138Self  = types.RouterID{1, 1, 1, 1}
	rfc6138PeerB = types.RouterID{2, 2, 2, 2}
	rfc6138PeerC = types.RouterID{3, 3, 3, 3}
	// rfc6138LAN is this router's eth1 address. This router is the DR, so it is also the
	// Link State ID of the segment's Network-LSA (the pseudonode).
	rfc6138LAN = [4]byte{10, 0, 0, 1}
)

// rfc6138Engine builds an engine whose only interface, eth1, is broadcast with ldp-sync
// enabled. The LDP-sync machine runs on fake timers so the test controls the hold-down.
func rfc6138Engine(t *testing.T) (*engine, *fakeTimers) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth1":{"name":"eth1","area":"0","network-type":"broadcast","cost":"10",`+
		`"ldp-sync":{"enable":true,"holddown":"30"}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(nil)
	ft := &fakeTimers{}
	eng.ldpSync.afterFunc = ft.afterFunc
	eng.setConfig(cfg)
	rfc5443Reconcile(t, eng, &cfg)
	return eng, ft
}

// rfc6138LANInfo is router's view of the shared segment: it has address local, the DR is
// this router (1.1.1.1 at 10.0.0.1), and every router in peers is a Full neighbor.
func rfc6138LANInfo(router types.RouterID, local [4]byte, peers map[types.RouterID]string) ospflsdb.InterfaceInfo {
	info := ospflsdb.InterfaceInfo{
		Name: "eth1", AreaID: types.BackboneArea, NetworkType: types.NetworkBroadcast, State: "dr",
		Address: local, NetworkMask: [4]byte{255, 255, 255, 0}, Cost: 10, RouterID: router,
		Options: types.OptionE, DR: rfc6138Self,
	}
	for rid, addr := range peers {
		info.Neighbors = append(info.Neighbors, ospflsdb.NeighborInfo{RouterID: rid, Address: netip.MustParseAddr(addr), State: ospflsdb.NeighborStateFull})
	}
	return info
}

// rfc6138P2P is router's point-to-point link to peer over local.
func rfc6138P2P(router types.RouterID, local [4]byte, peer types.RouterID) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{
		Name: "eth2", AreaID: types.BackboneArea, NetworkType: types.NetworkPointToPoint, State: "point-to-point",
		Address: local, Cost: 5, RouterID: router, Options: types.OptionE,
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: peer, State: ospflsdb.NeighborStateFull}},
	}
}

// rfc6138SPF gives eng a real SPF computer over a three-router LSDB: 1.1.1.1 (DR), 2.2.2.2 and
// 3.3.3.3 share the segment 10.0.0.0/24. With alternate set, 1.1.1.1 and 2.2.2.2 also share a
// point-to-point link, so the segment stays reachable without 1.1.1.1's own transit link.
func rfc6138SPF(t *testing.T, eng *engine, alternate bool) {
	t.Helper()
	topo := ospflsdb.New(func() time.Time { return time.Unix(0, 0) })
	selfIfaces := []ospflsdb.InterfaceInfo{rfc6138LANInfo(rfc6138Self, rfc6138LAN, map[types.RouterID]string{rfc6138PeerB: "10.0.0.2", rfc6138PeerC: "10.0.0.3"})}
	peerIfaces := []ospflsdb.InterfaceInfo{rfc6138LANInfo(rfc6138PeerB, [4]byte{10, 0, 0, 2}, map[types.RouterID]string{rfc6138Self: "10.0.0.1"})}
	if alternate {
		selfIfaces = append(selfIfaces, rfc6138P2P(rfc6138Self, [4]byte{172, 16, 0, 1}, rfc6138PeerB))
		peerIfaces = append(peerIfaces, rfc6138P2P(rfc6138PeerB, [4]byte{172, 16, 0, 2}, rfc6138Self))
	}
	origins := []ospflsdb.OriginInput{
		{AreaID: types.BackboneArea, RouterID: rfc6138Self, Options: types.OptionE, Interfaces: selfIfaces},
		{AreaID: types.BackboneArea, RouterID: rfc6138PeerB, Options: types.OptionE, Interfaces: peerIfaces},
		{AreaID: types.BackboneArea, RouterID: rfc6138PeerC, Options: types.OptionE, Interfaces: []ospflsdb.InterfaceInfo{
			rfc6138LANInfo(rfc6138PeerC, [4]byte{10, 0, 0, 3}, map[types.RouterID]string{rfc6138Self: "10.0.0.1"}),
		}},
	}
	for _, in := range origins {
		if _, ok := topo.OriginateRouter(in); !ok {
			t.Fatalf("OriginateRouter %v returned false", in.RouterID)
		}
	}
	if _, ok := topo.OriginateNetwork(types.BackboneArea, rfc6138Self, types.OptionE, selfIfaces[0]); !ok {
		t.Fatal("OriginateNetwork returned false")
	}
	computer := ospfspf.NewComputer(ospfspf.Config{Source: topo, Root: rfc6138Self, Areas: []types.AreaID{types.BackboneArea}})
	t.Cleanup(computer.Stop)
	computer.Run()
	// newEngine started the engine's own SPF computer, whose post-run hook reads eng.spf on
	// a timer goroutine. Stop it (Stop waits for a run in flight) before replacing the field.
	if eng.spf != nil {
		eng.spf.Stop()
	}
	eng.spf = computer
}

// rfc6138OriginatedLinks runs the production LDP-sync override over eth1's snapshot, with both
// peers Full, originates the Router-LSA into a fresh LSDB, and returns its decoded links.
func rfc6138OriginatedLinks(t *testing.T, eng *engine) []packet.RouterLink {
	t.Helper()
	info := rfc6138LANInfo(rfc6138Self, rfc6138LAN, map[types.RouterID]string{rfc6138PeerB: "10.0.0.2", rfc6138PeerC: "10.0.0.3"})
	eng.applyLDPSyncOverride(&info, eng.cfg.Interfaces[0])
	in := ospflsdb.OriginInput{AreaID: types.BackboneArea, RouterID: rfc6138Self, Options: types.OptionE, Interfaces: []ospflsdb.InterfaceInfo{info}}
	db := ospflsdb.New(func() time.Time { return time.Unix(0, 0) })
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatal("OriginateRouter returned false")
	}
	lsa, ok := db.LookupLSA(types.BackboneArea, h.Key())
	if !ok {
		t.Fatal("originated Router-LSA not installed")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	return body.Links
}

// rfc6138CountLinks returns how many links there are of each type the segment can produce:
// the transit link to the pseudonode, any point-to-point link, and the subnet stub.
func rfc6138CountLinks(t *testing.T, links []packet.RouterLink) (transit, p2p, stub int) {
	t.Helper()
	for _, l := range links {
		switch l.Type {
		case packet.RouterLinkTypeTransit:
			if l.LinkID != types.LinkStateID(rfc6138LAN) {
				t.Fatalf("transit link to %v, want the pseudonode %v", l.LinkID, rfc6138LAN)
			}
			transit++
		case packet.RouterLinkTypeP2P:
			p2p++
		case packet.RouterLinkTypeStub:
			stub++
		}
	}
	return transit, p2p, stub
}

// RFC requirement: RFC5443-3-1 positive -- on a broadcast segment with two IGP/LDP peers
// (2.2.2.2 and 3.3.3.3, both Full), while LDP is not synchronized and the segment is not a
// cut-edge, the originated Router-LSA carries no transit link for the segment at all and no
// link naming either peer: the cost-out covers the link as a whole. Once synchronized it
// carries exactly one transit link, to the pseudonode, and still no per-peer link.
// RFC requirement: RFC6138-4-1 negative -- the same not-synchronized segment that is NOT a
// cut-edge (an alternate path through 2.2.2.2 exists in the SPF graph) has its transit link
// withheld, so the cut-edge answer, and not LDP alone, is what advertises a cut-edge.
func TestRFC5443BroadcastCostOutCoversWholeSegment(t *testing.T) {
	// Goal: a whole-segment cost-out on the originated LSA. Method: two peers on the segment,
	// an SPF graph with an alternate path, originate before and after synchronization.
	eng, ft := rfc6138Engine(t)
	rfc6138SPF(t, eng, true)

	transit, p2p, stub := rfc6138CountLinks(t, rfc6138OriginatedLinks(t, eng))
	if transit != 0 || p2p != 0 {
		t.Fatalf("not synchronized: transit links = %d, point-to-point links = %d, want 0 and 0 (the whole segment is costed out, no peer kept)", transit, p2p)
	}
	if stub != 1 {
		t.Fatalf("not synchronized: stub links = %d, want the one subnet stub", stub)
	}

	rfc5443Synchronize(t, eng, ft)
	transit, p2p, _ = rfc6138CountLinks(t, rfc6138OriginatedLinks(t, eng))
	if transit != 1 || p2p != 0 {
		t.Fatalf("synchronized: transit links = %d, point-to-point links = %d, want 1 and 0", transit, p2p)
	}
}

// RFC requirement: RFC6138-4-1 positive -- a not-synchronized broadcast segment with two
// Full peers that IS a cut-edge (the SPF graph holds no other path from 1.1.1.1 to the
// segment) keeps its transit link in the originated Router-LSA: LDP does not delay it.
func TestRFC6138CutEdgeTransitLinkNotDelayed(t *testing.T) {
	// Goal: the cut-edge answer from the real SPF graph reaches origination. Method: the same
	// segment without the alternate point-to-point link.
	eng, _ := rfc6138Engine(t)
	rfc6138SPF(t, eng, false)
	if state, managed := eng.ldpSync.stateFor("eth1"); !managed || state == ldpSyncSynchronized {
		t.Fatalf("eth1 state = %s (managed %v), want managed and not synchronized", ldpSyncStateName(state), managed)
	}

	transit, _, _ := rfc6138CountLinks(t, rfc6138OriginatedLinks(t, eng))
	if transit != 1 {
		t.Fatalf("cut-edge, not synchronized: transit links = %d, want 1 (a cut-edge MUST NOT wait for LDP)", transit)
	}
}

// RFC requirement: RFC5443-3-1 negative -- on the same two-peer segment, an LDP session that
// comes up (one peer's) before the hold-down completes does not bring back any part of the
// segment: the originated Router-LSA still carries no transit link and no link naming either
// peer, so no individual peer is costed in while the segment is costed out.
func TestRFC5443BroadcastOnePeerUpKeepsSegmentCostedOut(t *testing.T) {
	// Goal: no per-peer cost-in. Method: push the input toward the violation with one LDP
	// session up, then originate before the hold-down expires.
	eng, _ := rfc6138Engine(t)
	rfc6138SPF(t, eng, true)
	eng.ldpSync.onSessionUp("eth1")

	transit, p2p, stub := rfc6138CountLinks(t, rfc6138OriginatedLinks(t, eng))
	if transit != 0 || p2p != 0 {
		t.Fatalf("one LDP session up, hold-down pending: transit links = %d, point-to-point links = %d, want 0 and 0", transit, p2p)
	}
	if stub != 1 {
		t.Fatalf("stub links = %d, want the one subnet stub", stub)
	}
}
