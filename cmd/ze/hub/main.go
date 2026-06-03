// Design: docs/architecture/hub-architecture.md -- hub CLI entry point
// Detail: mcp.go -- MCP server startup
// Detail: api.go -- REST/gRPC API server startup
// Detail: infra_setup.go -- infrastructure server setup hook
// Related: main_reload.go -- SIGHUP config reload
// Related: main_servers.go -- web, LG, SSH server startup
// Related: main_system.go -- host tuning, resolvers, telemetry
//
// Package hub provides the ze hub subcommand.
package hub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	mdns "github.com/miekg/dns"

	"codeberg.org/thomas-mangin/ze/internal/component/audit"
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	zecli "codeberg.org/thomas-mangin/ze/internal/component/cli/client"
	showCmd "codeberg.org/thomas-mangin/ze/internal/component/cmd/show"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	yangloader "codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	zegnmi "codeberg.org/thomas-mangin/ze/internal/component/gnmi"
	"codeberg.org/thomas-mangin/ze/internal/component/hub"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	_ "codeberg.org/thomas-mangin/ze/internal/component/ike/engine"
	_ "codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/managed"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
	zepki "codeberg.org/thomas-mangin/ze/internal/component/pki"
	zePlugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginmgr "codeberg.org/thomas-mangin/ze/internal/component/plugin/manager"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/pppoe"
	_ "codeberg.org/thomas-mangin/ze/internal/component/pppoeclient"
	resolvecmd "codeberg.org/thomas-mangin/ze/internal/component/resolve/cmd"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/privilege"
	"codeberg.org/thomas-mangin/ze/internal/core/reboot"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

var (
	errCannotResolveConfigDirectory = errors.New("cannot resolve config directory")
	errEmptyUsernameInZefs          = errors.New("empty username in zefs")
	errEmptyPasswordInZefs          = errors.New("empty password hash in zefs")
)

// Env var registrations are centralized in internal/component/config/environment.go.
// No duplicate registrations here -- import that package to trigger init.

// rebootRequested is set by the SSH/RPC reboot handler before triggering
// reactor.Stop(). After the graceful shutdown sequence completes, the main
// loop checks this flag and attempts an OS-level reboot if set.
var (
	rebootRequested atomic.Bool
	skipRootCheck   bool
)

// PeerLifecycleCallback is set by cmdStart when a pushed config was applied at boot.
// The reactor factory wires it as a peer lifecycle observer for health-based auto-revert.
var PeerLifecycleCallback registry.PeerLifecycleCallback

