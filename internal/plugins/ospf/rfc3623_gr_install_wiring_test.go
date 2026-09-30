// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- planned restart keeps the FIB.
// Related: spf_wiring.go -- initSPF, which wires the graceful-stop state into the SPF installer.
// Related: rfc3623_prepare_fib_test.go -- the pending-run half of the same requirement.
//
// VALIDATES: RFC 3623 Section 2.1, "Router X must ensure that its forwarding table(s) is/are
// up-to-date and will remain in place across the restart", on the SPF computer newEngine builds
// through initSPF (nothing rebuilt by the test): once the restart is prepared, an SPF run whose
// result no longer holds a route leaves that route in the Loc-RIB.
// PREVENTS: initSPF building the computer without the graceful-stop install suppression, so the
// first SPF run after prepare (an LSA aged out or re-originated during the restart) withdraws
// routes from the forwarding table the restart is meant to keep.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc3623WiredRoute is the stub prefix this file advertises; no other test uses it, so its
// presence in the process-wide Loc-RIB is this file's own doing.
var rfc3623WiredRoute = netip.MustParsePrefix("203.0.113.128/26")

// TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes drives the production engine: the
// neighbor 10.0.0.2 reaches Full on eth0 (so the engine's own next-hop resolver answers), SPF
// installs the neighbor's stub prefix, the operator prepares the restart, then the neighbor's
// Router-LSA is re-originated without the stub and SPF runs again.
func TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes(t *testing.T) {
	loc := locrib.Default()
	if loc == nil {
		t.Fatal("no process-wide Loc-RIB in this test process")
	}
	installed := func() bool {
		_, ok := loc.Best(family.IPv4Unicast, rfc3623WiredRoute)
		return ok
	}
	t.Cleanup(func() { rfc3623ClearPrefix(t, rfc3623WiredRoute) })

	e := grPrepareEngine(t, time.Unix(1_000_000, 0))
	self := types.RouterID{10, 0, 0, 1}
	peer := types.RouterID{10, 0, 0, 2}
	e.neighbors.ConfigureInterface(ospfneighbor.InterfaceConfig{
		Name: bfdTestIface, AreaID: types.BackboneArea, RouterID: self,
		NetworkType: types.NetworkPointToPoint, Options: types.OptionE, DeadInterval: 40, InterfaceMTU: 1500,
	})
	driveNeighborFull(t, e, peer, netip.MustParseAddr("10.0.0.2"))
	// The configuration applyConfig pushes; the install suppression is initSPF's own wiring.
	e.spf.SetTimers(time.Hour, time.Hour, time.Hour)
	e.spf.SetRoot(self)
	e.spf.SetAreas([]types.AreaID{types.BackboneArea})

	toPeer := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(peer),
		LinkData: [4]byte{10, 0, 0, 1}, Metric: 10}
	toSelf := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(self),
		LinkData: [4]byte{10, 0, 0, 2}, Metric: 10}
	stub := packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{203, 0, 113, 128},
		LinkData: [4]byte{255, 255, 255, 192}, Metric: 5}
	if !e.lsdb.Install(types.BackboneArea, bgplsReachabilityRouter(self, toPeer)) {
		t.Fatal("install the self router-LSA")
	}
	if !e.lsdb.Install(types.BackboneArea, bgplsReachabilityRouter(peer, toSelf, stub)) {
		t.Fatal("install the peer router-LSA")
	}
	e.spf.Run()
	if !installed() {
		t.Fatalf("%s: SPF did not install the neighbor's stub before the restart; the setup is void", rfc3623WiredRoute)
	}

	if err := e.gr.prepareRestart(grReasonReload); err != nil {
		t.Fatalf("prepareRestart: %v", err)
	}
	withdrawn := bgplsReachabilityRouter(peer, toSelf)
	withdrawn.Header.Sequence++
	if !e.lsdb.Install(types.BackboneArea, withdrawn) {
		t.Fatal("install the peer router-LSA without the stub")
	}
	e.spf.Run()

	// RFC requirement: RFC3623-2.1-1 positive -- once the restart is prepared, the forwarding
	// table remains in place: an SPF run on the engine's own computer (built by initSPF) whose
	// result no longer holds the neighbor's stub leaves that route in the Loc-RIB.
	if !installed() {
		t.Fatalf("%s: an SPF run after the prepared restart withdrew a route from the retained forwarding table", rfc3623WiredRoute)
	}
}

// rfc3623ClearPrefix withdraws every path of prefix from the process-wide Loc-RIB, so a repeated
// run (-count=N) starts from an empty state.
func rfc3623ClearPrefix(t *testing.T, prefix netip.Prefix) {
	t.Helper()
	loc := locrib.Default()
	group, ok := loc.Lookup(family.IPv4Unicast, prefix)
	if !ok {
		return
	}
	for index := range group.Paths {
		path := &group.Paths[index]
		loc.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance)
	}
}
