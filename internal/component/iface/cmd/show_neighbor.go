// Design: docs/guide/command-reference.md -- `show neighbor` / `show arp`.
// Owned by the iface component: both read the kernel neighbor table (IPv4 ARP
// + IPv6 ND) through the iface backend (iface.ListNeighbors). `show arp` is the
// IPv4 alias for `show neighbor ipv4`. See ai/rules/plugins.md
// and docs/architecture/cli/command-namespacing.md (object-rooted commands).

package cmd

import (
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:neighbor",
			Handler:    handleShowNeighbor,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:arp",
			Handler:    handleShowArp,
		},
	)
}

// parseNeighborFamily maps a user family token (ipv4/ipv6/any/all) to an
// iface.NeighborFamily. The bool is false for an unrecognized token so the
// caller can produce a usage error. The CLI is case-sensitive and the
// dispatcher validates the family enum (lowercase) against the YANG leaf before
// this handler runs, so the switch only ever sees a valid lowercase token.
func parseNeighborFamily(s string) (int, bool) {
	switch s {
	case "ipv4":
		return iface.NeighborFamilyIPv4, true
	case "ipv6":
		return iface.NeighborFamilyIPv6, true
	case "any", "all":
		return iface.NeighborFamilyAny, true
	default:
		return iface.NeighborFamilyAny, false
	}
}

// handleShowNeighbor returns the kernel neighbor table (IPv4 ARP + IPv6 ND)
// via the iface component's active backend. Accepts an optional positional
// "ipv4", "ipv6", or "any" argument to narrow the dump; no argument returns
// both families.
//
// Unknown positional args (e.g. `show neighbor eth0`, anticipating a
// per-interface filter that does not exist today) reject with a usage line
// rather than silently returning the full table. Backends that cannot produce
// a neighbor table (VPP today) reject per exact-or-reject via
// iface.ListNeighbors; the error string carries the backend name so the
// operator knows what is unsupported.
func handleShowNeighbor(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const usage = "usage: show neighbor [ipv4|ipv6|any]"
	var tb textbuf.Buffer
	family := iface.NeighborFamilyAny
	switch len(args) {
	case 0:
		// default: both families
	case 1:
		f, ok := parseNeighborFamily(args[0])
		if !ok {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  tb.Str("unknown family ").Quoted(args[0]).Str("; valid: ipv4, ipv6, any").String(),
			}, nil
		}
		family = f
	default:
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("too many arguments; ").Str(usage).String(),
		}, nil
	}
	return neighborResponse(family)
}

// handleShowArp is the IPv4 alias for `show neighbor ipv4`. ARP is an IPv4
// protocol, so this view never shows IPv6 ND entries; use `show neighbor` for
// both families or `show neighbor ipv6` for ND. It takes no argument.
func handleShowArp(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) > 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "unexpected argument; 'show arp' shows the IPv4 table -- use 'show neighbor ipv6' for IPv6 ND",
		}, nil
	}
	return neighborResponse(iface.NeighborFamilyIPv4)
}

// neighborResponse dumps the kernel neighbor table for the given family and
// wraps it under the single `neighbors` key so `| table` renders a columnar
// view and `| count` returns the entry count.
func neighborResponse(family int) (*plugin.Response, error) {
	neighbors, err := iface.ListNeighbors(family)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error via Response
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"neighbors": neighbors,
		},
	}, nil
}
