package ifacevpp

import (
	"fmt"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// clearStatsReplyTarget is a local alias so the test file's reply handler
// names the concrete interfaces reply type exactly once. Using the alias
// gives the goimports pass a reason to keep the interfaces import even
// before every test has been wired.
type clearStatsReplyTarget = interfaces.SwInterfaceClearStatsReply

// routeChannel is a mock api.Channel for IPRouteV2Dump multi-requests.
// It returns v4Details on the first dump (IsIP6=false) and v6Details on
// the second. SwInterfaceClearStats replies are served through the
// single-request path so counter-reset tests can share the same fake.
type routeChannel struct {
	lastRequest   api.Message
	allRequests   []api.Message
	v4Details     []ip.IPRouteV2Details
	v6Details     []ip.IPRouteV2Details
	clearReply    clearStatsReply
	sendErr       error
	receiveErr    error
	dumpCallCount int
}

type clearStatsReply struct {
	retval int32
}

var _ api.Channel = (*routeChannel)(nil)

func (c *routeChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	return &routeRequestCtx{ch: c}
}

func (c *routeChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	isIP6 := false
	if m, ok := msg.(*ip.IPRouteV2Dump); ok {
		isIP6 = m.Table.IsIP6
	}
	details := c.v4Details
	if isIP6 {
		details = c.v6Details
	}
	c.dumpCallCount++
	return &routeMultiCtx{ch: c, details: details}
}

func (c *routeChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *routeChannel) SetReplyTimeout(time.Duration)          {}
func (c *routeChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *routeChannel) Close()                                 {}

type routeRequestCtx struct{ ch *routeChannel }

func (r *routeRequestCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	if reply, ok := msg.(*clearStatsReplyTarget); ok {
		reply.Retval = r.ch.clearReply.retval
	}
	return nil
}

type routeMultiCtx struct {
	ch      *routeChannel
	details []ip.IPRouteV2Details
	pos     int
}

func (m *routeMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.ch.receiveErr != nil {
		return false, m.ch.receiveErr
	}
	if m.pos >= len(m.details) {
		return true, nil
	}
	d, ok := msg.(*ip.IPRouteV2Details)
	if !ok {
		return false, fmt.Errorf("routeMultiCtx: unexpected reply type %T", msg)
	}
	*d = m.details[m.pos]
	m.pos++
	return false, nil
}

// makeIP4Route builds an ip.IPRouteV2Details representing a single v4
// route with the given prefix, next-hop, and fib source. gw == "" produces
// a connected route (all-zero next-hop).
func makeIP4Route(t *testing.T, cidr, gw string, src uint8, swIfIndex uint32) ip.IPRouteV2Details {
	t.Helper()
	prefix, err := ip_types.ParseIP4Prefix(cidr)
	if err != nil {
		t.Fatalf("ParseIP4Prefix(%q): %v", cidr, err)
	}
	route := ip.IPRouteV2{
		Src: src,
		Prefix: ip_types.Prefix{
			Address: ip_types.Address{
				Af: ip_types.ADDRESS_IP4,
				Un: ip_types.AddressUnionIP4(prefix.Address),
			},
			Len: prefix.Len,
		},
		NPaths: 1,
		Paths: []fib_types.FibPath{{
			SwIfIndex:  swIfIndex,
			Proto:      fib_types.FIB_API_PATH_NH_PROTO_IP4,
			Preference: 0,
			Weight:     1,
		}},
	}
	if gw != "" {
		gwAddr, err := ip_types.ParseIP4Address(gw)
		if err != nil {
			t.Fatalf("ParseIP4Address(%q): %v", gw, err)
		}
		var un ip_types.AddressUnion
		copy(un.XXX_UnionData[:4], gwAddr[:])
		route.Paths[0].Nh.Address = un
	}
	return ip.IPRouteV2Details{Route: route}
}

// makeIP6Route builds an ip.IPRouteV2Details representing a single v6 route.
func makeIP6Route(t *testing.T, cidr, gw string, src uint8) ip.IPRouteV2Details {
	t.Helper()
	prefix, err := ip_types.ParseIP6Prefix(cidr)
	if err != nil {
		t.Fatalf("ParseIP6Prefix(%q): %v", cidr, err)
	}
	route := ip.IPRouteV2{
		Src: src,
		Prefix: ip_types.Prefix{
			Address: ip_types.Address{
				Af: ip_types.ADDRESS_IP6,
				Un: ip_types.AddressUnionIP6(prefix.Address),
			},
			Len: prefix.Len,
		},
		NPaths: 1,
		Paths: []fib_types.FibPath{{
			Proto:      fib_types.FIB_API_PATH_NH_PROTO_IP6,
			Preference: 0,
			Weight:     1,
		}},
	}
	if gw != "" {
		gwAddr, err := ip_types.ParseIP6Address(gw)
		if err != nil {
			t.Fatalf("ParseIP6Address(%q): %v", gw, err)
		}
		var un ip_types.AddressUnion
		copy(un.XXX_UnionData[:16], gwAddr[:])
		route.Paths[0].Nh.Address = un
	}
	return ip.IPRouteV2Details{Route: route}
}

// --- ListKernelRoutes ---

// TestListKernelRoutesDumpsBothFamilies ensures the VPP backend queries
// both the v4 and v6 FIB tables and merges the results into a single slice.
// VALIDATES: ListKernelRoutes now returns real VPP FIB entries, not errNotSupported.
// PREVENTS: silent regression to the old errNotSupported stub.
func TestListKernelRoutesDumpsBothFamilies(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "192.168.1.1", 19, 0),
		},
		v6Details: []ip.IPRouteV2Details{
			makeIP6Route(t, "2001:db8::/32", "fe80::1", 19),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // mark populated so ensureChannel short-circuits

	routes, err := b.ListKernelRoutes("", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if ch.dumpCallCount != 2 {
		t.Errorf("dump calls: got %d, want 2 (v4 + v6)", ch.dumpCallCount)
	}
	if len(routes) != 2 {
		t.Fatalf("routes: got %d, want 2", len(routes))
	}
	families := map[string]bool{}
	for _, r := range routes {
		families[r.Family] = true
	}
	if !families["ipv4"] || !families["ipv6"] {
		t.Errorf("families: got %v, want both ipv4 and ipv6", families)
	}
}

// TestListKernelRoutesDecodesFields verifies the v4 decoding path produces
// the expected Destination, NextHop, Protocol, and Family.
// VALIDATES: FibPath + Prefix + Src decoding matches KernelRoute shape.
// PREVENTS: silently dropping or mangling a well-formed VPP reply.
func TestListKernelRoutesDecodesFields(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "192.168.1.1", 19 /* bgp */, 0),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes: got %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Destination != "10.0.0.0/8" {
		t.Errorf("Destination: got %q, want 10.0.0.0/8", r.Destination)
	}
	if r.NextHop != "192.168.1.1" {
		t.Errorf("NextHop: got %q, want 192.168.1.1", r.NextHop)
	}
	if r.Family != "ipv4" {
		t.Errorf("Family: got %q, want ipv4", r.Family)
	}
	if r.Protocol != "bgp" {
		t.Errorf("Protocol: got %q, want bgp", r.Protocol)
	}
}

