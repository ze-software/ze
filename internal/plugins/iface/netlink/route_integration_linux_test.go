//go:build integration && linux

// Integration proof for spec-fixit-route-removal-protocol-blind. A zero
// Protocol in RTM_DELROUTE is a kernel WILDCARD, so only a running kernel can
// show that a stamped delete refuses a foreign route and a blind delete still
// takes one. Every assertion here reads the kernel back.

package ifacenetlink

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
)

const (
	routeTestLink   = "zert0"
	routeTestLocal  = "192.0.2.1/24"
	routeTestGW     = "192.0.2.2"
	routeTestDest   = "0.0.0.0/0"
	routeTestMetric = 5
	// deprioritized metric the link-down handler uses in
	// internal/component/iface/register.go (base + 1024).
	routeTestDownMetric = routeTestMetric + 1024
	routeTestDest6      = "::/0"
	routeTestGW6        = "fe80::1"
)

// withRouteNetNS runs fn inside a fresh named network namespace so route
// installs cannot touch the host table. Skips (not fails) without
// CAP_NET_ADMIN per ai/rules/platform-linux.md.
func withRouteNetNS(t *testing.T, fn func()) {
	t.Helper()

	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}

	nsName := routeNetNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort; test is stopping either way
		unlock()
		// Only a privilege problem is a skip. Anything else -- EEXIST from a
		// namespace an earlier run left behind, a missing /var/run/netns -- is a
		// broken environment, and skipping it would delete an acceptance
		// criterion from the run with a green bar on top.
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("requires CAP_NET_ADMIN: cannot create namespace %q: %v", nsName, err)
		}
		t.Fatalf("create namespace %q: %v", nsName, err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort cleanup
		newNS.Close()             //nolint:errcheck // best-effort cleanup
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})

	fn()
}

// routeNetNSName derives the netns name for one test. It MUST be unique per
// test: netns.NewNamed refuses a name that already exists, and two tests
// deriving one name make the second depend on the first having cleaned up.
// Keeping the last 8 characters was not unique -- StampedRemoveLeavesStaticRoute
// and LinkBounceKeepsStaticRoute both ended in "ticRoute" -- so the full name is
// hashed and the readable tail is kept only for whoever reads /var/run/netns.
func routeNetNSName(testName string) string {
	sum := sha256.Sum256([]byte(testName))
	tail := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(tail) > 8 {
		tail = tail[len(tail)-8:]
	}
	return "zert_" + tail + "_" + hex.EncodeToString(sum[:8])
}

