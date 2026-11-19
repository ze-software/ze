// Design: docs/architecture/l2tp/subscriber-session-model.md -- framed routes through the session interface
// Related: framed_route.go -- the installer under test

package ppp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

const framedRouteTestInterface = "framed-route-session"

// framedRouteLeaseStart is frameLeaseStart with the data plane recorder
// returned, so a test can read the routes the real session installed.
func framedRouteLeaseStart(t *testing.T, sid uint16, backend *fakeBackend) (StartSession, net.Conn) {
	t.Helper()
	control, peer := net.Pipe()
	t.Cleanup(func() { closeConn(peer) })
	ops, _, _ := newFakeOps()
	handle := newTestSessionHandle(backend, framedRouteTestInterface)
	handle.ops = ops
	handle.owned = []io.Closer{control}
	handle.readCounters = func() (SessionCounters, error) {
		return SessionCounters{RxBytes: 13, TxBytes: 27, RxPackets: 3, TxPackets: 5}, nil
	}
	owner := &sessionLeaseCounter{SessionHandle: handle}
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := NewSessionAttempt(birth.String(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return StartSession{
		LCP: testLCPConfig(), TunnelID: 1, SessionID: sid,
		L2TPBirth: birth, SubscriberID: birth.String(), Attempt: attempt,
		Frames: &testSessionFrames{control: control}, Dataplane: owner,
		LNSMode: true, DisableIPCP: false, DisableIPv6CP: true,
		AuthMethod: AuthMethodCHAPMD5, AuthRequired: true,
	}, peer
}

// framedRouteProfile is one Access-Accept's routes: one naming the subscriber
// through the 0.0.0.0 gateway, one with an explicit gateway, one IPv6.
var framedRouteProfile = []l2tpevents.FramedRoute{
	{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Gateway: netip.IPv4Unspecified(), Metric: 11},
	{Prefix: netip.MustParsePrefix("203.0.113.0/25"), Gateway: netip.MustParseAddr("192.0.2.77")},
	{Prefix: netip.MustParsePrefix("2001:db8:12::/48"), Gateway: netip.IPv6Unspecified(), Metric: 5},
}

// framedRouteWant is what the session must install for framedRouteProfile:
// the IPv4 routes through its own interface, 0.0.0.0 replaced by the address
// IPCP assigned to the peer, the explicit gateway that is not the peer
// installed as a device route (no gateway, rendered as the invalid Addr).
var framedRouteWant = []routeCall{
	{framedRouteTestInterface, "198.51.100.0/24", ipcpTestPeer.String(), 11, rtproto.Iface},
	{framedRouteTestInterface, "203.0.113.0/25", netip.Addr{}.String(), 0, rtproto.Iface},
}

// Goal: an L2TP session installs each RADIUS Framed-Route through its own
// interface once that interface is up, before session-up, and withdraws it
// when the network lifetime ends. Linux refuses a route through an interface
// that is down (ENETDOWN), so a route added before admin-up fails the session.
// Method: a real Driver runs LCP, CHAP and IPCP against a recording data plane
// that refuses a route on an interface not yet up; the profile is staged the
// way the RADIUS provider stages it, and the recorder is read at the
// assignment, at session-up and at LCP-down.
func TestFramedRouteInstalledThroughL2TPSessionInterface(t *testing.T) {
	backend := &fakeBackend{requireAdminUp: true}
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer := framedRouteLeaseStart(t, 31, backend)
	t.Cleanup(func() { closeConn(peer); driver.Stop() })
	driver.SessionsIn() <- start
	openLifecycleLCP(t, peer)
	request := readFramedRouteAuthRequest(t, driver, peer)
	if err := request.StageFramedRoutes(framedRouteProfile); err != nil {
		t.Fatal(err)
	}
	assignment := awaitFramedRouteAssignment(t, driver, peer, request)
	if got := backend.RouteAddCalls(); len(got) != 0 {
		t.Fatalf("routes added before the interface is up = %+v, want none", got)
	}
	if !slices.Equal(assignment.FramedRoutes, framedRouteProfile) {
		t.Errorf("assignment routes = %+v, want the accepted profile", assignment.FramedRoutes)
	}
	assignment.Acknowledge()
	awaitLifecycleUp(t, driver.EventsOut())
	if got := backend.UpCalls(); len(got) == 0 {
		t.Fatal("session-up without bringing the interface up")
	}
	if got := backend.RouteAddCalls(); !slices.Equal(got, framedRouteWant) {
		t.Fatalf("routes installed at session-up = %+v, want %+v", got, framedRouteWant)
	}
	if removed := backend.RouteRemoveCalls(); len(removed) != 0 {
		t.Fatalf("routes withdrawn while the network is live = %+v", removed)
	}

	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, 0x42, nil)
	down, ok := waitForEvent(t, driver.EventsOut(), time.Second).(EventLCPDown)
	if !ok || !down.NetworkPhase {
		t.Fatalf("LCP-down = %+v, want the end of the network lifetime", down)
	}
	want := []routeCall{framedRouteWant[1], framedRouteWant[0]}
	if got := backend.RouteRemoveCalls(); !slices.Equal(got, want) {
		t.Fatalf("routes withdrawn at LCP-down = %+v, want newest first %+v", got, want)
	}
	down.Acknowledge()
}

// Goal: a route the data plane refuses fails the session instead of serving
// a subset of the authorized routes, and the uncertain add is still withdrawn.
// Method: the recorder refuses every route add on an interface that is up;
// after the assignment the session must publish session-down naming the
// Framed-Route, never session-up, and withdraw the route.
func TestFramedRouteRefusedFailsTheSession(t *testing.T) {
	backend := &fakeBackend{requireAdminUp: true, addRouteErr: errors.New("route refused by the data plane")}
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer := framedRouteLeaseStart(t, 32, backend)
	t.Cleanup(func() { closeConn(peer); driver.Stop() })
	driver.SessionsIn() <- start
	openLifecycleLCP(t, peer)
	request := readFramedRouteAuthRequest(t, driver, peer)
	if err := request.StageFramedRoutes(framedRouteProfile[:1]); err != nil {
		t.Fatal(err)
	}
	if err := driver.AuthResponse(request, true, "", nil); err != nil {
		t.Fatal(err)
	}
	readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeSuccess)
	select {
	case event := <-driver.AuthEventsOut():
		if _, ok := event.(eventAuthSuccess); !ok {
			t.Fatalf("accepted authentication = %T, want success", event)
		}
	case <-time.After(time.Second):
		t.Fatal("the accepted decision was not consumed")
	}
	select {
	case event := <-driver.IPEventsOut():
		ipRequest, ok := event.(EventIPRequest)
		if !ok {
			t.Fatalf("IP provider event = %T", event)
		}
		if err := driver.IPResponse(ipRequest, IPResponseArgs{Accept: true, Family: AddressFamilyIPv4, Local: ipcpTestLocal, Peer: ipcpTestPeer}); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("IPCP did not request an address")
	}
	exchange := &ncpTestDriver{driver: driver, peer: peer}
	exchange.completeIPCP(t)
	assigned, ok := waitForEvent(t, driver.EventsOut(), time.Second).(EventSessionIPAssigned)
	if !ok {
		t.Fatal("IPCP did not publish an assignment")
	}
	assigned.Acknowledge()
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	down, ok := event.(EventSessionDown)
	if !ok {
		t.Fatalf("event after a refused framed route = %T, want session-down and no session-up", event)
	}
	if !strings.Contains(down.Reason, "Framed-Route") {
		t.Errorf("session-down reason = %q, want it to name the Framed-Route", down.Reason)
	}
	closeConn(peer)
	driver.Stop()
	if got := backend.RouteRemoveCalls(); !slices.Equal(got, framedRouteWant[:1]) {
		t.Fatalf("withdrawals after the uncertain add = %+v, want %+v", got, framedRouteWant[:1])
	}
}

