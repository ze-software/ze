//go:build integration && linux

// Design: docs/architecture/policyroute/policy-routing.md -- the ip rule and the auto route proven here
// Overview: rules_linux.go -- applyIPRules and applyAutoRoutes, the two producers under test
//
// applyIPRules and applyAutoRoutes are the only functions in this plugin that
// write to the kernel, and reading either object back proves it exists and
// nothing more. A rule the kernel never consults reads back exactly like one it
// obeys, and a route sitting in a table no lookup reaches reads back exactly
// like a route that carries traffic.
//
// So these proofs assert the kernel's own answer. The specs come from the
// plugin's own translation of an operator policy, applyAll installs them, and
// the observable is a route lookup: with the fwmark ze marks the packet with,
// the lookup MUST resolve in the table ze selected; without the mark, the same
// destination MUST resolve somewhere else. The second half is the control,
// because a lookup that answers the same way with and without the mark says
// nothing about the rule.
//
// Each test owns a network namespace, so no route and no rule reaches the
// machine the test runs on.

package policyroute

import (
	"net"
	"net/netip"
	"runtime"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// The addresses the proofs use. All three are documentation ranges (RFC 5737),
// so a packet that escapes the namespace names nothing real.
var (
	localAddress = netip.MustParseAddr("192.0.2.1")
	nextHop      = netip.MustParseAddr("192.0.2.2")
	probe        = net.ParseIP("198.51.100.7")
)

// operatorTable is the table an operator names in a `table N` action. It is
// outside the ze-reserved range, which is what an operator-supplied table is.
const operatorTable = 100

// TestApplyIPRulesSendsAMarkedLookupToTheSelectedTable proves the rule
// applyIPRules installs changes what the kernel ANSWERS, rather than only
// appearing in the rule list.
//
// The method: the test puts a route to the probe destination in table 100 and
// nowhere else, so the table is reachable only through the rule under test. A
// lookup carrying the mark resolves there; the same lookup without the mark
// finds no route at all. Without the second half the first would also pass with
// the route installed in the main table, where every lookup would find it.
func TestApplyIPRulesSendsAMarkedLookupToTheSelectedTable(t *testing.T) {
	enterNamespace(t)
	linkIndex := buildDummyLink(t)

	mark := installPolicy(t, PolicyAction{Type: ActionTable, Table: operatorTable})
	addProbeRoute(t, linkIndex, operatorTable)

	marked := routeLookup(t, mark)
	if len(marked) != 1 {
		t.Fatalf("the marked lookup answered with %d routes, want 1", len(marked))
	}
	if marked[0].Table != operatorTable {
		t.Fatalf("the marked lookup resolved in table %d, want %d: the ip rule did not steer it", marked[0].Table, operatorTable)
	}

	// The control. The route lives only in table 100, so an unmarked lookup that
	// finds anything means the rule is not what selected the table.
	plain, err := netlink.RouteGet(probe)
	if err == nil {
		t.Fatalf("the unmarked lookup resolved to %v; only the marked lookup reaches table %d", plain, operatorTable)
	}
}

// TestApplyAutoRoutesResolvesTheNextHopInsideItsAutoTable proves the auto route
// applyAutoRoutes installs is the route a marked lookup USES, not merely a row
// in a table nobody reads.
//
// A next-hop action allocates a table in 2000-2999 and puts a default route
// through the operator's next hop in it, so both producers are on the path: the
// rule has to select the auto table and the route in it has to resolve. The
// control is the same lookup without the mark, which reaches the main table and
// its connected route only.
func TestApplyAutoRoutesResolvesTheNextHopInsideItsAutoTable(t *testing.T) {
	enterNamespace(t)
	buildDummyLink(t)

	mark := installPolicy(t, PolicyAction{Type: ActionNextHop, NextHop: nextHop})

	marked := routeLookup(t, mark)
	if len(marked) != 1 {
		t.Fatalf("the marked lookup answered with %d routes, want 1", len(marked))
	}
	if marked[0].Table < autoTableBase {
		t.Fatalf("the marked lookup resolved in table %d, want one in %d-%d: the ip rule did not steer it", marked[0].Table, autoTableBase, autoTableMax)
	}
	if marked[0].Table > autoTableMax {
		t.Fatalf("the marked lookup resolved in table %d, want one in %d-%d: the ip rule did not steer it", marked[0].Table, autoTableBase, autoTableMax)
	}
	if !marked[0].Gw.Equal(net.IP(nextHop.AsSlice())) {
		t.Fatalf("the marked lookup resolved via %v, want %v: the auto route is not the route the kernel used", marked[0].Gw, nextHop)
	}

	// The control. Nothing in the main table reaches the probe destination, so a
	// lookup that resolves without the mark did not need the auto table.
	plain, err := netlink.RouteGet(probe)
	if err == nil {
		t.Fatalf("the unmarked lookup resolved to %v; only the marked lookup reaches the auto table", plain)
	}
}

// installPolicy translates one operator policy through the plugin's own
// allocator, installs the result with applyAll, and answers with the fwmark the
// nftables term sets. Building the specs by hand would prove the netlink calls
// and leave the mark the chain really writes untested.
func installPolicy(t *testing.T, action PolicyAction) uint32 {
	t.Helper()

	policy := PolicyRoute{
		Name:       "steer",
		Interfaces: []InterfaceSpec{{Name: "zepr0"}},
		Rules:      []PolicyRule{{Name: "web", Match: PolicyMatch{DestinationPort: "80", Protocol: "tcp"}, Action: action}},
	}
	result, err := newAllocator().translate([]PolicyRoute{policy})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(result.IPRules) != 1 {
		t.Fatalf("the policy produced %d ip rules, want 1", len(result.IPRules))
	}

	manager, err := newRuleManager()
	if err != nil {
		t.Skipf("needs a netlink route handle: %v", err)
	}
	t.Cleanup(manager.close)

	if err := manager.applyAll(result); err != nil {
		t.Fatalf("applyAll: %v", err)
	}
	t.Cleanup(func() { manager.removeAll(result) })

	return result.IPRules[0].Mark
}

// routeLookup asks the kernel where a packet carrying the policy's fwmark would
// go. RTM_F_LOOKUP_TABLE is set by the netlink helper, so the answer names the
// table the rule chain selected.
func routeLookup(t *testing.T, mark uint32) []netlink.Route {
	t.Helper()

	routes, err := netlink.RouteGetWithOptions(probe, &netlink.RouteGetOptions{Mark: mark})
	if err != nil {
		t.Fatalf("route lookup for %v with mark 0x%x: %v", probe, mark, err)
	}
	return routes
}

// addProbeRoute puts the probe destination in one table and no other, so the
// lookup can only answer through the rule under test.
func addProbeRoute(t *testing.T, linkIndex, table int) {
	t.Helper()

	_, destination, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("parse the probe prefix: %v", err)
	}
	route := &netlink.Route{
		LinkIndex: linkIndex,
		Dst:       destination,
		Table:     table,
		Protocol:  rtproto.PolicyRoute,
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route add %v table %d: %v", destination, table, err)
	}
	t.Cleanup(func() {
		if err := netlink.RouteDel(route); err != nil {
			t.Logf("route del %v table %d: %v", destination, table, err)
		}
	})
}

