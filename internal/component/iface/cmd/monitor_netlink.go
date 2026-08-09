// Design: docs/architecture/iface/netlink-monitor.md -- netlink monitor handler registration
// Related: interface_rate.go -- existing streaming monitor handler in iface/cmd

package cmd

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	netlinkGroupRoute   = "route"
	netlinkGroupLink    = "link"
	netlinkGroupAddress = "address"
	netlinkGroupAll     = "all"
)

// netlinkGroups is the canonical list of accepted netlink monitor groups.
// Every "valid: ..." message derives from it (ai/rules/evidence.md).
var netlinkGroups = []string{netlinkGroupRoute, netlinkGroupLink, netlinkGroupAddress, netlinkGroupAll}

var errUnknownNetlinkGroup = newUnknownNetlinkGroupError()

func newUnknownNetlinkGroupError() error {
	var tb textbuf.Buffer
	tb.Str("unknown netlink group (valid: ").Join(netlinkGroups, ", ").Byte(')')
	return errors.New(tb.String())
}

// netlinkGroupFromArgs parses and validates the requested netlink group.
// No argument selects the "all" group.
func netlinkGroupFromArgs(args []string) (string, error) {
	group := netlinkGroupAll
	if len(args) > 0 {
		group = strings.ToLower(args[0])
	}
	if !slices.Contains(netlinkGroups, group) {
		return "", errUnknownNetlinkGroup
	}
	return group, nil
}

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
	group, err := netlinkGroupFromArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"status": "monitor-configured",
			"group":  group,
		},
	}, nil
}