// Goal: an accepted profile over the framed-route limit fails the session
// before any route reaches the data plane, rather than installing a truncated
// set. Method: the setup of TestFramedRouteRefusedFailsTheSession with one
// route past l2tpevents.FramedRoutesMax staged; the session must publish
// session-down naming the limit, and the recorder must see no route add.
func TestFramedRouteOverLimitFailsTheSession(t *testing.T) {
	backend := &fakeBackend{}
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer := framedRouteLeaseStart(t, 33, backend)
	t.Cleanup(func() { closeConn(peer); driver.Stop() })
	driver.SessionsIn() <- start
	openLifecycleLCP(t, peer)
	request := readFramedRouteAuthRequest(t, driver, peer)
	routes := make([]l2tpevents.FramedRoute, 0, l2tpevents.FramedRoutesMax+1)
	for index := range l2tpevents.FramedRoutesMax + 1 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 51, 100, byte(index)}), 32)
		routes = append(routes, l2tpevents.FramedRoute{Prefix: prefix, Gateway: netip.IPv4Unspecified()})
	}
	if err := request.StageFramedRoutes(routes); err != nil {
		t.Fatal(err)
	}
	if err := driver.AuthResponse(request, true, "", nil); err != nil {
		t.Fatal(err)
	}
	readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeSuccess)
	select {
	case event := <-driver.AuthEventsOut():
		if _, ok := event.(eventAuthSuccess); !ok {
			t.Fatalf("accepted authentication = %T, want success", event)
		}
	case <-time.After(time.Second):
		t.Fatal("the accepted decision was not consumed")
	}
	select {
	case event := <-driver.IPEventsOut():
		ipRequest, ok := event.(EventIPRequest)
		if !ok {
			t.Fatalf("IP provider event = %T", event)
		}
		if err := driver.IPResponse(ipRequest, IPResponseArgs{Accept: true, Family: AddressFamilyIPv4, Local: ipcpTestLocal, Peer: ipcpTestPeer}); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("IPCP did not request an address")
	}
	exchange := &ncpTestDriver{driver: driver, peer: peer}
	exchange.completeIPCP(t)
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	down, ok := event.(EventSessionDown)
	if !ok {
		t.Fatalf("event after an over-limit profile = %T, want session-down and no assignment", event)
	}
	if !strings.Contains(down.Reason, "more than 64 framed routes") {
		t.Errorf("session-down reason = %q, want it to name the limit", down.Reason)
	}
	if got := backend.RouteAddCalls(); len(got) != 0 {
		t.Fatalf("routes added for an over-limit profile = %+v, want none", got)
	}
}

