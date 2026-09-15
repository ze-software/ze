//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the readiness check this component owns
// Detail: doctor.go -- checkKernelState, driven through the registry the doctor runner reads
//
// Every case stands in the kernel through the same vars the reconcile writes
// through (routeList, routeResolve) and the table reader the check owns
// (readKernelTables), so no case touches a kernel and each one names the
// state it pretends the kernel is in.

package gtsm

import (
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// kernelStandIn is one pretended kernel: the routes it lists, the route it
// resolves, and the ze-owned tables it holds.
type kernelStandIn struct {
	routes     []netlink.Route
	resolveErr error
	tables     []firewall.Table
	tablesErr  error
	tableReads int
}

func standInKernel(t *testing.T, kernel *kernelStandIn) {
	t.Helper()
	list, resolve, read := routeList, routeResolve, readKernelTables
	routeList = func(int) ([]netlink.Route, error) { return kernel.routes, nil }
	routeResolve = func(net.IP) ([]netlink.Route, error) {
		if kernel.resolveErr != nil {
			return nil, kernel.resolveErr
		}
		return []netlink.Route{{LinkIndex: 2}}, nil
	}
	readKernelTables = func() ([]firewall.Table, error) {
		kernel.tableReads++
		return kernel.tables, kernel.tablesErr
	}
	t.Cleanup(func() { routeList, routeResolve, readKernelTables = list, resolve, read })
}

// installedRoute is the host route SetPeers installs for a peer, as the
// kernel lists it back.
func installedRoute(p Peer, hopLimit int) netlink.Route {
	route := hostRoute(p.Addr)
	route.Hoplimit = hopLimit
	route.Protocol = netlink.RouteProtocol(rtproto.GTSM)
	return route
}

// publishedInThisProcess puts the check in its in-daemon mode: SetPeers ran
// here and asked the kernel for these peers.
func publishedInThisProcess(t *testing.T, peers ...Peer) {
	t.Helper()
	captureSeams(t)
	current = sortedPeers(peers)
	published = true
}

func ipv6Peer() Peer {
	return Peer{Addr: netip.MustParseAddr("2001:db8::1"), Port: 179, HopLimit: 255, Floor: 255}
}

// TestGTSMDoctorReportsAPublishedPeerWhoseRouteIsMissing is the case the
// daemon's own log line describes: the peer was published, the route install
// failed, and the kernel holds the table but not the route.
//
// VALIDATES: one warning naming the peer and the host route, and not the
// table the kernel does hold.
// PREVENTS: a GTSM peer whose ICMP errors leave at the default TTL with
// nothing but a log line to say so.
func TestGTSMDoctorReportsAPublishedPeerWhoseRouteIsMissing(t *testing.T) {
	p := gtsmPeer()
	publishedInThisProcess(t, p)
	standInKernel(t, &kernelStandIn{tables: filterTables([]Peer{p})})

	diags := registeredKernelStateCheck(t).Check(diagnostic.DoctorCheckContext{Tree: bgpTree()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %+v, want 1", diags)
	}
	if diags[0].Code != doctorKernelStateCode || diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("diagnostic = %+v, want a %s warning", diags[0], doctorKernelStateCode)
	}
	if diags[0].Path != p.Addr.String() {
		t.Fatalf("path = %q, want the peer address", diags[0].Path)
	}
	if !strings.Contains(diags[0].Message, "host route") || strings.Contains(diags[0].Message, filterTableName) {
		t.Fatalf("message = %q, want the route named and the table not", diags[0].Message)
	}
}

// TestGTSMDoctorReportsAPublishedPeerWhoseTableIsMissing is the other half:
// the route is there with its metric, and the kernel holds no ze_gtsm table.
//
// VALIDATES: one warning naming the table and not the route.
func TestGTSMDoctorReportsAPublishedPeerWhoseTableIsMissing(t *testing.T) {
	p := gtsmPeer()
	publishedInThisProcess(t, p)
	standInKernel(t, &kernelStandIn{routes: []netlink.Route{installedRoute(p, 255)}})

	diags := registeredKernelStateCheck(t).Check(diagnostic.DoctorCheckContext{Tree: bgpTree()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %+v, want 1", diags)
	}
	if !strings.Contains(diags[0].Message, filterTableName) || strings.Contains(diags[0].Message, "host route") {
		t.Fatalf("message = %q, want the table named and the route not", diags[0].Message)
	}
}

// TestGTSMDoctorIsSilentWhenTheKernelHoldsWhatWasPublished is the negative
// half, and it holds the two readings apart: a route carrying another hop
// limit is not the route SetPeers asked for.
//
// VALIDATES: no diagnostic when both halves are present; one when the route
// carries the wrong metric.
func TestGTSMDoctorIsSilentWhenTheKernelHoldsWhatWasPublished(t *testing.T) {
	p := gtsmPeer()
	publishedInThisProcess(t, p)
	kernel := &kernelStandIn{routes: []netlink.Route{installedRoute(p, 255)}, tables: filterTables([]Peer{p})}
	standInKernel(t, kernel)

	check := registeredKernelStateCheck(t)
	if diags := check.Check(diagnostic.DoctorCheckContext{Tree: bgpTree()}); len(diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diags)
	}

	kernel.routes = []netlink.Route{installedRoute(p, 64)}
	diags := check.Check(diagnostic.DoctorCheckContext{Tree: bgpTree()})
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "hop limit 255") {
		t.Fatalf("diagnostics = %+v, want one naming hop limit 255", diags)
	}
}

