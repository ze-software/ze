// Design: docs/architecture/api/commands.md — show flow export handler.
// Owned by the flow-export component so that removing it removes the
// `show flow export` command, its schema, and this handler together. See
// ai/rules/plugin-self-containment.md.

package flowexport

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:flow-export",
			Handler:    handleShowFlowExport,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:flow-recent",
			Handler:    handleShowFlowRecent,
		},
	)
}

func handleShowFlowExport(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	exp := getExporter()
	if exp == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"status": "not-configured"},
		}, nil
	}

	collectors := exp.status()
	switch len(args) {
	case 0:
		result := make([]plugin.Map, 0, len(collectors))
		for _, c := range collectors {
			result = append(result, plugin.Map(c))
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[plugin.Map](result)}, nil
	case 2:
		if args[0] == "name" {
			name := args[1]
			for _, c := range collectors {
				if n, ok := c["name"].(string); ok && n == name {
					return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(c)}, nil
				}
			}
			return &plugin.Response{Status: plugin.StatusError, Error: "collector not found: " + name}, nil
		}
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "usage: show flow export [name <name>]",
	}, nil
}

// handleShowFlowRecent returns recent conntrack flow records from the bounded
// recent-flow ring. Without arguments it returns every ring record (oldest to
// newest, up to the configured ring capacity); `dst <prefix>` filters to flows
// whose destination is inside that prefix -- the shape the DDoS characterizer
// uses to inspect flows to a victim. Reports not-configured when no exporter is
// active. The ring is fed only while conntrack export is enabled.
//
// The filter is by destination prefix, not interface: conntrack is host-global
// and carries no ingress interface, so a `name <iface>` filter is not derivable
// from the data (see spec Deviations).
func handleShowFlowRecent(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	exp := getExporter()
	if exp == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"status": "not-configured"},
		}, nil
	}

	var dst netip.Prefix
	switch len(args) {
	case 0:
	case 2:
		if args[0] != "dst" {
			return flowRecentUsage(), nil
		}
		p, ok := parseDstPrefix(args[1])
		if !ok {
			var tb textbuf.Buffer
			tb.Str("invalid dst prefix: ").Str(args[1])
			return &plugin.Response{Status: plugin.StatusError, Error: tb.String()}, nil
		}
		dst = p
	default:
		return flowRecentUsage(), nil
	}

	flows := exp.recentFlows(dst)
	result := make(plugin.Slice[plugin.Map], 0, len(flows))
	for i := range flows {
		result = append(result, renderFlow(flows[i]))
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
}

func flowRecentUsage() *plugin.Response {
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "usage: show flow recent [dst <prefix>]",
	}
}

// parseDstPrefix accepts either a CIDR ("203.0.113.0/24") or a bare address
// ("203.0.113.42", treated as a host /32 or /128).
func parseDstPrefix(s string) (netip.Prefix, bool) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return p.Masked(), true
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return netip.PrefixFrom(a, a.BitLen()), true
	}
	return netip.Prefix{}, false
}

// renderFlow converts one ConntrackFlow into a structured map. Field names are
// the parse contract consumed by the DDoS characterizer (detect/characterize.go).
func renderFlow(f ConntrackFlow) plugin.Map {
	m := plugin.Map{
		"src-addr":  f.SrcAddr.String(),
		"dst-addr":  f.DstAddr.String(),
		"src-port":  int(f.SrcPort),
		"dst-port":  int(f.DstPort),
		"protocol":  int(f.Protocol),
		"bytes":     f.Bytes,
		"packets":   f.Packets,
		"tcp-state": int(f.TCPState),
		"first-ms":  f.FirstMs,
		"last-ms":   f.LastMs,
	}
	if f.SrcAS != 0 {
		m["src-as"] = f.SrcAS
	}
	if f.DstAS != 0 {
		m["dst-as"] = f.DstAS
	}
	return m
}
