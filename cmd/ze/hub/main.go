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
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	unicli "github.com/ze-software/ze/internal/component/cli"
	zecli "github.com/ze-software/ze/internal/component/cli/client"
	showCmd "github.com/ze-software/ze/internal/component/cmd/show"
	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/engine"
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

	// One config, one parser, one runtime. Every config the YANG schema accepts
	// boots here, `plugin {}` on its own included: a config that passes
	// `ze config validate` must boot, and a second runtime with its own parser
	// is how the two used to disagree.
	return runYANGConfig(store, configPath, data, plugins, chaosSeed, chaosRate, stdinOpen, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, cliAttach, managedClient)
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

// bootPowerUsers reads the zefs break-glass accounts once. The read is not
// fatal: config users can still authenticate.
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

// recoverableLoadError reports whether a LoadConfig failure is the kind
// RecoverConfig answers. RecoverConfig exists for ONE failure: this binary is
// older than the release that stamped the config on disk, so it cannot read
// what the newer schema wrote. Its answer is to walk rollback history and
// REWRITE the config file with the newest version it can load.
//
// A ze:validate custom validator refusal is not that failure. It says the
// operator wrote a value the rules refuse, and it names the leaf, the value and
// the rule. Rewriting their file and starting on an older config answers a
// typo by discarding the edit, and it makes the daemon start on a config the
// operator never wrote -- the opposite of AC-1 in
// spec-fixit-config-validators-bypassed-at-startup, which is that the daemon
// refuses and says why.
//
// The route only became reachable when LoadConfig gained its validation call:
// before that, a validator refusal was never a LoadConfig error at all.
func recoverableLoadError(err error) bool {
	return !errors.Is(err, zeconfig.ErrCustomValidation)
}

// noBGPAAAWiring returns the authenticator and authorizer standalone SSH takes
// on a daemon that runs without BGP.
//
// Both are LIVE indirections over the atomic bundle slot rather than the built
// bundle's own fields. SSH reads its authenticator once, at construction, so a
// captured chain keeps authenticating against the RADIUS or TACACS+ server the
// BOOT tree named after a reload has replaced it (aaa_lifecycle.go).
//
// A nil bundle returns a nil pair, because a nil authenticator is what tells ssh
// to fall back to local users; a live indirection over an absent bundle would
// reject every login instead.
func noBGPAAAWiring(bundle *aaa.Bundle) (aaa.Authenticator, aaa.Authorizer) {
	if bundle == nil {
		return nil, nil
	}
	return liveAAABundleAuthenticator{}, liveAAABundleAuthorizer{}
}

// installNoBGPAAADispatch installs live bundle adapters on the shared
// management dispatcher used by API, MCP, and standalone SSH.
func installNoBGPAAADispatch(d *pluginserver.Dispatcher) {
	if d == nil {
		return
	}
	d.SetAuthorizer(liveAAABundleAuthorizer{})
	d.SetAccountingHook(newLiveAAABundleAccountant())
}

