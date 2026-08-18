package firewallvpp

import (
	"net/netip"
	"testing"

	"go.fd.io/govpp/binapi/acl_types"
	"go.fd.io/govpp/binapi/ip_types"

	"github.com/ze-software/ze/internal/component/firewall"
)

func TestTranslateTermAcceptDrop(t *testing.T) {
	term := firewall.Term{
		Name:    "allow-all",
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].IsPermit != acl_types.ACL_ACTION_API_PERMIT {
		t.Errorf("expected PERMIT, got %v", rules[0].IsPermit)
	}

	term.Name = "deny-all"
	term.Actions = []firewall.Action{firewall.Drop{}}
	rules, err = translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].IsPermit != acl_types.ACL_ACTION_API_DENY {
		t.Errorf("expected DENY, got %v", rules[0].IsPermit)
	}
}

func TestTranslateTermStatefulAccept(t *testing.T) {
	term := firewall.Term{
		Name: "allow-established",
		Matches: []firewall.Match{
			firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].IsPermit != acl_types.ACL_ACTION_API_PERMIT_REFLECT {
		t.Errorf("expected PERMIT_REFLECT, got %v", rules[0].IsPermit)
	}
}

func TestTranslateTermSrcDstAddress(t *testing.T) {
	term := firewall.Term{
		Name: "src-dst",
		Matches: []firewall.Match{
			firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
			firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("192.168.1.0/24")},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := rules[0]
	if r.SrcPrefix.Len != 8 {
		t.Errorf("src prefix len: want 8, got %d", r.SrcPrefix.Len)
	}
	if r.SrcPrefix.Address.Af != ip_types.ADDRESS_IP4 {
		t.Errorf("src address family: want IP4, got %v", r.SrcPrefix.Address.Af)
	}
	if r.DstPrefix.Len != 24 {
		t.Errorf("dst prefix len: want 24, got %d", r.DstPrefix.Len)
	}
}

func TestTranslateTermIPv6Address(t *testing.T) {
	term := firewall.Term{
		Name: "ipv6",
		Matches: []firewall.Match{
			firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("2001:db8::/32")},
		},
		Actions: []firewall.Action{firewall.Drop{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].SrcPrefix.Address.Af != ip_types.ADDRESS_IP6 {
		t.Errorf("want IP6, got %v", rules[0].SrcPrefix.Address.Af)
	}
	if rules[0].SrcPrefix.Len != 32 {
		t.Errorf("want len 32, got %d", rules[0].SrcPrefix.Len)
	}
}

func TestTranslateTermProtocol(t *testing.T) {
	term := firewall.Term{
		Name: "tcp-only",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "tcp"},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].Proto != ip_types.IP_API_PROTO_TCP {
		t.Errorf("want TCP (6), got %v", rules[0].Proto)
	}
}

func TestTranslateTermUnknownProtocol(t *testing.T) {
	term := firewall.Term{
		Name: "bad-proto",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "nonexistent"},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	_, err := translateTerm(&term)
	if err == nil {
		t.Fatal("expected error for unknown protocol")
	}
}

func TestTranslateTermPorts(t *testing.T) {
	term := firewall.Term{
		Name: "ssh",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "tcp"},
			firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := rules[0]
	if r.DstportOrIcmpcodeFirst != 22 || r.DstportOrIcmpcodeLast != 22 {
		t.Errorf("dst port: want 22-22, got %d-%d", r.DstportOrIcmpcodeFirst, r.DstportOrIcmpcodeLast)
	}
}

func TestTranslateTermPortRange(t *testing.T) {
	term := firewall.Term{
		Name: "high-ports",
		Matches: []firewall.Match{
			firewall.MatchSourcePort{Ranges: []firewall.PortRange{{Lo: 1024, Hi: 65535}}},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := rules[0]
	if r.SrcportOrIcmptypeFirst != 1024 || r.SrcportOrIcmptypeLast != 65535 {
		t.Errorf("src port: want 1024-65535, got %d-%d", r.SrcportOrIcmptypeFirst, r.SrcportOrIcmptypeLast)
	}
}

func TestTranslateTermICMPType(t *testing.T) {
	term := firewall.Term{
		Name: "echo-request",
		Matches: []firewall.Match{
			firewall.MatchICMPType{Type: 8},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := rules[0]
	if r.Proto != ip_types.IP_API_PROTO_ICMP {
		t.Errorf("want ICMP proto, got %v", r.Proto)
	}
	if r.SrcportOrIcmptypeFirst != 8 || r.SrcportOrIcmptypeLast != 8 {
		t.Errorf("icmp type: want 8-8, got %d-%d", r.SrcportOrIcmptypeFirst, r.SrcportOrIcmptypeLast)
	}
	if r.DstportOrIcmpcodeFirst != 0 || r.DstportOrIcmpcodeLast != 255 {
		t.Errorf("icmp code: want 0-255, got %d-%d", r.DstportOrIcmpcodeFirst, r.DstportOrIcmpcodeLast)
	}
}

func TestTranslateTermICMPv6Type(t *testing.T) {
	term := firewall.Term{
		Name: "neighbor-solicit",
		Matches: []firewall.Match{
			firewall.MatchICMPv6Type{Type: 135},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].Proto != ip_types.IP_API_PROTO_ICMP6 {
		t.Errorf("want ICMPv6 proto, got %v", rules[0].Proto)
	}
	if rules[0].SrcportOrIcmptypeFirst != 135 {
		t.Errorf("want type 135, got %d", rules[0].SrcportOrIcmptypeFirst)
	}
}

func TestTranslateTermTCPFlags(t *testing.T) {
	term := firewall.Term{
		Name: "syn-only",
		Matches: []firewall.Match{
			firewall.MatchTCPFlags{Flags: firewall.TCPFlagSYN, Mask: firewall.TCPFlagSYN | firewall.TCPFlagACK},
		},
		Actions: []firewall.Action{firewall.Drop{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := rules[0]
	if r.Proto != ip_types.IP_API_PROTO_TCP {
		t.Errorf("want TCP proto, got %v", r.Proto)
	}
	if r.TCPFlagsValue != uint8(firewall.TCPFlagSYN) {
		t.Errorf("want SYN flag value, got %d", r.TCPFlagsValue)
	}
	if r.TCPFlagsMask != uint8(firewall.TCPFlagSYN|firewall.TCPFlagACK) {
		t.Errorf("want SYN|ACK mask, got %d", r.TCPFlagsMask)
	}
}

func TestTranslateTermNoVerdict(t *testing.T) {
	term := firewall.Term{
		Name:    "no-verdict",
		Actions: []firewall.Action{firewall.FlowOffload{FlowtableName: "ft"}},
	}
	_, err := translateTerm(&term)
	if err == nil {
		t.Fatal("expected error for term with no verdict")
	}
}

func TestTranslateTermFlowOffloadIgnored(t *testing.T) {
	term := firewall.Term{
		Name: "with-offload",
		Actions: []firewall.Action{
			firewall.FlowOffload{FlowtableName: "ft"},
			firewall.Accept{},
		},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].IsPermit != acl_types.ACL_ACTION_API_PERMIT {
		t.Errorf("expected PERMIT, got %v", rules[0].IsPermit)
	}
}

func TestChainToACLRules(t *testing.T) {
	ch := firewall.Chain{
		Name:   "input",
		IsBase: true,
		Type:   firewall.ChainFilter,
		Hook:   firewall.HookInput,
		Policy: firewall.PolicyDrop,
		Terms: []firewall.Term{
			{
				Name:    "allow-ssh",
				Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}}},
				Actions: []firewall.Action{firewall.Accept{}},
			},
		},
	}
	rules, err := chainToACLRules(&ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules (1 term + default policy), got %d", len(rules))
	}
	if rules[0].IsPermit != acl_types.ACL_ACTION_API_PERMIT {
		t.Errorf("first rule: want PERMIT, got %v", rules[0].IsPermit)
	}
	if rules[1].IsPermit != acl_types.ACL_ACTION_API_DENY {
		t.Errorf("default policy rule: want DENY, got %v", rules[1].IsPermit)
	}
}

func TestDefaultPolicyRule(t *testing.T) {
	accept := defaultPolicyRule(firewall.PolicyAccept)
	if accept.IsPermit != acl_types.ACL_ACTION_API_PERMIT {
		t.Errorf("accept policy: want PERMIT, got %v", accept.IsPermit)
	}
	drop := defaultPolicyRule(firewall.PolicyDrop)
	if drop.IsPermit != acl_types.ACL_ACTION_API_DENY {
		t.Errorf("drop policy: want DENY, got %v", drop.IsPermit)
	}
	if drop.SrcportOrIcmptypeFirst != 0 || drop.SrcportOrIcmptypeLast != 65535 {
		t.Error("default rule should match all ports")
	}
}

func TestPrefixToVPPIPv4(t *testing.T) {
	p := prefixToVPP(netip.MustParsePrefix("192.168.1.0/24"))
	if p.Address.Af != ip_types.ADDRESS_IP4 {
		t.Errorf("want IP4, got %v", p.Address.Af)
	}
	if p.Len != 24 {
		t.Errorf("want len 24, got %d", p.Len)
	}
}

func TestPrefixToVPPIPv6(t *testing.T) {
	p := prefixToVPP(netip.MustParsePrefix("fd00::/64"))
	if p.Address.Af != ip_types.ADDRESS_IP6 {
		t.Errorf("want IP6, got %v", p.Address.Af)
	}
	if p.Len != 64 {
		t.Errorf("want len 64, got %d", p.Len)
	}
}

func TestTranslateTermTCPFlagsMaskZeroDefaultsToFlags(t *testing.T) {
	term := firewall.Term{
		Name: "syn-mask-zero",
		Matches: []firewall.Match{
			firewall.MatchTCPFlags{Flags: firewall.TCPFlagSYN, Mask: 0},
		},
		Actions: []firewall.Action{firewall.Drop{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].TCPFlagsMask != uint8(firewall.TCPFlagSYN) {
		t.Errorf("mask=0 should default to flags value %d, got %d",
			firewall.TCPFlagSYN, rules[0].TCPFlagsMask)
	}
}

func TestTranslateTermICMPProtocolWithoutType(t *testing.T) {
	term := firewall.Term{
		Name: "icmp-all",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "icmp"},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	}
	rules, err := translateTerm(&term)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := rules[0]
	if r.Proto != ip_types.IP_API_PROTO_ICMP {
		t.Errorf("want ICMP, got %v", r.Proto)
	}
	if r.SrcportOrIcmptypeFirst != 0 || r.SrcportOrIcmptypeLast != 255 {
		t.Errorf("icmp type range: want 0-255, got %d-%d",
			r.SrcportOrIcmptypeFirst, r.SrcportOrIcmptypeLast)
	}
	if r.DstportOrIcmpcodeFirst != 0 || r.DstportOrIcmpcodeLast != 255 {
		t.Errorf("icmp code range: want 0-255, got %d-%d",
			r.DstportOrIcmpcodeFirst, r.DstportOrIcmpcodeLast)
	}
}

func TestChainToACLRulesAcceptPolicy(t *testing.T) {
	ch := firewall.Chain{
		Name:   "output",
		IsBase: true,
		Type:   firewall.ChainFilter,
		Hook:   firewall.HookOutput,
		Policy: firewall.PolicyAccept,
		Terms: []firewall.Term{{
			Name:    "deny-bad",
			Actions: []firewall.Action{firewall.Drop{}},
		}},
	}
	rules, err := chainToACLRules(&ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(rules))
	}
	if rules[1].IsPermit != acl_types.ACL_ACTION_API_PERMIT {
		t.Errorf("default accept policy: want PERMIT, got %v", rules[1].IsPermit)
	}
}

func TestChainToACLRulesMultipleTerms(t *testing.T) {
	ch := firewall.Chain{
		Name:   "input",
		IsBase: true,
		Type:   firewall.ChainFilter,
		Hook:   firewall.HookInput,
		Policy: firewall.PolicyDrop,
		Terms: []firewall.Term{
			{Name: "allow-ssh", Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}}}, Actions: []firewall.Action{firewall.Accept{}}},
			{Name: "allow-dns", Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "udp"}, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 53, Hi: 53}}}}, Actions: []firewall.Action{firewall.Accept{}}},
			{Name: "allow-bgp", Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 179, Hi: 179}}}}, Actions: []firewall.Action{firewall.Accept{}}},
		},
	}
	rules, err := chainToACLRules(&ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 4 {
		t.Fatalf("want 4 rules (3 terms + default policy), got %d", len(rules))
	}
	for i := range 3 {
		if rules[i].IsPermit != acl_types.ACL_ACTION_API_PERMIT {
			t.Errorf("rule %d: want PERMIT, got %v", i, rules[i].IsPermit)
		}
	}
	if rules[3].IsPermit != acl_types.ACL_ACTION_API_DENY {
		t.Errorf("default policy: want DENY, got %v", rules[3].IsPermit)
	}
}

