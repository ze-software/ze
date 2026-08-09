// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- Translation contract
//
// Pure translation functions mapping ze firewall Match/Action types to
// VPP ACL rule fields. These functions contain no I/O and no references
// to api.Channel; the backend composes their outputs into actual binary
// API calls. Keeping translation pure lets us unit-test the wire-level
// parameters without a running VPP.

package firewallvpp

import (
	"fmt"
	"net/netip"

	"go.fd.io/govpp/binapi/acl_types"
	"go.fd.io/govpp/binapi/ip_types"

	"github.com/ze-software/ze/internal/component/firewall"
)

var protoNumbers = map[string]ip_types.IPProto{
	"tcp": ip_types.IP_API_PROTO_TCP, "udp": ip_types.IP_API_PROTO_UDP,
	"icmp": ip_types.IP_API_PROTO_ICMP, "icmpv6": ip_types.IP_API_PROTO_ICMP6,
	"sctp": ip_types.IP_API_PROTO_SCTP, "gre": ip_types.IP_API_PROTO_GRE,
	"esp": ip_types.IP_API_PROTO_ESP, "ah": ip_types.IP_API_PROTO_AH,
	"ospf": ip_types.IP_API_PROTO_OSPF, "vrrp": 112,
}

// translateTerm converts a ze Term into one or more VPP ACL rules.
// Most terms produce exactly one rule; the verifier guarantees that
// multi-range ports and unsupported expressions have been rejected
// before this point.
func translateTerm(term *firewall.Term) ([]acl_types.ACLRule, error) {
	rule := acl_types.ACLRule{
		SrcportOrIcmptypeFirst: 0,
		SrcportOrIcmptypeLast:  65535,
		DstportOrIcmpcodeFirst: 0,
		DstportOrIcmpcodeLast:  65535,
	}

	var hasVerdict bool
	var isStateful bool
	var isICMP bool
	var hasICMPType bool

	for _, m := range term.Matches {
		if err := applyMatch(&rule, m, &isStateful, &isICMP, &hasICMPType); err != nil {
			return nil, fmt.Errorf("term %q: %w", term.Name, err)
		}
	}

	for _, a := range term.Actions {
		switch a.(type) {
		case firewall.Accept:
			if isStateful {
				rule.IsPermit = acl_types.ACL_ACTION_API_PERMIT_REFLECT
			} else {
				rule.IsPermit = acl_types.ACL_ACTION_API_PERMIT
			}
			hasVerdict = true
		case firewall.Drop:
			rule.IsPermit = acl_types.ACL_ACTION_API_DENY
			hasVerdict = true
		case firewall.FlowOffload:
			// VPP IS the fast path; silently ignored.
		}
	}

	if !hasVerdict {
		return nil, fmt.Errorf("term %q: no verdict action (accept/drop) found", term.Name)
	}

	return []acl_types.ACLRule{rule}, nil
}

func applyMatch(rule *acl_types.ACLRule, m firewall.Match, isStateful, isICMP, hasICMPType *bool) error {
	switch v := m.(type) {
	case firewall.MatchSourceAddress:
		rule.SrcPrefix = prefixToVPP(v.Prefix)
	case firewall.MatchDestinationAddress:
		rule.DstPrefix = prefixToVPP(v.Prefix)
	case firewall.MatchProtocol:
		proto, ok := protoNumbers[v.Protocol]
		if !ok {
			return fmt.Errorf("unknown protocol %q", v.Protocol)
		}
		rule.Proto = proto
		if proto == ip_types.IP_API_PROTO_ICMP || proto == ip_types.IP_API_PROTO_ICMP6 {
			*isICMP = true
			if !*hasICMPType {
				rule.SrcportOrIcmptypeFirst = 0
				rule.SrcportOrIcmptypeLast = 255
				rule.DstportOrIcmpcodeFirst = 0
				rule.DstportOrIcmpcodeLast = 255
			}
		}
	case firewall.MatchSourcePort:
		if len(v.Ranges) == 1 {
			rule.SrcportOrIcmptypeFirst = v.Ranges[0].Lo
			rule.SrcportOrIcmptypeLast = v.Ranges[0].Hi
		}
	case firewall.MatchDestinationPort:
		if len(v.Ranges) == 1 {
			rule.DstportOrIcmpcodeFirst = v.Ranges[0].Lo
			rule.DstportOrIcmpcodeLast = v.Ranges[0].Hi
		}
	case firewall.MatchConnState:
		*isStateful = true
	case firewall.MatchICMPType:
		*isICMP = true
		*hasICMPType = true
		rule.Proto = ip_types.IP_API_PROTO_ICMP
		rule.SrcportOrIcmptypeFirst = uint16(v.Type)
		rule.SrcportOrIcmptypeLast = uint16(v.Type)
		rule.DstportOrIcmpcodeFirst = 0
		rule.DstportOrIcmpcodeLast = 255
	case firewall.MatchICMPv6Type:
		*isICMP = true
		*hasICMPType = true
		rule.Proto = ip_types.IP_API_PROTO_ICMP6
		rule.SrcportOrIcmptypeFirst = uint16(v.Type)
		rule.SrcportOrIcmptypeLast = uint16(v.Type)
		rule.DstportOrIcmpcodeFirst = 0
		rule.DstportOrIcmpcodeLast = 255
	case firewall.MatchTCPFlags:
		rule.Proto = ip_types.IP_API_PROTO_TCP
		rule.TCPFlagsValue = uint8(v.Flags)
		mask := uint8(v.Mask)
		if mask == 0 {
			mask = uint8(v.Flags)
		}
		rule.TCPFlagsMask = mask
	}
	return nil
}

// prefixToVPP converts a Go netip.Prefix to a VPP ip_types.Prefix.
func prefixToVPP(p netip.Prefix) ip_types.Prefix {
	addr := p.Masked().Addr()
	bits := p.Bits()
	if addr.Is4() {
		a4 := addr.As4()
		return ip_types.Prefix{
			Address: ip_types.Address{
				Af: ip_types.ADDRESS_IP4,
				Un: ip_types.AddressUnionIP4(ip_types.IP4Address(a4)),
			},
			Len: uint8(bits),
		}
	}
	a16 := addr.As16()
	return ip_types.Prefix{
		Address: ip_types.Address{
			Af: ip_types.ADDRESS_IP6,
			Un: ip_types.AddressUnionIP6(ip_types.IP6Address(a16)),
		},
		Len: uint8(bits),
	}
}

// chainToACLRules translates all terms in a chain to a flat list of
// VPP ACL rules. The chain's default policy becomes the final
// catch-all rule.
func chainToACLRules(ch *firewall.Chain) ([]acl_types.ACLRule, error) {
	var rules []acl_types.ACLRule
	for i := range ch.Terms {
		termRules, err := translateTerm(&ch.Terms[i])
		if err != nil {
			return nil, err
		}
		rules = append(rules, termRules...)
	}
	rules = append(rules, defaultPolicyRule(ch.Policy))
	return rules, nil
}

func defaultPolicyRule(p firewall.Policy) acl_types.ACLRule {
	action := acl_types.ACL_ACTION_API_DENY
	if p == firewall.PolicyAccept {
		action = acl_types.ACL_ACTION_API_PERMIT
	}
	return acl_types.ACLRule{
		IsPermit:               action,
		SrcportOrIcmptypeFirst: 0,
		SrcportOrIcmptypeLast:  65535,
		DstportOrIcmpcodeFirst: 0,
		DstportOrIcmpcodeLast:  65535,
	}
}
