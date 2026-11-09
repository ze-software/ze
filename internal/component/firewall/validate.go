// Design: docs/architecture/core-design.md -- Firewall verify-time validation
// Related: model.go -- Types validated here
// Related: engine.go -- OnConfigVerify call site

package firewall

import (
	"fmt"
	"strings"
)

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
		if err := validateCtHelpers(tbl); err != nil {
			return err
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
	// An arp table hands a rule the ARP header where the other families hand
	// it the IP header, and the nft lowering of these matches reads fixed IP
	// offsets (lowerAddrMatch, internal/plugins/firewall/nft/lower_linux.go).
	// In arp those offsets land inside the ARP header, so the rule compares
	// the wrong bytes and fires on packets the operator never named.
	if tbl.Family == FamilyARP {
		if leaf, readsIP := ipHeaderMatchLeaf(m); readsIP {
			return fmt.Errorf("table %q chain %q term %q: %s match reads the IP header, and family arp carries no IP header; move the term to a table of family ip, ip6 or inet",
				tbl.Name, ch.Name, term.Name, leaf)
		}
	}
	switch v := m.(type) {
	case MatchInSet:
		setType, ok := sets[v.SetName]
		if !ok {
			// A set another registry owner supplies is absent from this
			// table's own Sets at verify time: the owners meet only at
			// ApplyAll. The match carries the element type that owner will
			// supply, so the field check below still runs. Only the config
			// parser sets ProvidedType, and only for the IRR leaves, so a
			// name the operator typed is still refused here (model.go).
			if v.ProvidedType == 0 {
				return fmt.Errorf("table %q chain %q term %q: match references unknown set %q",
					tbl.Name, ch.Name, term.Name, v.SetName)
			}
			setType = v.ProvidedType
		}
		if err := validateSetFieldMatch(tbl, ch, term, v, setType); err != nil {
			return err
		}
	case MatchICMPType:
		// ICMPv4 type numbers only apply to IPv4. An `inet` table is
		// accepted, and lowerICMPTypeMatch
		// (internal/plugins/firewall/nft/lower_linux.go) puts a
		// `meta l4proto 1` dependency ahead of the type-byte read there,
		// so the match keeps to ICMPv4 in a table that sees both
		// protocols. Any other family is a configuration error.
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
		// fine because the lowering emits a `meta nfproto ipv4` guard
		// ahead of the read there (lowerDSCPMatch,
		// internal/plugins/firewall/nft/lower_linux.go).
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: dscp match is IPv4-only; move to a table with family ip or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
	case MatchIPv4TTLBelow:
		// The TTL byte is at IPv4 network-header offset 8, so the read is
		// IPv4-only for the same reason the DSCP read above is, and inet is
		// accepted for the same reason: lowerIPv4TTLBelowMatch
		// (internal/plugins/firewall/nft/lower_linux.go) emits the
		// `meta nfproto ipv4` guard ahead of it.
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: ttl-below match is IPv4-only; move to a table with family ip or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
		if v.Floor == 0 {
			return fmt.Errorf("table %q chain %q term %q: ttl-below floor is 0, which no TTL can be below",
				tbl.Name, ch.Name, term.Name)
		}
	case MatchIPv6HopLimitBelow:
		// The hop limit is at IPv6 network-header offset 7, so the read is
		// IPv6-only, and inet is accepted because lowerIPv6HopLimitBelowMatch
		// (internal/plugins/firewall/nft/lower_linux.go) emits the
		// `meta nfproto ipv6` guard ahead of it.
		if tbl.Family != FamilyIP6 && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: hop-limit-below match is IPv6-only; move to a table with family ip6 or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
		if v.Floor == 0 {
			return fmt.Errorf("table %q chain %q term %q: hop-limit-below floor is 0, which no hop limit can be below",
				tbl.Name, ch.Name, term.Name)
		}
	case MatchICMPErrorQuotedTCPPort:
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: icmp-quoted-tcp-port match is IPv4-only; move to a table with family ip or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
		if v.Side != QuotedPortSource && v.Side != QuotedPortDestination {
			return fmt.Errorf("table %q chain %q term %q: icmp-quoted-tcp-port names no side",
				tbl.Name, ch.Name, term.Name)
		}
		if v.Port == 0 {
			return fmt.Errorf("table %q chain %q term %q: icmp-quoted-tcp-port is 0, which no TCP session carries",
				tbl.Name, ch.Name, term.Name)
		}
	case MatchICMPErrorQuotedDestination:
		// The read is 4 octets at a fixed offset inside a quoted IPv4 header,
		// so the family rule is the one MatchICMPErrorQuotedTCPPort follows.
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: icmp-quoted-destination match is IPv4-only; move to a table with family ip or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
		// The unspecified address is no peer. An IPv6 or zero address has no
		// 4-octet form. Each would be a compare against bytes that name nothing.
		if !v.Addr.Is4() || v.Addr.IsUnspecified() {
			return fmt.Errorf("table %q chain %q term %q: icmp-quoted-destination names no IPv4 address (got %v)",
				tbl.Name, ch.Name, term.Name, v.Addr)
		}
	case MatchARPOperation:
		// Only an arp table hands a rule the ARP header as its network
		// header; in any other family the opcode offset is an IP field.
		if tbl.Family != FamilyARP {
			return fmt.Errorf("table %q chain %q term %q: arp-operation match is valid only in family arp, got %s",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
		if v.Operation != ARPOperationRequest && v.Operation != ARPOperationReply {
			return fmt.Errorf("table %q chain %q term %q: arp-operation names no opcode (got %d)",
				tbl.Name, ch.Name, term.Name, v.Operation)
		}
	case MatchARPSenderAddress:
		if tbl.Family != FamilyARP {
			return fmt.Errorf("table %q chain %q term %q: arp-sender-address match is valid only in family arp, got %s",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
		// The compare is 4 octets of an Ethernet/IPv4 ARP packet, so only a
		// specified IPv4 address names a sender.
		if !v.Addr.Is4() {
			return fmt.Errorf("table %q chain %q term %q: arp-sender-address names no IPv4 address (got %v)",
				tbl.Name, ch.Name, term.Name, v.Addr)
		}
		if v.Addr.IsUnspecified() {
			return fmt.Errorf("table %q chain %q term %q: arp-sender-address names no IPv4 address (got %v)",
				tbl.Name, ch.Name, term.Name, v.Addr)
		}
	case MatchNDTargetAddress:
		if tbl.Family != FamilyIP6 && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: nd-target-address match is valid only in family ip6 or inet, got %s",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
		if !v.Addr.Is6() {
			return fmt.Errorf("table %q chain %q term %q: nd-target-address names no IPv6 address (got %v)",
				tbl.Name, ch.Name, term.Name, v.Addr)
		}
		if v.Addr.Is4In6() {
			return fmt.Errorf("table %q chain %q term %q: nd-target-address names no IPv6 address (got %v)",
				tbl.Name, ch.Name, term.Name, v.Addr)
		}
		if v.Addr.IsUnspecified() {
			return fmt.Errorf("table %q chain %q term %q: nd-target-address names no IPv6 address (got %v)",
				tbl.Name, ch.Name, term.Name, v.Addr)
		}
	}
	return nil
}

// ipHeaderMatchLeaf names the config leaf of a match whose lowering reads the
// IP header or the transport header behind it, or reports false for a match
// that reads none (an interface, a mark, the ARP fields). The family-specific
// IP matches (icmp-type, icmpv6-type, dscp, the TTL and quoted-header reads)
// answer false here because validateMatch refuses them in arp with their own
// message.
func ipHeaderMatchLeaf(m Match) (string, bool) {
	switch v := m.(type) {
	case MatchSourceAddress:
		return "source-address", true
	case MatchDestinationAddress:
		return "destination-address", true
	case MatchSourcePort:
		return "source-port", true
	case MatchDestinationPort:
		return "destination-port", true
	case MatchProtocol:
		return "protocol", true
	case MatchConnState:
		return "connection-state", true
	case MatchConnMark:
		return "connection-mark", true
	case MatchTCPFlags:
		return "tcp-flags", true
	case MatchInSet:
		return setMatchLeaf(v), true
	}
	return "", false
}

// setMatchLeaf names the leaf that produced a set match: the IRR and
// domain-group leaves build their set names from irrV4Prefix and
// domainV4Prefix (irrSetMatch, domainSetMatch), and every other set match is
// an address or port leaf naming "@set".
func setMatchLeaf(m MatchInSet) string {
	source := m.MatchField == SetFieldSourceAddr || m.MatchField == SetFieldSourcePort
	if strings.HasPrefix(m.SetName, irrV4Prefix) {
		if source {
			return "source-asn or source-as-set"
		}
		return "destination-asn or destination-as-set"
	}
	if strings.HasPrefix(m.SetName, domainV4Prefix) {
		if source {
			return "source-domain-group"
		}
		return "destination-domain-group"
	}
	switch m.MatchField {
	case SetFieldSourceAddr:
		return "source-address set"
	case SetFieldDestAddr:
		return "destination-address set"
	case SetFieldSourcePort:
		return "source-port set"
	case SetFieldDestPort:
		return "destination-port set"
	}
	return "set"
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
// offsets (wrong bytes). Both families are valid in an inet table
// because the lowering emits a `meta nfproto` guard ahead of the read
// there (nfprotoGuard, internal/plugins/firewall/nft/lower_linux.go).
// The chain itself dispatches nothing: it sees both families, and a raw
// payload read takes whichever bytes sit at the offset.
//
// arp/bridge/netdev are also rejected: those families dispatch on
// non-IP headers, so an address match has no meaningful offset to
// read. In arp, validateMatch already refused every IP-header match,
// literal or set, before this runs (ipHeaderMatchLeaf).
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
	default:
		panic("BUG: unknown validated firewall table family")
	}
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
	case SetCtHelper:
		if !tableHasCtHelper(tbl, v.Name) {
			return fmt.Errorf("table %q chain %q term %q: assigns unknown ct helper %q",
				tbl.Name, ch.Name, term.Name, v.Name)
		}
	case Counter:
		if v.Name != "" {
			return fmt.Errorf("table %q chain %q term %q: named counter %q not yet supported; omit the name for an anonymous counter",
				tbl.Name, ch.Name, term.Name, v.Name)
		}
	case SetDSCP:
		// dscp-set rewrites the DSCP field, which sits in the IPv4 TOS
		// byte and in the IPv6 traffic class. An `inet` chain sees both
		// families, so lowerTerm
		// (internal/plugins/firewall/nft/lower_linux.go) programs the
		// term as two rules there, one guarded on each family, and each
		// rule writes its own header layout.
		// arp/bridge/netdev carry no IP header to rewrite and are
		// rejected. ip6 is still rejected: setDSCPv6 can now program it,
		// so widening this to FamilyIP6 is a config-surface change with
		// its own functional test, not a gap in the lowering.
		if tbl.Family != FamilyIP && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: dscp-set is IPv4-only; move to a table with family ip or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
	case SetTCPMSS:
		if tbl.Family != FamilyIP && tbl.Family != FamilyIP6 && tbl.Family != FamilyInet {
			return fmt.Errorf("table %q chain %q term %q: tcp-mss-set requires family ip, ip6, or inet (got %s)",
				tbl.Name, ch.Name, term.Name, tbl.Family)
		}
	case ClampTCPMSSToPMTU:
		return validateClampTCPMSSToPMTU(tbl, ch, term)
	}
	return nil
}

// validateClampTCPMSSToPMTU refuses every placement the kernel would refuse,
// so a commit never fails at netlink with an error the operator cannot map to
// a term. nft_rt_validate (net/netfilter/nft_rt.c) accepts the route MSS only
// in the ip, ip6 and inet families, and only in a chain whose hooks are all
// forward, output or postrouting; for a regular chain the kernel checks the
// hook of every base chain that reaches it through jump or goto.
func validateClampTCPMSSToPMTU(tbl *Table, ch *Chain, term *Term) error {
	if tbl.Family != FamilyIP && tbl.Family != FamilyIP6 && tbl.Family != FamilyInet {
		return fmt.Errorf("table %q chain %q term %q: tcp-mss-clamp-to-pmtu requires family ip, ip6, or inet (got %s)",
			tbl.Name, ch.Name, term.Name, tbl.Family)
	}
	for _, other := range term.Actions {
		if _, ok := other.(SetTCPMSS); ok {
			return fmt.Errorf("table %q chain %q term %q: tcp-mss-set and tcp-mss-clamp-to-pmtu cannot be combined in one term; keep one of them",
				tbl.Name, ch.Name, term.Name)
		}
	}
	if ch.IsBase {
		if !routeMSSHook(ch.Hook) {
			return fmt.Errorf("table %q chain %q term %q: tcp-mss-clamp-to-pmtu is not available at hook %s, because the kernel computes the route MSS only at forward, output or postrouting; move the term to a chain with hook forward or postrouting",
				tbl.Name, ch.Name, term.Name, ch.Hook)
		}
		return nil
	}
	base, hook, found := baseChainReaching(tbl, ch.Name)
	if found {
		return fmt.Errorf("table %q chain %q term %q: tcp-mss-clamp-to-pmtu is not available at hook %s, which reaches this chain from base chain %q through jump or goto, because the kernel computes the route MSS only at forward, output or postrouting; move the term to a chain reached only from hook forward or postrouting",
			tbl.Name, ch.Name, term.Name, hook, base)
	}
	return nil
}

// routeMSSHook reports whether the kernel computes the route MSS at hook h:
// the hook mask NFT_RT_TCPMSS carries in nft_rt_validate is forward, local
// out and postrouting.
func routeMSSHook(h ChainHook) bool {
	switch h {
	case HookForward, HookOutput, HookPostrouting:
		return true
	case chainHookUnknown, HookInput, HookPrerouting, HookIngress, HookEgress:
		return false
	}
	return false
}

// baseChainReaching finds a base chain of tbl whose hook does not allow the
// route MSS and that reaches the regular chain target through jump or goto.
// It answers that base chain's name and hook. A regular chain no base chain
// reaches is never run, so the kernel never checks it and neither does this.
// The walk visits each chain once, so it is bounded by len(tbl.Chains).
func baseChainReaching(tbl *Table, target string) (string, ChainHook, bool) {
	byName := make(map[string]*Chain, len(tbl.Chains))
	for i := range tbl.Chains {
		byName[tbl.Chains[i].Name] = &tbl.Chains[i]
	}
	for i := range tbl.Chains {
		base := &tbl.Chains[i]
		if !base.IsBase {
			continue
		}
		if routeMSSHook(base.Hook) {
			continue
		}
		if chainReaches(byName, base, target) {
			return base.Name, base.Hook, true
		}
	}
	return "", chainHookUnknown, false
}

// chainReaches reports whether a jump or goto path leads from chain from to
// the chain named target. A breadth-first walk with a visited set, so a jump
// cycle (which the kernel refuses on its own) cannot loop it.
func chainReaches(byName map[string]*Chain, from *Chain, target string) bool {
	visited := make(map[string]bool, len(byName))
	queue := []*Chain{from}
	visited[from.Name] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for i := range cur.Terms {
			for _, a := range cur.Terms[i].Actions {
				next := jumpTarget(a)
				if next == "" {
					continue
				}
				if next == target {
					return true
				}
				if visited[next] {
					continue
				}
				visited[next] = true
				if c, ok := byName[next]; ok {
					queue = append(queue, c)
				}
			}
		}
	}
	return false
}

// jumpTarget answers the chain a jump or goto action enters, and "" for an
// action that enters no chain.
func jumpTarget(a Action) string {
	switch v := a.(type) {
	case Jump:
		return v.Target
	case Goto:
		return v.Target
	}
	return ""
}

// validateCtHelpers checks each conntrack helper object of a table. The kernel
// holds ct helper objects only in the ip, ip6 and inet families
// (nft_ct_helper_obj_init, net/netfilter/nft_ct.c, refuses every other family
// with EOPNOTSUPP), so a table of another family is refused here rather than
// at Apply.
func validateCtHelpers(tbl *Table) error {
	if len(tbl.CtHelpers) == 0 {
		return nil
	}
	if tbl.Family != FamilyIP && tbl.Family != FamilyIP6 && tbl.Family != FamilyInet {
		return fmt.Errorf("table %q: ct helper objects require family ip, ip6, or inet (got %s)",
			tbl.Name, tbl.Family)
	}
	seen := make(map[string]bool, len(tbl.CtHelpers))
	for i := range tbl.CtHelpers {
		h := &tbl.CtHelpers[i]
		if err := h.Validate(); err != nil {
			return fmt.Errorf("table %q: %w", tbl.Name, err)
		}
		if seen[h.Name] {
			return fmt.Errorf("table %q: ct helper %q declared twice", tbl.Name, h.Name)
		}
		seen[h.Name] = true
	}
	return nil
}

// tableHasCtHelper reports whether tbl declares the ct helper object name.
func tableHasCtHelper(tbl *Table, name string) bool {
	for i := range tbl.CtHelpers {
		if tbl.CtHelpers[i].Name == name {
			return true
		}
	}
	return false
}
