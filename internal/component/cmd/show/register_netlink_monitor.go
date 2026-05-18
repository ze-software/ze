// Design: plan/spec-diag-netlink-monitor.md -- streaming handler registration

package show

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const (
	netlinkGroupRoute   = "route"
	netlinkGroupLink    = "link"
	netlinkGroupAddress = "address"
	netlinkGroupAll     = "all"
)

func init() {
	pluginserver.RegisterStreamingHandler("monitor system netlink", streamNetlinkMonitor)
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-monitor:system-netlink",
			Handler:    handleMonitorSystemNetlink,
		},
	)
}

func handleMonitorSystemNetlink(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	group := netlinkGroupAll
	if len(args) > 0 {
		group = strings.ToLower(args[0])
	}
	switch group {
	case netlinkGroupRoute, netlinkGroupLink, netlinkGroupAddress, netlinkGroupAll:
	default:
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "unknown netlink group (valid: route, link, address, all)",
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"status": "monitor-configured",
			"group":  group,
		},
	}, nil
}
