// Design: docs/architecture/mpls/mpls-kernel.md -- label push through an MPLS lwtunnel
// Related: nexthop_linux.go -- buildRichRoute, buildMPLSEncap
//
// VALIDATES: every label push fib-kernel hands the kernel (the route-level
// stack, each ECMP member's stack, and each fast-reroute backup's repair
// stack) is an MPLS lwtunnel whose encoded attributes are the label stack
// (MPLS_IPTUNNEL_DST) alone, with no MPLS_IPTUNNEL_TTL. Method: build each
// route with buildRichRoute, encode every Encap the route carries exactly as
// netlink sends it, and walk the attributes.
// PREVENTS: a fixed TTL on the push. Linux mpls_xmit (net/mpls/mpls_iptunnel.c)
// writes a tunnel's own TTL into the label it imposes when the tunnel carries
// one, so the label would no longer carry the IP TTL (RFC 3032 Section 2.4.3).
// What the kernel does without the attribute depends on
// net.mpls.ip_ttl_propagate, which the sysctl registry refuses to set to 0
// (internal/core/sysctl/known_linux.go, refuseMPLSTTLNoPropagate).

//go:build linux

package fibkernel

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

// pushAttributes encodes one route encap as netlink sends it and returns the
// type of each attribute, with the label values of the MPLS_IPTUNNEL_DST one.
func pushAttributes(t *testing.T, encap netlink.Encap) (types []uint16, labels []uint32) {
	t.Helper()
	if encap == nil {
		t.Fatal("the route carries no encap, so no label is pushed")
	}
	if encap.Type() != unix.LWTUNNEL_ENCAP_MPLS {
		t.Fatalf("encap type = %d, want LWTUNNEL_ENCAP_MPLS (%d)", encap.Type(), unix.LWTUNNEL_ENCAP_MPLS)
	}
	wire, err := encap.Encode()
	if err != nil {
		t.Fatalf("encode encap: %v", err)
	}
	// Each attribute is a native-endian 2-octet length (header included), a
	// 2-octet type, then the payload padded to 4 octets. The walk is bounded
	// by the encoded length.
	for off := 0; off < len(wire); {
		if len(wire)-off < 4 {
			t.Fatalf("truncated attribute header at offset %d of %d", off, len(wire))
		}
		size := int(binary.NativeEndian.Uint16(wire[off:]))
		kind := binary.NativeEndian.Uint16(wire[off+2:])
		if size < 4 || off+size > len(wire) {
			t.Fatalf("attribute at offset %d has length %d, outside the %d encoded octets", off, size, len(wire))
		}
		types = append(types, kind)
		if kind == unix.MPLS_IPTUNNEL_DST {
			// Each label stack entry is 4 octets, network order, the label in
			// the high 20 bits (RFC 3032 Section 2.1).
			for e := off + 4; e+4 <= off+size; e += 4 {
				labels = append(labels, binary.BigEndian.Uint32(wire[e:])>>12)
			}
		}
		off += (size + 3) &^ 3
	}
	return types, labels
}

// assertPushCarriesNoTTL checks one encap: its only attribute is the label
// stack, holding want.
func assertPushCarriesNoTTL(t *testing.T, where string, encap netlink.Encap, want []uint32) {
	t.Helper()
	types, labels := pushAttributes(t, encap)
	for _, kind := range types {
		if kind == unix.MPLS_IPTUNNEL_TTL {
			t.Errorf("%s: the push carries MPLS_IPTUNNEL_TTL, so the kernel imposes that fixed TTL instead of the IP TTL", where)
		}
	}
	if len(types) != 1 || types[0] != unix.MPLS_IPTUNNEL_DST {
		t.Errorf("%s: encap attributes = %v, want only MPLS_IPTUNNEL_DST (%d)", where, types, unix.MPLS_IPTUNNEL_DST)
	}
	if len(labels) != len(want) {
		t.Fatalf("%s: pushed labels = %v, want %v", where, labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("%s: pushed labels = %v, want %v", where, labels, want)
			break
		}
	}
}

// RFC requirement: RFC3032-2.4.3-1 positive -- every label push fib-kernel builds (route-level, ECMP member and fast-reroute backup) is an MPLS lwtunnel that carries the label stack and no MPLS_IPTUNNEL_TTL, so the push itself imposes no fixed TTL in place of the IP TTL.
func TestBuildRichRoutePushCarriesNoTTL(t *testing.T) {
	prefix := netip.MustParsePrefix("198.51.100.0/24")

	single, err := buildRichRoute(RichRoute{
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{16000, 24001},
	})
	if err != nil {
		t.Fatalf("buildRichRoute single path: %v", err)
	}
	assertPushCarriesNoTTL(t, "route-level push", single.Encap, []uint32{16000, 24001})

	ecmp, err := buildRichRoute(RichRoute{
		Prefix: prefix,
		ECMPPaths: []sysribevents.ECMPPath{
			{NextHop: netip.MustParseAddr("192.0.2.1"), Labels: []uint32{16001}},
			{NextHop: netip.MustParseAddr("192.0.2.2"), Labels: []uint32{16002, 24002}},
		},
	})
	if err != nil {
		t.Fatalf("buildRichRoute ECMP: %v", err)
	}
	if len(ecmp.MultiPath) != 2 {
		t.Fatalf("ECMP route has %d next hops, want 2", len(ecmp.MultiPath))
	}
	assertPushCarriesNoTTL(t, "ECMP member 1", ecmp.MultiPath[0].Encap, []uint32{16001})
	assertPushCarriesNoTTL(t, "ECMP member 2", ecmp.MultiPath[1].Encap, []uint32{16002, 24002})

	backup, err := buildRichRoute(RichRoute{
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{16003},
		Backup: []sysribevents.ECMPPath{
			{NextHop: netip.MustParseAddr("192.0.2.9"), Labels: []uint32{16010, 24003}},
		},
	})
	if err != nil {
		t.Fatalf("buildRichRoute backup: %v", err)
	}
	var repairs int
	for _, nh := range backup.MultiPath {
		if nh.Flags&unix.RTNH_F_LINKDOWN == 0 {
			assertPushCarriesNoTTL(t, "primary beside a backup", nh.Encap, []uint32{16003})
			continue
		}
		repairs++
		assertPushCarriesNoTTL(t, "fast-reroute repair", nh.Encap, []uint32{16010, 24003})
	}
	if repairs != 1 {
		t.Fatalf("backup route carries %d repair next hops, want 1", repairs)
	}
}
