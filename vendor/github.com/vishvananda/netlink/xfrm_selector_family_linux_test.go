package netlink

import (
	"bytes"
	"net"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// xfrmSelectorFamilyCases are selectors whose family follows the mask width.
var xfrmSelectorFamilyCases = []struct {
	name   string
	source net.IP
	target net.IP
	bits   int
	prefix int
	family uint16
	saddr  nl.XfrmAddress
	daddr  nl.XfrmAddress
}{
	{
		name: "ipv4", source: net.IP{198, 51, 100, 9}, target: net.IP{192, 0, 2, 4},
		bits: 32, prefix: 32, family: nl.FAMILY_V4,
		saddr: nl.XfrmAddress{198, 51, 100, 9}, daddr: nl.XfrmAddress{192, 0, 2, 4},
	},
	{
		name: "ipv4-in-sixteen-bytes", source: net.ParseIP("198.51.100.9"), target: net.ParseIP("192.0.2.4"),
		bits: 32, prefix: 32, family: nl.FAMILY_V4,
		saddr: nl.XfrmAddress{198, 51, 100, 9}, daddr: nl.XfrmAddress{192, 0, 2, 4},
	},
	{
		name: "native-ipv6", source: net.ParseIP("2001:db8:1::"), target: net.ParseIP("2001:db8:2::"),
		bits: 128, prefix: 64, family: nl.FAMILY_V6,
		saddr: nl.XfrmAddress{0x20, 1, 0x0d, 0xb8, 0, 1}, daddr: nl.XfrmAddress{0x20, 1, 0x0d, 0xb8, 0, 2},
	},
	{
		name: "mapped-ipv6-host", source: net.ParseIP("::ffff:198.51.100.9"), target: net.ParseIP("::ffff:192.0.2.4"),
		bits: 128, prefix: 128, family: nl.FAMILY_V6,
		saddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 198, 13: 51, 14: 100, 15: 9},
		daddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 192, 14: 2, 15: 4},
	},
	{
		name: "mapped-ipv6-network", source: net.ParseIP("::ffff:198.51.100.0"), target: net.ParseIP("::ffff:192.0.2.0"),
		bits: 128, prefix: 120, family: nl.FAMILY_V6,
		saddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 198, 13: 51, 14: 100},
		daddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 192, 14: 2},
	},
}

// TestXfrmSelectorFamilyFromMask checks the selector a policy and a transport
// state write: the family follows the destination mask width and an IPv6
// selector keeps all sixteen address bytes.
func TestXfrmSelectorFamilyFromMask(t *testing.T) {
	for _, tc := range xfrmSelectorFamilyCases {
		t.Run(tc.name, func(t *testing.T) {
			policy := &XfrmPolicy{
				Src:   &net.IPNet{IP: tc.source, Mask: net.CIDRMask(tc.prefix, tc.bits)},
				Dst:   &net.IPNet{IP: tc.target, Mask: net.CIDRMask(tc.prefix, tc.bits)},
				Proto: 17, SrcPort: 0, SrcPortMask: 0xffff, DstPort: 0x1200, DstPortMask: 0xff00,
				Ifindex: 9,
			}
			want := nl.XfrmSelector{
				Family: tc.family, Saddr: tc.saddr, Daddr: tc.daddr,
				PrefixlenS: uint8(tc.prefix), PrefixlenD: uint8(tc.prefix), Proto: 17,
				Sport: 0, SportMask: nl.Swap16(0xffff), Dport: nl.Swap16(0x1200), DportMask: nl.Swap16(0xff00),
				Ifindex: 9,
			}
			state := XfrmState{Src: tc.source, Dst: tc.target, Mode: XFRM_MODE_TRANSPORT, Selector: policy}
			msg := xfrmUsersaInfoFromXfrmState(&state)
			if !bytes.Equal(msg.Sel.Serialize(), want.Serialize()) {
				t.Fatalf("transport state selector: got %x, want %x", msg.Sel.Serialize(), want.Serialize())
			}
			var selector nl.XfrmSelector
			selFromPolicy(&selector, policy)
			if !bytes.Equal(selector.Serialize(), want.Serialize()) {
				t.Fatalf("policy selector: got %x, want %x", selector.Serialize(), want.Serialize())
			}
		})
	}
}

