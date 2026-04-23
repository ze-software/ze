package policyroute

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

const (
	tableReservedMin = 1000
	tableReservedMax = 2999
)

func parsePolicyConfig(jsonData string) ([]PolicyRoute, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal policy config: %w", err)
	}

	policyTree, ok := tree["policy"].(map[string]any)
	if !ok {
		return nil, nil
	}

	routeMap, ok := policyTree["route"].(map[string]any)
	if !ok {
		return nil, nil
	}

	policyNames := make([]string, 0, len(routeMap))
	for name := range routeMap {
		policyNames = append(policyNames, name)
	}
	sort.Strings(policyNames)

	var policies []PolicyRoute
	for _, name := range policyNames {
		m, ok := routeMap[name].(map[string]any)
		if !ok {
			continue
		}
		pr, err := parsePolicyRoute(name, m)
		if err != nil {
			return nil, fmt.Errorf("policy route %q: %w", name, err)
		}
		policies = append(policies, pr)
	}
	return policies, nil
}

func parsePolicyRoute(name string, m map[string]any) (PolicyRoute, error) {
	if err := firewall.ValidateName(name); err != nil {
		return PolicyRoute{}, fmt.Errorf("policy route name: %w", err)
	}
	pr := PolicyRoute{Name: name}

	if v, ok := m["interface"].(string); ok {
		pr.Interfaces = append(pr.Interfaces, parseIfaceSpec(v))
	}
	if list, ok := m["interface"].([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				pr.Interfaces = append(pr.Interfaces, parseIfaceSpec(s))
			}
		}
	}

	ruleMap, ok := m["rule"].(map[string]any)
	if !ok {
		return pr, nil
	}

	for rName, rv := range ruleMap {
		rm, ok := rv.(map[string]any)
		if !ok {
			continue
		}
		rule, err := parsePolicyRule(rName, rm)
		if err != nil {
			return PolicyRoute{}, fmt.Errorf("rule %q: %w", rName, err)
		}
		pr.Rules = append(pr.Rules, rule)
	}

	sort.Slice(pr.Rules, func(i, j int) bool {
		if pr.Rules[i].Order != pr.Rules[j].Order {
			return pr.Rules[i].Order < pr.Rules[j].Order
		}
		return pr.Rules[i].Name < pr.Rules[j].Name
	})

	return pr, nil
}

func parseIfaceSpec(v string) InterfaceSpec {
	if strings.HasSuffix(v, "*") {
		return InterfaceSpec{Name: v[:len(v)-1], Wildcard: true}
	}
	return InterfaceSpec{Name: v, Wildcard: false}
}

func parsePolicyRule(name string, m map[string]any) (PolicyRule, error) {
	if err := firewall.ValidateName(name); err != nil {
		return PolicyRule{}, fmt.Errorf("rule name: %w", err)
	}
	rule := PolicyRule{Name: name}

	if v, ok := m["order"].(string); ok {
		order, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return PolicyRule{}, fmt.Errorf("order: invalid value %q: %w", v, err)
		}
		rule.Order = uint32(order)
	}

	if fromMap, ok := m["from"].(map[string]any); ok {
		rule.Match = parsePolicyMatch(fromMap)
	}

	if thenMap, ok := m["then"].(map[string]any); ok {
		action, err := parsePolicyAction(thenMap)
		if err != nil {
			return PolicyRule{}, fmt.Errorf("then: %w", err)
		}
		rule.Action = action
	}

	return rule, nil
}

func parsePolicyMatch(m map[string]any) PolicyMatch {
	var pm PolicyMatch
	if v, ok := m["source-address"].(string); ok {
		pm.SourceAddress = v
	}
	if v, ok := m["destination-address"].(string); ok {
		pm.DestinationAddress = v
	}
	if v, ok := m["source-port"].(string); ok {
		pm.SourcePort = v
	}
	if v, ok := m["destination-port"].(string); ok {
		pm.DestinationPort = v
	}
	if v, ok := m["protocol"].(string); ok {
		pm.Protocol = v
	}
	if v, ok := m["tcp-flags"].(string); ok {
		pm.TCPFlags = v
	}
	return pm
}

func parsePolicyAction(m map[string]any) (PolicyAction, error) {
	var action PolicyAction

	if v, ok := m["tcp-mss"].(string); ok {
		mss, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return PolicyAction{}, fmt.Errorf("tcp-mss: invalid value %q: %w", v, err)
		}
		if mss == 0 {
			return PolicyAction{}, fmt.Errorf("tcp-mss: value must be 1-65535, got 0")
		}
		action.TCPMSS = uint16(mss)
	}

	var terminals []string

	if _, ok := m["table"]; ok {
		terminals = append(terminals, "table")
	}
	if _, ok := m["next-hop"]; ok {
		terminals = append(terminals, "next-hop")
	}
	if _, ok := m["accept"]; ok {
		terminals = append(terminals, "accept")
	}
	if _, ok := m["drop"]; ok {
		terminals = append(terminals, "drop")
	}

	if len(terminals) > 1 {
		return PolicyAction{}, fmt.Errorf("conflicting actions: %s (only one terminal action allowed per rule)", strings.Join(terminals, ", "))
	}

	if v, ok := m["table"].(string); ok {
		tbl, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return PolicyAction{}, fmt.Errorf("table: invalid value %q: %w", v, err)
		}
		if tbl == 0 {
			return PolicyAction{}, fmt.Errorf("table: value must be >= 1, got 0")
		}
		if tbl >= 253 && tbl <= 255 {
			return PolicyAction{}, fmt.Errorf("table: value %d is a kernel system table (253=default, 254=main, 255=local)", tbl)
		}
		if tbl >= tableReservedMin && tbl <= tableReservedMax {
			return PolicyAction{}, fmt.Errorf("table: value %d is in ze-reserved range %d-%d", tbl, tableReservedMin, tableReservedMax)
		}
		action.Type = ActionTable
		action.Table = uint32(tbl)
		return action, nil
	}

	if v, ok := m["next-hop"].(string); ok {
		addr, err := netip.ParseAddr(v)
		if err != nil {
			return PolicyAction{}, fmt.Errorf("next-hop: invalid address %q: %w", v, err)
		}
		if !addr.Is4() {
			return PolicyAction{}, fmt.Errorf("next-hop: IPv6 not yet supported (%s)", v)
		}
		action.Type = ActionNextHop
		action.NextHop = addr
		return action, nil
	}

	if _, ok := m["accept"]; ok {
		action.Type = ActionAccept
		return action, nil
	}

	if _, ok := m["drop"]; ok {
		action.Type = ActionDrop
		return action, nil
	}

	return action, nil
}
