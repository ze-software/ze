// Design: docs/architecture/policyroute/policy-routing.md -- config to nftables/ip-rule translation

package policyroute

import (
	"fmt"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/iprule"
)

const policyRoutingTable = "ze_pr"

type translationResult struct {
	Tables     []firewall.Table
	IPRules    []iprule.Mark
	AutoRoutes []autoRouteSpec
}

type autoRouteSpec struct {
	Table   uint32
	NextHop netip.Addr
}

func (a *allocator) translate(policies []PolicyRoute) (*translationResult, error) {
	result := &translationResult{}
	basePriority := 100

	// Type is FILTER, not ROUTE. nftables only accepts a `type route` chain on the
	// OUTPUT hook -- it exists to force a re-route of LOCALLY GENERATED packets
	// after a mark change. At prerouting the kernel rejects the combination with
	// EOPNOTSUPP, which surfaces as "netlink receive: operation not supported" and
	// takes the whole policy-routes plugin down at startup, so every test/policy
	// case timed out. Marking at prerouting in a filter chain is the correct and
	// standard way to drive `ip rule fwmark`: the routing decision for forwarded
	// traffic happens after prerouting, so it sees the mark this chain sets.
	chain := firewall.Chain{
		Name:     "prerouting",
		IsBase:   true,
		Type:     firewall.ChainFilter,
		Hook:     firewall.HookPrerouting,
		Priority: -150,
		Policy:   firewall.PolicyAccept,
	}

	for _, policy := range policies {
		terms, rules, autoRoutes, err := a.translatePolicy(policy, &basePriority)
		if err != nil {
			return nil, fmt.Errorf("policy %q: %w", policy.Name, err)
		}
		chain.Terms = append(chain.Terms, terms...)
		result.IPRules = append(result.IPRules, rules...)
		result.AutoRoutes = append(result.AutoRoutes, autoRoutes...)
	}

	if len(chain.Terms) > 0 {
		result.Tables = []firewall.Table{{
			Name:   policyRoutingTable,
			Family: firewall.FamilyInet,
			Chains: []firewall.Chain{chain},
			Sets:   referencedSets(policies),
		}}
	}

	return result, nil
}

// referencedSets returns one copy of every firewall set the policies' rules
// match, in the order the rules first name them.
//
// The copy is the whole set: its type, its flags and its elements. nftables
// resolves a set only inside the table that holds the rule, so the ze_pr chain
// cannot read the set in the firewall table that declares it, and a term that
// names it there fails the apply. Each translate takes the copy afresh from the
// staged policies, whose sets resolveSetReferences read from the same commit's
// firewall section, so the two kernel sets hold the same elements after every
// commit.
func referencedSets(policies []PolicyRoute) []firewall.Set {
	var sets []firewall.Set
	seen := make(map[string]bool)
	add := func(set *firewall.Set) {
		if set == nil {
			return
		}
		if seen[set.Name] {
			return
		}
		seen[set.Name] = true
		sets = append(sets, *set)
	}
	for i := range policies {
		for j := range policies[i].Rules {
			match := &policies[i].Rules[j].Match
			add(match.SourceSet)
			add(match.DestinationSet)
			add(match.SourcePortSet)
			add(match.DestinationPortSet)
		}
	}
	return sets
}

func (a *allocator) translatePolicy(policy PolicyRoute, basePriority *int) ([]firewall.Term, []iprule.Mark, []autoRouteSpec, error) {
	var terms []firewall.Term
	var ipRules []iprule.Mark
	var autoRoutes []autoRouteSpec

	for i := range policy.Rules {
		matches, err := buildPolicyMatch(policy.Rules[i].Match)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("rule %q: %w", policy.Rules[i].Name, err)
		}

		actions, ruleSpecs, routeSpecs, err := a.buildActions(policy.Name, policy.Rules[i], basePriority)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("rule %q: %w", policy.Rules[i].Name, err)
		}

		terms = append(terms, ruleTerms(policy, policy.Rules[i].Name, matches, actions)...)

		ipRules = append(ipRules, ruleSpecs...)
		autoRoutes = append(autoRoutes, routeSpecs...)
	}

	return terms, ipRules, autoRoutes, nil
}

