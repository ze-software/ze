// Design: docs/architecture/container/network.md -- the translation tables
// Related: register_tables.go -- reserves the two kernel table names
// Related: apply.go -- sends the tables to the firewall component
// Related: port.go -- the published ports the destination translation terms carry

package network

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"
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
	chainPrerouting    = "prerouting"
	chainOutput        = "output"
	chainOutbound      = "outbound-"
	leafFrom           = "from"
	leafThen           = "then"
	termNetworkPrefix  = "network-"
	termPortPrefix     = "port_"
	termSeparator      = "_" // no instance or port name holds it, so two term names never collide
	termBridge         = "bridge"
	termMasquerade     = "masquerade"
	priorityNATSource  = "100"  // the nftables source translation priority
	priorityNATTarget  = "-100" // the nftables destination translation priority
	chainTypeNAT       = "nat"
	chainPolicyAccept  = "accept"
	sectionFirewallKey = "firewall"
	sectionTableKey    = "table"
)

// buildTables answers the complete set of tables the config asks for, as a
// firewall config section: {"firewall":{"table":{...}}}. records holds each
// network's address assignments, which the published ports translate to.
//
// A network whose masquerade is true gets, in each family it has a prefix
// of, a postrouting term that sends traffic from its prefix to the network's
// own chain. That chain returns when the packet leaves by the network's
// bridge, so traffic between two containers of the network keeps its source,
// and masquerades otherwise. Only the first packet of a connection crosses a
// nat chain, so the replies of a published port's connections are never
// masqueraded: the kernel reverses their destination translation.
//
// A published port gets, in each family it is published in, one destination
// translation term at prerouting, for a client beyond the router, and the
// same term at output, for a connection the router itself opens. Each matches
// the protocol, the listen port, a destination that is an address of the
// router (destination-local, so traffic the router forwards to another host
// is never captured) and the listen address when one is set, then translates
// to the instance's address on its first network and the container port.
//
// A family nothing uses has no table, and a config with none answers an
// empty table map, which withdraws both tables. The tables follow the
// config, not the run state: a stopped, enabled container keeps its
// translation, and a disabled one has none.
func buildTables(cfg Config, records map[string]addressRecord) (json.RawMessage, error) {
	tables := map[string]any{}
	for _, family := range []struct {
		table, family string
		ipv4          bool
	}{{tableIPv4, familyIPv4, true}, {tableIPv6, familyIPv6, false}} {
		table, used, err := familyTable(cfg, records, family.family, family.ipv4)
		if err != nil {
			return nil, err
		}
		if used {
			tables[family.table] = table
		}
	}
	return json.Marshal(map[string]any{sectionFirewallKey: map[string]any{sectionTableKey: tables}})
}

// familyTable answers the table of one family, and whether anything uses it.
func familyTable(cfg Config, records map[string]addressRecord, family string, ipv4 bool) (map[string]any, bool, error) {
	chains := map[string]any{}
	postrouting := outboundTerms(cfg, ipv4, chains)
	published, err := publishedTerms(cfg, records, ipv4)
	if err != nil {
		return nil, false, err
	}
	if len(postrouting) > 0 {
		chains[chainPostrouting] = natChain(chainPostrouting, priorityNATSource, postrouting)
	}
	if len(published) > 0 {
		chains[chainPrerouting] = natChain(chainPrerouting, priorityNATTarget, published)
		chains[chainOutput] = natChain(chainOutput, priorityNATTarget, published)
	}
	if len(chains) == 0 {
		return nil, false, nil
	}
	return map[string]any{"family": family, "chain": chains}, true, nil
}

// natChain is a base chain of type nat at hook. The term list is ordered-by
// user, and the slice form carries the order.
func natChain(hook, priority string, terms []any) map[string]any {
	return map[string]any{
		"type":     chainTypeNAT,
		"hook":     hook,
		"priority": priority,
		"policy":   chainPolicyAccept,
		"term":     terms,
	}
}

