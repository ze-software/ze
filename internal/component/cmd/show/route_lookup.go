// Design: plan/spec-diag-5-active-probes.md -- route lookup via netlink

package show

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func handleRouteLookup(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{Status: plugin.StatusError, Data: "usage: show ip route lookup <destination-ip>"}, nil
	}
	dest, err := netip.ParseAddr(args[0])
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid destination %q: %v", args[0], err)}, nil //nolint:nilerr // operational error in Response
	}

	route, lookupErr := iface.RouteLookup(dest)
	if lookupErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: lookupErr.Error()}, nil //nolint:nilerr // operational error in Response
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: route}, nil
}
