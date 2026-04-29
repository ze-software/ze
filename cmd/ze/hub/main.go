// Design: docs/architecture/hub-architecture.md -- hub CLI entry point
// Detail: mcp.go -- MCP server startup
// Detail: api.go -- REST/gRPC API server startup
// Detail: infra_setup.go -- infrastructure server setup hook
//
// Package hub provides the ze hub subcommand.
package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	yangloader "codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	zegokrazy "codeberg.org/thomas-mangin/ze/internal/component/gokrazy"
	"codeberg.org/thomas-mangin/ze/internal/component/hub"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/lg"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
	zePlugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginmgr "codeberg.org/thomas-mangin/ze/internal/component/plugin/manager"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve"
	resolvecmd "codeberg.org/thomas-mangin/ze/internal/component/resolve/cmd"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/cymru"
	resolveDNS "codeberg.org/thomas-mangin/ze/internal/component/resolve/dns"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	"codeberg.org/thomas-mangin/ze/internal/component/telemetry/collector"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/health"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/privilege"
	"codeberg.org/thomas-mangin/ze/internal/core/reboot"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Env var registrations are centralized in internal/component/config/environment.go.
// No duplicate registrations here -- import that package to trigger init.

// rebootRequested is set by the SSH/RPC reboot handler before triggering
// reactor.Stop(). After the graceful shutdown sequence completes, the main
// loop checks this flag and attempts an OS-level reboot if set.
var rebootRequested atomic.Bool

// RunWebOnly starts only the web server (no BGP engine).
// Used when ze start --web is called without a config.
// listenAddr overrides the default "0.0.0.0:3443" when non-empty.
func RunWebOnly(store storage.Storage, listenAddr string, insecureWeb bool) int {
	resolvers := newResolvers(system.SystemConfig{DNSTimeout: 5, DNSCacheSize: 10000, DNSCacheTTL: 86400})
	defer resolvers.Close()

	var listenAddrs []string
	if listenAddr != "" {
		listenAddrs = []string{listenAddr}
	}
	ring := pluginserver.NewEventRing(128)
	ring.Append("web", "server.started")
	dispatch := webOnlyDispatcher(ring)
	webSrv, broker := startWebServer(store, listenAddrs, insecureWeb, dispatch, resolvers)
	if webSrv == nil {
		return 1
	}

	sigCh := make(chan os.Signal, 2) //nolint:mnd // buffer 2: graceful + force
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Ze web running. Press Ctrl+C to stop.")
	<-sigCh
	fmt.Println("\nShutting down (Ctrl+C again to force)...")

	// Second signal forces immediate exit (lifecycle goroutine, not hot path).
	go forceExitOnSignal(sigCh)

	broker.Close()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = webSrv.Shutdown(shutdownCtx)

	return 0
}

// forceExitOnSignal waits for a second signal and exits immediately.
// Started once during shutdown to handle impatient Ctrl+C.
func forceExitOnSignal(sigCh <-chan os.Signal) {
	<-sigCh
	fmt.Fprintf(os.Stderr, "forced exit\n")
	os.Exit(1)
}

// Run executes the hub with the given config file path and optional CLI plugins.
// store provides the I/O backend (filesystem or blob); used for config reads and reload.
// chaosSeed > 0 enables chaos self-test mode; chaosRate < 0 means "use default".
// Returns exit code.
func Run(store storage.Storage, configPath string, plugins []string, chaosSeed int64, chaosRate float64, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr, mcpToken string) int {
	// Read config content first (to probe type without parsing).
	// When reading from stdin, we look for a NUL sentinel that signals
	// "config complete but pipe stays open for liveness monitoring."
	var data []byte
	var stdinOpen bool
	var err error
	if configPath == "-" {
		data, stdinOpen, err = readStdinConfig()
	} else {
		data, err = store.ReadFile(configPath)
		if err != nil && storage.IsBlobStorage(store) {
			// Config may live on the filesystem (e.g., gokrazy read-only root)
			// while blob handles TLS certs, SSH keys, and persistent state.
			data, err = os.ReadFile(configPath) //nolint:gosec // user-provided config path
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read config: %v\n", err)
		return 1
	}

	// Probe config type using shared function
	switch zeconfig.ProbeConfigType(string(data)) {
	case zeconfig.ConfigTypeBGP, zeconfig.ConfigTypeUnknown:
		// Non-BGP YANG config: auto-load plugins via ConfigRoots.
		return runYANGConfig(store, configPath, data, plugins, chaosSeed, chaosRate, stdinOpen, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken)
	case zeconfig.ConfigTypeHub:
		if len(plugins) > 0 {
			fmt.Fprintf(os.Stderr, "error: --plugin is not supported with hub/orchestrator configs; use plugin { external ... } in the config file\n")
			return 1
		}
		return runOrchestratorWithData(store, configPath, data)
	}

	return 1
}

// readStdinConfig reads config from stdin, stopping at a NUL byte sentinel
// or EOF. Returns the config data and whether stdin remains open (NUL found).
//
// When stdin remains open, the caller can monitor it for EOF to detect
// upstream process exit — e.g., in a pipeline like "ze-chaos | ze -",
// when the chaos tool exits, stdin closes, and Ze initiates clean shutdown.
//
// When no NUL is found (plain "cat config.conf | ze -"), reading stops at
// EOF with stdinOpen=false — the normal case.
func readStdinConfig() (data []byte, stdinOpen bool, err error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, readErr := os.Stdin.Read(tmp)
		if n > 0 {
			for i := range n {
				if tmp[i] == 0 {
					buf = append(buf, tmp[:i]...)
					return buf, true, nil
				}
			}
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			if readErr == io.EOF {
				return buf, false, nil
			}
			return nil, false, readErr
		}
	}
}

