package fibvpp

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// testChannel is a mock api.Channel that captures the last SendRequest message
// and returns a configurable reply via ReceiveReply.
type testChannel struct {
	lastRequest api.Message
	retval      int32 // reply retval for IPRouteAddDelReply
	sendErr     error // error returned by ReceiveReply
	closed      bool
}

var _ api.Channel = (*testChannel)(nil)

type testRequestCtx struct {
	ch *testChannel
}

func (r *testRequestCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	if reply, ok := msg.(*ip.IPRouteAddDelReply); ok {
		reply.Retval = r.ch.retval
	}
	return nil
}

func (c *testChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	return &testRequestCtx{ch: c}
}

func (c *testChannel) SendMultiRequest(api.Message) api.MultiRequestCtx { return nil }
func (c *testChannel) SubscribeNotification(chan api.Message, api.Message) (api.SubscriptionCtx, error) {
	return nil, nil //nolint:nilnil // test stub, never called
}
func (c *testChannel) SetReplyTimeout(time.Duration)          {}
func (c *testChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *testChannel) Close()                                 { c.closed = true }

// --- toVPPPrefix tests ---

func TestToVPPPrefixIPv4Bytes(t *testing.T) {
	// VALIDATES: AC-7 -- IPv4 prefix bytes and length correct
	p := netip.MustParsePrefix("10.20.30.0/24")
	vp := toVPPPrefix(p)

	if vp.Address.Af != ip_types.ADDRESS_IP4 {
		t.Fatalf("AF: got %d, want ADDRESS_IP4 (0)", vp.Address.Af)
	}
	if vp.Len != 24 {
		t.Fatalf("Len: got %d, want 24", vp.Len)
	}

	ip4 := vp.Address.Un.GetIP4()
	if ip4[0] != 10 || ip4[1] != 20 || ip4[2] != 30 || ip4[3] != 0 {
		t.Errorf("IP4 bytes: got %v, want [10 20 30 0]", ip4)
	}
}

func TestToVPPPrefixIPv6Bytes(t *testing.T) {
	// VALIDATES: AC-8 -- IPv6 prefix bytes and length correct
	p := netip.MustParsePrefix("2001:db8:abcd::/48")
	vp := toVPPPrefix(p)

	if vp.Address.Af != ip_types.ADDRESS_IP6 {
		t.Fatalf("AF: got %d, want ADDRESS_IP6 (1)", vp.Address.Af)
	}
	if vp.Len != 48 {
		t.Fatalf("Len: got %d, want 48", vp.Len)
	}

	ip6 := vp.Address.Un.GetIP6()
	// 2001:0db8:abcd::
	if ip6[0] != 0x20 || ip6[1] != 0x01 || ip6[2] != 0x0d || ip6[3] != 0xb8 {
		t.Errorf("IP6 first 4 bytes: got %x %x %x %x, want 20 01 0d b8", ip6[0], ip6[1], ip6[2], ip6[3])
	}
}

func TestToVPPPrefixHostRoute(t *testing.T) {
	// VALIDATES: boundary -- /32 IPv4 host route
	p := netip.MustParsePrefix("192.168.1.1/32")
	vp := toVPPPrefix(p)

	if vp.Len != 32 {
		t.Errorf("Len: got %d, want 32", vp.Len)
	}
}

func TestToVPPPrefixDefaultRoute(t *testing.T) {
	// VALIDATES: boundary -- /0 default route
	p := netip.MustParsePrefix("0.0.0.0/0")
	vp := toVPPPrefix(p)

	if vp.Len != 0 {
		t.Errorf("Len: got %d, want 0", vp.Len)
	}
}

// --- toFibPath tests ---

func TestToFibPathIPv4(t *testing.T) {
	// VALIDATES: AC-7 -- IPv4 next-hop FibPath
	nh := netip.MustParseAddr("192.168.1.1")
	path := toFibPath(nh)

	if path.Proto != fib_types.FIB_API_PATH_NH_PROTO_IP4 {
		t.Fatalf("Proto: got %d, want FIB_API_PATH_NH_PROTO_IP4", path.Proto)
	}
	if path.Weight != 1 {
		t.Errorf("Weight: got %d, want 1", path.Weight)
	}

	ip4 := path.Nh.Address.GetIP4()
	if ip4[0] != 192 || ip4[1] != 168 || ip4[2] != 1 || ip4[3] != 1 {
		t.Errorf("Nh IP4: got %v, want [192 168 1 1]", ip4)
	}
}