// TestXfrmSelectorFamilySourceOnly checks a selector with a source and no
// destination: the source mask width gives the family.
func TestXfrmSelectorFamilySourceOnly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source net.IP
		bits   int
		prefix int
		family uint16
		saddr  nl.XfrmAddress
	}{
		{
			name: "mapped-ipv6-host", source: net.ParseIP("::ffff:198.51.100.9"), bits: 128, prefix: 128, family: nl.FAMILY_V6,
			saddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 198, 13: 51, 14: 100, 15: 9},
		},
		{
			name: "native-ipv6", source: net.ParseIP("2001:db8:1::"), bits: 128, prefix: 64, family: nl.FAMILY_V6,
			saddr: nl.XfrmAddress{0x20, 1, 0x0d, 0xb8, 0, 1},
		},
		{
			name: "ipv4-in-sixteen-bytes", source: net.ParseIP("198.51.100.9"), bits: 32, prefix: 32, family: nl.FAMILY_V4,
			saddr: nl.XfrmAddress{198, 51, 100, 9},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := &XfrmPolicy{Src: &net.IPNet{IP: tc.source, Mask: net.CIDRMask(tc.prefix, tc.bits)}}
			want := nl.XfrmSelector{Family: tc.family, Saddr: tc.saddr, PrefixlenS: uint8(tc.prefix)}
			var got nl.XfrmSelector
			selFromPolicy(&got, policy)
			if !bytes.Equal(got.Serialize(), want.Serialize()) {
				t.Fatalf("got %x, want %x", got.Serialize(), want.Serialize())
			}
		})
	}
}

// TestXfrmSelectorFamilySixteenByteIPv4 checks an IPv4 address held in sixteen
// bytes: a 32-bit mask writes an IPv4 prefix and a 128-bit mask an IPv6 one, so
// no selector is written as IPv4 with a prefix length above 32, which Linux
// refuses.
func TestXfrmSelectorFamilySixteenByteIPv4(t *testing.T) {
	v4 := nl.XfrmAddress{192, 0, 2, 4}
	mapped := nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 192, 14: 2, 15: 4}
	for _, tc := range []struct {
		name   string
		bits   int
		prefix int
		family uint16
		daddr  nl.XfrmAddress
	}{
		{name: "32-bit mask, prefix 32", bits: 32, prefix: 32, family: nl.FAMILY_V4, daddr: v4},
		{name: "128-bit mask, prefix 33", bits: 128, prefix: 33, family: nl.FAMILY_V6, daddr: mapped},
		{name: "128-bit mask, prefix 120", bits: 128, prefix: 120, family: nl.FAMILY_V6, daddr: mapped},
		{name: "128-bit mask, prefix 128", bits: 128, prefix: 128, family: nl.FAMILY_V6, daddr: mapped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := &XfrmPolicy{Dst: &net.IPNet{IP: net.ParseIP("192.0.2.4"), Mask: net.CIDRMask(tc.prefix, tc.bits)}}
			want := nl.XfrmSelector{Family: tc.family, Daddr: tc.daddr, PrefixlenD: uint8(tc.prefix)}
			var got nl.XfrmSelector
			selFromPolicy(&got, policy)
			if !bytes.Equal(got.Serialize(), want.Serialize()) {
				t.Fatalf("got %x, want %x", got.Serialize(), want.Serialize())
			}
			if got.Family == nl.FAMILY_V4 && got.PrefixlenD > 32 {
				t.Fatalf("IPv4 selector with prefix length %d", got.PrefixlenD)
			}
		})
	}
}
