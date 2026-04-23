package policyroute

import (
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

const policyRoutingTable = "ze_pr"

type translationResult struct {
	Tables     []firewall.Table
	IPRules    []ipRuleSpec
	AutoRoutes []autoRouteSpec
}

type ipRuleSpec struct {
	Mark     uint32
	Mask     uint32
	Table    uint32
	Priority int
}

type autoRouteSpec struct {
	Table   uint32
	NextHop netip.Addr
}

func (a *allocator) translate(policies []PolicyRoute) (*translationResult, error) {
	result := &translationResult{}
	basePriority := 100

	chain := firewall.Chain{
		Name:     "prerouting",
		IsBase:   true,
		Type:     firewall.ChainRoute,
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
		}}
	}

	return result, nil
}

func (a *allocator) translatePolicy(policy PolicyRoute, basePriority *int) ([]firewall.Term, []ipRuleSpec, []autoRouteSpec, error) {
	var terms []firewall.Term
	var ipRules []ipRuleSpec
	var autoRoutes []autoRouteSpec

	for _, rule := range policy.Rules {
		matches, err := buildMatches(rule, policy.Interfaces)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("rule %q: %w", rule.Name, err)
		}

		actions, ruleSpecs, routeSpecs, err := a.buildActions(policy.Name, rule, basePriority)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("rule %q: %w", rule.Name, err)
		}

		terms = append(terms, firewall.Term{
			Name:    policy.Name + "-" + rule.Name,
			Matches: matches,
			Actions: actions,
		})

		ipRules = append(ipRules, ruleSpecs...)
		autoRoutes = append(autoRoutes, routeSpecs...)
	}

	return terms, ipRules, autoRoutes, nil
}

func buildMatches(rule PolicyRule, interfaces []InterfaceSpec) ([]firewall.Match, error) {
	var matches []firewall.Match

	for _, iface := range interfaces {
		matches = append(matches, firewall.MatchInputInterface{
			Name:     iface.Name,
			Wildcard: iface.Wildcard,
		})
	}

	m, err := buildPolicyMatch(rule.Match)
	if err != nil {
		return nil, err
	}
	matches = append(matches, m...)

	return matches, nil
}

func buildPolicyMatch(pm PolicyMatch) ([]firewall.Match, error) {
	var matches []firewall.Match

	if pm.SourceAddress != "" {
		m, err := parseAddressMatch(pm.SourceAddress, true)
		if err != nil {
			return nil, fmt.Errorf("source address: %w", err)
		}
		matches = append(matches, m)
	}

	if pm.DestinationAddress != "" {
		m, err := parseAddressMatch(pm.DestinationAddress, false)
		if err != nil {
			return nil, fmt.Errorf("destination address: %w", err)
		}
		matches = append(matches, m)
	}

	if pm.Protocol != "" {
		matches = append(matches, firewall.MatchProtocol{Protocol: pm.Protocol})
	}

	if pm.DestinationPort != "" {
		ranges, err := firewall.ParsePortSpec(pm.DestinationPort)
		if err != nil {
			return nil, fmt.Errorf("destination port: %w", err)
		}
		matches = append(matches, firewall.MatchDestinationPort{Ranges: ranges})
	}

	if pm.SourcePort != "" {
		ranges, err := firewall.ParsePortSpec(pm.SourcePort)
		if err != nil {
			return nil, fmt.Errorf("source port: %w", err)
		}
		matches = append(matches, firewall.MatchSourcePort{Ranges: ranges})
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

func parseAddressMatch(v string, isSource bool) (firewall.Match, error) {
	if strings.HasPrefix(v, "@") {
		setName := v[1:]
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

func (a *allocator) buildActions(policyName string, rule PolicyRule, basePriority *int) ([]firewall.Action, []ipRuleSpec, []autoRouteSpec, error) {
	var actions []firewall.Action
	var ipRules []ipRuleSpec
	var autoRoutes []autoRouteSpec

	if rule.Action.TCPMSS > 0 {
		actions = append(actions, firewall.SetTCPMSS{Size: rule.Action.TCPMSS})
	}

	switch rule.Action.Type {
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
		ipRules = append(ipRules, ipRuleSpec{
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
		ipRules = append(ipRules, ipRuleSpec{
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
	}

	return actions, ipRules, autoRoutes, nil
}
