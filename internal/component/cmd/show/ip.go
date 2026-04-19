// Design: docs/guide/command-reference.md -- `show ip *` operational commands
// Related: show.go -- show verb RPC registration
// Related: system.go -- sibling `show system *` handlers

package show

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// defaultIPRouteLimit caps the number of entries returned by
// `show ip route` so an operator on a full DFZ does not turn a single
// read into a multi-hundred-megabyte RPC response. Callers who want
// more can raise the cap via `--limit N`; when the kernel FIB has
// more rows than the cap the handler trims the response and sets
// `truncated: true, limit: N` in the envelope so the caller can
// retry with a larger limit if desired.
const defaultIPRouteLimit = 100_000

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:ip-arp",
			Handler:    handleShowArp,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:ip-route",
			Handler:    handleShowIPRoute,
		},
	)
}

// handleShowArp returns the kernel neighbor table (IPv4 ARP + IPv6 ND)
// via the iface component's active backend. The optional `--family ipv4`
// or `--family ipv6` flag narrows the dump; no flag returns both.
//
// Unknown positional args (e.g. `show ip arp eth0`, anticipating a
// per-interface filter that does not exist today) reject with a
// usage line rather than silently returning the full table. Repeated
// `--family` rejects too, so `--family ipv4 --family ipv6` is not a
// last-wins surprise.
//
// Backends that cannot produce a neighbor table (VPP today) reject per
// exact-or-reject via iface.ListNeighbors; the error string carries the
// backend name so the operator knows what is unsupported.
func handleShowArp(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const usage = "usage: show ip arp [--family ipv4|ipv6|any]"
	family := iface.NeighborFamilyAny
	familySet := false
	for i := 0; i < len(args); i++ {
		if args[i] != "--family" {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("unknown argument %q; %s", args[i], usage),
			}, nil
		}
		if i+1 >= len(args) {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   "--family requires a value: ipv4, ipv6, or any",
			}, nil
		}
		if familySet {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   "--family given more than once",
			}, nil
		}
		switch strings.ToLower(args[i+1]) {
		case "ipv4":
			family = iface.NeighborFamilyIPv4
		case "ipv6":
			family = iface.NeighborFamilyIPv6
		case "any", "all":
			family = iface.NeighborFamilyAny
		default:
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("unknown family %q; valid: ipv4, ipv6, any", args[i+1]),
			}, nil
		}
		familySet = true
		i++
	}

	neighbors, err := iface.ListNeighbors(family)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error via Response
	}

	// Single-key wrapper so `| table` renders a proper columnar view;
	// `| count` returns the entry count.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"neighbors": neighbors,
		},
	}, nil
}

// handleShowIPRoute returns the kernel routing table. A single positional
// argument restricts the output to one CIDR; use "default" for the
// 0.0.0.0/0 / ::/0 entries. The optional `--limit N` caps the response
// size -- default 100 000 rows so a single read on a full DFZ does not
// produce a multi-hundred-megabyte RPC reply. Backends that do not own
// the kernel FIB (VPP today) reject per exact-or-reject -- kernel routes
// are not authoritative for the VPP fastpath and returning them would
// mislead operators.
//
// Invalid prefixes reject with the usage line rather than silently
// returning an empty result. "default" is accepted as a synonym for the
// 0.0.0.0/0 / ::/0 entries.
func handleShowIPRoute(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return dumpKernelRoutes(args, "usage: show ip route [<cidr>|default] [--limit N]", defaultIPRouteLimit)
}

// dumpKernelRoutes implements the shared argument parsing + truncation
// envelope for `show ip route` and `show kernel-routes`. Both entry
// points dump the same data; only their verb paths and usage strings
// differ. `defaultLimit` lets callers pick a cap appropriate for their
// audience (interactive operator vs. programmatic scrape).
func dumpKernelRoutes(args []string, usage string, defaultLimit int) (*plugin.Response, error) {
	filter := ""
	limit := defaultLimit
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--limit":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "--limit requires a value"}, nil
			}
			n, parseErr := strconv.Atoi(args[i+1])
			if parseErr != nil || n <= 0 {
				msg := fmt.Sprintf("invalid --limit %q: must be a positive integer", args[i+1])
				return &plugin.Response{Status: plugin.StatusError, Data: msg}, nil //nolint:nilerr // operational error via Response
			}
			limit = n
			i++
		case strings.HasPrefix(args[i], "--"):
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("unknown flag %q; %s", args[i], usage),
			}, nil
		default:
			if filter != "" {
				return &plugin.Response{
					Status: plugin.StatusError,
					Data:   fmt.Sprintf("too many positional arguments; %s", usage),
				}, nil
			}
			if args[i] != "default" {
				if _, err := netip.ParsePrefix(args[i]); err != nil {
					return &plugin.Response{
						Status: plugin.StatusError,
						Data:   fmt.Sprintf("invalid prefix %q: %v", args[i], err),
					}, nil
				}
			}
			filter = args[i]
		}
	}

	// Ask the backend for one more than `limit` so we can still detect
	// "there was more" without a separate count call; the backend stops
	// populating the Go slice once the cap is hit, bounding the real
	// allocation cost rather than just the response-size cost.
	routes, err := iface.ListKernelRoutes(filter, limit+1)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error via Response
	}
	truncated := false
	if len(routes) > limit {
		routes = routes[:limit]
		truncated = true
	}
	data := map[string]any{"routes": routes}
	if truncated {
		data["truncated"] = true
		data["limit"] = limit
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}
