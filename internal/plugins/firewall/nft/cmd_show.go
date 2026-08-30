// Design: docs/architecture/firewall/backend-command-dispatch.md -- nft firewall show handlers

package firewallnft

import (
	"sort"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyName is the response payload key carrying a table, chain or group name.
const keyName = "name"

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

func handleShowFirewallRuleset(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: show firewall ruleset <name>",
		}, nil
	}
	wanted := args[0]

	b := firewall.GetBackend()
	if b == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "firewall: no backend configured; firewall section absent from config",
		}, nil
	}
	backendName := firewall.ActiveBackendName()
	if backendName != "" && backendName != "nft" {
		var buf textbuf.Buffer
		buf.Str("firewall: backend \"").Str(backendName).Str("\" does not support ruleset readback; only nft is supported")
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  buf.String(),
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
		var buf textbuf.Buffer
		buf.Str("firewall: table \"").Str(wanted).Str("\" not found")
		if len(names) == 0 {
			buf.Str("; no firewall tables have been applied")
		} else {
			buf.Str("; valid: ").Str(textbuf.Join(names, ", "))
		}
		return &plugin.Response{Status: plugin.StatusError, Error: buf.String()}, nil
	}

	counters, err := b.GetCounters(target.Name)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error via Response
	}

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
				keyName:   name,
				"packets": tc.Packets,
				"bytes":   tc.Bytes,
			})
		}
		chainRows = append(chainRows, map[string]any{
			keyName:   ch.Name,
			"is-base": ch.IsBase,
			"hook":    ch.Hook.String(),
			"policy":  ch.Policy.String(),
			"terms":   termRows,
		})
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"table":  wanted,
			"family": target.Family.String(),
			"chains": chainRows,
		},
	}, nil
}

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
				keyName:   n,
				"tables":  tables,
				"members": total,
			})
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				"groups": list,
			},
		}, nil
	}

	wanted := args[0]
	entries, ok := groups[wanted]
	if !ok {
		var buf textbuf.Buffer
		buf.Str("firewall: group \"").Str(wanted).Str("\" not found")
		if len(names) == 0 {
			buf.Str("; no firewall groups have been applied")
		} else {
			buf.Str("; valid: ").Str(textbuf.Join(names, ", "))
		}
		return &plugin.Response{Status: plugin.StatusError, Error: buf.String()}, nil
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
		Data: plugin.Map{
			keyName:  wanted,
			"tables": perTable,
		},
	}, nil
}
