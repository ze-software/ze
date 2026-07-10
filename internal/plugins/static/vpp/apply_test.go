// VALIDATES: static/vpp ApplyRoute/RemoveRoute send the correct IPRouteAddDel
// request (IsAdd, NPaths, prefix, table-id) and surface send/retval/path-cap
// errors through a fake GoVPP api.Channel — no real VPP daemon needed.
// PREVENTS: silent breakage of VPP static-route programming (wrong add/del
// flag, dropped paths, swallowed VPP errors, ECMP overflow).
package staticvpp

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip"
)

// testChannel is a fake api.Channel that captures the last SendRequest message
// and returns a configurable reply via ReceiveReply. Shared across the
// static/vpp backend test files.
type testChannel struct {
	lastRequest api.Message
	retval      int32 // reply retval for IPRouteAddDelReply
	sendErr     error // error returned by ReceiveReply
	closed      bool
}

var _ api.Channel = (*testChannel)(nil)

type testRequestCtx struct{ ch *testChannel }

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

func fwd(nh string) Path { return Path{NextHop: netip.MustParseAddr(nh), Weight: 1} }

func TestApplyRouteSendsAdd(t *testing.T) {
	ch := &testChannel{}
	b := NewBackend(ch, 7)

	err := b.ApplyRoute(Route{
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		Action: ActionForward,
		Paths:  []Path{fwd("192.168.1.1"), fwd("192.168.1.2")},
	})
	if err != nil {
		t.Fatalf("ApplyRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *ip.IPRouteAddDel", ch.lastRequest)
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.Route.TableID != 7 {
		t.Errorf("TableID: got %d, want 7", req.Route.TableID)
	}
	if req.Route.NPaths != 2 || len(req.Route.Paths) != 2 {
		t.Errorf("NPaths/Paths: got %d/%d, want 2/2", req.Route.NPaths, len(req.Route.Paths))
	}
}

func TestRemoveRouteSendsDelNoPaths(t *testing.T) {
	ch := &testChannel{}
	b := NewBackend(ch, 0)

	if err := b.RemoveRoute(netip.MustParsePrefix("10.0.0.0/24")); err != nil {
		t.Fatalf("RemoveRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.IsAdd {
		t.Error("IsAdd: got true, want false for remove")
	}
	if req.Route.NPaths != 0 || req.Route.Paths != nil {
		t.Errorf("delete carried paths: NPaths=%d Paths=%v", req.Route.NPaths, req.Route.Paths)
	}
}

func TestApplyRouteRetvalError(t *testing.T) {
	ch := &testChannel{retval: -1}
	b := NewBackend(ch, 0)

	err := b.ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionForward, Paths: []Path{fwd("1.1.1.1")}})
	if err == nil {
		t.Fatal("expected error for retval=-1")
	}
}

func TestApplyRouteSendError(t *testing.T) {
	ch := &testChannel{sendErr: fmt.Errorf("connection lost")}
	b := NewBackend(ch, 0)

	err := b.ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionForward, Paths: []Path{fwd("1.1.1.1")}})
	if err == nil {
		t.Fatal("expected error for send failure")
	}
}

// TestApplyRoutePathCap covers the ECMP boundary: 255 paths accepted, 256 rejected.
func TestApplyRoutePathCap(t *testing.T) {
	mkPaths := func(n int) []Path {
		paths := make([]Path, n)
		for i := range paths {
			// distinct 10.<hi>.<lo>.1 next-hops
			paths[i] = fwd(fmt.Sprintf("10.%d.%d.1", i/256, i%256))
		}
		return paths
	}

	ch := &testChannel{}
	b := NewBackend(ch, 0)

	if err := b.ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionForward, Paths: mkPaths(255)}); err != nil {
		t.Fatalf("255 paths should be accepted: %v", err)
	}
	if err := b.ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionForward, Paths: mkPaths(256)}); err == nil {
		t.Fatal("256 paths must be rejected (max 255)")
	}
}
