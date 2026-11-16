// Design: docs/labs/pppoe-interop.md -- discriminate actual IPv6 acceptance observations.
// Related: product_ipv6_wire.go -- independent receiver, not Ze's own codecs.
package testvpppppoe

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
)

func ipv6AcceptanceRAFixture() []byte {
	// ICMPv6 RA16 + PIO32, independently specified wire bytes.
	p := []byte{134, 0, 0, 0, 64, 0xc0, 7, 8, 0, 0, 0, 0, 0, 0, 0, 0,
		3, 4, 64, 0x80, 0, 0, 14, 16, 0, 0, 7, 8, 0, 0, 0, 0,
		0x20, 1, 0x0d, 0xb8, 0, 0x20, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	return p
}

func TestProductIPv6PlanUsesActualDaemonAndIndependentCPE(t *testing.T) {
	for _, mode := range []mechanismMode{modePlain, modeNamespace, modeVLAN} {
		plan := productIPv6Plan("checkout", "/owned", "unit", mode)
		if plan.Source.Checker == nil || plan.Source.Name != "pppoe-vpp-ze-ac-pppd-ipv6-"+mode.name() || len(plan.Peers) != 3 {
			t.Fatal("missing bounded native product scenario")
		}
		server, client := false, false
		for _, peer := range plan.Peers {
			for _, mount := range peer.Mounts {
				if peer.Name == vppPeer && mount.Target == "/product/ze" && mount.Source == filepath.Join(string(filepath.Separator), "owned", "ze") && mount.ReadOnly {
					server = true
				}
				if peer.Name == clientPeer && mount.Target == productHelper && mount.Source == filepath.Join(string(filepath.Separator), "owned", "le") && mount.ReadOnly {
					client = true
				}
				if mount.Target == helperPath {
					t.Fatal("scratch mechanism AC escaped into actual product acceptance")
				}
			}
		}
		if !server || !client {
			t.Fatal("actual daemon/independent CPE helper not staged")
		}
	}
	for _, verb := range []string{"product-ipv6", "product-ipv6-peer", "product-family-peer"} {
		found := false
		for _, action := range actions.Actions().Actions {
			if action.Verb == verb {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s is not registered", verb)
		}
	}
}

func FuzzProductIPv6AcceptanceReceivers(f *testing.F) {
	ra := ipv6AcceptanceRAFixture()
	lease, client, server := ipv6AcceptanceLeaseFixture()
	f.Add(ra)
	f.Add(lease)
	f.Add([]byte{0, 1, 0, 255})
	f.Fuzz(func(t *testing.T, wire []byte) {
		_ = productIPv6RA(wire, netip.MustParseAddr("fe80::1"), 255)
		_, _ = productIPv6Lease(wire, 7, [3]byte{4, 5, 6}, client, server)
	})
}

func TestProductIPv6RARequiresConfiguredPrefixAndLink(t *testing.T) {
	p := ipv6AcceptanceRAFixture()
	if err := productIPv6RA(p, netip.MustParseAddr("fe80::1"), 255); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		wire   []byte
		source netip.Addr
		hop    int
	}{
		{"no PIO", p[:16], netip.MustParseAddr("fe80::1"), 255},
		{"global source", p, netip.MustParseAddr("2001:db8::1"), 255},
		{"wrong hop", p, netip.MustParseAddr("fe80::1"), 64},
		{"short PIO", p[:47], netip.MustParseAddr("fe80::1"), 255},
	} {
		t.Run(test.name, func(t *testing.T) {
			if productIPv6RA(test.wire, test.source, test.hop) == nil {
				t.Fatal("invalid wire RA accepted")
			}
		})
	}
	wrong := bytes.Clone(p)
	wrong[37] = 0x21
	if productIPv6RA(wrong, netip.MustParseAddr("fe80::1"), 255) == nil {
		t.Fatal("another prefix accepted")
	}
	zero := bytes.Clone(p)
	zero[6], zero[7] = 0, 0
	if productIPv6RA(zero, netip.MustParseAddr("fe80::1"), 255) == nil {
		t.Fatal("final RA counted as live router")
	}
}

func ipv6AcceptanceLeaseFixture() ([]byte, []byte, []byte) {
	client := []byte{0, 3, 0, 1, 2, 0, 0, 0, 0, 3}
	server := []byte{0, 3, 0, 1, 2, 0, 0, 0, 0, 2}
	out := []byte{7, 4, 5, 6}
	out = productIPv6Option(out, 1, client)
	out = productIPv6Option(out, 2, server)
	na := make([]byte, 12)
	binary.BigEndian.PutUint32(na[:4], 1)
	binary.BigEndian.PutUint32(na[4:8], 300)
	binary.BigEndian.PutUint32(na[8:12], 600)
	addr := netip.MustParseAddr("2001:db8:20::2").As16()
	iaaddr := append([]byte{}, addr[:]...)
	iaaddr = append(iaaddr, 0, 0, 7, 8, 0, 0, 14, 16)
	na = productIPv6Option(na, 5, iaaddr)
	out = productIPv6Option(out, 3, na)
	pd := make([]byte, 12)
	binary.BigEndian.PutUint32(pd[:4], 2)
	binary.BigEndian.PutUint32(pd[4:8], 300)
	binary.BigEndian.PutUint32(pd[8:12], 600)
	prefix := []byte{0, 0, 7, 8, 0, 0, 14, 16, 64}
	a := netip.MustParseAddr("2001:db8:100::").As16()
	prefix = append(prefix, a[:]...)
	pd = productIPv6Option(pd, 26, prefix)
	out = productIPv6Option(out, 25, pd)
	return out, client, server
}

func TestProductIPv6DHCPRequiresBothAssociationsAndCorrelation(t *testing.T) {
	wire, client, server := ipv6AcceptanceLeaseFixture()
	lease, err := productIPv6Lease(wire, 7, [3]byte{4, 5, 6}, client, server)
	if err != nil || lease.Address.String() != "2001:db8:20::2" || lease.Prefix.String() != "2001:db8:100::/64" {
		t.Fatalf("lease=%+v err=%v", lease, err)
	}
	for _, test := range []struct {
		name           string
		wire           []byte
		client, server []byte
	}{
		{"no associations", wire[:32], client, server},
		{"truncated", wire[:len(wire)-1], client, server},
		{"wrong client", wire, []byte{0, 3, 0, 1, 9}, server},
		{"wrong server", wire, client, []byte{0, 3, 0, 1, 9}},
		{"duplicate server", productIPv6Option(bytes.Clone(wire), 2, server), client, server},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := productIPv6Lease(test.wire, 7, [3]byte{4, 5, 6}, test.client, test.server); err == nil {
				t.Fatal("uncorrelated/incomplete lease accepted")
			}
		})
	}
	wrong := bytes.Clone(wire)
	wrong[3]++
	if _, err := productIPv6Lease(wrong, 7, [3]byte{4, 5, 6}, client, server); err == nil {
		t.Fatal("wrong transaction accepted")
	}
	// Top-level status cannot turn refusal into a successful lease.
	refused := productIPv6Option(bytes.Clone(wire), 13, []byte{0, 6})
	if _, err := productIPv6Lease(refused, 7, [3]byte{4, 5, 6}, client, server); err == nil {
		t.Fatal("NoPrefixAvail accepted")
	}
}

