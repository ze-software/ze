// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- Commit-time rejection matrix

package firewallvpp

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// aclTagMaxLen is VPP's string[64] limit on ACL tags. The backend uses
// the format "ze/<table>/<chain>" as the tag; if the resulting tag would
// exceed this, two chains could truncate to the same tag. Reject at
// verify so the operator picks shorter names.
const aclTagMaxLen = 64

// Verify walks the parsed desired state and rejects expression types
// that the VPP ACL backend cannot represent faithfully. Registered via
// firewall.RegisterVerifier("vpp", Verify); runs in OnConfigVerify
// after ValidateTables passes, so operators see rejections at commit
// time before the backend loads.
//
// Accepted matches: MatchSourceAddress, MatchDestinationAddress,
// MatchSourcePort (single range only), MatchDestinationPort (single
// range only), MatchProtocol, MatchConnState, MatchICMPType,
// MatchICMPv6Type, MatchTCPFlags.
//
// Accepted actions: Accept, Drop.
//
// FlowOffload is silently accepted (VPP IS the fast path).
//
// Everything else is rejected with a specific error naming the
// unsupported type and the reason. Errors from every table are
// collected via errors.Join so the operator sees all issues in one
// commit attempt.
func Verify(desired []firewall.Table) error {
	var errs []error
	for i := range desired {
		if err := verifyTable(&desired[i]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func verifyTable(tbl *firewall.Table) error {
	var errs []error
	if len(tbl.Sets) > 0 {
		errs = append(errs, fmt.Errorf("table %q: named sets not supported by backend vpp (VPP ACL plugin has no set equivalent)", tbl.Name))
	}
	if len(tbl.Flowtables) > 0 {
		errs = append(errs, fmt.Errorf("table %q: flowtables not supported by backend vpp", tbl.Name))
	}
	for i := range tbl.Chains {
		if err := verifyChain(tbl, &tbl.Chains[i]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func verifyChain(tbl *firewall.Table, ch *firewall.Chain) error {
	tag := aclTag(tbl.Name, ch.Name)
	if len(tag) > aclTagMaxLen {
		return fmt.Errorf("table %q chain %q: ACL tag %q exceeds VPP's %d-byte limit; shorten table or chain name",
			tbl.Name, ch.Name, tag, aclTagMaxLen)
	}
	if !ch.IsBase {
		return fmt.Errorf("table %q chain %q: non-base chains (jump/goto targets) not supported by backend vpp (VPP ACL is flat rules, no chain traversal)",
			tbl.Name, ch.Name)
	}
	if ch.Type == firewall.ChainNAT {
		return verifyNATChain(tbl, ch)
	}
	var errs []error
	for i := range ch.Terms {
		if err := verifyTerm(tbl, ch, &ch.Terms[i]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// verifyNATChain validates terms in a NAT chain for VPP NAT44-ED
// compatibility. NAT chains use VPP's NAT44-ED plugin instead of ACLs.
//
// Accepted NAT actions: SNAT, DNAT, Masquerade.
// Accepted matches in NAT terms: protocol, destination port, destination
// address (these map to static mapping fields). Source address matches
// are accepted but ignored (VPP NAT44 is interface-level, not per-source).
//
// SNAT and Masquerade only accept terms with no match constraints or
// source-only matches (VPP NAT44 applies to all traffic on the interface).
func verifyNATChain(tbl *firewall.Table, ch *firewall.Chain) error {
	var errs []error
	for i := range ch.Terms {
		term := &ch.Terms[i]
		tag := natTag(tbl.Name, ch.Name, term.Name)
		if len(tag) > aclTagMaxLen {
			errs = append(errs, fmt.Errorf("table %q chain %q term %q: NAT tag %q exceeds VPP's %d-byte limit; shorten names",
				tbl.Name, ch.Name, term.Name, tag, aclTagMaxLen))
			continue
		}
		prefix := fmt.Sprintf("table %q chain %q term %q", tbl.Name, ch.Name, term.Name)
		if err := verifyNATTerm(prefix, term); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func verifyNATTerm(prefix string, term *firewall.Term) error {
	var errs []error
	var hasNATAction bool
	var hasSrcNAT, hasDstNAT bool
	for _, a := range term.Actions {
		switch v := a.(type) {
		case firewall.SNAT:
			hasNATAction = true
			hasSrcNAT = true
			if err := verifyNATAddrFamily(prefix, "snat", v.Address, v.AddressEnd); err != nil {
				errs = append(errs, err)
			}
		case firewall.Masquerade:
			hasNATAction = true
			hasSrcNAT = true
			if v.Port != 0 || v.PortEnd != 0 {
				errs = append(errs, fmt.Errorf("%s: masquerade port mapping not supported by backend vpp", prefix))
			}
			if v.Flags != 0 {
				errs = append(errs, fmt.Errorf("%s: masquerade flags not supported by backend vpp", prefix))
			}
		case firewall.DNAT:
			hasNATAction = true
			hasDstNAT = true
			if err := verifyNATAddrFamily(prefix, "dnat", v.Address, v.AddressEnd); err != nil {
				errs = append(errs, err)
			}
		case firewall.Accept, firewall.Drop, firewall.FlowOffload:
			// Verdict actions are valid alongside NAT actions.
		default:
			errs = append(errs, fmt.Errorf("%s: action %T not supported in NAT chain by backend vpp", prefix, a))
		}
	}
	if !hasNATAction {
		errs = append(errs, fmt.Errorf("%s: NAT chain term has no NAT action (snat/dnat/masquerade)", prefix))
	}
	if hasSrcNAT {
		if err := verifyNATSourceMatches(prefix, term); err != nil {
			errs = append(errs, err)
		}
	}
	if hasDstNAT {
		if err := verifyNATDestMatches(prefix, term); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func verifyNATSourceMatches(prefix string, term *firewall.Term) error {
	for _, m := range term.Matches {
		switch m.(type) {
		case firewall.MatchSourceAddress:
			// Accepted but informational: VPP NAT44 is interface-level.
		case firewall.MatchDestinationPort, firewall.MatchDestinationAddress,
			firewall.MatchProtocol:
			return fmt.Errorf("%s: destination/protocol match not supported with snat/masquerade by backend vpp (VPP NAT44 applies to all traffic on inside interfaces)", prefix)
		default:
			return fmt.Errorf("%s: match %T not supported in NAT chain by backend vpp", prefix, m)
		}
	}
	return nil
}

func verifyNATDestMatches(prefix string, term *firewall.Term) error {
	var hasProto, hasPort bool
	for _, m := range term.Matches {
		switch m.(type) {
		case firewall.MatchProtocol:
			hasProto = true
		case firewall.MatchDestinationPort:
			hasPort = true
		case firewall.MatchDestinationAddress, firewall.MatchSourceAddress:
			// Accepted: destination/source address narrow the mapping scope.
		default:
			return fmt.Errorf("%s: match %T not supported with dnat by backend vpp (only protocol, destination port/address, source address)", prefix, m)
		}
	}
	if !hasProto {
		return fmt.Errorf("%s: dnat requires a protocol match (VPP NAT44 static mapping needs an L4 protocol)", prefix)
	}
	if !hasPort {
		return fmt.Errorf("%s: dnat requires a destination port match (VPP NAT44 static mapping needs an external port)", prefix)
	}
	return nil
}

func verifyNATAddrFamily(prefix, action string, addrs ...netip.Addr) error {
	for _, a := range addrs {
		if a.IsValid() && a.Is6() {
			return fmt.Errorf("%s: %s address %s is IPv6, but VPP NAT44-ED only supports IPv4", prefix, action, a)
		}
	}
	return nil
}

func verifyTerm(tbl *firewall.Table, ch *firewall.Chain, term *firewall.Term) error {
	prefix := fmt.Sprintf("table %q chain %q term %q", tbl.Name, ch.Name, term.Name)
	var errs []error
	for _, m := range term.Matches {
		if err := verifyMatch(prefix, m); err != nil {
			errs = append(errs, err)
		}
	}
	for _, a := range term.Actions {
		if err := verifyAction(prefix, a); err != nil {
			errs = append(errs, err)
		}
	}
	if termUsesClassify(term) {
		polName := classifyPolicerName(tbl.Name, ch.Name, term.Name)
		if len(polName) > aclTagMaxLen {
			errs = append(errs, fmt.Errorf("%s: classify policer name %q exceeds VPP's %d-byte limit; shorten names",
				prefix, polName, aclTagMaxLen))
		}
		if err := verifyClassifyConstraints(prefix, term); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func termUsesClassify(term *firewall.Term) bool {
	for _, a := range term.Actions {
		switch a.(type) {
		case firewall.SetMark, firewall.Limit:
			return true
		}
	}
	return false
}

func verifyClassifyConstraints(prefix string, term *firewall.Term) error {
	var hasMark, hasLimit bool
	for _, a := range term.Actions {
		switch a.(type) {
		case firewall.SetMark:
			hasMark = true
		case firewall.Limit:
			hasLimit = true
		}
	}
	if hasMark && hasLimit {
		return fmt.Errorf("%s: set-mark and limit cannot be combined in the same term by backend vpp (VPP classify session key collision; use separate terms)", prefix)
	}
	var errs []error
	for _, m := range term.Matches {
		switch v := m.(type) {
		case firewall.MatchSourcePort:
			if len(v.Ranges) > 0 && v.Ranges[0].Lo != v.Ranges[0].Hi {
				errs = append(errs, fmt.Errorf("%s: source port range %d-%d not supported with set-mark/limit by backend vpp (VPP classify needs exact port; use separate terms)", prefix, v.Ranges[0].Lo, v.Ranges[0].Hi))
			}
		case firewall.MatchDestinationPort:
			if len(v.Ranges) > 0 && v.Ranges[0].Lo != v.Ranges[0].Hi {
				errs = append(errs, fmt.Errorf("%s: destination port range %d-%d not supported with set-mark/limit by backend vpp (VPP classify needs exact port; use separate terms)", prefix, v.Ranges[0].Lo, v.Ranges[0].Hi))
			}
		case firewall.MatchSourceAddress:
			if v.Prefix.Addr().Is6() {
				errs = append(errs, fmt.Errorf("%s: IPv6 source address not supported with set-mark/limit by backend vpp (VPP classify table uses IPv4 header layout)", prefix))
			}
		case firewall.MatchDestinationAddress:
			if v.Prefix.Addr().Is6() {
				errs = append(errs, fmt.Errorf("%s: IPv6 destination address not supported with set-mark/limit by backend vpp (VPP classify table uses IPv4 header layout)", prefix))
			}
		}
	}
	return errors.Join(errs...)
}

func verifyMatch(prefix string, m firewall.Match) error {
	switch v := m.(type) {
	case firewall.MatchSourceAddress,
		firewall.MatchDestinationAddress,
		firewall.MatchProtocol,
		firewall.MatchICMPType,
		firewall.MatchICMPv6Type,
		firewall.MatchTCPFlags:
		return nil
	case firewall.MatchConnState:
		return verifyConnState(prefix, v)
	case firewall.MatchSourcePort:
		if len(v.Ranges) > 1 {
			return fmt.Errorf("%s: source port with %d ranges not supported by backend vpp (VPP ACLRule has one port range per rule; use separate terms)", prefix, len(v.Ranges))
		}
		return nil
	case firewall.MatchDestinationPort:
		if len(v.Ranges) > 1 {
			return fmt.Errorf("%s: destination port with %d ranges not supported by backend vpp (VPP ACLRule has one port range per rule; use separate terms)", prefix, len(v.Ranges))
		}
		return nil
	case firewall.MatchInputInterface:
		return fmt.Errorf("%s: input-interface match not supported by backend vpp (VPP ACL is bound per-interface, not matched per-rule)", prefix)
	case firewall.MatchOutputInterface:
		return fmt.Errorf("%s: output-interface match not supported by backend vpp (VPP ACL is bound per-interface, not matched per-rule)", prefix)
	case firewall.MatchConnMark:
		return fmt.Errorf("%s: connection-mark match not supported by backend vpp", prefix)
	case firewall.MatchMark:
		return fmt.Errorf("%s: mark match not supported by backend vpp (VPP ACL matches packet headers, not SKB metadata)", prefix)
	case firewall.MatchDSCP:
		return fmt.Errorf("%s: dscp match not supported by backend vpp (deferred: VPP classify pipeline not yet implemented)", prefix)
	case firewall.MatchInSet:
		return fmt.Errorf("%s: set match %q not supported by backend vpp (VPP ACL has no set equivalent)", prefix, v.SetName)
	}
	return fmt.Errorf("%s: match type %T not recognized by backend vpp", prefix, m)
}

func verifyAction(prefix string, a firewall.Action) error {
	switch a.(type) {
	case firewall.Accept, firewall.Drop:
		return nil
	case firewall.FlowOffload:
		return nil
	case firewall.Reject:
		return fmt.Errorf("%s: reject action not supported by backend vpp (VPP ACL has deny, not reject-with-response)", prefix)
	case firewall.Jump:
		return fmt.Errorf("%s: jump action not supported by backend vpp (VPP ACL is flat rules, no chain traversal)", prefix)
	case firewall.Goto:
		return fmt.Errorf("%s: goto action not supported by backend vpp (VPP ACL is flat rules, no chain traversal)", prefix)
	case firewall.Return:
		return fmt.Errorf("%s: return action not supported by backend vpp (VPP ACL is flat rules, no chain traversal)", prefix)
	case firewall.SNAT:
		return fmt.Errorf("%s: snat action requires a NAT chain (type nat), not a filter chain", prefix)
	case firewall.DNAT:
		return fmt.Errorf("%s: dnat action requires a NAT chain (type nat), not a filter chain", prefix)
	case firewall.Masquerade:
		return fmt.Errorf("%s: masquerade action requires a NAT chain (type nat), not a filter chain", prefix)
	case firewall.Redirect:
		return fmt.Errorf("%s: redirect action not supported by backend vpp", prefix)
	case firewall.Notrack:
		return fmt.Errorf("%s: notrack action not supported by backend vpp", prefix)
	case firewall.SetMark:
		return nil
	case firewall.SetConnMark:
		return fmt.Errorf("%s: connection-mark-set action not supported by backend vpp", prefix)
	case firewall.SetDSCP:
		return fmt.Errorf("%s: dscp-set action not supported by backend vpp", prefix)
	case firewall.SetTCPMSS:
		return fmt.Errorf("%s: tcp-mss-set action not supported by backend vpp", prefix)
	case firewall.Counter:
		return fmt.Errorf("%s: counter action not supported by backend vpp (VPP ACL has implicit per-ACL counters, not per-rule named counters)", prefix)
	case firewall.Log:
		return fmt.Errorf("%s: log action not supported by backend vpp (deferred: VPP trace pipeline not yet integrated)", prefix)
	case firewall.Limit:
		return nil
	}
	return fmt.Errorf("%s: action type %T not recognized by backend vpp", prefix, a)
}

// verifyConnState rejects MatchConnState combinations that don't map
// faithfully to VPP's PERMIT_REFLECT. PERMIT_REFLECT creates a reflexive
// session for return traffic, which is the VPP equivalent of nftables'
// "connection state established,related" pattern. Only that combination
// (with or without related) is accepted. ConnStateNew alone or
// ConnStateInvalid have no VPP ACL equivalent.
func verifyConnState(prefix string, m firewall.MatchConnState) error {
	acceptable := firewall.ConnStateEstablished | firewall.ConnStateRelated
	if m.States&^acceptable != 0 {
		return fmt.Errorf("%s: connection state match with new/invalid not supported by backend vpp (VPP PERMIT_REFLECT only models established+related)", prefix)
	}
	if m.States == 0 {
		return fmt.Errorf("%s: connection state match with empty state mask", prefix)
	}
	return nil
}

// aclTag builds the VPP ACL tag from table and chain names. VPP ACL
// tags are descriptive strings stored alongside the ACL; the verifier
// rejects names whose tag exceeds the 64-byte limit.
func aclTag(tableName, chainName string) string {
	var tb textbuf.Buffer
	return tb.Str("ze/").Str(tableName).Byte('/').Str(chainName).String()
}

// natTag builds the VPP NAT static mapping tag from table, chain, and
// term names. Same 64-byte limit as ACL tags.
func natTag(tableName, chainName, termName string) string {
	var tb textbuf.Buffer
	return tb.Str("ze/").Str(tableName).Byte('/').Str(chainName).Byte('/').Str(termName).String()
}

// classifyPolicerName builds the VPP policer name for classify-backed
// actions (Limit). Same 64-byte limit as ACL/NAT tags.
func classifyPolicerName(tableName, chainName, termName string) string {
	var tb textbuf.Buffer
	return tb.Str("ze/fw/").Str(tableName).Byte('/').Str(chainName).Byte('/').Str(termName).String()
}