// Goal: the IPv6 service installs each Framed-IPv6-Route through the session
// interface with the :: gateway replaced by the peer's link-local address,
// installs an explicit gateway that is not the peer as a device route,
// installs a duplicate once, and withdraws them
// newest first before the link-local pair. Method: the real session start
// boundary over a recording data plane, mirroring the delegated-route test.
func TestIPv6FramedRouteInstalledThroughPeerLinkLocal(t *testing.T) {
	session, _, events := newRFC1661Session(LCPStateOpened)
	handle := testHandleFor(session)
	backend, ok := handle.backend.(*fakeBackend)
	if !ok {
		t.Fatalf("session backend = %T, want *fakeBackend", handle.backend)
	}
	carrier := newServiceCarrier()
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) { return carrier, nil }
	session.localInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 1}
	session.peerInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 2}
	session.peerInterfaceIDNegotiated = true
	session.authenticatedFramedRoutes = []l2tpevents.FramedRoute{
		framedRouteProfile[0],
		{Prefix: netip.MustParsePrefix("2001:db8:12::/48"), Gateway: netip.IPv6Unspecified(), Metric: 5},
		{Prefix: netip.MustParsePrefix("2001:db8:34::/48"), Gateway: netip.MustParseAddr("2001:db8::7")},
		{Prefix: netip.MustParsePrefix("2001:db8:12::/48"), Gateway: netip.IPv6Unspecified(), Metric: 5},
	}
	if err := session.afterLCPOpenIPv6Service(); err != nil {
		t.Fatal(err)
	}
	if session.ipv6Svc == nil {
		t.Fatal("the IPv6 service did not start")
	}
	name := handle.Identity().InterfaceName
	want := []routeCall{
		{name, "2001:db8:12::/48", "fe80::200:0:0:2", 5, rtproto.Iface},
		{name, "2001:db8:34::/48", netip.Addr{}.String(), 0, rtproto.Iface},
	}
	if got := backend.RouteAddCalls(); !slices.Equal(got, want) {
		t.Fatalf("IPv6 framed routes installed = %+v, want %+v", got, want)
	}
	if len(events) != 0 {
		t.Fatal("a successful IPv6 framed-route install published a session event")
	}
	if err := session.stopIPv6Service(); err != nil {
		t.Fatal(err)
	}
	if got := backend.RouteRemoveCalls(); !slices.Equal(got, []routeCall{want[1], want[0]}) {
		t.Fatalf("IPv6 framed routes withdrawn = %+v, want newest first", got)
	}
	if got := backend.AddrRemoveCalls(); len(got) != 1 {
		t.Fatalf("link-local withdrawals = %+v, want one", got)
	}
}