// buildDummyLink gives the namespace one addressed, up interface. The connected
// route it creates is what lets the next hop resolve, and its index is where the
// probe route points.
func buildDummyLink(t *testing.T) int {
	t.Helper()

	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "zepr0"}} //nolint:modernize // netlink.Dummy embeds LinkAttrs, and naming it is how the field is set
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("needs CAP_NET_ADMIN to create a dummy interface: %v", err)
	}
	created, err := netlink.LinkByName("zepr0")
	if err != nil {
		t.Fatalf("dummy interface zepr0: %v", err)
	}
	address := &netlink.Addr{IPNet: &net.IPNet{IP: net.IP(localAddress.AsSlice()), Mask: net.CIDRMask(24, 32)}}
	if err := netlink.AddrAdd(created, address); err != nil {
		t.Fatalf("address %v on zepr0: %v", localAddress, err)
	}
	if err := netlink.LinkSetUp(created); err != nil {
		t.Fatalf("link up zepr0: %v", err)
	}
	return created.Attrs().Index
}

// enterNamespace moves the test onto a network namespace of its own, and back
// when it ends. A namespace belongs to a THREAD, so the thread is locked for
// the whole test: a goroutine that migrated would program the host.
//
// It SKIPS rather than fails when the namespace is refused. This file runs
// unprivileged in some environments and privileged in the QEMU and container
// runs, and a missing capability is not a broken product.
func enterNamespace(t *testing.T) {
	t.Helper()

	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("needs a network namespace of its own: %v", err)
	}
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		origin.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
		t.Skipf("needs CAP_NET_ADMIN to unshare a network namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := netns.Set(origin); err != nil {
			t.Errorf("cannot return to the original network namespace: %v", err)
		}
		origin.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})
}
