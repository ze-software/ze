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
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	"codeberg.org/thomas-mangin/ze/internal/component/hub"
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
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/privilege"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Env var registrations are centralized in internal/component/config/environment.go.
// No duplicate registrations here -- import that package to trigger init.

// RunWebOnly starts only the web server (no BGP engine).
// Used when ze start --web is called without a config.
// listenAddr overrides the default "0.0.0.0:3443" when non-empty.
func RunWebOnly(store storage.Storage, listenAddr string, insecureWeb bool) int {
	resolvers := newResolvers()
	defer resolvers.Close()

	webSrv, broker := startWebServer(store, listenAddr, insecureWeb, nil, resolvers)
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = webSrv.Shutdown(shutdownCtx)
	broker.Close()

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
		// Run hub orchestrator using hub parser
		// TODO: pass plugins to orchestrator when hub mode supports them
		_ = plugins // Currently unused in hub mode
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
	// Phase 1: Parse config and resolve plugins.
	loadResult, err := zeconfig.LoadConfig(string(data), configPath, plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	configPaths := zeconfig.CollectContainerPaths(loadResult.Tree)

	// Resolve web/LG/MCP config: env > config file > CLI defaults.
	var lgListenAddr string
	var lgTLS bool
	if listen := env.Get("ze.looking-glass.listen"); listen != "" {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.looking-glass.listen: %v\n", parseErr)
			return 1
		}
		lgListenAddr = endpoints[0].String()
		if len(endpoints) > 1 {
			fmt.Fprintf(os.Stderr, "warning: ze.looking-glass.listen: only first endpoint used, multi-bind not yet supported\n")
		}
	}
	if env.IsEnabled("ze.looking-glass.tls") {
		lgTLS = true
	}
	if env.IsEnabled("ze.looking-glass.enabled") && lgListenAddr == "" {
		lgListenAddr = "0.0.0.0:8443"
	}

	if listen := env.Get("ze.web.listen"); listen != "" && webListenAddr == "" {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.web.listen: %v\n", parseErr)
			return 1
		}
		webEnabled = true
		webListenAddr = endpoints[0].String()
		if len(endpoints) > 1 {
			fmt.Fprintf(os.Stderr, "warning: ze.web.listen: only first endpoint used, multi-bind not yet supported\n")
		}
	}
	if env.IsEnabled("ze.web.enabled") && !webEnabled {
		webEnabled = true
	}
	if env.IsEnabled("ze.web.insecure") && !insecureWeb {
		insecureWeb = true
	}
	if listen := env.Get("ze.mcp.listen"); listen != "" && mcpAddr == "" {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.mcp.listen: %v\n", parseErr)
			return 1
		}
		mcpAddr = endpoints[0].String()
		if len(endpoints) > 1 {
			fmt.Fprintf(os.Stderr, "warning: ze.mcp.listen: only first endpoint used, multi-bind not yet supported\n")
		}
	}
	if env.IsEnabled("ze.mcp.enabled") && mcpAddr == "" {
		mcpAddr = "127.0.0.1:8080"
	}
	if token := env.Get("ze.mcp.token"); token != "" && mcpToken == "" {
		mcpToken = token
	}

	if webCfg, ok := zeconfig.ExtractWebConfig(loadResult.Tree); ok {
		if !webEnabled {
			webEnabled = true
			webListenAddr = webCfg.Listen()
			insecureWeb = webCfg.Insecure
		}
	}
	if mcpCfg, ok := zeconfig.ExtractMCPConfig(loadResult.Tree); ok {
		if mcpAddr == "" {
			mcpAddr = mcpCfg.Listen()
		}
		if mcpToken == "" && mcpCfg.Token != "" {
			mcpToken = mcpCfg.Token
		}
	}
	if lgCfg, ok := zeconfig.ExtractLGConfig(loadResult.Tree); ok {
		if lgListenAddr == "" {
			lgListenAddr = lgCfg.Listen()
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
	if configPath != "" && configPath != "-" {
		apiServer.SetConfigLoader(func() (map[string]any, error) {
			reloadData, readErr := store.ReadFile(configPath)
			if readErr != nil {
				return nil, fmt.Errorf("read config: %w", readErr)
			}
			result, loadErr := zeconfig.LoadConfig(string(reloadData), configPath, plugins)
			if loadErr != nil {
				return nil, loadErr
			}
			return result.Tree.ToMap(), nil
		})
	}

	// apiServer implements ze.EventBus via its Emit/Subscribe methods, so the
	// engine, plugins, and subsystems all share one namespaced pub/sub
	// backbone. The standalone bus in internal/component/bus/ is gone.
	eng := engine.NewEngine(apiServer, configProvider, pm)

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

	if err := dropPrivileges(); err != nil {
		fmt.Fprintf(os.Stderr, "error: drop privileges: %v\n", err)
		apiServer.Stop()
		_ = eng.Stop(startCtx)
		return 1
	}

	// Command dispatcher for web/LG/MCP (uses plugin server, not reactor directly).
	dispatch := serverDispatcher(apiServer)

	// Create shared resolvers for web UI, looking glass, and MCP.
	resolvers := newResolvers()
	defer resolvers.Close()
	resolvecmd.SetResolvers(resolvers)

	if webEnabled {
		if webSrv, broker := startWebServer(store, webListenAddr, insecureWeb, dispatch, resolvers); webSrv != nil {
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer shutdownCancel()
				_ = webSrv.Shutdown(shutdownCtx)
				broker.Close()
			}()
		}
	}

	if lgListenAddr != "" {
		if lgSrv := startLGServer(store, lgListenAddr, lgTLS, dispatch, resolvers); lgSrv != nil {
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer shutdownCancel()
				_ = lgSrv.Shutdown(shutdownCtx)
			}()
		}
	}

	var mcpSrv *http.Server
	if mcpAddr != "" {
		mcpSrv = startMCPServer(mcpAddr, dispatch, serverCommandLister(apiServer), mcpToken)
	}

	// Start REST/gRPC API servers if configured (env > config file).
	var apiSrvs *apiServers
	apiCfg, apiCfgOK := zeconfig.ExtractAPIConfig(loadResult.Tree)
	if env.IsEnabled("ze.api-server.rest.enabled") && !apiCfg.RESTOn {
		apiCfg.RESTOn = true
		apiCfgOK = true
	}
	if listen := env.Get("ze.api-server.rest.listen"); listen != "" && apiCfg.RESTOn {
		host, port, parseErr := net.SplitHostPort(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.api.rest.listen: %v\n", parseErr)
			return 1
		}
		apiCfg.REST.Host = host
		apiCfg.REST.Port = port
	}
	if env.IsEnabled("ze.api-server.grpc.enabled") && !apiCfg.GRPCOn {
		apiCfg.GRPCOn = true
		apiCfgOK = true
	}
	if listen := env.Get("ze.api-server.grpc.listen"); listen != "" && apiCfg.GRPCOn {
		host, port, parseErr := net.SplitHostPort(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.api.grpc.listen: %v\n", parseErr)
			return 1
		}
		apiCfg.GRPC.Host = host
		apiCfg.GRPC.Port = port
	}
	if token := env.Get("ze.api-server.token"); token != "" && apiCfg.Token == "" {
		apiCfg.Token = token
	}
	if apiCfgOK {
		apiSrvs = startAPIServers(apiCfg, apiServer)
	}

	// Signal handling: SIGINT/SIGTERM for shutdown, SIGHUP for config reload.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// SIGHUP reload worker: re-reads config from disk, auto-loads/stops plugins.
	reloadCh := make(chan os.Signal, 1)
	go handleSIGHUPReload(reloadCh, apiServer)

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
	return 0
}

