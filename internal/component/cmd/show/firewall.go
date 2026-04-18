// Design: docs/guide/command-reference.md -- `show firewall *` operational commands
// Related: show.go -- show verb RPC registration
// Related: ip.go -- sibling `show ip *` handlers

package show

import (
	"fmt"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:firewall-ruleset",
			Handler:    handleShowFirewallRuleset,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:firewall-group",
			Handler:    handleShowFirewallGroup,
		},
	)
}

// handleShowFirewallRuleset returns the per-rule packet/byte counters for
// a named table, joined with the applied desired state so operators see
// both what was configured and how much traffic has hit each term.
//
// Under exact-or-reject:
//   - no backend loaded -> reject (firewall plugin idle)
//   - active backend is not nft -> reject with the backend name
//   - table not present in the last applied snapshot -> reject with the
//     sorted list of known table names
//
// The positional argument is the bare table name (without the "ze_"
// prefix that the kernel carries); unknown names reject.
func handleShowFirewallRuleset(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: show firewall ruleset <name>",
		}, nil
	}
	wanted := args[0]

	b := firewall.GetBackend()
	if b == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "firewall: no backend configured; firewall section absent from config",
		}, nil
	}
	backendName := firewall.ActiveBackendName()
	if backendName != "" && backendName != "nft" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("firewall: backend %q does not support ruleset readback; only nft is supported", backendName),
		}, nil
	}

	applied := firewall.LastApplied()
	var target *firewall.Table
	names := make([]string, 0, len(applied))
	for i := range applied {
		bare := firewall.StripZeTablePrefix(applied[i].Name)
		names = append(names, bare)
		if bare == wanted {
			target = &applied[i]
		}
	}
	if target == nil {
		sort.Strings(names)
		msg := fmt.Sprintf("firewall: table %q not found", wanted)
		if len(names) == 0 {
			msg += "; no firewall tables have been applied"
		} else {
			msg += "; valid: " + strings.Join(names, ", ")
		}
		return &plugin.Response{Status: plugin.StatusError, Data: msg}, nil
	}

	counters, err := b.GetCounters(target.Name)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error via Response
	}

	// Index counters by chain name, then by term name, so we can join
	// O(chains * terms) cleanly against the desired-state chains/terms.
	byChain := make(map[string]map[string]firewall.TermCounter, len(counters))
	for i := range counters {
		m := make(map[string]firewall.TermCounter, len(counters[i].Terms))
		for _, tc := range counters[i].Terms {
			if tc.Name == "" {
				continue
			}
			m[tc.Name] = tc
		}
		byChain[counters[i].Chain] = m
	}

	chainRows := make([]map[string]any, 0, len(target.Chains))
	for i := range target.Chains {
		ch := &target.Chains[i]
		ctrs := byChain[ch.Name]
		termRows := make([]map[string]any, 0, len(ch.Terms))
		for j := range ch.Terms {
			name := ch.Terms[j].Name
			tc := ctrs[name]
			termRows = append(termRows, map[string]any{
				"name":    name,
				"packets": tc.Packets,
				"bytes":   tc.Bytes,
			})
		}
		chainRows = append(chainRows, map[string]any{
			"name":    ch.Name,
			"is-base": ch.IsBase,
			"hook":    ch.Hook.String(),
			"policy":  ch.Policy.String(),
			"terms":   termRows,
		})
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"table":  wanted,
			"family": target.Family.String(),
			"chains": chainRows,
		},
	}, nil
}

// handleShowFirewallGroup returns the members of a named firewall set
// (ze's equivalent of VyOS firewall groups). No kernel readback is
// required -- the applied desired state carries the set definition.
//
// A bare invocation with no argument lists every known group name
// across every applied table; a positional argument narrows to one.
func handleShowFirewallGroup(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	applied := firewall.LastApplied()
	type groupEntry struct {
		table string
		set   *firewall.Set
	}
	groups := make(map[string][]groupEntry)
	names := make([]string, 0)
	for i := range applied {
		tbl := &applied[i]
		for j := range tbl.Sets {
			name := tbl.Sets[j].Name
			if _, seen := groups[name]; !seen {
				names = append(names, name)
			}
			groups[name] = append(groups[name], groupEntry{
				table: firewall.StripZeTablePrefix(tbl.Name),
				set:   &tbl.Sets[j],
			})
		}
	}
	sort.Strings(names)

	if len(args) == 0 {
		list := make([]map[string]any, 0, len(names))
		for _, n := range names {
			entries := groups[n]
			tables := make([]string, 0, len(entries))
			total := 0
			for _, e := range entries {
				tables = append(tables, e.table)
				total += len(e.set.Elements)
			}
			list = append(list, map[string]any{
				"name":    n,
				"tables":  tables,
				"members": total,
			})
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"groups": list,
			},
		}, nil
	}

	wanted := args[0]
	entries, ok := groups[wanted]
	if !ok {
		msg := fmt.Sprintf("firewall: group %q not found", wanted)
		if len(names) == 0 {
			msg += "; no firewall groups have been applied"
		} else {
			msg += "; valid: " + strings.Join(names, ", ")
		}
		return &plugin.Response{Status: plugin.StatusError, Data: msg}, nil
	}
	perTable := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		elems := make([]string, 0, len(e.set.Elements))
		for _, el := range e.set.Elements {
			elems = append(elems, el.Value)
		}
		perTable = append(perTable, map[string]any{
			"table":    e.table,
			"type":     int(e.set.Type),
			"elements": elems,
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"name":   wanted,
			"tables": perTable,
		},
	}, nil
}