func runYANGConfig(store storage.Storage, configPath string, data []byte, plugins []string, chaosSeed int64, chaosRate float64, stdinOpen, webEnabled bool, webListenAddr string, insecureWeb bool, mcpAddr, mcpToken string, cliAttach bool, managedClient *managed.ClientConfig) int { //nolint:cyclop // startup orchestration
	// Close the AAA bundle and clear the accepted local identity on every exit
	// path so backend workers drain and no daemon or test run inherits
	// credentials or policy.
	defer closeAAABundle(slogutil.Logger("hub.aaa"))

	// Phase 1: Parse config and resolve plugins.
	loadResult, err := zeconfig.LoadConfig(string(data), configPath, plugins)
	if err != nil {
		var recovered *zeconfig.LoadConfigResult
		ok := false
		if recoverableLoadError(err) {
			recovered, ok = zeconfig.RecoverConfig(store, configPath, data, plugins)
		}
		if ok {
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
	// ze.looking-glass.enabled says START, and says nothing about WHERE. Its
	// 0.0.0.0:8443 default is applied below, after the config block has been
	// read, so an env-started looking glass still binds the address the operator
	// wrote. Applying it here published on every interface a block that named
	// 127.0.0.1 (same defect as environment.gnmi, cmd/ze/hub/gnmi_infra.go).
	lgEnabledByEnv := env.IsEnabled("ze.looking-glass.enabled")

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
	// SETTINGS (certificate, and the address the block names) apply whenever the
	// block exists, whatever supplied the address. Gating this on `enabled`
	// discarded the operator's certificate choice when --web, ze.web.listen, or
	// ze.web.enabled started the server, leaving a self-signed certificate on a
	// listener the operator believed was serving their own chain.
	if webSettings, ok := zeconfig.ExtractWebSettings(loadResult.Tree); ok {
		// Only when something else already said START: a dormant block must not
		// supply an address, because webEnabled is what starts the web server.
		// Without this, ze.web.enabled bound the 0.0.0.0:3443 default over the
		// loopback address the block named (resolveWebListeners below), and with
		// ze.web.insecure set the daemon refused to boot over an address the
		// operator had never written.
		//
		// Insecure is deliberately NOT taken here. It removes authentication, so
		// reading it outside the enable gate would let a dormant block disarm a
		// listener an env var started -- the opposite of what this split fixes
		// for every other setting.
		if webEnabled && len(webAddrs) == 0 {
			webAddrs = endpointsToAddrs(webSettings.Servers)
		}
		if webCertificate == "" {
			webCertificate = webSettings.Certificate
		}
		// Say so when that exclusion changes the outcome. The operator wrote
		// `insecure true` and the web server starts authenticated anyway, which
		// is silent divergence unless the daemon names the leaf it dropped.
		// webAuthFollowsConfig is exactly "the block decided the switch", so it
		// is false both for a dormant block and for an enabled block whose
		// address a flag or an environment variable supplied first.
		if webEnabled && webSettings.Insecure && !insecureWeb && !webAuthFollowsConfig {
			slogutil.Logger("hub.web").Warn(
				"environment.web insecure not honored: this leaf removes authentication. The block decides web authentication only when it starts the server and names the listen address",
				"leaf", "environment.web.insecure",
				"remedy", "set ze.web.insecure=1, or write enabled true in the block and let it name the server address",
			)
		}
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
	// SETTINGS (tls, token, and the address the block names) apply whenever the
	// block exists, whatever said START. Gating them on `enabled` discarded the
	// operator's TLS and token instruction when ze.looking-glass.enabled or
	// ze.looking-glass.listen started the server, leaving a plaintext, open
	// looking glass (ai/rules/protocol.md).
	//
	// Precedence: env var > config file > default-on. The config file's own TLS
	// value already defaults true, so the config lowers TLS only when the
	// operator wrote `tls false`, and an env var overrides both.
	if lgCfg, ok := zeconfig.ExtractLGSettings(loadResult.Tree); ok {
		// Only when something else already said START: a dormant block must not
		// supply an address, because a non-empty lgAddrs is what starts the
		// looking glass.
		if lgEnabledByEnv && len(lgAddrs) == 0 {
			lgAddrs = endpointsToAddrs(lgCfg.Servers)
		}
		if !lgTLSSet {
			lgTLS = lgCfg.TLS
			lgTLSSet = lgCfg.TLSExplicit
		}
		if lgToken == "" {
			lgToken = lgCfg.Token
		}
	}
	// No looking-glass block at all, so nothing named an address: bind the same
	// default extractServerList synthesizes for a block that names no server.
	if lgEnabledByEnv && len(lgAddrs) == 0 {
		lgAddrs = []string{"0.0.0.0:8443"}
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

	// ToPluginMap, not ToMap: this map becomes the coordinator's config tree,
	// which is where deliverConfigRPC and every reload build a plugin's config
	// section from. It therefore has to carry the entry order of a list whose
	// evaluation depends on it -- a prefix-list, a firewall chain, a failover
	// server list (internal/core/configorder).
	configTree := loadResult.Tree.ToPluginMap()
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
	// Assemble the boot identity after ConfigProvider population. API listener
	// resolution runs here as part of that assembly so local users, policy,
	// per-user mode, shared token mode, and no-auth mode publish once.
	zefsAuthUsers := bootPowerUsers(slogutil.Logger("hub.aaa"))
	configUsersCandidate := func() ([]authz.UserConfig, error) { return liveConfigUsers(configProvider) }
	resolveCandidateUsers := liveLocalUsers(zefsAuthUsers, configUsersCandidate, slogutil.Logger("hub.aaa"))
	bootUsers, bootUsersErr := resolveBootUsers(resolveCandidateUsers)
	if bootUsersErr != nil {
		fmt.Fprintf(os.Stderr, "error: resolve boot users: %v\n", bootUsersErr)
		logStartupFailure("resolve boot users", bootUsersErr)
		return 1
	}
	apiCfg, apiCfgOK, apiResolveErr := resolveAPIListeners(loadResult.Tree)
	if apiResolveErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", apiResolveErr)
		logStartupFailure("api-server listen", apiResolveErr)
		return 1
	}
	apiTokenEnv := env.Get("ze.api-server.token")
	if apiTokenEnv != "" && apiCfg.Token == "" {
		apiCfg.Token = apiTokenEnv
	}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(
		bootUsers,
		infra.ExtractAuthzStore(loadResult.Tree),
		resolveCandidateUsers,
		apiCfg.Token,
	))
	setupInfraHook(auditLog, sessionReload, liveAcceptedLocalUsers)
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
		loadConfigFromDisk, loadBoth = diskConfigLoaders(store, configPath, plugins)
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
	setIdentityStore(store)
	startUpdateChecker(&sc)
	defer stopBackend()
	startArchiveScheduler(loadResult.Tree, configPath, store, apiServer)
	defer stopArchiveScheduler()

	lm := newListenerMigrator()
	reloadAfterCommitContext := func(ctx context.Context) error {
		startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
		defer startupCancel()
		if err := apiServer.WaitForStartupComplete(startupCtx); err != nil {
			return fmt.Errorf("wait for plugin startup: %w", err)
		}
		if err := doReloadContext(ctx, apiServer, eng, configProvider, store, configPath, loadBoth, lm); err != nil {
			return err
		}
		if gnmiReloadNotify != nil {
			gnmiReloadNotify()
		}
		return nil
	}
	reloadAfterCommit := func() error {
		return reloadAfterCommitContext(context.Background())
	}
	// Publish the reload for SSH session editors created by the infra hook
	// (registered before this closure could exist).
	sessionReloadHolder.Store(&reloadAfterCommit)

	// Without BGP, main owns the AAA bundle for every management surface. Build
	// and install it before standalone SSH, MCP, REST, or gRPC can bind. The BGP
	// path remains owned by infraSetup through the reactor hook.
	_, hasBGPBlock := configTree["bgp"]
	var noBGPAuthenticator aaa.Authenticator
	var noBGPAuthorizer aaa.Authorizer
	if !hasBGPBlock {
		aaaLog := slogutil.Logger("hub.aaa")
		aaaBundle, aaaErr := buildAAABundle(loadResult.Tree, bootUsers, liveAcceptedLocalUsers, aaaLog)
		if aaaErr != nil {
			aaaLog.Warn("AAA backend build failed", "error", aaaErr)
			aaaBundle = nil
		}
		registerAAAAccountingProvider(aaaBundle)
		swapAAABundle(aaaBundle, aaaLog)
		noBGPAuthenticator, noBGPAuthorizer = noBGPAAAWiring(aaaBundle)

		// Resolve authorization and accounting per dispatch so later swaps take
		// effect without rewiring. This belongs to no-BGP startup, not to
		// optional SSH.
		if d := apiServer.Dispatcher(); d != nil {
			installNoBGPAAADispatch(d)
			aaaLog.Info("authorization and accounting configured", "source", "live aaa bundle", "path", "no-bgp")
		}
	}

	if _, hasTelemetry := configTree["telemetry"]; hasTelemetry && !hasBGPBlock {
		if st := startStandaloneTelemetry(loadResult.Tree); st != nil {
			defer st.Close()
		}
	}

	// Start SSH directly when configured without BGP. The shared boot snapshot
	// and already-installed no-BGP AAA authenticator cross the compile-out seam;
	// SSH does not own either producer.
	sshCfg := infra.ExtractSSHConfig(loadResult.Tree)
	ephemeralFile := env.Get("ze.ssh.ephemeral")
	if !sshCfg.HasConfig && !hasBGPBlock && ephemeralFile != "" {
		sshCfg = infra.SSHExtractedConfig{
			Listen:    "127.0.0.1:0",
			HasConfig: true,
		}
	}
	if sshCfg.HasConfig && !hasBGPBlock && sshBuildStandalone != nil {
		configDir := loadResult.ConfigDir
		if configDir == "" {
			configDir = env.Get("ze.config.dir")
		}
		inputs := &sshStandaloneInputs{
			Config:        sshCfg,
			Users:         bootUsers,
			UsersFunc:     liveAcceptedLocalUsers,
			Authenticator: noBGPAuthenticator,
			Authorizer:    noBGPAuthorizer,
			Recorder:      auditLog,
			ConfigDir:     configDir,
			Storage:       infra.ResolveSSHStorage(store, configDir),
			ConfigPath:    configPath,
			EphemeralFile: ephemeralFile,
			Dispatch:      sshDispatch,
			ReloadFn:      reloadAfterCommit,
			Log:           slogutil.Logger("hub.ssh"),
		}
		if stop := sshBuildStandalone(inputs); stop != nil {
			defer stop()
		}
	}

	if apiCfgOK {

		// Report active auth mode to make silent degradation visible.
		switch {
		case len(bootUsers) > 0:
			fmt.Fprintf(os.Stderr, "API auth mode: per-user (%d users)\n", len(bootUsers))
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
	apiAuthed := len(bootUsers) > 0 || apiCfg.Token != ""

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
		apiAddrs := apiGuardAddrs(apiCfg)
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
	// classifies it above. It has no listenerMigrator slot, so buildChanges can
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
		webFollowsConfig:  webAuthFollowsConfig,
		mcpTokenBase:      mcpTokenBase,
		apiTokenEnv:       apiTokenEnv,
		apiCandidateUsers: resolveCandidateUsers,
	})

	// REST/gRPC API listen config and users were resolved above (before the
	// management-listener guard); apiCfg, apiCfgOK, and bootUsers are reused here.
	var apiShutdowns []func(context.Context)
	apiServer.SetFullReloadFunc(reloadAfterCommitContext)
	managedCtx, managedCancel := context.WithCancel(context.Background())
	defer managedCancel()
	if managedClient != nil && storage.IsBlobStorage(store) {
		wireManagedCommit(managedClient, store, configPath, reloadAfterCommit, auditLog)
	}
	if apiCfgOK {
		// bootUsers and the auth-mode report were resolved above; the boot-time
		// management-listener guard has already refused any non-loopback API
		// listener without authentication (the former apiHasNonLoopback inline
		// refusal is folded into the single shared classifier).

		// Build REST and gRPC through their compile-out seams (service_rest.go /
		// service_grpc.go). Each transport is independently gated (ze_rest /
		// ze_grpc); with a transport off its hook is nil and it is skipped. The
		// config resolution above and the shared engine/sessions stay always-on.
		if restBuild != nil || grpcBuild != nil {
			apiIn := &apiBuildInputs{
				Config:         apiCfg,
				Server:         apiServer,
				Store:          store,
				ConfigPath:     configPath,
				Authentication: liveAcceptedAPIAuthentication,
				ReloadHook:     reloadAfterCommit,
				Recorder:       auditLog,
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
					lm.setREST(h.Server)
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
					lm.setGRPC(h.Server)
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
	// reloadDone lets the shutdown below wait for a reload still running when the
	// daemon is asked to stop, so its verdict is not lost with the process
	// (awaitReloadWorker, main_reload.go).
	reloadDone := make(chan struct{})
	go handleSIGHUPReload(reloadCh, reloadDone, apiServer, eng, configProvider, store, configPath, loadBoth, lm, auditLog)

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

	// Build optional, compile-out-able services through the construction
	// registry after plugin startup completes and the dispatcher command registry
	// freezes. A plugin startup failure returns before these factories can bind
	// their listeners. With a feature's ze_<feature> tag off, its factory is not
	// registered and the service is silently skipped. Looking-glass (ze_lg) is
	// the pilot; its listen binding (lgAddrs/lgTLS) is resolved above.
	builtServices := buildServices(serviceDeps{
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
		// The zefs break-glass account is read once above. The accepted merged
		// generation is the authentication source; a reload candidate in
		// ConfigProvider cannot invalidate a session or admit a login before the
		// transaction commits.
		PowerUsers:        zefsAuthUsers,
		LocalUsersLive:    liveAcceptedLocalUsers,
		EventRing:         apiServer.EventRing(),
		WebPortalServices: webPortalServices,
		// Plugin-registered commands for web tab-completion. Startup has frozen
		// the initial command registry. Resolve each completion request lazily
		// so plugin additions and removals from reload are visible. This mirrors
		// the SSH per-session merge.
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
		zecli.RunAttached(func(command string) (unicli.CommandOutput, error) {
			rendered, err := cliDispatch.JSON(context.Background(), zePlugin.CallerIdentity{Username: "root", RemoteAddr: "local"}, command)
			if rendered == nil {
				return unicli.CommandOutput{}, err
			}
			return unicli.CommandOutput{
				Text:              rendered.Output,
				TransportComplete: rendered.TransportComplete,
			}, err
		})
		fmt.Println("CLI detached. Press Ctrl+C to stop daemon.")
	}

	// Wait for either a signal or an explicit server shutdown request.
	// Server.Wait reports the request so this loop can call Server.Stop.
	// A reload can remove every plugin without ending this daemon lifecycle.
	doneCh := make(chan struct{})
	go waitForServerDone(apiServer, doneCh)
	waitLoop(sigCh, reloadCh, doneCh)
	close(reloadCh)
	awaitReloadWorker(reloadDone, reloadShutdownGrace)
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

func (st *standaloneTelemetry) Close() {
	if st.closer != nil {
		_ = st.closer.Close()
	}
}
