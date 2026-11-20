// Design: docs/architecture/system-architecture.md -- ze start command and managed mode

//go:build ze_core

package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/cmd/ze/hub"
	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	"github.com/ze-software/ze/cmd/ze/startprep"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/managed"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// openExplicitStore creates only a genuinely absent store. Permission,
// ownership, corruption and artifact refusals are never startup fallbacks.
func openExplicitStore(configPath string) (storage.Storage, error) {
	store, err := internalresolve.StorageFor(configPath)
	if !errors.Is(err, storage.ErrNoStore) {
		return store, err
	}
	return createAbsentStore(internalresolve.StoreDir(configPath))
}

// createAbsentStore makes the store openExplicitStore found absent. A shipped
// build always creates the live tree; only a functional-test DUT build
// (register_store_zetest.go, the zetest tag) can swap it, so no operator
// reaches another store by accident.
var createAbsentStore = createLiveStore

// createLiveStore creates the live tree under dir and says so on stderr.
func createLiveStore(dir string) (storage.Storage, error) {
	store, err := storage.Create(dir)
	if err != nil {
		return nil, err
	}
	var tb textbuf.Buffer
	tb.Str("created live store: ").Str(filepath.Join(dir, "database")).Str("\n").StdErr() //nolint:errcheck // a lost notice line does not change the store
	return store, nil
}

func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

func startUsage() {
	p := helpfmt.Page{
		Command:   "ze start",
		ShortHelp: "Start the Ze daemon from the live store, or from an explicit config file",
		Usage:     []string{"ze start [<config-file>] [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: helpOptionsSectionTitle, Entries: []helpfmt.HelpEntry{
				{Name: "--cli", Desc: "Attach interactive CLI after startup"},
				{Name: "--web <port>", Desc: "Enable web UI on given port (requires config)"},
				{Name: flagStartWebOnly, Desc: "Start web UI only, no daemon (config editing only)"},
				{Name: flagStartInsecureWeb, Desc: "Disable web auth (binds to localhost only)"},
				{Name: helpMCPPortOption, Desc: "Enable MCP server on given port"},
				{Name: "--mcp-token <token>", Desc: "Bearer token for MCP authentication"},
			}},
			{Title: "Prerequisites", Entries: []helpfmt.HelpEntry{
				{Name: "ze init", Desc: "Bootstrap database (required before first start without a config file)"},
				{Name: "ze config edit", Desc: "Create or edit configuration"},
			}},
		},
		Examples: []string{
			"ze start                           Start daemon with default config",
			"ze start /etc/ze/router.conf       Start daemon from a specific config file",
			"ze start --cli                     Start daemon and attach interactive CLI",
			"ze start --web 3443                Start with web UI on port 3443",
			"ze start --web 3443 --insecure-web Start with web UI, no auth (localhost)",
			"ze start --web-only --web 3443     Web UI only (no operational commands)",
		},
	}
	p.WriteErr()
}

// Flag names shared between cmdStart's argument loop and startConfigPath. They
// are named constants so the path-extraction helper cannot drift from the flags
// the loop actually consumes.
const (
	flagStartCLI         = "--cli"
	flagStartWeb         = "--web"
	flagStartWebOnly     = "--web-only"
	flagStartInsecureWeb = "--insecure-web"
	flagStartMCP         = "--mcp"
	flagStartMCPToken    = "--mcp-token" //nolint:gosec // G101 false positive: this is a CLI flag name, not a credential
)

// startConfigPath returns the first positional (non-flag) token in a `ze start`
// argument list, or "" when there is none. It skips the flags that consume a
// following value (--web, --mcp, --mcp-token) so a port or token is never
// mistaken for the config path, and it ignores value-less flags. Keyword-first
// grammar (ai/rules/cli.md R1): the config path is the sole free-form
// value cmdStart accepts, and it follows the `start` keyword.
//
// This helper mirrors the value-consuming flag set of cmdStart's arg loop; keep
// the two in sync when adding a flag that takes a value.
func startConfigPath(args []string) string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case flagStartWeb, flagStartMCP, flagStartMCPToken:
			i++ // the following token is this flag's value, not the config path
		case flagStartCLI, flagStartWebOnly, flagStartInsecureWeb:
			// value-less flags: no path here
		default:
			if args[i] == "-" || !strings.HasPrefix(args[i], "-") {
				return args[i]
			}
		}
	}
	return ""
}

