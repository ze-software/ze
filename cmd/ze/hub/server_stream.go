// Design: docs/architecture/api/commands.md -- shared terminal stream dispatch.
// Related: main_servers.go -- single-response dispatcher and trusted identities.

package hub

import (
	"context"
	"io"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
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
		return serverStreamCommand(s, ctx, caller, input, func(streamCtx context.Context, accepted plugin.CallerIdentity, handler pluginserver.StreamingHandler, args command.ValidatedArgs) error {
			return handler(streamCtx, s, output, accepted.Username, args)
		})
	}
}

// serverStreamAdmission shares raw streaming's checks without starting its raw
// producer. The admitted continuation owns the same accounting interval.
func serverStreamAdmission(s *pluginserver.Server) plugin.CommandStreamAdmission {
	return func(ctx context.Context, caller plugin.CallerIdentity, input string, run func(context.Context, plugin.CallerIdentity) error) (bool, error) {
		return serverStreamCommand(s, ctx, caller, input, func(streamCtx context.Context, accepted plugin.CallerIdentity, _ pluginserver.StreamingHandler, _ command.ValidatedArgs) error {
			return run(streamCtx, accepted)
		})
	}
}

func serverStreamCommand(s *pluginserver.Server, ctx context.Context, caller plugin.CallerIdentity, input string, run func(context.Context, plugin.CallerIdentity, pluginserver.StreamingHandler, command.ValidatedArgs) error) (bool, error) {
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
	streamCommand, format, _, pipeErr := command.ProcessRemoteStreamPipes(input, "")
	if pipeErr != "" {
		return true, opererr.New("the live monitor pipe chain was refused", pipeErr,
			"remove the refused pipe and repeat the monitor command")
	}
	handler, args, err := pluginserver.GetStreamingHandlerForCommand(streamCommand)
	if err != nil {
		return true, err
	}
	if handler == nil {
		return true, opererr.New("the monitor command is no longer registered", "its owning plugin stopped before the stream opened", "run help monitor and select an available monitor")
	}
	if words.FirstPipe() >= 0 {
		producer := handler
		handler = func(ctx context.Context, server *pluginserver.Server, output io.Writer, username string, args command.ValidatedArgs) error {
			return command.RunStreamPipes(output, format, func(piped io.Writer) error {
				return producer(ctx, server, piped, username, args)
			})
		}
	}
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(daemonContext, cancel)
	defer stop()
	if err := streamCtx.Err(); err != nil {
		return true, err
	}
	runErr := run(streamCtx, caller, handler, args)
	if streamCtx.Err() != nil {
		if ctx.Err() == nil {
			return true, opererr.New("the daemon stopped this monitor", "live answers are no longer available from this daemon", "sign in again after the daemon starts and repeat the monitor command")
		}
	}
	return true, runErr
}