// addRouteTestLink creates the dummy device the routes point at and gives it an
// address in the gateway's subnet, so the kernel can resolve the next hop.
func addRouteTestLink(t *testing.T) {
	t.Helper()

	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: routeTestLink}}); err != nil {
		t.Fatalf("add dummy %q: %v", routeTestLink, err)
	}
	link, err := netlink.LinkByName(routeTestLink)
	if err != nil {
		t.Fatalf("link %q: %v", routeTestLink, err)
	}
	addr, err := netlink.ParseAddr(routeTestLocal)
	if err != nil {
		t.Fatalf("parse %q: %v", routeTestLocal, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("add address %q: %v", routeTestLocal, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set %q up: %v", routeTestLink, err)
	}
}

// addForeignRoute installs a route at routeTestMetric through raw netlink with
// an explicit protocol, standing in for a producer that is not the interface
// layer: the static plugin (rtproto.Static), or the kernel itself (RTPROT_RA).
// The metric is the one the interface layer also uses, because a delete matches
// on it and the whole hazard is two producers sharing that key.
func addForeignRoute(t *testing.T, destCIDR, gateway string, proto int) {
	metric := routeTestMetric
	t.Helper()

	dst, err := netlink.ParseIPNet(destCIDR)
	if err != nil {
		t.Fatalf("parse dest %q: %v", destCIDR, err)
	}
	link, err := netlink.LinkByName(routeTestLink)
	if err != nil {
		t.Fatalf("link %q: %v", routeTestLink, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        net.ParseIP(gateway),
		Priority:  metric,
		Protocol:  netlink.RouteProtocol(proto),
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("add route %s via %s metric %d proto %d: %v", destCIDR, gateway, metric, proto, err)
	}
}

// kernelRoute returns the route matching destination, gateway and metric, or
// nil when the kernel holds none. The protocol is deliberately not part of the
// lookup: a test asserting survival must see the route whatever stamp it wears.
func kernelRoute(t *testing.T, destCIDR, gateway string, metric int) *netlink.Route {
	t.Helper()

	family := netlink.FAMILY_V4
	if strings.Contains(destCIDR, ":") {
		family = netlink.FAMILY_V6
	}
	routes, err := netlink.RouteList(nil, family)
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	want, err := netlink.ParseIPNet(destCIDR)
	if err != nil {
		t.Fatalf("parse dest %q: %v", destCIDR, err)
	}
	gw := net.ParseIP(gateway)
	for i := range routes {
		r := &routes[i]
		if r.Dst == nil || r.Dst.String() != want.String() {
			continue
		}
		if !r.Gw.Equal(gw) || r.Priority != metric {
			continue
		}
		return r
	}
	return nil
}

// captureIfaceLog redirects the package logger into a buffer for the duration
// of the test, so an assertion can read the attributes of a record. It reads
// what the WARN SAYS, and says nothing about where a production WARN goes: a
// logger a test installs is wiring the product does not have, which is how a
// package logging into slogutil.DiscardLogger passed this file once.
// TestRemoveRouteMissWarnsThroughTheProductionLogger (manage_linux_test.go)
// owns that half and needs no kernel.
func captureIfaceLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	previous := logger
	captured := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = previous })
	return buf
}

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-1 -- a DHCP lease
// expiry removes the route the interface layer installed and leaves an
// operator's static default route with the same destination, gateway,
// interface and metric in place.
// PREVENTS: the protocol-blind RTM_DELROUTE, which matches on the four-tuple
// alone and deletes the static route whoever installed it. Remove the Protocol
// field from (*netlinkBackend).RemoveRoute and this test fails.
func TestRouteIntegration_StampedRemoveLeavesStaticRoute(t *testing.T) {
	withRouteNetNS(t, func() {
		addRouteTestLink(t)
		addForeignRoute(t, routeTestDest, routeTestGW, rtproto.Static)

		b := &netlinkBackend{}
		if err := b.RemoveRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("RemoveRoute: %v", err)
		}

		got := kernelRoute(t, routeTestDest, routeTestGW, routeTestMetric)
		if got == nil {
			t.Fatal("static default route was deleted by an interface-layer remove")
		}
		if got.Protocol != netlink.RouteProtocol(rtproto.Static) {
			t.Fatalf("static route protocol = %d, want %d", got.Protocol, rtproto.Static)
		}
	})
}

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-4 -- the stamp does not
// stop the interface layer removing a route it installed itself, and AddRoute
// puts the interface layer's protocol on the wire.
// PREVENTS: a delete that matches nothing because add and remove disagree on
// the protocol, which would leak a route on every lease expiry.
func TestRouteIntegration_AddStampsAndRemoveDeletesOwnRoute(t *testing.T) {
	withRouteNetNS(t, func() {
		addRouteTestLink(t)

		b := &netlinkBackend{}
		if err := b.AddRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("AddRoute: %v", err)
		}
		installed := kernelRoute(t, routeTestDest, routeTestGW, routeTestMetric)
		if installed == nil {
			t.Fatal("AddRoute installed no route")
		}
		if installed.Protocol != netlink.RouteProtocol(rtproto.Iface) {
			t.Fatalf("installed route protocol = %d, want %d", installed.Protocol, rtproto.Iface)
		}

		logs := captureIfaceLog(t)
		if err := b.RemoveRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("RemoveRoute: %v", err)
		}
		if got := kernelRoute(t, routeTestDest, routeTestGW, routeTestMetric); got != nil {
			t.Fatalf("route survived its owner's remove: protocol %d", got.Protocol)
		}
		if logs.Len() != 0 {
			t.Fatalf("removing an owned route warned: %s", logs.String())
		}
	})
}

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-3 -- a caller that names
// rtproto.Any still removes a route the kernel installed. This is the match
// cleanupStaleIPv6DefaultRoutes needs (internal/component/iface/register.go):
// it clears ::/0 routes the kernel built from Router Advertisements, which
// carry RTPROT_RA and were never installed by Ze.
// PREVENTS: an unconditional stamp, which would make that cleanup delete
// nothing while ESRCH reports success.
func TestRouteIntegration_BlindRemoveDeletesKernelRoute(t *testing.T) {
	withRouteNetNS(t, func() {
		addRouteTestLink(t)
		addForeignRoute(t, routeTestDest6, routeTestGW6, unix.RTPROT_RA)

		b := &netlinkBackend{}
		if err := b.RemoveRoute(routeTestLink, routeTestDest6, routeTestGW6, routeTestMetric, rtproto.Any); err != nil {
			t.Fatalf("RemoveRoute: %v", err)
		}
		if got := kernelRoute(t, routeTestDest6, routeTestGW6, routeTestMetric); got != nil {
			t.Fatalf("kernel RA default route survived a blind remove: protocol %d", got.Protocol)
		}
	})
}

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-5 -- a route an earlier
// Ze version installed carries RTPROT_BOOT, so a stamped delete cannot match
// it. The delete reports success, and the orphan it left behind is named in a
// WARN with its destination, gateway and interface.
// PREVENTS: swallowing ESRCH into nil with no trace, which hides a route Ze
// believes it removed and the kernel still holds.
func TestRouteIntegration_LegacyBootRouteIsReported(t *testing.T) {
	withRouteNetNS(t, func() {
		addRouteTestLink(t)
		addForeignRoute(t, routeTestDest, routeTestGW, unix.RTPROT_BOOT)

		logs := captureIfaceLog(t)
		b := &netlinkBackend{}
		if err := b.RemoveRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("RemoveRoute: %v", err)
		}

		if got := kernelRoute(t, routeTestDest, routeTestGW, routeTestMetric); got == nil {
			t.Fatal("legacy route was deleted: the delete was not stamped")
		}
		out := logs.String()
		if !strings.Contains(out, "level=WARN") {
			t.Fatalf("no WARN for the orphaned route: %q", out)
		}
		// held-by is read off the kernel, not assumed: the WARN fires because
		// reportRemoveRouteMiss found the surviving route and its protocol.
		for _, want := range []string{routeTestLink, routeTestDest, routeTestGW, "held-by=boot"} {
			if !strings.Contains(out, want) {
				t.Fatalf("WARN does not name %q: %s", want, out)
			}
		}
	})
}