// runYANGConfig handles all YANG-based configs. Plugins are auto-loaded
// via ConfigRoots matching: bgp {} loads BGP, interface {} loads iface, etc.
// This is the unified startup path for all ze configs (except hub orchestrator mode).
func runYANGConfig(store storage.Storage, configPath string, data []byte, plugins []string, chaosSeed int64, chaosRate float64, stdinOpen, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr, mcpToken string) int { //nolint:cyclop // startup orchestration
	// Close the AAA bundle on every exit path so TACACS+ accounting and other
	// backend workers drain before the process terminates. swapAAABundle is
	// called by infraSetup on config load; closeAAABundle here matches it.
	defer closeAAABundle(slogutil.Logger("hub.aaa"))

	// Phase 1: Parse config and resolve plugins.
	loadResult, err := zeconfig.LoadConfig(string(data), configPath, plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	configPaths := zeconfig.CollectContainerPaths(loadResult.Tree)

	// Resolve web/LG/MCP listen addresses. Precedence per service:
	//   env var (compound ip:port[,ip:port]) > CLI flag > config file > off
	// Each service collects a []string of addresses; every binder is
	// multi-listener and binds the full slice.
	var (
		webAddrs []string
		lgAddrs  []string
		lgTLS    bool
		mcpAddrs []string
	)
	if webListenAddr != "" {
		webAddrs = []string{webListenAddr}
		webEnabled = true
	}

	if listen := env.Get("ze.looking-glass.listen"); listen != "" {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.looking-glass.listen: %v\n", parseErr)
			return 1
		}
		lgAddrs = make([]string, 0, len(endpoints))
		for _, ep := range endpoints {
			lgAddrs = append(lgAddrs, ep.String())
		}
	}
	if env.IsEnabled("ze.looking-glass.tls") {
		lgTLS = true
	}
	if env.IsEnabled("ze.looking-glass.enabled") && len(lgAddrs) == 0 {
		lgAddrs = []string{"0.0.0.0:8443"}
	}

	if listen := env.Get("ze.web.listen"); listen != "" && len(webAddrs) == 0 {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.web.listen: %v\n", parseErr)
			return 1
		}
		webAddrs = make([]string, 0, len(endpoints))
		for _, ep := range endpoints {
			webAddrs = append(webAddrs, ep.String())
		}
		webEnabled = true
	}
	if env.IsEnabled("ze.web.enabled") && !webEnabled {
		webEnabled = true
	}
	if env.IsEnabled("ze.web.insecure") && !insecureWeb {
		insecureWeb = true
	}
	if mcpAddr != "" {
		mcpAddrs = []string{mcpAddr}
	}
	if listen := env.Get("ze.mcp.listen"); listen != "" && len(mcpAddrs) == 0 {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.mcp.listen: %v\n", parseErr)
			return 1
		}
		mcpAddrs = make([]string, 0, len(endpoints))
		for _, ep := range endpoints {
			mcpAddrs = append(mcpAddrs, ep.String())
		}
	}
	if env.IsEnabled("ze.mcp.enabled") && len(mcpAddrs) == 0 {
		mcpAddrs = []string{"127.0.0.1:8080"}
	}
	if token := env.Get("ze.mcp.token"); token != "" && mcpToken == "" {
		mcpToken = token
	}

	// Config file fills in whatever the env vars and CLI flags left blank.
	// ExtractXxx returns cfg.Servers with at least one entry when the block
	// is enabled in YANG; every entry flows through to the binder below.
	if webCfg, ok := zeconfig.ExtractWebConfig(loadResult.Tree); ok {
		if len(webAddrs) == 0 {
			webAddrs = endpointsToAddrs(webCfg.Servers)
			insecureWeb = webCfg.Insecure
		}
		webEnabled = true
	}
	mcpCfg, mcpCfgOK := zeconfig.ExtractMCPConfig(loadResult.Tree)
	if mcpCfgOK {
		if len(mcpAddrs) == 0 {
			mcpAddrs = endpointsToAddrs(mcpCfg.Servers)
		}
		if mcpToken == "" && mcpCfg.Token != "" {
			mcpToken = mcpCfg.Token
		}
	}
	if lgCfg, ok := zeconfig.ExtractLGConfig(loadResult.Tree); ok {
		if len(lgAddrs) == 0 {
			lgAddrs = endpointsToAddrs(lgCfg.Servers)
		}
		if !lgTLS {
			lgTLS = lgCfg.TLS
		}
	}

	// Phase 2: Populate ConfigProvider.
	configProvider := zeconfig.NewProvider()
	for root, subtree := range loadResult.Tree.ToMap() {
		if sub, ok := subtree.(map[string]any); ok {
			configProvider.SetRoot(root, sub)
		}
	}

	// Phase 3: Create PluginCoordinator and plugin server.
	// The plugin server implements ze.EventBus via its Emit/Subscribe
	// methods, so there is no separate standalone bus any more; one
	// namespaced pub/sub backbone serves everyone.

	configTree := loadResult.Tree.ToMap()
	// Register infrastructure hook before engine starts.
	// The BGP plugin calls this when creating the reactor.
	setupInfraHook()
	coordinator := zePlugin.NewCoordinator(configTree)

	// Store config state for the BGP plugin's reactor factory.
	// The BGP plugin builds its own createReactor closure using these values.
	coordinator.SetExtra("bgp.configPath", configPath)
	coordinator.SetExtra("bgp.cliPlugins", plugins)
	coordinator.SetExtra("bgp.store", store)
	coordinator.SetExtra("bgp.configData", data)
	coordinator.SetExtra("bgp.chaosSeed", chaosSeed)
	coordinator.SetExtra("bgp.chaosRate", chaosRate)

	pm := pluginmgr.NewManager()

	// Wire hub config into process manager for external plugin startup.
	if hubCfg, hubErr := zeconfig.ExtractHubConfig(loadResult.Tree); hubErr == nil {
		if len(hubCfg.Servers) > 1 {
			slog.Warn("plugin hub: only first server listener is used, extra listeners ignored", "configured", len(hubCfg.Servers))
		}
		pm.SetHubConfig(&hubCfg)
	}

	// Convert explicit plugin configs from reactor format to plugin server format.
	var explicitPlugins []zePlugin.PluginConfig
	for _, pc := range loadResult.Plugins {
		explicitPlugins = append(explicitPlugins, zePlugin.PluginConfig{
			Name:          pc.Name,
			Run:           pc.Run,
			Encoder:       pc.Encoder,
			Respawn:       pc.Respawn,
			WorkDir:       loadResult.ConfigDir,
			ReceiveUpdate: pc.ReceiveUpdate,
			StageTimeout:  pc.StageTimeout,
			Internal:      pc.Internal,
		})
	}

	// Extract hub TLS config for external plugin connect-back.
	var hubConfig *zePlugin.HubConfig
	if hubCfg, hubErr := zeconfig.ExtractHubConfig(loadResult.Tree); hubErr == nil {
		hubConfig = &hubCfg
	}

	serverConfig := &pluginserver.ServerConfig{
		ConfigPath:      configPath,
		ConfiguredPaths: configPaths,
		Plugins:         explicitPlugins,
		Hub:             hubConfig,
	}
	apiServer, serverErr := pluginserver.NewServer(serverConfig, coordinator)
	if serverErr != nil {
		fmt.Fprintf(os.Stderr, "error: create plugin server: %v\n", serverErr)
		return 1
	}
	apiServer.SetProcessSpawner(pm)
	registry.SetPluginServer(apiServer)
	// The plugin server implements ze.EventBus via its Emit/Subscribe
	// methods, so internal plugins receive a single namespaced pub/sub
	// handle that is backed by the same fan-out path as plugin-process
	// stream events. This is the replacement for the standalone Bus.
	registry.SetEventBus(apiServer)

	// Set config loader for SIGHUP reload support.
	// Mirrors the initial-load fallback above: try the blob store first, and
	// if the store is blob-only (e.g., gokrazy read-only root, ze-test tmpfs)
	// fall back to a direct filesystem read. Without this fallback, SIGHUP
	// reload fails with "read file/active/...: file does not exist" on any
	// daemon started with a filesystem path that is not a blob key.
	// loadConfigFromDisk re-reads the config path and parses it. Used as
	// both the plugin server's ConfigLoader (SIGHUP diff + plugin reload)
	// and directly by doReload so subsystems see the freshly loaded tree
	// without depending on the plugin server's internal diff/short-circuit
	// behavior.
	var loadConfigFromDisk func() (map[string]any, error)
	if configPath != "" && configPath != "-" {
		loadConfigFromDisk = func() (map[string]any, error) {
			reloadData, readErr := store.ReadFile(configPath)
			if readErr != nil && storage.IsBlobStorage(store) {
				reloadData, readErr = os.ReadFile(configPath) //nolint:gosec // daemon operator supplied path
			}
			if readErr != nil {
				return nil, fmt.Errorf("read config: %w", readErr)
			}
			result, loadErr := zeconfig.LoadConfig(string(reloadData), configPath, plugins)
			if loadErr != nil {
				return nil, loadErr
			}
			return result.Tree.ToMap(), nil
		}
		apiServer.SetConfigLoader(loadConfigFromDisk)
	}

	// apiServer implements ze.EventBus via its Emit/Subscribe methods, so the
	// engine, plugins, and subsystems all share one namespaced pub/sub
	// backbone. The standalone bus in internal/component/bus/ is gone.
	eng := engine.NewEngine(apiServer, configProvider, pm)

	// L2TP subsystem (phase 3 scaffolding). ExtractParameters returns a
	// zero-value struct when the config tree has no `environment { l2tp {} }`
	// block; we only register with the engine when the operator actually
	// asked for L2TP (Enabled=true or at least one listener configured).
	// Full tunnel-reactor wiring lands in later phases.
	l2tpParams, l2tpErr := l2tp.ExtractParameters(loadResult.Tree)
	if l2tpErr != nil {
		fmt.Fprintf(os.Stderr, "error: parse l2tp config: %v\n", l2tpErr)
		return 1
	}
	if l2tpParams.Enabled || len(l2tpParams.ListenAddrs) > 0 {
		if regErr := eng.RegisterSubsystem(l2tp.NewSubsystem(l2tpParams)); regErr != nil {
			fmt.Fprintf(os.Stderr, "error: register l2tp subsystem: %v\n", regErr)
			return 1
		}
		zeweb.RegisterPortalService(zeweb.PortalService{Key: "l2tp", Title: "L2TP Sessions", Path: "/l2tp"})
	}

	startCtx := context.Background()
	if err := eng.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "error starting engine: %v\n", err)
		return 1
	}

	// Start plugin server (auto-loads BGP, iface, fib, etc. via ConfigRoots).
	if err := apiServer.StartWithContext(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "error starting plugin server: %v\n", err)
		_ = eng.Stop(startCtx)
		return 1
	}

	// Write PID file BEFORE dropping privileges so operator-supplied paths
	// in root-owned directories (e.g. /var/run/ze.pid) accept the create.
	// writePIDFile chowns to ze.user when set so removePIDFile succeeds at
	// shutdown (running post-drop).
	pidPath, pidErr := writePIDFile()
	if pidErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", pidErr)
		apiServer.Stop()
		_ = eng.Stop(startCtx)
		return 1
	}
	defer removePIDFile(pidPath)

	if err := dropPrivileges(); err != nil {
		fmt.Fprintf(os.Stderr, "error: drop privileges: %v\n", err)
		apiServer.Stop()
		_ = eng.Stop(startCtx)
		return 1
	}

	// Command dispatcher for web/LG/MCP (uses plugin server, not reactor directly).
	dispatch := serverDispatcher(apiServer)

	// Create shared resolvers for web UI, looking glass, and MCP.
	sc := system.ExtractSystemConfig(loadResult.Tree)
	iface.SetDHCPSystemConfig(sc.ResolvConfPath, len(sc.NameServers) > 0)
	resolvers := newResolvers(sc)
	defer resolvers.Close()
	resolvecmd.SetResolvers(resolvers)

	if len(sc.NameServers) > 0 {
		if err := system.WriteResolvConf(sc.ResolvConfPath, sc.NameServers); err != nil {
			slogutil.Logger("hub").Warn("resolv.conf write failed", "path", sc.ResolvConfPath, "err", err)
		}
	}

	if webEnabled {
		if len(webAddrs) == 0 {
			webAddrs = []string{"0.0.0.0:3443"}
		}
		if webSrv, broker := startWebServer(store, webAddrs, insecureWeb, dispatch, resolvers); webSrv != nil {
			if ring := apiServer.EventRing(); ring != nil {
				ring.Append("web", "server.started")
			}
			defer func() {
				broker.Close()
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer shutdownCancel()
				_ = webSrv.Shutdown(shutdownCtx)
			}()
		}
	}

	// Start SSH server directly when config has ssh {} block AND no bgp {} block.
	// When bgp {} is present, the BGP plugin's infra hook owns SSH startup so it
	// can wire the command executor factory in the reactor's post-start callback.
	// Starting SSH here in that case produces a second listener with no executor
	// factory -- clients connecting to it see "command executor not ready".
	// Without bgp {}, SSH must start here (e.g., gokrazy appliance with only
	// environment {}).
	_, hasBGPBlock := configTree["bgp"]

	if _, hasTelemetry := configTree["telemetry"]; hasTelemetry && !hasBGPBlock {
		if st := startStandaloneTelemetry(loadResult.Tree); st != nil {
			defer st.Close()
		}
	}

	sshCfg := bgpconfig.ExtractSSHConfig(loadResult.Tree)
	if sshCfg.HasConfig && !hasBGPBlock {
		cfg := zessh.Config{
			Listen:      sshCfg.Listen,
			ListenAddrs: sshCfg.ListenAddrs,
			HostKeyPath: sshCfg.HostKeyPath,
			IdleTimeout: sshCfg.IdleTimeout,
			MaxSessions: sshCfg.MaxSessions,
			Users:       sshCfg.Users,
		}
		if zefsUsers, err := loadZefsUsers(); err == nil {
			cfg.Users = append(zefsUsers, cfg.Users...)
		}

		// Build the AAA bundle via the registry (local + any enabled remote backends).
		// swapAAABundle installs it as the live bundle so closeAAABundle (deferred
		// at the top of runYANGConfig) drains backend workers on process exit.
		aaaLog := slogutil.Logger("hub.aaa")
		aaaBundle, aaaErr := buildAAABundle(loadResult.Tree, cfg.Users, nil, aaaLog)
		if aaaErr != nil {
			aaaLog.Warn("AAA backend build failed; SSH authenticator not set", "error", aaaErr)
		} else {
			cfg.Authenticator = aaaBundle.Authenticator
			swapAAABundle(aaaBundle, aaaLog)
		}

		cfg.ConfigDir = loadResult.ConfigDir
		if cfg.ConfigDir == "" {
			cfg.ConfigDir = env.Get("ze.config.dir")
		}
		cfg.Storage = bgpconfig.ResolveSSHStorage(store, cfg.ConfigDir)
		cfg.ConfigPath = configPath

		sshSrv, sshErr := zessh.NewServer(cfg)
		if sshErr != nil {
			slog.Warn("SSH server config error", "error", sshErr)
		} else {
			// Wire session model factory so interactive SSH sessions work.
			sshSrv.SetSessionModelFactory(buildSessionModelFactory(sshSrv, bgpconfig.InfraHookParams{
				ConfigPath: configPath,
				Store:      cfg.Storage,
			}))
			if startErr := sshSrv.Start(context.Background(), nil, nil); startErr != nil {
				slog.Warn("SSH server failed to start", "error", startErr)
			} else {
				slog.Info("SSH server listening", "address", sshSrv.Address())
				defer func() {
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer shutdownCancel()
					_ = sshSrv.Stop(shutdownCtx)
				}()
			}
		}
	}

	if len(lgAddrs) > 0 {
		lgDispatch := func(cmd string) (string, error) { return dispatch(cmd, "", "") }
		if lgSrv := startLGServer(store, lgAddrs, lgTLS, lgDispatch, resolvers); lgSrv != nil {
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer shutdownCancel()
				_ = lgSrv.Shutdown(shutdownCtx)
			}()
		}
	}

	var mcpSrv *MCPServerHandle
	if len(mcpAddrs) > 0 {
		mcpStreamCfg := zemcp.StreamableConfig{Token: mcpToken}
		var mcpTLSCert, mcpTLSKey string
		if mcpCfgOK {
			mcpStreamCfg = mcpConfigToStreamable(mcpCfg, mcpStreamCfg)
			mcpTLSCert = mcpCfg.TLS.Cert
			mcpTLSKey = mcpCfg.TLS.Key
		}
		mcpSrv = startMCPServer(mcpAddrs, dispatch, serverCommandLister(apiServer), mcpStreamCfg, mcpTLSCert, mcpTLSKey)
	}

	// Start REST/gRPC API servers if configured (env > config file).
	var apiSrvs *apiServers
	apiCfg, apiCfgOK := zeconfig.ExtractAPIConfig(loadResult.Tree)
	if env.IsEnabled("ze.api-server.rest.enabled") && !apiCfg.RESTOn {
		apiCfg.RESTOn = true
		apiCfg.REST = []zeconfig.APIListenConfig{{Host: "0.0.0.0", Port: "8081"}}
		apiCfgOK = true
	}
	if listen := env.Get("ze.api-server.rest.listen"); listen != "" && apiCfg.RESTOn {
		host, port, parseErr := net.SplitHostPort(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.api.rest.listen: %v\n", parseErr)
			return 1
		}
		// Env-var override replaces the config-provided list with one entry.
		// Compound multi-listener env support lands in a later chunk.
		apiCfg.REST = []zeconfig.APIListenConfig{{Host: host, Port: port}}
	}
	if env.IsEnabled("ze.api-server.grpc.enabled") && !apiCfg.GRPCOn {
		apiCfg.GRPCOn = true
		apiCfg.GRPC = []zeconfig.APIListenConfig{{Host: "0.0.0.0", Port: "50051"}}
		apiCfgOK = true
	}
	if listen := env.Get("ze.api-server.grpc.listen"); listen != "" && apiCfg.GRPCOn {
		host, port, parseErr := net.SplitHostPort(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.api.grpc.listen: %v\n", parseErr)
			return 1
		}
		apiCfg.GRPC = []zeconfig.APIListenConfig{{Host: host, Port: port}}
	}
	if token := env.Get("ze.api-server.token"); token != "" && apiCfg.Token == "" {
		apiCfg.Token = token
	}
	if apiCfgOK {
		// Load zefs users for per-user auth; if unavailable, falls back to Token.
		var apiUsers []authz.UserConfig
		if storage.IsBlobStorage(store) {
			u, uErr := loadZefsUsers()
			switch {
			case uErr != nil:
				fmt.Fprintf(os.Stderr, "warning: API per-user auth disabled: load zefs users: %v\n", uErr)
			default:
				apiUsers = u
			}
		} else {
			fmt.Fprintln(os.Stderr, "warning: API per-user auth disabled: requires blob storage (run ze init first)")
		}

		// Report active auth mode to make silent degradation visible.
		switch {
		case len(apiUsers) > 0:
			fmt.Fprintf(os.Stderr, "API auth mode: per-user (%d users from zefs)\n", len(apiUsers))
		case apiCfg.Token != "":
			fmt.Fprintln(os.Stderr, "API auth mode: single-token (shared bearer)")
		default:
			fmt.Fprintln(os.Stderr, "warning: API auth mode: NONE (no users, no token) -- set ze.api-server.token or initialize zefs")
		}

		if len(apiUsers) == 0 && apiCfg.Token == "" && apiHasNonLoopback(apiCfg) {
			fmt.Fprintln(os.Stderr, "error: refusing to start API on non-loopback listener without authentication")
			fmt.Fprintln(os.Stderr, "  set ze.api-server.token, initialize zefs users, or bind to 127.0.0.1/::1 only")
			return 1
		}

		reloadAfterCommit := func() error {
			return doReload(apiServer, eng, configProvider, loadConfigFromDisk)
		}
		var apiErr error
		apiSrvs, apiErr = startAPIServers(apiCfg, apiServer, store, configPath, apiUsers, reloadAfterCommit)
		if apiErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", apiErr)
			apiServer.Stop()
			_ = eng.Stop(startCtx)
			return 1
		}
	}

	// Signal handling: SIGINT/SIGTERM for shutdown, SIGHUP for config reload.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// SIGHUP reload worker: re-reads config from disk, auto-loads/stops plugins,
	// refreshes the shared ConfigProvider, then notifies every registered
	// subsystem so it can hot-apply diff-able knobs.
	reloadCh := make(chan os.Signal, 1)
	go handleSIGHUPReload(reloadCh, apiServer, eng, configProvider, loadConfigFromDisk)

	if stdinOpen {
		go monitorStdinEOF(sigCh)
	}

	fmt.Printf("Starting ze with config: %s\n", configPath)

	// Wait for all plugins to complete startup (BGP reactor starts, peers connect, etc.)
	// before signaling readiness. The test infrastructure polls ze.ready.file.
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := apiServer.WaitForStartupComplete(startupCtx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		startupCancel()
		apiServer.Stop()
		_ = eng.Stop(startCtx)
		return 1
	}
	startupCancel()

	if readyFile := env.Get("ze.ready.file"); readyFile != "" {
		if f, createErr := os.Create(readyFile); createErr == nil { //nolint:gosec // test infrastructure path from env
			f.Close() //nolint:errcheck,gosec // best-effort readiness signal
		}
	}

	fmt.Println("Ze running. Press Ctrl+C to stop.")

	// Wait for either signal or server shutdown (e.g., "daemon shutdown" command).
	// Server.Wait blocks until all plugin processes exit -- happens when a plugin
	// dispatches "daemon shutdown" which calls reactor.Stop().
	// Only listen for server-done when plugins actually started; otherwise the
	// WaitGroup is zero from the start and Wait returns immediately -- causing
	// the daemon to exit before SSH/web servers are ready (breaks "config edit").
	if apiServer.HasProcesses() {
		doneCh := make(chan struct{})
		go waitForServerDone(apiServer, doneCh)

		waitLoop(sigCh, reloadCh, doneCh)
	} else {
		waitLoop(sigCh, reloadCh, nil)
	}
	close(reloadCh)
	fmt.Println("\nShutting down...")

	if mcpSrv != nil {
		mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = mcpSrv.Shutdown(mcpCtx)
		mcpCancel()
	}

	if apiSrvs != nil {
		apiCtx, apiCancel := context.WithTimeout(context.Background(), 3*time.Second)
		apiSrvs.Shutdown(apiCtx)
		apiCancel()
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	apiServer.Stop()
	if err := eng.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: shutdown timeout: %v\n", err)
	}

	fmt.Println("Ze stopped.")

	if rebootRequested.Load() {
		fmt.Println("Initiating system reboot...")
		if err := reboot.Reboot(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}

	return 0
}

