package ppp

import (
	"net/netip"
	"sync/atomic"
	"testing"
)

func newTestIPv6Service() (*IPv6Service, *fakeBackend) {
	fb := &fakeBackend{}
	svc := newIPv6Service(iPv6ServiceConfig{
		Ifname:          "ppp0",
		TunnelID:        1,
		SessionID:       2,
		PeerInterfaceID: [8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11},
		Backend:         fb,
	})
	return svc, fb
}

func newTestIPv6ServiceWithRelease(released *atomic.Value) *IPv6Service {
	return newIPv6Service(iPv6ServiceConfig{
		Ifname:          "ppp0",
		TunnelID:        1,
		SessionID:       2,
		PeerInterfaceID: [8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11},
		Backend:         &fakeBackend{},
		ReleasePrefix: func(p netip.Prefix) {
			released.Store(p)
		},
	})
}

func testAllocator(prefix netip.Prefix) func() (netip.Prefix, bool) {
	called := false
	return func() (netip.Prefix, bool) {
		if called {
			return netip.Prefix{}, false
		}
		called = true
		return prefix, true
	}
}

func testExhaustedAllocator() func() (netip.Prefix, bool) {
	return func() (netip.Prefix, bool) {
		return netip.Prefix{}, false
	}
}

func TestHandleDHCPv6SolicitAllocatesPrefix(t *testing.T) {
	svc, _ := newTestIPv6Service()
	srv := testServerID()
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	solicit := buildSolicit([3]byte{0x12, 0x34, 0x56}, clientDUID, 1)
	msg, err := parseDHCPv6(solicit)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := svc.handleDHCPv6(msg, srv, testAllocator(prefix))
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	reply, err := parseDHCPv6(resp)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Type != DHCPv6Advertise {
		t.Errorf("type = %d, want %d (Advertise)", reply.Type, DHCPv6Advertise)
	}
	if reply.IAPD == nil || reply.IAPD.Prefix == nil {
		t.Fatal("missing IA_PD/prefix in Advertise")
	}
	if reply.IAPD.Prefix.Prefix != prefix {
		t.Errorf("prefix = %s, want %s", reply.IAPD.Prefix.Prefix, prefix)
	}
}

func TestHandleDHCPv6SolicitNilAllocator(t *testing.T) {
	svc, _ := newTestIPv6Service()
	srv := testServerID()

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	solicit := buildSolicit([3]byte{0x12, 0x34, 0x56}, clientDUID, 1)
	msg, err := parseDHCPv6(solicit)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := svc.handleDHCPv6(msg, srv, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected NoPrefixAvail response")
	}

	reply, err := parseDHCPv6(resp)
	if err != nil {
		t.Fatal(err)
	}
	if reply.StatusCode == nil || *reply.StatusCode != D6StatusNoPrefixAvail {
		t.Error("expected NoPrefixAvail status code")
	}
}

func TestHandleDHCPv6RequestInstallsRoute(t *testing.T) {
	svc, fb := newTestIPv6Service()
	srv := testServerID()
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}

	solicit := buildSolicit([3]byte{0x12, 0x34, 0x56}, clientDUID, 1)
	msg, _ := parseDHCPv6(solicit)
	if _, err := svc.handleDHCPv6(msg, srv, testAllocator(prefix)); err != nil {
		t.Fatal(err)
	}

	req := buildRenew([3]byte{0x12, 0x34, 0x57}, clientDUID, srv, 1)
	req[0] = DHCPv6Request
	msg, _ = parseDHCPv6(req)

	resp, err := svc.handleDHCPv6(msg, srv, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	reply, _ := parseDHCPv6(resp)
	if reply.Type != DHCPv6Reply {
		t.Errorf("type = %d, want %d (Reply)", reply.Type, DHCPv6Reply)
	}

	routes := fb.RouteAddCalls()
	if len(routes) != 1 {
		t.Fatalf("route adds = %d, want 1", len(routes))
	}
	if routes[0].dest != "2001:db8:abcd::/48" {
		t.Errorf("route dest = %s, want 2001:db8:abcd::/48", routes[0].dest)
	}
	wantGW := "fe80::aabb:ccdd:eeff:11"
	if routes[0].gateway != wantGW {
		t.Errorf("route gw = %s, want %s", routes[0].gateway, wantGW)
	}
}

