// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client show command
// Related: show_interface.go -- registers this handler with the `show interface` family
// Related: interface_rate.go -- the same forwarding shape for the rate tracker
// Related: show_delegated_prefix.go -- the second caller of ifacePluginList
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
var forwardPPPoEClients = forwardToIfacePlugin(iface.PPPoEClientCommand)

// handleShowInterfacePPPoEClient serves `show interface pppoe-client`: one
// object per configured PPPoE client, sorted by name.
func handleShowInterfacePPPoEClient(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	statuses, err := ifacePluginList[iface.PPPoEClientStatus](ctx, forwardPPPoEClients,
		"PPPoE client status", "which runs the PPPoE clients")
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[iface.PPPoEClientStatus](statuses)}, nil
}

// forwardToIfacePlugin returns a forwarder that reaches the iface plugin's
// command name through the dispatcher, in whichever process the plugin runs.
func forwardToIfacePlugin(name string) func(*pluginserver.CommandContext) (*plugin.Response, error) {
	return func(ctx *pluginserver.CommandContext) (*plugin.Response, error) {
		if ctx == nil {
			return nil, errNoDispatcher
		}
		d := ctx.Dispatcher()
		if d == nil {
			return nil, errNoDispatcher
		}
		return d.ForwardToPlugin(ctx, name, nil, ctx.PeerSelector())
	}
}

// ifacePluginList asks the iface plugin through forward and decodes the list
// it answers. subject names the status in every refusal, and owner says why
// the plugin is the one that holds it. A refusal carries the text the
// operator reads.
func ifacePluginList[T any](ctx *pluginserver.CommandContext, forward func(*pluginserver.CommandContext) (*plugin.Response, error), subject, owner string) ([]T, error) {
	resp, err := forward(ctx)
	if errors.Is(err, pluginserver.ErrUnknownCommand) {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str(subject).Str(" unavailable: the interface plugin, ").Str(owner).Str(", is not running. ").
			Str("Ze starts it only when the configuration holds an interface section. ").
			Str("To start it, add the section and commit: set interface backend netlink").String())
	}
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str(subject).Str(" unavailable: ").Err(err).String())
	}
	if resp.Status == plugin.StatusError {
		return nil, errors.New(resp.Error)
	}
	raw, ok := resp.Data.(plugin.RawJSON)
	if !ok {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str(subject).Str(" unavailable: the interface plugin answered no document").String())
	}
	var list []T
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str(subject).Str(" unavailable: the interface plugin's answer does not decode: ").Err(err).String())
	}
	return list, nil
}
