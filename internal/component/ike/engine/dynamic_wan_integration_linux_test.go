// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the addresses a connect cycle measured
// Related: cycle_addr.go -- resolveCyclePair, the route-chosen local address
// Related: mobike_route_linux.go -- routeSourceTo, the lookup that must not cross the SPD
// Related: mobike_route_linux_test.go -- mobikeRouteNetns, the namespace this test reuses
// Related: unmatched.go -- installUnmatched, the default catch-all DISCARD
//
// VALIDATES: AC-21 against a real Linux XFRM stack. A route-chosen peer (no `vpn ipsec
// interface`, no local-address) starts its connect cycle while Ze's default SPD
// catch-all DISCARD is installed: the cycle measures the source the routing table
// picks toward the peer, and the IKE_SA_INIT it builds hashes NAT_DETECTION_SOURCE_IP
// over that source (RFC 7296 Section 2.23).
// PREVENTS: a route-chosen mode that finds its source by connecting a UDP socket. The
// connect is an output route lookup the kernel runs through the SPD, no IKE bypass names
// the ephemeral port, and under the catch-all it fails with EPERM, so every cycle of a
// route-chosen peer would fail before it sent anything.
//
// userns.Enter gives the test its own user and network namespace, so the catch-all and
// the dummy interface touch nothing on the host.

//go:build linux

package engine

import (
	"bytes"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/test/userns"
)

func TestRouteChosenSourceUnderCatchAllDiscard(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	mobikeRouteNetns(t)
	peerIP := net.ParseIP(mobikeRoutePeer)

	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load the xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the xfrm backend: %v", err)
		}
	})
	dp := dataplane.Get()
	log := slogutil.DiscardLogger()
	if err := installUnmatched(dp, dataplane.SPActionDiscard, log); err != nil {
		t.Fatalf("install the catch-all: %v", err)
	}
	t.Cleanup(func() { removeUnmatched(dp, log) })
	installIKEBypass(dp, log)

	// Positive control: the catch-all refuses a socket no bypass names, so the cycle
	// below discriminates a lookup that crosses the SPD from one that does not.
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: peerIP, Port: transport.IKEPort})
	if err == nil {
		_ = conn.Close()
		t.Fatal("a UDP connect from an ephemeral port crossed the catch-all DISCARD; the test cannot discriminate")
	}
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("UDP connect under the catch-all returned %v, want EPERM", err)
	}

	// The production lookup, not a stub: routeSourceFn is routeSourceTo here.
	peer := testPeer()
	peer.RemoteAddress = mobikeRoutePeer
	peer.LocalAddress = ""
	pair, err := resolveCyclePair(peer.Name, peer)
	if err != nil {
		t.Fatalf("the connect cycle of a route-chosen peer failed under the catch-all: %v", err)
	}
	if !pair.local.Equal(net.ParseIP(mobikeRouteMoved)) {
		t.Fatalf("the cycle sends from %s, want the route's source %s", pair.local, mobikeRouteMoved)
	}

	sa, err := newInitiatorSA(peer.Name, peer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	sa.cycleRemote = pair.remote.IP
	sa.cycleLocal = pair.local
	msg, err := encodeSAInitRequest(sa, testIKEGroup())
	if err != nil {
		t.Fatalf("encodeSAInitRequest: %v", err)
	}
	source, _ := natDetectionOf(t, msg)
	want := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, net.ParseIP(mobikeRouteMoved), transport.IKEPort)
	if !bytes.Equal(source, want) {
		t.Fatalf("NAT_DETECTION_SOURCE_IP %x, want the hash over the route's source %s", source, mobikeRouteMoved)
	}
}
