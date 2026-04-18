// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration
// Detail: system.go -- system/* handlers (memory, cpu, date)

package show

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-memory",
			Handler:    handleShowSystemMemory,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-cpu",
			Handler:    handleShowSystemCPU,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-date",
			Handler:    handleShowSystemDate,
		},
	)
	// ze-show:host-* RPCs are registered from host.go's own init()
	// via a loop over host.SectionNames(). See rules/derive-not-hardcode.md.
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
// "type <type>" to filter by iface.InterfaceInfo.Type, "errors" to list
// interfaces with non-zero error/dropped counters, or "<name> counters"
// for RX/TX statistics only.
func handleShowInterface(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	// "show interface brief" -- compact one-line-per-interface.
	if len(args) > 0 && args[0] == "brief" {
		return showInterfaceBrief()
	}

	// "show interface type <type>" -- filter by interface type.
	if len(args) >= 2 && args[0] == "type" {
		return showInterfaceByType(args[1])
	}

	// "show interface errors" -- list ifaces with non-zero error/dropped counters.
	if len(args) > 0 && args[0] == "errors" {
		return showInterfaceErrors()
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

// showInterfaceByType filters the interface list to entries whose Type
// field matches (case-insensitive) the caller's argument. Unknown types
// reject with a sorted list of valid types derived from the running set.
func showInterfaceByType(wanted string) (*plugin.Response, error) {
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	wantedLower := strings.ToLower(wanted)
	seen := make(map[string]struct{})
	filtered := make([]iface.InterfaceInfo, 0, len(ifaces))
	for i := range ifaces {
		t := strings.ToLower(ifaces[i].Type)
		seen[t] = struct{}{}
		if t == wantedLower {
			filtered = append(filtered, ifaces[i])
		}
	}
	if len(filtered) == 0 {
		valid := make([]string, 0, len(seen))
		for t := range seen {
			if t != "" {
				valid = append(valid, t)
			}
		}
		sort.Strings(valid)
		msg := fmt.Sprintf("unknown interface type %q", wanted)
		if len(valid) == 0 {
			msg += "; no interfaces have a classified type"
		} else {
			msg += "; valid types: " + strings.Join(valid, ", ")
		}
		return &plugin.Response{Status: plugin.StatusError, Data: msg}, nil
	}
	// Single-key wrapper so the `| table` renderer unwraps to the
	// slice and produces a proper columnar table (see
	// internal/component/command/pipe_table.go renderValue). Count is
	// available via `| count`; the requested type is known to the
	// caller from the command line.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"interfaces": filtered,
		},
	}, nil
}

// showInterfaceErrors returns the interfaces with any non-zero error or
// drop counter (RxErrors, RxDropped, TxErrors, TxDropped). Interfaces
// without stats are skipped.
func showInterfaceErrors() (*plugin.Response, error) {
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	rows := make([]map[string]any, 0, len(ifaces))
	for i := range ifaces {
		s := ifaces[i].Stats
		if s == nil {
			continue
		}
		if s.RxErrors == 0 && s.RxDropped == 0 && s.TxErrors == 0 && s.TxDropped == 0 {
			continue
		}
		rows = append(rows, map[string]any{
			"name":       ifaces[i].Name,
			"rx-errors":  s.RxErrors,
			"rx-dropped": s.RxDropped,
			"tx-errors":  s.TxErrors,
			"tx-dropped": s.TxDropped,
		})
	}
	// Single-key wrapper so `| table` unwraps to the slice and renders
	// columnar output. Count is derivable via `| count`.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"interfaces": rows,
		},
	}, nil
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