// VALIDATES: spec-fixit-route-removal-protocol-blind, Critical Review row "the
// ESRCH warning fires for a stamped delete and stays silent for a blind one, so
// a double-remove on the teardown path does not shout". A route that is simply
// gone is not an orphan, and only the kernel can say which of the two an ESRCH
// was: this asserts what reportRemoveRouteMiss found when it asked.
// PREVENTS: a WARN on a remove of a route another path already took away. That
// case carries no orphan and no action for an operator, and a WARN they see on
// healthy teardown is a WARN they stop reading.
func TestRouteIntegration_RepeatedRemoveDoesNotWarn(t *testing.T) {
	withRouteNetNS(t, func() {
		addRouteTestLink(t)

		b := &netlinkBackend{}
		if err := b.AddRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("AddRoute: %v", err)
		}
		if err := b.RemoveRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("first RemoveRoute: %v", err)
		}

		logs := captureIfaceLog(t)
		if err := b.RemoveRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("repeated RemoveRoute: %v", err)
		}
		if logs.Len() != 0 {
			t.Fatalf("removing a route that is already gone warned: %s", logs.String())
		}
	})
}

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-2 -- a link bounce runs
// remove-then-add twice (handleLinkDown deprioritizes, handleLinkUp restores),
// and the operator's static default route is still routed through the same
// gateway at the same metric at the end of it.
// PREVENTS: the deprioritize step deleting the static route, which is the
// second way the blind match lost an operator's default route.
func TestRouteIntegration_LinkBounceKeepsStaticRoute(t *testing.T) {
	withRouteNetNS(t, func() {
		addRouteTestLink(t)
		addForeignRoute(t, routeTestDest, routeTestGW, rtproto.Static)

		b := &netlinkBackend{}
		// handleLinkDown: drop the base-metric route, install a deprioritized one.
		if err := b.RemoveRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("link-down RemoveRoute: %v", err)
		}
		down := kernelRoute(t, routeTestDest, routeTestGW, routeTestMetric)
		if down == nil {
			t.Fatal("link-down handling deleted the static route")
		}
		if down.Protocol != netlink.RouteProtocol(rtproto.Static) {
			t.Fatalf("static route protocol after link down = %d, want %d", down.Protocol, rtproto.Static)
		}
		if err := b.AddRoute(routeTestLink, routeTestDest, routeTestGW, routeTestDownMetric, rtproto.Iface); err != nil {
			t.Fatalf("link-down AddRoute: %v", err)
		}

		// handleLinkUp: drop the deprioritized route, restore the base metric.
		if err := b.RemoveRoute(routeTestLink, routeTestDest, routeTestGW, routeTestDownMetric, rtproto.Iface); err != nil {
			t.Fatalf("link-up RemoveRoute: %v", err)
		}
		if got := kernelRoute(t, routeTestDest, routeTestGW, routeTestDownMetric); got != nil {
			t.Fatalf("deprioritized route survived its owner's remove: protocol %d", got.Protocol)
		}
		if err := b.AddRoute(routeTestLink, routeTestDest, routeTestGW, routeTestMetric, rtproto.Iface); err != nil {
			t.Fatalf("link-up AddRoute: %v", err)
		}

		// The destination is still reachable through the operator's gateway at
		// the operator's metric. The protocol that route now carries is NOT
		// asserted: RouteReplace matches on destination, metric and table and
		// cannot be constrained by protocol, so the restore re-stamps a route
		// that shares that key. That is the add side, recorded in
		// plan/journal/guard-added-to-one-half-of-a-pair.md.
		if got := kernelRoute(t, routeTestDest, routeTestGW, routeTestMetric); got == nil {
			t.Fatal("the link bounce left no default route through the static gateway")
		}
	})
}