// waitForServerDone blocks until the plugin server's Wait returns, then closes doneCh.
// Lifecycle goroutine (one-time, not hot path): bridges Server.Wait to a select channel.
func waitForServerDone(s *pluginserver.Server, doneCh chan struct{}) {
	_ = s.Wait(context.Background())
	close(doneCh)
}

// handleSIGHUPReload is the SIGHUP reload worker. Reads signals from reloadCh,
// triggers plugin-level reload via ReloadFromDisk, refreshes the shared
// ConfigProvider with the freshly loaded tree, then fans Reload out to every
// registered subsystem (engine.Reload) so diff-able knobs hot-apply.
// If a transaction is in progress (lock held), the SIGHUP is queued and replayed
// after the current reload completes.
// Lifecycle goroutine (one-time, runs for daemon lifetime).
func handleSIGHUPReload(reloadCh <-chan os.Signal, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, load func() (map[string]any, error)) {
	for range reloadCh {
		fmt.Fprintf(os.Stderr, "received SIGHUP, reloading config...\n")
		if err := doReload(s, eng, cp, load); err != nil {
			if errors.Is(err, pluginserver.ErrReloadInProgress) {
				fmt.Fprintf(os.Stderr, "transaction in progress, queuing SIGHUP...\n")
				s.QueueSIGHUP()
				continue
			}
			fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
		}
		// After reload completes, drain any queued SIGHUP.
		if s.DrainSIGHUP() {
			fmt.Fprintf(os.Stderr, "replaying queued SIGHUP...\n")
			if err := doReload(s, eng, cp, load); err != nil {
				fmt.Fprintf(os.Stderr, "queued reload error: %v\n", err)
			}
		}
	}
}