func TestProductIPv6RouteRequiresExactSourcePathAndNextHop(t *testing.T) {
	fixture := "2001:db8:100::/64 fib:0 index:9\n  ze-pppoe refs:1 entry-flags:[]\n    path-list:[7] locks:1 flags:shared, uPRF-list:8 len:1 itfs:[1]\n      path:[3] pl-index:7 ip6 weight=1 pref=0 attached-nexthop: \n        fe80::2 pppoe_session0\n forwarding: unicast-ip6-chain\n  unrelated pppoe_session99\n"
	if err := productIPv6Route(fixture, netip.MustParsePrefix("2001:db8:100::/64"), "pppoe_session0", netip.MustParseAddr("fe80::2")); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(fixture, "  ze-pppoe refs:", "  api refs:", 1),
		strings.Replace(fixture, "fe80::2 pppoe_session0", "fe80::3 pppoe_session0", 1),
		strings.Replace(fixture, "fe80::2 pppoe_session0", "fe80::2 pppoe_session99", 1),
		strings.Replace(fixture, "ip6 weight=1", "ip4 weight=1", 1),
		fixture + fixture,
	} {
		if productIPv6Route(bad, netip.MustParsePrefix("2001:db8:100::/64"), "pppoe_session0", netip.MustParseAddr("fe80::2")) == nil {
			t.Fatal("unowned/mismatched IPv6 source-path accepted")
		}
	}
}

func TestProductIPv6ControlTAPCannotOwnAnyIPv6Address(t *testing.T) {
	if err := productIPv6NoTAPAddress(`[{"ifname":"cp0","addr_info":[]}]`, "cp0"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`null`, `[]`, `[{"ifname":"other","addr_info":[]}]`, `[{"ifname":"cp0","addr_info":[{"local":"fe80::1","family":"inet6"}]}]`, `[{"ifname":"cp0","addr_info":[{"local":"2001:db8::1","family":"inet6"}]}]`} {
		if productIPv6NoTAPAddress(bad, "cp0") == nil {
			t.Fatal("missing TAP/address leak accepted")
		}
	}
}

func TestProductFamilyGateRequiresIndependentCaptureControl(t *testing.T) {
	positive := "listening on eth0, link-type EN10MB\n1 packet captured\n1 packet received by filter\n0 packets dropped by kernel\n"
	empty := "listening on eth0, link-type EN10MB\n0 packets captured\n0 packets received by filter\n0 packets dropped by kernel\n"
	if err := productFamilyCapture(positive, true); err != nil {
		t.Fatal(err)
	}
	if err := productFamilyCapture(empty, false); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		output string
		want   bool
	}{{positive, false}, {empty, true}, {"", false}, {"1 packet captured", true}, {strings.Replace(positive, "0 packets dropped", "1 packets dropped", 1), true}, {positive + "1 packet captured\n", true}} {
		if productFamilyCapture(test.output, test.want) == nil {
			t.Fatal("uncontrolled or leaking independent capture accepted")
		}
	}
}
