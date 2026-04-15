// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
// Related: main.go -- hub entry point calls SetInfraHook before engine start

package hub

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/grmarker"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	coreenv "codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// buildAAABundle composes the AAA bundle through the pluggable backend
// registry. The hub does not import any backend package by name: every
// backend self-registers via init() in its own package (authz, tacacs,
// future RADIUS/LDAP) and Build assembles the live Authenticator chain,
// Authorizer, and Accountant.
//
// A nil store yields a nil LocalAuthorizer so the local backend does not
// contribute a permissive "allow all" authorizer when the caller explicitly
// has no RBAC configured.
func buildAAABundle(tree *zeconfig.Tree, users []aaa.UserCredential, store *authz.Store, logger *slog.Logger) (*aaa.Bundle, error) {
	var localAuthorizer aaa.Authorizer
	if store != nil {
		localAuthorizer = authz.StoreAuthorizer{Store: store}
	}
	params := aaa.BuildParams{
		Ctx:             context.Background(),
		ConfigTree:      tree,
		Logger:          logger,
		LocalUsers:      users,
		LocalAuthorizer: localAuthorizer,
	}
	return aaa.Default.Build(params)
}

// setupInfraHook creates and registers the infrastructure setup hook.
func setupInfraHook() {
	bgpconfig.SetInfraHook(infraSetup)
}

// infraSetup creates SSH server, wires authorization, command executors,
// monitor factory, and login warnings on the reactor's post-start callback.
func infraSetup(params bgpconfig.InfraHookParams) {
	log := slogutil.Logger("hub.infra")
	r := params.Reactor

	// Convert plain SSH config to ssh.Config and create server.
	var sshSrv *zessh.Server
	sshCfg := params.SSHConfig
	hasSSHConfig := sshCfg.HasConfig

	// Ephemeral mode: config edit starts daemon with ze.ssh.ephemeral.
	ephemeralFile := coreenv.Get("ze.ssh.ephemeral")
	if !hasSSHConfig && ephemeralFile != "" {
		sshCfg = bgpconfig.SSHExtractedConfig{
			Listen:    "127.0.0.1:0",
			HasConfig: true,
		}
		hasSSHConfig = true
	}

	// Users from zefs + config. Loaded regardless of SSH so the local AAA
	// backend sees them even on API-only or MCP-only deployments where
	// authorization and accounting must still apply.
	users := append([]aaa.UserCredential{}, sshCfg.Users...)
	if zefsUsers, err := loadZefsUsers(); err == nil {
		users = append(zefsUsers, users...)
	}

	// Build the AAA bundle unconditionally. TACACS+ accounting fires on
	// every dispatched command (SSH, MCP, API), so the bundle must exist
	// even when SSH is disabled. On config reload the previous bundle is
	// closed so backend workers (TACACS+ accounting) drain.
	bundle, buildErr := buildAAABundle(params.ConfigTree, users, params.AuthzStore, log)
	if buildErr != nil {
		log.Warn("AAA backend build failed", "error", buildErr)
		bundle = nil
	}
	swapAAABundle(bundle, log)

	if hasSSHConfig && bundle != nil {
		cfg := zessh.Config{
			Listen:        sshCfg.Listen,
			ListenAddrs:   sshCfg.ListenAddrs,
			HostKeyPath:   sshCfg.HostKeyPath,
			IdleTimeout:   sshCfg.IdleTimeout,
			MaxSessions:   sshCfg.MaxSessions,
			Users:         users,
			Authenticator: bundle.Authenticator,
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
		} else if startErr := srv.Start(context.Background(), nil, nil); startErr != nil {
			log.Warn("SSH server failed to start", "error", startErr)
		} else {
			log.Info("SSH server listening", "address", srv.Address())
			sshSrv = srv
			if ephemeralFile != "" {
				if writeErr := os.WriteFile(ephemeralFile, []byte(srv.Address()), 0o600); writeErr != nil {
					log.Warn("failed to write ephemeral SSH address", "error", writeErr)
				}
				sshSrv.SetSessionModelFactory(buildSessionModelFactory(sshSrv, params))
			}
		}
	}

	authzStore := params.AuthzStore
	needsPostStart := authzStore != nil || sshSrv != nil || bundle != nil
	if needsPostStart {
		r.SetPostStartFunc(func() {
			d := r.Dispatcher()
			if d == nil {
				return
			}

			writeGRMarker := func() {
				apiSrv := params.APIServer()
				if apiSrv == nil {
					return
				}
				allCaps := apiSrv.AllPluginCapabilities()
				maxRT := grmarker.MaxRestartTime(allCaps)
				if maxRT > 0 {
					expiresAt := time.Now().Add(time.Duration(maxRT) * time.Second)
					if writeErr := grmarker.Write(params.Store, expiresAt); writeErr != nil {
						log.Error("failed to write GR marker", "error", writeErr)
					} else {
						log.Info("GR marker written", "expires", expiresAt)
					}
				}
			}

			if apiSrv := params.APIServer(); apiSrv != nil {
				apiSrv.SetRebootFunc(func() {
					writeGRMarker()
					rebootRequested.Store(true)
					r.Stop()
				})
			}

			if bundle != nil && bundle.Authorizer != nil {
				d.SetAuthorizer(bundle.Authorizer)
				log.Info("authorization configured", "source", "aaa bundle")
			} else if authzStore != nil {
				d.SetAuthorizer(authz.StoreAuthorizer{Store: authzStore})
				log.Info("authorization profiles loaded")
			}

			if bundle != nil && bundle.Accountant != nil {
				d.SetAccountingHook(bundle.Accountant)
				log.Info("AAA accounting enabled")
			}

			if sshSrv != nil {
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
				sshSrv.SetStreamingExecutorFactory(func(username string) zessh.StreamingExecutor {
					return func(ctx context.Context, w io.Writer, args []string) error {
						if len(args) == 0 {
							return fmt.Errorf("no command provided")
						}
						input := args[0]
						handler, handlerArgs := pluginserver.GetStreamingHandlerForCommand(input)
						if handler == nil {
							return fmt.Errorf("unknown streaming command: %q", input)
						}
						return handler(ctx, apiServer, w, username, handlerArgs)
					}
				})
				sshSrv.SetMonitorFactory(func(ctx context.Context, args []string) (*contract.MonitorSession, error) {
					opts, err := pluginserver.ParseEventMonitorArgs(args)
					if err != nil {
						return nil, err
					}
					subs := pluginserver.BuildEventMonitorSubscriptions(opts)
					id := fmt.Sprintf("tui-monitor-%d", time.Now().UnixNano())
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
				log.Info("SSH command executor wired")
			}
		})
	}
}
