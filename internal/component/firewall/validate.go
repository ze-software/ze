// Design: docs/architecture/core-design.md -- Firewall verify-time validation
// Related: model.go -- Types validated here
// Related: engine.go -- OnConfigVerify call site

package firewall

import "fmt"

// ifaceNameMaxLen mirrors the kernel's IFNAMSIZ-1 byte cap on interface
// names. A name longer than this is truncated by the kernel when stored
// in the chain register, so any match against it would silently never
// fire. We reject at verify so `ze config validate` surfaces the typo
// before reload.
const ifaceNameMaxLen = 15

// ValidateTables runs every verify-time check that does not need the kernel:
// per-item Validate, term non-emptiness, cross-references (Jump / Goto target
// chains, FlowOffload flowtable, MatchInSet named set), and the "exact or
// reject" guards on actions the backend cannot yet program exactly
// (SetDSCP on non-IPv4 tables, named Counter). Callers are OnConfigVerify and
// OnConfigure; everything the backend touches at Apply must pass here first.
func ValidateTables(tables []Table) error {
	for i := range tables {
		tbl := &tables[i]
		if err := tbl.Validate(); err != nil {
			return err
		}
		chainNames := collectChainNames(tbl)
		setNames := collectSetNames(tbl)
		flowtableNames := collectFlowtableNames(tbl)

		for j := range tbl.Sets {
			if err := tbl.Sets[j].Validate(); err != nil {
				return fmt.Errorf("table %q: %w", tbl.Name, err)
			}
		}
		for j := range tbl.Flowtables {
			if err := tbl.Flowtables[j].Validate(); err != nil {
				return fmt.Errorf("table %q: %w", tbl.Name, err)
			}
		}
		for j := range tbl.Chains {
			ch := &tbl.Chains[j]
			if err := ch.Validate(); err != nil {
				return fmt.Errorf("table %q: %w", tbl.Name, err)
			}
			for k := range ch.Terms {
				if err := validateTerm(tbl, ch, &ch.Terms[k], chainNames, setNames, flowtableNames); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func collectChainNames(t *Table) map[string]bool {
	m := make(map[string]bool, len(t.Chains))
	for i := range t.Chains {
		m[t.Chains[i].Name] = true
	}
	return m
}

// collectSetNames returns a map keyed by set name with the set's
// element type as value. validateMatch uses the type to reject
// MatchInSet where the field and the set type disagree (e.g.
// source-address pointing at an inet_service port set) at verify
// time, rather than letting the mismatch reach the lowering layer
// at Apply.
func collectSetNames(t *Table) map[string]SetType {
	m := make(map[string]SetType, len(t.Sets))
	for i := range t.Sets {
		m[t.Sets[i].Name] = t.Sets[i].Type
	}
	return m
}

func collectFlowtableNames(t *Table) map[string]bool {
	m := make(map[string]bool, len(t.Flowtables))
	for i := range t.Flowtables {
		m[t.Flowtables[i].Name] = true
	}
	return m
}

func validateTerm(tbl *Table, ch *Chain, term *Term, chains map[string]bool, sets map[string]SetType, flowtables map[string]bool) error {
	if err := ValidateName(term.Name); err != nil {
		return fmt.Errorf("table %q chain %q: term: %w", tbl.Name, ch.Name, err)
	}
	if len(term.Actions) == 0 {
		return fmt.Errorf("table %q chain %q term %q: at least one action required",
			tbl.Name, ch.Name, term.Name)
	}
	for _, m := range term.Matches {
		if err := validateMatch(tbl, ch, term, m, sets); err != nil {
			return err
		}
	}
	for _, a := range term.Actions {
		if err := validateAction(tbl, ch, term, a, chains, flowtables); err != nil {
			return err
		}
	}
	return nil
}

func validateMatch(tbl *Table, ch *Chain, term *Term, m Match, sets map[string]SetType) error {
	switch v := m.(type) {
	case MatchInSet:
		setType, ok := sets[v.SetName]
		if !ok {
			return fmt.Errorf("table %q chain %q term %q: match references unknown set %q",
				tbl.Name, ch.Name, term.Name, v.SetName)
		}
		if err := validateSetFieldMatch(tbl, ch, term, v, setType); err != nil {
			return err
		}
	case MatchICMPType:
		// ICMPv4 type numbers only apply to IPv4. An `inet` table
		// dispatches to ipv4/ipv6 by the packet's L3 proto, so it is
		// also valid: the rule simply never matches for IPv6 packets.
		// Any other family is a configuration error.
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: icmp-type is valid only in family ip or inet, got %s",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
	case MatchICMPv6Type:
		if tbl.Family != FamilyIP6 && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: icmpv6-type is valid only in family ip6 or inet, got %s",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
	case MatchInputInterface:
		if err := validateInterfaceName(tbl, ch, term, "input-interface", v.Name); err != nil {
			return err
		}
	case MatchOutputInterface:
		if err := validateInterfaceName(tbl, ch, term, "output-interface", v.Name); err != nil {
			return err
		}
	case MatchDSCP:
		// DSCP lives in the IPv4 TOS byte (offset 1 of the network
		// header). Lowering is IPv4-only; in ip6/arp/bridge/netdev
		// the same offset points at a different byte (traffic-class
		// high + flow-label high nibble for IPv6; non-IP header for
		// the others) so the rule would silently misfire. inet is
		// fine because its chains dispatch per packet to the IPv4
		// header layout for ipv4 packets.
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: dscp match is IPv4-only; move to a table with family ip or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
	}
	return nil
}

// validateSetFieldMatch rejects MatchInSet where (a) the field and
// the referenced set's element type disagree, or (b) the set's
// family and the parent table's family disagree. (a) catches
// `source-address "@ports"` against an inet_service set; (b) catches
// `source-address "@ipv4set"` in an ip6 table, which would lower to
// Payload(Network,12,4) reading the middle of the IPv6 source addr.
// Unknown MatchField values also reject: the set of valid fields is
// finite and a future enum entry that bypasses this function would
// silently accept.
func validateSetFieldMatch(tbl *Table, ch *Chain, term *Term, m MatchInSet, setType SetType) error {
	isAddr := m.MatchField == SetFieldSourceAddr || m.MatchField == SetFieldDestAddr
	isPort := m.MatchField == SetFieldSourcePort || m.MatchField == SetFieldDestPort
	if !isAddr && !isPort {
		return fmt.Errorf("table %q chain %q term %q: match against %q uses unknown set field %d",
			tbl.Name, ch.Name, term.Name, m.SetName, m.MatchField)
	}
	if isAddr {
		if setType != SetTypeIPv4 && setType != SetTypeIPv6 {
			return fmt.Errorf("table %q chain %q term %q: match against %q expects an ipv4/ipv6 set, set type is %s",
				tbl.Name, ch.Name, term.Name, m.SetName, setType)
		}
		return validateSetFamilyCompat(tbl, ch, term, m, setType)
	}
	// port field
	if setType != SetTypeInetService {
		return fmt.Errorf("table %q chain %q term %q: match against %q expects an inet-service set, set type is %s",
			tbl.Name, ch.Name, term.Name, m.SetName, setType)
	}
	return nil
}

// validateSetFamilyCompat rejects address-field matches where the set
// element family and the parent table family contradict. An ipv4 set
// used in an ip6 table would lower against the IPv6 header at IPv4
// offsets (wrong bytes). inet tables dispatch per packet to the
// matching header so both families are valid there.
//
// arp/bridge/netdev are also rejected: those families dispatch on
// non-IP headers, so an address match has no meaningful offset to
// read. Parallel gap for literal MatchSourceAddress/DestinationAddress
// is tracked separately (pre-existing, not introduced here).
func validateSetFamilyCompat(tbl *Table, ch *Chain, term *Term, m MatchInSet, setType SetType) error {
	switch tbl.Family {
	case FamilyInet:
		return nil
	case FamilyIP:
		if setType == SetTypeIPv6 {
			return fmt.Errorf("table %q chain %q term %q: match against %q (ipv6_addr set) is invalid in family ip (use family ip6 or inet)",
				tbl.Name, ch.Name, term.Name, m.SetName)
		}
		return nil
	case FamilyIP6:
		if setType == SetTypeIPv4 {
			return fmt.Errorf("table %q chain %q term %q: match against %q (ipv4_addr set) is invalid in family ip6 (use family ip or inet)",
				tbl.Name, ch.Name, term.Name, m.SetName)
		}
		return nil
	case familyUnknown, FamilyARP, FamilyBridge, FamilyNetdev:
		return fmt.Errorf("table %q chain %q term %q: address match against %q is invalid in family %s (use ip, ip6, or inet)",
			tbl.Name, ch.Name, term.Name, m.SetName, tbl.Family)
	}
	return nil
}

// validateInterfaceName rejects the two ways an interface-name match can
// silently never fire: an empty name (e.g. operator typed `input-interface
// "*"`, which parses to Name="" + Wildcard=true) and a name longer than
// IFNAMSIZ-1 (the kernel stores interface names in a 16-byte register;
// anything longer is truncated before the compare).
func validateInterfaceName(tbl *Table, ch *Chain, term *Term, leaf, name string) error {
	if name == "" {
		return fmt.Errorf("table %q chain %q term %q: %s name must not be empty",
			tbl.Name, ch.Name, term.Name, leaf)
	}
	if len(name) > ifaceNameMaxLen {
		return fmt.Errorf("table %q chain %q term %q: %s name %q exceeds %d-byte kernel limit",
			tbl.Name, ch.Name, term.Name, leaf, name, ifaceNameMaxLen)
	}
	return nil
}

func validateAction(tbl *Table, ch *Chain, term *Term, a Action, chains, flowtables map[string]bool) error {
	switch v := a.(type) {
	case Jump:
		if !chains[v.Target] {
			return fmt.Errorf("table %q chain %q term %q: jump target chain %q not defined in table",
				tbl.Name, ch.Name, term.Name, v.Target)
		}
	case Goto:
		if !chains[v.Target] {
			return fmt.Errorf("table %q chain %q term %q: goto target chain %q not defined in table",
				tbl.Name, ch.Name, term.Name, v.Target)
		}
	case FlowOffload:
		if !flowtables[v.FlowtableName] {
			return fmt.Errorf("table %q chain %q term %q: flow-offload references unknown flowtable %q",
				tbl.Name, ch.Name, term.Name, v.FlowtableName)
		}
	case Counter:
		if v.Name != "" {
			return fmt.Errorf("table %q chain %q term %q: named counter %q not yet supported; omit the name for an anonymous counter",
				tbl.Name, ch.Name, term.Name, v.Name)
		}
	case SetDSCP:
		// dscp-set rewrites the IPv4 TOS byte. An `inet` chain is
		// valid because its dispatch runs the rule on the ipv4
		// path only for ipv4 packets. ip6/arp/bridge/netdev are
		// rejected -- the lowered Payload-write would target the
		// wrong byte (IPv6 traffic class + flow label) or a
		// non-IP header.
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: dscp-set is IPv4-only; move to a table with family ip or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
	}
	return nil
}
