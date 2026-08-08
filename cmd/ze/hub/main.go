// Design: docs/architecture/hub-architecture.md -- hub CLI entry point
// Detail: service_mcp.go -- MCP server factory (//go:build ze_mcp)
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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/authz"
	zecli "github.com/ze-software/ze/internal/component/cli/client"
	showCmd "github.com/ze-software/ze/internal/component/cmd/show"
	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/component/hub"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/managed"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	resolvecmd "github.com/ze-software/ze/internal/component/resolve/cmd"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/reboot"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"
)

var (
	errCannotResolveConfigDirectory = errors.New("cannot resolve config directory")
	errEmptyUsernameInZefs          = errors.New("empty username in zefs")
	errEmptyPasswordInZefs          = errors.New("empty password hash in zefs")
	errAdminDisabledInZefs          = errors.New("local admin login disabled in zefs")
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

// defaultWebListen is the address the web server binds when none is configured.
// It is NOT loopback, so it must be resolved before the management-listener
// guard runs -- see resolveWebListeners.
const defaultWebListen = "0.0.0.0:3443"

// resolveWebListeners returns the addresses the web server will actually bind.
// An enabled web server with no configured address falls back to
// defaultWebListen; the web service builder applies the same fallback, so the
// two must not diverge (both call this).
//
// This exists so the fallback happens BEFORE checkMgmtListeners rather than
// inside the builder afterwards. Web is the only management surface that binds
// a default when its address list is empty (MCP, LG and REST all skip), so it
// was the only one whose guard declaration could be an empty slice while the
// process went on to bind 0.0.0.0 -- an unauthenticated listener the guard
// iterated zero times and therefore never refused.
func resolveWebListeners(webEnabled bool, addrs []string) []string {
	if !webEnabled || len(addrs) > 0 {
		return addrs
	}
	return []string{defaultWebListen}
}

// RunWebOnly starts only the web server (no BGP engine).
// Used when ze start --web is called without a config.
// listenAddr overrides the default "0.0.0.0:3443" when non-empty.
func RunWebOnly(store storage.Storage, listenAddr string, insecureWeb bool) int {
	if webBuildStandalone == nil {
		return webNotCompiledIn()
	}
	return webBuildStandalone(store, listenAddr, insecureWeb)
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
	// Register the daemon's blob store so in-core state persisters
	// (internal/core/statestore) write into the SAME in-memory tree as config,
	// not a separate transient instance whose keys the config store's next flush
	// would drop.
	if bs, ok := storage.BlobStoreFrom(store); ok {
		statestore.SetStore(bs)
	} else if env.Get("ze.config.dir") != "" {
		// Config is on a filesystem backend (the `ze -` / `ze <file>` CLI path,
		// storage.NewFilesystem) but the operator pinned a config dir: open a
		// SEPARATE state-only zefs store at {config-dir}/database.zefs so runtime
		// state (BFD auth sequence, DDoS baselines, NTP last-time, ...) still
		// persists across restarts in dev, not only on the appliance. There is no
		// shared database.zefs with config on this path, so the lost-update BLOCKER
		// the shared-handle design guards against cannot arise.
		//
		// Gated on an EXPLICIT ze.config.dir: without it the default is derived
		// from the binary location (paths.ConfigDirFromBinary), which is shared by
		// every `ze` invocation -- so a one-off `ze -` (and the whole functional
		// test suite) must not create or contend on a database.zefs there. When no
		// config dir is pinned, statestore stays a best-effort no-op, exactly as
		// the pre-migration loose-file path was non-fatal on a read-only disk.
		//
		// The store is opened DIRECTLY (zefs.Open/Create), NOT through
		// internalresolve.Storage(): that helper answers "where does my CONFIG
		// live" and returns a filesystem backend whenever ze.storage.blob=false,
		// which silently defeated this branch and dropped ALL runtime state --
		// including the tc original-qdisc snapshot, whose absence makes the
		// traffic backend refuse to program a qdisc at all
		// (internal/plugins/traffic/netlink/backend_linux.go errSnapshotPersistUnavailable).
		// ze.storage.blob selects the CONFIG backend; it must not decide whether
		// runtime state survives a restart. Opening directly also skips
		// storage.NewBlob's config migration, so this store stays state-only and
		// never shadows the on-disk config a SIGHUP reload re-reads.
		if bs, err := openStateOnlyStore(env.Get("ze.config.dir")); err != nil {
			fmt.Fprintf(os.Stderr, "warning: runtime state persistence unavailable: %v\n", err)
		} else {
			statestore.SetStore(bs)
		}
	}

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
		logStartupFailure("read config", err)
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
			logStartupFailure("flag validation", errors.New("--plugin is not supported with hub/orchestrator configs"))
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
// errUnauthenticatedMgmtListener summarizes the mgmt-listener refusal for the
// slog mirror; the per-listener detail is on stderr from checkMgmtListeners.
var errUnauthenticatedMgmtListener = errors.New("refusing unauthenticated non-loopback management listener (detail on stderr)")

// logStartupFailure mirrors a fatal pre-serve failure onto the slog backend.
// The stderr print beside each call reaches interactive operators, but on the
// gokrazy appliance stderr is captured by the supervisor and never reaches
// the serial console; the slog backend there is kmsg, which does. A daemon
// that dies before "web server listening" must say why on a channel the
// operator can see, or it crash-loops undiagnosably from the console (this
// hid the L2TP kernel-probe refusal on the pinned no-l2tp kernel).
func logStartupFailure(stage string, err error) {
	slogutil.Logger("hub").Error("startup failed", "stage", stage, "err", err)
}

// bootPowerUsers reads the zefs break-glass accounts once, and says so when it
// cannot. The read is not fatal: config users can still authenticate.
//
// Two outcomes arrive here with no power user and they are not the same event.
// errAdminDisabledInZefs is the operator's own declaration, written when the
// appliance was built (internal/appliance/cmd_assemble.go), so it is SILENT:
// a console that prints a repair instruction on every boot of such a box tells
// the operator to undo what they asked for. Every other error is a database
// this daemon expected to read and could not, and that one stays audible.
//
// The diagnostic is at WARN because this is the only place it is produced. The
// web factory used to print its own on stderr when it read zefs for itself;
// that second read is gone (one producer), so a level the default logger drops
// makes an unreadable database completely silent on a daemon that never builds
// a web server. What is lost is the break-glass account: the operator finds out
// at the login prompt, with nothing anywhere saying why.
//
// slog rather than stderr, for the reason logStartupFailure gives above: on the
// gokrazy appliance stderr goes to the supervisor and the console sees kmsg.
func bootPowerUsers(log *slog.Logger) []authz.UserConfig {
	users, err := loadZefsUsers()
	if err == nil {
		return users
	}
	// admin-disabled is a declaration, not a fault: nothing is broken and there
	// is nothing to repair. Anything else is a read this daemon expected to work.
	if log != nil && !errors.Is(err, errAdminDisabledInZefs) {
		log.Warn("zefs power user unavailable: the break-glass account cannot log in until this is repaired", "error", err)
	}
	return users
}

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
			logStartupFailure("load config", err)
			return 1
		}
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
			logStartupFailure("initialize active config", activeErr)
			return 1
		}
	}

	configPaths := zeconfig.CollectContainerPaths(loadResult.Tree)
	auditLog, auditErr := openAuditLog(configPath)
	if auditErr != nil {
		fmt.Fprintf(os.Stderr, "error: audit log: %v\n", auditErr)
		logStartupFailure("audit log", auditErr)
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
		mcpAddrs []string
	)
	// The looking glass serves TLS by default: it binds 0.0.0.0 and publishes
	// route data and session state. lgTLSSet records that the operator stated a
	// choice through the environment, so an explicit `false` opts out instead of
	// being overwritten by the config file's own default-true.
	lgTLS := true
	lgTLSSet := false
	lgToken := env.Get("ze.looking-glass.token")
	// Names the PKI store entry the HTTPS listener serves. Env first; the config
	// file fills it in below only when the environment left it blank, matching
	// the precedence every other web setting uses.
	webCertificate := env.Get("ze.web.certificate")
	if webListenAddr != "" {
		webAddrs = []string{webListenAddr}
		webEnabled = true
	}

	if listen := env.Get("ze.looking-glass.listen"); listen != "" {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.looking-glass.listen: %v\n", parseErr)
			logStartupFailure("ze.looking-glass.listen", parseErr)
			return 1
		}
		lgAddrs = make([]string, 0, len(endpoints))
		for _, ep := range endpoints {
			lgAddrs = append(lgAddrs, ep.String())
		}
	}
	if env.Get("ze.looking-glass.tls") != "" {
		lgTLS = env.GetBool("ze.looking-glass.tls", true)
		lgTLSSet = true
	}
	if env.IsEnabled("ze.looking-glass.enabled") && len(lgAddrs) == 0 {
		lgAddrs = []string{"0.0.0.0:8443"}
	}

	if listen := env.Get("ze.web.listen"); listen != "" && len(webAddrs) == 0 {
		endpoints, parseErr := zeconfig.ParseCompoundListen(listen)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: ze.web.listen: %v\n", parseErr)
			logStartupFailure("ze.web.listen", parseErr)
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
			logStartupFailure("ze.mcp.listen", parseErr)
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
	//
	// ADDRESSES come only from a config block that asks for a listener, so
	// `enabled false` still means "config does not start the web server".
	// webAuthFollowsConfig records WHICH input decided the web authentication
	// switch, so a SIGHUP reload can re-answer the same question. The flag and
	// the environment variable are fixed for the life of the process; only the
	// config leaf can change under a reload, and it decides only when it decided
	// here (see the auth reloader registered after the boot guard).
	webAuthFollowsConfig := false
	if webCfg, ok := zeconfig.ExtractWebConfig(loadResult.Tree); ok {
		if len(webAddrs) == 0 {
			webAddrs = endpointsToAddrs(webCfg.Servers)
			insecureWeb = webCfg.Insecure
			webAuthFollowsConfig = true
		}
		webEnabled = true
	}
	// SETTINGS (certificate) apply whenever the block exists, whatever supplied
	// the address. Gating this on `enabled` discarded the operator's certificate
	// choice when --web, ze.web.listen, or ze.web.enabled started the server,
	// leaving a self-signed certificate on a listener the operator believed was
	// serving their own chain
	// (plan/learned/1327-enabled-gate-discards-service-settings.md).
	if webSettings, ok := zeconfig.ExtractWebSettings(loadResult.Tree); ok && webCertificate == "" {
		webCertificate = webSettings.Certificate
	}
	// Two questions, deliberately asked separately (see ExtractMCPSettings).
	//
	// ADDRESSES come only from a config block that asks for a listener, so
	// `enabled false` still means "config does not start MCP".
	if mcpListenCfg, mcpListenOK := zeconfig.ExtractMCPConfig(loadResult.Tree); mcpListenOK && len(mcpAddrs) == 0 {
		mcpAddrs = endpointsToAddrs(mcpListenCfg.Servers)
	}
	// SETTINGS (auth-mode, token, identities, oauth, tls) apply whenever the
	// block exists, whatever supplied the address. Gating these on `enabled`
	// silently discarded the operator's authentication instruction when the
	// listener came from `--mcp <port>` or ze.mcp.listen, leaving an accept-all
	// server (ai/rules/protocol.md).
	// The flag and environment half of the MCP token, captured before the config
	// block fills it in. A reload re-answers the config half against the same
	// base, so the precedence has one implementation (mgmt_auth_reload.go).
	mcpTokenBase := mcpToken
	mcpCfg, mcpCfgOK := zeconfig.ExtractMCPSettings(loadResult.Tree)
	if mcpCfgOK && mcpToken == "" && mcpCfg.Token != "" {
		mcpToken = mcpCfg.Token
	}
	// Two questions, asked separately (the same split as ExtractMCPSettings).
	//
	// ADDRESSES come only from a block that asks for a listener, so
	// `enabled false` still means "config does not start the looking glass".
	if lgListenCfg, ok := zeconfig.ExtractLGConfig(loadResult.Tree); ok && len(lgAddrs) == 0 {
		lgAddrs = endpointsToAddrs(lgListenCfg.Servers)
	}
	// SETTINGS (tls, token) apply whenever the block exists, whatever supplied
	// the address. Gating them on `enabled` discarded the operator's TLS and
	// token instruction when ze.looking-glass.enabled or ze.looking-glass.listen
	// started the server, leaving a plaintext, open looking glass
	// (ai/rules/protocol.md).
	//
	// Precedence: env var > config file > default-on. The config file's own TLS
	// value already defaults true, so the config lowers TLS only when the
	// operator wrote `tls false`, and an env var overrides both.
	if lgCfg, ok := zeconfig.ExtractLGSettings(loadResult.Tree); ok {
		if !lgTLSSet {
			lgTLS = lgCfg.TLS
			lgTLSSet = lgCfg.TLSExplicit
		}
		if lgToken == "" {
			lgToken = lgCfg.Token
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
		logStartupFailure("pki config", pkiErr)
		return 1
	}
	if pkiErr := zepki.Load(pkiConfig); pkiErr != nil {
		fmt.Fprintf(os.Stderr, "error: pki config: %v\n", pkiErr)
		logStartupFailure("pki load", pkiErr)
		return 1
	}

	// Fail closed on a broken web certificate reference (AC-3, R-5). The check
	// runs the SAME resolution the listener will run, so a name that passes here
	// is a name the listener can serve. Refusing to start is the point: a daemon
	// that booted and served a self-signed certificate instead would look
	// healthy while presenting the wrong identity to every client.
	if webEnabled && webCertificate != "" {
		if _, _, certErr := zepki.ServerTLSMaterial(webCertificate); certErr != nil {
			fmt.Fprintf(os.Stderr, "error: environment.web.certificate: %v\n", certErr)
			logStartupFailure("web certificate", certErr)
			return 1
		}
	}

	system.RegisterConntrackManagedKeys()
	// Register infrastructure hook before engine starts.
	// The BGP plugin calls this when creating the reactor.
	// The session reload function is late-bound: the hook registers here,
	// but reloadAfterCommit can only be built after the API server and
	// engine exist. The holder is populated alongside reloadAfterCommit;
	// SSH commits happen long after startup, so sessions always see it.
	var sessionReloadHolder atomic.Pointer[func() error]
	sessionReload := func() error {
		fn := sessionReloadHolder.Load()
		if fn == nil {
			return errors.New("config reload not ready: daemon still starting")
		}
		return (*fn)()
	}
	// The AAA chain's local backend answers from the running config, not from
	// the tree the reactor was built with. The zefs power users are read once
	// here: they live in the blob store, so no reload can change them, and a
	// failure to read them is not fatal while config users exist.
	zefsAuthUsers := bootPowerUsers(slogutil.Logger("hub.aaa"))
	configUsersLive := func() ([]authz.UserConfig, error) { return liveConfigUsers(configProvider) }
	localUsers := liveLocalUsers(zefsAuthUsers, configUsersLive, slogutil.Logger("hub.aaa"))
	setupInfraHook(auditLog, sessionReload, localUsers)
	coordinator := zePlugin.NewCoordinator(configTree)

	// Store config state for the BGP plugin's reactor factory as a typed
	// bootstrap struct (formerly a string-keyed extra bag). The BGP plugin
	// builds its own createReactor closure from these values
	// (bgp/config createReactorFromCoordinator). Callback fields may be nil;
	// the reader guards each.
	coordinator.SetBootstrap(registry.BGPBootstrap{
		ConfigPath:         configPath,
		CLIPlugins:         plugins,
		ConfigData:         data,
		Store:              store,
		ChaosSeed:          chaosSeed,
		ChaosRate:          chaosRate,
		HealthPeerCallback: PeerLifecycleCallback,
		// MRT bridges are self-registered by the MRT plugin's init() into the
		// registry seam (registry.SetMRTMessageCallback / SetMRTPeerCallback) and
		// read by bgp/config; the hub no longer imports internal/plugins/mrt, so
		// //go:build ze_mrt can drop MRT from the binary.
	})

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
		logStartupFailure("create plugin server", serverErr)
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

	var webPortalServices []webPortalService

	// L2TP / PPPoE (BNG) subsystem construction is gated on ze_l2tp: the
	// register_l2tp.go init sets bngRegister (bng_infra.go), which extracts
	// the l2tp/pppoe parameters and registers the subsystems with the engine.
	// With the tag off the seam stays nil and the schema rejects l2tp/pppoe
	// config at parse, so nothing is silently skipped here.
	if bngRegister != nil {
		portals, bngErr := bngRegister(loadResult.Tree, configTree, eng)
		if bngErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", bngErr)
			logStartupFailure("register bng subsystems", bngErr)
			return 1
		}
		webPortalServices = append(webPortalServices, portals...)
	}

	startCtx := context.Background()
	if err := eng.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "error starting engine: %v\n", err)
		logStartupFailure("starting engine", err)
		return 1
	}

	// Start plugin server (auto-loads BGP, iface, fib, etc. via ConfigRoots).
	if err := apiServer.StartWithContext(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "error starting plugin server: %v\n", err)
		logStartupFailure("starting plugin server", err)
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
		logStartupFailure("write pid file", pidErr)
		apiServer.Stop()
		_ = eng.Stop(startCtx)
		return 1
	}
	defer removePIDFile(pidPath)

	if err := dropPrivileges(); err != nil {
		fmt.Fprintf(os.Stderr, "error: drop privileges: %v\n", err)
		logStartupFailure("drop privileges", err)
		apiServer.Stop()
		_ = eng.Stop(startCtx)
		return 1
	}

	// Command dispatchers for user surfaces (use plugin server, not reactor
	// directly). Each is the unified plugin.CommandDispatcher with fixed audit
	// surface attribution; text surfaces flatten via plugin.CommandDispatcher.JSON.
	webDispatch := serverDispatcher(apiServer, audit.Web)
	sshDispatch := serverDispatcher(apiServer, audit.SSH)
	mcpDispatch := serverDispatcher(apiServer, audit.MCP)
	cliDispatch := serverDispatcher(apiServer, audit.CLI)

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
	startArchiveScheduler(loadResult.Tree, configPath, store, apiServer)
	defer stopArchiveScheduler()

	lm := NewListenerMigrator(nil)
	reloadAfterCommit := func() error {
		startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer startupCancel()
		if err := apiServer.WaitForStartupComplete(startupCtx); err != nil {
			return fmt.Errorf("wait for plugin startup: %w", err)
		}
		if err := doReload(apiServer, eng, configProvider, store, configPath, loadBoth, lm); err != nil {
			return err
		}
		if gnmiReloadNotify != nil {
			gnmiReloadNotify()
		}
		return nil
	}
	// Publish the reload for SSH session editors created by the infra hook
	// (registered before this closure could exist).
	sessionReloadHolder.Store(&reloadAfterCommit)

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

	sshCfg := infra.ExtractSSHConfig(loadResult.Tree)
	ephemeralFile := env.Get("ze.ssh.ephemeral")
	if !sshCfg.HasConfig && !hasBGPBlock && ephemeralFile != "" {
		sshCfg = infra.SSHExtractedConfig{
			Listen:    "127.0.0.1:0",
			HasConfig: true,
		}
	}
	// Start ssh through the compile-out seam (ssh_infra.go). The AAA bundle is
	// built always-on (it may also serve MCP/API); only the resolved
	// authenticator crosses the seam. When ssh is compiled out (ze_ssh off)
	// sshBuildStandalone is nil and ssh is skipped.
	if sshCfg.HasConfig && !hasBGPBlock && sshBuildStandalone != nil {
		users := sshCfg.Users
		if zefsUsers, err := loadZefsUsers(); err == nil {
			users = mergeAuthUsers(zefsUsers, users)
		}
		configDir := loadResult.ConfigDir
		if configDir == "" {
			configDir = env.Get("ze.config.dir")
		}
		inputs := &sshStandaloneInputs{
			Config:        sshCfg,
			Users:         users,
			UsersFunc:     localUsers,
			Recorder:      auditLog,
			ConfigDir:     configDir,
			Storage:       infra.ResolveSSHStorage(store, configDir),
			ConfigPath:    configPath,
			EphemeralFile: ephemeralFile,
			Dispatch:      sshDispatch,
			ReloadFn:      reloadAfterCommit,
			Log:           slogutil.Logger("hub.ssh"),
		}

		// Build the AAA bundle via the registry (local + any enabled remote
		// backends). swapAAABundle installs it as the live bundle so
		// closeAAABundle (deferred at the top of runYANGConfig) drains backend
		// workers on process exit.
		aaaLog := slogutil.Logger("hub.aaa")
		aaaBundle, aaaErr := buildAAABundle(loadResult.Tree, users, localUsers, infra.ExtractAuthzStore(loadResult.Tree), aaaLog)
		if aaaErr != nil {
			aaaLog.Warn("AAA backend build failed; SSH authenticator not set", "error", aaaErr)
			registerAAAAccountingProvider(nil)
		} else {
			registerAAAAccountingProvider(aaaBundle)
			inputs.Authenticator = aaaBundle.Authenticator
			swapAAABundle(aaaBundle, aaaLog)
		}

		// Authorization must be wired here, not only in infra_setup.go. That
		// hook runs from the reactor's post-start callback, which only exists
		// when the config has a bgp{} block; on this path it never runs, so
		// without this the dispatcher's authorizer stays nil and
		// Dispatcher.isAuthorized allows every command. An ssh-only box (the
		// gokrazy appliance, environment{}-only configs) would then accept
		// system.authorization profiles and silently enforce none of them --
		// authentication would pass and RBAC would not apply at all.
		//
		// liveAAABundleAuthorizer resolves the bundle per call rather than
		// pinning the one built above, so a reload that changes profiles takes
		// effect without re-wiring.
		if d := apiServer.Dispatcher(); d != nil {
			d.SetAuthorizer(liveAAABundleAuthorizer{})
			aaaLog.Info("authorization configured", "source", "live aaa bundle", "path", "ssh-standalone")
		}

		if stop := sshBuildStandalone(inputs); stop != nil {
			defer stop()
		}
	}

	// Resolve REST/gRPC API listen config (env > config file) up front so the
	// boot-time management-listener guard below sees every management surface's
	// (address, auth) pair before anything binds. The API build path further
	// down reuses apiCfg / apiCfgOK / apiUsers.
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
			logStartupFailure("ze.api.rest.listen", parseErr)
			return 1
		}
		// Env-var override replaces the config-provided list with one entry.
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
			logStartupFailure("ze.api.grpc.listen", parseErr)
			return 1
		}
		apiCfg.GRPC = []zeconfig.APIListenConfig{{Host: host, Port: port}}
	}
	apiTokenEnv := env.Get("ze.api-server.token")
	if apiTokenEnv != "" && apiCfg.Token == "" {
		apiCfg.Token = apiTokenEnv
	}

	var apiUsers []authz.UserConfig
	// Whether the zefs power user was readable at boot. A reload that suddenly
	// cannot read it must fail closed rather than rebuild the API servers
	// without those credentials; a reload on a daemon that never had them keeps
	// working (mgmt_auth_reload.go).
	apiZefsUsersOK := false
	if apiCfgOK {
		if u, uErr := loadZefsUsers(); uErr != nil {
			fmt.Fprintf(os.Stderr, "warning: API power-user auth unavailable: %v\n", uErr)
		} else {
			apiUsers = u
			apiZefsUsersOK = true
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
	}

	// Boot-time management-listener exposure guard (mgmt_guard.go): one
	// fail-closed check that refuses to serve any management surface bound
	// non-loopback without authentication. Each surface declares its resolved
	// (address, auth) pair; the guard function names no service. A surface whose
	// feature-gate is compiled out never declares (it cannot bind).
	webFactoryOn := serviceFactoryRegistered("web")
	mcpFactoryOn := serviceFactoryRegistered("mcp")
	// A secure web listener carries auth middleware (and its own no-users
	// fail-closed disable), so only the insecure path is unauthenticated. MCP
	// auth mirrors the server's effective-mode precedence (see
	// mcpListenerAuthenticated): an explicit "none" makes the server accept-all
	// even with a token set, so a token alone does NOT authenticate.
	webAuthed := !insecureWeb
	mcpAuthed := mcpListenerAuthenticated(mcpCfgOK, mcpCfg.AuthMode, mcpToken)
	apiAuthed := len(apiUsers) > 0 || apiCfg.Token != ""

	// Apply the web default BEFORE declaring, so the guard evaluates the address
	// that will actually be bound rather than an empty slice it iterates zero
	// times. buildWebService applies the same fallback.
	webAddrs = resolveWebListeners(webEnabled, webAddrs)

	var mgmtListeners []mgmtListener
	if webFactoryOn && webEnabled {
		mgmtListeners = append(mgmtListeners, mgmtListener{
			service:       "web (insecure)",
			addrs:         webAddrs,
			authenticated: webAuthed,
			remedy:        "bind ze.web.listen to 127.0.0.1/::1, or drop ze.web.insecure and configure users",
		})
	}
	// Declared only when MCP will actually bind: buildMCPService skips on an
	// empty address list, and a declaration that binds nothing must not reach
	// the guard's no-addresses refusal.
	if mcpFactoryOn && len(mcpAddrs) > 0 {
		mgmtListeners = append(mgmtListeners, mgmtListener{
			service:       "MCP",
			addrs:         mcpAddrs,
			authenticated: mcpAuthed,
			remedy:        "set ze.mcp.token (or environment.mcp auth-mode), or bind to 127.0.0.1/::1 only",
		})
	}
	if gnmiBuild != nil {
		if gnmiAddr, gnmiToken, gnmiEnabled := resolveGNMIListeners(loadResult.Tree); gnmiEnabled {
			mgmtListeners = append(mgmtListeners, mgmtListener{
				service:       "gNMI",
				addrs:         []string{gnmiAddr},
				authenticated: gnmiToken != "",
				remedy:        "set ze.gnmi.token (or environment.gnmi token), or bind to 127.0.0.1/::1 only",
			})
		}
	}
	if apiCfgOK && (restBuild != nil || grpcBuild != nil) {
		apiAddrs := append(apiListenToAddrs(apiCfg.REST), apiListenToAddrs(apiCfg.GRPC)...)
		// Same reason as MCP: restBuildImpl/grpcBuildImpl return an empty handle
		// when their endpoint list is empty, so an empty declaration binds
		// nothing and must not trip the no-addresses refusal.
		if len(apiAddrs) > 0 {
			mgmtListeners = append(mgmtListeners, mgmtListener{
				service:       "API",
				addrs:         apiAddrs,
				authenticated: apiAuthed,
				remedy:        "set ze.api-server.token, initialize zefs users, or bind to 127.0.0.1/::1 only",
			})
		}
	}

	// Run the existing precise MCP semantic checks on the boot path too:
	// bind-remote without auth, bearer without token, and oauth without TLS.
	// They catch config-level inconsistencies that the loopback and auth guard
	// alone does not find. They print the same messages as `ze config validate`.
	//
	// The gate is the listener that binds, not the block that exists.
	// ExtractMCPSettings returns a config for any environment.mcp block. Without
	// the address check, a dormant block refuses boot over an inconsistency that
	// harms nobody. A dormant block is one with enabled false, where MCP never
	// starts.
	if serviceFactoryRegistered("mcp") && mcpCfgOK && len(mcpAddrs) > 0 {
		if err := mcpCfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			logStartupFailure("mcp config", err)
			return 1
		}
	}

	if checkMgmtListeners(mgmtListeners) {
		// checkMgmtListeners printed the per-listener refusals to stderr.
		logStartupFailure("management listener guard", errUnauthenticatedMgmtListener)
		return 1
	}

	// Hand the guard's classification to the listener migrator, BEFORE any
	// server handle is installed on it. The reload guard reads authAtBoot, and
	// every commit path below (the SSH ReloadFn, the web CommitHook,
	// apiServer.SetFullReloadFunc, wireManagedCommit) is wired into a live
	// surface as its server is built. Classifying after those would leave a
	// window in which the migrator holds a handle with no record for it, and an
	// unknown service is the PERMISSIVE branch of checkReloadExposure: a web
	// migration to a non-loopback address would pass unauthenticated. Nothing
	// else backstops web, which carries no loopback rule of its own.
	//
	// A surface the daemon never builds is handled at RELOAD time instead, in
	// resolveAuthIntents, which needs no startup ordering to be correct.
	//
	// gNMI is deliberately absent from this map though checkMgmtListeners
	// classifies it above. It has no ListenerMigrator slot, so buildChanges can
	// never move its listener and there is no migration for the guard to judge.
	markMgmtAuth(lm, map[string]bool{
		svcWeb:  webAuthed,
		svcMCP:  mcpAuthed,
		svcREST: apiAuthed,
		svcGRPC: apiAuthed,
	})

	// Teach the migrator how to re-answer each surface's authentication
	// question from a reloaded tree, so an auth-mode change takes effect on
	// SIGHUP instead of waiting for a restart.
	registerMgmtAuthReloaders(lm, mgmtAuthInputs{
		webFollowsConfig: webAuthFollowsConfig,
		mcpTokenBase:     mcpTokenBase,
		apiTokenEnv:      apiTokenEnv,
		apiZefsUsersOK:   apiZefsUsersOK,
		apiUsersLive:     localUsers,
	})

	// Build optional, compile-out-able services through the construction
	// registry. With a feature's ze_<feature> tag off, its factory is not
	// registered and the service is silently skipped. Looking-glass (ze_lg) is
	// the pilot; its listen binding (lgAddrs/lgTLS) is resolved above.
	builtServices := buildServices(ServiceDeps{
		Store:          store,
		ConfigPath:     configPath,
		Resolvers:      resolvers,
		Dispatch:       webDispatch,
		LGAddrs:        lgAddrs,
		LGTLS:          lgTLS,
		LGTLSExplicit:  lgTLSSet,
		LGToken:        lgToken,
		WebEnabled:     webEnabled,
		WebAddrs:       webAddrs,
		InsecureWeb:    insecureWeb,
		WebCertificate: webCertificate,
		Authorizer:     liveAAABundleAuthorizer{},
		Recorder:       auditLog,
		CommitHook:     reloadAfterCommit,
		// The zefs break-glass account, read once above and merged into
		// localUsers. Handing the factory the same snapshot and the same closure
		// the AAA chain uses is what keeps one producer: a web server that read
		// zefs again could fail where the chain succeeded, admit a power user
		// through the chain, and then revoke their session on the next request
		// for not being declared.
		PowerUsers: zefsAuthUsers,
		// The power users merged with the config users, read from the shared
		// ConfigProvider at each login instead of pinned here. Every applied
		// reload refreshes that provider, so a deleted user stops authenticating
		// without a restart. It answers the serve-or-not question too, which
		// used to come from sshCfg.Users: that list is empty when the config
		// declares users but no `environment { ssh { } }` block, so a web server
		// with users to admit could refuse to start.
		LocalUsersLive:    localUsers,
		EventRing:         apiServer.EventRing(),
		WebPortalServices: webPortalServices,
		// Plugin-registered commands for web tab-completion, resolved lazily
		// (plugins register after this point; the web factory reads it on first
		// completion request). Mirrors the SSH per-session merge.
		WebCommands: func() []command.CommandEntry {
			d := apiServer.Dispatcher()
			if d == nil {
				return nil
			}
			return d.Registry().VisibleCommandEntries()
		},
		// MCP is built through the registry (service_mcp.go, //go:build ze_mcp)
		// like web/lg. The listen/token/config resolution stays always-on here
		// (plain values); the gated factory converts them into zemcp types. With
		// ze_mcp off no factory consumes these fields and the mcp package is not
		// linked.
		MCP: &mcpServiceDeps{
			Addrs:    mcpAddrs,
			Token:    mcpToken,
			Config:   mcpCfg,
			ConfigOK: mcpCfgOK,
			Dispatch: mcpDispatch,
			Commands: commandMetaSource(apiServer),
			Recorder: auditLog,
		},
	})
	for _, svc := range builtServices {
		registerBuiltService(lm, svc)
	}
	if len(builtServices) > 0 {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer shutdownCancel()
			for _, svc := range builtServices {
				_ = svc.Shutdown(shutdownCtx)
			}
		}()
	}

	// REST/gRPC API listen config and users were resolved above (before the
	// management-listener guard); apiCfg / apiCfgOK / apiUsers are reused here.
	var apiShutdowns []func(context.Context)
	apiServer.SetFullReloadFunc(func(context.Context) error {
		return reloadAfterCommit()
	})
	managedCtx, managedCancel := context.WithCancel(context.Background())
	defer managedCancel()
	if managedClient != nil && storage.IsBlobStorage(store) {
		wireManagedCommit(managedClient, store, configPath, reloadAfterCommit, auditLog)
	}
	if apiCfgOK {
		// apiUsers and the auth-mode report were resolved above; the boot-time
		// management-listener guard has already refused any non-loopback API
		// listener without authentication (the former apiHasNonLoopback inline
		// refusal is folded into the single shared classifier).

		// Build REST and gRPC through their compile-out seams (service_rest.go /
		// service_grpc.go). Each transport is independently gated (ze_rest /
		// ze_grpc); with a transport off its hook is nil and it is skipped. The
		// config resolution above and the shared engine/sessions stay always-on.
		if restBuild != nil || grpcBuild != nil {
			apiIn := &apiBuildInputs{
				Config:     apiCfg,
				Server:     apiServer,
				Store:      store,
				ConfigPath: configPath,
				Users:      apiUsers,
				UsersLive:  localUsers,
				Authorizer: liveAAABundleAuthorizer{},
				ReloadHook: reloadAfterCommit,
				Recorder:   auditLog,
			}
			shared := buildAPIShared(apiIn)
			if restBuild != nil {
				h, apiErr := restBuild(apiIn, shared)
				if apiErr != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", apiErr)
					logStartupFailure("start rest api", apiErr)
					apiServer.Stop()
					_ = eng.Stop(startCtx)
					return 1
				}
				if h.Server != nil {
					lm.SetREST(h.Server)
					apiShutdowns = append(apiShutdowns, h.Shutdown)
				}
			}
			if grpcBuild != nil {
				h, apiErr := grpcBuild(apiIn, shared)
				if apiErr != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", apiErr)
					logStartupFailure("start grpc api", apiErr)
					apiServer.Stop()
					_ = eng.Stop(startCtx)
					return 1
				}
				if h.Server != nil {
					lm.SetGRPC(h.Server)
					apiShutdowns = append(apiShutdowns, h.Shutdown)
				}
			}
		}
	}

	// Start gNMI through the compile-out seam (gnmi_infra.go). When ze_gnmi is
	// absent, gnmiBuild is nil and gNMI is skipped without naming the package.
	var gnmiSrv gnmiServer
	if gnmiBuild != nil {
		gnmiSrv = gnmiBuild(&gnmiBuildInputs{
			Tree:              loadResult.Tree,
			TreeFn:            func() *zeconfig.Tree { return loadResult.Tree },
			Store:             store,
			ConfigPath:        configPath,
			ReloadAfterCommit: reloadAfterCommit,
		})
	}

	// Signal handling: SIGINT/SIGTERM for shutdown, SIGHUP for config reload.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Reactor-independent `request shutdown`: a BGP daemon stops via the reactor,
	// but a reactorless daemon (OSPF-only, etc.) needs the command to reach the
	// signal-based teardown below. Wired ungated so `request shutdown` works
	// regardless of which protocols are configured (non-blocking, mirrors
	// monitorStdinEOF).
	apiServer.SetShutdownFunc(func() {
		select {
		case sigCh <- syscall.SIGTERM:
		default:
		}
	})

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
		logStartupFailure("plugin startup", err)
		startupCancel()
		if gnmiSrv != nil {
			gnmiSrv.Stop()
		}
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
	// Serve managed fleet clients declared under hub server blocks (answers
	// config-fetch, pushes config-changed). No-op unless a server block has client
	// entries and storage is blob-backed. Independent of the outbound managed client.
	startManagedServer(managedCtx, store, hubConfig)

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
			return cliDispatch.JSON(context.Background(), zePlugin.CallerIdentity{Username: "root", RemoteAddr: "local"}, command)
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

	// MCP shuts down through the construction registry's builtServices defer
	// (like web/lg), so it is not stopped explicitly here.

	if gnmiSrv != nil {
		gnmiSrv.Stop()
	}

	if len(apiShutdowns) > 0 {
		apiCtx, apiCancel := context.WithTimeout(context.Background(), 3*time.Second)
		for _, sd := range apiShutdowns {
			sd(apiCtx)
		}
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

// runOrchestratorWithData parses hub config and runs the orchestrator.
func runOrchestratorWithData(store storage.Storage, configPath string, data []byte) int {
	cfg, err := hub.ParseHubConfig(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse config: %v\n", err)
		logStartupFailure("parse hub config", err)
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
					// A failed hub reload shuts the daemon down; mirror the
					// reason onto slog (kmsg on the appliance) like the
					// pre-serve failures, or the death is invisible on serial.
					slogutil.Logger("hub").Error("config reload failed; shutting down", "err", err)
					cancel()
					return
				}
				// The hub-config reload loop, selected by Run on
				// zeconfig.ProbeConfigType. It must announce completion for the
				// same reason handleSIGHUPReload does: without it a hub daemon's
				// last word is "received SIGHUP, reloading config...", whether
				// the reload finished or wedged, and no .ci can fence on it.
				reloadComplete()
			}
		}
	}()

	// Start orchestrator
	if err := o.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: start: %v\n", err)
		logStartupFailure("start orchestrator", err)
		return 1
	}

	// Drop privileges after port binding.
	if err := dropPrivileges(); err != nil {
		fmt.Fprintf(os.Stderr, "error: drop privileges: %v\n", err)
		logStartupFailure("drop privileges", err)
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
	if st.closer != nil {
		_ = st.closer.Close()
	}
}