// doReload performs a single config reload with a 30-second timeout.
//
// The load/plugin-apply/provider-refresh/subsystem-reload sequence runs in
// lock-step from a SINGLE tree snapshot:
//
//  1. load() reads and parses the config file once.
//  2. ReloadConfig(ctx, newTree) drives the plugin-server diff + plugin
//     verify/apply path with that tree (public API that accepts a
//     pre-parsed tree, so we don't re-read the file).
//  3. The shared ConfigProvider is refreshed root-by-root from the same
//     tree: new/changed roots are SetRoot'd, orphan roots (present in
//     cp but absent from newTree) get an empty map so watchers see the
//     removal.
//  4. engine.Reload(ctx) fans the refreshed provider out to every
//     registered subsystem so they hot-apply diff-able knobs (e.g.,
//     l2tp shared-secret / hello-interval).
//
// Keeping the tree single-sourced eliminates the race where the file
// changes between the plugin-server read and the subsystem read, and
// avoids redundant I/O + YANG parse on every SIGHUP.
func doReload(s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, load func() (map[string]any, error)) error {
	reloadCtx, reloadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reloadCancel()

	if load == nil {
		// stdin-config daemons have no reload source. Fall back to the
		// plugin server's own ReloadFromDisk (which also errors if no
		// loader is configured) so the error message stays familiar.
		return s.ReloadFromDisk(reloadCtx)
	}

	newTree, loadErr := load()
	if loadErr != nil {
		return fmt.Errorf("reload: parse config: %w", loadErr)
	}

	if err := s.ReloadConfig(reloadCtx, newTree); err != nil {
		return err
	}

	if cp != nil {
		priorRoots := cp.Roots()
		existing := make(map[string]struct{}, len(priorRoots))
		for _, k := range priorRoots {
			existing[k] = struct{}{}
		}
		for root, subtree := range newTree {
			sub, ok := subtree.(map[string]any)
			if !ok {
				continue
			}
			cp.SetRoot(root, sub)
			delete(existing, root)
		}
		// Any root left in `existing` disappeared from the new tree.
		// DeleteRoot removes the entry entirely (not just emptied) so
		// the next reload does not re-run the orphan path for the same
		// root and re-fire watcher notifications.
		for orphan := range existing {
			cp.DeleteRoot(orphan)
		}
	}

	if eng != nil {
		if err := eng.Reload(reloadCtx); err != nil {
			return fmt.Errorf("subsystem reload: %w", err)
		}
	}
	return nil
}

