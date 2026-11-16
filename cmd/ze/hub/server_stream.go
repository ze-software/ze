// Design: docs/architecture/api/commands.md -- shared terminal stream dispatch.
// Related: main_servers.go -- single-response dispatcher and trusted identities.

package hub

import (
	"context"
	"io"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/opererr"
)

// serverStreamDispatcher adapts the same registered streaming handlers SSH uses
// to an authenticated terminal transport. A stream MUST end with either the
// request or daemon; the caller MUST stop reading when it cancels its request.
func serverStreamDispatcher(s *pluginserver.Server) plugin.CommandStreamDispatcher {
	return func(ctx context.Context, caller plugin.CallerIdentity, input string, output io.Writer) (bool, error) {
		if !pluginserver.IsStreamingCommand(input) {
			return false, nil
		}
		daemonContext := s.Context()
		if daemonContext == nil {
			return true, errServerNotReady
		}
		d := s.Dispatcher()
		if d == nil {
			return true, errServerNotReady
		}
		if caller.Authorizer == nil {
			caller.Authorizer = plugin.CallerAuthorizer(ctx)
		}
		cmdCtx := &pluginserver.CommandContext{
			Server: s, Username: caller.Username, RemoteAddr: caller.RemoteAddr,
			Surface: audit.Web, Authorizer: caller.Authorizer, Sender: plugin.OperatorSender(),
			RequestContext: ctx,
		}
		if caller.Surface != "" {
			cmdCtx.Surface = caller.Surface
		}
		defer d.BeginAccounting(cmdCtx, input)()
		if !d.IsAuthorized(cmdCtx, input, true) {
			return true, plugin.ErrStreamUnauthorized
		}
		words, err := cli.ParseCommandWords(input)
		if err != nil {
			return true, err
		}
		if words.FirstPipe() >= 0 {
			return true, opererr.New("live monitor output does not support pipe filters", "the monitor stays open until it is stopped", "run the monitor command without a pipe")
		}
		handler, args, err := pluginserver.GetStreamingHandlerForCommand(input)
		if err != nil {
			return true, err
		}
		if handler == nil {
			return true, opererr.New("the monitor command is no longer registered", "its owning plugin stopped before the stream opened", "run help monitor and select an available monitor")
		}
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(daemonContext, cancel)
		defer stop()
		if err := handler(streamCtx, s, output, caller.Username, args); err != nil {
			return true, err
		}
		if streamCtx.Err() != nil {
			if ctx.Err() == nil {
				return true, opererr.New("the daemon stopped this monitor", "live answers are no longer available from this daemon", "sign in again after the daemon starts and repeat the monitor command")
			}
		}
		return true, nil
	}
}
