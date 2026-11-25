package netlink

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// 2001:db8:: and 2001:db9:: end in twelve zero bytes, which a reader that takes
// the family from the address bytes decodes as 32.1.13.184 and 32.1.13.185.
var (
	xfrmReadbackV6Dst = nl.XfrmAddress{0x20, 0x01, 0x0d, 0xb8}
	xfrmReadbackV6Src = nl.XfrmAddress{0x20, 0x01, 0x0d, 0xb9}
)

// TestXfrmPolicyReadbackSelectorFamily reads a policy from an all-family dump:
// its prefixes take the selector's family, not the family the caller asked for.
func TestXfrmPolicyReadbackSelectorFamily(t *testing.T) {
	info := nl.XfrmUserpolicyInfo{}
	info.Sel.Family = nl.FAMILY_V6
	info.Sel.Daddr = xfrmReadbackV6Dst
	info.Sel.PrefixlenD = 32

	policy, err := parseXfrmPolicy(info.Serialize(), FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.Dst.String(); got != "2001:db8::/32" {
		t.Errorf("Dst = %s, want 2001:db8::/32", got)
	}
	if got := policy.Src.String(); got != "::/0" {
		t.Errorf("Src = %s, want ::/0", got)
	}
}

// TestXfrmPolicyReadbackTemplateFamily reads an IPv4 policy whose template has
// IPv6 tunnel endpoints: each address takes the family of the struct that holds it.
func TestXfrmPolicyReadbackTemplateFamily(t *testing.T) {
	info := nl.XfrmUserpolicyInfo{}
	info.Sel.Family = nl.FAMILY_V4
	info.Sel.Daddr = nl.XfrmAddress{192, 0, 2, 0}
	info.Sel.PrefixlenD = 24
	tmpl := nl.XfrmUserTmpl{Family: nl.FAMILY_V6}
	tmpl.XfrmId.Daddr = xfrmReadbackV6Dst
	tmpl.Saddr = xfrmReadbackV6Src
	m := append(info.Serialize(), nl.NewRtAttr(nl.XFRMA_TMPL, tmpl.Serialize()).Serialize()...)

	policy, err := parseXfrmPolicy(m, FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.Dst.String(); got != "192.0.2.0/24" {
		t.Errorf("Dst = %s, want 192.0.2.0/24", got)
	}
	if len(policy.Dst.IP) != net.IPv4len {
		t.Errorf("Dst holds %d bytes, want %d", len(policy.Dst.IP), net.IPv4len)
	}
	if len(policy.Tmpls) != 1 {
		t.Fatalf("got %d templates, want 1", len(policy.Tmpls))
	}
	if got := policy.Tmpls[0].Dst.String(); got != "2001:db8::" {
		t.Errorf("template Dst = %s, want 2001:db8::", got)
	}
	if got := policy.Tmpls[0].Src.String(); got != "2001:db9::" {
		t.Errorf("template Src = %s, want 2001:db9::", got)
	}
}

// TestXfrmStateReadbackAddressFamily reads states whose outer family differs
// from their selector family: the outer addresses take the message family and
// the selector prefixes take the selector family.
func TestXfrmStateReadbackAddressFamily(t *testing.T) {
	t.Run("ipv4 in ipv6", func(t *testing.T) {
		msg := nl.XfrmUsersaInfo{Family: nl.FAMILY_V6}
		msg.Id.Daddr = xfrmReadbackV6Dst
		msg.Saddr = xfrmReadbackV6Src
		msg.Sel.Family = nl.FAMILY_V4
		msg.Sel.Daddr = nl.XfrmAddress{192, 0, 2, 0}
		msg.Sel.Saddr = nl.XfrmAddress{198, 51, 100, 0}
		msg.Sel.PrefixlenD = 24
		msg.Sel.PrefixlenS = 24

		state, err := parseXfrmState(msg.Serialize(), FAMILY_ALL)
		if err != nil {
			t.Fatal(err)
		}
		if got := state.Dst.String(); got != "2001:db8::" {
			t.Errorf("Dst = %s, want 2001:db8::", got)
		}
		if got := state.Src.String(); got != "2001:db9::" {
			t.Errorf("Src = %s, want 2001:db9::", got)
		}
		if got := state.Selector.Dst.String(); got != "192.0.2.0/24" {
			t.Errorf("selector Dst = %s, want 192.0.2.0/24", got)
		}
		if got := state.Selector.Src.String(); got != "198.51.100.0/24" {
			t.Errorf("selector Src = %s, want 198.51.100.0/24", got)
		}
	})
	t.Run("ipv6 in ipv4", func(t *testing.T) {
		msg := nl.XfrmUsersaInfo{Family: nl.FAMILY_V4}
		msg.Id.Daddr = nl.XfrmAddress{192, 0, 2, 1}
		msg.Saddr = nl.XfrmAddress{198, 51, 100, 1}
		msg.Sel.Family = nl.FAMILY_V6
		msg.Sel.Daddr = xfrmReadbackV6Dst
		msg.Sel.Saddr = xfrmReadbackV6Src
		msg.Sel.PrefixlenD = 32
		msg.Sel.PrefixlenS = 32

		state, err := parseXfrmState(msg.Serialize(), FAMILY_ALL)
		if err != nil {
			t.Fatal(err)
		}
		if got := state.Dst.String(); got != "192.0.2.1" {
			t.Errorf("Dst = %s, want 192.0.2.1", got)
		}
		if got := state.Selector.Dst.String(); got != "2001:db8::/32" {
			t.Errorf("selector Dst = %s, want 2001:db8::/32", got)
		}
		if got := state.Selector.Src.String(); got != "2001:db9::/32" {
			t.Errorf("selector Src = %s, want 2001:db9::/32", got)
		}
	})
}