// waitLoop dispatches signals: SIGHUP to reloadCh, others trigger shutdown return.
// If doneCh is non-nil, also returns when it closes (server exit).
func waitLoop(sigCh <-chan os.Signal, reloadCh chan<- os.Signal, doneCh <-chan struct{}) {
	for {
		if doneCh != nil {
			select {
			case sig := <-sigCh:
				if sig == syscall.SIGHUP {
					reloadCh <- sig
					continue
				}
				return
			case <-doneCh:
				return
			}
		} else {
			sig := <-sigCh
			if sig == syscall.SIGHUP {
				reloadCh <- sig
				continue
			}
			return
		}
	}
}

// webOnlyDispatcher creates a minimal CommandDispatcher backed by a local event
// ring. Used by RunWebOnly where no plugin server exists.
func webOnlyDispatcher(ring *pluginserver.EventRing) zeweb.CommandDispatcher {
	return func(command, _, _ string) (string, error) {
		switch {
		case strings.HasPrefix(command, "show event namespaces"):
			counts := ring.NamespaceCounts()
			rows := make([]map[string]any, 0, len(counts))
			for ns, count := range counts {
				rows = append(rows, map[string]any{"namespace": ns, "count": count})
			}
			b, err := json.Marshal(map[string]any{"namespaces": rows})
			if err != nil {
				return "", err
			}
			return string(b), nil

		case strings.HasPrefix(command, "show event recent"):
			namespace := ""
			if _, after, ok := strings.Cut(command, "namespace "); ok {
				namespace = strings.TrimSpace(after)
			}
			records := ring.Snapshot(50, namespace)
			out := make([]map[string]any, 0, len(records))
			for i := range records {
				out = append(out, map[string]any{
					"timestamp":  records[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
					"namespace":  records[i].Namespace,
					"event-type": records[i].EventType,
				})
			}
			b, err := json.Marshal(map[string]any{"events": out, "count": len(out)})
			if err != nil {
				return "", err
			}
			return string(b), nil

		default:
			return "", fmt.Errorf("command not available in web-only mode: %s", command)
		}
	}
}

// serverDispatcher creates a CommandDispatcher from the plugin server's dispatcher.
func serverDispatcher(s *pluginserver.Server) func(command, username, remoteAddr string) (string, error) {
	return func(input, username, remoteAddr string) (string, error) {
		d := s.Dispatcher()
		if d == nil {
			return "", fmt.Errorf("server not ready")
		}
		ctx := &pluginserver.CommandContext{Server: s, Username: username, RemoteAddr: remoteAddr}
		resp, err := d.Dispatch(ctx, input)
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", nil
		}
		data, ok := resp.Data.(string)
		if !ok {
			b, jsonErr := json.Marshal(resp.Data)
			if jsonErr != nil {
				return "", fmt.Errorf("marshal response: %w", jsonErr)
			}
			return string(b), nil
		}
		return data, nil
	}
}

// serverCommandLister creates a CommandLister from the plugin server's dispatcher.
func serverCommandLister(s *pluginserver.Server) zemcp.CommandLister {
	var (
		paramOnce    sync.Once
		paramsByPath map[string][]zemcp.ParamInfo
	)

	initParams := func() {
		paramOnce.Do(func() {
			paramsByPath = buildParamMap()
		})
	}

	return func() []zemcp.CommandInfo {
		d := s.Dispatcher()
		if d == nil {
			return nil
		}

		initParams()

		var infos []zemcp.CommandInfo
		for _, cmd := range d.Commands() {
			infos = append(infos, zemcp.CommandInfo{
				Name:     cmd.Name,
				Help:     cmd.Help,
				ReadOnly: cmd.ReadOnly,
				Params:   paramsByPath[cmd.Name],
			})
		}

		// Plugin-registered commands.
		for _, cmd := range d.Registry().All() {
			infos = append(infos, zemcp.CommandInfo{
				Name: cmd.Name,
				Help: cmd.Description,
			})
		}

		return infos
	}
}

// endpointsToAddrs converts a slice of config.ServerEndpoint into the
// "host:port" string slice that every multi-listener binder accepts.
func endpointsToAddrs(servers []zeconfig.ServerEndpoint) []string {
	out := make([]string, 0, len(servers))
	for _, ep := range servers {
		out = append(out, ep.Listen())
	}
	return out
}