// RunWebOnly starts only the web server (no BGP engine).
// Used when ze start --web is called without a config.
// listenAddr overrides the default "0.0.0.0:3443" when non-empty.
func RunWebOnly(store storage.Storage, listenAddr string, insecureWeb bool) int {
	resolvers := newResolvers(&system.SystemConfig{DNSTimeout: 5, DNSCacheSize: 10000, DNSCacheTTL: 86400})
	defer resolvers.Close()
	if resolvers.DNS != nil {
		command.SetPTRResolver(resolvers.DNS)
	}
	if resolvers.Cymru != nil {
		command.SetOriginResolver(cymruOriginAdapter{resolvers.Cymru})
	}

	var listenAddrs []string
	if listenAddr != "" {
		listenAddrs = []string{listenAddr}
	}
	ring := pluginserver.NewEventRing(128)
	ring.Append("web", "server.started")
	dispatch := webOnlyDispatcher(ring)
	auditLog, auditErr := openAuditLog("")
	if auditErr != nil {
		fmt.Fprintf(os.Stderr, "error: audit log: %v\n", auditErr)
		return 1
	}
	showCmd.RegisterAuditProvider(auditLog.Query)
	// Web-only mode runs before any config is loaded, so there are no
	// config-file users; only the zefs power user authenticates here.
	webSrv, broker, _ := startWebServer(store, "", listenAddrs, insecureWeb, dispatch, resolvers, nil, auditLog, nil, nil)
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
func Run(store storage.Storage, configPath string, plugins []string, chaosSeed int64, chaosRate float64, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr, mcpToken string, cliAttach ...bool) int {
	return run(store, configPath, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, len(cliAttach) > 0 && cliAttach[0], nil)
}

// RunWithManagedClient executes the hub and starts the managed client after
// the runtime commit hook is available.
func RunWithManagedClient(store storage.Storage, configPath string, plugins []string, chaosSeed int64, chaosRate float64, managedClient *managed.ClientConfig) int {
	return run(store, configPath, plugins, chaosSeed, chaosRate, false, "", false, "", "", false, managedClient)
}

func run(store storage.Storage, configPath string, plugins []string, chaosSeed int64, chaosRate float64, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr, mcpToken string, cliAttach bool, managedClient *managed.ClientConfig) int {
	if !skipRootCheck {
		for _, w := range privilege.CheckPrivileges() {
			fmt.Fprintln(os.Stderr, "warning: "+w)
		}
	}

	// Read config content first (to probe type without parsing).
	// When reading from stdin, we look for a NUL sentinel that signals
	// "config complete but pipe stays open for liveness monitoring."
	var data []byte
	var stdinOpen bool
	var err error
	switch {
	case configPath == "-":
		data, stdinOpen, err = readStdinConfig()
	case storage.IsBlobStorage(store):
		if clearErr := clearStaleCandidateOnBoot(store, configPath); clearErr != nil {
			fmt.Fprintf(os.Stderr, "warning: clear stale candidate: %v\n", clearErr)
		}
		data, err = storage.ReadActiveConfig(store, configPath)
		if err != nil {
			// Config may live on the filesystem (e.g., gokrazy read-only root)
			// while blob handles TLS certs, SSH keys, and persistent state.
			data, err = os.ReadFile(configPath) //nolint:gosec // user-provided config path
		}
	default:
		data, err = store.ReadFile(configPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read config: %v\n", err)
		return 1
	}

	// Probe config type using shared function
	switch zeconfig.ProbeConfigType(string(data)) {
	case zeconfig.ConfigTypeBGP, zeconfig.ConfigTypeUnknown:
		// Non-BGP YANG config: auto-load plugins via ConfigRoots.
		return runYANGConfig(store, configPath, data, plugins, chaosSeed, chaosRate, stdinOpen, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, cliAttach, managedClient)
	case zeconfig.ConfigTypeHub:
		if len(plugins) > 0 {
			fmt.Fprintf(os.Stderr, "error: --plugin is not supported with hub/orchestrator configs; use plugin { external ... } in the config file\n")
			return 1
		}
		return runOrchestratorWithData(store, configPath, data)
	}

	return 1
}

func clearStaleCandidateOnBoot(store storage.Storage, configPath string) error {
	if store == nil || configPath == "" || configPath == "-" {
		return nil
	}
	return storage.ClearCandidate(store, configPath)
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
func runYANGConfig(store storage.Storage, configPath string, data []byte, plugins []string, chaosSeed int64, chaosRate float64, stdinOpen, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr, mcpToken string, cliAttach bool, managedClient *managed.ClientConfig) int { //nolint:cyclop // startup orchestration
	// Close the AAA bundle on every exit path so TACACS+ accounting and other
	// backend workers drain before the process terminates. swapAAABundle is
	// called by infraSetup on config load; closeAAABundle here matches it.
	defer closeAAABundle(slogutil.Logger("hub.aaa"))

	// Phase 1: Parse config and resolve plugins.
	loadResult, err := zeconfig.LoadConfig(string(data), configPath, plugins)
	if err != nil {
		if recovered, ok := zeconfig.RecoverConfig(store, configPath, data, plugins); ok {
			loadResult = recovered
		} else {
			fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
			return 1
		}
	}

	// Phase 1a: Apply persistent debug flags from zefs (after config-driven log levels).
	if bs, ok := storage.BlobStoreFrom(store); ok {
		slogutil.ApplyDebugFlags(bs)
	}

	// Phase 1b: Schema evolution. Apply registered evolutions newer than the
	// config's stamped release, then re-stamp and write back.
	evolveLogger := slogutil.Logger("hub.evolve")
	outcome, evolveErr := applyEvolutions(evolveLogger, store, configPath, data, loadResult.Tree, zeconfig.ScanStampRelease(data))
	if evolveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: schema evolution failed: %v\n", evolveErr)
	}
	loadResult.Tree = outcome.tree
	data = outcome.data

	if configPath != "" && configPath != "-" {
		if _, _, activeErr := storage.EnsureActiveVersion(store, configPath, data, time.Now()); activeErr != nil {
			fmt.Fprintf(os.Stderr, "error: initialize active config: %v\n", activeErr)
			return 1
		}
	}

	configPaths := zeconfig.CollectContainerPaths(loadResult.Tree)
	auditLog, auditErr := openAuditLog(configPath)
	if auditErr != nil {
		fmt.Fprintf(os.Stderr, "error: audit log: %v\n", auditErr)
		return 1
	}
	showCmd.RegisterAuditProvider(auditLog.Query)

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
	pkiConfig, pkiErr := preparePKIConfig(configTree)
	if pkiErr != nil {
		fmt.Fprintf(os.Stderr, "error: pki config: %v\n", pkiErr)
		return 1
	}
	if pkiErr := zepki.Load(pkiConfig); pkiErr != nil {
		fmt.Fprintf(os.Stderr, "error: pki config: %v\n", pkiErr)
		return 1
	}

	system.RegisterConntrackManagedKeys()
	// Register infrastructure hook before engine starts.
	// The BGP plugin calls this when creating the reactor.
	setupInfraHook(auditLog)
	coordinator := zePlugin.NewCoordinator(configTree)

	// Store config state for the BGP plugin's reactor factory.
	// The BGP plugin builds its own createReactor closure using these values.
	coordinator.SetExtra("bgp.configPath", configPath)
	coordinator.SetExtra("bgp.cliPlugins", plugins)
	coordinator.SetExtra("bgp.store", store)
	coordinator.SetExtra("bgp.configData", data)
	coordinator.SetExtra("bgp.chaosSeed", chaosSeed)
	coordinator.SetExtra("bgp.chaosRate", chaosRate)

	if PeerLifecycleCallback != nil {
		coordinator.SetExtra("health.peerCallback", PeerLifecycleCallback)
	}

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
	apiServer.Dispatcher().SetAuditRecorder(auditLog)
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
	var loadBoth func() (map[string]any, *zeconfig.Tree, error)
	if configPath != "" && configPath != "-" {
		readAndParse := func() (*zeconfig.LoadConfigResult, error) {
			var reloadData []byte
			var readErr error
			var hasCandidate bool
			reloadData, _, hasCandidate, readErr = storage.ReadCandidateConfig(store, configPath)
			if readErr == nil && !hasCandidate {
				reloadData, readErr = storage.ReadActiveConfig(store, configPath)
			}
			if readErr != nil {
				reloadData, readErr = os.ReadFile(configPath) //nolint:gosec // daemon operator supplied path
			}
			if readErr != nil {
				return nil, fmt.Errorf("read config: %w", readErr)
			}
			return zeconfig.LoadConfig(string(reloadData), configPath, plugins)
		}
		loadBoth = func() (map[string]any, *zeconfig.Tree, error) {
			result, err := readAndParse()
			if err != nil {
				return nil, nil, err
			}
			return result.Tree.ToMap(), result.Tree, nil
		}
		loadConfigFromDisk = func() (map[string]any, error) {
			m, _, err := loadBoth()
			return m, err
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

	// PPPoE subsystem. ExtractParameters returns defaults when the config
	// tree has no `pppoe {}` block; we only register when the operator
	// configured at least one access interface.
	pppoeParams := pppoe.ExtractParameters(configTree)
	if pppoeParams.Enabled && len(pppoeParams.Interfaces) > 0 {
		if regErr := eng.RegisterSubsystem(pppoe.NewSubsystem(pppoeParams)); regErr != nil {
			fmt.Fprintf(os.Stderr, "error: register pppoe subsystem: %v\n", regErr)
			return 1
		}
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

	// Command dispatchers for user surfaces (use plugin server, not reactor directly).
	webDispatch := serverDispatcherWithSurface(apiServer, audit.Web)
	sshDispatch := serverDispatcherWithSurface(apiServer, audit.SSH)
	mcpDispatch := serverDispatcherWithSurface(apiServer, audit.MCP)
	cliDispatch := serverDispatcherWithSurface(apiServer, audit.CLI)

	// Create shared resolvers for web UI, looking glass, and MCP.
	sc := system.ExtractSystemConfig(loadResult.Tree)
	iface.SetDHCPSystemConfig(sc.ResolvConfPath, len(sc.NameServers) > 0)
	resolvers := newResolvers(&sc)
	defer resolvers.Close()
	resolvecmd.SetResolvers(resolvers)
	if resolvers.DNS != nil {
		command.SetPTRResolver(resolvers.DNS)
	}
	if resolvers.Cymru != nil {
		command.SetOriginResolver(cymruOriginAdapter{resolvers.Cymru})
	}
	if resolvers.DNS != nil {
		resolvecmd.RegisterDNSStatsProvider(func() map[string]any {
			s := resolvers.DNS.CacheStats()
			total := s.Hits + s.Misses
			var hitRate, missRate float64
			if total > 0 {
				hitRate = float64(s.Hits) / float64(total) * 100
				missRate = float64(s.Misses) / float64(total) * 100
			}
			return map[string]any{
				"entries":   s.Entries,
				"capacity":  s.Capacity,
				"hits":      s.Hits,
				"misses":    s.Misses,
				"hit-rate":  hitRate,
				"miss-rate": missRate,
				"evictions": s.Evictions,
				"expired":   s.Expired,
			}
		})
		resolvecmd.RegisterDNSLookupProvider(func(name string, qtype uint16) (*resolvecmd.DNSLookupResult, error) {
			records, ttl, err := resolvers.DNS.ResolveWithTTL(name, qtype)
			if err != nil {
				return nil, err
			}
			return &resolvecmd.DNSLookupResult{Records: records, TTL: ttl}, nil
		})
		resolvecmd.RegisterDNSEntriesProvider(func() []map[string]any {
			entries := resolvers.DNS.CacheEntries()
			out := make([]map[string]any, len(entries))
			for i, e := range entries {
				out[i] = map[string]any{
					"name":        e.Name,
					"type":        e.TypeName,
					"records":     e.Records,
					"ttl-seconds": e.TTLSeconds,
				}
			}
			return out
		})
		resolvecmd.RegisterDNSCacheClearProvider(func(action, name, typeName string) map[string]any {
			switch action {
			case "record":
				if typeName != "" {
					qtype, ok := mdns.StringToType[strings.ToUpper(typeName)]
					if !ok {
						return map[string]any{"action": "delete-entry", "error": "unknown type: " + typeName}
					}
					found := resolvers.DNS.CacheDelete(name, qtype)
					return map[string]any{"action": "delete-entry", "name": name, "type": typeName, "found": found}
				}
				removed := resolvers.DNS.CacheDeleteByName(name)
				return map[string]any{"action": "delete-entry", "name": name, "removed": removed}
			case "stats":
				resolvers.DNS.CacheResetStats()
				return map[string]any{"action": "reset-stats"}
			default:
				resolvers.DNS.CacheClear()
				return map[string]any{"action": "clear-all"}
			}
		})
	}

	if len(sc.NameServers) > 0 {
		if err := system.WriteResolvConf(sc.ResolvConfPath, sc.NameServers); err != nil {
			slogutil.Logger("hub").Warn("resolv.conf write failed", "path", sc.ResolvConfPath, "err", err)
		}
	}

	applyHostTuning(&sc)
	startSmartManager(loadResult.Tree)
	defer stopSmartManager()
	applyConsole(&sc)
	applyConntrack(&sc, apiServer)
	SetIdentityStore(store)
	startUpdateChecker(&sc)
	defer stopBackend()
	startArchiveScheduler(loadResult.Tree, configPath, apiServer)
	defer stopArchiveScheduler()

	lm := NewListenerMigrator(nil)
	var gnmiNotifier *zegnmi.ChangeNotifier
	reloadAfterCommit := func() error {
		startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer startupCancel()
		if err := apiServer.WaitForStartupComplete(startupCtx); err != nil {
			return fmt.Errorf("wait for plugin startup: %w", err)
		}
		if err := doReload(apiServer, eng, configProvider, store, configPath, loadBoth, lm); err != nil {
			return err
		}
		if gnmiNotifier != nil {
			gnmiNotifier.NotifyConfigReload()
		}
		return nil
	}

	var webEditorMgr *zeweb.EditorManager
	if webEnabled && storage.IsBlobStorage(store) {
		if len(webAddrs) == 0 {
			webAddrs = []string{"0.0.0.0:3443"}
		}
		// Config-file users authenticate on the web UI alongside the power user.
		webConfigUsers := bgpconfig.ExtractSSHConfig(loadResult.Tree).Users
		if webSrv, broker, editorMgr := startWebServer(store, configPath, webAddrs, insecureWeb, webDispatch, resolvers, liveAAABundleAuthorizer{}, auditLog, reloadAfterCommit, webConfigUsers); webSrv != nil {
			webEditorMgr = editorMgr
			lm.SetWeb(webSrv)
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
	ephemeralFile := env.Get("ze.ssh.ephemeral")
	if !sshCfg.HasConfig && !hasBGPBlock && ephemeralFile != "" {
		sshCfg = bgpconfig.SSHExtractedConfig{
			Listen:    "127.0.0.1:0",
			HasConfig: true,
		}
	}
	if sshCfg.HasConfig && !hasBGPBlock {
		cfg := zessh.Config{
			Listen:        sshCfg.Listen,
			ListenAddrs:   sshCfg.ListenAddrs,
			HostKeyPath:   sshCfg.HostKeyPath,
			HostCertPath:  sshCfg.HostCertPath,
			IdleTimeout:   sshCfg.IdleTimeout,
			MaxSessions:   sshCfg.MaxSessions,
			Users:         sshCfg.Users,
			AuditRecorder: auditLog,
		}
		if zefsUsers, err := loadZefsUsers(); err == nil {
			cfg.Users = append(zefsUsers, cfg.Users...)
		}

		// Build the AAA bundle via the registry (local + any enabled remote backends).
		// swapAAABundle installs it as the live bundle so closeAAABundle (deferred
		// at the top of runYANGConfig) drains backend workers on process exit.
		aaaLog := slogutil.Logger("hub.aaa")
		aaaBundle, aaaErr := buildAAABundle(loadResult.Tree, cfg.Users, bgpconfig.ExtractAuthzStore(loadResult.Tree), aaaLog)
		if aaaErr != nil {
			aaaLog.Warn("AAA backend build failed; SSH authenticator not set", "error", aaaErr)
			registerAAAAccountingProvider(nil)
		} else {
			registerAAAAccountingProvider(aaaBundle)
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
			}, auditLog))
			// Wire executor factory for non-interactive exec commands
			// (e.g., config edit's "run show traceroute" via SSH exec).
			sshSrv.SetExecutorFactory(func(username, remoteAddr string) zessh.CommandExecutor {
				return func(input string) (string, error) {
					return sshDispatch(input, username, remoteAddr)
				}
			})
			if startErr := sshSrv.Start(context.Background(), nil, nil); startErr != nil {
				slog.Warn("SSH server failed to start", "error", startErr)
			} else {
				slog.Info("SSH server listening", "address", sshSrv.Address())
				if ephemeralFile != "" {
					if writeErr := os.WriteFile(ephemeralFile, []byte(sshSrv.Address()), 0o600); writeErr != nil {
						slog.Warn("failed to write ephemeral SSH address", "error", writeErr)
					}
				}
				defer func() {
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer shutdownCancel()
					_ = sshSrv.Stop(shutdownCtx)
				}()
			}
		}
	}

	if len(lgAddrs) > 0 {
		lgDispatch := func(cmd string) (string, error) { return webDispatch(cmd, "", "") }
		if lgSrv := startLGServer(store, lgAddrs, lgTLS, lgDispatch, resolvers); lgSrv != nil {
			lm.SetLG(lgSrv)
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer shutdownCancel()
				_ = lgSrv.Shutdown(shutdownCtx)
			}()
		}
	}

	var mcpSrv *MCPServerHandle
	if len(mcpAddrs) > 0 {
		mcpStreamCfg := zemcp.StreamableConfig{Token: mcpToken, AuditRecorder: auditLog}
		var mcpTLSCert, mcpTLSKey string
		if mcpCfgOK {
			mcpStreamCfg = mcpConfigToStreamable(mcpCfg, mcpStreamCfg)
			mcpTLSCert = mcpCfg.TLS.Cert
			mcpTLSKey = mcpCfg.TLS.Key
		}
		mcpSrv = startMCPServer(mcpAddrs, mcpDispatch, serverCommandLister(apiServer), mcpStreamCfg, mcpTLSCert, mcpTLSKey)
		if mcpSrv != nil {
			lm.SetMCP(mcpSrv)
		}
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
	apiServer.SetFullReloadFunc(func(context.Context) error {
		return reloadAfterCommit()
	})
	managedCtx, managedCancel := context.WithCancel(context.Background())
	defer managedCancel()
	if managedClient != nil && storage.IsBlobStorage(store) {
		wireManagedCommit(managedClient, store, configPath, reloadAfterCommit, auditLog)
	}
	if webEditorMgr != nil {
		webEditorMgr.SetCommitHook(reloadAfterCommit)
	}
	if apiCfgOK {
		var apiUsers []authz.UserConfig
		if u, uErr := loadZefsUsers(); uErr != nil {
			fmt.Fprintf(os.Stderr, "warning: API power-user auth unavailable: %v\n", uErr)
		} else {
			apiUsers = u
		}
		// Config-file users authenticate alongside the always-on power user.
		apiUsers = mergeAuthUsers(apiUsers, sshCfg.Users)

		// Report active auth mode to make silent degradation visible.
		switch {
		case len(apiUsers) > 0:
			fmt.Fprintf(os.Stderr, "API auth mode: per-user (%d users)\n", len(apiUsers))
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

		var apiErr error
		apiSrvs, apiErr = startAPIServers(apiCfg, apiServer, store, configPath, apiUsers, liveAAABundleAuthorizer{}, reloadAfterCommit, auditLog)
		if apiErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", apiErr)
			apiServer.Stop()
			_ = eng.Stop(startCtx)
			return 1
		}
		if apiSrvs != nil {
			if apiSrvs.rest != nil {
				lm.SetREST(apiSrvs.rest)
			}
			if apiSrvs.grpc != nil {
				lm.SetGRPC(apiSrvs.grpc)
			}
		}
	}

	// Start gNMI server if enabled (env var or YANG config).
	var (
		gnmiSrv     *zegnmi.Server
		gnmiEnabled bool
		gnmiAddr    string
		gnmiToken   string
		gnmiTLSCert string
		gnmiTLSKey  string
	)
	if env.IsEnabled("ze.gnmi.enabled") {
		gnmiEnabled = true
	}
	gnmiAddr = env.Get("ze.gnmi.listen")
	gnmiToken = env.Get("ze.gnmi.token")
	gnmiTLSCert = env.Get("ze.gnmi.tls.cert")
	gnmiTLSKey = env.Get("ze.gnmi.tls.key")

	if gnmiYANG, ok := zeconfig.ExtractGNMIConfig(loadResult.Tree); ok {
		gnmiEnabled = true
		if gnmiAddr == "" {
			gnmiAddr = endpointsToAddrs(gnmiYANG.Servers)[0]
			if len(gnmiYANG.Servers) > 1 {
				slog.Warn("gNMI: only first server listener is used, extra listeners ignored", "configured", len(gnmiYANG.Servers))
			}
		}
		if gnmiToken == "" {
			gnmiToken = gnmiYANG.Token
		}
		if gnmiTLSCert == "" {
			gnmiTLSCert = gnmiYANG.TLS.Cert
		}
		if gnmiTLSKey == "" {
			gnmiTLSKey = gnmiYANG.TLS.Key
		}
	}

	if gnmiEnabled {
		if gnmiAddr == "" {
			gnmiAddr = "0.0.0.0:9339"
		}
		gnmiCfg := zegnmi.Config{
			ListenAddr: gnmiAddr,
			Token:      gnmiToken,
		}
		if gnmiTLSCert != "" {
			var err error
			if gnmiCfg.CertPEM, err = os.ReadFile(gnmiTLSCert); err != nil { //nolint:gosec // operator-configured cert path
				fmt.Fprintf(os.Stderr, "warning: gNMI TLS cert: %v\n", err)
			}
		}
		if gnmiTLSKey != "" {
			var err error
			if gnmiCfg.KeyPEM, err = os.ReadFile(gnmiTLSKey); err != nil { //nolint:gosec // operator-configured key path
				fmt.Fprintf(os.Stderr, "warning: gNMI TLS key: %v\n", err)
			}
		}
		gnmiCtx, gnmiCancel := context.WithCancel(context.Background())
		gnmiSessions := buildGNMISessionManager(store, configPath, reloadAfterCommit)
		go gnmiSessions.RunCleanup(gnmiCtx)
		gnmiNotifier = zegnmi.NewChangeNotifier()
		gnmiSrv = zegnmi.NewServer(gnmiCfg, func() *zeconfig.Tree { return loadResult.Tree }, gnmiSessions, yangloader.DefaultLoader, gnmiNotifier)
		if reg, ok := registry.GetMetricsRegistry().(metrics.Registry); ok {
			gnmiSrv.SetMetricsRegistry(reg)
		}
		zegnmi.RegisterGlobal(gnmiSrv)
		defer zegnmi.RegisterGlobal(nil)
		// Component startup goroutine (one-time, same pattern as startWebServer).
		go serveGNMI(gnmiCtx, gnmiSrv)
		defer gnmiCancel()
		readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		gnmiReady := waitForGNMIBind(readyCtx, gnmiSrv)
		readyCancel()
		if gnmiReady {
			fmt.Fprintf(os.Stderr, "gNMI server listening on %s\n", gnmiSrv.Address())
		} else {
			fmt.Fprintf(os.Stderr, "warning: gNMI server failed to bind on %s\n", gnmiAddr)
		}
	}

	// Signal handling: SIGINT/SIGTERM for shutdown, SIGHUP for config reload.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// SIGHUP reload worker: re-reads config from disk, auto-loads/stops plugins,
	// refreshes the shared ConfigProvider, then notifies every registered
	// subsystem so it can hot-apply diff-able knobs.
	reloadCh := make(chan os.Signal, 1)
	go handleSIGHUPReload(reloadCh, apiServer, eng, configProvider, store, configPath, loadBoth, lm, auditLog)

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
	if managedClient != nil && storage.IsBlobStorage(store) {
		go managed.RunManagedClient(managedCtx, *managedClient)
	}

	if cliAttach {
		fmt.Println("Ze running. Type 'exit' or Ctrl+D to detach CLI (daemon keeps running).")
	} else {
		fmt.Println("Ze running. Press Ctrl+C to stop.")
	}

	if cliAttach {
		zecli.RunAttached(func(command string) (string, error) {
			return cliDispatch(command, "root", "local")
		})
		fmt.Println("CLI detached. Press Ctrl+C to stop daemon.")
	}

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

	if gnmiSrv != nil {
		gnmiSrv.Stop()
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

func (st *standaloneTelemetry) Close() {
	if st.manager != nil {
		st.manager.Stop()
	}
	_ = st.srv.Close()
}
