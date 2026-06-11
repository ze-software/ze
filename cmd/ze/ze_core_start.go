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

	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/managed"
	pluginipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	internalresolve "codeberg.org/thomas-mangin/ze/internal/core/resolve"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/fleet"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
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
		Summary: "Start the Ze daemon from blob storage",
		Usage:   []string{"ze start [options]"},
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
				{Name: "ze init", Desc: "Bootstrap database (required before first start)"},
				{Name: "ze config edit", Desc: "Create or edit configuration"},
			}},
		},
		Examples: []string{
			"ze start                           Start daemon with default config",
			"ze start --cli                     Start daemon and attach interactive CLI",
			"ze start --web 3443                Start with web UI on port 3443",
			"ze start --web 3443 --insecure-web Start with web UI, no auth (localhost)",
			"ze start --web-only --web 3443     Web UI only (no operational commands)",
		},
	}
	p.Write()
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

	store := resolveStorage()
	defer store.Close() //nolint:errcheck // best-effort cleanup

	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "error: ze start requires blob storage (run ze init first)\n")
		return 1
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
		hr := NewHealthRevert(store, configName)
		hr.Start(preChange)
		hub.PeerLifecycleCallback = hr
	}

	ct := detectConfigType(store, configName)
	if ct == config.ConfigTypeUnknown && webEnabled {
		fmt.Fprintf(os.Stderr, "error: config %q has unknown type; cannot start daemon with --web\n", configName)
		fmt.Fprintf(os.Stderr, "hint: use --web-only to start the web UI without a daemon\n")
		return 1
	}

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
		Name:        cli.Name,
		Server:      cli.Address(),
		Token:       cli.Secret,
		TLSInsecure: env.GetBool("ze.managed.tls.insecure", false),
		Version:     fleet.VersionHash(data),
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