func TestToFibPathIPv6(t *testing.T) {
	// VALIDATES: AC-8 -- IPv6 next-hop FibPath
	nh := netip.MustParseAddr("fe80::1")
	path := toFibPath(nh)

	if path.Proto != fib_types.FIB_API_PATH_NH_PROTO_IP6 {
		t.Fatalf("Proto: got %d, want FIB_API_PATH_NH_PROTO_IP6", path.Proto)
	}

	ip6 := path.Nh.Address.GetIP6()
	if ip6[0] != 0xfe || ip6[1] != 0x80 {
		t.Errorf("Nh IP6 first 2 bytes: got %x %x, want fe 80", ip6[0], ip6[1])
	}
	if ip6[15] != 0x01 {
		t.Errorf("Nh IP6 last byte: got %x, want 01", ip6[15])
	}
}

// --- govppBackend via mock channel tests ---

func TestBackendAddRoute(t *testing.T) {
	// VALIDATES: AC-1 -- addRoute sends IPRouteAddDel with IsAdd=true
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("192.168.1.1"))
	if err != nil {
		t.Fatalf("addRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *ip.IPRouteAddDel", ch.lastRequest)
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.Route.NPaths != 1 {
		t.Errorf("NPaths: got %d, want 1", req.Route.NPaths)
	}
	if len(req.Route.Paths) != 1 {
		t.Fatalf("Paths len: got %d, want 1", len(req.Route.Paths))
	}
}

func TestBackendDelRoute(t *testing.T) {
	// VALIDATES: AC-2 -- delRoute sends IPRouteAddDel with IsAdd=false, no paths
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.delRoute(netip.MustParsePrefix("10.0.0.0/24"))
	if err != nil {
		t.Fatalf("delRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.IsAdd {
		t.Error("IsAdd: got true, want false")
	}
	if req.Route.NPaths != 0 {
		t.Errorf("NPaths: got %d, want 0 for delete", req.Route.NPaths)
	}
	if req.Route.Paths != nil {
		t.Errorf("Paths: got %v, want nil for delete", req.Route.Paths)
	}
}

func TestBackendReplaceRoute(t *testing.T) {
	// VALIDATES: AC-3 -- replaceRoute sends IsAdd=true (VPP overwrites)
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.replaceRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("10.10.10.1"))
	if err != nil {
		t.Fatalf("replaceRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if !req.IsAdd {
		t.Error("replace should use IsAdd=true")
	}
}

func TestBackendVRFTableID(t *testing.T) {
	// VALIDATES: AC-9 -- table-id propagated to VPP request
	ch := &testChannel{}
	b := newGovppBackend(ch, 42)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("1.1.1.1"))
	if err != nil {
		t.Fatalf("addRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.Route.TableID != 42 {
		t.Errorf("TableID: got %d, want 42", req.Route.TableID)
	}
}

func TestBackendRetvalError(t *testing.T) {
	// VALIDATES: VPP retval != 0 produces error
	ch := &testChannel{retval: -1}
	b := newGovppBackend(ch, 0)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("1.1.1.1"))
	if err == nil {
		t.Fatal("expected error for retval=-1")
	}
}

func TestBackendSendError(t *testing.T) {
	// VALIDATES: GoVPP send error propagated
	ch := &testChannel{sendErr: fmt.Errorf("connection lost")}
	b := newGovppBackend(ch, 0)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("1.1.1.1"))
	if err == nil {
		t.Fatal("expected error for send failure")
	}
}

func TestBackendClose(t *testing.T) {
	// VALIDATES: AC-10 -- close releases channel
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if !ch.closed {
		t.Error("channel not closed")
	}
}

func TestBackendIPv4PrefixConversion(t *testing.T) {
	// VALIDATES: AC-7 -- IPv4 prefix in IPRouteAddDel has correct AF and length
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	_ = b.addRoute(netip.MustParsePrefix("172.16.0.0/12"), netip.MustParseAddr("10.0.0.1"))

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.Route.Prefix.Address.Af != ip_types.ADDRESS_IP4 {
		t.Errorf("Prefix AF: got %d, want ADDRESS_IP4", req.Route.Prefix.Address.Af)
	}
	if req.Route.Prefix.Len != 12 {
		t.Errorf("Prefix Len: got %d, want 12", req.Route.Prefix.Len)
	}
}

func TestBackendIPv6PrefixConversion(t *testing.T) {
	// VALIDATES: AC-8 -- IPv6 prefix in IPRouteAddDel has correct AF and length
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	_ = b.addRoute(netip.MustParsePrefix("2001:db8::/32"), netip.MustParseAddr("fe80::1"))

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.Route.Prefix.Address.Af != ip_types.ADDRESS_IP6 {
		t.Errorf("Prefix AF: got %d, want ADDRESS_IP6", req.Route.Prefix.Address.Af)
	}
	if req.Route.Prefix.Len != 32 {
		t.Errorf("Prefix Len: got %d, want 32", req.Route.Prefix.Len)
	}
	if req.Route.Paths[0].Proto != fib_types.FIB_API_PATH_NH_PROTO_IP6 {
		t.Errorf("Path Proto: got %d, want FIB_API_PATH_NH_PROTO_IP6", req.Route.Paths[0].Proto)
	}
}
