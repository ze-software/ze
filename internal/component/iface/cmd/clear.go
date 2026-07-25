// Design: docs/guide/command-reference.md -- clear verb for interface counters
// Related: cmd.go -- sibling iface RPC handlers (interface lifecycle)

package cmd

import (
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-clear:interface-counters",
			Handler:    handleClearInterfaceCounters,
		},
	)
}

// handleClearInterfaceCounters zeros RX/TX counters.
//
// Canonical grammar:
//
//	clear interface counters               -> all
//	clear interface name <name> counters   -> one
func handleClearInterfaceCounters(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ""
	if ctx != nil {
		name = ctx.Selector("name")
	}
	if len(args) != 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: clear interface counters or clear interface name <name> counters",
		}, nil
	}

	if err := iface.ResetCounters(name); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error via Response
	}

	scope := name
	if scope == "" {
		scope = "all"
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"cleared": scope,
		},
	}, nil
}
