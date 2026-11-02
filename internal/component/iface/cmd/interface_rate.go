// Design: docs/features/interfaces.md -- interface rate show + monitor handlers
// Related: show_interface.go -- the `show interface` family; this adds the rate branch
// Related: register_rate.go -- registers the monitor handlers below
//
// Owned by the iface component: the rate handlers forward to the iface
// plugin's own rate command (iface.RateCommand), because the rate tracker
// lives in the plugin's process, which is not the daemon's when the plugin
// runs external. Relocated from the central cmd/show package together with
// the rest of the interface surface. See ai/rules/plugins.md.

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// errNoDispatcher refuses a rate call made with no command dispatcher, the
// only route to the process that owns the rate tracker.
var errNoDispatcher = errors.New("no command dispatcher reaches the interface plugin")

// forwardRates reaches the iface plugin's RateCommand through the dispatcher.
// It is a variable only so a unit test can stand in for the plugin process,
// which a test of this package cannot start; production never reassigns it.
var forwardRates = func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if ctx == nil {
		return nil, errNoDispatcher
	}
	d := ctx.Dispatcher()
	if d == nil {
		return nil, errNoDispatcher
	}
	return d.ForwardToPlugin(ctx, iface.RateCommand, args, ctx.PeerSelector())
}

// ratesOf asks the iface plugin for its rates through the dispatcher, which
// reaches the plugin whether it runs internal or external. The tracker lives
// in the plugin's process, never in the daemon's (iface/rate_command.go), so
// reading it here would answer the daemon's empty copy.
//
// The answer is the plugin's document: with a name in args, that interface's
// rate (read by oneRate); otherwise every interface's, sorted by name (read
// by everyRate). A refusal carries the text the operator reads.
func ratesOf(ctx *pluginserver.CommandContext, args []string) (plugin.RawJSON, error) {
	resp, err := forwardRates(ctx, args)
	// No process serves the command: the iface plugin is not running. The
	// dispatcher's own text names only the command it could not find, so the
	// refusal says instead which plugin is missing, why, and how to start it,
	// and ends on the command that fixes it.
	if errors.Is(err, pluginserver.ErrUnknownCommand) {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("interface rates unavailable: ").Err(iface.ErrRatePluginAbsent).String())
	}
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("interface rates unavailable: ").Err(err).String())
	}
	if resp.Status == plugin.StatusError {
		return "", errors.New(resp.Error)
	}
	raw, ok := resp.Data.(plugin.RawJSON)
	if !ok {
		return "", errors.New("interface rates unavailable: the interface plugin answered no document")
	}
	return raw, nil
}

// oneRate reads the plugin's answer to a call that named an interface.
func oneRate(raw plugin.RawJSON) (iface.InterfaceRate, error) {
	var rate iface.InterfaceRate
	if err := json.Unmarshal([]byte(raw), &rate); err != nil {
		return iface.InterfaceRate{}, fmt.Errorf("interface rates unavailable: the interface plugin's answer does not decode: %w", err)
	}
	return rate, nil
}

// everyRate reads the plugin's answer to a call that named no interface.
func everyRate(raw plugin.RawJSON) ([]iface.InterfaceRate, error) {
	var rates []iface.InterfaceRate
	if err := json.Unmarshal([]byte(raw), &rates); err != nil {
		return nil, fmt.Errorf("interface rates unavailable: the interface plugin's answer does not decode: %w", err)
	}
	return rates, nil
}

// rateResponse is the one-shot answer of `show interface rate` and `monitor
// interface rate`: one interface's rate, or every interface's as a list.
func rateResponse(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	raw, err := ratesOf(ctx, args)
	if err != nil {
		return rateRefusal(err), nil
	}
	if len(args) > 0 {
		rate, err := oneRate(raw)
		if err != nil {
			return rateRefusal(err), nil
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: rate}, nil
	}
	rates, err := everyRate(raw)
	if err != nil {
		return rateRefusal(err), nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[iface.InterfaceRate](rates)}, nil
}

// rateRefusal carries a rate refusal in the Response: the dispatch itself
// succeeded, so the operator reads the refusal text, not a transport error.
func rateRefusal(err error) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}
}

// handleMonitorInterfaceRate is the one-shot answer of the streaming command
// (pluginserver.OneShotSample): one sample of the rates the stream prints each
// second, asked of the same plugin, so MCP, REST, gRPC and the web admin
// console get the rates rather than an echo of the request.
func handleMonitorInterfaceRate(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return rateResponse(ctx, validated.Tokens())
}

// streamInterfaceRate prints one sample a second, each asked of the iface
// plugin like the one-shot answer. A second the plugin cannot answer (not
// running yet, the filtered interface absent) prints nothing, as before.
//
// The loop has no count: it ends only when ctx is cancelled, which the
// streaming caller does when the operator's session ends.
func streamInterfaceRate(ctx context.Context, server *pluginserver.Server, w io.Writer, _ string, args command.ValidatedArgs) error {
	tokens := args.Tokens()
	named := len(tokens) > 0
	cmdCtx := &pluginserver.CommandContext{Server: server, RequestContext: ctx, Sender: plugin.OperatorSender()}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// A second the plugin cannot answer, or answers with a document
			// that does not decode, prints nothing; only a failed write ends
			// the stream.
			raw, err := ratesOf(cmdCtx, tokens)
			if err != nil {
				continue
			}
			if named {
				rate, err := oneRate(raw)
				if err != nil {
					continue
				}
				if err := enc.Encode(rate); err != nil {
					return err
				}
				continue
			}
			rates, err := everyRate(raw)
			if err != nil {
				continue
			}
			if err := enc.Encode(rates); err != nil {
				return err
			}
		}
	}
}
