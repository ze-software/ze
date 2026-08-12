// Design: docs/architecture/system-architecture.md -- ze start command and managed mode

//go:build ze_core

package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/cmd/ze/hub"
	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/managed"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/env"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// resolveStorage creates the appropriate storage backend.
// Default: blob storage at {configDir}/database.zefs.
// Fallback: filesystem if blob cannot be created or ZE_STORAGE_BLOB=false.
func resolveStorage() storage.Storage {
	s, err := internalresolve.Storage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: blob storage unavailable (%v), using filesystem\n", err)
	}
	return s
}

func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

func startUsage() {
	p := helpfmt.Page{
		Command: "ze start",
		Summary: "Start the Ze daemon from blob storage, or from an optional config file",
		Usage:   []string{"ze start [<config-file>] [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--cli", Desc: "Attach interactive CLI after startup"},
				{Name: "--web <port>", Desc: "Enable web UI on given port (requires config)"},
				{Name: "--web-only", Desc: "Start web UI only, no daemon (config editing only)"},
				{Name: "--insecure-web", Desc: "Disable web auth (binds to localhost only)"},
				{Name: "--mcp <port>", Desc: "Enable MCP server on given port"},
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
			if !strings.HasPrefix(args[i], "-") {
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
		case "--cli":
			cliEnabled = true
		case "--web":
			if i+1 < len(args) {
				i++
				if !validPort(args[i]) {
					fmt.Fprintf(os.Stderr, "error: --web port must be 1-65535, got %q\n", args[i])
					return 1
				}
				webPort = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "error: --web requires a port\n")
				return 1
			}
		case "--web-only":
			webOnly = true
		case "--insecure-web":
			insecureWeb = true
		case "--mcp":
			if i+1 < len(args) {
				i++
				if !validPort(args[i]) {
					fmt.Fprintf(os.Stderr, "error: --mcp port must be 1-65535, got %q\n", args[i])
					return 1
				}
				var tb textbuf.Buffer
				mcpAddr = tb.Str("127.0.0.1:").Str(args[i]).String()
			} else {
				fmt.Fprintf(os.Stderr, "error: --mcp requires a port\n")
				return 1
			}
		case "--mcp-token":
			if i+1 < len(args) {
				i++
				mcpToken = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "error: --mcp-token requires a value\n")
				return 1
			}
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

	// An explicit config path (ze start <config-file>) launches the daemon from
	// that file. Keyword-first grammar (ai/rules/cli.md R1) places the
	// path behind the `start` keyword; this is the SUPPORTED (and only) form. The
	// free-form positional path in zeDispatch (`ze <config-file>`) was REMOVED by
	// spec-fixit-config-file-positional-grammar; only the `-` stdin sentinel
	// remains there. This branch is the simple file-launch flow (blob-then-
	// filesystem fallback), NOT the managed/bootstrap blob-default path below,
	// which applies only when no explicit path is given.
	if configPath := startConfigPath(args); configPath != "" {
		if webOnly {
			fmt.Fprintf(os.Stderr, "error: --web-only cannot be combined with a config-file path\n")
			return 1
		}
		store := resolveStorage()
		configPath = config.ResolveConfigPath(configPath)
		if storage.IsBlobStorage(store) && !store.Exists(configPath) {
			if _, statErr := os.Stat(configPath); statErr != nil {
				store.Close() //nolint:errcheck // closing blob before filesystem fallback
				store = storage.NewFilesystem()
			}
		}
		// One runtime for every config: hub.Run reads the file and parses it
		// against the YANG schema, whatever top-level blocks it declares.
		return withPanicCapture(func() int {
			return hub.Run(store, configPath, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, cliEnabled)
		})
	}

	store := resolveStorage()
	defer func() {
		if store != nil {
			store.Close() //nolint:errcheck // best-effort
		}
	}()

	if !storage.IsBlobStorage(store) {
		if env.IsEnabled("ze.gokrazy.enabled") {
			store.Close() //nolint:errcheck // closing filesystem fallback before re-resolve
			var initErr error
			store, initErr = gokrazyAutoInit()
			if initErr != nil {
				slog.Error("gokrazy auto-init failed", "error", initErr)
				return 1
			}
			slog.Info("gokrazy: auto-init fallback, created database")
		} else {
			fmt.Fprintf(os.Stderr, "error: ze start requires blob storage (run ze init first)\n")
			return 1
		}
	}

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

	// Check managed mode: meta/instance/managed=true in blob.
	if isManaged(store) {
		return cmdStartManaged(store, plugins, chaosSeed, chaosRate)
	}

	configName := internalresolve.DefaultConfig(store)
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

	applied, preChange := checkPushedConfig(store, configName)
	writeConfigActiveHash(store, configName)

	if applied {
		hr := newHealthRevert(store, configName)
		hr.Start(preChange)
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
}

// isManaged returns true if the blob has meta/instance/managed=true.
func isManaged(store storage.Storage) bool {
	data, err := store.ReadFile(zefs.KeyInstanceManaged.Pattern)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "true"
}

// cmdStartManaged handles ze start for managed clients.
// With cached config: starts BGP immediately, connects to hub in background for updates.
// Without cached config (first boot): requires hub connection to fetch initial config.
func cmdStartManaged(store storage.Storage, plugins []string, chaosSeed int64, chaosRate float64) int {
	configName := internalresolve.DefaultConfig(store)

	if store.Exists(configName) {
		clientCfg := extractManagedClientConfig(store, configName)

		return withPanicCapture(func() int {
			return hub.RunWithManagedClient(store, configName, plugins, chaosSeed, chaosRate, clientCfg)
		})
	}

	// No cached config: first boot after ze init --managed.
	server := env.Get("ze.managed.server")
	name := env.Get("ze.managed.name")
	token := env.Get("ze.managed.token")

	if server == "" || name == "" {
		fmt.Fprintf(os.Stderr, "error: managed mode with no cached config\n")
		fmt.Fprintf(os.Stderr, "hint: set ze.managed.server and ze.managed.name to bootstrap from hub\n")
		fmt.Fprintf(os.Stderr, "  export ZE_MANAGED_SERVER=hub-host:1791\n")
		fmt.Fprintf(os.Stderr, "  export ZE_MANAGED_NAME=edge-01\n")
		fmt.Fprintf(os.Stderr, "  export ZE_MANAGED_TOKEN=secret\n")
		return 1
	}
	if token == "" {
		fmt.Fprintf(os.Stderr, "error: ze.managed.token is required for first boot\n")
		return 1
	}

	fmt.Fprintf(os.Stderr, "managed: first boot, connecting to hub %s as %s\n", server, name)
	configData, err := fetchInitialConfig(server, name, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: fetch config from hub: %v\n", err)
		return 1
	}

	if _, parseErr := config.LoadConfig(string(configData), "", nil); parseErr != nil {
		fmt.Fprintf(os.Stderr, "error: hub config failed validation: %v\n", parseErr)
		return 1
	}

	if writeErr := store.WriteFile(configName, configData, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: cache config: %v\n", writeErr)
		return 1
	}

	clientCfg := extractManagedClientConfig(store, configName)

	return withPanicCapture(func() int {
		return hub.RunWithManagedClient(store, configName, plugins, chaosSeed, chaosRate, clientCfg)
	})
}

// extractManagedClientConfig reads config from blob and extracts the hub client block.
func extractManagedClientConfig(store storage.Storage, configName string) *managed.ClientConfig {
	data, err := store.ReadFile(configName)
	if err != nil {
		slog.Warn("managed: cannot read config for hub extraction", "config", configName, "error", err)
		return nil
	}

	loadResult, err := config.LoadConfig(string(data), "", nil)
	if err != nil {
		slog.Warn("managed: cannot parse config for hub extraction", "config", configName, "error", err)
		return nil
	}

	hubCfg, err := config.ExtractHubConfig(loadResult.Tree)
	if err != nil {
		slog.Warn("managed: cannot extract hub config", "error", err)
		return nil
	}
	if len(hubCfg.Clients) == 0 {
		return nil
	}

	cli := hubCfg.Clients[0]

	return &managed.ClientConfig{
		Name:          cli.Name,
		Server:        cli.Address(),
		Token:         cli.Secret,
		TLSInsecure:   env.GetBool("ze.managed.tls.insecure", false),
		SourceAddress: cli.SourceAddress,
		Version:       fleet.VersionHash(data),
		Handler: &managed.Handler{
			Validate: func(cfgData []byte) error {
				_, parseErr := config.LoadConfig(string(cfgData), "", nil)
				return parseErr
			},
		},
		CheckManaged: func() bool {
			return isManaged(store)
		},
	}
}

// fetchInitialConfig connects to the hub, authenticates, and fetches the initial config.
func fetchInitialConfig(server, name, token string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), env.GetDuration("ze.managed.connect.timeout", 5*time.Second))
	defer cancel()

	tlsInsecure := env.GetBool("ze.managed.tls.insecure", false)
	tlsConf := &tls.Config{
		InsecureSkipVerify: tlsInsecure, //nolint:gosec // opt-in via explicit env var
		MinVersion:         tls.VersionTLS13,
	}
	if tlsInsecure {
		slog.Warn("managed TLS: certificate verification disabled (insecure)")
	}

	conn, err := (&tls.Dialer{Config: tlsConf}).DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", server, err)
	}
	defer conn.Close() //nolint:errcheck // cleanup

	if err := pluginipc.SendAuth(ctx, conn, token, name); err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	authLine, readErr := readAuthLine(conn, 512)
	if readErr != nil {
		return nil, fmt.Errorf("read auth response: %w", readErr)
	}
	_ = conn.SetReadDeadline(time.Time{})

	_, verb, _, parseErr := rpc.ParseLine(authLine)
	if parseErr != nil || verb != "ok" {
		return nil, errAuthRejected
	}

	rc := rpc.NewConn(conn, conn)
	mc := rpc.NewMuxConn(rc)
	defer mc.Close() //nolint:errcheck // cleanup

	resp, err := managed.FetchConfig(ctx, mc, "")
	if err != nil {
		return nil, err
	}

	if resp.Config == "" {
		return nil, errHubReturnedEmptyConfig
	}

	data, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return data, nil
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
