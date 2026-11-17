// Design: docs/architecture/container/network.md -- the translation tables
// Related: register_tables.go -- reserves the two kernel table names
// Related: apply.go -- sends the tables to the firewall component

package network

import (
	"encoding/json"
	"net/netip"
)

// The two tables the package contributes, as the firewall config section
// names them. The firewall component prefixes ze_, so the kernel holds
// ze_container and ze_container6.
const (
	tableIPv4          = "container"
	tableIPv6          = "container6"
	kernelTablePrefix  = "ze_"
	familyIPv4         = "ip"
	familyIPv6         = "ip6"
	chainPostrouting   = "postrouting"
	chainOutbound      = "outbound-"
	leafThen           = "then"
	termNetworkPrefix  = "network-"
	termBridge         = "bridge"
	termMasquerade     = "masquerade"
	priorityNATSource  = "100" // the nftables source translation priority
	chainTypeNAT       = "nat"
	chainPolicyAccept  = "accept"
	sectionFirewallKey = "firewall"
	sectionTableKey    = "table"
)

// buildTables answers the complete set of tables the config asks for, as a
// firewall config section: {"firewall":{"table":{...}}}. A network whose
// masquerade is true gets, in each family it has a prefix of, a postrouting
// term that sends traffic from its prefix to the network's own chain. That
// chain returns when the packet leaves by the network's bridge, so traffic
// between two containers of the network keeps its source, and masquerades
// otherwise. A family no such network uses has no table, and a config with
// none answers an empty table map, which withdraws both tables.
//
// The tables follow the config, not the run state: a stopped container's
// network keeps its translation.
func buildTables(cfg Config) (json.RawMessage, error) {
	tables := map[string]any{}
	for _, family := range []struct {
		table, family string
		ipv4          bool
	}{{tableIPv4, familyIPv4, true}, {tableIPv6, familyIPv6, false}} {
		table, used := familyTable(cfg, family.family, family.ipv4)
		if used {
			tables[family.table] = table
		}
	}
	return json.Marshal(map[string]any{sectionFirewallKey: map[string]any{sectionTableKey: tables}})
}

// familyTable answers the table of one family, and whether any network uses
// it.
func familyTable(cfg Config, family string, ipv4 bool) (map[string]any, bool) {
	// The term list is ordered-by user, and the slice form carries the order.
	var postrouting []any
	chains := map[string]any{}
	for _, network := range cfg.Networks {
		if !network.Masquerade {
			continue
		}
		prefix, found := prefixOfFamily(network.Prefixes, ipv4)
		if !found {
			continue
		}
		outbound := chainOutbound + network.Name
		postrouting = append(postrouting, map[string]any{
			leafName: termNetworkPrefix + network.Name,
			"from":   map[string]any{"source-address": prefix.String()},
			leafThen: map[string]any{"jump": outbound},
		})
		chains[outbound] = map[string]any{"term": []any{
			map[string]any{
				leafName: termBridge,
				"from":   map[string]any{"output-interface": network.Bridge},
				leafThen: map[string]any{"return": map[string]any{}},
			},
			map[string]any{
				leafName: termMasquerade,
				leafThen: map[string]any{"masquerade": map[string]any{}},
			},
		}}
	}
	if len(postrouting) == 0 {
		return nil, false
	}
	chains[chainPostrouting] = map[string]any{
		"type":     chainTypeNAT,
		"hook":     chainPostrouting,
		"priority": priorityNATSource,
		"policy":   chainPolicyAccept,
		"term":     postrouting,
	}
	return map[string]any{"family": family, "chain": chains}, true
}

// prefixOfFamily answers the prefix of prefixes in the asked family.
func prefixOfFamily(prefixes []netip.Prefix, ipv4 bool) (netip.Prefix, bool) {
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() == ipv4 {
			return prefix, true
		}
	}
	return netip.Prefix{}, false
}

// hasTables answers whether the config asks for any table.
func hasTables(cfg Config) bool {
	for _, network := range cfg.Networks {
		if network.Masquerade {
			return true
		}
	}
	return false
}