func cmdStart(args, plugins []string, chaosSeed int64, chaosRate float64, globalMCPAddr, globalMCPToken, globalWebPort string, globalInsecureWeb, globalWebOnly bool) int {
	// Start with global flag values, allow local flags to override.
	mcpAddr := globalMCPAddr
	mcpToken := globalMCPToken
	webPort := globalWebPort
	insecureWeb := globalInsecureWeb
	webOnly := globalWebOnly
	cliEnabled := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case flagStartCLI:
			cliEnabled = true
		case flagStartWeb:
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --web requires a port\n")
				return 1
			}

			i++
			if !validPort(args[i]) {
				fmt.Fprintf(os.Stderr, "error: --web port must be 1-65535, got %q\n", args[i])
				return 1
			}
			webPort = args[i]
		case flagStartWebOnly:
			webOnly = true
		case flagStartInsecureWeb:
			insecureWeb = true
		case flagStartMCP:
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --mcp requires a port\n")
				return 1
			}
			i++
			if !validPort(args[i]) {
				fmt.Fprintf(os.Stderr, "error: --mcp port must be 1-65535, got %q\n", args[i])
				return 1
			}
			var tb textbuf.Buffer
			mcpAddr = tb.Str("127.0.0.1:").Str(args[i]).String()
		case flagStartMCPToken:
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --mcp-token requires a value\n")
				return 1
			}

			i++
			mcpToken = args[i]
		}
	}

	webEnabled := webPort != ""
	webListenAddr := ""
	if webEnabled {
		var tb textbuf.Buffer
		webListenAddr = tb.Str("0.0.0.0:").Str(webPort).String()
		if insecureWeb {
			webListenAddr = tb.Reset().Str("127.0.0.1:").Str(webPort).String()
		}
	}
	if insecureWeb && !webEnabled && !webOnly {
		fmt.Fprintf(os.Stderr, "error: --insecure-web requires --web <port> or --web-only\n")
		return 1
	}
	if webOnly && webListenAddr == "" {
		webListenAddr = "0.0.0.0:3443"
		if insecureWeb {
			webListenAddr = "127.0.0.1:3443"
		}
	}

	// The registered start steps run before the store is opened and before
	// anything enumerates interfaces: bootstrapFromDiscovery below and the
	// iface component's apply in hub.Run both list the NICs, and a step may
	// be what makes a NIC appear. Each step logs its own faults and returns.
	startprep.Run()

	// Explicit-file mode keeps an absolute path through reload and commit.
	// Stored-config mode below carries only the config's name.
	if configPath := startConfigPath(args); configPath != "" {
		if webOnly {
			fmt.Fprintf(os.Stderr, "error: --web-only cannot be combined with a config-file path\n")
			return 1
		}
		if configPath == "-" {
			return withPanicCapture(func() int {
				return hub.Run(nil, configPath, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, cliEnabled)
			})
		}
		configPath, err := filepath.Abs(config.ResolveConfigPath(configPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: resolve config path: %v\n", err)
			return 1
		}
		store, err := openExplicitStore(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: open config store: %v\n", err)
			return 1
		}
		store = storage.BindConfigSource(store, configPath, storage.ConfigSourceFile)
		// One runtime for every config: hub.Run reads the file and parses it
		// against the YANG schema, whatever top-level blocks it declares.
		return runOwnedDaemonStore(store, func(borrowed storage.Storage) int {
			return withPanicCapture(func() int {
				return hub.Run(borrowed, configPath, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, cliEnabled)
			})
		})
	}

	store, err := internalresolve.StorageFor("")
	if err != nil {
		if !env.IsEnabled("ze.gokrazy.enabled") {
			fmt.Fprintf(os.Stderr, "error: open config store: %v\n", err)
			return 1
		}
		store, err = gokrazyAutoInit()
		if err != nil {
			slog.Error("gokrazy auto-init failed", "error", err)
			return 1
		}
	}
	configName, err := internalresolve.DefaultConfig(store)
	if err != nil {
		return daemonStoreFailure(errors.Join(err, store.Close()))
	}
	store = storage.BindConfigSource(store, configName, storage.ConfigSourceStored)
	return runOwnedDaemonStore(store, func(store storage.Storage) int {

		// Explicit --web-only: start standalone web UI, no daemon.
		if webOnly {
			if cliEnabled {
				fmt.Fprintf(os.Stderr, "warning: --cli ignored in --web-only mode (no daemon to attach to)\n")
			}
			if mcpAddr != "" {
				fmt.Fprintf(os.Stderr, "warning: --mcp ignored in --web-only mode (no daemon)\n")
			}
			return withPanicCapture(func() int { return hub.RunWebOnly(store, webListenAddr, insecureWeb) })
		}

		// Check managed mode: meta/instance/managed=true in the store.
		if isManaged(store) {
			return cmdStartManaged(store, plugins, chaosSeed, chaosRate)
		}

		if !store.Exists(configName) {
			// Config does not exist at all: try first-boot bootstrap.
			switch {
			case bootstrapConfigFromTemplate(store, configName):
				fmt.Fprintf(os.Stderr, "bootstrap: created config from template + discovery\n")
			case bootstrapFromDiscovery(store, configName):
				fmt.Fprintf(os.Stderr, "bootstrap: created config from interface discovery (DHCP + SSH)\n")
			default:
				fmt.Fprintf(os.Stderr, "error: no config found in database (run ze config edit first)\n")
				if webEnabled {
					fmt.Fprintf(os.Stderr, "hint: use --web-only to start the web UI without a daemon\n")
				}
				return 1
			}
		}

		applied, preChange := checkPushedConfig(store, configName, plugins)
		writeConfigActiveHash(store, configName)

		if applied {
			hr := newHealthRevert(store, configName)
			hr.Start(preChange)
			defer hr.Wait()
			hub.PeerLifecycleCallback = hr
		}

		// No config-type gate on --web. Every config the YANG schema accepts runs on
		// one daemon, so "the daemon cannot serve this config" is not a state a probe
		// can report: a config it cannot parse fails in runYANGConfig and says why.
		// The gate used to key on ConfigTypeUnknown, which covers every config with
		// no `bgp {}` block -- an interface-only or plugin-only config among them.
		return withPanicCapture(func() int {
			return hub.Run(store, configName, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, cliEnabled)
		})
	})
}