// ruleTerms returns the terms one rule of a policy installs: one term for each
// interface the policy names, and one term carrying no interface match where it
// names none.
//
// The interface leaf-list is an OR, because a packet arrives on exactly one
// interface and carries one name. nftables ANDs the matches inside a rule and
// has no branch inside one, so the alternatives cannot share a term: two
// interface matches in one term ask for a packet whose input interface is both
// names at once, which no packet satisfies. Separate terms become separate
// rules, and separate rules are how nftables ORs.
//
// The order the rules run in is unchanged. At most one term of the group can
// match a given packet, so their relative order decides nothing, and the group
// sits where the single term sat: after the terms of the previous rule and
// before those of the next, in the order the policy's `order` leaf established.
//
// Each term gets its own matches and actions slice. The mark and the ip rule
// are allocated once for the rule, before this call, so the group shares one
// mark and one ip rule rather than one per interface.
//
// The clone is on the per-interface branch alone. Several terms sharing one
// actions slice would alias, and no reader can see from a term that its
// neighbor holds the same backing array; the single term of the no-interface
// branch has nobody to alias with, so it takes the caller's slice as it is.
func ruleTerms(policy PolicyRoute, ruleName string, ruleMatches []firewall.Match, actions []firewall.Action) []firewall.Term {
	base := policy.Name + "-" + ruleName

	if len(policy.Interfaces) == 0 {
		return []firewall.Term{{Name: base, Matches: ruleMatches, Actions: actions}}
	}

	terms := make([]firewall.Term, 0, len(policy.Interfaces))
	for i, iface := range policy.Interfaces {
		matches := make([]firewall.Match, 0, 1+len(ruleMatches))
		matches = append(matches, firewall.MatchInputInterface{
			Name:     iface.Name,
			Wildcard: iface.Wildcard,
		})
		matches = append(matches, ruleMatches...)

		terms = append(terms, firewall.Term{
			Name:    termName(base, i, len(policy.Interfaces)),
			Matches: matches,
			Actions: slices.Clone(actions),
		})
	}
	return terms
}

// termName names the term an interface gets inside a rule's group. One
// interface keeps the <policy>-<rule> name the operator reads in the docs and
// in `show firewall ruleset`. Several interfaces take the interface's 1-based
// position in the leaf-list, because the counters of two terms that share a
// name merge into one row (mergeRuleCounters,
// internal/plugins/firewall/nft/backend_linux.go) and the show output would
// then report the group's total once for each interface.
//
// The position rather than the interface name: the name is operator input, it
// reaches the operator again through Rule.UserData and every show path, and it
// tells the reader nothing the position does not.
//
// No check stands behind that choice, so do not read one into it. ValidateName
// (internal/component/firewall/model.go) reaches a term name only from
// validateTerm. Only ValidateTables calls validateTerm, and ValidateTables has
// two callers (internal/component/firewall/engine.go). Both read the firewall
// engine's OWN cfg.Tables. firewall.RegisterTables checks the ze_ table-name
// prefix and nothing else. A term this plugin registers is therefore never
// name-checked.
func termName(base string, index, count int) string {
	if count == 1 {
		return base
	}
	return base + "-" + strconv.Itoa(index+1)
}

func buildPolicyMatch(pm PolicyMatch) ([]firewall.Match, error) {
	var matches []firewall.Match

	if pm.SourceAddress != "" {
		m, err := parseAddressMatch(pm.SourceAddress, pm.SourceSet, true)
		if err != nil {
			return nil, fmt.Errorf("source address: %w", err)
		}
		matches = append(matches, m)
	}

	if pm.DestinationAddress != "" {
		m, err := parseAddressMatch(pm.DestinationAddress, pm.DestinationSet, false)
		if err != nil {
			return nil, fmt.Errorf("destination address: %w", err)
		}
		matches = append(matches, m)
	}

	if pm.Protocol != "" {
		matches = append(matches, firewall.MatchProtocol{Protocol: pm.Protocol})
	}

	if pm.DestinationPort != "" {
		m, err := parsePortMatch(pm.DestinationPort, pm.DestinationPortSet, false)
		if err != nil {
			return nil, fmt.Errorf("destination port: %w", err)
		}
		matches = append(matches, m)
	}

	if pm.SourcePort != "" {
		m, err := parsePortMatch(pm.SourcePort, pm.SourcePortSet, true)
		if err != nil {
			return nil, fmt.Errorf("source port: %w", err)
		}
		matches = append(matches, m)
	}

	if pm.TCPFlags != "" {
		flags, mask, err := firewall.ParseTCPFlags(pm.TCPFlags)
		if err != nil {
			return nil, fmt.Errorf("tcp-flags: %w", err)
		}
		matches = append(matches, firewall.MatchTCPFlags{Flags: flags, Mask: mask})
	}

	return matches, nil
}

