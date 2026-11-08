// Design: docs/guide/pppoe.md -- `show interface delegated-prefix`
// Related: show_interface.go -- registers this handler with the `show interface` family
// Related: show_pppoe_client.go -- forwardToIfacePlugin and ifacePluginList, shared with the PPPoE clients
//
// The delegation worker lives in the iface plugin's process, so this handler
// forwards to the plugin's own command (iface.DelegatedPrefixCommand) rather
// than reading a copy the daemon never started.

package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// forwardDelegatedPrefixes reaches the iface plugin's DelegatedPrefixCommand
// through the dispatcher. It is a variable only so a unit test can stand in
// for the plugin process; production never reassigns it.
var forwardDelegatedPrefixes = forwardToIfacePlugin(iface.DelegatedPrefixCommand)

// handleShowInterfaceDelegatedPrefix serves `show interface delegated-prefix`:
// one object per delegated-prefix entry of every enabled unit, sorted by
// interface, unit and source.
func handleShowInterfaceDelegatedPrefix(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	rows, err := ifacePluginList[iface.DelegatedUnitStatus](ctx, forwardDelegatedPrefixes,
		"delegated prefix status", "which numbers units from delegated prefixes")
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[iface.DelegatedUnitStatus](rows)}, nil
}
