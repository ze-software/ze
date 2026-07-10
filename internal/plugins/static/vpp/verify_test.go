// VALIDATES: static/vpp action semantics reach the wire correctly — blackhole
// emits a DROP path, reject emits an ICMP-unreachable path, forward emits the
// next-hop paths, and a non-zero VPP retval produces a clear, wrapped error.
// PREVENTS: an action being programmed as the wrong FibPath type, or a VPP
// rejection being swallowed instead of surfaced to the caller.
package staticvpp

import (
	"net/netip"
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
)

func sentPaths(t *testing.T, ch *testChannel) []fib_types.FibPath {
	t.Helper()
	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *ip.IPRouteAddDel", ch.lastRequest)
	}
	return req.Route.Paths
}

func TestVerifyBlackholeEmitsDrop(t *testing.T) {
	ch := &testChannel{}
	if err := NewBackend(ch, 0).ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionBlackhole}); err != nil {
		t.Fatalf("ApplyRoute: %v", err)
	}
	paths := sentPaths(t, ch)
	if len(paths) != 1 || paths[0].Type != fib_types.FIB_API_PATH_TYPE_DROP {
		t.Fatalf("blackhole on the wire: got %+v, want single DROP", paths)
	}
}

func TestVerifyRejectEmitsIcmpUnreach(t *testing.T) {
	ch := &testChannel{}
	if err := NewBackend(ch, 0).ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionReject}); err != nil {
		t.Fatalf("ApplyRoute: %v", err)
	}
	paths := sentPaths(t, ch)
	if len(paths) != 1 || paths[0].Type != fib_types.FIB_API_PATH_TYPE_ICMP_UNREACH {
		t.Fatalf("reject on the wire: got %+v, want single ICMP_UNREACH", paths)
	}
}

func TestVerifyForwardEmitsNextHops(t *testing.T) {
	ch := &testChannel{}
	err := NewBackend(ch, 0).ApplyRoute(Route{
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		Action: ActionForward,
		Paths:  []Path{{NextHop: netip.MustParseAddr("192.168.1.1"), Weight: 1}},
	})
	if err != nil {
		t.Fatalf("ApplyRoute: %v", err)
	}
	paths := sentPaths(t, ch)
	if len(paths) != 1 || paths[0].Type != fib_types.FIB_API_PATH_TYPE_NORMAL {
		t.Fatalf("forward on the wire: got %+v, want single NORMAL next-hop path", paths)
	}
}

// TestVerifyRetvalClearError proves a VPP rejection surfaces as a clear error
// naming the retval, not a silent success.
func TestVerifyRetvalClearError(t *testing.T) {
	ch := &testChannel{retval: -3}
	err := NewBackend(ch, 0).ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionForward, Paths: []Path{{NextHop: netip.MustParseAddr("1.1.1.1"), Weight: 1}}})
	if err == nil {
		t.Fatal("expected error for retval=-3")
	}
	if !strings.Contains(err.Error(), "retval") {
		t.Errorf("error %q should name the retval", err)
	}
}