// TestGTSMDoctorNeverReadsTheFirewallForAnIPv6OnlySet keeps the check's
// footprint the same as SetPeers': an IPv6 peer owes no table, so the
// firewall backend is not asked for one.
//
// VALIDATES: the table reader is not called, and a present route is silent.
// PREVENTS: a "no firewall backend" warning on a box whose GTSM peers never
// loaded one.
func TestGTSMDoctorNeverReadsTheFirewallForAnIPv6OnlySet(t *testing.T) {
	p := ipv6Peer()
	publishedInThisProcess(t, p)
	kernel := &kernelStandIn{routes: []netlink.Route{installedRoute(p, 255)}, tablesErr: errors.New("must not be read")}
	standInKernel(t, kernel)

	diags := registeredKernelStateCheck(t).Check(diagnostic.DoctorCheckContext{Tree: bgpTree()})
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diags)
	}
	if kernel.tableReads != 0 {
		t.Fatalf("the firewall was read %d times for a peer that owes no table", kernel.tableReads)
	}
}

// TestGTSMDoctorReportsAnUnreadableFirewallOnce: a table read that fails is
// one warning, and no peer is then also reported as missing a table nothing
// was learned about.
//
// VALIDATES: one diagnostic for the read, none for the peer whose route is
// present.
func TestGTSMDoctorReportsAnUnreadableFirewallOnce(t *testing.T) {
	p := gtsmPeer()
	publishedInThisProcess(t, p)
	standInKernel(t, &kernelStandIn{routes: []netlink.Route{installedRoute(p, 255)}, tablesErr: errors.New("netlink refused")})

	diags := registeredKernelStateCheck(t).Check(diagnostic.DoctorCheckContext{Tree: bgpTree()})
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "netlink refused") {
		t.Fatalf("diagnostics = %+v, want one naming the read failure", diags)
	}
}

// offlineWithConfiguredPeers puts the check in its `ze doctor` mode: nothing
// was published here, and the configuration derives these peers. The BGP
// resolver seam is stood in because this package's test binary carries no
// BGP engine; the real derivation is tested beside the reactor
// (TestGTSMPeersFromResolvedTreeReadsTheConfigAlone).
func offlineWithConfiguredPeers(t *testing.T, peers ...Peer) {
	t.Helper()
	captureSeams(t)
	previous := configPeers
	configPeers = func(map[string]any) ([]Peer, error) { return peers, nil }
	infra.SetBGPTreeResolver(func(*config.Tree) (map[string]any, error) { return map[string]any{}, nil })
	t.Cleanup(func() {
		configPeers = previous
		infra.SetBGPTreeResolver(nil)
	})
}

// TestGTSMDoctorReportsAConfiguredPeerTheKernelCannotRoute is the offline
// answer: before a start, the kernel state is legitimately absent, and the
// one thing that can be known is whether the route install will have a
// nexthop to copy.
//
// VALIDATES: a peer the kernel resolves no route to is one warning; a peer it
// does resolve is silent, whatever state is installed.
// PREVENTS: a warning about absent kernel state on every box before every
// start.
func TestGTSMDoctorReportsAConfiguredPeerTheKernelCannotRoute(t *testing.T) {
	p := gtsmPeer()
	offlineWithConfiguredPeers(t, p)
	kernel := &kernelStandIn{resolveErr: errors.New("network is unreachable")}
	standInKernel(t, kernel)

	check := registeredKernelStateCheck(t)
	diags := check.Check(diagnostic.DoctorCheckContext{Tree: bgpTree()})
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "network is unreachable") {
		t.Fatalf("diagnostics = %+v, want one naming the unresolved route", diags)
	}
	if diags[0].Path != p.Addr.String() {
		t.Fatalf("path = %q, want the peer address", diags[0].Path)
	}

	kernel.resolveErr = nil
	if diags := check.Check(diagnostic.DoctorCheckContext{Tree: bgpTree()}); len(diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none while the kernel resolves the peer", diags)
	}
	if kernel.tableReads != 0 {
		t.Fatalf("the firewall was read %d times before a start", kernel.tableReads)
	}
}