func TestHandleDHCPv6RequestWrongServerID(t *testing.T) {
	svc, _ := newTestIPv6Service()
	srv := testServerID()
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}

	solicit := buildSolicit([3]byte{0x12, 0x34, 0x56}, clientDUID, 1)
	msg, _ := parseDHCPv6(solicit)
	if _, err := svc.handleDHCPv6(msg, srv, testAllocator(prefix)); err != nil {
		t.Fatal(err)
	}

	wrongServer := DHCPv6DUID{Type: DUIDTypeEN, EnterpriseNum: 99999, ID: []byte{0xff}}
	req := buildRenew([3]byte{0x12, 0x34, 0x57}, clientDUID, wrongServer, 1)
	req[0] = DHCPv6Request
	msg, _ = parseDHCPv6(req)

	resp, err := svc.handleDHCPv6(msg, srv, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil {
		t.Error("expected nil response for wrong ServerID")
	}
}

func TestHandleDHCPv6ReleaseRemovesRoute(t *testing.T) {
	svc, fb := newTestIPv6Service()
	srv := testServerID()
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}

	solicit := buildSolicit([3]byte{0x01, 0x02, 0x03}, clientDUID, 1)
	msg, _ := parseDHCPv6(solicit)
	if _, err := svc.handleDHCPv6(msg, srv, testAllocator(prefix)); err != nil {
		t.Fatal(err)
	}

	reqBuf := buildRenew([3]byte{0x01, 0x02, 0x04}, clientDUID, srv, 1)
	reqBuf[0] = DHCPv6Request
	msg, _ = parseDHCPv6(reqBuf)
	if _, err := svc.handleDHCPv6(msg, srv, nil); err != nil {
		t.Fatal(err)
	}

	relBuf := buildRelease([3]byte{0x01, 0x02, 0x05}, clientDUID, srv, 1)
	msg, _ = parseDHCPv6(relBuf)
	resp, err := svc.handleDHCPv6(msg, srv, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	removes := fb.RouteRemoveCalls()
	if len(removes) != 1 {
		t.Fatalf("route removes = %d, want 1", len(removes))
	}
	if removes[0].dest != "2001:db8:abcd::/48" {
		t.Errorf("removed route dest = %s", removes[0].dest)
	}
}

func TestHandleDHCPv6ReleaseCallsReleasePrefix(t *testing.T) {
	var released atomic.Value
	svc := newTestIPv6ServiceWithRelease(&released)
	srv := testServerID()
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}

	solicit := buildSolicit([3]byte{0x01, 0x02, 0x03}, clientDUID, 1)
	msg, _ := parseDHCPv6(solicit)
	if _, err := svc.handleDHCPv6(msg, srv, testAllocator(prefix)); err != nil {
		t.Fatal(err)
	}

	reqBuf := buildRenew([3]byte{0x01, 0x02, 0x04}, clientDUID, srv, 1)
	reqBuf[0] = DHCPv6Request
	msg, _ = parseDHCPv6(reqBuf)
	if _, err := svc.handleDHCPv6(msg, srv, nil); err != nil {
		t.Fatal(err)
	}

	relBuf := buildRelease([3]byte{0x01, 0x02, 0x05}, clientDUID, srv, 1)
	msg, _ = parseDHCPv6(relBuf)
	if _, err := svc.handleDHCPv6(msg, srv, nil); err != nil {
		t.Fatal(err)
	}

	got, ok := released.Load().(netip.Prefix)
	if !ok {
		t.Fatal("ReleasePrefix not called")
	}
	if got != prefix {
		t.Errorf("released prefix = %s, want %s", got, prefix)
	}
}

func TestHandleDHCPv6PoolExhaustedReturnsNoPrefixAvail(t *testing.T) {
	svc, _ := newTestIPv6Service()
	srv := testServerID()

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	solicit := buildSolicit([3]byte{0xab, 0xcd, 0xef}, clientDUID, 1)
	msg, _ := parseDHCPv6(solicit)

	resp, err := svc.handleDHCPv6(msg, srv, testExhaustedAllocator())
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected NoPrefixAvail response")
	}

	reply, _ := parseDHCPv6(resp)
	if reply.Type != DHCPv6Reply {
		t.Errorf("type = %d, want %d", reply.Type, DHCPv6Reply)
	}
	if reply.StatusCode == nil || *reply.StatusCode != D6StatusNoPrefixAvail {
		t.Error("expected NoPrefixAvail status code")
	}
}

