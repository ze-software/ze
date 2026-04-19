// Design: docs/guide/command-reference.md -- `show neighbors` top-level shortcut
// Related: ip.go -- `show ip arp` does the same work under the ip/ subtree

package show

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:neighbors",
			Handler:    handleShowNeighbors,
		},
	)
}

// handleShowNeighbors returns the kernel neighbor table (IPv4 ARP +
// IPv6 ND) via the iface component's active backend. Accepts an
// optional positional "ipv4", "ipv6", or "any" argument to narrow the
// dump; no argument returns both families. Differs from the sibling
// handleShowArp (under `show ip arp`) only in argument shape -- the
// short form is positional, the long form uses --family.
//
// Backends that cannot produce a neighbor table (VPP today) reject per
// exact-or-reject via iface.ListNeighbors.
func handleShowNeighbors(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const usage = "usage: show neighbors [ipv4|ipv6|any]"
	family := iface.NeighborFamilyAny
	switch len(args) {
	case 0:
		// default: both families
	case 1:
		switch strings.ToLower(args[0]) {
		case "ipv4":
			family = iface.NeighborFamilyIPv4
		case "ipv6":
			family = iface.NeighborFamilyIPv6
		case "any", "all":
			family = iface.NeighborFamilyAny
		default:
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("unknown family %q; valid: ipv4, ipv6, any", args[0]),
			}, nil
		}
	default:
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("too many arguments; %s", usage),
		}, nil
	}

	neighbors, err := iface.ListNeighbors(family)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error via Response
	}

	// Single-key wrapper so `| table` renders a columnar view and
	// `| count` returns the entry count.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"neighbors": neighbors,
		},
	}, nil
}
