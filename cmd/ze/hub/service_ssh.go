// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
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

	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	coreenv "codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

var errNoCommandProvided = errors.New("no command provided")

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
		Authenticator: in.Authenticator,
		AuditRecorder: in.Recorder,
	}
	cfg.ConfigDir = params.ConfigDir
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = coreenv.Get("ze.config.dir")
	}
	cfg.Storage = bgpconfig.ResolveSSHStorage(params.Store, params.ConfigDir)
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
	writeGRMarker := in.WriteGRMarker
	d := r.Dispatcher()
	apiServer := params.APIServer()

	sshSrv.SetExecutorFactory(func(username, remoteAddr string) zessh.CommandExecutor {
		return func(input string) (string, error) {
			ctx := &pluginserver.CommandContext{
				Server:     apiServer,
				Username:   username,
				RemoteAddr: remoteAddr,
			}
			resp, err := d.Dispatch(ctx, input)
			if err != nil {
				return "", err
			}
			if resp == nil {
				return "", nil
			}
			return params.FormatResponseData(resp.Data), nil
		}
	})
	sshSrv.SetStreamingExecutorFactory(func(username, remoteAddr string) zessh.StreamingExecutor {
		return func(ctx context.Context, w io.Writer, args []string) error {
			if len(args) == 0 {
				return errNoCommandProvided
			}
			input := args[0]
			cmdCtx := &pluginserver.CommandContext{
				Server:     apiServer,
				Username:   username,
				RemoteAddr: remoteAddr,
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
	sshSrv.SetShutdownFunc(func() { r.Stop() })
	sshSrv.SetRestartFunc(func() {
		writeGRMarker()
		r.Stop()
	})
	sshSrv.SetRebootFunc(func() {
		writeGRMarker()
		rebootRequested.Store(true)
		r.Stop()
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
		Authenticator: in.Authenticator,
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
	srv.SetSessionModelFactory(buildSessionModelFactory(srv, bgpconfig.InfraHookParams{
		ConfigPath: in.ConfigPath,
		Store:      in.Storage,
	}, in.Recorder, in.ReloadFn))
	dispatch := in.Dispatch
	srv.SetExecutorFactory(func(username, remoteAddr string) zessh.CommandExecutor {
		return func(input string) (string, error) {
			return dispatch(input, username, remoteAddr)
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