func TestHandleDHCPv6StopCleansUpRoute(t *testing.T) {
	svc, fb := newTestIPv6Service()
	srv := testServerID()
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}

	solicit := buildSolicit([3]byte{0x01, 0x02, 0x03}, clientDUID, 1)
	msg, _ := parseDHCPv6(solicit)
	if _, err := svc.handleDHCPv6(msg, srv, testAllocator(prefix)); err != nil {
		t.Fatal(err)
	}

	reqBuf := buildRenew([3]byte{0x01, 0x02, 0x04}, clientDUID, srv, 1)
	reqBuf[0] = DHCPv6Request
	msg, _ = parseDHCPv6(reqBuf)
	if _, err := svc.handleDHCPv6(msg, srv, nil); err != nil {
		t.Fatal(err)
	}

	svc.Stop()

	removes := fb.RouteRemoveCalls()
	if len(removes) != 1 {
		t.Fatalf("Stop should remove route, got %d removes", len(removes))
	}
}

func TestHandleDHCPv6StopReleasesPrefixToPool(t *testing.T) {
	var released atomic.Value
	svc := newTestIPv6ServiceWithRelease(&released)
	srv := testServerID()
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}

	solicit := buildSolicit([3]byte{0x01, 0x02, 0x03}, clientDUID, 1)
	msg, _ := parseDHCPv6(solicit)
	if _, err := svc.handleDHCPv6(msg, srv, testAllocator(prefix)); err != nil {
		t.Fatal(err)
	}

	svc.Stop()

	got, ok := released.Load().(netip.Prefix)
	if !ok {
		t.Fatal("Stop did not call ReleasePrefix")
	}
	if got != prefix {
		t.Errorf("released prefix = %s, want %s", got, prefix)
	}
}

func TestHandleDHCPv6LifetimeDefaults(t *testing.T) {
	svc := newIPv6Service(iPv6ServiceConfig{
		Ifname:          "ppp0",
		PeerInterfaceID: [8]byte{},
		Backend:         &fakeBackend{},
	})
	if svc.lifetimes.T1 != 302400 {
		t.Errorf("T1 = %d, want 302400", svc.lifetimes.T1)
	}
	if svc.lifetimes.ValidLifetime != 2592000 {
		t.Errorf("ValidLifetime = %d, want 2592000", svc.lifetimes.ValidLifetime)
	}

	svc2 := newIPv6Service(iPv6ServiceConfig{
		Ifname:          "ppp0",
		PeerInterfaceID: [8]byte{},
		Backend:         &fakeBackend{},
		Lifetimes:       DHCPv6Lifetimes{T1: 100, T2: 200, PrefLifetime: 300, ValidLifetime: 400},
	})
	if svc2.lifetimes.T1 != 100 {
		t.Errorf("custom T1 = %d, want 100", svc2.lifetimes.T1)
	}
}

func TestPeerLinkLocal(t *testing.T) {
	id := [8]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}
	got := peerLinkLocal(id)
	want := netip.MustParseAddr("fe80::11:2233:4455:6677")
	if got != want {
		t.Errorf("peerLinkLocal = %s, want %s", got, want)
	}
}

func TestDUIDEqual(t *testing.T) {
	a := &DHCPv6DUID{Type: DUIDTypeEN, EnterpriseNum: 12345, ID: []byte{0x01, 0x02}}
	b := &DHCPv6DUID{Type: DUIDTypeEN, EnterpriseNum: 12345, ID: []byte{0x01, 0x02}}
	c := &DHCPv6DUID{Type: DUIDTypeEN, EnterpriseNum: 99999, ID: []byte{0x01, 0x02}}

	if !duidEqual(a, b) {
		t.Error("identical DUIDs should be equal")
	}
	if duidEqual(a, c) {
		t.Error("different DUIDs should not be equal")
	}
	if duidEqual(a, nil) {
		t.Error("non-nil and nil should not be equal")
	}
	if !duidEqual(nil, nil) {
		t.Error("two nils should be equal")
	}
}