// startWebServer creates and starts the web server with zefs credentials.
// Returns the server and SSE event broker on success, nil on failure (logged, non-fatal).
// Caller MUST call broker.Close() during shutdown to release SSE clients.
// Every entry in listenAddrs becomes a bound listener on the same
// *http.Server; Shutdown closes all of them.
// Requires blob storage -- TLS keys and config must not leak to the filesystem.
func startWebServer(store storage.Storage, listenAddrs []string, insecureWeb bool, dispatch zeweb.CommandDispatcher, resolvers *resolve.Resolvers) (*zeweb.WebServer, *zeweb.EventBroker) {
	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: requires blob storage (run ze init first)\n")
		return nil, nil
	}

	if len(listenAddrs) == 0 {
		listenAddrs = []string{"0.0.0.0:3443"}
	}

	var users []authz.UserConfig
	if !insecureWeb {
		var err error
		users, err = loadZefsUsers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: %v\n", err)
			return nil, nil
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: authentication disabled (--insecure-web)\n")
	}

	// Persist TLS cert in zefs so browsers don't have to re-accept on every restart.
	// The SAN hint is derived from the first endpoint; GenerateWebCertWithAddr
	// already fans out to all interface IPs when the host is 0.0.0.0.
	certStore := &blobCertStore{store: store}
	certPEM, keyPEM, err := zeweb.LoadOrGenerateCert(certStore, listenAddrs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: TLS cert: %v\n", err)
		return nil, nil
	}

	renderer, err := zeweb.NewRenderer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: renderer: %v\n", err)
		return nil, nil
	}

	// Register display-time decorators (e.g., ASN -> org name via Team Cymru DNS).
	decorators := zeweb.NewDecoratorRegistry()
	if resolvers != nil && resolvers.Cymru != nil {
		decorators.Register(zeweb.NewASNNameDecoratorFromCymru(resolvers.Cymru))
	}
	renderer.SetDecorators(decorators)

	srv, err := zeweb.NewWebServer(zeweb.WebConfig{
		ListenAddrs: listenAddrs,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: %v\n", err)
		return nil, nil
	}

	// Load YANG schema for config tree navigation.
	schema, schemaErr := zeconfig.YANGSchema()
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: YANG schema: %v\n", schemaErr)
		return nil, nil
	}

	// Strict ze:related validation against the full operational command
	// tree. Surfaces typos and renamed-command drift at hub startup so
	// operators see the diagnostic before any workbench click. Logged as
	// a warning (not fatal) so a single drifted descriptor never prevents
	// the hub from serving the rest of the UI.
	if loader, loaderErr := yangloader.DefaultLoader(); loaderErr == nil {
		commandTree := yangloader.BuildCommandTree(loader)
		if validateErr := zeconfig.ValidateSchemaAgainstCommandTree(schema, commandTree); validateErr != nil {
			fmt.Fprintf(os.Stderr, "warning: ze:related validation: %v\n", validateErr)
		}
	}

	// Ensure a config file exists for the editor.
	configPath := resolveConfigPath(store)
	if !store.Exists(configPath) {
		if writeErr := store.WriteFile(configPath, []byte("# ze config\n"), 0o600); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot create config: %v\n", writeErr)
		}
	}

	// Parse the committed config into the live baseline tree used by compare
	// targets. Saved drafts are a separate source, not the startup-loaded config.
	tree := zeconfig.NewTree()
	if configData, readErr := store.ReadFile(configPath); readErr == nil {
		if parsed, parseErr := zeconfig.ParseTreeWithYANG(string(configData), nil); parseErr == nil {
			tree = parsed
		}
	}

	// Create editor manager for config editing via web.
	editorMgr := zeweb.NewEditorManager(store, configPath, schema, newEditorFactory(), newEditSessionFactory())

	// Create CLI completer for Tab/? autocomplete.
	completer := cli.NewCompleter()

	sessionStore := zeweb.NewSessionStore()
	loginRenderer := func(w http.ResponseWriter, r *http.Request) {
		if renderErr := renderer.RenderLogin(w, zeweb.LoginData{ReturnTo: r.URL.RequestURI()}); renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	}

	// SSE broker for live config change notifications and log streaming.
	broker := zeweb.NewEventBroker(0)

	// Both UIs are always available. The ze.web.ui env var (default: finder)
	// controls which one /show/ renders when no ze-ui cookie is set. Users
	// switch at runtime via the Finder/Workbench links in the topbar.
	uiMode := zeweb.GetUIMode()
	finderHandler := zeweb.HandleFragment(renderer, schema, tree, editorMgr, insecureWeb)
	workbenchHandler := zeweb.HandleWorkbench(renderer, schema, tree, editorMgr, insecureWeb,
		zeweb.WithDispatch(dispatch), zeweb.WithBroker(broker))
	fmt.Fprintf(os.Stderr, "web UI default: %s\n", uiMode)
	showHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch zeweb.ReadUIModeFromRequest(r, uiMode) {
		case zeweb.UIModeWorkbench:
			workbenchHandler(w, r)
		default:
			finderHandler(w, r)
		}
	})
	// Fragment handler still serves /fragment/detail HTMX requests regardless
	// of mode; both UIs share the same OOB swap path.
	fragmentHandler := finderHandler

	// Config set, add, and delete handlers for editing leaf values.
	setHandler := zeweb.HandleConfigSet(editorMgr, schema, renderer)
	addHandler := zeweb.HandleConfigAdd(editorMgr, schema, renderer)
	addFormHandler := zeweb.HandleConfigAddForm(editorMgr, schema, renderer)
	renameHandler := zeweb.HandleConfigRename(editorMgr, schema)
	deleteHandler := zeweb.HandleConfigDelete(editorMgr)

	// Commit and discard handlers.
	commitHandler := zeweb.HandleConfigCommit(editorMgr, renderer, broker)
	discardHandler := zeweb.HandleConfigDiscard(editorMgr)

	// Diff handler: returns the diff modal HTML (open, with content).
	diffHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := zeweb.GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		diff, _ := editorMgr.Diff(username)
		count := editorMgr.ChangeCount(username)
		type diffData struct {
			Diff        string
			ChangeCount int
		}
		html := renderer.RenderFragment("diff_modal_open", diffData{Diff: diff, ChangeCount: count})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	})

	// Diff close: returns the closed modal HTML.
	diffCloseHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		html := renderer.RenderFragment("diff_modal", nil)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	})

	// CLI handlers: command execution, autocomplete, terminal mode.
	cliHandler := zeweb.HandleCLICommand(editorMgr, schema, renderer)
	completeHandler := zeweb.HandleCLIComplete(completer, editorMgr, schema)
	terminalHandler := zeweb.HandleCLITerminal(editorMgr, schema, tree)
	modeHandler := zeweb.HandleCLIModeToggle(editorMgr, schema, renderer)

	// Auth wrapper for protecting individual routes.
	webAuth := &authz.LocalAuthenticator{Users: users}
	var authWrap func(http.Handler) http.Handler
	if insecureWeb {
		authWrap = zeweb.InsecureMiddleware
	} else {
		authWrap = func(h http.Handler) http.Handler {
			return zeweb.AuthMiddleware(sessionStore, webAuth, loginRenderer, h)
		}
	}
	mutationWrap := func(h http.Handler) http.Handler {
		return authWrap(zeweb.RequireSameOrigin(h))
	}

	loginHandler := zeweb.LoginHandler(sessionStore, webAuth, loginRenderer)
	assetHandler := http.StripPrefix("/assets/", renderer.AssetHandler())

	// Admin command tree for web UI. Derive from the merged YANG command
	// tree so plugin-contributed commands appear in the admin nav without
	// editing the static map (spec-web-2 Phase 6 / Spec D6). The static
	// fallback was removed because its tree shape (`peer/route/cache`)
	// drifted from the YANG-derived shape (`peer/show/summary/...`); a
	// silent fallback after loader failure would surface as broken admin
	// links rather than a clear error.
	var adminChildren map[string][]string
	if loader, loaderErr := yangloader.DefaultLoader(); loaderErr == nil {
		adminChildren = zeweb.AdminTreeFromYANG(yangloader.BuildCommandTree(loader))
	} else {
		fmt.Fprintf(os.Stderr, "warning: admin command tree unavailable: %v\n", loaderErr)
		adminChildren = map[string][]string{}
	}
	adminViewHandler := zeweb.HandleAdminView(renderer, adminChildren)
	adminExecHandler := zeweb.HandleAdminExecute(renderer, dispatch)

	srv.HandleFunc("POST /login", loginHandler)
	srv.Handle("/assets/", assetHandler)
	srv.Handle("/events", authWrap(broker))
	srv.Handle("/admin/", mutationWrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			adminExecHandler(w, r)
			return
		}
		adminViewHandler(w, r)
	})))
	srv.Handle("GET /cli", authWrap(zeweb.HandleCLIPageHTTP(renderer, insecureWeb)))
	srv.Handle("POST /cli", mutationWrap(cliHandler))
	srv.Handle("/cli/complete", authWrap(completeHandler))
	srv.Handle("POST /cli/terminal", mutationWrap(terminalHandler))
	srv.Handle("POST /cli/mode", mutationWrap(modeHandler))
	srv.Handle("/fragment/detail", authWrap(fragmentHandler))
	srv.Handle("POST /config/set/", mutationWrap(setHandler))
	srv.Handle("POST /config/add/", mutationWrap(addHandler))
	srv.Handle("GET /config/add-form/", authWrap(addFormHandler))
	srv.Handle("POST /config/rename/", mutationWrap(renameHandler))
	srv.Handle("GET /config/changes", authWrap(zeweb.HandleConfigChanges(editorMgr, renderer)))
	srv.Handle("POST /config/delete/", mutationWrap(deleteHandler))
	srv.Handle("/config/diff", authWrap(diffHandler))
	srv.Handle("/config/diff-close", authWrap(diffCloseHandler))
	srv.Handle("/config/commit", mutationWrap(commitHandler))
	srv.Handle("POST /config/discard", mutationWrap(discardHandler))
	// V2 workbench related-tool execution. Browser submits only tool id +
	// context path; the handler resolves the descriptor server-side and
	// dispatches via the standard CommandDispatcher (same authz pipeline
	// as /cli and /admin).
	srv.Handle("POST /tools/related/run", mutationWrap(zeweb.HandleRelatedToolRun(renderer, schema, tree, editorMgr, dispatch)))
	// L2TP web UI: session list, detail, CQM chart feeds, disconnect.
	l2tpHandlers := &zeweb.L2TPHandlers{Renderer: renderer, Dispatch: dispatch}
	srv.Handle("GET /l2tp", authWrap(l2tpHandlers.HandleL2TPList()))
	srv.Handle("GET /l2tp/{sid}", authWrap(l2tpHandlers.HandleL2TPDetail()))
	srv.Handle("POST /l2tp/{sid}/disconnect", mutationWrap(l2tpHandlers.HandleL2TPDisconnect()))
	srv.Handle("GET /l2tp/{login}/samples", authWrap(zeweb.HandleL2TPSamplesJSON()))
	srv.Handle("GET /l2tp/{login}/samples.csv", authWrap(zeweb.HandleL2TPSamplesCSV()))
	srv.Handle("GET /l2tp/{login}/samples/stream", authWrap(zeweb.HandleL2TPSamplesSSE()))

	// Portal: iframe wrapper for embedded services (gokrazy, etc.).
	if env.IsEnabled("ze.gokrazy.enabled") {
		srv.Handle("/gokrazy/", authWrap(zegokrazy.Handler(env.Get("ze.gokrazy.socket"))))
		zeweb.RegisterPortalService(zeweb.PortalService{
			Key: "gokrazy", Title: "Gokrazy", Path: "/gokrazy/",
			Icon: "/gokrazy/assets/gokrazy-logo.svg",
		})
	}
	srv.Handle("/portal/", authWrap(zeweb.HandlePortal(renderer, uiMode)))
	srv.Handle("GET /health", authWrap(health.DefaultRegistry.Handler()))
	srv.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)
			return
		}
		// /show and /monitor go through the active UI handler; everything else
		// (e.g. /fragment/detail HTMX requests) keeps using the Finder fragment
		// handler so the OOB swap protocol is identical across both UIs.
		if strings.HasPrefix(r.URL.Path, "/show/") || strings.HasPrefix(r.URL.Path, "/monitor/") {
			authWrap(showHandler).ServeHTTP(w, r)
			return
		}
		authWrap(fragmentHandler).ServeHTTP(w, r)
	})

	go func() {
		if serveErr := srv.ListenAndServe(context.Background()); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slogutil.Logger("web.server").Error("web server error", "error", serveErr)
		}
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if waitErr := srv.WaitReady(readyCtx); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web server failed to start: %v\n", waitErr)
		_ = srv.Shutdown(context.Background())
		return nil, nil
	}

	fmt.Fprintf(os.Stderr, "web server listening on https://%s/\n", srv.Address())
	return srv, broker
}

