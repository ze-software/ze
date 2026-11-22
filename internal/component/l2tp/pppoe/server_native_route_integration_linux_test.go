//go:build integration && linux

// Design: docs/architecture/l2tp/subscriber-session-model.md -- framed routes through the session interface
// Related: server_native_ipv6_integration_linux_test.go -- the admitted-session fixture and the scripted subscriber
// Related: ../ppp/kernel/session.go -- AddRoute and RemoveRoute keep and withdraw the exact owned route intent

package pppoe

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	serverV6PAPUser     = "zv6subscriber"
	serverV6PAPPassword = "zv6secret"
	// serverV6OperatorMetric keeps the operator's route out of the framed
	// route's priority group: Linux refuses a second route of one destination
	// and one metric with EEXIST.
	serverV6OperatorMetric = 2000
	// serverV6TerminateID is the subscriber's IPv6CP Terminate-Request
	// identifier. A fixed value keeps the test goroutine off the wire
	// worker's identifier counter.
	serverV6TerminateID = 0x77
)

var serverV6FramedPrefix = netip.MustParsePrefix("2001:db8:77::/48")

// TestServerNativeIPv6FramedRouteOwnedByOriginalUnit admits a PAP subscriber
// whose accepted profile carries a Framed-IPv6-Route, gives the session's
// pppN name to a dummy that holds an operator route to the same prefix, then
// withdraws the framed route by closing IPv6CP and releases the session by
// PADT.
//
// VALIDATES: the kernel session installs the Framed-IPv6-Route on the original
// created unit, via the subscriber's link-local address with Ze's interface
// protocol, and keeps it there after the rename; closing IPv6CP withdraws
// that exact route while the original unit still exists, and leaves the
// operator's route to the same prefix on the dummy; released cleanup leaves
// the operator's route too.
// PREVENTS: a session that adds a route without retaining its ownership
// intent, so the withdrawal finds nothing owned and the route outlives the
// family that installed it; and a withdrawal that removes a route it does
// not own, the operator's route of the same destination on the dummy.
func TestServerNativeIPv6FramedRouteOwnedByOriginalUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	// RFC 3162 Section 2.5: the unspecified gateway names the subscriber.
	framed := []l2tpevents.FramedRoute{{Prefix: serverV6FramedPrefix, Gateway: netip.IPv6Unspecified()}}
	// auth-method pap alone offers CHAP-MD5 first (ppp.ResolveAuthAllow); the
	// allow list makes PAP the one method the server's LCP request names.
	f := startServerV6Fixture(t, "pppoe { auth-method pap; authentication-allow [pap]; interface "+serverV6Access+" {} }", framed)
	original := int(f.identity.NativeIndex)
	serverV6RequireFramedRoute(t, serverV6PrefixRoutes(t, f.route), original)

	link, err := f.route.LinkByIndex(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.route.LinkSetName(link, serverV6Renamed); err != nil {
		t.Fatalf("rename the original %s: %v", f.identity.InterfaceName, err)
	}
	if err := f.route.LinkAdd(&netlink.Dummy{Name: f.identity.InterfaceName}); err != nil {
		t.Fatalf("give the old name %s to a dummy: %v", f.identity.InterfaceName, err)
	}
	replacement, err := f.route.LinkByName(f.identity.InterfaceName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.route.LinkDel(replacement); err != nil {
			t.Error(err)
		}
	})
	dummy := replacement.Attrs().Index
	if dummy == original {
		t.Fatal("the dummy reused the original's index; the rename proves nothing")
	}
	// Linux fib6_nh_init refuses a route through a device that is down.
	if err := f.route.LinkSetUp(replacement); err != nil {
		t.Fatal(err)
	}
	serverV6RequireFramedRoute(t, serverV6PrefixRoutes(t, f.route), original)

	operator := &netlink.Route{LinkIndex: dummy, Dst: serverV6IPNet(serverV6FramedPrefix), Priority: serverV6OperatorMetric}
	if err := f.route.RouteAdd(operator); err != nil {
		t.Fatalf("add the operator's route to %s on the dummy: %v", serverV6FramedPrefix, err)
	}

	// RFC 1661 Section 4.1: Terminate-Request in Opened runs tld, zrc and
	// sta, in that order, so the Terminate-Ack follows the withdrawal.
	if err := f.peer.sendControl(ppp.ProtoIPv6CP, ppp.LCPTerminateRequest, serverV6TerminateID, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.peer.v6Closed:
	case <-time.After(serverV6Wait):
		t.Fatal("the server never acknowledged the subscriber's IPv6CP Terminate-Request")
	}
	if _, err := f.route.LinkByIndex(original); err != nil {
		t.Fatalf("the original unit is gone after IPv6CP closed, so a missing route proves no withdrawal: %v", err)
	}
	serverV6RequireOperatorOnly(t, serverV6PrefixRoutes(t, f.route), dummy, "after IPv6CP closed")

	f.releaseByPADT(t)
	serverV6RequireOperatorOnly(t, serverV6PrefixRoutes(t, f.route), dummy, "after released cleanup")
}

// serverV6PrefixRoutes lists the main-table IPv6 routes to serverV6FramedPrefix.
func serverV6PrefixRoutes(t *testing.T, route *netlink.Handle) []netlink.Route {
	t.Helper()
	filter := &netlink.Route{Dst: serverV6IPNet(serverV6FramedPrefix)}
	routes, err := route.RouteListFiltered(netlink.FAMILY_V6, filter, netlink.RT_FILTER_DST)
	if err != nil {
		t.Fatalf("list the routes to %s: %v", serverV6FramedPrefix, err)
	}
	return routes
}

