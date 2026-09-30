// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- planned restart keeps the FIB.
// Related: gr_restarter.go -- prepareRestart, which runs SPF before raising the graceful stop.
// Related: spf/install.go -- Installer.Apply / RemoveAll, both gated by the graceful-stop state.
//
// VALIDATES: RFC 3623 Section 2.1, "Router X must ensure that its forwarding table(s) is/are
// up-to-date and will remain in place across the restart." A route whose SPF run is still
// pending (back-off timer armed) when the operator prepares the restart reaches the Loc-RIB
// before the graceful stop, and the ensuing engine stop leaves it there.
// PREVENTS: prepareRestart raising the graceful stop while an SPF result is still pending, so
// the pending run later computes the route but its install is suppressed and the FIB kept
// across the restart is stale.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc3623PendingRoute is the stub prefix the tests advertise; it is used by no other test, so
// its presence in the process-wide Loc-RIB is this file's own doing.
var rfc3623PendingRoute = netip.MustParsePrefix("203.0.113.64/26")

// rfc3623NextHop resolves the neighbor's point-to-point address to eth0. The engine's own
// resolver answers only for a neighbor its table holds in Full, which this unit does not bring
// up, so the rebuilt SPF computer takes this one instead.
type rfc3623NextHop struct{}

func (rfc3623NextHop) ResolveInterface(addr netip.Addr) (string, bool) {
	if addr == netip.MustParseAddr("10.0.0.2") {
		return "eth0", true
	}
	return "", false
}

// rfc3623PendingEngine builds a GR-enabled OSPFv2 engine whose SPF computer is wired as initSPF
// wires it (the engine LSDB as source, an Installer on the process-wide Loc-RIB, and the
// graceful-restart install gate), with a one-hour back-off. It then installs a point-to-point
// pair of router-LSAs: 10.0.0.1 reaches 10.0.0.2, which carries rfc3623PendingRoute as a stub.
// The LSDB change arms the SPF timer, so the route's run is pending and nothing is installed.
func rfc3623PendingEngine(t *testing.T) *engine {
	t.Helper()
	t.Cleanup(func() { rfc3623ClearRoute(t) })
	e := grPrepareEngine(t, time.Unix(1_000_000, 0))
	self := types.RouterID{10, 0, 0, 1}
	peer := types.RouterID{10, 0, 0, 2}
	e.spf = ospfspf.NewComputer(ospfspf.Config{
		Source:    e.lsdb,
		Resolver:  rfc3623NextHop{},
		Installer: ospfspf.NewInstallerFamily(locrib.Default(), family.IPv4Unicast),
	})
	e.spf.SetInstallSuppress(e.gr.suppressInstall)
	e.spf.SetTimers(time.Hour, time.Hour, time.Hour)
	e.spf.SetRoot(self)
	e.spf.SetAreas([]types.AreaID{types.BackboneArea})
	e.lsdb.SetOnChange(e.triggerSPF)
	toPeer := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(peer),
		LinkData: [4]byte{10, 0, 0, 1}, Metric: 10}
	toSelf := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(self),
		LinkData: [4]byte{10, 0, 0, 2}, Metric: 10}
	stub := packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{203, 0, 113, 64},
		LinkData: [4]byte{255, 255, 255, 192}, Metric: 5}
	if !e.lsdb.Install(types.BackboneArea, bgplsReachabilityRouter(self, toPeer)) {
		t.Fatal("install the self router-LSA")
	}
	if !e.lsdb.Install(types.BackboneArea, bgplsReachabilityRouter(peer, toSelf, stub)) {
		t.Fatal("install the peer router-LSA")
	}
	if e.spf.Routes() != nil {
		t.Fatalf("SPF ran before prepare (routes %v); the pending-run setup is void", e.spf.Routes())
	}
	if rfc3623RouteInstalled(t) {
		t.Fatalf("%s is in the Loc-RIB before any SPF ran; the pending-run setup is void", rfc3623PendingRoute)
	}
	return e
}

// rfc3623RouteInstalled reports whether the OSPF route for rfc3623PendingRoute is in the
// process-wide Loc-RIB the engine's installer writes to.
func rfc3623RouteInstalled(t *testing.T) bool {
	t.Helper()
	loc := locrib.Default()
	if loc == nil {
		t.Fatal("no process-wide Loc-RIB in this test process")
	}
	_, ok := loc.Best(family.IPv4Unicast, rfc3623PendingRoute)
	return ok
}

// TestRFC3623PrepareInstallsPendingSPFAndKeepsIt drives the planned-restart path: the route's
// SPF run is pending when prepareRestart runs, and the engine stop follows. The route MUST be
// in the Loc-RIB after prepare (up to date) and still there after the stop (kept in place).
func TestRFC3623PrepareInstallsPendingSPFAndKeepsIt(t *testing.T) {
	e := rfc3623PendingEngine(t)

	if err := e.gr.prepareRestart(grReasonReload); err != nil {
		t.Fatalf("prepareRestart: %v", err)
	}

	// RFC requirement: RFC3623-2.1-1 positive -- a route whose SPF run is still pending when the
	// restart is prepared is in the Loc-RIB once prepareRestart returns (the forwarding table is
	// up to date), and the engine's SPF stop, whose RemoveAll the graceful stop suppresses, leaves
	// it there (the forwarding table remains in place across the restart).
	if !rfc3623RouteInstalled(t) {
		t.Fatalf("%s: the SPF run pending at prepare time was not installed before the graceful stop", rfc3623PendingRoute)
	}
	e.spf.Stop()
	if !rfc3623RouteInstalled(t) {
		t.Fatalf("%s: the engine stop after a prepared restart withdrew the retained route", rfc3623PendingRoute)
	}
}

// rfc3623ClearRoute withdraws every path of rfc3623PendingRoute from the process-wide Loc-RIB,
// so a repeated run (-count=N) starts from the empty state rfc3623PendingEngine checks.
func rfc3623ClearRoute(t *testing.T) {
	t.Helper()
	loc := locrib.Default()
	group, ok := loc.Lookup(family.IPv4Unicast, rfc3623PendingRoute)
	if !ok {
		return
	}
	for index := range group.Paths {
		path := &group.Paths[index]
		loc.Remove(family.IPv4Unicast, rfc3623PendingRoute, path.Source, path.Instance)
	}
}