// isManaged returns true if the store has meta/instance/managed=true. It reads
// the flag through the one reader every caller shares (managed.ManagedFlag).
// A flag that cannot be read starts the daemon unmanaged, as before that
// reader existed: ze doctor reports the store that cannot answer.
func isManaged(store storage.Storage) bool {
	flag, err := managed.ManagedFlag(store)
	if err != nil {
		return false
	}
	return flag
}

// cmdStartManaged handles ze start for managed clients.
// With cached config: starts BGP immediately, connects to hub in background for updates.
// Without cached config (first boot): requires hub connection to fetch initial config.
func cmdStartManaged(store storage.Storage, plugins []string, chaosSeed int64, chaosRate float64) int {
	configName, err := internalresolve.DefaultConfig(store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	// The daemon is handed a builder, never a built client: it builds one each
	// time it starts its hub client, at boot and at every `request fleet
	// enable`, from the configuration the store holds then. A start whose
	// configuration names no hub therefore still owns a hub client it can
	// start once the configuration names one.
	buildClient := func() (*managed.ClientConfig, error) {
		return extractManagedClientConfig(store, configName)
	}

	if store.Exists(configName) {
		return withPanicCapture(func() int {
			return hub.RunWithManagedClient(store, configName, plugins, chaosSeed, chaosRate, buildClient)
		})
	}

	// No cached config: first boot after ze init --managed.
	boot, err := readManagedBootstrap(store)
	if err != nil {
		return firstStartRefused(err)
	}
	if missing := boot.missing(); missing != nil {
		return firstStartRefused(missing)
	}

	var notice textbuf.Buffer
	notice.Str("managed: first boot, connecting to hub ").Str(boot.server).Str(" as ").Str(boot.name).Byte('\n').
		StdErr() //nolint:errcheck // stderr progress notice
	configData, kept, err := fetchInitialConfig(boot)
	if err != nil {
		return firstStartRefused(fetchRefusal(boot, err))
	}
	if kept != nil {
		// Stored before the config is cached, so no later connection is ever
		// made without it: first use happens once.
		if err := keepHubRoot(store, kept); err != nil {
			return firstStartRefused(err)
		}
	}

	// The fetched config is judged before it is cached. When it declares an
	// external plugin the full verdict is boot's (judgeConfigBeforeBoot), and
	// a refusal there removes the cached copy, below.
	judgedAtBoot, parseErr := judgeConfigBeforeBoot(configData, nil, plugins)
	if parseErr != nil {
		return firstStartRefused(opererr.Wrap(parseErr, "hub config failed validation",
			"a config that does not pass is never cached or started",
			"ze data import client-"+boot.name+".conf   (on the hub, after correcting that file)"))
	}

	if cacheErr := cacheFirstConfig(store, configName, configData); cacheErr != nil {
		return firstStartRefused(cacheErr)
	}

	if judgedAtBoot {
		hub.ConfigRefused = func(refusal error) {
			var tb textbuf.Buffer
			tb.Str("error: hub config failed validation: ").Err(refusal).
				Str("\nthe cached copy was removed, so the next start fetches the config from the hub again; ").
				Str("correct the config on the hub, then: ze start\n")
			tb.StdErr() //nolint:errcheck // stderr of a process that is exiting
			if rmErr := store.Remove(configName); rmErr != nil {
				var failed textbuf.Buffer
				failed.Str("error: the refused config could not be removed from the cache, so the next start reads it again: ").
					Err(rmErr).Str("\nremove it by hand, then: ze start\n")
				failed.StdErr() //nolint:errcheck // stderr of a process that is exiting
			}
		}
	}

	return withPanicCapture(func() int {
		return hub.RunWithManagedClient(store, configName, plugins, chaosSeed, chaosRate, buildClient)
	})
}

// cacheFirstConfig caches the configuration a first start fetched from the hub
// and records it as the device's baseline: the hash of the cached copy, read
// back from the store. The first start does not pass the push path
// (hub applyManagedPush), so this is where its baseline is written, before the
// managed client makes its first connection.
//
// A baseline that cannot be recorded refuses the start. The configuration
// stays cached, so the next start runs it with no baseline, which is a clean
// device: the fetch its client makes on connecting is answered "current" and
// records the baseline then.
func cacheFirstConfig(store storage.Storage, configName string, configData []byte) error {
	if writeErr := store.WriteFile(configName, configData, 0); writeErr != nil {
		return opererr.Wrap(writeErr, "the config fetched from the hub could not be cached",
			"a managed client starts from its cached copy", "ze doctor")
	}
	if _, baselineErr := managed.RecordActiveAsBaseline(store); baselineErr != nil {
		return opererr.Wrap(baselineErr, "the baseline of the config fetched from the hub could not be recorded",
			"the baseline is how this device tells a local change from the configuration its hub gave it",
			"ze show data check")
	}
	return nil
}

// extractManagedClientConfig builds the hub client configuration from the
// active version of the config the store holds under configName now: the hub
// its client block names and the callbacks that read this store. It is the
// daemon's builder (hub.ManagedClientBuilder), called at each start of the hub
// client. It answers managed.ErrNoHubClient for a config with no hub client
// block, and an operator error naming the config, with its fix, when the
// config or its hub block cannot be read; the daemon reports either to
// whoever asked for the client.
func extractManagedClientConfig(store storage.Storage, configName string) (*managed.ClientConfig, error) {
	// The configuration the device runs is the version the active pointer
	// names, as `request fleet enable` checks it and as the hold and the fleet
	// status read it (managed.ActiveVersion): the plain mirror can differ.
	data, err := storage.ReadActiveConfig(store, configName)
	if err != nil {
		return nil, opererr.Wrap(err,
			"the configuration "+configName+" could not be read from the store",
			"its plugin hub block names the hub this device follows, so no hub client was built",
			"ze show data check")
	}

	endpoint, err := hubClientEndpoint(data)
	if errors.Is(err, managed.ErrNoHubClient) {
		return nil, err
	}
	if err != nil {
		return nil, opererr.Wrap(err,
			"the plugin hub block of the configuration "+configName+" could not be read",
			"it names the hub this device follows, so no hub client was built",
			"ze config validate")
	}

	return &managed.ClientConfig{
		Name:          endpoint.Name,
		Server:        endpoint.Server,
		Token:         endpoint.Token,
		TLSInsecure:   env.GetBool("ze.managed.tls.insecure", false),
		SourceAddress: endpoint.SourceAddress,
		// Without this the configured leaf reaches nothing, so a client
		// configured through its config file falls through to the system CA it
		// cannot satisfy.
		CA: endpoint.CA,
		// The root ze init stored or the first start kept. It anchors the hub
		// when the config names no ca.
		HubRoot:        storedHubRoot(store),
		ConnectTimeout: env.GetDuration("ze.managed.connect.timeout", 5*time.Second),
		// The same reading for every config the hub pushes later, so a push
		// that changes the hub block changes the running client.
		Endpoint: hubClientEndpoint,
		Handler: &managed.Handler{
			Validate: func(cfgData []byte) error {
				_, parseErr := config.ReadConfig(string(cfgData), "", nil)
				return parseErr
			},
		},
		CheckManaged: func() bool {
			return isManaged(store)
		},
		// The hold: before each fetch the client compares the configuration
		// this store holds as active with the last one the hub gave, and
		// applies nothing the hub sends while they differ. This is the one
		// place the client is given the store's baseline, beside the other
		// callback that reads the store.
		Baseline: managed.NewBaselineAccess(store),
	}, nil
}

// hubClientEndpoint reads the hub connection the config in data declares: its
// first hub-level client block, with the troubleshooting overrides applied on
// top (ze.managed.server, ze.managed.name and ze.managed.token, which
// --server, --name and --token set). It answers managed.ErrNoHubClient for a
// config with no such block: an override replaces a value and never stands in
// for the block.
func hubClientEndpoint(data []byte) (managed.Endpoint, error) {
	// The block itself is read by the managed component, which `show fleet
	// status` asks too, so the two cannot disagree on which block is the hub.
	endpoint, err := managed.DeclaredEndpoint(data)
	if err != nil {
		return managed.Endpoint{}, err
	}
	if server := env.Get("ze.managed.server"); server != "" {
		endpoint.Server = server
	}
	if name := env.Get("ze.managed.name"); name != "" {
		endpoint.Name = name
	}
	if token := env.Get("ze.managed.token"); token != "" {
		endpoint.Token = token
	}
	return endpoint, nil
}

// firstStartRefused prints why the first start of a managed client stopped
// and returns its exit code.
func firstStartRefused(refusal error) int {
	var output textbuf.Buffer
	output.Str("error: ").Err(refusal).Byte('\n').StdErr() //nolint:errcheck // The start exits with failure after its diagnostic.
	return 1
}

// fetchRefusal says why the first fetch failed and what fixes it. A hub whose
// certificate the client could not verify is fixed at ze init, where the
// client is told how to authenticate its hub; any other failure is a hub the
// client could not reach or that refused it.
func fetchRefusal(boot managedBootstrap, cause error) error {
	if _, unverified := errors.AsType[*tls.CertificateVerificationError](cause); unverified {
		return opererr.Wrap(cause, "fetch config from hub "+boot.server+": the hub's certificate could not be verified",
			"the client sends its secret only to a hub it has authenticated",
			"ze init --managed --force --hub-ca <file>   (the file holds what 'ze show pki local-ca pem' prints on the hub)")
	}
	if errors.Is(cause, managed.ErrHubPresentedNoAuthority) {
		return opererr.Wrap(cause, "fetch config from hub "+boot.server+": the hub presented no certificate authority to keep",
			"trust on first use keeps the authority the hub presents after its certificate",
			"ze init --managed --force --hub-ca <file>   (the file holds what 'ze show pki local-ca pem' prints on the hub)")
	}
	return opererr.Wrap(cause, "fetch config from hub "+boot.server+" as "+boot.name+" failed",
		"a managed client with no cached config has nothing to start from",
		"show doctor   (on the hub: it lists a client it refused and why)")
}

// managedBootstrap is what a managed client with no cached config connects
// to its hub with. Each value comes from --server, --name or --token (the
// ze.managed keys they set) when given, and from what ze init --managed
// stored otherwise.
type managedBootstrap struct {
	server string // Hub address (host:port).
	name   string // Client name.
	token  string // Client secret.
	// root is the hub root ze init stored (--hub-ca). nil when none is stored.
	root *x509.Certificate
	// firstUse says the operator asked for the root the hub presents to be
	// kept (ze init --hub-trust-first-use) and none is stored yet.
	firstUse bool
}

// storeText reads the text of one store key, and "" when the store does not
// hold it. A key that exists and cannot be read is an error.
func storeText(store storage.Storage, key string) (string, error) {
	if !store.Exists(key) {
		return "", nil
	}
	data, err := store.ReadFile(key)
	if err != nil {
		return "", opererr.New("the store key "+key+" could not be read",
			"a managed client reads it to reach its hub on the first start", "ze doctor")
	}
	return strings.TrimSpace(string(data)), nil
}

// readManagedBootstrap gathers what the first start connects with.
func readManagedBootstrap(store storage.Storage) (managedBootstrap, error) {
	var boot managedBootstrap
	var err error
	if boot.server = env.Get("ze.managed.server"); boot.server == "" {
		if boot.server, err = storeText(store, zefs.KeyManagedHubAddress.Pattern); err != nil {
			return managedBootstrap{}, err
		}
	}
	if boot.name = env.Get("ze.managed.name"); boot.name == "" {
		if boot.name, err = storeText(store, zefs.KeyInstanceName.Pattern); err != nil {
			return managedBootstrap{}, err
		}
	}
	if boot.token = env.Get("ze.managed.token"); boot.token == "" {
		if boot.token, err = storeText(store, zefs.KeyManagedHubToken.Pattern); err != nil {
			return managedBootstrap{}, err
		}
	}
	rootPEM, err := storeText(store, zefs.KeyManagedHubCA.Pattern)
	if err != nil {
		return managedBootstrap{}, err
	}
	if rootPEM != "" {
		if boot.root, err = parseHubRoot([]byte(rootPEM)); err != nil {
			return managedBootstrap{}, err
		}
		return boot, nil
	}
	trust, err := storeText(store, zefs.KeyManagedHubTrust.Pattern)
	if err != nil {
		return managedBootstrap{}, err
	}
	boot.firstUse = trust == zefs.ManagedHubTrustFirstUse
	return boot, nil
}

// missing names the first value the first start cannot connect without, or
// nil when it has them all.
func (boot managedBootstrap) missing() error {
	if boot.server == "" {
		return opererr.New("managed mode with no cached config, and this client does not know where its hub is",
			"the first start fetches the config from the hub, whose address ze init --managed stores, "+
				"or --server (ze.managed.server) gives", "ze --server <hub-host:port> start")
	}
	if boot.name == "" {
		return opererr.New("this managed client has no config yet and has no name to give its hub",
			"the hub serves each client the config stored under its name", "ze --name <client-name> start")
	}
	if boot.token == "" {
		return opererr.New("this managed client has no config yet and has no secret to authenticate to its hub with",
			"the hub admits a client by the secret of its entry under plugin hub server", "ze --token <secret> start")
	}
	return nil
}

// parseHubRoot reads the hub root the store holds.
func parseHubRoot(rootPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(rootPEM)
	if block == nil {
		return nil, opererr.New("the hub root in the store key "+zefs.KeyManagedHubCA.Pattern+" is not PEM",
			"the client authenticates its hub against that root", "ze init --managed --force --hub-ca <file>")
	}
	root, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, opererr.New("the hub root in the store key "+zefs.KeyManagedHubCA.Pattern+" is not a certificate",
			"the client authenticates its hub against that root", "ze init --managed --force --hub-ca <file>")
	}
	return root, nil
}

