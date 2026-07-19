// VALIDATES: static/vpp pure translation — netip prefix/next-hop → VPP wire
// types (address family, byte order, prefix length), ECMP weight coercion, and
// per-action FibPath construction.
// PREVENTS: mis-encoded prefixes/next-hops reaching VPP (wrong AF, swapped
// bytes, weight-0 paths that VPP rejects, wrong path type per action).
package staticvpp

import (
	"net/netip"
	"testing"

	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip_types"
)

func TestToVPPPrefixIPv4(t *testing.T) {
	vp := toVPPPrefix(netip.MustParsePrefix("10.20.30.0/24"))
	if vp.Address.Af != ip_types.ADDRESS_IP4 {
		t.Fatalf("AF: got %d, want ADDRESS_IP4", vp.Address.Af)
	}
	if vp.Len != 24 {
		t.Fatalf("Len: got %d, want 24", vp.Len)
	}
	ip4 := vp.Address.Un.GetIP4()
	if ip4[0] != 10 || ip4[1] != 20 || ip4[2] != 30 || ip4[3] != 0 {
		t.Errorf("IP4 bytes: got %v, want [10 20 30 0]", ip4)
	}
}

func TestToVPPPrefixIPv6(t *testing.T) {
	vp := toVPPPrefix(netip.MustParsePrefix("2001:db8:abcd::/48"))
	if vp.Address.Af != ip_types.ADDRESS_IP6 {
		t.Fatalf("AF: got %d, want ADDRESS_IP6", vp.Address.Af)
	}
	if vp.Len != 48 {
		t.Fatalf("Len: got %d, want 48", vp.Len)
	}
	ip6 := vp.Address.Un.GetIP6()
	if ip6[0] != 0x20 || ip6[1] != 0x01 || ip6[2] != 0x0d || ip6[3] != 0xb8 {
		t.Errorf("IP6 first 4 bytes: got %x %x %x %x, want 20 01 0d b8", ip6[0], ip6[1], ip6[2], ip6[3])
	}
}

func TestToVPPPrefixBoundaryLengths(t *testing.T) {
	if got := toVPPPrefix(netip.MustParsePrefix("192.168.1.1/32")).Len; got != 32 {
		t.Errorf("/32 Len: got %d, want 32", got)
	}
	if got := toVPPPrefix(netip.MustParsePrefix("0.0.0.0/0")).Len; got != 0 {
		t.Errorf("/0 Len: got %d, want 0", got)
	}
}

func TestToFibPathIPv4(t *testing.T) {
	fp := toFibPath(Path{NextHop: netip.MustParseAddr("192.168.1.1"), Weight: 5, SwIfIndex: 3}, netip.MustParsePrefix("10.0.0.0/24"))
	if fp.Proto != fib_types.FIB_API_PATH_NH_PROTO_IP4 {
		t.Fatalf("Proto: got %d, want IP4", fp.Proto)
	}
	if fp.Weight != 5 {
		t.Errorf("Weight: got %d, want 5 (preserved)", fp.Weight)
	}
	if fp.SwIfIndex != 3 {
		t.Errorf("SwIfIndex: got %d, want 3", fp.SwIfIndex)
	}
	ip4 := fp.Nh.Address.GetIP4()
	if ip4[0] != 192 || ip4[3] != 1 {
		t.Errorf("Nh IP4: got %v, want [192 168 1 1]", ip4)
	}
}

func TestToFibPathIPv6(t *testing.T) {
	fp := toFibPath(Path{NextHop: netip.MustParseAddr("fe80::1"), Weight: 1}, netip.MustParsePrefix("2001:db8::/32"))
	if fp.Proto != fib_types.FIB_API_PATH_NH_PROTO_IP6 {
		t.Fatalf("Proto: got %d, want IP6", fp.Proto)
	}
	ip6 := fp.Nh.Address.GetIP6()
	if ip6[0] != 0xfe || ip6[1] != 0x80 || ip6[15] != 0x01 {
		t.Errorf("Nh IP6: got %x, want fe80::1", ip6)
	}
}