// loadZefsUsers reads credentials from the zefs database (created by ze init).
func loadZefsUsers() ([]authz.UserConfig, error) {
	dir := env.Get("ze.config.dir")
	if dir == "" {
		dir = paths.DefaultConfigDir()
	}
	if dir == "" {
		return nil, fmt.Errorf("cannot resolve config directory")
	}
	dbPath := filepath.Join(dir, "database.zefs")
	db, err := zefs.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer db.Close() //nolint:errcheck // read-only access
	username, err := db.ReadFile(zefs.KeySSHUsername.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read username: %w", err)
	}
	hash, err := db.ReadFile(zefs.KeySSHPassword.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read password hash: %w", err)
	}
	name := string(username)
	if name == "" {
		return nil, fmt.Errorf("empty username in zefs")
	}
	return []authz.UserConfig{{Name: name, Hash: string(hash)}}, nil
}

// blobCertStore implements web.CertStore backed by zefs blob storage.
type blobCertStore struct {
	store storage.Storage
}

func (s *blobCertStore) ReadCert() ([]byte, error) { return s.store.ReadFile(zefs.KeyWebCert.Pattern) }
func (s *blobCertStore) ReadKey() ([]byte, error)  { return s.store.ReadFile(zefs.KeyWebKey.Pattern) }
func (s *blobCertStore) WriteCert(data []byte) error {
	return s.store.WriteFile(zefs.KeyWebCert.Pattern, data, 0o600)
}
func (s *blobCertStore) WriteKey(data []byte) error {
	return s.store.WriteFile(zefs.KeyWebKey.Pattern, data, 0o600)
}
func (s *blobCertStore) Exists() bool {
	return s.store.Exists(zefs.KeyWebCert.Pattern) && s.store.Exists(zefs.KeyWebKey.Pattern)
}

// resolveConfigPath returns the config file path for the editor.
func resolveConfigPath(store storage.Storage) string {
	data, err := store.ReadFile(zefs.KeyInstanceName.Pattern)
	if err == nil && len(data) > 0 {
		name := strings.TrimSpace(string(data))
		if name != "" {
			return name + ".conf"
		}
	}
	return "ze.conf"
}

// startLGServer creates and starts the looking glass HTTP server.
// Returns the server on success, nil on failure (logged, non-fatal).
// Every entry in listenAddrs becomes a bound listener on the same
// *http.Server; Shutdown closes all of them.
func startLGServer(store storage.Storage, listenAddrs []string, useTLS bool, dispatch lg.CommandDispatcher, resolvers *resolve.Resolvers) *lg.LGServer {
	if len(listenAddrs) == 0 {
		return nil
	}
	cfg := lg.LGConfig{
		ListenAddrs: listenAddrs,
		TLS:         useTLS,
		Dispatch:    dispatch,
		DecorateASN: func(asn string) string {
			if resolvers == nil || resolvers.Cymru == nil {
				return ""
			}
			name, _ := resolvers.Cymru.LookupASNName(context.Background(), parseASNForDecorator(asn))
			return name
		},
	}

	// When TLS is enabled, load or generate cert from blob storage. The SAN
	// hint is derived from the first endpoint; GenerateWebCertWithAddr
	// already fans out to all interface IPs when the host is 0.0.0.0.
	if useTLS {
		if !storage.IsBlobStorage(store) {
			fmt.Fprintf(os.Stderr, "error: looking glass TLS requires blob storage (run ze init first)\n")
			return nil
		}
		certStore := &blobCertStore{store: store}
		certPEM, keyPEM, err := zeweb.LoadOrGenerateCert(certStore, listenAddrs[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: looking glass TLS cert: %v\n", err)
			return nil
		}
		cfg.CertPEM = certPEM
		cfg.KeyPEM = keyPEM
	}

	srv, err := lg.NewLGServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: looking glass disabled: %v\n", err)
		return nil
	}

	// Component startup goroutine (one-time, same pattern as startWebServer).
	serveLG(srv)

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if waitErr := srv.WaitReady(readyCtx); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: looking glass server failed to start: %v\n", waitErr)
		_ = srv.Shutdown(context.Background())
		return nil
	}

	scheme := "http"
	if cfg.TLS {
		scheme = "https"
	}
	for _, addr := range srv.Addresses() {
		fmt.Fprintf(os.Stderr, "looking glass listening on %s://%s/\n", scheme, addr)
	}
	return srv
}

