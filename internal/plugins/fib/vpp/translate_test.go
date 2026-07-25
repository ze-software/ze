// VPP FIB translate: pure conversion of ze prefix/next-hop values into GoVPP
// wire types (toVPPPrefix, toFibPath) plus the sysrib event field that carries
// MPLS labels. No VPP channel or backend is involved -- these are pure
// functions exercised directly.
package fibvpp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"

	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip_types"
)

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

func TestToVPPPrefixIPv4(t *testing.T) {
	// VALIDATES: AC-7 -- IPv4 prefix conversion
	// PREVENTS: wrong AF or prefix length
	p := netip.MustParsePrefix("10.0.0.0/24")
	vp := toVPPPrefix(p)

	if vp.Address.Af != 0 { // ADDRESS_IP4 = 0
		t.Errorf("expected AF=0 (IPv4), got %d", vp.Address.Af)
	}
	if vp.Len != 24 {
		t.Errorf("expected prefix length 24, got %d", vp.Len)
	}
}

func TestToVPPPrefixIPv6(t *testing.T) {
	// VALIDATES: AC-8 -- IPv6 prefix conversion
	// PREVENTS: wrong AF or prefix length
	p := netip.MustParsePrefix("2001:db8::/32")
	vp := toVPPPrefix(p)

	if vp.Address.Af != 1 { // ADDRESS_IP6 = 1
		t.Errorf("expected AF=1 (IPv6), got %d", vp.Address.Af)
	}
	if vp.Len != 32 {
		t.Errorf("expected prefix length 32, got %d", vp.Len)
	}
}

// verify the sysrib event type includes Labels field.
func TestSysribEventLabelsField(t *testing.T) {
	entry := sysribevents.BestChangeEntry{
		Action: routeaction.Add,
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		Labels: []uint32{100, 200},
	}
	assert.Equal(t, []uint32{100, 200}, entry.Labels)

	entryNoLabels := sysribevents.BestChangeEntry{
		Action: routeaction.Add,
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
	}
	assert.Nil(t, entryNoLabels.Labels)
}
