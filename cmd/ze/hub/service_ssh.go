// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
// Related: session_factory.go -- builds the session model factory this file wires in
//
// The ssh side of the compile-out seam (see ssh_infra.go): the build + post-
// start wiring implementations that touch internal/component/ssh. Compiled only
// under //go:build ze_ssh; absent the tag this file is not built, the seam vars
// stay nil (register_ssh.go is also gated), and the ssh server is dropped.

//go:build ze_ssh

package hub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNoCommandProvided = errors.New("no command provided")
var errStatusErrorNoMessage = errors.New("command reported status=error")

// sshBuildImpl builds and starts the ssh server from generic infra inputs and
// returns it as the opaque sshServer handle (nil on config/start error).
func sshBuildImpl(in *sshBuildInputs) sshServer {
	log := in.Log
	params := in.Params

	cfg := zessh.Config{
		Listen:        in.Config.Listen,
		ListenAddrs:   in.Config.ListenAddrs,
		HostKeyPath:   in.Config.HostKeyPath,
		HostCertPath:  in.Config.HostCertPath,
		IdleTimeout:   in.Config.IdleTimeout,
		MaxSessions:   in.Config.MaxSessions,
		Users:         in.Users,
		UsersFunc:     in.UsersFunc,
		Authenticator: in.Authenticator,
		Authorizer:    in.Authorizer,
		AuditRecorder: in.Recorder,
	}
	cfg.ConfigDir = params.ConfigDir
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = coreenv.Get("ze.config.dir")
	}
	cfg.Storage = infra.ResolveSSHStorage(params.Store, params.ConfigDir)
	cfg.ConfigPath = params.ConfigPath

	srv, sshErr := zessh.NewServer(cfg)
	if sshErr != nil {
		log.Warn("SSH server config error", "error", sshErr)
		return nil
	}
	if startErr := srv.Start(context.Background(), nil, nil); startErr != nil {
		log.Warn("SSH server failed to start", "error", startErr)
		return nil
	}
	log.Info("SSH server listening", "address", srv.Address())
	srv.SetSessionModelFactory(buildSessionModelFactory(srv, params, in.Recorder, in.ReloadFn))
	if in.EphemeralFile != "" {
		if writeErr := os.WriteFile(in.EphemeralFile, []byte(srv.Address()), 0o600); writeErr != nil {
			log.Warn("failed to write ephemeral SSH address", "error", writeErr)
		}
	}
	return srv
}

// sshWireImpl wires command executors, monitor, plugin-protocol, and lifecycle
// callbacks onto a built ssh server. Called from the reactor post-start
// callback in infraSetup (after authorization/accounting are configured).
func sshWireImpl(handle sshServer, in *sshWireInputs) {
	sshSrv, ok := handle.(*zessh.Server)
	if !ok || sshSrv == nil {
		return
	}
	r := in.Reactor
	params := in.Params
	stopForRestart := in.StopForRestart
	d := r.Dispatcher()
	apiServer := params.APIServer()

	sshSrv.SetExecutorFactory(func(username, remoteAddr string, authorizer plugin.Authorizer) zessh.CommandExecutor {
		return func(input string) (*plugin.RenderedResponse, error) {
			ctx := &pluginserver.CommandContext{
				Server:     apiServer,
				Username:   username,
				RemoteAddr: remoteAddr,
				// An SSH session is an operator, authenticated as Username above.
				Authorizer: authorizer,
				Sender:     plugin.OperatorSender(),
			}
			resp, err := d.Dispatch(ctx, input)
			if err != nil {
				return &plugin.RenderedResponse{Response: resp}, err
			}
			if resp == nil {
				return &plugin.RenderedResponse{}, nil
			}
			if _, generated := plugin.RecordRows(resp); generated {
				// A row generator is walked once, and the SSH exec channel
				// walks it as it renders. Flattening it here would consume the
				// rows the answer is made of and leave the channel writing an
				// answer of none.
				return &plugin.RenderedResponse{Response: resp}, responseExecErr(resp, "")
			}
			formatted := params.FormatResponseData(resp.Data)
			return &plugin.RenderedResponse{Output: formatted, Response: resp}, responseExecErr(resp, formatted)
		}
	})
	sshSrv.SetStreamingExecutorFactory(func(username, remoteAddr string, authorizer plugin.Authorizer) zessh.StreamingExecutor {
		return func(ctx context.Context, w io.Writer, args []string) error {
			if len(args) == 0 {
				return errNoCommandProvided
			}
			input := args[0]
			cmdCtx := &pluginserver.CommandContext{
				Server:     apiServer,
				Username:   username,
				RemoteAddr: remoteAddr,
				// An SSH session is an operator, authenticated as Username above.
				Authorizer: authorizer,
				Sender:     plugin.OperatorSender(),
			}
			// Streaming commands are currently monitor-style read-only commands.
			// They still must pass through the same AAA authorizer/accountant as
			// normal SSH commands; future write-capable streaming commands need
			// explicit registry metadata instead of this read-only default.
			if !d.IsAuthorized(cmdCtx, input, true) {
				return pluginserver.ErrUnauthorized
			}
			handler, handlerArgs := pluginserver.GetStreamingHandlerForCommand(input)
			if handler == nil {
				return fmt.Errorf("unknown streaming command: %q", input)
			}
			defer d.BeginAccounting(cmdCtx, input)()
			return handler(ctx, apiServer, w, username, handlerArgs)
		}
	})
	sshSrv.SetMonitorFactory(func(ctx context.Context, args []string) (*contract.MonitorSession, error) {
		opts, err := pluginserver.ParseEventMonitorArgs(args)
		if err != nil {
			return nil, err
		}
		subs := pluginserver.BuildEventMonitorSubscriptions(opts)
		var tb textbuf.Buffer
		id := tb.Str("tui-monitor-").Int(time.Now().UnixNano()).String()
		client := pluginserver.NewMonitorClient(ctx, id, subs, 64)
		apiServer.Monitors().Add(client)
		cancel := func() {
			apiServer.Monitors().Remove(id)
		}
		return &contract.MonitorSession{
			EventChan:  client.EventChan,
			Cancel:     cancel,
			FormatFunc: pluginserver.MonitorEventFormatter(),
		}, nil
	})
	sshSrv.SetPluginProtocolFunc(func(ctx context.Context, reader io.ReadCloser, writer io.WriteCloser) error {
		return apiServer.HandleAdHocPluginSession(reader, writer)
	})
	// `shutdown` ends the daemon, so its peers are told why (Cease /
	// Administrative Shutdown). `restart` and `reboot` come back, so they take
	// stopForRestart, which writes the graceful-restart marker and stops in
	// silence: RFC 4724 Section 5 has a peer delete every route of a connection
	// that ends in a NOTIFICATION, with no condition on it. A bare TCP close is
	// the only end a peer can ever retain across, so it is the only one a
	// restart can use. See infra_setup.go, where the pair is built.
	sshSrv.SetShutdownFunc(func() { r.Stop() })
	sshSrv.SetRestartFunc(stopForRestart)
	sshSrv.SetRebootFunc(func() {
		rebootRequested.Store(true)
		stopForRestart()
	})
	rl := apiServer.Reactor()
	sshSrv.SetLoginWarnings(func() []contract.LoginWarning {
		bw := params.CollectLoginWarnings(rl)
		warnings := make([]contract.LoginWarning, len(bw))
		for i, w := range bw {
			warnings[i] = contract.LoginWarning{Message: w.Message, Command: w.Command}
		}
		return warnings
	})
}