// serveLG runs the LG server's ListenAndServe in a background goroutine.
// This is a one-time component startup, not a per-event goroutine.
func serveLG(srv *lg.LGServer) {
	go serveLGBlocking(srv)
}

func serveLGBlocking(srv *lg.LGServer) {
	if serveErr := srv.ListenAndServe(context.Background()); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		slogutil.Logger("lg.server").Error("looking glass server error", "error", serveErr)
	}
}

// dropPrivileges drops to the user/group from ze.user/ze.group env vars.
// Called after port binding, before accepting connections or spawning plugins.
// No-op if not running as root or if ze.user is not set.
// Warns if running as root without ze.user configured.
func dropPrivileges() error {
	cfg := privilege.DropConfigFromEnv()
	if cfg.User == "" {
		if os.Getuid() == 0 {
			fmt.Fprintln(os.Stderr, "warning: running as root, set ze.user to drop privileges")
		}
		return nil
	}
	return privilege.Drop(cfg)
}

// monitorStdinEOF blocks until stdin is closed (EOF or error), then sends
// SIGTERM to sigCh to trigger reactor shutdown.
func monitorStdinEOF(sigCh chan<- os.Signal) {
	b := make([]byte, 1)
	if _, err := os.Stdin.Read(b); err != nil {
		fmt.Fprintf(os.Stderr, "\nUpstream pipe closed (%v), shutting down...\n", err)
	} else {
		fmt.Fprintln(os.Stderr, "\nUpstream pipe closed, shutting down...")
	}
	select {
	case sigCh <- syscall.SIGTERM:
	default:
	}
}

// runOrchestratorWithData parses hub config and runs the orchestrator.
func runOrchestratorWithData(store storage.Storage, configPath string, data []byte) int {
	cfg, err := hub.ParseHubConfig(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse config: %v\n", err)
		return 1
	}
	cfg.ConfigPath = configPath

	o := hub.NewOrchestrator(cfg)
	o.SetStorage(store)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				fmt.Fprintf(os.Stderr, "received %s, shutting down...\n", sig)
				cancel()
				return
			case syscall.SIGHUP:
				fmt.Fprintf(os.Stderr, "received SIGHUP, reloading config...\n")
				if err := o.Reload(configPath); err != nil {
					fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
					cancel()
					return
				}
			}
		}
	}()

	// Start orchestrator
	if err := o.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: start: %v\n", err)
		return 1
	}

	// Drop privileges after port binding.
	if err := dropPrivileges(); err != nil {
		fmt.Fprintf(os.Stderr, "error: drop privileges: %v\n", err)
		o.Stop()
		return 1
	}

	fmt.Fprintf(os.Stderr, "hub: started with config %s\n", configPath)

	// Signal readiness to test infrastructure. Written after signal.Notify
	// and o.Start so the test runner knows signal handlers are registered.
	if readyFile := env.Get("ze.ready.file"); readyFile != "" {
		if f, err := os.Create(readyFile); err == nil { //nolint:gosec // test infrastructure path from env
			f.Close() //nolint:errcheck,gosec // best-effort readiness signal
		}
	}

	// Wait for shutdown
	<-ctx.Done()

	// Clean shutdown — stop signal handler goroutine before returning.
	signal.Stop(sigCh)
	close(sigCh)
	o.Stop()
	return 0
}

// newResolvers creates a shared Resolvers struct with a single DNS instance
// and a Cymru resolver wired to it. Called once at hub startup.
func newResolvers(sc system.SystemConfig) *resolve.Resolvers {
	cfg := resolveDNS.ResolverConfig{
		Timeout:        sc.DNSTimeout,
		ResolvConfPath: sc.ResolvConfPath,
		CacheSize:      sc.DNSCacheSize,
		CacheTTL:       sc.DNSCacheTTL,
	}
	if len(sc.NameServers) > 0 {
		cfg.Server = sc.NameServers[0]
	}

	dnsResolver := resolveDNS.NewResolver(cfg)

	// Wrap DNS ResolveTXT to match Cymru's TXTResolver signature (adds context).
	txtResolver := func(_ context.Context, name string) ([]string, error) {
		return dnsResolver.ResolveTXT(name)
	}

	return &resolve.Resolvers{
		DNS:       dnsResolver,
		Cymru:     cymru.New(txtResolver, nil),
		PeeringDB: peeringdb.NewPeeringDB(sc.PeeringDBURL),
		IRR:       irr.NewIRR(""),
	}
}

// parseASNForDecorator converts an ASN string to uint32 for the Cymru resolver.
// Returns 0 on parse failure (Cymru handles ASN 0 gracefully).
func parseASNForDecorator(asn string) uint32 {
	var n uint64
	for _, c := range asn {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + uint64(c-'0')
		if n > 4294967295 {
			return 0
		}
	}
	return uint32(n)
}

// startStandaloneTelemetry starts the Prometheus metrics server when
// telemetry{} is configured but bgp{} is absent. Mirrors the startup
// logic in loader_create.go but without requiring a reactor.
type standaloneTelemetry struct {
	srv     metrics.Server
	manager *collector.Manager
}

func (st *standaloneTelemetry) Close() {
	if st.manager != nil {
		st.manager.Stop()
	}
	_ = st.srv.Close()
}

func startStandaloneTelemetry(tree *zeconfig.Tree) *standaloneTelemetry {
	telemetryCfg := metrics.ExtractTelemetryConfig(tree.ToMap())
	if !telemetryCfg.Enabled {
		return nil
	}

	st := &standaloneTelemetry{}
	reg := metrics.NewPrometheusRegistry()
	if err := st.srv.Start(reg, telemetryCfg); err != nil {
		slog.Warn("standalone telemetry: metrics server failed to start", "error", err)
		return nil
	}
	for _, path := range telemetryCfg.DeprecatedAliases {
		slog.Warn("standalone telemetry: deprecated prometheus config; move setting under telemetry.prometheus.netdata", "path", path)
	}
	for _, ep := range telemetryCfg.Endpoints {
		slog.Info("standalone telemetry: prometheus metrics enabled",
			"address", ep.Host, "port", ep.Port, "path", telemetryCfg.Path)
	}

	if telemetryCfg.Netdata.Enabled {
		overrides := make(map[string]collector.CollectorOverride, len(telemetryCfg.Netdata.Collectors))
		for name, cc := range telemetryCfg.Netdata.Collectors {
			overrides[name] = collector.CollectorOverride{
				Enabled:  cc.Enabled,
				Interval: time.Duration(cc.Interval) * time.Second,
			}
		}
		st.manager = collector.StartOSCollectors(reg, telemetryCfg.Netdata.Prefix, time.Duration(telemetryCfg.Netdata.Interval)*time.Second, overrides, slog.Default())
	}
	return st
}
