// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- selector family encoding.
// Related: xfrm_policy_linux.go -- selFromPolicy; xfrm_state_linux.go -- xfrmUsersaInfoFromXfrmState.
// Upstream: github.com/vishvananda/netlink v1.3.1, https://github.com/vishvananda/netlink.

package netlink

import (
	"bytes"
	"net"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

func TestZeXfrmSelectorUsesPrefixFamily(t *testing.T) {
	cases := []struct {
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
	for _, tc := range cases {
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
			scope := XfrmStateScope{Selectors: []nl.XfrmSelector{want}}
			var payload [8 + nl.SizeofXfrmSelector]byte
			if _, err := scope.WriteTo(payload[:], 0); err != nil {
				t.Fatalf("canonical scope rejected: %v", err)
			}
			state := XfrmState{Src: tc.source, Dst: tc.target, Mode: XFRM_MODE_TRANSPORT, Selector: policy}
			msg := xfrmUsersaInfoFromXfrmState(&state)
			if !bytes.Equal(msg.Sel.Serialize(), payload[8:]) {
				t.Fatalf("transport selector differs from canonical scope: got %x, want %x", msg.Sel.Serialize(), payload[8:])
			}
			var selector nl.XfrmSelector
			selFromPolicy(&selector, policy)
			if !bytes.Equal(selector.Serialize(), payload[8:]) {
				t.Fatalf("policy selector differs from canonical scope: got %x, want %x", selector.Serialize(), payload[8:])
			}
		})
	}
}
