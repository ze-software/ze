// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client show command
// Related: show_interface.go -- registers this handler with the `show interface` family
// Related: interface_rate.go -- the same forwarding shape for the rate tracker
//
// The PPPoE clients live in the iface plugin's process, so this handler
// forwards to the plugin's own command (iface.PPPoEClientCommand) rather
// than reading a copy the daemon never started.

package cmd

import (
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// forwardPPPoEClients reaches the iface plugin's PPPoEClientCommand through
// the dispatcher. It is a variable only so a unit test can stand in for the
// plugin process; production never reassigns it.
var forwardPPPoEClients = func(ctx *pluginserver.CommandContext) (*plugin.Response, error) {
	if ctx == nil {
		return nil, errNoDispatcher
	}
	d := ctx.Dispatcher()
	if d == nil {
		return nil, errNoDispatcher
	}
	return d.ForwardToPlugin(ctx, iface.PPPoEClientCommand, nil, ctx.PeerSelector())
}

// handleShowInterfacePPPoEClient serves `show interface pppoe-client`: one
// object per configured PPPoE client, sorted by name.
func handleShowInterfacePPPoEClient(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	statuses, err := pppoeClientsOf(ctx)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[iface.PPPoEClientStatus](statuses)}, nil
}

// pppoeClientsOf asks the iface plugin for its clients' status and decodes
// the document it answers. A refusal carries the text the operator reads.
func pppoeClientsOf(ctx *pluginserver.CommandContext) ([]iface.PPPoEClientStatus, error) {
	resp, err := forwardPPPoEClients(ctx)
	if errors.Is(err, pluginserver.ErrUnknownCommand) {
		return nil, errors.New("PPPoE client status unavailable: the interface plugin, which runs the PPPoE clients, is not running. " +
			"Ze starts it only when the configuration holds an interface section. " +
			"To start it, add the section and commit: set interface backend netlink")
	}
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("PPPoE client status unavailable: ").Err(err).String())
	}
	if resp.Status == plugin.StatusError {
		return nil, errors.New(resp.Error)
	}
	raw, ok := resp.Data.(plugin.RawJSON)
	if !ok {
		return nil, errors.New("PPPoE client status unavailable: the interface plugin answered no document")
	}
	var statuses []iface.PPPoEClientStatus
	if err := json.Unmarshal([]byte(raw), &statuses); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("PPPoE client status unavailable: the interface plugin's answer does not decode: ").Err(err).String())
	}
	return statuses, nil
}