// TestListKernelRoutesLimitCaps asserts the caller's limit stops the scan
// without returning an error.
// VALIDATES: limit parameter respected (0 = unbounded, N>0 = cap).
// PREVENTS: gigabyte allocation on full-DFZ dumps.
func TestListKernelRoutesLimitCaps(t *testing.T) {
	var v4 []ip.IPRouteV2Details
	for i := range 5 {
		cidr := fmt.Sprintf("10.%d.0.0/16", i)
		v4 = append(v4, makeIP4Route(t, cidr, "", 2 /* interface */, 0))
	}
	ch := &routeChannel{v4Details: v4}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("", 3)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 3 {
		t.Fatalf("routes: got %d, want 3 (capped by limit)", len(routes))
	}
}

// TestListKernelRoutesFilterPrefixExact restricts output to a single CIDR.
// VALIDATES: filterPrefix exact-match semantics mirror the netlink backend.
// PREVENTS: unexpectedly returning sibling routes sharing a prefix substring.
func TestListKernelRoutesFilterPrefixExact(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "", 2, 0),
			makeIP4Route(t, "10.0.0.0/16", "", 2, 0),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("10.0.0.0/8", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes: got %d, want 1", len(routes))
	}
	if routes[0].Destination != "10.0.0.0/8" {
		t.Errorf("Destination: got %q, want 10.0.0.0/8", routes[0].Destination)
	}
}

// TestListKernelRoutesFilterDefault matches the default-route sentinel.
// VALIDATES: "default" filter matches both 0.0.0.0/0 and ::/0.
// PREVENTS: operator confusion when asking for the default route on VPP.
func TestListKernelRoutesFilterDefault(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "0.0.0.0/0", "192.168.1.1", 19, 0),
			makeIP4Route(t, "10.0.0.0/8", "", 2, 0),
		},
		v6Details: []ip.IPRouteV2Details{
			makeIP6Route(t, "::/0", "fe80::1", 19),
			makeIP6Route(t, "2001:db8::/32", "", 2),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("default", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("routes: got %d, want 2 (v4 + v6 default)", len(routes))
	}
}

// TestListKernelRoutesResolvesDevice ensures the SwIfIndex path renders as
// a ze name once the name map is seeded.
// VALIDATES: Device field populated from nameMap lookup.
// PREVENTS: showing opaque ifindex integers to operators.
func TestListKernelRoutesResolvesDevice(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "192.168.1.1", 19, 7),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe7", 7, "xe7")
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes: got %d, want 1", len(routes))
	}
	if routes[0].Device != "xe7" {
		t.Errorf("Device: got %q, want xe7", routes[0].Device)
	}
}

// TestListKernelRoutesReceiveError propagates a channel failure.
// VALIDATES: VPP-side errors bubble up as Go errors (not silently empty).
// PREVENTS: silent loss of operator-observable failures.
func TestListKernelRoutesReceiveError(t *testing.T) {
	ch := &routeChannel{receiveErr: fmt.Errorf("VPP dead")}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	// v4Details needs at least one element so ReceiveReply is called (and
	// returns the error) before the last=true sentinel.
	ch.v4Details = []ip.IPRouteV2Details{makeIP4Route(t, "10.0.0.0/8", "", 2, 0)}
	if _, err := b.ListKernelRoutes("", 0); err == nil {
		t.Fatal("expected error from channel, got nil")
	}
}

// --- fibSourceName ---

func TestFibSourceNameKnown(t *testing.T) {
	if got := fibSourceName(19); got != "bgp" {
		t.Errorf("fibSourceName(19): got %q, want bgp", got)
	}
	if got := fibSourceName(10); got != "dhcp" {
		t.Errorf("fibSourceName(10): got %q, want dhcp", got)
	}
}

func TestFibSourceNameUnknown(t *testing.T) {
	// 200 is outside the well-known range; expect decimal string.
	if got := fibSourceName(200); got != "200" {
		t.Errorf("fibSourceName(200): got %q, want 200", got)
	}
}
