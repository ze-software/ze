// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration

package show

import (
	"encoding/json"
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:version",
			Handler:    handleShowVersion,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:uptime",
			Handler:    handleShowUptime,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:warnings",
			Handler:    handleShowWarnings,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:errors",
			Handler:    handleShowErrors,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:interface",
			Handler:    handleShowInterface,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:interface-scan",
			Handler:    handleShowInterfaceScan,
		},
	)
}

// handleShowWarnings returns the snapshot of all active warnings on the report bus.
// Used by `ze show warnings`. Output is a JSON object with a sorted list and count.
func handleShowWarnings(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	issues := report.Warnings()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"warnings": issues,
			"count":    len(issues),
		},
	}, nil
}

// handleShowErrors returns the most-recent error events on the report bus,
// newest first. Used by `ze show errors`. The bus retains up to errorCap
// events; this handler returns all retained events.
func handleShowErrors(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	issues := report.Errors(0)
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"errors": issues,
			"count":  len(issues),
		},
	}, nil
}

// handleShowVersion returns the ze version and build date.
func handleShowVersion(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	v, d := pluginserver.GetVersion()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("ze %s (built %s)", v, d),
	}, nil
}

// handleShowUptime returns daemon start time and uptime duration.
func handleShowUptime(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "daemon not running",
		}, nil
	}
	r := ctx.Reactor()
	if r == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "daemon not running",
		}, nil
	}
	stats := r.Stats()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"start-time": stats.StartTime.Format(time.RFC3339),
			"uptime":     stats.Uptime.Truncate(time.Second).String(),
		},
	}, nil
}

// handleShowInterface lists all interfaces or shows one by name.
// Args: optional interface name, "brief" for one-line-per-interface summary,
// or "<name> counters" for RX/TX statistics only.
func handleShowInterface(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	// "show interface brief" -- compact one-line-per-interface.
	if len(args) > 0 && args[0] == "brief" {
		return showInterfaceBrief()
	}

	// "show interface <name> [counters]" -- single interface, optionally counters only.
	if len(args) > 0 {
		info, err := iface.GetInterface(args[0])
		if err != nil {
			return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
		}
		// "show interface <name> counters" -- just the stats.
		if len(args) > 1 && args[1] == "counters" {
			if info.Stats == nil {
				return &plugin.Response{Status: plugin.StatusDone, Data: map[string]any{
					"name":  info.Name,
					"stats": "no counters available",
				}}, nil
			}
			return &plugin.Response{Status: plugin.StatusDone, Data: map[string]any{
				"name":  info.Name,
				"stats": info.Stats,
			}}, nil
		}
		data, jsonErr := json.Marshal(info)
		if jsonErr != nil {
			return nil, fmt.Errorf("show interface: marshal: %w", jsonErr)
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
	}

	// "show interface" -- full list.
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	data, jsonErr := json.Marshal(ifaces)
	if jsonErr != nil {
		return nil, fmt.Errorf("show interface: marshal: %w", jsonErr)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}

// handleShowInterfaceScan discovers OS interfaces, classifies them by Ze
// type, and returns a JSON array of DiscoveredInterface. The interactive
// CLI pipe framework handles table/yaml/json rendering on the client side.
func handleShowInterfaceScan(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	discovered, err := iface.DiscoverInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	data, jsonErr := json.Marshal(discovered)
	if jsonErr != nil {
		return nil, fmt.Errorf("show interface scan: marshal: %w", jsonErr)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}

// showInterfaceBrief returns a compact one-line-per-interface summary.
func showInterfaceBrief() (*plugin.Response, error) {
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	rows := make([]map[string]any, 0, len(ifaces))
	for i := range ifaces {
		row := map[string]any{
			"name":  ifaces[i].Name,
			"state": ifaces[i].State,
			"mtu":   ifaces[i].MTU,
		}
		if len(ifaces[i].Addresses) > 0 {
			row["address"] = ifaces[i].Addresses[0].Address + "/" + fmt.Sprintf("%d", ifaces[i].Addresses[0].PrefixLength)
		}
		rows = append(rows, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"interfaces": rows,
			"count":      len(rows),
		},
	}, nil
}
