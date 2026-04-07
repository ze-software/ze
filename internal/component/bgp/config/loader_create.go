// Design: docs/architecture/config/syntax.md — reactor creation from config tree
// Overview: loader.go — config loading pipeline

package bgpconfig

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	coreenv "codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/family"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/grmarker"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// ze.bgp.tcp.port is a runtime-only env var for the test infrastructure.
// It creates a global listener so the ze-test peer can connect to ze.
// This is NOT the ExaBGP "bgp > listen" config leaf (removed from YANG).
const envKeyTCPPort = "ze.bgp.tcp.port"

var _ = coreenv.MustRegister(coreenv.EnvEntry{
	Key:         envKeyTCPPort,
	Type:        "int",
	Default:     "",
	Description: "BGP listen port (test infrastructure)",
	Private:     true,
})

// CreateReactorFromTree creates a Reactor directly from a parsed config tree.
func CreateReactorFromTree(tree *config.Tree, configDir, configPath string, plugins []reactor.PluginConfig, store storage.Storage) (*reactor.Reactor, error) {
	// Prune inactive containers and list entries before reading any config values.
	// PeersFromConfigTree also prunes (idempotent), but we need to prune early
	// so that ExtractEnvironment and BGP field extraction see the pruned tree.
	pruneSchema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema for inactive pruning: %w", err)
	}
	config.PruneInactive(tree, pruneSchema)

	// Load environment with config block values (if any)
	envValues := config.ExtractEnvironment(tree)
	env, err := config.LoadEnvironmentWithConfig(envValues)
	if err != nil {
		return nil, fmt.Errorf("load environment: %w", err)
	}

	// Extract global BGP settings directly from tree
	var routerID uint32
	var localAS uint32

	if bgpContainer := tree.GetContainer("bgp"); bgpContainer != nil {
		if v, ok := bgpContainer.Get("router-id"); ok {
			if ip, parseErr := netip.ParseAddr(v); parseErr == nil {
				routerID = ipToUint32(ip)
			}
		}
		if localContainer := bgpContainer.GetContainer("local"); localContainer != nil {
			if v, ok := localContainer.Get("as"); ok {
				if n, parseErr := strconv.ParseUint(v, 10, 32); parseErr == nil {
					localAS = uint32(n)
				}
			}
		}
	}

	// Build peers from tree (resolves templates, extracts routes).
	// Incomplete peers (missing required fields) are skipped so the daemon
	// can start for config editing with partial configs. Hard validation
	// errors (unknown family, invalid address) still fail.
	peers, err := PeersFromConfigTree(tree)
	if err != nil {
		if errors.Is(err, reactor.ErrIncompleteConfig) {
			slogutil.Logger("bgp.reactor").Warn("skipping peer with incomplete config", "error", err)
			peers = nil // continue with no peers
		} else {
			return nil, fmt.Errorf("build peers: %w", err)
		}
	}

	// Validate plugin references
	if err := ValidatePluginReferences(tree, plugins); err != nil {
		return nil, fmt.Errorf("validate plugin references: %w", err)
	}

	// Validate listener port conflicts across all services.
	listeners := config.CollectListeners(tree)
	if err := config.ValidateListenerConflicts(listeners); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	// Derive ConfiguredFamilies from peer capabilities.
	// Multiprotocol capabilities declare which families each peer supports.
	var configuredFamilies []string
	familySeen := make(map[string]bool)
	for _, ps := range peers {
		for _, cap := range ps.Capabilities {
			if mp, ok := cap.(*capability.Multiprotocol); ok {
				fam := family.Family{AFI: mp.AFI, SAFI: mp.SAFI}
				fs := fam.String()
				if !familySeen[fs] {
					familySeen[fs] = true
					configuredFamilies = append(configuredFamilies, fs)
				}
			}
		}
	}

	// Derive ConfiguredCustomEvents from peer process receive bindings.
	// Custom event types (e.g., "update-rpki") trigger auto-loading of producing plugins.
	var configuredCustomEvents []string
	customEventSeen := make(map[string]bool)
	for _, ps := range peers {
		for _, pb := range ps.ProcessBindings {
			for et := range pb.ReceiveCustom {
				if !customEventSeen[et] {
					customEventSeen[et] = true
					configuredCustomEvents = append(configuredCustomEvents, et)
				}
			}
		}
	}

	// Derive ConfiguredCustomSendTypes from peer process send bindings.
	// Custom send types (e.g., "enhanced-refresh") trigger auto-loading of enabling plugins.
	var configuredCustomSendTypes []string
	customSendSeen := make(map[string]bool)
	for _, ps := range peers {
		for _, pb := range ps.ProcessBindings {
			for st := range pb.SendCustom {
				if !customSendSeen[st] {
					customSendSeen[st] = true
					configuredCustomSendTypes = append(configuredCustomSendTypes, st)
				}
			}
		}
	}

	// Extract hub config for TLS plugin transport.
	hubConfig, hubErr := config.ExtractHubConfig(tree)
	if hubErr != nil {
		return nil, fmt.Errorf("hub config: %w", hubErr)
	}
	// Convert to pointer: nil when not configured (no servers).
	var hubPtr *plugin.HubConfig
	if len(hubConfig.Servers) > 0 {
		hubPtr = &hubConfig
	}

	// Build reactor config
	reactorCfg := &reactor.Config{
		// No global ListenAddr -- Ze derives listeners from per-peer connection > local.
		RouterID:                  routerID,
		LocalAS:                   localAS,
		ConfigDir:                 configDir,
		ConfigTree:                tree.ToMap(),
		MaxSessions:               env.TCP.Attempts, // tcp.attempts: exit after N sessions (0=unlimited)
		ConfiguredFamilies:        configuredFamilies,
		ConfiguredCustomEvents:    configuredCustomEvents,
		ConfiguredCustomSendTypes: configuredCustomSendTypes,
		ConfiguredPaths:           config.CollectContainerPaths(tree),
		Plugins:                   plugins,
		Hub:                       hubPtr,
		RecentUpdateMax:           env.Reactor.CacheMax,
	}

	r := reactor.New(reactorCfg)

	// Start pprof HTTP server from config environment block.
	// CLI --pprof flag takes precedence (started earlier in main.go).
	if env.Debug.Pprof != "" {
		startPprofServer(env.Debug.Pprof)
	}

	// Start Prometheus metrics HTTP server from telemetry config block.
	// Creates a shared registry that the reactor (and future components) register metrics into.
	if addr, port, path, ok := metrics.ExtractTelemetryConfig(tree.ToMap()); ok {
		reg := metrics.NewPrometheusRegistry()
		var srv metrics.Server
		if err := srv.Start(reg, addr, port, path); err != nil {
			configLogger().Warn("metrics server failed to start", "error", err)
		} else {
			configLogger().Info("prometheus metrics enabled",
				"address", addr, "port", port, "path", path)
			r.SetMetricsRegistry(reg)
			registry.SetMetricsRegistry(reg)
		}
	}

	// Validate authorization config (AC-8: reject undefined profile references).
	if err := ValidateAuthzConfig(tree); err != nil {
		return nil, fmt.Errorf("authorization config: %w", err)
	}

	// Extract authz profiles from config (independent of SSH).
	authzStore := extractAuthzConfig(tree)

	// Start SSH server from system config block.
	// SSH binds the port immediately, but the command executor is wired later
	// via SetExecutorFactory after the reactor's API server starts (post-start hook).
	var sshSrv *zessh.Server
	sshCfg, hasSSHConfig := extractSSHConfig(tree)
	// Ephemeral mode: config edit starts the daemon with ze.ssh.ephemeral set.
	// Start SSH on port 0 (OS-assigned) so the editor can connect even when
	// the config has no SSH settings.
	ephemeralFile := coreenv.Get("ze.ssh.ephemeral")
	if !hasSSHConfig && ephemeralFile != "" {
		sshCfg = zessh.Config{Listen: "127.0.0.1:0"}
		hasSSHConfig = true
	}
	if hasSSHConfig {
		// Merge users from zefs database (ze init) with config-based users.
		// Zefs users prepended so hash-as-token auth finds them first when
		// the same username appears in both config and zefs.
		if zefsUsers, err := loadZefsUsers(); err == nil {
			sshCfg.Users = append(zefsUsers, sshCfg.Users...)
		}
		sshCfg.Storage = resolveSSHStorage(store, configDir)
		sshCfg.ConfigPath = configPath
		srv, sshErr := zessh.NewServer(sshCfg)
		if sshErr != nil {
			configLogger().Warn("SSH server config error", "error", sshErr)
		} else if startErr := srv.Start(context.Background(), nil, nil); startErr != nil {
			configLogger().Warn("SSH server failed to start", "error", startErr)
		} else {
			configLogger().Info("SSH server listening", "address", srv.Address())
			sshSrv = srv
			// Write ephemeral SSH address so the parent (config edit) can connect.
			if ephemeralFile != "" {
				if writeErr := os.WriteFile(ephemeralFile, []byte(srv.Address()), 0o600); writeErr != nil {
					configLogger().Warn("failed to write ephemeral SSH address", "error", writeErr)
				}
			}
		}
	}

	// Web server startup moved to hub/main.go startWebServer() which wires
	// full route handling. Config detection via HasWebConfig().
	var webSrv *zeweb.WebServer

	// Deferred wiring: after reactor starts and Dispatcher is available,
	// connect authorization store, SSH executor, and web handlers (if configured).
	if authzStore != nil || sshSrv != nil || webSrv != nil {
		r.SetPostStartFunc(func() {
			d := r.Dispatcher()
			if d == nil {
				return
			}

			// Wire authorization (applies to API socket + SSH)
			if authzStore != nil {
				d.SetAuthorizer(authzStore)
				configLogger().Info("authorization profiles loaded")
			}

			// Wire SSH command executor with per-session username
			if sshSrv != nil {
				apiServer := r.APIServer()
				sshSrv.SetExecutorFactory(func(username string) zessh.CommandExecutor {
					return func(input string) (string, error) {
						ctx := &pluginserver.CommandContext{
							Server:   apiServer,
							Username: username,
						}
						resp, err := d.Dispatch(ctx, input)
						if err != nil {
							return "", err
						}
						if resp == nil {
							return "", nil
						}
						return formatResponseData(resp.Data), nil
					}
				})
				// Wire streaming executor for monitor commands via registry.
				// The SSH layer passes the full command input as args[0].
				// The executor looks up the handler from the streaming registry.
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
				// Wire TUI monitor factory for interactive SSH sessions.
				sshSrv.SetMonitorFactory(func(ctx context.Context, args []string) (*cli.MonitorSession, error) {
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
					return &cli.MonitorSession{
						EventChan:  client.EventChan,
						Cancel:     cancel,
						FormatFunc: pluginserver.MonitorEventFormatter(),
					}, nil
				})
				// Wire plugin protocol handler for debug shell sessions.
				// The SSH channel becomes bidirectional plugin transport.
				sshSrv.SetPluginProtocolFunc(func(ctx context.Context, reader io.ReadCloser, writer io.WriteCloser) error {
					return apiServer.HandleAdHocPluginSession(reader, writer)
				})
				sshSrv.SetShutdownFunc(func() { r.Stop() })
				sshSrv.SetRestartFunc(func() {
					// Compute max restart-time from all GR capabilities and write marker.
					apiServer := r.APIServer()
					if apiServer != nil {
						allCaps := apiServer.AllPluginCapabilities()
						maxRT := grmarker.MaxRestartTime(allCaps)
						if maxRT > 0 {
							expiresAt := time.Now().Add(time.Duration(maxRT) * time.Second)
							if writeErr := grmarker.Write(store, expiresAt); writeErr != nil {
								configLogger().Error("failed to write GR marker", "error", writeErr)
							} else {
								configLogger().Info("GR marker written", "expires", expiresAt)
							}
						}
					}
					r.Stop()
				})
				// Wire login warnings: collect prefix warnings on each SSH session.
				rl := apiServer.Reactor()
				sshSrv.SetLoginWarnings(func() []cli.LoginWarning {
					return collectPrefixWarnings(rl)
				})
				configLogger().Info("SSH command executor wired")
			}

			// Shut down web server when reactor stops.
			// Spawns a lifecycle goroutine that waits for reactor completion,
			// then gracefully shuts down the HTTP server.
			if webSrv != nil {
				go func() {
					_ = r.Wait(context.Background())
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer shutdownCancel()
					if err := webSrv.Shutdown(shutdownCtx); err != nil {
						configLogger().Warn("web server shutdown error", "error", err)
					} else {
						configLogger().Info("web server stopped")
					}
				}()
			}
		})
	}

	// Inject chaos wrappers from config environment block.
	// CLI flags (--chaos-seed) override this via SetClock/SetDialer/SetListenerFactory after load.
	if env.Chaos.Seed != 0 {
		resolved := chaos.ResolveSeed(env.Chaos.Seed)
		chaosLogger := slogutil.Logger("chaos")
		chaosCfg := chaos.ChaosConfig{Seed: resolved, Rate: env.Chaos.Rate, Logger: chaosLogger}
		clock, dialer, lf := chaos.NewChaosWrappers(clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, chaosCfg)
		r.SetClock(clock)
		r.SetDialer(dialer)
		r.SetListenerFactory(lf)
		chaosLogger.Info("chaos self-test mode enabled (config)", "seed", resolved, "rate", env.Chaos.Rate)
	}

	// Add peers
	for _, ps := range peers {
		if err := r.AddPeer(ps); err != nil {
			return nil, fmt.Errorf("add peer %s: %w", ps.Address, err)
		}
	}

	return r, nil
}

// createReloadFunc creates a ReloadFunc that parses config files.
// It returns full PeerSettings to ensure reloaded peers are identical to initial load.
// Uses PeersFromConfigTree which resolves templates and extracts routes directly.
func createReloadFunc(store storage.Storage) reactor.ReloadFunc {
	return func(configPath string) ([]*reactor.PeerSettings, error) {
		data, err := store.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", configPath, err)
		}

		// Parse the config using YANG-derived schema.
		schema, err := config.YANGSchema()
		if err != nil {
			return nil, fmt.Errorf("YANG schema: %w", err)
		}
		p := config.NewParser(schema)
		tree, err := p.Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}

		return PeersFromConfigTree(tree)
	}
}