func TestTranslateTermAllProtocols(t *testing.T) {
	protocols := map[string]ip_types.IPProto{
		"tcp": ip_types.IP_API_PROTO_TCP, "udp": ip_types.IP_API_PROTO_UDP,
		"icmp": ip_types.IP_API_PROTO_ICMP, "icmpv6": ip_types.IP_API_PROTO_ICMP6,
		"sctp": ip_types.IP_API_PROTO_SCTP, "gre": ip_types.IP_API_PROTO_GRE,
		"esp": ip_types.IP_API_PROTO_ESP, "ah": ip_types.IP_API_PROTO_AH,
		"ospf": ip_types.IP_API_PROTO_OSPF, "vrrp": 112,
	}
	for name, want := range protocols {
		t.Run(name, func(t *testing.T) {
			term := firewall.Term{
				Name:    name,
				Matches: []firewall.Match{firewall.MatchProtocol{Protocol: name}},
				Actions: []firewall.Action{firewall.Accept{}},
			}
			rules, err := translateTerm(&term)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rules[0].Proto != want {
				t.Errorf("want %v, got %v", want, rules[0].Proto)
			}
		})
	}
}

// TestTranslateTermEveryCanonicalProtocol is the completeness ratchet beside
// TestTranslateTermAllProtocols, which walks its own literal list and so cannot
// see a name the canonical table gains.
//
// VALIDATES: every name firewall.ProtocolNames returns translates to a VPP ACL
// rule without error.
// PREVENTS: this backend knowing a different set of protocol names from nft and
// from the YANG enum. It kept a private name -> IPProto table until this spec;
// the FlowSpec bridge kept another, and that one knew five of the ten.
func TestTranslateTermEveryCanonicalProtocol(t *testing.T) {
	for _, name := range firewall.ProtocolNames() {
		t.Run(name, func(t *testing.T) {
			term := firewall.Term{
				Name:    name,
				Matches: []firewall.Match{firewall.MatchProtocol{Protocol: name}},
				Actions: []firewall.Action{firewall.Accept{}},
			}
			rules, err := translateTerm(&term)
			if err != nil {
				t.Fatalf("canonical protocol %q must translate: %v", name, err)
			}
			num, ok := firewall.ProtocolNumber(name)
			if !ok {
				t.Fatalf("ProtocolNumber(%q) = ok false, but the name came from ProtocolNames", name)
			}
			if rules[0].Proto != ip_types.IPProto(num) {
				t.Errorf("protocol %q programmed as %v, want the IANA number %d", name, rules[0].Proto, num)
			}
		})
	}
}