// TestToFibPathWeightZeroCoerced covers the weight boundary: VPP rejects
// weight 0, so the translator must coerce it to 1.
func TestToFibPathWeightZeroCoerced(t *testing.T) {
	fp := toFibPath(Path{NextHop: netip.MustParseAddr("10.0.0.1"), Weight: 0}, netip.MustParsePrefix("10.0.0.0/24"))
	if fp.Weight != 1 {
		t.Errorf("weight-0 coercion: got %d, want 1", fp.Weight)
	}
}

// TestToFibPathInterfaceOnlyIPv4UsesIP4Proto pins AC-9 / A-2a: an interface-only
// path (zero next-hop, SwIfIndex set) on an IPv4 route must encode PROTO_IP4, not
// the PROTO_IP6 the zero netip.Addr would otherwise default to. The next-hop
// address stays zero; the sw_if_index scopes the path.
func TestToFibPathInterfaceOnlyIPv4UsesIP4Proto(t *testing.T) {
	fp := toFibPath(Path{SwIfIndex: 7, Weight: 1}, netip.MustParsePrefix("10.0.0.0/24"))
	if fp.Proto != fib_types.FIB_API_PATH_NH_PROTO_IP4 {
		t.Fatalf("Proto: got %d, want IP4 for an IPv4 route with an interface-only next-hop", fp.Proto)
	}
	if fp.SwIfIndex != 7 {
		t.Errorf("SwIfIndex: got %d, want 7", fp.SwIfIndex)
	}
	if ip4 := fp.Nh.Address.GetIP4(); ip4 != (ip_types.IP4Address{}) {
		t.Errorf("Nh.Address: got %v, want zero (interface-only path carries no gateway)", ip4)
	}
}

// TestToFibPathInterfaceOnlyIPv6UsesIP6Proto is the IPv6 sibling: an
// interface-only path on an IPv6 route encodes PROTO_IP6.
func TestToFibPathInterfaceOnlyIPv6UsesIP6Proto(t *testing.T) {
	fp := toFibPath(Path{SwIfIndex: 9, Weight: 1}, netip.MustParsePrefix("2001:db8::/48"))
	if fp.Proto != fib_types.FIB_API_PATH_NH_PROTO_IP6 {
		t.Fatalf("Proto: got %d, want IP6 for an IPv6 route with an interface-only next-hop", fp.Proto)
	}
	if fp.SwIfIndex != 9 {
		t.Errorf("SwIfIndex: got %d, want 9", fp.SwIfIndex)
	}
}

func TestBuildFibPathsBlackhole(t *testing.T) {
	paths := buildFibPaths(Route{Action: ActionBlackhole})
	if len(paths) != 1 || paths[0].Type != fib_types.FIB_API_PATH_TYPE_DROP {
		t.Fatalf("blackhole: got %+v, want single DROP path", paths)
	}
}

func TestBuildFibPathsReject(t *testing.T) {
	paths := buildFibPaths(Route{Action: ActionReject})
	if len(paths) != 1 || paths[0].Type != fib_types.FIB_API_PATH_TYPE_ICMP_UNREACH {
		t.Fatalf("reject: got %+v, want single ICMP_UNREACH path", paths)
	}
}

func TestBuildFibPathsForwardMulti(t *testing.T) {
	paths := buildFibPaths(Route{
		Action: ActionForward,
		Paths: []Path{
			{NextHop: netip.MustParseAddr("10.0.0.1"), Weight: 1},
			{NextHop: netip.MustParseAddr("10.0.0.2"), Weight: 2},
		},
	})
	if len(paths) != 2 {
		t.Fatalf("forward: got %d paths, want 2", len(paths))
	}
	if paths[1].Weight != 2 {
		t.Errorf("second path weight: got %d, want 2", paths[1].Weight)
	}
}

func TestBuildFibPathsUnknownActionEmpty(t *testing.T) {
	if paths := buildFibPaths(Route{Action: ActionType(99)}); paths != nil {
		t.Errorf("unknown action: got %+v, want nil", paths)
	}
}