// Goal: a Framed-IPv6-Route the data plane refuses fails the session, unlike
// every other IPv6 service failure, and the uncertain add is withdrawn.
// Method: the recorder refuses route adds; afterLCPOpenIPv6Service must return
// errFramedRouteRefused, start no service and withdraw the attempted route.
func TestIPv6FramedRouteRefusedFailsTheSession(t *testing.T) {
	session, _, _ := newRFC1661Session(LCPStateOpened)
	handle := testHandleFor(session)
	backend, ok := handle.backend.(*fakeBackend)
	if !ok {
		t.Fatalf("session backend = %T, want *fakeBackend", handle.backend)
	}
	backend.addRouteErr = errors.New("route refused by the data plane")
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		t.Error("carrier acquired after a refused framed route")
		return nil, io.ErrClosedPipe
	}
	session.localInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 1}
	session.peerInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 2}
	session.peerInterfaceIDNegotiated = true
	session.authenticatedFramedRoutes = framedRouteProfile[2:]
	err := session.afterLCPOpenIPv6Service()
	if !errors.Is(err, errFramedRouteRefused) || !strings.Contains(err.Error(), "Framed-IPv6-Route") {
		t.Fatalf("refused IPv6 framed route = %v, want errFramedRouteRefused naming the Framed-IPv6-Route", err)
	}
	if session.ipv6Svc != nil {
		t.Fatal("a completed rollback retained the IPv6 service")
	}
	if got := backend.RouteRemoveCalls(); len(got) != 1 || got[0].dest != "2001:db8:12::/48" {
		t.Fatalf("withdrawals after the uncertain add = %+v, want the attempted route", got)
	}
	if got := backend.AddrRemoveCalls(); len(got) != 1 {
		t.Fatalf("link-local withdrawals = %+v, want one", got)
	}
}

