// Design: plan/learned/977-traffic-usage.md -- show traffic-usage command handler.
// Owned by the traffic-usage plugin so removing it removes the command, its
// schema (yang/ze-traffic-usage-cmd.yang), and this handler together. See
// ai/rules/plugins.md.

package trafficusage

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:traffic-usage",
			Handler:    handleShowTrafficUsage,
		},
	)
}

// handleShowTrafficUsage renders the current per-interface byte counters.
// Without arguments it lists every monitored interface; `name <interface>`
// filters to one. Reports not-configured when the plugin is idle.
func handleShowTrafficUsage(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	mon := getMonitor()
	if mon == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"status": "not-configured"},
		}, nil
	}

	snapshot := mon.Snapshot()

	switch len(args) {
	case 0:
		result := make(plugin.Slice[plugin.Map], 0, len(snapshot))
		for i := range snapshot {
			result = append(result, renderInterface(snapshot[i]))
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
	case 2:
		if args[0] == "name" {
			for i := range snapshot {
				if snapshot[i].ifname == args[1] {
					return &plugin.Response{Status: plugin.StatusDone, Data: renderInterface(snapshot[i])}, nil
				}
			}
			var tb textbuf.Buffer
			tb.Str("interface not monitored: ").Str(args[1])
			return &plugin.Response{Status: plugin.StatusError, Error: tb.String()}, nil
		}
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "usage: show traffic usage [name <interface>]",
	}, nil
}

// renderInterface converts one interface's counters into a structured map.
// Per-IP sections are present only when track-ip populated them.
func renderInterface(c counts) plugin.Map {
	m := plugin.Map{
		"interface":     c.ifname,
		"ingress-ports": renderPorts(c.ingressPort),
		"egress-ports":  renderPorts(c.egressPort),
	}
	if len(c.ingressIP) > 0 {
		m["ingress-ips"] = renderIPs(c.ingressIP)
	}
	if len(c.egressIP) > 0 {
		m["egress-ips"] = renderIPs(c.egressIP)
	}
	if len(c.mapEntries) > 0 {
		entries := make(plugin.Map, len(c.mapEntries))
		for name, n := range c.mapEntries {
			entries[name] = n
		}
		m["map-entries"] = entries
	}
	return m
}

func renderPorts(byKey map[portProto]uint64) plugin.Slice[plugin.Map] {
	out := make(plugin.Slice[plugin.Map], 0, len(byKey))
	for k, v := range byKey {
		out = append(out, plugin.Map{
			"port":     int(k.port),
			"protocol": protoName(k.proto),
			"bytes":    v,
		})
	}
	return out
}

func renderIPs(byKey map[uint32]uint64) plugin.Slice[plugin.Map] {
	out := make(plugin.Slice[plugin.Map], 0, len(byKey))
	for k, v := range byKey {
		out = append(out, plugin.Map{
			"ip":    ipString(k),
			"bytes": v,
		})
	}
	return out
}