// responseExecErr maps a dispatched Response's Status to the error
// CommandExecutor must return, so ssh.go's exec middleware maps the SSH
// session to the correct exit code. Many handlers deliberately return
// Status:StatusError with a nil Go error -- an operational failure encoded
// in the Response, not a Go error (e.g. as112's handleAS112Health returns
// {"healthy":false} with Status:StatusError, no Go error, when a health
// query legitimately fails). Without this mapping, d.Dispatch's nil error
// made the executor always return (formatted, nil), so a real `ze cli -c
// "..."` invocation over SSH always exited 0 regardless of Status -- silently
// breaking any script (a BGP healthcheck probe, in particular) that relies on
// the process exit code to detect an operational failure.
func responseExecErr(resp *plugin.Response, formatted string) error {
	if resp.Status != plugin.StatusError {
		return nil
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	if formatted != "" {
		return errors.New(formatted)
	}
	return errStatusErrorNoMessage
}

// sshBuildStandaloneImpl builds, wires (session model + dispatch executor), and
// starts the ssh server for the no-bgp{} startup path, returning a shutdown
// func (nil if ssh did not start). The caller (main.go) builds the AAA bundle
// always-on and passes the resolved authenticator.
func sshBuildStandaloneImpl(in *sshStandaloneInputs) func() {
	log := in.Log
	cfg := zessh.Config{
		Listen:        in.Config.Listen,
		ListenAddrs:   in.Config.ListenAddrs,
		HostKeyPath:   in.Config.HostKeyPath,
		HostCertPath:  in.Config.HostCertPath,
		IdleTimeout:   in.Config.IdleTimeout,
		MaxSessions:   in.Config.MaxSessions,
		Users:         in.Users,
		UsersFunc:     in.UsersFunc,
		Authenticator: in.Authenticator,
		Authorizer:    in.Authorizer,
		AuditRecorder: in.Recorder,
		ConfigDir:     in.ConfigDir,
		Storage:       in.Storage,
		ConfigPath:    in.ConfigPath,
	}

	srv, sshErr := zessh.NewServer(cfg)
	if sshErr != nil {
		log.Warn("SSH server config error", "error", sshErr)
		return nil
	}

	// Wire session model factory so interactive SSH sessions work, and the
	// dispatch executor for non-interactive exec commands.
	srv.SetSessionModelFactory(buildSessionModelFactory(srv, infra.HookParams{
		ConfigPath: in.ConfigPath,
		Store:      in.Storage,
	}, in.Recorder, in.ReloadFn))
	dispatch := in.Dispatch
	srv.SetExecutorFactory(func(username, remoteAddr string, authorizer plugin.Authorizer) zessh.CommandExecutor {
		return func(input string) (*plugin.RenderedResponse, error) {
			// Answer rather than JSON: the exec channel renders a row
			// generator itself, and Records is walked once.
			return dispatch.Answer(context.Background(), plugin.CallerIdentity{
				Username: username, RemoteAddr: remoteAddr, Authorizer: authorizer,
			}, input)
		}
	})

	if startErr := srv.Start(context.Background(), nil, nil); startErr != nil {
		log.Warn("SSH server failed to start", "error", startErr)
		return nil
	}
	log.Info("SSH server listening", "address", srv.Address())
	if in.EphemeralFile != "" {
		if writeErr := os.WriteFile(in.EphemeralFile, []byte(srv.Address()), 0o600); writeErr != nil {
			log.Warn("failed to write ephemeral SSH address", "error", writeErr)
		}
	}
	return func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		_ = srv.Stop(shutdownCtx)
	}
}
