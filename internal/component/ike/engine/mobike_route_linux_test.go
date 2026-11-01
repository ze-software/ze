//go:build linux

// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- authenticated endpoint mobility
// Related: mobike_route_linux.go -- routeSourceTo
// Related: rfc4301_sad_selector_linux_test.go -- sadSelOwnNamespace, the private namespace
// Related: unmatched.go -- installUnmatched, the catch-all this lookup must not cross
//
// VALIDATES: the source address MOBIKE moves to (RFC 4555 Section 3.5) is found while
// Ze's SPD catch-all DISCARD is in force, which is its default (RFC 4301 Section
// 4.4.1, `vpn ipsec unmatched`).
// PREVENTS: an original initiator that never sends UPDATE_SA_ADDRESSES after its
// address changes. A route lookup made by connecting a UDP socket from an ephemeral
// port is an output route lookup, and the kernel runs it through the SPD: no IKE
// bypass names that port, the catch-all matches it, and connect returns EPERM.
// Measured in evidence-nightly runs 40478425007 to 41298258262 (mobike-initiator):
// strongSwan's IKE endpoint stayed at the old address for the whole wait.

package engine

import (
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	mobikeRouteMoved = "172.28.0.8"
	mobikeRoutePeer  = "172.28.0.3"
)

// mobikeRouteNetns gives the namespace one interface holding the moved address and a
// host route to the peer whose preferred source is that address: the state the
// mobike-initiator lab leaves after it moves Ze (mobikeMove).
func mobikeRouteNetns(t *testing.T) {
	t.Helper()
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "mobike0"}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("the namespace refused a dummy interface: %v", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("mobike0 up: %v", err)
	}
	address, err := netlink.ParseAddr(mobikeRouteMoved + "/32")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.AddrAdd(link, address); err != nil {
		t.Fatalf("add %s: %v", mobikeRouteMoved, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.ParseIP(mobikeRoutePeer).To4(), Mask: net.CIDRMask(32, 32)},
		Src:       net.ParseIP(mobikeRouteMoved),
		Scope:     netlink.SCOPE_LINK,
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route to %s: %v", mobikeRoutePeer, err)
	}
}

func TestMOBIKERouteSourceUnderTheCatchAllDiscard(t *testing.T) {
	if !sadSelOwnNamespace(t) {
		return
	}
	mobikeRouteNetns(t)
	peer := net.ParseIP(mobikeRoutePeer)

	// Without any SPD entry the lookup already answers: the baseline.
	source, err := routeSourceTo(peer)
	if err != nil {
		t.Fatalf("route source with no SPD: %v", err)
	}
	if !source.Equal(net.ParseIP(mobikeRouteMoved)) {
		t.Fatalf("route source with no SPD = %s, want %s", source, mobikeRouteMoved)
	}

	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close xfrm backend: %v", err)
		}
	})
	dp := dataplane.Get()
	log := slogutil.DiscardLogger()
	if err := installUnmatched(dp, dataplane.SPActionDiscard, log); err != nil {
		t.Fatalf("install the catch-all: %v", err)
	}
	t.Cleanup(func() { removeUnmatched(dp, log) })
	installIKEBypass(dp, log)

	// Positive control: the catch-all is in force for a socket no bypass names, so
	// the assertion below discriminates a lookup that crosses the SPD from one that
	// does not.
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: peer, Port: 4500})
	if err == nil {
		_ = conn.Close()
		t.Fatal("a UDP connect from an ephemeral port crossed the catch-all DISCARD; the probe cannot discriminate")
	}
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("UDP connect under the catch-all returned %v, want EPERM", err)
	}

	source, err = routeSourceTo(peer)
	if err != nil {
		t.Fatalf("route source under the catch-all DISCARD: %v", err)
	}
	if !source.Equal(net.ParseIP(mobikeRouteMoved)) {
		t.Fatalf("route source under the catch-all DISCARD = %s, want %s", source, mobikeRouteMoved)
	}
}