// storedHubRoot returns the hub root the store holds, and nil when it holds
// none or holds one that cannot be read. An unreadable root is logged: the
// client then authenticates its hub the way its config says, and a config
// that names no ca fails closed against the system pool.
func storedHubRoot(store storage.Storage) *x509.Certificate {
	rootPEM, err := storeText(store, zefs.KeyManagedHubCA.Pattern)
	if err != nil {
		slog.Warn("managed: the stored hub root could not be read", "error", err, "code", diagnostic.CodeConfigHubInvalid)
		return nil
	}
	if rootPEM == "" {
		return nil
	}
	root, err := parseHubRoot([]byte(rootPEM))
	if err != nil {
		slog.Warn("managed: the stored hub root could not be read", "error", err, "code", diagnostic.CodeConfigHubInvalid)
		return nil
	}
	return root
}

// keepHubRoot stores the authority a first-use connection kept and tells the
// operator which one it was, so it can be compared with the hub's own.
func keepHubRoot(store storage.Storage, root *x509.Certificate) error {
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw})
	if err := store.WriteKey(zefs.KeyManagedHubCA.Pattern, rootPEM); err != nil {
		return opererr.New("the certificate authority the hub presented could not be stored",
			"without it the next connection could not authenticate the hub", "ze doctor")
	}
	fingerprint := sha256.Sum256(root.Raw)
	var tb textbuf.Buffer
	tb.Str("managed: trusting the certificate authority the hub presented on this first connection: ").
		Str(root.Subject.String()).Str(", SHA-256 ").Str(hex.EncodeToString(fingerprint[:])).
		Str("\nevery later connection is authenticated against it; compare it with the hub's own: ze show pki local-ca pem\n")
	tb.StdErr() //nolint:errcheck // stderr notice
	return nil
}

