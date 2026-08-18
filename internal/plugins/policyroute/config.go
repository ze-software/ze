// Design: docs/architecture/policyroute/policy-routing.md -- config parsing

package policyroute

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errTcpMssValueMustBe1   = errors.New("tcp-mss: value must be 1-65535, got 0")
	errTableValueMustBe1Got = errors.New("table: value must be >= 1, got 0")
)

const (
	tableReservedMin = 1000
	tableReservedMax = 2999

	// maxEncodableTable is the largest table value this build can program.
	// netlink.Rule.Table is a Go int and the encoder emits FRA_TABLE only for
	// Table >= 256 and the compat byte only for 0 <= Table < 256
	// (vendor/github.com/vishvananda/netlink/rule_linux.go:57,126), so on a
	// 32-bit build a value above MaxInt32 turns negative and the rule is
	// installed with RT_TABLE_UNSPEC instead of the operator's table, with no
	// error anywhere. On the 64-bit targets Ze ships this bound is above every
	// uint32 and never bites, so the full kernel-legal range stays available.
	//
	// This tracks the build's own int, exactly like its two siblings
	// (internal/core/routingtable/registry.go maxEncodableTableID,
	// internal/plugins/static/config.go maxNetlinkInt), which bound the same
	// netlink int-typed fields.
	//
	// Do NOT narrow this to a literal math.MaxInt32 to quiet CodeQL's
	// go/incorrect-integer-conversion. That was tried (alert 171) and it costs
	// table IDs 2^31..2^32-1 that the kernel accepts and every shipped target
	// can program; test/parse/netlink-int-field-range.ci pins the full range.
	// The alert is about the int conversion in newIPRule, not about this
	// constant, and it is answered there: the conversion lives in
	// architecture-constrained files where int is known to be 64-bit
	// (netlinkint_linux_amd64.go, netlinkint_linux_arm64.go) and the narrow
	// bound applies only on the builds that need it (netlinkint_linux_generic.go).
	maxEncodableTable = uint64(math.MaxInt)
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
		match, err := parsePolicyMatch(fromMap)
		if err != nil {
			return PolicyRule{}, fmt.Errorf("from: %w", err)
		}
		rule.Match = match
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

// parsePolicyMatch reads the "from" block. It refuses a protocol the firewall
// backends cannot lower, so the operator learns at commit rather than at the
// next reconcile -- where the failure is not local to this rule, because the
// nft backend returns a lowering error before its single Flush and leaves
// every firewall owner's ruleset unapplied in the kernel.
func parsePolicyMatch(m map[string]any) (PolicyMatch, error) {
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
	if v, ok := m["protocol"].(string); ok && v != "" {
		if _, known := firewall.ProtocolNumber(v); !known {
			return PolicyMatch{}, fmt.Errorf("protocol: %q is not a protocol the firewall can match; accepted values are %s",
				v, strings.Join(firewall.ProtocolNames(), ", "))
		}
		pm.Protocol = v
	}
	if v, ok := m["tcp-flags"].(string); ok {
		pm.TCPFlags = v
	}
	return pm, nil
}

// validateActionTable rejects table values Ze must not program (0, the kernel
// system tables, the ze-reserved range) and values this build cannot program
// without truncation. maxEncodable is a parameter, not the constant, so the
// 32-bit rejection stays testable on a 64-bit host where it can never be hit.
func validateActionTable(tbl, maxEncodable uint64) error {
	if tbl == 0 {
		return errTableValueMustBe1Got
	}
	if tbl >= 253 && tbl <= 255 {
		return fmt.Errorf("table: value %d is a kernel system table (253=default, 254=main, 255=local)", tbl)
	}
	if tbl >= tableReservedMin && tbl <= tableReservedMax {
		return fmt.Errorf("table: value %d is in ze-reserved range %d-%d", tbl, tableReservedMin, tableReservedMax)
	}
	if tbl > maxEncodable {
		return fmt.Errorf("table: value %d exceeds %d, the largest this build can program through netlink", tbl, maxEncodable)
	}
	return nil
}

func parsePolicyAction(m map[string]any) (PolicyAction, error) {
	var action PolicyAction

	if v, ok := m["tcp-mss"].(string); ok {
		mss, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return PolicyAction{}, fmt.Errorf("tcp-mss: invalid value %q: %w", v, err)
		}
		if mss == 0 {
			return PolicyAction{}, errTcpMssValueMustBe1
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
		return PolicyAction{}, fmt.Errorf("conflicting actions: %s (only one terminal action allowed per rule)", textbuf.Join(terminals, ", "))
	}

	if v, ok := m["table"].(string); ok {
		tbl, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return PolicyAction{}, fmt.Errorf("table: invalid value %q: %w", v, err)
		}
		if err := validateActionTable(tbl, maxEncodableTable); err != nil {
			return PolicyAction{}, err
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
