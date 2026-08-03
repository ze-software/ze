// Design: plan/learned/664-diag-5-active-probes.md -- route lookup via netlink.
// Owned by the iface component: resolves the kernel FIB next-hop through the
// iface backend (iface.RouteLookup). See ai/rules/plugins.md.

package cmd

import (
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:route-lookup",
			Handler:    handleRouteLookup,
		},
	)
}

func handleRouteLookup(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: show route lookup <destination-ip>"}, nil
	}
	dest, err := netip.ParseAddr(args[0])
	if err != nil {
		msg := "invalid destination " + strconv.Quote(args[0]) + ": " + err.Error()
		return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil //nolint:nilerr // operational error in Response
	}

	route, lookupErr := iface.RouteLookup(dest)
	if lookupErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: lookupErr.Error()}, nil //nolint:nilerr // operational error in Response
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(route)}, nil
}