// fetchInitialConfig connects to the hub, authenticates, and fetches the
// initial config. The second return value is the authority a first-use
// connection kept, and nil on every other connection: the caller MUST store it
// before any later connection is made.
//
// ze.managed.connect.timeout bounds reaching the hub only: the dial, the TLS
// handshake, the auth line and the read of its answer. The fetch runs under a
// context not derived from that bound, so managed.FetchConfig's own bound
// (configFetchTimeout, 30 seconds) alone ends it, as it ends every fetch of a
// running client: a large first configuration on an ordinary link takes
// longer than reaching the hub does.
func fetchInitialConfig(boot managedBootstrap) ([]byte, *x509.Certificate, error) {
	server, name, token := boot.server, boot.name, boot.token
	connectCtx, cancel := context.WithTimeout(context.Background(), env.GetDuration("ze.managed.connect.timeout", 5*time.Second))
	defer cancel()

	// The SAME rules RunManagedClient uses, through managed.ClientTLSConfig
	// rather than a second tls.Config spelled here. This path is first boot:
	// there is no config yet, so it carries the token to a hub the client has
	// never spoken to, and it was the least authenticated exchange of them all.
	// No config means no pki ca entry to name, so the anchor here is the root
	// ze init stored (--hub-ca). With none stored and first use asked for
	// (--hub-trust-first-use), this one connection keeps the authority the hub
	// presents. With neither, the anchor is the system pool, which fails
	// closed against a privately issued hub.
	keptRoot := func() *x509.Certificate { return nil }
	var tlsConf *tls.Config
	if boot.firstUse {
		tlsConf, keptRoot = managed.FirstUseTLSConfig(server)
	} else {
		var tlsErr error
		tlsConf, tlsErr = managed.ClientTLSConfig(&managed.ClientConfig{
			Server:      server,
			HubRoot:     boot.root,
			TLSInsecure: env.GetBool("ze.managed.tls.insecure", false),
		})
		if tlsErr != nil {
			return nil, nil, fmt.Errorf("hub trust anchor: %w", tlsErr)
		}
	}

	conn, err := (&tls.Dialer{Config: tlsConf}).DialContext(connectCtx, "tcp", server)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", server, err)
	}
	defer conn.Close() //nolint:errcheck // cleanup

	if err := pluginipc.SendAuth(connectCtx, conn, token, name); err != nil {
		return nil, nil, fmt.Errorf("auth: %w", err)
	}

	authDeadline, _ := connectCtx.Deadline() // Always set: WithTimeout above.
	_ = conn.SetReadDeadline(authDeadline)
	authLine, readErr := readAuthLine(conn, 512)
	if readErr != nil {
		return nil, nil, fmt.Errorf("read auth response: %w", readErr)
	}
	_ = conn.SetReadDeadline(time.Time{})

	_, verb, _, parseErr := rpc.ParseLine(authLine)
	if parseErr != nil || verb != "ok" {
		return nil, nil, errAuthRejected
	}

	rc := rpc.NewConn(conn, conn)
	mc := rpc.NewMuxConn(rc)
	defer mc.Close() //nolint:errcheck // cleanup

	// Not connectCtx: the connect bound would cut a large first fetch.
	resp, err := managed.FetchConfig(context.Background(), mc, "")
	if err != nil {
		return nil, nil, err
	}

	if resp.Config == "" {
		return nil, nil, errHubReturnedEmptyConfig
	}

	data, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("decode config: %w", err)
	}

	return data, keptRoot(), nil
}

