// Design: plan/spec-fw-6-firewall-vpp.md -- Commit-time rejection matrix

package firewallvpp

import (
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
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
	var errs []error
	for i := range ch.Terms {
		if err := verifyTerm(tbl, ch, &ch.Terms[i]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
		return fmt.Errorf("%s: snat action not supported by backend vpp (deferred: VPP NAT44 plugin not yet integrated)", prefix)
	case firewall.DNAT:
		return fmt.Errorf("%s: dnat action not supported by backend vpp (deferred: VPP NAT44 plugin not yet integrated)", prefix)
	case firewall.Masquerade:
		return fmt.Errorf("%s: masquerade action not supported by backend vpp (deferred: VPP NAT44 plugin not yet integrated)", prefix)
	case firewall.Redirect:
		return fmt.Errorf("%s: redirect action not supported by backend vpp", prefix)
	case firewall.Notrack:
		return fmt.Errorf("%s: notrack action not supported by backend vpp", prefix)
	case firewall.SetMark:
		return fmt.Errorf("%s: mark-set action not supported by backend vpp (deferred: VPP classifier pipeline not yet implemented)", prefix)
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
		return fmt.Errorf("%s: limit action not supported by backend vpp (deferred: VPP policer integration for per-rule rate limiting)", prefix)
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
	return fmt.Sprintf("ze/%s/%s", tableName, chainName)
}