// parseAddressMatch builds the match for one address leaf. A @name value needs
// the set resolveSetReferences attached (set): without it the match would name
// a set the ze_pr table does not carry, which the verify never checked and the
// apply refuses, so it is refused here instead.
func parseAddressMatch(v string, set *firewall.Set, isSource bool) (firewall.Match, error) {
	if isSetReference(v) {
		setName := v[1:]
		if set == nil {
			return nil, fmt.Errorf("set %q was never resolved against the firewall configuration", setName)
		}
		field := firewall.SetFieldSourceAddr
		if !isSource {
			field = firewall.SetFieldDestAddr
		}
		return firewall.MatchInSet{SetName: setName, MatchField: field}, nil
	}
	prefix, err := netip.ParsePrefix(v)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix %q: %w", v, err)
	}
	if isSource {
		return firewall.MatchSourceAddress{Prefix: prefix}, nil
	}
	return firewall.MatchDestinationAddress{Prefix: prefix}, nil
}

// parsePortMatch builds the match for one port leaf, with the same @name rule
// as parseAddressMatch: a set reference needs the set resolveSetReferences
// attached.
func parsePortMatch(v string, set *firewall.Set, isSource bool) (firewall.Match, error) {
	if isSetReference(v) {
		setName := v[1:]
		if set == nil {
			return nil, fmt.Errorf("set %q was never resolved against the firewall configuration", setName)
		}
		field := firewall.SetFieldSourcePort
		if !isSource {
			field = firewall.SetFieldDestPort
		}
		return firewall.MatchInSet{SetName: setName, MatchField: field}, nil
	}
	ranges, err := firewall.ParsePortSpec(v)
	if err != nil {
		return nil, err
	}
	if isSource {
		return firewall.MatchSourcePort{Ranges: ranges}, nil
	}
	return firewall.MatchDestinationPort{Ranges: ranges}, nil
}

func (a *allocator) buildActions(policyName string, rule PolicyRule, basePriority *int) ([]firewall.Action, []iprule.Mark, []autoRouteSpec, error) {
	var actions []firewall.Action
	var ipRules []iprule.Mark
	var autoRoutes []autoRouteSpec

	if rule.Action.TCPMSS > 0 {
		actions = append(actions, firewall.SetTCPMSS{Size: rule.Action.TCPMSS})
	}

	switch rule.Action.Type {
	case 0:
		// A rule without a terminal action retains any TCP-MSS adjustment.
	case ActionAccept:
		actions = append(actions, firewall.Accept{})

	case ActionDrop:
		actions = append(actions, firewall.Drop{})

	case ActionTable:
		mark, err := a.allocateMark(markKey(policyName, rule.Action.Table))
		if err != nil {
			return nil, nil, nil, err
		}
		actions = append(actions, firewall.SetMark{Value: mark, Mask: 0xFFFFFFFF})
		ipRules = append(ipRules, iprule.Mark{
			Mark:     mark,
			Mask:     0xFFFFFFFF,
			Table:    rule.Action.Table,
			Priority: *basePriority,
		})
		*basePriority++

	case ActionNextHop:
		tbl, isNew, err := a.allocateTable(rule.Action.NextHop)
		if err != nil {
			return nil, nil, nil, err
		}
		mark, err := a.allocateMark(markKeyNextHop(policyName, rule.Action.NextHop))
		if err != nil {
			return nil, nil, nil, err
		}
		actions = append(actions, firewall.SetMark{Value: mark, Mask: 0xFFFFFFFF})
		ipRules = append(ipRules, iprule.Mark{
			Mark:     mark,
			Mask:     0xFFFFFFFF,
			Table:    tbl,
			Priority: *basePriority,
		})
		*basePriority++
		if isNew {
			autoRoutes = append(autoRoutes, autoRouteSpec{
				Table:   tbl,
				NextHop: rule.Action.NextHop,
			})
		}
	default:
		panic("BUG: policyroute: invalid action type")
	}

	return actions, ipRules, autoRoutes, nil
}