// readAuthLine reads from conn byte-by-byte until newline or maxSize.
func readAuthLine(conn net.Conn, maxSize int) ([]byte, error) {
	buf := make([]byte, 0, 128)
	b := make([]byte, 1)
	for {
		n, err := conn.Read(b)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if b[0] == '\n' {
			if len(buf) > 0 && buf[len(buf)-1] == '\r' {
				buf = buf[:len(buf)-1]
			}
			return buf, nil
		}
		buf = append(buf, b[0])
		if len(buf) >= maxSize {
			return nil, fmt.Errorf("auth response exceeds %d bytes", maxSize)
		}
	}
}

// bootstrapConfigFromTemplate reads file/template/ze.conf from zefs,
// runs interface discovery, merges them, and writes the result to the
// active config. When discovery succeeds, explicit DHCPv4 is added
// to every ethernet interface so the machine gets an IP regardless of
// which port has a cable, without relying on runtime re-discovery via
// dhcp-auto.
func bootstrapConfigFromTemplate(store storage.Storage, configName string) bool {
	templateKey := zefs.KeyFileTemplate.Key("ze.conf")
	tmpl, err := store.ReadFile(templateKey)
	if err != nil {
		return false
	}

	var merged []byte
	if loadErr := iface.LoadBackend("netlink"); loadErr != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: netlink backend unavailable: %v\n", loadErr)
		merged = tmpl
	} else {
		discovered, discErr := iface.DiscoverInterfaces()
		if closeErr := iface.CloseBackend(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: bootstrap: close backend: %v\n", closeErr)
		}
		if discErr != nil || len(discovered) == 0 {
			merged = tmpl
		} else {
			ifaceCfg := iface.EmitSetConfigWithDHCP(discovered)
			merged = make([]byte, 0, len(tmpl)+1+len(ifaceCfg))
			merged = append(merged, tmpl...)
			merged = append(merged, '\n')
			merged = append(merged, []byte(ifaceCfg)...)
		}
	}

	activeKey := zefs.KeyFileActive.Key(configName)
	if writeErr := store.WriteFile(activeKey, merged, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bootstrap: write config: %v\n", writeErr)
		return false
	}
	return true
}

// bootstrapFromDiscovery generates a minimal config from interface discovery
// when no config and no template exist.
func bootstrapFromDiscovery(store storage.Storage, configName string) bool {
	if loadErr := iface.LoadBackend("netlink"); loadErr != nil {
		return false
	}
	discovered, discErr := iface.DiscoverInterfaces()
	if closeErr := iface.CloseBackend(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bootstrap: close backend: %v\n", closeErr)
	}
	if discErr != nil {
		return false
	}

	cfg := iface.EmitBootstrapConfig(discovered)
	if cfg == "" {
		return false
	}

	activeKey := zefs.KeyFileActive.Key(configName)
	if writeErr := store.WriteFile(activeKey, []byte(cfg), 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bootstrap: write config: %v\n", writeErr)
		return false
	}
	return true
}