// waitForServerDone blocks until the plugin server's Wait returns, then closes doneCh.
// Lifecycle goroutine (one-time, not hot path): bridges Server.Wait to a select channel.
func waitForServerDone(s *pluginserver.Server, doneCh chan struct{}) {
	_ = s.Wait(context.Background())
	close(doneCh)
}

// handleSIGHUPReload is the SIGHUP reload worker. Reads signals from reloadCh,
// triggers config reload via the plugin server's ReloadFromDisk.
// If a transaction is in progress (lock held), the SIGHUP is queued and replayed
// after the current reload completes.
// Lifecycle goroutine (one-time, runs for daemon lifetime).
func handleSIGHUPReload(reloadCh <-chan os.Signal, s *pluginserver.Server) {
	for range reloadCh {
		fmt.Fprintf(os.Stderr, "received SIGHUP, reloading config...\n")
		if err := doReload(s); err != nil {
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
			if err := doReload(s); err != nil {
				fmt.Fprintf(os.Stderr, "queued reload error: %v\n", err)
			}
		}
	}
}

// doReload performs a single config reload with a 30-second timeout.
func doReload(s *pluginserver.Server) error {
	reloadCtx, reloadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reloadCancel()
	return s.ReloadFromDisk(reloadCtx)
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

// serverDispatcher creates a CommandDispatcher from the plugin server's dispatcher.
func serverDispatcher(s *pluginserver.Server) func(string) (string, error) {
	return func(input string) (string, error) {
		d := s.Dispatcher()
		if d == nil {
			return "", fmt.Errorf("server not ready")
		}
		ctx := &pluginserver.CommandContext{Server: s}
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

// startWebServer creates and starts the web server with zefs credentials.
// Returns the server and SSE event broker on success, nil on failure (logged, non-fatal).
// Caller MUST call broker.Close() during shutdown to release SSE clients.
// If the port is already in use, attempts to identify and kill the stale process.
// Requires blob storage -- TLS keys and config must not leak to the filesystem.
func startWebServer(store storage.Storage, listenAddr string, insecureWeb bool, dispatch zeweb.CommandDispatcher, resolvers *resolve.Resolvers) (*zeweb.WebServer, *zeweb.EventBroker) {
	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: requires blob storage (run ze init first)\n")
		return nil, nil
	}

	if listenAddr == "" {
		listenAddr = "0.0.0.0:3443"
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
	certStore := &blobCertStore{store: store}
	certPEM, keyPEM, err := zeweb.LoadOrGenerateCert(certStore, listenAddr)
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
		ListenAddr: listenAddr,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
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
	tree := zeconfig.NewTree()

	// Ensure a config file exists for the editor.
	configPath := resolveConfigPath(store)
	if !store.Exists(configPath) {
		if writeErr := store.WriteFile(configPath, []byte("# ze config\n"), 0o600); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot create config: %v\n", writeErr)
		}
	}

	// Create editor manager for config editing via web.
	editorMgr := zeweb.NewEditorManager(store, configPath, schema, newEditorFactory(), newEditSessionFactory())

	// Create CLI completer for Tab/? autocomplete.
	completer := cli.NewCompleter()

	sessionStore := zeweb.NewSessionStore()
	loginRenderer := func(w http.ResponseWriter, _ *http.Request) {
		if renderErr := renderer.RenderLogin(w, zeweb.LoginData{}); renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	}

	// Fragment handler serves HTMX components for YANG tree navigation.
	fragmentHandler := zeweb.HandleFragment(renderer, schema, tree, editorMgr, insecureWeb)

	// Config set, add, and delete handlers for editing leaf values.
	setHandler := zeweb.HandleConfigSet(editorMgr, schema, renderer)
	addHandler := zeweb.HandleConfigAdd(editorMgr, schema, renderer)
	addFormHandler := zeweb.HandleConfigAddForm(editorMgr, schema, renderer)
	deleteHandler := zeweb.HandleConfigDelete(editorMgr)

	// SSE broker for live config change notifications.
	broker := zeweb.NewEventBroker(0)

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
	terminalHandler := zeweb.HandleCLITerminal(editorMgr)
	modeHandler := zeweb.HandleCLIModeToggle(editorMgr, schema, renderer)

	// Auth wrapper for protecting individual routes.
	var authWrap func(http.Handler) http.Handler
	if insecureWeb {
		authWrap = zeweb.InsecureMiddleware
	} else {
		authWrap = func(h http.Handler) http.Handler {
			return zeweb.AuthMiddleware(sessionStore, users, loginRenderer, h)
		}
	}

	loginHandler := zeweb.LoginHandler(sessionStore, users, loginRenderer)
	assetHandler := http.StripPrefix("/assets/", renderer.AssetHandler())

	// Admin command tree for web UI.
	adminChildren := zeweb.BuildAdminCommandTree()
	adminViewHandler := zeweb.HandleAdminView(renderer, adminChildren)
	adminExecHandler := zeweb.HandleAdminExecute(renderer, dispatch)

	srv.HandleFunc("POST /login", loginHandler)
	srv.Handle("/assets/", assetHandler)
	srv.Handle("/events", authWrap(broker))
	srv.Handle("/admin/", authWrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			adminExecHandler(w, r)
			return
		}
		adminViewHandler(w, r)
	})))
	srv.Handle("POST /cli", authWrap(cliHandler))
	srv.Handle("/cli/complete", authWrap(completeHandler))
	srv.Handle("POST /cli/terminal", authWrap(terminalHandler))
	srv.Handle("POST /cli/mode", authWrap(modeHandler))
	srv.Handle("/fragment/detail", authWrap(fragmentHandler))
	srv.Handle("POST /config/set/", authWrap(setHandler))
	srv.Handle("POST /config/add/", authWrap(addHandler))
	srv.Handle("GET /config/add-form/", authWrap(addFormHandler))
	srv.Handle("GET /config/changes", authWrap(zeweb.HandleConfigChanges(editorMgr, renderer)))
	srv.Handle("POST /config/delete/", authWrap(deleteHandler))
	srv.Handle("/config/diff", authWrap(diffHandler))
	srv.Handle("/config/diff-close", authWrap(diffCloseHandler))
	srv.Handle("/config/commit", authWrap(commitHandler))
	srv.Handle("POST /config/discard", authWrap(discardHandler))
	srv.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)
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
	dir := paths.DefaultConfigDir()
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
func startLGServer(store storage.Storage, listenAddr string, useTLS bool, dispatch lg.CommandDispatcher, resolvers *resolve.Resolvers) *lg.LGServer {
	cfg := lg.LGConfig{
		ListenAddr: listenAddr,
		TLS:        useTLS,
		Dispatch:   dispatch,
		DecorateASN: func(asn string) string {
			if resolvers == nil || resolvers.Cymru == nil {
				return ""
			}
			name, _ := resolvers.Cymru.LookupASNName(context.Background(), parseASNForDecorator(asn))
			return name
		},
	}

	// When TLS is enabled, load or generate cert from blob storage.
	if useTLS {
		if !storage.IsBlobStorage(store) {
			fmt.Fprintf(os.Stderr, "error: looking glass TLS requires blob storage (run ze init first)\n")
			return nil
		}
		certStore := &blobCertStore{store: store}
		certPEM, keyPEM, err := zeweb.LoadOrGenerateCert(certStore, listenAddr)
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
	fmt.Fprintf(os.Stderr, "looking glass listening on %s://%s/\n", scheme, srv.Address())
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
func newResolvers() *resolve.Resolvers {
	dnsResolver := resolveDNS.NewResolver(resolveDNS.ResolverConfig{})

	// Wrap DNS ResolveTXT to match Cymru's TXTResolver signature (adds context).
	txtResolver := func(_ context.Context, name string) ([]string, error) {
		return dnsResolver.ResolveTXT(name)
	}

	return &resolve.Resolvers{
		DNS:   dnsResolver,
		Cymru: cymru.New(txtResolver, nil),
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
