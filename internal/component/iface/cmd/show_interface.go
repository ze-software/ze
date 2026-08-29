// Design: docs/features/interfaces.md -- `show interface` family handlers
// Related: show_neighbor.go -- sibling iface-owned show command
//
// Owned by the iface component: every handler here reads interface state
// through the iface backend (iface.ListInterfaces / GetInterface /
// DiscoverInterfaces). Relocated from the central cmd/show package so that
// removing iface removes the whole `show interface` surface. See
// ai/rules/plugins.md.

package cmd

import (
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface", Handler: handleShowInterface},
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface-brief", Handler: handleShowInterfaceBrief},
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface-type", Handler: handleShowInterfaceType},
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface-errors", Handler: handleShowInterfaceErrors},
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface-rate", Handler: handleShowInterfaceRateCmd},
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface-detail", Handler: handleShowInterfaceDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface-counters", Handler: handleShowInterfaceCounters},
		pluginserver.RPCRegistration{WireMethod: "ze-show:interface-scan", Handler: handleShowInterfaceScan},
	)
}

// usageShowInterface names every form of the command, for a caller who typed a
// token no handler here owns.
const usageShowInterface = "usage: show interface [brief | type <type> | errors | rate [<name>]] " +
	"or show interface name <name> detail|counters"

// handleShowInterface serves the bare `show interface`: every interface, full
// detail. Each subcommand has its OWN wire method and handler below.
//
// They must. The dispatcher registers one command key per YANG path, matches the
// LONGEST key first (Dispatcher.updateSortedKeys,
// internal/component/plugin/server/command.go), and hands the handler only the
// tokens AFTER that key. While `brief`, `type`, `errors` and `rate` shared this
// wire method they were alias paths of it, so the key ate the keyword and a
// switch on args[0] could never see it: `show interface errors` answered with
// every interface as if each one had errors.
func handleShowInterface(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return showInterfaceAll()
	}
	return &plugin.Response{Status: plugin.StatusError, Error: usageShowInterface}, nil
}

// handleShowInterfaceBrief serves `show interface brief`.
func handleShowInterfaceBrief(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return showInterfaceBrief()
}

// handleShowInterfaceType serves `show interface type <type>`. Without a type
// there is nothing to filter on.
//
// The type arrives as a SELECTOR rather than in args: the container and its leaf
// are both called `type`, so matchCommandTokens
// (internal/component/plugin/server/command.go) matches the keyword against the
// leaf of the same name and lifts the value out of the argument list. The args
// fallback keeps every caller that hands the value as a bare token working,
// which is the shape handleShowInterfaceDetail already takes.
func handleShowInterfaceType(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	wanted := ctx.Selector("type")
	if wanted == "" && len(args) > 0 {
		wanted = args[0]
	}
	if wanted == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: show interface type <type>"}, nil
	}
	return showInterfaceByType(wanted)
}

// handleShowInterfaceErrors serves `show interface errors`.
func handleShowInterfaceErrors(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return showInterfaceErrors()
}

// handleShowInterfaceRateCmd serves `show interface rate [<name>]`. The optional
// name is the one remaining token.
func handleShowInterfaceRateCmd(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return handleShowInterfaceRate(args)
}

func handleShowInterfaceDetail(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ""
	if ctx != nil {
		name = ctx.Selector("name")
	}
	if name == "" && len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: show interface name <name> detail"}, nil
	}
	return showInterfaceDetail(name)
}

func handleShowInterfaceCounters(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ""
	if ctx != nil {
		name = ctx.Selector("name")
	}
	if name == "" && len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: show interface name <name> counters"}, nil
	}
	return showInterfaceCounters(name)
}

func showInterfaceDetail(name string) (*plugin.Response, error) {
	info, err := iface.GetInterface(name)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: info}, nil
}

func showInterfaceCounters(name string) (*plugin.Response, error) {
	info, err := iface.GetInterface(name)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	if info.Stats == nil {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
			fieldName: info.Name,
			"stats":   "no counters available",
		}}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		fieldName: info.Name,
		"stats":   info.Stats,
	}}, nil
}

func showInterfaceAll() (*plugin.Response, error) {
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[iface.InterfaceInfo](ifaces)}, nil
}

// handleShowInterfaceScan discovers OS interfaces, classifies them by Ze
// type, and returns a JSON array of DiscoveredInterface. The interactive
// CLI pipe framework handles table/yaml/json rendering on the client side.
func handleShowInterfaceScan(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	discovered, err := iface.DiscoverInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[iface.DiscoveredInterface](discovered)}, nil
}

// showInterfaceByType filters the interface list to entries whose Type
// field matches (case-insensitive) the caller's argument. Unknown types
// reject with a sorted list of valid types derived from the running set.
func showInterfaceByType(wanted string) (*plugin.Response, error) {
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
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
		msg := "unknown interface type " + strconv.Quote(wanted)
		if len(valid) == 0 {
			msg += "; no interfaces have a classified type"
		} else {
			msg += "; valid types: " + textbuf.Join(valid, ", ")
		}
		return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil
	}
	// Single-key wrapper so the `| table` renderer unwraps to the
	// slice and produces a proper columnar table (see
	// internal/component/command/pipe_table.go renderValue). Count is
	// available via `| count`; the requested type is known to the
	// caller from the command line.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldInterfaces: filtered,
		},
	}, nil
}

// showInterfaceErrors returns the interfaces with any non-zero error or
// drop counter (RxErrors, RxDropped, TxErrors, TxDropped). Interfaces
// without stats are skipped.
func showInterfaceErrors() (*plugin.Response, error) {
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
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
			fieldName:    ifaces[i].Name,
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
		Data: plugin.Map{
			fieldInterfaces: rows,
		},
	}, nil
}

// showInterfaceBrief returns a compact one-line-per-interface summary.
func showInterfaceBrief() (*plugin.Response, error) {
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	rows := make([]map[string]any, 0, len(ifaces))
	for i := range ifaces {
		row := map[string]any{
			fieldName: ifaces[i].Name,
			"state":   ifaces[i].State,
			"mtu":     ifaces[i].MTU,
		}
		if len(ifaces[i].Addresses) > 0 {
			row["address"] = ifaces[i].Addresses[0].Address + textbuf.StrInt("/", int64(ifaces[i].Addresses[0].PrefixLength))
		}
		rows = append(rows, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldInterfaces: rows,
			"count":         len(rows),
		},
	}, nil
}
