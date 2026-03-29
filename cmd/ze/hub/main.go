// Design: docs/architecture/hub-architecture.md -- hub CLI entry point
// Detail: mcp.go -- MCP server startup
//
// Package hub provides the ze hub subcommand.
package hub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/grmarker"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/subsystem"
	"codeberg.org/thomas-mangin/ze/internal/component/bus"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	"codeberg.org/thomas-mangin/ze/internal/component/hub"
	pluginmgr "codeberg.org/thomas-mangin/ze/internal/component/plugin/manager"
	"codeberg.org/thomas-mangin/ze/internal/component/ssh"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/privilege"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Env var registrations.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ready.file", Type: "string", Description: "Write signal file when hub is ready (test infrastructure)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.web.host", Type: "string", Description: "Web server listen host"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.web.port", Type: "string", Description: "Web server listen port"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.web.insecure", Type: "bool", Description: "Disable web authentication"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.mcp.host", Type: "string", Description: "MCP server listen host (127.0.0.1 only)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.mcp.port", Type: "string", Description: "MCP server listen port"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.server", Type: "string", Default: defaultDNSServer(), Description: "DNS server address (e.g., 8.8.8.8:53)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.timeout", Type: "int", Default: "5", Description: "DNS query timeout in seconds (1-60)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.cache-size", Type: "int", Default: "10000", Description: "DNS cache max entries (0 = disabled)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.cache-ttl", Type: "int", Default: "86400", Description: "DNS cache max TTL in seconds (0 = response TTL only)"})
)