// Goal: a failed Framed-IPv6-Route removal keeps the link-local pair the
// route resolves through, for the retry. Method: the recorder refuses route
// removal; Stop must fail without removing the address, then succeed.
func TestIPv6FramedRouteRemovalFailureKeepsLinkLocal(t *testing.T) {
	backend := &fakeBackend{}
	handle := newTestSessionHandle(backend, "ppp0")
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) { return newServiceCarrier(), nil }
	cfg := serviceHandleConfig(handle)
	cfg.FramedRoutes = framedRouteProfile[2:]
	service, err := startIPv6Service(cfg, testServerID(), nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	backend.removeRouteErr = errors.New("route removal failed")
	if err := service.Stop(); err == nil {
		t.Fatal("a failed framed-route removal was reported as a clean stop")
	}
	if got := backend.AddrRemoveCalls(); len(got) != 0 {
		t.Fatalf("link-local removed under a retained framed route: %+v", got)
	}
	backend.removeRouteErr = nil
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if got := backend.AddrRemoveCalls(); len(got) != 1 {
		t.Fatalf("retry link-local withdrawals = %+v, want one", got)
	}
	if len(service.framedRoutes) != 0 {
		t.Fatalf("retry retained framed-route receipts: %+v", service.framedRoutes)
	}
}

// Goal: the 0.0.0.0 gateway names the subscriber, an explicit gateway equal
// to the subscriber is kept, any other explicit gateway becomes a device
// route (the invalid Addr), and a route with no gateway or no subscriber
// address is refused.
// Method: table over newFramedRouteReceipt, the installer's resolver.
func TestFramedRouteReceiptResolvesTheGateway(t *testing.T) {
	subscriber := netip.MustParseAddr("192.0.2.41")
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	for _, tt := range []struct {
		name       string
		route      l2tpevents.FramedRoute
		subscriber netip.Addr
		gateway    netip.Addr
		ok         bool
	}{
		{"unspecified names the subscriber", l2tpevents.FramedRoute{Prefix: prefix, Gateway: netip.IPv4Unspecified()}, subscriber, subscriber, true},
		{"explicit gateway equal to the subscriber kept", l2tpevents.FramedRoute{Prefix: prefix, Gateway: subscriber}, subscriber, subscriber, true},
		{"explicit non-subscriber gateway becomes a device route", l2tpevents.FramedRoute{Prefix: prefix, Gateway: netip.MustParseAddr("192.0.2.77")}, subscriber, netip.Addr{}, true},
		{"explicit gateway without a subscriber address becomes a device route", l2tpevents.FramedRoute{Prefix: prefix, Gateway: netip.MustParseAddr("192.0.2.77")}, netip.Addr{}, netip.Addr{}, true},
		{"no gateway refused", l2tpevents.FramedRoute{Prefix: prefix}, subscriber, netip.Addr{}, false},
		{"subscriber without address refused", l2tpevents.FramedRoute{Prefix: prefix, Gateway: netip.IPv4Unspecified()}, netip.Addr{}, netip.Addr{}, false},
		{"ipv6 gateway refused", l2tpevents.FramedRoute{Prefix: prefix, Gateway: netip.IPv6Unspecified()}, subscriber, netip.Addr{}, false},
		{"ipv6 prefix with an ipv4 subscriber refused", l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("2001:db8::/32"), Gateway: netip.IPv6Unspecified()}, subscriber, netip.Addr{}, false},
		{"ipv6 unspecified names the peer link-local", l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("2001:db8::/32"), Gateway: netip.IPv6Unspecified()}, netip.MustParseAddr("fe80::2"), netip.MustParseAddr("fe80::2"), true},
		{"ipv6 explicit gateway equal to the peer link-local kept", l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("2001:db8::/32"), Gateway: netip.MustParseAddr("fe80::2")}, netip.MustParseAddr("fe80::2"), netip.MustParseAddr("fe80::2"), true},
		{"ipv6 explicit non-peer gateway becomes a device route", l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("2001:db8::/32"), Gateway: netip.MustParseAddr("2001:db8::7")}, netip.MustParseAddr("fe80::2"), netip.Addr{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			receipt, err := newFramedRouteReceipt(tt.route, tt.subscriber)
			if tt.ok != (err == nil) {
				t.Fatalf("error = %v, want ok %v", err, tt.ok)
			}
			if receipt.gateway != tt.gateway {
				t.Errorf("gateway = %v, want %v", receipt.gateway, tt.gateway)
			}
		})
	}
}