// serverV6RequireFramedRoute requires exactly one route to the prefix: the
// session's, on the original unit, via the subscriber's link-local address.
func serverV6RequireFramedRoute(t *testing.T, routes []netlink.Route, original int) {
	t.Helper()
	if len(routes) != 1 {
		t.Fatalf("routes to %s=%s, want only the session's framed route", serverV6FramedPrefix, serverV6Routes(routes))
	}
	route := routes[0]
	if route.LinkIndex != original {
		t.Fatalf("framed route on index %d, want the original unit %d", route.LinkIndex, original)
	}
	if !route.Gw.Equal(serverV6PeerLL.AsSlice()) {
		t.Fatalf("framed route gateway=%s, want the subscriber %s", route.Gw, serverV6PeerLL)
	}
	if route.Protocol != netlink.RouteProtocol(rtproto.Iface) {
		t.Fatalf("framed route protocol=%d, want %d", route.Protocol, rtproto.Iface)
	}
}

// serverV6RequireOperatorOnly requires that the only route to the prefix is
// the operator's device route on the dummy.
func serverV6RequireOperatorOnly(t *testing.T, routes []netlink.Route, dummy int, when string) {
	t.Helper()
	if len(routes) != 1 {
		t.Fatalf("%s, routes to %s=%s, want only the operator's route on index %d", when, serverV6FramedPrefix, serverV6Routes(routes), dummy)
	}
	route := routes[0]
	if route.LinkIndex != dummy {
		t.Fatalf("%s, the remaining route is on index %d, want the operator's on the dummy %d", when, route.LinkIndex, dummy)
	}
	if route.Priority != serverV6OperatorMetric {
		t.Fatalf("%s, the remaining route has metric %d, want the operator's %d", when, route.Priority, serverV6OperatorMetric)
	}
	if route.Gw != nil {
		t.Fatalf("%s, the remaining route has gateway %s, want the operator's device route", when, route.Gw)
	}
}

func serverV6Routes(routes []netlink.Route) string {
	text := "["
	for i := range routes {
		route := &routes[i]
		if i > 0 {
			text += " "
		}
		text += fmt.Sprintf("{index %d via %s metric %d protocol %d}", route.LinkIndex, route.Gw, route.Priority, route.Protocol)
	}
	return text + "]"
}

func serverV6IPNet(prefix netip.Prefix) *net.IPNet {
	return &net.IPNet{IP: prefix.Addr().AsSlice(), Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen())}
}

// serverV6AsksPAP reports whether an LCP Configure-Request's options name PAP,
// and refuses any other Authentication-Protocol: the scripted subscriber
// speaks only PAP, and acknowledging CHAP would leave it silent.
// RFC 1661 Section 6.2: Authentication-Protocol, type 3, carries the protocol.
func serverV6AsksPAP(options []byte) (bool, error) {
	parsed, err := ppp.ParseLCPOptions(options)
	if err != nil {
		return false, fmt.Errorf("server LCP Configure-Request options %x: %w", options, err)
	}
	for _, option := range parsed {
		if option.Type != ppp.LCPOptAuthProto {
			continue
		}
		if len(option.Data) < 2 {
			return false, fmt.Errorf("server Authentication-Protocol option %x is too short", option.Data)
		}
		protocol := binary.BigEndian.Uint16(option.Data)
		if protocol != ppp.ProtoPAP {
			return false, fmt.Errorf("server asked for authentication protocol %#04x; the scripted subscriber speaks only PAP (set authentication-allow [pap])", protocol)
		}
		return true, nil
	}
	return false, nil
}

// serverV6Trace names one PPP control packet for the subscriber's trace.
func serverV6Trace(proto uint16, packet []byte) string {
	name := fmt.Sprintf("protocol %#04x", proto)
	switch proto {
	case ppp.ProtoLCP:
		name = "LCP"
	case ppp.ProtoPAP:
		name = "PAP"
	case ppp.ProtoCHAP:
		name = "CHAP"
	case ppp.ProtoIPCP:
		name = "IPCP"
	case ppp.ProtoIPv6CP:
		name = "IPv6CP"
	}
	if len(packet) < 2 {
		return name + " (truncated)"
	}
	return fmt.Sprintf("%s code %d id %d data %x", name, packet[0], packet[1], packet[2:])
}

// serverV6PAPCredentials is the Data of an RFC 1334 Section 2.2.1
// Authenticate-Request: Peer-ID Length, Peer-ID, Passwd-Length, Password.
func serverV6PAPCredentials() []byte {
	data := []byte{byte(len(serverV6PAPUser))}
	data = append(data, serverV6PAPUser...)
	data = append(data, byte(len(serverV6PAPPassword)))
	return append(data, serverV6PAPPassword...)
}

// serverV6PAPReply refuses a PAP Authenticate-Nak: the fixture accepts every
// authentication, so a Nak is a server defect.
func serverV6PAPReply(info []byte) error {
	if len(info) == 0 {
		return nil
	}
	if info[0] == ppp.PAPAuthenticateNak {
		return fmt.Errorf("server refused the subscriber's PAP credentials: %x", info)
	}
	return nil
}
