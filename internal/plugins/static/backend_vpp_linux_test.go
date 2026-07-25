// VALIDATES: parent static plugin's VPP backend selection and the
// staticRoute -> static/vpp Route translation (action mapping, address
// next-hops -> paths, weight cap, interface-only next-hop rejection), plus the
// end-to-end apply through a fake GoVPP channel.
// PREVENTS: static routes silently going to the wrong data plane, or being
// mistranslated (wrong action/table/paths) when programmed into VPP.

//go:build linux && ze_vpp

package static

import (
	"net/netip"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip"

	"github.com/ze-software/ze/internal/component/iface"
	staticvpp "github.com/ze-software/ze/internal/plugins/static/vpp"
)

// fakeChannel is a minimal api.Channel capturing the last request.
type fakeChannel struct {
	lastRequest api.Message
	closed      bool
}

var _ api.Channel = (*fakeChannel)(nil)

type fakeRequestCtx struct{ ch *fakeChannel }

func (r *fakeRequestCtx) ReceiveReply(msg api.Message) error {
	if reply, ok := msg.(*ip.IPRouteAddDelReply); ok {
		reply.Retval = 0
	}
	return nil
}

func (c *fakeChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	return &fakeRequestCtx{ch: c}
}
func (c *fakeChannel) SendMultiRequest(api.Message) api.MultiRequestCtx { return nil }
func (c *fakeChannel) SubscribeNotification(chan api.Message, api.Message) (api.SubscriptionCtx, error) {
	return nil, nil //nolint:nilnil // test stub, never called
}
func (c *fakeChannel) SetReplyTimeout(time.Duration)          {}
func (c *fakeChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *fakeChannel) Close()                                 { c.closed = true }

func TestNewStaticBackendNoVPPUsesKernel(t *testing.T) {
	// With no VPP connector active (the unit-test host), the backend must not be
	// the VPP backend — it falls through to the kernel/netlink path.
	b := newStaticBackend()
	t.Cleanup(func() { _ = b.close() })
	if _, isVPP := b.(*vppStaticBackend); isVPP {
		t.Fatal("selected VPP backend without an active VPP connector")
	}
}

func TestToVPPRouteForwardPaths(t *testing.T) {
	r := staticRoute{
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		Table:  5,
		Metric: 100,
		Action: actionForward,
		NextHops: []nextHop{
			{Address: netip.MustParseAddr("192.168.1.1"), Weight: 1},
			{Address: netip.MustParseAddr("192.168.1.2"), Weight: 300}, // caps to 255
		},
	}
	out, err := toVPPRoute(r)
	if err != nil {
		t.Fatalf("toVPPRoute: %v", err)
	}
	if len(out.Paths) != 2 {
		t.Fatalf("Paths: got %d, want 2", len(out.Paths))
	}
	if out.Paths[1].Weight != 255 {
		t.Errorf("weight cap: got %d, want 255", out.Paths[1].Weight)
	}
	if out.Metric != 100 {
		t.Errorf("Metric: got %d, want 100", out.Metric)
	}
}

func TestToVPPRouteActionMapping(t *testing.T) {
	for _, tc := range []struct {
		in   actionType
		want staticvpp.ActionType
	}{
		{actionForward, staticvpp.ActionForward},
		{actionBlackhole, staticvpp.ActionBlackhole},
		{actionReject, staticvpp.ActionReject},
	} {
		out, err := toVPPRoute(staticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: tc.in})
		if err != nil {
			t.Fatalf("action %v: %v", tc.in, err)
		}
		if out.Action != tc.want {
			t.Errorf("action %v mapped to %d, want %d", tc.in, out.Action, tc.want)
		}
	}
}

// TestToVPPRouteInterfaceNexthopRejectedNoBackend: with no iface backend loaded
// (the default unit-test state), an interface-only next-hop cannot resolve to a
// VPP sw_if_index, so toVPPRoute must reject it rather than emit an index-0
// path. The positive resolve case is covered in backend_vpp_iface_linux_test.go.
func TestToVPPRouteInterfaceNexthopRejectedNoBackend(t *testing.T) {
	if iface.ActiveBackendName() != "" {
		_ = iface.CloseBackend() // ensure the no-backend precondition
	}
	r := staticRoute{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Action:   actionForward,
		NextHops: []nextHop{{Interface: "eth0"}},
	}
	if _, err := toVPPRoute(r); err == nil {
		t.Fatal("interface-only next-hop must be rejected when no vpp iface backend is loaded")
	}
}

func TestToVPPRouteUnknownActionRejected(t *testing.T) {
	if _, err := toVPPRoute(staticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: actionType(99)}); err == nil {
		t.Fatal("unknown action must be rejected")
	}
}

func TestVPPStaticBackendApplyAndRemove(t *testing.T) {
	ch := &fakeChannel{}
	b := &vppStaticBackend{ch: ch}

	err := b.applyRoute(staticRoute{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Table:    7,
		Action:   actionForward,
		NextHops: []nextHop{{Address: netip.MustParseAddr("192.168.1.1"), Weight: 1}},
	})
	if err != nil {
		t.Fatalf("applyRoute: %v", err)
	}
	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if !req.IsAdd {
		t.Error("apply: IsAdd false")
	}
	if req.Route.TableID != 7 {
		t.Errorf("apply table id: got %d, want 7 (from route.Table)", req.Route.TableID)
	}

	if err := b.removeRoute(staticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Table: 7}); err != nil {
		t.Fatalf("removeRoute: %v", err)
	}
	del, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type after remove: got %T", ch.lastRequest)
	}
	if del.IsAdd {
		t.Error("remove: IsAdd true, want false")
	}
}

func TestVPPStaticBackendCloseClosesChannel(t *testing.T) {
	ch := &fakeChannel{}
	b := &vppStaticBackend{ch: ch}
	if err := b.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !ch.closed {
		t.Error("channel not closed")
	}
}