// outboundTerms answers the postrouting terms of one family, and adds each
// translating network's own chain to chains.
func outboundTerms(cfg Config, ipv4 bool, chains map[string]any) []any {
	var postrouting []any
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
			leafFrom: map[string]any{"source-address": prefix.String()},
			leafThen: map[string]any{"jump": outbound},
		})
		chains[outbound] = map[string]any{"term": []any{
			map[string]any{
				leafName: termBridge,
				leafFrom: map[string]any{"output-interface": network.Bridge},
				leafThen: map[string]any{"return": map[string]any{}},
			},
			map[string]any{
				leafName: termMasquerade,
				leafThen: map[string]any{"masquerade": map[string]any{}},
			},
		}}
	}
	return postrouting
}

// publishedTerms answers the destination translation terms of one family,
// in instance and port name order. A disabled instance publishes nothing:
// the tables follow the config, and a disabled container is one the config
// says does not run, so a connection to its port is never translated to an
// address nothing holds.
func publishedTerms(cfg Config, records map[string]addressRecord, ipv4 bool) ([]any, error) {
	var terms []any
	for _, instance := range cfg.Instances {
		if !instance.publishes() {
			continue
		}
		network, _ := cfg.network(instance.Attachments[0].Network) // parseSections refused an attachment to an unknown network
		index := familyIndex(network.Prefixes, ipv4)
		if index < 0 {
			continue
		}
		given := records[network.Name].Instances[instance.Name]
		if index >= len(given.Addresses) {
			return nil, errors.New("container " + instance.Name + " has no assigned address on network " + network.Name)
		}
		target := given.Addresses[index]
		for _, port := range instance.Ports {
			terms = append(terms, portTerms(instance.Name, port, ipv4, target)...)
		}
	}
	return terms, nil
}

// portTerms answers one port's terms in one family: one with no address
// match when the port has no listen address, else one per listen address of
// the family.
func portTerms(instance string, port Port, ipv4 bool, target netip.Addr) []any {
	name := termPortPrefix + instance + termSeparator + port.Name
	if len(port.Listen) == 0 {
		return []any{portTerm(name, port, portMatch(port), target)}
	}
	var terms []any
	for position, address := range port.Listen {
		if address.Is4() != ipv4 {
			continue
		}
		match := portMatch(port)
		match["destination-address"] = netip.PrefixFrom(address, address.BitLen()).String()
		terms = append(terms, portTerm(name+termSeparator+strconv.Itoa(position+1), port, match, target))
	}
	return terms
}

// portMatch is the from block every term of a port holds.
func portMatch(port Port) map[string]any {
	return map[string]any{
		leafProtocol:        port.Protocol,
		"destination-port":  port.ListenPorts.String(),
		"destination-local": "", // an empty leaf: its presence is the whole value
	}
}

func portTerm(name string, port Port, match map[string]any, target netip.Addr) map[string]any {
	return map[string]any{
		leafName: name,
		leafFrom: match,
		leafThen: map[string]any{"dnat": map[string]any{"to": translationTarget(port, target)}},
	}
}

// translationTarget is the dnat target of a port. One container port is
// written with the address; a range carries the address alone, because
// checkRanges accepted only a range published onto itself, and the kernel
// keeps the destination port of a connection whose port is in the range.
func translationTarget(port Port, target netip.Addr) string {
	if port.ContainerPorts.size() == 1 {
		return netip.AddrPortFrom(target, port.ContainerPorts.first).String()
	}
	return target.String()
}

// prefixOfFamily answers the prefix of prefixes in the asked family.
func prefixOfFamily(prefixes []netip.Prefix, ipv4 bool) (netip.Prefix, bool) {
	index := familyIndex(prefixes, ipv4)
	if index < 0 {
		return netip.Prefix{}, false
	}
	return prefixes[index], true
}

// familyIndex answers the position of the prefix of the asked family, or -1.
func familyIndex(prefixes []netip.Prefix, ipv4 bool) int {
	for index, prefix := range prefixes {
		if prefix.Addr().Is4() == ipv4 {
			return index
		}
	}
	return -1
}

// hasTables answers whether the config asks for any table.
func hasTables(cfg Config) bool {
	for _, network := range cfg.Networks {
		if network.Masquerade {
			return true
		}
	}
	for _, instance := range cfg.Instances {
		if instance.publishes() {
			return true
		}
	}
	return false
}

// publishes answers whether the instance's ports are translated: it is
// enabled and has at least one. A disabled instance's ports are still judged
// at verify, so the config stays valid when it is enabled again.
func (i Instance) publishes() bool {
	if !i.Enabled {
		return false
	}
	return len(i.Ports) > 0
}