// RunWebOnly starts only the web server (no BGP engine).
// Used when ze start --web is called without a config.
// listenAddr overrides the default "0.0.0.0:8443" when non-empty.
func RunWebOnly(store storage.Storage, listenAddr string, insecureWeb bool) int {
	webSrv := startWebServer(store, listenAddr, insecureWeb, nil)
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
func Run(store storage.Storage, configPath string, plugins []string, chaosSeed int64, chaosRate float64, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr string) int {
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
	case zeconfig.ConfigTypeBGP:
		// Run BGP in-process using YANG parser
		return runBGPInProcess(store, configPath, data, plugins, chaosSeed, chaosRate, stdinOpen, webEnabled, webListenAddr, insecureWeb, mcpAddr)
	case zeconfig.ConfigTypeHub:
		// Run hub orchestrator using hub parser
		// TODO: pass plugins to orchestrator when hub mode supports them
		_ = plugins // Currently unused in hub mode
		return runOrchestratorWithData(store, configPath, data)
	case zeconfig.ConfigTypeUnknown:
		fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
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

// runBGPInProcess loads BGP config using YANG parser and runs reactor in-process.
// When stdinOpen is true, a background goroutine monitors stdin for EOF and
// triggers shutdown when the upstream process exits (pipe mode).
func runBGPInProcess(store storage.Storage, configPath string, data []byte, plugins []string, chaosSeed int64, chaosRate float64, stdinOpen, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr string) int {
	// Phase 1: Parse config and resolve plugins (no reactor created yet).
	loadResult, err := bgpconfig.LoadConfig(string(data), configPath, plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	// Precedence: CLI > env > config.
	// Config provides defaults, env overrides config, CLI overrides everything.

	// Layer 1: config file values.
	if webCfg, ok := bgpconfig.ExtractWebConfig(loadResult.Tree); ok {
		if !webEnabled {
			webEnabled = true
			webListenAddr = webCfg.Listen()
			insecureWeb = webCfg.Insecure
		}
	}
	if mcpCfg, ok := bgpconfig.ExtractMCPConfig(loadResult.Tree); ok {
		if mcpAddr == "" {
			mcpAddr = mcpCfg.Listen()
		}
	}

	// Layer 2: environment variables override config (but not CLI).
	if h, p := env.Get("ze.web.host"), env.Get("ze.web.port"); p != "" && webListenAddr == "" {
		webEnabled = true
		host := h
		if host == "" {
			host = "0.0.0.0"
		}
		webListenAddr = host + ":" + p
	}
	if env.IsEnabled("ze.web.insecure") && !insecureWeb {
		insecureWeb = true
	}
	if h, p := env.Get("ze.mcp.host"), env.Get("ze.mcp.port"); p != "" && mcpAddr == "" {
		host := h
		if host == "" {
			host = "127.0.0.1"
		}
		mcpAddr = host + ":" + p
	}
	// Layer 3: CLI flags already applied (they were set before this point).

	// Phase 2: Populate ConfigProvider with parsed config tree.
	configProvider := zeconfig.NewProvider()
	for root, subtree := range loadResult.Tree.ToMap() {
		if sub, ok := subtree.(map[string]any); ok {
			configProvider.SetRoot(root, sub)
		} else {
			fmt.Fprintf(os.Stderr, "warning: config root %q has non-map value, skipping\n", root)
		}
	}

	// Phase 3: Create reactor from parsed config.
	reactor, err := bgpconfig.CreateReactor(loadResult, configPath, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create reactor: %v\n", err)
		return 1
	}

	// Inject chaos wrappers if chaos mode is enabled.
	// CLI flags override env vars/config; seed=0 means disabled, -1 means time-based.
	if chaosSeed != 0 {
		chaosSeed = chaos.ResolveSeed(chaosSeed)
		if chaosRate < 0 {
			chaosRate = 0.1 // Default rate when not specified by CLI
		}
		logger := slogutil.Logger("chaos")
		cfg := chaos.ChaosConfig{
			Seed:   chaosSeed,
			Rate:   chaosRate,
			Logger: logger,
		}
		clock, dialer, listenerFactory := chaos.NewChaosWrappers(
			clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, cfg,
		)
		reactor.SetClock(clock)
		reactor.SetDialer(dialer)
		reactor.SetListenerFactory(listenerFactory)
		logger.Info("chaos self-test mode enabled",
			"seed", chaosSeed,
			"rate", chaosRate,
		)
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Monitor stdin for EOF when running in pipe mode (ze-chaos | ze -).
	// After reading config (delimited by NUL), stdin stays open. When the
	// upstream process exits, the pipe closes and this goroutine triggers
	// clean shutdown — no Ctrl-C needed.
	if stdinOpen {
		go monitorStdinEOF(sigCh)
	}

	// RFC 4724 Section 4.1: Read GR marker from storage. If valid (not expired),
	// set RestartUntil so OPEN messages include R=1 during the restart window.
	// Marker is consumed (removed) after reading to prevent stale restart on next cold start.
	if expiry, ok := grmarker.Read(store); ok {
		reactor.SetRestartUntil(expiry)
		slogutil.Logger("bgp.gr").Info("GR restart marker found", "expires", expiry)
	}
	if removeErr := grmarker.Remove(store); removeErr != nil {
		slogutil.Logger("bgp.gr").Warn("failed to remove GR marker", "error", removeErr)
	}

	fmt.Printf("Starting ze BGP with config: %s\n", configPath)

	// Create Bus and Engine. Wire reactor as a Subsystem via adapter.
	// The Bus is a notification layer for cross-component signaling.
	// Plugin data delivery stays on the existing EventDispatcher direct path.
	b := bus.NewBus()
	reactor.SetBus(b)
	pm := pluginmgr.NewManager()
	reactor.SetProcessSpawner(pm)
	bgpSub := subsystem.NewBGPSubsystem(reactor)
	eng := engine.NewEngine(b, configProvider, pm)
	if err := eng.RegisterSubsystem(bgpSub); err != nil {
		fmt.Fprintf(os.Stderr, "error: register subsystem: %v\n", err)
		return 1
	}

	startCtx := context.Background()
	if err := eng.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "error starting engine: %v\n", err)
		return 1
	}

	// Drop privileges after port binding (while still root).
	// All subsequent work (plugins, connections) runs as the configured user.
	if err := dropPrivileges(); err != nil {
		fmt.Fprintf(os.Stderr, "error: drop privileges: %v\n", err)
		_ = eng.Stop(startCtx)
		return 1
	}

	// Start web server if --web flag was passed.
	if webEnabled {
		if webSrv := startWebServer(store, webListenAddr, insecureWeb, reactor.ExecuteCommand); webSrv != nil {
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer shutdownCancel()
				_ = webSrv.Shutdown(shutdownCtx)
			}()
		}
	}

	// Start MCP server if --mcp flag was passed.
	var mcpSrv *http.Server
	if mcpAddr != "" {
		mcpSrv = startMCPServer(mcpAddr, reactor.ExecuteCommand)
	}

	// Wait for either signal or reactor to stop itself
	doneCh := make(chan struct{})
	go func() {
		_ = reactor.Wait(context.Background())
		close(doneCh)
	}()

	fmt.Println("Ze BGP running. Press Ctrl+C to stop.")

	// Signal readiness to test infrastructure. Written after signal.Notify
	// and reactor.Start so the test runner knows the daemon is fully operational.
	if readyFile := env.Get("ze.ready.file"); readyFile != "" {
		if f, err := os.Create(readyFile); err == nil { //nolint:gosec // test infrastructure path from env
			f.Close() //nolint:errcheck,gosec // best-effort readiness signal
		}
	}

	select {
	case <-sigCh:
		fmt.Println("\nShutting down...")
	case <-doneCh:
		fmt.Println("\nShutting down...")
	}

	// Shut down MCP before reactor to prevent requests during teardown.
	if mcpSrv != nil {
		mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = mcpSrv.Shutdown(mcpCtx)
		mcpCancel()
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := eng.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: shutdown timeout: %v\n", err)
	}
	b.Stop()

	fmt.Println("Ze BGP stopped.")
	return 0
}

// startWebServer creates and starts the web server with zefs credentials.
// Returns the server on success, nil on failure (logged, non-fatal).
// If the port is already in use, attempts to identify and kill the stale process.
// Requires blob storage -- TLS keys and config must not leak to the filesystem.
func startWebServer(store storage.Storage, listenAddr string, insecureWeb bool, dispatch zeweb.CommandDispatcher) *zeweb.WebServer {
	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: requires blob storage (run ze init first)\n")
		return nil
	}

	if listenAddr == "" {
		listenAddr = "0.0.0.0:8443"
	}

	var users []ssh.UserConfig
	if !insecureWeb {
		var err error
		users, err = loadZefsUsers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: %v\n", err)
			return nil
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: authentication disabled (--insecure-web)\n")
	}

	// Persist TLS cert in zefs so browsers don't have to re-accept on every restart.
	certStore := &blobCertStore{store: store}
	certPEM, keyPEM, err := zeweb.LoadOrGenerateCert(certStore, listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: TLS cert: %v\n", err)
		return nil
	}

	renderer, err := zeweb.NewRenderer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: renderer: %v\n", err)
		return nil
	}

	srv, err := zeweb.NewWebServer(zeweb.WebConfig{
		ListenAddr: listenAddr,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: %v\n", err)
		return nil
	}

	// Load YANG schema for config tree navigation.
	schema, schemaErr := zeconfig.YANGSchema()
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: YANG schema: %v\n", schemaErr)
		return nil
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
	editorMgr := zeweb.NewEditorManager(store, configPath, schema)

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
	defer broker.Close()

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
		return nil
	}

	fmt.Fprintf(os.Stderr, "web server listening on https://%s/\n", srv.Address())
	return srv
}

// loadZefsUsers reads credentials from the zefs database (created by ze init).
func loadZefsUsers() ([]ssh.UserConfig, error) {
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
	return []ssh.UserConfig{{Name: name, Hash: string(hash)}}, nil
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

// defaultDNSServer reads /etc/resolv.conf and returns the first nameserver,
// or "8.8.8.8:53" if unavailable. Used at init time for env var default display.
func defaultDNSServer() string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return "8.8.8.8:53"
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			return fields[1] + ":53"
		}
	}
	return "8.8.8.8:53"
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
